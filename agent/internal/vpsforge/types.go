// Package vpsforge implements the staging-only Vit VPS Forge workspace. It
// never writes the canonical VPS Library or issues Provider Credentials.
package vpsforge

import (
	"encoding/json"
	"time"

	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/vps"
)

const (
	ManifestSchema       = "vit.vpsforge.authoring_manifest.v1"
	LegacyManifestSchema = "vit.vps_authoring_manifest.v1"
	LedgerSchema         = "vit.vps_evidence_ledger.v1"
	SurfaceSchema        = "vit.vps_surface_snapshot.v1"
)

type Manifest struct {
	SchemaVersion  string             `json:"schema_version"`
	WorkspaceID    string             `json:"workspace_id"`
	PluginIdentity vps.PluginIdentity `json:"plugin_identity"`
	CapabilityIDs  []string           `json:"capability_ids"`
	Status         string             `json:"status"`
	HostAdapter    HostAdapter        `json:"host_adapter"`
	Artifacts      map[string]string  `json:"artifacts"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	Limitations    []string           `json:"limitations,omitempty"`
}

type HostAdapter struct {
	Protocol     string `json:"protocol"`
	Endpoint     string `json:"endpoint,omitempty"`
	PluginFormat string `json:"plugin_format"`
	Status       string `json:"status"`
}

type EvidenceLedger struct {
	SchemaVersion string          `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id"`
	Entries       []EvidenceEntry `json:"entries"`
}

type EvidenceEntry struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Trust      string          `json:"trust"`
	Source     string          `json:"source"`
	Summary    string          `json:"summary"`
	Artifact   string          `json:"artifact,omitempty"`
	CapturedAt time.Time       `json:"captured_at"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type SurfaceSnapshot struct {
	SchemaVersion    string             `json:"schema_version"`
	PluginIdentity   vps.PluginIdentity `json:"plugin_identity"`
	Parameters       []SurfaceParameter `json:"parameters"`
	DisplaySurface   map[string]any     `json:"display_surface,omitempty"`
	HostCapabilities map[string]any     `json:"host_capabilities,omitempty"`
	Logs             []string           `json:"logs,omitempty"`
	CapturedAt       time.Time          `json:"captured_at"`
}

type SurfaceParameter struct {
	ID                     string            `json:"id"`
	Name                   string            `json:"name,omitempty"`
	NormalizedValue        *float64          `json:"normalized_value,omitempty"`
	DefaultNormalizedValue *float64          `json:"default_normalized_value,omitempty"`
	DisplayText            string            `json:"display_text,omitempty"`
	Unit                   string            `json:"unit,omitempty"`
	Automation             string            `json:"automation,omitempty"`
	IDProvenance           string            `json:"id_provenance,omitempty"`
	StableID               bool              `json:"stable_id,omitempty"`
	HostControllable       bool              `json:"host_controllable"`
	DisplayDomain          vps.DisplayDomain `json:"display_domain,omitempty"`
	Observed               map[string]any    `json:"observed,omitempty"`
}

type HostSnapshot struct {
	Identity        vps.PluginIdentity `json:"plugin_identity"`
	Surface         SurfaceSnapshot    `json:"surface"`
	Logs            []string           `json:"logs,omitempty"`
	AdapterSnapshot json.RawMessage    `json:"adapter_snapshot,omitempty"`
}

type Status struct {
	Workspace        string           `json:"workspace"`
	Manifest         Manifest         `json:"manifest"`
	EvidenceCount    int              `json:"evidence_count"`
	SurfaceAvailable bool             `json:"surface_available"`
	FXMAvailable     bool             `json:"fxm_available"`
	FXMStatus        string           `json:"fxm_status,omitempty"`
	Draft            vps.VPSDocument  `json:"vps_draft"`
	Validation       ValidationResult `json:"validation"`
}

type ValidationResult struct {
	Status       string   `json:"status"`
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	InstallReady bool     `json:"install_ready"`
	NextSteps    []string `json:"next_steps,omitempty"`
}

type FXMArtifact struct {
	Input      fxm.Input      `json:"input"`
	Projection fxm.Projection `json:"projection"`
}
