package curl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNullOutputPreservesWriteOutWithoutCreatingCaseFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("discard me"))
	}))
	defer srv.Close()
	for _, null := range []string{"/dev/null", os.DevNull} {
		t.Run(null, func(t *testing.T) {
			dir := t.TempDir()
			out, _, err := run(t, []string{"-s", "-o", null, "-D", null, "--trace-ascii", null, "-w", "%{http_code}", srv.URL}, "", dir)
			if err != nil || out != "200" {
				t.Fatalf("output=%q error=%v", out, err)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("null device became a case file: %v %v", files, err)
			}
		})
	}
}
