use std::time::Duration;

/// Throttles collection when the WAL's unacked backlog grows too large
/// (F-113). The delay scales with how far over threshold the backlog is,
/// capped at `max_delay`, so a shipper that's merely slow gets a gentle
/// slowdown while one that's fully stuck (or a downstream outage) gets the
/// agent to back off hard rather than growing the WAL unbounded.
#[derive(Debug, Clone, Copy)]
pub struct Backpressure {
    pub threshold_bytes: u64,
    pub max_delay: Duration,
}

impl Backpressure {
    pub fn new(threshold_bytes: u64) -> Self {
        Self {
            threshold_bytes,
            max_delay: Duration::from_secs(5),
        }
    }

    /// How long to pause before accepting more data, given `pending_bytes`
    /// currently unacked in the WAL. `Duration::ZERO` means no throttling.
    pub fn delay_for(&self, pending_bytes: u64) -> Duration {
        if pending_bytes <= self.threshold_bytes || self.threshold_bytes == 0 {
            return Duration::ZERO;
        }
        let over_ratio =
            (pending_bytes - self.threshold_bytes) as f64 / self.threshold_bytes as f64;
        self.max_delay
            .mul_f64(over_ratio.min(1.0))
            .max(Duration::from_millis(50))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn no_delay_below_threshold() {
        let bp = Backpressure::new(1000);
        assert_eq!(bp.delay_for(500), Duration::ZERO);
        assert_eq!(bp.delay_for(1000), Duration::ZERO);
    }

    #[test]
    fn delay_increases_and_caps_past_threshold() {
        let bp = Backpressure::new(1000);
        let mild = bp.delay_for(1100);
        let severe = bp.delay_for(3000);

        assert!(mild > Duration::ZERO);
        assert!(severe >= mild);
        assert!(severe <= bp.max_delay);
    }

    #[test]
    fn zero_threshold_disables_throttling() {
        let bp = Backpressure::new(0);
        assert_eq!(bp.delay_for(1_000_000), Duration::ZERO);
    }
}
