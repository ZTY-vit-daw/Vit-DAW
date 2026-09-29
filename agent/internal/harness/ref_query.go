package harness

// ref_query.go — L1-3-IMPL-C 工具面分发：ref.query / ref.diff（QUERY_ENGINE
// §2.3 工具面 JSON Schema 的 Go 落地面）。
//
// 三条边界（§1/§4，工具面版）：
//  1. 纯只读——bootstrap 适配器只扫盘，不触发 finalize/现算（扫描不到即如实
//     空行集）；
//  2. 不装配披露——响应只回 refs+声明标量+五元数据（cost_class/degraded/
//     snapshot/next_cursor/total_matches），无 LLMContext/预算/审计；
//  3. 校验 fail-closed（T10）——未知字段拒绝、limit 1..500、至少一段谓词
//     必填（防"全库拉取"）、ref.diff base 侧恰选一、depth=content 显式拒绝
//     （IMPL-D 未接线，不静默降级）。
//
// 引擎生命周期：按解析出的 BootstrapConfig 惰性构建并缓存（工程切换=配置指纹
// 变化→重建）；每次调用前 Sync 强制从盘面重建中央索引（on-demand 新鲜度，
// 不启动后台轮询 goroutine——泊位零常驻进程）。

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/queryengine"
)

// refQueryAllowedKeys ref.query 输入键白名单（与 tools/catalog.go refQuerySpec
// properties 同步维护；未知键拒绝防拼写漂移到引擎语义之外）。
var refQueryAllowedKeys = []string{
	"cmd", "query_id", "kinds", "scope_kind", "scope_values", "scope_value_prefix",
	"time_window", "snapshot", "payload_conditions", "sort", "limit", "cursor",
	"expand", "expand_max_bytes",
}

// refDiffAllowedKeys ref.diff 输入键白名单（base/head/depth + ref.query 同构
// 限定谓词子集）。
var refDiffAllowedKeys = []string{
	"cmd", "base_revision", "base_observation_id", "head", "depth",
	"kinds", "scope_kind", "scope_values", "scope_value_prefix", "time_window",
}

// refQueryEngineHandle 缓存的引擎句柄（bootstrap store + engine 同配置成对）。
type refQueryEngineHandle struct {
	store  *queryengine.BootstrapStore
	engine *queryengine.Engine
}

// refQueryBootstrapConfig 解析 v0 bootstrap 扫描配置：观察票/桥快照=当前激活
// 工程的 .vit_agent 根（<uuid>/observations 的父目录，canonical v2 布局）；
// 声学包=默认 store 解析（与 acousticpackage.NewStore("") 同口径）；COM 工件
// =VIT_DAW_DEV_ROOT 族 env（与 comEvidenceArtifactPath 同族）。路径缺席=该
// 产物族空集（如实 empty，不报错）。
func refQueryBootstrapConfig() queryengine.BootstrapConfig {
	cfg := queryengine.BootstrapConfig{
		AcousticStore: acousticpackage.DefaultStorePath(nil),
	}
	if roots, ok := projectstore.Current(); ok {
		cfg.AgentRoot = filepath.Dir(roots.Agent)
	}
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT"} {
		if root := strings.TrimSpace(os.Getenv(key)); root != "" {
			cfg.COMEvidenceDir = filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence")
			break
		}
	}
	return cfg
}

func refQueryConfigKey(cfg queryengine.BootstrapConfig) string {
	return strings.Join([]string{cfg.AgentRoot, cfg.AcousticStore, cfg.COMEvidenceDir}, "\x1f")
}

