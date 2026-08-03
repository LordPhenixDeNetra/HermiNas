package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"herminas/kernel/permissions"
)

func testDeps(t *testing.T) (*JWTManager, *Store) {
	t.Helper()
	return NewJWTManager([]byte("test-secret"), time.Hour), openTestStore(t)
}

func echoIdentityHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := IdentityFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Subject", id.Subject)
		w.Header().Set("X-Role", string(id.Role))
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthenticateAcceptsValidJWT(t *testing.T) {
	jwtMgr, store := testDeps(t)
	token, _ := jwtMgr.Issue("alice", permissions.RoleAnalyst)

	handler := Authenticate(jwtMgr, store)(echoIdentityHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Subject") != "alice" || rec.Header().Get("X-Role") != string(permissions.RoleAnalyst) {
		t.Fatalf("unexpected identity headers: %v", rec.Header())
	}
}

func TestAuthenticateAcceptsValidAPIToken(t *testing.T) {
	jwtMgr, store := testDeps(t)
	raw, created, _ := store.CreateAPIToken([]string{"logs"}, permissions.RoleEngineer, 0)

	handler := Authenticate(jwtMgr, store)(echoIdentityHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Subject") != created.ID {
		t.Fatalf("expected subject %q, got %q", created.ID, rec.Header().Get("X-Subject"))
	}
}

func TestAuthenticateRejectsMissingHeader(t *testing.T) {
	jwtMgr, store := testDeps(t)
	handler := Authenticate(jwtMgr, store)(echoIdentityHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthenticateRejectsGarbageCredential(t *testing.T) {
	jwtMgr, store := testDeps(t)
	handler := Authenticate(jwtMgr, store)(echoIdentityHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRoleAllowsSufficientRole(t *testing.T) {
	jwtMgr, store := testDeps(t)
	token, _ := jwtMgr.Issue("alice", permissions.RoleAdmin)

	handler := Authenticate(jwtMgr, store)(RequireRole(permissions.RoleEngineer)(echoIdentityHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRoleRejectsInsufficientRole(t *testing.T) {
	jwtMgr, store := testDeps(t)
	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)

	handler := Authenticate(jwtMgr, store)(RequireRole(permissions.RoleAdmin)(echoIdentityHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireDatasetScopeAllowsJWTSessionRegardlessOfDataset(t *testing.T) {
	jwtMgr, store := testDeps(t)
	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)

	byDataset := func(r *http.Request) string { return "any-dataset" }
	handler := Authenticate(jwtMgr, store)(RequireDatasetScope(byDataset)(echoIdentityHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireDatasetScopeRejectsTokenOutsideItsScope(t *testing.T) {
	jwtMgr, store := testDeps(t)
	raw, _, _ := store.CreateAPIToken([]string{"logs"}, permissions.RoleEngineer, 0)

	byDataset := func(r *http.Request) string { return "metrics" }
	handler := Authenticate(jwtMgr, store)(RequireDatasetScope(byDataset)(echoIdentityHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
