//! Minimal real DataPlane gRPC server (M0.3): only GetHealth is
//! implemented, to prove the gRPC plumbing end to end (proto -> generated
//! stubs in 3 languages -> real server -> real client -> latency
//! benchmark, see engine/health/grpc_bench_test.go). The rest of the
//! service is exactly what M3.2 (pipeline manager), M4.3 (ML deploy) and
//! M6.2 (WASM rules) will implement — calling them today returns
//! UNIMPLEMENTED rather than a fake success.

use std::pin::Pin;
use std::time::{SystemTime, UNIX_EPOCH};

use tokio_stream::Stream;
use tonic::{transport::Server, Request, Response, Status};

use herminas_protocol::common::v1::{HealthRequest, HealthStatus};
use herminas_protocol::dataplane::v1::data_plane_server::{DataPlane, DataPlaneServer};
use herminas_protocol::dataplane::v1::{
    DeployResult, Event, EventFilter, ModelSpec, PipelineId, PipelineSpec, PipelineStats, RuleSpec, StopResult,
};

#[derive(Default)]
struct DataPlaneService;

#[tonic::async_trait]
impl DataPlane for DataPlaneService {
    async fn deploy_pipeline(&self, _request: Request<PipelineSpec>) -> Result<Response<DeployResult>, Status> {
        Err(Status::unimplemented("DeployPipeline lands with the pipeline manager (M3.2)"))
    }

    async fn stop_pipeline(&self, _request: Request<PipelineId>) -> Result<Response<StopResult>, Status> {
        Err(Status::unimplemented("StopPipeline lands with the pipeline manager (M3.2)"))
    }

    async fn get_pipeline_stats(&self, _request: Request<PipelineId>) -> Result<Response<PipelineStats>, Status> {
        Err(Status::unimplemented("GetPipelineStats lands with the pipeline manager (M3.2)"))
    }

    async fn deploy_model(&self, _request: Request<ModelSpec>) -> Result<Response<DeployResult>, Status> {
        Err(Status::unimplemented("DeployModel lands with ML deploy (M4.3)"))
    }

    async fn deploy_rule(&self, _request: Request<RuleSpec>) -> Result<Response<DeployResult>, Status> {
        Err(Status::unimplemented("DeployRule lands with WASM rules (M6.2)"))
    }

    type StreamEventsStream = Pin<Box<dyn Stream<Item = Result<Event, Status>> + Send + 'static>>;

    async fn stream_events(
        &self,
        _request: Request<EventFilter>,
    ) -> Result<Response<Self::StreamEventsStream>, Status> {
        Err(Status::unimplemented("StreamEvents lands with anomaly detection (M4.2)"))
    }

    async fn get_health(&self, _request: Request<HealthRequest>) -> Result<Response<HealthStatus>, Status> {
        let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default();
        Ok(Response::new(HealthStatus {
            service: "dataplane".to_string(),
            state: "healthy".to_string(),
            message: String::new(),
            checked_at: Some(prost_types::Timestamp {
                seconds: now.as_secs() as i64,
                nanos: now.subsec_nanos() as i32,
            }),
        }))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::var("HERMINAS_DATAPLANE_ADDR")
        .unwrap_or_else(|_| "127.0.0.1:50051".to_string())
        .parse()?;

    println!("herminas-protocol: DataPlane gRPC server listening on {addr}");
    Server::builder()
        .add_service(DataPlaneServer::new(DataPlaneService))
        .serve(addr)
        .await?;

    Ok(())
}
