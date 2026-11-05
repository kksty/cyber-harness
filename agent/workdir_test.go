package agent

import (
	"context"
	"testing"

	aop "github.com/chainreactors/cyber/aop"
	"github.com/chainreactors/cyber/core/operation"
	"github.com/chainreactors/cyber/core/telemetry"
)

// workDirRecorder reports the workspace a tool invocation carried.
type workDirRecorder struct{ dir string }

func (w *workDirRecorder) ToolDefinitions() []*aop.ToolDefinition { return nil }

func (w *workDirRecorder) ExecuteTool(ctx context.Context, name, arguments string) (*aop.ToolResult, error) {
	w.dir = operation.InvocationFromContext(ctx).WorkDir
	return &aop.ToolResult{}, nil
}

// The control plane runs many sessions in one agent process; each session's
// tools must resolve relative paths against that session's case directory, not
// the agent's process directory (Tianwu fork patch 4).
func TestToolInvocationCarriesSessionWorkDir(t *testing.T) {
	recorder := &workDirRecorder{}
	cfg := Config{
		Tools:   recorder,
		Logger:  telemetry.NopLogger(),
		WorkDir: "/data/projects/web/cases/in_1",
	}
	result := runToolCallSafely(context.Background(), cfg, &aop.ToolCall{Id: "call-1", Name: "bash"}, 1)
	if result == nil {
		t.Fatal("tool call returned no result")
	}
	if recorder.dir != cfg.WorkDir {
		t.Fatalf("invocation work dir = %q, want %q", recorder.dir, cfg.WorkDir)
	}
}

// An empty session work dir keeps the tools' own default: older control planes
// must not suddenly run relative to a path they never sent.
func TestToolInvocationWithoutWorkDirKeepsDefault(t *testing.T) {
	recorder := &workDirRecorder{}
	cfg := Config{Tools: recorder, Logger: telemetry.NopLogger()}
	_ = runToolCallSafely(context.Background(), cfg, &aop.ToolCall{Id: "call-2", Name: "bash"}, 1)
	if recorder.dir != "" {
		t.Fatalf("invocation work dir = %q, want empty", recorder.dir)
	}
}
