//! Rust data plane (L1): receives agent batches over gRPC, validates and
//! enriches them, deduplicates at the batch level, and sinks valid records
//! to ClickHouse (invalid ones to a local DLQ). The Redpanda bus stage
//! between receiver and sink (cahier des charges: Agent -> Redpanda ->
//! ClickHouse) is not wired yet — see `dedup`'s module doc for why, and
//! tasks-herminas.md M1.2 for the plan. `receiver::AgentService` sinks
//! straight to ClickHouse in the meantime, behind the same `Sink` trait a
//! Redpanda-backed pipeline will use later.

pub mod dedup;
pub mod dlq;
pub mod receiver;
pub mod sink;
