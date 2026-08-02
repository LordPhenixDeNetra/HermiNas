//! Ships buffered batches to their destination (F-107: "compression +
//! batching vers le serveur"). The gRPC/mTLS transport to the Rust
//! data-plane receiver depends on the agent.proto contract (M0.3) and
//! stream/receiver.rs (M1.2) — neither exists yet. `FileShipper` below
//! exercises the *real* batching + zstd-compression path against a local
//! file, so the WAL → backpressure → ship loop is fully working and tested
//! today; a future `GrpcShipper` implements the same `Shipper` trait and
//! nothing upstream of it needs to change.

use std::io::{Read, Write};
use std::path::{Path, PathBuf};

use crate::buffer::wal::WalEntry;

pub trait Shipper: Send + Sync {
    fn ship(&self, batch: &[WalEntry]) -> Result<(), crate::AgentError>;
}

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

impl Shipper for FileShipper {
    fn ship(&self, batch: &[WalEntry]) -> Result<(), crate::AgentError> {
        if batch.is_empty() {
            return Ok(());
        }

        // Frame each record the same way the WAL does, then zstd-compress
        // the whole batch as one unit — mirrors the intended wire format.
        let mut framed = Vec::new();
        for entry in batch {
            framed.extend_from_slice(&(entry.data.len() as u32).to_le_bytes());
            framed.extend_from_slice(&entry.data);
        }

        let compressed = zstd::stream::encode_all(framed.as_slice(), 3)
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
/// round trip, and doubles as a worked example of the framing a future
/// GrpcShipper's receiver counterpart needs to match.
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

        let framed = zstd::stream::decode_all(compressed)
            .map_err(|e| crate::AgentError::Ship(format!("zstd decompress: {e}")))?;

        let mut records = Vec::new();
        let mut fpos = 0;
        while fpos < framed.len() {
            let rlen = u32::from_le_bytes(framed[fpos..fpos + 4].try_into().unwrap()) as usize;
            fpos += 4;
            records.push(framed[fpos..fpos + rlen].to_vec());
            fpos += rlen;
        }
        batches.push(records);
    }

    Ok(batches)
}

#[cfg(test)]
mod tests {
    use std::sync::atomic::{AtomicU32, Ordering};

    use super::*;

    #[test]
    fn ship_and_read_back_round_trip() {
        let dir = tempfile::tempdir().unwrap();
        let out = dir.path().join("shipped.bin");
        let shipper = FileShipper::new(&out);

        let batch = vec![
            WalEntry { offset: 10, data: b"one".to_vec() },
            WalEntry { offset: 20, data: b"two".to_vec() },
        ];
        shipper.ship(&batch).unwrap();

        let batches = read_shipped_batches(&out).unwrap();
        assert_eq!(batches.len(), 1);
        assert_eq!(batches[0], vec![b"one".to_vec(), b"two".to_vec()]);
    }

    #[test]
    fn multiple_ship_calls_append_separate_batches() {
        let dir = tempfile::tempdir().unwrap();
        let out = dir.path().join("shipped.bin");
        let shipper = FileShipper::new(&out);

        shipper
            .ship(&[WalEntry { offset: 1, data: b"a".to_vec() }])
            .unwrap();
        shipper
            .ship(&[WalEntry { offset: 2, data: b"b".to_vec() }])
            .unwrap();

        let batches = read_shipped_batches(&out).unwrap();
        assert_eq!(batches, vec![vec![b"a".to_vec()], vec![b"b".to_vec()]]);
    }

    /// Stands in for M1.1's "coupure réseau simulée → zéro perte après
    /// reconnexion" until a real gRPC shipper exists: a Shipper that fails
    /// a few times before succeeding must never lose WAL entries — they
    /// simply stay unacked until a ship() call finally succeeds. This
    /// exercises the exact retry contract the future GrpcShipper must
    /// satisfy.
    #[test]
    fn flaky_shipper_causes_zero_loss_until_it_recovers() {
        struct FlakyShipper {
            fail_times: AtomicU32,
        }
        impl Shipper for FlakyShipper {
            fn ship(&self, _batch: &[WalEntry]) -> Result<(), crate::AgentError> {
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

        // Retry loop mirroring what the agent's main ship tick does.
        for _ in 0..5 {
            let pending = wal.pending().unwrap();
            if pending.is_empty() {
                break;
            }
            if shipper.ship(&pending).is_ok() {
                wal.ack(pending.last().unwrap().offset).unwrap();
            }
            // On error: nothing acked, nothing lost — just retried.
        }

        assert!(
            wal.pending().unwrap().is_empty(),
            "the record must ship once the simulated outage clears"
        );
    }
}
