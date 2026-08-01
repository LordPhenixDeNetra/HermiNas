// Command herminas-control-plane is the Go composition root (L3). Three
// modes, dispatched on argv[1]:
//
//   - (none)   kernel-only smoke test: proves settings load cleanly (M0.1)
//   - "run"    supervised startup: Redpanda → ClickHouse → ... (M0.5),
//     ordered per cahier des charges §5.3.1; blocks until SIGINT/SIGTERM
//   - "status" health-checks the same services without starting anything
//
// The Rust data plane / Python intelligence / Go API slots in the ordered
// startup are prepared but not yet registered: they only become
// long-running daemons in M1.2 (Rust gRPC server), M4 (Python FastAPI) and
// M1.5 (Go HTTP API) respectively. Registering today's one-shot bootstrap
// binaries here would misrepresent them as supervised daemons and produce
// a restart-crash loop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"herminas/engine/clickhouse"
	"herminas/engine/redpanda"
	"herminas/engine/supervisor"
	"herminas/kernel/paths"
	"herminas/kernel/settings"
)

func main() {
	switch arg(1) {
	case "run":
		runControlPlane()
	case "status":
		statusControlPlane()
	default:
		checkKernel()
	}
}

func arg(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

func loadSettingsOrExit() (*settings.Settings, paths.Layout) {
	layout := paths.Default()

	configPath := os.Getenv("HERMINAS_CONFIG")
	if configPath == "" {
		configPath = "config/herminas.example.yaml"
	}

	cfg, err := settings.Load(configPath)
	if err != nil {
		exitf("HermiNas control plane bootstrap FAILED: %v", err)
	}
	return cfg, layout
}

func checkKernel() {
	cfg, _ := loadSettingsOrExit()
	fmt.Println("HermiNas control plane bootstrap OK")
	fmt.Printf("  environment : %s\n", cfg.Environment())
	fmt.Printf("  http_port   : %d\n", cfg.HTTPPort())
	fmt.Printf("  grpc_port   : %d\n", cfg.GRPCPort())
	fmt.Printf("  data_dir    : %s\n", cfg.DataDir())
	fmt.Printf("  llm_provider: %s\n", cfg.LLMProvider())
}

func clickhouseConfig(layout paths.Layout) clickhouse.Config {
	return clickhouse.Config{
		HTTPPort: 8123,
		TCPPort:  9000,
		DataDir:  filepath.Join(layout.DataDir, "clickhouse"),
		BinDir:   filepath.Join(layout.RuntimeDir, "clickhouse", "bin"),
	}
}

func redpandaConfig(layout paths.Layout) redpanda.Config {
	return redpanda.Config{
		KafkaPort: 9092,
		AdminPort: 9644,
		RPCPort:   33145,
		DataDir:   filepath.Join(layout.DataDir, "redpanda"),
		BinDir:    filepath.Join(layout.RuntimeDir, "redpanda", "bin"),
	}
}

func runControlPlane() {
	_, layout := loadSettingsOrExit()

	if err := os.MkdirAll(layout.DataDir, 0o755); err != nil {
		exitf("cannot create data dir: %v", err)
	}

	s := supervisor.New()

	// Ordered startup per cahier des charges §5.3.1: Redpanda → ClickHouse
	// → Rust → Python → API. The last three slots join once they expose
	// long-running daemons (M1.2, M4, M1.5).
	if runtime.GOOS == "linux" {
		rpCfg := redpandaConfig(layout)
		if err := redpanda.GenerateConfig(rpCfg); err != nil {
			exitf("generate redpanda config: %v", err)
		}
		s.Register(redpanda.NewProcess(rpCfg), 30*time.Second, supervisor.Backoff{})
	} else {
		fmt.Printf("supervisor: skipping Redpanda on %s/%s — Linux-only binary (engine/redpanda is unit-tested via mocks; see tasks-herminas.md M0.5)\n", runtime.GOOS, runtime.GOARCH)
	}

	chCfg := clickhouseConfig(layout)
	if err := clickhouse.GenerateConfig(chCfg); err != nil {
		exitf("generate clickhouse config: %v", err)
	}
	if _, err := os.Stat(chCfg.BinaryPath()); err != nil {
		exitf("clickhouse binary not found at %s — run clickhouse.Download first", chCfg.BinaryPath())
	}
	s.Register(clickhouse.NewProcess(chCfg), 30*time.Second, supervisor.Backoff{})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("supervisor: starting services...")
	if err := s.Start(ctx); err != nil {
		_ = s.Stop(context.Background())
		exitf("supervisor: start failed: %v", err)
	}
	fmt.Println("supervisor: all services healthy")

	<-ctx.Done()
	fmt.Println("supervisor: shutting down...")
	if err := s.Stop(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "supervisor: stop error: %v\n", err)
	}
	fmt.Println("supervisor: stopped cleanly")
}

func statusControlPlane() {
	_, layout := loadSettingsOrExit()

	s := supervisor.New()
	if runtime.GOOS == "linux" {
		s.Register(redpanda.NewProcess(redpandaConfig(layout)), 0, supervisor.Backoff{})
	}
	s.Register(clickhouse.NewProcess(clickhouseConfig(layout)), 0, supervisor.Backoff{})

	for _, st := range s.Status(context.Background()) {
		fmt.Printf("%-14s %-10s %s\n", st.Service, st.State, st.Message)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
