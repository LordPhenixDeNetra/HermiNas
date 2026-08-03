//! Real Agent gRPC service (M1.2's `stream/receiver.rs`): receives
//! zstd-compressed, length-prefixed batches from `rust/agent`'s
//! GrpcShipper (same framing — see `rust/agent/src/ship/grpc.rs`),
//! validates + enriches each record, deduplicates at the batch level, and
//! sinks valid records to ClickHouse (invalid ones to the DLQ).

use std::time::{SystemTime, UNIX_EPOCH};

use herminas_protocol::agent::v1::agent_server::Agent;
use herminas_protocol::agent::v1::{
    AgentConfigRequest, AgentConfigResponse, BatchAck, BatchRequest, HeartbeatRequest, HeartbeatResponse,
};
use serde_json::Value;
use tonic::{Request, Response, Status};

use crate::dedup::Deduplicator;
use crate::dlq::Dlq;
use crate::sink::Sink;

pub struct AgentService {
    sink: Box<dyn Sink>,
    dlq: Dlq,
    dedup: Deduplicator,
}

impl AgentService {
    pub fn new(sink: Box<dyn Sink>, dlq: Dlq, dedup_max_entries: usize) -> Self {
        Self {
            sink,
            dlq,
            dedup: Deduplicator::new(dedup_max_entries),
        }
    }
}

/// Decodes a GrpcShipper batch: zstd over the whole thing, then
/// length-prefixed records inside — the exact framing
/// `rust/agent/src/ship/grpc.rs::encode_batch` produces.
fn decode_batch(compressed: &[u8]) -> Result<Vec<Vec<u8>>, String> {
    let framed = zstd::stream::decode_all(compressed).map_err(|e| format!("zstd decompress: {e}"))?;

    let mut records = Vec::new();
    let mut pos = 0;
    while pos + 4 <= framed.len() {
        let len = u32::from_le_bytes(framed[pos..pos + 4].try_into().unwrap()) as usize;
        pos += 4;
        if pos + len > framed.len() {
            return Err("truncated record in batch".to_string());
        }
        records.push(framed[pos..pos + len].to_vec());
        pos += len;
    }
    Ok(records)
}

#[tonic::async_trait]
impl Agent for AgentService {
    async fn ship_batch(&self, request: Request<BatchRequest>) -> Result<Response<BatchAck>, Status> {
        let req = request.into_inner();

        let dedup_key = Deduplicator::key_for(&req.agent_id, &req.dataset, &req.compressed_payload);
        if self.dedup.check_and_remember(dedup_key) {
            return Ok(Response::new(BatchAck {
                accepted: true,
                message: "duplicate batch, already processed".to_string(),
            }));
        }

        let raw_records = match decode_batch(&req.compressed_payload) {
            Ok(r) => r,
            Err(e) => {
                return Ok(Response::new(BatchAck {
                    accepted: false,
                    message: format!("cannot decode batch: {e}"),
                }))
            }
        };

        let received_at_unix_ms = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as u64;

        let mut valid_records = Vec::with_capacity(raw_records.len());
        let mut rejected = 0u32;

        for raw in &raw_records {
            match serde_json::from_slice::<Value>(raw) {
                Ok(Value::Object(mut obj)) => {
                    // Enrichment (M1.2): tag every record with where it
                    // came from and when the receiver saw it.
                    obj.insert("agent_id".to_string(), Value::String(req.agent_id.clone()));
                    obj.insert("dataset".to_string(), Value::String(req.dataset.clone()));
                    obj.insert("received_at_unix_ms".to_string(), serde_json::json!(received_at_unix_ms));
                    valid_records.push(Value::Object(obj));
                }
                Ok(_) => {
                    rejected += 1;
                    self.dlq.reject(&req.agent_id, &req.dataset, "record is not a JSON object", raw);
                }
                Err(e) => {
                    rejected += 1;
                    self.dlq
                        .reject(&req.agent_id, &req.dataset, &format!("invalid JSON: {e}"), raw);
                }
            }
        }

        if !valid_records.is_empty() {
            if let Err(e) = self.sink.write(&req.dataset, &valid_records).await {
                return Ok(Response::new(BatchAck {
                    accepted: false,
                    message: format!("sink write failed: {e}"),
                }));
            }
        }

        let message = if rejected > 0 {
            format!("{} inserted, {} sent to DLQ", valid_records.len(), rejected)
        } else {
            format!("{} inserted", valid_records.len())
        };

        Ok(Response::new(BatchAck { accepted: true, message }))
    }

    async fn get_agent_config(
        &self,
        _request: Request<AgentConfigRequest>,
    ) -> Result<Response<AgentConfigResponse>, Status> {
        Err(Status::unimplemented("GetAgentConfig lands with fleet management (M8.3)"))
    }

