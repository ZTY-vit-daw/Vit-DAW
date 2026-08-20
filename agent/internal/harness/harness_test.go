package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/collaboration"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tim"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

type fakeKernelClient struct {
	replies  []map[string]any
	commands []map[string]any
}

func TestLatestAuthoritativeProjectChangeSurvivesNewerPendingTelemetry(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "1007", "is_audio_track": true}},
	})
	project.ApplyDelta(map[string]any{
		"type": "delta_update", "target_uid": "1040", "action": "property_changed:state", "value": "after_write",
	})
	project.Initialize(map[string]any{
		"status": "ok", "project_uuid": "p1", "project_epoch": "e1", "project_revision": 1,
		"tracks": []any{map[string]any{"track_id": "1007", "is_audio_track": true}},
	})
	project.ApplyDelta(map[string]any{
		"type": "delta_update", "target_uid": "0/0", "action": "property_changed:lastSignificantChange", "value": "later_telemetry",
	})

	change := NewWithSender(nil, project, nil).LatestAuthoritativeProjectChange(4)
	if change == nil || change["freshness"] != "current_snapshot" || change["authoritative"] != true {
		t.Fatalf("authoritative change = %#v", change)
	}
}

func TestEnsureProjectAudioAnalysisUsesPersistedManifestWithoutKernelCall(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "persisted.vit")
	projectUUID := "vitproj_manifest_hit"
	status := map[string]any{
		"dad_fact_status":      "ready",
		"dad_fact_ready_count": 1,
		"dad_fact_total_count": 1,
		"track_waveform_envelopes": []map[string]any{{
			"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "rms_dbfs": -18.0, "peak_dbfs": -6.0,
		}},
	}
	if _, _, err := projectworkspace.SaveAnalysisManifest(projectPath, projectUUID, status); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": projectUUID})
	kernel := &fakeKernelClient{}
	h := NewWithSender(kernel, project, nil)

	result, err := h.ensureProjectAudioAnalysis(context.Background(), map[string]any{"timeout_ms": 1000})
	if err != nil || !audioAnalysisFactsReady(result) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(kernel.commands) != 0 || result["analysis_manifest_recovered"] != true {
		t.Fatalf("manifest recovery should avoid kernel calls: commands=%+v result=%+v", kernel.commands, result)
	}
}

func TestEnsureAppBoundVitSaveCommand(t *testing.T) {
	cmd := map[string]any{}
	projectPath := filepath.Join(t.TempDir(), "mix.vit")
	ensureAppBoundVitSaveCommand(cmd, projectPath)
	if got := firstString(cmd, "file_path"); got != projectPath {
		t.Fatalf("file_path=%q want %q", got, projectPath)
	}
	encryption := mapFromAny(cmd["encryption"])
	if firstString(encryption, "mode") != "app_bound_aes" {
		t.Fatalf("encryption=%+v", encryption)
	}

	explicit := map[string]any{
		"file_path":  filepath.Join(t.TempDir(), "explicit.vit"),
		"encryption": map[string]any{"mode": "custom"},
	}
	ensureAppBoundVitSaveCommand(explicit, projectPath)
	if firstString(mapFromAny(explicit["encryption"]), "mode") != "custom" {
		t.Fatalf("explicit encryption should be preserved: %+v", explicit)
	}

	plain := map[string]any{}
	ensureAppBoundVitSaveCommand(plain, filepath.Join(t.TempDir(), "legacy.xml"))
	if _, exists := plain["encryption"]; exists {
		t.Fatalf("non-.vit save should not force encryption: %+v", plain)
	}
}

func TestEnsureProjectAudioAnalysisRebuildsMissingRuntimeJob(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "rebuild.vit")
	projectUUID := "vitproj_manifest_rebuild"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": projectUUID})
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "error", "message": "project.audio_analysis_status could not find an analysis job"},
		{"status": "ok", "analysis_job_id": "job_rebuilt"},
		{"status": "ok", "dad_fact_status": "partial", "dad_fact_ready_count": 0, "dad_fact_total_count": 1},
		{"status": "ok", "dad_fact_status": "ready", "dad_fact_ready_count": 1, "dad_fact_total_count": 1,
			"track_waveform_envelopes": []map[string]any{{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "rms_dbfs": -20.0, "peak_dbfs": -8.0}}},
	}}
	h := NewWithSender(kernel, project, nil)

	result, err := h.ensureProjectAudioAnalysis(context.Background(), map[string]any{"timeout_ms": 2000, "poll_interval_ms": 50})
	if err != nil || !audioAnalysisFactsReady(result) || result["analysis_ensure_rebuilt"] != true {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var commands []string
	for _, command := range kernel.commands {
		commands = append(commands, firstString(command, "cmd"))
	}
	want := []string{"project.audio_analysis_status", "project.audio_analysis_start", "project.audio_analysis_status", "project.audio_analysis_status"}
	if fmt.Sprint(commands) != fmt.Sprint(want) {
		t.Fatalf("commands=%v want=%v", commands, want)
	}
	if _, _, err := projectworkspace.LoadAnalysisManifest(projectPath, projectUUID); err != nil {
		t.Fatalf("rebuilt ready facts were not persisted: %v", err)
	}
}

func TestEnsureProjectAudioAnalysisHonorsCancellation(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "cancel.vit")
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": "vitproj_manifest_cancel"})
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "error", "message": "missing job"},
		{"status": "ok", "analysis_job_id": "job_pending"},
		{"status": "ok", "dad_fact_status": "partial", "dad_fact_ready_count": 0, "dad_fact_total_count": 1},
	}}
	h := NewWithSender(kernel, project, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.ensureProjectAudioAnalysis(ctx, map[string]any{"timeout_ms": 5000, "poll_interval_ms": 50}); err == nil {
		t.Fatal("cancelled ensure unexpectedly succeeded")
	}
}

func TestInvokeLatestAudioAnalysisStatusTreatsMissingRuntimeJobAsRecoverable(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "missing-runtime-job.vit")
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": "vitproj_missing_runtime_job"})
	kernel := &fakeKernelClient{replies: []map[string]any{{
		"status": "error", "message": "project.audio_analysis_status could not find an analysis job",
	}}}
	h := NewWithSender(kernel, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "project.audio_analysis_status", Args: map[string]any{"latest": true}, Source: "test",
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("latest missing status should be recoverable: resp=%+v err=%v", resp, err)
	}
	if firstString(resp.Result, "analysis_queue_status") != "missing" ||
		!boolValueDefault(resp.Result["analysis_job_missing"], false) ||
		!boolValueDefault(resp.Result["analysis_recovery_required"], false) {
		t.Fatalf("recoverable missing status omitted recovery contract: %+v", resp.Result)
	}
}

func TestInvokeExplicitMissingAudioAnalysisJobRemainsError(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "explicit-missing-job.vit")
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": "vitproj_explicit_missing_job"})
	kernel := &fakeKernelClient{replies: []map[string]any{{
		"status": "error", "message": "project.audio_analysis_status could not find an analysis job",
	}}}
	h := NewWithSender(kernel, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "project.audio_analysis_status", Args: map[string]any{"analysis_job_id": "stale_job"}, Source: "test",
	})
	if err == nil || resp.Status != "kernel_error" {
		t.Fatalf("explicit missing job should remain an error: resp=%+v err=%v", resp, err)
	}
}

func testMap(t *testing.T, value any) map[string]any {
	t.Helper()
	row, ok := value.(map[string]any)
	if ok {
		return row
	}
	data, err := json.Marshal(value)
	if err == nil {
		if err := json.Unmarshal(data, &row); err == nil && row != nil {
			return row
		}
	}
	if !ok {
		t.Fatalf("value is %T, want map[string]any: %+v", value, value)
	}
	return row
}

func statusIn(value any, allowed ...string) bool {
	text := strings.TrimSpace(fmt.Sprint(value))
	for _, candidate := range allowed {
		if text == candidate {
			return true
		}
	}
	return false
}

func TestPublicAudioAnalysisResultKeepsDADFactRows(t *testing.T) {
	reply := map[string]any{
		"status":                    "ok",
		"analysis_job_id":           "audio_analysis_rows",
		"dad_fact_status":           "partial",
		"dad_fact_ready_count":      1,
		"dad_fact_total_count":      2,
		"dad_fact_pending_count":    1,
		"dad_fact_completion_scope": "waveform_baker_latest_status",
		"track_waveform_envelopes": []any{
			map[string]any{
				"status":               "ready",
				"track_id":             "track_001",
				"clip_id":              "clip_001",
				"source_path":          "E:/stems/stem_001.wav",
				"rms_dbfs":             -18.25,
				"peak_dbfs":            -1.5,
				"headroom_db":          1.5,
				"balance_db":           0.2,
				"correlation_estimate": 0.91,
				"tile_count_seen":      46,
				"tile_count_expected":  46,
				"large_raw_payload":    strings.Repeat("x", 256),
			},
		},
		"analysis_job": map[string]any{
			"analysis_job_id":           "audio_analysis_rows",
			"dad_fact_status":           "partial",
			"dad_fact_ready_count":      1,
			"dad_fact_total_count":      2,
			"dad_fact_pending_count":    1,
			"dad_fact_completion_scope": "waveform_baker_latest_status",
			"track_waveform_envelopes": []any{
				map[string]any{
					"status":               "ready",
					"track_id":             "track_001",
					"clip_id":              "clip_001",
					"source_path":          "E:/stems/stem_001.wav",
					"rms_dbfs":             -18.25,
					"peak_dbfs":            -1.5,
					"headroom_db":          1.5,
					"balance_db":           0.2,
					"correlation_estimate": 0.91,
					"tile_count_seen":      46,
					"tile_count_expected":  46,
					"large_raw_payload":    strings.Repeat("x", 256),
				},
			},
		},
	}

	out := publicAudioAnalysisResult("project.audio_analysis_status", nil, reply)
	if firstString(out, "dad_fact_completion_scope") != "waveform_baker_latest_status" {
		t.Fatalf("top-level dad fact scope missing: %+v", out)
	}
	rows := mapRowsFromAny(out["track_waveform_envelopes"])
	if len(rows) != 1 || firstString(rows[0], "track_id") != "track_001" {
		t.Fatalf("top-level waveform rows missing: %+v", out)
	}
	if _, ok := rows[0]["large_raw_payload"]; ok {
		t.Fatalf("raw waveform payload should be compacted out: %+v", rows[0])
	}
	for _, key := range []string{"rms_dbfs", "peak_dbfs", "headroom_db", "balance_db", "correlation_estimate"} {
		if _, ok := rows[0][key]; !ok {
			t.Fatalf("compact DAD waveform row should retain %s: %+v", key, rows[0])
		}
	}
	job := testMap(t, out["analysis_job"])
	jobRows := mapRowsFromAny(job["track_waveform_envelopes"])
	if firstString(job, "dad_fact_completion_scope") != "waveform_baker_latest_status" || len(jobRows) != 1 {
		t.Fatalf("analysis_job dad fact rows missing: %+v", job)
	}
}

func (f *fakeKernelClient) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.commands = append(f.commands, tools.CloneCommand(cmd))
	if len(f.replies) == 0 {
		return map[string]any{"status": "ok"}, `{"status":"ok"}`, nil
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	if errText := strings.TrimSpace(fmt.Sprint(reply["error"])); errText != "" && errText != "<nil>" {
		return reply, "", fmt.Errorf("%s", errText)
	}
	return reply, "", nil
}

func TestProjectOpenLifecycleRecoversParentWorkspaceUsingCommandPath(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "B1完成.vit")
	targetPath := filepath.Join(root, "B2完成.vit")
	if err := os.WriteFile(sourcePath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_open_parent"
	const targetUUID = "vitproj_open_child"
	history.BindProjectIdentity(sourcePath, sourceUUID)
	if _, err := history.EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{
		"project_path": sourcePath, "message": "B1 baseline", "source": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.AppendConversationNode(map[string]any{
		"project_path": sourcePath, "kind": "vit", "commit_id": checkpoint["commit_id"], "text": "B1 complete",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := history.CommitWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}

	h := New(nil, nil, nil)
	result, err := h.applyProjectLifecycle(context.Background(), tools.CommandSpec{CommandName: "open_project"},
		map[string]any{"file_path": targetPath},
		map[string]any{
			"status": "ok", "project_lifecycle": "open",
			"project_path": filepath.Join(root, "B2瀹屾垚.vit"),
			"project_uuid": targetUUID, "parent_project_uuid": sourceUUID,
		})
	if err != nil {
		t.Fatal(err)
	}
	if got := firstString(result, "project_path"); !samePath(got, targetPath) {
		t.Fatalf("project path=%q want=%q result=%+v", got, targetPath, result)
	}
	graph := testMap(t, result["conversation_graph"])
	nodes := mapRowsFromAny(graph["nodes"])
	if len(nodes) != 1 || firstString(nodes[0], "text") != "B1 complete" {
		t.Fatalf("child did not inherit parent conversation: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, history.DirName, targetUUID, "workspace.json")); err != nil {
		t.Fatalf("target canonical workspace missing: %v", err)
	}
}

func TestProjectOpenLifecycleFindsParentWorkspaceInAncestorDirectory(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "A5-parent.vit")
	targetDir := filepath.Join(root, "A5-child")
	targetPath := filepath.Join(targetDir, "A5-child.vit")
	if err := os.WriteFile(sourcePath, []byte("parent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_ancestor_parent"
	const targetUUID = "vitproj_ancestor_child"
	history.BindProjectIdentity(sourcePath, sourceUUID)
	if _, err := history.EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{
		"project_path": sourcePath, "message": "A5 parent baseline", "source": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.AppendConversationNode(map[string]any{
		"project_path": sourcePath, "kind": "vit", "commit_id": checkpoint["commit_id"], "text": "A5 parent history",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := history.CommitWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}

	h := New(nil, nil, nil)
	result, err := h.applyProjectLifecycle(context.Background(), tools.CommandSpec{CommandName: "open_project"},
		map[string]any{"file_path": targetPath},
		map[string]any{
			"status": "ok", "project_lifecycle": "open", "project_path": targetPath,
			"project_uuid": targetUUID, "parent_project_uuid": sourceUUID,
		})
	if err != nil {
		t.Fatal(err)
	}
	graph := testMap(t, result["conversation_graph"])
	nodes := mapRowsFromAny(graph["nodes"])
	if len(nodes) != 1 || firstString(nodes[0], "text") != "A5 parent history" {
		t.Fatalf("nested child did not inherit ancestor parent history: %+v", result)
	}
}

type fakeVSPKernelClient struct {
	fakeKernelClient
	snapshots      []*kernel.VSPStateResult
	deltas         []*kernel.VSPStateResult
	resyncs        []*kernel.VSPStateResult
	vspCommands    []string
	vspLegacyCmds  []string
	commandReplies []*kernel.VSPCommandResult
}

func (f *fakeVSPKernelClient) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	f.vspCommands = append(f.vspCommands, command)
	if len(f.commandReplies) > 0 {
		reply := f.commandReplies[0]
		f.commandReplies = f.commandReplies[1:]
		reply.Command = command
		return reply, nil
	}
	return fakeVSPCommandReply(command, "legacy", map[string]any{"status": "ok"}), nil
}

func (f *fakeVSPKernelClient) SendVSPLegacyCommand(_ context.Context, cmd map[string]any) (*kernel.VSPCommandResult, error) {
	legacy := strings.TrimSpace(fmt.Sprint(cmd["cmd"]))
	f.vspLegacyCmds = append(f.vspLegacyCmds, legacy)
	if len(f.commandReplies) > 0 {
		reply := f.commandReplies[0]
		f.commandReplies = f.commandReplies[1:]
		reply.Command = "legacy.command"
		reply.LegacyCommand = legacy
		return reply, nil
	}
	return fakeVSPCommandReply("legacy.command", legacy, map[string]any{"status": "ok"}), nil
}

func (f *fakeVSPKernelClient) VSPStateSnapshot(_ context.Context, scope string) (*kernel.VSPStateResult, error) {
	if len(f.snapshots) > 0 {
		reply := f.snapshots[0]
		f.snapshots = f.snapshots[1:]
		return reply, nil
	}
	return fakeVSPSnapshot(1, scope, []any{}), nil
}

func (f *fakeVSPKernelClient) VSPStateDelta(_ context.Context, baseRevision int64, scope string) (*kernel.VSPStateResult, error) {
	if len(f.deltas) > 0 {
		reply := f.deltas[0]
		f.deltas = f.deltas[1:]
		return reply, nil
	}
	return fakeVSPDelta(baseRevision, baseRevision, scope, nil), nil
}

func (f *fakeVSPKernelClient) VSPStateResync(_ context.Context, scope string) (*kernel.VSPStateResult, error) {
	if len(f.resyncs) > 0 {
		reply := f.resyncs[0]
		f.resyncs = f.resyncs[1:]
		return reply, nil
	}
	return fakeVSPSnapshot(1, scope, []any{}), nil
}

func fakeVSPCommandReply(command, legacyCommand string, legacyReply map[string]any) *kernel.VSPCommandResult {
	return &kernel.VSPCommandResult{
		Response: map[string]any{
			"type": "command.response",
			"ack":  map[string]any{"stage": "completed", "message": "ok"},
		},
		Payload:       map[string]any{"command": command, "legacy_command": legacyCommand, "legacy_reply": legacyReply},
		LegacyReply:   legacyReply,
		Command:       command,
		LegacyCommand: legacyCommand,
		TransactionID: "tx_fake",
	}
}

func fakeVSPSnapshot(revision int64, scope string, tracks []any) *kernel.VSPStateResult {
	project := map[string]any{"project_id": "project_current", "project_path": "D:/song/test.vit", "scope": scope, "track_count": len(tracks)}
	legacy := map[string]any{
		"status": "ok", "project_path": "D:/song/test.vit", "tracks": tracks,
		"snapshot_hash": fmt.Sprintf("hash_%d", revision), "project_revision": revision, "project_epoch": "epoch_test",
	}
	payload := map[string]any{"status": "ok", "scope": scope, "snapshot_hash": fmt.Sprintf("hash_%d", revision), "project": project, "tracks": tracks, "snapshot": map[string]any{"project": project, "tracks": tracks}}
	return &kernel.VSPStateResult{
		Response:     map[string]any{"type": "state.snapshot", "ack": map[string]any{"stage": "completed"}},
		Payload:      payload,
		LegacyState:  legacy,
		Revision:     revision,
		ProjectEpoch: "epoch_test",
		SnapshotHash: fmt.Sprintf("hash_%d", revision),
		Scope:        scope,
	}
}

func fakeVSPDelta(baseRevision, revision int64, scope string, ops []any) *kernel.VSPStateResult {
	payload := map[string]any{"status": "ok", "scope": scope, "snapshot_hash": fmt.Sprintf("hash_%d", revision), "ops": ops}
	return &kernel.VSPStateResult{
		Response:      map[string]any{"type": "state.delta", "ack": map[string]any{"stage": "completed"}},
		Payload:       payload,
		Revision:      revision,
		BaseRevision:  baseRevision,
		ProjectEpoch:  "epoch_test",
		SnapshotHash:  fmt.Sprintf("hash_%d", revision),
		Scope:         scope,
		Ops:           ops,
		ChangedTracks: []any{"track_2"},
	}
}

func TestInvokeConfirmCommandDoesNotNeedKernelBeforeApproval(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{"cmd": "delete_track", "track_id": "1007"},
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.AgentActionID == "" || resp.Preview == "" {
		t.Fatalf("missing action id or preview: %+v", resp)
	}
}

func TestResolveRejectsUnknownCommand(t *testing.T) {
	h := New(nil, nil, nil)
	_, _, err := h.resolveCommand(InvokeRequest{
		Tool: "daw.invoke",
		Args: map[string]any{"cmd": "totally_not_registered"},
	})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestResolveRackAddMacroAlias(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd":   "rack.add_macro",
			"name":  "通用宏控件",
			"value": 0.5,
		},
	})
	if err != nil {
		t.Fatalf("resolve rack.add_macro: %v", err)
	}
	if spec.CommandName != "control_add_macro" {
		t.Fatalf("command = %q, want control_add_macro", spec.CommandName)
	}
	if cmd["cmd"] != "control_add_macro" {
		t.Fatalf("resolved cmd = %v, want control_add_macro", cmd["cmd"])
	}
}

func TestInvokeRackAddMacroAliasUsesSelectedTrack(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "rack.add_macro",
			"name":  "通用宏控件",
			"value": 0.5,
		},
		Context: map[string]any{
			"selected_track_id": "1007",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.CommandName != "control_add_macro" || resp.Tool != "rack.add_macro" {
		t.Fatalf("resolved response = %+v", resp)
	}
	if !strings.Contains(resp.Preview, "Macro 通用宏控件") || !strings.Contains(resp.Preview, "Track 1007") {
		t.Fatalf("preview did not include macro/track: %q", resp.Preview)
	}
}

func TestInvokeControlAddMacroConfirmedCreatesRackMacroMutation(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "control_add_macro",
			"name":  "Agent Macro",
			"value": 0.25,
		},
		Context: map[string]any{
			"selected_track_id": "1007",
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_add_macro" {
		t.Fatalf("command = %q, want control_add_macro", resp.CommandName)
	}
	if resp.Result["ui_action"] != "rack_macro_upserted" || resp.Result["kind"] != "rack_macro_upserted" {
		t.Fatalf("result did not request rack macro upsert: %+v", resp.Result)
	}
	macroID := strings.TrimSpace(fmt.Sprint(resp.Result["macro_id"]))
	if macroID == "" {
		t.Fatalf("missing macro_id: %+v", resp.Result)
	}
	macro, ok := resp.Result["macro"].(map[string]any)
	if !ok {
		t.Fatalf("macro payload missing: %+v", resp.Result)
	}
	if macro["macro_id"] != macroID || macro["track_id"] != "1007" || macro["name"] != "Agent Macro" {
		t.Fatalf("macro payload mismatch: id=%q macro=%+v", macroID, macro)
	}
	if got := fmt.Sprint(macro["value"]); got != "0.25" {
		t.Fatalf("macro value = %v, want 0.25; macro=%+v", got, macro)
	}
}

func TestInvokeControlAddBindingConfirmedCreatesRackMacroBindingMutation(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":               "control_add_binding",
			"source_node_id":    "macro_ccc50053d5f770bf",
			"target_track_id":   "1007",
			"target_plugin_id":  "1012",
			"target_param_id":   "2",
			"target_param_name": "B1 Gain",
			"target_min":        -12,
			"target_max":        12,
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_add_binding" {
		t.Fatalf("command = %q, want control_add_binding", resp.CommandName)
	}
	if resp.Result["ui_action"] != "rack_macro_binding_added" || resp.Result["kind"] != "rack_macro_binding_added" {
		t.Fatalf("result did not request rack macro binding: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_ccc50053d5f770bf" {
		t.Fatalf("macro_id = %v", resp.Result["macro_id"])
	}
	binding, ok := resp.Result["binding"].(map[string]any)
	if !ok {
		t.Fatalf("binding payload missing: %+v", resp.Result)
	}
	if binding["track_id"] != "1007" || binding["plugin_id"] != "1012" || binding["param_id"] != "2" || binding["param_name"] != "B1 Gain" {
		t.Fatalf("binding payload mismatch: %+v", binding)
	}
}

func TestInvokeControlSetMacroValuesAppliesTrackVolumeBinding(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_set_macro_values",
			"macro_id": "macro_volume",
			"value":    -5.25,
			"macro": map[string]any{
				"macro_id": "macro_volume",
				"name":     "轨道电平",
				"track_id": "track_1",
				"value":    -6.0,
				"min":      -60.0,
				"max":      12.0,
				"bindings": []map[string]any{{
					"control":    "track.volume",
					"track_id":   "track_1",
					"param_id":   "track.volume",
					"param_name": "轨道音量",
					"target_min": -60.0,
					"target_max": 12.0,
				}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, result=%+v error=%q", resp.Status, resp.Result, resp.Error)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("commands = %+v", kernel.commands)
	}
	cmd := kernel.commands[0]
	if cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-5.25" {
		t.Fatalf("kernel command = %+v", cmd)
	}
	if resp.Result["ui_action"] != "rack_macro_value_changed" {
		t.Fatalf("result = %+v", resp.Result)
	}
}

func TestInvokeControlSetMacroValuesTreatsTrackVolumeAsDBWhenBindingHasDefaultRange(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_set_macro_values",
			"macro_id": "macro_volume",
			"value":    -1.5,
			"macro": map[string]any{
				"macro_id": "macro_volume",
				"name":     "Track volume",
				"track_id": "track_1",
				"value":    0.0,
				"min":      -60.0,
				"max":      12.0,
				"unit":     "dB",
				"bindings": []map[string]any{{
					"control":    "track.volume",
					"track_id":   "track_1",
					"param_id":   "track.volume",
					"target_min": 0.0,
					"target_max": 1.0,
				}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" || len(kernel.commands) != 1 {
		t.Fatalf("resp=%+v commands=%+v", resp, kernel.commands)
	}
	if got := fmt.Sprint(kernel.commands[0]["db"]); got != "-1.5" {
		t.Fatalf("db = %s, want -1.5; command=%+v", got, kernel.commands[0])
	}
}

func TestMixTickProposeApplyRollbackTrackGainAdjust(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "volume_db": -6.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_gain_adjust",
			"track_id":       "track_1",
			"delta_db":       3.75,
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" {
		t.Fatalf("propose status = %q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("propose should not send kernel command: %+v", kernel.commands)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	if tickID == "" || fmt.Sprint(propose.Result["delta_db"]) != "2" || fmt.Sprint(propose.Result["after_db"]) != "-4" {
		t.Fatalf("unexpected proposal: %+v", propose.Result)
	}

	rejected, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err == nil || rejected.Status != "error" || !strings.Contains(rejected.Error, "confirmation") {
		t.Fatalf("unconfirmed apply should fail with confirmation error: resp=%+v err=%v", rejected, err)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	volumeCommands := testCommandsByName(kernel.commands, "set_volume")
	if len(volumeCommands) != 1 {
		t.Fatalf("set_volume commands = %+v all=%+v", volumeCommands, kernel.commands)
	}
	if cmd := volumeCommands[0]; cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-4" {
		t.Fatalf("apply kernel command = %+v", cmd)
	}

	duplicate, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err == nil || duplicate.Status != "error" || !strings.Contains(duplicate.Error, "already applied") {
		t.Fatalf("duplicate apply should fail: resp=%+v err=%v", duplicate, err)
	}

	rolledBack, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.rollback_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if rolledBack.Status != "ok" {
		t.Fatalf("rollback status = %q result=%+v error=%q", rolledBack.Status, rolledBack.Result, rolledBack.Error)
	}
	volumeCommands = testCommandsByName(kernel.commands, "set_volume")
	if len(volumeCommands) != 2 {
		t.Fatalf("set_volume commands after rollback = %+v all=%+v", volumeCommands, kernel.commands)
	}
	if cmd := volumeCommands[1]; cmd["cmd"] != "set_volume" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["db"]) != "-6" {
		t.Fatalf("rollback kernel command = %+v", cmd)
	}
}

func TestMixTickProposeApplyRollbackTrackPanAdjust(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_pan_adjust",
			"track_id":       "track_1",
			"delta_pan":      0.30,
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" {
		t.Fatalf("propose status = %q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("propose should not send kernel command: %+v", kernel.commands)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	if tickID == "" || fmt.Sprint(propose.Result["delta_pan"]) != "0.15" || fmt.Sprint(propose.Result["after_pan"]) != "0.15" {
		t.Fatalf("unexpected proposal: %+v", propose.Result)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	panCommands := testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 1 {
		t.Fatalf("set_pan commands = %+v all=%+v", panCommands, kernel.commands)
	}
	if cmd := panCommands[0]; cmd["cmd"] != "set_pan" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["pan"]) != "0.15" {
		t.Fatalf("apply kernel command = %+v", cmd)
	}

	rolledBack, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.rollback_tick",
		Args: map[string]any{"tick_id": tickID},
	})
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if rolledBack.Status != "ok" {
		t.Fatalf("rollback status = %q result=%+v error=%q", rolledBack.Status, rolledBack.Result, rolledBack.Error)
	}
	panCommands = testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 2 {
		t.Fatalf("set_pan commands after rollback = %+v all=%+v", panCommands, kernel.commands)
	}
	if cmd := panCommands[1]; cmd["cmd"] != "set_pan" || cmd["track_id"] != "track_1" || fmt.Sprint(cmd["pan"]) != "0" {
		t.Fatalf("rollback kernel command = %+v", cmd)
	}
}

func TestMixTickProposeApplyTrackPanSet(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, shadow.New(nil), nil)
	h.kernel = kernel
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.25},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation": "track_pan_set",
			"track_id":  "track_1",
			"pan":       -0.25,
		},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("propose returned error: %v", err)
	}
	if propose.Status != "ok" || fmt.Sprint(propose.Result["before_pan"]) != "0.25" || fmt.Sprint(propose.Result["after_pan"]) != "-0.25" {
		t.Fatalf("unexpected proposal: status=%q result=%+v error=%q", propose.Status, propose.Result, propose.Error)
	}
	tickID := strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"]))
	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": tickID, "confirmation": true},
	})
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if applied.Status != "ok" {
		t.Fatalf("apply status = %q result=%+v error=%q", applied.Status, applied.Result, applied.Error)
	}
	panCommands := testCommandsByName(kernel.commands, "set_pan")
	if len(panCommands) != 1 || fmt.Sprint(panCommands[0]["pan"]) != "-0.25" {
		t.Fatalf("set_pan commands = %+v all=%+v", panCommands, kernel.commands)
	}
}

