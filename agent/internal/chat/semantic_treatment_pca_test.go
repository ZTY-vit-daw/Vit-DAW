package chat

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/shadow"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestLoadedSurfacePCAQualificationRequiresExactCurrentBinaryAndCoverage(t *testing.T) {
	path := t.TempDir() + "\\Limiter.vst3"
	if err := os.WriteFile(path, []byte("limiter-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\attestations.v2.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Limiter", Manufacturer: "Test", Format: "VST3", Identifier: "limiter-v1", InstalledPath: path}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyLimiter,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}},
	}, "test_promotion")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTreatmentPCAInput{Family: processorintent.FamilyLimiter, RequiredCoverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}}}
	digest := plugingrabber.ParameterDigest{PluginName: "Limiter", PluginIdentifier: subject.Identifier, PluginPath: path, PluginFormat: "VST3", PluginManufacturer: "Test"}
	surfaces := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: processorintent.FamilyLimiter, NextPlanner: "semantic_limiter"}}, digest, processorattestation.Subject{}, input)
	if len(surfaces) != 1 || !surfaces[0].PCAEligible || surfaces[0].PCAAttestationID != attestation.AttestationID {
		t.Fatalf("eligible surface=%+v", surfaces)
	}
	if err := os.WriteFile(path, []byte("limiter-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: processorintent.FamilyLimiter, NextPlanner: "semantic_limiter"}}, digest, processorattestation.Subject{}, input)
	if stale[0].PCAEligible || stale[0].PCAReason != "binary_fingerprint_changed" {
		t.Fatalf("fingerprint change was not fail-closed: %+v", stale[0])
	}
}

func TestLoadedSurfaceWithoutIdentityIsInspectOnly(t *testing.T) {
	input := semanticTreatmentPCAInput{Family: processorintent.FamilyDeEsser, RequiredCoverage: []processorattestation.Coverage{{Action: "adjust", Axis: "sibilance_reduction"}}}
	surfaces := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: processorintent.FamilyDeEsser, NextPlanner: "semantic_de_esser"}}, plugingrabber.ParameterDigest{PluginName: "De-esser"}, processorattestation.Subject{}, input)
	if len(surfaces) != 1 || surfaces[0].PCAEligible || !surfaces[0].InspectOnly || surfaces[0].PCAReason != "pca_exact_identity_unresolved" {
		t.Fatalf("missing identity did not become inspect-only: %+v", surfaces)
	}
}

func TestLoadedSurfacePCARejectsStaleRevokedAndInsufficientCoverage(t *testing.T) {
	path := t.TempDir() + "\\Gate.vst3"
	if err := os.WriteFile(path, []byte("gate-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\attestations.v2.json")
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Gate", Manufacturer: "Test", Format: "VST3", Identifier: "gate-v1", InstalledPath: path}
	issued, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyGateExpander,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_threshold"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}},
	}, "test_promotion")
	if err != nil {
		t.Fatal(err)
	}
	digest := plugingrabber.ParameterDigest{PluginName: subject.Name, PluginIdentifier: subject.Identifier, PluginPath: path, PluginFormat: subject.Format, PluginManufacturer: subject.Manufacturer}
	input := semanticTreatmentPCAInput{Family: processorintent.FamilyGateExpander, RequiredCoverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_threshold"}}}
	if _, err := store.MarkStale(issued.AttestationID, "test_stale"); err != nil {
		t.Fatal(err)
	}
	stale := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: input.Family}}, digest, processorattestation.Subject{}, input)
	if stale[0].PCAEligible || stale[0].PCAStatus != processorattestation.StatusStale {
		t.Fatalf("stale attestation remained executable: %+v", stale[0])
	}
	revokedSubject := subject
	revokedSubject.Identifier = "gate-revoked"
	issued, err = store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: revokedSubject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyGateExpander,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_threshold"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt-2", Kind: "test", SHA256: "sha256:" + strings.Repeat("c", 64), ObservedAt: time.Now().UTC()}},
	}, "test_repromotion")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Revoke(issued.AttestationID, "test_revoked"); err != nil {
		t.Fatal(err)
	}
	revokedDigest := digest
	revokedDigest.PluginIdentifier = revokedSubject.Identifier
	revoked := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: input.Family}}, revokedDigest, processorattestation.Subject{}, input)
	if revoked[0].PCAEligible || revoked[0].PCAStatus != processorattestation.StatusRevoked {
		t.Fatalf("revoked attestation remained executable: %+v", revoked[0])
	}
	insufficient := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: input.Family}}, digest, processorattestation.Subject{}, semanticTreatmentPCAInput{
		Family: input.Family, RequiredCoverage: []processorattestation.Coverage{{Action: "adjust", Axis: "attenuation_floor"}},
	})
	if insufficient[0].PCAEligible || insufficient[0].PCAReason == "" {
		t.Fatalf("insufficient coverage was not rejected: %+v", insufficient[0])
	}
}

