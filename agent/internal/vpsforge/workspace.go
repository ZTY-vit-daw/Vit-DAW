package vpsforge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"vit-daw-agent/internal/fxm"
)

const (
	manifestFile      = "authoring_manifest.json"
	ledgerFile        = "evidence_ledger.json"
	surfaceFile       = "surface_snapshot.json"
	fxmProjectionFile = "fxm_projection.json"
)

type InitRequest struct {
	Root         string
	Identity     PluginIdentity
	Capabilities []string
	HostProtocol string
	HostEndpoint string
	Now          time.Time
}

func authoringWorkspaceRoot(value string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(value))
	if root == "" || root == "." {
		return "", fmt.Errorf("authoring workspace root is required")
	}
	return root, nil
}
func Init(req InitRequest) (Status, error) {
	root, err := authoringWorkspaceRoot(req.Root)
	if err != nil {
		return Status{}, err
	}
	if _, err = os.Stat(filepath.Join(root, manifestFile)); err == nil {
		return Status{}, fmt.Errorf("authoring workspace already exists: %s", root)
	}
	req.Identity.Format = strings.ToUpper(strings.TrimSpace(req.Identity.Format))
	if !supportedAuthoringFormat(req.Identity.Format) {
		return Status{}, fmt.Errorf("unsupported authoring format %q; expected VST3, CLAP, AAX or AU", req.Identity.Format)
	}
	if strings.TrimSpace(req.Identity.Name) == "" {
		return Status{}, fmt.Errorf("plugin name is required")
	}
	now := nowOrCurrent(req.Now)
	caps := uniqueSorted(req.Capabilities)
	if len(caps) == 0 {
		caps = []string{"unknown"}
	}
	id := stableID("vps_author", req.Identity.Manufacturer, req.Identity.Name, req.Identity.Format, req.Identity.Version, now.Format(time.RFC3339Nano))
	m := Manifest{SchemaVersion: ManifestSchema, WorkspaceID: id, PluginIdentity: req.Identity, CapabilityIDs: caps, Status: "discovery", HostAdapter: HostAdapter{Protocol: firstNonEmpty(req.HostProtocol, "vit.vps_host_adapter.v1"), Endpoint: strings.TrimSpace(req.HostEndpoint), PluginFormat: req.Identity.Format, Status: "unconnected"}, Artifacts: map[string]string{"evidence_ledger": ledgerFile, "surface_snapshot": surfaceFile, "fxm_projection": fxmProjectionFile, "workspace_readme": "README.md", "agent_protocol": "agent_protocol.json"}, CreatedAt: now, UpdatedAt: now, Limitations: []string{"authoring workspace only; no runtime installation or project mutation", "automatic host facts are observed evidence and never semantic authority"}}
	l := EvidenceLedger{SchemaVersion: LedgerSchema, WorkspaceID: id, Entries: []EvidenceEntry{{ID: stableID("evidence", id, "authoring-init"), Kind: "authoring_intent", Trust: "user-confirmed", Source: "vpsforge.init", Summary: "Created an isolated surface-observation workspace.", CapturedAt: now}}}
	if err = os.MkdirAll(root, 0755); err != nil {
		return Status{}, err
	}
	if err = writeJSON(filepath.Join(root, manifestFile), m); err != nil {
		return Status{}, err
	}
	if err = writeJSON(filepath.Join(root, ledgerFile), l); err != nil {
		return Status{}, err
	}
	if err = writeAgentGuide(root, m); err != nil {
		return Status{}, err
	}
	return Inspect(root)
}
func Inspect(root string) (Status, error) {
	root, err := authoringWorkspaceRoot(root)
	if err != nil {
		return Status{}, err
	}
	var m Manifest
	var l EvidenceLedger
	if err = readJSON(filepath.Join(root, manifestFile), &m); err != nil {
		return Status{}, err
	}
	if err = readJSON(filepath.Join(root, ledgerFile), &l); err != nil {
		return Status{}, err
	}
	s := Status{Workspace: root, Manifest: m, EvidenceCount: len(l.Entries)}
	if _, err = os.Stat(filepath.Join(root, surfaceFile)); err == nil {
		s.SurfaceAvailable = true
	}
	var a FXMArtifact
	if readJSON(filepath.Join(root, fxmProjectionFile), &a) == nil {
		s.FXMAvailable = true
		s.FXMStatus = a.Projection.Status
	}
	s.Validation = Validate(root)
	return s, nil
}
func Validate(root string) ValidationResult {
	errs, warns := []string{}, []string{}
	var m Manifest
	var l EvidenceLedger
	if err := readJSON(filepath.Join(root, manifestFile), &m); err != nil {
		errs = append(errs, err.Error())
	}
	if err := readJSON(filepath.Join(root, ledgerFile), &l); err != nil {
		errs = append(errs, err.Error())
	}
	if m.SchemaVersion != ManifestSchema && m.SchemaVersion != LegacyManifestSchema {
		errs = append(errs, "authoring manifest schema mismatch")
	} else if m.SchemaVersion == LegacyManifestSchema {
		warns = append(warns, "legacy VPS Forge manifest accepted read-only; new workspaces use "+ManifestSchema)
	}
	if l.SchemaVersion != LedgerSchema {
		errs = append(errs, "evidence ledger schema mismatch")
	}
	if _, err := os.Stat(filepath.Join(root, surfaceFile)); err != nil {
		warns = append(warns, "host parameter/display surface has not been captured")
	}
	var a FXMArtifact
	if err := readJSON(filepath.Join(root, fxmProjectionFile), &a); err != nil {
		warns = append(warns, "FXM observation has not been captured")
	}
	status := "valid"
	if len(errs) > 0 {
		status = "invalid"
	}
	return ValidationResult{Status: status, Errors: errs, Warnings: warns, InstallReady: false, NextSteps: []string{"capture a real host surface", "draft a minimal VPS file", "verify write/readback/display/rollback before installation"}}
}
func IngestSurface(root string, s SurfaceSnapshot, source string, now time.Time) (Status, error) {
	if s.SchemaVersion == "" {
		s.SchemaVersion = SurfaceSchema
	}
	if s.SchemaVersion != SurfaceSchema {
		return Status{}, fmt.Errorf("surface snapshot schema must be %s", SurfaceSchema)
	}
	if s.CapturedAt.IsZero() {
		s.CapturedAt = nowOrCurrent(now)
	}
	var m Manifest
	if err := readJSON(filepath.Join(root, manifestFile), &m); err != nil {
		return Status{}, err
	}
	if err := samePluginFamily(m.PluginIdentity, s.PluginIdentity); err != nil {
		return Status{}, err
	}
	seen := map[string]bool{}
	for _, p := range s.Parameters {
		id := strings.TrimSpace(p.ID)
		if id == "" || seen[id] {
			return Status{}, fmt.Errorf("surface snapshot contains a missing or duplicate parameter id %q", p.ID)
		}
		seen[id] = true
	}
	if err := writeJSON(filepath.Join(root, surfaceFile), s); err != nil {
		return Status{}, err
	}
	data, _ := json.Marshal(s)
	if err := appendEvidence(root, EvidenceEntry{Kind: "host_surface", Trust: "observed", Source: firstNonEmpty(source, "host_adapter"), Summary: fmt.Sprintf("Captured %d host parameters and display-surface facts.", len(s.Parameters)), Artifact: surfaceFile, CapturedAt: s.CapturedAt, Data: data}); err != nil {
		return Status{}, err
	}
	if err := updateManifest(root, func(m *Manifest) {
		m.Status = "surface_observed"
		m.HostAdapter.Status = "connected"
		m.UpdatedAt = s.CapturedAt
	}); err != nil {
		return Status{}, err
	}
	return Inspect(root)
}
func RecordFXM(root string, input fxm.Input, now time.Time) (Status, error) {
	if input.CreatedAt == "" {
		input.CreatedAt = nowOrCurrent(now).Format(time.RFC3339Nano)
	}
	p := fxm.Build(input)
	if err := writeJSON(filepath.Join(root, fxmProjectionFile), FXMArtifact{Input: input, Projection: p}); err != nil {
		return Status{}, err
	}
	data, _ := json.Marshal(p)
	if err := appendEvidence(root, EvidenceEntry{Kind: "fxm_projection", Trust: "observed", Source: "vpsforge.measure", Summary: p.LLMContext.SummaryMD, Artifact: fxmProjectionFile, CapturedAt: nowOrCurrent(now), Data: data}); err != nil {
		return Status{}, err
	}
	_ = updateManifest(root, func(m *Manifest) { m.Status = "draft_ready"; m.UpdatedAt = nowOrCurrent(now) })
	return Inspect(root)
}
func ProbeHost(ctx context.Context, root, baseURL string, client *http.Client) (Status, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return Status{}, fmt.Errorf("host adapter endpoint is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/plugin/snapshot", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Status{}, fmt.Errorf("host adapter returned %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var snap HostSnapshot
	if err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&snap); err != nil {
		return Status{}, err
	}
	if snap.Surface.SchemaVersion == "" {
		snap.Surface.SchemaVersion = SurfaceSchema
	}
	if snap.Surface.PluginIdentity.Name == "" {
		snap.Surface.PluginIdentity = snap.Identity
	}
	status, err := IngestSurface(root, snap.Surface, baseURL, time.Now().UTC())
	if err == nil {
		_ = updateManifest(root, func(m *Manifest) { m.HostAdapter.Endpoint = baseURL; m.HostAdapter.Status = "connected" })
	}
	return status, err
}
func appendEvidence(root string, e EvidenceEntry) error {
	var l EvidenceLedger
	if err := readJSON(filepath.Join(root, ledgerFile), &l); err != nil {
		return err
	}
	e.CapturedAt = nowOrCurrent(e.CapturedAt)
	e.ID = stableID("evidence", l.WorkspaceID, e.Kind, e.Source, e.CapturedAt.Format(time.RFC3339Nano))
	l.Entries = append(l.Entries, e)
	return writeJSON(filepath.Join(root, ledgerFile), l)
}
func updateManifest(root string, fn func(*Manifest)) error {
	var m Manifest
	if err := readJSON(filepath.Join(root, manifestFile), &m); err != nil {
		return err
	}
	fn(&m)
	return writeJSON(filepath.Join(root, manifestFile), m)
}
func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func stableID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:12])
}
func uniqueSorted(v []string) []string {
	m := map[string]bool{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		if x != "" {
			m[x] = true
		}
	}
	out := make([]string, 0, len(m))
	for x := range m {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}
func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
func nowOrCurrent(v time.Time) time.Time {
	if v.IsZero() {
		return time.Now().UTC()
	}
	return v.UTC()
}
func supportedAuthoringFormat(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "VST3", "CLAP", "AAX", "AU":
		return true
	}
	return false
}
func samePluginFamily(a, b PluginIdentity) error {
	for _, x := range []struct{ name, a, b string }{{"manufacturer", a.Manufacturer, b.Manufacturer}, {"name", a.Name, b.Name}, {"format", a.Format, b.Format}} {
		if strings.TrimSpace(x.a) != "" && strings.TrimSpace(x.b) != "" && !strings.EqualFold(strings.TrimSpace(x.a), strings.TrimSpace(x.b)) {
			return fmt.Errorf("plugin %s mismatch", x.name)
		}
	}
	return nil
}
