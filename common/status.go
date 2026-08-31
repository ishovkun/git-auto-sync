package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kirsle/configdir"
	"github.com/ztrue/tracerr"
)

// RepoStatus is the outcome of the most recent sync attempt for one repo.
type RepoStatus struct {
	Repo  string `json:"repo"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// SyncedAt is the human-readable timestamp; SyncedAtUnix is the same moment
	// as an epoch second so shell consumers can compute "N minutes ago" without
	// parsing RFC3339.
	SyncedAt     time.Time `json:"synced_at"`
	SyncedAtUnix int64     `json:"synced_at_unix"`
}

// Status is the on-disk snapshot the daemon writes after every sync attempt so
// external tools (e.g. the SwiftBar menubar plugin) can show live state without
// reaching into each git repo themselves.
type Status struct {
	Repos map[string]RepoStatus `json:"repos"`
}

// statusMu guards read-modify-write of the shared status file, since each
// watched repo runs its own goroutine and they all update the same file.
var statusMu sync.Mutex

func statusFilePath() string {
	return filepath.Join(configdir.LocalConfig("git-auto-sync"), "status.json")
}

// ReadStatus loads the current status snapshot, returning an empty (non-nil)
// status if the file doesn't exist yet.
func ReadStatus() (Status, error) {
	status := Status{Repos: map[string]RepoStatus{}}

	data, err := os.ReadFile(statusFilePath())
	if os.IsNotExist(err) {
		return status, nil
	}
	if err != nil {
		return status, tracerr.Wrap(err)
	}

	if err := json.Unmarshal(data, &status); err != nil {
		return status, tracerr.Wrap(err)
	}
	if status.Repos == nil {
		status.Repos = map[string]RepoStatus{}
	}
	return status, nil
}

// RecordSync updates the status file with the result of a sync attempt for the
// given repo. A nil syncErr records success; otherwise the error message is
// stored. Status reporting is best-effort and never affects sync behaviour.
func RecordSync(repoPath string, syncErr error, at time.Time) error {
	_, _, err := recordSync(repoPath, syncErr, at)
	return err
}

// recordSync also returns the previous entry so callers can react to status
// transitions without racing another repository's status update.
func recordSync(repoPath string, syncErr error, at time.Time) (RepoStatus, bool, error) {
	statusMu.Lock()
	defer statusMu.Unlock()

	configPath := configdir.LocalConfig("git-auto-sync")
	if err := configdir.MakePath(configPath); err != nil {
		return RepoStatus{}, false, tracerr.Wrap(err)
	}

	status, err := ReadStatus()
	if err != nil {
		return RepoStatus{}, false, err
	}
	previous, hadPrevious := status.Repos[repoPath]

	entry := RepoStatus{Repo: repoPath, OK: syncErr == nil, SyncedAt: at, SyncedAtUnix: at.Unix()}
	if syncErr != nil {
		entry.Error = syncErr.Error()
	}
	status.Repos[repoPath] = entry

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return previous, hadPrevious, tracerr.Wrap(err)
	}

	// Readers live in other processes (SwiftBar and Plasma), so replace the
	// snapshot atomically rather than exposing a partially written JSON file.
	tmp, err := os.CreateTemp(configPath, ".status-*.tmp")
	if err != nil {
		return previous, hadPrevious, tracerr.Wrap(err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return previous, hadPrevious, tracerr.Wrap(err)
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return previous, hadPrevious, tracerr.Wrap(err)
	}
	if err := tmp.Close(); err != nil {
		return previous, hadPrevious, tracerr.Wrap(err)
	}
	if err := os.Rename(tmpPath, statusFilePath()); err != nil {
		return previous, hadPrevious, tracerr.Wrap(err)
	}
	return previous, hadPrevious, nil
}
