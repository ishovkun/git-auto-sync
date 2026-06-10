package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kardianos/service"
	"github.com/ztrue/tracerr"
)

type Service struct {
	Service service.Service
}

type emptyDaemon struct{}

func (d emptyDaemon) Start(s service.Service) error {
	return nil
}

func (d emptyDaemon) Stop(s service.Service) error {
	return nil
}

func NewServiceWithDaemon(daemon service.Interface) (Service, error) {
	options := make(service.KeyValue)
	options["Restart"] = "on-success"
	options["UserService"] = true
	options["RunAtLoad"] = true

	ex, err := os.Executable()
	if err != nil {
		return Service{}, tracerr.Wrap(err)
	}
	exDirPath := filepath.Dir(ex)
	executablePath := filepath.Join(exDirPath, "git-auto-sync-daemon")

	deps := []string{}
	if runtime.GOOS == "linux" {
		deps = []string{"After=network-online.target syslog.target"}
	}

	// On macOS the default kardianos launchd template writes logs to
	// /usr/local/var/log, which does not exist on Apple Silicon (Homebrew uses
	// /opt/homebrew) and is not the conventional location for a user service.
	// launchd silently refuses to start a job whose log paths can't be opened,
	// so point the logs at ~/Library/Logs and make sure that directory exists.
	// See https://github.com/GitJournal/git-auto-sync/issues/22
	if runtime.GOOS == "darwin" {
		launchdCfg, err := darwinLaunchdConfig("git-auto-sync-daemon")
		if err != nil {
			return Service{}, err
		}
		options["LaunchdConfig"] = launchdCfg
	}

	svcConfig := &service.Config{
		Name:        "git-auto-sync-daemon",
		DisplayName: "Git Auto Sync Daemon",
		Description: "Background Process for Auto Syncing Git Repos",

		Executable:   executablePath,
		Dependencies: deps,
		Option:       options,
	}

	s, err := service.New(daemon, svcConfig)
	if err != nil {
		return Service{}, tracerr.Wrap(err)
	}

	return Service{Service: s}, nil
}

func NewService() (Service, error) {
	return NewServiceWithDaemon(emptyDaemon{})
}

func (srv Service) Enable() error {
	s := srv.Service

	status, err := s.Status()
	if err != nil {
		if !strings.Contains(err.Error(), "the service is not installed") {
			return tracerr.Wrap(err)
		}
	}

	stopped := false
	if status == service.StatusRunning {
		err := s.Stop()
		if err != nil {
			return tracerr.Wrap(err)
		}
		stopped = true
	}

	err = s.Install()
	if err != nil {
		if strings.Contains(err.Error(), "Init already exists") {
			_ = s.Uninstall()
			_ = s.Install()
		} else {
			return tracerr.Wrap(err)
		}
	} else {
		fmt.Println("Installing git-auto-sync as a daemon")
	}

	if stopped {
		fmt.Println("Restarting git-auto-sync-daemon")
	} else {
		fmt.Println("Starting git-auto-sync-daemon")
	}

	err = s.Start()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (srv Service) Disable() error {
	fmt.Println("Stopping git-auto-sync-daemon")
	err := srv.Service.Stop()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Println("Uninstalling git-auto-sync as a daemon")
	err = srv.Service.Uninstall()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

// LogDir returns the directory where the daemon's log files should live for
// the current OS, creating it if it does not already exist. On Linux logs go
// to the systemd journal and on Windows to the event log, so there is no
// file-based log directory to manage and an empty string is returned.
func LogDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", tracerr.Wrap(err)
		}
		logDir := filepath.Join(home, "Library", "Logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return "", tracerr.Wrap(err)
		}
		return logDir, nil
	default:
		return "", nil
	}
}

// darwinLaunchdConfig builds a launchd plist template that writes the daemon's
// stdout/stderr to ~/Library/Logs instead of the kardianos default of
// /usr/local/var/log, and ensures that log directory exists.
func darwinLaunchdConfig(name string) (string, error) {
	logDir, err := LogDir()
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(logDir, name+".out.log")
	errPath := filepath.Join(logDir, name+".err.log")
	return fmt.Sprintf(launchdConfigTemplate, outPath, errPath), nil
}

// launchdConfigTemplate mirrors the default template in kardianos/service but
// with the StandardOutPath/StandardErrorPath log locations left as %s
// placeholders so they can be filled in at install time. The {{...}} actions
// are evaluated by kardianos against the service config.
const launchdConfigTemplate = `<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN"
"http://www.apple.com/DTDs/PropertyList-1.0.dtd" >
<plist version='1.0'>
  <dict>
    <key>Label</key>
    <string>{{html .Name}}</string>
    <key>ProgramArguments</key>
    <array>
      <string>{{html .Path}}</string>
    {{range .Config.Arguments}}
      <string>{{html .}}</string>
    {{end}}
    </array>
    {{if .UserName}}<key>UserName</key>
    <string>{{html .UserName}}</string>{{end}}
    {{if .ChRoot}}<key>RootDirectory</key>
    <string>{{html .ChRoot}}</string>{{end}}
    {{if .WorkingDirectory}}<key>WorkingDirectory</key>
    <string>{{html .WorkingDirectory}}</string>{{end}}
    <key>SessionCreate</key>
    <{{bool .SessionCreate}}/>
    <key>KeepAlive</key>
    <{{bool .KeepAlive}}/>
    <key>RunAtLoad</key>
    <{{bool .RunAtLoad}}/>
    <key>Disabled</key>
    <false/>

    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>

  </dict>
</plist>
`

func (srv Service) Status() error {
	status, err := srv.Service.Status()
	if err != nil {
		return tracerr.Wrap(err)
	}

	switch status {
	case service.StatusRunning:
		fmt.Println("git-auto-sync-daemon is Running!")
	case service.StatusStopped:
		fmt.Println("git-auto-sync-daemon is NOT Running!")
	case service.StatusUnknown:
	default:
		fmt.Println("git-auto-sync-daemon status is Unknown. How mysterious!")
	}

	return nil
}