func TestLoadedInstancePCAAdmissionReReadsExactIdentityAndCurrentSurface(t *testing.T) {
	path := t.TempDir() + "\\Compressor.vst3"
	if err := os.WriteFile(path, []byte("compressor-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", t.TempDir()+"\\attestations.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Pro-C 2", Manufacturer: "Test", Format: "VST3", Identifier: "compressor-v1", InstalledPath: path}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("d", 64), ObservedAt: time.Now().UTC()}},
	}, "test_promotion"); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "track_type": "audio", "is_audio_track": true, "plugins": []any{map[string]any{
			"plugin_id": "comp-1", "plugin_name": subject.Name, "plugin_identifier": subject.Identifier,
			"plugin_path": path, "format": subject.Format, "manufacturer": subject.Manufacturer,
		}},
	}}})
	server := New(nil, project, nil)
	server.eqKernelOverride = newFakeCompressorKernel()
	input := semanticTreatmentPCAInput{Family: processorintent.FamilyBroadbandCompressor,
		RequiredCoverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}}}
	surface, err := server.semanticLoadedInstancePCAAdmission(context.Background(), "track-1", "comp-1", input)
	if err != nil || !surface.PCAEligible || surface.PCABinaryFingerprint != fingerprint {
		t.Fatalf("loaded-instance admission=%+v err=%v", surface, err)
	}
	if err := os.WriteFile(path, []byte("compressor-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := server.semanticLoadedInstancePCAAdmission(context.Background(), "track-1", "comp-1", input); err == nil || !strings.Contains(err.Error(), "pca_rejected") {
		t.Fatalf("loaded-instance admission accepted changed fingerprint: %v", err)
	}
}

func TestLoadedInstancePCAAdmissionReceiptBridgesMissingLiveIdentifier(t *testing.T) {
	path := t.TempDir() + "\\ReceiptEQ.vst3"
	if err := os.WriteFile(path, []byte("receipt-eq-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", t.TempDir()+"\\attestations.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Receipt EQ", Manufacturer: "Test", Format: "VST3", Identifier: "receipt-eq-v1", InstalledPath: path}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt-eq", Kind: "test", SHA256: "sha256:" + strings.Repeat("9", 64), ObservedAt: time.Now().UTC()}},
	}, "receipt_eq_test")
	if err != nil {
		t.Fatal(err)
	}
	subjectKey, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}

	// The live rack node and parameter read deliberately contain only the
	// numeric instance ID.  PCA subject identity must come from the receipt,
	// while the live data still proves the current EQ topology.
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "track_type": "audio", "is_audio_track": true,
		"plugins": []any{map[string]any{"plugin_id": "1041", "plugin_name": subject.Name}},
	}}})
	server := New(nil, project, nil)
	server.eqKernelOverride = newFakeEQKernel()
	receipt := semanticPCAAdmissionReceipt{
		ProcessorFamily: processorattestation.FamilyStaticEQ, Name: subject.Name, Manufacturer: subject.Manufacturer,
		Format: subject.Format, Identifier: subject.Identifier, PluginPath: subject.InstalledPath,
		SubjectKey: subjectKey, BinaryFingerprint: fingerprint, AttestationID: attestation.AttestationID,
	}
	input := semanticTreatmentPCAInput{Family: processorintent.FamilyStaticEQ, RequiredCoverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}}}
	surface, err := server.semanticLoadedInstancePCAAdmissionWithReceipt(context.Background(), "track-1", "1041", input, &receipt)
	if err != nil || !surface.PCAEligible || surface.PCAAttestationID != attestation.AttestationID {
		t.Fatalf("receipt-backed admission=%+v err=%v", surface, err)
	}
	if err := os.WriteFile(path, []byte("receipt-eq-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := server.semanticLoadedInstancePCAAdmissionWithReceipt(context.Background(), "track-1", "1041", input, &receipt); err == nil || !strings.Contains(err.Error(), "binary fingerprint changed") {
		t.Fatalf("changed receipt binary remained executable: %v", err)
	}
}

