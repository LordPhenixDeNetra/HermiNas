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
    /// Destination for `ship::FileShipper` until the gRPC transport lands
    /// (M0.3/M1.2) — see the crate-level doc comment in lib.rs.
    pub ship_output_path: PathBuf,
    #[serde(default = "default_batch_size")]
    pub batch_size: usize,
    #[serde(default = "default_flush_interval_ms")]
    pub flush_interval_ms: u64,
    #[serde(default = "default_backpressure_threshold")]
    pub backpressure_threshold_bytes: u64,
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
