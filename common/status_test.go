package common

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kirsle/configdir"
	"gotest.tools/v3/assert"
)

func Test_RecordSyncConcurrentRepositories(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	configdir.Refresh()
	t.Cleanup(configdir.Refresh)

	const repoCount = 8
	at := time.Unix(1_700_000_000, 0).UTC()
	var wg sync.WaitGroup
	for i := 0; i < repoCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			repo := filepath.Join(configHome, fmt.Sprintf("repo-%d", index))
			var syncErr error
			if index == 3 {
				syncErr = errors.New("test sync failure")
			}
			assert.NilError(t, RecordSync(repo, syncErr, at))
		}(i)
	}
	wg.Wait()

	status, err := ReadStatus()
	assert.NilError(t, err)
	assert.Equal(t, len(status.Repos), repoCount)

	failed := status.Repos[filepath.Join(configHome, "repo-3")]
	assert.Assert(t, !failed.OK)
	assert.Equal(t, failed.Error, "test sync failure")
	assert.Equal(t, failed.SyncedAtUnix, at.Unix())

	succeeded := status.Repos[filepath.Join(configHome, "repo-4")]
	assert.Assert(t, succeeded.OK)
	assert.Equal(t, succeeded.Error, "")
}