func TestPCAReceiptOnlyResolvesOwnershipConflictAgainstUnadmittedSurface(t *testing.T) {
	path := t.TempDir() + "\\SharedSurface.vst3"
	if err := os.WriteFile(path, []byte("shared-surface-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", t.TempDir()+"\\attestations.v1.json")
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\attestations.v2.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Shared Surface", Manufacturer: "Test", Format: "VST3", Identifier: "shared-surface-v1", InstalledPath: path}
	v2, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	dynamic, err := v2.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyMultiband,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "band_dynamics"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "dynamic-receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("8", 64), ObservedAt: time.Now().UTC()}},
	}, "dynamic_promotion")
	if err != nil {
		t.Fatal(err)
	}
	subjectKey, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	receipt := semanticPCAAdmissionReceipt{ProcessorFamily: processorattestation.FamilyMultiband, Name: subject.Name, Manufacturer: subject.Manufacturer, Format: subject.Format, Identifier: subject.Identifier, PluginPath: path, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, AttestationID: dynamic.AttestationID}
	surfaces := []semanticProcessorSurface{
		{Family: processorintent.FamilyMultibandDynamics, QualificationStatus: "ownership_conflict", OwnedParameterIDs: []string{"shared"}},
		{Family: processorintent.FamilyStaticEQ, QualificationStatus: "ownership_conflict", OwnedParameterIDs: []string{"shared"}},
	}
	resolved, err := semanticPCAReceiptResolvesOwnershipConflict(surfaces, processorintent.FamilyMultibandDynamics, receipt)
	if err != nil || !resolved {
		t.Fatalf("unadmitted competing surface blocked receipt-backed target: resolved=%v err=%v", resolved, err)
	}

	v1, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v1.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "eq-receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("7", 64), ObservedAt: time.Now().UTC()}},
	}, "eq_promotion"); err != nil {
		t.Fatal(err)
	}
	resolved, err = semanticPCAReceiptResolvesOwnershipConflict(surfaces, processorintent.FamilyMultibandDynamics, receipt)
	if err != nil || resolved {
		t.Fatalf("PCA-admitted competing surface did not remain fail-closed: resolved=%v err=%v", resolved, err)
	}
}

func TestDynamicFamilyPCAQualificationMatrixCoversAllFiveFamilies(t *testing.T) {
	attestationPath := t.TempDir() + "\\attestations.v2.json"
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", attestationPath)
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		family string
		axis   string
		pca    string
	}{
		{family: processorintent.FamilyLimiter, axis: "ceiling", pca: processorattestation.FamilyLimiter},
		{family: processorintent.FamilyGateExpander, axis: "threshold", pca: processorattestation.FamilyGateExpander},
		{family: processorintent.FamilyDeEsser, axis: "range", pca: processorattestation.FamilyDeEsser},
		{family: processorintent.FamilyTransientShaper, axis: "attack", pca: processorattestation.FamilyTransient},
		{family: processorintent.FamilyMultibandDynamics, axis: "threshold", pca: processorattestation.FamilyMultiband},
	}
	for _, test := range cases {
		t.Run(test.family, func(t *testing.T) {
			path := t.TempDir() + "\\processor.vst3"
			if err := os.WriteFile(path, []byte(test.family+"-binary-v1"), 0o600); err != nil {
				t.Fatal(err)
			}
			proof, err := registry.PCARequiredCoverage(test.family, []string{test.axis})
			if err != nil || len(proof) == 0 {
				t.Fatalf("coverage proof unavailable: %v %+v", err, proof)
			}
			fingerprint, err := processorattestation.FingerprintPath(path)
			if err != nil {
				t.Fatal(err)
			}
			subject := processorattestation.Subject{Name: test.family, Manufacturer: "Test", Format: "VST3", Identifier: test.family + "-v1", InstalledPath: path}
			if _, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
				Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: test.pca, Coverage: proof,
				Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt-" + test.family, Kind: "semantic-dynamic-test", SHA256: "sha256:" + strings.Repeat("e", 64), ObservedAt: time.Now().UTC()}},
			}, "dynamic_family_matrix"); err != nil {
				t.Fatal(err)
			}
			surfaces := qualifySemanticTreatmentSurfaces([]semanticProcessorSurface{{Family: test.family, NextPlanner: semanticDynamicPlannerForFamily(test.family)}}, plugingrabber.ParameterDigest{
				PluginName: test.family, PluginIdentifier: subject.Identifier, PluginPath: path, PluginFormat: subject.Format, PluginManufacturer: subject.Manufacturer,
			}, processorattestation.Subject{}, semanticTreatmentPCAInput{Family: test.family, RequiredCoverage: proof})
			if len(surfaces) != 1 || !surfaces[0].PCAEligible || surfaces[0].PCAStatus != processorattestation.StatusPromoted {
				t.Fatalf("family %s did not qualify: %+v", test.family, surfaces)
			}
		})
	}
}

