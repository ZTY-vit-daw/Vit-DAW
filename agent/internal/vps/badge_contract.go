package vps

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/spal"
)

// BadgeContractSchemaVersion is the machine-readable authoring contract for a
// routeable task badge.  It deliberately sits between the taxonomy document
// and a plug-in VPS: the contract defines vendor-neutral actions and audit
// requirements, while a VPS supplies the plug-in-specific implementation.
const BadgeContractSchemaVersion = "vit.vps.badge_contract.v1"

const (
	BadgeFeatureStatusUnsupported  = "unsupported"
	BadgeFeatureStatusStagingReady = "staging_ready"
	BadgeFeatureStatusConformed    = "conformed"
)

// BadgeAuditInstruction is one required authoring proof step for an action.
// The sequence is fixed by the badge, not improvised per plug-in.
type BadgeAuditInstruction struct {
	ID          string `json:"id"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// BadgeActionContract defines one canonical SPAL action a plug-in can
// implement for a task badge.  Feature IDs describe optional rows of the
// per-plug-in feature matrix; they are never independent routeable badges.
type BadgeActionContract struct {
	ID                 string                  `json:"id"`
	SchemaID           string                  `json:"schema_id"`
	RequiredFeatures   []string                `json:"required_features,omitempty"`
	OptionalFeatures   []string                `json:"optional_features,omitempty"`
	AuthoringAudit     []BadgeAuditInstruction `json:"authoring_audit"`
	RuntimeTransaction []string                `json:"runtime_transaction"`
	Description        string                  `json:"description,omitempty"`
}

// BadgeContract is the fixed functional grammar and audit table for one
// routeable task badge.  It contains no vendor parameter IDs and cannot by
// itself grant a Credential or Catalog entry.
type BadgeContract struct {
	SchemaVersion     string                `json:"schema_version"`
	BadgeID           string                `json:"badge_id"`
	Version           string                `json:"version"`
	CategoryID        string                `json:"category_id"`
	Actions           []BadgeActionContract `json:"actions"`
	AdmissionEvidence []string              `json:"admission_evidence"`
}

// BadgeFeatureMatrixEntry is the VPS-side status of one feature row under a
// badge action.  A conformed row is eligible for Catalog-backed selection only
// after the enclosing Credential is verified; staging_ready is explicit local
// testing authority only.
type BadgeFeatureMatrixEntry struct {
	ActionID     string   `json:"action_id"`
	FeatureID    string   `json:"feature_id"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// VPSActionImplementation is the plug-in-specific counterpart to a badge
// action. BindingRef is intentionally opaque here: concrete SPAL adapters own
// physical parameter transforms, while this record keeps the action contract,
// feature coverage and evidence together for Forge and Library review.
type VPSActionImplementation struct {
	BadgeID      string                    `json:"badge_id"`
	BadgeVersion string                    `json:"badge_version"`
	ActionID     string                    `json:"action_id"`
	SchemaID     string                    `json:"schema_id"`
	Status       string                    `json:"status"`
	BindingRef   string                    `json:"binding_ref"`
	Features     []BadgeFeatureMatrixEntry `json:"features,omitempty"`
	EvidenceRefs []string                  `json:"evidence_refs,omitempty"`
}

func EqualizerV2BadgeContract() BadgeContract {
	audit := []BadgeAuditInstruction{
		{ID: "capture_preimage", Required: true, Description: "Capture a complete fresh parameter preimage before any write."},
		{ID: "write_and_fresh_readback", Required: true, Description: "Write every physical control and verify fresh host readback."},
		{ID: "state_roundtrip", Required: true, Description: "Verify state serialization/reload/restoration for the action state."},
		{ID: "behavior_probe", Required: true, Description: "Measure the action with a bounded offline behavior or FXM probe."},
		{ID: "rollback_verify", Required: true, Description: "Restore the preimage and verify a fresh rollback readback."},
	}
	runtime := []string{"capture_preimage", "compile_action", "write", "fresh_readback", "retain_postimage", "guarded_manual_rollback"}
	return BadgeContract{
		SchemaVersion: BadgeContractSchemaVersion,
		BadgeID:       EqualizerCapabilityID,
		Version:       EqualizerProfileVersion,
		CategoryID:    "spectral_processing",
		Actions: []BadgeActionContract{
			{
				ID:               "eq.static_band.patch",
				SchemaID:         spal.EQBandPatchControlID,
				RequiredFeatures: []string{"static_band", "bell"},
				OptionalFeatures: []string{"low_shelf", "high_shelf"},
				AuthoringAudit:   cloneBadgeAudit(audit), RuntimeTransaction: append([]string(nil), runtime...),
				Description: "Patch one static parametric EQ band using frequency, gain, Q and a response shape.",
			},
			{
				ID:       "eq.pass_filter.patch",
				SchemaID: spal.EQPassFilterPatchControlID,
				// Slope availability is deliberately value-specific. A plug-in that
				// has only been evidenced at 12 dB/oct must not silently satisfy a
				// request for 24 dB/oct merely because both expose a control called
				// "Slope".
				OptionalFeatures: []string{"highpass", "lowpass", "slope_12_db_per_octave", "slope_24_db_per_octave"},
				AuthoringAudit:   cloneBadgeAudit(audit), RuntimeTransaction: append([]string(nil), runtime...),
				Description: "Apply a semantic high-pass or low-pass filter; a VPS may implement it through a dedicated filter or an allocated EQ band.",
			},
			{
				ID:               "eq.output.patch",
				SchemaID:         spal.EQOutputPatchControlID,
				OptionalFeatures: []string{"bypass", "output_gain", "dry_mix"},
				AuthoringAudit:   cloneBadgeAudit(audit), RuntimeTransaction: append([]string(nil), runtime...),
				Description: "Patch a conformed EQ output control.",
			},
		},
		AdmissionEvidence: []string{"human_semantic_confirmation", "write_readback", "state_roundtrip", "behavior_probe", "rollback", "verified_credential"},
	}
}

// BadgeContractFor is the single registry seam for routeable badge contracts.
// The taxonomy may describe more badges, but only contracts registered here
// can be used to compile an implementation or validate an admission package.
func BadgeContractFor(badgeID string) (BadgeContract, bool) {
	switch strings.TrimSpace(badgeID) {
	case EqualizerCapabilityID:
		return EqualizerV2BadgeContract(), true
	default:
		return BadgeContract{}, false
	}
}

func (c BadgeContract) Action(id string) (BadgeActionContract, bool) {
	for _, action := range c.Actions {
		if action.ID == strings.TrimSpace(id) {
			return cloneBadgeAction(action), true
		}
	}
	return BadgeActionContract{}, false
}

func (c BadgeContract) Validate() error {
	if c.SchemaVersion != BadgeContractSchemaVersion || strings.TrimSpace(c.BadgeID) == "" || strings.TrimSpace(c.Version) == "" || strings.TrimSpace(c.CategoryID) == "" {
		return fmt.Errorf("badge contract requires schema version, badge id, version and category")
	}
	if len(c.Actions) == 0 {
		return fmt.Errorf("badge contract %s has no actions", c.BadgeID)
	}
	seenActions := map[string]bool{}
	for _, action := range c.Actions {
		action.ID = strings.TrimSpace(action.ID)
		if action.ID == "" || seenActions[action.ID] {
			return fmt.Errorf("badge contract %s has an empty or duplicate action id", c.BadgeID)
		}
		seenActions[action.ID] = true
		if _, known := spal.Schema(action.SchemaID); !known {
			return fmt.Errorf("badge action %s has unknown SPAL schema %s", action.ID, action.SchemaID)
		}
		if len(action.AuthoringAudit) == 0 || len(action.RuntimeTransaction) == 0 {
			return fmt.Errorf("badge action %s requires authoring and runtime sequences", action.ID)
		}
		seenFeatures := map[string]bool{}
		for _, feature := range append(append([]string(nil), action.RequiredFeatures...), action.OptionalFeatures...) {
			feature = strings.TrimSpace(feature)
			if feature == "" || seenFeatures[feature] {
				return fmt.Errorf("badge action %s has an empty or duplicate feature", action.ID)
			}
			seenFeatures[feature] = true
		}
	}
	return nil
}

func (implementation VPSActionImplementation) ValidateAgainst(contract BadgeContract) error {
	if err := contract.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(implementation.BadgeID) != contract.BadgeID || strings.TrimSpace(implementation.BadgeVersion) != contract.Version {
		return fmt.Errorf("VPS action implementation does not match badge contract %s@%s", contract.BadgeID, contract.Version)
	}
	action, found := contract.Action(implementation.ActionID)
	if !found || implementation.SchemaID != action.SchemaID {
		return fmt.Errorf("VPS action implementation has no matching badge action %s", implementation.ActionID)
	}
	if strings.TrimSpace(implementation.BindingRef) == "" {
		return fmt.Errorf("VPS action implementation %s requires a binding reference", implementation.ActionID)
	}
	switch implementation.Status {
	case BadgeFeatureStatusUnsupported, BadgeFeatureStatusStagingReady, BadgeFeatureStatusConformed:
	default:
		return fmt.Errorf("VPS action implementation %s has unknown status %s", implementation.ActionID, implementation.Status)
	}
	allowedFeatures := map[string]bool{}
	for _, featureID := range append(append([]string(nil), action.RequiredFeatures...), action.OptionalFeatures...) {
		allowedFeatures[featureID] = true
	}
	featureStatus := map[string]string{}
	for _, feature := range implementation.Features {
		featureID := strings.TrimSpace(feature.FeatureID)
		if feature.ActionID != implementation.ActionID || featureID == "" || !allowedFeatures[featureID] || featureStatus[featureID] != "" {
			return fmt.Errorf("VPS action implementation %s has invalid feature matrix entries", implementation.ActionID)
		}
		switch feature.Status {
		case BadgeFeatureStatusUnsupported, BadgeFeatureStatusStagingReady, BadgeFeatureStatusConformed:
		default:
			return fmt.Errorf("VPS action implementation %s has unknown feature status %s", implementation.ActionID, feature.Status)
		}
		featureStatus[featureID] = feature.Status
	}
	for _, required := range action.RequiredFeatures {
		if featureStatus[required] == "" {
			return fmt.Errorf("VPS action implementation %s omits required feature %s", implementation.ActionID, required)
		}
		if implementation.Status == BadgeFeatureStatusConformed && featureStatus[required] != BadgeFeatureStatusConformed {
			return fmt.Errorf("conformed VPS action implementation %s has unconformed required feature %s", implementation.ActionID, required)
		}
	}
	return nil
}

// SupportsRequiredFeatures reports whether this one implementation can take a
// particular action request. It never infers a parent feature from a child
// parameter name: every requested feature must have an explicit matrix row at
// the requested minimum status.
func (implementation VPSActionImplementation) SupportsRequiredFeatures(required []string, minimumStatus string) bool {
	order := map[string]int{BadgeFeatureStatusUnsupported: 0, BadgeFeatureStatusStagingReady: 1, BadgeFeatureStatusConformed: 2}
	minimum, known := order[minimumStatus]
	if !known || order[implementation.Status] < minimum {
		return false
	}
	features := map[string]string{}
	for _, feature := range implementation.Features {
		features[strings.TrimSpace(feature.FeatureID)] = feature.Status
	}
	for _, featureID := range required {
		if order[features[strings.TrimSpace(featureID)]] < minimum {
			return false
		}
	}
	return true
}

func cloneBadgeAudit(in []BadgeAuditInstruction) []BadgeAuditInstruction {
	return append([]BadgeAuditInstruction(nil), in...)
}

func cloneBadgeAction(in BadgeActionContract) BadgeActionContract {
	out := in
	out.RequiredFeatures = append([]string(nil), in.RequiredFeatures...)
	out.OptionalFeatures = append([]string(nil), in.OptionalFeatures...)
	out.AuthoringAudit = cloneBadgeAudit(in.AuthoringAudit)
	out.RuntimeTransaction = append([]string(nil), in.RuntimeTransaction...)
	sort.Strings(out.RequiredFeatures)
	sort.Strings(out.OptionalFeatures)
	return out
}
