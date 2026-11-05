package session

import (
	"context"
	"testing"

	aop "github.com/chainreactors/cyber/aop"
	"github.com/chainreactors/cyber/pkg/testutil/apptest"
	"google.golang.org/protobuf/types/known/anypb"
)

func workDirAny(dir string) *anypb.Any {
	return &anypb.Any{TypeUrl: workDirExtensionURL, Value: []byte(`{"work_dir":"` + dir + `"}`)}
}

func TestSessionWorkDirExtensionParsing(t *testing.T) {
	const dir = "/data/projects/web/cases/in_1"
	req := &aop.OpenSessionRequest{SessionId: "s1", Extensions: []*anypb.Any{
		{TypeUrl: "type.googleapis.com/other.Extension", Value: []byte(`{"work_dir":"/tmp/wrong"}`)},
		workDirAny(dir),
	}}
	if got := sessionWorkDir(req); got != dir {
		t.Fatalf("sessionWorkDir = %q, want %q", got, dir)
	}
	if got := sessionWorkDir(&aop.OpenSessionRequest{}); got != "" {
		t.Fatalf("missing extension = %q, want empty", got)
	}
	if got := sessionWorkDir(&aop.OpenSessionRequest{Extensions: []*anypb.Any{
		{TypeUrl: workDirExtensionURL, Value: []byte(`not-json`)},
	}}); got != "" {
		t.Fatalf("malformed extension = %q, want empty", got)
	}
}

// The session workspace is a property of the session: one agent process serves
// many runs, each with its own case directory (Tianwu fork patch 4). The
// extension is how the control plane sets it without changing the AOP schema.
func TestOpenSessionAppliesWorkDirExtension(t *testing.T) {
	f := apptest.NewFixture(t, nil, nil)
	resource, err := newUnitResource(t, f, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := resource.Start(t.Context(), t.Context()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resource.Close(context.Background()) }()

	rt := resource.Runtime()
	const dir = "/data/projects/web/cases/in_1"
	response := rt.OpenAOPSession(&aop.OpenSessionRequest{SessionId: "s-workdir", Extensions: []*anypb.Any{workDirAny(dir)}})
	if response.GetAccepted() == nil {
		t.Fatalf("open session rejected: %v", response.GetRejected())
	}
	rt.mu.RLock()
	state := rt.sessions["s-workdir"]
	rt.mu.RUnlock()
	if state == nil {
		t.Fatal("session was not registered")
	}
	if got := state.agent.ConfigSnapshot().WorkDir; got != dir {
		t.Fatalf("session work dir = %q, want %q", got, dir)
	}
}
