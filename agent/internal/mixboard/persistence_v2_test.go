package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/projectstore"
)

func TestProjectObservationPersistsCompactPacketAndEvidence(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	roots, _, err := projectstore.Activate(projectPath, "vitproj_mix_v2")
	if err != nil {
		t.Fatal(err)
	}
	largePayload := strings.Repeat("x", 3*1024*1024)
	timeSegments := make([]any, 100)
	for i := range timeSegments {
		timeSegments[i] = map[string]any{"start_seconds": float64(i), "end_seconds": float64(i + 1), "rms_dbfs": -20.0}
	}
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_v2",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"project_uuid": roots.ProjectUUID, "project_path": projectPath, "duration_seconds": 12.0},
		Args: map[string]any{
			"feature_snapshot": map[string]any{
				"schema_version":        "mixboard_feature_snapshot.v1",
				"updated_at":            "2026-08-01T00:00:00Z",
				"waveform_envelope":     map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "rms": 0.2, "peak_abs": 0.7, "time_segments": timeSegments},
				"spectrogram_tiles":     map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "tile_count_seen": 1, "tile_count_expected": 1},
				"spectrogram_tile_rows": []any{map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "tile_payload": largePayload}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.ProjectUUID != roots.ProjectUUID || result.Observation.SchemaVersion != ObservationSchemaVersion {
		t.Fatalf("observation identity=%+v", result.Observation)
	}
	if !strings.HasPrefix(result.ObservationPath, filepath.Join(roots.Agent, "observations")) {
		t.Fatalf("observation path=%q", result.ObservationPath)
	}
	info, err := os.Stat(result.ObservationPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 2*1024*1024 {
		t.Fatalf("observation size=%d", info.Size())
	}
	data, err := os.ReadFile(result.ObservationPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "tile_payload") || strings.Contains(string(data), largePayload[:1024]) {
		t.Fatal("large tile payload remained inline")
	}
	if len(result.Observation.EvidenceRefs) != 1 {
		t.Fatalf("evidence refs=%#v", result.Observation.EvidenceRefs)
	}
	blob, err := projectstore.GetEvidence(roots, result.Observation.EvidenceRefs[0])
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(blob.Content)
	if !strings.Contains(string(encoded), "tile_payload") || !strings.Contains(string(encoded), largePayload[:1024]) {
		t.Fatal("evidence blob did not retain the raw feature payload")
	}
	legacyDuplicate := filepath.Join(roots.Agent, "mixboard", "sessions", "mix_v2", "observations", result.Observation.ObservationID+".json")
	if _, err := os.Stat(legacyDuplicate); !os.IsNotExist(err) {
		t.Fatalf("v2 session duplicated canonical observation: %v", err)
	}
	read, err := store.Read(ReadRequest{MixSessionID: "mix_v2", ObservationID: result.Observation.ObservationID})
	if err != nil || cleanAnyString(read["observation_id"]) != result.Observation.ObservationID {
		t.Fatalf("canonical observation was not readable through unchanged API: result=%#v err=%v", read, err)
	}
	raw, err := store.Read(ReadRequest{
		MixSessionID: "mix_v2", ObservationID: result.Observation.ObservationID,
		Keys: []string{"track.track_1.raw.time_energy.range"}, MaxItems: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	item := mapValue(mapValue(raw["items"])["track.track_1.raw.time_energy.range"])
	if got := int(numberFromMap(item, "returned_rows")); got != len(timeSegments) {
		t.Fatalf("lazy evidence rows=%d want=%d item=%#v", got, len(timeSegments), item)
	}
}

func TestProjectObservationPersistsCompactCOMProjectionAndCatalog(t *testing.T) {
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	roots, _, err := projectstore.Activate(projectPath, "vitproj_com_v2")
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_com_v2", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		ProjectState: map[string]any{"project_uuid": roots.ProjectUUID, "project_path": projectPath, "duration_seconds": 4.0},
		Args:         map[string]any{"com_mode": com.ModeSourceOnly, "feature_snapshot": comSourceFeatureSnapshot("ready")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.COMProjection == nil || result.Observation.COMProjection.Status != com.StatusReady {
		t.Fatalf("persisted COM projection = %+v", result.Observation.COMProjection)
	}
	data, err := os.ReadFile(result.ObservationPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	if !strings.Contains(text, `"com_projection"`) || !strings.Contains(text, result.Observation.COMProjection.ProjectionID) {
		t.Fatalf("canonical observation omitted COM projection: %s", text[:minInt(len(text), 1000)])
	}
	for _, forbidden := range []string{"aligned_envelope_frames", "input_event_candidates", "raw_samples", "render_file_path"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("canonical COM observation leaked %s", forbidden)
		}
	}
	read, err := store.Read(ReadRequest{MixSessionID: "mix_com_v2", ObservationID: result.Observation.ObservationID,
		Keys: []string{"observation.com_projection", "observation.catalog"}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(read["items"])
	projection := mapValue(items["observation.com_projection"])
	if cleanAnyString(projection["projection_id"]) != result.Observation.COMProjection.ProjectionID {
		t.Fatalf("project-store COM read mismatch: %+v", projection)
	}
	catalog := mapValue(items["observation.catalog"])
	if !strings.Contains(strings.ToLower(cleanAnyString(catalog["schema_version"])), "catalog") {
		t.Fatalf("project-store catalog missing: %+v", catalog)
	}
}