func TestMixTickApplyPanReportsObservedTrackPan(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok"},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "pan": -0.7},
		}},
	}}
	h := NewWithSender(kernel, shadow.New(nil), nil)
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "pan": 0.0},
		},
	})

	propose, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation": "track_pan_set",
			"track_id":  "track_1",
			"pan":       -0.7,
		},
		Source: "test",
	})
	if err != nil || propose.Status != "ok" {
		t.Fatalf("propose = status=%q result=%+v err=%v", propose.Status, propose.Result, err)
	}

	applied, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.apply_tick",
		Args: map[string]any{"tick_id": strings.TrimSpace(fmt.Sprint(propose.Result["tick_id"])), "confirmation": true},
	})
	if err != nil || applied.Status != "ok" {
		t.Fatalf("apply = status=%q result=%+v err=%v", applied.Status, applied.Result, err)
	}
	if fmt.Sprint(applied.Result["observed_pan"]) != "-0.7" || applied.Result["observed_pan_matches"] != true {
		t.Fatalf("observed pan result = %+v", applied.Result)
	}
	observed := testMap(t, applied.Result["observed_track"])
	if fmt.Sprint(observed["pan"]) != "-0.7" {
		t.Fatalf("observed track = %+v", observed)
	}
}

func TestDirectTrackPanReplyUpdatesShadowPan(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "track_id": "track_1", "track_name": "Lead", "pan": 0.75, "pan_value": 0.75},
		{"status": "ok", "tracks": []map[string]any{
			{"track_id": "track_1", "track_name": "Lead", "track_type": "audio", "is_audio_track": true, "pan": 0.0},
		}},
	}}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "audio", "is_audio_track": true, "pan": 0.0},
		},
	})
	h := NewWithSender(kernel, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "track.pan",
		Args:      map[string]any{"track_id": "track_1", "pan": 0.75},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("track pan = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	tracks := visibleTrackRows(h.UserStateSummary(context.Background()))
	if len(tracks) != 1 {
		t.Fatalf("tracks = %+v", tracks)
	}
	if fmt.Sprint(tracks[0]["pan"]) != "0.75" || fmt.Sprint(tracks[0]["pan_value"]) != "0.75" {
		t.Fatalf("track pan did not update shadow: %+v", tracks[0])
	}
}

func TestMixTickProposeRequiresExplicitDelta(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	h.shadow.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true, "volume_db": -6.0},
		},
	})

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.propose_tick",
		Args: map[string]any{
			"operation":      "track_gain_adjust",
			"track_id":       "track_1",
			"observation_id": "obs_1",
		},
		Source: "test",
	})
	if err == nil {
		t.Fatalf("expected error when delta_db is missing, got resp=%+v", resp)
	}
	if resp.Status != "error" || !strings.Contains(resp.Error, "delta_db is required") {
		t.Fatalf("unexpected response: %+v err=%v", resp, err)
	}
}

func testCommandsByName(commands []map[string]any, name string) []map[string]any {
	var out []map[string]any
	for _, cmd := range commands {
		if fmt.Sprint(cmd["cmd"]) == name {
			out = append(out, cmd)
		}
	}
	return out
}

func TestInvokeProjectStateUsesVSPObserve(t *testing.T) {
	tracks := []any{
		map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true},
	}
	kernel := &fakeVSPKernelClient{snapshots: []*kernel.VSPStateResult{fakeVSPSnapshot(7, "project.timeline", tracks)}}
	h := NewWithSender(kernel, shadow.New(nil), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "project.state", Source: "test"})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("project.state = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("legacy SendCommand used: %+v", kernel.commands)
	}
	observe := testMap(t, resp.Result["project_observe"])
	if fmt.Sprint(observe["revision"]) != "7" || observe["source"] != "vsp.state.snapshot" {
		t.Fatalf("project_observe = %+v", observe)
	}
	if fmt.Sprint(resp.Result["track_count"]) != "1" {
		t.Fatalf("visible state not initialized from VSP: %+v", resp.Result)
	}
	if resp.Result["snapshot_hash"] != "hash_7" || fmt.Sprint(resp.Result["project_revision"]) != "7" || resp.Result["project_epoch"] != "epoch_test" {
		t.Fatalf("VSP project cut missing from visible state: %+v", resp.Result)
	}
}

func TestInvokeMixObserveCarriesVSPProjectCutIntoMOM(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	tracks := []any{
		map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true},
		map[string]any{"track_id": "track_2", "track_name": "Drums", "track_type": "hybrid", "is_audio_track": true},
	}
	kernel := &fakeVSPKernelClient{snapshots: []*kernel.VSPStateResult{fakeVSPSnapshot(9, "project.timeline", tracks)}}
	initial := shadow.New(nil)
	initial.Initialize(map[string]any{"status": "ok", "tracks": []any{}})
	h := NewWithSender(kernel, initial, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "mix.observe",
		Args:      map[string]any{"mix_session_id": "mix_vsp_cut", "scope": "full_project", "goal_text": "B2"},
		Confirmed: true,
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("mix.observe = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	if obs.MOMProjection == nil || obs.MOMProjection.StaticLevelRelationship == nil || obs.MOMProjection.StaticLevelRelationship.ProjectCutRef != "hash_9" {
		t.Fatalf("VSP project cut did not reach MOM: %+v", obs.MOMProjection)
	}
}

func TestProjectStateLoadsPersistedAnalysisManifest(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "manifest-project.vit")
	projectUUID := "vitproj_manifest_project"
	_, _, err := projectworkspace.SaveAnalysisManifest(projectPath, projectUUID, map[string]any{
		"dad_fact_status": "ready",
		"track_waveform_envelopes": []any{map[string]any{
			"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_path": "D:/audio/lead.wav", "rms_dbfs": -24.0, "peak_dbfs": -8.0,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := projectStateWithPersistedAnalysisManifest(map[string]any{"project_path": projectPath, "project_uuid": projectUUID})
	manifest := mapFromAny(state["analysis_manifest"])
	if manifest["status"] != "ready" || len(mapRowsFromAny(manifest["l1_waveform_rows"])) != 1 {
		t.Fatalf("persisted analysis manifest was not loaded: %#v", state)
	}
}

func TestInvokeAudioAnalysisStatusViaVSPPersistsReadyDerivedManifest(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "vsp-analysis.vit")
	projectUUID := "vitproj_vsp_analysis"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": projectPath, "project_uuid": projectUUID})
	reply := map[string]any{
		"status": "ok", "command": "project.audio_analysis_status",
		"analysis_job_id": "job_vsp_ready", "analysis_queue_status": "submitted",
		"analysis_job": map[string]any{
			"analysis_job_id": "job_vsp_ready", "job_id": "job_vsp_ready",
			"dad_fact_status": "ready", "dad_fact_ready_count": 1, "dad_fact_total_count": 1,
			"track_waveform_envelopes": []any{map[string]any{
				"status": "ready", "track_id": "track_1", "clip_id": "clip_1",
				"source_path": "D:/audio/lead.wav", "rms_dbfs": -18.0, "peak_dbfs": -6.0,
			}},
		},
	}
	kernel := &fakeVSPKernelClient{commandReplies: []*kernel.VSPCommandResult{
		fakeVSPCommandReply("legacy.command", "project.audio_analysis_status", reply),
	}}
	h := NewWithSender(kernel, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "project.audio_analysis_status", Args: map[string]any{"latest": true}, Source: "test",
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("VSP audio status failed: resp=%+v err=%v", resp, err)
	}
	manifest, path, err := projectworkspace.LoadAnalysisManifest(projectPath, projectUUID)
	if err != nil {
		t.Fatalf("VSP ready status did not persist derived manifest: %v", err)
	}
	if manifest.Status != "ready" || len(manifest.Rows) != 1 || manifest.Rows[0]["track_id"] != "track_1" {
		t.Fatalf("persisted manifest=%+v path=%s", manifest, path)
	}
	if firstString(resp.Result, "analysis_manifest_path") != path {
		t.Fatalf("response omitted persisted manifest path: result=%+v want=%s", resp.Result, path)
	}
}

func TestInvokeMutatingCommandUsesVSPCommandDeltaAndResync(t *testing.T) {
	beforeTracks := []any{
		map[string]any{"track_id": "track_1", "track_name": "Lead", "track_type": "hybrid", "is_audio_track": true},
	}
	afterTracks := append(append([]any{}, beforeTracks...), map[string]any{"track_id": "track_2", "track_name": "Harmony", "track_type": "hybrid", "is_audio_track": true})
	deltaOps := []any{map[string]any{"op": "add", "path": "/tracks/track_2", "track_id": "track_2"}}
	kernel := &fakeVSPKernelClient{
		snapshots: []*kernel.VSPStateResult{fakeVSPSnapshot(10, "project.timeline", beforeTracks)},
		deltas:    []*kernel.VSPStateResult{fakeVSPDelta(10, 11, "project.timeline", deltaOps)},
		resyncs:   []*kernel.VSPStateResult{fakeVSPSnapshot(11, "project.timeline", afterTracks)},
		commandReplies: []*kernel.VSPCommandResult{
			fakeVSPCommandReply("track.create", "add_track", map[string]any{"status": "ok", "track_id": "track_2", "track_name": "Harmony"}),
		},
	}
	h := NewWithSender(kernel, shadow.New(nil), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "track.add", Args: map[string]any{"track_name": "Harmony"}, Source: "test"})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("track.add = status=%q result=%+v err=%v", resp.Status, resp.Result, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("legacy SendCommand used: %+v", kernel.commands)
	}
	if len(kernel.vspCommands) != 1 || kernel.vspCommands[0] != "track.create" {
		t.Fatalf("vsp commands = %+v", kernel.vspCommands)
	}
	ack := testMap(t, resp.Result["command_ack"])
	if ack["command"] != "track.create" || ack["legacy_command"] != "add_track" {
		t.Fatalf("command_ack = %+v", ack)
	}
	delta := testMap(t, resp.Result["state_delta"])
	if fmt.Sprint(delta["ops_count"]) != "1" {
		t.Fatalf("state_delta = %+v", delta)
	}
	resync := testMap(t, resp.Result["state_resync"])
	if fmt.Sprint(resync["revision"]) != "11" {
		t.Fatalf("state_resync = %+v", resync)
	}
}

func TestInvokeControlAddBindingResolvesExistingMacroByName(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":               "control_add_binding",
			"source_node_id":    "Macro 1",
			"target_plugin_id":  "1012",
			"target_param_id":   "2",
			"target_param_name": "B1 Gain",
		},
		Context: map[string]any{
			"user_message": "把 B1 Gain 绑定到 Macro 1 上",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" {
		t.Fatalf("macro_id = %v, want macro_existing; result=%+v", resp.Result["macro_id"], resp.Result)
	}
	binding, ok := resp.Result["binding"].(map[string]any)
	if !ok {
		t.Fatalf("binding payload missing: %+v", resp.Result)
	}
	if binding["plugin_id"] != "1012" || binding["param_id"] != "2" || binding["track_id"] != "1007" {
		t.Fatalf("binding payload mismatch: %+v", binding)
	}
}

func TestInvokeControlAddMacroBindingIntentSelectsExistingMacro(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":   "control_add_macro",
			"name":  "Agent Macro",
			"value": 0.5,
		},
		Context: map[string]any{
			"user_message": "把 B1 Gain 绑定到 Macro 1 上",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.Result["kind"] != "rack_macro_selected" || resp.Result["selected_existing_macro"] != true {
		t.Fatalf("result did not select existing macro: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" {
		t.Fatalf("macro_id = %v, want macro_existing; result=%+v", resp.Result["macro_id"], resp.Result)
	}
}

func TestInvokeControlRenameMacroResolvesExistingMacroByName(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{
			"cmd":      "control_rename_macro",
			"macro_id": "Macro 1",
			"name":     "Filter Sweep",
		},
		Context: map[string]any{
			"user_message": "rename Macro 1 to Filter Sweep",
			"available_macro_controls": []any{
				map[string]any{"macro_id": "macro_existing", "name": "Macro 1", "track_id": "1007", "bindings": []any{}},
			},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok; error=%q result=%+v", resp.Status, resp.Error, resp.Result)
	}
	if resp.CommandName != "control_rename_macro" {
		t.Fatalf("command = %q, want control_rename_macro", resp.CommandName)
	}
	if resp.Result["kind"] != "rack_macro_renamed" || resp.Result["ui_action"] != "rack_macro_renamed" {
		t.Fatalf("result did not request macro rename: %+v", resp.Result)
	}
	if resp.Result["macro_id"] != "macro_existing" || resp.Result["name"] != "Filter Sweep" || resp.Result["old_name"] != "Macro 1" {
		t.Fatalf("rename payload mismatch: %+v", resp.Result)
	}
	macro, ok := resp.Result["macro"].(map[string]any)
	if !ok || macro["macro_id"] != "macro_existing" || macro["name"] != "Filter Sweep" {
		t.Fatalf("macro payload mismatch: %+v", resp.Result)
	}
}

func TestResolveToolAcceptsCommandName(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "get_project_state",
	})
	if err != nil {
		t.Fatalf("resolve command-name tool: %v", err)
	}
	if spec.CommandName != "get_project_state" || cmd["cmd"] != "get_project_state" {
		t.Fatalf("resolved = spec:%+v cmd:%+v", spec, cmd)
	}
}

func TestResolveCommandAcceptsTypedToolCommandPayload(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool":     "mix.apply_tick",
			"tick_id":  "mix_tick_test",
			"track_id": "1192",
		},
	})
	if err != nil {
		t.Fatalf("resolve typed tool command payload: %v", err)
	}
	if spec.CommandName != "mix_apply_tick" || cmd["cmd"] != "mix_apply_tick" {
		t.Fatalf("resolved = spec:%+v cmd:%+v", spec, cmd)
	}
	if cmd["tick_id"] != "mix_tick_test" || cmd["track_id"] != "1192" {
		t.Fatalf("resolved command lost args: %+v", cmd)
	}
}

func TestRollbackWorkspaceApplyEditUsesReversePatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "workspace.apply_edit",
		Args: map[string]any{
			"path":            path,
			"old_text":        "vit",
			"new_text":        "history",
			"workspace_roots": []any{root},
		},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("apply edit: %v", err)
	}
	if resp.AgentActionID == "" {
		t.Fatalf("missing action id: %+v", resp)
	}
	if b, _ := os.ReadFile(path); string(b) != "hello history" {
		t.Fatalf("after apply = %q", string(b))
	}
	_, err = h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": resp.AgentActionID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "hello vit" {
		t.Fatalf("after rollback = %q", string(b))
	}
	action, ok := h.journal.Get(resp.AgentActionID)
	if !ok {
		t.Fatalf("missing journal action %s", resp.AgentActionID)
	}
	if action.Status != journal.StatusRolledBack || action.RollbackState != "succeeded" {
		t.Fatalf("rollback journal = %+v", action)
	}
}

func TestRollbackDAWActionCallsProjectUndo(t *testing.T) {
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "agent_action_id": "undo_1", "message": "undone"},
		},
	}
	h := New(nil, nil, nil)
	h.kernel = kernel
	h.journal.Record(journal.Action{
		AgentActionID: "act_daw",
		Domain:        "daw",
		Source:        "test",
		Tool:          "track.mute",
		CommandName:   "set_mute",
		Command:       map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true},
		Status:        journal.StatusSucceeded,
	})

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": "act_daw"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if resp.Status != "ok" || len(kernel.commands) != 1 {
		t.Fatalf("resp=%+v commands=%+v", resp, kernel.commands)
	}
	if kernel.commands[0]["cmd"] != "undo" || kernel.commands[0]["target_action_id"] != "act_daw" {
		t.Fatalf("undo command = %+v", kernel.commands[0])
	}
	action, ok := h.journal.Get("act_daw")
	if !ok {
		t.Fatal("target action missing")
	}
	if action.Status != journal.StatusRolledBack || action.RollbackActionID != "undo_1" || action.RollbackState != "succeeded" {
		t.Fatalf("rollback journal = %+v", action)
	}
}

func TestRollbackNonAutomaticDomainIsRejected(t *testing.T) {
	h := New(nil, nil, nil)
	h.journal.Record(journal.Action{
		AgentActionID: "act_web",
		Domain:        "web",
		Source:        "test",
		Tool:          "web.fetch",
		CommandName:   "web_fetch",
		Command:       map[string]any{"cmd": "web_fetch", "url": "https://example.com"},
		Status:        journal.StatusSucceeded,
	})
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": "act_web"},
		Confirmed: true,
		Source:    "test",
	})
	if err == nil {
		t.Fatalf("expected rollback rejection, resp=%+v", resp)
	}
	if !strings.Contains(err.Error(), `domain "web"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveToolBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"track_id": "1007", "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if spec.CommandName != "set_mute" || cmd["cmd"] != "set_mute" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestResolveToolFormCommandBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "track.solo",
			"args": map[string]any{
				"track_id": "1007",
				"enabled":  "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if spec.CommandName != "set_solo" || cmd["cmd"] != "set_solo" || cmd["track_id"] != "1007" || cmd["solo"] != true {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}

func TestResolvePluginTargetFromContext(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "get_plugin_parameters",
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_plugin_id":       "plugin_a",
		"selected_plugin_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" || cmd["plugin_id"] != "plugin_a" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestPublicPluginParametersResultStripsRetiredRuntimeProfile(t *testing.T) {
	out := publicPluginParametersResult(nil, map[string]any{
		"status":                 "ok",
		"track_id":               "1007",
		"plugin_id":              "plugin_a",
		"global_profile_applied": true,
		"global_profile_source":  "global_profile",
		"plugin_class":           "compressor",
		"plugin_groups": []any{
			map[string]any{"id": "main_dynamics", "role": "compressor", "label": "Main dynamics"},
		},
		"virtual_controls": []any{
			map[string]any{"name": "tighten dynamics", "component_id": "main_dynamics", "resolver": "local_profile_mapping"},
		},
		"safety_limits": map[string]any{"max_gain_change_db": 6},
		"global_profile": map[string]any{
			"plugin_skill": map[string]any{
				"schema_version": 2,
				"components": []any{
					map[string]any{
						"id":     "main_dynamics",
						"role":   "compressor",
						"params": map[string]any{"threshold": map[string]any{"param_id": "threshold", "confidence": 0.9}},
					},
				},
				"operations": []any{
					map[string]any{"name": "tighten dynamics", "component_id": "main_dynamics", "params": map[string]any{"threshold": "threshold"}},
				},
			},
		},
		"quick_controls": []any{
			map[string]any{"param_id": "threshold", "label": "Threshold"},
		},
		"recommended_groups": []any{},
		"parameters": []any{
			map[string]any{"id": "threshold"},
		},
	})
	for _, key := range []string{"plugin_class", "global_profile_applied", "plugin_groups", "virtual_controls", "safety_limits", "plugin_skill"} {
		if _, ok := out[key]; ok {
			t.Fatalf("retired profile field %q leaked into public result: %+v", key, out)
		}
	}
}

func TestPluginLoadToRackNormalizesArgs(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path": "C:/Program Files/Common Files/VST3/TDR Nova.vst3",
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "rack_add_node" || cmd["track_id"] != "1007" || cmd["plugin_path"] == "" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["x"] == nil || cmd["y"] == nil || cmd["zone_id"] != "Z3" {
		t.Fatalf("rack defaults missing: %+v", cmd)
	}
}

func TestBroadMixRequestCannotLoadPluginThroughHarness(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":     `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
			"track_id": "1007",
		},
		Context: map[string]any{
			"user_message": "我想你帮我对这段音频进行缩混可以吗",
		},
		Confirmed: true,
	})
	if err == nil {
		t.Fatal("expected broad mix plugin load to be blocked")
	}
	if resp.Status != "error" || !strings.Contains(resp.Error, "mix.request_observation") {
		t.Fatalf("resp = %+v err=%v", resp, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("blocked command reached kernel: %+v", kernel.commands)
	}
}

func TestQualifiedSemanticPluginSelectionCanLoadOnlyExactCandidateThroughHarness(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	const (
		trackID    = "1007"
		pluginPath = `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`
		identifier = "VST3-Pro-Q-3"
	)
	baseContext := map[string]any{
		"user_message": "减少一些浑浊",
		// These client-shaped fields alone must not bypass the guard.
		"semantic_plugin_recommendation_selection": true,
		"semantic_plugin_recommendation_candidate": map[string]any{
			"plugin_path": pluginPath, "identifier": identifier,
		},
	}

	unauthorizedKernel := &fakeKernelClient{}
	unauthorizedHarness := New(nil, nil, nil)
	unauthorizedHarness.kernel = unauthorizedKernel
	if _, err := unauthorizedHarness.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack", Args: map[string]any{
			"path": pluginPath, "track_id": trackID, "plugin_identifier": identifier,
		}, Context: baseContext, Confirmed: true,
	}); err == nil {
		t.Fatal("client-shaped semantic selection bypassed the harness guard")
	}
	if len(unauthorizedKernel.commands) != 0 {
		t.Fatalf("unauthorized command reached kernel: %+v", unauthorizedKernel.commands)
	}

	authorizedKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "1015"}}}
	authorizedHarness := New(nil, nil, nil)
	authorizedHarness.kernel = authorizedKernel
	authorizedContext := AuthorizeSemanticPluginSelectionLoad(baseContext, trackID, pluginPath, identifier)
	resp, err := authorizedHarness.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack", Args: map[string]any{
			"path": pluginPath, "track_id": trackID, "plugin_identifier": identifier,
		}, Context: authorizedContext, Confirmed: true,
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("qualified semantic selection should load exact candidate, resp=%+v err=%v", resp, err)
	}
	foundRackAdd := false
	for _, command := range authorizedKernel.commands {
		if command["cmd"] == "rack_add_node" {
			foundRackAdd = true
		}
	}
	if !foundRackAdd {
		t.Fatalf("authorized load did not reach kernel: %+v", authorizedKernel.commands)
	}
}

func TestQualifiedSemanticPluginSelectionAuthorizationRejectsTamperingAndOtherWrites(t *testing.T) {
	const (
		trackID    = "1007"
		pluginPath = `C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3`
		identifier = "VST3-Pro-Q-3"
	)
	ctx := AuthorizeSemanticPluginSelectionLoad(map[string]any{"user_message": "减少一些浑浊"}, trackID, pluginPath, identifier)
	tests := []struct {
		name string
		spec tools.CommandSpec
		cmd  map[string]any
	}{
		{name: "other path", spec: tools.CommandSpec{CommandName: "rack_add_node"}, cmd: map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": `C:\VST3\Other.vst3`, "plugin_identifier": identifier}},
		{name: "other track", spec: tools.CommandSpec{CommandName: "rack_add_node"}, cmd: map[string]any{"cmd": "rack_add_node", "track_id": "9999", "plugin_path": pluginPath, "plugin_identifier": identifier}},
		{name: "other identifier", spec: tools.CommandSpec{CommandName: "rack_add_node"}, cmd: map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": pluginPath, "plugin_identifier": "other-id"}},
		{name: "parameter write", spec: tools.CommandSpec{CommandName: "set_plugin_param"}, cmd: map[string]any{"cmd": "set_plugin_param", "track_id": trackID, "plugin_id": "1015", "param_id": "1", "value": 0.5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := broadMixObserveFirstWriteGuard(ctx, test.spec, test.cmd); err == nil {
				t.Fatalf("authorization escaped exact rack load boundary: spec=%+v cmd=%+v", test.spec, test.cmd)
			}
		})
	}
}

func TestNamedPluginProfileApplyIsRejectedByHarness(t *testing.T) {
	spec := tools.CommandSpec{ToolName: "plugin_grabber.apply_control", CommandName: "plugin_grabber_apply_control"}
	cmd := map[string]any{
		"cmd":       "plugin_grabber_apply_control",
		"track_id":  "1007",
		"plugin_id": "1013",
		"control":   "eq.cut_region",
		"target":    map[string]any{"freq_hz": 200.0, "gain_db": -2.5},
	}
	ctx := map[string]any{
		"user_message": "用 Pro-Q 3 切掉 200Hz 附近的浑浊",
		"tracks": []any{map[string]any{
			"track_id": "1007",
			"rack": map[string]any{"nodes": []any{map[string]any{
				"plugin_id": "1013", "plugin_name": "Pro-Q 3",
			}}},
		}},
	}
	if err := broadMixObserveFirstWriteGuard(ctx, spec, cmd); err == nil {
		t.Fatal("retired profile apply reached the harness")
	}
}

func TestExplicitPluginLoadStillReachesHarness(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "1015"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":     `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
			"track_id": "1007",
		},
		Context: map[string]any{
			"user_message": "请直接加载 TDR Nova 插件",
		},
		Confirmed: true,
	})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("explicit plugin load should be allowed, resp=%+v err=%v", resp, err)
	}
	foundRackAdd := false
	for _, cmd := range kernel.commands {
		if cmd["cmd"] == "rack_add_node" {
			foundRackAdd = true
			break
		}
	}
	if !foundRackAdd {
		t.Fatalf("explicit plugin load did not reach kernel: %+v", kernel.commands)
	}
}

func TestPluginLoadToRackInstrumentDefaultsToZ2FromSemanticIndex(t *testing.T) {
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":          "Surge XT Effects",
			"category":      "Fx",
			"plugin_path":   `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT Effects.vst3\Contents\x86_64-win\Surge XT Effects.vst3`,
			"is_instrument": false,
		},
		{
			"name":          "Surge XT",
			"category":      "Instrument|Synth",
			"plugin_path":   surgePath,
			"is_instrument": true,
		},
	}, time.Now().UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"path": surgePath},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want Z2; cmd=%+v", got, cmd)
	}
}

func TestPluginLoadToRackInstrumentOverridesPlannerZ3(t *testing.T) {
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":          "Surge XT",
			"category":      "Instrument|Synth",
			"plugin_path":   surgePath,
			"is_instrument": true,
		},
	}, time.Now().UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{
			"path":    surgePath,
			"zone_id": "Z3",
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want corrected Z2; cmd=%+v", got, cmd)
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "Zone Z3") || !strings.Contains(preview, "Zone Z2") {
		t.Fatalf("preview did not show corrected zone:\n%s", preview)
	}
}

