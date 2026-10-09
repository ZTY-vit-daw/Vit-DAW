package carriers

// ledger_dir_test.go — L4-LEDGER-DIR-1：共享 helper ResolveProjectDir（真栈
// project_path=.vit 文件→账本宿主=父目录；已是目录→原样；缺失路径→父目录；
// 空→空，无工程面不落 cwd 相对面）。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectDirVitFileFallsToParent(t *testing.T) {
	parent := t.TempDir()
	projectFile := filepath.Join(parent, "song.vit")
	if err := os.WriteFile(projectFile, []byte("kernel project file placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveProjectDir(projectFile); got != parent {
		t.Fatalf("ResolveProjectDir(%q) = %q, want parent %q", projectFile, got, parent)
	}
}

func TestResolveProjectDirDirectoryPassthrough(t *testing.T) {
	dir := t.TempDir()
	if got := ResolveProjectDir(dir); got != dir {
		t.Fatalf("ResolveProjectDir(%q) = %q, want as-is", dir, got)
	}
}

func TestResolveProjectDirMissingPathFallsToParent(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "absent.vit")
	if got := ResolveProjectDir(missing); got != parent {
		t.Fatalf("ResolveProjectDir(%q) = %q, want parent %q", missing, got, parent)
	}
}

func TestResolveProjectDirEmptyStaysEmpty(t *testing.T) {
	if got := ResolveProjectDir(""); got != "" {
		t.Fatalf(`ResolveProjectDir("") = %q, want empty (no project face must not resolve to a cwd-relative dir)`, got)
	}
}
