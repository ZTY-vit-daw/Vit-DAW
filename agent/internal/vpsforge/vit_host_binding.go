package vpsforge

// This file records the explicit boundary between the independent Forge VST3
// adapter and a particular Vit host instance.  The two hosts can expose
// different projections of the same VST3 parameter controller (for example,
// Vit exposes its controllable projection while the adapter records the full
// controller).  A projection difference must be observed and bounded; it must
// never be bypassed by pretending the two complete surfaces have the same
// fingerprint.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/vps"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const VitHostBindingSchema = "vit.vpsforge.vit_host_binding.v1"

// VitHostBindingRequest explicitly names the Vit plug-in instance and the
// physical parameter IDs needed by a pending authoring action.  Parameter IDs
// are inputs supplied by the reviewed staging implementation; this operation
// never infers them from a label or candidate badge.
type VitHostBindingRequest struct {
	Root                 string
	AgentURL             string
	TrackID              string
	PluginID             string
	RequiredParameterIDs []string
	Client               *http.Client
	Now                  time.Time
}

type VitHostBindingResult struct {
	Workspace       string                `json:"workspace"`
	Status          string                `json:"status"`
	Compatible      bool                  `json:"compatible"`
	Artifact        string                `json:"artifact"`
	HostSurface     string                `json:"host_surface"`
	HostFingerprint vps.PluginFingerprint `json:"host_fingerprint"`
}

type vitHostBindingArtifact struct {
	SchemaVersion                string                    `json:"schema_version"`
	Trust                        string                    `json:"trust"`
	Status                       string                    `json:"status"`
	CapturedAt                   time.Time                 `json:"captured_at"`
	WorkspaceID                  string                    `json:"workspace_id"`
	AgentEndpoint                string                    `json:"agent_endpoint"`
	Target                       vitHostBindingTarget      `json:"target"`
	AdapterIdentity              vps.PluginIdentity        `json:"adapter_identity"`
	AdapterParameterCount        int                       `json:"adapter_parameter_count"`
	AdapterFullFingerprint       vps.PluginFingerprint     `json:"adapter_full_fingerprint"`
	VitIdentity                  vps.PluginIdentity        `json:"vit_identity"`
	VitParameterCount            int                       `json:"vit_parameter_count"`
	VitHostFingerprint           vps.PluginFingerprint     `json:"vit_host_fingerprint"`
	InstallationFingerprintBasis string                    `json:"installation_fingerprint_basis"`
	AgentResultSHA256            string                    `json:"agent_result_sha256"`
	Projection                   vitHostBindingProjection  `json:"projection"`
	RequiredParameters           []vitHostBindingParameter `json:"required_parameters"`
	AuthorityBoundary            []string                  `json:"authority_boundary"`
	Limitations                  []string                  `json:"limitations"`
}

type vitHostBindingTarget struct {
	TrackID  string `json:"track_id"`
	PluginID string `json:"plugin_id"`
}

type vitHostBindingProjection struct {
	SharedParameterCount      int `json:"shared_parameter_count"`
	AdapterOnlyParameterCount int `json:"adapter_only_parameter_count"`
	VitOnlyParameterCount     int `json:"vit_only_parameter_count"`
}

type vitHostBindingParameter struct {
	ID                  string   `json:"id"`
	AdapterLabel        string   `json:"adapter_label,omitempty"`
	VitLabel            string   `json:"vit_label,omitempty"`
	AdapterStableID     bool     `json:"adapter_stable_id"`
	AdapterControllable bool     `json:"adapter_host_controllable"`
	VitControllable     bool     `json:"vit_host_controllable"`
	AdapterDiscrete     bool     `json:"adapter_discrete"`
	VitDiscrete         bool     `json:"vit_discrete"`
	AdapterBoolean      bool     `json:"adapter_boolean"`
	VitBoolean          bool     `json:"vit_boolean"`
	Status              string   `json:"status"`
	Mismatches          []string `json:"mismatches,omitempty"`
	SelectionBoundary   string   `json:"selection_boundary"`
}

