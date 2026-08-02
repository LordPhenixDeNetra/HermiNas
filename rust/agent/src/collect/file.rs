//! Tails a log file (F-102): reads new complete lines since the last
//! checkpoint, joins multiline records, persists the checkpoint so a
//! restart resumes where it left off, and detects rotation (inode change
//! or truncation) to restart cleanly from the top of the new file.

use std::fs::File;
use std::io::{BufRead, BufReader, Seek, SeekFrom};
use std::path::{Path, PathBuf};

use regex::Regex;

pub struct FileTailer {
    path: PathBuf,
    checkpoint_path: PathBuf,
    offset: u64,
    inode: Option<u64>,
    multiline_start: Option<Regex>,
    pending_record: Option<String>,
}

impl FileTailer {
    pub fn new(path: impl Into<PathBuf>, multiline_start: Option<&str>) -> Result<Self, crate::AgentError> {
        let path = path.into();
        let checkpoint_path = checkpoint_path_for(&path);
        let (offset, inode) = read_checkpoint(&checkpoint_path);

        let multiline_start = multiline_start
            .map(Regex::new)
            .transpose()
            .map_err(|e| crate::AgentError::Config(format!("invalid multiline_start regex: {e}")))?;

        Ok(Self {
            path,
            checkpoint_path,
            offset,
            inode,
            multiline_start,
            pending_record: None,
        })
    }

    /// Returns whatever new, complete records are available since the last
    /// poll and persists the new checkpoint. A partial (unterminated) line
    /// at EOF is left for the next poll rather than emitted early.
    pub fn poll(&mut self) -> Result<Vec<String>, crate::AgentError> {
        let file = match File::open(&self.path) {
            Ok(f) => f,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(Vec::new()),
            Err(e) => return Err(e.into()),
        };

        let meta = file.metadata()?;
        let current_inode = unix_inode(&meta);

        if let Some(prev) = self.inode {
            if (current_inode.is_some() && current_inode != Some(prev)) || meta.len() < self.offset {
                // Rotated to a new inode, or truncated in place: restart
                // from the top rather than seeking past the end.
                self.offset = 0;
                self.pending_record = None;
            }
        }
        self.inode = current_inode;

        let mut reader = BufReader::new(file);
        reader.seek(SeekFrom::Start(self.offset))?;

        let mut records = Vec::new();
        let mut line = String::new();
        loop {
            line.clear();
            let n = reader.read_line(&mut line)?;
            if n == 0 || !line.ends_with('\n') {
                break; // no more data, or a partial line still being written
            }
            self.offset += n as u64;
            self.push_line(line.trim_end_matches('\n').to_string(), &mut records);
        }

        write_checkpoint(&self.checkpoint_path, self.offset, self.inode);
        Ok(records)
    }

    fn push_line(&mut self, line: String, records: &mut Vec<String>) {
        let Some(re) = &self.multiline_start else {
            // No multiline pattern configured: every line is its own
            // record, emitted immediately — nothing to buffer.
            records.push(line);
            return;
        };

        if re.is_match(&line) {
            if let Some(prev) = self.pending_record.take() {
                records.push(prev);
            }
            self.pending_record = Some(line);
        } else if let Some(prev) = self.pending_record.as_mut() {
            prev.push('\n');
            prev.push_str(&line);
        } else {
            // Continuation line with no record started yet: still emit it
            // rather than silently dropping data.
            records.push(line);
        }
    }

    /// Flushes an in-progress multiline record. Call on shutdown, once you
    /// know no continuation line is still in flight.
    pub fn flush_pending(&mut self) -> Option<String> {
        self.pending_record.take()
    }
}

fn checkpoint_path_for(path: &Path) -> PathBuf {
    let mut name = path.file_name().unwrap_or_default().to_os_string();
    name.push(".checkpoint");
    path.with_file_name(name)
}

#[cfg(unix)]
fn unix_inode(meta: &std::fs::Metadata) -> Option<u64> {
    use std::os::unix::fs::MetadataExt;
    Some(meta.ino())
}

