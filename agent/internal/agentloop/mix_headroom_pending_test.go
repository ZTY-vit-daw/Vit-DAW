package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

func TestMessageLoopFullProjectDiagnosisOnlyCreatesHeadroomTreatmentPending(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Overall mix has two active audio tracks. Track 2 is the main risk: its peak is at full scale and available headroom is exhausted. Track 1 has safer headroom. LUFS, masking, and reference matching are still deferred, so this is based on the materialized level and headroom read model.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_project",
		"observation_id": "obs_project",
		"digest": map[string]any{
			"scope": "full_project",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
			"likely_first_attention_target": map[string]any{
				"reason": "headroom_risk",
				"track": map[string]any{
					"track_id":    "1010",
					"name":        "Track 2",
					"headroom_db": 0,
					"risk":        "high",
				},
			},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Track 1",
					"track_name":       "Track 1",
					"user_label":       "Track 1",
					"user_track_index": 1,
					"volume_db":        0,
					"rms_dbfs":         -9.032,
					"peak_dbfs":        -6.021,
					"headroom_db":      6.021,
				}, {
					"track_id":         "1010",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -10.603,
					"peak_dbfs":        0,
					"headroom_db":      0,
				}},
				"headroom_risk": []map[string]any{{
					"track_id":    "1010",
					"name":        "Track 2",
					"rank":        1,
					"risk":        "high",
					"headroom_db": 0,
				}},
				"likely_first_attention_target": map[string]any{
					"reason": "headroom_risk",
					"track": map[string]any{
						"track_id":    "1010",
						"name":        "Track 2",
						"risk":        "high",
						"headroom_db": 0,
					},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "look at the overall mix",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1", "volume_db": 0,
		}, {
			"track_id": "1010", "track_name": "Track 2", "volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("diagnosis-only fallback created a tick candidate instead of treatment: %+v", res.ExecutionMemory.PendingMixTickCandidate)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if treatment.ActionKind != "gain_balance" || treatment.TargetRef != "track:1010" || treatment.DeltaDB != -1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.ObservationID != "obs_project" || treatment.Fingerprint["source"] != "materialized_headroom_risk_after_observation" {
		t.Fatalf("pending treatment metadata = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopGainSuggestionWinsOverStereoCenterPanSummary(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Overall mix has two active audio tracks. The stereo balance is centered and correlation is stable. Track 2 is the main risk because its peak is at full scale and headroom is exhausted. Suggested first step: lower Track 2 by 1.5 dB to create headroom. Should I execute this adjustment?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_project",
		"observation_id": "obs_project",
		"digest": map[string]any{
			"scope": "full_project",
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Track 1",
					"track_name":       "Track 1",
					"user_label":       "Track 1",
					"user_track_index": 1,
					"volume_db":        0,
					"peak_dbfs":        -6.021,
					"headroom_db":      6.021,
				}, {
					"track_id":         "1010",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"peak_dbfs":        0,
					"headroom_db":      0,
				}},
				"headroom_risk": []map[string]any{{
					"track_id":    "1010",
					"name":        "Track 2",
					"risk":        "high",
					"headroom_db": 0,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "look at the overall mix",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1", "volume_db": 0,
		}, {
			"track_id": "1010", "track_name": "Track 2", "volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil {
		t.Fatalf("stereo summary was misread as pan treatment: %+v", treatment)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil {
		t.Fatalf("pending gain candidate missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if candidate.TrackID != "1010" || candidate.Operation != "track_gain_adjust" || candidate.DeltaDB != -1.5 {
		t.Fatalf("candidate = %+v", candidate)
	}
}
