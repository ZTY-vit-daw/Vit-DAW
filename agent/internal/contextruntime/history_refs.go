package contextruntime

// history_refs.go — 历史 refs 伴随索引 + 轮次边界挂点（设计 §5.2/§4.3 v1
// 触发形态，L1-4-IMPL-C）。
//
// 伴随索引是「残骸 → 句柄」的渐进形态（§5.2）：历史消息是不可变审计面，
// 不改写文本——装配时对 assistant 消息中的 evidence refs 做解析注记，产出
// HistoryRefEntry 进 AssemblyReport.HistoryRefs。模型从动态区看到结构化
// 目录；opaque 残骸照旧留文本（宽容透传）但不进可重拉面。Tax 递减度量=
// parsed 占比（遥测 history_refs_parsed/history_refs_total，§5.2 首审口径）。
//
// 挂点（RunTurnBoundaryHook）供 chat/agentloop 装配路径调用：伴随索引 +
// 退场执行器执法面，advisory——retain/ref 决策不消费、账本不落盘（生产
// 消费切换归 IMPL-D），Violations 经 AssemblyReport 进遥测。

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/queryengine"
)

// HistoryRefIndexer 构建伴随索引。零值可用（pure parse 注记——Handle/
// Freshness 空，账本反指缺席）；Expand/Ledger 供给时补全 §5.2 全字段。
type HistoryRefIndexer struct {
	// Expand 是 queryengine.Engine 的展开面（handle 深度）；nil=纯解析注记。
	// 生产 v1 挂点无引擎接线（IMPL-D 补）；测试与 RepullService 侧供给。
	Expand func(ctx context.Context, refs []string) ([]queryengine.ExpandedRef, error)
	// Ledger 供给账本条目（反指 LedgerEntry 字段——「可回指时」）；nil=跳过。
	Ledger func() []carriers.LedgerEntry
}

// Build 对 assistant 消息做 refs 解析注记。lastSeenTurn 注记为本次装配轮
// （refs 在当前窗内可见的最近轮）。输出序=首次出现序（确定性）；同一 ref
// 多次出现合并为一行。
func (ix *HistoryRefIndexer) Build(ctx context.Context, history []llm.Message, lastSeenTurn string) []promptruntime.HistoryRefEntry {
	var order []string
	seen := map[string]bool{}
	for _, message := range history {
		if !strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
			continue
		}
		for _, ref := range extractEvidenceRefs(message.Content) {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			order = append(order, ref)
		}
	}
	if len(order) == 0 {
		return []promptruntime.HistoryRefEntry{}
	}

	states := make(map[string]promptruntime.HistoryRefEntry, len(order))
	parsedRefs := make([]string, 0, len(order))
	for _, ref := range order {
		entry := promptruntime.HistoryRefEntry{Ref: ref, LastSeenTurn: lastSeenTurn}
		parsed, err := agentprotocol.ParseRef(ref)
		switch {
		case err != nil:
			// malformed vit://（文法硬拒绝）与无法归类的字面：以 opaque 形态
			// 留痕（宽容注记，不炸消费面）；不进可重拉面。
			entry.ParseState = string(agentprotocol.RefStateOpaque)
		default:
			entry.ParseState = string(parsed.State)
			if parsed.State == agentprotocol.RefStateParsed {
				parsedRefs = append(parsedRefs, ref)
			}
		}
		states[ref] = entry
	}

	// parsed refs 批量展开（handle 深度）：Handle/Freshness 物化层透传；
	// unresolved/展开失败=不可解析为空（不臆造）。
	if ix.Expand != nil && len(parsedRefs) > 0 {
		if expanded, err := ix.Expand(ctx, parsedRefs); err == nil {
			for _, er := range expanded {
				entry, ok := states[er.Ref]
				if !ok {
					continue
				}
				switch er.Freshness {
				case queryengine.FreshnessCurrent, queryengine.FreshnessMaterialReuse, queryengine.FreshnessStale:
					entry.Handle = er.Handle
					entry.Freshness = er.Freshness
				default:
					// unresolved/legacy/opaque 透传行：parsed 请求下 unresolved=
					// 物化层无此行——Handle/Freshness 留空（诚实缺席）。
				}
				states[er.Ref] = entry
			}
		}
	}

	// 账本反指（可回指时）：最近一条 EvidenceRefs 含该 ref 的条目。
	if ix.Ledger != nil {
		entries := ix.Ledger()
		for _, ref := range order {
			for i := len(entries) - 1; i >= 0; i-- {
				for _, evidence := range entries[i].EvidenceRefs {
					if evidence == ref {
						entry := states[ref]
						entry.LedgerEntry = entries[i].EntryID
						states[ref] = entry
						break
					}
				}
				if states[ref].LedgerEntry != 0 {
					break
				}
			}
		}
	}

	out := make([]promptruntime.HistoryRefEntry, 0, len(order))
	for _, ref := range order {
		out = append(out, states[ref])
	}
	return out
}

