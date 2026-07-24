package common

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/ztrue/tracerr"
)

var errNoGitAuthorEmail = errors.New("Missing git author email")
var errNoGitAuthorName = errors.New("Missing git author name")

func ensureGitAuthor(repoConfig RepoConfig) error {
	_, err := GitCommand(repoConfig, []string{"config", "user.email"})
	if err != nil {
		var exerr *exec.ExitError
		if errors.As(err, &exerr) && exerr.ExitCode() == 1 {
			if hasNonEmptyEnvVariable(repoConfig.Env, "GIT_AUTHOR_EMAIL") {
				goto checkName
			}
			return errNoGitAuthorEmail
		}
		return tracerr.Wrap(err)
	}

checkName:
	_, err = GitCommand(repoConfig, []string{"config", "user.name"})
	if err != nil {
		var exerr *exec.ExitError
		if errors.As(err, &exerr) && exerr.ExitCode() == 1 {
			if hasNonEmptyEnvVariable(repoConfig.Env, "GIT_AUTHOR_NAME") {
				return nil
			}
			return errNoGitAuthorName
		}
		return tracerr.Wrap(err)
	}

	return nil
}

func hasNonEmptyEnvVariable(all []string, name string) bool {
	for _, value := range all {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) == 2 && parts[0] == name && parts[1] != "" {
			return true
		}
	}
	return false
}