// refQueryEngine 取（或按配置指纹重建）引擎句柄。refQueryConfigOverride 仅
// 测试注入（隔离临时扫描面）；生产路径恒走 refQueryBootstrapConfig。
func (h *Harness) refQueryEngine() (*refQueryEngineHandle, error) {
	cfg := refQueryBootstrapConfig()
	if h.refQueryConfigOverride != nil {
		cfg = *h.refQueryConfigOverride
	}
	key := refQueryConfigKey(cfg)
	h.refQueryMu.Lock()
	defer h.refQueryMu.Unlock()
	if h.refQueryHandle != nil && h.refQueryConfigKey == key {
		return h.refQueryHandle, nil
	}
	store := queryengine.NewBootstrapStore(cfg)
	// v0 无局部载荷索引：R5/R6 降级是常态——降级在响应明示（§3.2），不是异常。
	engine := queryengine.NewEngine(store, nil)
	handle := &refQueryEngineHandle{store: store, engine: engine}
	h.refQueryHandle = handle
	h.refQueryConfigKey = key
	return handle, nil
}

// refHarnessManagedKeys invoke 管线在 invokeLocal 前注入的执行元数据键
// （harness.go Invoke 路径；非模型输入面）——校验前剥离，白名单只管模型参数。
var refHarnessManagedKeys = []string{
	"agent_action_id", "run_id", "goal_id", "tool_call_id", "undo_label",
	"confirmation", "confirmed",
}

