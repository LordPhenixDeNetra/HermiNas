//! A minimal, crash-safe write-ahead log (F-106): every record is appended
//! and fsynced before the agent considers it safely buffered. A separate,
//! small "ack" file records how far records have been durably shipped;
//! only the unacked tail is replayed after a crash, so a hard kill -9
//! between append and ship can never lose an already-acked record, and can
//! never silently drop an unacked one either — it just gets retried.

use std::fs::{File, OpenOptions};
use std::io::{self, BufReader, BufWriter, Read, Seek, SeekFrom, Write};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WalEntry {
    /// Byte offset in the data file *after* this record — pass this to
    /// `ack` once the entry (or a batch ending with it) has been shipped.
    pub offset: u64,
    pub data: Vec<u8>,
}

pub struct Wal {
    data_path: PathBuf,
    ack_path: PathBuf,
    writer: BufWriter<File>,
    write_offset: u64,
    ack_offset: u64,
}

impl Wal {
    /// Opens (or creates) the WAL under `dir`. If the process crashed after
    /// appending records but before shipping them, those records are still
    /// on disk past the last acked offset — `pending()` will replay them.
    pub fn open(dir: impl AsRef<Path>) -> Result<Self, crate::AgentError> {
        let dir = dir.as_ref();
        std::fs::create_dir_all(dir)?;

        let data_path = dir.join("wal.log");
        let ack_path = dir.join("wal.ack");

        let ack_offset = read_ack_offset(&ack_path)?;

        let file = OpenOptions::new()
            .create(true)
            .read(true)
            .append(true)
            .open(&data_path)?;
        let write_offset = file.metadata()?.len();

        Ok(Wal {
            data_path,
            ack_path,
            writer: BufWriter::new(file),
            write_offset,
            ack_offset,
        })
    }

    /// Appends a length-prefixed record, fsyncs it, and returns the offset
    /// usable as an ack cursor once it (or a later record) has shipped.
    pub fn append(&mut self, data: &[u8]) -> Result<u64, crate::AgentError> {
        self.writer.write_all(&(data.len() as u32).to_le_bytes())?;
        self.writer.write_all(data)?;
        self.writer.flush()?;
        self.writer.get_ref().sync_data()?;
        self.write_offset += 4 + data.len() as u64;
        Ok(self.write_offset)
    }

    /// Every record appended after the current ack offset: unshipped work,
    /// whether because it just hasn't been drained yet or because the
    /// process crashed before shipping it last time.
    pub fn pending(&self) -> Result<Vec<WalEntry>, crate::AgentError> {
        let mut file = File::open(&self.data_path)?;
        file.seek(SeekFrom::Start(self.ack_offset))?;
        let mut reader = BufReader::new(file);

        let mut entries = Vec::new();
        let mut cursor = self.ack_offset;

        loop {
            let mut len_buf = [0u8; 4];
            match reader.read_exact(&mut len_buf) {
                Ok(()) => {}
                Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => break,
                Err(e) => return Err(e.into()),
            }
            let len = u32::from_le_bytes(len_buf) as usize;
            let mut data = vec![0u8; len];
            reader.read_exact(&mut data)?;
            cursor += 4 + len as u64;
            entries.push(WalEntry { offset: cursor, data });
        }

        Ok(entries)
    }

    /// Marks everything up to `offset` as durably shipped. The ack file is
    /// fsynced before this returns, so an ack survives a crash right after.
    pub fn ack(&mut self, offset: u64) -> Result<(), crate::AgentError> {
        write_ack_offset(&self.ack_path, offset)?;
        self.ack_offset = offset;
        Ok(())
    }

    /// Bytes appended but not yet acked — what `backpressure::Backpressure`
    /// throttles against.
    pub fn pending_bytes(&self) -> u64 {
        self.write_offset.saturating_sub(self.ack_offset)
    }

