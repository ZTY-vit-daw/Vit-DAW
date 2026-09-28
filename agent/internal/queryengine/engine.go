package queryengine

// engine.go — Engine：查询引擎对外入口（§2.2：三动词 + 一个目录）。
//
// 三条职责边界（§1）：
//  1. 不判断新鲜度——物化层给什么状态透传什么状态（stale 不删不升级不过滤，
//     除非谓词显式要求）；
//  2. 不装配披露——LLMContext/预算/审计是 CCB（observe）的职责；query 只回
//     refs + 声明标量；
//  3. 不触发计算——永远只读物化结果，物化缺失如实报空，不背地里重算。
//
// observe 的求值路径不在引擎面（§2.6：observe 落 CCB 参数化卡）；本卡实现
// Query 全量求值 + Expand 基础面 + DiffEvidence identity 级（content 级委托
// 接线属 IMPL-D；工具面接线属 IMPL-C）。

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"vit-daw-agent/internal/agentprotocol"
)

// Engine 查询引擎。零值不可用；NewEngine 构造。
type Engine struct {
	store    MaterializedStore
	payloads PayloadIndexRegistry
	latest   atomic.Pointer[centralIndex]
}

// NewEngine 构造引擎（函数级依赖注入，§2.2）。payload 可为 nil（无局部索引，
// R5/R6 全部走降级路径——v1 常态）。
func NewEngine(store MaterializedStore, payload PayloadIndexRegistry) *Engine {
	if payload == nil {
		payload = NewPayloadIndexRegistry()
	}
	return &Engine{store: store, payloads: payload}
}

// Start 订阅物化层变更流并在后台换代（§3.4 copy-on-write 整表交换）。
// 生命周期由 ctx 控制：取消 ctx 即停（引擎无独立 Stop）。
func (e *Engine) Start(ctx context.Context) error {
	ch, unsubscribe, err := e.store.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("queryengine: subscribe: %w", err)
	}
	go func() {
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case change, ok := <-ch:
				if !ok {
					return
				}
				e.applyChange(change)
			}
		}
	}()
	return nil
}

// Sync 强制从物化层 latest 视图重建中央索引（v0 轮询换代腿；测试与 IMPL-B 用）。
func (e *Engine) Sync(ctx context.Context) error {
	_, err := e.reloadLatest(ctx)
	return err
}

func (e *Engine) reloadLatest(ctx context.Context) (*centralIndex, error) {
	rows, err := e.store.SnapshotView(ctx, SnapshotSelector{Mode: SnapshotLatest})
	if err != nil {
		return nil, fmt.Errorf("queryengine: snapshot view: %w", err)
	}
	idx := buildCentralIndex(rows)
	e.latest.Store(idx)
	return idx, nil
}

func (e *Engine) ensureLatest(ctx context.Context) (*centralIndex, error) {
	if idx := e.latest.Load(); idx != nil {
		return idx, nil
	}
	// 并发下两个建表者都完整建表后各自 Store：内容同源同摘要，后写覆盖无害。
	return e.reloadLatest(ctx)
}

// indexFor 解析快照谓词为单一不可变视图（read-committed-per-snapshot）：
// latest 用缓存的当前代；exact/at_or_before 每次向物化层取瞬时代（不缓存，
// v2 优化点）。
func (e *Engine) indexFor(ctx context.Context, sp SnapshotPredicate) (*centralIndex, error) {
	mode := sp.Mode
	if mode == "" {
		mode = SnapshotLatest
	}
	if mode == SnapshotLatest {
		return e.ensureLatest(ctx)
	}
	rows, err := e.store.SnapshotView(ctx, SnapshotSelector{Mode: mode, Revision: sp.Revision})
	if err != nil {
		return nil, fmt.Errorf("queryengine: snapshot view: %w", err)
	}
	return buildCentralIndex(rows), nil
}

func (e *Engine) applyChange(ch MaterializedChange) {
	// 变更流同时喂给已注册局部索引（ApplyChange 契约，非查询路径）
	for _, kind := range e.payloads.Kinds() {
		if idx, ok := e.payloads.ForKind(kind); ok {
			_ = idx.ApplyChange(ch) // kind 侧鲁棒性归其实现；引擎尽力转发
		}
	}
	base := e.latest.Load()
	if base == nil {
		return // 初次建表前的变更由 latest 视图自然吸收
	}
	if next := base.withChange(ch); next != base {
		e.latest.Store(next)
	}
}

