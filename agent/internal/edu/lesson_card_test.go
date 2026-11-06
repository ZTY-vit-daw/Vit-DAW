package edu

// 校验门单测（乐局 3a）：以 279 定稿乐局卡 coord/levels/first-run-twinkle/
// level.yaml 为唯一测试夹具（279 回执⑬消解：无需另行物化）。
//
// 阳性=夹具原样通过（互测验收）；阴性=夹具通用 map 化后逐项定向突变再
// yaml.Marshal 回流，断言拒绝原因含路径标记（12 条拒绝路径 + 6 新增对齐项
// 逐项独立断言）。突变载体先做无突变对照（突变台自身正确性）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fixtureLevelPath 是 279 定稿乐局卡（仓库内只读夹具；变更面见卡面 279）。
func fixtureLevelPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "coord", "levels", "first-run-twinkle", "level.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture level.yaml not found: %v", err)
	}
	return p
}

func loadFixtureLevel(t *testing.T) *LevelCard {
	t.Helper()
	card, err := LoadLevelCard(fixtureLevelPath(t))
	if err != nil {
		t.Fatalf("LoadLevelCard(fixture) = %v, want pass（279 互测验收面）", err)
	}
	return card
}

// mutatedLevel 把定稿夹具按 mutate 定向突变后重新序列化为 YAML 字节。
func mutatedLevel(t *testing.T, mutate func(m map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixtureLevelPath(t))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("fixture yaml unmarshal: %v", err)
	}
	mutate(m)
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("mutated yaml marshal: %v", err)
	}
	return out
}

// rejectWith 断言突变夹具被校验门拒绝且原因含 marker。
func rejectWith(t *testing.T, name, marker string, mutate func(m map[string]any)) {
	t.Helper()
	_, err := ParseLevelCard(mutatedLevel(t, mutate))
	if err == nil {
		t.Fatalf("[%s] 突变未被拒绝（want 拒绝原因含 %q）", name, marker)
	}
	if !strings.Contains(err.Error(), marker) {
		t.Fatalf("[%s] 拒绝原因未含 %q:\n%v", name, marker, err)
	}
}

// nodeMap 返回 id→节点映射（突变定位用）。
func nodeMap(m map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, n := range m["nodes"].([]any) {
		nm := n.(map[string]any)
		out[nm["id"].(string)] = nm
	}
	return out
}

func hookOf(t *testing.T, m map[string]any, nodeID, hookID string) map[string]any {
	t.Helper()
	for _, n := range m["nodes"].([]any) {
		nm := n.(map[string]any)
		if nm["id"] != nodeID {
			continue
		}
		teaching, _ := nm["teaching"].(map[string]any)
		hooks, _ := teaching["evidence_hooks"].([]any)
		for _, h := range hooks {
			hm := h.(map[string]any)
			if hm["id"] == hookID {
				return hm
			}
		}
	}
	t.Fatalf("hook %s@%s not found", hookID, nodeID)
	return nil
}

// ---- 阳性：夹具原样通过（互测）+ 突变台对照 ----

func TestLevelCardFixturePassesGate(t *testing.T) {
	card := loadFixtureLevel(t)
	if card.ID != "first-run-twinkle" {
		t.Fatalf("card.ID = %q", card.ID)
	}
	if len(card.Nodes) != 7 {
		t.Fatalf("nodes = %d, want 7", len(card.Nodes))
	}
	if _, ok := LookupJudge("note_sequence_match"); !ok {
		t.Fatal("registry missing note_sequence_match")
	}
}

func TestLevelCardMutationHarnessControl(t *testing.T) {
	// 无突变回流对照：突变台（map 化+重序列化）本身不得引入拒绝。
	if _, err := ParseLevelCard(mutatedLevel(t, func(m map[string]any) {})); err != nil {
		t.Fatalf("unmutated roundtrip rejected (mutation harness broken): %v", err)
	}
}

// ---- 12 条拒绝路径逐项 ----

func TestGateRejectPath01UnknownJudge(t *testing.T) {
	rejectWith(t, "P1", "未知 judge", func(m map[string]any) {
		nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["judge"] = "chord_match"
	})
}

func TestGateRejectPath02UnknownEvent(t *testing.T) {
	rejectWith(t, "P2", "未知 event", func(m map[string]any) {
		dw := nodeMap(m)["n0_photo_score_open"]["done_when"].(map[string]any)
		dw["all"].([]any)[1].(map[string]any)["event"] = "user_typed"
	})
}

