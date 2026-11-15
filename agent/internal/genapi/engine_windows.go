//go:build windows

package genapi

import (
	"fmt"
	"os/exec"
	"syscall"
)

// applyEngineSysProcAttr detaches the engine from our console (no window
// flashing) and puts it in its own process group so a group kill cannot take
// the agent down with it.
func applyEngineSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:            true,
		CreationFlags:         syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// stopEngineProcessTree kills the started engine and its children (wrapper
// scripts spawn the real engine process; killing only the wrapper would
// leave the engine behind holding the port).
func stopEngineProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := kill.Run(); err != nil {
		// Fall back to a direct kill; the tree kill may have partially
		// succeeded (taskkill returns nonzero when some children are gone).
		if kerr := cmd.Process.Kill(); kerr != nil {
			return fmt.Errorf("taskkill: %v; kill: %v", err, kerr)
		}
	}
	return nil
}
