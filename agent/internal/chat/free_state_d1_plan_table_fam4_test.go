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

type d1LimiterWhitelistFixture struct {
	Whitelist   experimentplugins.Whitelist
	PluginPath  string
	Fingerprint string
}

// d1LimiterWhitelistFixtureOnDisk materializes a whitelisted limiter on disk
// inside t.TempDir (never the developer's ~/.vit): one shared ceiling
// parameter (FabFilter Pro-L 2 probe 2026-09-02), admitted by a PCA v2
// library record that promotes exactly this binary fingerprint.
func d1LimiterWhitelistFixtureOnDisk(t *testing.T) d1LimiterWhitelistFixture {
	t.Helper()
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "Fixture Limiter.vst3")
	if err := os.WriteFile(pluginPath, []byte("fixture-limiter-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture := d1LimiterWhitelistFixture{
		Whitelist: experimentplugins.Whitelist{
			SchemaVersion: experimentplugins.SchemaVersion,
			Limiter: &experimentplugins.LimiterPlugin{
				PluginName:       "Fixture Limiter",
				Manufacturer:     "Fixture",
				Format:           "VST3",
				PluginIdentifier: "fixture-limiter",
				PluginPath:       pluginPath,
				CeilingParamID:   "lim_ceiling",
			},
		},
		PluginPath:  pluginPath,
		Fingerprint: fingerprint,
	}
	encoded, err := json.MarshalIndent(map[string]any{
		"schema_version": fixture.Whitelist.SchemaVersion,
		"limiter":        fixture.Whitelist.Limiter,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "free_state_experiment_plugins.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func d1LimiterPromotedLibrary(t *testing.T, subject processorattestation.Subject, fingerprint string) processorattestation.LibraryV2 {
	t.Helper()
	now := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	attestation, err := processorattestation.NewAttestationV2(processorattestation.IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   processorattestation.FamilyLimiter,
		Coverage:          []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "receipt-limiter-table-1", Kind: "limiter_regression_receipt",
			SHA256:     "sha256:" + strings.Repeat("f", 64),
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

// d1LimiterOverrideLoaders swaps the whitelist loader and the PCA v2
// attestation reader for the limiter gate; the v1 reader and the other
// family companions stay untouched.
func d1LimiterOverrideLoaders(t *testing.T, whitelist experimentplugins.Whitelist, loadErr error, library processorattestation.LibraryV2, readerErr error) {
	t.Helper()
	d1StaticEQWhitelistLoader = func() (experimentplugins.Whitelist, error) {
		if loadErr != nil {
			return experimentplugins.Whitelist{}, loadErr
		}
		return whitelist, nil
	}
	d1LimiterAttestationReader = func() (processorattestation.LibraryV2, error) {
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
		d1LimiterAttestationReader = func() (processorattestation.LibraryV2, error) {
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

func limiterTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "sparse peaks shoot past the mix ceiling", Hypothesis: "a bounded ceiling move may tame the overshoots",
		ExpectedEffect: "peaks sit under the ceiling without a level shift", ActionDomain: d1LimiterDomain,
		ActionKind: d1LimiterKind, ParameterBounds: map[string]any{"ceiling_db": -1.5},
		VerificationPlan: map[string]any{"view_ids": []any{"track.peak_structure"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1LimiterLoopForTest(t *testing.T, revision string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d1-lim", ConversationID: "conversation-d1-lim", GoalID: "goal-d1-lim", RunID: "run-d1-lim", Status: "awaiting_experiment", OriginalIntent: "tame sparse peak overshoot", LatestObservation: d1FreshObservationForTest(revision), CreatedAt: now, UpdatedAt: now}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: limiterTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

func TestResolveD1PluginParamBindingForLimiter(t *testing.T) {
	fixture := d1LimiterWhitelistFixtureOnDisk(t)
	subject := processorattestation.Subject{Name: "Fixture Limiter", Manufacturer: "Fixture", Format: "VST3", Identifier: "fixture-limiter", InstalledPath: fixture.PluginPath}
	typedAction := map[string]any{"action_domain": d1LimiterDomain, "action_kind": d1LimiterKind, "ceiling_db": -1.5}

	unconfiguredErr := func() error {
		_, err := experimentplugins.Load(filepath.Join(t.TempDir(), "free_state_experiment_plugins.json"))
		return err
	}()
	d1LimiterOverrideLoaders(t, experimentplugins.Whitelist{}, unconfiguredErr, processorattestation.LibraryV2{}, nil)
	_, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), d1LimiterDomain) {
		t.Fatalf("limiter unconfigured class wrong: %v", err)
	}

	corruptErr := func() error {
		corrupt := filepath.Join(t.TempDir(), "broken.json")
		if writeErr := os.WriteFile(corrupt, []byte("{nope"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		_, loadErr := experimentplugins.Load(corrupt)
		return loadErr
	}()
	d1LimiterOverrideLoaders(t, experimentplugins.Whitelist{}, corruptErr, processorattestation.LibraryV2{}, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "whitelist is invalid") {
		t.Fatalf("limiter invalid-whitelist class must stay distinguishable: %v", err)
	}

	readerErr := errors.New("processor attestation v2 store is unreadable at x")
	d1LimiterOverrideLoaders(t, fixture.Whitelist, nil, processorattestation.LibraryV2{}, readerErr)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "could not evaluate its PCA admission") {
		t.Fatalf("limiter unreadable store class wrong: %v", err)
	}

	emptyLibrary := processorattestation.LibraryV2{SchemaVersion: processorattestation.LibrarySchemaV2}
	d1LimiterOverrideLoaders(t, fixture.Whitelist, nil, emptyLibrary, nil)
	_, err = resolveD1PluginParamWhitelistBinding(typedAction)
	if err == nil || !strings.Contains(err.Error(), "refused by the PCA admission check") || !strings.Contains(err.Error(), "not PCA-promoted") {
		t.Fatalf("limiter ineligible class wrong: %v", err)
	}

	pinned := map[string]any{"action_domain": d1LimiterDomain, "plugin_identifier": "some-other-limiter"}
	d1LimiterOverrideLoaders(t, fixture.Whitelist, nil,
		d1LimiterPromotedLibrary(t, subject, fixture.Fingerprint), nil)
	_, err = resolveD1PluginParamWhitelistBinding(pinned)
	if err == nil || !strings.Contains(err.Error(), "pinned plugin_identifier") || !strings.Contains(err.Error(), "whitelist admits") {
		t.Fatalf("limiter pinned identity class wrong: %v", err)
	}

	binding, err := resolveD1PluginParamWhitelistBinding(typedAction)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Section != d1LimiterDomain || binding.PluginPath != fixture.PluginPath ||
		binding.ParamID != "lim_ceiling" || binding.ParamIDCH2 != "" || binding.FrequencyHz != 0 {
		t.Fatalf("resolved limiter binding=%+v", binding)
	}
}

func TestD1S1LimiterPlanEmbedsWhitelistBinding(t *testing.T) {
	loop := d1LimiterLoopForTest(t, "7")
	binding := &d1PluginParamWhitelistBinding{
		Section:    d1LimiterDomain,
		PluginName: "Fixture Limiter",
		PluginPath: "C:/plugins/Fixture Limiter.vst3",
		ParamID:    "lim_ceiling",
	}
	plan, err := d1PluginParamPlanWithBinding(loop, agentloop.PendingMixTickCandidate{Operation: d1LimiterKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7",
		map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}, binding, "")
	if err != nil {
		t.Fatal(err)
	}
	action := plan.ActionSet.Actions[0]
	if !strings.HasSuffix(action.ID, "_lim") || action.Command != d1LimiterKind {
		t.Fatalf("action id/command drifted: %+v", action)
	}
	if action.BeforeFingerprint != "track:vocal:lim:lim_ceiling:pending" {
		t.Fatalf("fingerprint=%q", action.BeforeFingerprint)
	}
	if plan.ActionSet.CapabilityID != "static_mix.limiter.v0" || plan.Proposal.CapabilityID != "static_mix.limiter.v0" {
		t.Fatalf("capability ids drifted: set=%+v proposal=%+v", plan.ActionSet.CapabilityID, plan.Proposal.CapabilityID)
	}
	if action.Args["write_mode"] != "normalized_batch_v1" || action.Args["plugin_path"] != "C:/plugins/Fixture Limiter.vst3" ||
		action.Args["plugin_name"] != "Fixture Limiter" || action.Args["param_id"] != "lim_ceiling" ||
		action.Args["target_value"] != -1.5 || action.Args["target_semantics"] != "delta_db" {
		t.Fatalf("binding args=%+v", action.Args)
	}
	if _, present := action.Args["param_id_ch2"]; present {
		t.Fatalf("single-channel action must omit param_id_ch2: %+v", action.Args)
	}
	if _, present := action.Args["frequency_hz"]; present {
		t.Fatalf("limiter action must not carry an EQ frequency: %+v", action.Args)
	}
	if _, present := action.Args["plugin_identifier"]; present {
		t.Fatalf("real-plugin action must not carry a known-list identifier: %+v", action.Args)
	}
	for _, want := range []string{"free_state:d1_s1", "action:limiter_ceiling_adjust"} {
		if !containsStringFold(plan.ProjectCut.ContractVersions, want) {
			t.Fatalf("contract %q missing from %+v", want, plan.ProjectCut.ContractVersions)
		}
	}
}
