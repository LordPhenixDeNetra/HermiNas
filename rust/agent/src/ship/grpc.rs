//! Real network transport for `Shipper`: gRPC to the Rust data-plane
//! receiver's Agent service (`kernel/proto/agent.proto`, M0.3;
//! `stream/receiver.rs`, M1.2). Framing (length-prefixed records, zstd
//! over the whole batch) is shared with `FileShipper` via `encode_batch`/
//! `decode_batch` so the receiver decodes both the same way.

use tonic::transport::Channel;

use herminas_protocol::agent::v1::agent_client::AgentClient;
use herminas_protocol::agent::v1::BatchRequest;

use super::Shipper;
use crate::buffer::wal::WalEntry;

pub(super) fn encode_batch(batch: &[WalEntry]) -> Result<Vec<u8>, std::io::Error> {
    let mut framed = Vec::new();
    for entry in batch {
        framed.extend_from_slice(&(entry.data.len() as u32).to_le_bytes());
        framed.extend_from_slice(&entry.data);
    }
    zstd::stream::encode_all(framed.as_slice(), 3)
}

pub(super) fn decode_batch(compressed: &[u8]) -> Result<Vec<Vec<u8>>, crate::AgentError> {
    let framed = zstd::stream::decode_all(compressed)
        .map_err(|e| crate::AgentError::Ship(format!("zstd decompress: {e}")))?;

    let mut records = Vec::new();
    let mut pos = 0;
    while pos < framed.len() {
        let len = u32::from_le_bytes(framed[pos..pos + 4].try_into().unwrap()) as usize;
        pos += 4;
        records.push(framed[pos..pos + len].to_vec());
        pos += len;
    }
    Ok(records)
}

pub struct GrpcShipper {
    agent_id: String,
    dataset: String,
    client: tokio::sync::Mutex<AgentClient<Channel>>,
}

impl GrpcShipper {
    pub async fn connect(addr: String, agent_id: String, dataset: String) -> Result<Self, crate::AgentError> {
        let client = AgentClient::connect(addr)
            .await
            .map_err(|e| crate::AgentError::Ship(format!("connect to receiver: {e}")))?;
        Ok(Self {
            agent_id,
            dataset,
            client: tokio::sync::Mutex::new(client),
        })
    }
}

#[async_trait::async_trait]
impl Shipper for GrpcShipper {
    async fn ship(&self, batch: &[WalEntry]) -> Result<(), crate::AgentError> {
        if batch.is_empty() {
            return Ok(());
        }

        let compressed =
            encode_batch(batch).map_err(|e| crate::AgentError::Ship(format!("zstd compress: {e}")))?;

        let request = tonic::Request::new(BatchRequest {
            agent_id: self.agent_id.clone(),
            dataset: self.dataset.clone(),
            compressed_payload: compressed,
            record_count: batch.len() as u32,
        });

        let mut client = self.client.lock().await;
        let response = client
            .ship_batch(request)
            .await
            .map_err(|e| crate::AgentError::Ship(format!("ShipBatch rpc failed: {e}")))?
            .into_inner();

        if !response.accepted {
            return Err(crate::AgentError::Ship(format!(
                "receiver rejected batch: {}",
                response.message
            )));
        }
        Ok(())
    }
}
