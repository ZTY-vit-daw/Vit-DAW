package chat

// LEGAME-DIR-1（314）乐局目录三级解析单测：三级解析顺序 × 存在性矩阵全表驱动
//（env 设/不设 × ~/.vit/levels 有/无 × cwd 在/不在仓库根，313 设计稿 §2.1），
// 外加三级各自的装载可达性与 enter 失败文案产品化断言。
//
// 环境轴控制说明：矩阵同时需要改 cwd 与改 env（HOME/USERPROFILE + 可选
// VIT_EDU_LEVELS_DIR），t.Chdir/t.Setenv 各自禁止与另一方混用，故本文件统一用
// 手动 os.Chdir/os.Setenv + defer 恢复（chat 包测试无 t.Parallel，串行安全）。
// home 目录轴走 os.UserHomeDir 的真实解析面（Windows=%USERPROFILE%、unix=$HOME，
// 两变量同时指到伪 home，另一平台静默忽略——不引入测试专用注缝，与生产同一条
// 代码路径）。每张卡的 level id 全局唯一（eduJamCardCache 进程内共享），三级各
// 用一个专属 id，互不串缓存。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eduJamCopyFixtureCardAs 取夹具卡并整体复制为指定 id 的整卡目录（assets/tools/
// persona 递归复制；level.yaml 的 id 行改写为新 id——夹具内该字面量仅出现于
// id: 行，注释行不含 "id: " 前缀，Replace 次数=1 即精准命中）。
func eduJamCopyFixtureCardAs(t *testing.T, dstDir, newID string) string {
	t.Helper()
	src := filepath.Join(eduJamTestLevelsDir, "first-run-twinkle")
	dst := filepath.Join(dstDir, newID)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if d.Name() == "level.yaml" {
			data = []byte(strings.Replace(string(data), "id: first-run-twinkle", "id: "+newID, 1))
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture card: %v", err)
	}
	return dst
}

// eduJamDirEnvScope 场景化三级解析环境（env 轴 + home 轴 + cwd 轴），defer 全恢复。
type eduJamDirEnvScope struct {
	t             *testing.T
	oldEnv        string
	oldEnvSet     bool
	oldHomeWin    string
	oldHomeWinSet bool
	oldHomeNix    string
	oldHomeNixSet bool
	oldCwd        string
}

// eduJamScopeDirEnv 进入指定环境：levelsEnv 空=unset；home 空=保持真实 home；
// cwd 空=保持当前目录。
func eduJamScopeDirEnv(t *testing.T, levelsEnv, home, cwd string) *eduJamDirEnvScope {
	t.Helper()
	s := &eduJamDirEnvScope{t: t}
	// VIT_EDU_LEVELS_DIR 轴。
	s.oldEnv, s.oldEnvSet = os.LookupEnv("VIT_EDU_LEVELS_DIR")
	if levelsEnv == "" {
		_ = os.Unsetenv("VIT_EDU_LEVELS_DIR")
	} else {
		_ = os.Setenv("VIT_EDU_LEVELS_DIR", levelsEnv)
	}
	// home 轴：两平台变量同设（UserHomeDir 只读各自平台那个，另一个无害）。
	s.oldHomeWin, s.oldHomeWinSet = os.LookupEnv("USERPROFILE")
	s.oldHomeNix, s.oldHomeNixSet = os.LookupEnv("HOME")
	if home != "" {
		_ = os.Setenv("USERPROFILE", home)
		_ = os.Setenv("HOME", home)
	}
	// cwd 轴。
	if cwd != "" {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		s.oldCwd = wd
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("chdir %s: %v", cwd, err)
		}
	}
	t.Cleanup(s.restore)
	return s
}

func (s *eduJamDirEnvScope) restore() {
	if s.oldEnvSet {
		_ = os.Setenv("VIT_EDU_LEVELS_DIR", s.oldEnv)
	} else {
		_ = os.Unsetenv("VIT_EDU_LEVELS_DIR")
	}
	if s.oldHomeWinSet {
		_ = os.Setenv("USERPROFILE", s.oldHomeWin)
	} else {
		_ = os.Unsetenv("USERPROFILE")
	}
	if s.oldHomeNixSet {
		_ = os.Setenv("HOME", s.oldHomeNix)
	} else {
		_ = os.Unsetenv("HOME")
	}
	if s.oldCwd != "" {
		if err := os.Chdir(s.oldCwd); err != nil {
			s.t.Fatalf("restore cwd %s: %v", s.oldCwd, err)
		}
	}
}

