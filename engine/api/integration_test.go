package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"herminas/engine/auth"
	"herminas/engine/querybroker"
	"herminas/engine/schemamgr"
	"herminas/kernel/permissions"
)

// newTestRouter wires the same Deps a real deployment would, backed by
// temp SQLite files, skipping (not failing) if no ClickHouse is reachable
// — consistent with every other real-service test in this repo.
func newTestRouter(t *testing.T) (http.Handler, *auth.Store) {
	t.Helper()

	clickhouseURL := os.Getenv("HERMINAS_CLICKHOUSE_URL")
	if clickhouseURL == "" {
		clickhouseURL = "http://127.0.0.1:8123"
	}
	req, _ := http.NewRequest(http.MethodGet, clickhouseURL+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("no ClickHouse reachable at %s (%v) — run `go run bootstrap.go run` to exercise this test", clickhouseURL, err)
	}
	resp.Body.Close()

	dir := t.TempDir()

	authStore, err := auth.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("auth.Open failed: %v", err)
	}
	t.Cleanup(func() { authStore.Close() })

	schemas, err := schemamgr.Open(filepath.Join(dir, "schemamgr.db"))
	if err != nil {
		t.Fatalf("schemamgr.Open failed: %v", err)
	}
	t.Cleanup(func() { schemas.Close() })

	broker, err := querybroker.New(querybroker.Config{
		ClickHouseURL: clickhouseURL,
		AuditLogPath:  filepath.Join(dir, "audit.jsonl"),
	})
	if err != nil {
		t.Fatalf("querybroker.New failed: %v", err)
	}
	t.Cleanup(func() { broker.Close() })

	jwtMgr := auth.NewJWTManager([]byte("test-secret"), time.Hour)

	router := NewRouter(Deps{
		AuthStore:  authStore,
		JWTManager: jwtMgr,
		Schemas:    schemas,
		Broker:     broker,
		Inserter:   NewInserter(clickhouseURL),
		DDL:        NewClickHouseDDLExecutor(clickhouseURL),
	})

	return router, authStore
}

func decodeJSON[T any](t *testing.T, body *bytes.Buffer) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(body).Decode(&v); err != nil {
		t.Fatalf("decode response body: %v (body was %q)", err, body.String())
	}
	return v
}

func TestFullAPIFlow_LoginQueryDatasetIngest(t *testing.T) {
	router, authStore := newTestRouter(t)
	srv := httptest.NewServer(router)
	defer srv.Close()

	if _, err := authStore.CreateUser("alice", "correct-horse", permissions.RoleEngineer); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 1. Login: get a real JWT for a real user.
	loginBody, _ := json.Marshal(loginRequest{Username: "alice", Password: "correct-horse"})
	resp, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from login, got %d", resp.StatusCode)
	}
	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	resp.Body.Close()
	if loginResp.Token == "" {
		t.Fatal("expected a non-empty session token")
	}
	authHeader := "Bearer " + loginResp.Token

	// 2. Query: run real SQL against real ClickHouse through the router.
	queryBody, _ := json.Marshal(queryRequest{SQL: "SELECT number FROM system.numbers LIMIT 3"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/query", bytes.NewReader(queryBody))
	req.Header.Set("Authorization", authHeader)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("query request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("expected 200 from query, got %d: %s", resp.StatusCode, body.String())
	}
	var queryResult struct {
		RowCount int               `json:"row_count"`
		Rows     []json.RawMessage `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queryResult); err != nil {
		t.Fatalf("decode query response: %v", err)
	}
	resp.Body.Close()
	if queryResult.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", queryResult.RowCount)
	}

	// 3. Datasets: create one via the mounted schemamgr routes.
	table := fmt.Sprintf("api_it_%d", os.Getpid())
	createBody, _ := json.Marshal(map[string]any{
		"name": table,
		"columns": []map[string]any{
			{"name": "message", "type": "String"},
		},
	})
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasets", bytes.NewReader(createBody))
	req.Header.Set("Authorization", authHeader)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create dataset request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("expected 201 from dataset create, got %d: %s", resp.StatusCode, body.String())
	}
	resp.Body.Close()
	t.Cleanup(func() { httpDropTable(t, table) })

	// A viewer-role token must not be able to create datasets (RBAC).
	if _, err := authStore.CreateUser("viewer-only", "password123", permissions.RoleViewer); err != nil {
		t.Fatalf("CreateUser(viewer-only) failed: %v", err)
	}
	viewerLoginBody, _ := json.Marshal(loginRequest{Username: "viewer-only", Password: "password123"})
	resp, _ = http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(viewerLoginBody))
	var viewerLogin loginResponse
	json.NewDecoder(resp.Body).Decode(&viewerLogin)
	resp.Body.Close()

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasets", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+viewerLogin.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("viewer create-dataset request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer creating a dataset, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Ingest: push a record via HTTP, using a scoped API token, and
	// confirm it actually lands in ClickHouse — not just "202 Accepted".
	rawToken, _, err := authStore.CreateAPIToken([]string{table}, permissions.RoleEngineer, 0)
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}

	ingestBody := []byte(`{"message":"hello from the api integration test"}` + "\n")
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/ingest/"+table, bytes.NewReader(ingestBody))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ingest request failed: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("expected 202 from ingest, got %d: %s", resp.StatusCode, body.String())
	}
	resp.Body.Close()

	// A token scoped to a *different* dataset must be rejected.
	otherToken, _, _ := authStore.CreateAPIToken([]string{"some-other-dataset"}, permissions.RoleEngineer, 0)
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/ingest/"+table, bytes.NewReader(ingestBody))
	req.Header.Set("Authorization", "Bearer "+otherToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("out-of-scope ingest request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for an out-of-scope token, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Confirm the ingested row is really queryable.
	verifyBody, _ := json.Marshal(queryRequest{SQL: fmt.Sprintf("SELECT message FROM %s", table)})
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/query", bytes.NewReader(verifyBody))
	req.Header.Set("Authorization", authHeader)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("verification query failed: %v", err)
	}
	var verifyResult struct {
		RowCount int `json:"row_count"`
	}
	json.NewDecoder(resp.Body).Decode(&verifyResult)
	resp.Body.Close()
	if verifyResult.RowCount != 1 {
		t.Fatalf("expected the ingested row to be queryable, got row_count=%d", verifyResult.RowCount)
	}
}

func TestHealthEndpointRequiresNoAuth(t *testing.T) {
	router, _ := newTestRouter(t)
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestQueryEndpointRequiresAuth(t *testing.T) {
	router, _ := newTestRouter(t)
	srv := httptest.NewServer(router)
	defer srv.Close()

	body, _ := json.Marshal(queryRequest{SQL: "SELECT 1"})
	resp, err := http.Post(srv.URL+"/api/v1/query", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", resp.StatusCode)
	}
}

func httpDropTable(t *testing.T, table string) {
	t.Helper()
	clickhouseURL := os.Getenv("HERMINAS_CLICKHOUSE_URL")
	if clickhouseURL == "" {
		clickhouseURL = "http://127.0.0.1:8123"
	}
	req, _ := http.NewRequest(http.MethodPost, clickhouseURL, nil)
	q := req.URL.Query()
	q.Set("query", "DROP TABLE IF EXISTS "+table)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Length", "0")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
