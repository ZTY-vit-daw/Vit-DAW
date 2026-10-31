package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/shadow"
)

// M3-OBS-IDENTITY-BIND-1：观察链 identity 对账四形态单测。projectstore 的
// active roots 是进程级状态——本文件全部用例 defer 复位且不并行。

const (
	reconcileSourceUUID = "vitproj_reconcile_src"
	reconcileTargetUUID = "vitproj_reconcile_dst"
	reconcileOtherUUID  = "vitproj_reconcile_other"
)

func reconcileTestHarness(t *testing.T, replies ...map[string]any) *Harness {
	t.Helper()
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok", "project_uuid": reconcileSourceUUID, "project_id": reconcileSourceUUID,
		"project_path": "seed.vit", "tracks": []any{},
	})
	h := New(nil, project, nil)
	h.kernel = &fakeKernelClient{replies: replies}
	return h
}

// seedReconcileFacts 在 source 身份名下落一份 bake 事实（L3 log + acoustic
// store 的 l1/l3 行），复刻 230 run 的分裂前提：事实在场、id 为铸造中间态。
func seedReconcileFacts(t *testing.T, projectPath string) (clipPath string, storePath string) {
	t.Helper()
	clipPath = filepath.Join(filepath.Dir(projectPath), "reconcile_clip.wav")
	if err := os.WriteFile(clipPath, []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	row := map[string]any{
		"feature_type": "loudness_summary", "status": "ready",
		"track_id": "1010", "clip_id": "1016",
		"file_path": clipPath, "duration_seconds": 6.0,
		"source_revision": "rev_reconcile", "approximate_lufs": -14.2,
	}
	if _, _, err := projectworkspace.AppendL3Feature(projectPath, reconcileSourceUUID, row); err != nil {
		t.Fatal(err)
	}
	dir, err := projectworkspace.DerivedDir(projectPath, reconcileSourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	storePath = filepath.Join(dir, "acoustic_package_status.json")
	identity := acousticpackage.Identity{
		ProjectID: reconcileSourceUUID, TrackID: "1010", ClipID: "1016",
		SourcePath: clipPath, DurationSec: 6, SourceRevision: "rev_reconcile",
	}
	snapshot := map[string]any{
		"waveform_envelope": map[string]any{
			"status": "ready", "track_id": "1010", "clip_id": "1016",
			"project_id": reconcileSourceUUID, "file_path": clipPath,
			"duration_seconds": 6.0, "source_revision": "rev_reconcile",
			"rms": 0.25, "peak_abs": 0.62, "rms_dbfs": -12.04, "peak_dbfs": -4.15,
		},
		"loudness_summaries": []any{map[string]any{
			"status": "ready", "track_id": "1010", "clip_id": "1016",
			"project_id": reconcileSourceUUID, "file_path": clipPath,
			"duration_seconds": 6.0, "source_revision": "rev_reconcile",
		}},
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := acousticpackage.BuildStatus(identity, snapshot, now, "reconcile_test")
	if _, err := acousticpackage.NewStore(storePath).Upsert(status); err != nil {
		t.Fatal(err)
	}
	return clipPath, storePath
}

// TestObservationIdentityReconcileForkFollowsSaveAs：内核 save_as 铸新 id
// （parent=当前绑定 id）→ 对账 fork 跟随 → store 绑定/事实/观察身份三面同 id，
// 事实不再被判 stale（230 缺陷的往返闭环）。
func TestObservationIdentityReconcileForkFollowsSaveAs(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")
	targetPath := filepath.Join(root, "dst.vit")
	clipPath, _ := seedReconcileFacts(t, sourcePath)

	h := reconcileTestHarness(t, map[string]any{
		"status": "ok", "project_uuid": reconcileTargetUUID, "project_id": reconcileTargetUUID,
		"project_path": targetPath, "parent_project_uuid": reconcileSourceUUID,
		"tracks": []any{},
	}) // shadow 初始=source（滞后视图）；对账刷新时内核回 save_as 后的新身份
	defer projectstore.Deactivate()
	if _, err := h.ActivateProjectStore(sourcePath, reconcileSourceUUID); err != nil {
		t.Fatal(err)
	}

	// 观察时 shadow 已见 save_as 后的新身份（parent=source）。
	state := map[string]any{
		"status": "ok", "project_uuid": reconcileTargetUUID, "project_id": reconcileTargetUUID,
		"project_path": targetPath, "parent_project_uuid": reconcileSourceUUID,
		"tracks": []any{},
	}
	state = h.reconcileProjectIdentityForObservation(context.Background(), state)

	roots, ok := projectstore.Current()
	if !ok || !strings.EqualFold(roots.ProjectUUID, reconcileTargetUUID) {
		t.Fatalf("active store = %#v, want bound to %s", roots, reconcileTargetUUID)
	}

	// 观察读取身份 = live id（identityFromMaps 路径的等价投影）。
	identity := acousticpackage.IdentityFromMaps(state, map[string]any{
		"track_id": "1010", "clip_id": "1016", "file_path": clipPath, "duration_seconds": 6.0,
	}, map[string]any{"source_revision": "rev_reconcile"})
	if !strings.EqualFold(identity.ProjectID, reconcileTargetUUID) {
		t.Fatalf("observation identity project = %q, want %s", identity.ProjectID, reconcileTargetUUID)
	}

	// fork 事实链：目标的 acoustic store 行已再绑定到 live id，Find 命中且不 stale。
	targetDir, err := projectworkspace.DerivedDir(targetPath, reconcileTargetUUID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(targetDir, "acoustic_package_status.json"))
	if err != nil {
		t.Fatalf("forked acoustic store missing: %v", err)
	}
	if strings.Contains(string(raw), reconcileSourceUUID) {
		t.Fatalf("forked acoustic store still carries source identity %s", reconcileSourceUUID)
	}
	store := acousticpackage.NewStore(filepath.Join(targetDir, "acoustic_package_status.json"))
	found, ok, err := store.Find(identity)
	if err != nil || !ok {
		t.Fatalf("forked store find err=%v ok=%v", err, ok)
	}
	if found.PackageLayers["l1_static"].Features["peak_rms_summary"].Status == acousticpackage.StatusStale {
		t.Fatalf("forked l1 peak_rms = stale (%+v)", found.PackageLayers["l1_static"].Features["peak_rms_summary"])
	}
	if ref := found.PackageLayers["l1_static"].Features["peak_rms_summary"].Ref; ref != nil {
		if numberFromAny(ref["rms_dbfs"]) == 0 {
			t.Fatalf("forked l1 peak_rms ref lost numeric facts: %#v", ref)
		}
	}

	// L3 水合链：目标的 L3 log 已再绑定，BuildL3AcousticStatuses 以 live id 出包。
	statuses, _, err := projectworkspace.BuildL3AcousticStatuses(targetPath, reconcileTargetUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) == 0 {
		t.Fatal("forked L3 statuses empty")
	}
	for _, row := range statuses {
		if strings.EqualFold(row.ProjectID, reconcileSourceUUID) {
			t.Fatalf("forked L3 status still under source identity: %s", row.ProjectID)
		}
	}

	// fork 保源：source 身份的事实原样在盘。
	if _, err := os.Stat(filepath.Join(filepath.Dir(sourcePath), ".vit_derived", reconcileSourceUUID, "l3_feature_log.jsonl")); err != nil {
		t.Fatalf("source L3 log must remain untouched: %v", err)
	}
}

// TestObservationIdentityReconcileSwitchFollowsUnrelatedProject：live 身份
// 与当前绑定无 parent 关系（外部 open 他工程）→ switch 跟随：绑定 live 自己
// 的 store，不搬运旧身份事实。
func TestObservationIdentityReconcileSwitchFollowsUnrelatedProject(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")
	otherPath := filepath.Join(root, "other.vit")
	seedReconcileFacts(t, sourcePath)

	h := reconcileTestHarness(t)
	defer projectstore.Deactivate()
	if _, err := h.ActivateProjectStore(sourcePath, reconcileSourceUUID); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"status": "ok", "project_uuid": reconcileOtherUUID, "project_id": reconcileOtherUUID,
		"project_path": otherPath, "tracks": []any{},
	}
	state = h.reconcileProjectIdentityForObservation(context.Background(), state)

	roots, ok := projectstore.Current()
	if !ok || !strings.EqualFold(roots.ProjectUUID, reconcileOtherUUID) {
		t.Fatalf("active store = %#v, want switch-bound to %s", roots, reconcileOtherUUID)
	}
	otherDir, err := projectworkspace.DerivedDir(otherPath, reconcileOtherUUID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(otherDir, "l3_feature_log.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("switch-follow must not carry source facts into the unrelated store: %v", err)
	}
}

// TestObservationIdentityReconcileHealsStaleShadow：shadow 滞后于 store 绑定
// （state 显示他 id，新鲜内核视图=当前绑定 id）→ 只刷新不跟随。
func TestObservationIdentityReconcileHealsStaleShadow(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")
	targetPath := filepath.Join(root, "dst.vit")

	h := reconcileTestHarness(t, map[string]any{
		"status": "ok", "project_uuid": reconcileSourceUUID, "project_id": reconcileSourceUUID,
		"project_path": sourcePath, "tracks": []any{},
	})
	defer projectstore.Deactivate()
	if _, err := h.ActivateProjectStore(sourcePath, reconcileSourceUUID); err != nil {
		t.Fatal(err)
	}

	state := map[string]any{
		"status": "ok", "project_uuid": reconcileTargetUUID, "project_id": reconcileTargetUUID,
		"project_path": targetPath, "tracks": []any{},
	}
	state = h.reconcileProjectIdentityForObservation(context.Background(), state)

	if got := firstString(state, "project_uuid"); !strings.EqualFold(got, reconcileSourceUUID) {
		t.Fatalf("reconciled state project = %q, want fresh kernel view %s", got, reconcileSourceUUID)
	}
	roots, _ := projectstore.Current()
	if !strings.EqualFold(roots.ProjectUUID, reconcileSourceUUID) {
		t.Fatalf("active store drifted to %q; stale shadow must heal without following", roots.ProjectUUID)
	}
	if _, err := projectworkspace.DerivedDir(targetPath, reconcileTargetUUID); err == nil {
		if dir, err := projectworkspace.DerivedDir(targetPath, reconcileTargetUUID); err == nil {
			if _, statErr := os.Stat(dir); statErr == nil {
				t.Fatal("no fork may happen when the fresh kernel view matches the binding")
			}
		}
	}
}

// TestObservationIdentityReconcileSkipsUnsavedIdentity：未保存的 live 身份
// （无路径、无同 id hint）→ 跳过跟随，绑定不动（读面不物化 draft）。
func TestObservationIdentityReconcileSkipsUnsavedIdentity(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")

	h := reconcileTestHarness(t)
	defer projectstore.Deactivate()
	if _, err := h.ActivateProjectStore(sourcePath, reconcileSourceUUID); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"status": "ok", "project_uuid": reconcileTargetUUID, "project_id": reconcileTargetUUID,
		"parent_project_uuid": reconcileSourceUUID, "tracks": []any{}, // 无 project_path
	}
	_ = h.reconcileProjectIdentityForObservation(context.Background(), state)

	roots, _ := projectstore.Current()
	if !strings.EqualFold(roots.ProjectUUID, reconcileSourceUUID) {
		t.Fatalf("unsaved identity must not move the binding (now %q)", roots.ProjectUUID)
	}
}

// TestObservationIdentityReconcileNoopWhenAligned：state 与绑定一致 → 零刷新
// 零跟随（cheap path）。
func TestObservationIdentityReconcileNoopWhenAligned(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")

	h := reconcileTestHarness(t)
	defer projectstore.Deactivate()
	if _, err := h.ActivateProjectStore(sourcePath, reconcileSourceUUID); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"status": "ok", "project_uuid": reconcileSourceUUID, "project_id": reconcileSourceUUID,
		"project_path": sourcePath, "tracks": []any{},
	}
	_ = h.reconcileProjectIdentityForObservation(context.Background(), state)

	fake := h.kernel.(*fakeKernelClient)
	if len(fake.commands) != 0 {
		t.Fatalf("aligned identity must not trigger kernel refresh, commands=%v", fake.commands)
	}
}
