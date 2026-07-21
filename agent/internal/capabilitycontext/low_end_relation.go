package capabilitycontext

import (
	"encoding/json"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/lowendrelation"
)

const LowEndRelationContextPackSchema = "static_mix.low_end_relation.context_pack.v0"

type LowEndRelationInput struct {
	UserIntent      string
	ProjectState    map[string]any
	MixObservation  map[string]any
	MOMProjection   map[string]any
	TOMProjection   map[string]any
	ContextSnapshot map[string]any
	RequestContext  map[string]any
	ExecutionMemory map[string]any
	GeneratedAt     time.Time
	Budget          Budget
}

// LowEndRelationAnalysisDisclosure is the CCB disclosure serialised into
// the ContextBundle. It does not contain candidate actions.
type LowEndRelationAnalysisDisclosure struct {
	SchemaVersion string                      `json:"schema_version"`
	Readiness     capabilityruntime.Readiness `json:"readiness"`
	Coverage      lowendrelation.Coverage     `json:"coverage"`
	Summary       lowendrelation.LowEndSummary `json:"summary"`
	Observations  []lowendrelation.Observation `json:"observations,omitempty"`
}

type LowEndRelationPack struct {
	SchemaVersion      string                            `json:"schema_version"`
	PackID             string                            `json:"pack_id"`
	CapabilityID       string                            `json:"capability_id"`
	CapabilityName     string                            `json:"capability_name"`
	ContextManifestID  string                            `json:"context_manifest_id"`
	ContextBuilder     string                            `json:"context_builder"`
	GeneratedAt        string                            `json:"generated_at"`
	UserIntent         string                            `json:"user_intent,omitempty"`
	Scope              Scope                             `json:"scope"`
	Budget             Budget                            `json:"budget"`
	EvidenceRefs       []string                          `json:"evidence_refs,omitempty"`
	EvidenceStatus     map[string]EvidenceStatus         `json:"evidence_status,omitempty"`
	Readiness          capabilityruntime.Readiness       `json:"-"`
	AnalyzedTrackCount int                               `json:"analyzed_track_count"`
	DisclosedTrackCount int                              `json:"disclosed_track_count"`
	AnalysisDisclosure LowEndRelationAnalysisDisclosure  `json:"analysis_disclosure"`
	Summary            lowendrelation.LowEndSummary      `json:"summary"`
	Observations       []lowendrelation.Observation      `json:"observations,omitempty"`
	Conflicts          []lowendrelation.LowEndConflict   `json:"conflicts,omitempty"`
	Tracks             []lowendrelation.LowEndTrack      `json:"tracks,omitempty"`
	Limitations        []string                          `json:"limitations,omitempty"`
	Guidance           []string                          `json:"guidance,omitempty"`
	model              lowendrelation.Model
	result             lowendrelation.Result
}

func (p LowEndRelationPack) Map() map[string]any {
	data, _ := json.Marshal(p)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}
func (p LowEndRelationPack) Model() lowendrelation.Model   { return p.model }
func (p LowEndRelationPack) Result() lowendrelation.Result { return p.result }

func BuildLowEndRelationPack(in LowEndRelationInput) LowEndRelationPack {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	manifest := StaticMixLowEndRelationContextManifest()
	budget := in.Budget
	if budget.MaxDisclosedTracks == 0 && budget.MaxRankingRows == 0 && budget.MaxStringRunes == 0 {
		budget = manifest.DefaultBudget
	}
	budget = NormalizeBudget(budget)
	budget.MaxTracks = 0

	model := lowendrelation.BuildModel(lowendrelation.Input{
		UserIntent:      in.UserIntent,
		ProjectState:    in.ProjectState,
		MixObservation:  in.MixObservation,
		MOMProjection:   in.MOMProjection,
		TOMProjection:   in.TOMProjection,
		ContextSnapshot: in.ContextSnapshot,
		RequestContext:  in.RequestContext,
		ExecutionMemory: in.ExecutionMemory,
		GeneratedAt:     generatedAt,
	})
	result := lowendrelation.Analyze(model)

	disclosed := model.Tracks
	if budget.MaxDisclosedTracks > 0 && len(disclosed) > budget.MaxDisclosedTracks {
		disclosed = disclosed[:budget.MaxDisclosedTracks]
	}

	hasBandEvidence := model.Coverage.SubBandLeaderCount > 0 || model.Coverage.BassBandLeaderCount > 0
	evidenceStatus := map[string]EvidenceStatus{
		"mom_band_occupancy": {
			Status:     lowEndEvidenceStatus(hasBandEvidence),
			KnownCount: model.Coverage.SubBandLeaderCount + model.Coverage.BassBandLeaderCount,
			TotalCount: model.Coverage.AnalyzedTrackCount,
		},
		"low_end_tendency": {
			Status: lowEndEvidenceStatus(model.Coverage.LowEndTendencyAvailable),
		},
		"track_role_context": {
			Status:     lowEndEvidenceStatus(model.Coverage.RoleKnownCount > 0),
			KnownCount: model.Coverage.RoleKnownCount,
			TotalCount: model.Coverage.AnalyzedTrackCount,
		},
	}

	disclosure := LowEndRelationAnalysisDisclosure{
		SchemaVersion: "low_end_relation.analysis_disclosure.v0",
		Readiness:     model.Readiness,
		Coverage:      model.Coverage,
		Summary:       model.Summary,
		Observations:  append([]lowendrelation.Observation(nil), model.Observations...),
	}

	return LowEndRelationPack{
		SchemaVersion:       LowEndRelationContextPackSchema,
		PackID:              stablePackID(LowEndRelationCapabilityID, in.UserIntent, model.EvidenceRefs, generatedAt),
		CapabilityID:        LowEndRelationCapabilityID,
		CapabilityName:      "B4 Low-End Relation",
		ContextManifestID:   manifest.ManifestID,
		ContextBuilder:      CapabilityContextBuilderVersion,
		GeneratedAt:         generatedAt.Format(time.RFC3339),
		UserIntent:          compactText(in.UserIntent, budget.MaxStringRunes),
		Scope:               Scope{Kind: "project", Source: "project.state+MOM"},
		Budget:              budget,
		EvidenceRefs:        append([]string(nil), model.EvidenceRefs...),
		EvidenceStatus:      evidenceStatus,
		Readiness:           model.Readiness,
		AnalyzedTrackCount:  len(model.Tracks),
		DisclosedTrackCount: len(disclosed),
		AnalysisDisclosure:  disclosure,
		Summary:             model.Summary,
		Observations:        append([]lowendrelation.Observation(nil), model.Observations...),
		Conflicts:           append([]lowendrelation.LowEndConflict(nil), model.Conflicts...),
		Tracks:              disclosed,
		Limitations:         append([]string(nil), result.Limitations...),
		Guidance:            append([]string(nil), manifest.Guidance...),
		model:               model,
		result:              result,
	}
}

func lowEndEvidenceStatus(ok bool) string {
	if ok {
		return "available"
	}
	return "missing"
}