func semanticDynamicPlannerForFamily(family string) string {
	spec, ok := semanticDynamicSpecForFamily(family)
	if !ok {
		return ""
	}
	return spec.Planner
}

func TestDynamicFamilyPostLoadQualificationMatrixRechecksPCAAndTopology(t *testing.T) {
	attestationPath := t.TempDir() + "\\post_load_attestations.v2.json"
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", attestationPath)
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		family   string
		pca      string
		axis     string
		trackID  string
		pluginID string
		fetch    func() map[string]any
	}{
		{family: processorintent.FamilyLimiter, pca: processorattestation.FamilyLimiter, axis: "ceiling", trackID: "track-1", pluginID: "limit-1", fetch: func() map[string]any {
			fake := newFakeLimiterKernel()
			reply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
			if err != nil {
				panic(err)
			}
			return reply
		}},
		{family: processorintent.FamilyGateExpander, pca: processorattestation.FamilyGateExpander, axis: "threshold", trackID: "track-1", pluginID: "gate-1", fetch: func() map[string]any {
			fake := newFakeGateExpanderKernel()
			reply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
			if err != nil {
				panic(err)
			}
			return reply
		}},
		{family: processorintent.FamilyDeEsser, pca: processorattestation.FamilyDeEsser, axis: "range", trackID: "track-1", pluginID: "deesser-1", fetch: func() map[string]any {
			fake := newFakeDeEsserKernel()
			reply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
			if err != nil {
				panic(err)
			}
			return reply
		}},
		{family: processorintent.FamilyTransientShaper, pca: processorattestation.FamilyTransient, axis: "attack", trackID: "track-1", pluginID: "transient-1", fetch: func() map[string]any {
			fake := newFakeTransientShaperKernel()
			reply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
			if err != nil {
				panic(err)
			}
			return reply
		}},
		{family: processorintent.FamilyMultibandDynamics, pca: processorattestation.FamilyMultiband, axis: "threshold", trackID: "track-mb", pluginID: "mb-1", fetch: func() map[string]any {
			fake := newFakeMultibandKernel()
			reply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
			if err != nil {
				panic(err)
			}
			return reply
		}},
	}
	for _, test := range cases {
		t.Run(test.family, func(t *testing.T) {
			reply := test.fetch()
			if test.family == processorintent.FamilyGateExpander {
				filtered := make([]map[string]any, 0)
				for _, row := range mapRowsValue(reply["parameters"]) {
					if id := firstNonEmptyText(row, "id"); id == "hpf" || id == "lpf" {
						continue
					}
					filtered = append(filtered, row)
				}
				reply["parameters"] = filtered
			}
			digest := plugingrabber.BuildParameterDigest(reply)
			path := t.TempDir() + "\\loaded.vst3"
			if err := os.WriteFile(path, []byte(test.family+"-post-load-binary"), 0o600); err != nil {
				t.Fatal(err)
			}
			fingerprint, err := processorattestation.FingerprintPath(path)
			if err != nil {
				t.Fatal(err)
			}
			subject := processorattestation.Subject{Name: test.family, Manufacturer: "Test", Format: "VST3", Identifier: test.family + "-post-load", InstalledPath: path}
			proof, err := registry.PCARequiredCoverage(test.family, []string{test.axis})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
				Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: test.pca, Coverage: proof,
				Evidence: []processorattestation.EvidenceRef{{ReceiptID: "post-load-" + test.family, Kind: "post-load-test", SHA256: "sha256:" + strings.Repeat("f", 64), ObservedAt: time.Now().UTC()}},
			}, "post_load_matrix"); err != nil {
				t.Fatal(err)
			}
			digest.TrackID, digest.PluginID = test.trackID, test.pluginID
			digest.PluginName, digest.PluginIdentifier = subject.Name, subject.Identifier
			digest.PluginManufacturer, digest.PluginFormat, digest.PluginPath = subject.Manufacturer, subject.Format, subject.InstalledPath
			plan := PendingPlan{Context: map[string]any{"free_state_semantic_processor_intent": map[string]any{
				"schema_version": processorintent.SchemaVersion, "status": processorintent.StatusResolved, "family": test.family,
				"intent": "apply the selected dynamic treatment", "required_coverage": []string{test.axis},
				"scope": processorintent.ScopeCurrentTrack, "control_mode": processorintent.ControlModeSemantic, "confidence": 0.9,
			}}}
			surface, err := semanticPostLoadPCAQualification(plan, digest, test.family, test.trackID, test.pluginID, subject.Name)
			if err != nil || !surface.PCAEligible || surface.NextPlanner != semanticDynamicPlannerForFamily(test.family) {
				t.Fatalf("post-load qualification failed: surface=%+v err=%v", surface, err)
			}
		})
	}
}