    /// Rewrites the data file to drop already-acked bytes, bounding disk
    /// usage. Not required for correctness (only `pending()`/`ack()` are),
    /// so call it opportunistically — e.g. once acked bytes are a large
    /// fraction of the file. Any `WalEntry.offset` obtained before a
    /// compact() is no longer valid; call `pending()` again afterward.
    pub fn compact(&mut self) -> Result<(), crate::AgentError> {
        let remaining = self.pending()?;

        let tmp_path = self.data_path.with_extension("log.compact");
        {
            let mut tmp = BufWriter::new(File::create(&tmp_path)?);
            for entry in &remaining {
                tmp.write_all(&(entry.data.len() as u32).to_le_bytes())?;
                tmp.write_all(&entry.data)?;
            }
            tmp.flush()?;
            tmp.get_ref().sync_all()?;
        }
        std::fs::rename(&tmp_path, &self.data_path)?;

        let file = OpenOptions::new()
            .create(true)
            .read(true)
            .append(true)
            .open(&self.data_path)?;
        self.write_offset = file.metadata()?.len();
        self.writer = BufWriter::new(file);

        self.ack_offset = 0;
        write_ack_offset(&self.ack_path, 0)?;

        Ok(())
    }
}

fn read_ack_offset(path: &Path) -> Result<u64, crate::AgentError> {
    match std::fs::read_to_string(path) {
        Ok(s) => Ok(s.trim().parse().unwrap_or(0)),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(0),
        Err(e) => Err(e.into()),
    }
}

fn write_ack_offset(path: &Path, offset: u64) -> Result<(), crate::AgentError> {
    let mut f = File::create(path)?;
    f.write_all(offset.to_string().as_bytes())?;
    f.sync_all()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn append_and_pending_round_trip() {
        let dir = tempfile::tempdir().unwrap();
        let mut wal = Wal::open(dir.path()).unwrap();

        wal.append(b"one").unwrap();
        wal.append(b"two").unwrap();

        let pending = wal.pending().unwrap();
        assert_eq!(pending.len(), 2);
        assert_eq!(pending[0].data, b"one");
        assert_eq!(pending[1].data, b"two");
    }

    #[test]
    fn ack_removes_entries_from_pending() {
        let dir = tempfile::tempdir().unwrap();
        let mut wal = Wal::open(dir.path()).unwrap();

        wal.append(b"one").unwrap();
        let offset_after_two = wal.append(b"two").unwrap();
        wal.append(b"three").unwrap();

        wal.ack(offset_after_two).unwrap();

        let pending = wal.pending().unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].data, b"three");
    }

    #[test]
    fn crash_recovery_replays_unacked_only() {
        let dir = tempfile::tempdir().unwrap();

        {
            let mut wal = Wal::open(dir.path()).unwrap();
            wal.append(b"one").unwrap();
            let ack_offset = wal.append(b"two").unwrap();
            wal.ack(ack_offset).unwrap();
            wal.append(b"three").unwrap(); // never acked: simulates a crash right here
        } // Wal dropped without any special "close" — that's the point.

        let wal = Wal::open(dir.path()).unwrap();
        let pending = wal.pending().unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].data, b"three");
    }

    #[test]
    fn compact_reclaims_acked_space_and_keeps_unacked_data() {
        let dir = tempfile::tempdir().unwrap();
        let mut wal = Wal::open(dir.path()).unwrap();

        wal.append(b"one").unwrap();
        let ack_offset = wal.append(b"two").unwrap();
        wal.ack(ack_offset).unwrap();
        wal.append(b"three").unwrap();

        let size_before = std::fs::metadata(dir.path().join("wal.log")).unwrap().len();
        wal.compact().unwrap();
        let size_after = std::fs::metadata(dir.path().join("wal.log")).unwrap().len();

        assert!(size_after < size_before, "compact should shrink the data file");

        let pending = wal.pending().unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].data, b"three");
    }

    #[test]
    fn pending_bytes_reflects_unacked_size() {
        let dir = tempfile::tempdir().unwrap();
        let mut wal = Wal::open(dir.path()).unwrap();

        assert_eq!(wal.pending_bytes(), 0);
        let offset = wal.append(b"hello").unwrap();
        assert_eq!(wal.pending_bytes(), offset);

        wal.ack(offset).unwrap();
        assert_eq!(wal.pending_bytes(), 0);
    }
}