func TestPluginLoadToRackInstrumentDefaultsToZ2FromScannedPluginInventory(t *testing.T) {
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(t.TempDir(), "missing_plugin_semantics.json"))
	surgePath := `C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3`
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":          "Surge XT",
					"category":      "Instrument|Synth",
					"path":          surgePath,
					"is_instrument": true,
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"path": surgePath},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if got := cmd["zone_id"]; got != "Z2" {
		t.Fatalf("zone_id = %#v, want Z2; cmd=%+v", got, cmd)
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestPluginSemanticSearchMergesLivePluginSearch(t *testing.T) {
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	idx := pluginsemantics.Build([]map[string]any{
		{
			"name":        "Old Reverb",
			"category":    "Fx|Reverb",
			"plugin_path": `C:\Program Files\Common Files\VST3\Old Reverb.vst3`,
		},
	}, time.Unix(10, 0).UTC())
	if _, err := pluginsemantics.Save(semanticsPath, idx); err != nil {
		t.Fatalf("save semantics: %v", err)
	}
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)

	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":               "Live Compressor",
					"category":           "Fx|Dynamics",
					"file_or_identifier": "VST3-Live Compressor-1234",
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	result, err := h.searchPluginSemanticIndex(map[string]any{"query": "compressor", "type": "compressor", "limit": 4})
	if err != nil {
		t.Fatalf("searchPluginSemanticIndex: %v", err)
	}
	entries, _ := result["entries"].([]pluginsemantics.Entry)
	if len(entries) == 0 {
		t.Fatalf("expected live semantic entry, result=%+v", result)
	}
	if entries[0].Name != "Live Compressor" {
		t.Fatalf("top entry = %+v", entries[0])
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestPluginSemanticBuildIndexUsesScannedPluginInventory(t *testing.T) {
	semanticsPath := filepath.Join(t.TempDir(), "plugin_semantics.json")
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"plugins": []any{
				map[string]any{
					"name":               "Live Compressor",
					"category":           "Fx|Dynamics",
					"file_or_identifier": "VST3-Live Compressor-1234",
				},
			},
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	result, err := h.buildPluginSemanticIndex(map[string]any{"index_path": semanticsPath})
	if err != nil {
		t.Fatalf("buildPluginSemanticIndex: %v", err)
	}
	if result["plugin_count"] != 1 || result["source"] != "scan_plugins" {
		t.Fatalf("unexpected result = %+v", result)
	}
	if len(kernel.commands) != 1 || kernel.commands[0]["cmd"] != "scan_plugins" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	idx, err := pluginsemantics.Load(semanticsPath)
	if err != nil {
		t.Fatalf("load semantic index: %v", err)
	}
	if len(idx.Entries) != 1 || idx.Entries[0].Name != "Live Compressor" {
		t.Fatalf("index entries = %+v", idx.Entries)
	}
}

func TestScanAvailablePluginRowsPollsAsyncScanAndListsCompletedInventory(t *testing.T) {
	wavesPath := `C:\Program Files\Common Files\VST3\WaveShell1-VST3 17.0_x64.vst3`
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "scanning", "scan_id": "scan-waves", "completed_files": 0, "total_files": 1},
		{"status": "scanning", "scan_id": "scan-waves", "completed_files": 0, "total_files": 1},
		{"status": "completed", "scan_id": "scan-waves", "completed_files": 1, "total_files": 1},
		{"status": "ok", "plugins": []any{map[string]any{"name": "Waves SSL EV2", "path": wavesPath}}},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	rows, paths, err := h.scanAvailablePluginRows(context.Background(), map[string]any{
		"paths":                 []string{`C:\Program Files\Common Files\VST3`},
		"scan_poll_interval_ms": 10,
	})
	if err != nil {
		t.Fatalf("scanAvailablePluginRows: %v", err)
	}
	if len(rows) != 1 || firstString(rows[0], "name") != "Waves SSL EV2" || len(paths) != 1 {
		t.Fatalf("rows=%+v paths=%+v", rows, paths)
	}
	var commands []string
	for _, command := range kernel.commands {
		commands = append(commands, firstString(command, "cmd"))
	}
	want := []string{"scan_plugins", "plugin_scan_status", "plugin_scan_status", "plugin_list_available"}
	if fmt.Sprint(commands) != fmt.Sprint(want) {
		t.Fatalf("commands=%v want=%v", commands, want)
	}
	if got := firstString(kernel.commands[1], "scan_id"); got != "scan-waves" {
		t.Fatalf("status scan_id=%q", got)
	}
}

func TestScanAvailablePluginRowsHonorsContextCancellationWhilePolling(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "scanning", "scan_id": "scan-cancel"},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := h.scanAvailablePluginRows(ctx, map[string]any{"scan_poll_interval_ms": 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context.Canceled", err)
	}
	if len(kernel.commands) != 1 || firstString(kernel.commands[0], "cmd") != "scan_plugins" {
		t.Fatalf("kernel commands=%+v", kernel.commands)
	}
}

func TestPluginParametersPublicResultIsCompact(t *testing.T) {
	h := New(nil, nil, nil)
	result := h.publicResult(tools.CommandSpec{CommandName: "get_plugin_parameters"}, nil, map[string]any{
		"status":     "ok",
		"track_id":   "1007",
		"plugin_id":  "plugin_a",
		"parameters": []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}},
		"quick_controls": []any{
			map[string]any{"param_id": "a", "label": "A", "display_group": "Mix"},
		},
		"recommended_groups": []any{
			map[string]any{"name": "Mix", "parameter_ids": []any{"a", "b"}},
		},
	})
	if result["parameter_count"] != 2 || result["quick_control_count"] != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if _, ok := result["parameters"]; ok {
		t.Fatalf("public result leaked full parameters: %+v", result)
	}
	if summary, ok := result["display_probe_summary"].(map[string]any); !ok || summary["parameter_count"] != 2 {
		t.Fatalf("display probe summary = %+v", result["display_probe_summary"])
	}
}

func TestPluginParametersPublicResultKeepsCompactDisplayProbeWhenIncluded(t *testing.T) {
	h := New(nil, nil, nil)
	result := h.publicResult(tools.CommandSpec{CommandName: "get_plugin_parameters"}, map[string]any{"include_parameters": true}, map[string]any{
		"status":    "ok",
		"track_id":  "1007",
		"plugin_id": "plugin_a",
		"parameters": []any{map[string]any{
			"id":                "delay",
			"name":              "Delay",
			"host_controllable": true,
			"display_probe": map[string]any{
				"mode":  "read_only_value_to_string",
				"label": "ms",
				"samples": []any{
					map[string]any{"normalized_value": 0.0, "value": 0.0, "text": "0 ms"},
					map[string]any{"normalized_value": 1.0, "value": 1.0, "text": "2000 ms"},
				},
			},
		}},
	})
	params := mapRowsFromAny(result["parameters"])
	if len(params) != 1 {
		t.Fatalf("parameters = %+v", result["parameters"])
	}
	if _, ok := params[0]["display_probe"]; !ok {
		t.Fatalf("compact parameter missing display_probe: %+v", params[0])
	}
	if domain, ok := params[0]["display_domain_candidate"].(*plugingrabber.PluginDisplayDomain); !ok || domain.Unit != "ms" {
		t.Fatalf("display_domain_candidate = %+v", params[0]["display_domain_candidate"])
	}
}

func TestProjectAudioSettingsPublicResultKeepsUserFacingFields(t *testing.T) {
	h := New(nil, nil, nil)
	result := h.publicResult(tools.CommandSpec{CommandName: "project.get_audio_settings"}, nil, map[string]any{
		"status":  "ok",
		"command": "project.get_audio_settings",
		"audio_settings": map[string]any{
			"sample_rate_hz":            48000,
			"record_bit_depth":          24,
			"record_file_type":          "WAV/BWF",
			"import_sample_rate_policy": "ask",
			"recommended_presets": []any{map[string]any{
				"preset_id":                    "cd_export",
				"name":                         "CD Export",
				"role":                         "delivery_export",
				"sample_rate_hz":               44100,
				"bit_depth":                    16,
				"file_type":                    "WAV",
				"default_project_working_spec": false,
				"raw_internal_note":            strings.Repeat("x", 128),
			}},
			"capabilities": map[string]any{
				"project_sample_rate_is_audio_device_sample_rate": false,
				"changes_audio_device_sample_rate":                false,
			},
		},
		"warnings": []any{map[string]any{
			"code":                        "project_sample_rate_differs_from_audio_device",
			"project_sample_rate_hz":      48000,
			"audio_device_sample_rate_hz": 44100,
			"debug_internal_blob":         strings.Repeat("x", 128),
		}},
	})
	settings := testMap(t, result["audio_settings"])
	if settings["sample_rate_hz"] != 48000 || settings["record_bit_depth"] != 24 || settings["record_file_type"] != "WAV/BWF" {
		t.Fatalf("settings = %+v", settings)
	}
	presets := mapRowsFromAny(settings["recommended_presets"])
	if len(presets) != 1 || presets[0]["preset_id"] != "cd_export" || presets[0]["default_project_working_spec"] != false {
		t.Fatalf("recommended presets = %+v", presets)
	}
	if _, ok := presets[0]["raw_internal_note"]; ok {
		t.Fatalf("preset leaked internal field: %+v", presets[0])
	}
	warnings := anySliceFromAny(result["warnings"])
	if len(warnings) != 1 {
		t.Fatalf("warnings = %+v", result["warnings"])
	}
	if _, ok := testMap(t, warnings[0])["debug_internal_blob"]; ok {
		t.Fatalf("warning leaked debug field: %+v", warnings[0])
	}
}

func TestAudioPreflightPublicResultIsCompact(t *testing.T) {
	h := New(nil, nil, nil)
	files := []any{}
	for i := 0; i < 12; i++ {
		files = append(files, map[string]any{
			"file_path":            fmt.Sprintf("E:/stems/stem_%02d.wav", i),
			"file_name":            fmt.Sprintf("stem_%02d.wav", i),
			"readable":             true,
			"duration_seconds":     12.5,
			"sample_rate_hz":       44100,
			"bit_depth":            16,
			"channel_count":        2,
			"suggested_track_name": fmt.Sprintf("stem %02d", i),
			"large_internal_field": strings.Repeat("raw", 100),
		})
	}
	result := h.publicResult(tools.CommandSpec{CommandName: "project.import_preflight"}, nil, map[string]any{
		"status":  "ok",
		"command": "project.import_preflight",
		"summary": map[string]any{
			"discovered_audio_file_count":        12,
			"readable_file_count":                12,
			"tracks_to_create":                   12,
			"sample_rate_mismatch_count":         12,
			"bit_depth_or_format_mismatch_count": 12,
			"requires_user_confirmation":         true,
		},
		"audio_settings_snapshot": map[string]any{"sample_rate_hz": 48000, "record_bit_depth": 24},
		"files":                   files,
		"import_plan": map[string]any{
			"tracks_to_create": 12,
			"track_plan":       files,
			"sample_rate_mismatches": []any{map[string]any{
				"file_path":              "E:/stems/stem_00.wav",
				"source_sample_rate_hz":  44100,
				"project_sample_rate_hz": 48000,
				"policy":                 "ask",
			}},
			"bit_depth_or_format_mismatches": []any{map[string]any{
				"file_path":                "E:/stems/stem_00.wav",
				"source_bit_depth":         16,
				"project_record_bit_depth": 24,
				"policy":                   "keep_source",
			}},
		},
	})
	if _, ok := result["files"]; ok {
		t.Fatalf("public result leaked full files list: %+v", result)
	}
	preview := mapRowsFromAny(result["file_preview"])
	if len(preview) != 8 {
		t.Fatalf("file preview len=%d result=%+v", len(preview), result)
	}
	if _, ok := preview[0]["large_internal_field"]; ok {
		t.Fatalf("file preview leaked large field: %+v", preview[0])
	}
	plan := testMap(t, result["import_plan"])
	if len(mapRowsFromAny(plan["track_plan_preview"])) != 8 || len(mapRowsFromAny(plan["sample_rate_mismatch_examples"])) != 1 {
		t.Fatalf("compact plan = %+v", plan)
	}
}

func TestStemsImportPublicResultIsCompact(t *testing.T) {
	h := New(nil, nil, nil)
	rows := []any{}
	ids := []any{}
	for i := 0; i < 20; i++ {
		trackID := fmt.Sprintf("track_%02d", i)
		ids = append(ids, trackID)
		rows = append(rows, map[string]any{
			"track_id":             trackID,
			"track_name":           fmt.Sprintf("stem %02d", i),
			"clip_id":              fmt.Sprintf("clip_%02d", i),
			"clip_name":            fmt.Sprintf("stem %02d", i),
			"source_file_path":     fmt.Sprintf("E:/stems/stem_%02d.wav", i),
			"duration_seconds":     12.5,
			"sample_rate_hz":       44100,
			"bit_depth":            16,
			"channel_count":        2,
			"baking_status":        "baking_started",
			"large_internal_field": strings.Repeat("raw", 100),
		})
	}
	result := h.publicResult(tools.CommandSpec{CommandName: "project.import_folder_as_stems"}, nil, map[string]any{
		"status":                "ok",
		"command":               "project.import_folder_as_stems",
		"last_created_track_id": "track_19",
		"last_created_clip_id":  "clip_19",
		"summary": map[string]any{
			"tracks_created":      20,
			"clips_created":       20,
			"edit_length_seconds": 12.5,
		},
		"created_track_ids": ids,
		"created_clip_ids":  ids,
		"imported_tracks":   rows,
	})
	if _, ok := result["imported_tracks"]; ok {
		t.Fatalf("public result leaked full imported_tracks: %+v", result)
	}
	preview := mapRowsFromAny(result["imported_tracks_preview"])
	if len(preview) != 12 {
		t.Fatalf("import preview len=%d result=%+v", len(preview), result)
	}
	if _, ok := preview[0]["large_internal_field"]; ok {
		t.Fatalf("import preview leaked large field: %+v", preview[0])
	}
	refs := mapRowsFromAny(result["imported_track_refs"])
	if len(refs) != 20 {
		t.Fatalf("import refs len=%d result=%+v", len(refs), result)
	}
	if _, ok := refs[0]["large_internal_field"]; ok {
		t.Fatalf("import refs leaked large field: %+v", refs[0])
	}
	if refs[19]["track_id"] != "track_19" || refs[19]["clip_id"] != "clip_19" {
		t.Fatalf("import refs should keep all imported identities: %+v", refs[19])
	}
	if result["last_created_track_id"] != "track_19" || result["last_created_clip_id"] != "clip_19" {
		t.Fatalf("last created IDs missing: %+v", result)
	}
	timProjection := mapFromAny(result["tim_projection"])
	if timProjection["schema_version"] != tim.SchemaVersion {
		t.Fatalf("TIM projection missing from public import result: %+v", result)
	}
	timSummary := mapFromAny(timProjection["technical_summary"])
	if got, _ := firstPositiveInt(timSummary, "track_count"); got != 20 {
		t.Fatalf("TIM projection should use all imported rows before preview capping, got %d summary=%+v", got, timSummary)
	}
}

func TestNormalizeTrackBooleanAliases(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		tool  string
		args  map[string]any
		field string
		want  bool
	}{
		{"track.mute", map[string]any{"track_id": "1007", "enabled": "on"}, "mute", true},
		{"track.mute", map[string]any{"track_id": "1007", "muted": "0"}, "mute", false},
		{"track.solo", map[string]any{"track_id": "1007", "value": 1}, "solo", true},
		{"track.arm", map[string]any{"track_id": "1007", "armed": "false"}, "is_armed", false},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Tool: tc.tool, Args: tc.args})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.tool, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.tool, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.tool, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInferMissingTrackBooleanFromUserMessage(t *testing.T) {
	h := New(nil, nil, nil)
	cases := []struct {
		name    string
		command map[string]any
		context map[string]any
		field   string
		want    bool
	}{
		{
			name:    "mute on",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "我想让track1静音"},
			field:   "mute",
			want:    true,
		},
		{
			name:    "mute off",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007"},
			context: map[string]any{"user_message": "取消静音"},
			field:   "mute",
			want:    false,
		},
		{
			name:    "mute off overrides model true",
			command: map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true},
			context: map[string]any{"user_message": "取消静音"},
			field:   "mute",
			want:    false,
		},
		{
			name:    "solo on",
			command: map[string]any{"cmd": "set_solo", "track_id": "1012"},
			context: map[string]any{"user_message": "track 2 solo"},
			field:   "solo",
			want:    true,
		},
		{
			name:    "solo off overrides model true",
			command: map[string]any{"cmd": "set_solo", "track_id": "1012", "solo": true},
			context: map[string]any{"user_message": "取消solo"},
			field:   "solo",
			want:    false,
		},
	}
	for _, tc := range cases {
		cmd, spec, err := h.resolveCommand(InvokeRequest{Command: tc.command})
		if err != nil {
			t.Fatalf("%s resolveCommand: %v", tc.name, err)
		}
		if err := h.resolveImplicitTargets(context.Background(), spec, cmd, tc.context); err != nil {
			t.Fatalf("%s resolveImplicitTargets: %v", tc.name, err)
		}
		if got := cmd[tc.field]; got != tc.want {
			t.Fatalf("%s %s = %#v, want %#v; cmd=%+v", tc.name, tc.field, got, tc.want, cmd)
		}
	}
}

func TestInvokeRejectsMissingRequiredTargetID(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "track.rename",
		Args:   map[string]any{"name": "Lead Vocal"},
		Source: "test",
	})
	if err == nil {
		t.Fatal("expected missing target id error")
	}
	if resp.Status != "error" || resp.CommandName != "rename_track" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveImplicitTrackIDFromSingleVisibleTrack(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Lead Vocal"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" {
		t.Fatalf("track_id = %#v, want 1007", cmd["track_id"])
	}
	if cmd["name"] != "Lead Vocal" {
		t.Fatalf("name = %#v, want Lead Vocal", cmd["name"])
	}
}

func TestResolveFlattensNestedParams(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "rename_track",
			"params": map[string]any{
				"track_id": "1007",
				"new_name": "A",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["params"]; ok {
		t.Fatalf("params was not flattened: %+v", cmd)
	}
	if cmd["track_id"] != "1007" || cmd["name"] != "A" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveFlattensNestedArgsOnCommand(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
				"new_start":       10.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if _, ok := cmd["args"]; ok {
		t.Fatalf("args was not flattened: %+v", cmd)
	}
	if cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" || cmd["new_start"] != 10.0 {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveToolFormRemoveClipStringID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"tool": "clip.remove",
			"args": map[string]any{"clip_ids": "clip_a"},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 1 || got[0] != "clip_a" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestResolveImplicitTrackIDByUserTrackIndex(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Track 2", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"user_track_index": 2, "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveImplicitTrackIDFromSelectedContext(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Drums", "track_type": "hybrid", "is_audio_track": true},
			map[string]any{"track_id": "1010", "track_name": "Bass", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.rename",
		Args: map[string]any{"new_name": "Low End"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id":   "1010",
		"selected_track_name": "Bass",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" {
		t.Fatalf("track_id = %#v, want 1010", cmd["track_id"])
	}
}

func TestResolveClipResizeFromSelectedContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.resize",
		Args: map[string]any{"new_length": 4.5},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveClipResizeInfersLengthFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "resize_clip",
			"args": map[string]any{
				"clip_id": "clip_b",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把选中的clip裁到2秒",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_length"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("length args = %+v", cmd)
	}
}

func TestResolveClipSplitFromSelectedContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{"split_time": 1.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["split_time"] != 1.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveClipSplitInfersPlayheadFromContext(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "在播放头这里切开选中的clip",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["split_time"] != 1.25 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveClipSplitOverridesModelZeroForPlayhead(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.split",
		Args: map[string]any{"split_time": 0.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "split the selected clip at the playhead",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       1.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["split_time"] != 1.25 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestInvokeClipSelectIsLocalUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"clip_name": "Loop B"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.CommandName != "select_clip" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Result["ui_action"] != "select_clip" || resp.Result["clip_id"] != "clip_b" || resp.Result["track_id"] != "1010" {
		t.Fatalf("result = %+v", resp.Result)
	}
}

func TestVersionCheckpointUsesDraftProjectWhenProjectPathMissing(t *testing.T) {
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", t.TempDir())
	h := New(nil, shadow.New(nil), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.checkpoint",
		Args:      map[string]any{"message": "smoke"},
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if resp.Status != "ok" || resp.Tool != "version.checkpoint" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Result["project_path"] == "" || resp.Result["commit_id"] == "" || resp.Result["draft"] != true {
		t.Fatalf("draft result = %+v", resp.Result)
	}
	if resp.ProjectHistory["draft"] != true || resp.ProjectHistory["project_path"] == "" {
		t.Fatalf("project history = %+v", resp.ProjectHistory)
	}
}

func TestConfirmedMutatingToolCreatesOneGoalBaseline(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("<EDIT disk=\"one\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status":       "ok",
		"project_path": projectPath,
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Drums", "clips": []any{
				map[string]any{"id": "clip_a", "name": "Loop A"},
			}},
		},
	})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "snapshot_xml": "<EDIT memory=\"baseline\"/>", "project_path": projectPath},
			{"status": "ok", "saved": true},
			{"status": "ok", "saved": true},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	first, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.save",
		Confirmed: true,
		GoalID:    "goal_baseline",
		RunID:     "run_one",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	if first.Status != "ok" || first.ProjectHistory["baseline_commit"] == "" {
		t.Fatalf("first response = %+v", first)
	}
	second, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.save",
		Confirmed: true,
		GoalID:    "goal_baseline",
		RunID:     "run_two",
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	baselineID := fmt.Sprint(first.ProjectHistory["baseline_commit"])
	if fmt.Sprint(second.ProjectHistory["baseline_commit"]) != baselineID {
		t.Fatalf("baseline changed: first=%+v second=%+v", first.ProjectHistory, second.ProjectHistory)
	}
	if len(kernel.commands) != 3 || kernel.commands[0]["cmd"] != "project.snapshot_export" || kernel.commands[1]["cmd"] != "save_project" || kernel.commands[2]["cmd"] != "save_project" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	actions := h.Actions(2)
	if len(actions) != 2 || actions[0].VersionCommitID != baselineID || actions[1].VersionCommitID != baselineID {
		t.Fatalf("actions = %+v baseline=%s", actions, baselineID)
	}
	status, err := history.Status(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("history status: %v", err)
	}
	if status["commit_count"] != 1 {
		t.Fatalf("history status = %+v", status)
	}
	goal := h.RuntimeStatus("goal_baseline")
	if goal.ProjectHistory == nil || goal.ProjectHistory.BaselineCommitID != baselineID {
		t.Fatalf("goal project history = %+v", goal.ProjectHistory)
	}
}

func TestProjectHistoryCheckoutReloadsKernelAndShadow(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstCommit := firstResult["commit"].(history.Commit)
	if err := os.WriteFile(projectPath, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Checkpoint(map[string]any{"project_path": projectPath}); err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok"},
			{"status": "ok", "project_path": projectPath},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.checkout",
		Args:      map[string]any{"commit_id": firstCommit.ID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["commit_id"] != firstCommit.ID {
		t.Fatalf("resp = %+v", resp)
	}
	if got, err := os.ReadFile(projectPath); err != nil || string(got) != "one" {
		t.Fatalf("project after checkout = %q err=%v", string(got), err)
	}
	if len(kernel.commands) != 2 || kernel.commands[0]["cmd"] != "open_project" || kernel.commands[0]["file_path"] != projectPath || kernel.commands[1]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	refresh := resp.Result["refresh"].(map[string]any)
	if refresh["kernel_reloaded"] != true || refresh["shadow_refreshed"] != true {
		t.Fatalf("refresh = %+v", refresh)
	}
}

func TestProjectHistoryWorktreeCheckoutOpensTargetProject(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := checkpoint["commit"].(history.Commit)
	worktree, err := history.WorktreeCreate(map[string]any{"project_path": projectPath, "commit_id": commit.ID, "name": "wt-1"})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	targetProject := fmt.Sprint(worktree["project_file_path"])
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok"},
			{"status": "ok", "project_path": targetProject},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.worktree_checkout",
		Args:      map[string]any{"name": "wt-1"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("worktree checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["project_file_path"] != targetProject {
		t.Fatalf("resp = %+v target=%s", resp, targetProject)
	}
	if len(kernel.commands) != 3 ||
		kernel.commands[0]["cmd"] != "project.snapshot_export" ||
		kernel.commands[1]["cmd"] != "open_project" ||
		kernel.commands[1]["file_path"] != targetProject ||
		kernel.commands[2]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestProjectHistoryWorktreeCheckoutAutosaveUsesKernelProjectPath(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(projectPath, []byte("root-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := checkpoint["commit"].(history.Commit)
	worktree, err := history.WorktreeCreate(map[string]any{"project_path": projectPath, "commit_id": commit.ID, "name": "wt-1"})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	targetProject := fmt.Sprint(worktree["project_file_path"])
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{"status": "ok", "snapshot_xml": "worktree-live", "project_path": targetProject},
			{"status": "ok"},
			{"status": "ok", "project_path": projectPath},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.worktree_checkout",
		Args:      map[string]any{"name": "main"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("worktree checkout invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["project_file_path"] != projectPath {
		t.Fatalf("resp = %+v", resp)
	}
	rootList, err := history.List(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("root list: %v", err)
	}
	if commits := rootList["commits"].([]history.Commit); len(commits) != 1 || commits[0].ID != commit.ID {
		t.Fatalf("root history was polluted by worktree autosave: %+v", commits)
	}
	targetList, err := history.List(map[string]any{"project_path": targetProject})
	if err != nil {
		t.Fatalf("target list: %v", err)
	}
	foundAutosave := false
	for _, item := range targetList["commits"].([]history.Commit) {
		if item.Source == "worktree_checkout" && item.ProjectPath == targetProject {
			foundAutosave = true
			break
		}
	}
	if !foundAutosave {
		t.Fatalf("worktree autosave missing from target history: %+v", targetList["commits"])
	}
}

func TestResolveClipSelectFromCurrentTrackScope(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"track_id": "1007"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["clip_id"] != "clip_a" || resp.Result["track_id"] != "1007" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveClipSelectFromUserTrackIndexScope(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"user_track_index": 1},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["clip_id"] != "clip_a" || resp.Result["track_id"] != "1007" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveClipSelectAmbiguousTrackScopeRequiresClipIndex(t *testing.T) {
	project := shadowProjectWithClips()
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "Loop A"},
					map[string]any{"id": "clip_c", "name": "Loop C"},
				},
			},
		},
	})
	h := New(nil, project, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.select",
		Args: map[string]any{"track_id": "1007"},
	})
	if err == nil {
		t.Fatalf("expected ambiguity error, resp=%+v", resp)
	}
	if resp.Status != "error" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestResolveRemoveClipsFromSelectedIDs(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.remove",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_ids": []any{"clip_a", "clip_b"},
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	got := cmd["clip_ids"].([]string)
	if len(got) != 2 || got[0] != "clip_a" || got[1] != "clip_b" {
		t.Fatalf("clip_ids = %#v", got)
	}
}

func TestRemoveClipsPublicResultIncludesUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.remove",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_ids": []string{"clip_a", "clip_b"}},
		map[string]any{"status": "ok", "missing_ids": []any{"clip_b"}},
	)
	removed := result["removed_clip_ids"].([]string)
	if result["ui_action"] != "remove_clips" || len(removed) != 1 || removed[0] != "clip_a" {
		t.Fatalf("result = %+v", result)
	}
	requested := result["requested_clip_ids"].([]string)
	if len(requested) != 2 || requested[0] != "clip_a" || requested[1] != "clip_b" {
		t.Fatalf("requested = %#v", requested)
	}
}

func TestResolveClipFadeSetDefaultsSelectedClipAndAliases(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.fade.set",
		Args: map[string]any{"fade_in": 0.01, "fade_out": 0.02},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if !spec.RequiresConfirmation || spec.RiskLevel != tools.RiskConfirm {
		t.Fatalf("fade set spec = %+v", spec)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["fade_in_seconds"] != 0.01 || cmd["fade_out_seconds"] != 0.02 {
		t.Fatalf("fade args = %+v", cmd)
	}
}

func TestClipFadeGainPublicResultIncludesUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	for _, toolName := range []string{"clip.fade.read", "clip.gain.set"} {
		_, spec, err := h.resolveCommand(InvokeRequest{
			Tool: toolName,
			Args: map[string]any{"clip_id": "clip_a"},
		})
		if err != nil {
			t.Fatalf("resolveCommand %s: %v", toolName, err)
		}
		result := h.publicResult(spec,
			map[string]any{"clip_id": "clip_a", "track_id": "1007"},
			map[string]any{"status": "ok", "clip_id": "clip_a", "track_id": "1007"},
		)
		if result["ui_action"] != "clip_state_changed" || result["clip_id"] != "clip_a" || result["track_id"] != "1007" {
			t.Fatalf("%s result = %+v", toolName, result)
		}
	}
}

func TestApplyClipGainBatchUsesOneNativeKernelCommand(t *testing.T) {
	kernelClient := &fakeKernelClient{replies: []map[string]any{
		{
			"status":         "ok",
			"schema_version": "clip.gain.set_batch.v1",
			"applied_count":  2,
			"failed_count":   0,
			"actions": []any{
				map[string]any{"status": "ok", "clip_id": "clip_a", "clip_gain_db": -3.0},
				map[string]any{"status": "ok", "clip_id": "clip_b", "clip_gain_db": 2.0},
			},
		},
		{"status": "ok", "tracks": []any{}},
	}}
	h := NewWithSender(kernelClient, shadow.New(nil), nil)
	result, err := h.applyClipGainBatch(context.Background(), map[string]any{
		"pending_actions": []any{
			map[string]any{"clip_id": "clip_a", "track_id": "track_a", "gain_db": -3.0},
			map[string]any{"clip_id": "clip_b", "track_id": "track_b", "gain_db": 2.0},
		},
	})
	if err != nil {
		t.Fatalf("applyClipGainBatch: %v", err)
	}
	if len(kernelClient.commands) != 2 {
		t.Fatalf("kernel commands = %+v", kernelClient.commands)
	}
	if got := tools.CommandName(kernelClient.commands[0]); got != "clip.gain.set_batch" {
		t.Fatalf("first kernel command = %q, want native batch", got)
	}
	if rows := mapRowsFromAny(kernelClient.commands[0]["pending_actions"]); len(rows) != 2 {
		t.Fatalf("native batch pending actions = %+v", kernelClient.commands[0])
	}
	if result["internal_apply_tool"] != "clip.gain.set_batch" || result["applied_count"] != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyStaticBalanceBatchUsesOneNativeKernelCommand(t *testing.T) {
	kernelClient := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok", "schema_version": "track.volume.set_batch.v1", "applied_count": 2, "failed_count": 0,
			"actions": []any{
				map[string]any{"status": "ok", "track_id": "track_a", "volume_db": -2.0},
				map[string]any{"status": "ok", "track_id": "track_b", "volume_db": 1.5},
			},
		},
		{"status": "ok", "tracks": []any{}},
	}}
	h := NewWithSender(kernelClient, shadow.New(nil), nil)
	result, err := h.applyStaticBalanceBatch(context.Background(), map[string]any{
		"observation_id":    "obs_b2",
		"candidate_plan_id": "sbp_1",
		"actions": []any{
			map[string]any{"track_id": "track_a", "before_db": 0.0, "delta_db": -2.0, "target_db": -2.0},
			map[string]any{"track_id": "track_b", "before_db": 0.0, "delta_db": 1.5, "target_db": 1.5},
		},
	})
	if err != nil {
		t.Fatalf("applyStaticBalanceBatch: %v", err)
	}
	if len(kernelClient.commands) != 2 {
		t.Fatalf("kernel commands = %+v", kernelClient.commands)
	}
	if got := tools.CommandName(kernelClient.commands[0]); got != "track.volume.set_batch" {
		t.Fatalf("first kernel command = %q, want native fader batch", got)
	}
	if rows := mapRowsFromAny(kernelClient.commands[0]["actions"]); len(rows) != 2 {
		t.Fatalf("native batch actions = %+v", kernelClient.commands[0])
	}
	if result["internal_apply_tool"] != "track.volume.set_batch" || result["applied_count"] != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyPanLayoutBatchUsesOneNativeKernelCommand(t *testing.T) {
	kernelClient := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok", "schema_version": "track.pan.set_batch.v1", "applied_count": 2,
			"actions": []any{
				map[string]any{"status": "ok", "track_id": "track_a", "pan": -0.5},
				map[string]any{"status": "ok", "track_id": "track_b", "pan": 0.5},
			},
		},
		{"status": "ok", "tracks": []any{}},
	}}
	h := NewWithSender(kernelClient, shadow.New(nil), nil)
	result, err := h.applyPanLayoutBatch(context.Background(), map[string]any{
		"observation_id": "obs_b3", "candidate_plan_id": "pan_1", "style_hash": "style_hash",
		"actions": []any{
			map[string]any{"track_id": "track_a", "before_pan": 0.0, "delta_pan": -0.5, "target_pan": -0.5},
			map[string]any{"track_id": "track_b", "before_pan": 0.0, "delta_pan": 0.5, "target_pan": 0.5},
		},
	})
	if err != nil {
		t.Fatalf("applyPanLayoutBatch: %v", err)
	}
	if len(kernelClient.commands) != 2 {
		t.Fatalf("kernel commands=%+v", kernelClient.commands)
	}
	if got := tools.CommandName(kernelClient.commands[0]); got != "track.pan.set_batch" {
		t.Fatalf("first kernel command=%q, want native pan batch", got)
	}
	if rows := mapRowsFromAny(kernelClient.commands[0]["actions"]); len(rows) != 2 {
		t.Fatalf("native pan actions=%+v", kernelClient.commands[0])
	}
	if result["internal_apply_tool"] != "track.pan.set_batch" || result["internal_apply_count"] != 2 {
		t.Fatalf("result=%+v", result)
	}
}

