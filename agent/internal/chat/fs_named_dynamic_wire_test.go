package chat

import (
	"context"
	"strings"
	"testing"
)

// FS-NAMED-DYNAMIC-WIRE-1 unit faces: the free-state → semantic-plan handover
// (bindFreeStateAuthoritativeTrack) consumes the user-named target
// deterministically — the named track owns the plan target ahead of the
// model-selected TargetRef, the rewrite is disclosed on the reply faces, a
// loop without the pointer keeps the legacy binding byte-identically
// (fail-open), and the named source stays write-gate compatible because
// resolution and the gate read the same kernel-sourced track inventory.
// Fixture track names and ids are synthetic (AGENTS §10 sealed discipline).

func fsNamedWireLoop(conversationID, named, modelTrackID string) freeStateReasoningLoop {
	loop := freeStateReasoningLoop{
		SchemaVersion:   freeStateReasoningLoopSchema,
		LoopID:          "free_state_ndw1_" + conversationID,
		ConversationID:  conversationID,
		Status:          "awaiting_action",
		OriginalIntent:  "帮我把人声处理一下，高频有点刺",
		UserNamedTarget: named,
	}
	if modelTrackID != "" {
		loop.TargetRef = map[string]any{"kind": "track", "id": modelTrackID, "track_id": modelTrackID}
	}
	return loop
}

func TestFSNamedDynamicWireBindRewritesPlanTarget(t *testing.T) {
	server := fsNamedTargetServer()
	server.storeFreeStateLoop(fsNamedWireLoop("conv-ndw1-1", "9001", "9006"))
	out := server.bindFreeStateAuthoritativeTrack("conv-ndw1-1", map[string]any{
		"selected_track_id":        "9006",
		"selected_plugin_track_id": "9006",
		"selected_plugin_id":       "plug-fixture",
	})
	if got := firstStringFromMap(out, "selected_track_id"); got != "9001" {
		t.Fatalf("plan target not rewritten to the named track: %q", got)
	}
	if got := firstStringFromMap(out, "selected_track_name"); got != "Fixture Lead 人声 A" {
		t.Fatalf("named track name not bound: %q", got)
	}
	marker := firstMapFromAny(out[userNamedTargetRewriteContextKey])
	if len(marker) == 0 || firstStringFromMap(marker, "named_track_id") != "9001" ||
		firstStringFromMap(marker, "model_selected_track_id") != "9006" {
		t.Fatalf("rewrite marker missing or malformed: %+v", marker)
	}
	// The stale plugin binding on the superseded model track must clear: the
	// loaded instance no longer sits on the plan target.
	if firstStringFromMap(out, "selected_plugin_track_id") != "" || firstStringFromMap(out, "selected_plugin_id") != "" {
		t.Fatalf("stale plugin binding survived the rewrite: %+v", out)
	}
	// The pointer alone supplies the target when the model bound none — the
	// named pointer is the intent handover, not an override-only guard.
	supply := fsNamedTargetServer()
	supply.storeFreeStateLoop(fsNamedWireLoop("conv-ndw1-1b", "9001", ""))
	supplyOut := supply.bindFreeStateAuthoritativeTrack("conv-ndw1-1b", map[string]any{})
	if firstStringFromMap(supplyOut, "selected_track_id") != "9001" {
		t.Fatalf("named pointer did not supply the plan target: %+v", supplyOut)
	}
	if _, present := supplyOut[userNamedTargetRewriteContextKey]; present {
		t.Fatalf("pointer-supply case fabricated a rewrite marker: %+v", supplyOut)
	}
}

