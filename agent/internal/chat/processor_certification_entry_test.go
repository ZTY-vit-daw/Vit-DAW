package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

func TestProcessorCertificationCandidatesReportPromotedAndStaleFingerprint(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Processor.vst3")
	if err := os.WriteFile(pluginPath, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	semanticsPath := filepath.Join(root, "plugin_semantics.json")
	storePath := filepath.Join(root, "attestations.v2.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", storePath)
	entry := pluginsemantics.Entry{ID: "processor-id", Name: "Processor", Manufacturer: "Vendor", Format: "VST3", Identifier: "processor-id", PluginPath: pluginPath, UpdatedAt: time.Now().UTC()}
	if _, err := pluginsemantics.Save(semanticsPath, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStoreV2(storePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject:           processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, InstalledPath: entry.PluginPath},
		BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyLimiter,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}},
	}, "test_promoted")
	if err != nil {
		t.Fatal(err)
	}
	server := New(nil, nil, nil)
	readCandidate := func() processorCertificationCandidate {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/agent/processor-certification/candidates?family=limiter", nil)
		server.Routes().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var body struct {
			Families   []string                          `json:"families"`
			Candidates []processorCertificationCandidate `json:"candidates"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Candidates) != 1 || !containsText(body.Families, "spectral_dynamics") || !containsText(body.Families, "static_eq") {
			t.Fatalf("body=%+v", body)
		}
		return body.Candidates[0]
	}
	if candidate := readCandidate(); candidate.Status != processorattestation.StatusPromoted || !candidate.FingerprintMatch || candidate.CoverageLevel != "partial" {
		t.Fatalf("candidate=%+v", candidate)
	}
	if err := os.WriteFile(pluginPath, []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if candidate := readCandidate(); candidate.Status != processorattestation.StatusStale || candidate.FingerprintMatch || candidate.Reason != "binary_fingerprint_changed" {
		t.Fatalf("stale candidate=%+v", candidate)
	}
}

func TestProcessorCertificationStartRequiresExplicitConsent(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Processor.vst3")
	if err := os.WriteFile(pluginPath, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	semanticsPath := filepath.Join(root, "plugin_semantics.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	entry := pluginsemantics.Entry{ID: "processor-id", Name: "Processor", Format: "VST3", Identifier: "processor-id", PluginPath: pluginPath, UpdatedAt: time.Now().UTC()}
	if _, err := pluginsemantics.Save(semanticsPath, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"identifier": entry.Identifier, "family": processorattestation.FamilyLimiter})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/processor-certification/start", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	server := New(nil, nil, nil)
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "needs_confirmation") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if running := server.runningProcessorCertificationJob(); running != nil {
		t.Fatalf("unconfirmed request started job %+v", running)
	}
}

func TestProcessorCertificationAllViewIsPluginFirstAndShowsV1AndBoundaries(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Processor.vst3")
	if err := os.WriteFile(pluginPath, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	semanticsPath := filepath.Join(root, "plugin_semantics.json")
	v1Path := filepath.Join(root, "attestations.v1.json")
	v2Path := filepath.Join(root, "attestations.v2.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", v1Path)
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", v2Path)
	entry := pluginsemantics.Entry{ID: "processor-id", Name: "Processor", Manufacturer: "Vendor", Format: "VST3", Identifier: "processor-id", PluginPath: pluginPath, UpdatedAt: time.Now().UTC()}
	if _, err := pluginsemantics.Save(semanticsPath, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	v1Store, err := processorattestation.NewStore(v1Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v1Store.PromoteCurrent(processorattestation.IssueSpec{
		Subject:           processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, InstalledPath: entry.PluginPath},
		BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "eq-receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}},
	}, "test_promoted"); err != nil {
		t.Fatal(err)
	}
	server := New(nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/agent/processor-certification/candidates?family=all", nil)
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Candidates []processorCertificationCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Candidates) != 1 || body.Candidates[0].Family != "all" {
		t.Fatalf("plugin-first candidates=%+v", body.Candidates)
	}
	statuses := map[string]string{}
	reasons := map[string]string{}
	for _, capability := range body.Candidates[0].Capabilities {
		statuses[capability.Family] = capability.Status
		reasons[capability.Family] = capability.Reason
	}
	if statuses[processorattestation.FamilyStaticEQ] != processorattestation.StatusPromoted {
		t.Fatalf("static EQ status=%q", statuses[processorattestation.FamilyStaticEQ])
	}
	if statuses["spectral_dynamics"] != "inspect_only" || statuses["clipper"] != "separate_boundary" {
		t.Fatalf("boundary statuses=%+v", statuses)
	}
	if reasons["clipper"] != "independent_clipper_controller_not_in_this_entry" {
		t.Fatalf("clipper reason=%q", reasons["clipper"])
	}
}

func TestProcessorCertificationStartAcceptsBroadbandCompressorCapability(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Processor.vst3")
	if err := os.WriteFile(pluginPath, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	semanticsPath := filepath.Join(root, "plugin_semantics.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	t.Setenv("USERPROFILE", root)
	t.Setenv("HOME", root)
	entry := pluginsemantics.Entry{ID: "compressor-id", Name: "Compressor", Format: "VST3", Identifier: "compressor-id", PluginPath: pluginPath, UpdatedAt: time.Now().UTC()}
	if _, err := pluginsemantics.Save(semanticsPath, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"identifier": entry.Identifier, "family": processorattestation.FamilyBroadbandCompressor, "confirmed": true, "consent": processorCertificationConsent})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/processor-certification/start", bytes.NewReader(payload))
	request.Host = "127.0.0.1:7878"
	request.Header.Set("Content-Type", "application/json")
	server := New(nil, nil, nil)
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), "broadband_compressor") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func containsText(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
