package mom

import (
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

func evidenceRefs(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func targetID(input Input) string {
	return firstNonEmpty(text(input.TargetRef["id"]), text(input.ProjectPackage["track_id"]), "target")
}

// momEvidenceRef renders one vit://mom L0 evidence ref (REFSCHEMA-M2, G1 终审
// §4 M2 行)：legacy 数据键族头进 scope_kind，数据键余部进 scope_value，
// snapshot 段承载 observation_id（mom 身份族语义，M1/registry 注记裁定；
// 非内容哈希）。legacy 数据键不带采样窗，window 恒 t=all；mom refs 是数据面
// 地址而非内容寻址，hash 段显式 "-"（未 CAS 化）。必需段缺失（最典型是
// ObservationID 为空）时返回 ""——观察域 ref 没有身份即不发（legacy 写入面
// 曾发出无身份字面量；vit://mom 文法不允许伪造 snapshot，宁缺勿假）。
func momEvidenceRef(scopeKind, scopeValue, observationID string) string {
	ref, err := agentprotocol.FormatRef(agentprotocol.Ref{
		Kind:       "mom",
		ScopeKind:  scopeKind,
		ScopeValue: scopeValue,
		Window:     &agentprotocol.TimeWindow{AllTime: true},
		Snapshot:   observationID,
		Hash:       "-",
	})
	if err != nil {
		return ""
	}
	return ref
}

func trackReadKey(input Input, suffix string) string {
	return momEvidenceRef("mix.read", "track."+safeKey(targetID(input))+"."+suffix, input.ObservationID)
}

func acousticFeatureRef(input Input, layer, feature string) string {
	if strings.TrimSpace(layer) == "" || strings.TrimSpace(feature) == "" {
		return ""
	}
	return momEvidenceRef("acoustic_package_status", layer+"."+feature, input.ObservationID)
}

func observationRef(input Input) string {
	if strings.TrimSpace(input.ObservationID) == "" {
		return ""
	}
	return momEvidenceRef("observation", input.ObservationID, input.ObservationID)
}

func allEvidenceRefs(proj Projection) []string {
	refs := []string{}
	refs = append(refs, proj.ProjectStructure.EvidenceRefs...)
	refs = append(refs, proj.ProjectMixProfile.EvidenceRefs...)
	refs = append(refs, proj.MultitrackRelation.EvidenceRefs...)
	if proj.FrequencyRelationship != nil {
		refs = append(refs, proj.FrequencyRelationship.EvidenceRefs...)
	}
	if proj.MaskingRelationship != nil {
		refs = append(refs, proj.MaskingRelationship.EvidenceRefs...)
	}
	refs = append(refs, proj.Layers.BasicEnergy.EvidenceRefs...)
	refs = append(refs, proj.Layers.TimbreFrequency.EvidenceRefs...)
	refs = append(refs, proj.Layers.SpaceStereo.EvidenceRefs...)
	refs = append(refs, proj.Layers.TimeDynamicsStructure.EvidenceRefs...)
	refs = append(refs, proj.Layers.MultitrackRelationship.EvidenceRefs...)
	refs = append(refs, proj.Layers.ABResultComparison.EvidenceRefs...)
	return evidenceRefs(refs...)
}
