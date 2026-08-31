package common

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kirsle/configdir"
	"gotest.tools/v3/assert"
)

func Test_SyncFailureAlertsOnlyOnFailureTransition(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	configdir.Refresh()
	t.Cleanup(configdir.Refresh)

	originalAlert := sendSyncFailureAlert
	t.Cleanup(func() { sendSyncFailureAlert = originalAlert })

	alerts := 0
	var alertMessage string
	sendSyncFailureAlert = func(title, message, icon string) error {
		alerts++
		assert.Equal(t, title, "Git Auto Sync - Sync Failed")
		assert.Equal(t, icon, "")
		alertMessage = message
		return nil
	}

	repoPath := filepath.Join(configHome, "notes")
	repoConfig := RepoConfig{RepoPath: repoPath}
	failed := errors.New("git fetch failed")
	at := time.Unix(1_700_000_000, 0).UTC()

	err := finishSync(repoConfig, failed, at)
	assert.ErrorContains(t, err, "git fetch failed")
	assert.Equal(t, alerts, 1)
	assert.Equal(t, alertMessage, "Could not sync notes. Check the menu bar for details.")

	err = finishSync(repoConfig, failed, at.Add(time.Minute))
	assert.ErrorContains(t, err, "git fetch failed")
	assert.Equal(t, alerts, 1)

	differentFailure := errors.New("git push failed")
	err = finishSync(repoConfig, differentFailure, at.Add(2*time.Minute))
	assert.ErrorContains(t, err, "git push failed")
	assert.Equal(t, alerts, 2)

	err = finishSync(repoConfig, nil, at.Add(3*time.Minute))
	assert.NilError(t, err)
	assert.Equal(t, alerts, 2)

	err = finishSync(repoConfig, failed, at.Add(4*time.Minute))
	assert.ErrorContains(t, err, "git fetch failed")
	assert.Equal(t, alerts, 3)

	status, err := ReadStatus()
	assert.NilError(t, err)
	assert.Assert(t, !status.Repos[repoPath].OK)
	assert.Equal(t, status.Repos[repoPath].Error, failed.Error())
}

func Test_SafeNotificationText(t *testing.T) {
	assert.Equal(t, safeNotificationText("bad\\\"name\n"), "bad/'name ")
}
