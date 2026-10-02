package experiment

import (
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

const UserJudgmentEvidenceSchemaVersion = "vit.user_judgment_evidence.v1"

type HeardDifference string

const (
	HeardDifferenceYes    HeardDifference = "yes"
	HeardDifferenceNo     HeardDifference = "no"
	HeardDifferenceUnsure HeardDifference = "unsure"
)

type JudgmentPreference string

const (
	PreferenceA       JudgmentPreference = "a"
	PreferenceB       JudgmentPreference = "b"
	PreferenceNeither JudgmentPreference = "neither"
	PreferenceEqual   JudgmentPreference = "equal"
	PreferenceUnsure  JudgmentPreference = "unsure"
)

// UserJudgmentEvidence is the immutable, raw user statement captured from an
// AuditionSession. Corrections append a new record with supersedes_id; the
// original record is never rewritten.
type UserJudgmentEvidence struct {
	SchemaVersion     string `json:"schema_version"`
	ID                string `json:"id"`
	SupersedesID      string `json:"supersedes_id,omitempty"`
	ConversationID    string `json:"conversation_id"`
	TurnID            string `json:"turn_id"`
	RoundID           string `json:"round_id"`
	AuditionSessionID string `json:"audition_session_id"`

	CandidateARef            string `json:"candidate_a_ref"`
	CandidateBRef            string `json:"candidate_b_ref"`
	CandidateACheckpointRef  string `json:"candidate_a_checkpoint_ref,omitempty"`
	CandidateBCheckpointRef  string `json:"candidate_b_checkpoint_ref,omitempty"`
	CandidateACommitID       string `json:"candidate_a_commit_id,omitempty"`
	CandidateBCommitID       string `json:"candidate_b_commit_id,omitempty"`
	CandidateABranchRef      string `json:"candidate_a_branch_ref,omitempty"`
	CandidateBBranchRef      string `json:"candidate_b_branch_ref,omitempty"`
	CandidateAWorktreeRef    string `json:"candidate_a_worktree_ref,omitempty"`
	CandidateBWorktreeRef    string `json:"candidate_b_worktree_ref,omitempty"`
	CandidateAPreviewRef     string `json:"candidate_a_preview_ref,omitempty"`
	CandidateBPreviewRef     string `json:"candidate_b_preview_ref,omitempty"`
	CandidateARenderRevision string `json:"candidate_a_render_revision,omitempty"`
	CandidateBRenderRevision string `json:"candidate_b_render_revision,omitempty"`

	ProjectUUID            string         `json:"project_uuid,omitempty"`
	ProjectRevision        string         `json:"project_revision,omitempty"`
	Scope                  string         `json:"scope,omitempty"`
	TransportAnchor        map[string]any `json:"transport_anchor,omitempty"`
	LoudnessReference      map[string]any `json:"loudness_reference,omitempty"`
	AnalyticalEvidenceRefs []string       `json:"analytical_evidence_refs,omitempty"`

	HeardDifference HeardDifference    `json:"heard_difference"`
	Preference      JudgmentPreference `json:"preference"`
	ReasonTags      []string           `json:"reason_tags,omitempty"`
	FreeText        string             `json:"free_text,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
}

func (e UserJudgmentEvidence) Validate() error {
	if e.SchemaVersion != UserJudgmentEvidenceSchemaVersion {
		return fmt.Errorf("user judgment schema_version must be %s", UserJudgmentEvidenceSchemaVersion)
	}
	for name, value := range map[string]string{
		"id": e.ID, "conversation_id": e.ConversationID, "turn_id": e.TurnID,
		"round_id": e.RoundID, "audition_session_id": e.AuditionSessionID,
		"candidate_a_ref": e.CandidateARef, "candidate_b_ref": e.CandidateBRef,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("user judgment %s is required", name)
		}
	}
	switch e.HeardDifference {
	case HeardDifferenceYes, HeardDifferenceNo, HeardDifferenceUnsure:
	default:
		return fmt.Errorf("unsupported heard_difference %q", e.HeardDifference)
	}
	switch e.Preference {
	case PreferenceA, PreferenceB, PreferenceNeither, PreferenceEqual, PreferenceUnsure:
	default:
		return fmt.Errorf("unsupported preference %q", e.Preference)
	}
	if e.HeardDifference != HeardDifferenceYes && (e.Preference == PreferenceA || e.Preference == PreferenceB) {
		return fmt.Errorf("preference %q requires heard_difference=yes", e.Preference)
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("user judgment created_at is required")
	}
	return nil
}

func (t *Turn) RequestUserJudgmentForSession(summary, auditionSessionID string, now time.Time) ([]trajectory.Event, error) {
	if err := t.ensureLive(); err != nil {
		return nil, err
	}
	if t.ContractID != "" && t.TaskState != taskstate.StateHumanJudgmentRequired {
		return nil, fmt.Errorf("user judgment request requires canonical task state human_judgment_required")
	}
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if round.TargetResponse == nil || round.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady {
		return nil, fmt.Errorf("user judgment requires human_audition_ready target response")
	}
	auditionSessionID = strings.TrimSpace(auditionSessionID)
	if auditionSessionID == "" {
		return nil, fmt.Errorf("user judgment requires audition_session_id")
	}
	if round.UserJudgmentRequested && t.Status == StatusWaitingForUser {
		return nil, fmt.Errorf("user judgment is already pending")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.AuditionSessionID = auditionSessionID
	round.UserJudgmentRequested = true
	round.Status = RoundTargetResponse
	round.Phase = "user_judgment_waiting"
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	t.Status = StatusWaitingForUser
	t.UpdatedAt = now.UTC()
	return t.events(now, trajectory.EventUserJudgmentRequested, round.ID, trajectory.NodeJudgment,
		"user A/B judgment requested", round.TargetResponse.EvidenceRefs, nil,
		map[string]any{"summary": strings.TrimSpace(summary), "audition_session_id": auditionSessionID}), nil
}

// RequestUserJudgment is retained to fail old, unbound callers closed. Product
// paths must supply the exact AuditionSession identity.
func (t *Turn) RequestUserJudgment(summary string, now time.Time) ([]trajectory.Event, error) {
	return nil, fmt.Errorf("user judgment requires audition_session_id; use RequestUserJudgmentForSession")
}

// RequestUserJudgmentAtParkedBoundary arms the durable human-judgment request
// for the JUDGMENT-SETTLE-STALL-1 shape: the settle turn already recorded the
// round decision user_judgment_pending — the round is durably parked at the
// boundary by its own decision — but its companion report field (the
// human_audition_ready target response) never landed, so the ordinary
// RequestUserJudgmentForSession refuses the round and the boundary becomes
// unanswerable (2026-10-02 webui_muqwy5sv: the A/B card mounted at the applied
// boundary, the judgment seat never armed, the task stayed needs_experiment,
// and the parked controller kept the conversation). The parked decision is the
// durable statement that the human audition is the decider for this round, so
// arming the request from it weakens no evidence gate: every original guard
// except the missing-report one still applies, and a late settle report keeps
// landing harmlessly (its round decision is rejected by
// experimentJudgmentPending while materiality/target_response fields still
// append to the round).
func (t *Turn) RequestUserJudgmentAtParkedBoundary(summary, auditionSessionID string, now time.Time) ([]trajectory.Event, error) {
	if err := t.ensureLive(); err != nil {
		return nil, err
	}
	if t.ContractID != "" && t.TaskState != taskstate.StateHumanJudgmentRequired {
		return nil, fmt.Errorf("user judgment request requires canonical task state human_judgment_required")
	}
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if round.Decision != DecisionUserJudgment {
		return nil, fmt.Errorf("parked-boundary judgment request requires the user_judgment_pending round decision, got %q", round.Decision)
	}
	if round.TargetResponse != nil {
		return nil, fmt.Errorf("round carries a target response; use RequestUserJudgmentForSession")
	}
	if round.UserJudgmentRequested || len(round.UserJudgmentEvidence) > 0 {
		return nil, fmt.Errorf("user judgment is already armed for this round")
	}
	auditionSessionID = strings.TrimSpace(auditionSessionID)
	if auditionSessionID == "" {
		return nil, fmt.Errorf("user judgment requires audition_session_id")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.AuditionSessionID = auditionSessionID
	round.UserJudgmentRequested = true
	round.Status = RoundTargetResponse
	round.Phase = "user_judgment_waiting"
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	t.Status = StatusWaitingForUser
	t.UpdatedAt = now.UTC()
	evidenceRefs := RoundJudgmentBoundaryEvidenceRefs(*round)
	return t.events(now, trajectory.EventUserJudgmentRequested, round.ID, trajectory.NodeJudgment,
		"user A/B judgment requested", evidenceRefs, nil,
		map[string]any{"summary": strings.TrimSpace(summary), "audition_session_id": auditionSessionID,
			"parked_boundary": true}), nil
}

// roundJudgmentBoundaryEvidenceRefs names the evidence the parked boundary
// arbitrates over: the round's recorded observations (post-action first) and
// its materiality record when present. The ordinary armed path uses the target
// response's own refs; the parked shape reconstructs them from the round.
// RoundJudgmentBoundaryEvidenceRefs names the evidence the parked judgment boundary
// arbitrates over; shared by the experiment-layer arming/reconstruction and the
// chat-layer evidence builder so both cite the same refs.
func RoundJudgmentBoundaryEvidenceRefs(round Round) []string {
	out := []string{}
	if round.Materiality != nil {
		out = append(out, round.Materiality.EvidenceRefs...)
	}
	for index := len(round.Observations) - 1; index >= 0; index-- {
		observation := round.Observations[index]
		if !observation.PostAction {
			continue
		}
		out = append(out, observation.EvidenceRefs...)
		break
	}
	if len(out) == 0 {
		for _, observation := range round.Observations {
			out = append(out, observation.EvidenceRefs...)
		}
	}
	return unique(out)
}

func (t *Turn) RecordUserJudgmentEvidence(evidence UserJudgmentEvidence, now time.Time) ([]trajectory.Event, error) {
	if strings.TrimSpace(evidence.SupersedesID) == "" {
		if err := t.ensureLive(); err != nil {
			return nil, err
		}
	}
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	isCorrection := len(round.UserJudgmentEvidence) > 0
	// JUDGMENT-SETTLE-STALL-1: a round durably parked at the boundary by its own
	// user_judgment_pending decision may carry no target response at all (the
	// settle tail never landed it). The parked decision itself states that the
	// human audition is the decider, so the first judgment reconstructs the
	// human_audition_ready evaluation the boundary was waiting for — the exact
	// report shape RecordTargetResponse admits for this tier (ambiguous +
	// human_audition_ready) — over the evidence the round actually holds. The
	// promotion below then converts it from the recorded judgment exactly as it
	// does for the ordinary armed shape; nothing is marked sufficient before a
	// human preference B says so.
	if !isCorrection && round.TargetResponse == nil && round.Decision == DecisionUserJudgment {
		synthesized := TargetEvaluation{
			Response:     TargetAmbiguous,
			Outcome:      trajectory.EvaluationHumanAuditionReady,
			Summary:      "human audition arbitrates this round (parked-boundary reconstruction)",
			EvidenceRefs: RoundJudgmentBoundaryEvidenceRefs(*round),
		}
		if err := synthesized.Validate(); err != nil {
			return nil, fmt.Errorf("parked-boundary target response reconstruction: %w", err)
		}
		round.TargetResponse = &synthesized
	}
	if !isCorrection && (round.TargetResponse == nil || round.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady) {
		return nil, fmt.Errorf("user judgment requires human_audition_ready target response")
	}
	if !isCorrection && (!round.UserJudgmentRequested || t.Status != StatusWaitingForUser) {
		return nil, fmt.Errorf("user judgment is not currently pending")
	}
	if evidence.SchemaVersion == "" {
		evidence.SchemaVersion = UserJudgmentEvidenceSchemaVersion
	}
	if evidence.ID == "" {
		evidence.ID = newID("judgment")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if evidence.CreatedAt.IsZero() {
		evidence.CreatedAt = now.UTC()
	}
	evidence.ConversationID = strings.TrimSpace(evidence.ConversationID)
	evidence.TurnID = strings.TrimSpace(evidence.TurnID)
	evidence.RoundID = strings.TrimSpace(evidence.RoundID)
	evidence.AuditionSessionID = strings.TrimSpace(evidence.AuditionSessionID)
	if evidence.ConversationID != t.ConversationID || evidence.TurnID != t.ID || evidence.RoundID != round.ID || evidence.AuditionSessionID != round.AuditionSessionID {
		return nil, fmt.Errorf("user judgment identity does not match pending audition session")
	}
	if err := evidence.Validate(); err != nil {
		return nil, err
	}
	if evidence.SupersedesID != "" {
		found := false
		for _, previous := range round.UserJudgmentEvidence {
			if previous.ID == evidence.SupersedesID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("user judgment supersedes unknown evidence %q", evidence.SupersedesID)
		}
	} else if len(round.UserJudgmentEvidence) > 0 {
		return nil, fmt.Errorf("user judgment already recorded; corrections must supersede an existing record")
	}
	evidence.AnalyticalEvidenceRefs = unique(evidence.AnalyticalEvidenceRefs)
	evidence.ReasonTags = unique(evidence.ReasonTags)
	if !isCorrection {
		if evidence.Preference == PreferenceB && evidence.HeardDifference == HeardDifferenceYes {
			round.TargetResponse.Response = TargetSufficient
			round.TargetResponse.Outcome = trajectory.EvaluationHumanConfirmed
		} else if evidence.HeardDifference != HeardDifferenceYes || evidence.Preference == PreferenceEqual || evidence.Preference == PreferenceUnsure {
			round.TargetResponse.Response = TargetAmbiguous
			round.TargetResponse.Outcome = trajectory.EvaluationAmbiguous
		}
	}
	round.UserJudgmentEvidence = append(round.UserJudgmentEvidence, evidence)
	if !isCorrection {
		round.UserJudgmentRequested = false
		round.Status = RoundDeciding
		round.Phase = "user_judgment"
	}
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	if !isCorrection {
		t.Status = StatusRunning
	}
	t.UpdatedAt = now.UTC()

	outcome := trajectory.EvaluationAmbiguous
	if evidence.HeardDifference == HeardDifferenceYes && (evidence.Preference == PreferenceA || evidence.Preference == PreferenceB) {
		outcome = trajectory.EvaluationHumanConfirmed
	}
	details := map[string]any{"evidence": evidence, "audition_session_id": evidence.AuditionSessionID,
		"heard_difference": evidence.HeardDifference, "preference": evidence.Preference}
	events := t.events(now, trajectory.EventUserJudgmentRecorded, round.ID, trajectory.NodeJudgment,
		"user A/B judgment recorded", round.TargetResponse.EvidenceRefs, nil, details)
	events[0].Payload.Outcome = outcome
	events[0].Payload.EvidenceRefs = append(events[0].Payload.EvidenceRefs, evidence.ID)
	return events, nil
}

// RecordUserJudgment is retained to reject the legacy click-as-preference API.
func (t *Turn) RecordUserJudgment(candidateID string, preferTreatment bool, summary string, now time.Time) ([]trajectory.Event, error) {
	return nil, fmt.Errorf("legacy candidate selection cannot record user judgment; use RecordUserJudgmentEvidence")
}