func TestGateRejectPath03UnknownStructure(t *testing.T) {
	rejectWith(t, "P3", "未知 structure", func(m map[string]any) {
		dw := nodeMap(m)["n0_photo_score_open"]["done_when"].(map[string]any)
		dw["all"].([]any)[0].(map[string]any)["structure"] = "clip_playing"
	})
}

func TestGateRejectPath04BarOutOfSpanAndUncovered(t *testing.T) {
	// 4a：range_bars 超出 bar_span。
	rejectWith(t, "P4a", "range_bars 越界", func(m map[string]any) {
		ref := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["reference"].(map[string]any)
		ref["range_bars"] = []any{12, 14}
	})
	// 4b：bar_span 扩到 13 后，窗口 [12,13] 在 span 内但不被 notes 表覆盖。
	rejectWith(t, "P4b", "不被 notes 表覆盖", func(m map[string]any) {
		m["timing"].(map[string]any)["bar_span"].(map[string]any)["last_bar"] = 13
		ref := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["reference"].(map[string]any)
		ref["range_bars"] = []any{12, 13}
	})
	// 4c：range_beats 超 bar_span 覆盖。
	rejectWith(t, "P4c", "range_beats 超出 bar_span", func(m map[string]any) {
		ref := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["reference"].(map[string]any)
		ref["range_beats"] = []any{0.0, 52.0}
	})
}

func TestGateRejectPath05RangeShapeAndMismatch(t *testing.T) {
	rejectWith(t, "P5a", "range_beats 逆序/零宽/负", func(m map[string]any) {
		tgt := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["target"].(map[string]any)
		tgt["range_beats"] = []any{48.0, 0.0}
	})
	rejectWith(t, "P5b", "range_bars 与 range_beats 不一致", func(m map[string]any) {
		ref := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["reference"].(map[string]any)
		ref["range_bars"] = []any{2, 12} // 换算 [4,48) ≠ beats [0,48)
	})
	rejectWith(t, "P5c", "range_beats 须为", func(m map[string]any) {
		tgt := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["target"].(map[string]any)
		tgt["range_beats"] = []any{0.0}
	})
}

func TestGateRejectPath06MinMatchNotes(t *testing.T) {
	rejectWith(t, "P6", "min_match_notes", func(m map[string]any) {
		hookOf(t, m, "n3_find_wrong_note", "concept_b_section")["params"].(map[string]any)["min_match_notes"] = 1
	})
}

func TestGateRejectPath07Key(t *testing.T) {
	rejectWith(t, "P7", "key.tonic", func(m map[string]any) {
		params := hookOf(t, m, "n3_find_wrong_note", "concept_b_section")["params"].(map[string]any)
		params["key"].(map[string]any)["tonic"] = 12
	})
}

func TestGateRejectPath08Transformations(t *testing.T) {
	rejectWith(t, "P8", "unknown transformation", func(m map[string]any) {
		params := hookOf(t, m, "n3_find_wrong_note", "concept_b_section")["params"].(map[string]any)
		params["transformations"] = []any{"canon"}
	})
}

func TestGateRejectPath09DuplicateNodeID(t *testing.T) {
	rejectWith(t, "P9", "node.id 重复", func(m map[string]any) {
		nodeMap(m)["n6_name_and_export"]["id"] = "n5_make_it_ours"
	})
}

func TestGateRejectPath10AssetReferences(t *testing.T) {
	rejectWith(t, "P10a", "reference.asset 引用未知资产", func(m map[string]any) {
		ref := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["reference"].(map[string]any)
		ref["asset"] = "theme9"
	})
	rejectWith(t, "P10b", "缺 sha256 或非 64", func(m map[string]any) {
		delete(m["assets"].([]any)[0].(map[string]any), "sha256")
	})
	rejectWith(t, "P10c", "preload 引用未知资产", func(m map[string]any) {
		m["runtime_slots"].(map[string]any)["melody_corrupted"].(map[string]any)["preload"] = "ghost"
	})
	rejectWith(t, "P10d", "notes_ref", func(m map[string]any) {
		m["assets"].([]any)[0].(map[string]any)["notes_ref"] = "notes.ghost"
	})
}

func TestGateRejectPath11SchemaVersion(t *testing.T) {
	rejectWith(t, "P11", "schema_version", func(m map[string]any) {
		m["schema_version"] = "vit.level_card.v1"
	})
}

