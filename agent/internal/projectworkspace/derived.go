package projectworkspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DerivedDirName       = ".vit_derived"
	AnalysisManifestFile = "analysis_manifest.json"
	AnalysisManifestV1   = "vit_analysis_manifest.v1"
)

type AnalysisManifest struct {
	SchemaVersion string           `json:"schema_version"`
	ProjectUUID   string           `json:"project_uuid"`
	ProjectPath   string           `json:"project_path"`
	Analyzer      string           `json:"analyzer,omitempty"`
	Status        string           `json:"status"`
	Rows          []map[string]any `json:"l1_waveform_rows"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

func DerivedDir(projectPath, projectUUID string) (string, error) {
	projectPath = strings.TrimSpace(projectPath)
	projectUUID = safeName(projectUUID)
	if projectPath == "" || projectUUID == "" {
		return "", errors.New("project path and UUID are required")
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(abs), DerivedDirName, projectUUID), nil
}

func SaveAnalysisManifest(projectPath, projectUUID string, status map[string]any) (AnalysisManifest, string, error) {
	manifest := BuildAnalysisManifest(projectPath, projectUUID, status)
	if len(manifest.Rows) == 0 {
		return manifest, "", errors.New("audio analysis status has no L1 waveform rows")
	}
	dir, err := DerivedDir(projectPath, projectUUID)
	if err != nil {
		return manifest, "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return manifest, "", err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, "", err
	}
	target := filepath.Join(dir, AnalysisManifestFile)
	temp := target + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return manifest, "", err
	}
	if err := os.Rename(temp, target); err != nil {
		return manifest, "", err
	}
	return manifest, target, nil
}

func LoadAnalysisManifest(projectPath, projectUUID string) (AnalysisManifest, string, error) {
	dir, err := DerivedDir(projectPath, projectUUID)
	if err != nil {
		return AnalysisManifest{}, "", err
	}
	path := filepath.Join(dir, AnalysisManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return AnalysisManifest{}, path, err
	}
	manifest := AnalysisManifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return AnalysisManifest{}, path, err
	}
	if manifest.SchemaVersion != AnalysisManifestV1 || manifest.ProjectUUID != safeName(projectUUID) {
		return AnalysisManifest{}, path, fmt.Errorf("analysis manifest identity mismatch")
	}
	return manifest, path, nil
}

func ForkDerived(sourcePath, sourceUUID, targetPath, targetUUID string) (string, error) {
	sourceDir, err := DerivedDir(sourcePath, sourceUUID)
	if err != nil {
		return "", err
	}
	targetDir, err := DerivedDir(targetPath, targetUUID)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(sourceDir); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if err := copyDir(sourceDir, targetDir); err != nil {
		return "", err
	}
	manifest, _, err := LoadAnalysisManifest(sourcePath, sourceUUID)
	if err != nil {
		if os.IsNotExist(err) {
			return targetDir, nil
		}
		return "", err
	}
	manifest.ProjectUUID = safeName(targetUUID)
	manifest.ProjectPath = targetPath
	manifest.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, AnalysisManifestFile), data, 0o644); err != nil {
		return "", err
	}
	return targetDir, nil
}

func BuildAnalysisManifest(projectPath, projectUUID string, status map[string]any) AnalysisManifest {
	rows := waveformRows(status)
	compact := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out := map[string]any{}
		for _, key := range []string{
			"status", "reason", "track_id", "clip_id", "file_path", "source_path",
			"source_id", "source_fingerprint", "source_revision", "clip_revision",
			"render_revision", "analyzer_revision", "duration_seconds", "sample_rate",
			"peak_abs", "rms", "peak_dbfs", "rms_dbfs", "crest_db", "headroom_db",
			"frame_count", "feature_stride", "updated_at",
		} {
			if value, ok := row[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
				out[key] = value
			}
		}
		if strings.TrimSpace(fmt.Sprint(out["track_id"])) != "" && strings.TrimSpace(fmt.Sprint(out["clip_id"])) != "" {
			compact = append(compact, out)
		}
	}
	statusText := text(status["dad_fact_status"])
	if statusText == "" {
		statusText = text(status["status"])
	}
	return AnalysisManifest{
		SchemaVersion: AnalysisManifestV1,
		ProjectUUID:   safeName(projectUUID),
		ProjectPath:   projectPath,
		Analyzer:      "waveform_envelope_baker",
		Status:        statusText,
		Rows:          compact,
		UpdatedAt:     time.Now().UTC(),
	}
}

func (m AnalysisManifest) AudioAnalysisStatus(path string) map[string]any {
	ready := 0
	for _, row := range m.Rows {
		if strings.EqualFold(text(row["status"]), "ready") {
			ready++
		}
	}
	return map[string]any{
		"status":                      "ok",
		"command":                     "project.audio_analysis_status",
		"message":                     "Audio analysis restored from project derived manifest",
		"analysis_job_id":             "persisted_manifest",
		"job_id":                      "persisted_manifest",
		"analysis_queue_status":       "restored_manifest",
		"dad_fact_status":             readinessStatus(ready, len(m.Rows)),
		"dad_fact_ready_count":        ready,
		"dad_fact_total_count":        len(m.Rows),
		"dad_fact_pending_count":      len(m.Rows) - ready,
		"dad_fact_failed_count":       0,
		"dad_fact_completion_scope":   "project_derived_manifest",
		"track_waveform_envelopes":    m.Rows,
		"analysis_manifest_path":      path,
		"analysis_manifest_recovered": true,
		"project_uuid":                m.ProjectUUID,
	}
}

func waveformRows(status map[string]any) []map[string]any {
	for _, source := range []map[string]any{status, mapValue(status["analysis_job"])} {
		for _, key := range []string{"track_waveform_envelopes", "l1_waveform_rows"} {
			if rows := mapRows(source[key]); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if mapped, ok := row.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func mapValue(value any) map[string]any {
	row, _ := value.(map[string]any)
	return row
}

func readinessStatus(ready, total int) string {
	if total > 0 && ready == total {
		return "ready"
	}
	if ready > 0 {
		return "partial"
	}
	return "missing"
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "..", "_")
	return strings.Trim(value, " .")
}

func text(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}