// RecordVitHostBinding captures Vit's real host projection and proves only
// that explicitly selected physical controls remain present and controllable
// for this local instance.  It neither changes the Draft fingerprint nor
// creates a VPS Credential, Catalog entry or SPAL route.
func RecordVitHostBinding(ctx context.Context, request VitHostBindingRequest) (VitHostBindingResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return VitHostBindingResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return VitHostBindingResult{}, fmt.Errorf("inspect Vit host-binding workspace: %w", err)
	}
	if strings.TrimSpace(request.TrackID) == "" || strings.TrimSpace(request.PluginID) == "" {
		return VitHostBindingResult{}, fmt.Errorf("Vit host binding requires track_id and plugin_id")
	}
	required := uniqueSorted(request.RequiredParameterIDs)
	if len(required) == 0 {
		return VitHostBindingResult{}, fmt.Errorf("Vit host binding requires explicit required parameter IDs")
	}
	var adapterSurface SurfaceSnapshot
	if err := readJSON(filepath.Join(root, surfaceFile), &adapterSurface); err != nil {
		return VitHostBindingResult{}, fmt.Errorf("read independent adapter surface: %w", err)
	}
	if adapterSurface.PluginIdentity.Fingerprint.Installation == "" {
		return VitHostBindingResult{}, fmt.Errorf("independent adapter surface has no installation fingerprint")
	}
	result, endpoint, err := readVitHostParameterSurface(ctx, request.AgentURL, request.TrackID, request.PluginID, request.Client)
	if err != nil {
		return VitHostBindingResult{}, err
	}
	digest := plugingrabber.BuildParameterDigest(result)
	if len(digest.Parameters) == 0 {
		return VitHostBindingResult{}, fmt.Errorf("Vit host binding response has no parameter surface")
	}
	vitIdentity := vitHostBindingIdentity(digest.PluginIdentity)
	if err := validateVitHostBindingIdentity(adapterSurface.PluginIdentity, vitIdentity); err != nil {
		return VitHostBindingResult{}, err
	}
	vitFingerprint, err := vps.BuildPluginFingerprintFromDigest(adapterSurface.PluginIdentity.Fingerprint.Installation, digest)
	if err != nil {
		return VitHostBindingResult{}, fmt.Errorf("fingerprint Vit host projection: %w", err)
	}
	parameters, projection, compatible := compareVitHostProjection(adapterSurface.Parameters, digest.Parameters, required)
	now := nowOrCurrent(request.Now)
	encodedResult, err := json.Marshal(result)
	if err != nil {
		return VitHostBindingResult{}, err
	}
	sum := sha256.Sum256(encodedResult)
	artifact := vitHostBindingArtifact{
		SchemaVersion: VitHostBindingSchema,
		Trust:         "observed",
		Status:        map[bool]string{true: "observed_host_projection_compatible", false: "observed_host_projection_incompatible"}[compatible],
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		AgentEndpoint: endpoint,
		Target: vitHostBindingTarget{
			TrackID: strings.TrimSpace(request.TrackID), PluginID: strings.TrimSpace(request.PluginID),
		},
		AdapterIdentity:              adapterSurface.PluginIdentity,
		AdapterParameterCount:        len(adapterSurface.Parameters),
		AdapterFullFingerprint:       adapterSurface.PluginIdentity.Fingerprint,
		VitIdentity:                  vitIdentity,
		VitParameterCount:            len(digest.Parameters),
		VitHostFingerprint:           vitFingerprint,
		InstallationFingerprintBasis: "independent_vst3_adapter_file_fingerprint_after_exact_identity_and_install-path_match",
		AgentResultSHA256:            "sha256:" + hex.EncodeToString(sum[:]),
		Projection:                   projection,
		RequiredParameters:           parameters,
		AuthorityBoundary: []string{
			"This records an observed host-projection binding for one selected Vit plug-in instance.",
			"It does not make the independent full adapter surface equal to Vit's controllable projection.",
			"It does not define parameter semantics, grant a task badge, issue a Credential, mutate Catalog, or enable SPAL dispatch.",
		},
		Limitations: []string{
			"The Vit host fingerprint is a host-specific dispatch guard, not a cross-host plugin identity claim.",
			"Only the explicitly requested physical IDs are checked for mapping availability; unrelated projection differences remain observed evidence.",
		},
	}
	hostSurface := map[string]any{
		"schema_version": VitHostBindingSchema,
		"trust":          "observed",
		"captured_at":    now,
		"source":         endpoint,
		"target":         artifact.Target,
		"result":         result,
	}
	if err := writeJSON(filepath.Join(root, vitHostSurfaceFile), hostSurface); err != nil {
		return VitHostBindingResult{}, err
	}
	if err := writeJSON(filepath.Join(root, vitHostBindingFile), artifact); err != nil {
		return VitHostBindingResult{}, err
	}
	for _, entry := range []EvidenceEntry{
		{Kind: "vit_host_surface", Trust: "observed", Source: endpoint, Summary: "Captured Vit's complete host-visible parameter projection for the explicitly selected plug-in instance.", Artifact: vitHostSurfaceFile, CapturedAt: now, Data: encodedResult},
		{Kind: "vit_host_binding", Trust: "observed", Source: endpoint, Summary: "Compared the independent VST3 surface with Vit's host projection for explicit physical mapping IDs; this is not Credential or routing authority.", Artifact: vitHostBindingFile, CapturedAt: now, Data: mustMarshalVitHostBindingSummary(artifact)},
	} {
		if err := appendEvidence(root, entry); err != nil {
			return VitHostBindingResult{}, err
		}
	}
	if err := updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["vit_host_surface_snapshot"] = vitHostSurfaceFile
		manifest.Artifacts["vit_host_binding"] = vitHostBindingFile
		manifest.UpdatedAt = now
	}); err != nil {
		return VitHostBindingResult{}, err
	}
	return VitHostBindingResult{Workspace: root, Status: artifact.Status, Compatible: compatible, Artifact: vitHostBindingFile, HostSurface: vitHostSurfaceFile, HostFingerprint: vitFingerprint}, nil
}

