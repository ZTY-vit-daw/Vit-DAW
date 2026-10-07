package contextruntime

// repull.go — 三态重拉响应包装（设计 §5.3 契约表逐行实现，L1-4-IMPL-C）。
//
// 形态裁决（§5.1）：重拉就是 query，不是新动词——本包装组合既有读侧
// （ref.query 等值谓词 + Expand summary），工具面 JSON schema、谓词模型、
// 物化契约零改动（L1-3 接口冻结的字面执行）。
//
// 三态契约（§5.3 表）：
//   - resolved：票/CAS 工件在且读回成功——响应带 ref/handle/bytes/
//     freshness/summary（expand 预算内）；
//   - stale：内容在但物化层已标脏——freshness=stale 原样透传，无任何
//     升级路径（T6 纪律的重拉版）；
//   - missing：票/工件不存在——必带 pointer{ledger_entry_id, turn_id}
//     指引，不静默不猜（"该观察产生于 turn N，结论已入 entry M"——
//     retain 态存在的理由）。
//
// 边界（§5.5）：重拉走 query+expand（廉价看一眼，无审计回执）；重拉成功
// 只恢复引用材料，不恢复观察事实的 readiness——freshness 三态原样透传，
// 缺/stale 不因重拉而升级。重拉结果进动态区受既有动态区预算管（不占
// MaxDisclosureBytes——那是 observe 的）。

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/queryengine"
)

// RepullState 三态封闭枚举（§5.3）。
type RepullState string

const (
	RepullResolved RepullState = "resolved"
	RepullStale    RepullState = "stale"
	RepullMissing  RepullState = "missing"
)

// RepullPointer 是 missing 态的账本反向指引（§5.3 响应必带）。字段值诚实：
// 无账本条目时 LedgerEntryID=0、无伴随索引注记时 TurnID 空——Note 说明，
// 不臆造。
type RepullPointer struct {
	LedgerEntryID int64  `json:"ledger_entry_id"`
	TurnID        string `json:"turn_id"`
}

// RepullResponse 是模型可读的三态重拉注记。
type RepullResponse struct {
	Ref       string         `json:"ref"`
	State     RepullState    `json:"state"`
	Handle    string         `json:"handle,omitempty"`
	Bytes     int64          `json:"bytes,omitempty"`
	Freshness string         `json:"freshness,omitempty"` // resolved/stale 原样透传，不升级
	Summary   map[string]any `json:"summary,omitempty"`   // expand 预算内摘要
	Pointer   *RepullPointer `json:"pointer,omitempty"`   // missing 必带
	Note      string         `json:"note,omitempty"`      // 模型可读指引（missing 不静默）
}

// RefQueryExpander 是重拉所需的最小读侧面（*queryengine.Engine 满足）。
type RefQueryExpander interface {
	Query(ctx context.Context, q queryengine.RefQuery) (queryengine.QueryResult, error)
	Expand(ctx context.Context, refs []string, opt queryengine.ExpandOptions) ([]queryengine.ExpandedRef, error)
}

// RepullService 组合既有读侧产出三态响应。ProjectDir 供账本反查（missing
// 指引）；History 供 LastSeenTurn（伴随索引注记，§5.2）；两者可缺席——
// 指引字段诚实降级为 0 值+Note 说明。
type RepullService struct {
	Engine       RefQueryExpander
	ProjectDir   string
	History      []promptruntime.HistoryRefEntry
	SummaryBytes int // expand 预算；0=引擎默认（4096）
}

