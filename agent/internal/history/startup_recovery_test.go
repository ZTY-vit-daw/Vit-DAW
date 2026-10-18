package history

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FIX-START-STATE-RECOVERY-1 语义边界①：不恢复——内核报告的身份（仅 uuid /
// path+uuid / 双空 / 含路径穿越的脏 uuid）一律不作为恢复依据，钩子返回空路径
// 与显式 reason，启动即新会话。
func TestStartupWorkspaceRecoveryDisabledForKernelIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	cases := []struct {
		label string
		path  string
		uuid  string
	}{
		{"kernel uuid only", "", "vitproj_kernel_startup"},
		{"kernel path and uuid", filepath.Join(root, "draft_x", draftProjectFileName), "vitproj_kernel_startup"},
		{"both empty", "", ""},
		{"traversal uuid", "", "../../escape"},
	}
	for _, tc := range cases {
		path, reason := StartupWorkspaceRecovery(tc.path, tc.uuid)
		if path != "" {
			t.Fatalf("%s: StartupWorkspaceRecovery path = %q, want empty (no recovery)", tc.label, path)
		}
		if strings.TrimSpace(reason) == "" {
			t.Fatalf("%s: StartupWorkspaceRecovery reason is empty, want explicit disabled reason", tc.label)
		}
	}
	if reason := startupWorkspaceRecoveryDisabledReason; reason != "startup_recovery_disabled_new_session" {
		t.Fatalf("disabled reason constant = %q", reason)
	}
}

// 语义边界②：兼容位——「最近工程」显式入口的未来挂点保持零 IO：即便 root 内
// 存在同 uuid 的可恢复 draft，钩子也不扫描不读取（树零变化）。扫描原语本身
// 的行为由 TestRecoverableProjectPathForUUIDInRootIsReadOnly 与
// TestRecoverableProjectPathForUUIDFindsNewestDraftRuntimeState 继续钉住。
func TestStartupWorkspaceRecoveryPerformsNoIO(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()
	const projectUUID = "vitproj_recovery_hook"
	seedDir := filepath.Join(root, "draft_20261001T000000_seedprev")
	seedPath := filepath.Join(seedDir, draftProjectFileName)
	stateDir := filepath.Join(seedDir, DirName, ".sessions", projectUUID, "session_seed", "workspace", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedPath, []byte("<EDIT draft=\"true\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := `{"schema_version":"vit_project_agent_runtime.v2","project_path":"` + strings.ReplaceAll(seedPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-10-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(stateDir, agentRuntimeStateFile), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	// Let NTFS settle + pin fixture timestamps so the baseline comparison
	// measures the hook, not delayed directory timestamp folding.
	pauseForDirectoryTimestampSettle()
	freezeFixtureTimestamps(t, root)
	before := sideEffectTreeSnapshot(t, root)

	path, reason := StartupWorkspaceRecovery("", projectUUID)

	if path != "" || reason != startupWorkspaceRecoveryDisabledReason {
		t.Fatalf("StartupWorkspaceRecovery = (%q,%q), want (%q,%q)", path, reason, "", startupWorkspaceRecoveryDisabledReason)
	}
	sideEffectRequireSameTree(t, "startup recovery hook", root, before)
	// 兼容位自检：原语在显式调用下仍能找到种子（未来入口复用的能力仍在）。
	if got := RecoverableProjectPathForUUIDInRoot(projectUUID); !sameProjectPath(got, seedPath) {
		t.Fatalf("scan primitive lost the seed: RecoverableProjectPathForUUIDInRoot = %q, want %q", got, seedPath)
	}
}

// 语义边界③+AGENTS §11：旧记录反序列化兼容——本修复不新增持久化字段、不改
// 读写格式；v1（reopen_reset 形态）与 v2（含未来未知字段）的旧 runtime state
// 经 ReadAgentRuntimeState 原样读回，读路径不改写、不迁移、不重排。
func TestAgentRuntimeStateOldRecordsReadBackUnchanged(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "legacy-state.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_legacy_state"
	BindProjectIdentity(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentRuntimeState(projectPath, projectUUID, []byte("{}")); err != nil {
		t.Fatalf("seed runtime state location: %v", err)
	}
	var stateFile string
	err := filepath.WalkDir(filepath.Dir(projectPath), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && d.Name() == agentRuntimeStateFile {
			stateFile = path
		}
		return nil
	})
	if err != nil || stateFile == "" {
		t.Fatalf("locate %s: err=%v file=%q", agentRuntimeStateFile, err, stateFile)
	}
	legacyRecords := map[string][]byte{
		"v1 with reopen_reset":   []byte(`{"schema_version":"vit_project_agent_runtime.v1","project_path":"` + strings.ReplaceAll(projectPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-09-01T00:00:00Z","reopen_reset":{"schema_version":"vit_project_reopen_archive.v1","applied_at":"2026-09-01T00:00:00Z","archive_dir":"archive"},"durable_continuations":{"c1":{"status":"running"}}}`),
		"v2 with unknown fields": []byte("{\n  \"schema_version\": \"vit_project_agent_runtime.v2\",\n  \"project_path\": \"" + strings.ReplaceAll(projectPath, `\`, `\\`) + "\",\n  \"project_uuid\": \"" + projectUUID + "\",\n  \"saved_at\": \"2026-09-08T00:00:00Z\",\n  \"future_field_v3\": {\"nested\": [1, 2, 3]},\n  \"conversations\": {}\n}"),
	}
	for label, legacy := range legacyRecords {
		if err := os.WriteFile(stateFile, legacy, 0o600); err != nil {
			t.Fatalf("%s: seed legacy record: %v", label, err)
		}
		got, readErr := ReadAgentRuntimeState(projectPath, projectUUID)
		if readErr != nil {
			t.Fatalf("%s: ReadAgentRuntimeState: %v", label, readErr)
		}
		if !bytes.Equal(got, legacy) {
			t.Fatalf("%s: old record was rewritten on read: got %d bytes, want verbatim %d bytes", label, len(got), len(legacy))
		}
	}
}