func TestTrackGroupApplyControlRequiresConfirmationAndUpdatesPublicResult(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Vox", "is_audio_track": true, "volume_db": -12.0},
			map[string]any{"track_id": "1010", "track_name": "Gtr", "is_audio_track": true, "volume_db": -18.0},
		},
		"track_groups": []any{
			map[string]any{"group_id": "grp_v1", "name": "Smoke Group", "member_track_ids": []any{"1007", "1010"}},
		},
	})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{
				"status":         "ok",
				"group_id":       "grp_v1",
				"applied_count":  2,
				"verified_count": 2,
				"members": []any{
					map[string]any{"track_id": "1007", "after_db": 0.0},
					map[string]any{"track_id": "1010", "after_db": 0.0},
				},
			},
			{
				"status": "ok",
				"tracks": []any{
					map[string]any{"track_id": "1007", "track_name": "Vox", "is_audio_track": true, "volume_db": 0.0},
					map[string]any{"track_id": "1010", "track_name": "Gtr", "is_audio_track": true, "volume_db": 0.0},
				},
				"track_groups": []any{
					map[string]any{"group_id": "grp_v1", "name": "Smoke Group", "member_track_ids": []any{"1007", "1010"}},
				},
			},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel

	pending, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "track.group.apply_control",
		Args: map[string]any{"group_id": "grp_v1", "control": "volume", "mode": "absolute", "db": 0.0},
	})
	if err != nil {
		t.Fatalf("pending invoke: %v", err)
	}
	if pending.Status != "needs_confirmation" || !pending.RequiresConfirmation || len(kernel.commands) != 0 {
		t.Fatalf("pending response = %+v commands=%+v", pending, kernel.commands)
	}
	if !strings.Contains(pending.Preview, "grp_v1") || !strings.Contains(pending.Preview, "0") {
		t.Fatalf("preview = %q", pending.Preview)
	}

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "track.group.apply_control",
		Args:      map[string]any{"group_id": "grp_v1", "control": "volume", "mode": "absolute", "db": 0.0},
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("confirmed invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Result["ui_action"] != "track_group_control_applied" || resp.Result["group_id"] != "grp_v1" {
		t.Fatalf("confirmed response = %+v", resp)
	}
	affected := stringSliceFromAny(resp.Result["affected_track_ids"])
	if len(affected) != 2 || affected[0] != "1007" || affected[1] != "1010" {
		t.Fatalf("affected tracks = %#v", affected)
	}
	if len(kernel.commands) != 2 || kernel.commands[0]["cmd"] != "track.group.apply_control" || kernel.commands[1]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	summary := project.Summary()
	rows := mapRowsFromAny(summary["tracks"])
	if len(rows) != 2 {
		t.Fatalf("summary tracks = %+v", summary["tracks"])
	}
	for _, row := range rows {
		db, ok := numberValueFromMap(row, "volume_db")
		if !ok || db != 0.0 {
			t.Fatalf("shadow track not refreshed to 0 dB: %+v", row)
		}
	}
}

func TestTrackGroupApplyControlAllowsCreateGroupFromExplicitTrackIDs(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1007", "track_name": "Vox", "is_audio_track": true, "volume_db": -60.0},
			map[string]any{"track_id": "1010", "track_name": "Gtr", "is_audio_track": true, "volume_db": -60.0},
		},
	})
	kernel := &fakeKernelClient{
		replies: []map[string]any{
			{
				"status":         "ok",
				"group_id":       "grp_b1_generated",
				"applied_count":  2,
				"verified_count": 2,
				"members": []any{
					map[string]any{"track_id": "1007", "after_db": 0.0, "verified": true},
					map[string]any{"track_id": "1010", "after_db": 0.0, "verified": true},
				},
			},
			{
				"status": "ok",
				"tracks": []any{
					map[string]any{"track_id": "1007", "track_name": "Vox", "is_audio_track": true, "volume_db": 0.0},
					map[string]any{"track_id": "1010", "track_name": "Gtr", "is_audio_track": true, "volume_db": 0.0},
				},
				"track_groups": []any{
					map[string]any{"group_id": "grp_b1_generated", "name": "B1 Fader Reset", "member_track_ids": []any{"1007", "1010"}},
				},
			},
		},
	}
	h := New(nil, project, nil)
	h.kernel = kernel
	args := map[string]any{
		"track_ids":               []any{"1007", "1010"},
		"create_group_if_missing": true,
		"control":                 "volume",
		"mode":                    "absolute",
		"db":                      0.0,
	}

	pending, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "track.group.apply_control",
		Args: cloneAnyMap(args),
	})
	if err != nil {
		t.Fatalf("pending invoke without group_id: %v", err)
	}
	if pending.Status != "needs_confirmation" || len(kernel.commands) != 0 {
		t.Fatalf("pending response = %+v commands=%+v", pending, kernel.commands)
	}
	if !strings.Contains(pending.Preview, "1007") || !strings.Contains(pending.Preview, "1010") {
		t.Fatalf("preview should name explicit members: %q", pending.Preview)
	}

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "track.group.apply_control",
		Args:      cloneAnyMap(args),
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("confirmed invoke without group_id: %v", err)
	}
	if resp.Status != "ok" || resp.Result["group_id"] != "grp_b1_generated" {
		t.Fatalf("confirmed response = %+v", resp)
	}
	if len(kernel.commands) != 2 || kernel.commands[0]["cmd"] != "track.group.apply_control" || kernel.commands[1]["cmd"] != "get_project_state" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	firstCmd := kernel.commands[0]
	if firstCmd["group_id"] != nil {
		t.Fatalf("expected no synthetic group_id before kernel call, got %+v", firstCmd)
	}
	if !boolValueDefault(firstCmd["create_group_if_missing"], false) {
		t.Fatalf("create_group_if_missing not preserved: %+v", firstCmd)
	}
	affected := stringSliceFromAny(resp.Result["affected_track_ids"])
	if len(affected) != 2 || affected[0] != "1007" || affected[1] != "1010" {
		t.Fatalf("affected tracks = %#v", affected)
	}
}

func TestTrackGroupApplyControlRejectsTrackIDsWithoutCreateGroup(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "track.group.apply_control",
		Args: map[string]any{
			"track_ids": []any{"1007", "1010"},
			"control":   "volume",
			"mode":      "absolute",
			"db":        0.0,
		},
	})
	if err == nil {
		t.Fatalf("expected validation error, response=%+v", resp)
	}
	if !strings.Contains(err.Error(), "create_group_if_missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveMoveClipDefaultsTracksFromSelectedClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.move",
		Args: map[string]any{"new_start_seconds": 2.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["clip_id"] != "clip_a" || cmd["source_track_id"] != "1007" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_start"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveMoveClipInfersNewStartFromUserMessage(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Command: map[string]any{
			"cmd": "move_clip",
			"args": map[string]any{
				"clip_id":         "clip_a",
				"source_track_id": "1007",
				"target_track_id": "1007",
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message": "移动选中的clip移动10s",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 10.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveMoveClipInfersPlayheadTarget(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.move",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把这个音频移动到播放头",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       4.5,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 4.5 || cmd["time_unit"] != "seconds" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveCloneClipAliasesAndTargetTrack(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{
			"clip_id":            "clip_a",
			"target_track_index": 2,
			"new_start":          8.0,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["source_clip_id"] != "clip_a" || cmd["target_track_id"] != "1010" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["time_unit"] != "seconds" {
		t.Fatalf("time_unit = %#v", cmd["time_unit"])
	}
}

func TestCloneClipPublicResultSelectsNewClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a", "target_track_id": "1010", "new_start": 8.0},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"source_clip_id": "clip_a", "target_track_id": "1010"},
		map[string]any{"status": "ok", "new_clip_id": "clip_new"},
	)
	created := result["created_clip_ids"].([]string)
	if result["ui_action"] != "select_clip" || result["clip_id"] != "clip_new" || result["track_id"] != "1010" {
		t.Fatalf("result = %+v", result)
	}
	if result["source_clip_id"] != "clip_a" || len(created) != 1 || created[0] != "clip_new" {
		t.Fatalf("clone metadata = %+v", result)
	}
}

func TestResolveMidiPatchFromSelectedClipDefaultsBeats(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.insert_notes",
		Args: map[string]any{
			"notes": []any{
				map[string]any{"pitch": 60, "start": 0.0, "length": 0.5, "velocity": 96},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "apply_midi_note_patch" || cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 {
		t.Fatalf("ops = %#v", ops)
	}
}

func TestMidiPatchRequiresOperations(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected missing operations error, cmd=%+v", cmd)
	}
}

func TestMidiPatchInfersInsertOpForBareNoteOperations(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"operations": []any{
				map[string]any{"pitch": 51, "start": 0.0, "duration": 0.95, "velocity": 84},
				map[string]any{"pitch": 58, "start": 0.0, "length": 0.95, "velocity": 84},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 2 {
		t.Fatalf("ops = %#v", ops)
	}
	for i, op := range ops {
		if op["op"] != "insert_note" {
			t.Fatalf("op %d missing insert_note: %#v", i, op)
		}
		if op["length"] != 0.95 {
			t.Fatalf("op %d length not normalized: %#v", i, op)
		}
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "<unknown>") || !strings.Contains(preview, "1. insert_note pitch=51 start=0 length=0.95 velocity=84") {
		t.Fatalf("preview =\n%s", preview)
	}
}

func TestMidiPatchNormalizesOperationAliasesAndNestedNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"operations": []any{
				map[string]any{
					"operation": "insert",
					"note": map[string]any{
						"pitch":    60,
						"start":    0.0,
						"duration": 1.0,
						"velocity": 100,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := cmd["operations"].([]map[string]any)
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	preview := PreviewCommand(spec, cmd)
	if strings.Contains(preview, "<unknown>") || !strings.Contains(preview, "1. insert_note pitch=60 start=0 length=1 velocity=100") {
		t.Fatalf("preview =\n%s", preview)
	}
}

func TestMidiPatchInfersSingleTopLevelInsertNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"pitch":    60,
			"start":    0.0,
			"length":   1.0,
			"velocity": 100,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "apply_midi_note_patch" || cmd["clip_id"] != "clip_b" || cmd["track_id"] != "1010" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "insert_note" || ops[0]["pitch"] != 60 || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	preview := PreviewCommand(spec, cmd)
	if !strings.Contains(preview, "1. insert_note pitch=60 start=0 length=1 velocity=100") {
		t.Fatalf("preview did not show inferred top-level insert:\n%s", preview)
	}
}

func TestTranslateInsertNotePatchToLegacyCommand(t *testing.T) {
	cmd := map[string]any{
		"cmd":       "apply_midi_note_patch",
		"clip_id":   "clip_b",
		"track_id":  "1010",
		"time_unit": "beats",
		"operations": []map[string]any{
			{"op": "insert_note", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			{"action": "insert_note", "pitch": 62, "start": 1.0, "duration": 0.5, "velocity": 90},
		},
	}
	if !translateInsertNotePatchToLegacyCommand(cmd) {
		t.Fatalf("translation failed: %+v", cmd)
	}
	if cmd["cmd"] != "add_midi_notes" {
		t.Fatalf("cmd = %v", cmd["cmd"])
	}
	if _, ok := cmd["operations"]; ok {
		t.Fatalf("operations should be removed: %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 2 {
		t.Fatalf("notes = %#v", notes)
	}
	if notes[0]["pitch"] != 60 || notes[0]["length"] != 1.0 || notes[1]["pitch"] != 62 || notes[1]["length"] != 0.5 {
		t.Fatalf("notes not translated: %#v", notes)
	}
}

func TestMidiReplaceRegionNormalizesTopLevelReplacementNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.replace_region",
		Args: map[string]any{
			"region_start":  0.0,
			"region_length": 1.0,
			"pitch":         65,
			"note_start":    0.0,
			"note_length":   0.5,
			"velocity":      90,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "replace_region" || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	notes := operationRowsFromAny(ops[0]["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 65 || notes[0]["start"] != 0.0 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 90 {
		t.Fatalf("notes = %#v ops=%#v", notes, ops)
	}
}

func TestMidiReplaceRegionNormalizesNestedReplacementNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.replace_region",
		Args: map[string]any{
			"region_start":  0.0,
			"region_length": 1.0,
			"replacement_note": map[string]any{
				"pitch":          65,
				"relative_start": 0.25,
				"duration":       0.5,
				"velocity":       90,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_b",
		"selected_clip_track_id": "1010",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) != 1 || ops[0]["op"] != "replace_region" || ops[0]["start"] != 0.0 || ops[0]["length"] != 1.0 {
		t.Fatalf("ops = %#v", ops)
	}
	notes := operationRowsFromAny(ops[0]["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 65 || notes[0]["start"] != 0.25 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 90 {
		t.Fatalf("notes = %#v ops=%#v", notes, ops)
	}
}

func TestMidiPatchPreviewIsReadable(t *testing.T) {
	spec := tools.CommandSpec{
		CommandName: "apply_midi_note_patch",
		Description: "Apply a beat-based MIDI note patch to a clip.",
		RiskLevel:   tools.RiskConfirm,
	}
	preview := PreviewCommand(spec, map[string]any{
		"clip_id":   "clip_a",
		"time_unit": "beats",
		"operations": []map[string]any{
			{"op": "insert_note", "pitch": 64, "start": 0.5, "length": 0.25, "velocity": 88},
			{"op": "quantize_region", "start": 0.0, "length": 4.0, "grid": "1/16"},
		},
	})
	for _, want := range []string{"apply_midi_note_patch [confirm]", "Clip clip_a", "1. insert_note pitch=64 start=0.5 length=0.25 velocity=88", "2. quantize_region start=0 length=4 grid=1/16"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q in:\n%s", want, preview)
		}
	}
}

func TestLegacyMidiBulkNormalizesDurationDefaultsBeats(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.add_notes_bulk",
		Args: map[string]any{
			"notes": []any{
				map[string]any{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "add_midi_notes_bulk" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := cmd["notes"].([]map[string]any)
	if len(notes) != 1 || notes[0]["length"] != 0.75 || notes[0]["duration"] != 0.75 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiAddNormalizesSingleTopLevelNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_add_notes",
		Args: map[string]any{
			"pitch":    67,
			"start":    1.0,
			"duration": 0.5,
			"velocity": 88,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "add_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 1 || notes[0]["pitch"] != 67 || notes[0]["start"] != 1.0 || notes[0]["length"] != 0.5 || notes[0]["velocity"] != 88 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiMutateNormalizesSingleTopLevelNote(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_mutate_notes",
		Args: map[string]any{
			"note_id":  "note_a",
			"velocity": 72,
			"duration": 0.25,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "mutate_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" || cmd["time_unit"] != "beats" {
		t.Fatalf("cmd = %+v", cmd)
	}
	notes := operationRowsFromAny(cmd["notes"])
	if len(notes) != 1 || notes[0]["id"] != "note_a" || notes[0]["length"] != 0.25 || notes[0]["velocity"] != 72 {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestLegacyMidiDeleteNormalizesSingleNoteID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_delete_notes",
		Args: map[string]any{"note_id": "note_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["cmd"] != "delete_midi_notes" || cmd["clip_id"] != "clip_a" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	ids := stringSliceFromAny(cmd["note_ids"])
	if len(ids) != 1 || ids[0] != "note_a" {
		t.Fatalf("note_ids = %#v cmd=%+v", ids, cmd)
	}
}

func TestLegacyMidiPublicResultSyncsMidiUI(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.legacy_add_notes",
		Args: map[string]any{"clip_id": "clip_a", "notes": []any{map[string]any{"pitch": 67, "start": 1.0, "length": 0.5}}},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{
			"status":         "ok",
			"clip_id":        "clip_a",
			"track_id":       "1007",
			"inserted_count": 1,
			"notes": []any{
				map[string]any{"id": "note_legacy", "pitch": 67, "start": 1.0, "length": 0.5, "velocity": 88},
			},
		},
	)
	if result["ui_action"] != "midi_note_patch" || result["clip_id"] != "clip_a" || result["track_id"] != "1007" || result["inserted_count"] != 1 {
		t.Fatalf("result = %+v", result)
	}
	notes := operationRowsFromAny(result["notes"])
	if len(notes) != 1 || notes[0]["id"] != "note_legacy" {
		t.Fatalf("notes = %#v result=%+v", notes, result)
	}
}

func TestLegacyMidiBulkRejectsUnknownExplicitClipID(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.add_notes_bulk",
		Args: map[string]any{
			"clip_id": "1010",
			"notes": []any{
				map[string]any{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected unknown clip_id error, cmd=%+v", cmd)
	}
	if !strings.Contains(err.Error(), `clip_id "1010" is not present`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLegacyMidiPreviewIsReadable(t *testing.T) {
	spec := tools.CommandSpec{
		CommandName: "add_midi_notes_bulk",
		Description: "Add many MIDI notes to a clip.",
		RiskLevel:   tools.RiskConfirm,
	}
	preview := PreviewCommand(spec, map[string]any{
		"clip_id":   "clip_a",
		"time_unit": "beats",
		"notes": []map[string]any{
			{"pitch": 55, "start": 0.0, "duration": 0.75, "velocity": 100},
		},
	})
	for _, want := range []string{"add_midi_notes_bulk [confirm]", "Clip clip_a", "1. note pitch=55 start=0 velocity=100 duration=0.75"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q in:\n%s", want, preview)
		}
	}
	if strings.Contains(preview, `"notes"`) {
		t.Fatalf("preview should be readable, got raw JSON:\n%s", preview)
	}
}

func TestMidiPatchPublicResultIncludesUIAction(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{"clip_id": "clip_a", "operations": []any{map[string]any{"op": "quantize_region", "start": 0.0, "length": 1.0, "grid": "1/16"}}},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{"status": "ok", "clip_id": "clip_a", "quantized_count": 3},
	)
	if result["ui_action"] != "midi_note_patch" || result["clip_id"] != "clip_a" || result["track_id"] != "1007" || result["quantized_count"] != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestMidiPatchPublicResultPreservesInsertedNotes(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"clip_id": "clip_a",
			"operations": []any{
				map[string]any{"op": "insert_note", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"clip_id": "clip_a", "track_id": "1007"},
		map[string]any{
			"status":            "ok",
			"clip_id":           "clip_a",
			"track_id":          "1007",
			"inserted_count":    1,
			"inserted_note_ids": []any{"note_a"},
			"notes": []any{
				map[string]any{"id": "note_a", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
			},
		},
	)
	if result["ui_action"] != "midi_note_patch" || result["inserted_count"] != 1 {
		t.Fatalf("result = %+v", result)
	}
	ids := stringSliceFromAny(result["inserted_note_ids"])
	notes := operationRowsFromAny(result["notes"])
	if len(ids) != 1 || ids[0] != "note_a" {
		t.Fatalf("inserted ids = %#v result=%+v", ids, result)
	}
	if len(notes) != 1 || notes[0]["id"] != "note_a" || notes[0]["pitch"] != 60 {
		t.Fatalf("notes = %#v result=%+v", notes, result)
	}
}

func TestMidiClipCreatePublicResultSelectsNewClip(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	_, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "midi.create_clip",
		Args: map[string]any{"track_id": "1007"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	result := h.publicResult(spec,
		map[string]any{"track_id": "1007"},
		map[string]any{"status": "ok", "clip_id": "clip_new"},
	)
	created := result["created_clip_ids"].([]string)
	if result["ui_action"] != "select_clip" || result["clip_id"] != "clip_new" || result["new_clip_id"] != "clip_new" {
		t.Fatalf("result = %+v", result)
	}
	if result["track_id"] != "1007" || result["target_track_id"] != "1007" || len(created) != 1 || created[0] != "clip_new" {
		t.Fatalf("create metadata = %+v", result)
	}
}

func TestResolveCloneClipInfersPlayheadTarget(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "把这个音频复制到播放头",
		"selected_clip_track_id": "1007",
		"playhead_seconds":       6.25,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["new_start"] != 6.25 || cmd["time_unit"] != "seconds" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveCloneClipInfersStartAfterSource(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "clip_a"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"user_message":           "复制选中的clip到后面",
		"selected_clip_id":       "clip_a",
		"selected_clip_track_id": "1007",
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["source_clip_id"] != "clip_a" || cmd["target_track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["new_start"] != 2.0 || cmd["time_unit"] != "seconds" {
		t.Fatalf("time args = %+v", cmd)
	}
}

func TestResolveCloneClipRequiresStartWhenUnknownClipBounds(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.clone",
		Args: map[string]any{"clip_id": "external_clip"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	err = h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_clip_track_id": "1007",
	})
	if err == nil {
		t.Fatalf("expected missing new_start error, cmd=%+v", cmd)
	}
}

func TestResolveImportMediaFromSelectedLibraryAndTrack(t *testing.T) {
	audioPath := writeTempAudioFile(t, "Loop A.wav")
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_media_to_track",
		Args: map[string]any{},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id":          "1007",
		"selected_library_file_path": audioPath,
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1007" || cmd["file_path"] != audioPath {
		t.Fatalf("cmd = %+v", cmd)
	}
	if cmd["media_type"] != "audio" || cmd["mode"] != "non_destructive" || cmd["start_time"] != 0.0 {
		t.Fatalf("defaults = %+v", cmd)
	}
}

func TestResolveImportAudioPathAliases(t *testing.T) {
	audioPath := writeTempAudioFile(t, "Kick.wav")
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_audio",
		Args: map[string]any{
			"path":            audioPath,
			"target_track_id": "1010",
			"start_time":      3.5,
		},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, nil); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["track_id"] != "1010" || cmd["file_path"] != audioPath || cmd["offset_time"] != 3.5 {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestResolveImportMediaSearchesLibraryPlaces(t *testing.T) {
	root := t.TempDir()
	audioPath := filepath.Join(root, "Deep Kick Loop.wav")
	if err := os.WriteFile(audioPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}
	h := New(nil, shadowProjectWithClips(), nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "clip.import_media_to_track",
		Args: map[string]any{"asset_query": "kick loop"},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if err := h.resolveImplicitTargets(context.Background(), spec, cmd, map[string]any{
		"selected_track_id": "1007",
		"library_places":    []any{root},
	}); err != nil {
		t.Fatalf("resolveImplicitTargets: %v", err)
	}
	if cmd["file_path"] != audioPath || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v", cmd)
	}
}

func TestPublicResultSanitizesProjectState(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true},
		},
	})
	h := New(nil, project, nil)

	result := h.publicResult(tools.CommandSpec{CommandName: "get_project_state"}, nil, map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"track_id": "1002"},
			map[string]any{"track_id": "1007"},
		},
	})

	if result["status"] != "ok" || result["track_count"] != 1 {
		t.Fatalf("result counts = %+v", result)
	}
	if _, ok := result["engine_track_count"]; ok {
		t.Fatalf("public result leaked engine_track_count: %+v", result)
	}
	if _, ok := result["internal_track_count"]; ok {
		t.Fatalf("public result leaked internal_track_count: %+v", result)
	}
	tracks := result["tracks"].([]map[string]any)
	if len(tracks) != 1 || tracks[0]["track_id"] != "1007" {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func shadowProjectWithClips() *shadow.Project {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_a", "name": "Loop A", "start_seconds": 0.0, "length_seconds": 2.0},
				},
			},
			map[string]any{
				"track_id":       "1010",
				"track_name":     "Bass",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{"id": "clip_b", "name": "Loop B", "start_seconds": 4.0, "length_seconds": 2.0},
				},
			},
		},
	})
	return project
}

func TestInvokeClipStripSilenceSuggestUsesAnalyzeAndBuildsPendingApply(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":              "ok",
			"analysis_id":         "analysis_low",
			"clip_id":             "clip_a",
			"track_id":            "1007",
			"clip_length_seconds": 2.0,
			"analysis_ranges": []any{map[string]any{
				"range_id":      "range_1",
				"start_seconds": 0.25,
				"end_seconds":   1.75,
			}},
			"keep_segment_count": 1,
			"strip_region_count": 0,
			"keep_segments": []any{map[string]any{
				"start_seconds": 0.25,
				"end_seconds":   1.75,
			}},
			"strip_regions": []any{},
		},
		{
			"status":              "ok",
			"analysis_id":         "analysis_rec",
			"clip_id":             "clip_a",
			"track_id":            "1007",
			"clip_length_seconds": 2.0,
			"analysis_ranges": []any{map[string]any{
				"range_id":      "range_1",
				"start_seconds": 0.25,
				"end_seconds":   1.75,
			}},
			"keep_segment_count": 1,
			"strip_region_count": 1,
			"keep_segments": []any{map[string]any{
				"start_seconds": 0.55,
				"end_seconds":   1.45,
			}},
			"strip_regions": []any{map[string]any{
				"range_id":      "range_1",
				"clip_id":       "clip_a",
				"track_id":      "1007",
				"start_seconds": 0.25,
				"end_seconds":   0.55,
			}},
		},
	}}
	h := NewWithSender(kernel, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.strip_silence.suggest",
		Args: map[string]any{
			"candidate_thresholds_dbfs": []any{-60.0, -48.0},
			"min_silence_ms":            120.0,
		},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":       "clip_a",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{map[string]any{
					"range_id":         "range_1",
					"clip_id":          "clip_a",
					"track_id":         "1007",
					"start_seconds":    0.25,
					"end_seconds":      1.75,
					"duration_seconds": 1.5,
				}},
			},
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 2 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	for _, cmd := range kernel.commands {
		if got := fmt.Sprint(cmd["cmd"]); got != "clip.strip_silence.analyze" {
			t.Fatalf("suggest should only call analyze, got %+v", kernel.commands)
		}
		if firstString(cmd, "clip_id") != "clip_a" || firstString(cmd, "track_id") != "1007" {
			t.Fatalf("analyze target = %+v", cmd)
		}
		if ranges := mapRowsFromAny(cmd["ranges"]); len(ranges) != 1 || firstString(ranges[0], "range_id") != "range_1" {
			t.Fatalf("selected ranges not forwarded: %+v", cmd)
		}
	}
	params := testMap(t, resp.Result["recommended_params"])
	if params["threshold_dbfs"] != -48.0 {
		t.Fatalf("recommended params = %+v", params)
	}
	action := testMap(t, resp.Result["pending_action"])
	if firstString(action, "tool_name") != "clip.strip_silence.apply" {
		t.Fatalf("pending_action = %+v", action)
	}
	args := testMap(t, action["args"])
	if firstString(args, "analysis_id") != "analysis_rec" {
		t.Fatalf("pending args did not use recommended analysis: %+v", args)
	}
	regions := mapRowsFromAny(args["strip_regions"])
	if len(regions) != 1 || firstString(regions[0], "range_id") != "range_1" {
		t.Fatalf("pending strip_regions = %+v", regions)
	}
	if resp.RequiresConfirmation {
		t.Fatalf("suggest itself must not require confirmation: %+v", resp)
	}
}

func TestInvokeClipStripSilenceSuggestExplicitClipScopeIgnoresContextRanges(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":              "ok",
			"analysis_id":         "analysis_clip",
			"clip_id":             "clip_a",
			"track_id":            "1007",
			"clip_length_seconds": 2.0,
			"keep_segment_count":  1,
			"strip_region_count":  1,
			"keep_segments": []any{map[string]any{
				"start_seconds": 0.35,
				"end_seconds":   0.70,
			}},
			"strip_regions": []any{map[string]any{
				"clip_id":       "clip_a",
				"track_id":      "1007",
				"start_seconds": 0.0,
				"end_seconds":   0.35,
			}},
		},
	}}
	h := NewWithSender(kernel, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.strip_silence.suggest",
		Args: map[string]any{
			"scope":                     "selected_clip",
			"clip_id":                   "clip_a",
			"track_id":                  "1007",
			"candidate_thresholds_dbfs": []any{-48.0},
		},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":       "clip_a",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{map[string]any{
					"range_id":         "stale_range",
					"clip_id":          "old_clip",
					"track_id":         "old_track",
					"start_seconds":    10.0,
					"end_seconds":      11.0,
					"duration_seconds": 1.0,
				}},
			},
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	cmd := kernel.commands[0]
	if got := fmt.Sprint(cmd["cmd"]); got != "clip.strip_silence.analyze" {
		t.Fatalf("suggest should only call analyze, got %+v", kernel.commands)
	}
	if firstString(cmd, "clip_id") != "clip_a" || firstString(cmd, "track_id") != "1007" {
		t.Fatalf("analyze target = %+v", cmd)
	}
	if ranges := mapRowsFromAny(cmd["ranges"]); len(ranges) != 0 {
		t.Fatalf("explicit selected_clip scope should not forward stale ranges: %+v", cmd)
	}
	if scope := firstString(cmd, "scope"); scope != "selected_clip" {
		t.Fatalf("analyze scope = %q, cmd=%+v", scope, cmd)
	}
}

