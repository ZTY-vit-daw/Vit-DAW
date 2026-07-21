//go:build !windows

package vst3host

import "os/exec"

func configureCommand(command *exec.Cmd) {}
