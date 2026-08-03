fn main() -> Result<(), Box<dyn std::error::Error>> {
    // rust/protocol -> repo root/kernel/proto, two levels up then down.
    // The .proto files import each other by bare filename (e.g.
    // `import "common.proto";`), matching how the Go (`protoc -I
    // kernel/proto`) and Python (`grpc_tools.protoc -I kernel/proto`) stubs
    // are generated from the exact same source of truth.
    let proto_dir = format!("{}/../../kernel/proto", env!("CARGO_MANIFEST_DIR"));

    let proto = |name: &str| format!("{proto_dir}/{name}");

    // common.proto first, compiled on its own so it actually generates
    // HealthRequest/HealthStatus (an extern_path mapping for it, below,
    // would make prost skip generating the very definitions it points to).
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile_protos(&[proto("common.proto")], &[proto_dir.as_str()])?;

    // dataplane.proto and intelligence.proto both reference
    // herminas.common.v1's HealthRequest/HealthStatus; prost's automatic
    // relative-path resolution across separately-included packages doesn't
    // handle shared dotted prefixes correctly (it emits an invalid
    // `super::super::super::...` path), so point it directly at where
    // lib.rs actually puts the already-generated common module.
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .extern_path(".herminas.common.v1", "crate::common::v1")
        .compile_protos(
            &[proto("dataplane.proto"), proto("intelligence.proto"), proto("agent.proto")],
            &[proto_dir.as_str()],
        )?;

    Ok(())
}
