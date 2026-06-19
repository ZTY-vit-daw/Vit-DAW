package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
)

type fakeMessageCompleter struct {
	responses []string
	calls     [][]llm.Message
}

func (f *fakeMessageCompleter) Complete(_ context.Context, _ config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), messages...))
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"done"}`, nil
	}
	out := f.responses[0]
	f.responses = f.responses[1:]
	return out, nil
}

type fakeMessageExecutor struct {
	calls                      []planner.ToolCall
	omitNoteWriteObservedNotes bool
	mixObservationResult       map[string]any
}

func isTestMixObservationTool(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "mix.observe", "mix.request_observation":
		return true
	default:
		return false
	}
}

func (f *fakeMessageExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	f.calls = append(f.calls, in.ToolCall)
	switch strings.TrimSpace(in.ToolCall.Tool) {
	case "track.add":
		return executorpkg.Result{
			ToolCallID:    in.ToolCall.ID,
			Tool:          in.ToolCall.Tool,
			CommandName:   "track.add",
			Status:        "ok",
			Result:        map[string]any{"track_id": "1007", "track_name": "Track 1"},
			ObservedState: map[string]any{"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1"}}},
		}, nil
	case "midi.create_clip":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "create_midi_clip",
			Status:      "ok",
			Result: map[string]any{
				"track_id":    "1007",
				"clip_id":     "clip_1",
				"new_clip_id": "clip_1",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"clips": []map[string]any{{
					"id":    "clip_1",
					"type":  "midi",
					"notes": []map[string]any{},
				}},
			}}},
		}, nil
	case "midi.apply_note_patch":
		notes := []map[string]any{
			{"id": "note_1", "pitch": 60, "start": 0, "length": 1},
			{"id": "note_2", "pitch": 64, "start": 1, "length": 1},
		}
		observedNotes := notes
		if f.omitNoteWriteObservedNotes {
			observedNotes = []map[string]any{}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "apply_midi_note_patch",
			Status:      "ok",
			Result: map[string]any{
				"track_id":          "1007",
				"clip_id":           "clip_1",
				"inserted_count":    2,
				"inserted_note_ids": []string{"note_1", "note_2"},
				"notes":             notes,
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"clips": []map[string]any{{
					"id":    "clip_1",
					"notes": observedNotes,
				}},
			}}},
		}, nil
	case "midi.read_notes", "midi.read_clip_notes":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "get_midi_clip_notes",
			Status:      "ok",
			Result: map[string]any{
				"track_id": "1007",
				"clip_id":  "clip_1",
				"notes": []map[string]any{
					{"id": "note_1", "pitch": 60, "start": 0, "length": 1},
					{"id": "note_2", "pitch": 64, "start": 1, "length": 1},
				},
			},
		}, nil
	case "plugin.load_to_rack":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "plugin.load_to_rack",
			Status:      "ok",
			Result: map[string]any{
				"track_id":    in.ToolCall.Args["track_id"],
				"plugin_id":   "plugin_1",
				"plugin_name": "TDR Nova",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"rack": map[string]any{"nodes": []map[string]any{{
					"plugin_id":                       "plugin_1",
					"plugin_name":                     "TDR Nova",
					"track_id":                        "1007",
					"audio_reachable_from_rack_input": true,
					"vit_effective_in_output_path":    true,
				}}},
			}}},
		}, nil
	case "plugin.get_parameters":
		rows := make([]any, 0, 8)
		for i := 0; i < 8; i++ {
			row := map[string]any{"id": "param_verbose"}
			for j := 0; j < 40; j++ {
				row[fmt.Sprintf("field_%02d", j)] = "verbose parameter field " + strings.Repeat("x", 120)
			}
			rows = append(rows, row)
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "get_plugin_parameters",
			Status:      "ok",
			Result: map[string]any{
				"track_id":         "1007",
				"plugin_id":        "plugin_1",
				"plugin_name":      "TDR Nova",
				"parameter_count":  len(rows),
				"parameter_values": rows,
			},
		}, nil
	case "mix.read":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_read",
			Status:      "ok",
			Result: map[string]any{
				"status": "ok",
				"entries": []map[string]any{{
					"key":   "observation.digest",
					"value": map[string]any{"peak_dbfs": -6.0, "rms_dbfs": -12.0, "headroom_db": 6.0},
				}},
			},
		}, nil
	case "mix.derive":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_derive",
			Status:      "ok",
			Result: map[string]any{
				"status":         "ready",
				"observation_id": firstMapText(in.ToolCall.Args, "observation_id"),
				"mix_session_id": firstMapText(in.ToolCall.Args, "mix_session_id"),
				"relationship": map[string]any{
					"type":   firstMapText(in.ToolCall.Args, "type"),
					"status": "ready",
				},
			},
		}, nil
	case "mix.request_observation", "mix.observe":
		result := f.mixObservationResult
		if len(result) == 0 {
			result = map[string]any{
				"status":         "ok",
				"track_id":       firstMapText(in.ToolCall.Args, "track_id"),
				"artifact_id":    "obs_test",
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
				"summary":        "observation ready",
				"observation": map[string]any{
					"target_ref": map[string]any{"kind": "track", "id": firstMapText(in.ToolCall.Args, "track_id"), "label": "target"},
				},
			}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_request_observation",
			Status:      "ok",
			Result:      result,
		}, nil
	case "mix.propose_tick":
		return executorpkg.Result{
			ToolCallID:           in.ToolCall.ID,
			Tool:                 in.ToolCall.Tool,
			CommandName:          "mix_propose_tick",
			Status:               "ok",
			RequiresConfirmation: true,
			Result: map[string]any{
				"status":                "ok",
				"requires_confirmation": true,
				"tick_id":               "mix_tick_test",
				"track_id":              firstMapText(in.ToolCall.Args, "track_id"),
				"delta_db":              in.ToolCall.Args["delta_db"],
				"preview":               "track_gain_adjust 1007 -1 dB",
			},
		}, nil
	case "mix.apply_tick":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_apply_tick",
			Status:      "ok",
			Result: map[string]any{
				"status":   "ok",
				"tick_id":  firstMapText(in.ToolCall.Args, "tick_id"),
				"track_id": firstMapText(in.ToolCall.Args, "track_id"),
				"after_db": -1,
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id":  "1007",
				"volume_db": -1,
			}}},
		}, nil
	case "media.index_authorized_folder":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "media_index_authorized_folder",
			Status:      "ok",
			Result: map[string]any{
				"status": "ok",
				"count":  2,
				"artifacts": []map[string]any{
					{"id": "art_media_intro", "kind": "video", "title": "宣讲视频.mp4", "status": "ready"},
					{"id": "art_media_demo", "kind": "video", "title": "演示视频.mp4", "status": "ready"},
				},
			},
		}, nil
	default:
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "error", Error: "unexpected tool"}, nil
	}
}

func TestMessageLoopFastCompletesSimpleTrackAdd(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"正在创建轨道。","tool_calls":[{"id":"create_track","tool":"track.add","args":{},"reason":"创建一条新轨道"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一条轨道",
		AllowedTools: []string{"track.add"},
	})

	if res.Status != "completed" || res.Reply != "已成功创建新轨道「Track 1」。" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(client.calls))
	}
}

func TestMessageLoopDoesNotFastCompleteTrackAddWhenRequestHasMoreWork(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"先创建轨道。","tool_calls":[{"id":"create_track","tool":"track.add","args":{},"reason":"创建一条新轨道"}]}`,
		`{"final":true,"reply":"轨道已创建，插件还需要继续处理。"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一条轨道，然后加载 EQ",
		AllowedTools: []string{"track.add"},
	})

	if res.Status != "completed" || res.Reply != "轨道已创建，插件还需要继续处理。" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
}

func TestMessageLoopFastCompletesSimpleMidiClipCreate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"正在创建 MIDI 片段。","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"创建 MIDI 片段"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u65b0\u5efa\u4e00\u4e2a MIDI clip",
		AllowedTools: []string{"midi.create_clip"},
	})

	if res.Status != "completed" || res.Reply != "\u5df2\u6210\u529f\u521b\u5efa MIDI \u7247\u6bb5\u3002" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(client.calls))
	}
}

func TestMessageLoopDoesNotFinishMidiCompoundGoalBeforeNotesAreVerified(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"先创建 MIDI 片段。","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"创建 MIDI 片段"}]}`,
		`{"final":true,"reply":"已创建 MIDI 片段。"}`,
		`{"final":false,"reply":"继续写入音符。","tool_calls":[{"id":"write_notes","tool":"midi.apply_note_patch","args":{"clip_ref":"last_created_clip","time_unit":"beats","operations":[{"op":"insert_note","pitch":60,"start":0,"length":1},{"op":"insert_note","pitch":64,"start":1,"length":1}]},"reason":"写入 2 个音符"}]}`,
		`{"final":true,"reply":"已创建 MIDI 片段并写入音符。"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 6, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一个 MIDI clip 然后输入 2 个音符",
		AllowedTools: []string{"midi.create_clip", "midi.apply_note_patch"},
	})

	if res.Status != "completed" || res.Reply != "已创建 MIDI 片段并写入音符。" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[1].Args["clip_id"])); got != "clip_1" {
		t.Fatalf("resolved clip_id = %q, want clip_1; call=%+v", got, exec.calls[1])
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "MIDI note write") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected final gate before note write; trace=%+v", res.Trace)
	}
}

func TestMessageLoopFinishesMidiGoalAfterReadbackConfirmsNotes(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"先创建 MIDI 片段。","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"创建 MIDI 片段"}]}`,
		`{"final":true,"reply":"已创建 MIDI 片段。"}`,
		`{"final":false,"reply":"继续写入音符。","tool_calls":[{"id":"write_notes","tool":"midi.apply_note_patch","args":{"clip_ref":"last_created_clip","time_unit":"beats","operations":[{"op":"insert_note","pitch":60,"start":0,"length":1},{"op":"insert_note","pitch":64,"start":1,"length":1}]},"reason":"写入 2 个音符"}]}`,
		`{"final":true,"reply":"已写入音符。"}`,
		`{"final":false,"reply":"读取片段音符确认结果。","tool_calls":[{"id":"read_notes","tool":"midi.read_notes","args":{"clip_ref":"last_created_clip"},"reason":"确认音符已经写入"}]}`,
		`{"final":true,"reply":"已创建 MIDI 片段并写入 2 个音符。"}`,
	}}
	exec := &fakeMessageExecutor{omitNoteWriteObservedNotes: true}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一个 MIDI clip 然后输入 2 个音符",
		AllowedTools: []string{"midi.create_clip", "midi.apply_note_patch", "midi.read_notes"},
	})

	if res.Status != "completed" || res.Reply != "已创建 MIDI 片段并写入 2 个音符。" {
		t.Fatalf("result = status=%q reply=%q stop=%q error=%q", res.Status, res.Reply, res.StopReason, res.Error)
	}
	if len(exec.calls) != 3 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[2].Args["clip_id"])); got != "clip_1" {
		t.Fatalf("readback clip_id = %q, want clip_1; call=%+v", got, exec.calls[2])
	}
	foundUnverifiedWriteGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "not been verified") {
			foundUnverifiedWriteGate = true
			break
		}
	}
	if !foundUnverifiedWriteGate {
		t.Fatalf("expected final gate before readback verification; trace=%+v", res.Trace)
	}
}

