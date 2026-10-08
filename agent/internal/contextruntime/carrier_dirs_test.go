package contextruntime

// carrier_dirs_test.go — REVIEW-1 G-2 缺口补齐：DefaultCarrierWorkspaceDir
// （含 detectWorkspaceRoot 探测与 getwd 失败回落空串）零测试覆盖 → 本文件
// 覆盖 workspace-root 探测三分支（仓库根/agent 内层/VitApp 内层）与
// 非仓库 cwd 的 wd 拼接回落。getwd 失败回落分支在 Windows 上不可移植触发
//（实测：删除 cwd 后 os.Getwd 仍返回旧路径不报错），该分支为单行
// `return ""`，由代码读核覆盖、不造假触发。

import (
	"os"
	"path/filepath"
	"testing"
)

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

// makeWorkspaceRoot 搭一个最小可探测工作区根（agent/go.mod + VitApp/）。
func makeWorkspaceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o755); err != nil {
		t.Fatalf("mkdir agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent", "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "VitApp", "Workspace"), 0o755); err != nil {
		t.Fatalf("mkdir VitApp: %v", err)
	}
	return root
}

func TestDefaultCarrierWorkspaceDirProbesWorkspaceRoot(t *testing.T) {
	root := makeWorkspaceRoot(t)
	resolved := root
	if evaluated, err := filepath.EvalSymlinks(root); err == nil {
		resolved = evaluated
	}

	// 分支 1：仓库根直接命中（agent/go.mod + VitApp/）。
	chdirForTest(t, resolved)
	if got, want := DefaultCarrierWorkspaceDir(), filepath.Join(resolved, "VitApp", "Workspace"); got != want {
		t.Fatalf("from repo root: DefaultCarrierWorkspaceDir() = %q want %q", got, want)
	}

	// 分支 2：agent/ 内层（Base==agent + go.mod，向上探测到根）。
	chdirForTest(t, filepath.Join(resolved, "agent"))
	if got, want := DefaultCarrierWorkspaceDir(), filepath.Join(resolved, "VitApp", "Workspace"); got != want {
		t.Fatalf("from agent dir: DefaultCarrierWorkspaceDir() = %q want %q", got, want)
	}

	// 分支 3：VitApp/ 内层（Base==VitApp + Workspace/ 存在）。
	chdirForTest(t, filepath.Join(resolved, "VitApp"))
	if got, want := DefaultCarrierWorkspaceDir(), filepath.Join(resolved, "VitApp", "Workspace"); got != want {
		t.Fatalf("from VitApp dir: DefaultCarrierWorkspaceDir() = %q want %q", got, want)
	}
}

func TestDefaultCarrierWorkspaceDirFallsBackToCwdOutsideRepo(t *testing.T) {
	// 非仓库 cwd：探测失败 → 沿 getwd 拼接 VitApp/Workspace（不臆造根）。
	outside := t.TempDir()
	resolved := outside
	if evaluated, err := filepath.EvalSymlinks(outside); err == nil {
		resolved = evaluated
	}
	chdirForTest(t, resolved)
	if got, want := DefaultCarrierWorkspaceDir(), filepath.Join(resolved, "VitApp", "Workspace"); got != want {
		t.Fatalf("outside repo: DefaultCarrierWorkspaceDir() = %q want %q", got, want)
	}

	// detectWorkspaceRoot 直测：非仓库起点返回空串（探测失败语义）。
	if got := detectWorkspaceRoot(resolved); got != "" {
		t.Fatalf("detectWorkspaceRoot(%q) = %q, want empty", resolved, got)
	}
}
