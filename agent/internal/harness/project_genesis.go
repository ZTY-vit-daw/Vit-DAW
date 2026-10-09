package harness

// project_genesis.go — L4 工程账本 genesis 头部段接线（L4-GENESIS-1，
// CONTEXT_LAYERING_V1_DESIGN §2.4-2）：工程打开事件面（version_project_opened
// → applyExternalProjectOpened）在 shadow 刷新后从投影/状态面**只读渲染一次**
// genesis 事实——TOM 概览（轨道清单摘要行）、总线拓扑（路由树文本化）、交付
// 目标（RLM 交付 profile 引用行）——经 carriers.AppendGenesis 写
// Kind=topology_delta、Phase=genesis 条目（空账本幂等，非空跳过；本卡不做
// delta 增量腿）。
//
// 纪律红线（AGENTS §5 / 卡面）：只读消费投影摘要行与既有消费面——不展开
// raw package、不代发 observe、不扩投影披露面。TOM 摘要行走
// capabilitycontext.BuildProjectTOMProjection（chat 侧既有消费面）；总线拓扑
// 取自同一内核项目状态快照（TOM 消费面同源），文本化在本接线层完成；交付
// 目标取 rlm 交付 profile 目录与 harness 既有 render-profile 绑定快照。
//
// fail-open：genesis 任何失败（影子不可用/工程身份不可证/账本写入失败）都
// 不阻塞工程打开——WARN 日志 + result 警告字段可见，绝不向上返回错误。

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/rlm"
)

// genesis 头部段是前缀渲染常客：摘要行封顶防账本膨胀（超限截断并在行内
// 注记 "+N more"，不静默丢）。
const (
	genesisMaxTrackLines    = 32
	genesisMaxDeliveryLines = 8
	genesisMaxRouteEdges    = 32
)

// appendProjectLedgerGenesis 在工程打开处理后渲染并追加 genesis 头部段。
// 所有失败路径 fail-open（WARN + result 注记，不返回错误、不改打开结果）。
func (h *Harness) appendProjectLedgerGenesis(projectPath string, result map[string]any) {
	projectPath = strings.TrimSpace(projectPath)
	if h == nil || projectPath == "" {
		return
	}
	if h.shadow == nil {
		h.warnProjectLedgerGenesis("shadow unavailable; genesis skipped", result)
		return
	}
	engine := h.genesisEngineSnapshot()
	if !genesisIdentityMatches(engine, projectPath) {
		// 影子状态无法证归本工程（未初始化/串工程/无身份）——不写，防
		// 把别工程的事实写进本工程账本头部。
		h.warnProjectLedgerGenesis("shadow snapshot identity does not match opened project; genesis skipped", result)
		return
	}
	// ResolveProjectDir：project_path 是 .vit 文件时账本落其父目录（既有
	// 目录语义不变，L4-LEDGER-DIR-1 起与装配/retain 三面共用同一解析）。
	appended, err := carriers.AppendGenesis(carriers.ResolveProjectDir(projectPath), genesisFacts(engine, h.RenderProfileBindingsSnapshot()), time.Now().UTC())
	if err != nil {
		h.warnProjectLedgerGenesis(fmt.Sprintf("project ledger genesis not appended: %v", err), result)
		return
	}
	if result != nil {
		result["project_ledger_genesis"] = map[string]any{"appended": appended}
	}
	if h.logger != nil {
		h.logger.Info("[harness] project ledger genesis appended entries=%d project=%q", appended, projectPath)
	}
}

// genesisEngineSnapshot 取 shadow 的内核项目状态快照（engine_snapshot；
// 与 TOM 消费面同源的只读面）。
func (h *Harness) genesisEngineSnapshot() map[string]any {
	snapshot := h.shadow.Snapshot()
	engine, _ := snapshot["engine_snapshot"].(map[string]any)
	if engine == nil {
		return nil
	}
	return engine
}