// TestEduJamLevelsDirThreeTierMatrix 三级解析顺序×存在性矩阵（纯解析函数断言，
// 313 设计稿 §2.1 验收口径）：
//
//	env 设            → env 目录（即便 home/repo 两级也存在——覆写最高优先）
//	env 不设 + home 有 → ~/.vit/levels（cwd 两态下都选它）
//	env 不设 + home 无 → coord/levels（cwd 两态下都回退它；可达性另测）
//	home 不可解析      → coord/levels
func TestEduJamLevelsDirThreeTierMatrix(t *testing.T) {
	fakeHome := t.TempDir()
	fakeHomeLevels := filepath.Join(fakeHome, ".vit", "levels")
	if err := os.MkdirAll(fakeHomeLevels, 0o755); err != nil {
		t.Fatalf("mkdir fake home levels: %v", err)
	}
	fakeRepo := t.TempDir()    // cwd 在「仓库根」形态：其下有 coord/levels
	fakeNonRepo := t.TempDir() // cwd 非 repo 根：其下无 coord/levels
	if err := os.MkdirAll(filepath.Join(fakeRepo, "coord", "levels"), 0o755); err != nil {
		t.Fatalf("mkdir fake repo levels: %v", err)
	}
	envDir := filepath.Join(t.TempDir(), "env-levels")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir env levels: %v", err)
	}

	cases := []struct {
		name          string
		levelsEnv     string
		home          string // ""=真实 home（矩阵里不依赖其内容：home 有/无用显式 fake home 控制）
		cwd           string
		homeHasLevels bool
		want          string
	}{
		{"env set wins over home+repo", envDir, fakeHome, fakeRepo, true, envDir},
		{"env set wins when home empty", envDir, fakeHome, fakeNonRepo, false, envDir},
		{"env unset + home levels + repo cwd", "", fakeHome, fakeRepo, true, fakeHomeLevels},
		{"env unset + home levels + non-repo cwd", "", fakeHome, fakeNonRepo, true, fakeHomeLevels},
		{"env unset + no home levels + repo cwd", "", fakeHome, fakeRepo, false, filepath.Join("coord", "levels")},
		{"env unset + no home levels + non-repo cwd", "", fakeHome, fakeNonRepo, false, filepath.Join("coord", "levels")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := tc.home
			if !tc.homeHasLevels {
				bareHome := t.TempDir()
				if err := os.MkdirAll(filepath.Join(bareHome, ".vit"), 0o755); err != nil {
					t.Fatalf("mkdir bare .vit: %v", err)
				}
				home = bareHome
			}
			eduJamScopeDirEnv(t, tc.levelsEnv, home, tc.cwd)
			if got := eduJamLevelsDir(); got != tc.want {
				t.Fatalf("eduJamLevelsDir()=%q want %q", got, tc.want)
			}
		})
	}

	// home 不可解析（USERPROFILE/HOME 双空 → UserHomeDir 报错）→ repo 回退级。
	t.Run("home unresolvable falls to repo tier", func(t *testing.T) {
		scope := eduJamScopeDirEnv(t, "", "", "")
		_ = os.Unsetenv("USERPROFILE")
		_ = os.Unsetenv("HOME")
		t.Cleanup(func() { scope.restore() })
		if got := eduJamLevelsDir(); got != filepath.Join("coord", "levels") {
			t.Fatalf("eduJamLevelsDir()=%q want coord/levels fallback", got)
		}
	})
}

