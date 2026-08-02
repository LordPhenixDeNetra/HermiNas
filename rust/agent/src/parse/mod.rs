mod csv_format;
mod json;
mod regex_format;

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

pub use csv_format::parse_csv;
pub use json::parse_json;
pub use regex_format::parse_regex;

/// A parsed record: fields flattened to strings. Full typed/nested schema
/// inference (F-112) lands with the Arrow/DataFusion in-flight
/// representation in M3 — this is deliberately the simplest thing that
/// lets the agent buffer and ship structured data today.
pub type Record = BTreeMap<String, String>;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum Format {
    Json,
    Csv {
        #[serde(default)]
        headers: Option<Vec<String>>,
    },
    Regex {
        pattern: String,
    },
}

pub fn parse(raw: &str, format: &Format) -> Result<Record, crate::AgentError> {
    match format {
        Format::Json => parse_json(raw),
        Format::Csv { headers } => parse_csv(raw, headers.as_deref()),
        Format::Regex { pattern } => parse_regex(raw, pattern),
    }
}
