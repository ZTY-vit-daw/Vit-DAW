package chat

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/processorattestation"
)

// PCA-AUTONOMY-CATALOG-1: full project access is the user's standing grant of
// autonomous execution, and the harness PCA load gate
// (authorizeFullProjectAccessLoad) correctly fails closed unless the model
// names one exact identifier that is currently promoted AND whose installed
// binary still matches the recorded fingerprint. But no model-visible surface
// ever listed those identifiers, so autonomous selection could only guess,
// and a wrong guess dead-ended the turn in the gate (journey R4/R8 live
// observations). This file renders that authoritative catalog into the
// tool-catalog text of explicit full-access turns — the same disclosure the
// load gate enforces, made visible before the guess instead of after it.
//
// Disclosure boundaries (pinned by pca_autonomy_catalog_test.go):
//   - only explicit full-access turns (authority_mode=full_project_access plus
//     authority_mode_explicit, the same pair the harness gate reads) get the
//     appendix; every other turn's catalog stays byte-identical;
//   - the free-state family-selection turn keeps its observation-only catalog:
//     pre-family neutrality must not see plug-in identity;
//   - only fingerprint-verified, unambiguous admissions are listed — an
//     identifier the gate would reject must not appear here, or the appendix
//     would recreate the dead-end it exists to fix.

const (
	// pcaAutonomyCatalogFamilyLimit bounds the identifiers listed per
	// processor family. The full promoted library can be long; the model needs
	// enough per family to make a loadable choice, not the whole index.
	pcaAutonomyCatalogFamilyLimit = 3

	// pcaAutonomyCatalogVerifyLimit bounds how many promoted records get a
	// binary fingerprint verification per turn. Fingerprinting hashes the
	// installed file, so a pathological catalog must not turn prompt assembly
	// into a bulk disk scan; records beyond the cap are skipped with a note.
	// The cap sits above the real machine's current promoted library
	// (~60 admissions, v1+v2, 2026-09-15) so the listing stays complete there.
	pcaAutonomyCatalogVerifyLimit = 72

	pcaAutonomyCatalogAppendixHeading = "PCA-admitted autonomous-load processors (exact plugin_identifier values the admission gate currently accepts for rack.add_node / plugin.load_to_rack; any other identifier is rejected)"
)

// pcaAutonomyCatalogEntry is one loadable identity as the gate would admit it.
type pcaAutonomyCatalogEntry struct {
	Family     string
	Identifier string
}

// explicitFullAccessRequestContext mirrors the harness load gate's
// explicitFullProjectAccess and agentloop's messageLoopFullProjectAccess: the
// mode value alone is never a grant — the explicit flag must be present too.
func explicitFullAccessRequestContext(requestContext map[string]any) bool {
	if requestContext == nil || !boolValue(requestContext["authority_mode_explicit"]) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(firstStringFromMap(requestContext, "authority_mode", "permission_mode")), authorityModeFull)
}

