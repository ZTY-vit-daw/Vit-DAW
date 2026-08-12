package harness

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/tools"
)

func TestCCBObservationRequestIsReadOnlyAndRepeatable(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(t.TempDir(), "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)

	first, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id": "ccb-first", "mix_session_id": "ccb-repeat", "view_ids": []any{"track.basic_energy"},
			"target_kind": "track", "target_id": "1007", "max_disclosure_bytes": 8192,
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.CommandName != "ccb_observation_request" || first.RiskLevel != tools.RiskDirect || first.RequiresConfirmation {
		t.Fatalf("first response metadata = %+v", first)
	}
	firstBundle := testMap(t, first.Result["bundle"])
	observationID := firstString(firstBundle, "observation_id")
	if observationID == "" || firstBundle["read_only"] != true || firstBundle["mutation_authority"] != false {
		t.Fatalf("first bundle = %+v", firstBundle)
	}
	firstViews := testMap(t, firstBundle["views"])
	if firstViews["track.basic_energy"] == nil {
		read, readErr := mixboard.NewStore("").Read(mixboard.ReadRequest{ObservationID: observationID, MixSessionID: "ccb-repeat", Keys: []string{"observation.binding", "track.1007.static.identity", "track.1007.fast.levels"}})
		t.Fatalf("basic energy view was not assembled: bundle=%+v read=%+v err=%v", firstBundle, read, readErr)
	}

	second, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb_observation_request",
		Args: map[string]any{
			"request_id": "ccb-second", "mix_session_id": "ccb-repeat", "observation_id": observationID,
			"view_ids": []any{"track.stereo_space"}, "max_disclosure_bytes": 8192,
		},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondBundle := testMap(t, second.Result["bundle"])
	if firstString(secondBundle, "observation_id") != observationID {
		t.Fatalf("second request lost observation binding: %+v", secondBundle)
	}
	data, _ := json.Marshal(second.Result)
	for _, forbidden := range []string{"observation_path", "context_pack_path", "waveform_array", "raw_pcm"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("forbidden payload %q leaked: %s", forbidden, data)
		}
	}
}

func TestCCBObservationReceiptRecordsModelRequestAndScope(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(t.TempDir(), "mixboard"))
	t.Setenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS", "1")
	h := New(nil, shadowProjectWithClips(), nil)
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id": "ccb-audit", "mix_session_id": "ccb-audit-session",
			"view_ids":    []any{"track.peak_structure", "track.activity_structure"},
			"target_kind": "track", "target_id": "1007",
		},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect"}},
		Source:  "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	bundle := testMap(t, response.Result["bundle"])
	receipt := testMap(t, bundle["audit_receipt"])
	if receipt["requested_by"] != "model" || receipt["scope"] != "selected_track" {
		t.Fatalf("receipt attribution/scope = %+v", receipt)
	}
	requested := stringSliceFromAny(receipt["model_requested_view_ids"])
	actual := stringSliceFromAny(receipt["actual_executed_view_ids"])
	if len(requested) != 2 || len(actual) != 2 || receipt["view_set_matches"] != true || requested[0] != actual[0] || requested[1] != actual[1] {
		t.Fatalf("receipt view sets = requested=%v actual=%v receipt=%+v", requested, actual, receipt)
	}
}

func TestCCBObservationRejectsEmptyViewsWithoutDefault(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	response, err := h.Invoke(context.Background(), InvokeRequest{Tool: "ccb.observation_request", Args: map[string]any{}, Source: "agentloop"})
	if err != nil {
		t.Fatal(err)
	}
	result := testMap(t, response.Result)
	if result["status"] != "rejected" {
		t.Fatalf("empty view request status = %+v", result)
	}
	bundle := testMap(t, result["bundle"])
	receipt := testMap(t, bundle["audit_receipt"])
	if receipt["view_set_matches"] == true || len(stringSliceFromAny(receipt["rejection_reasons"])) == 0 {
		t.Fatalf("empty view rejection receipt = %+v", receipt)
	}
}

