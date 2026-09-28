package queryengine

// diff.go — §2.5 DiffEvidence：两级差分。
//
// 设计立场：载荷级差分一律委托既有承载者（COM change_delta / observation
// before_after / FXM A/B），查询引擎不新建差分算法——identity 级（集合差）是
// 引擎自己的毫秒级职责（R9）；content 级委托接线属 IMPL-D（本卡 ErrNotImplemented）。

import (
	"context"
	"fmt"
	"sort"

	"vit-daw-agent/internal/agentprotocol"
)

// DiffDepth 差分深度两级。
type DiffDepth string

const (
	DiffIdentity DiffDepth = "identity" // 两快照 ref 集合差（index）
	DiffContent  DiffDepth = "content"  // 载荷差异，委托既有差分承载者（IMPL-D）
)

// SnapshotRefSet 差分侧集：revision / observation_id / 显式 ref 列表三选一。
type SnapshotRefSet struct {
	Revision      string
	ObservationID string
	Refs          []string // 显式集合（模型从 query 结果里拿来对拍）
}

// DiffRequest 差分请求。Scope 可选：差分前先按谓词缩小两侧集合。
type DiffRequest struct {
	Base  SnapshotRefSet
	Head  SnapshotRefSet
	Scope *RefQuery
	Depth DiffDepth // 空 = identity
}

// RefPair 同主键坐标（kind+scope_kind+scope_value+window）的一对 ref。
type RefPair struct {
	Base string
	Head string
}

// DiffReport 差分产物。Changed 判定 = 坐标相同、hash 不同（内容变了）。
// fxm 类 kind hash 含时间戳（实例身份）——误报风险按 kind 注记在路由表
// Notes（T8 锁定），判定 refinement 归 IMPL-D。
type DiffReport struct {
	Added          []string
	Removed        []string
	Changed        []RefPair
	UnchangedCount int
	Delegated      map[string]any // Depth=content 时：承载者 → 工件/ref 映射（IMPL-D）
	CostClass      string         // identity=index；content=compile（需 render 则 probe）
}

// DiffEvidence 对两个快照（或显式 ref 集）做差分。
func (e *Engine) DiffEvidence(ctx context.Context, d DiffRequest) (DiffReport, error) {
	depth := d.Depth
	if depth == "" {
		depth = DiffIdentity
	}
	if depth == DiffContent {
		return DiffReport{}, fmt.Errorf("%w: diff depth=content（委托既有差分承载者的接线属 IMPL-D）", ErrNotImplemented)
	}
	baseRows, err := e.diffSideRows(ctx, d.Base)
	if err != nil {
		return DiffReport{}, err
	}
	headRows, err := e.diffSideRows(ctx, d.Head)
	if err != nil {
		return DiffReport{}, err
	}
	if d.Scope != nil {
		kinds, err := resolveQueryKinds(d.Scope.Kinds)
		if err != nil {
			return DiffReport{}, err
		}
		if err := validateRefQuery(*d.Scope); err != nil {
			return DiffReport{}, err
		}
		scopedBase := (&centralIndex{rows: baseRows}).evaluate(*d.Scope, kinds)
		scopedHead := (&centralIndex{rows: headRows}).evaluate(*d.Scope, kinds)
		baseRows, headRows = scopedBase, scopedHead
	}

	baseMap := coordMap(baseRows)
	headMap := coordMap(headRows)

	var added, removed []string
	var changed []RefPair
	unchanged := 0
	for coord, h := range headMap {
		b, ok := baseMap[coord]
		if !ok {
			added = append(added, h.canonical)
			continue
		}
		if b.ref.Hash != h.ref.Hash {
			changed = append(changed, RefPair{Base: b.canonical, Head: h.canonical})
		} else {
			unchanged++
		}
	}
	for coord, b := range baseMap {
		if _, ok := headMap[coord]; !ok {
			removed = append(removed, b.canonical)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Base < changed[j].Base })
	return DiffReport{
		Added:          sortedStringsCanon(added),
		Removed:        sortedStringsCanon(removed),
		Changed:        changed,
		UnchangedCount: unchanged,
		CostClass:      CostClassIndex,
	}, nil
}

// diffSideRows 解析差分侧集：显式 refs 须为 parsed 态（legacy/opaque 缺 L0
// 坐标，不可差分）；revision/observation_id 走物化层 exact 视图（§5.1.2 代际
// 解析归物化层）。
func (e *Engine) diffSideRows(ctx context.Context, s SnapshotRefSet) ([]indexedRow, error) {
	if len(s.Refs) > 0 {
		out := make([]indexedRow, 0, len(s.Refs))
		for _, raw := range s.Refs {
			parsed, err := agentprotocol.ParseRef(raw)
			if err != nil {
				return nil, fmt.Errorf("%w: %q: %v", ErrInvalidDiffRef, raw, err)
			}
			if parsed.State != agentprotocol.RefStateParsed {
				return nil, fmt.Errorf("%w: %q 非 parsed 态（legacy/opaque 缺 L0 坐标）", ErrInvalidDiffRef, raw)
			}
			c, err := agentprotocol.FormatRef(*parsed.Ref)
			if err != nil {
				return nil, fmt.Errorf("%w: %q: %v", ErrInvalidDiffRef, raw, err)
			}
			out = append(out, indexedRow{ref: *parsed.Ref, canonical: c})
		}
		return out, nil
	}
	if s.Revision == "" && s.ObservationID == "" {
		return nil, fmt.Errorf("%w: 侧集三选一全空（refs/revision/observation_id）", ErrInvalidDiffRef)
	}
	rows, err := e.store.SnapshotView(ctx, SnapshotSelector{
		Mode:          SnapshotExact,
		Revision:      s.Revision,
		ObservationID: s.ObservationID,
	})
	if err != nil {
		return nil, fmt.Errorf("queryengine: snapshot view: %w", err)
	}
	return buildCentralIndex(rows).rows, nil
}

// coordMap 主键坐标 → 行。同坐标重复取 canonical 最大者（确定性去重）。
func coordMap(rows []indexedRow) map[string]indexedRow {
	m := make(map[string]indexedRow, len(rows))
	for _, r := range rows {
		key := coordKeyOf(r)
		if prev, ok := m[key]; !ok || r.canonical > prev.canonical {
			m[key] = r
		}
	}
	return m
}

func coordKeyOf(r indexedRow) string {
	w := "all"
	if r.ref.Window != nil && !r.ref.Window.AllTime {
		w = fmt.Sprintf("%d..%d", r.ref.Window.SampleStart, r.ref.Window.SampleEnd)
	}
	return r.ref.Kind + "\x1f" + r.ref.ScopeKind + "\x1f" + r.ref.ScopeValue + "\x1f" + w
}
