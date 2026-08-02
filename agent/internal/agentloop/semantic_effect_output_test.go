package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

func TestDiagnosticWhyQuestionNeverCreatesConfirmationAtReadyOrPartialEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
	}{
		{name: "ready", status: "ready"},
		{name: "partial", status: "building"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeMessageCompleter{responses: []string{
				`{"final":false,"reply":"继续读取主唱频段证据。","tool_calls":[{"id":"read_vocal_bands","tool":"mix.read","args":{"keys":["track.vocal.slow.band_energy.summary"]},"reason":"核对主唱低中频证据"}]}`,
				`{"final":true,"reply":"观察显示主要问题集中在 250–500 Hz，可能是低中频能量堆积。\n建议先把 300–400 Hz 作为后续核对方向，但这次只做原因讨论。\n需要我继续执行吗？\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"减少浑浊\",\"target_ref\":\"track:vocal\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\"}","tool_calls":[]}`,
			}}
			exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
				"status":         "ok",
				"observation_id": "obs_vocal_mud",
				"mix_session_id": "mix_goal5",
				"track_id":       "vocal",
				"acoustic_package_status": map[string]any{
					"status": tc.status,
				},
				"observation": map[string]any{
					"target_ref": map[string]any{"kind": "track", "id": "vocal", "label": "Vocals"},
				},
			}}
			loop := &MessageLoop{
				Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
				Executor: exec, Budget: Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
			}
			res := loop.Start(context.Background(), Input{
				UserText:     "为什么这一段主唱听起来浑？",
				AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.propose_tick", "mix.apply_tick"},
				Context:      map[string]any{"selected_track_id": "vocal"},
				ExecutionMemory: ExecutionMemory{
					PendingMixTickCandidate: &PendingMixTickCandidate{Status: "pending_confirmation", TrackID: "stale"},
					PendingMixTreatment:     &MixTreatmentPending{Status: "pending_confirmation", TargetRef: "track:stale"},
				},
			})

			if res.Status != "completed" || res.StopReason != StopReasonDone {
				t.Fatalf("diagnosis stopped at status=%q reason=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
			}
			if res.SemanticAction != nil || res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
				t.Fatalf("diagnosis created mutation authority: semantic=%#v memory=%+v", res.SemanticAction, res.ExecutionMemory)
			}
			if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.read" {
				t.Fatalf("diagnosis route=%+v, want mix.observe -> mix.read only", exec.calls)
			}
			lower := strings.ToLower(res.Reply)
			if strings.Contains(lower, "mix_treatment_pending") || messageLoopMixReplyAsksForExecution(res.Reply) {
				t.Fatalf("diagnosis leaked proposal/confirmation language: %q", res.Reply)
			}
		})
	}
}

func TestParseMessageLoopSemanticEQAction(t *testing.T) {
	raw := `{"final":true,"reply":"建议先做一个保守的高架提升。","semantic_action":{"schema_version":"semantic_effect_action.v1","action_type":"eq_edit","payload_schema":"semantic_effect.eq_plan.v1","target":{"track_id":"track-1","plugin_id":"plugin-1"},"user_goal":"提高一些高频","evidence_decision":{"choice":"not_needed","basis":"user_report","reason":"the user requested a conservative tonal move"},"eq_plan":{"schema_version":"semantic_effect.eq_plan.v1","atomic":true,"atoms":[{"atom_id":"high-air","action":"upsert","shape":"high_shelf","frequency_hz":9000,"gain_db":1.25,"purpose":"增加高频空气感","field_origins":{"frequency_hz":"llm_selected","gain_db":"llm_selected"},"confidence":"medium"}]}},"tool_calls":[]}`
	out, err := parseMessageLoopOutput(raw)
	if err != nil {
		t.Fatalf("parse semantic action: %v", err)
	}
	if out.SemanticAction == nil || out.SemanticAction.EQPlan == nil || len(out.SemanticAction.EQPlan.Atoms) != 1 {
		t.Fatalf("semantic action missing: %#v", out.SemanticAction)
	}
	if err := out.SemanticAction.Validate(); err != nil {
		t.Fatalf("semantic action invalid: %v", err)
	}
}

