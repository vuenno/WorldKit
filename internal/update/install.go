package update

import (
	"fmt"
	"os/exec"
	"strconv"
)

// InstallOptions say how the installer runs.
type InstallOptions struct {
	// WaitForPID is the process the installer waits for before replacing files: the
	// running WorldKit, which quits right after starting it.
	WaitForPID int
	// LogPath is where Inno Setup writes its log; empty: no log.
	LogPath string
}

// StartInstaller runs a downloaded installer silently and returns without waiting. The
// installer (installer/worldkit.iss) waits for WaitForPID to exit, upgrades WorldKit in
// place and starts it again; the caller should quit straight away.
func StartInstaller(path string, options InstallOptions) error {
	command := exec.Command(path, installerArgs(options)...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start installer %q: %w", path, err)
	}
	// The installer outlives WorldKit, so nothing waits for it.
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release installer process: %w", err)
	}
	return nil
}

// installerArgs are Inno Setup's silent-upgrade switches plus worldkit.iss's own:
// /WAITPID (wait for the old WorldKit to exit) and /RELAUNCH (start it again afterwards).
func installerArgs(options InstallOptions) []string {
	args := []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS", "/RELAUNCH=1"}
	if options.WaitForPID > 0 {
		args = append(args, "/WAITPID="+strconv.Itoa(options.WaitForPID))
	}
	if options.LogPath != "" {
		args = append(args, "/LOG="+options.LogPath)
	}
	return args
}
