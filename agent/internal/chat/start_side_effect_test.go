package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
)

func startSideChannelShadow(uuid string) *shadow.Project {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status":       "ok",
		"project_uuid": uuid,
		"project_path": "",
		"tracks":       []any{},
	})
	return project
}

func draftDirsIn(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read draft root %s: %v", root, err)
	}
	out := []string{}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "draft_") {
			out = append(out, entry.Name())
		}
	}
	return out
}

// ① 激活是读/身份步骤：无 draft 时不物化 draft，也不切换激活工作区。
func TestActivateWorkspaceDoesNotMaterializeDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_activate_empty"
	server := newChatServerForTest(t, nil, startSideChannelShadow(projectUUID), nil)

	server.activateCurrentProjectWorkspace(context.Background())

	if dirs := draftDirsIn(t, root); len(dirs) != 0 {
		t.Fatalf("activation materialized draft dirs: %v", dirs)
	}
	server.workspaceMu.Lock()
	activePath := server.activeWorkspacePath
	server.workspaceMu.Unlock()
	if strings.TrimSpace(activePath) != "" {
		t.Fatalf("activation switched to %q without a draft", activePath)
	}
}

// ② 写路径已物化 draft 后，激活只读地接上该 draft，不产生第二个目录。
func TestActivateWorkspaceUsesMaterializedDraftWithoutNewDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_activate_materialized"
	created, err := history.ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("project new: %v", err)
	}
	draftPath := strings.TrimSpace(toStringFromAny(created["project_path"]))
	if draftPath == "" {
		t.Fatalf("project new returned no path: %+v", created)
	}
	server := newChatServerForTest(t, nil, startSideChannelShadow(projectUUID), nil)

	server.activateCurrentProjectWorkspace(context.Background())

	server.workspaceMu.Lock()
	activePath := server.activeWorkspacePath
	server.workspaceMu.Unlock()
	if !strings.EqualFold(filepath.Clean(activePath), filepath.Clean(draftPath)) {
		t.Fatalf("active workspace = %q, want materialized draft %q", activePath, draftPath)
	}
	if hintPath, hintUUID := server.harness.ProjectPathHint(); !strings.EqualFold(hintUUID, projectUUID) || !strings.EqualFold(filepath.Clean(hintPath), filepath.Clean(draftPath)) {
		t.Fatalf("project hint = (%q,%q), want (%q,%q)", hintPath, hintUUID, draftPath, projectUUID)
	}
	if dirs := draftDirsIn(t, root); len(dirs) != 1 {
		t.Fatalf("draft dirs after activation = %v, want 1", dirs)
	}
}

