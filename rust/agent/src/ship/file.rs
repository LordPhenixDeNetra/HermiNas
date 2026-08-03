use std::io::{Read, Write};
use std::path::{Path, PathBuf};

use super::Shipper;
use crate::buffer::wal::WalEntry;

/// Ships to a local file instead of a network receiver: used by tests, and
/// as the dev-mode default before `receiver_addr` is configured (see
/// config::AgentConfig). Real batching + zstd-compression, same framing
/// `GrpcShipper` sends over the wire, just written to disk instead.
pub struct FileShipper {
    output_path: PathBuf,
}

impl FileShipper {
    pub fn new(output_path: impl Into<PathBuf>) -> Self {
        Self {
            output_path: output_path.into(),
        }
    }
}

#[async_trait::async_trait]
impl Shipper for FileShipper {
    async fn ship(&self, batch: &[WalEntry]) -> Result<(), crate::AgentError> {
        if batch.is_empty() {
            return Ok(());
        }

        let compressed = super::grpc::encode_batch(batch)
            .map_err(|e| crate::AgentError::Ship(format!("zstd compress: {e}")))?;

        let mut file = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.output_path)?;

        // Length-prefix each compressed batch so a reader can split the
        // file back into the individual ship() calls that produced it.
        file.write_all(&(compressed.len() as u32).to_le_bytes())?;
        file.write_all(&compressed)?;
        Ok(())
    }
}

/// Reads back everything a FileShipper wrote: one entry per `ship()` call,
/// each a list of the records in that batch. Used by tests to prove the
/// round trip, and matches exactly what the receiver decodes from a
/// GrpcShipper's `BatchRequest.compressed_payload`.
pub fn read_shipped_batches(path: impl AsRef<Path>) -> Result<Vec<Vec<Vec<u8>>>, crate::AgentError> {
    let mut file = std::fs::File::open(path)?;
    let mut buf = Vec::new();
    file.read_to_end(&mut buf)?;

    let mut batches = Vec::new();
    let mut pos = 0;
    while pos < buf.len() {
        let len = u32::from_le_bytes(buf[pos..pos + 4].try_into().unwrap()) as usize;
        pos += 4;
        let compressed = &buf[pos..pos + len];
        pos += len;

        batches.push(super::grpc::decode_batch(compressed)?);
    }

    Ok(batches)
}

#[cfg(test)]
mod tests {
    use std::sync::atomic::{AtomicU32, Ordering};

    use super::*;

    #[tokio::test]
    async fn ship_and_read_back_round_trip() {
        let dir = tempfile::tempdir().unwrap();
        let out = dir.path().join("shipped.bin");
        let shipper = FileShipper::new(&out);

        let batch = vec![
            WalEntry { offset: 10, data: b"one".to_vec() },
            WalEntry { offset: 20, data: b"two".to_vec() },
        ];
        shipper.ship(&batch).await.unwrap();

        let batches = read_shipped_batches(&out).unwrap();
        assert_eq!(batches.len(), 1);
        assert_eq!(batches[0], vec![b"one".to_vec(), b"two".to_vec()]);
    }

    #[tokio::test]
    async fn multiple_ship_calls_append_separate_batches() {
        let dir = tempfile::tempdir().unwrap();
        let out = dir.path().join("shipped.bin");
        let shipper = FileShipper::new(&out);

        shipper
            .ship(&[WalEntry { offset: 1, data: b"a".to_vec() }])
            .await
            .unwrap();
        shipper
            .ship(&[WalEntry { offset: 2, data: b"b".to_vec() }])
            .await
            .unwrap();

        let batches = read_shipped_batches(&out).unwrap();
        assert_eq!(batches, vec![vec![b"a".to_vec()], vec![b"b".to_vec()]]);
    }

    /// Stands in for M1.1's "coupure réseau simulée → zéro perte après
    /// reconnexion": a Shipper that fails a few times before succeeding
    /// must never lose WAL entries — they simply stay unacked until a
    /// ship() call finally succeeds. GrpcShipper (M1.2) must satisfy the
    /// exact same retry contract against a real network.
    #[tokio::test]
    async fn flaky_shipper_causes_zero_loss_until_it_recovers() {
        struct FlakyShipper {
            fail_times: AtomicU32,
        }
        #[async_trait::async_trait]
        impl Shipper for FlakyShipper {
            async fn ship(&self, _batch: &[WalEntry]) -> Result<(), crate::AgentError> {
                let prev = self.fail_times.load(Ordering::SeqCst);
                if prev > 0 {
                    self.fail_times.store(prev - 1, Ordering::SeqCst);
                    return Err(crate::AgentError::Ship("simulated network outage".into()));
                }
                Ok(())
            }
        }

        let dir = tempfile::tempdir().unwrap();
        let mut wal = crate::buffer::wal::Wal::open(dir.path()).unwrap();
        wal.append(b"critical-event").unwrap();

        let shipper = FlakyShipper { fail_times: AtomicU32::new(2) };

        for _ in 0..5 {
            let pending = wal.pending().unwrap();
            if pending.is_empty() {
                break;
            }
            if shipper.ship(&pending).await.is_ok() {
                wal.ack(pending.last().unwrap().offset).unwrap();
            }
        }

        assert!(
            wal.pending().unwrap().is_empty(),
            "the record must ship once the simulated outage clears"
        );
    }
}
