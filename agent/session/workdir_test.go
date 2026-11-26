package session

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/chainreactors/cyber/agent"
	"github.com/chainreactors/cyber/aop"
)

// A resumed turn never re-opens its session: after a host restart the session is
// re-created and has no workspace, so the turn itself must be able to supply it.
func TestTurnWorkDirDecodesTheSakerExtension(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"work_dir": `C:\data\projects\p\cases\in_1`})
	if err != nil {
		t.Fatal(err)
	}
	req := &aop.RunTurnRequest{Extensions: []*anypb.Any{{TypeUrl: workDirExtensionURL, Value: raw}}}
	if got := turnWorkDir(req); got != `C:\data\projects\p\cases\in_1` {
		t.Fatalf("turnWorkDir = %q", got)
	}
	if turnWorkDir(&aop.RunTurnRequest{}) != "" {
		t.Fatal("a request without the extension must not name a workspace")
	}
}

func TestTurnWorkDirFillsAnEmptyWorkspaceOnly(t *testing.T) {
	ag := agent.NewAgent(agent.Config{})
	if err := applyTurnWorkDir(ag, `C:\data\project\cases\in_1`); err != nil {
		t.Fatal(err)
	}
	if got := ag.WorkDir(); got != `C:\data\project\cases\in_1` {
		t.Fatalf("workspace = %q, want the one the turn supplied", got)
	}

	// The same directory written differently is not a conflict.
	if err := applyTurnWorkDir(ag, `C:/data/project/cases/in_1/`); err != nil {
		t.Fatalf("same workspace reported as a conflict: %v", err)
	}

	// A different directory would move the session's relative paths mid-run.
	if err := applyTurnWorkDir(ag, `C:\data\project\cases\in_2`); err == nil {
		t.Fatal("a turn silently moved the session to another workspace")
	}
	if got := ag.WorkDir(); got != `C:\data\project\cases\in_1` {
		t.Fatalf("workspace changed despite the refusal: %q", got)
	}

	// A turn without a workspace leaves an existing one alone.
	if err := applyTurnWorkDir(ag, ""); err != nil {
		t.Fatal(err)
	}
	if got := ag.WorkDir(); got != `C:\data\project\cases\in_1` {
		t.Fatalf("workspace = %q after an empty turn", got)
	}
}
