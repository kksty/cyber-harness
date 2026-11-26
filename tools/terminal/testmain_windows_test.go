//go:build windows

package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	ensurePOSIXOnPath()
	os.Exit(m.Run())
}

// ensurePOSIXOnPath mirrors what the harness host does for an agent shell: the
// shell is an embedded POSIX interpreter, so external commands (cat, wc, sleep,
// …) resolve from PATH, and Git for Windows ships them under usr\bin. Saker's
// host prepends that directory before starting the shell; this test binary runs
// the same interpreter directly, so without the same fix every test that pipes
// through cat/wc or blocks on sleep fails with "executable file not found in
// %PATH%" on a machine whose PATH lacks coreutils. A machine without Git (or
// MSYS2) keeps those tests skipped rather than failed.
func ensurePOSIXOnPath() {
	if lookPOSIX("cat", "wc", "sleep") {
		return
	}
	for _, dir := range posixCandidateDirs() {
		if dirHasTools(dir, "cat.exe", "wc.exe", "sleep.exe") {
			os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			return
		}
	}
}

func lookPOSIX(names ...string) bool {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return false
		}
	}
	return true
}

func dirHasTools(dir string, names ...string) bool {
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// posixCandidateDirs lists the common Git for Windows/MSYS2 install locations.
// The git-on-PATH sibling comes first because it follows the operator's own
// install (portable Git, scoop, chocolatey) instead of assuming Program Files.
func posixCandidateDirs() []string {
	var dirs []string
	if git, err := exec.LookPath("git"); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(git), "..", "usr", "bin"))
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if root != "" {
			dirs = append(dirs, filepath.Join(root, "Git", "usr", "bin"))
		}
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "Programs", "Git", "usr", "bin"))
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		dirs = append(dirs, filepath.Join(home, "scoop", "apps", "git", "current", "usr", "bin"))
	}
	dirs = append(dirs, `C:\msys64\usr\bin`)
	return dirs
}
