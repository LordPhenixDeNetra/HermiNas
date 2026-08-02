use regex::Regex;

use super::Record;

/// Grok-like parsing via named capture groups (`(?P<name>...)`), which is
/// exactly what "regex nommées" (F-105) asks for without pulling in a full
/// grok-pattern library. Recompiles the pattern on every call for
/// simplicity/testability; the hot collection loop should pre-compile and
/// cache a `Regex` per source instead once this is under real load (M3
/// perf work), rather than calling this function directly.
pub fn parse_regex(raw: &str, pattern: &str) -> Result<Record, crate::AgentError> {
    let re = Regex::new(pattern).map_err(|e| crate::AgentError::Parse(format!("invalid regex: {e}")))?;
    let caps = re
        .captures(raw)
        .ok_or_else(|| crate::AgentError::Parse(format!("regex did not match: {raw:?}")))?;

    let mut record = Record::new();
    for name in re.capture_names().flatten() {
        if let Some(m) = caps.name(name) {
            record.insert(name.to_string(), m.as_str().to_string());
        }
    }
    Ok(record)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extracts_named_groups() {
        let record = parse_regex(
            "2026-08-01T10:00:00 ERROR disk full",
            r"^(?P<timestamp>\S+) (?P<level>\w+) (?P<message>.*)$",
        )
        .unwrap();
        assert_eq!(record.get("timestamp").unwrap(), "2026-08-01T10:00:00");
        assert_eq!(record.get("level").unwrap(), "ERROR");
        assert_eq!(record.get("message").unwrap(), "disk full");
    }

    #[test]
    fn non_matching_line_is_an_error() {
        let err = parse_regex("nope", r"^(?P<level>ERROR|WARN) .*$").unwrap_err();
        assert!(matches!(err, crate::AgentError::Parse(_)));
    }

    #[test]
    fn invalid_pattern_is_an_error() {
        let err = parse_regex("anything", r"(unclosed").unwrap_err();
        assert!(matches!(err, crate::AgentError::Parse(_)));
    }
}