func mustMarshalVitHostBindingSummary(artifact vitHostBindingArtifact) json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"status": artifact.Status, "target": artifact.Target, "adapter_parameter_count": artifact.AdapterParameterCount,
		"vit_parameter_count": artifact.VitParameterCount, "projection": artifact.Projection,
		"required_parameters": artifact.RequiredParameters, "vit_host_fingerprint": artifact.VitHostFingerprint,
	})
	return data
}

func readVitHostParameterSurface(ctx context.Context, endpoint, trackID, pluginID string, client *http.Client) (map[string]any, string, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return nil, "", fmt.Errorf("Vit host binding Agent URL must be an http URL")
	}
	host := parsed.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, "", fmt.Errorf("Vit host binding Agent URL must target a loopback address")
		}
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	payload, err := json.Marshal(map[string]any{
		"tool": "plugin.get_parameters",
		"args": map[string]any{
			"track_id": trackID, "plugin_id": pluginID, "include_vps_v3_surface": true,
		},
		"source":    "vpsforge_vit_host_binding",
		"confirmed": false,
	})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/agent/invoke", bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	var envelope struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
		Error  string         `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, "", fmt.Errorf("decode Vit host binding response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !strings.EqualFold(envelope.Status, "ok") || strings.TrimSpace(envelope.Error) != "" {
		return nil, "", fmt.Errorf("Vit host binding parameter read failed: %s", firstNonEmpty(strings.TrimSpace(envelope.Error), strings.TrimSpace(string(data))))
	}
	return envelope.Result, endpoint, nil
}

func vitHostBindingIdentity(raw map[string]any) vps.PluginIdentity {
	return vps.PluginIdentity{
		Manufacturer: firstVitHostBindingText(raw, "manufacturer", "vendor", "maker"),
		Name:         firstVitHostBindingText(raw, "plugin_name", "name"),
		Format:       strings.ToUpper(firstVitHostBindingText(raw, "plugin_format", "format")),
		Version:      firstVitHostBindingText(raw, "version", "plugin_version"),
		InstallPath:  firstVitHostBindingText(raw, "plugin_path", "install_path", "path"),
		ProfileKey:   firstVitHostBindingText(raw, "profile_key"),
	}
}

func firstVitHostBindingText(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, found := values[key]; found {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func validateVitHostBindingIdentity(adapter, vit vps.PluginIdentity) error {
	for _, field := range []struct{ name, want, got string }{
		{"manufacturer", adapter.Manufacturer, vit.Manufacturer},
		{"name", adapter.Name, vit.Name},
		{"format", adapter.Format, vit.Format},
		{"version", adapter.Version, vit.Version},
		{"install path", adapter.InstallPath, vit.InstallPath},
	} {
		if strings.TrimSpace(field.want) == "" {
			continue
		}
		if strings.TrimSpace(field.got) == "" {
			return fmt.Errorf("Vit host binding lacks plugin %s needed to match the independent Adapter", field.name)
		}
		if field.name == "install path" {
			if !sameInstallPath(field.want, field.got) {
				return fmt.Errorf("Vit host binding install path does not match the independent Adapter")
			}
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(field.want), strings.TrimSpace(field.got)) {
			return fmt.Errorf("Vit host binding %s mismatch (adapter %q, Vit %q)", field.name, field.want, field.got)
		}
	}
	return nil
}

func compareVitHostProjection(adapter []SurfaceParameter, vit []plugingrabber.ParameterInfo, required []string) ([]vitHostBindingParameter, vitHostBindingProjection, bool) {
	adapterByID := map[string]SurfaceParameter{}
	for _, parameter := range adapter {
		adapterByID[strings.TrimSpace(parameter.ID)] = parameter
	}
	vitByID := map[string]plugingrabber.ParameterInfo{}
	for _, parameter := range vit {
		vitByID[strings.TrimSpace(parameter.ID)] = parameter
	}
	projection := vitHostBindingProjection{}
	for id := range adapterByID {
		if _, found := vitByID[id]; found {
			projection.SharedParameterCount++
		} else {
			projection.AdapterOnlyParameterCount++
		}
	}
	for id := range vitByID {
		if _, found := adapterByID[id]; !found {
			projection.VitOnlyParameterCount++
		}
	}
	parameters := make([]vitHostBindingParameter, 0, len(required))
	compatible := true
	for _, id := range required {
		row := vitHostBindingParameter{ID: id, Status: "matched_observed", SelectionBoundary: "This physical ID was explicitly supplied by the reviewed staging action; it is checked only for local host availability, not interpreted as semantic authority."}
		adapterParameter, adapterFound := adapterByID[id]
		vitParameter, vitFound := vitByID[id]
		if !adapterFound {
			row.Status = "missing_from_independent_adapter"
			row.Mismatches = append(row.Mismatches, "required physical ID is absent from the independent Adapter surface")
		}
		if !vitFound {
			row.Status = "missing_from_vit_host"
			row.Mismatches = append(row.Mismatches, "required physical ID is absent from the Vit host projection")
		}
		if adapterFound {
			row.AdapterLabel = adapterParameter.Name
			row.AdapterStableID = adapterParameter.StableID
			row.AdapterControllable = adapterParameter.HostControllable
			row.AdapterDiscrete = vitHostBindingBool(adapterParameter.Observed["is_discrete"])
			row.AdapterBoolean = vitHostBindingBool(adapterParameter.Observed["is_boolean"])
			if !adapterParameter.StableID {
				row.Mismatches = append(row.Mismatches, "independent Adapter did not mark this ID stable")
			}
			if !adapterParameter.HostControllable {
				row.Mismatches = append(row.Mismatches, "independent Adapter did not mark this ID host-controllable")
			}
		}
		if vitFound {
			row.VitLabel = vitParameter.Name
			row.VitControllable = vitParameter.HostControllable
			row.VitDiscrete = vitParameter.IsDiscrete
			row.VitBoolean = vitParameter.IsBoolean
			if !vitParameter.HostControllable {
				row.Mismatches = append(row.Mismatches, "Vit does not mark this required ID host-controllable")
			}
		}
		if adapterFound && vitFound {
			if row.AdapterLabel != "" && row.VitLabel != "" && !strings.EqualFold(strings.TrimSpace(row.AdapterLabel), strings.TrimSpace(row.VitLabel)) {
				row.Mismatches = append(row.Mismatches, "host labels differ between the Adapter and Vit")
			}
			if row.AdapterDiscrete != row.VitDiscrete || row.AdapterBoolean != row.VitBoolean {
				row.Mismatches = append(row.Mismatches, "observed control kind differs between the Adapter and Vit")
			}
		}
		if len(row.Mismatches) > 0 {
			row.Status = "incompatible"
			compatible = false
		}
		parameters = append(parameters, row)
	}
	return parameters, projection, compatible
}

func vitHostBindingBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}
