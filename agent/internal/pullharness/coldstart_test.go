package pullharness

// coldstart_test.go — 冷启动底座验收（卡面验收标准 1）：
//   - 底座字节恒定：同输入 ×100 渲染与 ×100 PrefixService 装配的
//     content_hash 全等；
//   - 缺失投影面 → 对应事实组 absent（声明缺席，不报错不臆造）；
//   - 会话首装一次性：快照源只读一次，之后跨轮字节恒定（源变异不影响）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"testing"

	"vit-daw-agent/internal/promptruntime"
)

// coldStartFixtureEngine 是确定性内核项目状态快照夹具：三轨+一条汇总线
// （parent/vit_type 同源字段面）。夹具标识全部中性（无域处理词）。
func coldStartFixtureEngine() map[string]any {
	return map[string]any{
		"project_path": "D:/demo/demo.vit",
		"tracks": []any{
			map[string]any{"track_id": "t1", "track_name": "Drums", "parent_track_id": "bus1"},
			map[string]any{"track_id": "t2", "track_name": "Bass", "parent_track_id": "bus1"},
			map[string]any{"track_id": "t3", "track_name": "Vox"},
			map[string]any{"track_id": "bus1", "track_name": "Rhythm Bus", "vit_type": "bus"},
		},
	}
}

