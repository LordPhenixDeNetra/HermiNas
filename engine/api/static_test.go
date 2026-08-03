package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeStaticFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa-shell</html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log('hi')"), 0o644); err != nil {
		t.Fatalf("write app.js: %v", err)
	}
	return dir
}

func TestSPAHandlerServesRealAsset(t *testing.T) {
	dir := writeStaticFixture(t)
	h := SPAHandler(dir)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/app.js", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "console.log('hi')" {
		t.Fatalf("body = %q, want the real asset content", rr.Body.String())
	}
}

func TestSPAHandlerFallsBackToIndexForClientRoutes(t *testing.T) {
	dir := writeStaticFixture(t)
	h := SPAHandler(dir)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/datasets/orders", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "<html>spa-shell</html>" {
		t.Fatalf("body = %q, want the index.html shell", rr.Body.String())
	}
}

func TestStaticDirAvailable(t *testing.T) {
	dir := writeStaticFixture(t)
	if !staticDirAvailable(dir) {
		t.Fatal("expected dir with index.html to be available")
	}
	if staticDirAvailable(filepath.Join(dir, "does-not-exist")) {
		t.Fatal("expected missing dir to be unavailable")
	}
	if staticDirAvailable("") {
		t.Fatal("expected empty dir to be unavailable")
	}
}

func TestNewRouterOmitsStaticRouteWhenUnset(t *testing.T) {
	router := NewRouter(Deps{})

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no StaticDir is configured", rr.Code)
	}
}