func TestCCBObservationMixedUnknownViewRejectsOnlyBlockingMember(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id":  "ccb-mixed-unknown",
			"view_ids":    []any{"track.basic_energy", "track.not_in_catalog"},
			"target_kind": "track", "target_id": "1007",
		},
		Source: "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	bundle := testMap(t, testMap(t, response.Result)["bundle"])
	if bundle["status"] != "rejected" || bundle["rejection_scope"] != "exact_view_set" {
		t.Fatalf("mixed unknown request was not rejected as exact set: %#v", bundle)
	}
	blocking := stringSliceFromAny(bundle["blocking_view_ids"])
	nonBlocking := stringSliceFromAny(bundle["non_blocking_view_ids"])
	if len(blocking) != 1 || blocking[0] != "track.not_in_catalog" {
		t.Fatalf("blocking views = %v, want only unknown view", blocking)
	}
	if len(nonBlocking) != 1 || nonBlocking[0] != "track.basic_energy" {
		t.Fatalf("non-blocking views = %v, want known view", nonBlocking)
	}
	receipt := testMap(t, bundle["audit_receipt"])
	if !sameStringSetTest(stringSliceFromAny(receipt["blocking_view_ids"]), blocking) ||
		!sameStringSetTest(stringSliceFromAny(receipt["non_blocking_view_ids"]), nonBlocking) {
		t.Fatalf("receipt view classes disagree with bundle: %#v", receipt)
	}
}

func sameStringSetTest(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		if !seen[value] {
			return false
		}
	}
	return true
}

func TestCCBObservationRejectsDeferredViewBeforeMaterialization(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"request_id":  "ccb-deferred",
			"view_ids":    []any{"mix.masking_relationship"},
			"target_kind": "project",
			"target_id":   "current",
		},
		Source: "agentloop",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := testMap(t, response.Result)
	if result["status"] != "rejected" {
		t.Fatalf("deferred view status = %#v", result)
	}
	bundle := testMap(t, result["bundle"])
	if got := stringSliceFromAny(bundle["requested_views"]); len(got) != 1 || got[0] != "mix.masking_relationship" {
		t.Fatalf("deferred requested views = %#v", bundle["requested_views"])
	}
	receipt := testMap(t, bundle["audit_receipt"])
	if receipt["view_set_matches"] == true || len(stringSliceFromAny(receipt["actual_executed_view_ids"])) != 0 {
		t.Fatalf("deferred receipt indicates execution = %#v", receipt)
	}
	reasons := stringSliceFromAny(receipt["rejection_reasons"])
	found := false
	for _, reason := range reasons {
		if strings.Contains(reason, "availability=deferred") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("deferred reason missing: %#v", receipt)
	}
}

func TestCCBObservationCatalogDoesNotMutateOrRequireConfirmation(t *testing.T) {
	h := New(nil, shadowProjectWithClips(), nil)
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "ccb.observation_catalog",
		Args:   map[string]any{"target_kind": "track", "target_id": "1007"},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.RiskLevel != tools.RiskDirect || response.RequiresConfirmation || response.Result["catalog"] == nil {
		t.Fatalf("catalog response = %+v", response)
	}
}

func TestCCBObservationScopeHonorsExplicitTrackForMixedViews(t *testing.T) {
	req := capabilitycontext.FreeStateObservationRequest{
		ViewIDs:   []string{"track.time_dynamics", "mix.multitrack_relationship"},
		TargetRef: mixboard.TargetRef{Kind: "track", ID: "track-vocal"},
	}
	if got := ccbObservationScope(req); got != "full_project_with_focus_track" {
		t.Fatalf("scope = %q, want full_project_with_focus_track", got)
	}
}

func TestCCBObservationCommandCannotNarrowMixViewToSelectedTrack(t *testing.T) {
	req := capabilitycontext.FreeStateObservationRequest{
		ViewIDs:   []string{"mix.multitrack_relationship"},
		TargetRef: mixboard.TargetRef{Kind: "project", ID: "current"},
	}
	cmd := ccbObservationCommand(map[string]any{"scope": "selected_track"}, req)
	if got := firstString(cmd, "scope"); got != "full_project" {
		t.Fatalf("scope = %q, want full_project: %#v", got, cmd)
	}
}
