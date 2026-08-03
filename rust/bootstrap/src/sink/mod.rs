//! Where validated, enriched records end up. `clickhouse` is the only
//! implementation today — the cahier des charges' full path is Agent ->
//! Redpanda -> ClickHouse (`sink/redpanda.rs` producing, a separate
//! consumer inserting), but there's no Redpanda bus wired yet (see lib.rs'
//! doc comment), so `receiver::AgentService` uses `ClickHouseSink`
//! directly. `Sink` is the seam: a Redpanda-backed implementation slots in
//! without changing the receiver's call site.

mod clickhouse;

pub use clickhouse::ClickHouseSink;

#[derive(Debug, thiserror::Error)]
pub enum SinkError {
    #[error("http error: {0}")]
    Http(#[from] reqwest::Error),
    #[error("clickhouse returned {status}: {body}")]
    ClickHouse { status: u16, body: String },
    #[error("serialize error: {0}")]
    Serialize(#[from] serde_json::Error),
}

#[async_trait::async_trait]
pub trait Sink: Send + Sync {
    async fn write(&self, dataset: &str, records: &[serde_json::Value]) -> Result<(), SinkError>;
}
