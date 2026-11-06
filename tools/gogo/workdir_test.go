package gogo_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	coretool "github.com/chainreactors/cyber/core/tool"
	"github.com/chainreactors/cyber/pkg/testutil/hosttest"
	gogotool "github.com/chainreactors/cyber/tools/gogo"
	"github.com/chainreactors/cyber/tools/resources"
	"github.com/chainreactors/cyber/tools/terminal"
	sdkgogo "github.com/chainreactors/sdk/gogo"
)

// Exercise the actual shell -> registry -> native parser -> result file path.
// Only a loopback HTTP fixture is scanned; no model or harness host is started.
func TestGogoReadsAndWritesInTheShellExecutionDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("fixture")) }))
	defer server.Close()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	hostDir, caseDir := t.TempDir(), t.TempDir()
	executionDir := filepath.Join(caseDir, "nested")
	if err := os.MkdirAll(filepath.Join(executionDir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(executionDir, "targets.txt"), []byte(host+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Production installs the scanner-owned resources. noembed builds have no
	// SDK default resource bytes, so the fixture must use that same contract.
	resourceSet, err := resources.Init(context.Background(), resources.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if resourceSet.Neutron != nil {
		defer resourceSet.Neutron.Close()
	}
	engine, err := sdkgogo.NewEngine(sdkgogo.NewConfig().WithResourceProvider(resourceSet.GogoConfig).WithFingersEngine(resourceSet.Fingers).WithNeutronEngine(resourceSet.Neutron))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	command := gogotool.New(engine)
	command.SetWorkDir(hostDir)
	registry := hosttest.Commands(t, coretool.Command{Name: command.Name(), Run: command.Run})
	bash := terminal.NewBashTool(hostDir, 15, nil)
	bash.SetCommandRegistry(registry)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var captured bytes.Buffer
	execution, err := bash.RunForeground(ctx, "cd nested && gogo -l targets.txt -p "+port+" -t 1 -o jl -f raw/results.jsonl -O jl", terminal.BashExecOptions{WorkDir: caseDir, OnOutput: func(data []byte) { captured.Write(data) }})
	if err != nil {
		t.Fatal(err)
	}
	if info, ok := execution.Session(); !ok || info.ExitStatus() != 0 {
		t.Fatalf("shell failed: %+v\n%s", info, captured.String())
	}
	output, err := os.ReadFile(filepath.Join(executionDir, "raw", "results.jsonl"))
	if err != nil || len(output) == 0 {
		t.Fatalf("case result bytes=%d err=%v\n%s", len(output), err, captured.String())
	}
	if _, err := os.Stat(filepath.Join(hostDir, "raw", "results.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("host directory received a task result: %v", err)
	}
	if command.WorkDir != hostDir {
		t.Fatal("shared command configuration was mutated")
	}
}
