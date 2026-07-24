package common

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_EnsureGitAuthorFromEnvironment(t *testing.T) {
	repoConfig := PrepareFixture(t, "no_changes")
	repoConfig.Env = []string{
		"HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=Applet User",
		"GIT_AUTHOR_EMAIL=applet@example.com",
		"GIT_COMMITTER_NAME=Applet User",
		"GIT_COMMITTER_EMAIL=applet@example.com",
	}

	assert.NilError(t, ensureGitAuthor(repoConfig))
}

func Test_CommitWithEnvironmentAuthor(t *testing.T) {
	repoConfig := PrepareFixture(t, "new_file")
	repoConfig.Env = []string{
		"HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=Applet User",
		"GIT_AUTHOR_EMAIL=applet@example.com",
		"GIT_COMMITTER_NAME=Applet User",
		"GIT_COMMITTER_EMAIL=applet@example.com",
	}

	assert.NilError(t, commit(repoConfig))
}
