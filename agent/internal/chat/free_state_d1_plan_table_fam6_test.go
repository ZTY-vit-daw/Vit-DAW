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

type d1MultibandWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1MultibandWhitelistFixtureOnDisk materializes a whitelisted multiband on
// disk inside t.TempDir (never the developer's ~/.vit): one threshold
// parameter per band (Lindell MBC probe 2026-09-02), admitted by a PCA v2
// library record that promotes exactly this binary fingerprint.
func d1MultibandWhitelistFixtureOnDisk(t *testing.T) d1MultibandWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture MBC.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-mbc-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1MultibandWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Multiband: &experimentplugins.MultibandPlugin{
				PluginName:            "Fixture MBC",
				Manufacturer:          "Fixture",
				Format:                "VST3",
				PluginIdentifier:      "fixture-mbc",
				PluginPath:            pluginPath,
				BandThresholdParamIDs: []string{"mb_low_threshold", "mb_mid_threshold", "mb_high_threshold"},
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version": fixture.Whitelist.SchemaVersion,
		"multiband":      fixture.Whitelist.Multiband,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1MultibandPromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.LibraryV2 {
	t.Helper()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyMultiband,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "band_dynamics"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-mbc-table-1", Kind: "multiband_regression_receipt",
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

// d1MultibandOverrideLoaders swaps the whitelist loader and the PCA v2
// attestation reader for the multiband gate; the other family companions stay
// untouched.
func d1MultibandOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.LibraryV2, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1MultibandAttestationReader = func() (processorattestation.LibraryV2, error) {
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
		d1MultibandAttestationReader = func() (processorattestation.LibraryV2, error) {
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

func multibandTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "the low band pumps audibly on every sustained note", Hypothesis: "a bounded band threshold move may ease the low-band pumping",
		ExpectedEffect: "low-band level steps flatten while the other bands stay unchanged", ActionDomain: d1MultibandDomain,
		ActionKind: d1MultibandKind, ParameterBounds: map[string]any{"band_threshold_db": 1.0, "band_index": 2},
		VerificationPlan: map[string]any{"view_ids": []any{"track.band_dynamics"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1MultibandLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-mb", ConversationID: "conversation-d1-mb", GoalID: "goal-d1-mb", RunID: "run-d1-mb", Status: "awaiting_experiment", OriginalIntent: "ease low-band pumping", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: multibandTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestResolveD1PluginParamBindingForMultiband(t *testing.T) {
	fixture := d1MultibandWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture MBC", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-mbc", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1MultibandDomain, "action_kind": d1MultibandKind, "band_threshold_db": 1.0, "band_index": 2}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1MultibandOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.LibraryV2{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1MultibandDomain) {
		t.Fatalf("multiband unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1MultibandOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.LibraryV2{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("multiband invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation v2 store is unreadable at x")
	d1MultibandOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.LibraryV2{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("multiband unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}
	d1MultibandOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("multiband ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1MultibandDomain, "plugin_identifier": "some-other-mbc", "band_threshold_db": 1.0}
	d1MultibandOverrideLoaders(t, fixture.Whitelist, nil,
		d1MultibandPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("multiband pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1MultibandDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "mb_high_threshold" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved multiband binding=%+v", binding)
	}

	for name, index := range map[string]any{"defaults_to_band_0": nil, "band_0": 0.0, "band_1": 1.0} {
		action := map[string]any{"action_domain": d1MultibandDomain, "action_kind": d1MultibandKind, "band_threshold_db": -1.0}
		if index != nil {
			action["band_index"] = index
		}
		binding, err := resolveD1PluginParamWhitelistBinding(action)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := []string{"mb_low_threshold", "mb_mid_threshold"}[map[string]int{"defaults_to_band_0": 0, "band_0": 0, "band_1": 1}[name]]
		if binding.ParamID != want {
			t.Fatalf("%s: param id=%q want %q", name, binding.ParamID, want)
		}
	}
	for name, index := range map[string]any{"negative": -1.0, "out_of_range": 3.0, "non_integer": 1.5} {
		action := map[string]any{"action_domain": d1MultibandDomain, "action_kind": d1MultibandKind, "band_threshold_db": 1.0, "band_index": index}
		if _, err := resolveD1PluginParamWhitelistBinding(action); err == nil || !strings.Contains(err.Error(), "band_index") {
			t.Fatalf("%s: err=%v must name band_index", name, err)
		}
	}
}

func TestD1S1MultibandPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1MultibandLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:          d1MultibandDomain,
		PluginName:       "Fixture MBC",
		PluginPath:       "C:/plugins/Fixture MBC.vst3",
		PluginIdentifier: "fixture-mbc",
		ParamID:          "mb_high_threshold",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1MultibandKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_mb") || action.Command != d1MultibandKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:mb:mb_high_threshold:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.multiband.v0" || plan.Proposal.CapabilityID != "static_mix.multiband.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture MBC.vst3" ||
		action.Args["plugin_name"] != "Fixture MBC" || action.Args["param_id"] != "mb_high_threshold" ||
		action.Args["target_value"] != 1.0 || action.Args["target_semantics"] != "delta_db" {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("single-channel action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("multiband action must not carry an EQ frequency: %+v", action.Args)
	}
	if got := action.Args["plugin_identifier"]; got != "fixture-mbc" {
		t.Fatalf("whitelist-bound action must pin the whitelisted plugin identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:multiband_band_threshold_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
}
