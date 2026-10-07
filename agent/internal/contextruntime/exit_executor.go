package contextruntime

// exit_executor.go — 退场执行器（CONTEXT_LAYERING_V1_DESIGN §4，L1-4-IMPL-C）。
//
// 定位（§4.2 归并原则）：五处既有退场机制照旧运转，本执行器是增量前置的
// 执法点，不是替换层——统一的是「什么该退场」的判据语言与「退成什么」的
// 三态词汇。安全不变式（§4.1 执法红线）：任何退场动作必须满足「结论已留
// OR 句柄可重拉」二者之一；不满足者拒绝退场并记 WARN（宁可超预算不丢
// 证据链）。执法在本文件内机械完成，零 LLM 参与。
//
// retain 接线（§4.1 落点表）：retain 态产物=待写入器落盘的账本条目
// （carriers.LedgerEntry，EntryID=0），经 WriteRetains 走 IMPL-B 写入器
// （append-only 字面执行，T-B5）。CAS 化是例外路径不是默认路径（§5.4）：
// v1 白名单见 casEligibleKinds。写入侧解析预检（§4.4-1）：evidence refs
// 经 agentprotocol.ParseRef 三态分类，parsed/legacy 为结构可重拉面；
// opaque 宽容透传（G1 #8 语义，WARN 一次由 ParseRef 自带记账面承担）。
//
// v1 触发形态：chat/agentloop 装配处挂点（TurnBoundaryHook，advisory——
// 决策不消费、账本不落盘）；生产消费切换归 IMPL-D。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/contextruntime/carriers"
)

// 退场单元 Kind 词汇（§4.3 ExitUnit.Kind）。
const (
	ExitUnitObservationBundle = "observation_bundle"
	ExitUnitToolResult        = "tool_result"
	ExitUnitTrace             = "trace"
	ExitUnitHistoryMessage    = "history_message"
)

// 退场判据四值（§4.1）。
const (
	ExitReasonTurnEnd        = "turn_end"
	ExitReasonBudget         = "budget"
	ExitReasonSemanticExpiry = "semantic_expiry"
	ExitReasonWindowSlide    = "window_slide"
)

// ExitAction 退场动作三态（§4.1 落点表）。
type ExitAction string

const (
	ExitRetain ExitAction = "retain" // 结论入账本（LedgerEntry 待写入器落盘）
	ExitRef    ExitAction = "ref"    // 句柄化（vit:// 或既有冷引用/CAS evidence://）
	ExitDrop   ExitAction = "drop"   // 出窗（不变式已满足：结论已留或句柄在）
)

type ExitUnit struct {
	Kind string // observation_bundle | tool_result | trace | history_message
	ID   string // observation_id / CAS hash / 消息序号
}

// CASPayload 是 §5.4 例外路径的输入：无票且 retain-worthy 的长文本，
// 由 CAS 写入器句柄化为 evidence://<sha256>。
type CASPayload struct {
	Kind    string // projectstore 证据 kind 标签
	Content any
}

// WindowUnit 是动态窗内容清单的一行（§4.3 WindowState 内容清单）。
type WindowUnit struct {
	Unit  ExitUnit
	Bytes int64

	// TurnID 是产生该单元的轮次；== 边界事件 TurnID → turn_end 候选。
	TurnID string
	// SemanticExpiry 标记 ExpiresAfterContextChange 族条件成立（§4.1-3，
	// 语义对齐 agentloop/execution_memory.go 既有机制，本包不消费该机制）。
	SemanticExpiry bool
	// RetainedStatement 非空=该单元有结论级陈述（观察账本结论面/上游显式
	// 提供）——retain 态输入。
	RetainedStatement string
	// HandleRef 非空=既有可重拉句柄（观察票、audit_snapshot:// 冷引用、
	// evidence:// CAS）——ref 态输入。
	HandleRef string
	// EvidenceRefs 是该单元携带的证据引用（retain 入账随条目落账；判据
	// 面：非空=证据链存在，drop 需不变式支撑）。
	EvidenceRefs []string
	// CAS 供给且单元类型在白名单内时走 §5.4 例外路径。
	CAS *CASPayload
}

