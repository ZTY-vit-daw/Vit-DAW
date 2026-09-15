package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/shadow"
)

// PCA-AUTONOMY-CATALOG-1: full project access makes the model execute the
// whole chain itself, including the plug-in load, but the PCA load gate only
// admits exact identifiers whose attestation is currently promoted AND whose
// installed binary still matches the recorded fingerprint. Until now the model
// had no surface that listed those identifiers, so autonomous selection was a
// guess and a wrong guess dead-ended the turn in the fail-closed gate
// (journey R4/R8 live observations). These tests pin the tool-catalog half:
// an explicit full-access turn must disclose the fingerprint-verified promoted
// catalog, and every other turn's catalog must stay byte-identical.

const (
	pcaAutonomyEQIdentifier      = "VST3-Autonomy-EQ-probe"
	pcaAutonomyLimiterIdentifier = "VST3-Autonomy-Limiter-probe"
	pcaAutonomyMutatedIdentifier = "VST3-Autonomy-Mutated-probe"
)

func pcaAutonomyEvidence() []processorattestation.EvidenceRef {
	return []processorattestation.EvidenceRef{{
		ReceiptID: "pca-autonomy-catalog", Kind: "test", SHA256: "sha256:" + strings.Repeat("c", 64),
		ObservedAt: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
	}}
}

