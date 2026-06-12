package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
)

const TelemetrySchemaVersion = "vit_llm_telemetry.v1"

type telemetryRecord struct {
	SchemaVersion     string         `json:"schema_version"`
	CreatedAt         string         `json:"created_at"`
	Source            string         `json:"source,omitempty"`
	ConversationID    string         `json:"conversation_id,omitempty"`
	GoalID            string         `json:"goal_id,omitempty"`
	Model             string         `json:"model,omitempty"`
	PromptFingerprint string         `json:"prompt_fingerprint,omitempty"`
	SectionStats      map[string]any `json:"section_stats,omitempty"`
	MessageCount      int            `json:"message_count"`
	RequestBuildMs    int64          `json:"request_build_ms"`
	HTTPMs            int64          `json:"http_ms"`
	TotalMs           int64          `json:"total_ms"`
	Usage             Usage          `json:"usage"`
	Error             string         `json:"error"`
}

func recordTelemetry(cfg config.EngineConfig, req Request, resp Response, callErr error) {
	source := strings.TrimSpace(req.Metadata.Source)
	if source == "" {
		return
	}
	path := telemetryPath()
	if path == "" {
		return
	}
	record := telemetryRecord{
		SchemaVersion:     TelemetrySchemaVersion,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		Source:            source,
		ConversationID:    strings.TrimSpace(req.Metadata.ConversationID),
		GoalID:            strings.TrimSpace(req.Metadata.GoalID),
		Model:             strings.TrimSpace(cfg.DefaultModel),
		PromptFingerprint: strings.TrimSpace(req.Metadata.PromptFingerprint),
		SectionStats:      cloneStats(req.Metadata.PromptStats),
		MessageCount:      len(req.Messages),
		RequestBuildMs:    resp.Timings.RequestBuildMs,
		HTTPMs:            resp.Timings.HTTPMs,
		TotalMs:           resp.Timings.TotalMs,
		Usage:             resp.Usage,
	}
	if callErr != nil {
		record.Error = callErr.Error()
	}
	_ = appendTelemetryJSONL(path, record)
}

func telemetryPath() string {
	if override := strings.TrimSpace(os.Getenv("VIT_AGENT_LLM_TELEMETRY_PATH")); override != "" {
		if strings.EqualFold(override, "off") || strings.EqualFold(override, "disabled") {
			return ""
		}
		return override
	}
	return DefaultTelemetryPath()
}

func DefaultTelemetryPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root := detectWorkspaceRoot(wd); root != "" {
		return filepath.Join(root, "VitApp", "Workspace", "Logs", "agent_llm_telemetry.jsonl")
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Logs", "agent_llm_telemetry.jsonl")
}

func appendTelemetryJSONL(path string, record telemetryRecord) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(record)
}

func cloneStats(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func detectWorkspaceRoot(start string) string {
	for dir := start; dir != ""; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "agent", "go.mod")) && dirExists(filepath.Join(dir, "VitApp")) {
			return dir
		}
		if filepath.Base(dir) == "agent" && fileExists(filepath.Join(dir, "go.mod")) {
			root := filepath.Dir(dir)
			if dirExists(filepath.Join(root, "VitApp")) {
				return root
			}
		}
		if filepath.Base(dir) == "VitApp" && dirExists(filepath.Join(dir, "Workspace")) {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
