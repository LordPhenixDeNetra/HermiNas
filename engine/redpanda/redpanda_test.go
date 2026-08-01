package redpanda

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"herminas/kernel/contracts"
)

func TestGenerateConfigWritesExpectedFile(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		KafkaPort: 9092,
		AdminPort: 9644,
		RPCPort:   33145,
		DataDir:   filepath.Join(dir, "data"),
		BinDir:    filepath.Join(dir, "bin"),
	}

	if err := GenerateConfig(cfg); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	b, err := os.ReadFile(cfg.ConfigPath())
	if err != nil {
		t.Fatalf("reading redpanda.yaml: %v", err)
	}
	if !strings.Contains(string(b), "port: 9092") {
		t.Fatalf("redpanda.yaml missing kafka port: %s", b)
	}
	if !strings.Contains(string(b), "port: 9644") {
		t.Fatalf("redpanda.yaml missing admin port: %s", b)
	}
}

// TestDownloadRejectsNonLinuxPlatforms documents and locks in the honest
// platform limitation: Redpanda ships Linux binaries only, so Download must
// fail loudly rather than silently on macOS/Windows dev machines.
func TestDownloadRejectsNonLinuxPlatforms(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("platform gate only applies to non-Linux hosts")
	}
	if _, err := Download(t.TempDir()); err == nil {
		t.Fatal("expected Download to fail on a non-Linux platform")
	}
}

// mockRedpandaAdminServer stands in for Redpanda's admin API in unit tests
// (M0.5: "Créer mocks ClickHouse/Redpanda pour tests unitaires").
func mockRedpandaAdminServer(ready bool) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status/ready", func(w http.ResponseWriter, r *http.Request) {
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"status":"ready"}`))
	})
	return httptest.NewServer(mux)
}

func portFromURL(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestHealthCheckHealthyWhenReady(t *testing.T) {
	srv := mockRedpandaAdminServer(true)
	defer srv.Close()

	check := healthCheck(portFromURL(t, srv.URL))
	status, err := check(context.Background())
	if err != nil {
		t.Fatalf("healthCheck returned error: %v", err)
	}
	if status.State != contracts.HealthHealthy {
		t.Fatalf("expected healthy, got %s (%s)", status.State, status.Message)
	}
}

func TestHealthCheckUnhealthyWhenNotReady(t *testing.T) {
	srv := mockRedpandaAdminServer(false)
	defer srv.Close()

	check := healthCheck(portFromURL(t, srv.URL))
	status, err := check(context.Background())
	if err != nil {
		t.Fatalf("healthCheck returned error: %v", err)
	}
	if status.State != contracts.HealthUnhealthy {
		t.Fatalf("expected unhealthy, got %s", status.State)
	}
}