// genesisIdentityMatches 校验影子快照确属刚打开的工程（归一化路径比对；
// 任一侧缺身份=不匹配）。
func genesisIdentityMatches(engine map[string]any, projectPath string) bool {
	if engine == nil {
		return false
	}
	snapshotPath := genesisFirstText(engine, "project_path", "current_project_path", "file_path")
	if snapshotPath == "" {
		return false
	}
	return genesisNormalizePath(snapshotPath) == genesisNormalizePath(projectPath)
}

func genesisNormalizePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	return strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
}

// genesisFacts 从只读面渲染三组 genesis 事实（任一组不可读=留空，
// AppendGenesis 对空语句组跳过入账）。
func genesisFacts(engine map[string]any, bindings []rlm.RenderProfileBinding) carriers.GenesisFacts {
	return carriers.GenesisFacts{
		TracksSummary:   genesisTracksSummary(engine),
		BusTopology:     genesisBusTopology(engine),
		DeliveryTargets: genesisDeliveryTargets(bindings),
	}
}

// genesisTracksSummary 消费 TOM 投影 LLMContext/清单摘要行：概览行
// （llm_context.summary_md）+ 逐轨 "name → group [confidence]" 行。
func genesisTracksSummary(engine map[string]any) []string {
	projection := capabilitycontext.BuildProjectTOMProjection(engine, "", "")
	if len(projection) == 0 {
		return nil
	}
	lines := make([]string, 0, 8)
	if llmContext, ok := projection["llm_context"].(map[string]any); ok {
		if summary := genesisFirstText(llmContext, "summary_md"); summary != "" {
			lines = append(lines, summary)
		}
	}
	trackLines := genesisManifestTrackLines(projection)
	if len(trackLines) > genesisMaxTrackLines-len(lines) {
		kept := genesisMaxTrackLines - len(lines)
		if kept < 0 {
			kept = 0
		}
		trackLines = append(trackLines[:kept], fmt.Sprintf("…+%d more", len(trackLines)-kept))
	}
	lines = append(lines, trackLines...)
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// genesisManifestTrackLines 从 TOM full_assignment_manifest 抽逐轨摘要行
// （投影披露面内的既序列化行，不回读 raw rows）。
func genesisManifestTrackLines(projection map[string]any) []string {
	manifest, ok := projection["full_assignment_manifest"].(map[string]any)
	if !ok {
		return nil
	}
	groups, ok := manifest["groups"].([]any)
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(groups))
	for _, groupAny := range groups {
		group, ok := groupAny.(map[string]any)
		if !ok {
			continue
		}
		label := genesisFirstText(group, "label")
		assignments, _ := group["assignments"].([]any)
		for _, assignmentAny := range assignments {
			assignment, ok := assignmentAny.(map[string]any)
			if !ok {
				continue
			}
			name := genesisFirstText(assignment, "track_name", "track_id")
			if name == "" {
				continue
			}
			line := name
			if label != "" {
				line += " → " + label
			}
			if confidence := genesisFirstText(assignment, "confidence"); confidence != "" {
				line += " [" + confidence + "]"
			}
			lines = append(lines, line)
			if len(lines) >= genesisMaxTrackLines {
				return lines
			}
		}
	}
	return lines
}