func TestInvokeClipStripSilenceSuggestSelectedTrackUsesSelectedTrackClips(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":              "ok",
			"analysis_id":         "analysis_track",
			"clip_id":             "clip_b",
			"track_id":            "1010",
			"clip_length_seconds": 2.0,
			"keep_segment_count":  1,
			"strip_region_count":  1,
			"keep_segments": []any{map[string]any{
				"start_seconds": 4.35,
				"end_seconds":   5.70,
			}},
			"strip_regions": []any{map[string]any{
				"clip_id":       "clip_b",
				"track_id":      "1010",
				"start_seconds": 4.0,
				"end_seconds":   4.35,
			}},
		},
	}}
	h := NewWithSender(kernel, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "clip.strip_silence.suggest",
		Args: map[string]any{
			"scope":                     "selected_track",
			"candidate_thresholds_dbfs": []any{-48.0},
		},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_track_id":      "1010",
				"selected_clip_id":       "clip_a",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{map[string]any{
					"range_id":      "stale_range",
					"clip_id":       "clip_a",
					"track_id":      "1007",
					"start_seconds": 0.0,
					"end_seconds":   1.0,
				}},
			},
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	cmd := kernel.commands[0]
	if firstString(cmd, "scope") != "selected_track" {
		t.Fatalf("analyze scope = %+v", cmd)
	}
	if firstString(cmd, "clip_id") != "clip_b" || firstString(cmd, "track_id") != "1010" {
		t.Fatalf("selected track analyze target = %+v", cmd)
	}
	if ranges := mapRowsFromAny(cmd["ranges"]); len(ranges) != 0 {
		t.Fatalf("selected_track should not forward stale clip ranges: %+v", cmd)
	}
	if firstString(resp.Result, "scope") != "selected_track" || int(numberFromAny(resp.Result["target_count"])) != 1 {
		t.Fatalf("suggest result = %+v", resp.Result)
	}
}

func TestInvokeClipStripSilenceApplyBatchRunsKernelAppliesAndReturnsCompactSummary(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":               "ok",
			"clip_id":              "clip_a",
			"track_id":             "1007",
			"applied_region_count": 1,
			"strip_regions": []any{map[string]any{
				"clip_id":       "clip_a",
				"track_id":      "1007",
				"start_seconds": 0.0,
				"end_seconds":   0.25,
			}},
			"created_clip_ids":  []any{"clip_a_1", "clip_a_2"},
			"affected_clip_ids": []any{"clip_a", "clip_a_1", "clip_a_2"},
		},
		{
			"status":               "ok",
			"clip_id":              "clip_b",
			"track_id":             "1010",
			"applied_region_count": 2,
			"strip_regions": []any{
				map[string]any{"clip_id": "clip_b", "track_id": "1010", "start_seconds": 1.0, "end_seconds": 1.25},
				map[string]any{"clip_id": "clip_b", "track_id": "1010", "start_seconds": 2.0, "end_seconds": 2.25},
			},
			"removed_clip_ids":  []any{"clip_b"},
			"affected_clip_ids": []any{"clip_b"},
		},
	}}
	h := NewWithSender(kernel, nil, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "clip.strip_silence.apply_batch",
		Confirmed: true,
		Args: map[string]any{
			"pending_actions": []any{
				map[string]any{
					"tool_name": "clip.strip_silence.apply",
					"args": map[string]any{
						"clip_id":     "clip_a",
						"track_id":    "1007",
						"analysis_id": "analysis_a",
						"strip_regions": []any{map[string]any{
							"clip_id":       "clip_a",
							"track_id":      "1007",
							"start_seconds": 0.0,
							"end_seconds":   0.25,
						}},
					},
				},
				map[string]any{
					"tool_name": "clip.strip_silence.apply",
					"args": map[string]any{
						"clip_id":     "clip_b",
						"track_id":    "1010",
						"analysis_id": "analysis_b",
						"strip_regions": []any{
							map[string]any{"clip_id": "clip_b", "track_id": "1010", "start_seconds": 1.0, "end_seconds": 1.25},
							map[string]any{"clip_id": "clip_b", "track_id": "1010", "start_seconds": 2.0, "end_seconds": 2.25},
						},
					},
				},
			},
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 2 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	for _, cmd := range kernel.commands {
		if firstString(cmd, "cmd") != "clip.strip_silence.apply" {
			t.Fatalf("batch should only issue apply commands internally: %+v", kernel.commands)
		}
	}
	if firstString(resp.Result, "schema_version") != "clip.strip_silence.apply_batch.v0" {
		t.Fatalf("batch result = %+v", resp.Result)
	}
	if int(numberFromAny(resp.Result["applied_clip_count"])) != 2 || int(numberFromAny(resp.Result["applied_region_count"])) != 3 {
		t.Fatalf("batch counts = %+v", resp.Result)
	}
	if _, ok := resp.Result["strip_regions"]; ok {
		t.Fatalf("batch result should not expose raw strip_regions: %+v", resp.Result)
	}
}

func TestInvokeClipStripSilenceApplyBatchClassifiesNoCleanupSkipAndRealFailure(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":               "ok",
			"clip_id":              "clip_apply",
			"track_id":             "track_apply",
			"applied_region_count": 1,
		},
		{
			"status":   "error",
			"clip_id":  "clip_protected",
			"track_id": "track_protected",
			"message":  "clip.strip_silence.apply would remove the entire clip; set allow_remove_entire_clip=true to confirm",
		},
	}}
	h := NewWithSender(kernel, nil, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "clip.strip_silence.apply_batch",
		Confirmed: true,
		Args: map[string]any{
			"target_count":                 3,
			"analyzed_clip_count":          3,
			"no_cleanup_needed_clip_count": 1,
			"analysis_failed_clip_count":   0,
			"pending_actions": []any{
				map[string]any{
					"tool_name": "clip.strip_silence.apply",
					"args": map[string]any{
						"clip_id":     "clip_apply",
						"track_id":    "track_apply",
						"analysis_id": "analysis_apply",
						"strip_regions": []any{map[string]any{
							"clip_id":       "clip_apply",
							"track_id":      "track_apply",
							"start_seconds": 0.0,
							"end_seconds":   0.25,
						}},
					},
				},
				map[string]any{
					"tool_name": "clip.strip_silence.apply",
					"args": map[string]any{
						"clip_id":     "clip_protected",
						"track_id":    "track_protected",
						"analysis_id": "analysis_protected",
						"strip_regions": []any{map[string]any{
							"clip_id":       "clip_protected",
							"track_id":      "track_protected",
							"start_seconds": 0.0,
							"end_seconds":   2.0,
						}},
					},
				},
			},
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(resp.Result, "status") != "partial" {
		t.Fatalf("result status = %q, result = %+v", firstString(resp.Result, "status"), resp.Result)
	}
	if got := int(numberFromAny(resp.Result["applied_clip_count"])); got != 1 {
		t.Fatalf("applied_clip_count = %d, result = %+v", got, resp.Result)
	}
	if got := int(numberFromAny(resp.Result["no_cleanup_needed_clip_count"])); got != 1 {
		t.Fatalf("no_cleanup_needed_clip_count = %d, result = %+v", got, resp.Result)
	}
	if got := int(numberFromAny(resp.Result["protected_skip_clip_count"])); got != 1 {
		t.Fatalf("protected_skip_clip_count = %d, result = %+v", got, resp.Result)
	}
	if got := int(numberFromAny(resp.Result["failed_clip_count"])); got != 0 {
		t.Fatalf("failed_clip_count should mean real failure only, got %d: %+v", got, resp.Result)
	}
	errors := mapRowsFromAny(resp.Result["errors"])
	if len(errors) != 1 || firstString(errors[0], "disposition") != "protected_skip" {
		t.Fatalf("protected skip error row missing classification: %+v", resp.Result)
	}
}

func writeTempAudioFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}
	return path
}

func testAcousticSourceRevision(sessionID, trackID, clipID string, duration float64) string {
	return acousticpackage.ComputeSourceRevision(acousticpackage.Identity{
		ProjectID:   "current",
		SessionID:   sessionID,
		TrackID:     trackID,
		ClipID:      clipID,
		DurationSec: duration,
	})
}

func TestInvokeMixRequestObservationWritesMixBoardWithoutKernel(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_test",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" || resp.CommandName != "mix_request_observation" {
		t.Fatalf("resp = %+v", resp)
	}
	if firstString(resp.Result, "status") != "partial" {
		t.Fatalf("result status = %+v", resp.Result)
	}
	for _, key := range []string{"board_path", "observation_path", "context_pack_path"} {
		path := firstString(resp.Result, key)
		if path == "" {
			t.Fatalf("%s missing from result: %+v", key, resp.Result)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s does not exist: %v", key, err)
		}
	}
}

func TestInvokeMixObserveAliasReturnsDigestAndCatalog(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_observe",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CommandName != "mix_observe" {
		t.Fatalf("command = %q", resp.CommandName)
	}
	if resp.Result["digest"] == nil || resp.Result["catalog"] == nil {
		t.Fatalf("missing digest/catalog: %+v", resp.Result)
	}
}

func TestInvokeMixObserveWithPreviousObservationRequestsL2RenderProbe(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_L2_RENDER_PROBE_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := NewWithSender(kernel, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id":       "mix_ab_reobserve",
			"previous_observation": "obs_before",
			"observation_only":     true,
			"scope":                "selected_track",
			"track_id":             "1007",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	commands := testCommandsByName(kernel.commands, "l2_render_probe")
	if len(commands) != 1 {
		t.Fatalf("l2_render_probe commands = %+v all=%+v", commands, kernel.commands)
	}
	if commands[0]["track_id"] != "1007" || commands[0]["tap_point"] != "track_post_fader" || commands[0]["render_mode"] != "offline_probe" {
		t.Fatalf("l2_render_probe command = %+v", commands[0])
	}
	request := testMap(t, resp.Result["l2_render_probe_request"])
	if request["status"] == "" {
		t.Fatalf("missing l2 request summary: %+v", resp.Result)
	}
}

func TestInvokeMixObserveFullProjectScopeKeepsProjectTarget(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project",
			"scope":          "full_project",
			"goal_text":      "帮我看一下整体混音",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "project" || target["id"] != "current" {
		t.Fatalf("target = %+v", target)
	}
	digest := testMap(t, resp.Result["digest"])
	if digest["scope"] != "full_project" {
		t.Fatalf("digest = %+v", digest)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	if source["mode"] != "full_project" {
		t.Fatalf("listen scope = %+v", listen)
	}
	if _, ok := source["focus_ids"]; ok {
		t.Fatalf("full project scope should not carry focus ids: %+v", listen)
	}
}

func TestVisibleTrackRowsSkipsEmptyTopLevelRows(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{"clips": []any{map[string]any{"clip_id": "ghost_clip"}}},
		},
		"shadow": map[string]any{
			"tracks": []any{
				map[string]any{"track_id": "1007", "track_name": "Track 1"},
				map[string]any{"track_id": "1010", "track_name": "Track 2"},
			},
		},
	}
	rows := visibleTrackRows(state)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if visibleTrackID(rows[0]) != "1007" || visibleTrackID(rows[1]) != "1010" {
		t.Fatalf("unexpected rows = %+v", rows)
	}
}

func TestInvokeMixObserveFullProjectRefreshesProjectState(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	stale := shadow.New(nil)
	stale.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{"clips": []any{map[string]any{"clip_id": "ghost_clip"}}},
		},
	})
	kernel := &fakeKernelClient{replies: []map[string]any{{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1007",
				"track_name":       "Track 1",
				"user_track_index": 1,
				"is_audio_track":   true,
				"clips":            []any{map[string]any{"clip_id": "1014", "length_seconds": 10.0, "file_path": "D:\\Vit_DAW\\test_100hz_10s.wav"}},
			},
			map[string]any{
				"track_id":         "1010",
				"track_name":       "Track 2",
				"user_track_index": 2,
				"is_audio_track":   true,
				"clips":            []any{map[string]any{"clip_id": "1016", "length_seconds": 3.0, "file_path": "D:\\Vit_DAW\\test_target_3s.wav"}},
			},
		},
	}}}
	h := NewWithSender(kernel, stale, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project_refresh",
			"scope":          "full_project",
			"goal_text":      "比较一下各轨频段占用和声像关系，不要修改。",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kernel.commands) == 0 || firstString(kernel.commands[0], "cmd") != "get_project_state" {
		t.Fatalf("expected project-state refresh, commands=%+v", kernel.commands)
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	tracks := mapRowsFromAny(obs.ProjectPackage["tracks"])
	if len(tracks) != 2 {
		t.Fatalf("project tracks = %+v", tracks)
	}
	if obs.MOMProjection == nil {
		t.Fatalf("missing MOM projection")
	}
	if obs.MOMProjection.IntentPolicy.Name != "project_multitrack_relation_observation" {
		t.Fatalf("MOM intent = %q", obs.MOMProjection.IntentPolicy.Name)
	}
	if obs.MOMProjection.MultitrackRelation.Status == "not_applicable_single_track" {
		t.Fatalf("MOM relation downgraded to single track: %+v", obs.MOMProjection.MultitrackRelation)
	}
	if obs.MOMProjection.MultitrackRelation.TrackCount < 2 {
		t.Fatalf("MOM relation track count = %+v", obs.MOMProjection.MultitrackRelation)
	}
}

func TestInvokeMixObserveFullProjectWritesBlockedPerTrackAcousticsWithoutKernel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project_blocked_acoustic",
			"scope":          "full_project",
			"goal_text":      "whole mix",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.Result["feature_request"]; ok {
		t.Fatalf("feature_request should be absent for read-only observation: %+v", resp.Result["feature_request"])
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	if got := obs.SourceCapabilities["track_waveform_envelopes"]; got != "missing" {
		t.Fatalf("track waveform capability = %q observation=%+v", got, obs)
	}
	project := obs.ProjectPackage
	if project["acoustic_track_count"] != 2 || project["active_acoustic_track_count"] != 0 {
		t.Fatalf("project package counts = %+v", project)
	}
	tracks := mapRowsFromAny(project["tracks"])
	if len(tracks) != 2 {
		t.Fatalf("tracks = %+v", tracks)
	}
	for _, track := range tracks {
		acoustic := testMap(t, track["acoustic"])
		if firstString(acoustic, "status") != "missing" {
			t.Fatalf("read-only observation should report missing acoustic rows: acoustic=%+v track=%+v", acoustic, track)
		}
	}
	digest := testMap(t, resp.Result["digest"])
	available := testMap(t, digest["available_detail"])
	if !statusIn(available["project_track_waveforms"], "missing") || !statusIn(available["full_project_acoustic_render"], "missing") {
		t.Fatalf("available detail = %+v", available)
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		data, _ := os.ReadFile(snapshotPath)
		t.Fatalf("read-only observation wrote feature snapshot: %s", string(data))
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat snapshot: %v", err)
	}
}

func TestInvokeMixObserveFocusHintResolvesVocalTrack(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "vocal_1",
				"track_name":     "Lead Vocal",
				"is_audio_track": true,
				"level_db":       -12.0,
				"peak_dbfs":      -3.0,
				"clips":          []any{map[string]any{"clip_id": "clip_v", "length_seconds": 8.0}},
			},
			map[string]any{
				"track_id":       "bass_1",
				"track_name":     "Bass",
				"is_audio_track": true,
				"level_db":       -8.0,
				"peak_dbfs":      -2.0,
				"clips":          []any{map[string]any{"clip_id": "clip_b", "length_seconds": 8.0}},
			},
		},
	})
	h := New(nil, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_focus_vocal",
			"scope":          "full_project_with_focus_track",
			"focus_hint":     map[string]any{"role": "vocal"},
			"goal_text":      "bring lead vocal forward",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "track" || target["id"] != "vocal_1" {
		t.Fatalf("target = %+v", target)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	if source["mode"] != "full_project_with_focus_track" {
		t.Fatalf("listen scope = %+v", listen)
	}
	focusIDs := stringSliceFromAny(source["focus_ids"])
	if len(focusIDs) != 1 || focusIDs[0] != "vocal_1" {
		t.Fatalf("focus ids = %+v", source["focus_ids"])
	}
	projectPackage := testMap(t, obs["project_package"])
	tracks := mapRowsFromAny(projectPackage["tracks"])
	if len(tracks) != 2 || tracks[0]["focused"] != true {
		t.Fatalf("project tracks = %+v", tracks)
	}
}

func TestInvokeMixObserveFocusHintResolvesUserTrackIndex(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1007",
				"track_name":       "Track 1",
				"user_track_index": 1,
				"is_audio_track":   true,
				"level_db":         -12.0,
				"peak_dbfs":        -3.0,
				"clips":            []any{map[string]any{"clip_id": "clip_1", "length_seconds": 8.0}},
			},
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"user_track_index": 2,
				"is_audio_track":   true,
				"level_db":         -8.0,
				"peak_dbfs":        -2.0,
				"clips":            []any{map[string]any{"clip_id": "clip_2", "length_seconds": 8.0}},
			},
		},
	})
	h := New(nil, project, nil)

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_focus_index",
			"scope":          "full_project_with_focus_track",
			"focus_hint":     map[string]any{"role": "vocal", "user_track_index": 1},
			"goal_text":      "让主唱更靠前",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := testMap(t, resp.Result["observation"])
	target := testMap(t, obs["target_ref"])
	if target["kind"] != "track" || target["id"] != "1007" {
		t.Fatalf("target = %+v", target)
	}
	listen := testMap(t, obs["listen_scope"])
	source := testMap(t, listen["source"])
	focusIDs := stringSliceFromAny(source["focus_ids"])
	if len(focusIDs) != 1 || focusIDs[0] != "1007" {
		t.Fatalf("focus ids = %+v", source["focus_ids"])
	}
}

func TestInvokeMixReadAndDeriveUseStoredObservation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	observation, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_read_derive",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	obsID := firstString(observation.Result, "observation_id")
	if obsID == "" {
		t.Fatalf("observation id missing: %+v", observation.Result)
	}
	readResp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.read",
		Args: map[string]any{
			"observation_id": obsID,
			"keys":           []any{"observation.digest"},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if readResp.CommandName != "mix_read" {
		t.Fatalf("command = %q", readResp.CommandName)
	}
	if readResp.Result["items"] == nil {
		t.Fatalf("read result missing items: %+v", readResp.Result)
	}
	deriveResp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.derive",
		Args: map[string]any{
			"observation_id": obsID,
			"type":           "before_after",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deriveResp.CommandName != "mix_derive" {
		t.Fatalf("command = %q", deriveResp.CommandName)
	}
	if firstString(deriveResp.Result, "status") == "" {
		t.Fatalf("derive result missing status: %+v", deriveResp.Result)
	}
}

func TestProjectSnapshotExportFallsBackWhenKernelCommandMissing(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "error", "message": "Unknown command: project.snapshot_export"},
		{"status": "error", "message": "Unknown command: project_snapshot_export"},
	}}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.snapshot_export",
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if firstString(resp.Result, "source") != "agent_shadow_snapshot_compat" {
		t.Fatalf("expected compat snapshot, got %+v", resp.Result)
	}
	if resp.Result["project_state"] == nil {
		t.Fatalf("project_state missing: %+v", resp.Result)
	}
}

func TestInvokeMixRequestObservationDoesNotRequestAudioFeatures(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_feature_request",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "1007",
				"label": "Drums",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	if _, ok := resp.Result["feature_request"]; ok {
		t.Fatalf("feature_request should be absent for read-only observation: %+v", resp.Result["feature_request"])
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		data, _ := os.ReadFile(snapshotPath)
		if strings.Contains(string(data), `"requested"`) || strings.Contains(string(data), `"latest_request"`) {
			t.Fatalf("observation wrote request-style snapshot: %s", string(data))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat snapshot: %v", err)
	}
}

func TestInvokeMixRequestObservationDoesNotWriteRequestedSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	_, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_feature_order",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if featureCommands := testCommandsByName(kernel.commands, "request_audio_feature"); len(featureCommands) != 0 {
		t.Fatalf("feature kernel commands = %+v all=%+v", featureCommands, kernel.commands)
	}
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		data, _ := os.ReadFile(snapshotPath)
		text := string(data)
		if strings.Contains(text, `"requested"`) || strings.Contains(text, `"mixboard_feature_request.v1"`) {
			t.Fatalf("snapshot should not contain requested rows: %s", text)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat snapshot: %v", err)
	}
}

func TestInvokeMixRequestObservationReportsMissingWithoutBackgroundFill(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_feature_missing_reasons",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if featureCommands := testCommandsByName(kernel.commands, "request_audio_feature"); len(featureCommands) != 0 {
		t.Fatalf("feature kernel commands = %+v all=%+v", featureCommands, kernel.commands)
	}
	status := testMap(t, resp.Result["acoustic_package_status"])
	if status["schema_version"] != acousticpackage.SchemaVersion {
		t.Fatalf("acoustic package status missing: %+v", status)
	}
	layers := testMap(t, status["package_layers"])
	l3 := testMap(t, layers["l3_deep"])
	features := testMap(t, l3["features"])
	for _, featureName := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"} {
		feature := testMap(t, features[featureName])
		if firstString(feature, "status") == acousticpackage.StatusBuilding {
			t.Fatalf("%s should not be marked building during read-only observation: %+v package=%+v", featureName, feature, status)
		}
	}
	obs := resp.Result["observation"].(mixboard.ObservationPacket)
	metrics := obs.MixPackage["current_metrics"].(map[string]any)
	bandMetrics := metrics["band_energy"].(map[string]any)
	if bandMetrics["status"] == acousticpackage.StatusBuilding {
		t.Fatalf("observation exposed building band readiness: %+v", bandMetrics)
	}
}

func TestInvokeMixObserveReadFirstUsesStoredBuildingPackageWithoutKernelRequest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	statusPath := filepath.Join(root, "acoustic_package_status.json")
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", statusPath)
	project := shadowProjectWithClips()
	h := New(nil, project, nil)
	state := h.UserStateSummary(context.Background())
	if targets := visibleAudioTrackFeatureTargets(state); len(targets) == 0 {
		t.Fatalf("test fixture has no visible audio targets: state=%+v", state)
	}
	cmd := map[string]any{
		"mix_session_id": "mix_read_first",
		"target_ref": map[string]any{
			"kind": "track",
			"id":   "1007",
		},
	}
	target := mixTargetFromCommand(cmd)
	resolved := resolveMixObservationTargetContext(cmd, state, target)
	cmd = canonicalizeMixObservationCommand(cmd, target, resolved)
	identity := acousticpackage.IdentityFromMaps(state, resolved, cmd)
	status := acousticpackage.BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "1007", "clip_id": "clip_a", "rms": 0.2, "peak_abs": 0.8, "time_segments": []any{map[string]any{"start_seconds": 0, "end_seconds": 2}}},
	}, "2026-06-22T00:00:00Z", "test")
	status = acousticpackage.MarkBackgroundRequested(status, "test", "already_building", "2026-06-22T00:00:01Z")
	if _, err := acousticpackage.NewStore(statusPath).Upsert(status); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{replies: []map[string]any{{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Drums",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips":          []any{map[string]any{"id": "clip_a", "name": "Loop A", "start_seconds": 0.0, "length_seconds": 2.0}},
			},
			map[string]any{
				"track_id":       "1010",
				"track_name":     "Bass",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips":          []any{map[string]any{"id": "clip_b", "name": "Loop B", "start_seconds": 4.0, "length_seconds": 2.0}},
			},
		},
	}}}
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "mix.observe",
		Args:      cmd,
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("read-first observe should not request more features while package is building: %+v", kernel.commands)
	}
	if _, ok := resp.Result["feature_request"]; ok {
		t.Fatalf("feature_request should be absent when store already reports building: %+v", resp.Result["feature_request"])
	}
	acousticStatus := testMap(t, resp.Result["acoustic_package_status"])
	l3 := testMap(t, testMap(t, acousticStatus["package_layers"])["l3_deep"])
	if firstString(l3, "status") != acousticpackage.StatusBuilding {
		t.Fatalf("l3 status = %+v", l3)
	}
	if events := mapRowsFromAny(resp.Result["typed_events"]); len(events) != 1 || firstString(events[0], "event_type") != agentprotocol.KindAcousticPackageStatus {
		t.Fatalf("typed_events = %+v", resp.Result["typed_events"])
	}
}

