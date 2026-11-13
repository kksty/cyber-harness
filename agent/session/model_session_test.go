package session

import "testing"

func TestModelConversationSurvivesPhysicalSessionRotation(t *testing.T) {
	rt := newBareRuntime(t, nil, &runtimeSemanticProvider{})
	s, err := rt.OpenSession(t.Context(), SessionOptions{ID: "physical-first", LogicalID: "conversation-stable"})
	if err != nil {
		t.Fatal(err)
	}
	before := s.currentState().agent.ConfigSnapshot()
	if before.ModelSessionID != "conversation-stable" {
		t.Fatalf("initial model id = %q", before.ModelSessionID)
	}
	state, err := s.rotate(t.Context(), SessionCloseCompacted, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	after := state.agent.ConfigSnapshot()
	if after.SessionID == before.SessionID {
		t.Fatal("fixture did not rotate physical session")
	}
	if after.ModelSessionID != before.ModelSessionID {
		t.Fatalf("model conversation changed: %q -> %q", before.ModelSessionID, after.ModelSessionID)
	}
	child := after.ForTask("child", "call-1", nil)
	if child.ModelSessionID != "" || child.SessionID != "" {
		t.Fatal("independent child inherited parent's model conversation")
	}
}
