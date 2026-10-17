package mom

import (
	"fmt"
	"sort"
)

// AGENT-SENDROUTE-SEMANTICS-1（设计件 §3 MOM 行）：send_topology 投影——
// 共享空间结构事实（源轨→FX 轨发送边+发送量+回流边），输入=mixboard 新包
// project.send_topology_inputs（agent 侧轻量派生自内核 track.send.list 读回）。
// 投影只消费结构事实：evidence refs 指向 mix.read / kernel track.send.list，
// 不展开音频包；标注「共享空间结构事实，非效果语义」。peer 投影纪律：
// 投影行按观察装配原样呈现，缺失/partial 不升级，零自动 view 选择。

const SendTopologySchema = "mom.send_topology.v0_1"

// SendTopology 是轨间 send 拓扑的紧凑投影契约（与 static/frequency/masking
// 关系并列的 peer 面）。
type SendTopology struct {
	SchemaVersion string             `json:"schema_version"`
	Status        string             `json:"status"`
	Freshness     string             `json:"freshness"`
	Reason        string             `json:"reason,omitempty"`
	ProjectCutRef string             `json:"project_cut_ref,omitempty"`
	Edges         []SendTopologyEdge `json:"edges"`
	FXTracks      []SendTopologyFX   `json:"fx_tracks"`
	DanglingCount int                `json:"dangling_edge_count"`
	Coverage      map[string]any     `json:"coverage"`
	EvidenceRefs  []string           `json:"evidence_refs,omitempty"`
	Limitations   []string           `json:"limitations,omitempty"`
	Note          string             `json:"note"`
}

// SendTopologyEdge 一条发送边：source→fx + 发送量 + 回流边（湿声回源轨）。
type SendTopologyEdge struct {
	SourceTrackID string  `json:"source_track_id"`
	SourceName    string  `json:"source_name,omitempty"`
	FXTrackID     string  `json:"fx_track_id"`
	FXName        string  `json:"fx_name,omitempty"`
	AmountDB      float64 `json:"amount_db"`
	SendBus       int     `json:"send_bus"`
	ReturnBus     int     `json:"return_bus"`
	ReturnEdge    string  `json:"return_edge"`
}

// SendTopologyFX 一条 FX 轨行：bus 对+源计数（关系细节归边行，不重复）。
type SendTopologyFX struct {
	TrackID     string `json:"track_id"`
	Name        string `json:"name,omitempty"`
	SendBus     int    `json:"send_bus"`
	ReturnBus   int    `json:"return_bus"`
	SourceCount int    `json:"source_count"`
}

func buildSendTopology(input Input) *SendTopology {
	projected := mapValue(input.ProjectPackage["send_topology_inputs"])
	if len(projected) == 0 {
		// 读源缺席（观察装配未注入 send_topology）：投影不出现，不虚构
		// missing 行——由 CCB view 侧如实披露缺口。
		return nil
	}
	edges := []SendTopologyEdge{}
	for _, row := range rowsFromAny(projected["sends"]) {
		sourceID := text(row["source_track_id"])
		fxID := text(row["fx_track_id"])
		if sourceID == "" || fxID == "" {
			continue
		}
		edges = append(edges, SendTopologyEdge{
			SourceTrackID: sourceID,
			SourceName:    text(row["source_name"]),
			FXTrackID:     fxID,
			FXName:        text(row["fx_name"]),
			AmountDB:      number(row["amount_db"]),
			SendBus:       int(number(row["send_bus"])),
			ReturnBus:     int(number(row["return_bus"])),
			ReturnEdge:    "fx_wet_return_to_source",
		})
	}
	fxTracks := []SendTopologyFX{}
	for _, row := range rowsFromAny(projected["fx_tracks"]) {
		trackID := text(row["track_id"])
		if trackID == "" {
			continue
		}
		fxTracks = append(fxTracks, SendTopologyFX{
			TrackID:     trackID,
			Name:        text(row["name"]),
			SendBus:     int(number(row["send_bus"])),
			ReturnBus:   int(number(row["return_bus"])),
			SourceCount: int(number(row["source_count"])),
		})
	}
	dangling := rowsFromAny(projected["dangling"])
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].FXTrackID != edges[j].FXTrackID {
			return edges[i].FXTrackID < edges[j].FXTrackID
		}
		return edges[i].SourceTrackID < edges[j].SourceTrackID
	})
	sort.SliceStable(fxTracks, func(i, j int) bool { return fxTracks[i].TrackID < fxTracks[j].TrackID })
	status := StatusFromSource(text(projected["status"]))
	limitations := stringsFromAny(projected["limitations"])
	if len(dangling) > 0 {
		limitations = append(limitations, fmt.Sprintf("dangling_send_edges:%d", len(dangling)))
	}
	return &SendTopology{
		SchemaVersion: firstNonEmpty(text(projected["schema_version"]), SendTopologySchema),
		Status:        status,
		Freshness:     FreshnessForStatus(status),
		Reason:        text(projected["reason"]),
		ProjectCutRef: firstNonEmpty(text(projected["project_cut_ref"]), text(input.ProjectPackage["project_state_hash"]), text(input.ProjectPackage["project_revision"])),
		Edges:         edges,
		FXTracks:      fxTracks,
		DanglingCount: len(dangling),
		Coverage: map[string]any{
			"send_edge_count": len(edges),
			"fx_track_count":  len(fxTracks),
		},
		EvidenceRefs: evidenceRefs(append(evidenceRefs(observationRef(input), "project_package.send_topology_inputs", "kernel.track_send_list"), stringsFromAny(projected["evidence_refs"])...)...),
		Limitations: limitations,
		Note:        "shared-space structural facts from the track.send.list read-back; not effect semantics",
	}
}

// sendTopologyLLMFacts 渲染 LLMContext 紧凑事实（不展开 raw package）。
func sendTopologyLLMFacts(topology *SendTopology) []map[string]any {
	if topology == nil {
		return nil
	}
	facts := []map[string]any{{
		"layer":            "send_topology",
		"status":           topology.Status,
		"send_edge_count":  len(topology.Edges),
		"fx_track_count":   len(topology.FXTracks),
		"dangling_count":   topology.DanglingCount,
		"structure_note":   topology.Note,
	}}
	for i, edge := range topology.Edges {
		if i >= 12 {
			facts = append(facts, map[string]any{"layer": "send_topology", "omitted_edges": len(topology.Edges) - i})
			break
		}
		facts = append(facts, map[string]any{
			"layer": "send_topology.edge", "source": firstNonEmpty(edge.SourceName, edge.SourceTrackID),
			"fx": firstNonEmpty(edge.FXName, edge.FXTrackID), "amount_db": edge.AmountDB,
		})
	}
	return facts
}