func TestInvokeMixObserveFullProjectDoesNotRefreshBackgroundWhenStoredPackageStillBuilding(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	statusPath := filepath.Join(root, "acoustic_package_status.json")
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", statusPath)
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-22T08:00:00Z",
		"latest_request":{
			"request_id":"old_blocked_selection",
			"status":"blocked",
			"reason":"clip_source_required_for_current_feature_bakers",
			"resolved_target":{"kind":"selection","id":"Track 1","track_id":"Track 1"}
		},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"1007","clip_id":"clip_a","request_id":"older_req_1","rms":0.2,"peak_abs":0.5,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.5,"energy_state":"high"}]},
			{"status":"ready","track_id":"1010","clip_id":"clip_b","request_id":"older_req_2","rms":0.1,"peak_abs":0.7,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.1,"peak_abs":0.7,"energy_state":"medium"}]}
		],
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"},
		"band_energy_summary":{"status":"missing"},
		"stereo_relation_summary":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	project := shadowProjectWithClips()
	h := New(nil, project, nil)
	state := h.UserStateSummary(context.Background())
	cmd := map[string]any{
		"mix_session_id":      "mix_project_read_first_refresh",
		"scope":               "full_project",
		"mixboard_request_id": "current_project_refresh",
	}
	target := mixTargetFromCommand(cmd)
	resolved := resolveMixObservationTargetContext(cmd, state, target)
	cmd = canonicalizeMixObservationCommand(cmd, target, resolved)
	identity := acousticpackage.IdentityFromMaps(state, resolved, cmd)
	status := acousticpackage.BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{
			"status":        "ready",
			"track_id":      "1007",
			"clip_id":       "clip_a",
			"rms":           0.2,
			"peak_abs":      0.8,
			"time_segments": []any{map[string]any{"start_seconds": 0, "end_seconds": 2}},
		},
	}, "2026-06-22T00:00:00Z", "test")
	status = acousticpackage.MarkBackgroundRequested(status, "test", "already_building", "2026-06-22T00:00:01Z")
	if _, err := acousticpackage.NewStore(statusPath).Upsert(status); err != nil {
		t.Fatal(err)
	}
	kernelState := testMap(t, testMap(t, project.Snapshot())["engine_snapshot"])
	kernel := &fakeKernelClient{replies: []map[string]any{kernelState}}
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "mix.observe",
		Args:      cmd,
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if _, ok := resp.Result["feature_request"]; ok {
		t.Fatalf("feature_request should be absent for read-only full-project observe: %+v", resp.Result["feature_request"])
	}
	if featureCommands := testCommandsByName(kernel.commands, "request_audio_feature"); len(featureCommands) != 0 {
		t.Fatalf("feature kernel commands = %+v all=%+v", featureCommands, kernel.commands)
	}
	acousticStatus := testMap(t, resp.Result["acoustic_package_status"])
	l3 := testMap(t, testMap(t, acousticStatus["package_layers"])["l3_deep"])
	if firstString(l3, "status") != acousticpackage.StatusBuilding {
		t.Fatalf("stored building package should be preserved: %+v", l3)
	}
}

func TestInvokeMixObserveFullProjectAutoTriggersL3BandAnalysisWhenMissing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	t.Setenv("VIT_MIXBOARD_BACKGROUND_SUBSCRIBE_WAIT_MS", "1")
	t.Setenv("VIT_MIXBOARD_BACKGROUND_SEND_WAIT_MS", "50")
	project := shadowProjectWithClips()
	h := New(nil, project, nil)
	kernelState := testMap(t, testMap(t, project.Snapshot())["engine_snapshot"])
	kernel := &fakeKernelClient{replies: []map[string]any{kernelState}}
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id": "mix_full_project_auto_trigger",
			"scope":          "full_project",
			"mom_intent":     "project_multitrack_relation_observation",
			"goal_text":      "low end relations",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	featureRequest := testMap(t, resp.Result["feature_request"])
	if firstString(featureRequest, "status") != "requested" {
		t.Fatalf("feature_request should report requested when L3 core is missing: %+v", featureRequest)
	}
	if firstString(featureRequest, "scope") != "full_project" {
		t.Fatalf("feature_request scope = %+v", featureRequest)
	}
	bandCommands := 0
	for _, cmd := range testCommandsByName(kernel.commands, "warm_waveform_bake") {
		if fmt.Sprint(cmd["feature_type"]) == "l3_acoustic_summary" {
			bandCommands++
		}
	}
	if bandCommands == 0 {
		t.Fatalf("expected at least one l3_acoustic_summary warm_waveform_bake command, commands=%+v", kernel.commands)
	}
}

func TestInvokeMixRequestObservationResolvesVisibleTrackClipSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{
						"id":                  "1011",
						"name":                "Paper Crown",
						"file_path":           "D:\\Vit_DAW\\Paper Crown.mp3",
						"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
						"length_seconds":      219.384,
					},
				},
			},
		},
	})
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_resolve_clip",
			"round":          1,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	resolved := testMap(t, resp.Result["resolved_target"])
	if resolved["track_id"] != "1007" || resolved["clip_id"] != "1011" {
		t.Fatalf("resolved target = %+v", resolved)
	}
	acoustic, _ := resp.Result["acoustic_digest"].(map[string]any)
	if acoustic["clip_id"] != "1011" || acoustic["file_path"] == "" {
		t.Fatalf("acoustic digest missing resolved clip data: %+v", acoustic)
	}
}

func TestInvokeMixRequestObservationResolvesTrackAliasAmongMultipleTracks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	kernel := &fakeKernelClient{}
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1007",
				"track_name":       "Track 1",
				"track_type":       "hybrid",
				"user_track_index": 1,
				"is_audio_track":   true,
				"clips": []any{
					map[string]any{
						"id":                  "1011",
						"name":                "Paper Crown",
						"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
						"length_seconds":      219.384,
					},
				},
			},
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"track_type":       "hybrid",
				"user_track_index": 2,
				"is_audio_track":   true,
				"clips":            []any{},
			},
		},
	})
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_track_alias",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("kernel commands = %+v resp=%+v", kernel.commands, resp.Result)
	}
	resolved := testMap(t, resp.Result["resolved_target"])
	if resolved["track_id"] != "1007" || resolved["clip_id"] != "1011" {
		t.Fatalf("track alias did not resolve to audio clip source: %+v", resolved)
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1007" {
		t.Fatalf("board target was not canonicalized: %+v", board.TargetRef)
	}
	if len(board.MixObjects) == 0 || board.MixObjects[0].ID != "1007" {
		t.Fatalf("board mix objects were not canonicalized: %+v", board.MixObjects)
	}
}

func TestInvokeMixRequestObservationUsesRenamedVisibleTrackLabel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"track_type":       "hybrid",
				"user_track_index": 2,
				"is_audio_track":   true,
				"clips": []any{
					map[string]any{
						"id":                  "2012",
						"name":                "test_100hz_10s",
						"current_source_path": "D:\\Vit_DAW\\test_100hz_10s.wav",
						"length_seconds":      10.0,
					},
				},
			},
		},
	})
	project.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1012",
		"action":     "property_changed:name",
		"value":      "vocal",
	})
	kernel := &fakeKernelClient{}
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_renamed_label",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_2",
				"label": "Track 2",
			},
		},
		Context: map[string]any{
			"selected_track_id":   "1012",
			"selected_track_name": "Track 2",
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1012" || board.TargetRef.Label != "vocal" {
		t.Fatalf("board target did not use renamed visible label: %+v", board.TargetRef)
	}
	acoustic, _ := resp.Result["acoustic_digest"].(map[string]any)
	if acoustic["track_name"] != "vocal" || acoustic["user_label"] != "vocal" {
		t.Fatalf("acoustic digest missing renamed label: %+v", acoustic)
	}
	obs := resp.Result["observation"].(mixboard.ObservationPacket)
	read, err := mixboard.NewStore(filepath.Join(root, "mixboard")).Read(mixboard.ReadRequest{
		ObservationID: obs.ObservationID,
		Keys:          []string{"project.tracks.summary", "track.1012.static.identity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	tracks := items["project.tracks.summary"].(map[string]any)
	rows := tracks["tracks"].([]map[string]any)
	if rows[0]["track_name"] != "vocal" || rows[0]["user_label"] != "vocal" {
		t.Fatalf("project track summary missing renamed label: %+v", rows[0])
	}
	identity := items["track.1012.static.identity"].(map[string]any)
	trackIdentity := identity["track_identity"].(map[string]any)
	if trackIdentity["track_name"] != "vocal" || trackIdentity["user_label"] != "vocal" {
		t.Fatalf("identity missing renamed label: %+v", trackIdentity)
	}
}

func TestInvokeMixRequestObservationRefreshesStaleAliasTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "old_track",
				"track_name":     "Old Track",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"clips":          []any{},
			},
		},
	})
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status": "ok",
			"tracks": []any{
				map[string]any{
					"track_id":         "1007",
					"track_name":       "Track 1",
					"track_type":       "hybrid",
					"user_track_index": 1,
					"is_audio_track":   true,
					"clips": []any{
						map[string]any{
							"id":                  "1011",
							"name":                "Paper Crown",
							"file_path":           "D:\\Vit_DAW\\Paper Crown.mp3",
							"current_source_path": "D:\\Vit_DAW\\Paper Crown.mp3",
							"length_seconds":      219.384,
						},
					},
				},
			},
		},
		{"status": "ok"},
	}}
	h := New(nil, project, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_stale_alias",
			"goal_text":      "auto mix",
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    "track_1",
				"label": "Vocal",
			},
			"mix_objects": []any{map[string]any{
				"mode":         "target_ref",
				"kind":         "track",
				"id":           "track_1",
				"label":        "Vocal",
				"effect_scope": "track_rack",
				"source":       "target_ref",
			}},
			"listen_scope": map[string]any{
				"source": map[string]any{"focus_ids": []any{"track_1"}},
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kernel.commands) != 1 {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
	if kernel.commands[0]["cmd"] != "get_project_state" {
		t.Fatalf("first command should refresh shadow: %+v", kernel.commands)
	}
	if _, ok := resp.Result["feature_request"]; ok {
		t.Fatalf("feature_request should be absent for read-only observation: %+v", resp.Result["feature_request"])
	}
	resolved := testMap(t, resp.Result["resolved_target"])
	if resolved["track_id"] != "1007" || resolved["clip_id"] != "1011" {
		t.Fatalf("resolved target = %+v", resolved)
	}
	board, _ := resp.Result["mixboard"].(mixboard.Board)
	if board.TargetRef.ID != "1007" {
		t.Fatalf("board target = %+v", board.TargetRef)
	}
	if len(board.MixObjects) == 0 || board.MixObjects[0].ID != "1007" {
		t.Fatalf("mix objects = %+v", board.MixObjects)
	}
	if len(board.ListenScope.Source.FocusIDs) != 1 || board.ListenScope.Source.FocusIDs[0] != "1007" {
		t.Fatalf("listen scope = %+v", board.ListenScope)
	}
}

func TestWaveformFeatureCollectorWritesReadySnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	cmd := map[string]any{
		"mix_session_id":   "mix_waveform_snapshot",
		"duration_seconds": 10.0,
	}
	packet := newMixboardFeatureRequestPacket(cmd, mixboard.TargetRef{Kind: "track", ID: "1007"})
	packet["status"] = "requested"
	packet["resolved_target"] = map[string]any{"track_id": "1007", "clip_id": "1011"}
	collector := &waveformFeatureCollector{
		TrackID:        "1007",
		ClipID:         "1011",
		FilePath:       "D:\\Vit_DAW\\Paper Crown.mp3",
		TotalDuration:  10,
		ExpectedTiles:  2,
		TilesSeen:      2,
		PeakAbs:        0.5,
		SumSquares:     0.01 + 0.04,
		SampleFrames:   2,
		FloatCount:     24,
		TimeSegments:   []map[string]any{{"start_seconds": 0.0, "end_seconds": 5.0, "rms": 0.1, "peak_abs": 0.3}, {"start_seconds": 5.0, "end_seconds": 10.0, "rms": 0.2, "peak_abs": 0.5}},
		LastReceivedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	writeMixboardReadyWaveformSnapshot(cmd, packet, collector.SnapshotRow(firstString(packet, "request_id")))
	result, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: "mix_waveform_snapshot",
		TargetRef:    mixboard.TargetRef{Kind: "track", ID: "1007"},
		ProjectState: map[string]any{"duration_seconds": 10.0},
		Args:         cmd,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" {
		t.Fatalf("status = %q observation=%+v", result.Status, result.Observation)
	}
	metrics := result.Observation.MixPackage["current_metrics"].(map[string]any)
	waveform := metrics["waveform"].(map[string]any)
	if waveform["peak_dbfs"] == nil || waveform["rms_dbfs"] == nil || waveform["headroom_db"] == nil {
		t.Fatalf("waveform metrics missing: %+v", waveform)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q", got)
	}
}

