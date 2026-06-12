package agentloop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
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
