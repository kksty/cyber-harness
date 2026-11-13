package engine

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/chainreactors/cyber/tools/resources"
)

func TestMain(m *testing.M) {
	// SDK adapter tests run after the same resource installation as a host,
	// including when dependency templates are disabled by the build tags.
	engines, err := InitWithOptions(context.Background(), resources.Options{}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize adapter test resources:", err)
		os.Exit(1)
	}
	if engines.Gogo == nil || engines.Spray == nil || engines.Zombie == nil || engines.Neutron == nil {
		engines.Close()
		fmt.Fprintln(os.Stderr, "adapter test resources did not initialize every engine")
		os.Exit(1)
	}
	code := m.Run()
	engines.Close()
	os.Exit(code)
}
