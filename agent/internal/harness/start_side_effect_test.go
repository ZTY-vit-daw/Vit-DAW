package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

func startSideEffectShadow(uuid string) *shadow.Project {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status":       "ok",
		"project_uuid": uuid,
		"project_path": "",
		"tracks":       []any{},
	})
	return project
}

func startSideEffectTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		if d.IsDir() {
			out[key] = "dir|" + info.ModTime().UTC().String()
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		out[key] = "file|" + info.ModTime().UTC().String() + "|" + hex.EncodeToString(sum[:])
		return nil
	}); err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func startSideEffectFreeze(t *testing.T, root string) {
	t.Helper()
	frozen := time.Unix(1730000000, 0).UTC()
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr == nil && d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	for _, dir := range dirs {
		_ = os.Chtimes(dir, frozen, frozen)
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() {
			_ = os.Chtimes(path, frozen, frozen)
		}
		return nil
	})
	for _, dir := range dirs {
		_ = os.Chtimes(dir, frozen, frozen)
	}
	time.Sleep(20 * time.Millisecond)
}

func startSideEffectRequireSame(t *testing.T, label, root string, before map[string]string) {
	t.Helper()
	after := startSideEffectTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("%s changed tree shape: before=%v after=%v", label, startSideEffectKeys(before), startSideEffectKeys(after))
	}
	for key, want := range before {
		if got, ok := after[key]; !ok || got != want {
			t.Fatalf("%s changed %s: before=%q after=%q", label, key, want, after[key])
		}
	}
}

func startSideEffectKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func startSideEffectSeedPreviousDraft(t *testing.T, root, projectUUID string) string {
	t.Helper()
	seedDir := filepath.Join(root, "draft_20261001T000000_seedprev")
	seedPath := filepath.Join(seedDir, "Unsaved.vit")
	stateDir := filepath.Join(seedDir, ".vit_history", ".sessions", projectUUID, "session_seed", "workspace", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	if err := os.WriteFile(seedPath, []byte("<EDIT draft=\"true\"/>"), 0o644); err != nil {
		t.Fatalf("write seed project: %v", err)
	}
	state := `{"schema_version":"vit_project_agent_runtime.v2","project_path":"` + strings.ReplaceAll(seedPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-10-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(stateDir, "agent_runtime_state.json"), []byte(state), 0o644); err != nil {
		t.Fatalf("write seed runtime state: %v", err)
	}
	return seedPath
}

// ① identity probe 读路径不得物化 draft；summary 以 draft_pending 披露。
func TestCurrentProjectIdentityDoesNotMaterializeDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_identity_probe"
	h := New(nil, startSideEffectShadow(projectUUID), nil)

	projectPath, projectUUIDOut := h.CurrentProjectIdentity(context.Background())
	if projectPath != "" {
		t.Fatalf("CurrentProjectIdentity materialized a draft path: %q", projectPath)
	}
	if projectUUIDOut != projectUUID {
		t.Fatalf("uuid = %q, want %q", projectUUIDOut, projectUUID)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("identity probe created draft entries: %v", entries)
	}

	summary := h.ProjectHistorySummaryForProject(context.Background(), "", "")
	if summary["available"] != false || summary["draft_pending"] != true {
		t.Fatalf("summary should be unavailable/pending without materializing, got %+v", summary)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("summary read created draft entries: %v", entries)
	}
}

