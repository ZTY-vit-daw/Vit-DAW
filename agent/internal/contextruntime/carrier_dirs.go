package contextruntime

// carrier_dirs.go — 四层载体的目录解析（L1-4-IMPL-D D3 真实工程装载）。
//
// L2（user_profile.v1.json）/L3（env_instance.v1.json）是用户级/机器级载体，
// 宿主目录沿 journal 的 workspace-root 探测约定（DefaultSnapshotPath 同族）：
// <workspaceRoot>/VitApp/Workspace（源码树内探测到仓库根；部署栈上探测到
// VitApp 同级运行目录）。L4 工程账本目录归 ProjectDir（工程包内，装配面
// 由 chat/agentloop 按 run 上下文供给）。

import (
	"os"
	"path/filepath"
)

// DefaultCarrierWorkspaceDir 返回用户级载体宿主目录；工作目录不可得时
// 返回空串（调用方按 absent 处理——不臆造目录）。
func DefaultCarrierWorkspaceDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root := detectWorkspaceRoot(wd); root != "" {
		return filepath.Join(root, "VitApp", "Workspace")
	}
	return filepath.Join(wd, "VitApp", "Workspace")
}
