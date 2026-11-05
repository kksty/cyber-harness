package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainreactors/cyber/core/operation"
)

// The files extension stays rooted at the agent directory, but a session may
// set a narrower workspace inside it (Tianwu fork patch 4). Relative paths must
// resolve there for read/write/list/glob, exactly like bash, or the write tool
// still drops files at the agent root.
func TestSessionWorkDirScopesFileTools(t *testing.T) {
	root := t.TempDir()
	resource, err := New(Config{Directory: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := resource.Files
	set := filesystemSet(t, resource)
	if err := set.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = set.Close(context.Background()) }()

	work := filepath.Join(root, "projects", "p", "cases", "in_1")
	ctx := operation.ContextWithInvocation(t.Context(), operation.Invocation{WorkDir: work})
	if err := f.Mkdir(ctx, ".tw"); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(ctx, ".tw/facts.jsonl", []byte(`{"legacy":false}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work, ".tw", "facts.jsonl")); err != nil {
		t.Fatalf("write did not land in the session workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".tw", "facts.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("write escaped to the agent root: %v", err)
	}
	data, err := f.Read(ctx, ".tw/facts.jsonl")
	if err != nil || len(data) == 0 {
		t.Fatalf("read from the session workspace: %q, %v", data, err)
	}
	if entries, err := f.List(ctx, ".tw"); err != nil || len(entries) != 1 {
		t.Fatalf("list in the session workspace = %v, %v", entries, err)
	}
	if matches, err := f.Glob(ctx, ".tw/*.jsonl", 10); err != nil || len(matches) != 1 {
		t.Fatalf("glob in the session workspace = %v, %v", matches, err)
	}

	// A workspace outside the root would widen os.Root; refuse it instead.
	outside := operation.ContextWithInvocation(t.Context(), operation.Invocation{WorkDir: t.TempDir()})
	if _, err := f.Read(outside, "x"); err == nil {
		t.Fatal("a session workspace outside the tool root was accepted")
	}
}
