//! HermiNas Rust kernel (L0): domain contracts, error taxonomy, path
//! layout, RBAC roles and immutable settings for the data plane. Mirrors
//! (conceptually, not by shared code) kernel/ (Go) and
//! python/src/herminas_kernel (Python) until the protobuf contracts land in
//! M0.3.

pub mod contracts;
pub mod errors;
pub mod paths;
pub mod permissions;
pub mod settings;
