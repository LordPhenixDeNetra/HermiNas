//! Generated gRPC stubs for HermiNas' cross-language contracts (M0.3).
//! Compiled from `kernel/proto/*.proto` by `build.rs` (tonic-build); the Go
//! and Python equivalents are generated the same way into
//! `kernel/proto/*pb/` and `python/src/herminas_proto/` respectively — all
//! three from the exact same `.proto` source of truth.
//!
//! Module nesting mirrors each `.proto`'s `package` declaration exactly
//! (`herminas.common.v1` -> `common::v1`) — `build.rs`'s `extern_path`
//! mapping for `.herminas.common.v1` points at `crate::common::v1`, so this
//! nesting isn't cosmetic, it's load-bearing.

pub mod common {
    pub mod v1 {
        tonic::include_proto!("herminas.common.v1");
    }
}

pub mod dataplane {
    pub mod v1 {
        tonic::include_proto!("herminas.dataplane.v1");
    }
}

pub mod intelligence {
    pub mod v1 {
        tonic::include_proto!("herminas.intelligence.v1");
    }
}

pub mod agent {
    pub mod v1 {
        tonic::include_proto!("herminas.agent.v1");
    }
}
