package materialize

// propagate.go — 事件接线与脏传播（MAT-B，设计 §3.1/§4.2/§4.3）。
//
// 三个入口对应三个挂点（§3.1，harness/shadow 侧纯 if-n-nil 尾挂，本包是物化层
// 实现）：
//
//	HandleReceipt        挂点 1——shadow ApplyDelta/Initialize 的 ChangeReceipt 后
//	                     （invalidate 型：表 A scopes→表 B kinds→行坐标匹配）
//	HandleFeatureArrival 挂点 2——IngestKernelTelemetry 四分支+#9 收编分支写完
//	                     snapshot 后（arrive 型：事件分类→arrive 域→表 B）
//	HandleRenderJob      挂点 3——ingestKernelRenderTelemetry 唤醒 waiters 前
//	                     （批 3：render 换代→全 kind 保守标脏）
//
// 两类事件在脏传播算法里同构（都进 dirty 集合，§3.2），差别只在 metrics 计数。
// 宁多勿漏（§4.2）：未知域/空域/无法收窄的 track 域一律放大标脏范围——失效正确性
// 的第一敌人是漏标，过标只是浪费一次 lazy 重算。标记一律走 MAT-A 的 MarkStale
// 原语（不删行/幂等/changeID 记账）。
//
// v1 边界（§4.4）：window 段不参与匹配（整投影重算）；track 域按 ScopeValue 匹配
// 行（clip→track 坐标映射留 v2）；级联经 kind 输出域（<kind>.output 约定）传递
// 闭包——生产表 B 无 kind 输出域条目（§4.1 E1/E2 v1 不进图），闭包在生产表上
// 是单跳，机制由 G2-A① 合成链测试锁定。

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/shadow"
)

// kindOutputSuffix 是 kind 输出域的约定后缀：kind K 变脏 ⇒ 探测表 B 的
// "K.output" 键，命中则依赖该输出域的 kind 级联标脏（whole-project 保守档）。
const kindOutputSuffix = ".output"

// Notifier 是挂点侧（harness）持有的物化层通知接口（§3.1：harness 持接口、物化
// 层实现、尾挂 if n != nil——nil 时与现状逐字节一致，由 harness 侧
// TestNotifierNilIsByteIdentical 锁定）。
type Notifier interface {
	// NotifyShadowChange：ChangeReceipt 生成后（挂点 1）。实现不得回调 shadow
	// （观察者在 p.mu 内被调用）。
	NotifyShadowChange(receipt shadow.ChangeReceipt)
	// NotifyTelemetry：IngestKernelTelemetry 尾挂转发原始遥测事件（挂点 2，
	// 含 #9 收编分支）；render 主题事件由 NotifyRenderJob 专职，本方法忽略。
	NotifyTelemetry(event map[string]any)
	// NotifyRenderJob：render 终态遥测（挂点 3）。
	NotifyRenderJob(jobID, status, filePath string)
}

// Notifier 返回本 Store 的 Notifier 适配器（三方法直通三个 Handle 入口）。
func (s *Store) Notifier() Notifier { return storeNotifier{store: s} }

type storeNotifier struct{ store *Store }

func (n storeNotifier) NotifyShadowChange(receipt shadow.ChangeReceipt) {
	n.store.HandleReceipt(receipt)
}

func (n storeNotifier) NotifyTelemetry(event map[string]any) { n.store.HandleFeatureArrival(event) }

func (n storeNotifier) NotifyRenderJob(jobID, status, filePath string) {
	n.store.HandleRenderJob(jobID, status, filePath)
}

// propagationTable 返回本 Store 生效的表 B（测试注入覆盖优先；nil=生产表）。
func (s *Store) propagationTable() map[string][]string {
	if s.propagationOverride != nil {
		return s.propagationOverride
	}
	return tableBIndex
}

// newStoreWithTableB 构造带测试表 B 的 Store（G2-A 合成链：测试条目表达
// kind 输出域依赖边，§5.2）。
func newStoreWithTableB(entries []TableBEntry) *Store {
	s := NewStore()
	s.propagationOverride = buildTableBIndex(entries)
	return s
}

// ---------------------------------------------------------------------------
// 挂点 1：ChangeReceipt → 脏传播（invalidate 型，§4.3）
// ---------------------------------------------------------------------------

