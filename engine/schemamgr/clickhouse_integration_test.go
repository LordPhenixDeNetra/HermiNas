package schemamgr

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

// TestGeneratedDDLIsValidClickHouseSQL proves GenerateDDL's output isn't
// just a plausible-looking string: it actually creates a table in a real
// ClickHouse, accepts an insert shaped like the dataset's columns, and
// returns it via SELECT. Skips (not fails) if no ClickHouse is reachable,
// consistent with every other real-service test in this repo — start one
// with `go run bootstrap.go run` (M0.5 supervisor) to exercise this.
func TestGeneratedDDLIsValidClickHouseSQL(t *testing.T) {
	baseURL := os.Getenv("HERMINAS_CLICKHOUSE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8123"
	}

	if _, err := chQuery(baseURL, "SELECT 1", ""); err != nil {
		t.Skipf("no ClickHouse reachable at %s (%v) — run `go run bootstrap.go run` to exercise this test", baseURL, err)
	}

	sample := map[string]any{
		"agent_id":            "agent-1",
		"dataset":             "e2e_schemamgr",
		"message":             "hello from schemamgr",
		"received_at_unix_ms": float64(1_700_000_000_000),
	}
	columns := InferColumns(sample)

	table := fmt.Sprintf("schemamgr_it_%d", os.Getpid())
	d := Dataset{
		Name:    table,
		Columns: columns,
		OrderBy: []string{"received_at_unix_ms"},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("generated dataset failed validation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = chQuery(baseURL, fmt.Sprintf("DROP TABLE IF EXISTS %s", table), "")
	})

	ddl := GenerateDDL(d)
	if _, err := chQuery(baseURL, ddl, ""); err != nil {
		t.Fatalf("generated DDL was rejected by ClickHouse: %v\nDDL was:\n%s", err, ddl)
	}

	insertBody := `{"agent_id":"agent-1","dataset":"e2e_schemamgr","message":"hello from schemamgr","received_at_unix_ms":1700000000000}` + "\n"
	if _, err := chQuery(baseURL, fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", table), insertBody); err != nil {
		t.Fatalf("insert into generated table failed: %v", err)
	}

	out, err := chQuery(baseURL, fmt.Sprintf("SELECT message FROM %s FORMAT TabSeparated", table), "")
	if err != nil {
		t.Fatalf("select from generated table failed: %v", err)
	}
	if strings.TrimSpace(out) != "hello from schemamgr" {
		t.Fatalf("unexpected row content: %q", out)
	}
}

func chQuery(baseURL, query, body string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/?"+url.Values{"query": {query}}.Encode(), strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.ContentLength = int64(len(body)) // ClickHouse's HTTP interface 411s without this, even for an empty body

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, respBody)
	}
	return string(respBody), nil
}
