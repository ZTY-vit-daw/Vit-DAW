package executionverifiers

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

type fakeSPALSignalProbe struct {
	result spal.SignalVerification
	err    error
}

func (f fakeSPALSignalProbe) VerifySPALSignal(context.Context, spal.ExecutionManifest, orchestration.ActionReceipt) (spal.SignalVerification, error) {
	return f.result, f.err
}

func TestSPALVerifierSeparatesStructuralSignalAndMusicalStates(t *testing.T) {
	manifest, action := verifierSPALManifestAction(t)
	result, err := (SPAL{Signal: fakeSPALSignalProbe{result: spal.SignalVerification{Status: "pass", Summary: "band energy decreased", EvidenceRefs: []string{"l2:before", "l2:after"}}}}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", EvidenceRefs: []string{"plugin.readback:ok"},
		Details: map[string]any{"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID},
	}})
	if err != nil || result.Status != "inconclusive" || result.Structural != "pass" || result.Acoustic != "pass" || result.UserAcceptance != "unknown" {
		t.Fatalf("unexpected SPAL verification result: %#v err=%v", result, err)
	}
}

func TestSPALVerifierKeepsSignalMismatchAsReviewNotMutationFailure(t *testing.T) {
	manifest, action := verifierSPALManifestAction(t)
	result, err := (SPAL{Signal: fakeSPALSignalProbe{result: spal.SignalVerification{Status: "mismatch", Summary: "target band did not decrease"}}}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", Details: map[string]any{"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID},
	}})
	if err != nil || result.Status != "inconclusive" || result.Structural != "pass" || result.Acoustic != "mismatch" || result.UserAcceptance != "unknown" {
		t.Fatalf("signal mismatch must remain a review state, got %#v err=%v", result, err)
	}
}

func TestSPALVerifierRequiresControlledRoundtripRestoreWhenRequested(t *testing.T) {
	manifest, action := verifierSPALManifestAction(t)
	missing, err := (SPAL{RequireControlledRoundtrip: true}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", Details: map[string]any{
			"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID,
		},
	}})
	if err == nil || missing.Status != "fail" || missing.Structural != "fail" {
		t.Fatalf("controlled roundtrip verifier accepted a missing restore: %#v err=%v", missing, err)
	}
	restored, err := (SPAL{RequireControlledRoundtrip: true}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", Details: map[string]any{
			"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID,
			"controlled_roundtrip_restored_preimage": true, "controlled_roundtrip_restore_status": "restored",
		},
	}})
	if err != nil || restored.Structural != "pass" || restored.Status != "inconclusive" {
		t.Fatalf("controlled roundtrip verifier rejected a restored preimage: %#v err=%v", restored, err)
	}
}

func TestReceiptSPALSignalProbeReadsPersistedEvidence(t *testing.T) {
	manifest, action := verifierSPALManifestAction(t)
	evidence := spal.NewSignalProbeEvidence(spal.SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: "clip-1"})
	evidence.Before = &spal.RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "before", TrackID: "bass", EvidenceRef: "l2:before", Bands: []spal.BandEnergy{{MinHz: 80, MaxHz: 110, EnergyDB: -20}}}
	evidence.After = &spal.RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "after", TrackID: "bass", EvidenceRef: "l2:after", Bands: []spal.BandEnergy{{MinHz: 80, MaxHz: 110, EnergyDB: -23}}}
	evidence.RefreshEvidenceRefs()
	result, err := (SPAL{Signal: ReceiptSPALSignalProbe{}}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", EvidenceRefs: evidence.EvidenceRefs,
		Details: map[string]any{"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID, "spal_signal_evidence": evidence},
	}})
	if err != nil || result.Status != "inconclusive" || result.Structural != "pass" || result.Acoustic != "pass" || len(result.EvidenceRefs) != 2 {
		t.Fatalf("persisted signal evidence did not survive verification: %#v err=%v", result, err)
	}
}

func TestSPALVerifierKeepsMissingSignalEvidenceInconclusive(t *testing.T) {
	manifest, action := verifierSPALManifestAction(t)
	result, err := (SPAL{}).Verify(context.Background(), orchestration.ActionSet{Actions: []orchestration.Action{action}}, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", Details: map[string]any{"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID},
	}})
	if err != nil || result.Status != "inconclusive" || result.Structural != "pass" || result.Acoustic != "unsupported" || result.UserAcceptance != "unknown" {
		t.Fatalf("missing signal evidence should not become musical failure: %#v err=%v", result, err)
	}
}

func verifierSPALManifestAction(t *testing.T) (spal.ExecutionManifest, orchestration.Action) {
	t.Helper()
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	instruction := spal.Instruction{
		SchemaID: spal.StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2},
		ExpectedSignalChange: spal.SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"},
	}
	binding, err := adapter.Bind(spal.ProviderInstance{
		ID: "instance", ProviderID: spal.ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin", Status: spal.InstanceVerified,
		PluginSignature: spal.ExperimentalTDRNovaSignature,
		Metadata:        map[string]string{"band_slot": "band1", "static_bell_ready": "true"},
	}, instruction)
	if err != nil {
		t.Fatal(err)
	}
	preimage := []spal.PhysicalParameter{{ParameterID: "1", Value: 0}, {ParameterID: "6", Value: 1}, {ParameterID: "4", Value: 100}, {ParameterID: "2", Value: 0}, {ParameterID: "3", Value: 1}}
	manifest, err := spal.CompileManifest(spal.Resolution{Status: spal.ResolutionBound, Binding: &binding}, instruction, preimage, adapter)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "p", ProjectEpoch: "e", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	set, err := manifest.ToActionSet("static_mix.low_end_relation.v0", cut)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, set.Actions[0]
}
