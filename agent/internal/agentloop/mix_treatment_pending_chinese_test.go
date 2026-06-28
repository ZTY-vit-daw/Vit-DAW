package agentloop

import "testing"

func TestMessageLoopChineseLowEQPrepCreatesPendingAfterObservation(t *testing.T) {
	state := &runState{
		input: Input{UserText: "轻微低频 EQ，可以进行处理。"},
		executed: []map[string]any{{
			"tool":         "mix.observe",
			"command_name": "mix_request_observation",
			"status":       "ok",
			"result": map[string]any{
				"status":         "ok",
				"observation_id": "obs_low_eq",
				"mix_session_id": "mix_low_eq",
				"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			},
		}},
		contextSnapshot: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1"}},
		},
	}

	if !messageLoopLowMudPluginPrepRequest(state.input.UserText) {
		t.Fatalf("Chinese low-EQ action text was not routed as plugin prep")
	}
	treatment := messageLoopConservativeLowMudTreatmentPendingFromReply(state, "建议先准备一个保守低切或低中频小幅削减方案，确认后再处理。")
	if treatment == nil {
		t.Fatalf("pending treatment was not created")
	}
	if treatment.Status != "pending_confirmation" || treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || treatment.ObservationID != "obs_low_eq" || len(treatment.EvidenceRefs) == 0 {
		t.Fatalf("pending treatment lost observation linkage: %+v", treatment)
	}
}

func TestMessageLoopChineseLowCutWithEvidenceFirstIsActionPreflight(t *testing.T) {
	userText := "\u5e2e\u6211\u628a\u4f4e\u9891\u7a0d\u5fae\u6536\u4e00\u70b9\uff0c\u4f46\u5148\u544a\u8bc9\u6211\u4f9d\u636e\u3002"
	if !messageLoopLowMudPluginPrepRequest(userText) {
		t.Fatalf("Chinese low-frequency adjustment with evidence-first wording was not routed as action preflight")
	}
	args := messageLoopMixObservationArgs(userText, map[string]any{})
	if args["mom_intent"] != "action_preflight_observation" {
		t.Fatalf("mom_intent = %v, want action_preflight_observation; args=%+v", args["mom_intent"], args)
	}
}

func TestMessageLoopChineseObservationFollowupCanProceed(t *testing.T) {
	state := &runState{
		input: Input{UserText: "可以进行处理"},
		recentObservation: &RecentObservation{
			Tool:   "mix.observe",
			Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs_followup",
			},
		},
	}

	if !messageLoopMixObservationActionFollowupRequest(state) {
		t.Fatalf("Chinese action follow-up was not recognized")
	}
}
