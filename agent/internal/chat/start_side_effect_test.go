package chat

import (
	"context"
	"os"
	"path/filepath"
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

// ③ 重启恢复：root 内已有该 uuid 的历史 draft 时，激活只读扫描认领它，
// 不新建默认 draft。
func TestActivateWorkspaceRecoversSeededDraftWithoutMaterializing(t *testing.T) {
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
	state := `{"schema_version":"vit_project_agent_runtime.v2","project_path":"` + strings.ReplaceAll(seedPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-10-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(stateDir, "agent_runtime_state.json"), []byte(state), 0o644); err != nil {
		t.Fatalf("write seed runtime state: %v", err)
	}
	server := newChatServerForTest(t, nil, startSideChannelShadow(projectUUID), nil)

	server.activateCurrentProjectWorkspace(context.Background())

	server.workspaceMu.Lock()
	activePath := server.activeWorkspacePath
	server.workspaceMu.Unlock()
	if !strings.EqualFold(filepath.Clean(activePath), filepath.Clean(seedPath)) {
		t.Fatalf("active workspace = %q, want recovered seed %q", activePath, seedPath)
	}
	if dirs := draftDirsIn(t, root); len(dirs) != 1 || !strings.HasPrefix(dirs[0], "draft_20261001T000000_seedprev") {
		t.Fatalf("draft dirs after recovery activation = %v, want only the seed", dirs)
	}
	if hintPath, hintUUID := server.harness.ProjectPathHint(); !strings.EqualFold(hintUUID, projectUUID) || !strings.EqualFold(filepath.Clean(hintPath), filepath.Clean(seedPath)) {
		t.Fatalf("project hint = (%q,%q), want (%q,%q)", hintPath, hintUUID, seedPath, projectUUID)
	}
}

func toStringFromAny(value any) string {
	text, _ := value.(string)
	return text
}
