use super::Record;

pub fn parse_csv(raw: &str, headers: Option<&[String]>) -> Result<Record, crate::AgentError> {
    let mut reader = csv::ReaderBuilder::new()
        .has_headers(false)
        .from_reader(raw.as_bytes());

    let row = reader
        .records()
        .next()
        .ok_or_else(|| crate::AgentError::Parse("empty CSV line".into()))?
        .map_err(|e| crate::AgentError::Parse(format!("invalid CSV: {e}")))?;

    let mut record = Record::new();
    for (i, field) in row.iter().enumerate() {
        let key = headers
            .and_then(|hs| hs.get(i).cloned())
            .unwrap_or_else(|| format!("field_{i}"));
        record.insert(key, field.to_string());
    }
    Ok(record)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_with_explicit_headers() {
        let headers = vec!["ts".to_string(), "level".to_string(), "message".to_string()];
        let record = parse_csv("2026-08-01,error,disk full", Some(&headers)).unwrap();
        assert_eq!(record.get("ts").unwrap(), "2026-08-01");
        assert_eq!(record.get("level").unwrap(), "error");
        assert_eq!(record.get("message").unwrap(), "disk full");
    }

    #[test]
    fn falls_back_to_positional_field_names() {
        let record = parse_csv("a,b,c", None).unwrap();
        assert_eq!(record.get("field_0").unwrap(), "a");
        assert_eq!(record.get("field_2").unwrap(), "c");
    }

    #[test]
    fn empty_line_is_an_error() {
        assert!(parse_csv("", None).is_err());
    }
}