    async fn heartbeat(&self, request: Request<HeartbeatRequest>) -> Result<Response<HeartbeatResponse>, Status> {
        let _ = request.into_inner();
        Ok(Response::new(HeartbeatResponse { ok: true }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    fn encode(records: &[&str]) -> Vec<u8> {
        let mut framed = Vec::new();
        for r in records {
            let bytes = r.as_bytes();
            framed.extend_from_slice(&(bytes.len() as u32).to_le_bytes());
            framed.extend_from_slice(bytes);
        }
        zstd::stream::encode_all(framed.as_slice(), 3).unwrap()
    }

    #[tokio::test]
    async fn valid_batch_is_enriched_and_accepted() {
        let dir = tempfile::tempdir().unwrap();
        let sink = RecordingRealSink::default();
        let dlq = Dlq::open(dir.path().join("dlq.jsonl")).unwrap();
        let svc = AgentService::new(Box::new(sink.clone()), dlq, 1000);

        let payload = encode(&[r#"{"level":"info","msg":"hello"}"#]);
        let resp = svc
            .ship_batch(Request::new(BatchRequest {
                agent_id: "agent-1".to_string(),
                dataset: "logs".to_string(),
                compressed_payload: payload,
                record_count: 1,
            }))
            .await
            .unwrap()
            .into_inner();

        assert!(resp.accepted);
        assert_eq!(resp.message, "1 inserted");

        let calls = sink.calls.lock().unwrap();
        assert_eq!(calls.len(), 1);
        let (dataset, records) = &calls[0];
        assert_eq!(dataset, "logs");
        assert_eq!(records[0]["agent_id"], "agent-1");
        assert_eq!(records[0]["dataset"], "logs");
        assert_eq!(records[0]["level"], "info");
        assert!(records[0].get("received_at_unix_ms").is_some());
    }

    #[tokio::test]
    async fn invalid_records_go_to_dlq_valid_ones_still_insert() {
        let dir = tempfile::tempdir().unwrap();
        let sink = RecordingRealSink::default();
        let dlq_path = dir.path().join("dlq.jsonl");
        let dlq = Dlq::open(&dlq_path).unwrap();
        let svc = AgentService::new(Box::new(sink.clone()), dlq, 1000);

        let payload = encode(&[r#"{"ok":true}"#, "not json at all", r#"["also","not","an","object"]"#]);
        let resp = svc
            .ship_batch(Request::new(BatchRequest {
                agent_id: "agent-1".to_string(),
                dataset: "logs".to_string(),
                compressed_payload: payload,
                record_count: 3,
            }))
            .await
            .unwrap()
            .into_inner();

        assert!(resp.accepted);
        assert_eq!(resp.message, "1 inserted, 2 sent to DLQ");
        assert_eq!(sink.calls.lock().unwrap().len(), 1);

        let dlq_contents = std::fs::read_to_string(&dlq_path).unwrap();
        assert_eq!(dlq_contents.lines().count(), 2);
    }

    #[tokio::test]
    async fn duplicate_batch_is_acked_without_reinserting() {
        let dir = tempfile::tempdir().unwrap();
        let sink = RecordingRealSink::default();
        let dlq = Dlq::open(dir.path().join("dlq.jsonl")).unwrap();
        let svc = AgentService::new(Box::new(sink.clone()), dlq, 1000);

        let payload = encode(&[r#"{"a":1}"#]);
        let make_req = || BatchRequest {
            agent_id: "agent-1".to_string(),
            dataset: "logs".to_string(),
            compressed_payload: payload.clone(),
            record_count: 1,
        };

        svc.ship_batch(Request::new(make_req())).await.unwrap();
        let second = svc.ship_batch(Request::new(make_req())).await.unwrap().into_inner();

        assert!(second.accepted);
        assert_eq!(second.message, "duplicate batch, already processed");
        assert_eq!(sink.calls.lock().unwrap().len(), 1, "sink should only see the first attempt");
    }

    #[tokio::test]
    async fn sink_failure_is_reported_as_not_accepted() {
        let dir = tempfile::tempdir().unwrap();
        let dlq = Dlq::open(dir.path().join("dlq.jsonl")).unwrap();
        let svc = AgentService::new(Box::new(FailingSink), dlq, 1000);

        let payload = encode(&[r#"{"a":1}"#]);
        let resp = svc
            .ship_batch(Request::new(BatchRequest {
                agent_id: "agent-1".to_string(),
                dataset: "logs".to_string(),
                compressed_payload: payload,
                record_count: 1,
            }))
            .await
            .unwrap()
            .into_inner();

        assert!(!resp.accepted);
        assert!(resp.message.contains("sink write failed"));
    }

    type RecordedCalls = std::sync::Arc<Mutex<Vec<(String, Vec<Value>)>>>;

    #[derive(Default, Clone)]
    struct RecordingRealSink {
        calls: RecordedCalls,
    }

    #[async_trait::async_trait]
    impl Sink for RecordingRealSink {
        async fn write(&self, dataset: &str, records: &[Value]) -> Result<(), crate::sink::SinkError> {
            self.calls.lock().unwrap().push((dataset.to_string(), records.to_vec()));
            Ok(())
        }
    }

    struct FailingSink;

    #[async_trait::async_trait]
    impl Sink for FailingSink {
        async fn write(&self, _dataset: &str, _records: &[Value]) -> Result<(), crate::sink::SinkError> {
            Err(crate::sink::SinkError::ClickHouse {
                status: 500,
                body: "simulated failure".to_string(),
            })
        }
    }
}
