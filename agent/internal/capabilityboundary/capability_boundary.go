// Package capabilityboundary renders the structured capability_boundary.v1
// remediation payload for capability-blocked responses (CAP-BOUNDARY-1).
//
// The resolver is a pure function over a fact snapshot assembled by the
// caller: zero LLM calls, zero I/O, zero gate mutations. It may only order,
// filter, and phrase facts that were supplied to it — when a remediation
// class has no facts, the class is omitted and the reason is disclosed in
// UnavailableReasons. It never invents a candidate, a family, or a fallback,
// and it never emits an approximate write (L5 is out of scope by design).
package capabilityboundary

import "strings"

const SchemaVersion = "capability_boundary.v1"

// Boundary classes carried on the Class field.
const (
	ClassFreeState      = "free_state"
	ClassDynamicControl = "c2_dynamic_control"
)

// NextAction kinds. v1 has exactly these five kinds; the model may explain or
// choose among supplied actions but must never invent a new kind.
const (
	KindAlternateProcessor = "alternate_processor"
	KindNativeProcessor    = "native_processor"
	KindManualGuidance     = "manual_guidance"
	KindOpenPluginUI       = "open_plugin_ui"
	KindCertificationEntry = "certification_entry"
)

// Read-only families have no PCA controller vocabulary in this build
// (spectral_dynamics is inspect-only; clipper is a separate boundary). For
// these families the resolver offers no processor substitution and no native
// processor action — manual guidance and (where a runner exists) the
// certification entry remain.
var readOnlyFamilies = map[string]bool{
	"spectral_dynamics": true,
	"clipper":           true,
}

// adjacentFamilies is the deterministic acoustic-domain adjacency used to rank
// adjacent-family alternates. Order inside the slice is the rank order.
var adjacentFamilies = map[string][]string{
	"broadband_compressor": {"limiter", "multiband_dynamics", "gate_expander", "transient_shaper"},
	"limiter":              {"broadband_compressor", "multiband_dynamics"},
	"gate_expander":        {"broadband_compressor", "transient_shaper"},
	"de_esser":             {"multiband_dynamics", "static_eq"},
	"transient_shaper":     {"broadband_compressor", "gate_expander"},
	"multiband_dynamics":   {"broadband_compressor", "limiter", "de_esser"},
	"static_eq":            {"de_esser", "multiband_dynamics"},
	"reverb":               {"delay"},
	"delay":                {"reverb"},
}

// MaxAlternateActions caps the deterministic alternate list after the stable
// sort, so a large installed base cannot flood the boundary response.
const MaxAlternateActions = 5

// Candidate is one PCA-admitted alternate processor fact. The wiring layer
// obtains these exclusively through the existing PCA admission/coverage
// queries; the resolver does not re-run or relax any gate.
type Candidate struct {
	Key               string
	Family            string
	FamilyRank        int // 0 = same family, 1..n = adjacency rank
	Identifier        string
	PluginPath        string
	PluginName        string
	Manufacturer      string
	SubjectKey        string
	BinaryFingerprint string
	AttestationID     string
	CoverageAxes      []string
	CoverageLevel     string
}

// InstanceRef is one loaded plugin instance on the boundary's target track,
// used for manual guidance (parameter surface) and open-plugin-ui actions.
type InstanceRef struct {
	TrackID    string
	TrackName  string
	PluginID   string
	PluginName string
	Family     string
	Params     []ParamRef
}

// ParamRef is one live context pack parameter row for manual guidance: role,
// param id, range, current value, and the expected direction. Descriptive
// only — no write is ever derived from it.
type ParamRef struct {
	Role         string
	ParamID      string
	Range        string
	CurrentValue string
	Expected     string
}

// NativeOption describes the ungated native path for the family. v1 has a
// single product fact: the DAW manual loading path calls the native rack
// service directly and is not subject to the agent PCA load gate
// (docs/PCA_PLUGIN_LOAD_GATE_V1.md). The wiring layer decides whether a
// native row exists for the boundary at all; an absent row is disclosed, not
// fabricated.
type NativeOption struct {
	Family      string
	Label       string
	Description string
}

// CertificationEntry is the L4 entry pointer: the read-only candidate listing
// and the consent-gated start endpoint of the in-product certification flow.
// v1 exposes the entry only — the flow internals stay untouched.
type CertificationEntry struct {
	Family       string
	Endpoint     string
	ConsentToken string
	Note         string
}

