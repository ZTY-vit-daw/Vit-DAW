package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
)

func TestPluginControlRequirementIsChosenWithoutCandidateDisclosure(t *testing.T) {
	response := `{"schema_version":"processor_control_requirement.v1","processor_family":"static_eq","coverage":[{"action":"upsert","shape":"low_cut"}],"reason":"remove rumble"}`
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{response})
	requirement, err := server.inferPluginControlRequirement(context.Background(), "conversation-1", "remove low rumble", "eq",
		map[string]any{"selected_track_id": "track-1"}, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 1 || len(requirement.Coverage) != 1 || requirement.Coverage[0].Shape != "low_cut" {
		t.Fatalf("requirement=%+v calls=%d", requirement, *calls)
	}
	if len(*bodies) != 1 || strings.Contains((*bodies)[0], "loadable_local_candidates") || strings.Contains((*bodies)[0], "plugin_candidate_") {
		t.Fatalf("control requirement prompt disclosed candidates: %s", (*bodies)[0])
	}
}

func TestAttestationFilterRequiresExactCurrentActionAndFingerprint(t *testing.T) {
	root := t.TempDir()
	bellPath := filepath.Join(root, "Bell EQ.vst3")
	cutPath := filepath.Join(root, "Cut EQ.vst3")
	if err := os.WriteFile(bellPath, []byte("bell-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cutPath, []byte("cut-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := processorattestation.NewStore(filepath.Join(root, "attestations.json"))
	bell := attestationTestCandidate("Bell EQ", "bell-eq", bellPath)
	cut := attestationTestCandidate("Cut EQ", "cut-eq", cutPath)
	promoteRecommendationFixture(t, store, bell, "bell")
	promoteRecommendationFixture(t, store, cut, "low_cut")
	library, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	requirement := pluginControlRequirement{SchemaVersion: pluginControlRequirementSchema,
		ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage:        []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}}, Reason: "current action"}
	filtered, err := filterAttestedPluginRecommendationCandidates(library, []pluginRecommendationCandidate{cut, bell}, requirement)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Identifier != bell.Identifier || filtered[0].Key != "plugin_candidate_1" {
		t.Fatalf("filtered=%+v", filtered)
	}
	encoded, _ := json.Marshal(filtered)
	if strings.Contains(string(encoded), "attestation_id") || strings.Contains(string(encoded), "coverage") {
		t.Fatalf("attestation details leaked to recommendation candidates: %s", encoded)
	}
	if err := os.WriteFile(bellPath, []byte("bell-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	filtered, err = filterAttestedPluginRecommendationCandidates(library, []pluginRecommendationCandidate{bell}, requirement)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("changed binary remained eligible: %+v", filtered)
	}
}

func TestSelectionAttestationRecheckUsesGlobalStore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Compressor.vst3")
	if err := os.WriteFile(path, []byte("compressor-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(root, "attestations.json")
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", storePath)
	store, _ := processorattestation.NewStore(storePath)
	candidate := attestationTestCandidate("Compressor", "compressor", path)
	fingerprint, _ := processorattestation.FingerprintPath(path)
	subject := processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format,
		Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath}
	_, err := store.PromoteCurrent(processorattestation.IssueSpec{Subject: subject, BinaryFingerprint: fingerprint,
		ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage:        []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}},
		Evidence:        []processorattestation.EvidenceRef{attestationTestEvidence()}}, "strong_receipt")
	if err != nil {
		t.Fatal(err)
	}
	requirement := pluginControlRequirement{SchemaVersion: pluginControlRequirementSchema,
		ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage:        []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}}}
	selected := map[string]any{"name": candidate.Name, "manufacturer": candidate.Manufacturer, "format": candidate.Format,
		"identifier": candidate.Identifier, "plugin_path": candidate.PluginPath}
	if err := verifyPluginRecommendationAttestation(selected, requirement); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("compressor-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPluginRecommendationAttestation(selected, requirement); err == nil {
		t.Fatal("selection recheck accepted a changed binary")
	}
}

func attestationTestCandidate(name, identifier, path string) pluginRecommendationCandidate {
	return pluginRecommendationCandidate{Name: name, Manufacturer: "Vendor", Format: "VST3", Identifier: identifier,
		PluginPath: path, PrimaryType: "eq"}
}

func promoteRecommendationFixture(t *testing.T, store *processorattestation.Store, candidate pluginRecommendationCandidate, shape string) {
	t.Helper()
	fingerprint, err := processorattestation.FingerprintPath(candidate.PluginPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: processorattestation.Subject{Name: candidate.Name, Manufacturer: candidate.Manufacturer, Format: candidate.Format,
			Identifier: candidate.Identifier, InstalledPath: candidate.PluginPath},
		BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: shape}},
		Evidence: []processorattestation.EvidenceRef{attestationTestEvidence()},
	}, "strong_receipt")
	if err != nil {
		t.Fatal(err)
	}
}

func attestationTestEvidence() processorattestation.EvidenceRef {
	return processorattestation.EvidenceRef{ReceiptID: "receipt", Kind: "test_receipt",
		SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)}
}
