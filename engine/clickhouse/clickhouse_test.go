package clickhouse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"herminas/kernel/contracts"
)

func TestGenerateConfigWritesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		HTTPPort: 8123,
		TCPPort:  9000,
		DataDir:  filepath.Join(dir, "data"),
		BinDir:   filepath.Join(dir, "bin"),
	}

	if err := GenerateConfig(cfg); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	configBytes, err := os.ReadFile(cfg.ConfigPath())
	if err != nil {
		t.Fatalf("reading config.xml: %v", err)
	}
	if !strings.Contains(string(configBytes), "<http_port>8123</http_port>") {
		t.Fatalf("config.xml missing http_port: %s", configBytes)
	}
	if !strings.Contains(string(configBytes), "<tcp_port>9000</tcp_port>") {
		t.Fatalf("config.xml missing tcp_port: %s", configBytes)
	}

	if _, err := os.Stat(cfg.UsersPath()); err != nil {
		t.Fatalf("users.xml not created: %v", err)
	}
}

// mockClickHouseServer stands in for a real ClickHouse HTTP interface in
// unit tests (M0.5: "Créer mocks ClickHouse/Redpanda pour tests unitaires").
func mockClickHouseServer(ping, query string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ping))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "SELECT 1" {
			w.Write([]byte(query))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
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

func TestHealthCheckHealthyAgainstMockServer(t *testing.T) {
	srv := mockClickHouseServer("Ok.\n", "1\n")
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

func TestHealthCheckUnhealthyWhenPingWrong(t *testing.T) {
	srv := mockClickHouseServer("nope", "1\n")
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

func TestHealthCheckUnhealthyWhenServerDown(t *testing.T) {
	srv := mockClickHouseServer("Ok.\n", "1\n")
	port := portFromURL(t, srv.URL)
	srv.Close() // nothing listening anymore

	check := healthCheck(port)
	status, err := check(context.Background())
	if err != nil {
		t.Fatalf("healthCheck returned error: %v", err)
	}
	if status.State != contracts.HealthUnhealthy {
		t.Fatalf("expected unhealthy when server is down, got %s", status.State)
	}
}
