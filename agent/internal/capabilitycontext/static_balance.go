package capabilitycontext

import (
	"encoding/json"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/staticbalance"
)

const StaticBalanceContextPackSchema = "static_mix.static_balance.context_pack.v1"

type StaticBalanceInput struct {
	UserIntent      string
	ProjectState    map[string]any
	MixObservation  map[string]any
	MOMProjection   map[string]any
	TOMProjection   map[string]any
	ContextSnapshot map[string]any
	RequestContext  map[string]any
	ExecutionMemory map[string]any
	Style           mixstyle.MixStyle
	StyleExplicit   bool
	GeneratedAt     time.Time
	Budget          Budget
}

type StaticBalanceTrack = staticbalance.Track

type StaticBalanceStyle = staticbalance.Style

type StaticBalancePack struct {
	SchemaVersion       string                             `json:"schema_version"`
	PackID              string                             `json:"pack_id"`
	CapabilityID        string                             `json:"capability_id"`
	CapabilityName      string                             `json:"capability_name"`
	ContextManifestID   string                             `json:"context_manifest_id"`
	ContextBuilder      string                             `json:"context_builder"`
	GeneratedAt         string                             `json:"generated_at"`
	UserIntent          string                             `json:"user_intent,omitempty"`
	Scope               Scope                              `json:"scope"`
	Budget              Budget                             `json:"budget"`
	EvidenceRefs        []string                           `json:"evidence_refs,omitempty"`
	EvidenceStatus      map[string]EvidenceStatus          `json:"evidence_status,omitempty"`
	Style               StaticBalanceStyle                 `json:"mix_style"`
	Readiness           capabilityruntime.Readiness        `json:"-"`
	AnalyzedTrackCount  int                                `json:"analyzed_track_count"`
	DisclosedTrackCount int                                `json:"disclosed_track_count"`
	Admission           StaticBalanceAdmissionDisclosure   `json:"admission"`
	Decision            StaticBalanceDecisionDisclosure    `json:"decision"`
	RoleModel           map[string]any                     `json:"role_model,omitempty"`
	MOMSummary          map[string]any                     `json:"mom_summary,omitempty"`
	TOMSummary          map[string]any                     `json:"tom_summary,omitempty"`
	Candidates          []StaticBalanceCandidateDisclosure `json:"-"`
	Tracks              []StaticBalanceTrack               `json:"tracks,omitempty"`
	Limitations         []string                           `json:"limitations,omitempty"`
	Guidance            []string                           `json:"guidance,omitempty"`
	model               staticbalance.Model
	result              staticbalance.Result
}

type StaticBalanceAdmissionDisclosure struct {
	SchemaVersion string                      `json:"schema_version"`
	Readiness     capabilityruntime.Readiness `json:"readiness"`
	Coverage      staticbalance.Coverage      `json:"coverage"`
}

type StaticBalanceDecisionDisclosure struct {
	SchemaVersion     string                             `json:"schema_version"`
	CandidatePlans    []StaticBalanceCandidateDisclosure `json:"candidate_plans,omitempty"`
	SelectionContract []string                           `json:"selection_contract"`
}

type StaticBalanceCandidateDisclosure struct {
	CandidatePlanID string                          `json:"candidate_plan_id"`
	Strategy        string                          `json:"strategy"`
	Label           string                          `json:"label"`
	Recommended     bool                            `json:"recommended"`
	Confidence      string                          `json:"confidence"`
	ActionCount     int                             `json:"action_count"`
	MaxAbsDeltaDB   float64                         `json:"max_abs_delta_db"`
	FunctionSummary []staticbalance.FunctionSummary `json:"function_summary,omitempty"`
	ActionExamples  []StaticBalanceActionDraft      `json:"action_examples,omitempty"`
}

type StaticBalanceActionDraft = staticbalance.Action

