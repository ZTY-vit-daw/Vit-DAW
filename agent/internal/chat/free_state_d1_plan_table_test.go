package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorattestation"
)

// ---- fixtures ---------------------------------------------------------------

type d1TableWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1TableWriteWhitelistFixture materializes a whitelisted static_eq plugin on
// disk inside t.TempDir (never the developer's ~/.vit) and its PCA v1 library
// record that promotes exactly this binary fingerprint.
func d1TableWriteWhitelistFixture(t *testing.T) d1TableWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture EQ.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-eq-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1TableWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			StaticEQ: experimentplugins.StaticEQPlugins{experimentplugins.StaticEQPlugin{
				PluginName:       "Fixture EQ",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-eq",
				PluginPath:       pluginPath,
				Bands: []experimentplugins.Band{
					{CenterHz: 100, GainParamIDCH1: "p100_c1", GainParamIDCH2: "p100_c2"},
					{CenterHz: 315, GainParamIDCH1: "p315_c1", GainParamIDCH2: "p315_c2"},
					{CenterHz: 4000, GainParamIDCH1: "p4000_c1", GainParamIDCH2: "p4000_c2"},
				}},
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version": fixture.Whitelist.SchemaVersion,
		"static_eq":      fixture.Whitelist.StaticEQ,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1TablePromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.Library {
	t.Helper()
	now := time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestation(processorattestation.IssueSpec{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyStaticEQ,
		Coverage:          []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-table-1", Kind: "eq_regression_receipt",
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
	return processorattestation.Library{
		SchemaVersion: processorattestation.LibrarySchema,
		UpdatedAt:     promotedAt,
		Attestations:  []processorattestation.Attestation{attestation},
	}
}

func d1TableOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.Library, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1StaticEQAttestationReader = func() (processorattestation.Library, error) {
		if readerErr != nil {
			return processorattestation.Library{}, readerErr
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
		d1StaticEQAttestationReader = func() (processorattestation.Library, error) {
			store, err := processorattestation.NewStore("")
			if err != nil {
				return processorattestation.Library{}, err
			}
			library, report, err := store.Read()
			if err != nil {
				return processorattestation.Library{}, err
			}
			_ = report
			return library, nil
		}
	})
}

// ---- domain table carries execution descriptors -----------------------------

func TestD1S1TableDrivenDomainSpecsCarryExecutionDescriptors(t *testing.T) {
	gainSpec, okGain := experiment.D1S1SpecForAction(experiment.D1S1ActionDomain, experiment.D1S1ActionKind)
	eqSpec, okEq := experiment.D1S1SpecForAction(d1StaticEQDomain, d1StaticEQKind)
	if !okGain || !okEq {
		t.Fatalf("domain table missing rows: gain=%v eq=%v", okGain, okEq)
	}
	if gainSpec.ActionIDSuffix != "_gain" || gainSpec.CapabilityID != "static_mix.static_balance.v0" ||
		len(gainSpec.ContractVersions) != 2 || len(gainSpec.ObservationViewIDs) == 0 ||
		gainSpec.Journal.CommandLabel != "set_volume" || len(gainSpec.Journal.Fields) != 1 ||
		gainSpec.WriteBinding.PluginBound || gainSpec.WriteBinding.Channels != 1 {
		t.Fatalf("track_gain descriptor drifted: %+v", gainSpec)
	}
	if !strings.Contains(gainSpec.TargetFingerprintTemplate, "{db}") {
		t.Fatalf("track_gain fingerprint template lost its db marker: %q", gainSpec.TargetFingerprintTemplate)
	}
	if eqSpec.ActionIDSuffix != "_eq" || eqSpec.CapabilityID != "static_mix.static_eq.v0" ||
		eqSpec.WriteBinding.StubParamIDFormat != "band_%d_gain" || eqSpec.WriteBinding.Channels != 2 ||
		eqSpec.Journal.Tool != "set_plugin_param" || len(eqSpec.Journal.Fields) != 3 {
		t.Fatalf("static_eq descriptor drifted: %+v", eqSpec)
	}
}

// Table-driven builders reproduce the historical byte shapes for both domains;
// this locks the equivalence refactor while the execution pipeline switches to
// the parameter-driven port.
func TestD1S1TableDrivenBuildersReproduceLegacyPlans(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	plan, err := d1StaticEQPlan(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7", state)
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_eq") || plan.ActionSet.CapabilityID != "static_mix.static_eq.v0" || plan.Proposal.CapabilityID != "static_mix.static_eq.v0" {
		t.Fatalf("action=%+v set=%+v", action, plan.ActionSet)
	}
	if action.BeforeFingerprint != "track:vocal:eq:band_0_gain:pending" || action.Args["plugin_identifier"] != defaultD1StaticEQPluginIdentifier {
		t.Fatalf("stub args changed: %+v", action)
	}
	// projectcut.Build canonicalizes version order; assert membership.
	for _, want := range []string{"free_state:d1_s1", "action:static_eq_band_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
	// Journal mapping from the table equals the historical audit wording.
	record := d1JournalRecordForAction(true, "a1", "g", "r", "vocal",
		map[string]any{"plugin_identifier": "juce_eq", "param_id": "band_0_gain", "target_value": -1.5})
	if record.Summary != "D2-1 bounded static EQ band adjustment" || record.Tool != "set_plugin_param" ||
		record.Command["cmd"] != "set_plugin_param" || record.Command["param_id"] != "band_0_gain" ||
		record.Command["value"] != -1.5 || record.Command["plugin_id"] != "juce_eq" {
		t.Fatalf("table journal record=%+v", record)
	}
	gainRecord := d1JournalRecordForAction(false, "a2", "g", "r", "vocal", map[string]any{"target_db": -3.0})
	if gainRecord.Tool != "track_gain_adjust" || gainRecord.Command["cmd"] != "set_volume" || gainRecord.Command["db"] != -3.0 {
		t.Fatalf("gain journal record=%+v", gainRecord)
	}
}

func TestD1S1StaticEQPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1StaticEQLoopForTest(t, "7")
	binding := &d1StaticEQWhitelistBinding{
		Plugin: experimentplugins.StaticEQPlugin{
			PluginName: "Fixture EQ", Manufacturer: "Fixture", Format: "VST3",
			PluginIdentifier: "fixture-eq", PluginPath: "C:/plugins/Fixture EQ.vst3",
		},
		Band:        experimentplugins.Band{CenterHz: 315, GainParamIDCH1: "p315_c1", GainParamIDCH2: "p315_c2"},
		FrequencyHz: 400,
	}
	plan, err := d1StaticEQPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding)
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if action.BeforeFingerprint != "track:vocal:eq:p315_c1:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture EQ.vst3" ||
		action.Args["param_id"] != "p315_c1" || action.Args["param_id_ch2"] != "p315_c2" ||
		action.Args["target_value"] != -1.0 || action.Args["frequency_hz"] != 400.0 {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if got := action.Args["plugin_identifier"]; got != "fixture-eq" {
		t.Fatalf("whitelist-bound action must pin the whitelisted plugin identifier: %+v", action.Args)
	}
}

// ---- execution gate classification ------------------------------------------

func TestResolveD1StaticEQWhitelistBindingClasses(t *testing.T) {
	fixture := d1TableWriteWhitelistFixture(t)
	subject := processorattestation.Subject{Name: "Fixture EQ", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-eq", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"frequency_hz": 400.0, "gain_db": -1.0}

	missingDir := t.TempDir()
	unconfiguredLoadErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(missingDir, "free_state_experiment_plugins.json"))
		return err
	}()
	d1TableOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredLoadErr, processorattestation.Library{}, nil)
	if _, err := resolveD1StaticEQWhitelistBinding(typedAction); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured class wrong: %v", err)
	} else if !strings.Contains(err.Error(), "free_state_experiment_plugins.json") {
		t.Fatalf("unconfigured error should surface the machine-local path: %v", err)
	}

	corruptPath := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(corruptPath, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	corruptLoadErr := func() error {
		_, err := experimentplugins.Load(corruptPath)
		return err
	}()
	d1TableOverrideLoaders(t, experimentplugins.Whitelist{}, corruptLoadErr, processorattestation.Library{}, nil)
	_, err := resolveD1StaticEQWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("invalid-whitelist class must stay distinguishable: %v", err)
	}

	emptyReaderErr := errors.New("processor attestation store is unreadable at x")
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.Library{}, emptyReaderErr)
	if _, err := resolveD1StaticEQWhitelistBinding(typedAction); err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("unreadable store class wrong: %v", err)
	}

	// A store whose file is missing decodes to a valid-schema empty library,
	// exactly what the production attestation reader returns; every query on
	// it answers not-promoted.
	emptyLibrary := processorattestation.Library{SchemaVersion: processorattestation.LibrarySchema}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1StaticEQWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("ineligible class wrong: %v", err)
	}

	d1TableOverrideLoaders(t, fixture.Whitelist, nil,
		d1TablePromotedLibrary(t, subject, fixture.Fingerprint), nil)
	binding, err := resolveD1StaticEQWhitelistBinding(map[string]any{"frequency_hz": 400.0, "plugin_identifier": "some-other"})
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("pinned identity class wrong: %v", err)
	}
	if binding != nil {
		t.Fatalf("unexpected binding on refusal: %+v", binding)
	}

	binding, err = resolveD1StaticEQWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Band.GainParamIDCH1 != "p315_c1" || binding.Band.CenterHz != 315 || binding.Plugin.PluginName != "Fixture EQ" {
		t.Fatalf("resolved binding=%+v", binding)
	}
}

