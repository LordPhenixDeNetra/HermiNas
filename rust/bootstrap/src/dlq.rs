//! Dead letter queue for records that fail validation (M1.2). The cahier
//! des charges' target is "événements invalides vers topic dédié" — a
//! Redpanda topic — but there's no Redpanda bus wired yet (see the crate
//! doc comment in lib.rs), so this writes newline-delimited JSON to a
//! local file instead, same stand-in pattern as `rust/agent`'s
//! FileShipper. Swapping in a Redpanda-backed Dlq later doesn't change
//! `receiver::AgentService`'s call site.

use std::io::Write;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;

use serde::Serialize;

#[derive(Serialize)]
struct DlqEntry<'a> {
    agent_id: &'a str,
    dataset: &'a str,
    reason: &'a str,
    record_base64: String,
    rejected_at_unix_ms: u128,
}

pub struct Dlq {
    path: PathBuf,
    file: Mutex<std::fs::File>,
    count: AtomicU64,
}

impl Dlq {
    pub fn open(path: impl Into<PathBuf>) -> std::io::Result<Self> {
        let path = path.into();
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let file = std::fs::OpenOptions::new().create(true).append(true).open(&path)?;
        Ok(Self {
            path,
            file: Mutex::new(file),
            count: AtomicU64::new(0),
        })
    }

    pub fn path(&self) -> &std::path::Path {
        &self.path
    }

    /// Records one rejected raw record with the reason it failed
    /// validation. Never returns an error to the caller's hot path on a
    /// write failure — it logs to stderr instead, since a broken DLQ must
    /// not take down ingestion of otherwise-valid records.
    pub fn reject(&self, agent_id: &str, dataset: &str, reason: &str, raw_record: &[u8]) {
        self.count.fetch_add(1, Ordering::SeqCst);

        let entry = DlqEntry {
            agent_id,
            dataset,
            reason,
            record_base64: base64_encode(raw_record),
            rejected_at_unix_ms: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis(),
        };

        let line = match serde_json::to_string(&entry) {
            Ok(l) => l,
            Err(e) => {
                eprintln!("dlq: failed to serialize entry: {e}");
                return;
            }
        };

        let mut file = self.file.lock().unwrap();
        if let Err(e) = writeln!(file, "{line}") {
            eprintln!("dlq: failed to write to {:?}: {e}", self.path);
        }
    }

    pub fn rejected_count(&self) -> u64 {
        self.count.load(Ordering::SeqCst)
    }
}

// Minimal base64 (standard alphabet, with padding) so the DLQ has no extra
// dependency beyond serde_json, which the sink already needs.
fn base64_encode(data: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    for chunk in data.chunks(3) {
        let b0 = chunk[0];
        let b1 = *chunk.get(1).unwrap_or(&0);
        let b2 = *chunk.get(2).unwrap_or(&0);

        out.push(ALPHABET[(b0 >> 2) as usize] as char);
        out.push(ALPHABET[(((b0 & 0x03) << 4) | (b1 >> 4)) as usize] as char);
        out.push(if chunk.len() > 1 {
            ALPHABET[(((b1 & 0x0f) << 2) | (b2 >> 6)) as usize] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            ALPHABET[(b2 & 0x3f) as usize] as char
        } else {
            '='
        });
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reject_appends_a_json_line_and_increments_count() {
        let dir = tempfile::tempdir().unwrap();
        let dlq = Dlq::open(dir.path().join("dlq.jsonl")).unwrap();

        dlq.reject("agent-1", "logs", "invalid JSON", b"not json");
        dlq.reject("agent-1", "logs", "invalid JSON", b"still not json");

        assert_eq!(dlq.rejected_count(), 2);

        let contents = std::fs::read_to_string(dlq.path()).unwrap();
        let lines: Vec<&str> = contents.lines().collect();
        assert_eq!(lines.len(), 2);

        let parsed: serde_json::Value = serde_json::from_str(lines[0]).unwrap();
        assert_eq!(parsed["agent_id"], "agent-1");
        assert_eq!(parsed["reason"], "invalid JSON");
    }

    #[test]
    fn base64_round_trips_via_standard_decoder() {
        // No decode helper of our own (nothing in this crate needs to
        // decode DLQ entries back), so prove correctness against the
        // well-known test vectors instead.
        assert_eq!(base64_encode(b"f"), "Zg==");
        assert_eq!(base64_encode(b"fo"), "Zm8=");
        assert_eq!(base64_encode(b"foo"), "Zm9v");
        assert_eq!(base64_encode(b"foobar"), "Zm9vYmFy");
    }
}
