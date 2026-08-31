package common

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/ztrue/tracerr"
)

var sendSyncFailureAlert = beeep.Alert

// AutoSync runs a full sync and records the outcome to the status file so the
// menubar plugin (and any other observer) can see the latest result.
func AutoSync(repoConfig RepoConfig) error {
	return finishSync(repoConfig, autoSync(repoConfig), time.Now())
}

func finishSync(repoConfig RepoConfig, syncErr error, at time.Time) error {
	previous, hadPrevious, recordErr := recordSync(repoConfig.RepoPath, syncErr, at)
	if recordErr != nil {
		log.Printf("Could not record sync status for %s: %v", repoConfig.RepoPath, recordErr)
	}

	if syncErr != nil && shouldAlertForFailure(previous, hadPrevious, syncErr) {
		repoName := safeNotificationText(filepath.Base(repoConfig.RepoPath))
		message := fmt.Sprintf("Could not sync %s. Check the menu bar for details.", repoName)
		if alertErr := sendSyncFailureAlert("Git Auto Sync - Sync Failed", message, ""); alertErr != nil {
			log.Printf("Could not show sync failure notification for %s: %v", repoConfig.RepoPath, alertErr)
		}
	}
	return syncErr
}

func shouldAlertForFailure(previous RepoStatus, hadPrevious bool, syncErr error) bool {
	return !hadPrevious || previous.OK || previous.Error != syncErr.Error()
}

// beeep's macOS backend embeds the message in AppleScript source, so keep the
// repository name on one line and remove characters that could terminate the
// quoted string.
func safeNotificationText(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.ReplaceAll(value, "\"", "'")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

// FIXME: Add logs for when we commit, pull, and push
func autoSync(repoConfig RepoConfig) error {
	var err error
	err = ensureGitAuthor(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = commit(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = fetch(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = rebase(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = push(repoConfig)
	if err != nil {
		return tracerr.Wrap(err)
	}

	// -> do a merge
	// -> push the changes

	return nil
}
