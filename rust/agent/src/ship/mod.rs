//! Ships buffered batches to their destination (F-107: "compression +
//! batching vers le serveur"). `Shipper` is async because the real
//! transport (`grpc::GrpcShipper`, M1.2) makes a network call; `file`
//! exercises the same batching + zstd-compression path against a local
//! file for tests and for dev setups without a receiver.

mod file;
mod grpc;

pub use file::{read_shipped_batches, FileShipper};
pub use grpc::GrpcShipper;

use crate::buffer::wal::WalEntry;

#[async_trait::async_trait]
pub trait Shipper: Send + Sync {
    async fn ship(&self, batch: &[WalEntry]) -> Result<(), crate::AgentError>;
}
