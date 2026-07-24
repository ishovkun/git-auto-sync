package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GitJournal/git-auto-sync/common"
	cfg "github.com/GitJournal/git-auto-sync/common/config"
	"github.com/kardianos/service"
	cli "github.com/urfave/cli/v2"
	"github.com/ztrue/tracerr"
	"golang.org/x/exp/slices"
	git "gopkg.in/src-d/go-git.v4"
)

var errRepoPathInvalid = errors.New("Not a valid git repo")

func daemonStatus(ctx *cli.Context) error {
	s, err := common.NewService()
	if err != nil {
		return tracerr.Wrap(err)
	}

	config, err := cfg.Read()
	if err != nil {
		return tracerr.Wrap(err)
	}

	if ctx.Bool("json") {
		return printDaemonStatusJSON(s, config)
	}

	err = s.Status()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Println("Monitoring - ")
	for _, repoPath := range config.Repos {
		fmt.Println("  ", repoPath)
	}

	// FIXME: Print out if there are any 'rebasing' issues and we are paused

	return nil
}

type daemonStatusSnapshot struct {
	Daemon      string               `json:"daemon"`
	DaemonError string               `json:"daemon_error,omitempty"`
	Repos       []daemonRepoSnapshot `json:"repos"`
}

type daemonRepoSnapshot struct {
	Repo         string    `json:"repo"`
	Synced       bool      `json:"synced"`
	OK           bool      `json:"ok"`
	Error        string    `json:"error,omitempty"`
	SyncedAt     time.Time `json:"synced_at,omitempty"`
	SyncedAtUnix int64     `json:"synced_at_unix,omitempty"`
}

func printDaemonStatusJSON(s common.Service, config *cfg.ConfigV1) error {
	snapshot := daemonStatusSnapshot{
		Daemon: "unknown",
		Repos:  make([]daemonRepoSnapshot, 0, len(config.Repos)),
	}

	status, err := s.CurrentStatus()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not installed") {
			snapshot.Daemon = "stopped"
		} else {
			snapshot.DaemonError = err.Error()
		}
	} else {
		switch status {
		case service.StatusRunning:
			snapshot.Daemon = "running"
		case service.StatusStopped:
			snapshot.Daemon = "stopped"
		}
	}

	syncStatus, err := common.ReadStatus()
	if err != nil {
		return tracerr.Wrap(err)
	}
	for _, repoPath := range config.Repos {
		repo := daemonRepoSnapshot{Repo: repoPath}
		if recorded, ok := syncStatus.Repos[repoPath]; ok {
			repo.Synced = true
			repo.OK = recorded.OK
			repo.Error = recorded.Error
			repo.SyncedAt = recorded.SyncedAt
			repo.SyncedAtUnix = recorded.SyncedAtUnix
		}
		snapshot.Repos = append(snapshot.Repos, repo)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(snapshot); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func daemonList(ctx *cli.Context) error {
	config, err := cfg.Read()
	if err != nil {
		return tracerr.Wrap(err)
	}

	for _, repoPath := range config.Repos {
		fmt.Println(repoPath)
	}
	return nil
}

func daemonAdd(ctx *cli.Context) error {
	repoPath := ctx.Args().First()
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return tracerr.Wrap(err)
	}

	repoPath, err = isValidGitRepo(repoPath)
	if err != nil {
		return tracerr.Wrap(err)
	}

	config, err := cfg.Read()
	if err != nil {
		return tracerr.Wrap(err)
	}

	if slices.Contains(config.Repos, repoPath) {
		fmt.Println("The Daemon is already monitoring " + repoPath)
	} else {
		config.Repos = append(config.Repos, repoPath)
	}

	err = cfg.Write(config)
	if err != nil {
		return tracerr.Wrap(err)
	}

	s, err := common.NewService()
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = s.Enable()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func isValidGitRepo(repoPath string) (string, error) {
	info, err := os.Stat(repoPath)
	if os.IsNotExist(err) {
		return "", tracerr.Errorf("%w - %s", errRepoPathInvalid, repoPath)
	}

	if !info.IsDir() {
		return "", tracerr.Errorf("%w - %s", errRepoPathInvalid, repoPath)
	}

	_, err = git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", tracerr.Errorf("Not a valid git repo - %s\n%w", repoPath, err)
	}

	for {
		info, err := os.Stat(filepath.Join(repoPath, ".git"))
		if err != nil {
			if !os.IsNotExist(err) {
				return "", tracerr.Errorf("%w - %s", errRepoPathInvalid, repoPath)
			}
		}

		if os.IsNotExist(err) {
			repoPath = filepath.Dir(repoPath)
			continue
		}

		if !info.IsDir() {
			return "", tracerr.Errorf("%w - %s", errRepoPathInvalid, repoPath)
		}
		break
	}

	return repoPath, nil
}

func daemonRm(ctx *cli.Context) error {
	repoPath := ctx.Args().First()
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return tracerr.Wrap(err)
	}

	repoPath, err = isValidGitRepo(repoPath)
	if err != nil {
		return tracerr.Wrap(err)
	}

	config, err := cfg.Read()
	if err != nil {
		return tracerr.Wrap(err)
	}

	pos := -1
	for i, rp := range config.Repos {
		if rp == repoPath {
			pos = i
			break
		}
	}

	if pos == -1 {
		err = errors.New("Repo Not tracked")
		return tracerr.Errorf("%w - %s", err, repoPath)
	}

	config.Repos = remove(config.Repos, pos)
	err = cfg.Write(config)
	if err != nil {
		return tracerr.Wrap(err)
	}

	if len(config.Repos) == 0 {
		s, err := common.NewService()
		if err != nil {
			return tracerr.Wrap(err)
		}

		err = s.Disable()
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	return nil
}

func daemonEnv(ctx *cli.Context) error {
	vars := ctx.Args().Slice()

	for _, v := range vars {
		if !strings.Contains(v, "=") {
			log.Fatalln("Env variables must be in the format 'key=value'")
		}
	}

	config, err := cfg.Read()
	if err != nil {
		return tracerr.Wrap(err)
	}

	envMap := toEnvMap(config.Envs)
	newMap := toEnvMap(vars)

	for k, v := range newMap {
		envMap[k] = v
	}

	config.Envs = toEnvStrings(envMap)
	err = cfg.Write(config)
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Println(strings.Join(config.Envs, "\n"))

	// The daemon reads environment values at startup. Reinstall/restart it when
	// repositories are active so changes made by the Plasma applet take effect
	// immediately.
	if len(config.Repos) > 0 {
		s, err := common.NewService()
		if err != nil {
			return tracerr.Wrap(err)
		}
		if err := s.Enable(); err != nil {
			return tracerr.Wrap(err)
		}
	}

	return nil
}

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func toEnvMap(envs []string) map[string]string {
	m := map[string]string{}
	for _, e := range envs {
		parts := strings.Split(e, "=")
		if len(parts) > 1 {
			m[parts[0]] = strings.Join(parts[1:], "=")
		} else {
			m[e] = ""
		}
	}

	return m
}

func toEnvStrings(m map[string]string) []string {
	vals := []string{}
	for k, v := range m {
		x := fmt.Sprintf("%s=%s", k, v)
		vals = append(vals, x)
	}

	return vals
}
