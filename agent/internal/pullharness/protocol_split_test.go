package pullharness

// protocol_split_test.go — G3-ATTRIB-2 协议段拆分修复的驱动面回归
//（G3-RULING §2.4 登记项）：宿主双段供给（字节稳定骨架 stable=true +
// 逐轮指令块 stable=false/动态区）后——
//   - 族内连续轮稳定段字节恒等（P1 机械判定 starts-with 恒 true，层
//     content_hash 恒等，零 ruleset_changed）；
//   - 族切换（骨架字节变化）保留为合法 ruleset_changed 断裂，P1 判 false；
//   - 人为扰动稳定段被检出（负例：非追加字节变化 → 断裂+P1 false）；
//   - 指令块变化只落动态区，不断前缀。
// 断言经真实 promptruntime.PrefixService（recordingPrefix 包装），不走
// fakePrefix——判定面与生产同源。

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/promptruntime"
)

// recordingPrefix 包装真实 PrefixService，逐轮捕获 AssemblyReport
// （生产判定面：断裂归因+P1 供给字段）。
type recordingPrefix struct {
	inner   promptruntime.PrefixService
	reports []promptruntime.AssemblyReport
}

func newRecordingPrefix() *recordingPrefix {
	return &recordingPrefix{inner: promptruntime.NewPrefixService()}
}

func (r *recordingPrefix) Assemble(ctx context.Context, req promptruntime.PrefixRequest) (promptruntime.Assembly, promptruntime.AssemblyReport, error) {
	assembly, report, err := r.inner.Assemble(ctx, req)
	if err == nil {
		r.reports = append(r.reports, report)
	}
	return assembly, report, err
}

// protocolLayer 取报告中协议骨架层的层报告；缺失则失败。
func protocolLayer(t *testing.T, report promptruntime.AssemblyReport) promptruntime.LayerReport {
	t.Helper()
	for _, layer := range report.Layers {
		if layer.LayerID == protocolSkeletonSectionID {
			return layer
		}
	}
	t.Fatalf("protocol skeleton layer %q missing from report layers %+v", protocolSkeletonSectionID, report.Layers)
	return promptruntime.LayerReport{}
}

func rulesetBreaks(report promptruntime.AssemblyReport) []promptruntime.BreakEvent {
	out := []promptruntime.BreakEvent{}
	for _, event := range report.Breaks {
		if event.Reason == promptruntime.BreakRulesetChanged {
			out = append(out, event)
		}
	}
	return out
}

