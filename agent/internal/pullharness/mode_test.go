package pullharness

// mode_test.go — flag 面四态验收（卡面验收标准 1：env 赢/配置键/缺省
// push/BOM 文件）+ fail-visible 附加态。配置路径经 VIT_DAW_AGENT_ROOT
// 重定向到 t.TempDir()（测试不写源码树，AGENTS §5/§10）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func modeTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("VIT_DAW_AGENT_ROOT", root)
	return root
}

func writeModeConfig(t *testing.T, root string, content []byte) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(harnessModeConfigRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func modeConfigJSON(t *testing.T, mode string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]string{HarnessModeConfigKey: mode})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// 态 1：env 赢——env 与配置键同时在场时 env 决定，配置值被压过。
func TestHarnessModeEnvWins(t *testing.T) {
	root := modeTestRoot(t)
	writeModeConfig(t, root, modeConfigJSON(t, ModePush))
	t.Setenv(HarnessModeEnvVar, ModePull)

	resolved := ResolveHarnessMode()
	if resolved.Mode != ModePull {
		t.Fatalf("env pull must win over config push, got %q", resolved.Mode)
	}
	if !strings.HasPrefix(resolved.Source, "environment:") {
		t.Fatalf("source must record environment surface, got %q", resolved.Source)
	}

	t.Setenv(HarnessModeEnvVar, ModePush)
	writeModeConfig(t, root, modeConfigJSON(t, ModePull))
	if resolved := ResolveHarnessMode(); resolved.Mode != ModePush {
		t.Fatalf("env push must win over config pull, got %q", resolved.Mode)
	}
}

// 态 2：配置键——env 未设时配置键决定。
func TestHarnessModeConfigKey(t *testing.T) {
	root := modeTestRoot(t)
	path := writeModeConfig(t, root, modeConfigJSON(t, ModePull))
	t.Setenv(HarnessModeEnvVar, "")

	resolved := ResolveHarnessMode()
	if resolved.Mode != ModePull {
		t.Fatalf("config key pull must resolve, got %q", resolved.Mode)
	}
	if resolved.Source != "config:"+path {
		t.Fatalf("source must record config path, got %q", resolved.Source)
	}
}

// 态 3：缺省 push——env 未设、文件缺席（以及文件在但无键）都回落 push。
func TestHarnessModeDefaultPush(t *testing.T) {
	root := modeTestRoot(t)
	t.Setenv(HarnessModeEnvVar, "")

	resolved := ResolveHarnessMode()
	if resolved.Mode != ModePush {
		t.Fatalf("absent surfaces must default to push, got %q", resolved.Mode)
	}
	if resolved.Source != "default:absent" {
		t.Fatalf("absent source mismatch: %q", resolved.Source)
	}

	// 文件在但无键：仍 push（新键缺省语义，§11 兼容义务），来源记 config 面。
	writeModeConfig(t, root, []byte(`{"audition_blind":true}`))
	if resolved := ResolveHarnessMode(); resolved.Mode != ModePush {
		t.Fatalf("config without harness_mode key must default to push, got %q", resolved.Mode)
	}
}

// 态 4：BOM 文件——UTF-8 BOM 前缀不毁解析（CONFIG-BOM-1 教训：仅剥前缀，
// 其余解析失败保持 fail-visible）。
func TestHarnessModeBOMTolerated(t *testing.T) {
	root := modeTestRoot(t)
	writeModeConfig(t, root, append([]byte{0xEF, 0xBB, 0xBF}, modeConfigJSON(t, ModePull)...))
	t.Setenv(HarnessModeEnvVar, "")

	resolved := ResolveHarnessMode()
	if resolved.Mode != ModePull {
		t.Fatalf("BOM-prefixed config must parse to pull, got %q (source %q)", resolved.Mode, resolved.Source)
	}
	if strings.Contains(resolved.Source, "config_invalid") {
		t.Fatalf("BOM must not surface as config_invalid, got %q", resolved.Source)
	}
}

// fail-visible 附加态：无效 env 值/坏 JSON/非法值类型都显式留痕并回落 push
// （不静默吞掉，沿 blind 配置面先例）。
func TestHarnessModeFailVisible(t *testing.T) {
	root := modeTestRoot(t)
	t.Setenv(HarnessModeEnvVar, "banana")
	writeModeConfig(t, root, modeConfigJSON(t, ModePull))
	resolved := ResolveHarnessMode()
	if resolved.Mode != ModePush || !strings.HasPrefix(resolved.Source, "environment_invalid:") {
		t.Fatalf("invalid env must fall back to push with visible source, got %+v", resolved)
	}

	t.Setenv(HarnessModeEnvVar, "")
	writeModeConfig(t, root, []byte("{not json"))
	resolved = ResolveHarnessMode()
	if resolved.Mode != ModePush || !strings.HasPrefix(resolved.Source, "config_invalid:") {
		t.Fatalf("invalid JSON must be config_visible, got %+v", resolved)
	}

	writeModeConfig(t, root, []byte(`{"harness_mode":true}`))
	resolved = ResolveHarnessMode()
	if resolved.Mode != ModePush || !strings.HasPrefix(resolved.Source, "config_invalid:") {
		t.Fatalf("non-string value must be config_invalid, got %+v", resolved)
	}

	writeModeConfig(t, root, []byte(`{"harness_mode":"sideways"}`))
	resolved = ResolveHarnessMode()
	if resolved.Mode != ModePush || !strings.HasPrefix(resolved.Source, "config_invalid:") {
		t.Fatalf("unknown mode value must be config_invalid, got %+v", resolved)
	}

	// 大小写/空白宽容。
	writeModeConfig(t, root, []byte(`{"harness_mode":"  PULL "}`))
	if resolved := ResolveHarnessMode(); resolved.Mode != ModePull {
		t.Fatalf("case/space tolerant parse expected pull, got %q", resolved.Mode)
	}
}