// ③ FIX-START-STATE-RECOVERY-1（用户 2026-10-18 裁定=不恢复）：内核只报 uuid
// （path 空，即内核恢复了上一工程身份但未保存工程）时，激活不按 uuid 扫描、
// 不认领 root 内同 uuid 的历史 draft——启动即新会话。种子 draft 全树零读零写
// 零删除（不恢复=不读不写，不是删除）。
func TestActivateWorkspaceDoesNotRecoverSeededDraftByKernelUUID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_activate_recover"
	seedDir := filepath.Join(root, "draft_20261001T000000_seedprev")
	seedPath := filepath.Join(seedDir, "Unsaved.vit")
	stateDir := filepath.Join(seedDir, ".vit_history", ".sessions", projectUUID, "session_seed", "workspace", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	if err := os.WriteFile(seedPath, []byte("<EDIT draft=\"true\"/>"), 0o644); err != nil {
		t.Fatalf("write seed project: %v", err)
	}
	state := `{"schema_version":"vit_project_agent_runtime.v2","project_path":"` + strings.ReplaceAll(seedPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-10-01T00:00:00Z","conversations":{"old":["session"]}}`
	if err := os.WriteFile(filepath.Join(stateDir, "agent_runtime_state.json"), []byte(state), 0o644); err != nil {
		t.Fatalf("write seed runtime state: %v", err)
	}
	before := startSideEffectTreeSnapshot(t, root)
	server := newChatServerForTest(t, nil, startSideChannelShadow(projectUUID), nil)

	server.activateCurrentProjectWorkspace(context.Background())

	server.workspaceMu.Lock()
	activePath := server.activeWorkspacePath
	server.workspaceMu.Unlock()
	if strings.TrimSpace(activePath) != "" {
		t.Fatalf("activation recovered a durable draft by kernel uuid: active workspace = %q, want none (new session)", activePath)
	}
	if hintPath, hintUUID := server.harness.ProjectPathHint(); strings.TrimSpace(hintPath) != "" || strings.TrimSpace(hintUUID) != "" {
		t.Fatalf("project hint = (%q,%q), want empty (no recovery attachment)", hintPath, hintUUID)
	}
	if dirs := draftDirsIn(t, root); len(dirs) != 1 || !strings.HasPrefix(dirs[0], "draft_20261001T000000_seedprev") {
		t.Fatalf("draft dirs after no-recovery activation = %v, want only the untouched seed", dirs)
	}
	if diff := startSideEffectTreeDiff(before, startSideEffectTreeSnapshot(t, root)); len(diff) != 0 {
		t.Fatalf("no-recovery activation touched the seeded draft tree: %v", diff)
	}
}

// ④ fail-closed：root 内存在异常态（损坏的 runtime state / 缺 Unsaved.vit 的
// draft 壳 / 名如 draft_* 的普通文件）时，激活不 panic、不恢复、不物化——
// 异常态一律按新会话收敛，不产生半恢复。
func TestActivateWorkspaceFailsClosedOnAbnormalSeededState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	const projectUUID = "vitproj_activate_abnormal"
	corruptDir := filepath.Join(root, "draft_20260901T000000_corrupt")
	corruptStateDir := filepath.Join(corruptDir, ".vit_history", ".sessions", projectUUID, "session_x", "workspace", "state")
	if err := os.MkdirAll(corruptStateDir, 0o755); err != nil {
		t.Fatalf("mkdir corrupt seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corruptStateDir, "agent_runtime_state.json"), []byte("\x00not-json{"), 0o644); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corruptDir, "Unsaved.vit"), []byte("<EDIT draft=\"true\"/>"), 0o644); err != nil {
		t.Fatalf("write corrupt seed project: %v", err)
	}
	shellDir := filepath.Join(root, "draft_20260902T000000_shell", ".vit_history", ".sessions", projectUUID, "session_y", "workspace", "state")
	if err := os.MkdirAll(shellDir, 0o755); err != nil {
		t.Fatalf("mkdir shell draft: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "draft_20260903T000000_file"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file-shaped draft entry: %v", err)
	}
	before := startSideEffectTreeSnapshot(t, root)
	server := newChatServerForTest(t, nil, startSideChannelShadow(projectUUID), nil)

	server.activateCurrentProjectWorkspace(context.Background())

	server.workspaceMu.Lock()
	activePath := server.activeWorkspacePath
	server.workspaceMu.Unlock()
	if strings.TrimSpace(activePath) != "" {
		t.Fatalf("abnormal-state activation bound a workspace: %q, want none (fail closed to new session)", activePath)
	}
	if dirs := draftDirsIn(t, root); len(dirs) != 2 {
		t.Fatalf("abnormal-state activation materialized or removed draft dirs: %v", dirs)
	}
	if diff := startSideEffectTreeDiff(before, startSideEffectTreeSnapshot(t, root)); len(diff) != 0 {
		t.Fatalf("abnormal-state activation touched the seeded tree: %v", diff)
	}
}

// startSideEffectTreeSnapshot records rel-path -> size/mtime/sha256 for every
// file under root, so a zero-recovery activation can be proven zero-touch.
func startSideEffectTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	type row struct {
		path  string
		entry string
	}
	var rows []row
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
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
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		rows = append(rows, row{path: path, entry: rel + "|" + strconv.FormatInt(info.Size(), 10) + "|" + info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000") + "|" + hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	for _, item := range rows {
		out[item.path] = item.entry
	}
	return out
}

func startSideEffectTreeDiff(before, after map[string]string) []string {
	var diff []string
	for path, entry := range before {
		other, ok := after[path]
		if !ok {
			diff = append(diff, "removed:"+path)
			continue
		}
		if other != entry {
			diff = append(diff, "changed:"+path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			diff = append(diff, "added:"+path)
		}
	}
	sort.Strings(diff)
	return diff
}

func toStringFromAny(value any) string {
	text, _ := value.(string)
	return text
}
