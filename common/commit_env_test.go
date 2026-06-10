package common

import (
	"testing"

	"gotest.tools/v3/assert"
)

// SSH_AUTH_SOCK must be forwarded from the daemon's environment to the git
// subprocess, otherwise ssh cannot reach the agent and pushes fail with
// "internal error performing authentication" (the daemon runs but never syncs).
func Test_ToEnvStringForwardsSSHAuthSock(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/test-agent.sock")
	t.Setenv("HOME", "/tmp/test-home")

	env := toEnvString(RepoConfig{})

	assert.Assert(t, hasEnvVariable(env, "SSH_AUTH_SOCK"))
	assert.Assert(t, hasEnvVariable(env, "HOME"))
}

// An explicitly configured value must win over the forwarded one and must not
// be duplicated.
func Test_ToEnvStringConfigOverridesForwarded(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/from-launchd.sock")

	env := toEnvString(RepoConfig{Env: []string{"SSH_AUTH_SOCK=/tmp/configured.sock"}})

	count := 0
	for _, s := range env {
		if s == "SSH_AUTH_SOCK=/tmp/configured.sock" {
			count++
		}
	}
	assert.Equal(t, count, 1, "configured SSH_AUTH_SOCK should appear exactly once")
	for _, s := range env {
		assert.Assert(t, s != "SSH_AUTH_SOCK=/tmp/from-launchd.sock", "launchd value should not override configured value")
	}
}
