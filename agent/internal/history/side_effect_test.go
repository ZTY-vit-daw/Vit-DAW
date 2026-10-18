package history

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// sideEffectDraftEntries returns the sorted entry names of one directory.
func sideEffectDraftEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read dir %s: %v", dir, err)
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	sort.Strings(out)
	return out
}

// sideEffectTreeSnapshot captures every path, size, mtime and content hash under
// root so a read-path call can be proven side-effect free (not just
// directory-count free).
func sideEffectTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
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
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func sideEffectRequireSameTree(t *testing.T, label, root string, before map[string]string) {
	t.Helper()
	after := sideEffectTreeSnapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("%s changed the draft root tree: before=%v after=%v", label, sideEffectKeys(before), sideEffectKeys(after))
	}
	for key, want := range before {
		if got, ok := after[key]; !ok || got != want {
			t.Fatalf("%s changed %s: before=%q after=%q", label, key, want, after[key])
		}
	}
}

func sideEffectKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// pauseForDirectoryTimestampSettle gives NTFS a beat to fold pending directory
// timestamp updates from fixture writes so mtime comparisons measure the read
// path under test rather than fixture noise.
func pauseForDirectoryTimestampSettle() {
	time.Sleep(50 * time.Millisecond)
}

// freezeFixtureTimestamps pins every fixture file/dir mtime to one fixed instant
// so a read-path assertion can compare directory mtimes deterministically:
// Windows/NTFS lazily folds directory timestamp updates, which otherwise makes
// two consecutive snapshots of an untouched fixture differ by sub-millisecond
// amounts.
func freezeFixtureTimestamps(t *testing.T, root string) time.Time {
	t.Helper()
	frozen := time.Unix(1730000000, 0).UTC()
	var dirs []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk fixture %s: %v", root, err)
	}
	for _, dir := range dirs {
		_ = os.Chtimes(dir, frozen, frozen)
	}
	// Re-touch files after their directories so the directory mtime stays
	// pinned last (child timestamp updates can otherwise bubble up again).
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		_ = os.Chtimes(path, frozen, frozen)
		return nil
	}); err != nil {
		t.Fatalf("walk fixture files %s: %v", root, err)
	}
	for _, dir := range dirs {
		_ = os.Chtimes(dir, frozen, frozen)
	}
	return frozen
}