// refTokenDelimiters 终止一个 ref 字面的字符集（空白、引号、括号族与常见
// 中西文句读）。token 采集后再剥尾部句读（句点可能是 kind 段字符——
// dad.l3——只在结尾剥）。
const refTokenDelimiters = " \t\r\n\"'`<>[]{}()|,;，。；、（）】》《「』"

// extractEvidenceRefs 从文本中提取 evidence refs 字面：vit:// 前缀 token、
// legacyPrefixRegistry 前缀 token（最长前缀优先互不劫持由注册表保证）、
// 以及任意 scheme 头 token（X://…——opaque 残骸的发现面：未注册 scheme
// 如 weird://、audit_snapshot://、evidence:// 也注记为 opaque 行，模型得
// 以看见"此 ref 不在可重拉面"而不是漏注）。扫描按 rune 解码推进（分隔
// 符集含全角句读）。
func extractEvidenceRefs(text string) []string {
	var out []string
	seen := map[string]bool{}
	take := func(token string) {
		token = strings.TrimRight(token, ".,;:!?。），；：！？")
		if token == "" || seen[token] {
			return
		}
		seen[token] = true
		out = append(out, token)
	}
	scanToken := func(start int) int {
		end := start
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if strings.ContainsRune(refTokenDelimiters, r) {
				break
			}
			end += size
		}
		return end
	}
	scanFrom := func(start int, prefix string) int {
		return scanToken(start + len(prefix))
	}
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], agentprotocol.RefSchemePrefix):
			end := scanFrom(i, agentprotocol.RefSchemePrefix)
			take(text[i:end])
			i = end
			continue
		default:
			matched := ""
			for _, legacy := range agentprotocol.LegacyPrefixRegistry() {
				if legacy.LegacyPrefix != "" && strings.HasPrefix(text[i:], legacy.LegacyPrefix) {
					if len(legacy.LegacyPrefix) > len(matched) {
						matched = legacy.LegacyPrefix
					}
				}
			}
			if matched != "" {
				end := scanFrom(i, matched)
				take(text[i:end])
				i = end
				continue
			}
			if end, ok := schemeTokenEnd(text, i, scanToken); ok {
				take(text[i:end])
				i = end
				continue
			}
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
		}
	}
	return out
}

// schemeTokenEnd 报告 i 是否是一个 scheme 头 token（[A-Za-z0-9+.-]+://）
// 的起点，是则返回 token 终点。vit:// 已被上一分支处理，此处只会命中非
// 注册 scheme（opaque 注记面）。
func schemeTokenEnd(text string, i int, scanToken func(int) int) (int, bool) {
	if i >= len(text) || !isSchemeStartByte(text[i]) {
		return 0, false
	}
	j := i
	for j < len(text) && isSchemeChar(text[j]) {
		j++
	}
	if j == i || j+3 > len(text) || text[j:j+3] != "://" {
		return 0, false
	}
	return scanToken(j + 3), true
}

func isSchemeChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '+' || c == '-' || c == '.':
		return true
	default:
		return false
	}
}

func isSchemeStartByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// TurnBoundaryHookInput 是装配路径挂点的输入（v1 advisory 形态）。
type TurnBoundaryHookInput struct {
	SessionKey string
	TurnID     string
	History    []llm.Message
	// HistoryLimit 是既有截尾线（chat=12，§4.2 归并表锚点不动）；0=不评
	// window_slide 候选。
	HistoryLimit int
}

// RunTurnBoundaryHook 是 chat/agentloop 装配处的 v1 挂点（IMPL-C，挂点
// diff ≤30 行级的单入口）：伴随索引注记 + 退场执行器执法面。advisory——
// 决策不消费、账本不落盘、CAS 例外路径不接线（生产消费切换归 IMPL-D）；
// 返回值直接落 AssemblyReport（HistoryRefs/ExitViolations）。
func RunTurnBoundaryHook(ctx context.Context, in TurnBoundaryHookInput) ([]promptruntime.HistoryRefEntry, []string) {
	indexer := &HistoryRefIndexer{}
	historyRefs := indexer.Build(ctx, in.History, in.TurnID)

	executor := NewExitExecutor(ExitExecutorConfig{})
	window := WindowState{Units: make([]WindowUnit, 0, len(in.History)), HistoryLimit: in.HistoryLimit}
	for i, message := range in.History {
		window.Units = append(window.Units, WindowUnit{
			Unit:  ExitUnit{Kind: ExitUnitHistoryMessage, ID: messageIndexID(i)},
			Bytes: int64(len(message.Content)),
		})
	}
	report := executor.OnTurnBoundary(ctx, TurnBoundaryEvent{
		TurnID: in.TurnID,
		Window: window,
	})
	return historyRefs, report.Violations
}

func messageIndexID(index int) string {
	return "history:" + strconv.Itoa(index)
}
