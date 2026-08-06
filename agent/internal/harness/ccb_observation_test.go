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
