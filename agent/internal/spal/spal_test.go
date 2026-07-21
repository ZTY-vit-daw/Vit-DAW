package spal

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

type fakePreimageReader struct {
	values []PhysicalParameter
	err    error
}

func (f fakePreimageReader) CaptureSPALPreimage(_ context.Context, _ RuntimeBinding, writes []PhysicalParameter) ([]PhysicalParameter, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.values) != len(writes) {
		return nil, fmt.Errorf("test preimage does not cover writes")
	}
	return append([]PhysicalParameter(nil), f.values...), nil
}

func TestRegistryDoesNotFallbackToExperimentalTDRNova(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	instruction := testInstruction()
	resolution, err := registry.Resolve(instruction, nil, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != ResolutionNoVerifiedProvider || !resolution.RequiredPluginSkill || !strings.Contains(resolution.Reason, "Plugin Skill") {
		t.Fatalf("experimental provider became an implicit fallback: %#v", resolution)
	}
}

func TestTDRNovaBindingCompilesOnlyConformedStaticBell(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	instruction := testInstruction()
	instance := ProviderInstance{
		ID: "instance-1", ProviderID: ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin-1",
		PluginSignature: ExperimentalTDRNovaSignature, Status: InstanceVerified,
		Metadata: map[string]string{"band_slot": "band2", "static_bell_ready": "true"},
	}
	resolution, err := registry.Resolve(instruction, []ProviderInstance{instance}, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != ResolutionBound || resolution.Binding == nil {
		t.Fatalf("resolution = %#v", resolution)
	}
	preimage := []PhysicalParameter{
		{ParameterID: "13", Value: 0, Unit: "toggle"},
		{ParameterID: "18", Value: 0, Unit: "toggle"},
		{ParameterID: "16", Value: 0.25, Unit: "Hz"},
		{ParameterID: "14", Value: 0.5, Unit: "dB"},
		{ParameterID: "15", Value: 0.5, Unit: "Q"},
	}
	manifest, err := CompileManifest(resolution, instruction, preimage, adapter)
	if err != nil {
		t.Fatal(err)
	}
	expectedFrequency := math.Log(92.0/10.0) / math.Log(40000.0/10.0)
	expectedGain := (-2.5 + 18.0) / 36.0
	expectedQ := math.Log(1.2/tdrNovaQMin) / math.Log(tdrNovaQMax/tdrNovaQMin)
	if len(manifest.Writes) != 5 || manifest.Writes[2].ParameterID != "16" || math.Abs(manifest.Writes[2].Value-expectedFrequency) > 0.000001 || math.Abs(manifest.Writes[3].Value-expectedGain) > 0.000001 || math.Abs(manifest.Writes[4].Value-expectedQ) > 0.000001 {
		t.Fatalf("unexpected TDR Nova writes: %#v", manifest.Writes)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	proposal, actionSet, err := manifest.FreezeProposal("static_mix.low_end_relation.v0", "v0", cut, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.ActionSetHash != actionSet.Hash || actionSet.Actions[0].Command != ActionCommand || actionSet.Actions[0].BeforeFingerprint == "" {
		t.Fatalf("SPAL proposal did not freeze an exact action set: proposal=%#v set=%#v", proposal, actionSet)
	}
	decoded, err := ManifestFromAction(actionSet.Actions[0])
	if err != nil || decoded.ID != manifest.ID || decoded.Binding.Instance.PluginID != "plugin-1" {
		t.Fatalf("manifest was not retained in the frozen action: decoded=%#v err=%v", decoded, err)
	}
}

func TestRuntimeCapturesPreimageWithoutExposingRawParametersToCapability(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	instance := ProviderInstance{
		ID: "instance-1", ProviderID: ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin-1",
		PluginSignature: ExperimentalTDRNovaSignature, Status: InstanceVerified,
		Metadata: map[string]string{"band_slot": "band1", "static_bell_ready": "true"},
	}
	preimage := []PhysicalParameter{
		{ParameterID: "1", Value: 0}, {ParameterID: "6", Value: 1}, {ParameterID: "4", Value: 0.25}, {ParameterID: "2", Value: 0.5}, {ParameterID: "3", Value: 0.5},
	}
	prepared, err := (Runtime{Registry: registry}).Prepare(context.Background(), testInstruction(), []ProviderInstance{instance}, ResolveOptions{}, fakePreimageReader{values: preimage})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Resolution.Status != ResolutionBound || prepared.Manifest == nil || prepared.Manifest.Preimage[2].ParameterID != "4" {
		t.Fatalf("runtime did not create a frozen-ready manifest: %#v", prepared)
	}
}

func TestRollbackManifestReversesRequestedSignalDirection(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	instance := ProviderInstance{
		ID: "instance-rollback", ProviderID: ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin-1",
		PluginSignature: ExperimentalTDRNovaSignature, Status: InstanceVerified,
		Metadata: map[string]string{"band_slot": "band1", "static_bell_ready": "true"},
	}
	preimage := []PhysicalParameter{
		{ParameterID: "1", Value: 0}, {ParameterID: "6", Value: 1}, {ParameterID: "4", Value: 0.25}, {ParameterID: "2", Value: 0.5}, {ParameterID: "3", Value: 0.5},
	}
	prepared, err := (Runtime{Registry: registry}).Prepare(context.Background(), testInstruction(), []ProviderInstance{instance}, ResolveOptions{}, fakePreimageReader{values: preimage})
	if err != nil || prepared.Manifest == nil {
		t.Fatalf("prepare manifest: manifest=%#v err=%v", prepared.Manifest, err)
	}
	rollback, err := NewRollbackManifest(*prepared.Manifest, prepared.Manifest.Writes)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Operation != OperationRollback || rollback.Instruction.ExpectedSignalChange.Direction != "increase" {
		t.Fatalf("rollback kept original signal direction: %#v", rollback.Instruction.ExpectedSignalChange)
	}
}

func TestTDRNovaBindingRejectsUnknownBellShape(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Bind(ProviderInstance{
		ID: "instance-1", ProviderID: ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin-1", PluginSignature: ExperimentalTDRNovaSignature, Status: InstanceVerified,
		Metadata: map[string]string{"band_slot": "band1"},
	}, testInstruction())
	if err == nil || !strings.Contains(err.Error(), "static Bell") {
		t.Fatalf("unconformed static bell was accepted: %v", err)
	}
}

func TestTDRNovaConformanceRequiresVerifiedStaticBellAndMappedDomains(t *testing.T) {
	adapter, err := NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	profile := TDRNovaConformanceProfile{
		ProfileKey: "plugin_f9788adda4203df8", PluginName: "TDR Nova", PluginFormat: "VST3", PluginVersion: "2.2.2", ParameterSignature: ExperimentalTDRNovaSignature,
		Bands: map[string]TDRNovaBandConformance{"band1": {
			StaticBellReady: true,
			Parameters: map[string]TDRNovaParameterConformance{
				"enable": {ParameterID: "1", Unit: "toggle", Min: 0, Max: 1, Confirmed: true}, "dyn_enable": {ParameterID: "6", Unit: "toggle", Min: 0, Max: 1, Confirmed: true},
				"frequency": {ParameterID: "4", Unit: "Hz", Min: 10, Max: 40000, Confirmed: true}, "gain": {ParameterID: "2", Unit: "dB", Min: -18, Max: 18, Confirmed: true}, "q": {ParameterID: "3", Unit: "Q", Min: tdrNovaQMin, Max: tdrNovaQMax, Confirmed: true},
			},
		}},
	}
	report, err := adapter.Conform(profile, "band1")
	if err != nil || report.Status != ProviderVerified || len(report.EvidenceRefs) != 2 {
		t.Fatalf("TDR Nova conformance failed: report=%#v err=%v", report, err)
	}
	profile.Bands["band1"] = TDRNovaBandConformance{StaticBellReady: false, Parameters: profile.Bands["band1"].Parameters}
	if _, err := adapter.Conform(profile, "band1"); err == nil {
		t.Fatal("unverified Bell shape was conformed")
	}
}

func TestSignalVerificationRequiresSameTapAndChangedRevision(t *testing.T) {
	expectation := SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"}
	before := RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "before", EvidenceRef: "before", Bands: []BandEnergy{{MinHz: 20, MaxHz: 200, EnergyDB: -10}}}
	after := RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "after", EvidenceRef: "after", Bands: []BandEnergy{{MinHz: 20, MaxHz: 200, EnergyDB: -12}}}
	result := VerifySignalDirection(expectation, before, after)
	if result.Status != "pass" || len(result.EvidenceRefs) != 2 {
		t.Fatalf("expected directional signal pass, got %#v", result)
	}
	after.RenderRevision = "before"
	if result := VerifySignalDirection(expectation, before, after); result.Status != "inconclusive" {
		t.Fatalf("same render revision must be inconclusive: %#v", result)
	}
}

func TestSignalProbeScopeRequiresFrozenClipOrRange(t *testing.T) {
	scope := SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe"}
	if err := scope.Validate(); err == nil {
		t.Fatal("scope without a clip or explicit range was accepted")
	}
	start, end, tail := 0.0, 12.0, 0.25
	scope = SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", StartSeconds: &start, EndSeconds: &end, TailSeconds: &tail}
	if err := scope.Validate(); err != nil {
		t.Fatalf("bounded signal probe scope rejected: %v", err)
	}
	if err := (Instruction{SchemaID: StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2}, SignalProbeScope: scope}).Validate(); err == nil {
		t.Fatal("signal scope without an expected direction was accepted")
	}
}

func TestSignalProbeEvidenceUsesMostSpecificCoveringBand(t *testing.T) {
	expectation := SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"}
	evidence := NewSignalProbeEvidence(SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: "clip-1"})
	evidence.Before = &RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "before", TrackID: "bass", EvidenceRef: "l2:before", Bands: []BandEnergy{{MinHz: 60, MaxHz: 250, EnergyDB: -10}, {MinHz: 80, MaxHz: 110, EnergyDB: -20}}}
	evidence.After = &RenderProbe{TapPoint: "track_post_fader", RenderMode: "offline_probe", RenderRevision: "after", TrackID: "bass", EvidenceRef: "l2:after", Bands: []BandEnergy{{MinHz: 60, MaxHz: 250, EnergyDB: -10}, {MinHz: 80, MaxHz: 110, EnergyDB: -23}}}
	result := evidence.Verify(expectation)
	if result.Status != "pass" || result.BeforeDB != -20 || result.AfterDB != -23 || len(result.EvidenceRefs) != 2 {
		t.Fatalf("specific target band was not selected: %#v", result)
	}
	stored, err := SignalProbeEvidenceFromAny(map[string]any{
		"schema_version": SignalProbeEvidenceSchema,
		"scope":          evidence.Scope,
		"before":         evidence.Before,
		"after":          evidence.After,
	})
	if err != nil || stored.Verify(expectation).Status != "pass" {
		t.Fatalf("durable signal evidence failed to round-trip: %#v err=%v", stored, err)
	}
}

func testInstruction() Instruction {
	max := 3.0
	return Instruction{
		SchemaID:  StaticBellControlID,
		TargetRef: "track:bass",
		Parameters: map[string]float64{
			"center_frequency_hz": 92,
			"gain_db":             -2.5,
			"q":                   1.2,
		},
		SafetyBounds:         SafetyBounds{MaxAbsoluteGainDB: &max},
		EvidenceRefs:         []string{"mom.low_end_overlap:obs-1"},
		ExpectedSignalChange: SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"},
	}
}
