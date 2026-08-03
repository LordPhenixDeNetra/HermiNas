//! Real ClickHouse sink over the HTTP interface (cahier des charges: "Rust
//! -> ClickHouse | Native protocol / HTTP | RowBinary"). Uses JSONEachRow
//! instead of RowBinary for now — RowBinary needs a schema-aware encoder
//! that doesn't exist until schemamgr generates typed DDL (M1.3); JSONEachRow
//! is what proves the sink genuinely inserts and is queryable today, at the
//! cost of the RowBinary performance win this will need before M9's
//! ingestion benchmark.

use serde_json::Value;

use super::{Sink, SinkError};

pub struct ClickHouseSink {
    base_url: String,
    client: reqwest::Client,
}

impl ClickHouseSink {
    pub fn new(base_url: impl Into<String>) -> Self {
        Self {
            base_url: base_url.into(),
            client: reqwest::Client::new(),
        }
    }
}

#[async_trait::async_trait]
impl Sink for ClickHouseSink {
    async fn write(&self, dataset: &str, records: &[Value]) -> Result<(), SinkError> {
        if records.is_empty() {
            return Ok(());
        }

        let mut body = String::new();
        for record in records {
            body.push_str(&serde_json::to_string(record)?);
            body.push('\n');
        }

        let query = format!("INSERT INTO {dataset} FORMAT JSONEachRow");
        let resp = self
            .client
            .post(&self.base_url)
            .query(&[("query", query)])
            .body(body)
            .send()
            .await?;

        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(SinkError::ClickHouse {
                status: status.as_u16(),
                body,
            });
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Confirms the sink actually inserts and the data is queryable,
    /// against a real ClickHouse instance. Skips (not fails) if none is
    /// reachable, so `cargo test` stays dependency-free by default — start
    /// one with the M0.5 supervisor (`go run bootstrap.go run`) or plain
    /// `clickhouse server` to exercise this for real.
    #[tokio::test]
    async fn insert_and_select_round_trip_against_real_clickhouse() {
        let base_url =
            std::env::var("HERMINAS_CLICKHOUSE_URL").unwrap_or_else(|_| "http://127.0.0.1:8123".to_string());
        let client = reqwest::Client::new();

        let ping = client.get(format!("{base_url}/ping")).send().await;
        if ping.is_err() {
            eprintln!("skipping: no ClickHouse reachable at {base_url}");
            return;
        }

        // ClickHouse's HTTP interface returns 411 Length Required for any
        // POST without a Content-Length header. reqwest quietly omits that
        // header for an empty body (`.body("")` alone doesn't help — found
        // by actually running this test), so it must be set explicitly for
        // these no-payload DDL/SELECT calls. `ClickHouseSink::write` itself
        // is unaffected: its body is never empty, since it returns early
        // on an empty record slice, and reqwest sets Content-Length
        // correctly for any non-empty body.
        let table = format!("test_sink_{}", std::process::id());
        let create = format!(
            "CREATE TABLE {table} (agent_id String, dataset String, message String) ENGINE = Memory"
        );
        client
            .post(&base_url)
            .query(&[("query", create)])
            .header(reqwest::header::CONTENT_LENGTH, "0")
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap();

        let sink = ClickHouseSink::new(&base_url);
        let records = vec![
            serde_json::json!({"agent_id": "agent-1", "dataset": "logs", "message": "hello"}),
            serde_json::json!({"agent_id": "agent-1", "dataset": "logs", "message": "world"}),
        ];
        sink.write(&table, &records).await.unwrap();

        let select_resp = client
            .post(&base_url)
            .query(&[("query", format!("SELECT count() FROM {table} FORMAT TabSeparated"))])
            .header(reqwest::header::CONTENT_LENGTH, "0")
            .send()
            .await
            .unwrap();
        let count_text = select_resp.text().await.unwrap();
        assert_eq!(count_text.trim(), "2");

        let _ = client
            .post(&base_url)
            .query(&[("query", format!("DROP TABLE {table}"))])
            .header(reqwest::header::CONTENT_LENGTH, "0")
            .send()
            .await;
    }
}
