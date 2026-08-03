use std::path::PathBuf;

use serde::Deserialize;

use crate::parse::Format;

#[derive(Debug, Clone, Deserialize)]
pub struct SourceConfig {
    pub path: PathBuf,
    #[serde(default)]
    pub multiline_start: Option<String>,
    #[serde(default = "default_format")]
    pub format: Format,
}

fn default_format() -> Format {
    Format::Json
}

#[derive(Debug, Clone, Deserialize)]
pub struct AgentConfig {
    #[serde(default)]
    pub sources: Vec<SourceConfig>,
    #[serde(default = "default_http_addr")]
    pub http_addr: String,
    pub wal_path: PathBuf,
    /// gRPC address of the Rust data-plane receiver's Agent service
    /// (`stream/receiver.rs`, M1.2). When set, the agent ships via
    /// `ship::GrpcShipper`; when absent, it falls back to
    /// `ship::FileShipper` writing to `ship_output_path` — useful for dev
    /// setups and tests without a receiver running.
    #[serde(default)]
    pub receiver_addr: Option<String>,
    /// Identifies this agent to the receiver (BatchRequest.agent_id) and
    /// tags DLQ/dedup entries. Defaults to the machine hostname.
    #[serde(default = "default_agent_id")]
    pub agent_id: String,
    /// Dataset this agent's sources belong to (BatchRequest.dataset).
    /// One agent per dataset for now; multi-dataset agents land with
    /// per-source routing in a later phase.
    #[serde(default = "default_dataset")]
    pub dataset: String,
    /// FileShipper destination, used only when `receiver_addr` is unset.
    #[serde(default = "default_ship_output_path")]
    pub ship_output_path: PathBuf,
    #[serde(default = "default_batch_size")]
    pub batch_size: usize,
    #[serde(default = "default_flush_interval_ms")]
    pub flush_interval_ms: u64,
    #[serde(default = "default_backpressure_threshold")]
    pub backpressure_threshold_bytes: u64,
}

fn default_agent_id() -> String {
    // A real deployment always sets this explicitly (M8.3 fleet
    // enrollment assigns a stable ID); this default only covers local dev.
    std::env::var("HOSTNAME").unwrap_or_else(|_| "agent-local".to_string())
}
fn default_dataset() -> String {
    "default".to_string()
}
fn default_ship_output_path() -> PathBuf {
    PathBuf::from("shipped.bin")
}

fn default_http_addr() -> String {
    "127.0.0.1:8901".to_string()
}
fn default_batch_size() -> usize {
    500
}
fn default_flush_interval_ms() -> u64 {
    1000
}
fn default_backpressure_threshold() -> u64 {
    50 * 1024 * 1024
}

/// Loads (or reloads) the agent config from a YAML file. Called once at
/// startup and again on SIGHUP for a manual, on-demand reload (M1.1:
/// "rechargement à la demande"). Remote reload pushed from the server over
/// a command channel needs the agent.proto GetAgentConfig RPC (M0.3) and
/// fleet management (M8.3) — not wired yet.
pub fn load(path: impl AsRef<std::path::Path>) -> Result<AgentConfig, crate::AgentError> {
    let text = std::fs::read_to_string(&path)
        .map_err(|e| crate::AgentError::Config(format!("read {:?}: {e}", path.as_ref())))?;
    serde_yaml::from_str(&text)
        .map_err(|e| crate::AgentError::Config(format!("parse {:?}: {e}", path.as_ref())))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn loads_minimal_config_with_defaults() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent.yaml");
        std::fs::write(
            &path,
            r#"
wal_path: /tmp/herminas-agent/wal
ship_output_path: /tmp/herminas-agent/shipped.bin
sources:
  - path: /var/log/app.log
"#,
        )
        .unwrap();

        let cfg = load(&path).unwrap();
        assert_eq!(cfg.http_addr, "127.0.0.1:8901");
        assert_eq!(cfg.batch_size, 500);
        assert_eq!(cfg.sources.len(), 1);
        assert!(matches!(cfg.sources[0].format, Format::Json));
    }

    #[test]
    fn loads_regex_source_format() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent.yaml");
        std::fs::write(
            &path,
            r#"
wal_path: /tmp/herminas-agent/wal
ship_output_path: /tmp/herminas-agent/shipped.bin
sources:
  - path: /var/log/app.log
    format:
      type: regex
      pattern: '^(?P<level>\w+) (?P<message>.*)$'
"#,
        )
        .unwrap();

        let cfg = load(&path).unwrap();
        match &cfg.sources[0].format {
            Format::Regex { pattern } => assert!(pattern.contains("level")),
            other => panic!("expected Regex format, got {other:?}"),
        }
    }

    #[test]
    fn missing_file_returns_config_error() {
        let err = load("/nonexistent/agent.yaml").unwrap_err();
        assert!(matches!(err, crate::AgentError::Config(_)));
    }
}
