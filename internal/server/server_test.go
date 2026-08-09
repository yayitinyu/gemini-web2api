package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAFallbackAndImmutableAssets(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"index.html":           &fstest.MapFile{Data: []byte("<main>app</main>")},
		"assets/app-abc123.js": &fstest.MapFile{Data: []byte("console.log('ok')")},
	}
	root, err := fs.Sub(files, ".")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSPA(root)

	fallback := httptest.NewRecorder()
	handler.ServeHTTP(fallback, httptest.NewRequest(http.MethodGet, "/admin/settings", nil))
	if fallback.Code != http.StatusOK || !strings.Contains(fallback.Body.String(), "<main>app</main>") {
		t.Fatalf("SPA fallback failed: %d %s", fallback.Code, fallback.Body.String())
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/admin/assets/app-abc123.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset serving failed: %d headers=%v", asset.Code, asset.Header())
	}
}
