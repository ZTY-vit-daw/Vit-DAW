package harness

// ref_diff.go — L1-3-IMPL-D：ref.diff depth=content 委托接线（QUERY_ENGINE
// §2.5 设计立场的工具面落地）。
//
// 设计立场：载荷级差分一律委托既有差分承载者，不新建差分算法、不造第二套
// 差分真相——本文件只做"找到对的差分证据并给出句柄"，按 kind 路由：
//   - com.change_delta        观察票 com_projection（mode=change_delta，Before/
//                             After projection ID + behavior_change 维度差分；
//                             锚定校验 com/change.go compareChangeChildren 十六门：
//                             同源/同精确窗/同 tap/render_revision 必变）
//   - com.paired_artifact     COM paired 工件行 dad.compressor_dual_tap（Before/
//                             After 双 tap + 精确采样窗锚定，com/paired.go
//                             validatePairedArtifact；句柄=工件路径）
//   - fxm.ab                  观察票 fxm_projection（baseline/processed 测量 +
//                             effect_delta 差分值；same_source/same_window/
//                             same_format 门，fxm/projection.go Build）
//   - observation.before_after 观察票 mix_package.current_metrics.before_after_delta
//                             + ab_result（tap/render_revision 门在 ab_result 的
//                             quality_gates——mixboard.go abResultQualityGate；
//                             before_after_delta 本身仅门"前观察存在"）
//
// 分层裁定（接口冻结申报，详见设计文档 §2.5 修订段）：委托接线落 harness 工具层
// 而非 queryengine——承载者句柄面是 bootstrap/store 侧知识（票内 JSON 键、
// com_evidence 工件目录），引擎 MaterializedStore 契约只暴露 Resolve（handle
// +ReadAll），在引擎内解析承载者内部键会复制 bootstrap 解析知识（D2 分界）。
// 引擎 DiffEvidence 保持 identity-only；DiffRequest/DiffReport 签名零改动
// （IMPL-C 注记①的引擎面预授权未动用）。
//
// 成本分级（R10，route.go RouteDiff）：depth=content 恒 compile——委托是设计
// 路径非降级，degraded 恒空（与 R5/R6 降级口径区分）。

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/queryengine"
)

// refDiffCarrier 承载者名（响应 delegated 键；与卡面/设计 §2.5 三承载者对应，
// paired 工件单列——句柄形态与票不同）。
const (
	refDiffCarrierComChangeDelta = "com.change_delta"
	refDiffCarrierComPaired      = "com.paired_artifact"
	refDiffCarrierFXMAB          = "fxm.ab"
	refDiffCarrierBeforeAfter    = "observation.before_after"
)

// refDiffCarrierByKind kind → 承载者坐标路由表。无载者 kind 进 unrouted_kinds
// 如实报空（不伪造句柄、不静默吞掉）。
var refDiffCarrierByKind = map[string]string{
	"com":                     refDiffCarrierComChangeDelta,
	"fxm":                     refDiffCarrierFXMAB,
	"dad.compressor_dual_tap": refDiffCarrierComPaired,
}

// refDiffCarrierNotes 承载者注记：锚定校验面 + 差分内容所在键——模型按注记
// 决定是否对句柄深读（expand/read），引擎不复述差分内容。
var refDiffCarrierNotes = map[string]string{
	refDiffCarrierComChangeDelta: "COM change_delta 投影载于观察票 com_projection（behavior_change 维度差分 + before/after projection ID；锚定校验：同源/同精确采样窗/同 tap/render_revision 必变，com/change.go compareChangeChildren）",
	refDiffCarrierComPaired:      "COM paired 工件（Before/After 双 tap：compressor_input/compressor_output 同精确采样窗 + render_revision 锚定，com/paired.go validatePairedArtifact）；句柄=工件路径",
	refDiffCarrierFXMAB:          "FXM A/B 投影载于观察票 fxm_projection（baseline_ref/processed_ref 各携 render_revision + effect_delta 差分值；门=same_source/same_window/same_format，fxm/projection.go）",
	refDiffCarrierBeforeAfter:    "观察票 mix_package.current_metrics：before_after_delta（前后观察 metrics 块差）+ ab_result（mom_ab_result.v1，tap/render_revision 门在其 quality_gates——same_tap_point/render_revision_changed）",
}