type WindowState struct {
	// Units 最旧在前——budget（最旧优先）与 window_slide 判据的序基础。
	Units        []WindowUnit
	HistoryLimit int // >0：history_message 超限部分为 window_slide 候选（chat=12 既有线）
}

type BudgetState struct {
	HotBytes      int64
	HotLimitBytes int64 // <=0 = 无预算线（不产 budget 候选）
}

type TurnBoundaryEvent struct {
	TurnID string
	Window WindowState
	Budget BudgetState
	// Now 供 retain 条目时间戳；零值=time.Now（确定性测试注入时钟）。
	Now time.Time
}

type ExitDecision struct {
	Unit        ExitUnit
	Action      ExitAction
	Reason      string                // turn_end | budget | semantic_expiry | window_slide
	LedgerEntry *carriers.LedgerEntry // retain 态待写入器落盘（EntryID/PrevHash 归写入器）
	HandleRef   string                // ref 态产物
	Bytes       int64
}

type ExitReport struct {
	Decisions  []ExitDecision
	Violations []string // 被拒绝的退场（不变式不满足）+ WARN 文本——进遥测
}

// ExitExecutorConfig 是执行器依赖注入面（全部可选——零值执行器仍完整执法，
// 只是没有 CAS 例外路径与 WARN 出口）。
type ExitExecutorConfig struct {
	// CASWriter 把例外路径内容句柄化（projectstore.PutEvidence 形态）；
	// nil=CAS 不可用（白名单单元退回不变式判定）。
	CASWriter func(kind string, content any) (handle string, bytes int64, err error)
	// WARN 面（遥测/日志）；nil=丢弃（Violations 仍在报告内）。
	Warn func(line string)
	// Now 时钟注入；nil=time.Now。
	Now func() time.Time
}

// ExitExecutor 是 §4.3 设计态接口族。
type ExitExecutor interface {
	// OnTurnBoundary 在轮次边界被装配层调用（确定性，零 LLM）。
	OnTurnBoundary(ctx context.Context, ev TurnBoundaryEvent) ExitReport
}

type exitExecutor struct {
	cfg ExitExecutorConfig
}

// NewExitExecutor 构造默认执行器。
func NewExitExecutor(cfg ExitExecutorConfig) ExitExecutor {
	return &exitExecutor{cfg: cfg}
}

// casEligibleKinds 是 §5.4 CAS 化例外路径的单元类型白名单（OQ-3，v1 保守，
// 按单元类型实测覆盖裁定）：仅 tool_result——观察 bundle 已有票（F6 天然
// 可重拉，CAS 冗余）、trace 由审计快照冷引用覆盖（§4.2 归并表 ref 态）、
// history_message 是不可变审计面（伴随索引覆盖）。不做全量快照，写放大受控。
var casEligibleKinds = map[string]bool{
	ExitUnitToolResult: true,
}

type exitCandidate struct {
	unit   WindowUnit
	reason string
}

// OnTurnBoundary：候选推导（四判据，确定性序）→ 逐单元三态判定 → 不变式
// 执法（违例拒退场+WARN）。ctx 预留给 Resolve 预检扩展面（v1 读侧承担）。
func (x *exitExecutor) OnTurnBoundary(ctx context.Context, ev TurnBoundaryEvent) ExitReport {
	_ = ctx
	report := ExitReport{Decisions: []ExitDecision{}, Violations: []string{}}
	for _, candidate := range x.deriveCandidates(ev) {
		decision, violation := x.decide(candidate)
		if violation != "" {
			report.Violations = append(report.Violations, violation)
			// 拒绝退场：不产 decision，单元留在动态窗（宁可超预算，§4.1）。
			if x.cfg.Warn != nil {
				x.cfg.Warn("[contextruntime.exit] WARN " + violation)
			}
			continue
		}
		report.Decisions = append(report.Decisions, decision)
	}
	return report
}

