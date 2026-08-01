//! Command herminas-dataplane is the Rust composition root (L1): it wires
//! the L0 kernel (settings) at startup. The real pipeline/sink/anomaly
//! wiring lands in M1-M4; today this binary only proves the kernel loads
//! cleanly end to end.

use herminas_kernel::settings::Settings;

fn main() {
    let config_path =
        std::env::var("HERMINAS_CONFIG").unwrap_or_else(|_| "config/herminas.example.yaml".to_string());

    match Settings::load(&config_path) {
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
