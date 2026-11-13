package scan

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/chainreactors/cyber/tools/resources"
	"github.com/chainreactors/cyber/tools/scan/engine"
)

var testEngines *engine.Set

func TestMain(m *testing.M) {
	// The host installs Cyber's resources before using SDK engines. In
	// emptytemplates builds the dependencies have no embedded fallback.
	var err error
	testEngines, err = engine.InitWithOptions(context.Background(), resources.Options{}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize scan test resources:", err)
		os.Exit(1)
	}
	if testEngines.Gogo == nil || testEngines.Spray == nil {
		testEngines.Close()
		fmt.Fprintln(os.Stderr, "scan test resources did not initialize gogo and spray")
		os.Exit(1)
	}
	code := m.Run()
	testEngines.Close()
	os.Exit(code)
}
