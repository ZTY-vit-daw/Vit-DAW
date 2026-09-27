package materialize

// adapters.go — 三投影适配器与行化（MAT-C，设计 §7.2/§2.1/§4.5；QUERY_ENGINE
// §3.3 行字段建议）。
//
// 三个先切 kind 及输入域（§7.2 依据）：
//
//	tom ← DepInputs.ProjectState（shadow 快照 project.structure 域，最简输入）
//	acp ← DepInputs.AcousticPackages（M4 store 快照行，rev 指纹直接映射）
//	dom ← DepInputs.FeatureSnapshot（source_only 档；装配与观察路径共享同一
//	      函数 mixboard.DOMInputFromObservation——影子对账两侧同口径是
//	      ShadowDivergences==0 可解释的前提；paired/change 类请求携带测量不
//	      预计算，F9）
//
// 行坐标约定（MAT-C 裁定，随回执申报）：precomputable 行 snapshot 段=稳定实例
// token "current"（同坐标 in-place 换代；latest 视图的换代由 generation 机制
// 承载，OQ-1 v1 只保当前代；QUERY_ENGINE §3.3 的 acp rev_hex8/tom observation_id
// 承载改由 payload+hash 携带版本信息）；registered 行（fxm/com）snapshot=
// observation_id（历史产物按观察累积，exact 可查）。
//
// hash 口径（QUERY_ENGINE §3.3）：dom/tom/acp=内容身份（volatile 字段置空——
// 时间戳/实例 ID/LLMContext 不进内容身份）；fxm/com=实例身份（含时间戳，hash
// 段即实例身份——与 dom 内容身份语义不同，diff 的 Changed 判定按 kind 注记）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/tom"
)

// snapshotTokenCurrent 是 precomputable 行 snapshot 段的稳定实例 token。
const snapshotTokenCurrent = "current"

// allTimeWindow 是 v1 行的 window 段（§4.4：window 不参与匹配，整投影重算）。
func allTimeWindow() *agentprotocol.TimeWindow {
	return &agentprotocol.TimeWindow{AllTime: true}
}

// ---------------------------------------------------------------------------
// 内容身份与实例身份 hash
// ---------------------------------------------------------------------------

// volatileContentKeys 是内容身份里置空的字段名（时间戳/实例身份/LLM 装配面）。
//
// project_revision 与 measurement_key 的置空是 MAT-C 裁定（G2-B 对拍实测发现）：
// 全域 revision 轴（及由它派生的 comparability key）不是 per-scope 的测量素材——
// 若进内容身份，任何轨的工程变更都会改写全部轨的行 hash，track 收窄失效
// （track.level 收窄到 T7 时 T3 行内容也被 revision 改写→漏标）。revision 新鲜
// 度归 freshness 状态机（收据标脏）承载，不进行内容身份。
var volatileContentKeys = map[string]bool{
	"generated_at": true, "created_at": true, "updated_at": true,
	"projection_id": true, "llm_context": true,
	"observation_id": true, "mix_session_id": true,
	"project_revision": true, "measurement_key": true,
}

func isVolatileContentKey(key string) bool {
	return volatileContentKeys[key] || strings.HasSuffix(key, "_at")
}

// stripVolatileContent 递归剔除 volatile 字段（内容身份口径，§3.3 dom"时间戳
// 置空"的推广）。map 键序由 encoding/json 排序保证 canonical。
func stripVolatileContent(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if isVolatileContentKey(key) {
				continue
			}
			out[key] = stripVolatileContent(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = stripVolatileContent(item)
		}
		return out
	default:
		return value
	}
}

