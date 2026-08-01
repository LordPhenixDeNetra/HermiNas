// Package redpanda integrates the embedded Redpanda engine: config
// generation, versioned binary download and health checks. Redpanda is a
// supervised black box (leçon aNtaerus) — HermiNas pilots it, never forks
// it.
//
// Redpanda ships Linux binaries only. Download returns a clear error on
// other platforms; GenerateConfig and the health check are pure
// logic/HTTP-client code and are fully exercised by redpanda_test.go via a
// mock admin API on every platform, including macOS dev machines.
package redpanda

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"herminas/engine/supervisor"
	"herminas/kernel/contracts"
)

// Config describes one embedded Redpanda instance (single-node mode).
type Config struct {
	KafkaPort int
	AdminPort int
	RPCPort   int
	DataDir   string
	BinDir    string // holds the redpanda binary and redpanda.yaml
}

func (c Config) ConfigPath() string { return filepath.Join(c.BinDir, "redpanda.yaml") }
func (c Config) BinaryPath() string { return filepath.Join(c.BinDir, "redpanda") }

// GenerateConfig writes redpanda.yaml for cfg (single-node, developer
// mode), creating BinDir and DataDir as needed.
func GenerateConfig(cfg Config) error {
	if err := os.MkdirAll(cfg.BinDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	yaml := fmt.Sprintf(`redpanda:
  data_directory: %q
  node_id: 0
  developer_mode: true
  kafka_api:
    - address: 127.0.0.1
      port: %d
  rpc_server:
    address: 127.0.0.1
    port: %d
  admin:
    - address: 127.0.0.1
      port: %d
`, cfg.DataDir, cfg.KafkaPort, cfg.RPCPort, cfg.AdminPort)

	if err := os.WriteFile(cfg.ConfigPath(), []byte(yaml), 0o644); err != nil {
		return fmt.Errorf("write redpanda.yaml: %w", err)
	}
	return nil
}

// Download fetches the official Redpanda binary for the current OS/arch.
// Redpanda ships Linux builds only; on any other platform (e.g. this
// project's macOS dev machines) it returns a clear error instead of
// silently failing later — run Redpanda via Docker/`rpk container` for
// local dev, or rely on the Linux bundle target (M8.1) for the real thing.
func Download(destDir string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("redpanda ships Linux binaries only (host is %s/%s) — use Docker locally, or the Linux bundle target", runtime.GOOS, runtime.GOARCH)
	}

	switch runtime.GOARCH {
	case "amd64", "arm64":
		// fall through
	default:
		return "", fmt.Errorf("no Redpanda build known for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// The exact release asset URL is pinned once M0.5 settles on a Redpanda
	// version to track (mirrors ClickHouse's Download in the sibling
	// package); left unwired until this runs on a Linux target where it can
	// actually be exercised end to end.
	return "", fmt.Errorf("redpanda download not yet wired for %s/%s (tracked for the Linux bundle target, M8.1)", runtime.GOOS, runtime.GOARCH)
}

// NewProcess builds a supervisor.Process that runs `redpanda start` against
// cfg, health-checked via the admin API's readiness endpoint.
func NewProcess(cfg Config) *supervisor.ExecProcess {
	return supervisor.NewExecProcess(
		contracts.ServiceRedpanda,
		cfg.BinaryPath(),
		[]string{"start", "--config", cfg.ConfigPath()},
		supervisor.WithHealthCheck(healthCheck(cfg.AdminPort)),
	)
}

func healthCheck(adminPort int) supervisor.HealthCheckFunc {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/status/ready", adminPort)

	return func(ctx context.Context) (contracts.HealthStatus, error) {
		now := time.Now()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return unhealthy(now, err), nil
		}
		resp, err := client.Do(req)
		if err != nil {
			return unhealthy(now, err), nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return unhealthy(now, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)), nil
		}

		return contracts.HealthStatus{Service: contracts.ServiceRedpanda, State: contracts.HealthHealthy, CheckedAt: now}, nil
	}
}

func unhealthy(at time.Time, err error) contracts.HealthStatus {
	return contracts.HealthStatus{Service: contracts.ServiceRedpanda, State: contracts.HealthUnhealthy, Message: err.Error(), CheckedAt: at}
}