// Repull 对单个 ref 产出三态响应。确定性、零 LLM；任何读侧错误按 missing
// 如实上报（附错误注记），不静默。
func (s *RepullService) Repull(ctx context.Context, raw string) RepullResponse {
	parsed, err := agentprotocol.ParseRef(raw)
	if err != nil {
		// malformed vit://（文法硬拒绝）：宽容注记为 missing（原文透传在 Ref）。
		return s.missing(raw, fmt.Sprintf("malformed vit:// ref (grammar rejection): %v", err))
	}
	switch parsed.State {
	case agentprotocol.RefStateOpaque:
		return s.missing(raw, "opaque ref: 未收录进结构化重拉面（注册表补全前原样透传，见伴随索引 ParseState）")
	case agentprotocol.RefStateLegacy:
		translation := parsed.Legacy
		return s.missing(raw, fmt.Sprintf(
			"legacy ref: 可翻译（family=%s slot=%s）但无完整 L0 坐标，不在 v1 结构化重拉面；待注册表/生成点迁移",
			translation.Family, translation.Slot))
	}

	ref := *parsed.Ref
	exact := queryengine.RefQuery{
		Kinds:    queryengine.KindPredicate{Kinds: []string{ref.Kind}},
		Scope:    &queryengine.ScopePredicate{Kind: ref.ScopeKind, Values: []string{ref.ScopeValue}, ValueSet: true},
		Snapshot: queryengine.SnapshotPredicate{Mode: queryengine.SnapshotExact, Revision: ref.Snapshot},
		Expand:   &queryengine.ExpandOptions{Depth: queryengine.ExpandSummary, MaxBytes: s.SummaryBytes},
	}
	// hash 是值不是键（bootstrap Resolve 语义）：坐标四段等值重拉，不按
	// hash 过滤——旧 hash 引用解析到坐标行（§5.1）。
	result, err := s.Engine.Query(ctx, exact)
	if err != nil {
		return s.missing(raw, fmt.Sprintf("ref.query failed: %v", err))
	}
	if len(result.Rows) == 0 {
		// exact 现扫无行：查 latest 事件代——marked_stale 只改状态不删行
		// （§5.1.1），物化层标脏的行只在事件代可见（T-B2 形态）。latest 有
		// stale 行=内容在但已标脏；其余（poll 滞后/代漂移/真消失）按
		// missing 诚实上报——exact 现扫缺席即无内容可产。
		if stale := s.latestStaleRow(ctx, ref); stale != nil {
			response := RepullResponse{Ref: raw, Freshness: queryengine.FreshnessStale}
			if stale.Expanded != nil {
				response.Handle = stale.Expanded.Handle
				response.Bytes = stale.Expanded.Bytes
				response.Summary = stale.Expanded.Summary
			}
			response.State = RepullStale
			response.Note = "freshness=stale 原样透传：内容在但物化层已标脏，不得当 current 引用"
			return response
		}
		return s.missing(raw, "票/工件不存在（sessionDir 清理等）——重拉失败，指引见 pointer")
	}

	row := result.Rows[0]
	response := RepullResponse{Ref: raw, Freshness: row.Freshness}
	if row.Expanded != nil {
		response.Handle = row.Expanded.Handle
		response.Bytes = row.Expanded.Bytes
		response.Summary = row.Expanded.Summary
	}
	if row.Freshness == queryengine.FreshnessStale {
		// stale 原样透传，无升级路径（T-B2；§5.5 不恢复 readiness）。
		response.State = RepullStale
		response.Note = "freshness=stale 原样透传：内容在但物化层已标脏，不得当 current 引用"
		return response
	}
	response.State = RepullResolved
	return response
}

// latestStaleRow 在 latest 事件代中找该坐标的 stale 行（marked_stale 可见
// 性，§5.1.1）；找不到或非 stale 返回 nil。坐标匹配按 kind/scope/snapshot
// （hash 是值不是键）。
func (s *RepullService) latestStaleRow(ctx context.Context, ref agentprotocol.Ref) *queryengine.ResultRow {
	latest, err := s.Engine.Query(ctx, queryengine.RefQuery{
		Kinds: queryengine.KindPredicate{Kinds: []string{ref.Kind}},
		Scope: &queryengine.ScopePredicate{Kind: ref.ScopeKind, Values: []string{ref.ScopeValue}, ValueSet: true},
	})
	if err != nil {
		return nil
	}
	for i := range latest.Rows {
		parsed, err := agentprotocol.ParseRef(latest.Rows[i].Ref)
		if err != nil || parsed.Ref == nil {
			continue
		}
		row := parsed.Ref
		if row.Kind != ref.Kind || row.ScopeKind != ref.ScopeKind || row.ScopeValue != ref.ScopeValue || row.Snapshot != ref.Snapshot {
			continue
		}
		if latest.Rows[i].Freshness == queryengine.FreshnessStale {
			return &latest.Rows[i]
		}
		return nil
	}
	return nil
}

// missing 构造 missing 态响应：pointer 必带（§5.3），指引文本不静默不猜。
func (s *RepullService) missing(raw, why string) RepullResponse {
	pointer := &RepullPointer{}
	if s.ProjectDir != "" {
		if entries, err := carriers.ReadLedger(s.ProjectDir); err == nil {
			for i := len(entries) - 1; i >= 0; i-- {
				for _, evidence := range entries[i].EvidenceRefs {
					if evidence == raw {
						pointer.LedgerEntryID = entries[i].EntryID
						break
					}
				}
				if pointer.LedgerEntryID != 0 {
					break
				}
			}
		}
	}
	for _, entry := range s.History {
		if entry.Ref == raw {
			pointer.TurnID = entry.LastSeenTurn
			break
		}
	}
	note := strings.TrimSpace(why)
	if pointer.LedgerEntryID != 0 {
		turn := pointer.TurnID
		if turn == "" {
			turn = "(unknown)"
		}
		note += fmt.Sprintf("；该观察产生于 turn %s，结论已入 ledger entry #%d", turn, pointer.LedgerEntryID)
	} else {
		note += "；账本无该 ref 的结论兜底条目（结论未留）"
	}
	return RepullResponse{Ref: raw, State: RepullMissing, Pointer: pointer, Note: note}
}
