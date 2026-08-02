package capabilitycontext

import (
	"encoding/json"
	"time"

	"vit-daw-agent/internal/frequencycleanup"
)

const FrequencyCleanupContextPackSchema = "fine_mix.frequency_cleanup.context_pack.v1"

type FrequencyCleanupInput struct {
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

type FrequencyCleanupAnalysisDisclosure struct {
	SchemaVersion string                                `json:"schema_version"`
	Readiness     frequencycleanup.Readiness            `json:"readiness"`
	Coverage      frequencycleanup.Coverage             `json:"coverage"`
	Tracks        []frequencycleanup.TrackProfile       `json:"tracks"`
	Candidates    []frequencycleanup.DiagnosisCandidate `json:"diagnosis_candidates,omitempty"`
	Limitations   []string                              `json:"limitations,omitempty"`
}

type FrequencyCleanupPack struct {
	SchemaVersion      string                                `json:"schema_version"`
	PackID             string                                `json:"pack_id"`
	CapabilityID       string                                `json:"capability_id"`
	CapabilityName     string                                `json:"capability_name"`
	ContextManifestID  string                                `json:"context_manifest_id"`
	ContextBuilder     string                                `json:"context_builder"`
	GeneratedAt        string                                `json:"generated_at"`
	UserIntent         string                                `json:"user_intent,omitempty"`
	Scope              Scope                                 `json:"scope"`
	Budget             Budget                                `json:"budget"`
	EvidenceRefs       []string                              `json:"evidence_refs,omitempty"`
	EvidenceStatus     map[string]EvidenceStatus             `json:"evidence_status,omitempty"`
	Readiness          frequencycleanup.Readiness            `json:"-"`
	AnalyzedTrackCount int                                   `json:"analyzed_track_count"`
	AnalysisDisclosure FrequencyCleanupAnalysisDisclosure    `json:"analysis_disclosure"`
	Tracks             []frequencycleanup.TrackProfile       `json:"tracks"`
	Candidates         []frequencycleanup.DiagnosisCandidate `json:"diagnosis_candidates,omitempty"`
	Limitations        []string                              `json:"limitations,omitempty"`
	Guidance           []string                              `json:"guidance,omitempty"`
	model              frequencycleanup.Model
	result             frequencycleanup.Result
}

func (p FrequencyCleanupPack) Map() map[string]any {
	data, _ := json.Marshal(p)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}
func (p FrequencyCleanupPack) Model() frequencycleanup.Model   { return p.model }
func (p FrequencyCleanupPack) Result() frequencycleanup.Result { return p.result }

func BuildFrequencyCleanupPack(in FrequencyCleanupInput) FrequencyCleanupPack {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	manifest := FineMixFrequencyCleanupContextManifest()
	budget := in.Budget
	if budget.MaxStringRunes == 0 {
		budget = manifest.DefaultBudget
	}
	budget = NormalizeBudget(budget)
	// Whole-project classification is a C1 correctness invariant. The compact
	// profile roster is never top-N truncated by the presentation budget.
	budget.MaxTracks = 0
	budget.MaxDisclosedTracks = 0
	model := frequencycleanup.BuildModel(frequencycleanup.Input{UserIntent: in.UserIntent, ProjectState: in.ProjectState, MixObservation: in.MixObservation, MOMProjection: in.MOMProjection, TOMProjection: in.TOMProjection, ContextSnapshot: in.ContextSnapshot, RequestContext: in.RequestContext, ExecutionMemory: in.ExecutionMemory, GeneratedAt: generatedAt})
	result := frequencycleanup.Analyze(model)
	evidenceStatus := map[string]EvidenceStatus{
		"frequency_relationship":    {Status: availability(model.Readiness.Diagnosis.CanProceed), KnownCount: model.Coverage.EligibleTrackCount, TotalCount: model.Coverage.ProjectTrackCount},
		"ccb_frequency_summary":     {Status: availability(model.Readiness.Planning.CanProceed), KnownCount: model.Coverage.ProfileCount, TotalCount: model.Coverage.ProjectTrackCount},
		"post_fx_same_tap_baseline": {Status: "deferred_until_target_selection", KnownCount: 0, TotalCount: 0},
	}
	disclosure := FrequencyCleanupAnalysisDisclosure{SchemaVersion: "frequency_cleanup.analysis_disclosure.v1", Readiness: model.Readiness, Coverage: model.Coverage, Tracks: append([]frequencycleanup.TrackProfile(nil), model.Tracks...), Candidates: append([]frequencycleanup.DiagnosisCandidate(nil), model.Candidates...), Limitations: append([]string(nil), model.Limitations...)}
	return FrequencyCleanupPack{SchemaVersion: FrequencyCleanupContextPackSchema, PackID: stablePackID(FrequencyCleanupCapabilityID, in.UserIntent, model.EvidenceRefs, generatedAt), CapabilityID: FrequencyCleanupCapabilityID, CapabilityName: "C1 Frequency Cleanup", ContextManifestID: manifest.ManifestID, ContextBuilder: CapabilityContextBuilderVersion, GeneratedAt: generatedAt.Format(time.RFC3339), UserIntent: compactText(in.UserIntent, budget.MaxStringRunes), Scope: Scope{Kind: "project", IDs: frequencyCleanupTrackIDs(model.Tracks), Source: "project.state+MOM.frequency_relationship+TOM+Mixboard"}, Budget: budget, EvidenceRefs: append([]string(nil), model.EvidenceRefs...), EvidenceStatus: evidenceStatus, Readiness: model.Readiness, AnalyzedTrackCount: len(model.Tracks), AnalysisDisclosure: disclosure, Tracks: append([]frequencycleanup.TrackProfile(nil), model.Tracks...), Candidates: append([]frequencycleanup.DiagnosisCandidate(nil), model.Candidates...), Limitations: append([]string(nil), result.Limitations...), Guidance: append([]string(nil), manifest.Guidance...), model: model, result: result}
}

func availability(ok bool) string {
	if ok {
		return "available"
	}
	return "missing"
}
func frequencyCleanupTrackIDs(tracks []frequencycleanup.TrackProfile) []string {
	out := make([]string, 0, len(tracks))
	for _, track := range tracks {
		out = append(out, track.TrackID)
	}
	return out
}