func rejectUnknownRefKeys(tool string, cmd map[string]any, allowed []string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for _, key := range refHarnessManagedKeys {
		allowedSet[key] = struct{}{}
	}
	var unknown []string
	for key := range cmd {
		if _, ok := allowedSet[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("%s: unknown field(s) %s rejected (allowed: %s)", tool, strings.Join(unknown, ", "), strings.Join(allowed, ", "))
	}
	return nil
}

// ---- ref.query ----

// refQuery 分发入口：解析（T10 校验）→ Sync → Engine.Query → 五元数据响应。
func (h *Harness) refQuery(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if err := rejectUnknownRefKeys("ref.query", cmd, refQueryAllowedKeys); err != nil {
		return nil, err
	}
	q, narrowed, err := h.parseRefQuery(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if !narrowed {
		return nil, fmt.Errorf("ref.query: empty predicate set rejected (at least one of kinds/scope/time_window(object)/payload_conditions is required; full-store pulls are not allowed)")
	}
	handle, err := h.refQueryEngine()
	if err != nil {
		return nil, err
	}
	if err := handle.engine.Sync(ctx); err != nil {
		return nil, fmt.Errorf("ref.query: sync materialized view: %w", err)
	}
	result, err := handle.engine.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return refQueryResponse(result), nil
}

// parseRefQuery 把工具面 JSON 参数译成 RefQuery（接口冻结：引擎签名不动）。
// narrowed 报告是否含至少一段收窄谓词（T10 空谓词判定）。
func (h *Harness) parseRefQuery(ctx context.Context, cmd map[string]any) (queryengine.RefQuery, bool, error) {
	var q queryengine.RefQuery
	narrowed := false

	if values := stringSliceFromAny(cmd["kinds"]); len(values) > 0 {
		q.Kinds = queryengine.KindPredicate{Kinds: values}
		narrowed = true
	}

	scopeKind := firstString(cmd, "scope_kind")
	scopeValues := stringSliceFromAny(cmd["scope_values"])
	scopePrefix := firstString(cmd, "scope_value_prefix")
	if scopeKind != "" || len(scopeValues) > 0 || scopePrefix != "" {
		q.Scope = &queryengine.ScopePredicate{
			Kind:      scopeKind,
			Values:    scopeValues,
			Prefix:    scopePrefix,
			ValueSet:  len(scopeValues) > 0,
			PrefixSet: scopePrefix != "",
		}
		narrowed = true
	}

	if raw := cmd["time_window"]; raw != nil {
		window, isNarrowing, err := h.parseRefTimeWindow(ctx, raw)
		if err != nil {
			return q, false, err
		}
		if window != nil {
			q.Window = window
		}
		if isNarrowing {
			narrowed = true
		}
	}

	if snapshot := firstString(cmd, "snapshot"); snapshot != "" && snapshot != "latest" {
		return q, false, fmt.Errorf("ref.query: snapshot %q rejected (v1 supports latest only; snapshot backtracking is deferred)", snapshot)
	}

	if rows := mapRowsFromAny(cmd["payload_conditions"]); len(rows) > 0 {
		if len(rows) > 8 {
			return q, false, fmt.Errorf("ref.query: payload_conditions maxItems=8, got %d", len(rows))
		}
		conditions := make([]queryengine.PayloadCondition, 0, len(rows))
		for _, row := range rows {
			field := firstString(row, "field")
			if field == "" {
				return q, false, fmt.Errorf("ref.query: payload condition missing field")
			}
			condition := queryengine.PayloadCondition{
				Field: field,
				Op:    queryengine.PayloadOp(firstString(row, "op")),
				Value: numberFromAny(row["value"]),
				Str:   firstString(row, "str"),
				Set:   stringSliceFromAny(row["set"]),
			}
			conditions = append(conditions, condition)
		}
		q.Payload = conditions
		narrowed = true
	}

	if rows := mapRowsFromAny(cmd["sort"]); len(rows) > 0 {
		if len(rows) > 2 {
			return q, false, fmt.Errorf("ref.query: sort maxItems=2, got %d", len(rows))
		}
		for _, row := range rows {
			q.Sort = append(q.Sort, queryengine.SortKey{
				Field: firstString(row, "field"),
				Dir:   queryengine.SortDirection(firstString(row, "dir")),
			})
		}
	}

	if raw, ok := cmd["limit"]; ok && raw != nil {
		limit := numberFromAny(raw)
		if limit != math.Trunc(limit) || limit < 1 || limit > queryengine.MaxQueryLimit {
			return q, false, fmt.Errorf("ref.query: limit %v out of range (1..%d; omit for default %d)", raw, queryengine.MaxQueryLimit, queryengine.DefaultQueryLimit)
		}
		q.Limit = int(limit)
	}

	q.Cursor = firstString(cmd, "cursor")

	switch expand := firstString(cmd, "expand"); expand {
	case "", "none":
	case "handle", "summary":
		opt := queryengine.ExpandOptions{Depth: queryengine.ExpandDepth(expand)}
		if raw, ok := cmd["expand_max_bytes"]; ok && raw != nil {
			maxBytes := numberFromAny(raw)
			if maxBytes != math.Trunc(maxBytes) || maxBytes < 256 || maxBytes > 32768 {
				return q, false, fmt.Errorf("ref.query: expand_max_bytes %v out of range (256..32768)", raw)
			}
			opt.MaxBytes = int(maxBytes)
		}
		q.Expand = &opt
	default:
		return q, false, fmt.Errorf("ref.query: expand %q rejected (none|handle|summary)", expand)
	}

	return q, narrowed, nil
}

// parseRefTimeWindow 解析 §2.3 双标尺窗口："all"/缺省 → (nil,false)（全窗，
// 不收窄谓词）；对象 → WindowPredicate（收窄）。seconds 需工程采样率（kernel
// audio settings 只读命令）；不可得即 fail-closed，指导改用 samples——不做
// 静默假换算。
func (h *Harness) parseRefTimeWindow(ctx context.Context, raw any) (*queryengine.WindowPredicate, bool, error) {
	if text, ok := raw.(string); ok {
		if strings.EqualFold(strings.TrimSpace(text), "all") {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("ref.query: time_window %q rejected (\"all\" or {start_seconds,end_seconds,units})", text)
	}
	row := mapFromAny(raw)
	if len(row) == 0 {
		return nil, false, fmt.Errorf("ref.query: time_window must be \"all\" or an object with start_seconds/end_seconds")
	}
	units := firstString(row, "units")
	if units == "" {
		units = "seconds"
	}
	start := numberFromAny(row["start_seconds"])
	end := numberFromAny(row["end_seconds"])
	if _, ok := row["start_seconds"]; !ok {
		return nil, false, fmt.Errorf("ref.query: time_window missing start_seconds")
	}
	if _, ok := row["end_seconds"]; !ok {
		return nil, false, fmt.Errorf("ref.query: time_window missing end_seconds")
	}
	switch units {
	case "samples":
		return &queryengine.WindowPredicate{
			SampleStart: int64(start), SampleEnd: int64(end),
		}, true, nil
	case "seconds":
		rate, err := h.refQuerySampleRate(ctx)
		if err != nil {
			return nil, false, err
		}
		return &queryengine.WindowPredicate{
			SampleStart: int64(math.Round(start * rate)),
			SampleEnd:   int64(math.Round(end * rate)),
		}, true, nil
	default:
		return nil, false, fmt.Errorf("ref.query: time_window units %q rejected (seconds|samples)", units)
	}
}

// refQuerySampleRate 从 kernel 只读命令取工程采样率（seconds→samples 换算的
// 唯一权威源；引擎只见采样点，D3）。
func (h *Harness) refQuerySampleRate(ctx context.Context) (float64, error) {
	if h.kernel == nil {
		return 0, fmt.Errorf("ref.query: time_window units=seconds requires the kernel project sample rate, which is unavailable; pass units=samples")
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "project.get_audio_settings"})
	if err != nil {
		return 0, fmt.Errorf("ref.query: kernel audio settings unavailable for seconds conversion (%w); pass units=samples", err)
	}
	rate := numberFromAny(mapFromAny(reply["audio_settings"])["sample_rate_hz"])
	if rate <= 0 {
		return 0, fmt.Errorf("ref.query: project sample_rate_hz unavailable for seconds conversion; pass units=samples")
	}
	return rate, nil
}

// refQueryResponse 塑形五元数据响应（§2.3：成本分级是硬要求，不是装饰字段）。
func refQueryResponse(result queryengine.QueryResult) map[string]any {
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		out := map[string]any{
			"ref":       row.Ref,
			"freshness": row.Freshness,
			"payload":   row.Payload,
		}
		if row.Expanded != nil {
			expanded := map[string]any{
				"handle":    row.Expanded.Handle,
				"bytes":     row.Expanded.Bytes,
				"freshness": row.Expanded.Freshness,
				"truncated": row.Expanded.Truncated,
			}
			if row.Expanded.Summary != nil {
				expanded["summary"] = row.Expanded.Summary
			}
			out["expanded"] = expanded
		}
		rows = append(rows, out)
	}
	return map[string]any{
		"status":        "ok",
		"rows":          rows,
		"total_matches": result.TotalMatches,
		"next_cursor":   result.NextCursor,
		"snapshot":      result.Snapshot,
		"cost_class":    result.CostClass,
		"degraded":      result.Degraded,
	}
}

// ---- ref.diff ----

// refDiff 分发入口：base 恰选一校验 → head=latest 盘面物化 → identity 集合差。
func (h *Harness) refDiff(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if err := rejectUnknownRefKeys("ref.diff", cmd, refDiffAllowedKeys); err != nil {
		return nil, err
	}
	baseRevision := firstString(cmd, "base_revision")
	baseObservationID := firstString(cmd, "base_observation_id")
	if (baseRevision == "") == (baseObservationID == "") {
		return nil, fmt.Errorf("ref.diff: exactly one of base_revision / base_observation_id is required (got revision=%q observation_id=%q)", baseRevision, baseObservationID)
	}
	if head := firstString(cmd, "head"); head != "" && head != "latest" {
		return nil, fmt.Errorf("ref.diff: head %q rejected (v1 head is always latest)", head)
	}
	depth := firstString(cmd, "depth")
	if depth == "" {
		depth = string(queryengine.DiffIdentity)
	}
	switch queryengine.DiffDepth(depth) {
	case queryengine.DiffIdentity:
	case queryengine.DiffContent:
		// IMPL-D 未接线：显式拒绝（诚实边界），不静默降级为 identity。
		return nil, fmt.Errorf("ref.diff: depth=content is not implemented yet (delegation to existing diff bearers lands in IMPL-D); use depth=identity")
	default:
		return nil, fmt.Errorf("ref.diff: depth %q rejected (identity|content)", depth)
	}

	var scope *queryengine.RefQuery
	if rawScope, err := h.parseRefDiffScope(ctx, cmd); err != nil {
		return nil, err
	} else {
		scope = rawScope
	}

	handle, err := h.refQueryEngine()
	if err != nil {
		return nil, err
	}
	if err := handle.engine.Sync(ctx); err != nil {
		return nil, fmt.Errorf("ref.diff: sync materialized view: %w", err)
	}
	// head=latest 的物化：latest 视图全量行 → canonical ref 集（引擎签名冻结，
	// 不为工具面改 DiffEvidence；head 侧经显式 Refs 进入同一坐标对齐路径）。
	headRows, err := handle.store.SnapshotView(ctx, queryengine.SnapshotSelector{Mode: queryengine.SnapshotLatest})
	if err != nil {
		return nil, fmt.Errorf("ref.diff: latest view: %w", err)
	}
	headRefs := make([]string, 0, len(headRows))
	for _, row := range headRows {
		canonical, err := agentprotocol.FormatRef(row.Ref)
		if err != nil {
			return nil, fmt.Errorf("ref.diff: canonicalize head ref: %w", err)
		}
		headRefs = append(headRefs, canonical)
	}
	// 空盘面短路：head=latest 视图为空时（bootstrap 语义：exact 视图是同一次
	// 扫描的子集，latest 空 ⇒ base 侧必空），identity 差分退化为全空报告——
	// 引擎签名冻结（SnapshotRefSet 空 Refs 不可与"未选"区分），此处确定性
	// 返回，不伪造非空结果。
	if len(headRefs) == 0 {
		return map[string]any{
			"status":          "ok",
			"added":           []string{},
			"removed":         []string{},
			"changed":         []map[string]any{},
			"unchanged_count": 0,
			"cost_class":      queryengine.CostClassIndex,
		}, nil
	}

	report, err := handle.engine.DiffEvidence(ctx, queryengine.DiffRequest{
		Base:  queryengine.SnapshotRefSet{Revision: baseRevision, ObservationID: baseObservationID},
		Head:  queryengine.SnapshotRefSet{Refs: headRefs},
		Scope: scope,
		Depth: queryengine.DiffIdentity,
	})
	if err != nil {
		return nil, err
	}
	changed := make([]map[string]any, 0, len(report.Changed))
	for _, pair := range report.Changed {
		changed = append(changed, map[string]any{"base": pair.Base, "head": pair.Head})
	}
	return map[string]any{
		"status":          "ok",
		"added":           report.Added,
		"removed":         report.Removed,
		"changed":         changed,
		"unchanged_count": report.UnchangedCount,
		"cost_class":      report.CostClass,
	}, nil
}

// parseRefDiffScope 解析差分限定谓词（ref.query 同构子集；任一 presence 才建）。
func (h *Harness) parseRefDiffScope(ctx context.Context, cmd map[string]any) (*queryengine.RefQuery, error) {
	scoped := cmd["kinds"] != nil || cmd["scope_kind"] != nil || cmd["scope_values"] != nil ||
		cmd["scope_value_prefix"] != nil || cmd["time_window"] != nil
	if !scoped {
		return nil, nil
	}
	scopeCmd := cloneAnyMap(cmd)
	scopeCmd["cmd"] = "ref_query"
	scope, _, err := h.parseRefQuery(ctx, scopeCmd)
	if err != nil {
		return nil, fmt.Errorf("ref.diff scope: %w", err)
	}
	return &scope, nil
}
