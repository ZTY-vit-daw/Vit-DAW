//go:build windows

package runtimehost

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW：headless 内核/hub 无控制台窗（SHELL-4 链无窗语义，
// 与 Godot OS.create_process 在无窗宿主下的表现一致）。
const createNoWindow = 0x08000000

// execCommand 构造子进程命令（cwd 不设=继承 agent cwd：内核 Workspace 根按
// exe 回溯解析（VitPaths.h），与 Godot 链同一 <dev_root>/VitApp 结果）。
func execCommand(exe string) *exec.Cmd {
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
