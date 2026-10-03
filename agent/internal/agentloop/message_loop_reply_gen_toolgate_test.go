package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/planner"
)

// REPLY-GEN-TOOLGATE-1（2026-10-03 19:37 真机手测，webui_musbfgki /
// goal_05439ab7638762de / turn_d4f40501dcfdf594）：同一会话一轮判定结算后
// 发观察问句「再帮我看看bass轨道的低频有没有什么问题」，观察工具正常执行
// 完成，终局回复生成段死亡——error 节点 n_20261003T113752 报
// 「前面的工具操作已完成，但最终回复生成失败：未知或不允许的工具：」
// 冒号后为空。agent_message_loop_debug.jsonl 实锚两段原始输出：
//
//   - parse_failed（19:37:50）：模型输出 DSML 形态调用（非 JSON）；
//   - repair_succeeded（19:37:52）：修复 pass 返回
//     {"tool_calls":[{"name":"mix.read","arguments":{...}}]} —— OpenAI 风格
//     name/arguments 字段名。解码器只认 tool/args，name 丢失成空名调用，
//     准入连拒两次（MaxConsecutiveErrors=2）→ r.fail(空名报错)。
//
// 本文件钉住修复后行为：别名字段照常解码执行（根因修复），真·空名调用
// 的失败面收口到该轮回复（物化观察摘要 + 含因诊断），不再中断链。

// 现场问句（逐字）。
const replyGenToolgateUserText = "再帮我看看bass轨道的低频有没有什么问题"

// 现场 parse_failed 轮的 DSML 原始输出（逐字节选，保持现场形态）。
const replyGenToolgateDSMLRaw = `<｜｜DSML｜｜ calls>
<｜｜DSML｜｜ invoke name="mix.read">
<｜｜DSML｜｜ parameter name="observation_id" string="true">obs_20261003T113742_73fc5852fc1d</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="keys" string="false">["track.current.slow.band_energy.summary", "track.current.slow.time_energy.summary", "track.current.fast.levels", "project.relationship_inputs", "project.limitations"]</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="detail" string="true">summary</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="max_items" string="false">12</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
<｜｜DSML｜｜ invoke name="mix.derive">
<｜｜DSML｜｜ parameter name="observation_id" string="true">obs_20261003T113742_73fc5852fc1d</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="type" string="true">a_vs_b</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="a" string="false">{"kind":"track","id":"1007","label":"bass"}</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="b" string="false">{"kind":"track","id":"1012","label":"drums"}</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="dimensions" string="false">["sub", "bass", "low_mid", "headroom", "time_overlap"]</｜｜DSML｜｜ parameter>
<｜｜DSML｜｜ parameter name="max_items" string="false">12</｜｜DSML｜｜ parameter>
</｜｜DSML｜｜ invoke>
</｜｜DSML｜｜ calls>`

// 现场 repair_succeeded 轮的返回（逐字）：tool_calls 用 name/arguments 字段名。
const replyGenToolgateAliasRepairJSON = `{"final":false,"tool_calls":[{"name":"mix.read","arguments":{"observation_id":"obs_20261003T113742_73fc5852fc1d","keys":["track.current.slow.band_energy.summary","track.current.slow.time_energy.summary","track.current.fast.levels","project.relationship_inputs","project.limitations"],"detail":"summary","max_items":12}},{"name":"mix.derive","arguments":{"observation_id":"obs_20261003T113742_73fc5852fc1d","type":"a_vs_b","a":{"kind":"track","id":"1007","label":"bass"},"b":{"kind":"track","id":"1012","label":"drums"},"dimensions":["sub","bass","low_mid","headroom","time_overlap"],"max_items":12}}]}`

func replyGenToolgateObservationResult() map[string]any {
	return map[string]any{
		"status":         "ok",
		"scope":          "full_project",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"acoustic_digest": map[string]any{
			"waveform": map[string]any{
				"status":      "ready",
				"peak_dbfs":   -6.0,
				"rms_dbfs":    -9.0,
				"headroom_db": 6.0,
			},
			"source_capabilities": map[string]any{
				"waveform_envelope": "ready",
				"band_energy":       "ready",
				"stereo_relation":   "ready",
				"lufs_analysis":     "deferred",
				"masking_analysis":  "deferred",
				"reference_match":   "deferred",
			},
		},
		"observation": map[string]any{
			"status": "ready",
			"project_package": map[string]any{
				"tracks": []map[string]any{
					{"track_id": "1007", "track_name": "bass", "peak_dbfs": -12.0, "rms_dbfs": -18.0, "headroom_db": 12.0},
					{"track_id": "1010", "track_name": "Track 2", "peak_dbfs": 0.0, "rms_dbfs": -10.6, "headroom_db": 0.0},
				},
			},
			"headroom_risk": []map[string]any{
				{"track_id": "1010", "track_name": "Track 2", "peak_dbfs": 0.0, "rms_dbfs": -10.6, "headroom_db": 0.0, "risk": "high"},
			},
		},
	}
}

