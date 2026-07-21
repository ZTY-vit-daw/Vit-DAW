package vpsforge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"vit-daw-agent/internal/fxm"
)

func float(value float64) *float64 { return &value }
func TestWorkspaceRecordsGenericSurfaceEvidenceAndFXM(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nova")
	status, err := Init(InitRequest{Root: root, Identity: PluginIdentity{Manufacturer: "Tokyo Dawn Labs", Name: "TDR Nova", Format: "VST3", Version: "2.2.2"}, Capabilities: []string{"equalizer.v2"}, Now: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if status.Manifest.Status != "discovery" || status.Validation.InstallReady {
		t.Fatalf("unsafe initial workspace: %+v", status)
	}
	window := fxm.MeasurementWindow{SourceRevision: "source", StartSeconds: 0, EndSeconds: 3, SampleRate: 48000, ChannelCount: 2}
	quality := fxm.QualityEvidence{Deterministic: true, LatencyCompensated: true, Nonzero: true, Coverage: 1}
	status, err = RecordFXM(root, fxm.Input{CreatedAt: "2026-07-17T00:01:00Z", Conditions: window, Baseline: fxm.Measurement{ID: "before", Stage: "bypass_chain", Status: "ready", SourceRevision: "source", Window: window, RMSDBFS: float(-18), Quality: quality}, Processed: fxm.Measurement{ID: "after", Stage: "processed_chain", Status: "ready", SourceRevision: "source", Window: window, RMSDBFS: float(-16), Quality: quality}}, time.Date(2026, 7, 17, 0, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !status.FXMAvailable || status.FXMStatus != fxm.StatusReady || status.Validation.InstallReady {
		t.Fatalf("FXM was not recorded safely: %+v", status)
	}
}
func TestLegacyAuthoringManifestRemainsReadableAfterForgeRename(t *testing.T) {
	root := filepath.Join(t.TempDir(), "legacy")
	if _, err := Init(InitRequest{Root: root, Identity: PluginIdentity{Manufacturer: "Vit Test", Name: "Legacy Workspace", Format: "VST3", Version: "0"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, manifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), ManifestSchema, LegacyManifestSchema, 1))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	validation := Validate(root)
	if validation.Status != "valid" || len(validation.Errors) != 0 || len(validation.Warnings) == 0 {
		t.Fatalf("legacy workspace should be accepted with a migration warning: %+v", validation)
	}
}
