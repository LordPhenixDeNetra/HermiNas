use std::collections::HashMap;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ServiceName {
    ClickHouse,
    Redpanda,
    DataPlane,
    Intelligence,
    Api,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HealthState {
    Unknown,
    Starting,
    Healthy,
    Degraded,
    Unhealthy,
}

#[derive(Debug, Clone)]
pub struct HealthStatus {
    pub service: ServiceName,
    pub state: HealthState,
    pub message: String,
    /// Milliseconds since the Unix epoch (kept dependency-free at L0;
    /// upgrade to a proper timestamp type if/when chrono is pulled in by a
    /// higher layer).
    pub checked_at_ms: i64,
}

/// Implemented by anything the Rust data plane needs to report health for
/// (pipelines, sinks, the embedded-engine clients).
pub trait HealthChecker {
    fn health(&self) -> HealthStatus;
}

/// Minimal event envelope shared by ingestion, stream processing and sinks.
/// Mirrors kernel/contracts/contracts.go's `Event` and
/// herminas_kernel.contracts.Event (Python).
#[derive(Debug, Clone)]
pub struct Event {
    pub id: String,
    pub dataset: String,
    pub timestamp_ms: i64,
    pub payload: Vec<u8>,
    pub metadata: HashMap<String, String>,
}