func TestSemanticEffectDiscussionDoesNotAllowProposal(t *testing.T) {
	state := &runState{input: Input{UserText: "为什么听起来浑？"}}
	if messageLoopSemanticEffectProposalAllowed(state) {
		t.Fatal("discussion-only question allowed a semantic action")
	}
	state.input.UserText = "减少一些浑浊"
	if !messageLoopSemanticEffectProposalAllowed(state) {
		t.Fatal("actionable tonal request did not allow a semantic action")
	}
}

func TestOrdinarySemanticEQRequestBypassesLegacyMixTreatmentPending(t *testing.T) {
	state := &runState{input: Input{
		UserText: "减少一些浑浊",
		Context:  map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1015"},
	}}
	if !messageLoopOrdinarySemanticEQRequest(state) {
		t.Fatal("actionable selected-plugin EQ request was not recognized")
	}
	reply := messageLoopMixObservationFinalReply(state, "建议准备一个保守的 EQ 处理，是否执行？")
	if state.executionMemory.PendingMixTreatment != nil {
		t.Fatalf("legacy MixTreatmentPending was created: %#v", state.executionMemory.PendingMixTreatment)
	}
	if reply == "" {
		t.Fatal("semantic EQ reply was erased")
	}

	state.input.UserText = "为什么听起来浑？"
	if messageLoopOrdinarySemanticEQRequest(state) {
		t.Fatal("discussion-only question was classified as actionable semantic EQ")
	}
}

func TestCurrentTrackLowEndObservationKeepsExactTrackFocus(t *testing.T) {
	state := &runState{input: Input{
		UserText:     "帮我把当前轨道低频降低一些",
		AllowedTools: []string{"mix.observe"},
		Context: map[string]any{
			"selected_track_id":        "1007",
			"selected_plugin_track_id": "1007",
			"selected_plugin_id":       "1015",
		},
	}}
	state.executionMemory.ActiveWorkTargetTrackID = "stale-track"
	call := messageLoopDeterministicMixObservationCall(state)
	if got := firstMapText(call.Args, "scope"); got != "full_project_with_focus_track" {
		t.Fatalf("scope=%q, want focused project observation; args=%+v", got, call.Args)
	}
	if got := firstMapText(call.Args, "track_id"); got != "1007" {
		t.Fatalf("track_id=%q, want exact selected track; args=%+v", got, call.Args)
	}
	focus := messageLoopMapValue(call.Args["focus_hint"])
	if firstMapText(focus, "track_id") != "1007" || firstMapText(focus, "source") != "exact_selected_track" {
		t.Fatalf("focus_hint=%+v, want exact selected track", focus)
	}
}

func TestSemanticEQWithoutPluginRequiresSelectionOnlyForAction(t *testing.T) {
	state := &runState{input: Input{
		UserText: "减少一些浑浊",
		Context:  map[string]any{"selected_track_id": "1007"},
	}}
	if !messageLoopSemanticEQNeedsPluginSelection(state) {
		t.Fatal("actionable selected-track EQ request did not stop at plugin selection")
	}
	state.input.UserText = "你推荐用哪个 EQ？"
	if messageLoopSemanticEQNeedsPluginSelection(state) {
		t.Fatal("recommendation-only question was misclassified as an execution handoff")
	}
	state.input.UserText = "创建轨道并加载 TDR Nova EQ"
	state.executionMemory.ActiveWorkTargetTrackID = "1007"
	if messageLoopSemanticEQNeedsPluginSelection(state) {
		t.Fatal("explicit plugin-loading workflow was blocked as an acoustic EQ action")
	}
}

func TestLoadedSemanticEQFinalProseReturnsToDedicatedPlannerFallback(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"建议做一个保守的低中频处理。","tool_calls":[]}`}}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "减少一些浑浊",
		Context: map[string]any{
			"selected_track_id": "1007", "selected_plugin_id": "1015",
			"generic_eq_topology": map[string]any{"schema_version": "generic_eq_topology.prompt.v1", "sections": []any{map[string]any{"section": "1"}}},
		},
	})
	if res.Status != "completed" || len(client.calls) != 1 || res.SemanticAction != nil {
		t.Fatalf("result status=%q calls=%d semantic=%#v reply=%q", res.Status, len(client.calls), res.SemanticAction, res.Reply)
	}
}
