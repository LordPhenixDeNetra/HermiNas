//! Command herminas-dataplane is the Rust composition root (L1). Two
//! modes, dispatched on argv[1]:
//!
//!   - (none)  kernel-only smoke test: proves settings load cleanly (M0.1)
//!   - "serve" starts the real Agent gRPC receiver (M1.2:
//!     stream/receiver.rs) — validates, enriches, deduplicates and sinks
//!     to ClickHouse. Needs `HERMINAS_CLICKHOUSE_URL` (default
//!     http://127.0.0.1:8123) reachable; see the M0.5 supervisor
//!     (`go run bootstrap.go run`) to start it.

use herminas_dataplane_bootstrap::dlq::Dlq;
use herminas_dataplane_bootstrap::receiver::AgentService;
use herminas_dataplane_bootstrap::sink::ClickHouseSink;
use herminas_kernel::settings::Settings;
use herminas_protocol::agent::v1::agent_server::AgentServer;

fn main() {
    match std::env::args().nth(1).as_deref() {
        Some("serve") => {
            if let Err(e) = serve() {
                eprintln!("herminas-dataplane: serve failed: {e}");
                std::process::exit(1);
            }
        }
        _ => check_kernel(),
    }
}

fn load_settings() -> Result<Settings, herminas_kernel::errors::HerminasError> {
    let config_path =
        std::env::var("HERMINAS_CONFIG").unwrap_or_else(|_| "config/herminas.example.yaml".to_string());
    Settings::load(&config_path)
}

fn check_kernel() {
    match load_settings() {
        Ok(settings) => {
            println!("HermiNas data plane bootstrap OK");
            println!("  environment : {}", settings.environment());
            println!("  grpc_port   : {}", settings.grpc_port());
            println!("  data_dir    : {}", settings.data_dir());
            println!("  llm_provider: {}", settings.llm_provider());
        }
        Err(e) => {
            eprintln!("HermiNas data plane bootstrap FAILED: {e}");
            std::process::exit(1);
        }
    }
}

fn serve() -> Result<(), Box<dyn std::error::Error>> {
    tokio::runtime::Runtime::new()?.block_on(serve_async())
}

async fn serve_async() -> Result<(), Box<dyn std::error::Error>> {
    let settings = load_settings()?;

    let clickhouse_url =
        std::env::var("HERMINAS_CLICKHOUSE_URL").unwrap_or_else(|_| "http://127.0.0.1:8123".to_string());
    let sink = Box::new(ClickHouseSink::new(clickhouse_url.clone()));

    let dlq_path = std::path::Path::new(settings.data_dir()).join("dlq").join("agent.jsonl");
    let dlq = Dlq::open(&dlq_path)?;

    // 10k batches of dedup history before the bounded set evicts — see
    // dedup.rs for why this is "basic" idempotence, not a full LRU.
    let service = AgentService::new(sink, dlq, 10_000);

    let addr = format!("0.0.0.0:{}", settings.grpc_port()).parse()?;
    println!("herminas-dataplane: Agent gRPC receiver listening on {addr}");
    println!("herminas-dataplane: sinking to ClickHouse at {clickhouse_url}");
    println!("herminas-dataplane: DLQ at {dlq_path:?}");

    tonic::transport::Server::builder()
        .add_service(AgentServer::new(service))
        .serve(addr)
        .await?;

    Ok(())
}