#[cfg(not(unix))]
fn unix_inode(_meta: &std::fs::Metadata) -> Option<u64> {
    None
}

fn read_checkpoint(path: &Path) -> (u64, Option<u64>) {
    match std::fs::read_to_string(path) {
        Ok(s) => {
            let mut parts = s.trim().split(':');
            let offset = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
            let inode = parts.next().and_then(|s| s.parse().ok());
            (offset, inode)
        }
        Err(_) => (0, None),
    }
}

fn write_checkpoint(path: &Path, offset: u64, inode: Option<u64>) {
    let content = format!("{}:{}", offset, inode.map(|i| i.to_string()).unwrap_or_default());
    let _ = std::fs::write(path, content);
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    fn write_file(path: &Path, content: &str) {
        std::fs::write(path, content).unwrap();
    }

    fn append_file(path: &Path, content: &str) {
        let mut f = std::fs::OpenOptions::new().append(true).open(path).unwrap();
        f.write_all(content.as_bytes()).unwrap();
    }

    #[test]
    fn tails_new_lines_incrementally() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("app.log");
        write_file(&path, "line one\n");

        let mut tailer = FileTailer::new(&path, None).unwrap();
        let first = tailer.poll().unwrap();
        assert_eq!(first, vec!["line one".to_string()]);

        append_file(&path, "line two\n");
        let second = tailer.poll().unwrap();
        assert_eq!(second, vec!["line two".to_string()]);
    }

    #[test]
    fn leaves_partial_line_for_next_poll() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("app.log");
        write_file(&path, "complete\nincomple");

        let mut tailer = FileTailer::new(&path, None).unwrap();
        let records = tailer.poll().unwrap();
        assert_eq!(records, vec!["complete".to_string()]);

        append_file(&path, "te\n");
        let records = tailer.poll().unwrap();
        assert_eq!(records, vec!["incomplete".to_string()]);
    }

    #[test]
    fn checkpoint_survives_across_tailer_instances() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("app.log");
        write_file(&path, "line one\n");

        {
            let mut tailer = FileTailer::new(&path, None).unwrap();
            tailer.poll().unwrap();
        } // dropped: simulates the agent restarting

        // No new writes — a fresh tailer must not replay "line one".
        let mut tailer = FileTailer::new(&path, None).unwrap();
        assert!(tailer.poll().unwrap().is_empty());

        append_file(&path, "line two\n");
        assert_eq!(tailer.poll().unwrap(), vec!["line two".to_string()]);
    }

    #[test]
    fn detects_rotation_and_restarts_from_top() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("app.log");
        write_file(&path, "old content\n");

        let mut tailer = FileTailer::new(&path, None).unwrap();
        assert_eq!(tailer.poll().unwrap(), vec!["old content".to_string()]);

        // Simulate logrotate: remove and recreate with a fresh inode.
        std::fs::remove_file(&path).unwrap();
        write_file(&path, "new content\n");

        assert_eq!(tailer.poll().unwrap(), vec!["new content".to_string()]);
    }

    #[test]
    fn joins_multiline_records() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("app.log");
        write_file(
            &path,
            "2026-08-01 ERROR boom\n  at fn_a\n  at fn_b\n2026-08-01 INFO recovered\n",
        );

        let mut tailer = FileTailer::new(&path, Some(r"^\d{4}-\d{2}-\d{2}")).unwrap();
        let mut records = tailer.poll().unwrap();

        // The trailing record only closes once a *following* start-line
        // arrives (or flush_pending is called) — until then we can't tell
        // it isn't still accumulating continuation lines.
        assert_eq!(records.len(), 1);
        assert_eq!(records[0], "2026-08-01 ERROR boom\n  at fn_a\n  at fn_b");

        records.push(tailer.flush_pending().expect("a record should still be pending"));
        assert_eq!(records[1], "2026-08-01 INFO recovered");
    }

    #[test]
    fn missing_file_returns_no_records_without_error() {
        let mut tailer = FileTailer::new("/nonexistent/path.log", None).unwrap();
        assert!(tailer.poll().unwrap().is_empty());
    }
}