// neutralizeToMap 做一次 JSON 往返把结构体转 map[string]any（字段序由键排序
// 消除；marshal 失败返回 nil——投影类型皆有 json tag，失败即缺陷）。
func neutralizeToMap(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func hashHex(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		// json.Marshal 对这些投影/标量结构不应失败；失败时以错误文本做内容
		// （确定性兜底，不静默）。
		data = []byte(fmt.Sprintf("marshal-error:%v", err))
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

// contentIdentityHash 是内容身份（volatile 置空后取 hash）——dom/tom/acp 行。
func contentIdentityHash(value any) string {
	return hashHex(stripVolatileContent(neutralizeToMap(value)))
}

// instanceIdentityHash 是实例身份（全量 JSON hash，含时间戳）——fxm/com 行。
func instanceIdentityHash(value any) string {
	return hashHex(neutralizeToMap(value))
}

// ---------------------------------------------------------------------------
// tom 适配器（输入域：project.structure——shadow 工程快照）
// ---------------------------------------------------------------------------

// TomAdapter 返回 tom 的物化适配器：从 shadow 工程快照抽轨行（capabilitycontext
// projectTOMRows 同源逻辑），tom.BuildFromImportRows 现算，按轨出行。
func TomAdapter() KindAdapter {
	return KindAdapter{
		Kind:        "tom",
		Compute:     ComputePrecomputable,
		InputScopes: []string{"project.structure"},
		Build: func(deps DepInputs) ([]Row, error) {
			trackRows := tomTrackRows(deps.ProjectState)
			if len(trackRows) == 0 {
				return nil, nil
			}
			projection := tom.BuildFromImportRows(tom.ImportInput{
				CreatedAt: "1970-01-01T00:00:00Z", // 实例时间不进内容身份（hash 口径置空）
				Summary: map[string]any{
					"tracks_created": len(trackRows), "track_count": len(trackRows),
					"source": "materialize_adapter_shadow_project_state",
				},
				Rows: trackRows,
			})
			byTrack := map[string]tom.TrackAssignment{}
			for _, group := range projection.GroupProposals {
				for _, assignment := range group.Assignments {
					byTrack[assignment.TrackID] = assignment
				}
			}
			for _, assignment := range projection.NeedsReviewTracks {
				byTrack[assignment.TrackID] = assignment
			}
			rows := make([]Row, 0, len(trackRows))
			for _, raw := range trackRows {
				trackID := strings.TrimSpace(stringField(raw, "track_id"))
				if trackID == "" {
					continue
				}
				payload := map[string]any{
					"status":      projection.Status,
					"track_name":  stringField(raw, "track_name"),
					"tom_version": projection.TOMVersion,
				}
				for _, key := range []string{"duration_seconds", "channel_count", "clip_id"} {
					if value, ok := raw[key]; ok && value != nil {
						payload[key] = value
					}
				}
				if assignment, ok := byTrack[trackID]; ok {
					payload["group_id"] = assignment.GroupID
					payload["group_label"] = assignment.GroupLabel
					payload["role_hypothesis"] = assignment.RoleHypothesis
					payload["confidence"] = assignment.Confidence
					payload["confidence_score"] = assignment.ConfidenceScore
				}
				rows = append(rows, Row{
					Ref: agentprotocol.Ref{
						Kind: "tom", ScopeKind: "track", ScopeValue: trackID,
						Window: allTimeWindow(), Snapshot: snapshotTokenCurrent,
						Hash: contentIdentityHash(map[string]any{
							"track_row": raw, "assignment": byTrack[trackID],
							"status": projection.Status, "tom_version": projection.TOMVersion,
						}),
					},
					Payload: payload,
				})
			}
			return rows, nil
		},
	}
}

// tomTrackRows 从工程快照抽 tom 轨行（capabilitycontext.projectTOMRows 同源：
// tracks/visible_tracks/daw_state_summary.tracks 三级回退，容器轨剔除）。
func tomTrackRows(projectState map[string]any) []map[string]any {
	var tracks []map[string]any
	for _, container := range []map[string]any{
		projectState,
		mapAny(projectState["project_state"]),
		mapAny(projectState["daw_state_summary"]),
	} {
		if rows := mapRows(container["tracks"]); len(rows) > 0 {
			tracks = rows
			break
		}
		if rows := mapRows(container["visible_tracks"]); len(rows) > 0 {
			tracks = rows
			break
		}
	}
	out := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		if boolField(track, "is_folder_track") || boolField(track, "is_folder_container") || boolField(track, "can_contain_child_tracks") {
			continue
		}
		trackType := strings.ToLower(stringField(track, "track_type", "type", "kind"))
		if strings.Contains(trackType, "folder") {
			continue
		}
		trackID := stringField(track, "track_id", "id")
		trackName := stringField(track, "track_name", "name", "user_label")
		if trackID == "" || trackName == "" {
			continue
		}
		row := map[string]any{"track_id": trackID, "track_name": trackName}
		if clips := mapRows(track["clips"]); len(clips) > 0 {
			clip := clips[0]
			copyTextField(row, "clip_id", clip, "clip_id", "id")
			copyTextField(row, "clip_name", clip, "clip_name", "name", "file_name")
			copyTextField(row, "source_file_path", clip, "current_source_path", "source_file_path", "source_path", "file_path")
			copyNumberField(row, "duration_seconds", clip, "length_seconds", "duration_seconds")
			copyNumberField(row, "start_seconds", clip, "start_seconds", "start_time_seconds")
		}
		copyNumberField(row, "channel_count", track, "channel_count", "channels", "source_channel_count")
		out = append(out, row)
	}
	return out
}

// ---------------------------------------------------------------------------
// acp 适配器（输入域：acousticpackage store 快照行）
// ---------------------------------------------------------------------------

// ACPAdapter 返回 acp 的物化适配器：每个包一行（scope=track；M4 revision 指纹
// 字段进 payload+hash——QUERY_ENGINE §3.3 rev_hex8 语义的 v1 承载位）。
func ACPAdapter() KindAdapter {
	return KindAdapter{
		Kind:        "acp",
		Compute:     ComputePrecomputable,
		InputScopes: []string{"acoustic.l3.band_energy", "acoustic.l3.stereo_relation", "acoustic.l3.loudness"},
		Build: func(deps DepInputs) ([]Row, error) {
			rows := make([]Row, 0, len(deps.AcousticPackages))
			for _, pkg := range deps.AcousticPackages {
				trackID := strings.TrimSpace(pkg.TrackID)
				if trackID == "" {
					continue // 无 track 坐标的包行（v1 不物化，登记开放）
				}
				payload := map[string]any{
					"status":          pkg.Status,
					"schema_version":  pkg.SchemaVersion,
					"source_revision": pkg.SourceRevision,
					"clip_revision":   pkg.ClipRevision,
					"render_revision": pkg.RenderRevision,
				}
				if pkg.DurationSec > 0 {
					payload["duration_seconds"] = pkg.DurationSec
				}
				layerNames := make([]string, 0, len(pkg.PackageLayers))
				for name := range pkg.PackageLayers {
					layerNames = append(layerNames, name)
				}
				sort.Strings(layerNames)
				for _, name := range layerNames {
					payload["layer."+name+".status"] = pkg.PackageLayers[name].Status
				}
				rows = append(rows, Row{
					Ref: agentprotocol.Ref{
						Kind: "acp", ScopeKind: "track", ScopeValue: trackID,
						Window: allTimeWindow(), Snapshot: snapshotTokenCurrent,
						Hash: contentIdentityHash(pkg),
					},
					Payload: payload,
				})
			}
			return rows, nil
		},
	}
}

// ---------------------------------------------------------------------------
// dom 适配器（输入域：feature snapshot，source_only 档）
// ---------------------------------------------------------------------------

// DOMAdapter 返回 dom 的物化适配器：对 feature snapshot 中每个有特征行的轨，
// 经与观察路径共享的装配（mixboard.DOMInputFromObservation）+dom.Build 现算，
// 行化复用 DOMRowFromProjection（登记/对账同口径）。
func DOMAdapter() KindAdapter {
	return KindAdapter{
		Kind:    "dom",
		Compute: ComputePrecomputable,
		InputScopes: []string{"track.level", "track.stereo_space", "track.basic_energy",
			"track.time_dynamics", "track.timbre_frequency", "project.headroom",
			"feature.waveform", "feature.spectral"},
		Build: func(deps DepInputs) ([]Row, error) {
			trackIDs := featureTrackIDs(deps.FeatureSnapshot)
			if len(trackIDs) == 0 {
				return nil, nil
			}
			rows := make([]Row, 0, len(trackIDs))
			for _, trackID := range trackIDs {
				input := mixboard.DOMInputFromObservation(mixboard.ObservationPacket{
					TargetRef:      mixboard.TargetRef{Kind: "track", ID: trackID},
					GlobalSummary:  map[string]any{"feature_snapshot": deps.FeatureSnapshot},
					ProjectPackage: map[string]any{"project_revision": deps.ProjectRevision},
				}, mixboard.Request{})
				projection := dom.Build(input)
				if row, ok := DOMRowFromProjection(trackID, projection); ok {
					rows = append(rows, row)
				}
			}
			return rows, nil
		},
	}
}

// featureTrackIDs 提取 feature snapshot 中有轨级特征行的 track 集（去重+字典序，
// 重算批确定性）。
func featureTrackIDs(snapshot map[string]any) []string {
	seen := map[string]bool{}
	for _, key := range []string{"track_waveform_envelopes", "band_energy_summaries",
		"stereo_relation_summaries", "loudness_summaries"} {
		for _, row := range mapRows(snapshot[key]) {
			trackID := stringField(row, "track_id", "source_track_id")
			if trackID != "" {
				seen[trackID] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for trackID := range seen {
		out = append(out, trackID)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// 行化：观察产物 → 物化行（登记路径与对账基准共用）
// ---------------------------------------------------------------------------

// DOMRowFromProjection 把 dom 投影行化为 precomputable 行（对账基准与登记共
// 用——两侧同 hash 口径）。空投影（零值）不产行。
func DOMRowFromProjection(trackID string, projection dom.Projection) (Row, bool) {
	if strings.TrimSpace(projection.SchemaVersion) == "" && strings.TrimSpace(projection.Status) == "" {
		return Row{}, false
	}
	payload := map[string]any{
		"status": projection.Status,
		"mode":   projection.Mode,
	}
	if len(projection.DimensionReadiness) > 0 {
		payload["dimensions"] = len(projection.DimensionReadiness)
	}
	return Row{
		Ref: agentprotocol.Ref{
			Kind: "dom", ScopeKind: "track", ScopeValue: strings.TrimSpace(trackID),
			Window: allTimeWindow(), Snapshot: snapshotTokenCurrent,
			Hash: contentIdentityHash(projection),
		},
		Payload: payload,
	}, true
}

// FXMRowsFromObservation 把观察产物 fxm 投影登记为行（F9 登记型：不预计算；
// snapshot=observation_id、hash=实例身份含时间戳——QUERY_ENGINE §3.3）。
func FXMRowsFromObservation(observationID string, projection *fxm.Projection) []Row {
	if projection == nil || strings.TrimSpace(projection.SchemaVersion) == "" {
		return nil
	}
	scopeKind, scopeValue := targetScope(projection.TargetRef)
	payload := map[string]any{"status": projection.Status}
	if projection.ProjectionID != "" {
		payload["projection_id"] = projection.ProjectionID
	}
	return []Row{{
		Ref: agentprotocol.Ref{
			Kind: "fxm", ScopeKind: scopeKind, ScopeValue: scopeValue,
			Window: allTimeWindow(), Snapshot: strings.TrimSpace(observationID),
			Hash: instanceIdentityHash(projection),
		},
		Payload: payload,
	}}
}

// COMRowsFromObservation 把观察产物 com 投影登记为行（F9 登记型，同 fxm 口径）。
func COMRowsFromObservation(observationID string, projection *com.Projection) []Row {
	if projection == nil || strings.TrimSpace(projection.SchemaVersion) == "" {
		return nil
	}
	scopeKind, scopeValue := targetScope(projection.TargetRef)
	payload := map[string]any{"status": projection.Status, "mode": projection.Mode}
	if projection.ProjectionID != "" {
		payload["projection_id"] = projection.ProjectionID
	}
	return []Row{{
		Ref: agentprotocol.Ref{
			Kind: "com", ScopeKind: scopeKind, ScopeValue: scopeValue,
			Window: allTimeWindow(), Snapshot: strings.TrimSpace(observationID),
			Hash: instanceIdentityHash(projection),
		},
		Payload: payload,
	}}
}

// targetScope 从投影 TargetRef（map 形态）取行坐标；缺省档 target/unknown
// （refschema 要求 scope 两段非空）。
func targetScope(targetRef map[string]any) (kind, value string) {
	kind = strings.TrimSpace(stringField(targetRef, "kind"))
	if kind == "" {
		kind = "target"
	}
	value = strings.TrimSpace(stringField(targetRef, "id"))
	if value == "" {
		value = "unknown"
	}
	return kind, value
}

// ---------------------------------------------------------------------------
// 默认注册
// ---------------------------------------------------------------------------

// RegisterDefaultAdapters 注册白名单内的默认适配器（tom/acp/dom；kinds 空=
// §7.2 默认三先切）。白名单外 kind 不注册（无适配器即不参与重算）。
func RegisterDefaultAdapters(s *Store, kinds []string) error {
	if len(kinds) == 0 {
		kinds = defaultWhitelist
	}
	for _, adapter := range []KindAdapter{TomAdapter(), ACPAdapter(), DOMAdapter()} {
		if !containsString(kinds, adapter.Kind) {
			continue
		}
		if err := s.RegisterAdapters(adapter); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// map 取值小工具（harness firstString 同款语义的本包只读版）
// ---------------------------------------------------------------------------

func mapAny(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func stringField(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func boolField(row map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch value := row[key].(type) {
		case bool:
			if value {
				return true
			}
		case string:
			if strings.EqualFold(strings.TrimSpace(value), "true") {
				return true
			}
		}
	}
	return false
}

func copyTextField(dst map[string]any, dstKey string, source map[string]any, keys ...string) {
	if value := stringField(source, keys...); value != "" {
		dst[dstKey] = value
	}
}

func copyNumberField(dst map[string]any, dstKey string, source map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			dst[dstKey] = value
			return
		}
	}
}