// deriveCandidates 按判据优先序（turn_end > semantic_expiry > window_slide
// > budget）产出退场候选，候选间保持窗内单元序（最旧在前）。
func (x *exitExecutor) deriveCandidates(ev TurnBoundaryEvent) []exitCandidate {
	units := ev.Window.Units
	reason := make(map[int]string, len(units))
	order := make([]int, 0, len(units))
	for i, wu := range units {
		switch {
		case isRawProjectionKind(wu.Unit.Kind) && wu.TurnID != "" && wu.TurnID == ev.TurnID:
			reason[i] = ExitReasonTurnEnd
		case wu.SemanticExpiry:
			reason[i] = ExitReasonSemanticExpiry
		default:
			continue
		}
		order = append(order, i)
	}
	// window_slide：history_message 超出 HistoryLimit 的最旧部分滑出。
	if limit := ev.Window.HistoryLimit; limit > 0 {
		seen := 0
		totalHistory := 0
		for _, wu := range units {
			if wu.Unit.Kind == ExitUnitHistoryMessage {
				totalHistory++
			}
		}
		slideCount := totalHistory - limit
		for i, wu := range units {
			if wu.Unit.Kind != ExitUnitHistoryMessage {
				continue
			}
			if seen < slideCount {
				if _, taken := reason[i]; !taken {
					reason[i] = ExitReasonWindowSlide
					order = append(order, i)
				}
			}
			seen++
		}
	}
	// budget：超线时最旧优先逐单元淘汰（v1 不做"最大优先"次级排序——窗内
	// 序即淘汰序，确定性优先），直到模拟余量回到线内。history_message 不
	// 参与预算淘汰——历史窗由 window_slide 判据独管（chat limit=12 既有
	// 机制，§4.2 归并表），budget 判据管动态投影内容面（观察/工具/trace）。
	if ev.Budget.HotLimitBytes > 0 && ev.Budget.HotBytes > ev.Budget.HotLimitBytes {
		projected := ev.Budget.HotBytes
		for i, wu := range units {
			if projected <= ev.Budget.HotLimitBytes {
				break
			}
			if wu.Unit.Kind == ExitUnitHistoryMessage {
				continue // 窗内历史不是预算淘汰面
			}
			if _, taken := reason[i]; taken {
				projected -= wu.Bytes // 已是候选的单元照常让出预算
				continue
			}
			reason[i] = ExitReasonBudget
			order = append(order, i)
			projected -= wu.Bytes
		}
	}
	candidates := make([]exitCandidate, 0, len(order))
	for _, i := range order {
		candidates = append(candidates, exitCandidate{unit: units[i], reason: reason[i]})
	}
	return candidates
}

func isRawProjectionKind(kind string) bool {
	switch kind {
	case ExitUnitObservationBundle, ExitUnitToolResult, ExitUnitTrace:
		return true
	default:
		return false
	}
}

