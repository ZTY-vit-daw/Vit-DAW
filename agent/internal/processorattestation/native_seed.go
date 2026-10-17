package processorattestation

import (
	"fmt"
	"strings"
	"time"
)

// GENSHELL-PCA-1 (方案 B): native load-only families are shipped first-party
// bundles, not third-party processors. Their permanent load admission is
// driven by the registry's code-level registration facts: agent startup
// idempotently seeds one promoted v2 attestation per native family, bound to
// the current bundle fingerprint. Issuance goes through the ordinary
// zero-coverage v2 semantics; the three existing gates (chat certification
// entry, one-shot certification token, typed coverage requirement) are not
// involved and stay unchanged.

// NativeSeedSpec carries the registry-declared facts for one native
// load-only family bundle. BundlePath must already be resolved (no "~").
type NativeSeedSpec struct {
	ProcessorFamily string
	BundlePath      string
	Identifier      string
	Name            string
	Manufacturer    string
	Format          string
}

// NativeSeedResult reports what one seeding pass did.
type NativeSeedResult struct {
	// Seeded is true when this call issued or promoted a record, and false
	// when an equally-fingerprinted promoted attestation already existed
	// (idempotent re-run, zero writes).
	Seeded      bool
	Attestation AttestationV2
}

const nativeSeedPromotionReason = "native_registry_seeded"

// SeedNativeLoadOnly seeds the permanent load-only admission for one native
// family bundle. Semantics:
//
//   - A promoted attestation with the same subject, family, and current
//     bundle fingerprint short-circuits the call (idempotent, no write).
//   - Otherwise a fresh zero-coverage attestation is issued and promoted.
//     A runtime revocation of the previous record is honored within the
//     running process (nothing re-seeds mid-process); the next startup finds
//     no promoted record for the fingerprint and re-seeds a new one — native
//     families are session-scoped under revocation by design.
//   - A changed bundle (new fingerprint) supersedes the old promoted record
//     through the store's ordinary promotion semantics.
//
// Only load-only families (empty coverage vocabulary) may be seeded; typed
// families keep their evidence-driven certification path unchanged.
func SeedNativeLoadOnly(store *StoreV2, spec NativeSeedSpec, now time.Time) (NativeSeedResult, error) {
	family := strings.ToLower(strings.TrimSpace(spec.ProcessorFamily))
	if !IsV2Family(family) {
		return NativeSeedResult{}, fmt.Errorf("processor attestation v2: native seed has unsupported processor family %q", spec.ProcessorFamily)
	}
	if len(V2CoverageAxes(family)) != 0 {
		return NativeSeedResult{}, fmt.Errorf("processor attestation v2: native seed is load-only; family %s has a typed coverage vocabulary", family)
	}
	spec.BundlePath = strings.TrimSpace(spec.BundlePath)
	if spec.BundlePath == "" {
		return NativeSeedResult{}, fmt.Errorf("processor attestation v2: native seed for %s requires a bundle path", family)
	}
	if strings.TrimSpace(spec.Identifier) == "" || strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Format) == "" {
		return NativeSeedResult{}, fmt.Errorf("processor attestation v2: native seed for %s requires identifier, name, and format", family)
	}
	subject := Subject{Name: strings.TrimSpace(spec.Name), Manufacturer: strings.TrimSpace(spec.Manufacturer), Format: strings.TrimSpace(spec.Format), Identifier: strings.TrimSpace(spec.Identifier), InstalledPath: spec.BundlePath}
	key, err := BuildSubjectKey(subject)
	if err != nil {
		return NativeSeedResult{}, err
	}
	fingerprint, err := FingerprintPath(spec.BundlePath)
	if err != nil {
		return NativeSeedResult{}, err
	}
	library, _, err := store.Read()
	if err != nil {
		return NativeSeedResult{}, err
	}
	for _, attestation := range library.Attestations {
		if attestation.Status != StatusPromoted || attestation.Subject.SubjectKey != key || attestation.ProcessorFamily != family || attestation.BinaryFingerprint != fingerprint {
			continue
		}
		return NativeSeedResult{Seeded: false, Attestation: attestation}, nil
	}
	issue := IssueSpecV2{
		Subject:           subject,
		BinaryFingerprint: fingerprint,
		ProcessorFamily:   family,
		Evidence: []EvidenceRef{{
			// The registered fact and the bundle bytes themselves are the
			// evidence: the digest re-pins the fingerprint the load gate
			// recomputes at admission time.
			ReceiptID:  "native-registry:" + family,
			Kind:       "native_registry_seed",
			SHA256:     fingerprint,
			ObservedAt: now.UTC(),
		}},
	}
	promoted, err := store.PromoteCurrent(issue, nativeSeedPromotionReason)
	if err != nil {
		return NativeSeedResult{}, err
	}
	return NativeSeedResult{Seeded: true, Attestation: promoted}, nil
}
