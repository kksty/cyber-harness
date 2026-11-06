package toolargs

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/chainreactors/cyber/core/operation"
	coretool "github.com/chainreactors/cyber/core/tool"
)

func TestInvocationPathsPreferExecutionThenContextThenCommandDefault(t *testing.T) {
	root := t.TempDir()
	base := Base{WorkDir: filepath.Join(root, "host")}
	ctx := operation.ContextWithInvocation(context.Background(), operation.Invocation{WorkDir: filepath.Join(root, "context")})
	flags := map[string]bool{"-l": true, "-f": true}
	abs := filepath.Join(root, "absolute.jsonl")
	args := []string{"-l", "targets.txt", "-f=raw/result.jsonl", "-o", "jl", "-f", abs, "-l", "-"}
	original := append([]string(nil), args...)
	for _, tc := range []struct {
		name, executionDir, want string
		ctx                      context.Context
	}{
		{"execution", filepath.Join(root, "case"), filepath.Join(root, "case"), ctx},
		{"context", "", filepath.Join(root, "context"), ctx},
		{"default", "", base.WorkDir, context.Background()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := base.ResolvePaths(tc.ctx, &coretool.Execution{Args: args, Dir: tc.executionDir}, flags)
			want := []string{"-l", filepath.Join(tc.want, "targets.txt"), "-f=" + filepath.Join(tc.want, "raw/result.jsonl"), "-o", "jl", "-f", abs, "-l", "-"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got=%v want=%v", got, want)
			}
		})
	}
	if !reflect.DeepEqual(args, original) || base.WorkDir != filepath.Join(root, "host") {
		t.Fatal("invocation changed shared state")
	}
}

func TestConcurrentInvocationsKeepIndependentFilePaths(t *testing.T) {
	root := t.TempDir()
	base := Base{WorkDir: root}
	flags := map[string]bool{"-f": true}
	var wg sync.WaitGroup
	for _, directory := range []string{"one", "two", "three", "four"} {
		wg.Go(func() {
			dir := filepath.Join(root, directory)
			for range 100 {
				got := base.ResolvePaths(context.Background(), &coretool.Execution{Dir: dir, Args: []string{"-f", "result.jsonl"}}, flags)
				if got[1] != filepath.Join(dir, "result.jsonl") {
					t.Errorf("crossed execution directories: %v", got)
				}
			}
		})
	}
	wg.Wait()
	if base.WorkDir != root {
		t.Fatal("shared default mutated")
	}
}