// genesisBusTopology 把内核项目状态快照的轨道父路由文本化成一行
// （tracks 计数 + child→parent 边 + 总线/子混轨道标记；状态序即输出序，
// 确定性）。无轨道/无路由字段=留空（不入账）。
func genesisBusTopology(engine map[string]any) string {
	tracks := genesisTrackRows(engine)
	if len(tracks) == 0 {
		return ""
	}
	nameByID := make(map[string]string, len(tracks))
	for _, track := range tracks {
		if id := genesisFirstText(track, "track_id", "id"); id != "" {
			nameByID[id] = genesisTrackDisplayName(track)
		}
	}
	edges := make([]string, 0, len(tracks))
	truncated := 0
	buses := make([]string, 0, 4)
	for _, track := range tracks {
		name := genesisTrackDisplayName(track)
		if name == "" {
			continue
		}
		if genesisTrackIsBus(track) {
			buses = append(buses, name)
		}
		parentID := genesisFirstText(track, "parent_track_id", "parent_folder_track_id")
		parent, known := nameByID[parentID]
		if parentID == "" || !known || parent == name {
			continue
		}
		if len(edges) >= genesisMaxRouteEdges {
			truncated++
			continue
		}
		edges = append(edges, name+"→"+parent)
	}
	if len(edges) == 0 && len(buses) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("tracks=%d", len(tracks))}
	if len(edges) > 0 {
		parts = append(parts, "routes: "+strings.Join(edges, ", "))
	}
	if truncated > 0 {
		parts = append(parts, fmt.Sprintf("…+%d more", truncated))
	}
	if len(buses) > 0 {
		parts = append(parts, "bus/submix: "+strings.Join(buses, ", "))
	}
	return strings.Join(parts, "; ")
}

// genesisTrackIsBus 汇总线语义标记（vit_type=bus / 子混文件夹 / 路由总线）。
func genesisTrackIsBus(track map[string]any) bool {
	if strings.EqualFold(genesisFirstText(track, "vit_type"), "bus") {
		return true
	}
	for _, key := range []string{"is_submix_folder", "routing_bus_enabled"} {
		if value, ok := track[key].(bool); ok && value {
			return true
		}
	}
	return false
}

func genesisTrackDisplayName(track map[string]any) string {
	return genesisFirstText(track, "track_name", "name", "track_id", "id")
}

func genesisTrackRows(engine map[string]any) []map[string]any {
	raw, ok := engine["tracks"].([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, rowAny := range raw {
		if row, ok := rowAny.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// genesisDeliveryTargets 渲染 RLM 交付目标引用行：先已绑定 render→profile
// （工程态），再内建 profile 目标目录（引用面），封顶截断。
func genesisDeliveryTargets(bindings []rlm.RenderProfileBinding) []string {
	lines := make([]string, 0, 4)
	for _, binding := range bindings {
		profile, err := binding.ResolveProfile()
		if err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("render %s → %s (%s)", binding.RenderID, profile.ProfileID, profile.DisplayName))
		if len(lines) >= genesisMaxDeliveryLines {
			return lines
		}
	}
	for _, profile := range rlm.BuiltinDeliveryProfiles() {
		if len(lines) >= genesisMaxDeliveryLines {
			lines = append(lines, fmt.Sprintf("…+%d more", len(rlm.BuiltinDeliveryProfiles())+len(lines)-genesisMaxDeliveryLines))
			break
		}
		lines = append(lines, genesisDeliveryProfileLine(profile))
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func genesisDeliveryProfileLine(profile rlm.DeliveryProfile) string {
	line := profile.ProfileID
	if profile.DisplayName != "" {
		line += " (" + profile.DisplayName + ")"
	}
	if profile.Integrated.Max != 0 {
		if profile.Integrated.Min != nil {
			line += fmt.Sprintf(" %g…%g LUFS", *profile.Integrated.Min, profile.Integrated.Max)
		} else {
			line += fmt.Sprintf(" ≤%g LUFS", profile.Integrated.Max)
		}
	}
	if profile.TruePeakMaxDBTP != 0 {
		line += fmt.Sprintf(" ≤%gdBTP", profile.TruePeakMaxDBTP)
	}
	return line
}

func genesisFirstText(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := source[key].(string)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

// warnProjectLedgerGenesis 统一 fail-open 留痕：WARN 日志 + result 警告
// 字段（打开结果本身不变）。
func (h *Harness) warnProjectLedgerGenesis(reason string, result map[string]any) {
	if h != nil && h.logger != nil {
		h.logger.Warn("[harness] project ledger genesis: %s", reason)
	}
	if result != nil {
		result["project_ledger_genesis"] = map[string]any{"warning": reason}
	}
}
