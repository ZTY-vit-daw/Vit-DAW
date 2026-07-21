package capabilitycontext

import (
	"encoding/json"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/panlayout"
)

const PanLayoutContextPackSchema = "static_mix.pan_layout.context_pack.v1"

type PanLayoutInput struct {
	UserIntent                                                                      string
	ProjectState, MixObservation, MOMProjection, AudioAnalysisStatus, TOMProjection map[string]any
	ContextSnapshot, RequestContext, ExecutionMemory                                map[string]any
	Style                                                                           mixstyle.MixStyle
	StyleExplicit                                                                   bool
	GeneratedAt                                                                     time.Time
	Budget                                                                          Budget
}

type PanLayoutPack struct {
	SchemaVersion       string                       `json:"schema_version"`
	PackID              string                       `json:"pack_id"`
	CapabilityID        string                       `json:"capability_id"`
	CapabilityName      string                       `json:"capability_name"`
	ContextManifestID   string                       `json:"context_manifest_id"`
	ContextBuilder      string                       `json:"context_builder"`
	GeneratedAt         string                       `json:"generated_at"`
	UserIntent          string                       `json:"user_intent,omitempty"`
	Scope               Scope                        `json:"scope"`
	Budget              Budget                       `json:"budget"`
	EvidenceRefs        []string                     `json:"evidence_refs,omitempty"`
	EvidenceStatus      map[string]EvidenceStatus    `json:"evidence_status,omitempty"`
	Style               panlayout.Style              `json:"mix_style"`
	Readiness           capabilityruntime.Readiness  `json:"-"`
	AnalyzedTrackCount  int                          `json:"analyzed_track_count"`
	DisclosedTrackCount int                          `json:"disclosed_track_count"`
	Admission           PanLayoutAdmissionDisclosure `json:"admission"`
	Decision            PanLayoutDecisionDisclosure  `json:"decision"`
	LayoutSummary       panlayout.LayoutSummary      `json:"layout_summary"`
	MOMSummary          map[string]any               `json:"mom_summary,omitempty"`
	TOMSummary          map[string]any               `json:"tom_summary,omitempty"`
	Tracks              []panlayout.Track            `json:"tracks,omitempty"`
	Limitations         []string                     `json:"limitations,omitempty"`
	Guidance            []string                     `json:"guidance,omitempty"`
	model               panlayout.Model
	result              panlayout.Result
}

type PanLayoutAdmissionDisclosure struct {
	SchemaVersion string                      `json:"schema_version"`
	Readiness     capabilityruntime.Readiness `json:"readiness"`
	Coverage      panlayout.Coverage          `json:"coverage"`
}
type PanLayoutDecisionDisclosure struct {
	SchemaVersion     string                         `json:"schema_version"`
	CandidatePlans    []PanLayoutCandidateDisclosure `json:"candidate_plans,omitempty"`
	SelectionContract []string                       `json:"selection_contract"`
}
type PanLayoutCandidateDisclosure struct {
	CandidatePlanID string                   `json:"candidate_plan_id"`
	Strategy        string                   `json:"strategy"`
	Label           string                   `json:"label"`
	Recommended     bool                     `json:"recommended"`
	Confidence      string                   `json:"confidence"`
	ActionCount     int                      `json:"action_count"`
	MaxAbsPan       float64                  `json:"max_abs_pan"`
	GroupSummary    []panlayout.GroupSummary `json:"group_summary,omitempty"`
	ActionExamples  []panlayout.Action       `json:"action_examples,omitempty"`
}

func (p PanLayoutPack) Map() map[string]any {
	data, _ := json.Marshal(p)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}
func (p PanLayoutPack) Model() panlayout.Model   { return p.model }
func (p PanLayoutPack) Result() panlayout.Result { return p.result }