func coldStartSectionDigest(base ColdStartBase) string {
	hash := sha256.New()
	for _, section := range base.Sections {
		hash.Write([]byte(section.ID))
		hash.Write([]byte{0})
		hash.Write([]byte(section.Content))
		hash.Write([]byte{0})
	}
	for _, id := range base.AbsentIDs {
		hash.Write([]byte("absent:" + id))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func coldStartLayerHashes(report promptruntime.AssemblyReport) map[string]string {
	hashes := map[string]string{}
	for _, layer := range report.Layers {
		hashes[layer.LayerID] = layer.ContentHash
	}
	return hashes
}

// 底座字节恒定：同输入 ×100 渲染 digest 全等，且 ×100 PrefixService 装配
// 的逐层 content_hash 全等（卡面验收 1：同输入 ×100 装配 content_hash 全等）。
func TestColdStartByteConstantAcrossHundredAssemblies(t *testing.T) {
	engine := coldStartFixtureEngine()
	first := RenderColdStart(engine)
	firstDigest := coldStartSectionDigest(first)
	if len(first.Sections) != 4 {
		t.Fatalf("cold start sections = %d, want 4 (tom/bus/delivery/rules)", len(first.Sections))
	}
	if len(first.AbsentIDs) != 0 {
		t.Fatalf("fixture cold start absent ids = %v, want none", first.AbsentIDs)
	}

	service := promptruntime.NewPrefixService()
	wantLayerHashes := map[string]string{}
	for i := 0; i < 100; i++ {
		base := RenderColdStart(engine)
		if digest := coldStartSectionDigest(base); digest != firstDigest {
			t.Fatalf("render %d digest diverged: %s != %s", i, digest, firstDigest)
		}
		_, report, err := service.Assemble(context.Background(), promptruntime.PrefixRequest{
			AssemblyInput: promptruntime.AssemblyInput{SystemSections: base.Sections},
			SessionKey:    "coldstart-constancy",
			LayerStates:   base.LayerStates(),
		})
		if err != nil {
			t.Fatalf("assemble %d: %v", i, err)
		}
		hashes := coldStartLayerHashes(report)
		if i == 0 {
			wantLayerHashes = hashes
			for _, section := range first.Sections {
				if hashes[section.ID] == "" {
					t.Fatalf("layer %s has empty content hash on first assembly", section.ID)
				}
			}
			continue
		}
		for id, want := range wantLayerHashes {
			if hashes[id] != want {
				t.Fatalf("assembly %d layer %s content hash = %q, want %q (byte constancy broken)", i, id, hashes[id], want)
			}
		}
	}
}

// 三事实组内容就位：TOM 概览（summary+逐轨行）、总线拓扑（routes+bus）、
// 交付目标（内建 profile 引用行）——同源消费面的实证。
func TestColdStartThreeFactGroupsPresent(t *testing.T) {
	base := RenderColdStart(coldStartFixtureEngine())
	byID := map[string]string{}
	for _, section := range base.Sections {
		byID[section.ID] = section.Content
	}
	tom := byID[ColdStartSectionTOM]
	if !strings.Contains(tom, "status=") || !strings.Contains(tom, "tracks=4") {
		t.Fatalf("TOM overview missing summary line: %q", tom)
	}
	for _, track := range []string{"Drums", "Bass", "Vox", "Rhythm Bus"} {
		if !strings.Contains(tom, track) {
			t.Fatalf("TOM overview missing track line for %s: %q", track, tom)
		}
	}
	bus := byID[ColdStartSectionBus]
	if !strings.Contains(bus, "tracks=4") || !strings.Contains(bus, "Drums→Rhythm Bus") ||
		!strings.Contains(bus, "bus/submix: Rhythm Bus") {
		t.Fatalf("bus topology lines missing: %q", bus)
	}
	delivery := byID[ColdStartSectionDelivery]
	if !strings.Contains(delivery, "builtin:") || !strings.Contains(delivery, "LUFS") {
		t.Fatalf("delivery targets missing builtin profile lines: %q", delivery)
	}
	rules := byID[ColdStartSectionRules]
	if !strings.Contains(rules, anchorGatePath) || !strings.Contains(rules, anchorEvidenceGates) {
		t.Fatalf("rules section missing migrated disclosure rows: %q", rules)
	}
	if !base.Sections[0].Stable || base.Sections[0].Kind != promptruntime.SectionSession {
		t.Fatalf("cold start sections must be stable session sections, got %+v", base.Sections[0])
	}
}

// 缺失投影 → 对应事实组 absent 不臆造：nil 快照=TOM/总线缺席（声明）、
// 交付/规则恒在；无路由字段的快照=总线缺席、TOM 在。缺席经 LayerStates
// 与 PrefixService 报告双面可断言。
func TestColdStartMissingProjectionsAbsent(t *testing.T) {
	empty := RenderColdStart(nil)
	if got := strings.Join(empty.AbsentIDs, ","); got != ColdStartSectionTOM+","+ColdStartSectionBus {
		t.Fatalf("nil engine absent ids = %q", got)
	}
	if len(empty.Sections) != 2 {
		t.Fatalf("nil engine sections = %d, want 2 (delivery/rules)", len(empty.Sections))
	}

	noRouting := RenderColdStart(map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "t1", "track_name": "Solo"},
		},
	})
	if got := strings.Join(noRouting.AbsentIDs, ","); got != ColdStartSectionBus {
		t.Fatalf("no-routing engine absent ids = %q, want bus only", got)
	}
	for _, section := range noRouting.Sections {
		if section.ID == ColdStartSectionBus {
			t.Fatalf("bus section rendered without routing fields: %q", section.Content)
		}
	}

	// 缺席声明进 PrefixService 报告（§2.0 fail-open 但显式）。
	service := promptruntime.NewPrefixService()
	_, report, err := service.Assemble(context.Background(), promptruntime.PrefixRequest{
		AssemblyInput: promptruntime.AssemblyInput{SystemSections: empty.Sections},
		SessionKey:    "coldstart-absent",
		LayerStates:   empty.LayerStates(),
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	states := map[string]string{}
	for _, layer := range report.Layers {
		states[layer.LayerID] = layer.State
	}
	for _, id := range []string{ColdStartSectionTOM, ColdStartSectionBus} {
		if states[id] != "absent" {
			t.Fatalf("report row %s state = %q, want absent", id, states[id])
		}
	}
	if states[ColdStartSectionDelivery] != "rendered" || states[ColdStartSectionRules] != "rendered" {
		t.Fatalf("present sections must render, got %v", states)
	}
}

// capturingPrefix 记录全部装配请求（首装一次性断言面）。
type capturingPrefix struct {
	requests []promptruntime.PrefixRequest
}

func (c *capturingPrefix) Assemble(ctx context.Context, req promptruntime.PrefixRequest) (promptruntime.Assembly, promptruntime.AssemblyReport, error) {
	c.requests = append(c.requests, req)
	return promptruntime.Build(req.AssemblyInput), promptruntime.AssemblyReport{
		Layers: []promptruntime.LayerReport{}, Breaks: []promptruntime.BreakEvent{},
		CacheAnomalies: []string{}, HistoryRefs: []promptruntime.HistoryRefEntry{},
		ExitViolations: []string{},
	}, nil
}

// 会话首装一次性：快照源在整场 run 只读一次；之后跨轮装配字节恒定——
// 源中途变异不改变底座（冻结语义），缺席组声明随首装产物走。
func TestColdStartFirstInstallOnceAndFrozen(t *testing.T) {
	var reads int32
	source := ColdStartSourceFunc(func() map[string]any {
		if atomic.AddInt32(&reads, 1) == 1 {
			return coldStartFixtureEngine()
		}
		// 变异快照：若被重读，底座会混入后装轨道（首装一次性即破坏）。
		engine := coldStartFixtureEngine()
		engine["tracks"] = append(engine["tracks"].([]any),
			map[string]any{"track_id": "t9", "track_name": "LaterArrival"})
		return engine
	})
	prefix := &capturingPrefix{}
	loop := &PullLoop{
		LLM:       &fakeLLM{responses: []string{"call:probe:o1", "terminal reply, no tools"}},
		Tools:     &fakeTools{},
		Prefix:    prefix,
		ColdStart: source,
		Budget:    ObservationBudget{MaxCycles: 5},
	}
	result := loop.Run(context.Background(), goalInput("run-cold"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome = %q error=%q, want %q", result.Outcome, result.Error, OutcomeJudgmentOK)
	}
	if got := atomic.LoadInt32(&reads); got != 1 {
		t.Fatalf("cold start source reads = %d, want 1 (first install once)", got)
	}
	if len(prefix.requests) < 2 {
		t.Fatalf("assemblies = %d, want >= 2 (two cycles)", len(prefix.requests))
	}
	first := prefix.requests[0].SystemSections
	for i, req := range prefix.requests[1:] {
		if len(req.SystemSections) != len(first) {
			t.Fatalf("assembly %d section count = %d, want %d", i+1, len(req.SystemSections), len(first))
		}
		for j, section := range req.SystemSections {
			if section != first[j] {
				t.Fatalf("assembly %d section %d diverged from first install: %+v != %+v", i+1, j, section, first[j])
			}
		}
	}
	for _, section := range first {
		if section.ID == ColdStartSectionTOM && strings.Contains(section.Content, "LaterArrival") {
			t.Fatalf("cold start TOM mutated after first install: %q", section.Content)
		}
	}
}

// nil ColdStart 源 = 无底座（A 阶段形态逐字节保持：SystemSections 空、
// LayerStates 空声明）。
func TestColdStartNilSourceKeepsAForm(t *testing.T) {
	prefix := &capturingPrefix{}
	loop := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"done"}},
		Prefix: prefix,
	}
	loop.Run(context.Background(), goalInput("run-nocold"))
	if len(prefix.requests) != 1 {
		t.Fatalf("assemblies = %d, want 1", len(prefix.requests))
	}
	req := prefix.requests[0]
	if len(req.SystemSections) != 0 {
		t.Fatalf("nil source system sections = %d, want 0", len(req.SystemSections))
	}
	if len(req.LayerStates) != 0 {
		t.Fatalf("nil source layer states = %v, want empty", req.LayerStates)
	}
	if !strings.Contains(req.UserSections[len(req.UserSections)-1].Content, "budget_state:") {
		t.Fatalf("dynamic zone missing budget disclosure: %q", req.UserSections[0].Content)
	}
}
