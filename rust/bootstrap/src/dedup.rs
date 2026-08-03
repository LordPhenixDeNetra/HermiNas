//! Batch-level idempotence (M1.2: "Idempotence de base : clé de
//! déduplication par batch"). The key is a hash of (agent_id, dataset,
//! compressed payload) — if `rust/agent`'s GrpcShipper retries a batch
//! because it never saw the ack (e.g. the response was lost after the
//! receiver had already inserted it), the retry hashes to the same key and
//! the receiver acks it again without re-inserting.

use std::collections::HashSet;
use std::hash::{Hash, Hasher};
use std::sync::Mutex;

pub struct Deduplicator {
    seen: Mutex<HashSet<u64>>,
    max_entries: usize,
}

impl Deduplicator {
    pub fn new(max_entries: usize) -> Self {
        Self {
            seen: Mutex::new(HashSet::new()),
            max_entries: max_entries.max(1),
        }
    }

    pub fn key_for(agent_id: &str, dataset: &str, payload: &[u8]) -> u64 {
        let mut hasher = std::collections::hash_map::DefaultHasher::new();
        agent_id.hash(&mut hasher);
        dataset.hash(&mut hasher);
        payload.hash(&mut hasher);
        hasher.finish()
    }

    /// Returns true if this exact batch was already seen (and leaves the
    /// seen-set unchanged); otherwise records it and returns false.
    pub fn check_and_remember(&self, key: u64) -> bool {
        let mut seen = self.seen.lock().unwrap();
        if seen.contains(&key) {
            return true;
        }
        if seen.len() >= self.max_entries {
            // Crude bounded eviction: drop everything once full rather
            // than a full LRU. Good enough for "idempotence de base" —
            // revisit if retry windows longer than max_entries batches
            // turn out to matter in practice.
            seen.clear();
        }
        seen.insert(key);
        false
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn same_content_is_detected_as_duplicate() {
        let dedup = Deduplicator::new(100);
        let key = Deduplicator::key_for("agent-1", "logs", b"payload-bytes");

        assert!(!dedup.check_and_remember(key), "first sighting should not be a duplicate");
        assert!(dedup.check_and_remember(key), "second sighting of the same key should be a duplicate");
    }

    #[test]
    fn different_content_is_not_a_duplicate() {
        let dedup = Deduplicator::new(100);
        let a = Deduplicator::key_for("agent-1", "logs", b"payload-a");
        let b = Deduplicator::key_for("agent-1", "logs", b"payload-b");

        assert!(!dedup.check_and_remember(a));
        assert!(!dedup.check_and_remember(b));
    }

    #[test]
    fn different_agent_or_dataset_changes_the_key() {
        let k1 = Deduplicator::key_for("agent-1", "logs", b"same-bytes");
        let k2 = Deduplicator::key_for("agent-2", "logs", b"same-bytes");
        let k3 = Deduplicator::key_for("agent-1", "metrics", b"same-bytes");
        assert_ne!(k1, k2);
        assert_ne!(k1, k3);
    }

    #[test]
    fn eviction_bounds_memory_and_forgets_old_entries() {
        let dedup = Deduplicator::new(2);
        let a = Deduplicator::key_for("agent-1", "logs", b"a");
        let b = Deduplicator::key_for("agent-1", "logs", b"b");
        let c = Deduplicator::key_for("agent-1", "logs", b"c");

        assert!(!dedup.check_and_remember(a));
        assert!(!dedup.check_and_remember(b));
        assert!(!dedup.check_and_remember(c)); // triggers eviction of {a, b}

        // a is forgotten post-eviction: seen again as "new", which is the
        // accepted tradeoff for a bounded, non-LRU implementation.
        assert!(!dedup.check_and_remember(a));
    }
}
