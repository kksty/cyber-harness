package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/chainreactors/cyber/agent/inbox"
	coretool "github.com/chainreactors/cyber/core/tool"
)

func TestForegroundStdinIsEOF(t *testing.T) {
	for _, agentCall := range []bool{false, true} {
		t.Run(fmt.Sprintf("agent=%t", agentCall), func(t *testing.T) {
			bash := NewBashTool(t.TempDir(), 1, nil)
			defer bash.Close()
			ib := inbox.NewBuffered(4)
			defer ib.Close()
			ctx, cancel := context.WithTimeout(inbox.ContextWithInbox(t.Context(), ib), 3*time.Second)
			defer cancel()
			command := `if read line; then printf 'unexpected:%s' "$line"; else printf eof; fi`
			if agentCall {
				args, _ := json.Marshal(BashArgs{Command: command})
				result, err := bash.Execute(ctx, string(args))
				if err != nil || result == nil || result.IsError || coretool.ResultText(result) != "eof" {
					t.Fatalf("foreground result=%v err=%v", result, err)
				}
			} else {
				result, err := bash.RunForegroundTool(ctx, command, BashExecOptions{})
				if err != nil || result == nil || result.IsError || coretool.ResultText(result) != "eof" {
					t.Fatalf("foreground result=%v err=%v", result, err)
				}
			}
		})
	}
}

func TestExplicitStdinOverridesKeptInput(t *testing.T) {
	bash := NewBashTool(t.TempDir(), 1, nil)
	defer bash.Close()
	result, err := bash.RunForegroundTool(t.Context(), `read line; printf 'got:%s' "$line"`, BashExecOptions{
		Stdin: strings.NewReader("provided\n"), KeepStdin: true,
	})
	if err != nil || result == nil || result.IsError || coretool.ResultText(result) != "got:provided" {
		t.Fatalf("explicit stdin result=%v err=%v", result, err)
	}
}

func TestWaitKeepsStdinOpenForInput(t *testing.T) {
	bash := NewBashTool(t.TempDir(), 5, nil)
	defer bash.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	args, _ := json.Marshal(BashArgs{Command: `read line; printf 'got:%s' "$line"`, Wait: 1})
	result, err := bash.Execute(ctx, string(args))
	if err != nil || result == nil || result.IsError || !strings.Contains(coretool.ResultText(result), "after waiting 1s") {
		t.Fatalf("command did not move to background: result=%v err=%v", result, err)
	}
	cancel()
	sessions := bash.Manager().List()
	if len(sessions) != 1 {
		t.Fatalf("sessions=%d, want one", len(sessions))
	}
	id := sessions[0].ID
	tmux := NewTmuxCommand(bash)
	if _, err := tmux.Run(t.Context(), &coretool.Execution{Args: []string{"send-keys", "-t", id, "hello", "Enter"}, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bash.Manager().Done(id):
	case <-time.After(3 * time.Second):
		t.Fatal("background command did not finish after input")
	}
	if got := bash.Manager().PeekOrEmpty(id, 20); got != "got:hello" {
		t.Fatalf("output=%q, want got:hello", got)
	}
}

func TestKeptStdinStopsOnCancellation(t *testing.T) {
	bash := NewBashTool(t.TempDir(), 5, nil)
	defer bash.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	execution, err := bash.Start(ctx, "read line", BashExecOptions{KeepStdin: true})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-bash.Manager().Done(execution.ID):
	case <-time.After(3 * time.Second):
		t.Fatal("stdin read did not stop on cancellation")
	}
}

func TestInboxDetachKeepsStdinPolicy(t *testing.T) {
	for _, wait := range []int{0, 10} {
		t.Run(fmt.Sprintf("wait=%d", wait), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			registry, _ := loadTestRegistry(t, commandBatch(coretool.Command{Name: "hold_input", Run: func(ctx context.Context, _ *coretool.Execution) (any, error) {
				close(started)
				select {
				case <-release:
					return nil, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}}))
			bash := NewBashTool(t.TempDir(), 5, nil)
			defer bash.Close()
			bash.SetCommandRegistry(registry)
			ib := inbox.NewBuffered(4)
			defer ib.Close()
			ctx, cancel := context.WithCancel(inbox.ContextWithInbox(t.Context(), ib))
			defer cancel()
			args, _ := json.Marshal(BashArgs{Command: `hold_input; if read line; then printf 'got:%s' "$line"; else printf eof; fi`, Wait: wait})
			returned := make(chan string, 1)
			go func() {
				result, err := bash.Execute(ctx, string(args))
				if err != nil {
					returned <- err.Error()
					return
				}
				returned <- coretool.ResultText(result)
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("command did not start")
			}
			sessions := bash.Manager().List()
			if len(sessions) != 1 {
				t.Fatalf("sessions=%d, want one", len(sessions))
			}
			id := sessions[0].ID
			msg := inbox.NewUserMessage("handle new input")
			msg.Interrupt = true
			if err := ib.Push(msg); err != nil {
				t.Fatal(err)
			}
			select {
			case result := <-returned:
				if !strings.Contains(result, "because an interrupting Inbox message arrived") || !strings.Contains(result, id) {
					t.Fatalf("Inbox did not detach the same session: %s", result)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("foreground wait was not interrupted")
			}
			cancel() // Detachment must survive cancellation of the former tool call.
			want := "eof"
			if wait == 0 {
				if err := bash.Manager().Write(id, []byte("unexpected\n")); err == nil || !strings.Contains(err.Error(), "does not accept input") {
					t.Fatalf("EOF session accepted input: %v", err)
				}
			}
			close(release)
			if wait > 0 {
				want = "got:hello"
				tmux := NewTmuxCommand(bash)
				if _, err := tmux.Run(t.Context(), &coretool.Execution{Args: []string{"send-keys", "-t", id, "hello", "Enter"}, Stdout: io.Discard}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-bash.Manager().Done(id):
			case <-time.After(3 * time.Second):
				t.Fatal("detached command did not finish")
			}
			if got := bash.Manager().PeekOrEmpty(id, 20); got != want {
				t.Fatalf("output=%q, want %q", got, want)
			}
		})
	}
}

func TestTmuxScriptKeepsStdinOpen(t *testing.T) {
	bash := NewBashTool(t.TempDir(), 5, nil)
	defer bash.Close()
	tmux := NewTmuxCommand(bash)
	if _, err := tmux.Run(t.Context(), &coretool.Execution{Args: []string{
		"new-session", "-d", "-s", "input-script", `read line; printf 'got:%s' "$line"`,
	}, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if _, err := tmux.Run(t.Context(), &coretool.Execution{Args: []string{"send-keys", "-t", "input-script", "hello", "Enter"}, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bash.Manager().Done("input-script"):
	case <-time.After(3 * time.Second):
		t.Fatal("interactive script did not finish after input")
	}
	if got := bash.Manager().PeekOrEmpty("input-script", 20); got != "got:hello" {
		t.Fatalf("output=%q, want got:hello", got)
	}
}
