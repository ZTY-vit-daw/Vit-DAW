//go:build !windows

package genapi

import (
	"fmt"
	"os/exec"
	"syscall"
)

// applyEngineSysProcAttr puts the engine in its own process group so the
// supervisor can signal the whole group.
func applyEngineSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopEngineProcessTree kills the engine process group (a wrapper script's
// children die with the group).
func stopEngineProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		if kerr := cmd.Process.Kill(); kerr != nil {
			return fmt.Errorf("group kill: %v; kill: %v", err, kerr)
		}
	}
	return nil
}
