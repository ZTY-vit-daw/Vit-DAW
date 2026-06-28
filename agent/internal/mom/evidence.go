package mom

import "strings"

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

func trackReadKey(input Input, suffix string) string {
	return "mix.read:track." + safeKey(targetID(input)) + "." + suffix
}

func acousticFeatureRef(layer, feature string) string {
	if strings.TrimSpace(layer) == "" || strings.TrimSpace(feature) == "" {
		return ""
	}
	return "acoustic_package_status:" + layer + "." + feature
}

func observationRef(input Input) string {
	if strings.TrimSpace(input.ObservationID) == "" {
		return ""
	}
	return "observation:" + input.ObservationID
}

func allEvidenceRefs(proj Projection) []string {
	refs := []string{}
	refs = append(refs, proj.ProjectStructure.EvidenceRefs...)
	refs = append(refs, proj.ProjectMixProfile.EvidenceRefs...)
	refs = append(refs, proj.MultitrackRelation.EvidenceRefs...)
	refs = append(refs, proj.Layers.BasicEnergy.EvidenceRefs...)
	refs = append(refs, proj.Layers.TimbreFrequency.EvidenceRefs...)
	refs = append(refs, proj.Layers.SpaceStereo.EvidenceRefs...)
	refs = append(refs, proj.Layers.TimeDynamicsStructure.EvidenceRefs...)
	refs = append(refs, proj.Layers.MultitrackRelationship.EvidenceRefs...)
	refs = append(refs, proj.Layers.ABResultComparison.EvidenceRefs...)
	return evidenceRefs(refs...)
}
