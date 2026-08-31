package common

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func Test_WatcherRetriesAfterInitialSyncFailure(t *testing.T) {
	repoConfig := PrepareFixture(t, "no_changes")
	repoConfig.FSLag = 20 * time.Millisecond
	repoConfig.PollInterval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := make(chan int32, 10)
	var attemptCount int32
	deps := watchDependencies{
		sync: func(RepoConfig) error {
			attempt := atomic.AddInt32(&attemptCount, 1)
			attempts <- attempt
			if attempt == 1 {
				return errors.New("initial sync failed")
			}
			return nil
		},
		startAwakeNotifier: func(chan bool) error { return nil },
	}

	done := make(chan error, 1)
	go func() {
		done <- watchForChanges(ctx, repoConfig, deps)
	}()

	select {
	case attempt := <-attempts:
		assert.Equal(t, attempt, int32(1))
	case err := <-done:
		t.Fatalf("watcher stopped before initial sync: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("initial sync did not run")
	}

	changedFile := filepath.Join(repoConfig.RepoPath, "1.md")
	assert.NilError(t, os.WriteFile(changedFile, []byte("changed\n"), 0644))

	select {
	case attempt := <-attempts:
		assert.Assert(t, attempt >= 2)
	case <-time.After(5 * time.Second):
		t.Fatal("filesystem change did not trigger a retry after the initial failure")
	}

	cancel()
	select {
	case err := <-done:
		assert.NilError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop after cancellation")
	}
}
