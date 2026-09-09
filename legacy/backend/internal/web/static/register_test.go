package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMountServesAssetsAndSPA(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	if err := Mount(mux, "/autopack", root); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/autopack/assets/app.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("assets status %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/autopack/dashboard", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("spa status %d", rec2.Code)
	}
}

func TestSpaFallbackAPI404(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "index.html"), []byte("ok"), 0o644)

	mux := http.NewServeMux()
	_ = Mount(mux, "/autopack", root)

	req := httptest.NewRequest(http.MethodGet, "/autopack/api/unknown", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("api 404 status %d", rec.Code)
	}
}
