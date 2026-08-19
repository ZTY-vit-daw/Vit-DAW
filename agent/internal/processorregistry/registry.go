// Package processorregistry is a data-only registry for governed processor
// families. It intentionally has no keyword, phrase, example, or vendor
// matching API.
package processorregistry

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
)

type Definition struct {
	Family             string   `json:"family"`
	PCAFamily          string   `json:"pca_family"`
	Recognizer         string   `json:"recognizer"`
	CoverageVocabulary []string `json:"coverage_vocabulary"`
	// CoverageProofs maps model-owned semantic coverage keys to exact PCA
	// coverage entries. It contains no plugin, vendor, or parameter mapping.
	CoverageProofs   map[string][]processorattestation.Coverage `json:"coverage_proofs,omitempty"`
	Planner          string                                     `json:"planner"`
	Materializer     string                                     `json:"materializer"`
	TypedExecutor    string                                     `json:"typed_executor"`
	ReceiptProjector string                                     `json:"receipt_projector"`
	ObservationViews []string                                   `json:"observation_views"`
	InspectOnly      bool                                       `json:"inspect_only"`
}

type Registry struct{ definitions map[string]Definition }

func New(definitions ...Definition) (*Registry, error) {
	r := &Registry{definitions: map[string]Definition{}}
	for _, definition := range definitions {
		if err := r.Register(definition); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(definition Definition) error {
	if r == nil {
		return fmt.Errorf("nil processor registry")
	}
	definition.Family = strings.ToLower(strings.TrimSpace(definition.Family))
	definition.PCAFamily = strings.ToLower(strings.TrimSpace(definition.PCAFamily))
	if !processorintent.IsSupportedFamily(definition.Family) {
		return fmt.Errorf("unsupported processor family %q", definition.Family)
	}
	if definition.PCAFamily == "" {
		return fmt.Errorf("processor family %s requires pca_family", definition.Family)
	}
	if definition.Recognizer == "" || definition.Planner == "" || definition.Materializer == "" || definition.TypedExecutor == "" || definition.ReceiptProjector == "" {
		return fmt.Errorf("processor family %s requires all deterministic component identifiers", definition.Family)
	}
	if len(definition.CoverageVocabulary) == 0 || len(definition.ObservationViews) == 0 {
		return fmt.Errorf("processor family %s requires coverage and observation vocabulary", definition.Family)
	}
	if definition.Family == processorintent.FamilySpectralDynamics || definition.Family == processorintent.FamilyClipper {
		if !definition.InspectOnly {
			return fmt.Errorf("boundary family %s must be inspect_only", definition.Family)
		}
	}
	if definition.Family != processorintent.FamilySpectralDynamics && definition.Family != processorintent.FamilyClipper && definition.InspectOnly {
		return fmt.Errorf("executable family %s cannot be inspect_only", definition.Family)
	}
	if definition.InspectOnly {
		if definition.PCAFamily != "inspect_only" {
			return fmt.Errorf("inspect-only family %s cannot share executable PCA family %s", definition.Family, definition.PCAFamily)
		}
		if definition.Planner != "inspect_only" || definition.Materializer != "none" || definition.TypedExecutor != "none" {
			return fmt.Errorf("inspect-only family %s cannot register an executable planner, materializer, or typed executor", definition.Family)
		}
	} else if definition.PCAFamily == "inspect_only" || definition.PCAFamily != definition.Family {
		return fmt.Errorf("executable family %s must use its own PCA family", definition.Family)
	} else if !supportedPCAFamily(definition.PCAFamily) {
		return fmt.Errorf("executable family %s has unsupported PCA family %s", definition.Family, definition.PCAFamily)
	}
	definition.CoverageVocabulary = uniqueSorted(definition.CoverageVocabulary)
	definition.ObservationViews = uniqueSorted(definition.ObservationViews)
	definition.CoverageProofs = normalizeCoverageProofs(definition.CoverageProofs)
	for semantic, proofs := range definition.CoverageProofs {
		if !containsString(definition.CoverageVocabulary, semantic) {
			return fmt.Errorf("processor family %s has a coverage proof for undeclared semantic axis %s", definition.Family, semantic)
		}
		if len(proofs) == 0 {
			return fmt.Errorf("processor family %s has an empty coverage proof for %s", definition.Family, semantic)
		}
		for _, proof := range proofs {
			if definition.PCAFamily == "inspect_only" {
				return fmt.Errorf("inspect-only family %s cannot declare PCA coverage proofs", definition.Family)
			}
			if err := validatePCACoverage(definition.PCAFamily, proof); err != nil {
				return fmt.Errorf("processor family %s semantic axis %s: %w", definition.Family, semantic, err)
			}
		}
	}
	if _, exists := r.definitions[definition.Family]; exists {
		return fmt.Errorf("processor family %s already registered", definition.Family)
	}
	r.definitions[definition.Family] = definition
	return nil
}

func (r *Registry) Resolve(family string) (Definition, bool) {
	if r == nil {
		return Definition{}, false
	}
	definition, ok := r.definitions[strings.ToLower(strings.TrimSpace(family))]
	return definition, ok
}

func (r *Registry) List() []Definition {
	if r == nil {
		return nil
	}
	out := make([]Definition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definition.CoverageVocabulary = append([]string(nil), definition.CoverageVocabulary...)
		definition.ObservationViews = append([]string(nil), definition.ObservationViews...)
		definition.CoverageProofs = normalizeCoverageProofs(definition.CoverageProofs)
		out = append(out, definition)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Family < out[j].Family })
	return out
}

func (r *Registry) ValidateCoverage(family string, coverage []string) error {
	definition, ok := r.Resolve(family)
	if !ok {
		return fmt.Errorf("processor family %q is not registered", family)
	}
	allowed := map[string]bool{}
	for _, axis := range definition.CoverageVocabulary {
		allowed[axis] = true
	}
	seen := map[string]bool{}
	for _, raw := range coverage {
		axis := strings.TrimSpace(raw)
		if axis == "" || !allowed[axis] {
			return fmt.Errorf("coverage axis %q is not registered for family %s", raw, definition.Family)
		}
		if seen[axis] {
			return fmt.Errorf("coverage axis %q is duplicated", axis)
		}
		seen[axis] = true
	}
	return nil
}

// PCARequiredCoverage converts model-owned semantic coverage keys into exact
// PCA entries. It never adds a missing semantic key or chooses a default.
func (r *Registry) PCARequiredCoverage(family string, coverage []string) ([]processorattestation.Coverage, error) {
	definition, ok := r.Resolve(family)
	if !ok {
		return nil, fmt.Errorf("processor family %q is not registered", family)
	}
	if len(coverage) == 0 {
		return nil, fmt.Errorf("required coverage cannot be empty")
	}
	if err := r.ValidateCoverage(definition.Family, coverage); err != nil {
		return nil, err
	}
	result := make([]processorattestation.Coverage, 0, len(coverage))
	seen := map[string]bool{}
	for _, raw := range coverage {
		semantic := strings.ToLower(strings.TrimSpace(raw))
		proofs := definition.CoverageProofs[semantic]
		if len(proofs) == 0 {
			return nil, fmt.Errorf("coverage axis %q has no PCA proof for family %s", semantic, definition.Family)
		}
		for _, proof := range proofs {
			key := coverageKey(proof)
			if !seen[key] {
				seen[key] = true
				result = append(result, proof)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return coverageKey(result[i]) < coverageKey(result[j]) })
	return result, nil
}

func Default() (*Registry, error) {
	return New(
		// Static EQ's semantic axes do not identify an operation shape. Keeping
		// this proof table empty is intentional: a later typed EQ action must
		// supply its exact PCA action+shape rather than letting free-state intent
		// widen or guess a PCA certificate.
		Definition{Family: processorintent.FamilyStaticEQ, PCAFamily: processorattestation.FamilyStaticEQ, Recognizer: "plugingrabber.eq_band_summary", CoverageVocabulary: []string{"frequency", "gain", "q", "shape"}, Planner: "semantic_eq", Materializer: "semantic_eq_materializer", TypedExecutor: "plugin_grabber.apply_eq_edits", ReceiptProjector: "semantic_eq_receipt", ObservationViews: []string{"track.timbre_frequency"}},
		Definition{Family: processorintent.FamilyBroadbandCompressor, PCAFamily: processorattestation.FamilyBroadbandCompressor, Recognizer: "plugingrabber.compressor_topology", CoverageVocabulary: []string{"activation_intensity", "transfer_severity", "transient_timing", "recovery_motion", "detector_focus", "output_normalization", "parallel_balance", "character"}, CoverageProofs: identityV1CoverageProofs([]string{"activation_intensity", "transfer_severity", "transient_timing", "recovery_motion", "detector_focus", "output_normalization", "parallel_balance", "character"}, "adjust"), Planner: "semantic_compressor", Materializer: "semantic_compressor_materializer", TypedExecutor: "plugin_grabber.apply_compressor_controls", ReceiptProjector: "semantic_compressor_receipt", ObservationViews: []string{"track.time_dynamics"}},
		Definition{Family: processorintent.FamilyLimiter, PCAFamily: processorattestation.FamilyLimiter, Recognizer: "plugingrabber.limiter_topology", CoverageVocabulary: append(processorattestation.V2CoverageAxes(processorattestation.FamilyLimiter), "threshold", "ceiling", "release", "lookahead", "output_gain"), CoverageProofs: v2SemanticCoverageProofs(processorattestation.FamilyLimiter, map[string]string{"threshold": "protection_intensity", "ceiling": "output_ceiling", "release": "recovery_motion", "lookahead": "detector_latency", "output_gain": "output_normalization"}), Planner: "semantic_limiter", Materializer: "limiter_materializer", TypedExecutor: "plugin_grabber.apply_limiter_controls", ReceiptProjector: "limiter_receipt", ObservationViews: []string{"track.peak_structure"}},
		Definition{Family: processorintent.FamilyGateExpander, PCAFamily: processorattestation.FamilyGateExpander, Recognizer: "plugingrabber.gate_expander_topology", CoverageVocabulary: append(processorattestation.V2CoverageAxes(processorattestation.FamilyGateExpander), "threshold", "range", "frequency_focus", "direction", "attack", "hold", "release", "output_gain", "mix"), CoverageProofs: v2SemanticCoverageProofs(processorattestation.FamilyGateExpander, map[string]string{"threshold": "activation_threshold", "range": "attenuation_floor", "frequency_focus": "detector_focus", "direction": "direction_mode", "attack": "state_timing", "hold": "state_timing", "release": "state_timing", "output_gain": "output_normalization", "mix": "parallel_balance"}), Planner: "semantic_gate_expander", Materializer: "gate_expander_materializer", TypedExecutor: "plugin_grabber.apply_gate_expander_controls", ReceiptProjector: "gate_expander_receipt", ObservationViews: []string{"track.activity_structure", "track.time_dynamics"}},
		Definition{Family: processorintent.FamilyDeEsser, PCAFamily: processorattestation.FamilyDeEsser, Recognizer: "plugingrabber.de_esser_topology", CoverageVocabulary: append(processorattestation.V2CoverageAxes(processorattestation.FamilyDeEsser), "threshold", "frequency_focus", "range", "attack", "release", "output_gain", "mix"), CoverageProofs: v2SemanticCoverageProofs(processorattestation.FamilyDeEsser, map[string]string{"threshold": "threshold_sensitivity", "frequency_focus": "detector_focus", "range": "sibilance_reduction", "attack": "recovery_motion", "release": "recovery_motion", "output_gain": "output_normalization", "mix": "parallel_balance"}), Planner: "semantic_de_esser", Materializer: "de_esser_materializer", TypedExecutor: "plugin_grabber.apply_de_esser_controls", ReceiptProjector: "de_esser_receipt", ObservationViews: []string{"track.frequency_time_events"}},
		Definition{Family: processorintent.FamilyTransientShaper, PCAFamily: processorattestation.FamilyTransient, Recognizer: "plugingrabber.transient_shaper_topology", CoverageVocabulary: append(processorattestation.V2CoverageAxes(processorattestation.FamilyTransient), "attack", "sustain", "duration", "frequency_focus", "mode", "output_gain", "mix"), CoverageProofs: v2SemanticCoverageProofs(processorattestation.FamilyTransient, map[string]string{"attack": "envelope_emphasis", "sustain": "envelope_emphasis", "duration": "envelope_timing", "frequency_focus": "detector_focus", "mode": "shape_mode", "output_gain": "output_normalization", "mix": "parallel_balance"}), Planner: "semantic_transient_shaper", Materializer: "transient_shaper_materializer", TypedExecutor: "plugin_grabber.apply_transient_shaper_controls", ReceiptProjector: "transient_receipt", ObservationViews: []string{"track.transient_structure"}},
		Definition{Family: processorintent.FamilyMultibandDynamics, PCAFamily: processorattestation.FamilyMultiband, Recognizer: "plugingrabber.multiband_topology", CoverageVocabulary: append(processorattestation.V2CoverageAxes(processorattestation.FamilyMultiband), "threshold", "range", "ratio", "attack", "hold", "release", "crossover", "frequency_focus", "output_gain", "mix"), CoverageProofs: v2SemanticCoverageProofs(processorattestation.FamilyMultiband, map[string]string{"threshold": "band_dynamics", "range": "band_dynamics", "ratio": "band_dynamics", "attack": "band_timing", "hold": "band_timing", "release": "band_timing", "crossover": "crossover_layout", "frequency_focus": "detector_focus", "output_gain": "output_normalization", "mix": "parallel_balance"}), Planner: "semantic_multiband", Materializer: "multiband_materializer", TypedExecutor: "plugin_grabber.apply_multiband_controls", ReceiptProjector: "multiband_receipt", ObservationViews: []string{"track.band_dynamics"}},
		Definition{Family: processorintent.FamilySpectralDynamics, PCAFamily: "inspect_only", Recognizer: "plugingrabber.spectral_dynamics_topology", CoverageVocabulary: []string{"inspect_only"}, Planner: "inspect_only", Materializer: "none", TypedExecutor: "none", ReceiptProjector: "spectral_dynamics_receipt", ObservationViews: []string{"track.frequency_time_events"}, InspectOnly: true},
		Definition{Family: processorintent.FamilyClipper, PCAFamily: "inspect_only", Recognizer: "plugingrabber.clipper_boundary", CoverageVocabulary: []string{"inspect_only"}, Planner: "inspect_only", Materializer: "none", TypedExecutor: "none", ReceiptProjector: "clipper_boundary_receipt", ObservationViews: []string{"track.peak_structure"}, InspectOnly: true},
	)
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

func normalizeCoverageProofs(values map[string][]processorattestation.Coverage) map[string][]processorattestation.Coverage {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]processorattestation.Coverage, len(values))
	for rawSemantic, proofs := range values {
		semantic := strings.ToLower(strings.TrimSpace(rawSemantic))
		if semantic == "" {
			continue
		}
		seen := map[string]bool{}
		for _, raw := range proofs {
			proof := processorattestation.Coverage{
				Action: strings.ToLower(strings.TrimSpace(raw.Action)),
				Shape:  strings.ToLower(strings.TrimSpace(raw.Shape)),
				Axis:   strings.ToLower(strings.TrimSpace(raw.Axis)),
			}
			key := coverageKey(proof)
			if key != "\x00\x00" && !seen[key] {
				seen[key] = true
				out[semantic] = append(out[semantic], proof)
			}
		}
		sort.Slice(out[semantic], func(i, j int) bool { return coverageKey(out[semantic][i]) < coverageKey(out[semantic][j]) })
	}
	return out
}

func coverageKey(value processorattestation.Coverage) string {
	return strings.ToLower(strings.TrimSpace(value.Action)) + "\x00" +
		strings.ToLower(strings.TrimSpace(value.Shape)) + "\x00" +
		strings.ToLower(strings.TrimSpace(value.Axis))
}

func validatePCACoverage(family string, coverage processorattestation.Coverage) error {
	family = strings.ToLower(strings.TrimSpace(family))
	if family == "inspect_only" {
		return fmt.Errorf("inspect-only PCA family cannot prove executable coverage")
	}
	if processorattestation.IsV2Family(family) {
		return processorattestation.ValidateV2Coverage(family, coverage)
	}
	return processorattestation.ValidateCoverage(family, coverage)
}

func supportedPCAFamily(family string) bool {
	family = strings.ToLower(strings.TrimSpace(family))
	return family == processorattestation.FamilyStaticEQ ||
		family == processorattestation.FamilyBroadbandCompressor ||
		processorattestation.IsV2Family(family)
}

func identityV1CoverageProofs(axes []string, action string) map[string][]processorattestation.Coverage {
	result := make(map[string][]processorattestation.Coverage, len(axes))
	for _, axis := range axes {
		axis = strings.ToLower(strings.TrimSpace(axis))
		if axis == "" {
			continue
		}
		result[axis] = []processorattestation.Coverage{{Action: action, Axis: axis}}
	}
	return result
}

func v2SemanticCoverageProofs(family string, aliases map[string]string) map[string][]processorattestation.Coverage {
	result := identityV1CoverageProofs(processorattestation.V2CoverageAxes(family), "adjust")
	for semantic, axis := range aliases {
		semantic = strings.ToLower(strings.TrimSpace(semantic))
		axis = strings.ToLower(strings.TrimSpace(axis))
		if semantic == "" || axis == "" {
			continue
		}
		result[semantic] = []processorattestation.Coverage{{Action: "adjust", Axis: axis}}
	}
	return result
}
