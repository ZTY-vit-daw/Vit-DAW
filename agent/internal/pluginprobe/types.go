package pluginprobe

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const SurfaceSchema = "vit.pluginprobe.surface_snapshot.v1"

type PluginFingerprint struct {
	Installation     string `json:"installation,omitempty"`
	ParameterSurface string `json:"parameter_surface,omitempty"`
	DisplaySurface   string `json:"display_surface,omitempty"`
}

type PluginIdentity struct {
	Manufacturer string            `json:"manufacturer,omitempty"`
	Name         string            `json:"name"`
	Format       string            `json:"format"`
	Version      string            `json:"version,omitempty"`
	InstallPath  string            `json:"install_path,omitempty"`
	Fingerprint  PluginFingerprint `json:"fingerprint,omitempty"`
}

type DisplayDomain struct {
	Text  string   `json:"text,omitempty"`
	Unit  string   `json:"unit,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Scale string   `json:"scale,omitempty"`
}

type SurfaceSnapshot struct {
	SchemaVersion    string             `json:"schema_version"`
	PluginIdentity   PluginIdentity     `json:"plugin_identity"`
	Parameters       []SurfaceParameter `json:"parameters"`
	DisplaySurface   map[string]any     `json:"display_surface,omitempty"`
	HostCapabilities map[string]any     `json:"host_capabilities,omitempty"`
	Logs             []string           `json:"logs,omitempty"`
	CapturedAt       time.Time          `json:"captured_at"`
}

type SurfaceParameter struct {
	ID                     string         `json:"id"`
	Name                   string         `json:"name,omitempty"`
	NormalizedValue        *float64       `json:"normalized_value,omitempty"`
	DefaultNormalizedValue *float64       `json:"default_normalized_value,omitempty"`
	DisplayText            string         `json:"display_text,omitempty"`
	Unit                   string         `json:"unit,omitempty"`
	Automation             string         `json:"automation,omitempty"`
	IDProvenance           string         `json:"id_provenance,omitempty"`
	StableID               bool           `json:"stable_id,omitempty"`
	HostControllable       bool           `json:"host_controllable"`
	DisplayDomain          DisplayDomain  `json:"display_domain,omitempty"`
	Observed               map[string]any `json:"observed,omitempty"`
}

type HostSnapshot struct {
	Identity        PluginIdentity  `json:"plugin_identity"`
	Surface         SurfaceSnapshot `json:"surface"`
	Logs            []string        `json:"logs,omitempty"`
	AdapterSnapshot json.RawMessage `json:"adapter_snapshot,omitempty"`
}

func writeHTTP(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
