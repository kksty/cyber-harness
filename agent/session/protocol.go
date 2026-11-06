package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	aop "github.com/chainreactors/cyber/aop"
	types "github.com/chainreactors/cyber/core/types"
	protobuf "google.golang.org/protobuf/proto"
)

// workDirExtensionURL is the Tianwu fork's per-session workspace extension on
// OpenSessionRequest. The AOP schema has no session work dir (a session runs in
// the agent process directory), so a control plane that owns a per-run case
// directory sends it here instead of editing the shared schema. The value is
// JSON: {"work_dir":"<absolute path>","single_task":true}. The optional
// scheduling flag uses the runtime's existing SingleTask mailbox policy.
const workDirExtensionURL = "type.googleapis.com/tianwu.WorkDir"

type workDirExtension struct {
	WorkDir    string `json:"work_dir"`
	SingleTask bool   `json:"single_task,omitempty"`
}

// sessionWorkDir returns the workspace a control plane attached to this open
// request. Empty means "use the agent's own working directory", which keeps old
// control planes working.
func sessionWorkspace(req *aop.OpenSessionRequest) workDirExtension {
	if req == nil {
		return workDirExtension{}
	}
	for _, extension := range req.GetExtensions() {
		if extension == nil || extension.GetTypeUrl() != workDirExtensionURL {
			continue
		}
		var value workDirExtension
		if err := json.Unmarshal(extension.GetValue(), &value); err != nil {
			continue
		}
		value.WorkDir = strings.TrimSpace(value.WorkDir)
		if value.WorkDir != "" || value.SingleTask {
			return value
		}
	}
	return workDirExtension{}
}

func (rt *Runtime) OpenAOPSession(req *aop.OpenSessionRequest) *aop.OpenSessionResponse {
	response := &aop.OpenSessionResponse{}
	if rt == nil || req == nil || strings.TrimSpace(req.SessionId) == "" {
		response.Outcome = &aop.OpenSessionResponse_Rejected{Rejected: rejection("INVALID_ARGUMENT", "session_id is required")}
		return response
	}
	workspace := sessionWorkspace(req)
	session, err := rt.EnsureSession(SessionOptions{
		ID: req.SessionId, ParentSessionID: req.ParentSessionId, ParentToolCallID: req.ParentToolCallId,
		WorkDir: workspace.WorkDir, SingleTask: workspace.SingleTask,
	})
	if err != nil {
		response.Outcome = &aop.OpenSessionResponse_Rejected{Rejected: rejection("FAILED_PRECONDITION", err.Error())}
		return response
	}
	response.Outcome = &aop.OpenSessionResponse_Accepted{Accepted: &aop.Session{Id: session.ID(), State: "open", NodeId: req.NodeId, Title: req.Title}}
	return response
}

func (rt *Runtime) RunAOPTurn(ctx context.Context, req *aop.RunTurnRequest) *aop.RunTurnResponse {
	response := &aop.RunTurnResponse{}
	if rt == nil || req == nil || (!req.ContinueSession && req.Input == nil) || strings.TrimSpace(req.SessionId) == "" || strings.TrimSpace(req.TurnId) == "" {
		response.Outcome = &aop.RunTurnResponse_Rejected{Rejected: rejection("INVALID_ARGUMENT", "session_id, turn_id, and input are required unless continue_session is true")}
		return response
	}
	options := new(types.AgentRunOptions)
	for _, extension := range req.Extensions {
		if extension != nil && extension.MessageIs(options) {
			if err := extension.UnmarshalTo(options); err != nil {
				response.Outcome = &aop.RunTurnResponse_Rejected{Rejected: rejection("INVALID_ARGUMENT", "invalid Cyber run options: "+err.Error())}
				return response
			}
			break
		}
	}
	var message *aop.Message
	if req.Input != nil {
		message = protobuf.CloneOf(req.Input)
	}
	_, err := rt.RunSession(ctx, req.SessionId, RunInput{
		TurnID: req.TurnId, Message: message, Continue: req.ContinueSession,
		MaxTurns: int(req.MaxTurns), EvalCriteria: options.EvalCriteria, EvalRounds: options.EvalRounds,
	})
	if err != nil {
		response.Outcome = &aop.RunTurnResponse_Rejected{Rejected: rejection("FAILED_PRECONDITION", err.Error())}
		return response
	}
	response.Outcome = &aop.RunTurnResponse_Accepted{Accepted: &aop.TurnReceipt{SessionId: req.SessionId, TurnId: req.TurnId, State: "running"}}
	return response
}

func (rt *Runtime) CancelAOPTurn(req *aop.CancelTurnRequest) *aop.CancelTurnResponse {
	response := &aop.CancelTurnResponse{}
	if rt == nil || req == nil || strings.TrimSpace(req.SessionId) == "" || strings.TrimSpace(req.TurnId) == "" {
		response.Outcome = &aop.CancelTurnResponse_Rejected{Rejected: rejection("INVALID_ARGUMENT", "session_id and turn_id are required")}
		return response
	}
	if err := rt.CancelSessionRun(req.SessionId, req.TurnId); err != nil {
		response.Outcome = &aop.CancelTurnResponse_Rejected{Rejected: rejection("NOT_FOUND", err.Error())}
		return response
	}
	response.Outcome = &aop.CancelTurnResponse_Accepted{Accepted: &aop.TurnReceipt{SessionId: req.SessionId, TurnId: req.TurnId, State: "canceled"}}
	return response
}