// Input is the deterministic fact snapshot for one capability-blocked turn.
type Input struct {
	Code          string
	Class         string
	Family        string
	Reason        string
	EvidenceRefs  []string
	BlockedBy     []string
	TrackID       string
	TrackName     string
	Candidates    []Candidate
	Instances     []InstanceRef
	Native        *NativeOption
	Certification *CertificationEntry
}

// NextAction is one deterministic remediation option. None of the kinds
// performs or proposes a parameter write; RequiresConfirmation marks the
// actions whose follow-up needs an explicit user confirmation (plugin load
// selection, certification consent).
type NextAction struct {
	Kind                 string         `json:"kind"`
	Key                  string         `json:"key"`
	Rank                 int            `json:"rank"`
	Title                string         `json:"title"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	RiskLevel            string         `json:"risk_level"`
	ExpectedEffect       string         `json:"expected_effect"`
	Tradeoff             string         `json:"tradeoff"`
	Evidence             map[string]any `json:"evidence,omitempty"`
	Target               map[string]any `json:"target,omitempty"`
	ManualParams         []ParamRef     `json:"manual_params,omitempty"`
}

// Boundary is the capability_boundary.v1 payload embedded on
// ChatResponse.WorkflowData under the key "capability_boundary".
type Boundary struct {
	SchemaVersion      string       `json:"schema_version"`
	Code               string       `json:"code"`
	Class              string       `json:"class"`
	Family             string       `json:"family,omitempty"`
	TrackID            string       `json:"track_id,omitempty"`
	Reason             string       `json:"reason"`
	EvidenceRefs       []string     `json:"evidence_refs,omitempty"`
	BlockedBy          []string     `json:"blocked_by,omitempty"`
	MutationPerformed  bool         `json:"mutation_performed"`
	NextActions        []NextAction `json:"next_actions"`
	UnavailableReasons []string     `json:"unavailable_reasons,omitempty"`
}

// AdjacentFamilies returns the adjacency list for a family (nil when unknown).
func AdjacentFamilies(family string) []string {
	return adjacentFamilies[NormalizeFamily(family)]
}

// IsReadOnlyFamily reports whether the family has no controller vocabulary.
func IsReadOnlyFamily(family string) bool {
	return readOnlyFamilies[NormalizeFamily(family)]
}

// NormalizeFamily canonicalizes family identifiers the wiring may receive in
// either the processorintent spelling or the attestation spelling.
func NormalizeFamily(family string) string {
	switch family {
	case "transient_shaper", "transient":
		return "transient_shaper"
	case "multiband_dynamics", "multiband":
		return "multiband_dynamics"
	case "static_eq", "eq":
		return "static_eq"
	default:
		return family
	}
}

// FamilyFromImprovementActionDomain maps an improvement_proposal action_domain
// (agentprotocol's frozen enumeration) onto the boundary family vocabulary
// (M3-FS-GATE-EVIDENCE-WIRE-1 A2: a capability-blocked turn whose decision
// carries only an improvement proposal — no SemanticProcessorIntent, no
// ProcessorType — previously resolved family as "" and reported
// family_unresolved even though the proposal's own domain names the family).
// Plugin-owned domains map to their attestation family; the native parameter
// domains map to their own non-plugin families (volume/pan): no PCA candidate,
// native rack path, or certification runner exists for them, so the resolver
// discloses those classes as honestly absent instead of reporting the family
// itself as unresolved. Domains that name no single family (plugin,
// section_automation) stay "" — an unresolved family stays fail-closed.
func FamilyFromImprovementActionDomain(domain string) string {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "track_gain", "clip_gain":
		return "volume"
	case "pan":
		return "pan"
	case "eq", "static_eq":
		return "static_eq"
	case "broadband_compression", "compressor":
		return "broadband_compressor"
	case "limiter":
		return "limiter"
	case "gate_expander":
		return "gate_expander"
	case "de_esser":
		return "de_esser"
	case "transient_shaper":
		return "transient_shaper"
	case "multiband_dynamics":
		return "multiband_dynamics"
	case "reverb":
		return "reverb"
	case "delay":
		return "delay"
	default:
		return ""
	}
}