func BuildPanLayoutPack(in PanLayoutInput) PanLayoutPack {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	manifest := StaticMixPanLayoutContextManifest()
	budget := in.Budget
	if budget.MaxDisclosedTracks == 0 && budget.MaxRankingRows == 0 && budget.MaxStringRunes == 0 {
		budget = manifest.DefaultBudget
	}
	budget = NormalizeBudget(budget)
	budget.MaxTracks = 0
	style := mixstyle.Normalize(in.Style)
	if mixstyle.Validate(style) != nil {
		style = mixstyle.Default()
	}
	model := panlayout.BuildModel(panlayout.Input{UserIntent: in.UserIntent, ProjectState: in.ProjectState, MixObservation: in.MixObservation, MOMProjection: in.MOMProjection, AudioAnalysisStatus: in.AudioAnalysisStatus, TOMProjection: in.TOMProjection, ContextSnapshot: in.ContextSnapshot, RequestContext: in.RequestContext, ExecutionMemory: in.ExecutionMemory, Style: style, StyleExplicit: in.StyleExplicit, GeneratedAt: generatedAt})
	result := panlayout.Solve(model)
	tracks := panLayoutDisclosureTracks(model, result, budget.MaxDisclosedTracks)
	candidates := panLayoutCandidateDisclosures(result, budget.MaxRankingRows)
	return PanLayoutPack{SchemaVersion: PanLayoutContextPackSchema, PackID: stablePackID(PanLayoutCapabilityID, in.UserIntent, []string{result.ResultID, style.ID, mixstyle.Hash(style)}, generatedAt), CapabilityID: PanLayoutCapabilityID, CapabilityName: "B3 Pan Layout", ContextManifestID: manifest.ManifestID, ContextBuilder: CapabilityContextBuilderVersion, GeneratedAt: generatedAt.Format(time.RFC3339), UserIntent: compactText(in.UserIntent, budget.MaxStringRunes), Scope: Scope{Kind: "project", Source: "project.state+TOM+MOM+stereo_evidence"}, Budget: budget, EvidenceRefs: append([]string(nil), model.EvidenceRefs...), EvidenceStatus: map[string]EvidenceStatus{
		"project_pan":     {Status: staticBalanceStatus(model.Coverage.PanKnownCount > 0), KnownCount: model.Coverage.PanKnownCount, TotalCount: model.Coverage.RoleCandidateCount},
		"tom_roles":       {Status: staticBalanceStatus(model.Coverage.RoleKnownCount > 0), KnownCount: model.Coverage.RoleKnownCount, TotalCount: model.Coverage.RoleCandidateCount},
		"source_channels": {Status: staticBalanceStatus(model.Coverage.ChannelKnownCount > 0), KnownCount: model.Coverage.ChannelKnownCount, TotalCount: model.Coverage.RoleCandidateCount},
		"stereo_relation": {Status: staticBalanceStatus(model.Coverage.StereoEvidenceKnownCount > 0), KnownCount: model.Coverage.StereoEvidenceKnownCount, TotalCount: model.Coverage.RoleCandidateCount},
	}, Style: model.Style, Readiness: model.Readiness, AnalyzedTrackCount: len(model.Tracks), DisclosedTrackCount: len(tracks), Admission: PanLayoutAdmissionDisclosure{SchemaVersion: "pan_layout.admission_disclosure.v1", Readiness: model.Readiness, Coverage: model.Coverage}, Decision: PanLayoutDecisionDisclosure{SchemaVersion: "pan_layout.decision_disclosure.v1", CandidatePlans: candidates, SelectionContract: []string{"Select exactly one existing candidate_plan_id or ask for clarification.", "Do not generate or modify track IDs, pan values, or action lists."}}, LayoutSummary: model.Layout, MOMSummary: cloneMap(model.MOMSummary), TOMSummary: cloneMap(model.TOMSummary), Tracks: tracks, Limitations: append([]string(nil), result.Limitations...), Guidance: append([]string(nil), manifest.Guidance...), model: model, result: result}
}

func panLayoutDisclosureTracks(model panlayout.Model, result panlayout.Result, max int) []panlayout.Track {
	if max <= 0 || len(model.Tracks) <= max {
		return append([]panlayout.Track(nil), model.Tracks...)
	}
	byID := map[string]panlayout.Track{}
	for _, t := range model.Tracks {
		byID[t.TrackID] = t
	}
	out := []panlayout.Track{}
	seen := map[string]bool{}
	add := func(t panlayout.Track) {
		if len(out) >= max || t.TrackID == "" || seen[t.TrackID] {
			return
		}
		seen[t.TrackID] = true
		out = append(out, t)
	}
	for _, c := range result.Candidates {
		if c.Recommended {
			for _, a := range c.Actions {
				add(byID[a.TrackID])
			}
		}
	}
	for _, t := range model.Tracks {
		if len(t.RiskFlags) > 0 || t.Exclusion != "" {
			add(t)
		}
	}
	for _, t := range model.Tracks {
		add(t)
	}
	return out
}
func panLayoutCandidateDisclosures(result panlayout.Result, max int) []PanLayoutCandidateDisclosure {
	if max <= 0 {
		max = 8
	}
	out := []PanLayoutCandidateDisclosure{}
	for _, c := range result.Candidates {
		examples := []panlayout.Action{}
		if c.Recommended {
			examples = append(examples, c.Actions...)
			if len(examples) > max {
				examples = examples[:max]
			}
		}
		out = append(out, PanLayoutCandidateDisclosure{CandidatePlanID: c.CandidatePlanID, Strategy: c.Strategy, Label: c.Label, Recommended: c.Recommended, Confidence: c.Confidence, ActionCount: len(c.Actions), MaxAbsPan: c.MaxAbsPan, GroupSummary: append([]panlayout.GroupSummary(nil), c.GroupSummary...), ActionExamples: examples})
	}
	return out
}