func (rt *Runtime) CloseAOPSession(ctx context.Context, req *aop.CloseSessionRequest) *aop.CloseSessionResponse {
	response := &aop.CloseSessionResponse{}
	if rt == nil || req == nil || strings.TrimSpace(req.SessionId) == "" {
		response.Outcome = &aop.CloseSessionResponse_Rejected{Rejected: rejection("INVALID_ARGUMENT", "session_id is required")}
		return response
	}
	if err := rt.CloseSession(ctx, req.SessionId, SessionCloseReason(req.Reason)); err != nil {
		response.Outcome = &aop.CloseSessionResponse_Rejected{Rejected: rejection("FAILED_PRECONDITION", err.Error())}
		return response
	}
	response.Outcome = &aop.CloseSessionResponse_Accepted{Accepted: &aop.Session{Id: req.SessionId, State: "closed"}}
	return response
}

// NamespaceBindings publishes the protocols implemented by this runtime. The
// profile's typed namespace registry installs them on each connection.
func (rt *Runtime) NamespaceBindings() []aop.Binding {
	if rt == nil {
		return nil
	}
	return []aop.Binding{
		aop.Shared(&aop.ProtocolMessage{}, rt.HandleCoreNamespace),
		aop.Shared(&types.CommandProtocolMessage{}, rt.HandleCommandNamespace),
	}
}

// HandleCoreNamespace implements session control for all existing transports.
func (rt *Runtime) HandleCoreNamespace(ctx context.Context, envelope *aop.Envelope, message protobuf.Message, send aop.SendFunc) error {
	value, ok := message.(*aop.ProtocolMessage)
	if !ok {
		return fmt.Errorf("unexpected core namespace message %T", message)
	}
	reply := func(message protobuf.Message) error { return send(aop.Reply(envelope.Id, message)) }
	switch payload := value.Message.(type) {
	case *aop.ProtocolMessage_OpenSessionRequest:
		return reply(&aop.ProtocolMessage{Message: &aop.ProtocolMessage_OpenSessionResponse{OpenSessionResponse: rt.OpenAOPSession(payload.OpenSessionRequest)}})
	case *aop.ProtocolMessage_RunTurnRequest:
		return reply(&aop.ProtocolMessage{Message: &aop.ProtocolMessage_RunTurnResponse{RunTurnResponse: rt.RunAOPTurn(ctx, payload.RunTurnRequest)}})
	case *aop.ProtocolMessage_CancelTurnRequest:
		return reply(&aop.ProtocolMessage{Message: &aop.ProtocolMessage_CancelTurnResponse{CancelTurnResponse: rt.CancelAOPTurn(payload.CancelTurnRequest)}})
	case *aop.ProtocolMessage_CloseSessionRequest:
		return reply(&aop.ProtocolMessage{Message: &aop.ProtocolMessage_CloseSessionResponse{CloseSessionResponse: rt.CloseAOPSession(ctx, payload.CloseSessionRequest)}})
	default:
		return fmt.Errorf("unsupported AOP core message")
	}
}

// HandleCommandNamespace implements the existing command namespace.
func (rt *Runtime) HandleCommandNamespace(ctx context.Context, envelope *aop.Envelope, message protobuf.Message, send aop.SendFunc) error {
	value, ok := message.(*types.CommandProtocolMessage)
	if !ok {
		return fmt.Errorf("unexpected command namespace message %T", message)
	}
	reply := func(message protobuf.Message) error { return send(aop.Reply(envelope.Id, message)) }
	request := value.GetRequest()
	if request == nil || strings.TrimSpace(request.Line) == "" {
		return reply(aop.NewProtocolError("INVALID_ARGUMENT", "command line is required"))
	}
	if err := rt.ready(); err != nil {
		return reply(aop.NewProtocolError("COMMAND_FAILED", err.Error()))
	}
	// Close cancels the context before taking mu and waiting for operations.
	// Admission must use the same gate so Add cannot race with an empty Wait.
	rt.mu.Lock()
	if err := rt.ctx.Err(); err != nil {
		rt.mu.Unlock()
		return reply(aop.NewProtocolError("COMMAND_FAILED", err.Error()))
	}
	rt.operations.Add(1)
	rt.mu.Unlock()
	go func() {
		defer rt.operations.Done()
		result, err := rt.CommandSession(ctx, request.SessionId, request.Line)
		if err != nil {
			_ = reply(aop.NewProtocolError("COMMAND_FAILED", err.Error()))
			return
		}
		_ = reply(&types.CommandProtocolMessage{Message: &types.CommandProtocolMessage_Result{Result: result}})
	}()
	return nil
}

func rejection(code, message string) *aop.Rejection {
	return &aop.Rejection{Code: code, Message: message}
}
