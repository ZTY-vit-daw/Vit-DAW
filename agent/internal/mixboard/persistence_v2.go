package mixboard

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/projectstore"
)

func projectObservationForPersistence(req Request, observation ObservationPacket, rawSnapshot featureSnapshot) (ObservationPacket, string, error) {
	if strings.TrimSpace(os.Getenv("VIT_MIXBOARD_ROOT")) != "" {
		return observation, "", nil
	}
	roots, ok := projectstore.Current()
	if !ok {
		return observation, "", nil
	}
	projectUUID := observationProjectUUID(req)
	if projectUUID == "" {
		return observation, "", nil
	}
	if !strings.EqualFold(projectstore.SafeName(projectUUID), roots.ProjectUUID) {
		return ObservationPacket{}, "", errors.New("mix observation project identity does not match active v2 store")
	}
	projected := cloneObservationPacket(observation)
	projected.SchemaVersion = ObservationSchemaVersion
	projected.ProjectUUID = roots.ProjectUUID
	fullSnapshot := rawFeatureSnapshotMap(rawSnapshot)
	evidenceRef := ""
	if len(fullSnapshot) > 0 {
		ref, _, err := projectstore.PutEvidence(roots, "feature_snapshot", fullSnapshot, "obs:"+projected.ObservationID)
		if err == nil {
			evidenceRef = ref
			projected.EvidenceRefs = uniqueObservationRefs(append(projected.EvidenceRefs, ref))
		} else if !errors.Is(err, projectstore.ErrEvidenceDisabled) {
			return ObservationPacket{}, "", err
		}
	}
	applyObservationEvidenceProjection(&projected, evidenceRef)
	manifest, err := projectstore.Load(roots)
	if err != nil {
		return ObservationPacket{}, "", err
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		return ObservationPacket{}, "", err
	}
	if int64(len(encoded)) > manifest.Budgets.ObservationMaxBytes {
		applyObservationHardProjection(&projected, evidenceRef)
		encoded, err = json.Marshal(projected)
		if err != nil {
			return ObservationPacket{}, "", err
		}
	}
	if int64(len(encoded)) > manifest.Budgets.ObservationMaxBytes {
		_ = projectstore.RecordGateAudit(roots, projectstore.GateAuditEntry{Action: "reject_observation", Detail: projected.ObservationID + ":projection_still_too_large"})
		return ObservationPacket{}, "", errors.New("mix observation exceeds v2 projection limit")
	}
	path := filepath.Join(roots.Agent, "observations", projected.ObservationID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ObservationPacket{}, "", err
	}
	if err := writeJSON(path, projected); err != nil {
		return ObservationPacket{}, "", err
	}
	// Recalibrate first so the manifest reflects the newly committed
	// projection; then apply only renewable-data gates. A gate failure must not
	// turn a successful observation into a project-save failure.
	_, _ = projectstore.Recalibrate(roots)
	_, _ = projectstore.EnforceBudgets(roots)
	return projected, path, nil
}

func observationProjectUUID(req Request) string {
	for _, source := range []map[string]any{req.ProjectState, req.Args} {
		for _, key := range []string{"project_uuid", "project_id"} {
			if value := cleanAnyString(source[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func cloneObservationPacket(in ObservationPacket) ObservationPacket {
	data, _ := json.Marshal(in)
	out := ObservationPacket{}
	_ = json.Unmarshal(data, &out)
	return out
}

func rawFeatureSnapshotMap(snapshot featureSnapshot) map[string]any {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

func applyObservationEvidenceProjection(observation *ObservationPacket, evidenceRef string) {
	if observation == nil {
		return
	}
	projection := persistentFeatureProjection(mapValue(observation.GlobalSummary["feature_snapshot"]), evidenceRef)
	if len(projection) > 0 {
		observation.GlobalSummary["feature_snapshot"] = projection
	}
	for _, container := range []map[string]any{observation.EnvironmentPackage, observation.DeepPackage} {
		if container == nil {
			continue
		}
		delete(container, "feature_snapshot")
		if evidenceRef != "" {
			container["feature_snapshot_ref"] = evidenceRef
		}
	}
}

func persistentFeatureProjection(snapshot map[string]any, evidenceRef string) map[string]any {
	if len(snapshot) == 0 && evidenceRef == "" {
		return nil
	}
	out := stripLargeObservationValue(snapshot).(map[string]any)
	delete(out, "spectrogram_tile_rows")
	if evidenceRef != "" {
		out["evidence_ref"] = evidenceRef
	}
	return out
}

func stripLargeObservationValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "tile_payload", "payload", "samples", "sample_values", "pcm", "waveform", "spectrogram_tile_rows", "time_segments", "time_energy_rows", "raw_rows":
				continue
			}
			out[key] = stripLargeObservationValue(child)
		}
		return out
	case []any:
		limit := len(typed)
		if limit > 512 {
			limit = 512
		}
		out := make([]any, 0, limit)
		for _, child := range typed[:limit] {
			out = append(out, stripLargeObservationValue(child))
		}
		return out
	default:
		return value
	}
}

func applyObservationHardProjection(observation *ObservationPacket, evidenceRef string) {
	if observation == nil {
		return
	}
	observation.TimelineDigest = nil
	observation.SectionCandidates = compactObservationRows(observation.SectionCandidates, 128)
	observation.Hotspots = compactObservationRows(observation.Hotspots, 128)
	if observation.GlobalSummary == nil {
		observation.GlobalSummary = map[string]any{}
	}
	featureStatus := map[string]any{"status": "projected"}
	if evidenceRef != "" {
		featureStatus["evidence_ref"] = evidenceRef
	}
	observation.GlobalSummary["feature_snapshot"] = featureStatus
	for _, container := range []map[string]any{observation.EnvironmentPackage, observation.ProjectPackage, observation.MixPackage, observation.DeepPackage} {
		stripObservationContainer(container)
	}
}

func stripObservationContainer(container map[string]any) {
	for key, value := range container {
		container[key] = stripLargeObservationValue(value)
	}
}

func compactObservationRows(rows []map[string]any, limit int) []map[string]any {
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if compact, ok := stripLargeObservationValue(row).(map[string]any); ok {
			out = append(out, compact)
		}
	}
	return out
}

func uniqueObservationRefs(refs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}
