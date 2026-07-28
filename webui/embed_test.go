package webui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// handler mirrors how main.go mounts the embedded UI.
func handler(t *testing.T) http.Handler {
	t.Helper()
	uiFS, err := FS()
	if err != nil {
		t.Fatalf("FS(): %v", err)
	}
	return http.StripPrefix("/web", http.FileServer(http.FS(uiFS)))
}

func TestEmbeddedFSContainsIndex(t *testing.T) {
	uiFS, err := FS()
	if err != nil {
		t.Fatalf("FS(): %v", err)
	}
	if _, err := fs.Stat(uiFS, "index.html"); err != nil {
		t.Fatalf("index.html missing from embedded FS: %v", err)
	}
}

func TestServesIndexAtWebRoot(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/web/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /web/ = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<html") {
		t.Errorf("GET /web/ did not return an HTML document, got %.80q", body)
	}
}

// http.FileServer canonicalises an explicit /index.html to ./ with a 301. The
// redirect must stay inside the /web prefix — a StripPrefix mounted handler
// that emits an absolute Location would send the client out of the UI.
func TestIndexRedirectStaysUnderWebPrefix(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/web/index.html", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /web/index.html = %d, want 301", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "/web/") {
		t.Errorf("redirect escapes the UI mount point: Location = %q", loc)
	}
}

func TestUnknownAssetIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/web/does-not-exist.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /web/does-not-exist.js = %d, want 404", rec.Code)
	}
}