// refDiffInstanceHashKinds hash=实例身份（含时间戳，materialize instanceIdentityHash）
// 的 kind——identity 级 Changed ≠ 内容变化，按 kind 注记防误读（T7 口径）。
// dom/acp/dad 系=内容身份 hash（Changed 即内容变），无需注记。
var refDiffInstanceHashKinds = map[string]struct{}{
	"fxm": {},
	"com": {},
}

// refDiffTicketKinds 观察票行化 kind（bootstrap.go scanObservationTicket 现实面：
// dom/fxm/com 三投影）——before_after 承载者的票扫描面。
var refDiffTicketKinds = map[string]struct{}{
	"dom": {},
	"fxm": {},
	"com": {},
}

// refDiffChangedEntries 塑形 changed 对：附 kind 与 hash 语义注记（fxm/com 实例
// 身份——时间戳入哈希，identity Changed 不等于内容变化；内容判定走 depth=content
// 委托）。identity 与 content 两深度共用。
func refDiffChangedEntries(pairs []queryengine.RefPair) []map[string]any {
	out := make([]map[string]any, 0, len(pairs))
	for _, pair := range pairs {
		entry := map[string]any{"base": pair.Base, "head": pair.Head}
		kind := refDiffRefKind(pair.Head)
		if kind != "" {
			entry["kind"] = kind
			if _, instance := refDiffInstanceHashKinds[kind]; instance {
				entry["hash_semantics"] = "instance_identity"
			}
		}
		out = append(out, entry)
	}
	return out
}

func refDiffRefKind(raw string) string {
	parsed, err := agentprotocol.ParseRef(raw)
	if err != nil || parsed.State != agentprotocol.RefStateParsed || parsed.Ref == nil {
		return ""
	}
	return parsed.Ref.Kind
}

// refDiffDelegated 对 identity 差分产物做承载者委托映射（纯只读：Resolve 句柄
// + 票内承载者锚定键提取；不重算任何差分）。unrouted 返回无承载者的 kind 集
// （排序去重）。changed/added/removed 全空时 delegated 为空 map（如实）。
func refDiffDelegated(ctx context.Context, handle *refQueryEngineHandle, report queryengine.DiffReport, baseSelector string) (map[string]any, []string) {
	resolveHandle := func(raw string) string {
		parsed, err := agentprotocol.ParseRef(raw)
		if err != nil || parsed.State != agentprotocol.RefStateParsed || parsed.Ref == nil {
			return ""
		}
		resolved, err := handle.store.Resolve(ctx, *parsed.Ref)
		if err != nil {
			return "" // 坐标不可解析（盘面无行）：句柄缺席如实，不伪造
		}
		return resolved.Handle
	}

	pairs := map[string][]map[string]any{} // carrier → changed 对（base+head 双句柄）
	items := map[string][]map[string]any{} // carrier → added/removed 单侧句柄
	ticketHandles := map[string]struct{}{} // before_after 扫描面（票去重）
	unrouted := map[string]struct{}{}

	route := func(kind, baseRef, headRef, handle string) {
		carrier, ok := refDiffCarrierByKind[kind]
		if !ok {
			if kind != "" {
				unrouted[kind] = struct{}{}
			}
			return
		}
		if baseRef != "" {
			pairs[carrier] = append(pairs[carrier], map[string]any{
				"base_ref": baseRef, "base_handle": resolveHandle(baseRef),
				"head_ref": headRef, "head_handle": resolveHandle(headRef),
			})
			return
		}
		items[carrier] = append(items[carrier], map[string]any{"ref": headRef, "handle": handle})
	}

	for _, pair := range report.Changed {
		kind := refDiffRefKind(pair.Head)
		route(kind, pair.Base, pair.Head, "")
		noteTicket(ticketHandles, kind, resolveHandle(pair.Base))
		noteTicket(ticketHandles, kind, resolveHandle(pair.Head))
	}
	for _, added := range report.Added {
		kind := refDiffRefKind(added)
		handle := resolveHandle(added)
		route(kind, "", added, handle)
		noteTicket(ticketHandles, kind, handle)
	}
	for _, removed := range report.Removed {
		kind := refDiffRefKind(removed)
		handle := resolveHandle(removed)
		route(kind, "", removed, handle)
		noteTicket(ticketHandles, kind, handle)
	}

	// 承载者键序由 encoding/json 输出时排序（map 键确定性）；条目序跟随
	// DiffReport 的确定性序（changed 按 base 排序、added/removed canonical 排序）。
	delegated := map[string]any{}
	for carrier, entries := range pairs {
		body := map[string]any{"note": refDiffCarrierNotes[carrier], "pairs": entries}
		if extra := items[carrier]; len(extra) > 0 {
			body["items"] = extra
		}
		delegated[carrier] = body
	}
	for carrier, entries := range items {
		if _, ok := delegated[carrier]; ok {
			continue
		}
		delegated[carrier] = map[string]any{"note": refDiffCarrierNotes[carrier], "items": entries}
	}
	if tickets := refDiffBeforeAfterTickets(ticketHandles, baseSelector); len(tickets) > 0 {
		delegated[refDiffCarrierBeforeAfter] = map[string]any{
			"note":    refDiffCarrierNotes[refDiffCarrierBeforeAfter],
			"tickets": tickets,
		}
	}

	unroutedKinds := make([]string, 0, len(unrouted))
	for kind := range unrouted {
		unroutedKinds = append(unroutedKinds, kind)
	}
	sort.Strings(unroutedKinds)
	return delegated, unroutedKinds
}

