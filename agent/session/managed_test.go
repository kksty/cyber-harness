package session

import (
	"context"
	"testing"
	"time"

	"github.com/chainreactors/cyber/agent"
	"github.com/chainreactors/cyber/agent/inbox"
	aop "github.com/chainreactors/cyber/aop"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestAOPManagedMailboxWaitsForExplicitTurnAndPreservesPolicyOnRotation(t *testing.T) {
	provider := &runtimeSemanticProvider{}
	rt := newBareRuntime(t, nil, provider)
	response := rt.OpenAOPSession(&aop.OpenSessionRequest{SessionId: "managed", Extensions: []*anypb.Any{{TypeUrl: workDirExtensionURL, Value: []byte(`{"single_task":true}`)}}})
	if response.GetAccepted() == nil {
		t.Fatalf("open: %v", response.GetRejected())
	}
	s, err := rt.EnsureSession(SessionOptions{ID: "managed"})
	if err != nil {
		t.Fatal(err)
	}
	assertIdle := func() {
		t.Helper()
		before := provider.callCount()
		if err := s.Deliver(t.Context(), inbox.NewSystemMessage("peer update")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
		if provider.callCount() != before {
			t.Fatal("idle mailbox created an unowned model turn")
		}
		if s.state.inbox.Len() == 0 {
			t.Fatal("idle message was dropped")
		}
	}
	assertIdle()
	run, err := s.Run(t.Context(), RunInput{Message: agent.TextInput("tracked turn")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Wait(); err != nil {
		t.Fatal(err)
	}
	if provider.callCount() != 1 {
		t.Fatalf("explicit turn calls = %d", provider.callCount())
	}
	assertIdle()
	if _, err := s.rotate(context.Background(), SessionCloseCleared, s.ID(), nil); err != nil {
		t.Fatal(err)
	}
	assertIdle()
	run, err = s.Run(t.Context(), RunInput{Message: agent.TextInput("next tracked turn")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Wait(); err != nil {
		t.Fatal(err)
	}
	if provider.callCount() != 2 {
		t.Fatal("managed session could not continue explicitly")
	}
}

func TestManagedActiveSessionStillAdmitsPeerMail(t *testing.T) {
	rt := newBareRuntime(t, nil, &runtimeSemanticProvider{})
	entered, release := make(chan struct{}), make(chan struct{})
	cfg := rt.agentConfig.WithLoop(taskLoop(func(ctx context.Context, cfg agent.Config) (*agent.Result, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return &agent.Result{}, nil
	}))
	s, err := rt.OpenSession(t.Context(), SessionOptions{ID: "active", Config: &cfg, SingleTask: true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.Run(t.Context(), RunInput{Message: agent.TextInput("work")})
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := s.Deliver(t.Context(), inbox.NewSystemMessage("new peer observation")); err != nil {
		t.Fatal(err)
	}
	if s.state.inbox.Len() == 0 {
		t.Fatal("active peer mail was not admitted")
	}
	close(release)
	if _, err := run.Wait(); err != nil {
		t.Fatal(err)
	}
}