// pcaAutonomyCatalogFixture seeds the PCA stores inside a temp workspace:
//   - a v1 promoted static EQ whose binary still matches its fingerprint,
//   - a v2 promoted limiter whose binary still matches its fingerprint,
//   - a v1 promoted static EQ whose binary was swapped after promotion
//     (fingerprint mismatch — the load gate rejects it, so the catalog must
//     not list it).
func pcaAutonomyCatalogFixture(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "pca-v2.json"))
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(root, "missing_plugin_semantics.json"))

	eqBundle := filepath.Join(root, "Autonomy EQ.vst3")
	if err := os.WriteFile(eqBundle, []byte("pca-autonomy-eq-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	eqFingerprint, err := processorattestation.FingerprintPath(eqBundle)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: processorattestation.Subject{
			Name: "Autonomy EQ", Manufacturer: "Vendor", Format: "VST3",
			Identifier: pcaAutonomyEQIdentifier, InstalledPath: eqBundle,
		},
		BinaryFingerprint: eqFingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: pcaAutonomyEvidence(),
	}, "pca-autonomy-test"); err != nil {
		t.Fatal(err)
	}

	limiterBundle := filepath.Join(root, "Autonomy Limiter.vst3")
	if err := os.WriteFile(limiterBundle, []byte("pca-autonomy-limiter-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	limiterFingerprint, err := processorattestation.FingerprintPath(limiterBundle)
	if err != nil {
		t.Fatal(err)
	}
	storeV2, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storeV2.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: processorattestation.Subject{
			Name: "Autonomy Limiter", Manufacturer: "Vendor", Format: "VST3",
			Identifier: pcaAutonomyLimiterIdentifier, InstalledPath: limiterBundle,
		},
		BinaryFingerprint: limiterFingerprint, ProcessorFamily: processorattestation.FamilyLimiter,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: pcaAutonomyEvidence(),
	}, "pca-autonomy-test"); err != nil {
		t.Fatal(err)
	}

	mutatedBundle := filepath.Join(root, "Autonomy Mutated.vst3")
	if err := os.WriteFile(mutatedBundle, []byte("pca-autonomy-mutated-binary-original"), 0o600); err != nil {
		t.Fatal(err)
	}
	mutatedFingerprint, err := processorattestation.FingerprintPath(mutatedBundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: processorattestation.Subject{
			Name: "Autonomy Mutated", Manufacturer: "Vendor", Format: "VST3",
			Identifier: pcaAutonomyMutatedIdentifier, InstalledPath: mutatedBundle,
		},
		BinaryFingerprint: mutatedFingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: pcaAutonomyEvidence(),
	}, "pca-autonomy-test"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutatedBundle, []byte("pca-autonomy-mutated-binary-swapped-after-promotion"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func pcaAutonomyFullAccessContext() map[string]any {
	return map[string]any{"authority_mode": authorityModeFull, "authority_mode_explicit": true}
}

func newAutonomyCatalogServer() *Server {
	return New(nil, shadow.New(nil), nil)
}

// The model-visible tool catalog of an explicit full-access turn must carry
// the promoted identifiers, grouped by processor family, so autonomous
// selection can hit an admissible identity on the first attempt instead of
// guessing (RED until the catalog appendix exists).
func TestAgentLoopToolCatalogExposesPCAPromotedIdentifiersUnderFullAccess(t *testing.T) {
	pcaAutonomyCatalogFixture(t)
	server := newAutonomyCatalogServer()

	toolContext := server.agentLoopToolContext(agentModeGoal, "帮低音轨做个均衡实验，然后让我试听", pcaAutonomyFullAccessContext())
	for _, expected := range []string{pcaAutonomyEQIdentifier, pcaAutonomyLimiterIdentifier} {
		if !strings.Contains(toolContext.CatalogSummary, expected) {
			t.Fatalf("full-access tool catalog must list promoted identifier %s; catalog:\n%s", expected, toolContext.CatalogSummary)
		}
	}
	if !strings.Contains(toolContext.CatalogSummary, processorattestation.FamilyStaticEQ) ||
		!strings.Contains(toolContext.CatalogSummary, processorattestation.FamilyLimiter) {
		t.Fatalf("full-access tool catalog must group identifiers by processor family (%s / %s); catalog:\n%s",
			processorattestation.FamilyStaticEQ, processorattestation.FamilyLimiter, toolContext.CatalogSummary)
	}
}

// Catalog-to-gate consistency: only admissions whose installed binary still
// matches the promoted fingerprint are listed. The mutated admission stays
// promoted in the store, but the load gate would reject it, so listing it
// would recreate the dead-end this card fixes.
func TestFullAccessCatalogListsOnlyFingerprintVerifiedAdmissions(t *testing.T) {
	pcaAutonomyCatalogFixture(t)
	server := newAutonomyCatalogServer()

	toolContext := server.agentLoopToolContext(agentModeGoal, "help the bass track", pcaAutonomyFullAccessContext())
	if strings.Contains(toolContext.CatalogSummary, pcaAutonomyMutatedIdentifier) {
		t.Fatalf("promoted-but-fingerprint-mismatched admission must not be listed; catalog:\n%s", toolContext.CatalogSummary)
	}
	if !strings.Contains(toolContext.CatalogSummary, pcaAutonomyEQIdentifier) {
		t.Fatalf("verified admission missing from catalog; catalog:\n%s", toolContext.CatalogSummary)
	}
}

// Every non-full-access turn keeps the exact previous catalog text: the
// appendix is a full-access-only disclosure, like the FULLACCESS-AUTONOMY-1
// directive before it. The same text+context pair resolves to the same base
// catalog, so the full-access output must be the unmodified text plus the
// appendix and nothing else — which pins the manual output byte-for-byte.
func TestAgentLoopToolCatalogManualTurnStaysByteIdentical(t *testing.T) {
	pcaAutonomyCatalogFixture(t)
	server := newAutonomyCatalogServer()
	const userText = "帮低音轨做个均衡实验，然后让我试听"

	manual := server.agentLoopToolContext(agentModeGoal, userText,
		map[string]any{"authority_mode": authorityModeManual, "authority_mode_explicit": true})
	implicit := server.agentLoopToolContext(agentModeGoal, userText, map[string]any{})
	bareMode := server.agentLoopToolContext(agentModeGoal, userText,
		map[string]any{"authority_mode": authorityModeFull})
	for name, tc := range map[string]agentLoopToolContext{
		"manual": manual, "implicit": implicit, "bare-mode-without-explicit-flag": bareMode,
	} {
		if strings.Contains(tc.CatalogSummary, pcaAutonomyEQIdentifier) ||
			strings.Contains(tc.CatalogSummary, pcaAutonomyLimiterIdentifier) {
			t.Fatalf("%s turn catalog must not disclose promoted identifiers", name)
		}
		if tc.CatalogSummary != manual.CatalogSummary {
			t.Fatalf("%s turn catalog must match the manual catalog text", name)
		}
	}

	fullAccess := server.agentLoopToolContext(agentModeGoal, userText, pcaAutonomyFullAccessContext())
	if !strings.HasPrefix(fullAccess.CatalogSummary, manual.CatalogSummary) {
		t.Fatalf("full-access catalog must be the unmodified manual catalog plus the appendix")
	}
	if len(fullAccess.CatalogSummary) <= len(manual.CatalogSummary) {
		t.Fatalf("full-access catalog must append the admitted-catalog appendix")
	}
}

// The free-state family-selection turn must not see plug-in identity: the
// observation-only catalog keeps its exact shape even under full access
// (pre-family neutrality is a design constraint, not an oversight).
func TestFullAccessCatalogDoesNotLeakIntoFreeStateObservationCatalog(t *testing.T) {
	pcaAutonomyCatalogFixture(t)
	server := newAutonomyCatalogServer()

	freeState := map[string]any{
		"authority_mode": authorityModeFull, "authority_mode_explicit": true,
		"free_state_reasoning_loop": map[string]any{
			"schema_version": freeStateReasoningLoopSchema,
			"original_intent": "帮低音轨做个均衡实验", "status": "observing",
		},
	}
	toolContext := server.agentLoopToolContext(agentModeGoal, "帮低音轨做个均衡实验，然后让我试听", freeState)
	for _, forbidden := range []string{pcaAutonomyEQIdentifier, pcaAutonomyLimiterIdentifier} {
		if strings.Contains(toolContext.CatalogSummary, forbidden) {
			t.Fatalf("free-state observation catalog must not disclose plug-in identity %s; catalog:\n%s", forbidden, toolContext.CatalogSummary)
		}
	}
}

// An unreadable PCA store must degrade explicitly in the appendix instead of
// silently listing nothing (a silent empty list would read as "nothing is
// admissible" and push the model back into guessing).
func TestFullAccessCatalogDegradesExplicitlyWhenStoreUnreadable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "corrupt-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "corrupt-v2.json"))
	if err := os.WriteFile(filepath.Join(root, "corrupt-v1.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corrupt-v2.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newAutonomyCatalogServer()

	toolContext := server.agentLoopToolContext(agentModeGoal, "help the bass track", pcaAutonomyFullAccessContext())
	if !strings.Contains(toolContext.CatalogSummary, "unavailable") {
		t.Fatalf("unreadable PCA store must surface an explicit unavailable note; catalog tail:\n%s",
			toolContext.CatalogSummary[max(0, len(toolContext.CatalogSummary)-400):])
	}
}

// A promoted library larger than the verification cap must say so instead of
// silently pretending the listing is complete, and the per-family listing
// keeps its bound with an explicit "+N more" count.
func TestFullAccessCatalogCapsLargeLibrariesExplicitly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "pca-v2.json"))
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	identifiers := []string{}
	for i := 0; i <= pcaAutonomyCatalogVerifyLimit; i++ { // one record beyond the cap
		bundle := filepath.Join(root, fmt.Sprintf("CapEQ-%02d.vst3", i))
		if err := os.WriteFile(bundle, []byte(fmt.Sprintf("cap-binary-%02d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		fingerprint, err := processorattestation.FingerprintPath(bundle)
		if err != nil {
			t.Fatal(err)
		}
		identifier := fmt.Sprintf("VST3-CapEQ-%02d-probe", i)
		if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
			Subject: processorattestation.Subject{
				Name: fmt.Sprintf("CapEQ %02d", i), Manufacturer: "Vendor", Format: "VST3",
				Identifier: identifier, InstalledPath: bundle,
			},
			BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
			Coverage:  []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
			Evidence:  pcaAutonomyEvidence(),
		}, "pca-autonomy-test"); err != nil {
			t.Fatal(err)
		}
		identifiers = append(identifiers, identifier)
	}
	server := newAutonomyCatalogServer()

	toolContext := server.agentLoopToolContext(agentModeGoal, "help the bass track", pcaAutonomyFullAccessContext())
	if !strings.Contains(toolContext.CatalogSummary, "listing capped at") {
		t.Fatalf("a library beyond the verification cap must carry the cap note; catalog tail:\n%s",
			toolContext.CatalogSummary[max(0, len(toolContext.CatalogSummary)-400):])
	}
	// The first (verifyLimit) identifiers stay listed under the family bound.
	if !strings.Contains(toolContext.CatalogSummary, "+") || !strings.Contains(toolContext.CatalogSummary, "more") {
		t.Fatalf("family listing beyond the per-family bound must show an explicit +N more; catalog tail:\n%s",
			toolContext.CatalogSummary[max(0, len(toolContext.CatalogSummary)-400):])
	}
}
