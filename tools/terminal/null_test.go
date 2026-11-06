package terminal

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPOSIXNullRedirectionWorksWithoutAnExternalShell(t *testing.T) {
	dir := t.TempDir()
	bash := NewBashTool(dir, 5, nil)
	defer bash.Close()
	var output bytes.Buffer
	_, err := bash.RunForeground(t.Context(), "printf 'discard' >/dev/null; printf 'kept'", BashExecOptions{OnOutput: func(data []byte) { _, _ = output.Write(data) }})
	if err != nil || output.String() != "kept" {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Name() == "dev" || strings.EqualFold(file.Name(), "nul") {
			t.Fatalf("null device became a case file: %s", file.Name())
		}
	}
}