// ① 读路径（identity probe / autonomous reader faces）不得新建 draft。
func TestReadHistoryFacesDoNotMaterializeDefaultDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	assertNoDraftEntries := func(label string) {
		t.Helper()
		if entries := sideEffectDraftEntries(t, root); len(entries) != 0 {
			t.Fatalf("%s materialized draft dirs: %v", label, entries)
		}
	}

	status, err := Status(map[string]any{})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["draft_pending"] != true || status["available"] != false {
		t.Fatalf("status should disclose draft_pending/unavailable, got %+v", status)
	}
	assertNoDraftEntries("Status")

	list, err := List(map[string]any{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list["draft_pending"] != true {
		t.Fatalf("list should disclose draft_pending, got %+v", list)
	}
	assertNoDraftEntries("List")

	worktrees, err := WorktreeList(map[string]any{})
	if err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	if worktrees["draft_pending"] != true {
		t.Fatalf("worktree list should disclose draft_pending, got %+v", worktrees)
	}
	assertNoDraftEntries("WorktreeList")

	if cached := CurrentDraftProjectPath(); cached != "" {
		t.Fatalf("read faces left a cached draft path behind: %q", cached)
	}
}

// ② reader 路径既不得破坏 pristine draft，也不得改其内容/mtime。
func TestReadHistoryFacesKeepPristineDefaultDraft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	draftPath := BindProjectIdentity("", "vitproj_reader_probe")
	if draftPath == "" {
		t.Fatal("identity probe did not resolve a draft path")
	}
	if entries := sideEffectDraftEntries(t, filepath.Dir(draftPath)); len(entries) != 1 || entries[0] != draftProjectFileName {
		t.Fatalf("pristine draft entries = %v, want [%s]", entries, draftProjectFileName)
	}
	pauseForDirectoryTimestampSettle()
	freezeFixtureTimestamps(t, root)
	before := sideEffectTreeSnapshot(t, root)

	if _, err := Status(map[string]any{}); err != nil {
		t.Fatalf("status: %v", err)
	}
	if _, err := List(map[string]any{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := WorktreeList(map[string]any{}); err != nil {
		t.Fatalf("worktree list: %v", err)
	}

	sideEffectRequireSameTree(t, "read history faces", root, before)
}

// D/E 场景（真栈 118 复验）：reader 抢跑后 pristine 仍保持，随后 ProjectNew
// 复用该 draft（118 第二道防线不回退），新工程历史只落新 uuid。
func TestProjectNewReusesPristineDraftAfterReaderFaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	const previousUUID = "vitproj_reader_previous"
	draftPath := BindProjectIdentity("", previousUUID)
	pauseForDirectoryTimestampSettle()
	freezeFixtureTimestamps(t, root)
	before := sideEffectTreeSnapshot(t, root)
	if _, err := Status(map[string]any{}); err != nil {
		t.Fatalf("status: %v", err)
	}
	if _, err := List(map[string]any{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := WorktreeList(map[string]any{}); err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	sideEffectRequireSameTree(t, "reader faces", root, before)

	created, err := ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("project new: %v", err)
	}
	if created["reused_default_draft"] != true {
		t.Fatalf("ProjectNew should reuse the pristine reader-phase draft, got %+v", created)
	}
	if dirs := draftDirectoriesForTest(t, root); len(dirs) != 1 {
		t.Fatalf("draft dirs after reader+project new = %v, want 1", dirs)
	}

	const newUUID = "vitproj_reader_new"
	BindProjectIdentity(draftPath, newUUID)
	if _, err := EnsureWorkingSession(draftPath, newUUID); err != nil {
		t.Fatalf("ensure working session: %v", err)
	}
	repo, err := Open(draftPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if repo.ProjectUUID != newUUID {
		t.Fatalf("project uuid = %q, want %q", repo.ProjectUUID, newUUID)
	}
	if dirExists(filepath.Join(filepath.Dir(draftPath), DirName, previousUUID)) {
		t.Fatalf("stale previous uuid history leaked into the reused draft")
	}
}

// ③ 泄漏守卫：active workspace 绑定不得被旧 uuid 重绑；拒绝必须可观测。
func TestBindProjectIdentityRefusesStaleRebindOfActiveWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	const activeUUID = "vitproj_bind_active"
	const staleUUID = "vitproj_bind_stale"
	draftPath := BindProjectIdentity("", activeUUID)
	if _, err := EnsureWorkingSession(draftPath, activeUUID); err != nil {
		t.Fatalf("ensure working session: %v", err)
	}

	// Order 1: active binding exists -> stale uuid is refused.
	returned := BindProjectIdentity(draftPath, staleUUID)
	if !sameProjectPath(returned, draftPath) {
		t.Fatalf("BindProjectIdentity returned %q, want %q", returned, draftPath)
	}
	if got := boundProjectIdentity(draftPath); got != activeUUID {
		t.Fatalf("binding after stale rebind = %q, want %q", got, activeUUID)
	}
	repo, err := Open(draftPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if repo.ProjectUUID != activeUUID {
		t.Fatalf("repo uuid after stale rebind = %q, want %q", repo.ProjectUUID, activeUUID)
	}
	if dirExists(filepath.Join(filepath.Dir(draftPath), DirName, staleUUID)) {
		t.Fatalf("stale uuid workspace materialized on disk")
	}
	rejections := ProjectIdentityRebindRejections()
	if len(rejections) == 0 {
		t.Fatal("stale rebind was refused but not recorded (silent guard)")
	}
	last := rejections[len(rejections)-1]
	if last.ActiveProjectUUID != activeUUID || last.RejectedProjectUUID != staleUUID {
		t.Fatalf("rejection record = %+v", last)
	}

	// Order 2 (round2 race order): the new uuid binds first (no active
	// workspace yet), the session activates it, then the stale uuid arrives.
	resetDraftProjectStateForTest()
	secondPath := BindProjectIdentity("", activeUUID)
	if _, err := EnsureWorkingSession(secondPath, activeUUID); err != nil {
		t.Fatalf("ensure second working session: %v", err)
	}
	BindProjectIdentity(secondPath, staleUUID)
	if got := boundProjectIdentity(secondPath); got != activeUUID {
		t.Fatalf("second path binding = %q, want %q", got, activeUUID)
	}
}

// ③c 并发旧 uuid 重绑：并发调用下 active 绑定保持，旧 uuid 不得落盘。
func TestBindProjectIdentityConcurrentStaleRebindKeepsActiveBinding(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	const activeUUID = "vitproj_conc_active"
	const staleUUID = "vitproj_conc_stale"
	draftPath := BindProjectIdentity("", activeUUID)
	if _, err := EnsureWorkingSession(draftPath, activeUUID); err != nil {
		t.Fatalf("ensure working session: %v", err)
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			if index%2 == 0 {
				BindProjectIdentity(draftPath, staleUUID)
				return
			}
			BindProjectIdentity(draftPath, activeUUID)
		}(i)
	}
	wg.Wait()

	if got := boundProjectIdentity(draftPath); got != activeUUID {
		t.Fatalf("concurrent stale rebinds changed the active binding: %q", got)
	}
	if dirExists(filepath.Join(filepath.Dir(draftPath), DirName, staleUUID)) {
		t.Fatalf("concurrent stale rebinds materialized the stale uuid workspace")
	}
	if len(ProjectIdentityRebindRejections()) == 0 {
		t.Fatal("concurrent stale rebinds were refused but not recorded")
	}
}

// ③b 无 active workspace 的纯 uuid 绑定仍允许身份切换（不误伤正常流程）。
func TestBindProjectIdentityAllowsUuidChangeWithoutActiveWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	draftPath := BindProjectIdentity("", "vitproj_plain_a")
	BindProjectIdentity(draftPath, "vitproj_plain_b")
	if got := boundProjectIdentity(draftPath); got != "vitproj_plain_b" {
		t.Fatalf("plain identity change binding = %q, want vitproj_plain_b", got)
	}
}

// ④ 写路径仍显式建立 root worktree item。
func TestWritePathsKeepRootWorktreeMaterialization(t *testing.T) {
	root := filepath.Join(t.TempDir(), "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	created, err := ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("project new: %v", err)
	}
	draftPath := fmt.Sprint(created["project_path"])
	if draftPath == "" || draftPath == "<nil>" {
		t.Fatalf("project new returned no draft path: %+v", created)
	}
	unboundStateDir := filepath.Join(filepath.Dir(draftPath), DirName, "projects", projectKey(draftPath))
	if !fileExists(filepath.Join(unboundStateDir, "worktrees", rootWorktreeName, ".vit_worktree.json")) {
		t.Fatalf("ProjectNew did not materialize the legacy root worktree item under %s", unboundStateDir)
	}

	projectDir := t.TempDir()
	projectPath := filepath.Join(projectDir, "song.vit")
	if err := os.WriteFile(projectPath, []byte("<EDIT/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_write_probe"
	BindProjectIdentity(projectPath, projectUUID)
	checkpoint, err := Checkpoint(map[string]any{
		"project_path": projectPath, "message": "seed", "project_snapshot_xml": "<EDIT/>",
	})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commitID := fmt.Sprint(checkpoint["commit_id"])
	if commitID == "" || commitID == "<nil>" {
		t.Fatalf("checkpoint missing commit id: %+v", checkpoint)
	}
	rootItem := filepath.Join(projectDir, DirName, projectUUID, "state", "worktrees", rootWorktreeName, ".vit_worktree.json")
	if !fileExists(rootItem) {
		t.Fatalf("Checkpoint did not materialize root worktree item %s", rootItem)
	}

	worktree, err := WorktreeCreate(map[string]any{
		"project_path": projectPath, "name": "probe_wt", "commit_id": commitID,
	})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	if fmt.Sprint(worktree["name"]) != "probe_wt" {
		t.Fatalf("worktree create result = %+v", worktree)
	}
	if !fileExists(rootItem) {
		t.Fatalf("WorktreeCreate removed the root worktree item")
	}
}

// ⑤ root-only 恢复扫描：只读、无需 reference draft、不创建任何目录。
func TestRecoverableProjectPathForUUIDInRootIsReadOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ProjectHistory", "drafts")
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	resetDraftProjectStateForTest()

	const projectUUID = "vitproj_root_probe"
	seedDir := filepath.Join(root, "draft_20261001T000000_seed")
	seedPath := filepath.Join(seedDir, draftProjectFileName)
	statePath := filepath.Join(seedDir, DirName, ".sessions", projectUUID, "session_seed", "workspace", "state", agentRuntimeStateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedPath, []byte("<EDIT draft=\"true\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtimeState := `{"schema_version":"vit_project_agent_runtime.v2","project_path":"` + strings.ReplaceAll(seedPath, `\`, `\\`) + `","project_uuid":"` + projectUUID + `","saved_at":"2026-10-01T00:00:00Z"}`
	if err := os.WriteFile(statePath, []byte(runtimeState), 0o644); err != nil {
		t.Fatal(err)
	}
	// Let NTFS settle + pin fixture timestamps so the baseline comparison
	// measures the read path, not delayed directory timestamp folding.
	pauseForDirectoryTimestampSettle()
	freezeFixtureTimestamps(t, root)
	before := sideEffectTreeSnapshot(t, root)
	// Baseline stability: two consecutive snapshots of the untouched tree must
	// be identical; a mismatch here is fixture noise, not a read-path write.
	sideEffectRequireSameTree(t, "fixture baseline", root, before)

	got := RecoverableProjectPathForUUIDInRoot(projectUUID)
	if !sameProjectPath(got, seedPath) {
		t.Fatalf("RecoverableProjectPathForUUIDInRoot = %q, want %q", got, seedPath)
	}
	sideEffectRequireSameTree(t, "root recovery scan", root, before)
	if cached := CurrentDraftProjectPath(); cached != "" {
		t.Fatalf("root recovery scan materialized a draft path: %q", cached)
	}
}