// TestEduJamLevelsDirLoadReachability 每级命中时 loadEduJamLevelCard 真装载可达
// （整卡目录复制夹具，专属 level id 避开进程内卡缓存互串）：
//
//	tier1 env 目录装载 ✓（既有测试形态的矩阵化复证）
//	tier2 ~/.vit/levels 装载 ✓（产品主路径：前端拉起、无 env、cwd 非 repo）
//	tier3 cwd=伪仓库根装载 ✓（开发态回退路径）
//	三级全缺 → 装载失败 → enter fail-closed 产品化文案
func TestEduJamLevelsDirLoadReachability(t *testing.T) {
	// tier 1：env 目录。
	t.Run("tier1 env dir load", func(t *testing.T) {
		envRoot := t.TempDir()
		eduJamCopyFixtureCardAs(t, envRoot, "legame-dir-tier1")
		eduJamScopeDirEnv(t, envRoot, "", "")
		card, err := loadEduJamLevelCard("legame-dir-tier1")
		if err != nil {
			t.Fatalf("tier1 load: %v", err)
		}
		if card.ID != "legame-dir-tier1" {
			t.Fatalf("tier1 card id=%q", card.ID)
		}
	})

	// tier 2：~/.vit/levels（env unset + cwd 非 repo——产品主路径形态）。
	t.Run("tier2 home levels load (product path)", func(t *testing.T) {
		fakeHome := t.TempDir()
		eduJamCopyFixtureCardAs(t, filepath.Join(fakeHome, ".vit", "levels"), "legame-dir-tier2")
		nonRepo := t.TempDir()
		eduJamScopeDirEnv(t, "", fakeHome, nonRepo)
		card, err := loadEduJamLevelCard("legame-dir-tier2")
		if err != nil {
			t.Fatalf("tier2 load: %v", err)
		}
		if card.ID != "legame-dir-tier2" {
			t.Fatalf("tier2 card id=%q", card.ID)
		}
	})

	// tier 3：cwd=伪仓库根下的 coord/levels（env unset + home 无 levels）。
	t.Run("tier3 repo-relative load (dev fallback)", func(t *testing.T) {
		fakeRepo := t.TempDir()
		eduJamCopyFixtureCardAs(t, filepath.Join(fakeRepo, "coord", "levels"), "legame-dir-tier3")
		bareHome := t.TempDir()
		if err := os.MkdirAll(filepath.Join(bareHome, ".vit"), 0o755); err != nil {
			t.Fatalf("mkdir bare .vit: %v", err)
		}
		eduJamScopeDirEnv(t, "", bareHome, fakeRepo)
		card, err := loadEduJamLevelCard("legame-dir-tier3")
		if err != nil {
			t.Fatalf("tier3 load: %v", err)
		}
		if card.ID != "legame-dir-tier3" {
			t.Fatalf("tier3 card id=%q", card.ID)
		}
	})

	// 三级全缺（env unset + home 无 levels + cwd 无 coord/levels）：enter 走
	// fail-closed 产品化文案——含「乐局内容未安装」指引、不含 env 名与内部路径。
	t.Run("all tiers missing -> enter fail-closed productized", func(t *testing.T) {
		fakeHome := t.TempDir()
		if err := os.MkdirAll(filepath.Join(fakeHome, ".vit"), 0o755); err != nil {
			t.Fatalf("mkdir bare .vit: %v", err)
		}
		eduJamScopeDirEnv(t, "", fakeHome, t.TempDir())
		// 同进程早先测例可能已把 repo 夹具的 first-run-twinkle 载入进程内卡缓存；
		// 本测要证的是磁盘三级全缺，先清缓存让装载真实落盘（缓存是懒优化，
		// 删除不影响其它测例——后续用到会按当前解析目录重读）。
		eduJamCardCache.Delete(eduJamDefaultLevelID)
		server := newChatServerForTest(t, nil, nil, nil)
		resp, handled := server.eduJamEnter(context.Background(), "conv-legame-dir-none", "chat", "")
		if !handled || resp.StopReason != "edu_jam_level_card_unavailable" {
			t.Fatalf("enter must fail closed: handled=%v stop=%s", handled, resp.StopReason)
		}
		if !strings.Contains(resp.Reply, "乐局内容未安装") {
			t.Fatalf("reply must carry the productized not-installed wording: %s", resp.Reply)
		}
		if strings.Contains(resp.Reply, "VIT_EDU_LEVELS_DIR") || strings.Contains(resp.Reply, "level.yaml") {
			t.Fatalf("reply must not leak env names or internal paths: %s", resp.Reply)
		}
		if resp.Error == "" {
			t.Fatalf("machine-readable error detail must stay on base.Error for the placement-leg debug face")
		}
	})
}