func TestMixRequestObservationDoesNotReuseStaleBridgeFeatureSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	sourceRevision := testAcousticSourceRevision("mix_ready_snapshot", "1007", "clip_a", 2)
	if err := os.WriteFile(snapshotPath, []byte(fmt.Sprintf(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"1007","clip_id":"clip_a","source_revision":%q,"duration_seconds":2,"rms":0.2,"peak_abs":0.7},
		"band_energy_summary":{"status":"ready","track_id":"1007","clip_id":"clip_a","source":"live_level_meter_spectrum"},
		"stereo_relation_summary":{"status":"ready","track_id":"1007","clip_id":"clip_a","source":"live_level_meter_stereo","correlation_state":"stable"},
		"spectrogram_tiles":{"status":"ready","track_id":"1007","clip_id":"clip_a","tile_count_seen":2,"tile_count_expected":2}
	}`, sourceRevision)), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_ready_snapshot",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(resp.Result, "status") != "ready" {
		t.Fatalf("result = %+v", resp.Result)
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	waveform, _ := snapshot["waveform_envelope"].(map[string]any)
	if firstString(waveform, "status") != "ready" || firstString(waveform, "track_id") != "1007" || firstString(waveform, "clip_id") != "clip_a" {
		t.Fatalf("snapshot downgraded ready waveform: %+v\n%s", waveform, string(data))
	}
	metrics, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	current, _ := metrics.MixPackage["current_metrics"].(map[string]any)
	observedWaveform, _ := current["waveform"].(map[string]any)
	if observedWaveform["peak_dbfs"] == nil || observedWaveform["rms_dbfs"] == nil || observedWaveform["headroom_db"] == nil {
		t.Fatalf("observation did not consume ready waveform metrics: %+v", observedWaveform)
	}
	status := testMap(t, resp.Result["acoustic_package_status"])
	l3 := testMap(t, testMap(t, status["package_layers"])["l3_deep"])
	features := testMap(t, l3["features"])
	for _, featureName := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"} {
		feature := testMap(t, features[featureName])
		if firstString(feature, "status") == acousticpackage.StatusReady {
			t.Fatalf("stale %s was reused as ready: %+v package=%+v", featureName, feature, status)
		}
	}
}

func TestInvokeMixObserveKeepsExplicitSourceIdentityWhenLiveTargetDiffers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(root, "acoustic_package_status.json"))
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	statusPath := filepath.Join(root, "explicit_acoustic_package_status.json")
	paperPath := filepath.Join(root, "Paper Crown.mp3")
	oldPath := filepath.Join(root, "test_100hz_10s.wav")

	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{
						"id":             "1014",
						"name":           "Old 100Hz",
						"file_path":      oldPath,
						"length_seconds": 10.0,
					},
				},
			},
		},
	})
	featureSnapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"waveform_envelope": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "clip_a", "file_path": paperPath, "source_path": paperPath, "source_revision": "rev_paper", "duration_seconds": 219.0, "peak_abs": 0.55, "rms": 0.12,
		},
		"track_waveform_envelopes": []any{
			map[string]any{"status": "ready", "track_id": "1007", "clip_id": "1014", "file_path": oldPath, "source_path": oldPath, "source_revision": "rev_100hz", "duration_seconds": 10.0, "peak_dbfs": -6.02, "rms_dbfs": -9.03},
			map[string]any{"status": "ready", "track_id": "1007", "clip_id": "clip_a", "file_path": paperPath, "source_path": paperPath, "source_revision": "rev_paper", "duration_seconds": 219.0, "peak_dbfs": -5.19, "rms_dbfs": -18.41},
		},
		"spectrogram_tiles": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "1014", "file_path": oldPath, "source_path": oldPath, "source_revision": "rev_100hz", "duration_seconds": 10.0, "tile_count_seen": 2, "tile_count_expected": 2,
		},
		"spectrogram_tile_rows": []any{
			map[string]any{"status": "partial", "track_id": "1007", "clip_id": "clip_a", "file_path": paperPath, "source_path": paperPath, "source_revision": "rev_paper", "duration_seconds": 219.0, "tile_count_seen": 12, "tile_count_expected": 101, "coverage_seconds": 60},
		},
		"band_energy_summary": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "1014", "file_path": oldPath, "source_path": oldPath, "source_revision": "rev_100hz", "duration_seconds": 10.0, "bands": map[string]any{"bass": map[string]any{"energy_db": -6.02, "unit_energy": 0.5}},
		},
		"band_energy_summaries": []any{
			map[string]any{"status": "partial", "track_id": "1007", "clip_id": "clip_a", "file_path": paperPath, "source_path": paperPath, "source_revision": "rev_paper", "duration_seconds": 219.0, "coverage_seconds": 60, "bands": map[string]any{"bass": map[string]any{"energy_db": -18.2, "unit_energy": 0.12}}},
		},
		"stereo_relation_summary": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "1014", "file_path": oldPath, "source_path": oldPath, "source_revision": "rev_100hz", "duration_seconds": 10.0, "balance_db": 0, "correlation_estimate": 1.0,
		},
		"stereo_relation_summaries": []any{
			map[string]any{"status": "partial", "track_id": "1007", "clip_id": "clip_a", "file_path": paperPath, "source_path": paperPath, "source_revision": "rev_paper", "duration_seconds": 219.0, "coverage_seconds": 60, "balance_db": 0.3, "correlation_estimate": 0.72},
		},
	}
	data, err := json.MarshalIndent(featureSnapshot, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, project, nil)
	h.kernel = &fakeKernelClient{}

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"mix_session_id":               "mix_source_identity",
			"scope":                        "selected_track",
			"track_id":                     "1007",
			"clip_id":                      "clip_a",
			"file_path":                    paperPath,
			"source_revision":              "rev_paper",
			"duration_seconds":             219.0,
			"feature_snapshot_path":        snapshotPath,
			"acoustic_package_status_path": statusPath,
			"projection":                   "frequency_stereo",
			"include_raw":                  false,
			"feature_keys":                 []any{"band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"},
			"observation_only":             true,
			"mutation_barrier":             true,
			"no_pending":                   true,
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	status := testMap(t, resp.Result["acoustic_package_status"])
	if firstString(status, "source_revision") != "rev_paper" || firstString(status, "file_path", "source_path") != paperPath || firstString(status, "clip_id") != "clip_a" || numberFromAny(status["duration_seconds"]) != 219 {
		t.Fatalf("acoustic status mixed explicit source identity with live target: %+v", status)
	}
	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{oldPath, "rev_100hz", "-6.02", "-9.03"} {
		if strings.Contains(string(resultJSON), forbidden) {
			t.Fatalf("old source evidence leaked into observe result: %s\n%s", forbidden, string(resultJSON))
		}
	}
}

func TestFinalizeMixboardFeatureSnapshotPreservesFreshBridgeRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"1007","clip_id":"1011","request_id":"req_current","source":"kernel_tile_ready_direct_collector","tile_count_seen":2,"tile_count_expected":2},
		"band_energy_summary":{"status":"ready","track_id":"1007","clip_id":"1011","request_id":"req_current","source":"live_level_meter_spectrum"},
		"stereo_relation_summary":{"status":"ready","track_id":"1007","clip_id":"1011","request_id":"req_current","source":"live_level_meter_stereo","correlation_state":"stable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"request_id": "req_current",
		"resolved_target": map[string]any{
			"track_id": "1007",
			"clip_id":  "1011",
		},
		"requested_features": []any{
			map[string]any{"feature_type": "waveform_envelope"},
			map[string]any{"feature_type": "spectral_field"},
		},
		"spectral_tile_ready_seen": true,
	}
	finalizeMixboardFeatureSnapshotAfterWait(map[string]any{"feature_snapshot_path": snapshotPath}, packet, []map[string]any{{"track_id": "1007", "clip_id": "1011"}})
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	spectral := testMap(t, snapshot["spectrogram_tiles"])
	if firstString(spectral, "status") != "ready" || firstString(spectral, "request_id") != "req_current" || firstString(spectral, "source") != "kernel_tile_ready_direct_collector" {
		t.Fatalf("fresh spectral row not preserved: %+v\n%s", spectral, string(data))
	}
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "status") != "ready" || firstString(band, "request_id") != "req_current" || firstString(band, "source") != "live_level_meter_spectrum" {
		t.Fatalf("fresh band row not preserved: %+v\n%s", band, string(data))
	}
	stereo := testMap(t, snapshot["stereo_relation_summary"])
	if firstString(stereo, "status") != "ready" || firstString(stereo, "request_id") != "req_current" || firstString(stereo, "correlation_state") != "stable" {
		t.Fatalf("fresh stereo row not preserved: %+v\n%s", stereo, string(data))
	}
}

func TestSpectralFeatureCollectorSnapshotRowCarriesTileReadyMetadata(t *testing.T) {
	collector := &spectralFeatureCollector{TrackID: "1007", ClipID: "1011", FeatureType: "spectral_field", LastTileIndex: -1}
	collector.AddEvent(map[string]any{
		"command":                    "tile_ready",
		"feature_type":               "spectral_field",
		"track_id":                   "1007",
		"clip_id":                    "1011",
		"file_path":                  "D:\\Vit_DAW\\Paper Crown.mp3",
		"tile_index":                 0,
		"tile_count":                 2,
		"tile_duration":              5.0,
		"tile_content_start_seconds": 0.0,
		"total_duration":             9.5,
		"resolution_frame_width":     256,
		"resolution_frequency_bins":  128,
		"shared_memory":              "vit_spectral_tile_1007_1011_0",
	})
	row := collector.SnapshotRow("req_spectral")
	if firstString(row, "status") != "partial" || firstString(row, "feature_type") != "spectral_field" || firstString(row, "source") != "kernel_tile_ready_direct_collector" {
		t.Fatalf("unexpected spectral row identity: %+v", row)
	}
	if firstString(row, "track_id") != "1007" || firstString(row, "clip_id") != "1011" || firstString(row, "request_id") != "req_spectral" {
		t.Fatalf("unexpected spectral row target: %+v", row)
	}
	if int(numberFromAny(row["tile_count_seen"])) != 1 || int(numberFromAny(row["tile_count_expected"])) != 2 {
		t.Fatalf("unexpected spectral tile counts: %+v", row)
	}
	if int(numberFromAny(row["resolution_frame_width"])) != 256 || int(numberFromAny(row["resolution_frequency_bins"])) != 128 {
		t.Fatalf("unexpected spectral resolution metadata: %+v", row)
	}
	if firstString(row, "shared_memory") != "vit_spectral_tile_1007_1011_0" {
		t.Fatalf("shared memory metadata missing: %+v", row)
	}
}

func TestSpectralFeatureCollectorQualityFailurePreventsReady(t *testing.T) {
	collector := &spectralFeatureCollector{TrackID: "1007", ClipID: "1011", FeatureType: "spectral_field", LastTileIndex: -1}
	collector.AddEvent(map[string]any{
		"command":                   "tile_ready",
		"feature_type":              "spectral_field",
		"track_id":                  "1007",
		"clip_id":                   "1011",
		"file_path":                 "D:\\Vit_DAW\\Paper Crown.mp3",
		"source_revision":           "rev_paper",
		"clip_revision":             "cliprev_paper",
		"tile_index":                0,
		"tile_count":                1,
		"total_duration":            5.0,
		"resolution_frame_width":    500,
		"resolution_frequency_bins": 336,
		"float_count":               672000,
		"quality_status":            "failed",
		"quality_reason":            "tile_all_zero_before_write",
		"nonzero_count":             0,
	})
	row := collector.SnapshotRow("req_spectral")
	if firstString(row, "status") == "ready" {
		t.Fatalf("quality-failed spectral row must not be ready: %+v", row)
	}
	if firstString(row, "quality_status") != "failed" || firstString(row, "reason") != "tile_all_zero_before_write" {
		t.Fatalf("quality evidence missing from spectral row: %+v", row)
	}
}

func TestWaveformFeatureCollectorCountsUniqueTilesForReady(t *testing.T) {
	collector := &waveformFeatureCollector{
		TrackID:         "1007",
		ClipID:          "1011",
		ExpectedTiles:   2,
		TilesSeen:       3,
		TileIndexes:     map[int]bool{0: true, 1: true},
		RawTileEvents:   3,
		QualityStatus:   "ready",
		SumSquares:      2,
		SampleFrames:    2,
		TimeSegments:    []map[string]any{{"start_seconds": 0.0, "end_seconds": 5.0}, {"start_seconds": 5.0, "end_seconds": 10.0}},
		FirstReceivedAt: "2026-06-23T00:00:00Z",
		LastReceivedAt:  "2026-06-23T00:00:01Z",
	}

	if !collector.Complete() {
		t.Fatalf("collector should be complete from unique tile coverage")
	}
	row := collector.SnapshotRow("req_waveform")
	if int(numberFromAny(row["tile_count_seen"])) != 2 {
		t.Fatalf("tile_count_seen should use unique tile count: %+v", row)
	}
	if int(numberFromAny(row["tile_event_count"])) != 3 {
		t.Fatalf("tile_event_count should preserve raw duplicate evidence: %+v", row)
	}
	if firstString(row, "status") != "ready" {
		t.Fatalf("duplicate tile events should not prevent ready: %+v", row)
	}
}

func TestWaveformFeatureCollectorRequiresCoverageForReady(t *testing.T) {
	collector := &waveformFeatureCollector{
		TrackID:        "1007",
		ClipID:         "1011",
		ExpectedTiles:  2,
		TilesSeen:      1,
		QualityStatus:  "ready",
		QualityReason:  "ok",
		PeakAbs:        0.5,
		SumSquares:     1,
		SampleFrames:   1,
		NonzeroCount:   10,
		SumAbs:         1,
		MaxAbs:         0.5,
		TimeSegments:   []map[string]any{{"start_seconds": 0.0, "end_seconds": 5.0}},
		LastReceivedAt: "2026-06-23T00:00:01Z",
	}

	row := collector.SnapshotRow("req_waveform")
	if firstString(row, "status") != "partial" {
		t.Fatalf("incomplete coverage must not be ready: %+v", row)
	}
}

func TestWaveformFeatureCollectorAllowsSilentTilesWhenAggregateIsNonzero(t *testing.T) {
	collector := &waveformFeatureCollector{
		TrackID:        "1022",
		ClipID:         "1026",
		ExpectedTiles:  2,
		TilesSeen:      2,
		QualityStatus:  "suspect",
		QualityReason:  "input_all_zero; ok",
		PeakAbs:        0.5,
		SumSquares:     1,
		SampleFrames:   2,
		NonzeroCount:   10,
		SumAbs:         1,
		MaxAbs:         0.5,
		TimeSegments:   []map[string]any{{"start_seconds": 0.0, "end_seconds": 5.0}, {"start_seconds": 5.0, "end_seconds": 10.0}},
		LastReceivedAt: "2026-06-23T00:00:01Z",
	}

	row := collector.SnapshotRow("req_waveform")
	if firstString(row, "status") != "ready" {
		t.Fatalf("aggregate nonzero waveform with silent tiles should be ready: %+v", row)
	}
	if firstString(row, "quality_status") != "ready" || firstString(row, "quality_reason") != "ok_with_silent_tiles" {
		t.Fatalf("aggregate quality should be normalized with evidence: %+v", row)
	}
	if firstString(row, "tile_quality_status") != "suspect" || firstString(row, "tile_quality_reason") != "input_all_zero; ok" {
		t.Fatalf("tile quality evidence should be retained: %+v", row)
	}
}

func TestSpectralTileDoesNotMaterializeL3Summaries(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "req_tile_derived",
		"resolved_target": map[string]any{
			"track_id": "1007",
			"clip_id":  "1011",
		},
		"requested_features": []any{
			map[string]any{"feature_type": "spectral_field"},
		},
		"spectral_tile_ready_seen": true,
	}
	collector, _ := testSpectralCollectorWithTile(t, 24, 32, 2, func(bin, frame int) (float32, float32, float32, float32) {
		return 0.5, 0.45, 0.1, 0.7
	})
	spectral := collector.SnapshotRow("req_tile_derived")
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)
	finalizeMixboardFeatureSnapshotAfterWait(cmd, packet, []map[string]any{{"track_id": "1007", "clip_id": "1011"}})
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	gotBand := testMap(t, snapshot["band_energy_summary"])
	gotStereo := testMap(t, snapshot["stereo_relation_summary"])
	gotLoudness := testMap(t, snapshot["loudness_summary"])
	if firstString(gotBand, "status") != "missing" || firstString(gotBand, "reason") != "l3_acoustic_summary_not_requested_for_downstream_summary" {
		t.Fatalf("band summary should wait for L3 analyzer: %+v\n%s", gotBand, string(data))
	}
	if firstString(gotStereo, "status") != "missing" || firstString(gotStereo, "reason") != "l3_acoustic_summary_not_requested_for_downstream_summary" {
		t.Fatalf("stereo summary should wait for L3 analyzer: %+v\n%s", gotStereo, string(data))
	}
	if firstString(gotLoudness, "status") != "missing" || firstString(gotLoudness, "reason") != "l3_acoustic_summary_not_requested_for_downstream_summary" {
		t.Fatalf("loudness summary should wait for L3 analyzer: %+v\n%s", gotLoudness, string(data))
	}
}

func TestWriteMixboardReadySpectralSnapshotPromotesGrowingSpectralCoverageOnly(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_spectral_field_1011",
		"resolved_target": map[string]any{
			"track_id": "1007",
			"clip_id":  "1011",
		},
	}
	spectral := map[string]any{
		"status":              "partial",
		"feature_type":        "spectral_field",
		"source":              "kernel_tile_ready_direct_collector",
		"track_id":            "1007",
		"clip_id":             "1011",
		"tile_count_seen":     1,
		"tile_count_expected": 5,
		"coverage_seconds":    5.0,
		"total_duration":      25.0,
	}
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)

	spectral["tile_count_seen"] = 5
	spectral["coverage_seconds"] = 25.0
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	gotSpectral := testMap(t, snapshot["spectrogram_tiles"])
	if numberFromAny(gotSpectral["coverage_seconds"]) != 25 || int(numberFromAny(gotSpectral["tile_count_seen"])) != 5 {
		t.Fatalf("spectral summary did not promote later coverage: %+v\n%s", gotSpectral, string(data))
	}
	if got := firstString(testMap(t, snapshot["band_energy_summary"]), "status"); got != "missing" {
		t.Fatalf("spectral write should not materialize band summary: %s\n%s", got, string(data))
	}
}

func TestWriteMixboardReadySpectralSnapshotDoesNotReplaceLiveMeterWithDerivedSummary(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summary":{"status":"ready","source":"live_level_meter_spectrum","request_id":"kernel_prepared_spectral_field_1011","track_id":"1007","clip_id":"1011","bands":{"bass":{"unit_energy":0}}},
		"stereo_relation_summary":{"status":"ready","source":"live_level_meter_stereo","request_id":"kernel_prepared_spectral_field_1011","track_id":"1007","clip_id":"1011","correlation_estimate":0}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_spectral_field_1011",
		"resolved_target": map[string]any{
			"track_id": "1007",
			"clip_id":  "1011",
		},
	}
	spectral := map[string]any{"status": "partial", "feature_type": "spectral_field", "source": "kernel_tile_ready_direct_collector", "track_id": "1007", "clip_id": "1011", "tile_count_seen": 1, "tile_count_expected": 2}
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	if got := firstString(testMap(t, snapshot["band_energy_summary"]), "source"); got != "live_level_meter_spectrum" {
		t.Fatalf("band live summary should not be replaced by spectral tile path: %s\n%s", got, string(data))
	}
	if got := firstString(testMap(t, snapshot["stereo_relation_summary"]), "source"); got != "live_level_meter_stereo" {
		t.Fatalf("stereo live summary should not be replaced by spectral tile path: %s\n%s", got, string(data))
	}
}

func TestWriteMixboardReadySpectralSnapshotRelabelsMissingButPreservesReadyL3Lineage(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summary":{"status":"missing","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_waveform_envelope_1016","reason":"awaiting_current_target_band_summary"},
		"stereo_relation_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","request_id":"older_l3_stereo_request","source_revision":"rev_current","correlation_estimate":0.4},
		"loudness_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","source_revision":"rev_current","approximate_lufs":-18.4}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_spectral_field_1016",
		"resolved_target": map[string]any{
			"track_id": "1010",
			"clip_id":  "1016",
		},
	}
	spectral := map[string]any{"status": "ready", "feature_type": "spectral_field", "source": "kernel_tile_ready_direct_collector", "track_id": "1010", "clip_id": "1016", "request_id": "kernel_prepared_spectral_field_1016", "tile_count_seen": 2, "tile_count_expected": 2}
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(band, "reason") != "awaiting_current_target_band_summary" {
		t.Fatalf("band row was not relabelled for latest request: %+v\n%s", band, string(data))
	}
	stereo := testMap(t, snapshot["stereo_relation_summary"])
	if firstString(stereo, "request_id") != "older_l3_stereo_request" || firstString(stereo, "feature_type") != "stereo_relation_summary" || firstString(stereo, "source_revision") != "rev_current" {
		t.Fatalf("ready L3 stereo row should keep material lineage instead of latest request_id: %+v\n%s", stereo, string(data))
	}
	loudness := testMap(t, snapshot["loudness_summary"])
	if firstString(loudness, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(loudness, "feature_type") != "loudness_summary" || firstString(loudness, "source_revision") != "rev_current" {
		t.Fatalf("ready L3 loudness row without request_id should be attributed to latest request: %+v\n%s", loudness, string(data))
	}
}

func TestMixObservationCommandWithLiveProjectIdentityReplacesOnlyGenericAlias(t *testing.T) {
	state := map[string]any{"project_id": "vitproj_goal5_live"}
	for _, requested := range []string{"", "current", "project_current", "current_project"} {
		cmd := map[string]any{"project_id": requested}
		got := mixObservationCommandWithLiveProjectIdentity(cmd, state)
		if firstString(got, "project_id") != "vitproj_goal5_live" {
			t.Fatalf("requested=%q project_id=%q, want live identity", requested, firstString(got, "project_id"))
		}
	}

	cmd := map[string]any{"project_id": "vitproj_explicit_other"}
	got := mixObservationCommandWithLiveProjectIdentity(cmd, state)
	if firstString(got, "project_id") != "vitproj_explicit_other" {
		t.Fatalf("specific requested identity must not be overwritten: %+v", got)
	}
}

func TestStampMixboardFeatureRowIdentityUsesSpecificPacketProjectForCurrentKernelAlias(t *testing.T) {
	packet := map[string]any{
		"project_id": "vitproj_goal5_live",
		"session_id": "goal5_blind",
	}
	target := map[string]any{"track_id": "1032", "clip_id": "1036"}
	row := map[string]any{"project_id": "current", "status": "ready", "feature_type": "band_energy_summary"}
	stamped := stampMixboardFeatureRowIdentity(row, packet, target)
	if firstString(stamped, "project_id") != "vitproj_goal5_live" {
		t.Fatalf("generic kernel project alias was not stamped with live identity: %+v", stamped)
	}
	if firstString(stamped, "track_id") != "1032" || firstString(stamped, "clip_id") != "1036" {
		t.Fatalf("target identity missing from stamped row: %+v", stamped)
	}

	foreign := map[string]any{"project_id": "vitproj_specific_foreign", "status": "ready"}
	stampedForeign := stampMixboardFeatureRowIdentity(foreign, packet, target)
	if firstString(stampedForeign, "project_id") != "vitproj_specific_foreign" {
		t.Fatalf("specific row identity must remain available for mismatch rejection: %+v", stampedForeign)
	}
}

func TestKernelFeatureMaterializerLiveProjectIDRequiresExactCurrentSource(t *testing.T) {
	state := map[string]any{
		"project_id": "vitproj_goal5_live",
		"tracks": []any{
			map[string]any{
				"track_id":       "1032",
				"is_audio_track": true,
				"clips": []any{
					map[string]any{
						"clip_id":             "1036",
						"current_source_path": `C:\fixtures\g5_02\stems\Vocals.wav`,
						"length_seconds":      20.0,
					},
				},
			},
		},
	}
	current := map[string]any{
		"track_id":  "1032",
		"clip_id":   "1036",
		"file_path": `c:\FIXTURES\g5_02\stems\Vocals.wav`,
	}
	if got := kernelFeatureMaterializerLiveProjectID(state, current); got != "vitproj_goal5_live" {
		t.Fatalf("current source project_id=%q", got)
	}
	lateForeign := map[string]any{
		"track_id":  "1032",
		"clip_id":   "1036",
		"file_path": `C:\fixtures\g5_01\stems\Vocals.wav`,
	}
	if got := kernelFeatureMaterializerLiveProjectID(state, lateForeign); got != "" {
		t.Fatalf("late foreign event was attributed to live project: %q", got)
	}
	missingSource := map[string]any{"track_id": "1032", "clip_id": "1036"}
	if got := kernelFeatureMaterializerLiveProjectID(state, missingSource); got != "" {
		t.Fatalf("unanchored event was attributed to live project: %q", got)
	}
}

func TestWriteMixboardReadySpectralSnapshotPromotesReadyL3HistoryRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summary":{"status":"missing","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_waveform_envelope_1016","reason":"awaiting_current_target_band_summary"},
		"band_energy_summaries":[{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","source_path":"D:/Vit_DAW/test_target_3s.wav","source_revision":"rev_current","duration_seconds":3,"bands":{"bass":{"energy_db":-12}}}],
		"stereo_relation_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_stereo_relation_summary_1016","source_path":"D:/Vit_DAW/test_target_3s.wav","source_revision":"rev_current","duration_seconds":3,"correlation_state":"stable"},
		"loudness_summary":{"status":"missing","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_waveform_envelope_1016","reason":"awaiting_current_target_loudness_summary"},
		"loudness_summaries":[{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","source_path":"D:/Vit_DAW/test_target_3s.wav","source_revision":"rev_current","duration_seconds":3,"approximate_lufs":-18.4}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_spectral_field_1016",
		"resolved_target": map[string]any{
			"track_id":         "1010",
			"clip_id":          "1016",
			"source_path":      "D:/Vit_DAW/test_target_3s.wav",
			"duration_seconds": 3,
		},
	}
	spectral := map[string]any{"status": "ready", "feature_type": "spectral_field", "source": "kernel_tile_ready_direct_collector", "track_id": "1010", "clip_id": "1016", "request_id": "kernel_prepared_spectral_field_1016", "source_path": "D:/Vit_DAW/test_target_3s.wav", "duration_seconds": 3, "tile_count_seen": 1, "tile_count_expected": 1}
	writeMixboardReadySpectralSnapshot(cmd, packet, spectral)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "status") != "ready" || firstString(band, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(band, "source_revision") != "rev_current" {
		t.Fatalf("ready L3 band history row was not promoted: %+v\n%s", band, string(data))
	}
	stereo := testMap(t, snapshot["stereo_relation_summary"])
	if firstString(stereo, "status") != "ready" || firstString(stereo, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(stereo, "source_revision") != "rev_current" {
		t.Fatalf("ready sibling L3 stereo request was not normalized: %+v\n%s", stereo, string(data))
	}
	loudness := testMap(t, snapshot["loudness_summary"])
	if firstString(loudness, "status") != "ready" || firstString(loudness, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(loudness, "source_revision") != "rev_current" {
		t.Fatalf("ready L3 loudness history row was not promoted: %+v\n%s", loudness, string(data))
	}
}

func TestWriteMixboardFeatureRequestSnapshotPreservesCurrentL3HistoryRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summary":{"status":"missing","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_waveform_envelope_1016","reason":"awaiting_current_target_band_summary"},
		"band_energy_summaries":[
			{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1007","clip_id":"1014","source_path":"D:/Vit_DAW/test_100hz_10s.wav","source_revision":"rev_old","duration_seconds":10,"bands":{"bass":{"energy_db":-18}}},
			{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","source_path":"D:/Vit_DAW/test_target_3s.wav","source_revision":"rev_current","duration_seconds":3,"bands":{"bass":{"energy_db":-12}}}
		],
		"stereo_relation_summaries":[
			{"status":"ready","source":"kernel_l3_offline_analyzer","track_id":"1010","clip_id":"1016","source_path":"D:/Vit_DAW/test_target_3s.wav","source_revision":"rev_current","duration_seconds":3,"correlation_state":"decorrelated"}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_spectral_field_1016",
		"status":     "requested",
		"resolved_target": map[string]any{
			"track_id":         "1010",
			"clip_id":          "1016",
			"source_path":      "D:/Vit_DAW/test_target_3s.wav",
			"duration_seconds": 3,
		},
		"requested_features": []any{
			map[string]any{"feature_type": "waveform_envelope", "track_id": "1010", "clip_id": "1016"},
			map[string]any{"feature_type": "spectral_field", "track_id": "1010", "clip_id": "1016"},
			map[string]any{"feature_type": "l3_acoustic_summary", "track_id": "1010", "clip_id": "1016"},
		},
	}
	writeMixboardFeatureRequestSnapshot(cmd, packet)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "status") != "ready" || firstString(band, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(band, "track_id") != "1010" || firstString(band, "source_revision") != "rev_current" {
		t.Fatalf("current L3 band row was not preserved/promoted: %+v\n%s", band, string(data))
	}
	bands := mapRowsFromAny(snapshot["band_energy_summaries"])
	if len(bands) != 1 || firstString(bands[0], "track_id") != "1010" || firstString(bands[0], "clip_id") != "1016" {
		t.Fatalf("band history should keep only current target row: %+v\n%s", bands, string(data))
	}
	stereo := testMap(t, snapshot["stereo_relation_summary"])
	if firstString(stereo, "status") != "ready" || firstString(stereo, "track_id") != "1010" || firstString(stereo, "source_revision") != "rev_current" {
		t.Fatalf("current L3 stereo row was not preserved/promoted: %+v\n%s", stereo, string(data))
	}
}

func TestWriteMixboardFeatureRequestSnapshotPreservesFullProjectL3ContextRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summaries":[
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1007","clip_id":"1014","source_path":"C:/fixtures/current/Bass.wav","source_revision":"rev_bass","duration_seconds":20,"bands":{"bass":{"energy_db":-12}}},
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1010","clip_id":"1016","source_path":"C:/fixtures/current/Drums.wav","source_revision":"rev_drums","duration_seconds":20,"bands":{"bass":{"energy_db":-15}}},
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_old","track_id":"1010","clip_id":"1016","source_path":"C:/fixtures/old/Drums.wav","source_revision":"rev_old_drums","duration_seconds":20,"bands":{"bass":{"energy_db":-2}}}
		],
		"stereo_relation_summaries":[
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1007","clip_id":"1014","source_path":"C:/fixtures/current/Bass.wav","source_revision":"rev_bass","duration_seconds":20,"correlation_state":"stable"},
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1010","clip_id":"1016","source_path":"C:/fixtures/current/Drums.wav","source_revision":"rev_drums","duration_seconds":20,"correlation_state":"wide"}
		],
		"loudness_summaries":[
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1007","clip_id":"1014","source_path":"C:/fixtures/current/Bass.wav","source_revision":"rev_bass","duration_seconds":20,"approximate_lufs":-18},
			{"status":"ready","source":"kernel_l3_offline_analyzer","project_id":"vitproj_current","track_id":"1010","clip_id":"1016","source_path":"C:/fixtures/current/Drums.wav","source_revision":"rev_drums","duration_seconds":20,"approximate_lufs":-16}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "kernel_prepared_full_project",
		"project_id": "vitproj_current",
		"scope":      "full_project_with_focus_track",
		"resolved_target": map[string]any{
			"track_id": "1007", "clip_id": "1014", "source_path": "C:/fixtures/current/Bass.wav", "source_revision": "rev_bass", "duration_seconds": 20,
		},
		"track_feature_targets": []any{
			map[string]any{"track_id": "1007", "clip_id": "1014", "source_path": "C:/fixtures/current/Bass.wav", "source_revision": "rev_bass", "duration_seconds": 20},
			map[string]any{"track_id": "1010", "clip_id": "1016", "source_path": "C:/fixtures/current/Drums.wav", "source_revision": "rev_drums", "duration_seconds": 20},
		},
	}
	writeMixboardFeatureRequestSnapshot(cmd, packet)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	for _, key := range []string{"band_energy_summaries", "stereo_relation_summaries", "loudness_summaries"} {
		rows := mapRowsFromAny(snapshot[key])
		if len(rows) != 2 {
			t.Fatalf("%s should retain both requested project tracks and reject stale rows: %+v\n%s", key, rows, string(data))
		}
		if firstString(rows[0], "source_revision") != "rev_bass" || firstString(rows[1], "source_revision") != "rev_drums" {
			t.Fatalf("%s retained wrong project context rows: %+v\n%s", key, rows, string(data))
		}
	}
	primary := testMap(t, snapshot["band_energy_summary"])
	if firstString(primary, "track_id") != "1007" || firstString(primary, "source_revision") != "rev_bass" {
		t.Fatalf("primary band summary must remain the resolved focus target: %+v\n%s", primary, string(data))
	}
}

func TestReadMixboardFeatureSnapshotForAcousticPackageRelabelsAuthoritativeBridgeRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{
			"schema_version":"mixboard_feature_request.v1",
			"request_id":"kernel_prepared_spectral_field_1016",
			"status":"materialized",
			"resolved_target":{
				"track_id":"1010",
				"clip_id":"1016",
				"source_path":"D:/Vit_DAW/test_target_3s.wav",
				"duration_seconds":3
			},
			"requested_features":[
				{"feature_type":"waveform_envelope","request_id":"kernel_prepared_spectral_field_1016","track_id":"1010","clip_id":"1016"},
				{"feature_type":"spectral_field","request_id":"kernel_prepared_spectral_field_1016","track_id":"1010","clip_id":"1016"},
				{"feature_type":"l3_acoustic_summary","request_id":"kernel_prepared_spectral_field_1016","track_id":"1010","clip_id":"1016"}
			]
		},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_spectral_field_1016","source_path":"D:/Vit_DAW/test_target_3s.wav","duration_seconds":3},
		"band_energy_summary":{"status":"missing","feature_type":"band_energy_summary","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_waveform_envelope_1016","reason":"stale_feature_snapshot_for_current_request"},
		"stereo_relation_summary":{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1010","clip_id":"1016","request_id":"kernel_prepared_spectral_field_1016","source_revision":"rev_current","correlation_state":"decorrelated"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readMixboardFeatureSnapshotForAcousticPackage(snapshotPath)
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "request_id") != "kernel_prepared_spectral_field_1016" || firstString(band, "track_id") != "1010" || firstString(band, "clip_id") != "1016" {
		t.Fatalf("band row was not relabelled in memory: %+v", band)
	}

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	persistedBand := testMap(t, persisted["band_energy_summary"])
	if firstString(persistedBand, "request_id") != "kernel_prepared_spectral_field_1016" {
		t.Fatalf("band row was not relabelled on disk: %+v\n%s", persistedBand, string(data))
	}
}

func TestReadMixboardFeatureSnapshotForAcousticPackageBindsCurrentTargetInsteadOfOldLatestRequest(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{"request_id":"old_vocals_request","resolved_target":{"track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30}},
		"waveform_envelope":{"status":"ready","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"rms":0.08,"peak_abs":0.46},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"1007","clip_id":"1011","source_path":"D:/stems/bass.wav","source_revision":"rev_bass","duration_seconds":30,"rms":0.21,"peak_abs":0.71},
			{"status":"ready","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"rms":0.08,"peak_abs":0.46}
		],
		"band_energy_summary":{"status":"ready","feature_type":"band_energy_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"bands":{"bass":{"energy_db":-41}}},
		"band_energy_summaries":[
			{"status":"ready","feature_type":"band_energy_summary","track_id":"1007","clip_id":"1011","source_path":"D:/stems/bass.wav","source_revision":"rev_bass","duration_seconds":30,"bands":{"bass":{"energy_db":-12}}},
			{"status":"ready","feature_type":"band_energy_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"bands":{"bass":{"energy_db":-41}}}
		],
		"stereo_relation_summary":{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"correlation_estimate":0.62},
		"stereo_relation_summaries":[
			{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1007","clip_id":"1011","source_path":"D:/stems/bass.wav","source_revision":"rev_bass","duration_seconds":30,"correlation_estimate":0.98},
			{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"correlation_estimate":0.62}
		],
		"loudness_summary":{"status":"ready","feature_type":"loudness_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"approximate_lufs":-22},
		"loudness_summaries":[
			{"status":"ready","feature_type":"loudness_summary","track_id":"1007","clip_id":"1011","source_path":"D:/stems/bass.wav","source_revision":"rev_bass","duration_seconds":30,"approximate_lufs":-14},
			{"status":"ready","feature_type":"loudness_summary","track_id":"1032","clip_id":"1036","source_path":"D:/stems/vocals.wav","source_revision":"rev_vocals","duration_seconds":30,"approximate_lufs":-22}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	assertTarget := func(target map[string]any, wantTrack, wantClip, wantPath, wantRevision string, wantRMS float64) {
		t.Helper()
		snapshot := readMixboardFeatureSnapshotForAcousticPackage(snapshotPath, target)
		status := acousticpackage.BuildStatus(acousticpackage.Identity{
			ProjectID: "current", TrackID: wantTrack, ClipID: wantClip,
			SourcePath: wantPath, SourceRevision: wantRevision, SourceFingerprint: wantRevision, DurationSec: 30,
		}, snapshot, "2026-08-12T00:00:00Z", "test")
		if status.TrackID != wantTrack || status.ClipID != wantClip || status.SourcePath != wantPath || status.SourceRevision != wantRevision {
			t.Fatalf("status identity = track=%q clip=%q path=%q revision=%q, want %q/%q/%q/%q", status.TrackID, status.ClipID, status.SourcePath, status.SourceRevision, wantTrack, wantClip, wantPath, wantRevision)
		}
		waveform := status.PackageLayers["l1_static"].Features["waveform_envelope"]
		if got := numberFromAny(waveform.Ref["rms"]); math.Abs(got-wantRMS) > 0.0001 {
			t.Fatalf("waveform RMS = %v, want %v; ref=%+v", got, wantRMS, waveform.Ref)
		}
	}

	assertTarget(map[string]any{"track_id": "1007", "clip_id": "1011", "source_path": "D:/stems/bass.wav", "source_revision": "rev_bass", "duration_seconds": 30}, "1007", "1011", "D:/stems/bass.wav", "rev_bass", 0.21)
	assertTarget(map[string]any{"track_id": "1032", "clip_id": "1036", "source_path": "D:/stems/vocals.wav", "source_revision": "rev_vocals", "duration_seconds": 30}, "1032", "1036", "D:/stems/vocals.wav", "rev_vocals", 0.08)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	latest := testMap(t, persisted["latest_request"])
	if got := firstString(testMap(t, latest["resolved_target"]), "track_id"); got != "1032" {
		t.Fatalf("read binding rewrote persisted latest_request to %q", got)
	}
}

func testSpectralCollectorWithTile(t *testing.T, frameWidth, frequencyBins, expectedTiles int, fill func(bin, frame int) (float32, float32, float32, float32)) (*spectralFeatureCollector, map[string][]float32) {
	t.Helper()
	memoryName := fmt.Sprintf("test_spectral_tile_%d_%d_%d", frameWidth, frequencyBins, expectedTiles)
	data := make([]float32, frameWidth*frequencyBins*spectralTileChannelStride)
	for bin := 0; bin < frequencyBins; bin++ {
		for frame := 0; frame < frameWidth; frame++ {
			left, right, phase, weight := fill(bin, frame)
			base := (bin*frameWidth + frame) * spectralTileChannelStride
			data[base] = left
			data[base+1] = right
			data[base+2] = phase
			data[base+3] = weight
		}
	}
	collector := &spectralFeatureCollector{TrackID: "1007", ClipID: "1011", FeatureType: "spectral_field", LastTileIndex: -1}
	collector.AddEvent(map[string]any{
		"command":                    "tile_ready",
		"feature_type":               "spectral_field",
		"track_id":                   "1007",
		"clip_id":                    "1011",
		"tile_index":                 0,
		"tile_count":                 expectedTiles,
		"tile_duration":              5.0,
		"tile_content_start_seconds": 0.0,
		"total_duration":             float64(expectedTiles) * 5.0,
		"resolution_frame_width":     frameWidth,
		"resolution_frequency_bins":  frequencyBins,
		"frame_duration_seconds":     0.01,
		"shared_memory":              memoryName,
		"float_count":                len(data),
		"shm_bytes":                  len(data) * 4,
		"channels_semantics":         spectralTileChannelsSemantics,
	})
	return collector, map[string][]float32{memoryName: data}
}

func TestWriteMixboardProjectSpectralSnapshotFromCollectorsAggregatesDirectCollectors(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	packet := map[string]any{
		"request_id": "req_project_spectral",
		"resolved_target": map[string]any{
			"kind":     "project",
			"id":       "current",
			"track_id": "",
			"clip_id":  "",
		},
		"requested_features": []any{
			map[string]any{"feature_type": "spectral_field", "track_id": "1007", "clip_id": "1011"},
			map[string]any{"feature_type": "spectral_field", "track_id": "1012", "clip_id": "2012"},
		},
		"spectral_tile_ready_seen": true,
		"spectral_direct_collectors": []any{
			map[string]any{
				"feature_type":              "spectral_field",
				"track_id":                  "1007",
				"clip_id":                   "1011",
				"tile_count":                2,
				"tile_count_expected":       2,
				"tile_duration":             5.0,
				"resolution_frame_width":    256,
				"resolution_frequency_bins": 128,
				"first_received_at":         "2026-06-21T10:00:00Z",
				"last_received_at":          "2026-06-21T10:00:01Z",
			},
			map[string]any{
				"feature_type":              "spectral_field",
				"track_id":                  "1012",
				"clip_id":                   "2012",
				"tile_count":                3,
				"tile_count_expected":       5,
				"resolution_frame_width":    512,
				"resolution_frequency_bins": 256,
				"last_received_at":          "2026-06-21T10:00:02Z",
			},
		},
	}
	targets := []map[string]any{
		{"track_id": "1007", "clip_id": "1011"},
		{"track_id": "1012", "clip_id": "2012"},
	}
	writeMixboardProjectSpectralSnapshotFromCollectors(cmd, packet, targets)
	finalizeMixboardFeatureSnapshotAfterWait(cmd, packet, targets)

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	spectral := testMap(t, snapshot["spectrogram_tiles"])
	if firstString(spectral, "status") != "partial" || firstString(spectral, "feature_type") != "spectral_field" || firstString(spectral, "request_id") != "req_project_spectral" {
		t.Fatalf("unexpected project spectral identity: %+v\n%s", spectral, string(data))
	}
	if firstString(spectral, "source") != "kernel_tile_ready_direct_collector" || firstString(spectral, "scope") != "full_project" {
		t.Fatalf("unexpected project spectral source/scope: %+v", spectral)
	}
	if int(numberFromAny(spectral["target_count"])) != 2 || int(numberFromAny(spectral["tile_count_seen"])) != 5 || int(numberFromAny(spectral["tile_count_expected"])) != 7 {
		t.Fatalf("unexpected aggregate counts: %+v", spectral)
	}
	if int(numberFromAny(spectral["resolution_frame_width"])) != 256 || int(numberFromAny(spectral["resolution_frequency_bins"])) != 128 || numberFromAny(spectral["tile_duration"]) != 5 {
		t.Fatalf("unexpected aggregate metadata: %+v", spectral)
	}
	trackIDs := anyListFromAny(spectral["track_ids"])
	if len(trackIDs) != 2 || fmt.Sprint(trackIDs[0]) != "1007" || fmt.Sprint(trackIDs[1]) != "1012" {
		t.Fatalf("unexpected track_ids: %+v", spectral)
	}
}

func TestProjectSpectralSnapshotDoesNotReplaceL3RowsWithSpectralDerivedSummaries(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"band_energy_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","request_id":"req_project","track_id":"1007","clip_id":"1011","source_revision":"rev_l3","bands":{"bass":{"unit_energy":0.5}}},
		"stereo_relation_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","request_id":"req_project","track_id":"1007","clip_id":"1011","source_revision":"rev_l3","correlation_estimate":0.9},
		"loudness_summary":{"status":"ready","source":"kernel_l3_offline_analyzer","request_id":"req_project","track_id":"1007","clip_id":"1011","source_revision":"rev_l3","integrated_lufs":-18.2}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"request_id": "req_project",
		"resolved_target": map[string]any{
			"kind": "project",
			"id":   "current",
		},
		"spectral_tile_ready_seen": true,
		"spectral_direct_collectors": []any{
			map[string]any{"feature_type": "spectral_field", "track_id": "1007", "clip_id": "1011", "tile_count": 2, "tile_count_expected": 2, "resolution_frame_width": 32, "resolution_frequency_bins": 32},
			map[string]any{"feature_type": "spectral_field", "track_id": "1012", "clip_id": "2012", "tile_count": 1, "tile_count_expected": 3, "resolution_frame_width": 32, "resolution_frequency_bins": 32},
		},
	}
	targets := []map[string]any{{"track_id": "1007", "clip_id": "1011"}, {"track_id": "1012", "clip_id": "2012"}}
	writeMixboardProjectSpectralSnapshotFromCollectors(cmd, packet, targets)
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	band := testMap(t, snapshot["band_energy_summary"])
	if firstString(band, "status") != "ready" || firstString(band, "source") != "kernel_l3_offline_analyzer" || firstString(band, "source_revision") != "rev_l3" {
		t.Fatalf("project spectral write should not replace L3 band row: %+v\n%s", band, string(data))
	}
	stereo := testMap(t, snapshot["stereo_relation_summary"])
	if firstString(stereo, "status") != "ready" || firstString(stereo, "source") != "kernel_l3_offline_analyzer" || firstString(stereo, "source_revision") != "rev_l3" {
		t.Fatalf("project spectral write should not replace L3 stereo row: %+v\n%s", stereo, string(data))
	}
	loudness := testMap(t, snapshot["loudness_summary"])
	if firstString(loudness, "status") != "ready" || firstString(loudness, "source") != "kernel_l3_offline_analyzer" || firstString(loudness, "source_revision") != "rev_l3" {
		t.Fatalf("project spectral write should preserve L3 loudness row: %+v\n%s", loudness, string(data))
	}
}

func TestMixRequestObservationTreatsWaveformReadyAsSufficient(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	sourceRevision := testAcousticSourceRevision("mix_waveform_ready_sufficient", "1007", "clip_a", 2)
	if err := os.WriteFile(snapshotPath, []byte(fmt.Sprintf(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"1007","clip_id":"clip_a","source_revision":%q,"duration_seconds":2,"rms":0.2,"peak_abs":0.7,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.7,"energy_state":"high"}]},
		"band_energy_summary":{"status":"missing"},
		"stereo_relation_summary":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"}
	}`, sourceRevision)), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{}
	h := New(nil, shadowProjectWithClips(), nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "mix.request_observation",
		Args: map[string]any{
			"mix_session_id": "mix_waveform_ready_sufficient",
			"target_ref": map[string]any{
				"kind": "track",
				"id":   "1007",
			},
		},
		Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(resp.Result, "status") != "ready" {
		t.Fatalf("result = %+v", resp.Result)
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	waveform, _ := snapshot["waveform_envelope"].(map[string]any)
	if firstString(waveform, "status") != "ready" {
		t.Fatalf("waveform should remain ready: %+v\n%s", waveform, string(data))
	}
	obs, _ := resp.Result["observation"].(mixboard.ObservationPacket)
	if got := obs.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q observation=%+v", got, obs)
	}
	if len(resp.Result["mixboard"].(mixboard.Board).OpenBlockers) != 0 {
		t.Fatalf("board blockers = %+v", resp.Result["mixboard"].(mixboard.Board).OpenBlockers)
	}
}

