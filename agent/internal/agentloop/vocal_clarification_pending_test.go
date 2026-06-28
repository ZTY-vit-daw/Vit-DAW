package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestMessageLoopDeterministicVocalClarificationPendingUsesTraceRelationship(t *testing.T) {
	state := &runState{
		input: Input{
			UserText: "\u8ba9\u4e3b\u5531\u66f4\u9760\u524d\n\nUser clarification: Track 1 \u662f\u4e3b\u5531",
			Conversation: []llm.Message{{
				Role:    "user",
				Content: "\u8ba9\u4e3b\u5531\u66f4\u9760\u524d",
			}},
		},
		trace: testVocalClarificationTrace(),
	}

	candidate := messageLoopDeterministicVocalClarificationPendingTick(state, "please specify which track is vocal")
	if candidate == nil {
		t.Fatalf("pending candidate missing")
	}
	if candidate.Operation != "track_gain_adjust" || candidate.TrackID != "1010" || candidate.DeltaDB != -1 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.ObservationID != "obs_focus" {
		t.Fatalf("observation_id = %q", candidate.ObservationID)
	}
	if got := candidate.Evidence["source"]; got != "deterministic_vocal_clarification" {
		t.Fatalf("evidence = %+v", candidate.Evidence)
	}
}

func TestMessageLoopContinueSynthesizesPendingAfterVocalTrackClarification(t *testing.T) {
	rt := agentruntime.New()
	goal := rt.Ensure("goal_vocal_clarification", "run_original", "\u8ba9\u4e3b\u5531\u66f4\u9760\u524d")
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Observation is complete. If this is a vocal request, please specify which track is vocal.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Runtime:  rt,
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Continue(context.Background(), Continuation{
		GoalID:   goal.GoalID,
		Summary:  goal.Summary,
		UserText: "\u8ba9\u4e3b\u5531\u66f4\u9760\u524d\n\nUser clarification: Track 1 \u662f\u4e3b\u5531",
		Conversation: []llm.Message{{
			Role:    "user",
			Content: "\u8ba9\u4e3b\u5531\u66f4\u9760\u524d",
		}, {
			Role:    "assistant",
			Content: "\u9700\u8981\u5148\u786e\u8ba4\u54ea\u6761\u662f\u4e3b\u5531\u8f68\u3002",
		}},
		AllowedTools:   []string{"mix.observe"},
		Trace:          testVocalClarificationTrace(),
		CompletedSteps: 2,
	})

	if res.Status != "completed" || res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q trace=%+v", res.Status, res.NeedsClarification, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("continuation executed new tools: %+v", exec.calls)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil {
		t.Fatalf("pending candidate missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if candidate.TrackID != "1010" || candidate.DeltaDB != -1 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if strings.Contains(strings.ToLower(res.Reply), "specify which track") || !strings.Contains(res.Reply, "Track 2") {
		t.Fatalf("reply = %q", res.Reply)
	}
}

func testVocalClarificationTrace() []planner.TraceEvent {
	return []planner.TraceEvent{{
		Kind: "tool_result",
		ToolResult: &planner.ToolResult{
			Tool:   "mix.observe",
			Status: "ok",
			Result: map[string]any{
				"status":         "ready",
				"observation_id": "obs_focus",
				"mix_session_id": "mix_focus",
				"observation": map[string]any{
					"target_ref": map[string]any{"kind": "project", "id": "current"},
					"project_package": map[string]any{
						"track_count": 2,
						"tracks": []map[string]any{{
							"track_id":         "1007",
							"name":             "Track 1",
							"track_name":       "Track 1",
							"user_track_index": 1,
							"volume_db":        0,
							"peak_dbfs":        -6.0,
							"headroom_db":      6.0,
						}, {
							"track_id":         "1010",
							"name":             "Track 2",
							"track_name":       "Track 2",
							"user_track_index": 2,
							"volume_db":        0,
							"peak_dbfs":        0.0,
							"headroom_db":      0.0,
						}},
					},
				},
			},
		},
	}, {
		Kind: "tool_result",
		ToolResult: &planner.ToolResult{
			Tool:   "mix.derive",
			Status: "ok",
			Result: map[string]any{
				"status":         "partial",
				"observation_id": "obs_focus",
				"mix_session_id": "mix_focus",
				"relationship": map[string]any{
					"type":           "focus_vs_project",
					"observation_id": "obs_focus",
					"status":         "partial",
					"facts": map[string]any{
						"focus_track": map[string]any{"status": "missing"},
						"project_tracks": map[string]any{
							"peak_risk": []map[string]any{{
								"track_id":    "1010",
								"name":        "Track 2",
								"rank":        1,
								"peak_dbfs":   0.0,
								"headroom_db": 0.0,
								"risk":        "high",
							}},
							"tracks": []map[string]any{{
								"track_id": "1007",
								"name":     "Track 1",
							}, {
								"track_id": "1010",
								"name":     "Track 2",
							}},
						},
					},
				},
			},
		},
	}}
}