func TestGateRejectPath12ObjectivesCheck(t *testing.T) {
	rejectWith(t, "P12", "引用未知 judge", func(m map[string]any) {
		m["objectives"].([]any)[2].(map[string]any)["check"].(map[string]any)["judge"] = "melody_match"
	})
}

// ---- 6 新增对齐项 ----

func TestGateAlignment01EventStructureEnums(t *testing.T) {
	// ①=路径 2/3 的枚举闭集：另测枚举本体与夹具用法在集内。
	card := loadFixtureLevel(t)
	for _, n := range card.Nodes {
		if leaf := n.DoneWhen.Leaf; leaf != nil {
			if leaf.Event != "" && !levelEventSet[leaf.Event] {
				t.Fatalf("fixture event %q outside enum", leaf.Event)
			}
			if leaf.Structure != "" && !levelStructureSet[leaf.Structure] {
				t.Fatalf("fixture structure %q outside enum", leaf.Structure)
			}
		}
	}
}

func TestGateAlignment02PitchSetSemantics(t *testing.T) {
	rejectWith(t, "②a", "set_mode 未知口径", func(m map[string]any) {
		dw := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)
		dw["judge"] = JudgePitchSetMatch
		dw["set_mode"] = "octave"
	})
	rejectWith(t, "②b", "set_mode 仅 pitch_set_match", func(m map[string]any) {
		nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["set_mode"] = "absolute"
	})
	// pitch_set_match 合法形态应通过（改 judge 后原 target/reference 仍可解）。
	if _, err := ParseLevelCard(mutatedLevel(t, func(m map[string]any) {
		dw := nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)
		dw["judge"] = JudgePitchSetMatch
		dw["set_mode"] = "absolute"
		delete(dw, "accept_transposed")
	})); err != nil {
		t.Fatalf("pitch_set_match 合法形态被拒: %v", err)
	}
}

func TestGateAlignment03AcceptTransposed(t *testing.T) {
	card := loadFixtureLevel(t)
	// 夹具两处 accept_transposed=false（n1/n3）——卡面裁决落位实证。
	count := 0
	for _, n := range card.Nodes {
		if leaf := n.DoneWhen.Leaf; leaf != nil && leaf.Judge == JudgeNoteSequenceMatch && leaf.AcceptTransposed != nil {
			count++
			if *leaf.AcceptTransposed {
				t.Fatalf("%s accept_transposed=true, fixture 定稿为 false", n.ID)
			}
		}
	}
	if count != 2 {
		t.Fatalf("accept_transposed 落位数 = %d, want 2（n1/n3）", count)
	}
	rejectWith(t, "③", "accept_transposed 仅 note_sequence_match", func(m map[string]any) {
		nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["judge"] = JudgeRhythmMatch
	})
}

func TestGateAlignment04CombinatorDepth(t *testing.T) {
	rejectWith(t, "④", "组合子嵌套超过一层", func(m map[string]any) {
		dw := nodeMap(m)["n0_photo_score_open"]["done_when"].(map[string]any)
		inner := map[string]any{"all": dw["all"]}
		dw["all"] = []any{inner}
	})
}

func TestGateAlignment05EmbeddedNotesSource(t *testing.T) {
	card := loadFixtureLevel(t)
	notes, ok := card.AssetNotes("theme")
	if !ok || len(notes) != 42 {
		t.Fatalf("AssetNotes(theme) = %d notes (ok=%v), want 42", len(notes), ok)
	}
	if notes[0].ID != "asset:theme#0" || notes[41].ID != "asset:theme#41" {
		t.Fatalf("合成 note id 形态不符: %s / %s", notes[0].ID, notes[41].ID)
	}
	// 派生资产：deviation @37 → 68 应用（279 MIDI 投影对账基线）。
	corrupted, ok := card.AssetNotes("theme_corrupted")
	if !ok || len(corrupted) != 42 {
		t.Fatalf("AssetNotes(theme_corrupted) = %d (ok=%v)", len(corrupted), ok)
	}
	devApplied := 0
	for k := range corrupted {
		if corrupted[k].ID == "asset:theme_corrupted#33" {
			if corrupted[k].Pitch != 68 || notes[k].Pitch != 69 {
				t.Fatalf("deviation 未应用/基础值不符: corrupted=%d base=%d", corrupted[k].Pitch, notes[k].Pitch)
			}
			devApplied++
		}
	}
	if devApplied != 1 {
		t.Fatalf("deviation 音定位失败")
	}
}

