package common

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/ztrue/tracerr"
	git "gopkg.in/src-d/go-git.v4"
)

func commit(repoConfig RepoConfig) error {
	repoPath := repoConfig.RepoPath
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return tracerr.Wrap(err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return tracerr.Wrap(err)
	}

	status, err := w.Status()
	if err != nil {
		return tracerr.Wrap(err)
	}

	hasChanges := false
	commitMsg := []string{}
	for filePath, fileStatus := range status {
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		ignore, err := ShouldIgnoreFile(repoPath, filePath)
		if err != nil {
			return tracerr.Wrap(err)
		}

		if ignore {
			continue
		}

		hasChanges = true
		_, err = w.Add(filePath)
		if err != nil {
			return tracerr.Wrap(err)
		}

		msg := ""
		if fileStatus.Worktree == git.Untracked && fileStatus.Staging == git.Untracked {
			msg += "?? "
		} else {
			msg += " " + string(fileStatus.Worktree) + " "
		}
		msg += filePath
		commitMsg = append(commitMsg, msg)
	}

	sort.Strings(commitMsg)
	msg := strings.Join(commitMsg, "\n")

	if !hasChanges {
		return nil
	}

	_, err = GitCommand(repoConfig, []string{"commit", "-m", msg})
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func GitCommand(repoConfig RepoConfig, args []string) (bytes.Buffer, error) {
	repoPath := repoConfig.RepoPath

	var outb, errb bytes.Buffer

	cmd := "git"
	if repoConfig.GitExec != "" {
		cmd = repoConfig.GitExec
	}

	statusCmd := exec.Command(cmd, args...)
	statusCmd.Dir = repoPath
	statusCmd.Stdout = &outb
	statusCmd.Stderr = &errb
	env := toEnvString(repoConfig)
	statusCmd.Env = env
	err := statusCmd.Run()

	// toEnvString forwards SSH_AUTH_SOCK from the daemon's environment, so it's
	// only genuinely missing (and ssh auth will fail) when it's absent there
	// and not explicitly configured for the repo.
	if !hasEnvVariable(env, "SSH_AUTH_SOCK") {
		fmt.Println("WARNING: SSH_AUTH_SOCK is not set, ssh-based git operations may fail")
	}

	if err != nil {
		fullCmd := "git " + strings.Join(args, " ")
		err := tracerr.Errorf("%w: Command: %s\nEnv: %s\nStdOut: %s\nStdErr: %s", err, fullCmd, statusCmd.Env, outb.String(), errb.String())
		return outb, err
	}
	return outb, nil
}

func toEnvString(repoConfig RepoConfig) []string {
	vals := repoConfig.Env

	// Forward a small set of variables from the daemon's own environment that
	// git/ssh need to authenticate and find the user's config. SSH_AUTH_SOCK is
	// essential: launchd injects it into the daemon, but without forwarding it
	// to the git subprocess ssh can't reach the agent and pushes fail with
	// "internal error performing authentication". Variables explicitly set on
	// the repo config take precedence and are not overridden.
	forward := map[string]bool{"HOME": true, "SSH_AUTH_SOCK": true}
	for _, s := range os.Environ() {
		parts := strings.SplitN(s, "=", 2)
		k := parts[0]
		if forward[k] && !hasEnvVariable(repoConfig.Env, k) {
			vals = append(vals, s)
		}
	}

	return vals
}

func hasEnvVariable(all []string, name string) bool {
	for _, s := range all {
		parts := strings.Split(s, "=")
		k := parts[0]
		if k == name {
			return true
		}
	}
	return false
}
