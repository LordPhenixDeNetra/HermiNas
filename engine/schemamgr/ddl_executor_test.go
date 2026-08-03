package schemamgr

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// mockDDLExecutor proves Handler calls out to apply DDL, without needing a
// real ClickHouse — the actual HTTP-backed executor (engine/api) is
// exercised for real in engine/api's integration test.
type mockDDLExecutor struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (m *mockDDLExecutor) Exec(_ context.Context, ddl string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return http.ErrHandlerTimeout // any error value; content doesn't matter for these tests
	}
	m.calls = append(m.calls, ddl)
	return nil
}

func TestCreateWithDDLExecutorAppliesGeneratedDDL(t *testing.T) {
	exec := &mockDDLExecutor{}
	h := NewHandler(openTestStore(t)).WithDDLExecutor(exec).Routes()

	rec := doJSON(t, h, "POST", "/api/v1/datasets", createRequest{
		Name:    "logs",
		Columns: []Column{{Name: "message", Type: "String"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()
	if len(exec.calls) != 1 {
		t.Fatalf("expected 1 DDL execution, got %d", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0], "CREATE TABLE IF NOT EXISTS `logs`") {
		t.Errorf("unexpected DDL: %s", exec.calls[0])
	}
}

func TestCreateWithoutDDLExecutorSkipsClickHouse(t *testing.T) {
	// The default (no WithDDLExecutor call) must stay dependency-free —
	// this is what keeps schemamgr's own test suite fast and ClickHouse-free.
	h := NewHandler(openTestStore(t)).Routes()
	rec := doJSON(t, h, "POST", "/api/v1/datasets", createRequest{
		Name:    "logs",
		Columns: []Column{{Name: "message", Type: "String"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateReportsDDLExecutionFailure(t *testing.T) {
	exec := &mockDDLExecutor{fail: true}
	h := NewHandler(openTestStore(t)).WithDDLExecutor(exec).Routes()

	rec := doJSON(t, h, "POST", "/api/v1/datasets", createRequest{
		Name:    "logs",
		Columns: []Column{{Name: "message", Type: "String"}},
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when DDL execution fails, got %d", rec.Code)
	}
}

func TestAddColumnsWithDDLExecutorAppliesAlterTable(t *testing.T) {
	exec := &mockDDLExecutor{}
	h := NewHandler(openTestStore(t)).WithDDLExecutor(exec)
	routes := h.Routes()

	doJSON(t, routes, "POST", "/api/v1/datasets", createRequest{
		Name:    "logs",
		Columns: []Column{{Name: "message", Type: "String"}},
	})

	rec := doJSON(t, routes, "POST", "/api/v1/datasets/logs/columns", addColumnsRequest{
		Columns: []Column{{Name: "level", Type: "String"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 DDL executions (create + alter), got %d", len(exec.calls))
	}
	if !strings.Contains(exec.calls[1], "ALTER TABLE `logs`") || !strings.Contains(exec.calls[1], "ADD COLUMN `level`") {
		t.Errorf("unexpected ALTER DDL: %s", exec.calls[1])
	}
}
