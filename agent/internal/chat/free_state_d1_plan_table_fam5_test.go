package chat

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experimentplugins"
	"vit-daw-agent/internal/processorattestation"
)

type d1GateWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1GateWhitelistFixtureOnDisk materializes a whitelisted gate/expander on
// disk inside t.TempDir (never the developer's ~/.vit): one shared range
// (attenuation floor) parameter (FabFilter Pro-G probe 2026-09-02), admitted
// by a PCA v2 library record that promotes exactly this binary fingerprint.
func d1GateWhitelistFixtureOnDisk(t *testing.T) d1GateWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Gate.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-gate-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1GateWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			GateExpander: &experimentplugins.GateExpanderPlugin{
				PluginName:       "Fixture Gate",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-gate",
				PluginPath:       pluginPath,
				RangeParamID:     "gate_range",
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version": fixture.Whitelist.SchemaVersion,
		"gate_expander":  fixture.Whitelist.GateExpander,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1GatePromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.LibraryV2 {
	t.Helper()
	now := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyGateExpander,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "attenuation_floor"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-gate-table-1", Kind: "gate_expander_regression_receipt",
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

// d1GateOverrideLoaders swaps the whitelist loader and the PCA v2 attestation
// reader for the gate_expander gate; the v1 reader and the other family
// companions stay untouched.
func d1GateOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.LibraryV2, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1GateExpanderAttestationReader = func() (processorattestation.LibraryV2, error) {
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
		d1GateExpanderAttestationReader = func() (processorattestation.LibraryV2, error) {
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

func gateTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "quiet intervals carry an audible noise bed", Hypothesis: "a bounded range move may attenuate the low-level bed",
		ExpectedEffect: "quiet-interval noise drops while active events stay unchanged", ActionDomain: d1GateExpanderDomain,
		ActionKind: d1GateExpanderKind, ParameterBounds: map[string]any{"range_db": 1.5},
		VerificationPlan: map[string]any{"view_ids": []any{"track.activity_structure"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1GateLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-gate", ConversationID: "conversation-d1-gate", GoalID: "goal-d1-gate", RunID: "run-d1-gate", Status: "awaiting_experiment", OriginalIntent: "tame low-level noise bed", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: gateTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestResolveD1PluginParamBindingForGateExpander(t *testing.T) {
	fixture := d1GateWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Gate", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-gate", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1GateExpanderDomain, "action_kind": d1GateExpanderKind, "range_db": 1.5}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1GateOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.LibraryV2{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1GateExpanderDomain) {
		t.Fatalf("gate unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1GateOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.LibraryV2{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("gate invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation v2 store is unreadable at x")
	d1GateOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.LibraryV2{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("gate unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}
	d1GateOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("gate ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1GateExpanderDomain, "plugin_identifier": "some-other-gate"}
	d1GateOverrideLoaders(t, fixture.Whitelist, nil,
		d1GatePromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("gate pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1GateExpanderDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "gate_range" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved gate binding=%+v", binding)
	}
}

func TestD1S1GatePlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1GateLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:    d1GateExpanderDomain,
		PluginName: "Fixture Gate",
		PluginPath: "C:/plugins/Fixture Gate.vst3",
		ParamID:    "gate_range",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1GateExpanderKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_gate") || action.Command != d1GateExpanderKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:gate:gate_range:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.gate.v0" || plan.Proposal.CapabilityID != "static_mix.gate.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture Gate.vst3" ||
		action.Args["plugin_name"] != "Fixture Gate" || action.Args["param_id"] != "gate_range" ||
		action.Args["target_value"] != 1.5 || action.Args["target_semantics"] != "delta_db" {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("single-channel action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("gate action must not carry an EQ frequency: %+v", action.Args)
	}
	if _, present := action.Args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:gate_range_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
}
