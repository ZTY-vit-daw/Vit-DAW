package eqcontrolgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DirectoryOverrideEnv = "VIT_EQ_VPS_CONTROL_GRAPH_DIR"
const AllowCandidateEnv = "VIT_EQ_VPS_ALLOW_CANDIDATE"

type ResolvedDocument struct {
	Path        string
	Document    Document
	Attestation Attestation
	LiveChecks  int
}

func DefaultDirectory() string {
	if value := strings.TrimSpace(os.Getenv(DirectoryOverrideEnv)); value != "" {
		return value
	}
	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		return filepath.Join(appData, "Vit", "Agent", "vps", "eq-control-graph")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".vit", "agent", "vps", "eq-control-graph")
}

func ResolveDirectory(directory string, surface LiveSurface, allowCandidate bool) (*ResolvedDocument, []error, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) || strings.TrimSpace(directory) == "" {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(entries) > 4096 {
		return nil, nil, fmt.Errorf("EQ VPS directory exceeds entry resource limit")
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) })
	var warnings []error
	var matches []ResolvedDocument
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".vps.json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		document, loadErr := LoadDocument(path)
		if loadErr != nil {
			// A syntactically readable document that identifies the live plug-in
			// must fail closed even when the remainder of its schema is invalid.
			// Otherwise a typo or an unknown field could silently disable the VPS
			// and fall back to the generic recognizer.
			if identity, ok := documentIdentityHint(path); ok && pluginFamilyMatches(identity, surface.Plugin) {
				return nil, warnings, fmt.Errorf("matching VPS %s: %w", path, loadErr)
			}
			warnings = append(warnings, loadErr)
			continue
		}
		if !pluginFamilyMatches(document.Plugin, surface.Plugin) {
			continue
		}
		attestation, attestationErr := LoadAttestation(AttestationPath(path))
		if attestationErr != nil {
			return nil, warnings, fmt.Errorf("matching VPS %s attestation: %w", path, attestationErr)
		}
		if attestationErr = ValidateAttestation(document, attestation, allowCandidate); attestationErr != nil {
			return nil, warnings, fmt.Errorf("matching VPS %s: %w", path, attestationErr)
		}
		checks, liveErr := ValidateLive(document, surface)
		if liveErr != nil {
			return nil, warnings, fmt.Errorf("matching VPS %s: %w", path, liveErr)
		}
		if attestation.StaticChecks != checks {
			return nil, warnings, fmt.Errorf("matching VPS %s: attestation static_checks=%d does not match fresh checks=%d",
				path, attestation.StaticChecks, checks)
		}
		matches = append(matches, ResolvedDocument{Path: path, Document: document, Attestation: attestation, LiveChecks: checks})
	}
	if len(matches) > 1 {
		paths := make([]string, 0, len(matches))
		for _, match := range matches {
			paths = append(paths, match.Path)
		}
		return nil, warnings, fmt.Errorf("multiple verified EQ VPS documents match live plugin: %s", strings.Join(paths, ", "))
	}
	if len(matches) == 0 {
		return nil, warnings, nil
	}
	return &matches[0], warnings, nil
}

func documentIdentityHint(path string) (PluginIdentity, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PluginIdentity{}, false
	}
	var envelope struct {
		Plugin PluginIdentity `json:"plugin"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil || strings.TrimSpace(envelope.Plugin.Name) == "" ||
		strings.TrimSpace(envelope.Plugin.Format) == "" {
		return PluginIdentity{}, false
	}
	return envelope.Plugin, true
}

func ValidateAttestation(document Document, attestation Attestation, allowCandidate bool) error {
	if attestation.SchemaVersion != AttestationVersion {
		return fmt.Errorf("attestation schema_version must be %s", AttestationVersion)
	}
	hash, err := HashDocument(document)
	if err != nil {
		return err
	}
	if attestation.VPSHash != hash {
		return fmt.Errorf("attestation vps_hash is stale")
	}
	if attestation.SurfaceSignature != document.SurfaceSignature {
		return fmt.Errorf("attestation surface_signature is stale")
	}
	if attestation.Status == "candidate" {
		if !allowCandidate {
			return fmt.Errorf("candidate VPS is not installable")
		}
		if attestation.StaticChecks <= 0 {
			return fmt.Errorf("candidate attestation lacks static checks")
		}
		return nil
	}
	if attestation.Status != "verified" {
		return fmt.Errorf("attestation status must be verified")
	}
	if attestation.VerifiedAt.IsZero() || attestation.StaticChecks <= 0 {
		return fmt.Errorf("verified attestation is incomplete")
	}
	if !attestation.LiveChecks.ApplyReadback || !attestation.LiveChecks.FormalUndoZeroDrift || !attestation.LiveChecks.UnloadRestoresBaseline {
		return fmt.Errorf("verified attestation lacks required live checks")
	}
	return nil
}

func pluginFamilyMatches(expected, actual PluginIdentity) bool {
	if !strings.EqualFold(strings.TrimSpace(expected.Name), strings.TrimSpace(actual.Name)) ||
		!strings.EqualFold(strings.TrimSpace(expected.Format), strings.TrimSpace(actual.Format)) {
		return false
	}
	return true
}
