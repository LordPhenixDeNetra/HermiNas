// Command herminas-control-plane is the Go composition root (L3): it wires
// the L0 kernel (paths, settings) at startup. The supervisor that will
// actually launch ClickHouse/Redpanda/Rust/Python lands in M0.5; today this
// binary only proves the kernel loads cleanly end to end.
package main

import (
	"fmt"
	"os"

	"herminas/kernel/paths"
	"herminas/kernel/settings"
)

func main() {
	layout := paths.Default()
	_ = layout // wired into engine/supervisor in M0.5

	configPath := os.Getenv("HERMINAS_CONFIG")
	if configPath == "" {
		configPath = "config/herminas.example.yaml"
	}

	cfg, err := settings.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HermiNas control plane bootstrap FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("HermiNas control plane bootstrap OK")
	fmt.Printf("  environment : %s\n", cfg.Environment())
	fmt.Printf("  http_port   : %d\n", cfg.HTTPPort())
	fmt.Printf("  grpc_port   : %d\n", cfg.GRPCPort())
	fmt.Printf("  data_dir    : %s\n", cfg.DataDir())
	fmt.Printf("  llm_provider: %s\n", cfg.LLMProvider())
}
