//go:build !windows

package runtimehost

import "os/exec"

// execCommand 非 Windows 宿主（Mac 链）：默认属性即可（PORT-B1 命名已按平台
// 分流；Mac 侧无 CREATE_NO_WINDOW 语义）。
func execCommand(exe string) *exec.Cmd {
	return exec.Command(exe)
}