func TestPostLoadPCAUsesAdmissionReceiptInsteadOfRackInstanceID(t *testing.T) {
	path := t.TempDir() + "\\Limiter.vst3"
	if err := os.WriteFile(path, []byte("receipt-limiter-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\attestations.v2.json")
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Limiter", Manufacturer: "Test", Format: "VST3", Identifier: "limiter-v1", InstalledPath: path}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyLimiter,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "receipt-limiter", Kind: "test", SHA256: "sha256:" + strings.Repeat("1", 64), ObservedAt: time.Now().UTC()}},
	}, "receipt_test")
	if err != nil {
		t.Fatal(err)
	}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	plan := PendingPlan{Context: map[string]any{"free_state_semantic_processor_intent": map[string]any{
		"family": processorintent.FamilyLimiter,
	}}, WorkflowData: map[string]any{"pca_admission_receipt": map[string]any{
		"processor_family": processorintent.FamilyLimiter, "name": subject.Name, "manufacturer": subject.Manufacturer,
		"format": subject.Format, "identifier": subject.Identifier, "plugin_path": subject.InstalledPath,
		"subject_key": key, "binary_fingerprint": fingerprint, "attestation_id": attestation.AttestationID,
	}}}
	reply, _, err := newFakeLimiterKernel().SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	digest := plugingrabber.BuildParameterDigest(reply)
	digest.TrackID, digest.PluginID = "track-1", "1044"
	digest.PluginIdentifier, digest.PluginPath = "1044", ""
	surface, err := semanticPostLoadPCAQualification(plan, digest, processorintent.FamilyLimiter, "track-1", "1044", subject.Name)
	if err != nil || !surface.PCAEligible || surface.PCAAttestationID != attestation.AttestationID {
		t.Fatalf("receipt-backed post-load qualification=%+v err=%v", surface, err)
	}
	if _, _, err := semanticValidatePCAAdmissionReceipt(plan, processorintent.FamilyLimiter, "1044"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("numeric rack instance ID was accepted as PCA identity: %v", err)
	}
}

func TestLoadedInstancePCAAdmissionForContextRejectsMalformedReceiptBeforeTopology(t *testing.T) {
	server := &Server{}
	_, err := server.semanticLoadedInstancePCAAdmissionForContext(context.Background(), map[string]any{
		"pca_admission_receipt": map[string]any{
			"processor_family": processorintent.FamilyLimiter,
			"name":             "Limiter",
		},
	}, "track-1", "rack-node-1", semanticTreatmentPCAInput{Family: processorintent.FamilyLimiter})
	if err == nil || !strings.Contains(err.Error(), "pca_rejected:pca admission receipt is incomplete") {
		t.Fatalf("malformed context receipt did not fail closed before topology lookup: %v", err)
	}
}

