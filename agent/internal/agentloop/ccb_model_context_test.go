package agentloop

import (
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/contextruntime"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestCCBModelRequestsStayCanonicalAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name       string
		toolResult planner.ToolResult
		maxRequest int
	}{
		{
			name: "catalog",
			toolResult: planner.ToolResult{
				ToolCallID: "catalog-call", Tool: "ccb.observation_catalog", Status: "ok",
				Result: map[string]any{
					"status": "ok",
					"catalog": capabilitycontext.FreeStateObservationCatalogFor(mixboard.TargetRef{
						Kind: "project", ID: "project-1", Source: "test",
					}),
				},
			},
			maxRequest: 12 * 1024,
		},
		{
			name:       "observation",
			toolResult: agentLoopCCBObservationResult(),
			maxRequest: 12 * 1024,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &runState{
				goal: agentruntime.Goal{GoalID: "goal-ccb-size", RunID: "run-ccb-size"},
				input: Input{
					UserText: "inspect the project and decide from the evidence",
					Summary:  "inspect the project and decide from the evidence",
					Context: map[string]any{"free_state_reasoning_loop": map[string]any{
						"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "decision_phase": "processor_selection",
					}},
				},
				trace:  []planner.TraceEvent{{Kind: "tool_result", ToolResult: &tc.toolResult}},
				budget: Budget{MaxTurns: 4, MaxToolCalls: 4},
			}
			if messageLoopIsCCBObservationRequestName(tc.toolResult.Tool) {
				execResult := executorpkg.Result{
					ToolCallID: tc.toolResult.ToolCallID, Tool: tc.toolResult.Tool, Status: tc.toolResult.Status, Result: tc.toolResult.Result,
				}
				state.recentObservation = recentObservationForTool(planner.ToolCall{
					ID: tc.toolResult.ToolCallID, Tool: tc.toolResult.Tool,
				}, execResult, planner.VerificationResult{}, false, nil)
				if modelMap := state.recentObservation.Summary; modelMap["audit_receipt"] == nil || !strings.Contains(string(mustAgentLoopJSON(t, modelMap)), "raw-frequency-profile") {
					t.Fatalf("internal CCB bundle was compacted before audit/evaluation: %#v", modelMap)
				}
			}

			runner := &Runner{}
			full := runner.buildContextSnapshot(state)
			modelJSON := runner.buildModelContextSnapshot(state, full)
			assembly := (&MessageLoop{}).assembly(state, modelJSON)
			total := 0
			for _, message := range assembly.Messages {
				total += len(message.Content)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(modelJSON), &decoded); err != nil {
				t.Fatalf("model snapshot is not JSON: %v", err)
			}
			t.Logf("%s model=%d request=%d sections=%v", tc.name, len(modelJSON), total, contextruntime.ModelSectionBytes(decoded))
			if total > tc.maxRequest {
				t.Fatalf("request = %d bytes, max %d; model=%d sections=%v", total, tc.maxRequest, len(modelJSON), contextruntime.ModelSectionBytes(decoded))
			}
			if count := strings.Count(modelJSON, `"schema_version":"`+contextruntime.CCBModelProjectionSchema+`"`); count != 1 {
				t.Fatalf("canonical projection count = %d, want 1: %s", count, modelJSON)
			}
			for _, forbidden := range []string{"raw-frequency-profile", "Secret Vendor", "plugin-secret", "parameter-secret", "compressor-family"} {
				if strings.Contains(modelJSON, forbidden) {
					t.Fatalf("model snapshot leaked %q: %s", forbidden, modelJSON)
				}
			}
		})
	}
}

func mustAgentLoopJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func agentLoopCCBObservationResult() planner.ToolResult {
	return planner.ToolResult{
		ToolCallID: "observation-call", Tool: "ccb.observation_request", Status: "ok",
		Result: map[string]any{
			"status": "ready",
			"bundle": map[string]any{
				"schema_version":     "ccb_observation_bundle.v1",
				"bundle_id":          "bundle-size",
				"request_id":         "request-size",
				"status":             "ready",
				"read_only":          true,
				"mutation_authority": false,
				"observation_id":     "observation-size",
				"target_ref":         map[string]any{"kind": "project", "id": "project-1"},
				"freshness":          map[string]any{"class": "current_observation", "status": "ready"},
				"requested_views":    []any{"project.structure", "mix.multitrack_relationship", "mix.frequency_relationship"},
				"evidence_refs":      []any{"evidence://structure", "evidence://relationship", "evidence://frequency"},
				"limitations":        []any{"bounded semantic evidence"},
				"views": map[string]any{
					"project.structure": map[string]any{
						"view_id": "project.structure", "status": "ready",
						"facts": map[string]any{
							"project.static.summary": map[string]any{"status": "ready", "track_count": 6, "duration_seconds": 120.0},
							"project.tracks.summary": map[string]any{"status": "ready", "track_count": 6, "tracks": []any{
								map[string]any{"track_id": "track-1", "name": "Lead", "plugin_id": "plugin-secret", "vendor": "Secret Vendor"},
								map[string]any{"track_id": "track-2", "name": "Bass"},
							}},
						},
					},
					"mix.multitrack_relationship": map[string]any{
						"view_id": "mix.multitrack_relationship", "status": "ready",
						"facts": map[string]any{"observation.mom_projection": map[string]any{
							"multitrack_relation": map[string]any{
								"status": "ready", "loudness_order": []any{"track-1", "track-2"},
								"band_conflict_candidates": []any{map[string]any{"band": "low_mid", "tracks": []any{"track-1", "track-2"}}},
								"limitations":              []any{"relationship is broad-band"}, "evidence_refs": []any{"evidence://relationship"},
							},
						}},
					},
					"mix.frequency_relationship": map[string]any{
						"view_id": "mix.frequency_relationship", "status": "ready",
						"facts": map[string]any{
							"observation.mom_projection": map[string]any{"frequency_relationship": map[string]any{
								"status": "ready", "freshness": "fresh",
								"coverage":            map[string]any{"usable_track_count": 6, "track_count": 6},
								"conflict_candidates": []any{map[string]any{"band": "low_mid", "status": "plausible", "tracks": []any{"track-1", "track-2"}}},
								"tonal_tendencies":    []any{map[string]any{"track_id": "track-1", "tendency": "low_mid_weighted"}},
								"track_profiles":      []any{map[string]any{"track_id": "track-1", "raw_detail": strings.Repeat("raw-frequency-profile ", 2000)}},
								"limitations":         []any{"broad bands only"}, "evidence_refs": []any{"evidence://frequency"},
							}},
							"project.frequency_relationship_inputs": map[string]any{
								"status": "ready", "track_count": 6, "usable_track_count": 6,
								"tracks": []any{map[string]any{"track_id": "track-1", "name": "Lead"}, map[string]any{"track_id": "track-2", "name": "Bass"}},
							},
						},
					},
				},
				"audit_receipt": map[string]any{
					"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-size", "view_set_matches": true,
				},
			},
		},
	}
}
