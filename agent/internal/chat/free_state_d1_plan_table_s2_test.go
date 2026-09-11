package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorattestation"
)

// ---- D2-1.5 broadband_compression fixtures ---------------------------------

type d1CompressionWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1CompressionWhitelistFixtureOnDisk materializes a whitelisted broadband
// compressor on disk inside t.TempDir (never the developer's ~/.vit) plus its
// PCA v1 library record that promotes exactly this binary fingerprint.
func d1CompressionWhitelistFixtureOnDisk(t *testing.T) d1CompressionWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Comp.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-compressor-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1CompressionWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			BroadbandCompression: &experimentplugins.BroadbandCompressionPlugin{
				PluginName:          "Fixture Comp",
				Manufacturer:        "Fixture",
				Format:              "VST3",
				PluginIdentifier:    "fixture-comp",
				PluginPath:          pluginPath,
				ThresholdParamIDCH1: "thr_a",
				ThresholdParamIDCH2: "thr_b",
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version":        fixture.Whitelist.SchemaVersion,
		"broadband_compression": fixture.Whitelist.BroadbandCompression,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1CompressionPromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.Library {
	t.Helper()
	now := time.Date(2026, 8, 27, 22, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestation(processorattestation.IssueSpec{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyBroadbandCompressor,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "transfer_severity"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-comp-table-1", Kind: "compressor_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("b", 64),
			ObservedAt: now.Add(-time.Hour),
		}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	promotedAt := now.Add(time.Hour)
	attestation.Status = processorattestation.StatusPromoted
	attestation.StatusReason = "test_evidence_passed"
	attestation.PromotedAt = &promotedAt
	return processorattestation.Library{
		SchemaVersion: processorattestation.LibrarySchema,
		UpdatedAt:     promotedAt,
		Attestations:  []processorattestation.Attestation{attestation},
	}
}

func compressionTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "the vocal bus sounds over-compressed", Hypothesis: "a bounded threshold move may restore dynamics",
		ExpectedEffect: "more dynamic movement without level shift", ActionDomain: d1BroadbandCompressionDomain,
		ActionKind: d1BroadbandCompressionKind, ParameterBounds: map[string]any{"threshold_db": 1.5},
		VerificationPlan: map[string]any{"view_ids": []any{"track.time_dynamics"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1CompressionLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-comp", ConversationID: "conversation-d1-comp", GoalID: "goal-d1-comp", RunID: "run-d1-comp", Status: "awaiting_experiment", OriginalIntent: "restore dynamics", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: compressionTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

// ---- admission assembly ------------------------------------------------------

func TestD1S1CompressionAdmissionBindsBoundedThreshold(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "7")
	admission := loop.Experiment.Admission
	if !admission.IsD1S1() {
		t.Fatalf("compression admission is not D1-S1: %+v", admission)
	}
	threshold, ok := treatmentNumber(admission.TypedAction, "threshold_db")
	if !ok || threshold != 1.5 {
		t.Fatalf("typed threshold_db=%v ok=%v typed=%+v", threshold, ok, admission.TypedAction)
	}
	if _, present := admission.TypedAction["plugin_identifier"]; present {
		t.Fatalf("unpinned proposal must not grow a plugin identifier: %+v", admission.TypedAction)
	}
	for name, bounds := range map[string]map[string]any{"diagnostic": admission.DiagnosticDoseBounds, "retained": admission.RetainedDoseBounds} {
		doseThreshold, ok := treatmentNumber(bounds, "threshold_db")
		if !ok || doseThreshold != 1.5 || bounds["max_action_attempts"] != 1 {
			t.Fatalf("%s dose bounds=%+v", name, bounds)
		}
	}
	if err := admission.ValidateD1S1(); err != nil {
		t.Fatalf("validated compression admission rejected: %v", err)
	}

	// Same-equally-tight bounds: an out-of-band threshold is refused by the row.
	oversized := compressionTestProposal()
	oversized.ParameterBounds["threshold_db"] = -3.0
	if _, err := freeStateExperimentAdmission(loop, oversized); err == nil || !strings.Contains(err.Error(), "+/-2") {
		t.Fatalf("out-of-band threshold admitted: err=%v", err)
	}

	// The static_eq admission shape stays byte-identical through the shared
	// table-driven assembler (regression against accidental key drift).
	eqLoop := d1StaticEQLoopForTest(t, "7")
	eqTyped := eqLoop.Experiment.Admission.TypedAction
	if _, ok := treatmentNumber(eqTyped, "gain_db"); !ok {
		t.Fatalf("static_eq typed action lost gain_db through the shared assembler: %+v", eqTyped)
	}
	if eqTyped["frequency_hz"] != 400.0 || eqTyped["plugin_identifier"] != nil {
		t.Fatalf("static_eq typed action drifted: %+v", eqTyped)
	}
}

// ---- execution gate ----------------------------------------------------------

func TestResolveD1PluginParamBindingForCompression(t *testing.T) {
	fixture := d1CompressionWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Comp", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-comp", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1BroadbandCompressionDomain, "action_kind": d1BroadbandCompressionKind, "threshold_db": 1.5}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1TableOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.Library{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1BroadbandCompressionDomain) {
		t.Fatalf("compression unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1TableOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.Library{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("compression invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation store is unreadable at x")
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.Library{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("compression unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.Library{SchemaVersion: processorattestation.LibrarySchema}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("compression ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1BroadbandCompressionDomain, "plugin_identifier": "some-other-comp"}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil,
		d1CompressionPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("compression pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1BroadbandCompressionDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "thr_a" || binding.ParamIDCH2 != "thr_b" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved compression binding=%+v", binding)
	}
}

// ---- table-driven plan ---------------------------------------------------------

func TestD1S1CompressionPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:    d1BroadbandCompressionDomain,
		PluginName: "Fixture Comp",
		PluginPath: "C:/plugins/Fixture Comp.vst3",
		ParamID:    "thr_a",
		ParamIDCH2: "thr_b",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1BroadbandCompressionKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_comp") || action.Command != d1BroadbandCompressionKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:comp:thr_a:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.broadband_compression.v0" || plan.Proposal.CapabilityID != "static_mix.broadband_compression.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture Comp.vst3" ||
		action.Args["param_id"] != "thr_a" || action.Args["param_id_ch2"] != "thr_b" ||
		action.Args["target_value"] != 1.5 {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("compression action must not carry an EQ frequency: %+v", action.Args)
	}
	if _, present := action.Args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:broadband_threshold_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
}

func TestD1S1GenericPluginPlanReproducesLegacyStaticEQPlans(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	legacy, err := d1StaticEQPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7", state, &d1StaticEQWhitelistBinding{
		Plugin:      experimentplugins.StaticEQPlugin{PluginName: "Fixture EQ", PluginPath: "C:/plugins/Fixture EQ.vst3"},
		Band:        experimentplugins.Band{CenterHz: 315, GainParamIDCH1: "p315_c1", GainParamIDCH2: "p315_c2"},
		FrequencyHz: 400,
	})
	if err != nil {
		t.Fatal(err)
	}
	generic, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7", state, &d1PluginParamWhitelistBinding{
		Section:     d1StaticEQDomain,
		PluginName:  "Fixture EQ",
		PluginPath:  "C:/plugins/Fixture EQ.vst3",
		ParamID:     "p315_c1",
		ParamIDCH2:  "p315_c2",
		FrequencyHz: 400,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if generic.ActionSet.Hash != legacy.ActionSet.Hash || generic.ProjectCut.Hash != legacy.ProjectCut.Hash {
		t.Fatalf("generic plan hashes diverge: generic=%s/%s legacy=%s/%s",
			generic.ActionSet.Hash, generic.ProjectCut.Hash, legacy.ActionSet.Hash, legacy.ProjectCut.Hash)
	}
	if !reflect.DeepEqual(generic.ActionSet.Actions, legacy.ActionSet.Actions) {
		t.Fatalf("generic actions=%+v legacy=%+v", generic.ActionSet.Actions, legacy.ActionSet.Actions)
	}
}

// The journal port driven by any resolved spec records the same audit shape
// family; the historical bool signature remains untouched.
func TestD1JournalPortRecordsCompressionSpecShape(t *testing.T) {
	inner := &d1MutationPortForTest{}
	log := &d1JournalForTest{}
	spec, ok := experiment.D1S1SpecForAction(d1BroadbandCompressionDomain, d1BroadbandCompressionKind)
	if !ok {
		t.Fatal("broadband_compression row missing")
	}
	port := &d1JournalMutationPort{inner: inner, harness: log, goalID: "goal", runID: "run", journalSpec: &spec}
	action := orchestration.Action{ID: "d1-comp-action", Command: d1BroadbandCompressionKind, TargetRef: "vocal",
		Args: map[string]any{"param_id": "thr_a", "target_value": 1.5}}
	if _, err := port.Apply(context.Background(), action, "execution:"+action.ID); err != nil {
		t.Fatal(err)
	}
	if len(log.recorded) != 1 {
		t.Fatalf("journal records=%+v", log.recorded)
	}
	record := log.recorded[0]
	if record.Summary != "D2-1.5 bounded broadband compression threshold adjustment" || record.Tool != "set_plugin_param" ||
		record.Command["cmd"] != "set_plugin_param" || record.Command["param_id"] != "thr_a" ||
		record.Command["value"] != 1.5 || record.Command["track_id"] != "vocal" {
		t.Fatalf("spec journal record=%+v", record)
	}
}

// ---- FAM1-S1 de_esser fixtures ----------------------------------------------

type d1DeEsserWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1DeEsserWhitelistFixtureOnDisk materializes a whitelisted de-esser on disk
// inside t.TempDir (never the developer's ~/.vit): one shared threshold
// parameter (Pro-DS probe 2026-08-31), admitted by a PCA v2 library record
// that promotes exactly this binary fingerprint.
func d1DeEsserWhitelistFixtureOnDisk(t *testing.T) d1DeEsserWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture DeEss.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-deesser-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1DeEsserWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			DeEsser: &experimentplugins.DeEsserPlugin{
				PluginName:       "Fixture DeEss",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-deess",
				PluginPath:       pluginPath,
				ThresholdParamID: "deess_thresh",
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version": fixture.Whitelist.SchemaVersion,
		"de_esser":       fixture.Whitelist.DeEsser,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1DeEsserPromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.LibraryV2 {
	t.Helper()
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyDeEsser,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "sibilance_reduction"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-deess-table-1", Kind: "de_esser_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("a", 64),
			ObservedAt: now.Add(-time.Hour),
		}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	promotedAt := now.Add(time.Hour)
	attestation.Status = processorattestation.StatusPromoted
	attestation.StatusReason = "test_evidence_passed"
	attestation.PromotedAt = &promotedAt
	return processorattestation.LibraryV2{
		SchemaVersion: processorattestation.LibrarySchemaV2,
		UpdatedAt:     promotedAt,
		Attestations:  []processorattestation.AttestationV2{attestation},
	}
}

// d1DeEsserOverrideLoaders swaps the whitelist loader and the PCA v2
// attestation reader for the de_esser gate; the v1 reader stays untouched.
func d1DeEsserOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.LibraryV2, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1DeEsserAttestationReader = func() (processorattestation.LibraryV2, error) {
		if readerErr != nil {
			return processorattestation.LibraryV2{}, readerErr
		}
		return library, nil
	}
	t.Cleanup(func() {
		d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
			path, err := experimentplugins.DefaultPath()
			if err != nil {
				return experimentplugins.Whitelist{}, err
			}
			return experimentplugins.Load(path)
		}
		d1DeEsserAttestationReader = func() (processorattestation.LibraryV2, error) {
			store, err := processorattestation.NewStoreV2("")
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			library, report, err := store.Read()
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			_ = report
			return library, nil
		}
	})
}

func deEsserTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "the vocal sounds sibilant", Hypothesis: "a bounded de-esser threshold move may tame the sibilance",
		ExpectedEffect: "less sibilance without level shift", ActionDomain: d1DeEsserDomain,
		ActionKind: d1DeEsserKind, ParameterBounds: map[string]any{"threshold_db": -1.5},
		VerificationPlan: map[string]any{"view_ids": []any{"track.frequency_time_events"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1DeEsserLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-deess", ConversationID: "conversation-d1-deess", GoalID: "goal-d1-deess", RunID: "run-d1-deess", Status: "awaiting_experiment", OriginalIntent: "tame sibilance", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: deEsserTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestResolveD1PluginParamBindingForDeEsser(t *testing.T) {
	fixture := d1DeEsserWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture DeEss", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-deess", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1DeEsserDomain, "action_kind": d1DeEsserKind, "threshold_db": -1.5}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1DeEsserOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.LibraryV2{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1DeEsserDomain) {
		t.Fatalf("de_esser unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1DeEsserOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.LibraryV2{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("de_esser invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation v2 store is unreadable at x")
	d1DeEsserOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.LibraryV2{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("de_esser unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}
	d1DeEsserOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("de_esser ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1DeEsserDomain, "plugin_identifier": "some-other-deess"}
	d1DeEsserOverrideLoaders(t, fixture.Whitelist, nil,
		d1DeEsserPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("de_esser pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1DeEsserDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "deess_thresh" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved de_esser binding=%+v", binding)
	}
}

func TestD1S1DeEsserPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1DeEsserLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:    d1DeEsserDomain,
		PluginName: "Fixture DeEss",
		PluginPath: "C:/plugins/Fixture DeEss.vst3",
		ParamID:    "deess_thresh",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1DeEsserKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_deess") || action.Command != d1DeEsserKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:deess:deess_thresh:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.de_ess.v0" || plan.Proposal.CapabilityID != "static_mix.de_ess.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture DeEss.vst3" ||
		action.Args["plugin_name"] != "Fixture DeEss" || action.Args["param_id"] != "deess_thresh" ||
		action.Args["target_value"] != -1.5 || action.Args["target_semantics"] != "delta_db" {
		t.Fatalf("binding args=%+v", action.Args)
	}
	// GLM ruling on D2-FAM1-S1 ③, correction b: single-channel actions omit
	// the param_id_ch2 key entirely instead of carrying an empty string.
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("single-channel action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("de_esser action must not carry an EQ frequency: %+v", action.Args)
	}
	if _, present := action.Args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:de_esser_threshold_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
	// The dual-channel compression args shape stays byte-identical through the
	// shared args assembler (the ch2 key survives there).
	compLoop := d1CompressionLoopForTest(t, "7")
	compPlan, err := d1PluginParamPlanWithBinding(compLoop, agentloop.PendingMixTickCandidate{Operation: d1BroadbandCompressionKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, &d1PluginParamWhitelistBinding{
			Section: d1BroadbandCompressionDomain, PluginName: "Fixture Comp", PluginPath: "C:/plugins/Fixture Comp.vst3",
			ParamID: "thr_a", ParamIDCH2: "thr_b",
		}, "")
	if err != nil {
		t.Fatal(err)
	}
	compArgs := compPlan.ActionSet.Actions[0].Args
	if compArgs["param_id_ch2"] != "thr_b" {
		t.Fatalf("dual-channel ch2 key drifted: %+v", compArgs)
	}
}

// ---- FAM2-S1 transient_shaper fixtures ---------------------------------------

type d1TransientWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1TransientWhitelistFixtureOnDisk materializes a whitelisted transient
// shaper on disk inside t.TempDir (never the developer's ~/.vit): one shared
// attack parameter (SPL Transient Designer Plus probe 2026-09-01), admitted
// by a PCA v2 library record that promotes exactly this binary fingerprint.
func d1TransientWhitelistFixtureOnDisk(t *testing.T) d1TransientWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Transient.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-transient-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1TransientWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			TransientShaper: &experimentplugins.TransientShaperPlugin{
				PluginName:       "Fixture Transient",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-transient",
				PluginPath:       pluginPath,
				AttackParamID:    "trans_attack",
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version":   fixture.Whitelist.SchemaVersion,
		"transient_shaper": fixture.Whitelist.TransientShaper,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1TransientPromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.LibraryV2 {
	t.Helper()
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyTransient,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "envelope_emphasis"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-transient-table-1", Kind: "transient_shaper_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("c", 64),
			ObservedAt: now.Add(-time.Hour),
		}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	promotedAt := now.Add(time.Hour)
	attestation.Status = processorattestation.StatusPromoted
	attestation.StatusReason = "test_evidence_passed"
	attestation.PromotedAt = &promotedAt
	return processorattestation.LibraryV2{
		SchemaVersion: processorattestation.LibrarySchemaV2,
		UpdatedAt:     promotedAt,
		Attestations:  []processorattestation.AttestationV2{attestation},
	}
}

// d1TransientOverrideLoaders swaps the whitelist loader and the PCA v2
// attestation reader for the transient_shaper gate; the v1 reader and the
// de_esser companion stay untouched.
func d1TransientOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.LibraryV2, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1TransientShaperAttestationReader = func() (processorattestation.LibraryV2, error) {
		if readerErr != nil {
			return processorattestation.LibraryV2{}, readerErr
		}
		return library, nil
	}
	t.Cleanup(func() {
		d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
			path, err := experimentplugins.DefaultPath()
			if err != nil {
				return experimentplugins.Whitelist{}, err
			}
			return experimentplugins.Load(path)
		}
		d1TransientShaperAttestationReader = func() (processorattestation.LibraryV2, error) {
			store, err := processorattestation.NewStoreV2("")
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			library, report, err := store.Read()
			if err != nil {
				return processorattestation.LibraryV2{}, err
			}
			_ = report
			return library, nil
		}
	})
}

func transientTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "the material sounds dull on the attack", Hypothesis: "a bounded attack move may restore the onset punch",
		ExpectedEffect: "clearer transients without level shift", ActionDomain: d1TransientShaperDomain,
		ActionKind: d1TransientShaperKind, ParameterBounds: map[string]any{"attack_db": 1.5},
		VerificationPlan: map[string]any{"view_ids": []any{"track.transient_structure"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1TransientLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-trans", ConversationID: "conversation-d1-trans", GoalID: "goal-d1-trans", RunID: "run-d1-trans", Status: "awaiting_experiment", OriginalIntent: "restore transient punch", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: transientTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestResolveD1PluginParamBindingForTransient(t *testing.T) {
	fixture := d1TransientWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Transient", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-transient", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1TransientShaperDomain, "action_kind": d1TransientShaperKind, "attack_db": 1.5}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1TransientOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.LibraryV2{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1TransientShaperDomain) {
		t.Fatalf("transient unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1TransientOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.LibraryV2{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("transient invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation v2 store is unreadable at x")
	d1TransientOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.LibraryV2{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("transient unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}
	d1TransientOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("transient ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1TransientShaperDomain, "plugin_identifier": "some-other-transient"}
	d1TransientOverrideLoaders(t, fixture.Whitelist, nil,
		d1TransientPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("transient pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1TransientShaperDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "trans_attack" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved transient binding=%+v", binding)
	}
}

func TestD1S1TransientPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1TransientLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:    d1TransientShaperDomain,
		PluginName: "Fixture Transient",
		PluginPath: "C:/plugins/Fixture Transient.vst3",
		ParamID:    "trans_attack",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1TransientShaperKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_trans") || action.Command != d1TransientShaperKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:trans:trans_attack:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.transient.v0" || plan.Proposal.CapabilityID != "static_mix.transient.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture Transient.vst3" ||
		action.Args["plugin_name"] != "Fixture Transient" || action.Args["param_id"] != "trans_attack" ||
		action.Args["target_value"] != 1.5 || action.Args["target_semantics"] != "delta_db" {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("single-channel action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("transient action must not carry an EQ frequency: %+v", action.Args)
	}
	if _, present := action.Args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:transient_attack_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
}
