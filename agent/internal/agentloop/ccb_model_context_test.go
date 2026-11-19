package agentloop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type telemetryRequestCompleter struct {
	request llm.Request
	text    string
}

func (c *telemetryRequestCompleter) Complete(_ context.Context, _ config.EngineConfig, _ []llm.Message) (string, error) {
	return c.text, nil
}

func (c *telemetryRequestCompleter) CompleteRequest(_ context.Context, _ config.EngineConfig, request llm.Request) (llm.Response, error) {
	c.request = request
	return llm.Response{Text: c.text}, nil
}

func TestCCBModelRequestsStayCanonicalAndReportLayerSizes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		toolResult planner.ToolResult
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
		},
		{
			name:       "observation",
			toolResult: agentLoopCCBObservationResult(),
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
				recordFreeStateCCBObservation(state, state.recentObservation)
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
			size := messageLoopMapValue(decoded["context_size"])
			t.Logf("%s model=%d request=%d hot=%v warm=%v sections=%v", tc.name, len(modelJSON), total, size["hot_bytes"], size["warm_bytes"], contextruntime.ModelSectionBytes(decoded))
			if size["total_bytes"] == nil || size["hot_bytes"] == nil || size["warm_bytes"] == nil || size["full_budget"] == nil {
				t.Fatalf("model context size report is incomplete: %#v", size)
			}
			if tc.name == "observation" && contextruntime.ModelSectionBytes(decoded)["observation_ledger"] == 0 {
				t.Fatalf("observation size fixture omitted compact ledger: sections=%v", contextruntime.ModelSectionBytes(decoded))
			}
			hotBytes, ok := size["hot_bytes"].(float64)
			if !ok || int(hotBytes) > contextruntime.ModelHotBudgetBytes {
				t.Fatalf("representative %s hot layer exceeded boundary: size=%#v", tc.name, size)
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

func TestMessageLoopModelContextTelemetryReportsCanonicalSections(t *testing.T) {
	snapshot := map[string]any{
		"schema_version": "vit_context_snapshot.v1",
		"active_observation": map[string]any{
			"schema_version": contextruntime.CCBModelProjectionSchema, "kind": "observation",
			"tool_call_id": "ccb-call", "observation_id": "obs-1", "status": "ready",
		},
		"observation_ledger": map[string]any{
			"schema_version": "free_state_observation_ledger.v1",
			"receipts":       []any{map[string]any{"receipt_id": "receipt-1", "tool_call_id": "ccb-call", "observation_id": "obs-1"}},
		},
		"context_size": map[string]any{"hot_bytes": 512, "warm_bytes": 256},
	}
	stats := messageLoopModelContextPromptStats(contextruntime.ModelJSON(snapshot), []llm.Message{
		{Role: "system", Content: "static"}, {Role: "user", Content: "runtime"},
	})
	if stats["canonical_ccb_projection_count"] != 1 || stats["active_observation_id"] != "obs-1" || stats["active_tool_call_id"] != "ccb-call" {
		t.Fatalf("canonical telemetry = %#v", stats)
	}
	if stats["active_observation_bytes"].(int) == 0 || stats["observation_ledger_bytes"].(int) == 0 ||
		stats["static_prompt_bytes"] != 6 || stats["runtime_prompt_bytes"] != 7 || stats["full_request_content_bytes"] != 13 {
		t.Fatalf("section telemetry = %#v", stats)
	}
}

func TestMessageLoopSendsModelContextSectionTelemetryToLLMRequest(t *testing.T) {
	client := &telemetryRequestCompleter{text: `{"final":true,"reply":"The requested view is deferred.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"The requested view is deferred.","stop_reason":"ccb_view_deferred","limitations":["No equivalent view is available."]},"tool_calls":[]}`}
	result := agentLoopCCBObservationResult()
	execResult := executorpkg.Result{ToolCallID: result.ToolCallID, Tool: result.Tool, Status: result.Status, Result: result.Result}
	recent := recentObservationForTool(planner.ToolCall{ID: result.ToolCallID, Tool: result.Tool}, execResult, planner.VerificationResult{}, false, nil)
	loop := MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: &freeStateTestExecutor{}, Budget: Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "inspect the project", RecentObservation: recent, AllowedTools: []string{"ccb.observation_request"},
		Trace: []planner.TraceEvent{{Kind: "tool_result", ToolResult: &result}},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "decision_phase": "processor_selection",
			"original_intent": "inspect the project",
			"observation_ledger": map[string]any{
				"schema_version": freeStateObservationLedgerSchema,
				"receipts":       []any{map[string]any{"receipt_id": "receipt-size", "tool_call_id": "observation-call", "observation_id": "observation-size", "status": "ready"}},
			},
		}},
	})
	if res.FreeStateDecision == nil || res.FreeStateDecision.Status != FreeStateBlocked {
		t.Fatalf("test request did not complete: %+v", res)
	}
	stats := client.request.Metadata.PromptStats
	if client.request.Metadata.Source != "message_loop" || stats["canonical_ccb_projection_count"] != 1 ||
		stats["active_observation_bytes"].(int) == 0 || stats["observation_ledger_bytes"].(int) == 0 ||
		stats["model_snapshot_bytes"].(int) == 0 || stats["full_request_content_bytes"].(int) == 0 {
		t.Fatalf("LLM request metadata missing model context telemetry: metadata=%+v", client.request.Metadata)
	}
	if stats["active_observation_id"] != "observation-size" || stats["active_tool_call_id"] != "observation-call" {
		t.Fatalf("canonical identifiers missing from telemetry: %#v", stats)
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

// TestFreeStateNeutralModelSnapshotKeepsBoundedLessonContext 覆盖 W02-A1c
// run_20261119_2020 的 S4 生产路径：free-state active 走 neutral 投影时，
// A1b 的有界课程上下文（card/node/assets/selected clip）必须仍在模型可见
// JSON 里，同时 neutral 投影继续剥离 plugin 身份。非 free-state 轮不受影响。
func TestFreeStateNeutralModelSnapshotKeepsBoundedLessonContext(t *testing.T) {
	lesson := map[string]any{
		"schema_version": contextruntime.LessonContextSchemaVersion,
		"card":           map[string]any{"id": "first-run-twinkle", "title": "一个旋律的变奏"},
		"node":           map[string]any{"id": "n1_read_and_fix", "title": "读谱纠错", "goal": "把识别错的音改对"},
		"assets":         []map[string]any{{"id": "theme", "kind": "midi_reference"}},
		"selected_clip":  map[string]any{"clip_id": "c_take_second", "track_id": "t1"},
		"allowed":        []string{"mutate_midi_notes"},
		"truncated":      false,
	}
	state := &runState{
		goal: agentruntime.Goal{GoalID: "goal-lesson", RunID: "run-lesson"},
		input: Input{
			UserText: "这一关为什么要把这个音改对？",
			Summary:  "这一关为什么要把这个音改对？",
			Context: map[string]any{
				contextruntime.LessonContextKey: lesson,
				"selected_plugin_id":            "plugin-secret",
				"selected_plugin_name":          "Secret Vendor Comp",
				"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1",
					"status":         "reasoning",
					"decision_phase": "processor_selection",
				},
			},
			State: map[string]any{
				"initialized": true,
				"tracks": []any{map[string]any{
					"track_id": "t1", "track_name": "Lead",
					"plugins": []any{map[string]any{"plugin_id": "plugin-secret", "plugin_name": "Secret Vendor Comp"}},
				}},
			},
		},
	}
	runner := &Runner{}
	full := runner.buildContextSnapshot(state)
	if full.LessonContext == nil {
		t.Fatalf("precondition: full snapshot lost lesson_context")
	}
	modelJSON := runner.buildModelContextSnapshot(state, full)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(modelJSON), &decoded); err != nil {
		t.Fatalf("model snapshot is not JSON: %v", err)
	}
	projected, ok := decoded["lesson_context"].(map[string]any)
	if !ok {
		t.Fatalf("free-state neutral model snapshot dropped lesson_context: %s", modelJSON)
	}
	if projected["schema_version"] != contextruntime.LessonContextSchemaVersion {
		t.Fatalf("lesson schema = %v", projected["schema_version"])
	}
	card, _ := projected["card"].(map[string]any)
	node, _ := projected["node"].(map[string]any)
	clip, _ := projected["selected_clip"].(map[string]any)
	if card["id"] != "first-run-twinkle" || node["id"] != "n1_read_and_fix" || clip["clip_id"] != "c_take_second" || clip["track_id"] != "t1" {
		t.Fatalf("lesson projection lost card/node/selected clip: %v", projected)
	}
	if assets, _ := projected["assets"].([]any); len(assets) != 1 {
		t.Fatalf("lesson assets = %v", projected["assets"])
	}
	for _, forbidden := range []string{"plugin-secret", "Secret Vendor Comp", "selected_plugin"} {
		if strings.Contains(modelJSON, forbidden) {
			t.Fatalf("neutral model snapshot leaked plugin identity %q: %s", forbidden, modelJSON)
		}
	}
	if degradation, _ := decoded["context_degradation"].(map[string]any); degradation != nil {
		if omitted, _ := degradation["omitted_sections"].([]any); len(omitted) > 0 {
			for _, section := range omitted {
				if section == "lesson_context" {
					t.Fatalf("degradation still claims lesson_context was omitted: %v", omitted)
				}
			}
		}
	}

	// Non-free-state turns keep using the full profile: no regression.
	state.input.Context = map[string]any{
		contextruntime.LessonContextKey: lesson,
		"selected_clip_id":              "c_take_second",
	}
	full = runner.buildContextSnapshot(state)
	modelJSON = runner.buildModelContextSnapshot(state, full)
	if !strings.Contains(modelJSON, "\"lesson_context\"") {
		t.Fatalf("non-free-state model snapshot dropped lesson_context: %s", modelJSON)
	}
}
