package agentloop

// HYGIENE-FASTPATH-1 挂账②：fastpath 注册面漂移守卫从"轮中 panic"改为"显式
// run 失败"后的守卫测试面。
//
// 三层断言：
//  1. fastPathRegistrationDrift 判定面单测——遗漏/多余/乱序均返回显式错误
//     （错误消息含实际与期望两份词条清单），recover 探针证明无 panic 逃逸；
//  2. 正常路径零变化——无漂移时 newFastPathRouter 返回的注册面与契约逐项
//     恒等（既有行为的回归锚）；
//  3. 端到端——临时扰动 fastpath.DefaultEntryNames 使真实构造面漂移，
//     Start() 必须显式失败（fastpath_drift trace + fail 结果 + LLM 零调用），
//     而不是 panic 崩进程。
//
// 真实栈覆盖=无（纯单测域；行为红线=无漂移路径零变化，由既有 agentloop/fastpath
// 全量测试零改动全绿背书）。

import (
	"context"
	"slices"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/fastpath"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

// withDriftedDefaultEntryNames 临时把 fastpath.DefaultEntryNames 换成漂移清单
// （头部插入一个未知词条），defer 恢复原清单。包内测试串行（无 t.Parallel），
// 变量替换安全。
func withDriftedDefaultEntryNames(t *testing.T) {
	t.Helper()
	original := slices.Clone(fastpath.DefaultEntryNames)
	t.Cleanup(func() { fastpath.DefaultEntryNames = original })
	fastpath.DefaultEntryNames = append([]string{"__drift_probe_unknown_entry__"}, original...)
}

func TestFastPathRegistrationDriftDetectsListDeviations(t *testing.T) {
	canonical := fastpath.DefaultEntryNames

	if err := fastPathRegistrationDrift(canonical); err != nil {
		t.Fatalf("canonical registration must pass drift check, got error: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func([]string) []string
		wantSub string
	}{
		{
			name:    "missing entry",
			mutate:  func(names []string) []string { return slices.Delete(slices.Clone(names), 3, 4) },
			wantSub: "clip_fade_gain_read",
		},
		{
			name: "wrong order",
			mutate: func(names []string) []string {
				swapped := slices.Clone(names)
				swapped[0], swapped[1] = swapped[1], swapped[0]
				return swapped
			},
			wantSub: "project_blackboard_status",
		},
		{
			name:    "extra entry",
			mutate:  func(names []string) []string { return append(slices.Clone(names), "surprise_entry") },
			wantSub: "surprise_entry",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drifted := fastpath.NewRouter[*Runner, *runState, Result]()
			for _, name := range tc.mutate(canonical) {
				drifted.Register(fastpath.Entry[*Runner, *runState, Result]{Name: name})
			}
			// recover 探针：原实现此处 panic 崩进程（L1-5-IMPL-C ruling 挂账②
			// 明文轮中 panic 不可）；现契约=显式错误返回。
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("drift check must return an explicit error, not panic: %v", rec)
				}
			}()
			err := fastPathRegistrationDrift(drifted.Names())
			if err == nil {
				t.Fatalf("drifted registration (%s) must fail the completeness contract", tc.name)
			}
			if !strings.Contains(err.Error(), "fastpath 注册面漂移") {
				t.Fatalf("drift error missing fail-visible keyword: %q", err.Error())
			}
			if !strings.Contains(err.Error(), tc.wantSub) || !strings.Contains(err.Error(), "clip_fade_gain_set") {
				t.Fatalf("drift error must print both entry lists (got=%q)", err.Error())
			}
			if !strings.Contains(err.Error(), "want=") {
				t.Fatalf("drift error must print the contract list (want=…): %q", err.Error())
			}
		})
	}
}

func TestNewFastPathRouterCanonicalRegistrationSucceeds(t *testing.T) {
	// 行为红线回归锚：正常路径（无漂移）零变化——构造成功且注册面与契约逐项恒等。
	router, err := (&MessageLoop{}).newFastPathRouter()
	if err != nil {
		t.Fatalf("canonical registration must not drift, got error: %v", err)
	}
	if router == nil {
		t.Fatalf("canonical registration returned nil router without error")
	}
	names := router.Names()
	if len(names) != len(fastpath.DefaultEntryNames) {
		t.Fatalf("registered names length=%d, want=%d", len(names), len(fastpath.DefaultEntryNames))
	}
	for i, name := range names {
		if name != fastpath.DefaultEntryNames[i] {
			t.Fatalf("registered names[%d]=%q, want=%q（注册序=原 loop() preflight 链调用序契约）", i, name, fastpath.DefaultEntryNames[i])
		}
	}
}

func TestMessageLoopFastPathDriftFailsRunExplicitlyWithoutPanic(t *testing.T) {
	withDriftedDefaultEntryNames(t)
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"unexpected"}`}}
	loop := &MessageLoop{
		Runtime: loopTestRuntime(),
		Client:  client,
		Config:  config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget:  Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	result := loop.Start(context.Background(), Input{
		GoalID:   "goal_fastpath_drift_probe",
		RunID:    "run_fastpath_drift_probe",
		UserText: "run the fastpath registration drift probe",
		Summary:  "run the fastpath registration drift probe",
		Context:  map[string]any{"initialized": true},
	})

	if result.Status != agentruntime.StatusFailed {
		t.Fatalf("drift run must fail explicitly, got status=%q stop=%q reply=%q", result.Status, result.StopReason, result.Reply)
	}
	if !strings.Contains(result.Error, "fastpath 注册面漂移") || !strings.Contains(result.Error, "want=") {
		t.Fatalf("drift failure must carry the fail-visible registration lists: %q", result.Error)
	}
	driftSeen := false
	for _, event := range result.Trace {
		if event.Kind == "fastpath_drift" && strings.Contains(event.Message, "fastpath 注册面漂移") {
			driftSeen = true
			break
		}
	}
	if !driftSeen {
		t.Fatalf("trace must carry a fastpath_drift event; trace kinds=%v", traceKinds(result.Trace))
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM must never be called on registration drift, got %d calls", len(client.calls))
	}
}

func traceKinds(trace []planner.TraceEvent) []string {
	kinds := make([]string, 0, len(trace))
	for _, event := range trace {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}