// HandleReceipt 消费 shadow ChangeReceipt：AffectedScopes（表 A 输出，零改造）
// → 表 B kinds → 行坐标匹配 → MarkStale。未知域/空域 → 全行兜底标脏
// （宁多勿漏）。#12/#14 权威快照receipt 同路径（其 scopes 已按表 A 展开，
// 传播即"按依赖域重估"）；空 diff 的快照不产收据、无通知——存量行保持
// material_reuse 语义（§2.1），不属漏标。
func (s *Store) HandleReceipt(receipt shadow.ChangeReceipt) {
	allKinds, trackKinds, resolved := resolveDirtyKinds(s.propagationTable(), receipt.AffectedScopes)
	if !resolved {
		_, _ = s.MarkStale(RowMatch{}, receipt.ChangeID)
		return
	}
	if len(trackKinds) > 0 {
		match := RowMatch{Kinds: trackKinds}
		if trackIDs := receiptTrackIDs(receipt.ChangedEntities); len(trackIDs) > 0 {
			// track 域收窄：仅 ScopeValue∈变更 track 集的行；收不窄（无实体）则
			// 宁多勿漏全行（match 不带 ScopeValues）。
			match.ScopeValues = trackIDs
		}
		_, _ = s.MarkStale(match, receipt.ChangeID)
	}
	if len(allKinds) > 0 {
		_, _ = s.MarkStale(RowMatch{Kinds: allKinds}, receipt.ChangeID)
	}
}

// ---------------------------------------------------------------------------
// 挂点 2：遥测特征事件 → arrive 域（§3.2/§4.2）
// ---------------------------------------------------------------------------

// HandleFeatureArrival 分类遥测事件（#4/#5/#6-#8/#9，RECON §3.1 词表）→ arrive
// 域 → 表 B → MarkStale（arrive 型脏=现在可重算了）。render 主题事件忽略（挂点
// 3 专职）；未识别事件（#15-#19 UI 分发面等）不动作——分类词表与表 B arrive 域
// 同源（TestTableBInitialValuesLocked 锁定）。
func (s *Store) HandleFeatureArrival(event map[string]any) {
	domain, trackID := classifyFeatureArrival(event)
	if domain == "" {
		return
	}
	allKinds, trackKinds, resolved := resolveDirtyKinds(s.propagationTable(), []string{domain})
	if !resolved {
		// 分类命中的域必然在表 B（同源词表）；防御性兜底（宁多勿漏）。
		s.MarkStale(RowMatch{}, arrivalChangeID(event))
		return
	}
	dirty := append(append([]string(nil), allKinds...), trackKinds...)
	for _, kind := range dirty {
		s.metrics.addArrival(kind)
	}
	match := RowMatch{Kinds: dirty}
	if trackID != "" {
		match.ScopeValues = []string{trackID}
	}
	_, _ = s.MarkStale(match, arrivalChangeID(event))
}

// classifyFeatureArrival 返回（arrive 域, track_id）。域为空=未识别/非本入口
// 事件。分类镜像 IngestKernelTelemetry 四分支词表（harness.go:3900-3911）+#
// 收编分支。
func classifyFeatureArrival(event map[string]any) (domain, trackID string) {
	command := strings.ToLower(strings.TrimSpace(eventString(event, "command", "cmd")))
	featureType := strings.ToLower(strings.TrimSpace(eventString(event, "feature_type")))
	if strings.EqualFold(eventString(event, "topic"), "render") {
		return "", "" // render 换代归挂点 3（NotifyRenderJob）
	}
	switch {
	case command == "audio_feature_data_ready" && featureType == "waveform_envelope":
		return "feature.waveform", eventString(event, "track_id", "source_track_id")
	case command == "tile_ready" && (featureType == "" || featureType == "spectral_field"):
		return "feature.spectral", eventString(event, "track_id", "source_track_id")
	case command == "audio_feature_data_ready":
		switch featureType {
		case "band_energy_summary":
			return "acoustic.l3.band_energy", eventString(event, "track_id", "source_track_id")
		case "stereo_relation_summary":
			return "acoustic.l3.stereo_relation", eventString(event, "track_id", "source_track_id")
		case "loudness_summary":
			return "acoustic.l3.loudness", eventString(event, "track_id", "source_track_id")
		}
	case command == "l2_render_probe_ready" && featureType == "l2_render_probe":
		return "feature.l2_render_probe", eventString(event, "track_id", "source_track_id")
	}
	return "", ""
}