// The recommendation→grabber handoff must keep the no-receipt branch exactly
// as it was: a selected candidate without an accompanied receipt is still a
// fail-closed pca_rejected, byte for byte, and never falls back to the rack
// instance ID as a PCA identity.
func TestLoadedInstancePCAAdmissionForContextWithoutReceiptStillFailsClosed(t *testing.T) {
	server := &Server{}
	_, err := server.semanticLoadedInstancePCAAdmissionForContext(context.Background(), map[string]any{
		"semantic_plugin_recommendation_candidate": map[string]any{
			"processor_family": processorintent.FamilyDeEsser,
			"name":             "Pro-DS",
			"format":           "VST3",
			"identifier":       "pro-ds-v1",
			"plugin_path":      t.TempDir() + "\\Pro-DS.vst3",
		},
	}, "track-1", "rack-node-9", semanticTreatmentPCAInput{Family: processorintent.FamilyDeEsser})
	if err == nil || err.Error() != "pca_rejected:pca admission receipt is missing for the selected plugin" {
		t.Fatalf("no-receipt branch changed its fail-closed behavior: %v", err)
	}
}

// The exact request-context shape the fixed recommendation→grabber handoff
// produces (selected candidate + accompanied admission receipt) must pass the
// loaded-instance boundary, and the accompanied receipt must still be
// revalidated against the installed binary.
func TestLoadedInstancePCAAdmissionForContextPassesWithAccompaniedDeEsserReceipt(t *testing.T) {
	path := t.TempDir() + "\\ProDS.vst3"
	if err := os.WriteFile(path, []byte("deesser-receipt-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", t.TempDir()+"\\attestations.v2.json")
	registry, err := processorregistry.Default()
	if err != nil {
		t.Fatal(err)
	}
	proof, err := registry.PCARequiredCoverage(processorattestation.FamilyDeEsser, []string{"range"})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "Pro-DS", Manufacturer: "Test", Format: "VST3", Identifier: "pro-ds-v1", InstalledPath: path}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyDeEsser,
		Coverage: proof,
		Evidence: []processorattestation.EvidenceRef{{ReceiptID: "deesser-handoff-receipt", Kind: "test", SHA256: "sha256:" + strings.Repeat("e", 64), ObservedAt: time.Now().UTC()}},
	}, "handoff_receipt_test")
	if err != nil {
		t.Fatal(err)
	}
	subjectKey, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "track_type": "audio", "is_audio_track": true,
		"plugins": []any{map[string]any{"plugin_id": "deesser-1", "plugin_name": subject.Name}},
	}}})
	server := New(nil, project, nil)
	server.eqKernelOverride = newFakeDeEsserKernel()
	receipt := semanticPCAAdmissionReceipt{
		ProcessorFamily: processorattestation.FamilyDeEsser, Name: subject.Name, Manufacturer: subject.Manufacturer,
		Format: subject.Format, Identifier: subject.Identifier, PluginPath: subject.InstalledPath,
		SubjectKey: subjectKey, BinaryFingerprint: fingerprint, AttestationID: attestation.AttestationID,
	}
	requestContext := map[string]any{
		"semantic_plugin_recommendation_candidate": map[string]any{
			"processor_family": receipt.ProcessorFamily, "name": receipt.Name, "format": receipt.Format,
			"identifier": receipt.Identifier, "plugin_path": receipt.PluginPath,
		},
		"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt),
	}
	input := semanticTreatmentPCAInput{Family: processorattestation.FamilyDeEsser, RequiredCoverage: proof}
	surface, err := server.semanticLoadedInstancePCAAdmissionForContext(context.Background(), requestContext, "track-1", "deesser-1", input)
	if err != nil || !surface.PCAEligible || surface.InspectOnly || surface.PCAStatus != "promoted" ||
		surface.PCAReason != "admission_receipt_current" || surface.PCAAttestationID != attestation.AttestationID {
		t.Fatalf("accompanied receipt admission=%+v err=%v", surface, err)
	}
	if err := os.WriteFile(path, []byte("deesser-receipt-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := server.semanticLoadedInstancePCAAdmissionForContext(context.Background(), requestContext, "track-1", "deesser-1", input); err == nil || !strings.Contains(err.Error(), "binary fingerprint changed") {
		t.Fatalf("changed accompanied receipt binary remained executable: %v", err)
	}
}