func TestFSNamedDynamicWireBindFailsOpenWithoutPointer(t *testing.T) {
	server := fsNamedTargetServer()
	server.storeFreeStateLoop(fsNamedWireLoop("conv-ndw1-2", "", "9006"))
	out := server.bindFreeStateAuthoritativeTrack("conv-ndw1-2", map[string]any{})
	if firstStringFromMap(out, "selected_track_id") != "9006" {
		t.Fatalf("legacy binding drifted without the pointer: %+v", out)
	}
	if _, present := out[userNamedTargetRewriteContextKey]; present {
		t.Fatalf("rewrite marker invented without the pointer")
	}
	// A model that already bound the named track is an agreement, not a
	// rewrite: no marker, binding unchanged.
	agree := fsNamedTargetServer()
	agree.storeFreeStateLoop(fsNamedWireLoop("conv-ndw1-3", "9001", "9001"))
	agreeOut := agree.bindFreeStateAuthoritativeTrack("conv-ndw1-3", map[string]any{})
	if firstStringFromMap(agreeOut, "selected_track_id") != "9001" {
		t.Fatalf("agreement binding drifted: %+v", agreeOut)
	}
	if _, present := agreeOut[userNamedTargetRewriteContextKey]; present {
		t.Fatalf("agreement case produced a rewrite marker")
	}
	// No active loop: the context returns untouched.
	untouched := fsNamedTargetServer().bindFreeStateAuthoritativeTrack("conv-ndw1-missing", map[string]any{"selected_track_id": "9006"})
	if firstStringFromMap(untouched, "selected_track_id") != "9006" {
		t.Fatalf("no-loop face changed the context: %+v", untouched)
	}
	// An unreadable track name degrades to the bare-ID posture and never
	// carries the superseded model track's label onto the named track.
	bare := &Server{}
	bareOut := bare.bindFreeStateAuthoritativeTrack("conv-ndw1-4", map[string]any{
		"free_state_reasoning_loop": freeStateLoopMap(fsNamedWireLoop("conv-ndw1-4", "9001", "9006")),
	})
	if firstStringFromMap(bareOut, "selected_track_id") != "9001" {
		t.Fatalf("context-carried loop not honored: %+v", bareOut)
	}
	if firstStringFromMap(bareOut, "selected_track_name") != "" {
		t.Fatalf("bare-ID posture leaked a name: %+v", bareOut)
	}
}

func TestFSNamedDynamicWireDisclosureOnResponseFace(t *testing.T) {
	server := fsNamedTargetServer()
	server.storeFreeStateLoop(fsNamedWireLoop("conv-ndw1-5", "9001", "9006"))
	bound := server.bindFreeStateAuthoritativeTrack("conv-ndw1-5", map[string]any{})
	want := "按你的指名，目标轨=Fixture Lead 人声 A/9001。"
	out := server.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conv-ndw1-5", Reply: "类型化语义处理器调整已执行完成，带回读与事务证据。"}, bound)
	if !strings.Contains(out.Reply, want) {
		t.Fatalf("disclosure line missing: %q", out.Reply)
	}
	if got := firstStringFromMap(firstMapFromAny(out.WorkflowData[userNamedTargetRewriteContextKey]), "named_track_id"); got != "9001" {
		t.Fatalf("structured disclosure missing on WorkflowData: %q", got)
	}
	// Double wrap (inner continuation face plus the outer turn face) must not
	// duplicate the machine-owned sentence.
	twice := server.bindFreeStateContextToResponse(out, bound)
	if strings.Count(twice.Reply, want) != 1 {
		t.Fatalf("disclosure duplicated on rewrap: %q", twice.Reply)
	}
	// Without the marker the reply is untouched (fail-open).
	plain := server.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conv-ndw1-5", Reply: "普通回复"}, map[string]any{"free_state_reasoning_loop": freeStateLoopMap(fsNamedWireLoop("conv-ndw1-5", "", "9006"))})
	if strings.Contains(plain.Reply, "按你的指名") {
		t.Fatalf("disclosure invented without a rewrite: %q", plain.Reply)
	}
	// The bare-ID marker discloses the ID alone.
	bareLine := userNamedTargetRewriteDisclosure(map[string]any{"named_track_id": "9001"})
	if bareLine != "按你的指名，目标轨=9001。" {
		t.Fatalf("bare-ID disclosure drifted: %q", bareLine)
	}
}

func TestFSNamedDynamicWireWriteGateAcceptsNamedSource(t *testing.T) {
	server := fsNamedTargetServer()
	// The resolution source and the write-gate disclosure read the same
	// kernel-sourced inventory: a track the resolver can name is by
	// construction inside the observed track set the gate checks, so the
	// 155 write gate can never refuse a named source track.
	if got := server.resolveFreeStateUserNamedTarget("帮我把人声处理一下，高频有点刺"); got != "9001" {
		t.Fatalf("fixture resolution drifted: %q", got)
	}
	plan := &semanticTopologyPlan{
		SchemaVersion: semanticTopologyPlanSchema,
		Mode:          semanticTopologyModeBus,
		FXTrack:       semanticTopologyFXTrackRef{Action: semanticTopologyFXActionCreate, Name: "Fixture Reverb Bus"},
		Sends:         []semanticTopologySendRef{{SourceTrackID: "9001", SourceTrackName: "Fixture Lead 人声 A", DefaultAmountDB: semanticTopologyDefaultSendAmountDB}},
	}
	observed := server.semanticTopologyObservedState(context.Background())
	if err := semanticTopologyWriteGateIssue(plan, observed); err != nil {
		t.Fatalf("write gate refused the named source track: %v", err)
	}
	// Contrast face: a hallucinated source stays refused (155 unchanged).
	plan.Sends[0].SourceTrackID = "999999"
	if err := semanticTopologyWriteGateIssue(plan, observed); err == nil {
		t.Fatalf("hallucinated source passed the write gate")
	}
}
