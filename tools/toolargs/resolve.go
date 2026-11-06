package toolargs

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/chainreactors/cyber/core/operation"
	coretool "github.com/chainreactors/cyber/core/tool"
)

// ResolvePaths binds native file arguments to this command invocation. The
// shared command's WorkDir is only a default: mutating it per session would
// send concurrent callers' files to one another's directories.
func (b *Base) ResolvePaths(ctx context.Context, execution *coretool.Execution, fileFlags map[string]bool) []string {
	workDir := execution.Dir
	if workDir == "" {
		workDir = operation.WorkDirFromContext(ctx, b.WorkDir)
	}
	return ResolveRelativePaths(execution.Args, fileFlags, workDir)
}

// ResolveRelativePaths resolves relative file paths in args against workDir.
// fileFlags is the set of flags whose next argument (or =value) is a file path.
func ResolveRelativePaths(args []string, fileFlags map[string]bool, workDir string) []string {
	if workDir == "" {
		return args
	}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if key, value, ok := strings.Cut(arg, "="); ok {
			if fileFlags[key] {
				out = append(out, key+"="+resolvePath(value, workDir))
				continue
			}
			out = append(out, arg)
			continue
		}
		if fileFlags[arg] && i+1 < len(args) {
			out = append(out, arg)
			i++
			out = append(out, resolvePath(args[i], workDir))
			continue
		}
		out = append(out, arg)
	}
	return out
}

// resolvePath resolves a single path relative to workDir.
// Returns value unchanged if empty, absolute, or starts with "-".
func resolvePath(value, workDir string) string {
	if value == "" || filepath.IsAbs(value) || strings.HasPrefix(value, "-") {
		return value
	}
	return filepath.Join(workDir, value)
}