func TestExecuteD1StaticEQGateRunsBeforeDependencies(t *testing.T) {
	s := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-eq-gate", ConversationID: "conversation-d1-eq-gate", GoalID: "goal-d1-eq-gate", RunID: "run-d1-eq-gate",
		Status: "awaiting_experiment", OriginalIntent: "improve vocal clarity",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: staticEQTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(loop)

	unconfigured := filepath.Join(t.TempDir(), "free_state_experiment_plugins.json")
	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(unconfigured)
		return err
	}()
	d1TableOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.Library{}, nil)
	response, handled := s.executeD1StaticEQ(context.Background(), loop.ConversationID, ChatRequest{}, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"})
	if !handled {
		t.Fatal("gate refusal was not handled by the static_eq executor")
	}
	if !strings.Contains(response.Error, "not configured") {
		t.Fatalf("expected the whitelist boundary first, got %q", response.Error)
	}

	fixture := d1TableWriteWhitelistFixture(t)
	subject := processorattestation.Subject{Name: "Fixture EQ", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-eq", InstalledPath: fixture.PluginPath}
	d1TableOverrideLoaders(t, fixture.Whitelist, nil, d1TablePromotedLibrary(t, subject, fixture.Fingerprint), nil)
	response, handled = s.executeD1StaticEQ(context.Background(), loop.ConversationID, ChatRequest{}, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"})
	if !handled {
		t.Fatal("pass-through was not handled by the static_eq executor")
	}
	if !strings.Contains(response.Error, "durable execution dependencies are unavailable") {
		t.Fatalf("admitted session must proceed past the gate into dependency checks, got %q", response.Error)
	}
}

// ---- verifier accepts the normalized-batch receipt shape --------------------

func TestStaticEQVerifierHandlesNormalizedBatchReceipts(t *testing.T) {
	state := &kernel.VSPStateResult{Response: map[string]any{"type": "state.snapshot"}, Payload: map[string]any{"snapshot_hash": "snapshot-8"}, LegacyState: map[string]any{}, ProjectEpoch: "epoch-nb", Revision: 8, SnapshotHash: "snapshot-8"}
	goodReceipt := orchestration.ActionReceipt{ActionID: "d1-eq-action", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"write_mode": "normalized_batch_v1", "readback_verified": true,
		"requested_target_value": -1.5, "actual_readback_value": -1.5,
		"normalized_channels": []any{
			map[string]any{"parameter_id": "p315_c1", "requested_normalized": 0.46875, "actual_normalized": 0.46875001},
			map[string]any{"parameter_id": "p315_c2", "requested_normalized": 0.46875, "actual_normalized": 0.46874999},
		},
	}}
	acoustic := d1StaticEQAcousticForTest{result: executionverifiers.AcousticResult{Status: "pass", Fresh: true, ObservationID: "obs-after-nb", ObservationRevision: "8"}}
	verifier := executionverifiers.StaticEQ{State: d1StaticEQStateForTest{state: state}, Acoustic: acoustic}
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{ID: "d1-eq-action", Command: d1StaticEQKind, TargetRef: "vocal", Args: map[string]any{"param_id": "p315_c1", "target_value": -1.5}}}}
	result, err := verifier.Verify(context.Background(), actionSet, []orchestration.ActionReceipt{goodReceipt})
	if err != nil || result.Structural != "pass" || result.Status != "pass" {
		t.Fatalf("verified batch receipt rejected: result=%+v err=%v", result, err)
	}

	driftedReceipt := orchestration.ActionReceipt{ActionID: "d1-eq-action", Status: "applied", AppliedRevision: "8", Details: map[string]any{
		"write_mode": "normalized_batch_v1", "readback_verified": false,
		"normalized_channels": []any{
			map[string]any{"parameter_id": "p315_c1", "requested_normalized": 0.46875, "actual_normalized": 0.46985},
		},
	}}
	result, err = verifier.Verify(context.Background(), actionSet, []orchestration.ActionReceipt{driftedReceipt})
	if err == nil || result.Structural != "fail" || !strings.Contains(err.Error(), "normalized readback postcondition") {
		t.Fatalf("drifted channels must fail structurally: result=%+v err=%v", result, err)
	}

	emptyReceipt := orchestration.ActionReceipt{ActionID: "d1-eq-action", Status: "applied", AppliedRevision: "8", Details: map[string]any{
		"write_mode":         "normalized_batch_v1",
		"readback_verified":  true,
		"actual_readback_db": -1.5,
	}}
	result, err = verifier.Verify(context.Background(), actionSet, []orchestration.ActionReceipt{emptyReceipt})
	if err == nil || result.Structural != "fail" {
		t.Fatalf("batch mode without channel records must fail structurally: result=%+v err=%v", result, err)
	}
}