func TestGateAlignment06RuntimeSlots(t *testing.T) {
	rejectWith(t, "⑥a", "[⑥] runtime_slots 必填", func(m map[string]any) {
		delete(m, "runtime_slots")
	})
	rejectWith(t, "⑥b", "track_index 1 重复", func(m map[string]any) {
		m["runtime_slots"].(map[string]any)["accompaniment_blank"].(map[string]any)["track_index"] = 1
	})
	card := loadFixtureLevel(t)
	if len(card.RuntimeSlots) != 3 {
		t.Fatalf("fixture runtime_slots = %d, want 3（实例三轨）", len(card.RuntimeSlots))
	}
}

// ---- fail-closed 附加面 ----

func TestGateRejectUnknownFieldsAndParams(t *testing.T) {
	rejectWith(t, "top-unknown", "field", func(m map[string]any) {
		m["mystery_block"] = "x"
	})
	rejectWith(t, "leaf-unknown", "未知参数字段", func(m map[string]any) {
		nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["vibes"] = "groovy"
	})
	rejectWith(t, "hook-unknown-param", "未知参数字段", func(m map[string]any) {
		hookOf(t, m, "n3_find_wrong_note", "concept_b_section")["params"].(map[string]any)["groove"] = 1
	})
}

// ---- 文件级核对（LoadLevelCard；隔离副本，不写源码树）----

func copyFixtureDir(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "..", "coord", "levels", "first-run-twinkle")
	dst := filepath.Join(t.TempDir(), "first-run-twinkle")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func TestLoadLevelCardVerifiesAssetHash(t *testing.T) {
	dir := copyFixtureDir(t)
	// 篡改卡内 sha256 → 文件对账拒绝。
	levelPath := filepath.Join(dir, "level.yaml")
	raw, _ := os.ReadFile(levelPath)
	tampered := strings.Replace(string(raw),
		"276e1ffbeba0357329169aecbb8e060694ac8aace4ad09f2ba889ba97b0c8952",
		strings.Repeat("0", 64), 1)
	if tampered == string(raw) {
		t.Fatal("sha256 tamper target not found in fixture copy")
	}
	os.WriteFile(levelPath, []byte(tampered), 0o644)
	if _, err := LoadLevelCard(levelPath); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("want sha256 mismatch rejection, got %v", err)
	}
}

func TestLoadLevelCardChecksPersonaAndDirConvention(t *testing.T) {
	dir := copyFixtureDir(t)
	levelPath := filepath.Join(dir, "level.yaml")
	if err := os.Remove(filepath.Join(dir, "persona_default.md")); err != nil {
		t.Fatalf("remove persona: %v", err)
	}
	if _, err := LoadLevelCard(levelPath); err == nil || !strings.Contains(err.Error(), "persona_slot.source") {
		t.Fatalf("want persona source rejection, got %v", err)
	}
	// 目录名与卡 id 不一致 → 拒绝（内容侧惯例在加载口收口）。
	renamed := filepath.Join(t.TempDir(), "some-other-level")
	if err := os.Rename(dir, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := LoadLevelCard(filepath.Join(renamed, "level.yaml")); err == nil || !strings.Contains(err.Error(), "directory name") {
		t.Fatalf("want directory convention rejection, got %v", err)
	}
}

func TestValidateLevelCardAggregatesAllErrors(t *testing.T) {
	_, err := ParseLevelCard(mutatedLevel(t, func(m map[string]any) {
		m["schema_version"] = "v9"
		m["write_boundary"].(map[string]any)["confirm_downgrade"] = "yes"
		nodeMap(m)["n1_read_and_fix"]["done_when"].(map[string]any)["judge"] = "chord_match"
	}))
	if err == nil {
		t.Fatal("want aggregated rejection")
	}
	if !strings.Contains(err.Error(), "3 项") {
		t.Fatalf("want 3 aggregated errors, got:\n%v", err)
	}
}

// ---- 校验门与 279 自检工具的夹具互引（调试辅助，非断言面）----

func TestFixtureSummaryForReceipt(t *testing.T) {
	card := loadFixtureLevel(t)
	summary := fmt.Sprintf("id=%s nodes=%d slots=%d assets=%d judges=%v",
		card.ID, len(card.Nodes), len(card.RuntimeSlots), len(card.Assets), RegisteredJudges())
	if !strings.Contains(summary, "first-run-twinkle") {
		t.Fatal("summary sanity")
	}
}