// TestPullLoopProtocolSplitMountsStableSkeletonAndDynamicDirectives：双段
// 挂载面——骨架进稳定 system 段（SectionStatic stable=true），指令块进动态
// 区 user 段（stable=false）；system 消息不含指令块，动态区消息含之。
func TestPullLoopProtocolSplitMountsStableSkeletonAndDynamicDirectives(t *testing.T) {
	prefix := &fakePrefix{}
	session := &fakeSession{runID: "run-split", goalID: "goal-1", prefixKey: "pullharness:run-split"}
	session.protocolSkeleton = "STABLE SKELETON: fixed rule frame"
	session.protocolDirectives = "DIRECTIVES: allowed tools a,b,c"
	llmFake := &fakeLLM{responses: []string{"final answer"}}
	loop := newSessionLoop(session, llmFake, prefix, &fakeExit{})
	result := loop.Run(context.Background(), goalInput("run-split"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q (result=%+v)", result.Outcome, result)
	}
	var skeletonSection, directivesSection *promptruntime.Section
	for index := range prefix.last.SystemSections {
		if prefix.last.SystemSections[index].ID == protocolSkeletonSectionID {
			skeletonSection = &prefix.last.SystemSections[index]
		}
	}
	for index := range prefix.last.UserSections {
		if prefix.last.UserSections[index].ID == protocolDirectivesSectionID {
			directivesSection = &prefix.last.UserSections[index]
		}
	}
	if skeletonSection == nil || !skeletonSection.Stable || skeletonSection.Kind != promptruntime.SectionStatic {
		t.Fatalf("skeleton must mount as stable static system section, got %+v", skeletonSection)
	}
	if directivesSection == nil || directivesSection.Stable || directivesSection.Kind != promptruntime.SectionRuntime {
		t.Fatalf("directives must mount as non-stable runtime user section, got %+v", directivesSection)
	}
	systemMsg, userMsg := "", ""
	for _, message := range llmFake.gotMsgs[0] {
		switch strings.ToLower(message.Role) {
		case "system":
			systemMsg = message.Content
		case "user":
			userMsg = message.Content
		}
	}
	if !strings.Contains(systemMsg, "STABLE SKELETON") {
		t.Error("skeleton missing from system message")
	}
	if strings.Contains(systemMsg, "DIRECTIVES") {
		t.Error("per-turn directives must not ride the stable system message (that was the L1-5-D wiring defect)")
	}
	if !strings.Contains(userMsg, "DIRECTIVES") {
		t.Error("directives missing from dynamic-zone user message")
	}
}

// TestPullLoopProtocolSplitFamilyInternalTurnsStablePrefix：族内连续模型轮
// （指令块逐轮变化、骨架恒定）——协议层 content_hash 恒等、P1 机械判定
// starts-with 恒 true、PrefixContentHash 恒等、零 ruleset_changed 断裂。
func TestPullLoopProtocolSplitFamilyInternalTurnsStablePrefix(t *testing.T) {
	prefix := newRecordingPrefix()
	session := &fakeSession{runID: "run-family", goalID: "goal-1", prefixKey: "pullharness:run-family"}
	session.protocolSkeleton = "STABLE SKELETON: fixed rule frame"
	session.directivesFor = func(snapshot int) string {
		return fmt.Sprintf("DIRECTIVES turn %d: allowed tools a,b,c; autonomy block state=%d%s",
			snapshot, snapshot, strings.Repeat("|", snapshot))
	}
	llmFake := &fakeLLM{responses: []string{
		"call:track.list:receipt-1", // 模型批 → CloseCycle → 下一轮装配
		"final done",
	}}
	loop := newSessionLoop(session, llmFake, prefix, &fakeExit{})
	result := loop.Run(context.Background(), goalInput("run-family"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q (result=%+v)", result.Outcome, result)
	}
	if len(prefix.reports) != 2 {
		t.Fatalf("expected 2 assemblies (one per model turn), got %d", len(prefix.reports))
	}
	first, second := prefix.reports[0], prefix.reports[1]
	firstLayer, secondLayer := protocolLayer(t, first), protocolLayer(t, second)
	if firstLayer.ContentHash != secondLayer.ContentHash {
		t.Fatalf("family-internal protocol skeleton drifted: %s -> %s", firstLayer.ContentHash, secondLayer.ContentHash)
	}
	if first.PrefixContentHash != second.PrefixContentHash {
		t.Fatalf("family-internal stable prefix bytes drifted: %s -> %s", first.PrefixContentHash, second.PrefixContentHash)
	}
	for index, report := range prefix.reports {
		if got := rulesetBreaks(report); len(got) != 0 {
			t.Fatalf("turn %d: family-internal turn must not break the prefix, got %+v", index+1, got)
		}
	}
	if second.PrefixStartsWithPrevious == nil || !*second.PrefixStartsWithPrevious {
		t.Fatalf("P1 mechanical judgment must hold within a family, got %+v", second.PrefixStartsWithPrevious)
	}
	if second.DynamicBytes == first.DynamicBytes {
		t.Fatal("directives vary per turn — dynamic bytes should track them")
	}
}

// TestPullLoopProtocolSplitFamilySwitchIsLegalBreak：骨架字节变化（普通↔
// 中性族切换同型）=合法 ruleset_changed 断裂：P1 判 false、断裂归因指向
// pullharness.protocol 层（真实规则变化，非接线缺陷）。
func TestPullLoopProtocolSplitFamilySwitchIsLegalBreak(t *testing.T) {
	prefix := newRecordingPrefix()
	session := &fakeSession{runID: "run-switch", goalID: "goal-1", prefixKey: "pullharness:run-switch"}
	session.skeletonFor = func(snapshot int) string {
		if snapshot == 1 {
			return "ORDINARY FAMILY SKELETON: rule frame A"
		}
		return "NEUTRAL FAMILY SKELETON: rule frame B" // 族切换（真实规则变化）
	}
	session.protocolDirectives = "DIRECTIVES: allowed tools a,b"
	llmFake := &fakeLLM{responses: []string{"call:track.list:receipt-1", "final done"}}
	loop := newSessionLoop(session, llmFake, prefix, &fakeExit{})
	if result := loop.Run(context.Background(), goalInput("run-switch")); result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q (result=%+v)", result.Outcome, result)
	}
	if len(prefix.reports) != 2 {
		t.Fatalf("expected 2 assemblies, got %d", len(prefix.reports))
	}
	second := prefix.reports[1]
	if protocolLayer(t, prefix.reports[0]).ContentHash == protocolLayer(t, second).ContentHash {
		t.Fatal("family switch must change the skeleton layer bytes")
	}
	if second.PrefixStartsWithPrevious == nil || *second.PrefixStartsWithPrevious {
		t.Fatalf("family switch must fail the P1 starts-with judgment, got %+v", second.PrefixStartsWithPrevious)
	}
	breaks := rulesetBreaks(second)
	if len(breaks) != 1 || breaks[0].LayerID != protocolSkeletonSectionID {
		t.Fatalf("family switch must surface exactly one ruleset_changed on %s, got %+v", protocolSkeletonSectionID, breaks)
	}
}

// TestPullLoopProtocolSplitPerturbedSkeletonDetected（负例）：稳定段被人为
// 中部扰动（非追加、非族切换）——断裂归因非空且 P1 机械判定 false：
// 拆分后的判定面不漏报。
func TestPullLoopProtocolSplitPerturbedSkeletonDetected(t *testing.T) {
	prefix := newRecordingPrefix()
	session := &fakeSession{runID: "run-perturb", goalID: "goal-1", prefixKey: "pullharness:run-perturb"}
	session.skeletonFor = func(snapshot int) string {
		if snapshot == 1 {
			return "STABLE SKELETON: fixed rule frame"
		}
		return "STABLE SKELETON: fixed RULE frame" // 中部扰动
	}
	session.protocolDirectives = "DIRECTIVES: allowed tools a,b"
	llmFake := &fakeLLM{responses: []string{"call:track.list:receipt-1", "final done"}}
	loop := newSessionLoop(session, llmFake, prefix, &fakeExit{})
	if result := loop.Run(context.Background(), goalInput("run-perturb")); result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q (result=%+v)", result.Outcome, result)
	}
	second := prefix.reports[1]
	if second.PrefixStartsWithPrevious == nil || *second.PrefixStartsWithPrevious {
		t.Fatalf("mid-prefix perturbation must fail the P1 judgment, got %+v", second.PrefixStartsWithPrevious)
	}
	if got := rulesetBreaks(second); len(got) == 0 {
		t.Fatal("perturbation must surface a ruleset_changed attribution (detection, not silence)")
	}
}