// noteTicket 登记票句柄（kind ∈ 观察票行化面才可能是票；空句柄跳过）。
func noteTicket(handles map[string]struct{}, kind, handle string) {
	if handle == "" {
		return
	}
	if _, ok := refDiffTicketKinds[kind]; !ok {
		return
	}
	handles[handle] = struct{}{}
}

// refDiffBeforeAfterTickets 扫描票句柄集：提取 before_after 承载者的锚定键
// （status + 前后观察 ID + ab_result 的 render/tap 身份与 quality_gates）。
// 只给句柄与锚定面，不搬差分内容——深读走句柄。matches_base=该票 before 观察
// 与本次差分 base 对齐提示（false/缺席≠证据无效，链条证据照给，模型自判）。
func refDiffBeforeAfterTickets(handles map[string]struct{}, baseSelector string) []map[string]any {
	if len(handles) == 0 {
		return nil
	}
	paths := make([]string, 0, len(handles))
	for handle := range handles {
		paths = append(paths, handle)
	}
	sort.Strings(paths)
	tickets := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // 票消失于扫描与读取之间：如实跳过（bootstrap 下轮扫描反映）
		}
		var ticket map[string]any
		if err := json.Unmarshal(data, &ticket); err != nil {
			continue
		}
		metrics := mapFromAny(mapFromAny(ticket["mix_package"])["current_metrics"])
		entry := map[string]any{"handle": path}
		beforeID := ""
		if delta := mapFromAny(metrics["before_after_delta"]); len(delta) > 0 {
			beforeID = firstString(delta, "before_observation_id")
			entry["before_after_delta"] = map[string]any{
				"status":                firstString(delta, "status"),
				"reason":                firstString(delta, "reason"),
				"before_observation_id": beforeID,
				"after_observation_id":  firstString(delta, "after_observation_id"),
			}
		}
		if ab := mapFromAny(metrics["ab_result"]); len(ab) > 0 {
			abBefore := firstString(ab, "before_observation_id")
			if beforeID == "" {
				beforeID = abBefore
			}
			entry["ab_result"] = map[string]any{
				"status":                 firstString(ab, "status"),
				"before_observation_id":  abBefore,
				"after_observation_id":   firstString(ab, "after_observation_id"),
				"before_render_revision": firstString(ab, "before_render_revision"),
				"after_render_revision":  firstString(ab, "after_render_revision"),
				"tap_point":              firstString(ab, "tap_point"),
				"render_mode":            firstString(ab, "render_mode"),
				"quality_gates":          ab["quality_gates"],
			}
		}
		if len(entry) == 1 {
			continue // 无承载者键的票不进列表（如实，不占位）
		}
		if beforeID != "" && baseSelector != "" {
			entry["matches_base"] = beforeID == baseSelector
		}
		tickets = append(tickets, entry)
	}
	return tickets
}