// arrivalChangeID 生成 arrive 事件的失效记账 ID（request_id 优先，§4.3 步 5）。
func arrivalChangeID(event map[string]any) string {
	if requestID := eventString(event, "request_id"); requestID != "" {
		return "arrival:" + requestID
	}
	domain, trackID := classifyFeatureArrival(event)
	id := "arrival:" + domain
	if trackID != "" {
		id += ":" + trackID
	}
	return id
}

// ---------------------------------------------------------------------------
// 挂点 3：render 终态（批 3，§2.2）
// ---------------------------------------------------------------------------

// HandleRenderJob：render_done/render_failed = render 换代（take 落地/测量基准
// 整体失真）→ 全 kind 保守标脏（粗粒度兜底；#3 同时是 com paired 登记行的
// render_revision 失效源——登记行同样标脏，M6 消费时校验兜底）。
func (s *Store) HandleRenderJob(jobID, status, filePath string) {
	_, _ = s.MarkStale(RowMatch{}, "render_job:"+strings.TrimSpace(jobID))
}

// ---------------------------------------------------------------------------
// 传播核心：域→kind 解析（级联闭包+兜底语义）
// ---------------------------------------------------------------------------

// resolveDirtyKinds 把域列表解析为两个脏 kind 桶：
//
//	allKinds    全曲域桶——匹配时不收窄 scope（mix.*/project.*/processor.*/级联）
//	trackKinds  track 域桶——调用方按变更 track 集收窄 ScopeValue
//	resolved=false 事件输入域列表为空或任一输入域查表落空 → 调用方全行兜底
//	（宁多勿漏）
//
// 级联闭包（§5.2 G2-A①）：每个新变脏的 kind K 探测表 B 的 "K.output" 键，命中
// 则依赖该输出域的 kind 一并变脏（入全曲桶，保守），迭代至不动点；visited 防环。
// 输出域探测落空是常态而非未知域——生产表无此类条目（§4.1 E1/E2 v1 不进图），
// 闭包在生产表上单跳；测试表用它表达合成链依赖边。
func resolveDirtyKinds(table map[string][]string, scopes []string) (allKinds, trackKinds []string, resolved bool) {
	domains := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if domain := strings.ToLower(strings.TrimSpace(scope)); domain != "" {
			domains = append(domains, domain)
		}
	}
	if len(domains) == 0 {
		return nil, nil, false
	}
	allSet := map[string]bool{}
	trackSet := map[string]bool{}
	visited := map[string]bool{}
	type domainProbe struct {
		domain   string
		fromKind bool // true=kind 输出域级联探测（落空=常态，跳过）；false=事件输入域（落空=兜底）
	}
	queue := make([]domainProbe, 0, len(domains))
	for _, domain := range domains {
		queue = append(queue, domainProbe{domain: domain})
	}
	for len(queue) > 0 {
		probe := queue[0]
		queue = queue[1:]
		if visited[probe.domain] {
			continue
		}
		visited[probe.domain] = true
		kinds, ok := table[probe.domain]
		if !ok {
			if probe.fromKind {
				continue // 输出域探测落空：无投影依赖该 kind 的输出（生产常态）
			}
			return nil, nil, false // 未知输入域 → 兜底（宁多勿漏）
		}
		for _, kind := range kinds {
			kind = strings.TrimSpace(kind)
			if kind == "" || allSet[kind] {
				continue
			}
			if strings.HasPrefix(probe.domain, "track.") {
				if trackSet[kind] {
					continue
				}
				trackSet[kind] = true
			} else {
				// 全曲域命中升格：kind 同时被 track 域与全曲域命中时，全曲语义
				// 覆盖 track 收窄（匹配集取并集=宽者胜）。
				delete(trackSet, kind)
				allSet[kind] = true
			}
			queue = append(queue, domainProbe{domain: kind + kindOutputSuffix, fromKind: true})
		}
	}
	return sortedKeys(allSet), sortedKeys(trackSet), true
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// receiptTrackIDs 提取收据中 track 实体的 ID 集（scope 收窄用；§4.3 步 3）。
func receiptTrackIDs(entities []shadow.ChangeEntity) []string {
	var out []string
	for _, entity := range entities {
		if !strings.EqualFold(strings.TrimSpace(entity.Kind), "track") {
			continue
		}
		if id := strings.TrimSpace(entity.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// eventString 取事件里第一个非空字符串字段（harness firstString 同款语义的
// 本包只读版）。
func eventString(event map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := event[key]; ok {
			if text := strings.TrimSpace(valueString(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func valueString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}