func (p StaticBalancePack) Map() map[string]any {
	data, _ := json.Marshal(p)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func BuildStaticBalancePack(in StaticBalanceInput) StaticBalancePack {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	manifest := StaticMixStaticBalanceContextManifest()
	budget := in.Budget
	if budget.MaxTracks == 0 && budget.MaxDisclosedTracks == 0 && budget.MaxRankingRows == 0 && budget.MaxClipsPerTrack == 0 && budget.MaxStringRunes == 0 {
		budget = manifest.DefaultBudget
	}
	budget = NormalizeBudget(budget)
	// B2 never uses the legacy MaxTracks analysis budget. Only disclosure is bounded.
	budget.MaxTracks = 0
	style := in.Style
	if mixstyle.Validate(style) != nil {
		style = mixstyle.Default()
	}
	model := staticbalance.BuildModel(staticbalance.Input{
		UserIntent: in.UserIntent, ProjectState: in.ProjectState, MixObservation: in.MixObservation,
		MOMProjection: in.MOMProjection, TOMProjection: in.TOMProjection,
		ContextSnapshot: in.ContextSnapshot, RequestContext: in.RequestContext, ExecutionMemory: in.ExecutionMemory,
		Style: style, StyleExplicit: in.StyleExplicit, GeneratedAt: generatedAt,
	})
	result := staticbalance.Solve(model)
	tracks := staticBalanceDisclosureTracks(model, result, budget.MaxDisclosedTracks)
	candidates := staticBalanceCandidateDisclosures(result, budget.MaxRankingRows)
	return StaticBalancePack{
		SchemaVersion:     StaticBalanceContextPackSchema,
		PackID:            stablePackID(StaticBalanceCapabilityID, in.UserIntent, []string{result.ResultID, style.ID}, generatedAt),
		CapabilityID:      StaticBalanceCapabilityID,
		CapabilityName:    "B2 Static Balance",
		ContextManifestID: manifest.ManifestID,
		ContextBuilder:    CapabilityContextBuilderVersion,
		GeneratedAt:       generatedAt.Format(time.RFC3339),
		UserIntent:        compactText(in.UserIntent, budget.MaxStringRunes),
		Scope:             Scope{Kind: "project", Source: "project.state+TOM+MOM.static_level_relationship"},
		Budget:            budget,
		EvidenceRefs:      append([]string(nil), model.EvidenceRefs...),
		EvidenceStatus: map[string]EvidenceStatus{
			"project_state": {Status: staticBalanceStatus(len(model.Tracks) > 0), KnownCount: len(model.Tracks), TotalCount: len(model.Tracks)},
			"tom_roles":     {Status: staticBalanceStatus(model.Coverage.RoleKnownCount > 0), KnownCount: model.Coverage.RoleKnownCount, TotalCount: len(model.Tracks)},
			"static_levels": {Status: staticBalanceStatus(model.Coverage.ProjectedLevelKnownCount > 0), KnownCount: model.Coverage.ProjectedLevelKnownCount, TotalCount: len(model.Tracks)},
			"mom_relation":  {Status: staticBalanceStatus(len(model.MOMSummary) > 0), KnownCount: model.Coverage.LevelKnownCount, TotalCount: len(model.Tracks)},
			"track_faders":  {Status: staticBalanceStatus(model.Coverage.FaderKnownCount > 0), KnownCount: model.Coverage.FaderKnownCount, TotalCount: len(model.Tracks)},
		},
		Style:               StaticBalanceStyle{SchemaVersion: model.Style.SchemaVersion, Kind: model.Style.Kind, ID: model.Style.ID, Name: model.Style.Name, Explicit: model.Style.Explicit, Dimensions: model.Style.Dimensions},
		Readiness:           model.Readiness,
		AnalyzedTrackCount:  len(model.Tracks),
		DisclosedTrackCount: len(tracks),
		Admission: StaticBalanceAdmissionDisclosure{
			SchemaVersion: "static_balance.admission_disclosure.v1", Readiness: model.Readiness, Coverage: model.Coverage,
		},
		Decision: StaticBalanceDecisionDisclosure{
			SchemaVersion: "static_balance.decision_disclosure.v1", CandidatePlans: candidates,
			SelectionContract: []string{
				"Select exactly one existing candidate_plan_id or ask for clarification.",
				"Do not generate or modify track IDs, dB values, or action lists.",
			},
		},
		RoleModel: map[string]any{
			"raw_role_label_count":     model.Coverage.RawRoleKnownCount,
			"known_role_count":         model.Coverage.RoleKnownCount,
			"eligible_track_count":     model.Coverage.EligibleTrackCount,
			"role_candidate_count":     model.Coverage.RoleCandidateCount,
			"analyzed_track_count":     len(model.Tracks),
			"disclosed_track_count":    len(tracks),
			"role_coverage":            model.Coverage.RoleCoverage,
			"effective_level_coverage": model.Coverage.EffectiveLevelCoverage,
			"level_coverage":           model.Coverage.LevelCoverage,
			"functions":                []string{"foreground", "rhythm_anchor", "low_end_anchor", "harmonic_bed", "support", "effects"},
			"role_source_priority":     []string{"explicit_role", "tom_assignment", "mom_role_guess", "track_name_inference"},
		},
		MOMSummary:  cloneMap(model.MOMSummary),
		TOMSummary:  cloneMap(model.TOMSummary),
		Candidates:  candidates,
		Tracks:      tracks,
		Limitations: append([]string(nil), result.Limitations...),
		Guidance: []string{
			"Use functional relationships rather than universal instrument loudness laws.",
			"B2 does not universally require L3 spectral/stereo evidence or LUFS when comparable project-wide RMS/active RMS evidence is sufficient.",
			"Mix Style may change bounded weights but cannot bypass readiness, evidence, headroom, or +/-2 dB gates.",
			"B2 changes track faders only; do not change clip gain, plugins, pan, automation, or direct primitive volume commands.",
			"Select only an existing candidate_plan_id; never invent track IDs or dB values.",
		},
		model:  model,
		result: result,
	}
}

func staticBalanceDisclosureTracks(model staticbalance.Model, result staticbalance.Result, max int) []StaticBalanceTrack {
	if max <= 0 || len(model.Tracks) <= max {
		return append([]StaticBalanceTrack(nil), model.Tracks...)
	}
	byID := map[string]StaticBalanceTrack{}
	for _, track := range model.Tracks {
		byID[track.TrackID] = track
	}
	out := make([]StaticBalanceTrack, 0, max)
	seen := map[string]bool{}
	add := func(track StaticBalanceTrack) {
		if len(out) >= max || track.TrackID == "" || seen[track.TrackID] {
			return
		}
		seen[track.TrackID] = true
		out = append(out, track)
	}
	for _, candidate := range result.Candidates {
		if !candidate.Recommended {
			continue
		}
		for _, action := range candidate.Actions {
			add(byID[action.TrackID])
		}
	}
	for _, track := range model.Tracks {
		if track.Exclusion == "role_unresolved" || track.Exclusion == "role_confidence_low" {
			add(track)
		}
	}
	for _, track := range model.Tracks {
		add(track)
	}
	return out
}

func staticBalanceCandidateDisclosures(result staticbalance.Result, maxExamples int) []StaticBalanceCandidateDisclosure {
	if maxExamples <= 0 {
		maxExamples = 8
	}
	out := make([]StaticBalanceCandidateDisclosure, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		var examples []staticbalance.Action
		if candidate.Recommended {
			examples = candidate.Actions
			if len(examples) > maxExamples {
				examples = examples[:maxExamples]
			}
		}
		out = append(out, StaticBalanceCandidateDisclosure{
			CandidatePlanID: candidate.CandidatePlanID, Strategy: candidate.Strategy, Label: candidate.Label,
			Recommended: candidate.Recommended, Confidence: candidate.Confidence, ActionCount: len(candidate.Actions),
			MaxAbsDeltaDB: candidate.MaxAbsDeltaDB, FunctionSummary: append([]staticbalance.FunctionSummary(nil), candidate.FunctionSummary...),
			ActionExamples: append([]StaticBalanceActionDraft(nil), examples...),
		})
	}
	return out
}

func (p StaticBalancePack) Model() staticbalance.Model   { return p.model }
func (p StaticBalancePack) Result() staticbalance.Result { return p.result }

func staticBalanceStatus(ok bool) string {
	if ok {
		return "available"
	}
	return "missing"
}
