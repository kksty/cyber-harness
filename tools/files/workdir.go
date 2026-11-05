package files

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chainreactors/cyber/core/operation"
)

// sessionPath resolves a tool-supplied path against the session's workspace.
//
// The files extension is rooted at the agent's configured directory, but a
// session may set a narrower workspace inside it (the Tianwu fork's
// OpenSessionRequest work-dir extension, patch 4). Relative paths must resolve
// there, exactly like bash, or a write tool would still drop files at the agent
// root. The result stays root-relative so os.Root remains the boundary; a
// workspace outside the root is refused rather than silently widened.
func (f *Files) sessionPath(ctx context.Context, rel string) (string, error) {
	work := strings.TrimSpace(operation.WorkDirFromContext(ctx, ""))
	if work == "" {
		return rel, nil
	}
	rootDir, err := filepath.Abs(f.config.Directory)
	if err != nil {
		return "", err
	}
	workDir, err := filepath.Abs(work)
	if err != nil {
		return "", err
	}
	sub, err := filepath.Rel(rootDir, workDir)
	if err != nil {
		return "", err
	}
	if sub == ".." || strings.HasPrefix(sub, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("session working directory %s is outside the tool root %s", workDir, rootDir)
	}
	if sub == "." {
		return rel, nil
	}
	joined := filepath.ToSlash(filepath.Join(sub, rel))
	if !filepath.IsLocal(filepath.FromSlash(joined)) {
		return "", fmt.Errorf("path is not local")
	}
	return joined, nil
}
