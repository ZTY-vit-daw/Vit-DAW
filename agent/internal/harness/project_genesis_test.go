package harness

// project_genesis_test.go — L4-GENESIS-1 接线单测：genesis 头部段一次性入账
// （幂等形态）、fail-open 形态（影子不可读/身份不可证/账本损坏均不阻塞、
// 警告可见）、事实渲染确定性。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/shadow"
)

func genesisTestShadow(t *testing.T, projectDir string) *shadow.Project {
	t.Helper()
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectDir,
		"tracks": []any{
			map[string]any{
				"track_id": "t1", "track_name": "Drums", "track_type": "audio", "is_audio_track": true,
				"clips": []any{map[string]any{"id": "clip_a", "name": "Loop A", "start_seconds": 0.0, "length_seconds": 2.0}},
			},
			map[string]any{
				"track_id": "t2", "track_name": "Bass", "track_type": "audio", "is_audio_track": true,
				"parent_track_id": "b1",
				"clips":           []any{map[string]any{"id": "clip_b", "name": "Loop B", "start_seconds": 4.0, "length_seconds": 2.0}},
			},
			map[string]any{"track_id": "b1", "track_name": "Drum Bus", "vit_type": "bus"},
		},
	})
	return project
}

func TestProjectLedgerGenesisAppendsOnceThenIdempotent(t *testing.T) {
	// 真栈路径形态：project_path 是 .vit 文件，sidecar/账本落其父目录
	//（genesisLedgerDir 解析，对齐 projectstore.Resolve 的 ProjectDir 语义）。
	projectDir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectFile := filepath.Join(projectDir, "genesis_fixture.vit")
	if err := os.WriteFile(projectFile, []byte("kernel project file placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, genesisTestShadow(t, projectFile), nil)

	result := map[string]any{}
	h.appendProjectLedgerGenesis(projectFile, result)

	entries, err := carriers.ReadLedger(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("genesis appended no entries: result=%+v", result)
	}
	for _, entry := range entries {
		if entry.Kind != carriers.LedgerKindTopologyDelta || entry.Phase != "genesis" {
			t.Fatalf("entry #%d kind=%q phase=%q, want topology_delta/genesis", entry.EntryID, entry.Kind, entry.Phase)
		}
	}
	joined := genesisEntryStatements(entries)
	if !strings.Contains(joined, "tom_overview:") {
		t.Fatalf("TOM overview line missing: %s", joined)
	}
	if !strings.Contains(joined, "Drums") || !strings.Contains(joined, "Bass") {
		t.Fatalf("track summary lines missing: %s", joined)
	}
	if !strings.Contains(joined, "bus_topology:") || !strings.Contains(joined, "Bass→Drum Bus") {
		t.Fatalf("bus topology line missing or wrong: %s", joined)
	}
	if !strings.Contains(joined, "delivery_targets:") || !strings.Contains(joined, "builtin:spotify") {
		t.Fatalf("delivery target lines missing: %s", joined)
	}

	before, err := os.ReadFile(filepath.Join(projectDir, carriers.LedgerRelPath))
	if err != nil {
		t.Fatal(err)
	}

	// 同工程再打开：AppendGenesis 幂等跳过，账本字节不变。
	secondResult := map[string]any{}
	h.appendProjectLedgerGenesis(projectFile, secondResult)
	after, err := os.ReadFile(filepath.Join(projectDir, carriers.LedgerRelPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("re-open mutated ledger:\nbefore=%s\nafter=%s", before, after)
	}
	entries2, err := carriers.ReadLedger(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries2) != len(entries) {
		t.Fatalf("re-open appended entries: %d -> %d", len(entries), len(entries2))
	}
	if secondResult["project_ledger_genesis"].(map[string]any)["appended"] != 0 {
		t.Fatalf("re-open genesis result not zero-appended: %+v", secondResult)
	}
}

func TestProjectLedgerGenesisFailOpenWithoutReadableProjection(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 形态 1：shadow 不可用（无投影面可读）——不报错、不落盘、警告可见。
	h := New(nil, nil, nil)
	result := map[string]any{}
	h.appendProjectLedgerGenesis(projectDir, result)
	if genesisLedgerExists(projectDir) {
		t.Fatalf("ledger written despite unavailable shadow")
	}
	if warning := genesisResultWarning(result); !strings.Contains(warning, "shadow unavailable") {
		t.Fatalf("unavailable-shadow warning missing: %+v", result)
	}

	// 形态 2：影子快照身份不可证（串工程防护）——跳过不写。
	foreign := New(nil, genesisTestShadow(t, filepath.Join(t.TempDir(), "other")), nil)
	resultForeign := map[string]any{}
	foreign.appendProjectLedgerGenesis(projectDir, resultForeign)
	if genesisLedgerExists(projectDir) {
		t.Fatalf("ledger written from foreign shadow identity")
	}
	if warning := genesisResultWarning(resultForeign); !strings.Contains(warning, "identity") {
		t.Fatalf("identity-mismatch warning missing: %+v", resultForeign)
	}

	// 形态 3：空状态（无轨道、无身份字段）——同样 fail-open。
	empty := shadow.New(nil)
	empty.Initialize(map[string]any{"status": "ok"})
	hEmpty := New(nil, empty, nil)
	resultEmpty := map[string]any{}
	hEmpty.appendProjectLedgerGenesis(projectDir, resultEmpty)
	if genesisLedgerExists(projectDir) {
		t.Fatalf("ledger written from empty shadow state")
	}
}

func TestProjectLedgerGenesisFailOpenOnCorruptLedger(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(projectDir, "ledger"), 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := `{"entry_id":1,"kind":"decision","statement":"x"}` + "\n" + "not-json\n"
	if err := os.WriteFile(filepath.Join(projectDir, carriers.LedgerRelPath), []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, genesisTestShadow(t, projectDir), nil)

	result := map[string]any{}
	h.appendProjectLedgerGenesis(projectDir, result) // 不得 panic、不得报错阻塞

	if warning := genesisResultWarning(result); !strings.Contains(warning, "not appended") {
		t.Fatalf("corrupt-ledger warning missing: %+v", result)
	}
	// 账本只追加：损坏前缀不得被改写。
	raw, err := os.ReadFile(filepath.Join(projectDir, carriers.LedgerRelPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != corrupt {
		t.Fatalf("corrupt ledger prefix rewritten: %s", raw)
	}
}

func TestGenesisBusTopologyRendering(t *testing.T) {
	engine := map[string]any{"tracks": []any{
		map[string]any{"track_id": "t1", "track_name": "Vox", "parent_track_id": "b1"},
		map[string]any{"track_id": "t2", "track_name": "Bass"},
		map[string]any{"track_id": "b1", "track_name": "Mix Bus", "is_submix_folder": true},
		map[string]any{"track_id": "t3", "track_name": "Orphan", "parent_track_id": "missing"},
	}}
	topology := genesisBusTopology(engine)
	for _, want := range []string{"tracks=4", "Vox→Mix Bus", "bus/submix: Mix Bus"} {
		if !strings.Contains(topology, want) {
			t.Fatalf("topology %q missing %q", topology, want)
		}
	}
	if strings.Contains(topology, "Orphan") {
		t.Fatalf("unresolvable parent edge leaked: %q", topology)
	}
	if got := genesisBusTopology(map[string]any{"tracks": []any{}}); got != "" {
		t.Fatalf("empty tracks topology = %q, want empty", got)
	}
}

func genesisEntryStatements(entries []carriers.LedgerEntry) string {
	statements := make([]string, 0, len(entries))
	for _, entry := range entries {
		statements = append(statements, entry.Statement)
	}
	return strings.Join(statements, "\n")
}

func genesisLedgerExists(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, carriers.LedgerRelPath))
	return err == nil
}

func genesisResultWarning(result map[string]any) string {
	note, ok := result["project_ledger_genesis"].(map[string]any)
	if !ok {
		return ""
	}
	warning, _ := note["warning"].(string)
	return warning
}