func TestKernelTelemetryMaterializerWritesReadModelSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	h := New(nil, nil, nil)

	h.IngestKernelTelemetry(map[string]any{
		"command":                    "tile_ready",
		"feature_type":               "spectral_field",
		"track_id":                   "track_1",
		"clip_id":                    "clip_1",
		"file_path":                  filepath.Join(root, "tone.wav"),
		"tile_index":                 0,
		"tile_count":                 2,
		"tile_duration":              5.0,
		"tile_content_start_seconds": 0.0,
		"total_duration":             10.0,
		"resolution_frame_width":     4,
		"resolution_frequency_bins":  8,
		"float_count":                128,
		"shared_memory":              "vit_test_missing_shm",
	})

	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("parse snapshot: %v\n%s", err, string(data))
	}
	spectral := mapAnyFromAny(snapshot["spectrogram_tiles"])
	if firstString(spectral, "status") != "partial" {
		t.Fatalf("spectral telemetry was not materialized as partial: %+v", spectral)
	}
	if firstString(spectral, "source") != "kernel_tile_ready_direct_collector" {
		t.Fatalf("spectral source should describe direct tile evidence: %+v", spectral)
	}
	if firstString(spectral, "materialized_by") != "kernel_prepared_telemetry" {
		t.Fatalf("spectral row should preserve passive materializer provenance: %+v", spectral)
	}
	if firstString(spectral, "source_revision") == "" || firstString(spectral, "clip_revision") == "" {
		t.Fatalf("materialized spectral row missing stable identity revisions: %+v", spectral)
	}
	latest := mapAnyFromAny(snapshot["latest_request"])
	if firstString(latest, "lifecycle") != "kernel_prepared_materializer" {
		t.Fatalf("latest request should be a materializer marker, got %+v", latest)
	}
	requested := mapRowsFromAny(latest["requested_features"])
	if len(requested) != 3 {
		t.Fatalf("materializer requested_features missing: %+v", latest)
	}
	seen := map[string]bool{}
	for _, row := range requested {
		seen[firstString(row, "feature_type")] = true
		if firstString(row, "track_id") != "track_1" || firstString(row, "clip_id") != "clip_1" {
			t.Fatalf("requested feature target mismatch: %+v", row)
		}
	}
	if !seen["waveform_envelope"] || !seen["spectral_field"] || !seen["l3_acoustic_summary"] {
		t.Fatalf("materializer requested features missing waveform/spectral/l3: %+v", requested)
	}
}

func TestVisiblePluginRefsIncludesRackNodes(t *testing.T) {
	refs := visiblePluginRefs(map[string]any{
		"tracks": []any{map[string]any{
			"track_id":       "1007",
			"track_name":     "Bass",
			"is_audio_track": true,
			"plugins": []any{map[string]any{
				"plugin_id": "1008",
				"name":      "Volume & Pan",
			}},
			"rack": map[string]any{
				"rack_item_id": "1012",
				"nodes": []any{map[string]any{
					"plugin_item_id": "1013",
					"node_id":        "1013",
					"name":           "TDR Nova",
				}},
			},
		}},
	}, "1007")

	if len(refs) != 2 {
		t.Fatalf("visible plugin refs = %+v, want track plugin plus rack node", refs)
	}
	if refs[1].ID != "1013" || refs[1].Name != "TDR Nova" || refs[1].TrackID != "1007" {
		t.Fatalf("rack node ref = %+v", refs[1])
	}
}

func TestInvokeManualConfirmationAndExplicitFullProjectAccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "authority.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	args := map[string]any{"path": path, "old_text": "before", "new_text": "after", "workspace_roots": []any{root}}
	manual, err := h.Invoke(context.Background(), InvokeRequest{Tool: "workspace.apply_edit", Args: args, Source: "test", Context: map[string]any{"authority_mode": "manual_confirmation", "authority_mode_explicit": true}})
	if err != nil || manual.Status != "needs_confirmation" || !manual.RequiresConfirmation {
		t.Fatalf("manual=%+v err=%v", manual, err)
	}
	full, err := h.Invoke(context.Background(), InvokeRequest{Tool: "workspace.apply_edit", Args: args, Source: "test", Context: map[string]any{"authority_mode": "full_project_access", "authority_mode_explicit": true}})
	if err != nil || full.Status != "ok" || full.RequiresConfirmation {
		t.Fatalf("full=%+v err=%v", full, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "after" {
		t.Fatalf("full access did not execute reversible edit: %q", data)
	}
	if _, err := h.Invoke(context.Background(), InvokeRequest{Tool: "agent.rollback_action", Args: map[string]any{"target_action_id": full.AgentActionID}, Confirmed: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "before" {
		t.Fatalf("rollback failed: %q", data)
	}
}

func TestInvokeCheckoutGuardCoversAllActiveProjectPlaneCheckouts(t *testing.T) {
	h := New(nil, nil, nil)
	goal := h.BeginGoal("running experiment")
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"version.checkout", map[string]any{"commit_id": "commit-1"}},
		{"version.node_checkout", map[string]any{"node_id": "node-1"}},
		{"version.worktree_checkout", map[string]any{"name": "worktree-1"}},
	}
	for _, tc := range cases {
		resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: tc.tool, Args: tc.args, Confirmed: true, GoalID: goal.GoalID, RunID: goal.RunID, Source: "test"})
		if err == nil || resp.Status != "error" || resp.Result["error_code"] != "checkout_blocked_while_agent_running" {
			t.Fatalf("tool=%s resp=%+v err=%v", tc.tool, resp, err)
		}
	}
}

func TestInvokeCheckoutGuardRejectsRunningGoalAndAllowsStoppedGoal(t *testing.T) {
	h := New(nil, nil, nil)
	goal := h.BeginGoal("running experiment")
	blocked, err := h.Invoke(context.Background(), InvokeRequest{Tool: "version.checkout", Args: map[string]any{"commit_id": "commit-1"}, Confirmed: true, GoalID: goal.GoalID, RunID: goal.RunID, Source: "test"})
	if err == nil || blocked.Status != "error" || blocked.Result["error_code"] != "checkout_blocked_while_agent_running" {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}
	h.MarkGoalStopped(goal.GoalID, "checkpoint-stable")
	allowed, err := h.Invoke(context.Background(), InvokeRequest{Tool: "version.checkout", Args: map[string]any{"commit_id": "commit-1"}, Confirmed: true, GoalID: goal.GoalID, RunID: goal.RunID, Source: "test"})
	if err != nil && allowed.Error == "checkout_blocked_while_agent_running" {
		t.Fatalf("stopped goal remained blocked: %+v err=%v", allowed, err)
	}
}

func TestInvokeBranchCreateGuardAllowsOnlyInactiveCreationDuringRunningGoal(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": project, "message": "root"})
	if err != nil {
		t.Fatal(err)
	}
	commitID := checkpoint["commit_id"].(string)
	h := New(nil, nil, nil)
	goal := h.BeginGoal("parallel branch experiment")

	blocked, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.branch_create",
		Args:      map[string]any{"project_path": project, "name": "active-branch", "commit_id": commitID},
		Confirmed: true, GoalID: goal.GoalID, RunID: goal.RunID, Source: "test",
	})
	if err == nil || blocked.Result["error_code"] != "checkout_blocked_while_agent_running" {
		t.Fatalf("active branch creation was not guarded: response=%+v err=%v", blocked, err)
	}

	inactive, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.branch_create",
		Args:      map[string]any{"project_path": project, "name": "inactive-branch", "commit_id": commitID, "activate": false},
		Confirmed: true, GoalID: goal.GoalID, RunID: goal.RunID, Source: "test",
	})
	if err != nil || inactive.Status != "ok" || inactive.Result["activated"] != false {
		t.Fatalf("inactive branch creation failed or activated: response=%+v err=%v", inactive, err)
	}
	status, err := history.Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	if status["active_branch"] != "main" {
		t.Fatalf("inactive branch changed active history: %#v", status)
	}
}

func TestWorktreeReservationCommandsAndWriterGuard(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectUUID := "project-reservation"
	history.BindProjectIdentity(project, projectUUID)
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	created, err := history.WorktreeCreate(map[string]any{"project_path": project, "commit_id": checkpoint["commit_id"], "name": "vocal-natural"})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(created["project_uuid"]) != projectUUID {
		t.Fatalf("worktree project UUID=%q", created["project_uuid"])
	}
	h := New(nil, nil, nil)
	reserveArgs := map[string]any{"project_path": project, "project_uuid": projectUUID, "worktree_ref": "vocal-natural", "owner_agent_id": "agent-child", "owner_goal_id": "goal-child", "owner_run_id": "run-child", "owner_conversation_id": "conversation-child", "purpose": "vocal natural direction"}
	reserved, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.worktree_reserve", Args: reserveArgs, Confirmed: true, Source: "test"})
	if err != nil || reserved.Status != "ok" {
		t.Fatalf("reserved=%+v err=%v", reserved, err)
	}
	reservation := mapFromAny(reserved.Result["reservation"])
	if firstString(reservation, "id") == "" || firstString(reservation, "owner_agent_id") != "agent-child" {
		t.Fatalf("reservation=%+v", reservation)
	}

	conflictArgs := tools.CloneCommand(reserveArgs)
	conflictArgs["owner_agent_id"], conflictArgs["owner_goal_id"], conflictArgs["owner_run_id"] = "agent-other", "goal-other", "run-other"
	conflict, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.worktree_reserve", Args: conflictArgs, Confirmed: true, Source: "test"})
	if err == nil || conflict.Status != "error" {
		t.Fatalf("conflict=%+v err=%v", conflict, err)
	}

	blocked, err := h.Invoke(context.Background(), InvokeRequest{Tool: "track.rename", Args: map[string]any{"track_id": "1", "name": "blocked"}, Source: "test", GoalID: "goal-other", RunID: "run-other", Context: map[string]any{"project_uuid": projectUUID, "worktree_ref": "vocal-natural", "agent_id": "agent-other"}})
	if err == nil || blocked.Result["error_code"] != "worktree_reservation_owner_mismatch" {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}

	allowed, err := h.Invoke(context.Background(), InvokeRequest{Tool: "track.rename", Args: map[string]any{"track_id": "1", "name": "allowed"}, Source: "test", GoalID: "goal-child", RunID: "run-child", Context: map[string]any{"project_uuid": projectUUID, "worktree_ref": "vocal-natural", "agent_id": "agent-child"}})
	if err != nil && allowed.Result["error_code"] == "worktree_reservation_owner_mismatch" {
		t.Fatalf("owner was rejected: %+v err=%v", allowed, err)
	}

	released, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.worktree_release", Args: map[string]any{"reservation_id": firstString(reservation, "id"), "owner_agent_id": "agent-child", "owner_goal_id": "goal-child", "owner_run_id": "run-child"}, Confirmed: true, Source: "test"})
	if err != nil || firstString(mapFromAny(released.Result["reservation"]), "status") != "released" || !boolValueDefault(released.Result["worktree_retained"], false) {
		t.Fatalf("released=%+v err=%v", released, err)
	}
}

func TestCollaborationCandidatePrepareRequiresRealTargetAudioAndPreservesWorktreeProvenance(t *testing.T) {
	root := t.TempDir()
	audioPath := filepath.Join(root, "candidate.wav")
	if err := os.WriteFile(audioPath, []byte("RIFF-real-candidate-audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	if err := h.RestoreCollaboration(collaboration.Snapshot{SchemaVersion: collaboration.SchemaVersion, Reservations: []collaboration.WorktreeReservation{{SchemaVersion: collaboration.SchemaVersion, ID: "reservation-vocal", ProjectUUID: "project-1", WorktreeRef: "vocal-natural", WorktreePath: filepath.Join(root, "Vocal.vit"), OwnerAgentID: "agent-vocal", OwnerGoalID: "goal-vocal", OwnerRunID: "run-vocal", OwnerConversationID: "conversation-vocal", Status: collaboration.ReservationActive, CreatedAt: time.Now().UTC()}}}); err != nil {
		t.Fatal(err)
	}
	prepared, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.candidate_prepare", Args: map[string]any{
		"candidate_id": "candidate-b", "source_kind": "worktree", "source_ref": "worktree:vocal-natural", "audio_file": audioPath,
		"worktree_ref": "vocal-natural", "reservation_id": "reservation-vocal", "commit_id": "commit-worktree", "project_path": filepath.Join(root, "Vocal.vit"), "project_uuid": "project-1", "project_revision": "revision-7", "scope": "target",
	}, Source: "test"})
	if err != nil || prepared.Status != "ok" {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	candidate := mapFromAny(prepared.Result["candidate"])
	if firstString(candidate, "source_kind") != "audio_file" || firstString(candidate, "engineering_source_kind") != "worktree" || firstString(candidate, "worktree_ref") != "vocal-natural" || firstString(candidate, "source_ref") != audioPath || firstString(candidate, "render_revision") == "" || firstString(candidate, "preview_revision") == "" {
		t.Fatalf("candidate=%+v", candidate)
	}
	unsupported, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.candidate_prepare", Args: map[string]any{
		"source_kind": "worktree", "source_ref": "worktree:vocal-natural", "audio_file": audioPath, "project_path": filepath.Join(root, "Vocal.vit"), "project_uuid": "project-1", "scope": "full_project",
	}, Source: "test"})
	if err == nil || unsupported.Status != "error" || !strings.Contains(unsupported.Error, "full_project_preview_unsupported") {
		t.Fatalf("unsupported=%+v err=%v", unsupported, err)
	}
}

func TestCollaborationAuditionPrepareUsesKernelWithoutChangingActivePlane(t *testing.T) {
	fake := &fakeVSPKernelClient{commandReplies: []*kernel.VSPCommandResult{fakeVSPCommandReply("audition.prepare", "", map[string]any{"status": "ok", "session": map[string]any{"session_id": "audition-1", "status": "ready"}})}}
	h := NewWithSender(fake, nil, nil)
	base := map[string]any{"source_kind": "audio_file", "source_ref": `D:\Preview\candidate.wav`, "project_uuid": "project-1", "project_revision": "rev-1", "project_path": `D:\Project\Song.vit`, "scope": "target", "preview_ref": "audio-buffer://candidate"}
	candidateA := tools.CloneCommand(base)
	candidateA["id"] = "candidate-a"
	candidateA["source_ref"] = `D:\Preview\candidate-a.wav`
	candidateB := tools.CloneCommand(base)
	candidateB["id"] = "candidate-b"
	candidateB["source_ref"] = `D:\Preview\candidate-b.wav`
	response, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.audition_prepare", Args: map[string]any{
		"session_id": "audition-1", "conversation_id": "conversation-1", "active_project_ref": `D:\Project\Song.vit`, "active_project_revision": "rev-active", "timeline_revision": "timeline-1", "scope": "target", "candidate_a": candidateA, "candidate_b": candidateB,
	}, Source: "test"})
	if err != nil || response.Status != "ok" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	if len(fake.vspCommands) != 1 || fake.vspCommands[0] != "audition.prepare" {
		t.Fatalf("vsp commands=%v", fake.vspCommands)
	}
	if response.Result["active_project_plane_unchanged"] != true {
		t.Fatalf("response=%+v", response)
	}
}

func TestParentChildWorktreeIsolationAndFailureLifecycle(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("parent"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectUUID := "project-parent-child"
	history.BindProjectIdentity(project, projectUUID)
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	commitID := checkpoint["commit_id"].(string)
	vocal, err := history.WorktreeCreate(map[string]any{"project_path": project, "commit_id": commitID, "name": "vocal"})
	if err != nil {
		t.Fatal(err)
	}
	drums, err := history.WorktreeCreate(map[string]any{"project_path": project, "commit_id": commitID, "name": "drums"})
	if err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	reserve := func(worktree map[string]any, agent, goal, run string) map[string]any {
		resp, reserveErr := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.worktree_reserve", Args: map[string]any{"project_path": project, "project_uuid": projectUUID, "worktree_ref": worktree["name"], "owner_agent_id": "agent-parent", "owner_goal_id": "goal-parent", "owner_run_id": "run-parent", "owner_conversation_id": "conversation-parent"}, Confirmed: true, Source: "test"})
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		reservation := mapFromAny(resp.Result["reservation"])
		child, childErr := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.child_register", Args: map[string]any{"reservation_id": reservation["id"], "parent_agent_id": "agent-parent", "parent_goal_id": "goal-parent", "parent_run_id": "run-parent", "child_agent_id": agent, "child_goal_id": goal, "child_run_id": run, "child_conversation_id": "conversation-" + agent, "worktree_ref": worktree["name"], "allowed_scope": []any{"project"}}, Context: map[string]any{"agent_id": "agent-parent", "goal_id": "goal-parent", "run_id": "run-parent"}, Confirmed: true, Source: "test"})
		if childErr != nil {
			t.Fatal(childErr)
		}
		return mapFromAny(child.Result["child_task"])
	}
	vocalTask := reserve(vocal, "agent-vocal", "goal-vocal", "run-vocal")
	drumTask := reserve(drums, "agent-drums", "goal-drums", "run-drums")
	if child, ok := h.collaboration.ChildTaskForOwner("agent-vocal", "goal-vocal", "run-vocal"); !ok {
		t.Fatalf("registered vocal task not owned by child: task=%+v snapshot=%+v", child, h.collaboration.Snapshot())
	}
	childCheckout, checkoutErr := h.Invoke(context.Background(), InvokeRequest{Tool: "version.checkout", Args: map[string]any{"project_path": project, "commit_id": commitID}, Confirmed: true, GoalID: "goal-vocal", RunID: "run-vocal", Context: map[string]any{"agent_id": "agent-vocal"}, Source: "test"})
	if checkoutErr == nil || childCheckout.Result["error_code"] != "child_active_project_plane_forbidden" {
		t.Fatalf("child checkout=%+v err=%v", childCheckout, checkoutErr)
	}
	parentWrite, parentWriteErr := h.Invoke(context.Background(), InvokeRequest{Tool: "track.rename", Args: map[string]any{"project_path": project, "track_id": "1", "name": "parent-pollution"}, GoalID: "goal-vocal", RunID: "run-vocal", Context: map[string]any{"agent_id": "agent-vocal", "project_uuid": projectUUID, "worktree_ref": "vocal"}, Source: "test"})
	if parentWriteErr == nil || parentWrite.Result["error_code"] != "worktree_write_target_mismatch" {
		t.Fatalf("child parent write=%+v err=%v", parentWrite, parentWriteErr)
	}

	vocalPath, drumsPath := fmt.Sprint(vocal["project_file_path"]), fmt.Sprint(drums["project_file_path"])
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if writeErr := os.WriteFile(vocalPath, []byte("vocal-child"), 0o644); writeErr != nil {
			t.Error(writeErr)
		}
	}()
	go func() {
		defer wg.Done()
		if writeErr := os.WriteFile(drumsPath, []byte("drum-child"), 0o644); writeErr != nil {
			t.Error(writeErr)
		}
	}()
	wg.Wait()
	parentBytes, _ := os.ReadFile(project)
	vocalBytes, _ := os.ReadFile(vocalPath)
	drumBytes, _ := os.ReadFile(drumsPath)
	if string(parentBytes) != "parent" || string(vocalBytes) != "vocal-child" || string(drumBytes) != "drum-child" {
		t.Fatalf("parent=%q vocal=%q drums=%q", parentBytes, vocalBytes, drumBytes)
	}

	failed, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.child_update", Args: map[string]any{"child_task_id": vocalTask["id"], "status": "failed", "error": "child execution failed"}, Source: "test"})
	if err != nil || firstString(mapFromAny(failed.Result["reservation"]), "status") != "failed" || failed.Result["parent_project_unchanged"] != true {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	completed, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.child_update", Args: map[string]any{"child_task_id": drumTask["id"], "status": "completed", "result_candidate_ref": "candidate-drums", "artifact_ref": "audio-sha256:drums", "settlement_ref": "settlement-drums"}, Source: "test"})
	if err != nil || firstString(mapFromAny(completed.Result["reservation"]), "status") != "released" {
		t.Fatalf("completed=%+v err=%v", completed, err)
	}
}

func TestFullAccessAutonomousInactiveBranchAndReservedWorktree(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectUUID := "project-full-access-collaboration"
	history.BindProjectIdentity(project, projectUUID)
	checkpoint, err := history.Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	commitID := checkpoint["commit_id"].(string)
	h := New(nil, nil, nil)
	goal := h.BeginGoal("autonomous alternatives")
	fullContext := map[string]any{"authority_mode": "full_project_access", "authority_mode_explicit": true, "conversation_id": "conversation-full", "agent_id": "agent-primary"}
	branch, err := h.Invoke(context.Background(), InvokeRequest{Tool: "version.branch_create", Args: map[string]any{"project_path": project, "name": "natural", "commit_id": commitID, "activate": false, "owner_agent_id": "agent-primary", "conversation_id": "conversation-full", "purpose": "natural direction"}, Context: fullContext, GoalID: goal.GoalID, RunID: goal.RunID, Source: "agent"})
	if err != nil || branch.Status != "ok" || branch.RequiresConfirmation || branch.Result["activated"] != false {
		t.Fatalf("branch=%+v err=%v", branch, err)
	}
	manual, err := h.Invoke(context.Background(), InvokeRequest{Tool: "version.worktree_create", Args: map[string]any{"project_path": project, "name": "manual-worktree", "commit_id": commitID}, Context: map[string]any{"authority_mode": "manual_confirmation", "authority_mode_explicit": true}, Source: "agent"})
	if err != nil || manual.Status != "needs_confirmation" {
		t.Fatalf("manual=%+v err=%v", manual, err)
	}
	worktree, err := h.Invoke(context.Background(), InvokeRequest{Tool: "version.worktree_create", Args: map[string]any{"project_path": project, "project_uuid": projectUUID, "name": "aggressive", "commit_id": commitID, "reserve": true, "owner_agent_id": "agent-primary", "owner_goal_id": goal.GoalID, "owner_run_id": goal.RunID, "owner_conversation_id": "conversation-full", "purpose": "aggressive direction", "hypothesis": "tighter low end"}, Context: fullContext, GoalID: goal.GoalID, RunID: goal.RunID, Source: "agent"})
	if err != nil || worktree.Status != "ok" || worktree.RequiresConfirmation || worktree.Result["reservation_status"] != "reserved" {
		t.Fatalf("worktree=%+v err=%v", worktree, err)
	}
	if content, _ := os.ReadFile(project); string(content) != "root" {
		t.Fatalf("active parent project changed: %q", content)
	}
	status, _ := history.Status(map[string]any{"project_path": project})
	if status["active_branch"] != "main" || status["active_worktree"] != "" {
		t.Fatalf("active plane changed: %+v", status)
	}
}

func TestExpiredReservationRecoveryIsAvailableInProductionCommands(t *testing.T) {
	h := New(nil, nil, nil)
	expired := collaboration.WorktreeReservation{SchemaVersion: collaboration.SchemaVersion, ID: "reservation-expired", ProjectUUID: "project-expired", WorktreeRef: "expired-worktree", WorktreePath: `D:\Worktrees\expired.vit`, OwnerAgentID: "agent-expired", OwnerGoalID: "goal-expired", OwnerRunID: "run-expired", OwnerConversationID: "conversation-expired", Status: collaboration.ReservationActive, CreatedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(-time.Minute)}
	if err := h.RestoreCollaboration(collaboration.Snapshot{SchemaVersion: collaboration.SchemaVersion, Reservations: []collaboration.WorktreeReservation{expired}}); err != nil {
		t.Fatal(err)
	}
	snapshot := h.CollaborationSnapshot()
	if len(snapshot.Reservations) != 1 || snapshot.Reservations[0].Status != collaboration.ReservationStale {
		t.Fatalf("restore did not recover expired lease: %+v", snapshot)
	}
	result, err := h.Invoke(context.Background(), InvokeRequest{Tool: "collaboration.worktree_recover_stale", Source: "test"})
	if err != nil || result.Status != "ok" || result.Result["worktrees_retained"] != true {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
