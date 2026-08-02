//! HermiNas ingestion agent (F-101): a lightweight, deployable binary that
//! tails logs and accepts local HTTP ingestion, parses records, buffers
//! them durably (WAL) and ships them onward in compressed batches.
//!
//! The gRPC/mTLS transport to the Rust data-plane receiver (F-107's
//! "envoi gRPC mTLS") depends on the agent.proto contract (M0.3) and
//! stream/receiver.rs (M1.2), neither of which exist yet. `ship::Shipper`
//! is the seam: `ship::FileShipper` exercises the real batching +
//! zstd-compression path today so the collect → parse → WAL → backpressure
//! → ship loop is fully working and tested; a `GrpcShipper` slots in later
//! without changing anything upstream.

pub mod backpressure;
pub mod buffer;
pub mod collect;
pub mod config;
pub mod parse;
pub mod ship;

#[derive(Debug, thiserror::Error)]
pub enum AgentError {
    #[error("config error: {0}")]
    Config(String),
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("parse error: {0}")]
    Parse(String),
    #[error("ship error: {0}")]
    Ship(String),
}