// ② reader 使用激活 workspace hint 读取既有 draft：不建新 draft、不改既有文件。
func TestReaderUsesWorkspaceHintWithoutMaterializing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_reader_hint"
	seedPath := startSideEffectSeedPreviousDraft(t, root, projectUUID)
	startSideEffectFreeze(t, root)
	before := startSideEffectTree(t, root)

	h := New(nil, startSideEffectShadow(projectUUID), nil)
	h.SetProjectPathHint(seedPath, projectUUID)
	summary := h.ProjectHistorySummaryForProject(context.Background(), "", "")
	if summary["available"] != true {
		t.Fatalf("hint-backed summary should be available, got %+v", summary)
	}
	if got := strings.TrimSpace(toStringValue(summary["project_path"])); !strings.EqualFold(filepath.Clean(got), filepath.Clean(seedPath)) {
		t.Fatalf("summary project_path = %q, want %q", got, seedPath)
	}
	startSideEffectRequireSame(t, "hint-backed reader", root, before)

	// A uuid-mismatched hint must not be used (no accidental cross-project read).
	h.SetProjectPathHint(seedPath, "vitproj_someone_else")
	other := h.ProjectHistorySummaryForProject(context.Background(), "", "")
	if other["draft_pending"] != true {
		t.Fatalf("mismatched hint should fall back to draft_pending, got %+v", other)
	}
	startSideEffectRequireSame(t, "mismatched hint reader", root, before)
}

// ③ 写路径（save 准备）显式绑定/物化默认 draft——调用方显式绑定语义。
func TestSavePrepareMaterializesDraftExplicitly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_save_prepare"
	h := New(nil, startSideEffectShadow(projectUUID), nil)

	cmd := map[string]any{}
	spec := tools.CommandSpec{CommandName: "save_project"}
	if err := h.prepareProjectLifecycleCommand(spec, cmd); err != nil {
		t.Fatalf("prepareProjectLifecycleCommand: %v", err)
	}
	if got := strings.TrimSpace(toStringValue(cmd["source_project_path"])); got == "" {
		t.Fatalf("write-intent save prepare did not bind a project path: %+v", cmd)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("write-intent save prepare draft entries = %v (err=%v), want 1", entries, err)
	}
}

// ④ 118 复用不回退：ProjectNew 之后的 reader 读路径不新增 draft，且能读到
// 已物化 draft 的 summary（draft path 来自只读缓存）。
func TestReaderAfterProjectNewDoesNotAddDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_after_new"
	h := New(nil, startSideEffectShadow(projectUUID), nil)

	created, err := history.ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("project new: %v", err)
	}
	draftPath := strings.TrimSpace(toStringValue(created["project_path"]))
	if draftPath == "" {
		t.Fatalf("ProjectNew returned no path: %+v", created)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("draft entries after ProjectNew = %v, want 1", entries)
	}

	// No hint yet: the reader still resolves the cached draft path read-only.
	summary := h.ProjectHistorySummaryForProject(context.Background(), "", "")
	if summary["available"] != true {
		t.Fatalf("cached-draft summary should be available, got %+v", summary)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("reader after ProjectNew added draft entries: %v", entries)
	}

	// With the activated hint it keeps reading the same single draft.
	h.SetProjectPathHint(draftPath, projectUUID)
	_ = h.ProjectHistorySummaryForProject(context.Background(), "", "")
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("hint reader added draft entries: %v", entries)
	}
}

// ①b 自主 reader 实际走的工具调用面（version.list / version.worktree_list）也
// 不得物化 draft；回包保持形状并披露 draft_pending。
func TestReaderToolInvokesDoNotMaterializeDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_reader_tool"
	h := New(nil, startSideEffectShadow(projectUUID), nil)

	for _, tool := range []string{"version.list", "version.worktree_list"} {
		resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: tool, Confirmed: true})
		if err != nil {
			t.Fatalf("%s invoke: %v", tool, err)
		}
		if resp.Status != "ok" {
			t.Fatalf("%s status = %q", tool, resp.Status)
		}
		result := resp.Result
		if result == nil || result["draft_pending"] != true || result["available"] != false {
			t.Fatalf("%s result should disclose draft_pending/unavailable, got %+v", tool, result)
		}
		if entries, _ := os.ReadDir(root); len(entries) != 0 {
			t.Fatalf("%s materialized draft entries: %v", tool, entries)
		}
	}
}

func toStringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