// 钉①（现场主链，修复前 RED）：观察完成后的终局段——模型先输出 DSML（解析
// 失败进修复），修复返回 name/arguments 字段名的 tool_calls。修复前两条调用
// 均解码为空名，准入连拒耗尽连续错误预算，整轮死于空名报错
// 「未知或不允许的工具：」（冒号后为空、链中断）。修复后别名字段照常解码，
// mix.read / mix.derive 真实执行，模型终局回复正常落盘。
func TestMessageLoopPostObservationAliasToolCallRepairSurvivesFinalReply(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		replyGenToolgateDSMLRaw,
		replyGenToolgateAliasRepairJSON,
		`{"final":true,"reply":"bass 轨低频检查完成：低频段能量与整体余量在正常范围内，未发现明显问题。","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: replyGenToolgateObservationResult()}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     replyGenToolgateUserText,
		AllowedTools: []string{"mix.observe", "mix.request_observation", "mix.read", "mix.derive"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.Error != "" {
		t.Fatalf("the alias repair path must not surface a failure: error=%q", res.Error)
	}
	ranMixRead, ranMixDerive := false, false
	for _, call := range exec.calls {
		switch strings.TrimSpace(call.Tool) {
		case "mix.read":
			ranMixRead = true
		case "mix.derive":
			ranMixDerive = true
		}
	}
	if !ranMixRead || !ranMixDerive {
		t.Fatalf("alias-decoded calls must execute: calls=%+v", exec.calls)
	}
	if !strings.Contains(res.Reply, "低频检查完成") {
		t.Fatalf("final reply missing the model summary: %q", res.Reply)
	}
}

// 钉②（真·空名调用的失败面收口，修复前 RED）：模型在观察完成后连续发出
// 连工具名都没有的调用（tool 与 name 别名均为空）。修复前整轮死于
// 「未知或不允许的工具：」空名报错（链中断、无诊断）。修复后：
// 准入报错明示缺名原因，连续错误耗尽时收口为该轮回复——已采集观察的
// 物化摘要 + 含诊断附注（不静默吞），不再中断链。
func TestMessageLoopNamelessToolCallExhaustionSettlesWithMaterializedObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"tool_calls":[{"args":{"observation_id":"obs_test"}},{"args":{"type":"a_vs_b"}}]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: replyGenToolgateObservationResult()}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     replyGenToolgateUserText,
		AllowedTools: []string{"mix.observe", "mix.request_observation", "mix.read", "mix.derive"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.Error != "" {
		t.Fatalf("admission exhaustion must be contained into the round reply, not a failure: error=%q", res.Error)
	}
	if !strings.Contains(res.Reply, "Track 2") {
		t.Fatalf("reply missing the materialized observation digest:\n%s", res.Reply)
	}
	if !strings.Contains(res.Reply, "缺少工具名") {
		t.Fatalf("reply must keep the admission diagnosis visible (不静默吞):\n%s", res.Reply)
	}
	foundFallbackTrace := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "materialized observation fallback") {
			foundFallbackTrace = true
			break
		}
	}
	if !foundFallbackTrace {
		t.Fatalf("fallback trace marker missing: %+v", res.Trace)
	}
}

// 钉③（报错可诊断性）：非空未知工具名照常带名报错；空名报错必须明示缺名
// 原因，不允许再出现「未知或不允许的工具：」后接空串的形态。
func TestToolAdmissionErrorNamesTheTool(t *testing.T) {
	named := toolAdmissionError(planner.ToolCall{Tool: " not.real "})
	if !strings.Contains(named, "not.real") || !strings.HasPrefix(named, "未知或不允许的工具：") {
		t.Fatalf("named rejection = %q", named)
	}
	nameless := toolAdmissionError(planner.ToolCall{Args: map[string]any{"k": "v"}})
	if strings.HasSuffix(strings.TrimSpace(nameless), "：") {
		t.Fatalf("nameless rejection must not end with a bare colon: %q", nameless)
	}
	if !strings.Contains(nameless, "缺少工具名") {
		t.Fatalf("nameless rejection must state the missing-name cause: %q", nameless)
	}
}
