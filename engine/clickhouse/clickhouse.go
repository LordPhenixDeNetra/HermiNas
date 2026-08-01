// Package clickhouse integrates the embedded ClickHouse engine: config
// generation, versioned binary download and health checks. ClickHouse is a
// supervised black box (leçon aNtaerus) — HermiNas pilots it, never forks
// it.
package clickhouse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"herminas/engine/supervisor"
	"herminas/kernel/contracts"
)

// Config describes one embedded ClickHouse instance.
type Config struct {
	HTTPPort int
	TCPPort  int
	DataDir  string
	BinDir   string // holds the clickhouse binary, config.xml and users.xml
}

func (c Config) ConfigPath() string { return filepath.Join(c.BinDir, "config.xml") }
func (c Config) UsersPath() string  { return filepath.Join(c.BinDir, "users.xml") }
func (c Config) BinaryPath() string { return filepath.Join(c.BinDir, "clickhouse") }

// GenerateConfig writes config.xml and users.xml for cfg, creating BinDir
// and DataDir as needed. Safe to call repeatedly (overwrites in place).
func GenerateConfig(cfg Config) error {
	if err := os.MkdirAll(cfg.BinDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	configXML := fmt.Sprintf(`<clickhouse>
    <logger>
        <level>information</level>
        <console>true</console>
    </logger>
    <http_port>%d</http_port>
    <tcp_port>%d</tcp_port>
    <listen_host>127.0.0.1</listen_host>
    <path>%s/</path>
    <tmp_path>%s/tmp/</tmp_path>
    <user_directories>
        <users_xml>
            <path>%s</path>
        </users_xml>
    </user_directories>
</clickhouse>
`, cfg.HTTPPort, cfg.TCPPort, cfg.DataDir, cfg.DataDir, cfg.UsersPath())

	// Dev-mode default user: no password, localhost only. Hardened per-role
	// access lands with RBAC (M1.5/M7.1) — the box itself is the trust
	// boundary until then (leçon HermiNas: souveraineté locale).
	usersXML := `<clickhouse>
    <profiles>
        <default>
            <max_memory_usage>4000000000</max_memory_usage>
        </default>
    </profiles>
    <users>
        <default>
            <password></password>
            <networks>
                <ip>::1</ip>
                <ip>127.0.0.1</ip>
            </networks>
            <profile>default</profile>
            <quota>default</quota>
        </default>
    </users>
    <quotas>
        <default />
    </quotas>
</clickhouse>
`

	if err := os.WriteFile(cfg.ConfigPath(), []byte(configXML), 0o644); err != nil {
		return fmt.Errorf("write config.xml: %w", err)
	}
	if err := os.WriteFile(cfg.UsersPath(), []byte(usersXML), 0o644); err != nil {
		return fmt.Errorf("write users.xml: %w", err)
	}
	return nil
}

// Download fetches the official ClickHouse static binary for the current
// OS/arch into destDir/clickhouse.
func Download(destDir string) (string, error) {
	var asset string
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		asset = "macos"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		asset = "macos-aarch64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		asset = "amd64"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		asset = "aarch64"
	default:
		return "", fmt.Errorf("no ClickHouse build known for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("https://builds.clickhouse.com/master/%s/clickhouse", asset)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, "clickhouse")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download clickhouse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download clickhouse: unexpected status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("write clickhouse binary: %w", err)
	}

	return dest, nil
}

// NewProcess builds a supervisor.Process that runs `clickhouse server`
// against cfg, health-checked via HTTP ping + a witness SELECT 1 query
// (M0.5: "healthchecks ClickHouse (HTTP ping + requête témoin)").
func NewProcess(cfg Config) *supervisor.ExecProcess {
	return supervisor.NewExecProcess(
		contracts.ServiceClickHouse,
		cfg.BinaryPath(),
		[]string{"server", "--config-file", cfg.ConfigPath()},
		supervisor.WithHealthCheck(healthCheck(cfg.HTTPPort)),
	)
}

func healthCheck(httpPort int) supervisor.HealthCheckFunc {
	client := &http.Client{Timeout: 2 * time.Second}
	base := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	return func(ctx context.Context) (contracts.HealthStatus, error) {
		now := time.Now()

		if err := probe(ctx, client, base+"/ping", "Ok."); err != nil {
			return unhealthy(now, err), nil
		}
		if err := probe(ctx, client, base+"/?query=SELECT+1", "1"); err != nil {
			return unhealthy(now, err), nil
		}

		return contracts.HealthStatus{Service: contracts.ServiceClickHouse, State: contracts.HealthHealthy, CheckedAt: now}, nil
	}
}

func probe(ctx context.Context, client *http.Client, url, want string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	got := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK || got != want {
		return fmt.Errorf("GET %s: status %d, body %q (want %q)", url, resp.StatusCode, got, want)
	}
	return nil
}

func unhealthy(at time.Time, err error) contracts.HealthStatus {
	return contracts.HealthStatus{Service: contracts.ServiceClickHouse, State: contracts.HealthUnhealthy, Message: err.Error(), CheckedAt: at}
}