// Query 执行即席谓词查询。纯只读；不触发任何投影计算（§1 边界 3）。
func (e *Engine) Query(ctx context.Context, q RefQuery) (QueryResult, error) {
	kinds, err := resolveQueryKinds(q.Kinds)
	if err != nil {
		return QueryResult{}, err
	}
	if err := validateRefQuery(q); err != nil {
		return QueryResult{}, err
	}
	if err := e.validateSortFields(q, kinds); err != nil {
		return QueryResult{}, err
	}
	limit := q.Limit
	if limit == 0 {
		limit = DefaultQueryLimit
	}
	idx, err := e.indexFor(ctx, q.Snapshot)
	if err != nil {
		return QueryResult{}, err
	}
	offset := 0
	if q.Cursor != "" {
		if offset, err = decodeCursor(q.Cursor, idx.gen); err != nil {
			return QueryResult{}, err
		}
	}

	payloadInvolved := len(q.Payload) > 0 || hasPayloadSortKey(q.Sort)
	// R5/R6 单 kind 且已注册局部索引 → 整体委托（含排序/分页，成本 index）。
	// 多 kind 混合载荷（R7）v1 执行面走中央降级求值 + 引擎归并——分委托流的
	// 游标归并留后续卡（RouteQuery 的裁决仍是按 kind 拆分的真值）。
	if payloadInvolved && len(kinds) == 1 {
		if pi, ok := e.payloads.ForKind(kinds[0]); ok {
			return e.queryViaIndex(ctx, pi, idx, q, limit)
		}
	}

	matches := idx.evaluate(q, kinds)
	total := len(matches)
	res := QueryResult{Snapshot: idx.gen}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	} else {
		res.NextCursor = encodeCursor(idx.gen, end)
	}
	res.TotalMatches = total
	res.Rows = make([]ResultRow, 0, end-offset)
	for _, m := range matches[offset:end] {
		res.Rows = append(res.Rows, ResultRow{Ref: m.canonical, Freshness: m.freshness, Payload: copyPayload(m.payload)})
	}
	if payloadInvolved {
		// R5/R6 降级：v1 多数 kind 无局部索引，降级是常态——响应必明示（§3.2）
		res.CostClass = CostClassCompile
		res.Degraded = DegradedPayloadIndexUnavailable
	} else {
		res.CostClass = CostClassIndex
	}
	if q.Expand != nil && len(res.Rows) > 0 {
		refs := make([]string, 0, len(res.Rows))
		for _, r := range res.Rows {
			refs = append(refs, r.Ref)
		}
		expanded, err := e.Expand(ctx, refs, *q.Expand)
		if err != nil {
			return QueryResult{}, err
		}
		for i := range res.Rows {
			res.Rows[i].Expanded = &expanded[i]
		}
	}
	return res, nil
}

func (e *Engine) queryViaIndex(ctx context.Context, pi PayloadIndex, idx *centralIndex, q RefQuery, limit int) (QueryResult, error) {
	seg := SegmentConstraints{Scope: q.Scope, Window: q.Window, Snapshot: q.Snapshot, Hash: q.Hash}
	rows, next, err := pi.Search(ctx, seg, q.Payload, q.Sort, limit, q.Cursor)
	if err != nil {
		return QueryResult{}, fmt.Errorf("queryengine: payload index %s: %w", pi.Kind(), err)
	}
	// 契约缺口注记：Search 不回传总命中数，此处 TotalMatches=len(rows) 为下界
	//（next 非空时真实 total 更大）；kind 侧实现卡扩展契约后转精确值。
	return QueryResult{
		Rows:         rows,
		TotalMatches: len(rows),
		NextCursor:   next,
		Snapshot:     idx.gen,
		CostClass:    CostClassIndex,
	}, nil
}

func (e *Engine) validateSortFields(q RefQuery, kinds []string) error {
	for _, sk := range q.Sort {
		if sk.Field == "" {
			return fmt.Errorf("%w: 排序键缺字段名", ErrUnsupportedSort)
		}
		if sk.Dir != "" && sk.Dir != SortAsc && sk.Dir != SortDesc {
			return fmt.Errorf("%w: 未知方向 %q", ErrUnsupportedSort, sk.Dir)
		}
		if isRefSortAlias(sk.Field) {
			continue
		}
		for _, kind := range kinds {
			if !sortFieldDeclared(kind, sk.Field, e.payloads) {
				return fmt.Errorf("%w: 字段 %q 对 kind %q 未声明可排序（可排序列见 Catalog）", ErrUnsupportedSort, sk.Field, kind)
			}
		}
	}
	return nil
}

// ---- 游标（不透明续页：绑定快照代 + 偏移）----

func encodeCursor(gen string, offset int) string {
	return fmt.Sprintf("g=%s;o=%d", gen, offset)
}

func decodeCursor(cursor, gen string) (int, error) {
	if !strings.HasPrefix(cursor, "g=") {
		return 0, fmt.Errorf("%w: malformed %q", ErrStaleCursor, cursor)
	}
	rest := cursor[2:]
	genPart, offPart, ok := strings.Cut(rest, ";o=")
	if !ok {
		return 0, fmt.Errorf("%w: malformed %q", ErrStaleCursor, cursor)
	}
	if genPart != gen {
		return 0, fmt.Errorf("%w: 游标代 %s ≠ 当前代 %s（快照已换代，请重查）", ErrStaleCursor, genPart, gen)
	}
	n, err := strconv.Atoi(offPart)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: malformed offset %q", ErrStaleCursor, offPart)
	}
	return n, nil
}