// collectPCAAutonomyCatalogEntries reads both PCA stores and returns the
// admissions an autonomous full-access load could actually use right now:
// promoted status, non-empty identifier and installed path, current binary
// fingerprint equal to the recorded one, and a resolution to exactly one
// identity per identifier (the gate fails closed on ambiguity, so an
// ambiguous identifier must not be listed). Output is deterministic:
// families and identifiers sorted. capped reports that fingerprint
// verification stopped at pcaAutonomyCatalogVerifyLimit records.
func collectPCAAutonomyCatalogEntries() (entries []pcaAutonomyCatalogEntry, capped bool, err error) {
	type resolution struct {
		family string
		keys   []string
	}
	resolutions := map[string]*resolution{}

	record := func(subject processorattestation.Subject, family string) {
		if strings.TrimSpace(subject.Identifier) == "" || strings.TrimSpace(subject.InstalledPath) == "" {
			return
		}
		key := strings.ToLower(subject.InstalledPath) + "\x00" +
			strings.ToLower(strings.TrimSpace(family)) + "\x00" + strings.ToLower(subject.SubjectKey)
		res, exists := resolutions[subject.Identifier]
		if !exists {
			res = &resolution{family: strings.ToLower(strings.TrimSpace(family))}
			resolutions[subject.Identifier] = res
		}
		if !containsString(res.keys, key) {
			res.keys = append(res.keys, key)
		}
	}

	store, err := processorattestation.NewStore("")
	if err != nil {
		return nil, false, fmt.Errorf("pca admission store unavailable: %w", err)
	}
	library, _, err := store.Read()
	if err != nil {
		return nil, false, fmt.Errorf("pca admission store unreadable: %w", err)
	}
	storeV2, err := processorattestation.NewStoreV2("")
	if err != nil {
		return nil, false, fmt.Errorf("pca admission v2 store unavailable: %w", err)
	}
	libraryV2, _, err := storeV2.Read()
	if err != nil {
		return nil, false, fmt.Errorf("pca admission v2 store unreadable: %w", err)
	}

	verifiedCount := 0
	scan := func(subject processorattestation.Subject, family, recordedFingerprint string) bool {
		// Returns false when the verification cap is reached and scanning must stop.
		if verifiedCount >= pcaAutonomyCatalogVerifyLimit {
			return false
		}
		current, fpErr := processorattestation.FingerprintPath(subject.InstalledPath)
		if fpErr != nil || !strings.EqualFold(current, recordedFingerprint) {
			return true
		}
		verifiedCount++
		record(subject, family)
		return true
	}
	for _, attestation := range library.Attestations {
		if attestation.Status != processorattestation.StatusPromoted {
			continue
		}
		if !scan(attestation.Subject, attestation.ProcessorFamily, attestation.BinaryFingerprint) {
			capped = true
			break
		}
	}
	if !capped {
		for _, attestation := range libraryV2.Attestations {
			if attestation.Status != processorattestation.StatusPromoted {
				continue
			}
			if !scan(attestation.Subject, attestation.ProcessorFamily, attestation.BinaryFingerprint) {
				capped = true
				break
			}
		}
	}

	for identifier, res := range resolutions {
		if len(res.keys) != 1 {
			continue // ambiguous resolution: the load gate rejects it, so it is not a usable choice
		}
		entries = append(entries, pcaAutonomyCatalogEntry{Family: res.family, Identifier: identifier})
	}
	return entries, capped, nil
}

// renderPCAAutonomyCatalogAppendix renders the full-access catalog appendix
// for the current PCA state. Families are grouped so a task-scoped choice
// (an EQ request reads the static_eq line) does not need the whole list.
func renderPCAAutonomyCatalogAppendix(entries []pcaAutonomyCatalogEntry, capped bool, readErr error) string {
	var builder strings.Builder
	builder.WriteString("\n\n")
	builder.WriteString(pcaAutonomyCatalogAppendixHeading)
	builder.WriteString("\n")
	if readErr != nil {
		builder.WriteString("- admission catalog unavailable: ")
		builder.WriteString(readErr.Error())
		builder.WriteString("; do not guess identifiers — an autonomous load attempt would fail the admission gate")
		return builder.String()
	}

	families := map[string][]string{}
	for _, entry := range entries {
		family := entry.Family
		if family == "" {
			family = "unknown_family"
		}
		families[family] = append(families[family], entry.Identifier)
	}
	if len(families) == 0 {
		builder.WriteString("- none: no processor currently holds a promoted, fingerprint-matched admission; do not guess identifiers for rack.add_node / plugin.load_to_rack")
		return builder.String()
	}
	familyOrder := make([]string, 0, len(families))
	for family := range families {
		familyOrder = append(familyOrder, family)
	}
	sort.Strings(familyOrder)
	for _, family := range familyOrder {
		identifiers := append([]string(nil), families[family]...)
		sort.Strings(identifiers)
		extra := 0
		if len(identifiers) > pcaAutonomyCatalogFamilyLimit {
			extra = len(identifiers) - pcaAutonomyCatalogFamilyLimit
			identifiers = identifiers[:pcaAutonomyCatalogFamilyLimit]
		}
		builder.WriteString("- ")
		builder.WriteString(family)
		builder.WriteString(": ")
		builder.WriteString(strings.Join(identifiers, ", "))
		if extra > 0 {
			builder.WriteString(fmt.Sprintf(" (+%d more)", extra))
		}
		builder.WriteString("\n")
	}
	if capped {
		builder.WriteString(fmt.Sprintf("- listing capped at %d fingerprint-verified admissions; more promoted records exist but were not verified this turn", pcaAutonomyCatalogVerifyLimit))
	}
	return builder.String()
}

// appendFullAccessAdmittedCatalog is the disclosure gate for the tool-catalog
// text: an explicit full-access turn gets the current admitted catalog
// appended; every other turn gets the summary back byte-identical.
func appendFullAccessAdmittedCatalog(summary string, requestContext map[string]any) string {
	if !explicitFullAccessRequestContext(requestContext) {
		return summary
	}
	entries, capped, err := collectPCAAutonomyCatalogEntries()
	return summary + renderPCAAutonomyCatalogAppendix(entries, capped, err)
}