// The load-result boundary must keep rejecting a receipt whose plugin does not
// match the actually loaded instance, so a mismatched receipt can never reach
// the downstream semantic admission through the relayed context.
func TestPostLoadHandoffStillRejectsReceiptForADifferentLoadedPlugin(t *testing.T) {
	server := &Server{}
	plan := PendingPlan{Context: map[string]any{}, WorkflowData: map[string]any{"pca_admission_receipt": map[string]any{
		"processor_family": processorattestation.FamilyDeEsser, "name": "Selected DS", "manufacturer": "Test",
		"format": "VST3", "identifier": "selected-ds-v1", "plugin_path": "C:\\plugins\\SelectedDS.vst3",
		"subject_key": "selected-ds-subject", "binary_fingerprint": "sha256:" + strings.Repeat("a", 64), "attestation_id": "att-selected-ds",
	}}}
	replies := []map[string]any{{"command_name": "rack_add_node", "result": map[string]any{
		"track_id": "track-1", "plugin_id": "1042", "plugin_name": "Other Plugin", "plugin_identifier": "other-b-v1",
	}}}
	handoff, ok := server.semanticProcessorPostLoadHandoff(context.Background(), plan, replies)
	if !ok || handoff.StopReason != "semantic_post_load_identity_mismatch" ||
		!strings.Contains(firstStringFromMap(handoff.WorkflowData, "pca_rejection"), "does not match") ||
		boolValue(handoff.WorkflowData["mutation_performed"]) {
		t.Fatalf("receipt for a different loaded plugin was not fail-closed: ok=%v handoff=%+v", ok, handoff)
	}
}

// The post-load handoff relays only a complete validated plan receipt into the
// downstream request context; plans without a receipt relay nothing, and a
// malformed plan receipt fails the handoff closed.
func TestSemanticRelayPCAAdmissionReceiptRelaysOnlyACompleteReceipt(t *testing.T) {
	receipt := semanticPCAAdmissionReceipt{
		ProcessorFamily: processorattestation.FamilyDeEsser, Name: "Pro-DS", Manufacturer: "Test",
		Format: "VST3", Identifier: "pro-ds-v1", PluginPath: "C:\\plugins\\ProDS.vst3",
		SubjectKey: "pro-ds-subject", BinaryFingerprint: "sha256:" + strings.Repeat("b", 64), AttestationID: "att-pro-ds",
	}
	requestContext := map[string]any{"selected_plugin_id": "deesser-1"}
	if err := semanticRelayPCAAdmissionReceiptToContext(PendingPlan{WorkflowData: map[string]any{"pca_admission_receipt": semanticPCAAdmissionReceiptMap(receipt)}}, requestContext); err != nil {
		t.Fatal(err)
	}
	relayed, found, err := semanticPCAAdmissionReceiptFromContext(requestContext)
	if !found || err != nil || relayed.Identifier != receipt.Identifier || relayed.AttestationID != receipt.AttestationID ||
		relayed.BinaryFingerprint != receipt.BinaryFingerprint || relayed.SubjectKey != receipt.SubjectKey {
		t.Fatalf("receipt relay was incomplete: found=%v receipt=%+v err=%v", found, relayed, err)
	}

	plain := map[string]any{"selected_plugin_id": "deesser-1"}
	if err := semanticRelayPCAAdmissionReceiptToContext(PendingPlan{WorkflowData: map[string]any{}}, plain); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := semanticPCAAdmissionReceiptFromContext(plain); found {
		t.Fatal("plan without a receipt must not grow one in the request context")
	}

	broken := map[string]any{"selected_plugin_id": "deesser-1"}
	if err := semanticRelayPCAAdmissionReceiptToContext(PendingPlan{WorkflowData: map[string]any{"pca_admission_receipt": map[string]any{"processor_family": "de_esser"}}}, broken); err == nil {
		t.Fatal("malformed plan receipt did not fail the relay closed")
	}
	if _, found, _ := semanticPCAAdmissionReceiptFromContext(broken); found {
		t.Fatal("malformed plan receipt must not be relayed")
	}
}