func TestMessageLoopExecutesDependentToolCallsWithoutIntermediateReplan(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"working","tool_calls":[{"id":"create_track","tool":"track.add","args":{}},{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_ref":"last_created_track","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"Track created and EQ loaded."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一条轨道然后加载 TDR Nova 均衡器",
		AllowedTools: []string{"track.add", "plugin.load_to_rack"},
	})

	if res.Status != "completed" || res.Reply != "Track created and EQ loaded." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(exec.calls[1].Args["track_id"].(string)); got != "1007" {
		t.Fatalf("plugin target track_id = %q, want 1007; call=%+v", got, exec.calls[1])
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2 after both tools finish", len(client.calls))
	}
	secondPrompt := client.calls[1]
	toolResultCount := 0
	for _, msg := range secondPrompt {
		if strings.HasPrefix(strings.TrimSpace(msg.Content), "<tool_result>") {
			toolResultCount++
		}
	}
	if toolResultCount != 2 {
		t.Fatalf("second LLM prompt should include two tool results, got %d messages=%+v", toolResultCount, secondPrompt)
	}
}

func TestMessageLoopRequiresMediaToolForLocalMediaListing(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"找到了 2 个视频文件：宣讲视频.mp4、演示视频.mp4。"}`,
		`{"final":false,"reply":"正在注册媒体素材","tool_calls":[{"id":"index_media","tool":"media.index_authorized_folder","args":{"asset_location":"C:\\Users\\timoz\\Desktop\\LLM md\\参赛内容","media_kinds":["video"],"limit":80},"reason":"注册该位置下的视频素材为媒体池卡片"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     `查看 C:\Users\timoz\Desktop\LLM md\参赛内容 文件夹下有哪些视频文件`,
		AllowedTools: []string{"media.index_authorized_folder"},
	})

	if res.Status != "completed" || !strings.Contains(res.Reply, "2") || !strings.Contains(res.Reply, "卡片") {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "media.index_authorized_folder" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "media artifact tool") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected media final gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopToolResultHistorySummarizesLargeExecutionResult(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"reading","tool_calls":[{"id":"read_params","tool":"plugin.get_parameters","args":{"track_id":"1007","plugin_id":"plugin_1"}}]}`,
		`{"final":true,"reply":"Parameters summarized."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "inspect plugin params",
		AllowedTools: []string{"plugin.get_parameters"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
	var toolResult string
	for _, msg := range client.calls[1] {
		content := strings.TrimSpace(msg.Content)
		if strings.HasPrefix(content, "<tool_result>") {
			toolResult = content
		}
	}
	if toolResult == "" {
		t.Fatalf("second prompt missing tool result: %+v", client.calls[1])
	}
	if strings.Contains(toolResult, "verbose parameter field") || strings.Contains(toolResult, `"parameter_values":[`) {
		t.Fatalf("tool result history leaked raw parameter payload: %s", toolResult)
	}
	for _, want := range []string{`"result_summary"`, `"plugin_id":"plugin_1"`, `"parameter_count":8`, `"preview_omitted"`} {
		if !strings.Contains(toolResult, want) {
			t.Fatalf("tool result history missing %s: %s", want, toolResult)
		}
	}
}

func TestMessageLoopNaturalMixRequestObservesBeforePluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"先准备 EQ。","tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"},"reason":"用于混音"}]}`,
		`{"final":false,"reply":"我先观察当前音频。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"先观察再判断"}]}`,
		`{"final":true,"reply":"我已经完成观察，会先根据观察结果给出建议，再决定是否需要加载插件。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想你帮我对这段音频进行缩混可以吗？",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) == 0 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want first call to be mix observation", exec.calls)
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "mix.observe") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected observe-first gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopNaturalMixRequestPreflightsWhenModelFinalsWithoutTools(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"I checked the current track, but no acoustic data returned, so lower it 1 dB.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"track_id":       "1007",
		"acoustic_digest": map[string]any{
			"waveform": map[string]any{
				"status":      "ready",
				"peak_dbfs":   -0.4,
				"rms_dbfs":    -17.2,
				"headroom_db": 0.4,
				"crest_db":    16.8,
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
		UserText:     "帮我混一下当前轨道",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
			"clips": []map[string]any{{
				"id": "1011",
			}},
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe preflight", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["track_id"]); got != "1007" {
		t.Fatalf("track_id = %q, want 1007; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["clip_id"]); got != "1011" {
		t.Fatalf("clip_id = %q, want 1011; args=%+v", got, exec.calls[0].Args)
	}
	if !strings.Contains(res.Reply, "需要我继续执行吗") {
		t.Fatalf("reply did not ask for explicit execution confirmation: %q", res.Reply)
	}
}

func TestMessageLoopNaturalMixRequestDoesNotAllowObserveAndPluginLoadInSameModelTurn(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"先观察再加载。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"}},{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"已经观察完成，下一步我会先说明建议。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "帮我缩混当前轨道",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want same-turn plugin load blocked", exec.calls)
	}
}

func TestMessageLoopPanRequestBlocksPrimitivePanUntilConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will move the pan now.","tool_calls":[{"id":"pan_track","tool":"track.pan","args":{"track_id":"1007","pan":-0.1},"reason":"move Track 2 left"}]}`,
		`{"final":true,"reply":"I observed Track 2 and can move it left a little after confirmation.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move Track 2 left a little\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"target\":{\"delta_pan\":-0.1},\"reasoning_summary\":\"single explicit pan step after observation\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 2 pan left a little",
		AllowedTools: []string{"mix.observe", "mix.request_observation", "track.pan"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want only deterministic mix.observe", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["track_id"]); got != "Track 2" {
		t.Fatalf("track_id = %q, want Track 2; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["user_track_index"]); got != "2" {
		t.Fatalf("user_track_index = %q, want 2; args=%+v", got, exec.calls[0].Args)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopMixExecutionQuestionIsNotClarification(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"needs_clarification":true,"reply":"Should I lower Track 2 by 1 dB?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "overall mix feels messy, help me fix it",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":         "1007",
			"track_name":       "Track 1",
			"user_track_index": 1,
		}, {
			"track_id":         "1010",
			"track_name":       "Track 2",
			"user_track_index": 2,
		}}},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe", exec.calls)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil || candidate.Operation != "track_gain_adjust" || candidate.TrackID != "1010" || candidate.DeltaDB != -1 {
		t.Fatalf("pending candidate = %+v", candidate)
	}
	if !strings.Contains(strings.ToLower(res.Reply), "should i lower track 2") {
		t.Fatalf("reply = %q", res.Reply)
	}
}

func TestMessageLoopPanAmountClarificationUsesSmallStepDefault(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"needs_clarification":true,"reply":"How far left should Track 2 move? -10%, -20%, or a target pan value?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 2 pan left a little",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":         "1007",
			"track_name":       "Track 1",
			"user_track_index": 1,
		}, {
			"track_id":         "1010",
			"track_name":       "Track 2",
			"user_track_index": 2,
		}}},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetRef != "track:1010" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopMixObserveAddsScopeFromIntent(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先观察整体混音。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{},"reason":"先建立工程观察上下文"}]}`,
		`{"final":true,"reply":"观察完成，我会基于整体工程数据给出建议。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "帮我看一下整体混音",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want mix.observe", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["scope"]); got != "full_project" {
		t.Fatalf("scope = %q, want full_project; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["disclosure"]); got != "digest_catalog" {
		t.Fatalf("disclosure = %q, want digest_catalog; args=%+v", got, exec.calls[0].Args)
	}
	if exec.calls[0].Args["observation_only"] != true {
		t.Fatalf("observation_only not set: %+v", exec.calls[0].Args)
	}
}

func TestMessageLoopVocalForwardRequestAddsFocusScopeAndHint(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the vocal in project context first.","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{},"reason":"observe focus relationship before suggesting a move"}]}`,
		`{"final":true,"reply":"The vocal relationship observation is ready. I can suggest a small move after confirmation.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["scope"]); got != "full_project_with_focus_track" {
		t.Fatalf("scope = %q, want full_project_with_focus_track; args=%+v", got, exec.calls[0].Args)
	}
	focusHint := messageLoopMapValue(exec.calls[0].Args["focus_hint"])
	if focusHint["role"] != "vocal" || focusHint["source"] != "user_intent" {
		t.Fatalf("focus hint = %+v; args=%+v", focusHint, exec.calls[0].Args)
	}
	if exec.calls[0].Args["observation_only"] != true {
		t.Fatalf("observation_only not set: %+v", exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[1].Args["type"]); got != "focus_vs_project" {
		t.Fatalf("derive type = %q; args=%+v", got, exec.calls[1].Args)
	}
	deriveFocus := messageLoopMapValue(exec.calls[1].Args["focus"])
	if deriveFocus["role"] != "vocal" {
		t.Fatalf("derive focus = %+v; args=%+v", deriveFocus, exec.calls[1].Args)
	}
	if got := fmt.Sprint(exec.calls[1].Args["observation_id"]); got != "obs_test" {
		t.Fatalf("derive observation_id = %q; args=%+v", got, exec.calls[1].Args)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("relationship observation should not synthesize a pending gain tick without a concrete dB suggestion: %+v", res.ExecutionMemory.PendingMixTickCandidate)
	}
}

func TestMessageLoopVocalForwardClarificationAddsFocusTrackIndexHint(t *testing.T) {
	args := messageLoopMixObservationArgs("让主唱更靠前\n\nUser clarification: Track 1 是主唱", map[string]any{})
	focusHint := messageLoopMapValue(args["focus_hint"])
	if got := focusHint["role"]; got != "vocal" {
		t.Fatalf("focus hint = %+v", focusHint)
	}
	if got := fmt.Sprint(focusHint["user_track_index"]); got != "1" {
		t.Fatalf("focus hint index = %q; hint=%+v", got, focusHint)
	}
	if got := focusHint["source"]; got != "user_clarification" {
		t.Fatalf("focus hint source = %+v", focusHint)
	}
	if got := fmt.Sprint(args["scope"]); got != "full_project_with_focus_track" {
		t.Fatalf("scope = %q; args=%+v", got, args)
	}
}

func TestMessageLoopVocalForwardClarificationDoesNotStorePendingTick(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"needs_clarification":true,"reply":"Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_focus",
		"observation_id": "obs_focus",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Lead Vocal",
					"track_name":       "Lead Vocal",
					"user_label":       "Lead Vocal",
					"user_track_index": 1,
					"role_guess":       "vocal",
					"volume_db":        0,
					"rms_dbfs":         -18.0,
					"peak_dbfs":        -2.0,
					"headroom_db":      2.0,
				}, {
					"track_id":         "1012",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -14.0,
					"peak_dbfs":        -0.5,
					"headroom_db":      0.5,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "waiting_clarification" || !res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q", res.Status, res.NeedsClarification, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("clarification turn must not store a pending candidate: %+v", candidate)
	}
	if !strings.Contains(res.Reply, "which") && !strings.Contains(strings.ToLower(res.Reply), "track") && !strings.Contains(res.Reply, "哪条") {
		t.Fatalf("clarification reply should ask for the vocal track, got %q", res.Reply)
	}
}

func TestMessageLoopVocalForwardFinalSuggestionWithoutResolvedVocalAsksClarification(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Track 1 is masking the lead vocal. I suggest lowering Track 1 by 1.5 dB. Do you want me to continue?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_focus",
		"observation_id": "obs_focus",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
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
					"rms_dbfs":         -10.0,
					"peak_dbfs":        -0.3,
					"headroom_db":      0.3,
				}, {
					"track_id":         "1012",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -14.0,
					"peak_dbfs":        -2.5,
					"headroom_db":      2.5,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "waiting_clarification" || !res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q", res.Status, res.NeedsClarification, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("unresolved vocal target must not store a pending candidate: %+v", candidate)
	}
	if !strings.Contains(res.Reply, "哪条") {
		t.Fatalf("reply should ask which track is lead vocal, got %q", res.Reply)
	}
	if res.Continuation == nil {
		t.Fatalf("clarification should preserve continuation so the user's answer can resume the goal")
	}
}

func TestMessageLoopExtractsGainDeltaAfterTrackMention(t *testing.T) {
	delta, evidence, ok := messageLoopExtractSingleGainDelta("Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?")
	if !ok || delta != -1 || evidence != "1 dB" {
		t.Fatalf("delta=%v evidence=%q ok=%v", delta, evidence, ok)
	}
}

func TestMessageLoopTrackIDNearestToGainEvidenceIgnoresFocusMention(t *testing.T) {
	rows := []map[string]any{{
		"track_id":         "1007",
		"name":             "Lead Vocal",
		"track_name":       "Lead Vocal",
		"user_track_index": 1,
	}, {
		"track_id":         "1012",
		"name":             "Track 2",
		"track_name":       "Track 2",
		"user_track_index": 2,
	}}
	reply := "Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?"

	if got := messageLoopTrackIDNearestToEvidence(rows, reply, "1 dB"); got != "1012" {
		t.Fatalf("nearest track = %q, want 1012", got)
	}
	if got := messageLoopTrackIDMentionedOnce(rows, reply); got != "" {
		t.Fatalf("whole reply should remain ambiguous, got %q", got)
	}
}

func TestMessageLoopNaturalMixRequestDoesNotWriteAfterObservationWithoutExplicitConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先观察当前音频。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"先观察再判断"}]}`,
		`{"final":false,"reply":"观察完成，我准备把音量小幅提升 2 dB。","tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":2},"reason":"提高响度"}]}`,
		`{"final":true,"reply":"我已完成观察，建议先把这条轨道轻微提亮或增益整理；如果你确认，我再执行具体一步。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "帮我缩混选中轨道",
		AllowedTools: []string{"track.volume", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation before confirmation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "track.volume" {
			t.Fatalf("broad mix request should not write after observation without explicit confirmation: %+v", exec.calls)
		}
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "explicit user confirmation") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected post-observation confirmation gate; trace=%+v", res.Trace)
	}
}

func TestParseMessageLoopOutputRepairsBareNewlinesInReply(t *testing.T) {
	raw := "{\"final\":true,\"reply\":\"line one\nline two\",\"tool_calls\":[]}"
	out, err := parseMessageLoopOutput(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if out.Reply != "line one\nline two" {
		t.Fatalf("reply = %q", out.Reply)
	}
}

func TestMessageLoopConfirmedTrackVolumeBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe first","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"observe before action"}]}`,
		`{"final":false,"reply":"confirmed lower 1 dB","tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":-1},"reason":"lower 1 dB"}]}`,
		`{"final":true,"reply":"done","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":   "ok",
		"track_id": "1007",
		"summary":  "ready",
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "确认执行降低 1 dB",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" {
			t.Fatalf("natural-language mix tick proposal should become pending treatment before generic confirmation, calls=%+v", exec.calls)
		}
		if call.Tool == "mix.apply_tick" {
			t.Fatalf("mix tick proposal must wait for explicit pending treatment confirmation before apply, calls=%+v", exec.calls)
		}
		if call.Tool == "track.volume" {
			t.Fatalf("primitive track.volume should be wrapped through mix tick proposal, calls=%+v", exec.calls)
		}
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only observation before pending treatment", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -1 || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopImplicitLowerVolumeWithoutDBBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我准备小幅压低当前轨道。","tool_calls":[{"id":"lower_volume","tool":"track.volume","args":{"track_id":"1007"},"reason":"用户说这条轨道太响，稍微压低一点"}]}`,
		`{"final":true,"reply":"done","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"track_id":       "1007",
		"observation_id": "obs_test",
		"mix_session_id": "mix_test",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "这条轨道太响了，稍微压低一点",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	for _, call := range exec.calls {
		switch call.Tool {
		case "track.volume":
			t.Fatalf("implicit volume request must not execute primitive track.volume, calls=%+v", exec.calls)
		case "mix.propose_tick", "mix.apply_tick":
			t.Fatalf("implicit volume request should become pending treatment before executing mix tick, calls=%+v", exec.calls)
		}
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only observation before pending treatment", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaDB >= 0 || treatment.DeltaDB < -2 {
		t.Fatalf("pending treatment delta = %v, want a small negative gain move", treatment.DeltaDB)
	}
}

func TestMessageLoopImplicitLowerVolumeBecomesPendingAfterObservationPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"should not need model after deterministic observation","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"track_id":       "1007",
		"observation_id": "obs_test",
		"mix_session_id": "mix_test",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "这条轨道太响了，稍微压低一点",
		AllowedTools: []string{"mix.observe", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic gain pending should complete before another model turn, model calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaDB != -1.5 {
		t.Fatalf("pending treatment delta = %v, want -1.5", treatment.DeltaDB)
	}
	if !strings.Contains(res.Reply, "继续执行") {
		t.Fatalf("reply should ask for confirmation, got %q", res.Reply)
	}
}

func TestMessageLoopNaturalMixPrimitiveVolumeGuardBlocksDirectExecution(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先直接压低。","tool_calls":[{"id":"lower_volume","tool":"track.volume","args":{"track_id":"1007"},"reason":"用户说这条轨道太响，稍微压低一点"}]}`,
		`{"final":true,"reply":"我会先给出待确认的小幅音量建议。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "这条轨道太响了，稍微压低一点",
		AllowedTools: []string{"track.volume"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
		RecentObservation: &RecentObservation{
			Tool:        "mix.request_observation",
			CommandName: "mix_request_observation",
			Status:      "ok",
			Summary: map[string]any{
				"status":         "ok",
				"track_id":       "1007",
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
				"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("direct primitive volume write should be blocked before executor, calls=%+v", exec.calls)
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "direct track volume or pan writes") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected primitive volume guard; trace=%+v", res.Trace)
	}
}

func TestMessageLoopAmbiguousContinueDoesNotAutoApplyMixTick(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe first","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"observe before action"}]}`,
		`{"final":false,"reply":"lower it","tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":-1},"reason":"lower 1 dB"}]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":   "ok",
		"track_id": "1007",
		"summary":  "ready",
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "继续",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	foundPropose := false
	for _, call := range exec.calls {
		switch call.Tool {
		case "mix.propose_tick":
			foundPropose = true
		case "mix.apply_tick":
			t.Fatalf("ambiguous continue should not auto-apply mix tick, calls=%+v", exec.calls)
		case "track.volume":
			t.Fatalf("primitive track.volume should be wrapped through mix tick proposal, calls=%+v", exec.calls)
		}
	}
	if !foundPropose {
		t.Fatalf("expected mix.propose_tick before waiting confirmation, got %+v", exec.calls)
	}
}

func TestMessageLoopMixObservationReplyAsksForExecution(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先观察当前音频。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"先观察再判断"}]}`,
		`{"final":true,"reply":"已观察 Track 2：峰值约 -6 dBFS，RMS 约 -6.8 dBFS，动态余量还有约 6 dB。最安全的下一步是小幅提高音量，比如 +1 到 +2 dB。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "让当前轨道听感上更靠前",
		AllowedTools: []string{"track.volume", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if !strings.Contains(res.Reply, "需要我继续执行吗") {
		t.Fatalf("reply did not ask for execution confirmation: %q", res.Reply)
	}
}

func TestMessageLoopMixObservationFinalReplyStoresPendingTickCandidate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"track_id":"1007"},"reason":"observe"}]}`,
		`{"final":true,"reply":"观察完成，建议先降低当前轨道 1 dB。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"track_id":       "1007",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Vocal"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "帮我混一下当前轨道",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil {
		t.Fatalf("pending candidate missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if candidate.TrackID != "1007" || candidate.Operation != "track_gain_adjust" || candidate.DeltaDB != -1 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.ObservationID != "obs_test" || candidate.Status != "pending_confirmation" {
		t.Fatalf("candidate metadata = %+v", candidate)
	}
}

func TestMessageLoopFullProjectFinalReplyStoresPendingTickCandidateForMentionedTrack(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"整体看到了 2 条有效音频轨。Track 1 余量安全；Track 2 峰值已经到 0 dBFS，headroom 为 0 dB。建议下一步做一个很小的安全动作：把 Track 2 降低约 1.5 dB，先给工程留出一点峰值空间。要我继续执行这一步吗？","tool_calls":[]}`,
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
					"track_id":    "1012",
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
					"track_id":         "1012",
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
					"track_id": "1012",
					"name":     "Track 2",
					"rank":     1,
					"risk":     "high",
					"value":    0,
				}},
				"likely_first_attention_target": map[string]any{
					"reason": "headroom_risk",
					"track": map[string]any{
						"track_id": "1012",
						"name":     "Track 2",
						"risk":     "high",
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
		UserText:     "帮我看整体混音",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1", "volume_db": 0,
		}, {
			"track_id": "1012", "track_name": "Track 2", "volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil {
		t.Fatalf("pending candidate missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if candidate.TrackID != "1012" || candidate.Operation != "track_gain_adjust" || candidate.DeltaDB != -1.5 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.ObservationID != "obs_project" || candidate.Fingerprint["target_scope"] != "full_project" {
		t.Fatalf("candidate metadata = %+v", candidate)
	}
}

func TestMessageLoopFinalReplyStoresTreatmentPendingAndStripsMarker(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"full_project"},"reason":"observe"}]}`,
		`{"final":true,"reply":"低频有些糊，我建议先准备一个 EQ 类处理方向，确认后让 resolver 检查是否能安全执行。\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low-end mud\",\"target_ref\":\"project\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"reasoning_summary\":\"low end sounds muddy from available observation\",\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[\"target_track\",\"plugin_instance\",\"plugin_profile\",\"exact_control\"],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_project",
		"observation_id": "obs_treatment",
		"digest": map[string]any{
			"scope": "full_project",
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current"},
			"project_package": map[string]any{
				"track_count": 2,
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "低频有点糊，看看怎么调",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1",
		}, {
			"track_id": "1012", "track_name": "Track 2",
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if treatment.SchemaVersion != "mix_treatment_pending.v0" || treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.ObservationID != "obs_treatment" || treatment.Status != "pending_confirmation" {
		t.Fatalf("pending treatment metadata = %+v", treatment)
	}
}

func TestMessageLoopTreatmentPendingParsesNestedTarget(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"可以准备一个明确插件控制，确认后由 resolver 检查执行。\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low mids\",\"target_ref\":\"track:1007\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"plugin_id\":\"nova_1\",\"control\":\"control low mids with band 2\",\"target\":{\"freq_hz\":300,\"gain_db\":-1.5,\"q\":1.1},\"reasoning_summary\":\"explicit control from profile\",\"confidence\":\"high\",\"evidence_refs\":[\"profile.virtual_controls\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "低频糊，按已有 Nova 控制小调一下",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.PluginID != "nova_1" || treatment.Control != "control low mids with band 2" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["freq_hz"].(float64) != 300 || treatment.Target["gain_db"].(float64) != -1.5 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingParsesGainDelta(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Small gain move is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"bring vocal down slightly\",\"target_ref\":\"track:1007\",\"action_kind\":\"gain_balance\",\"processor_type\":\"utility\",\"delta_db\":-1.25,\"target\":{\"delta_db\":-1.25},\"reasoning_summary\":\"single explicit gain step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "bring the vocal down a little",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -1.25 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["delta_db"].(float64) != -1.25 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingParsesPanDelta(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Pan move is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move guitar left slightly\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"target\":{\"delta_pan\":-0.1},\"reasoning_summary\":\"single explicit pan step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "move the guitar left a little",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 || treatment.TargetPan != nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["delta_pan"].(float64) != -0.1 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingAcceptsPendingOnlyReply(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I can move Track 1 left after confirmation.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move current track left\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "pan the current track left",
		AllowedTools: []string{"mix.observe"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingParsesTargetPan(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Pan center is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"center the vocal\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"target\":{\"target_pan\":0},\"reasoning_summary\":\"explicit center pan step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "center the vocal",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.TargetPan == nil {
		t.Fatalf("pending treatment missing target pan; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "pan_balance" || *treatment.TargetPan != 0 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanConfirmationQuestionBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"可以。当前选中的是 Track 1，可以把它完全摆到右边（硬右声像）。不过这会让整首歌明显偏右，耳机里左边会很空。如果你确定要完全右置，我可以继续把 Track 1 设到最右。","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想把它完全摆到右边可以吗？",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if *treatment.TargetPan != 1 {
		t.Fatalf("target pan = %v, want 1", *treatment.TargetPan)
	}
}

func TestMessageLoopNaturalPanSetProposalBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Completed: Track 1 has been set hard left.","tool_calls":[{"id":"propose_pan","tool":"mix.propose_tick","args":{"operation":"track_pan_set","track_id":"1007","pan":-1},"reason":"user asked to pan Track 1 hard left"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u53ef\u4ee5\u5e2e\u6211\u628a\u8fd9\u6761\u8f68\u9053\u5b8c\u5168\u6446\u5411\u5de6\u8fb9\u5417\uff1f",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q trace=%+v", res.Status, res.StopReason, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only deterministic observation before pending confirmation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" {
			t.Fatalf("mix tick tools must not reach executor before pending confirmation, calls=%+v", exec.calls)
		}
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != -1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(strings.ToLower(res.Reply), "completed") || strings.Contains(strings.ToLower(res.Reply), "has been set") {
		t.Fatalf("reply should not claim execution before confirmation: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanRevisionProposalBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Completed: Track 1 has been set hard right.","tool_calls":[{"id":"propose_pan","tool":"mix.propose_tick","args":{"operation":"track_pan_set","track_id":"1007","target_pan":1},"reason":"user revised pending pan target hard right"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "彻底往右",
		AllowedTools: []string{"mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone || res.Continuation != nil {
		t.Fatalf("result = status=%q stop=%q continuation=%+v reply=%q trace=%+v", res.Status, res.StopReason, res.Continuation, res.Reply, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("mix proposal should be converted to pending treatment before executor, calls=%+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(strings.ToLower(res.Reply), "completed") || strings.Contains(strings.ToLower(res.Reply), "has been set") {
		t.Fatalf("reply should not claim execution before confirmation: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanRevisionPrimitivePanBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will set it right now.","tool_calls":[{"id":"set_pan","tool":"track.pan","args":{"track_id":"1007","pan":1},"reason":"user asked for hard right pan"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "彻底摆到右边去",
		AllowedTools: []string{"track.pan", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone || res.Continuation != nil {
		t.Fatalf("result = status=%q stop=%q continuation=%+v reply=%q trace=%+v", res.Status, res.StopReason, res.Continuation, res.Reply, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("primitive pan should be converted to pending treatment before executor, calls=%+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopPanClarificationBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"needs_clarification":true,"reply":"可以。当前只有 Track 1，所以“它”应该是这条轨道；但完全摆到右边是比较极端的声像设置。你要我现在把 Track 1 硬声像到最右边吗？","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想把它完全摆到右边可以吗？",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
		},
	})

	if res.Status != "completed" || res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q trace=%+v", res.Status, res.NeedsClarification, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("clarification-to-pending must not execute tools, got %+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopPendingTreatmentDoesNotDuplicateExecutionQuestion(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"更安全的第一步：先把 Track 1 轻微往右摆一点，比如 pan +0.10。要我现在执行吗？\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"small right pan\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":0.1,\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想把它完全摆到右边可以吗？",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if strings.Count(res.Reply, "执行吗") != 1 || strings.Contains(res.Reply, "需要我继续执行吗") {
		t.Fatalf("reply duplicated execution prompt: %q", res.Reply)
	}
}

func TestMessageLoopPanPendingUsesExplicitUserTargetOverSaferSuggestion(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"更安全的第一步：先把 Track 1 轻微往右摆一点，比如 pan +0.10。要我先执行这个小幅右移吗？\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"small right pan\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":0.1,\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想把它完全摆到右边可以吗？",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaPan != 0 || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v, want target_pan=1 from user text", treatment)
	}
}

func TestMessageLoopTreatmentPendingInfersPanPlacementTargets(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
	}{
		{name: "plain left placement", text: "把吉他摆到左边", want: -0.5},
		{name: "left percent placement", text: "pan the guitar 70% left", want: -0.7},
		{name: "hard left", text: "pan the guitar hard left", want: -1},
		{name: "hard right", text: "pan the guitar hard right", want: 1},
		{name: "fully right chinese", text: "把它完全摆到右边", want: 1},
		{name: "right placement", text: "把吉他靠右", want: 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, target := messageLoopTreatmentPanFromReply(tt.text)
			if delta != 0 || target == nil {
				t.Fatalf("pan parse = delta=%v target=%v", delta, target)
			}
			if *target != tt.want {
				t.Fatalf("target pan = %v, want %v", *target, tt.want)
			}
		})
	}
}

func TestMessageLoopTreatmentPendingKeepsSmallPanMovesAsDeltas(t *testing.T) {
	tests := []struct {
		text string
		want float64
	}{
		{text: "move the guitar left a little", want: -0.1},
		{text: "pan the guitar 10% right a little", want: 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			delta, target := messageLoopTreatmentPanFromReply(tt.text)
			if target != nil || delta != tt.want {
				t.Fatalf("pan parse = delta=%v target=%v, want delta %v", delta, target, tt.want)
			}
		})
	}
}

func TestMessageLoopTreatmentPendingInfersGainDeltaFromSuggestedMove(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Track 1 RMS is about -9.03 dBFS, peak is -6.02 dBFS, and headroom is about 6 dB. Suggested first step: lower Track 2 by 2 dB to create headroom.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce peak risk\",\"target_ref\":\"track:1010\",\"action_kind\":\"gain_balance\",\"processor_type\":\"utility\",\"reasoning_summary\":\"single explicit gain step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "help me check the overall mix",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -2 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopUnavailableMixObservationDoesNotUnlockPluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先观察当前音频。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"}}]}`,
		`{"final":false,"reply":"观察完了，准备加载 EQ。","tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"当前观察结果不可用，我不会加载插件；需要先补足可用观察或让你确认具体插件操作。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status": "unavailable",
		"mixboard": map[string]any{
			"status":        "unavailable",
			"open_blockers": []any{"audio_feature_request_blocked"},
			"package_status": map[string]any{
				"mix": "limited",
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "我想你帮我对这段音频进行缩混可以吗？",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only unavailable observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "plugin.load_to_rack" {
			t.Fatalf("plugin load should remain blocked after unavailable observation: %+v", exec.calls)
		}
	}
}

func TestMessageLoopObservationPackageQuestionRoutesPluginParamsToMixRead(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我查看 observation 数据包。","tool_calls":[{"id":"read_params","tool":"plugin.get_parameters","args":{"track_id":"1007","plugin_id":"plugin_1"},"reason":"查看数据包"}]}`,
		`{"final":true,"reply":"我读取的是 MixBoard 声学观察包，不是插件参数。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "observation 现在看不见声学数据包了，帮我看一下为什么",
		AllowedTools: []string{"plugin.get_parameters", "mix.read"},
		RecentObservation: &RecentObservation{
			Tool:        "mix.observe",
			CommandName: "mix_request_observation",
			Status:      "ok",
			Summary: map[string]any{
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
			},
		},
		ExecutionMemory: ExecutionMemory{ActiveWorkTargetTrackID: "1007"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.read" {
		t.Fatalf("executor calls = %+v, want mix.read", exec.calls)
	}
	keys := fmt.Sprint(exec.calls[0].Args["keys"])
	for _, want := range []string{"observation.digest", "track.1007.fast.levels", "track.1007.slow.time_energy.summary"} {
		if !strings.Contains(keys, want) {
			t.Fatalf("mix.read keys missing %s: %+v", want, exec.calls[0].Args)
		}
	}
	if fmt.Sprint(exec.calls[0].Args["plugin_id"]) != "" && fmt.Sprint(exec.calls[0].Args["plugin_id"]) != "<nil>" {
		t.Fatalf("plugin args leaked into mix.read: %+v", exec.calls[0].Args)
	}
}

func TestMessageLoopNaturalMixRequestBlocksDirectWaveformBake(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先准备波形。","tool_calls":[{"id":"bake","tool":"clip.warm_waveform_bake","args":{"track_id":"1007"},"reason":"准备观察"}]}`,
		`{"final":false,"reply":"我改用混音观察。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"统一观察"}]}`,
		`{"final":true,"reply":"已经完成观察。我会先基于观察给出建议，不会直接加载插件。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "帮我缩混这段音频",
		AllowedTools: []string{"clip.warm_waveform_bake", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) == 0 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want first call to be mix observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.warm_waveform_bake" {
			t.Fatalf("direct waveform bake should be blocked before executor: %+v", exec.calls)
		}
	}
	foundRewrite := false
	for _, event := range res.Trace {
		if event.Kind == "tool_call_rewritten" && strings.Contains(event.Message, "mix.observe") {
			foundRewrite = true
			break
		}
	}
	if !foundRewrite {
		t.Fatalf("expected waveform bake rewrite; trace=%+v", res.Trace)
	}
}

func TestMessageLoopAudioObservationRequestCoercesWaveformBake(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先准备波形。","tool_calls":[{"id":"bake","tool":"clip.warm_waveform_bake","args":{"track_id":"1007"},"reason":"准备声学观察"}]}`,
		`{"final":true,"reply":"已通过混音观察工具完成声学观察，没有直接调用低层波形缓存工具。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "需要你继续做音频观察分析",
		AllowedTools: []string{"mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v, want one coerced observation", exec.calls)
	}
	if !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor call = %+v, want mix observation", exec.calls[0])
	}
	if exec.calls[0].ID != "observe_mix" {
		t.Fatalf("coerced call id = %q, want observe_mix", exec.calls[0].ID)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.warm_waveform_bake" {
			t.Fatalf("direct waveform bake reached executor: %+v", exec.calls)
		}
	}
}

func TestMessageLoopExplicitPluginLoadDoesNotRequireMixObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"正在加载。","tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"TDR Nova 已加载。"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "请直接加载 TDR Nova 插件",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "plugin.load_to_rack" {
		t.Fatalf("executor calls = %+v, want plugin load", exec.calls)
	}
}

func TestMessageLoopNaturalMixConfirmationObservesBeforePendingPluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先观察当前音频。","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"先观察再判断"}]}`,
		`{"final":true,"reply":"已完成观察，先不加载插件。"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}
	pending := planner.ToolCall{
		ID:   "load_eq",
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"track_id": "1007", "plugin_query": "TDR Nova", "zone_id": "Z3"},
	}

	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:          "goal_test",
		RunID:           "run_test",
		UserText:        "帮我缩混当前轨道",
		AllowedTools:    []string{"plugin.load_to_rack", "mix.request_observation"},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "plugin.load_to_rack" {
			t.Fatalf("pending plugin load was executed: %+v", exec.calls)
		}
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "mix.observe") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected observe-first gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopMixObservationSummaryIncludesStructAcousticPackages(t *testing.T) {
	observation := mixboard.ObservationPacket{
		Status:        "ready",
		MixSessionID:  "mix_test",
		ObservationID: "obs_test",
		TargetRef: mixboard.TargetRef{
			Kind:  "track",
			ID:    "1007",
			Label: "Track 1",
		},
		TimeRuler: mixboard.TimeRuler{
			DurationSeconds: 219.384,
			SegmentSeconds:  2,
			FrameSeconds:    0.01,
		},
		MixPackage: map[string]any{
			"status": "baseline_ready",
			"current_metrics": map[string]any{
				"waveform": map[string]any{
					"status":      "ready",
					"peak_dbfs":   -0.109,
					"rms_dbfs":    -17.543,
					"headroom_db": 0.109,
					"crest_db":    17.434,
				},
				"time_energy": []map[string]any{
					{"start_seconds": 0.0, "end_seconds": 5.0, "rms_dbfs": -18.1, "peak_dbfs": -1.2},
				},
			},
			"source_capabilities": map[string]string{
				"waveform_envelope": "ready",
				"time_energy":       "ready",
			},
		},
	}
	result := map[string]any{
		"status":           "ready",
		"mix_session_id":   "mix_test",
		"observation_id":   "obs_test",
		"observation_path": "obs_test.json",
		"observation":      observation,
		"context_pack": mixboard.ContextPack{
			MixSessionID:      "mix_test",
			LatestObservation: mustMessageLoopMap(t, observation),
		},
	}

	summary := mixObservationPromptSummary(result)
	data, _ := json.Marshal(summary)
	text := string(data)
	for _, want := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "time_energy", "baseline_ready"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q: %s", want, text)
		}
	}
}

func mustMessageLoopMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestMessageLoopPlanModePromptNamesReadOnlyMode(t *testing.T) {
	prompt := messageLoopSystemPrompt(&runState{
		input: Input{
			Context:      map[string]any{"agent_mode": "plan"},
			AllowedTools: []string{"track.list", "plugin.search"},
		},
	})
	for _, want := range []string{
		"Plan mode",
		"read-only",
		"Do not say the DAW lacks that capability",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("plan prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestParseMessageLoopOutputAcceptsFencedJSON(t *testing.T) {
	out, err := parseMessageLoopOutput("```json\n{\"final\":true,\"reply\":\"完成\",\"tool_calls\":[]}\n```")
	if err != nil {
		t.Fatalf("parseMessageLoopOutput: %v", err)
	}
	if !out.Final || out.Reply != "完成" {
		t.Fatalf("out = %+v", out)
	}
}

func TestParseMessageLoopOutputExtractsBalancedJSONObject(t *testing.T) {
	out, err := parseMessageLoopOutput("我会执行：\n{\"final\":false,\"reply\":\"准备写入\",\"tool_calls\":[{\"tool\":\"midi.apply_note_patch\",\"args\":{\"clip_id\":\"clip_a\"},\"reason\":\"写入 MIDI\"}]}\n请确认。")
	if err != nil {
		t.Fatalf("parseMessageLoopOutput: %v", err)
	}
	if out.Final || len(out.ToolCalls) != 1 || out.ToolCalls[0].Tool != "midi.apply_note_patch" {
		t.Fatalf("out = %+v", out)
	}
}

func TestMessageLoopRepairsInvalidJSONOnce(t *testing.T) {
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", "off")
	client := &fakeMessageCompleter{responses: []string{
		`{"final": false, "reply": "准备执行", "tool_calls": [`,
		`{"final":true,"reply":"已恢复为合法计划。","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{UserText: "测试 JSON 修复"})
	if res.Status != "completed" || res.Reply != "已恢复为合法计划。" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want initial + repair", len(client.calls))
	}
	if got := client.calls[1][0].Role; got != "system" {
		t.Fatalf("repair prompt first role = %q", got)
	}
}
