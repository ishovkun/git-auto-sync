package common

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rjeczalik/notify"
	"github.com/ztrue/tracerr"
	git "gopkg.in/src-d/go-git.v4"
)

type RepoConfig struct {
	RepoPath     string
	PollInterval time.Duration
	FSLag        time.Duration
	GitExec      string
	Env          []string
}

type AwakeNotifier interface {
	Start(chan bool) error
}

func NewRepoConfig(repoPath string) (RepoConfig, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return RepoConfig{}, tracerr.Wrap(err)
	}

	config, err := repo.Config()
	if err != nil {
		return RepoConfig{}, tracerr.Wrap(err)
	}

	autoSyncSection := config.Raw.Section("auto-sync")

	pollInterval := 10 * time.Minute
	if autoSyncSection.Option("syncInterval") != "" {
		secondsStr := autoSyncSection.Option("syncInterval")
		seconds, err := strconv.Atoi(secondsStr)
		if err != nil {
			return RepoConfig{}, tracerr.Wrap(err)
		}

		pollInterval = time.Duration(seconds) * time.Second
	}

	gitExec := ""
	if autoSyncSection.Option("exec") != "" {
		gitExec = autoSyncSection.Option("exec")

		_, err := os.Stat(gitExec)
		if err != nil {
			return RepoConfig{}, tracerr.Wrap(err)
		}
	}

	return RepoConfig{
		RepoPath:     repoPath,
		PollInterval: pollInterval,
		FSLag:        1 * time.Second,
		GitExec:      gitExec,
	}, nil
}

func WatchForChanges(cfg RepoConfig) error {
	return watchForChanges(context.Background(), cfg, watchDependencies{
		sync: AutoSync,
		startAwakeNotifier: func(out chan bool) error {
			notifier, err := NewAwakeNotifier()
			if err != nil {
				return err
			}
			return notifier.Start(out)
		},
	})
}

type watchDependencies struct {
	sync               func(RepoConfig) error
	startAwakeNotifier func(chan bool) error
}

func watchForChanges(ctx context.Context, cfg RepoConfig, deps watchDependencies) error {
	repoPath := cfg.RepoPath

	notifyChannel := make(chan notify.EventInfo, 100)
	err := notify.Watch(filepath.Join(repoPath, "..."), notifyChannel, notify.Write, notify.Rename, notify.Remove, notify.Create)
	if err != nil {
		return tracerr.Wrap(err)
	}
	defer notify.Stop(notifyChannel)

	awakeChannel := make(chan bool, 100)
	if deps.startAwakeNotifier != nil {
		if err := deps.startAwakeNotifier(awakeChannel); err != nil {
			log.Printf("Could not start awake notifier for %s: %v", repoPath, err)
		}
	}

	pollTicker := time.NewTicker(cfg.PollInterval)
	defer pollTicker.Stop()

	var debounceTimer *time.Timer
	var debounceChannel <-chan time.Time
	defer func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
	}()

	scheduleSync := func() {
		if debounceTimer == nil {
			debounceTimer = time.NewTimer(cfg.FSLag)
		} else {
			if !debounceTimer.Stop() {
				select {
				case <-debounceTimer.C:
				default:
				}
			}
			debounceTimer.Reset(cfg.FSLag)
		}
		debounceChannel = debounceTimer.C
	}

	runSync := func() {
		if err := deps.sync(cfg); err != nil {
			log.Printf("Sync failed for %s; watching and retries will continue: %v", repoPath, err)
		}
	}

	// Install the filesystem watcher before the initial sync so a failed sync
	// cannot leave a service process running without a live repository worker.
	runSync()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-awakeChannel:
			scheduleSync()
		case <-pollTicker.C:
			scheduleSync()
		case <-debounceChannel:
			debounceChannel = nil
			runSync()
		case ei := <-notifyChannel:
			ignore, err := ShouldIgnoreFile(repoPath, ei.Path())
			if err != nil {
				return tracerr.Wrap(err)
			}
			if !ignore {
				scheduleSync()
			}
		}
	}
}
