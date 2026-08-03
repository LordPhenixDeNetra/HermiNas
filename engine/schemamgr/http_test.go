package schemamgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(openTestStore(t))
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHTTPCreateAndGet(t *testing.T) {
	h := newTestHandler(t).Routes()

	rec := doJSON(t, h, "POST", "/api/v1/datasets", createRequest{
		Name:    "logs",
		Columns: []Column{{Name: "message", Type: "String"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, "GET", "/api/v1/datasets/logs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got Dataset
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "logs" {
		t.Fatalf("unexpected dataset: %+v", got)
	}
}

func TestHTTPCreateDuplicateReturns409(t *testing.T) {
	h := newTestHandler(t).Routes()
	body := createRequest{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}}

	doJSON(t, h, "POST", "/api/v1/datasets", body)
	rec := doJSON(t, h, "POST", "/api/v1/datasets", body)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPCreateInvalidReturns400(t *testing.T) {
	h := newTestHandler(t).Routes()
	rec := doJSON(t, h, "POST", "/api/v1/datasets", createRequest{Name: "no-columns"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPGetMissingReturns404(t *testing.T) {
	h := newTestHandler(t).Routes()
	rec := doJSON(t, h, "GET", "/api/v1/datasets/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPListReturnsAllDatasets(t *testing.T) {
	h := newTestHandler(t).Routes()
	doJSON(t, h, "POST", "/api/v1/datasets", createRequest{Name: "a", Columns: []Column{{Name: "x", Type: "String"}}})
	doJSON(t, h, "POST", "/api/v1/datasets", createRequest{Name: "b", Columns: []Column{{Name: "x", Type: "String"}}})

	rec := doJSON(t, h, "GET", "/api/v1/datasets", nil)
	var list []Dataset
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 datasets, got %d", len(list))
	}
}

func TestHTTPAddColumnsAndVersionHistory(t *testing.T) {
	h := newTestHandler(t).Routes()
	doJSON(t, h, "POST", "/api/v1/datasets", createRequest{Name: "logs", Columns: []Column{{Name: "message", Type: "String"}}})

	rec := doJSON(t, h, "POST", "/api/v1/datasets/logs/columns", addColumnsRequest{
		Columns: []Column{{Name: "level", Type: "String"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var evolved Dataset
	if err := json.Unmarshal(rec.Body.Bytes(), &evolved); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if evolved.Version != 2 {
		t.Fatalf("expected version 2, got %d", evolved.Version)
	}

	rec = doJSON(t, h, "GET", "/api/v1/datasets/logs/versions", nil)
	var versions []Dataset
	if err := json.Unmarshal(rec.Body.Bytes(), &versions); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
}

func TestHTTPAddColumnsIncompatibleReturns400(t *testing.T) {
	h := newTestHandler(t).Routes()
	doJSON(t, h, "POST", "/api/v1/datasets", createRequest{Name: "logs", Columns: []Column{{Name: "message", Type: "String"}}})

	rec := doJSON(t, h, "POST", "/api/v1/datasets/logs/columns", addColumnsRequest{
		Columns: []Column{{Name: "message", Type: "Int64"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
