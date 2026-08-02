use super::Record;

pub fn parse_json(raw: &str) -> Result<Record, crate::AgentError> {
    let value: serde_json::Value =
        serde_json::from_str(raw).map_err(|e| crate::AgentError::Parse(format!("invalid JSON: {e}")))?;

    let obj = value
        .as_object()
        .ok_or_else(|| crate::AgentError::Parse("JSON record must be an object".into()))?;

    let mut record = Record::new();
    for (k, v) in obj {
        record.insert(k.clone(), json_value_to_string(v));
    }
    Ok(record)
}

fn json_value_to_string(v: &serde_json::Value) -> String {
    match v {
        serde_json::Value::String(s) => s.clone(),
        other => other.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_flat_object() {
        let record = parse_json(r#"{"level":"error","code":500,"ok":false}"#).unwrap();
        assert_eq!(record.get("level").unwrap(), "error");
        assert_eq!(record.get("code").unwrap(), "500");
        assert_eq!(record.get("ok").unwrap(), "false");
    }

    #[test]
    fn rejects_non_object_json() {
        assert!(parse_json("[1,2,3]").is_err());
        assert!(parse_json("not json").is_err());
    }
}