// decide 对单一候选做三态判定；violation 非空=拒绝退场（§4.1 不变式）。
func (x *exitExecutor) decide(candidate exitCandidate) (ExitDecision, string) {
	wu := candidate.unit
	base := ExitDecision{Unit: wu.Unit, Reason: candidate.reason, Bytes: wu.Bytes}

	// 1. 结论级陈述在 → retain：产出待写入器落盘条目（Kind∈
	//    observation_conclusion 族；evidence refs 过解析预检，opaque 宽容
	//    透传由 ParseRef 自带 WARN-once 记账）。
	if strings.TrimSpace(wu.RetainedStatement) != "" {
		entry := &carriers.LedgerEntry{
			Kind:         carriers.LedgerKindObservationConclusion,
			Statement:    strings.TrimSpace(wu.RetainedStatement),
			EvidenceRefs: admittedEvidenceRefs(wu.EvidenceRefs),
			CreatedAt:    x.now(),
		}
		base.Action = ExitRetain
		base.LedgerEntry = entry
		return base, ""
	}

	// 2. 既有句柄在 → ref（观察票/audit_snapshot:// 冷引用/evidence://）。
	if strings.TrimSpace(wu.HandleRef) != "" {
		base.Action = ExitRef
		base.HandleRef = strings.TrimSpace(wu.HandleRef)
		return base, ""
	}

	// 3. CAS 例外路径（§5.4）：无票 retain-worthy 长文本，白名单内且写入器
	//    在位 → 句柄化。写入失败按基础设施故障拒退场（不静默降级 drop）。
	if wu.CAS != nil && casEligibleKinds[wu.Unit.Kind] {
		if x.cfg.CASWriter == nil {
			return ExitDecision{}, x.violation(wu, candidate.reason,
				"CAS-eligible unit but no CAS writer wired (exception path unavailable)")
		}
		handle, _, err := x.cfg.CASWriter(wu.CAS.Kind, wu.CAS.Content)
		if err != nil {
			return ExitDecision{}, x.violation(wu, candidate.reason,
				fmt.Sprintf("CAS exception path failed: %v", err))
		}
		base.Action = ExitRef
		base.HandleRef = handle
		return base, ""
	}

	// 4a. 历史消息 → drop：不可变审计面本身持久（§5.2），证据 refs 由伴随
	//     索引进可重拉面，滑窗不销毁证据链。
	if wu.Unit.Kind == ExitUnitHistoryMessage {
		base.Action = ExitDrop
		return base, ""
	}

	// 4b. 无证据义务 → drop：refs 与结论皆空（纯会话文本/无 refs 的工具
	//     结果/trace）——证据链不存在，无可丢。observation_bundle 除外：
	//     其字节本身就是投影证据（§4.1 turn_end 原始投影内容）。
	if len(wu.EvidenceRefs) == 0 && wu.Unit.Kind != ExitUnitObservationBundle {
		base.Action = ExitDrop
		return base, ""
	}

	// 5. 不变式不满足（有证据负担而无结论无句柄）→ 拒绝退场 + WARN。
	return ExitDecision{}, x.violation(wu, candidate.reason,
		"exit refused: evidence-bearing unit has neither retained conclusion nor re-pullable handle "+
			"(invariant: 结论已留 OR 句柄可重拉; 宁可超预算不丢证据链)")
}

func (x *exitExecutor) violation(wu WindowUnit, reason, detail string) string {
	return fmt.Sprintf("exit refused: %s %s (reason=%s): %s", wu.Unit.Kind, wu.Unit.ID, reason, detail)
}

func (x *exitExecutor) now() time.Time {
	if x.cfg.Now != nil {
		return x.cfg.Now()
	}
	return time.Now()
}

// admittedEvidenceRefs 做写入侧解析预检（§4.4-1）：逐条 ParseRef 分类。
// parsed/legacy 是结构可重拉面；opaque 宽容透传留存（残骸不入结构面，
// WARN-once 由 agentprotocol 全局记账承担）；malformed vit:// 拒收该条
// （文法拒绝是硬错误，不进账本结构面）。
func admittedEvidenceRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	admitted := make([]string, 0, len(refs))
	for _, ref := range refs {
		parsed, err := agentprotocol.ParseRef(ref)
		if err != nil {
			continue // malformed：拒收（不静默——ParseRef 的 opaque 记账不覆盖硬错误，调用面 WARN 归遥测）
		}
		switch parsed.State {
		case agentprotocol.RefStateParsed, agentprotocol.RefStateLegacy, agentprotocol.RefStateOpaque:
			admitted = append(admitted, ref)
		}
	}
	return admitted
}

// WriteRetains 把 retain 决策的待落盘条目经 IMPL-B 写入器追加进工程账本
// （§4.1 retain 落点；append-only 由写入器字面保证——T-B5 驱动面）。
// 返回持久化后的条目（EntryID/PrevHash/CreatedAt 由写入器定）。
func WriteRetains(projectDir string, report ExitReport, now time.Time) ([]carriers.LedgerEntry, error) {
	written := make([]carriers.LedgerEntry, 0, len(report.Decisions))
	for _, decision := range report.Decisions {
		if decision.Action != ExitRetain || decision.LedgerEntry == nil {
			continue
		}
		entry, err := carriers.AppendLedgerEntry(
			projectDir,
			decision.LedgerEntry.Kind,
			decision.LedgerEntry.Phase,
			decision.LedgerEntry.Statement,
			decision.LedgerEntry.EvidenceRefs,
			decision.LedgerEntry.Supersedes,
			now,
		)
		if err != nil {
			return written, err
		}
		written = append(written, entry)
	}
	return written, nil
}