func copyPayload(p map[string]any) map[string]any {
	if p == nil {
		return nil
	}
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// ---- Expand（§2.4）----

const (
	defaultExpandMaxBytes = 4096
	maxExpandMaxBytes     = 32 * 1024
)

// Expand 把 ref 列表展开为受字节预算约束的句柄/摘要。不做 LLMContext 装配、
// 不走披露预算、不做 digest——grep 命中后的廉价"看一眼"；语义深读走 observe。
// 解析三态宽容：parsed→Resolve；legacy/opaque→原样透传（标 freshness）。
func (e *Engine) Expand(ctx context.Context, refs []string, opt ExpandOptions) ([]ExpandedRef, error) {
	depth := opt.Depth
	if depth == "" {
		depth = ExpandHandle
	}
	maxBytes := opt.MaxBytes
	if maxBytes == 0 {
		maxBytes = defaultExpandMaxBytes
	}
	if maxBytes < 0 || maxBytes > maxExpandMaxBytes {
		return nil, fmt.Errorf("%w: MaxBytes=%d（默认 %d，硬上限 %d）", ErrInvalidExpandOptions, opt.MaxBytes, defaultExpandMaxBytes, maxExpandMaxBytes)
	}
	out := make([]ExpandedRef, 0, len(refs))
	for _, raw := range refs {
		parsed, err := agentprotocol.ParseRef(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrInvalidRef, raw, err)
		}
		switch parsed.State {
		case agentprotocol.RefStateParsed:
			re, err := e.store.Resolve(ctx, *parsed.Ref)
			if err != nil {
				out = append(out, ExpandedRef{Ref: raw, Freshness: "unresolved"})
				continue
			}
			er := ExpandedRef{Ref: raw, Handle: re.Handle, Bytes: re.Bytes, Freshness: re.Freshness}
			if depth == ExpandSummary && re.ReadAll != nil {
				if data, err := re.ReadAll(); err == nil {
					er.Summary, er.Truncated = summarizeContent(data, maxBytes)
				}
			}
			out = append(out, er)
		case agentprotocol.RefStateLegacy:
			out = append(out, ExpandedRef{Ref: raw, Freshness: "legacy"})
		default: // opaque：宽容透传（ruling #8）
			out = append(out, ExpandedRef{Ref: raw, Freshness: "opaque"})
		}
	}
	return out, nil
}

// summarizeContent 禁键过滤（CCB freeStateForbiddenField 同款规则——该函数在
// capabilitycontext 未导出，此处按同规则实现；导出合并走后续接线卡）+ 字节预算
// 截断（截断可能破坏 JSON 完整性，Truncated 明示；"廉价看一眼"的诚实边界）。
func summarizeContent(data []byte, maxBytes int) (map[string]any, bool) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, false
	}
	filtered := filterForbiddenFields(doc)
	out, err := json.Marshal(filtered)
	if err != nil {
		return nil, false
	}
	m, ok := filtered.(map[string]any)
	if !ok {
		m = map[string]any{"value": filtered}
	}
	return m, len(out) > maxBytes
}

func filterForbiddenFields(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			if forbiddenSummaryField(k) {
				continue
			}
			out[k] = filterForbiddenFields(child)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = filterForbiddenFields(child)
		}
		return out
	}
	return v
}

func forbiddenSummaryField(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(k, "raw_") || strings.HasSuffix(k, "_samples") {
		return true
	}
	switch k {
	case "pcm", "pcm_data", "samples", "audio_buffer", "waveform_array", "spectrogram_tiles",
		"time_segments", "event_list", "observation_path", "board_path", "context_pack_path",
		"file_path", "source_path":
		return true
	}
	return false
}

// ---- Catalog（能力自描述：grep 前先有 --help）----

// EngineCatalog 引擎能力面：注册 kind、各 kind 可过滤/可排序字段、ref 排序
// 别名。工具面（IMPL-C）把它编译进 ref.query 的 schema description。
type EngineCatalog struct {
	Kinds       []KindCatalog
	SortAliases []string
}

// KindCatalog 单 kind 目录项（路由表建议字段 ∪ 已注册局部索引字段）。
type KindCatalog struct {
	Kind          string
	LegacyPrefix  string
	CentralSeed   string
	PayloadFields []PayloadFieldSpec
	Notes         []string
}

func (e *Engine) Catalog() EngineCatalog {
	entries := KindRouteTable()
	kinds := make([]KindCatalog, 0, len(entries))
	for _, entry := range entries {
		fields := make([]PayloadFieldSpec, 0, len(entry.PayloadFields)+4)
		fields = append(fields, entry.PayloadFields...)
		if pi, ok := e.payloads.ForKind(entry.Kind); ok {
			fields = append(fields, pi.Fields()...)
		}
		kinds = append(kinds, KindCatalog{
			Kind:          entry.Kind,
			LegacyPrefix:  entry.LegacyPrefix,
			CentralSeed:   entry.CentralSeed,
			PayloadFields: fields,
			Notes:         entry.Notes,
		})
	}
	return EngineCatalog{Kinds: kinds, SortAliases: refSortAliases()}
}

// sortedStringsCanon 工具（diff.go 用）。
func sortedStringsCanon(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
