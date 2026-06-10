package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// On macOS the launchd config must point logs at ~/Library/Logs (which is
// created if missing) rather than the kardianos default of /usr/local/var/log,
// otherwise launchd silently fails to start the daemon.
// See https://github.com/GitJournal/git-auto-sync/issues/22
func Test_DarwinLaunchdConfigLogPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd config only applies on macOS")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	plist, err := darwinLaunchdConfig("git-auto-sync-daemon")
	assert.NilError(t, err)

	logDir := filepath.Join(home, "Library", "Logs")
	info, err := os.Stat(logDir)
	assert.NilError(t, err)
	assert.Assert(t, info.IsDir(), "expected %s to be created", logDir)

	assert.Assert(t, strings.Contains(plist, filepath.Join(logDir, "git-auto-sync-daemon.out.log")))
	assert.Assert(t, strings.Contains(plist, filepath.Join(logDir, "git-auto-sync-daemon.err.log")))
	assert.Assert(t, !strings.Contains(plist, "/usr/local/var/log"))
}
