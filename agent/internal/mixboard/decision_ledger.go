package mixboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
)

const (
	DecisionRecordSchemaVersion = "mix_decision_record.v1"
	DecisionBoardSchemaVersion  = "mix_decision_board.v1"
	DecisionEventSchemaVersion  = "mix_decision_state_event.v1"
	MixReportSchemaVersion      = "mix_report.v1"
)

const (
	DecisionVerified     = "verified"
	DecisionNeedsReview  = "needs_review"
	DecisionInconclusive = "inconclusive"
	DecisionFailed       = "failed"
	DecisionStale        = "stale"
	DecisionCancelled    = "cancelled"
	DecisionSuperseded   = "superseded"
	DecisionReverted     = "reverted"
)

// CapabilityImpact is a reporting and freshness contract. It does not grant
// authority to plan or execute actions. The listed dimensions let Mixboard
// determine which prior conclusions need review after a later applied change.
type CapabilityImpact struct {
	Reads       []string `json:"reads_dimensions,omitempty"`
	Writes      []string `json:"writes_dimensions,omitempty"`
	RecheckOn   []string `json:"recheck_on_dimensions,omitempty"`
	ProjectWide bool     `json:"project_wide,omitempty"`
}

// DecisionRefs keeps the authoritative payloads in their owning stores. A
// Mixboard decision is an index over those records, never another executor or
// persistence authority.
type DecisionRefs struct {
	CapabilitySession string   `json:"capability_session"`
	ContextBundle     string   `json:"context_bundle,omitempty"`
	Proposal          string   `json:"proposal,omitempty"`
	ActionSet         string   `json:"action_set,omitempty"`
	ProjectHistory    []string `json:"project_history,omitempty"`
	Receipt           string   `json:"receipt,omitempty"`
	BeforeObservation string   `json:"before_observation,omitempty"`
	AfterObservation  string   `json:"after_observation,omitempty"`
}

// MixDecisionRecord is immutable audit data derived from one terminal v1
// PlanningSession. Current/superseded/needs-review state is computed in the
// project board so a later capability never rewrites historical evidence.
type MixDecisionRecord struct {
	SchemaVersion     string                            `json:"schema_version"`
	RecordID          string                            `json:"record_id"`
	ProjectUUID       string                            `json:"project_uuid"`
	CapabilityID      string                            `json:"capability_id"`
	CapabilityVersion string                            `json:"capability_version,omitempty"`
	Goal              string                            `json:"goal"`
	TargetScope       []string                          `json:"target_scope,omitempty"`
	ProjectCut        orchestration.ProjectCut          `json:"project_cut"`
	TerminalStatus    orchestration.SessionStatus       `json:"terminal_status"`
	DecisionStatus    string                            `json:"decision_status"`
	Applied           bool                              `json:"applied"`
	Impact            CapabilityImpact                  `json:"impact"`
	Refs              DecisionRefs                      `json:"refs"`
	EvidenceRefs      []string                          `json:"evidence_refs,omitempty"`
	Verification      *orchestration.VerificationResult `json:"verification,omitempty"`
	Limitations       []string                          `json:"limitations,omitempty"`
	CreatedAt         time.Time                         `json:"created_at"`
	CompletedAt       time.Time                         `json:"completed_at"`
	RecordedAt        time.Time                         `json:"recorded_at"`
}

type DecisionView struct {
	RecordID          string                            `json:"record_id"`
	CapabilityID      string                            `json:"capability_id"`
	CapabilityVersion string                            `json:"capability_version,omitempty"`
	Goal              string                            `json:"goal"`
	TargetScope       []string                          `json:"target_scope,omitempty"`
	CurrentStatus     string                            `json:"current_status"`
	TerminalStatus    orchestration.SessionStatus       `json:"terminal_status"`
	ProjectCutHash    string                            `json:"project_cut_hash"`
	Impact            CapabilityImpact                  `json:"impact"`
	Refs              DecisionRefs                      `json:"refs"`
	Verification      *orchestration.VerificationResult `json:"verification,omitempty"`
	AffectedBy        []string                          `json:"affected_by,omitempty"`
	StateEventRefs    []string                          `json:"state_event_refs,omitempty"`
	SupersededBy      string                            `json:"superseded_by,omitempty"`
	Limitations       []string                          `json:"limitations,omitempty"`
	CompletedAt       time.Time                         `json:"completed_at"`
}

// DecisionStateEvent records a later lifecycle fact about an immutable
// decision record. v1 admits an explicit reverted event; the rollback itself
// remains owned and evidenced by Project History/the execution authority.
type DecisionStateEvent struct {
	SchemaVersion string    `json:"schema_version"`
	EventID       string    `json:"event_id"`
	ProjectUUID   string    `json:"project_uuid"`
	RecordID      string    `json:"record_id"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
	EvidenceRefs  []string  `json:"evidence_refs"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type DecisionReversionRequest struct {
	EventID      string
	ProjectUUID  string
	RecordID     string
	Reason       string
	EvidenceRefs []string
	OccurredAt   time.Time
}

// DecisionContextRef is bounded, advisory context for a later capability.
// It carries references and freshness status only; it never copies the
// observation, proposal, receipt, or Project History payload.
type DecisionContextRef struct {
	Ref                  string   `json:"ref"`
	RecordID             string   `json:"record_id"`
	CapabilityID         string   `json:"capability_id"`
	CurrentStatus        string   `json:"current_status"`
	RelevantDimensions   []string `json:"relevant_dimensions,omitempty"`
	RequiresRevalidation bool     `json:"requires_revalidation"`
}

type ProjectDecisionBoard struct {
	SchemaVersion    string                   `json:"schema_version"`
	ProjectUUID      string                   `json:"project_uuid"`
	LatestProjectCut orchestration.ProjectCut `json:"latest_project_cut"`
	DecisionCount    int                      `json:"decision_count"`
	VerifiedCount    int                      `json:"verified_count"`
	NeedsReviewCount int                      `json:"needs_review_count"`
	Decisions        []DecisionView           `json:"decisions"`
	OpenItems        []string                 `json:"open_items,omitempty"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

type MixReportRequest struct {
	ProjectUUID       string                    `json:"project_uuid"`
	CurrentProjectCut *orchestration.ProjectCut `json:"current_project_cut,omitempty"`
	MixIntent         map[string]any            `json:"mix_intent,omitempty"`
	FinalMeasurements map[string]any            `json:"final_measurements,omitempty"`
}

type MixReport struct {
	SchemaVersion       string                   `json:"schema_version"`
	ProjectUUID         string                   `json:"project_uuid"`
	ProjectCut          orchestration.ProjectCut `json:"project_cut"`
	GeneratedAt         time.Time                `json:"generated_at"`
	MixIntent           map[string]any           `json:"mix_intent,omitempty"`
	CapabilitySummary   []map[string]any         `json:"capability_summary"`
	DecisionTimeline    []DecisionView           `json:"decision_timeline"`
	VerificationSummary map[string]int           `json:"verification_summary"`
	UnresolvedItems     []string                 `json:"unresolved_items,omitempty"`
	FinalMeasurements   map[string]any           `json:"final_measurements,omitempty"`
	ExportReadiness     string                   `json:"export_readiness"`
	Limitations         []string                 `json:"limitations,omitempty"`
}

type DecisionWriteResult struct {
	RecordPath string               `json:"record_path"`
	BoardPath  string               `json:"board_path"`
	Record     MixDecisionRecord    `json:"record"`
	Board      ProjectDecisionBoard `json:"board"`
}

// RecordCapabilitySession projects a terminal capability session into the
// project Mixboard. It is idempotent by session ID and never edits the owning
// orchestration record.
func (s Store) RecordCapabilitySession(session orchestration.PlanningSession) (DecisionWriteResult, error) {
	if !session.Terminal() {
		return DecisionWriteResult{}, fmt.Errorf("capability session %s is not terminal", session.ID)
	}
	projectUUID := strings.TrimSpace(session.ProjectUUID)
	if projectUUID == "" {
		return DecisionWriteResult{}, fmt.Errorf("capability session %s omitted project uuid", session.ID)
	}
	record := decisionRecordFromSession(session, s.now().UTC())
	projectDir := s.projectDecisionDir(projectUUID)
	decisionDir := filepath.Join(projectDir, "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		return DecisionWriteResult{}, err
	}
	recordPath := filepath.Join(decisionDir, safePathName(record.RecordID)+".json")
	if existing, readErr := os.ReadFile(recordPath); readErr == nil {
		var prior MixDecisionRecord
		if err := json.Unmarshal(existing, &prior); err != nil {
			return DecisionWriteResult{}, fmt.Errorf("read existing mix decision %s: %w", record.RecordID, err)
		}
		record.RecordedAt = prior.RecordedAt
		if !decisionRecordsEqual(prior, record) {
			return DecisionWriteResult{}, fmt.Errorf("mix decision %s already exists with different immutable content", record.RecordID)
		}
		record = prior
	} else if !os.IsNotExist(readErr) {
		return DecisionWriteResult{}, readErr
	} else if err := writeJSON(recordPath, record); err != nil {
		return DecisionWriteResult{}, err
	}
	board, err := s.buildProjectDecisionBoard(projectUUID)
	if err != nil {
		return DecisionWriteResult{}, err
	}
	boardPath := filepath.Join(projectDir, "current_decisions.json")
	if err := writeJSON(boardPath, board); err != nil {
		return DecisionWriteResult{}, err
	}
	return DecisionWriteResult{RecordPath: recordPath, BoardPath: boardPath, Record: record, Board: board}, nil
}

func (s Store) ReadProjectDecisionBoard(projectUUID string) (ProjectDecisionBoard, error) {
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		return ProjectDecisionBoard{}, fmt.Errorf("project uuid is required")
	}
	return s.buildProjectDecisionBoard(projectUUID)
}

// RecordDecisionReversion projects a confirmed rollback/revert event without
// rewriting the original decision. Callers must supply the authoritative
// rollback/Project History evidence reference.
func (s Store) RecordDecisionReversion(req DecisionReversionRequest) (DecisionStateEvent, error) {
	projectUUID := strings.TrimSpace(req.ProjectUUID)
	recordID := strings.TrimSpace(req.RecordID)
	eventID := strings.TrimSpace(req.EventID)
	reason := strings.TrimSpace(req.Reason)
	if projectUUID == "" || recordID == "" || eventID == "" || reason == "" {
		return DecisionStateEvent{}, fmt.Errorf("project uuid, record id, event id and reason are required")
	}
	refs := decisionUniqueStrings(req.EvidenceRefs)
	if len(refs) == 0 {
		return DecisionStateEvent{}, fmt.Errorf("authoritative reversion evidence is required")
	}
	records, err := s.readProjectDecisionRecords(projectUUID)
	if err != nil {
		return DecisionStateEvent{}, err
	}
	found := false
	for _, record := range records {
		if record.RecordID == recordID {
			found = true
			break
		}
	}
	if !found {
		return DecisionStateEvent{}, fmt.Errorf("mix decision %s was not found in project %s", recordID, projectUUID)
	}
	occurredAt := req.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now().UTC()
	}
	event := DecisionStateEvent{
		SchemaVersion: DecisionEventSchemaVersion, EventID: eventID,
		ProjectUUID: projectUUID, RecordID: recordID, Status: DecisionReverted,
		Reason: reason, EvidenceRefs: refs, OccurredAt: occurredAt,
	}
	eventDir := filepath.Join(s.projectDecisionDir(projectUUID), "decision_events")
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		return DecisionStateEvent{}, err
	}
	path := filepath.Join(eventDir, safePathName(eventID)+".json")
	if existing, readErr := os.ReadFile(path); readErr == nil {
		var prior DecisionStateEvent
		if json.Unmarshal(existing, &prior) == nil && decisionStateEventMatchesRequest(prior, req, refs) {
			return prior, nil
		}
		return DecisionStateEvent{}, fmt.Errorf("decision event %s already exists with different content", eventID)
	} else if !os.IsNotExist(readErr) {
		return DecisionStateEvent{}, readErr
	}
	if err := writeJSON(path, event); err != nil {
		return DecisionStateEvent{}, err
	}
	board, err := s.buildProjectDecisionBoard(projectUUID)
	if err != nil {
		return DecisionStateEvent{}, err
	}
	if err := writeJSON(filepath.Join(s.projectDecisionDir(projectUUID), "current_decisions.json"), board); err != nil {
		return DecisionStateEvent{}, err
	}
	return event, nil
}

// RelatedDecisionRefs returns only bounded references relevant to a later
// capability. needs_review/inconclusive entries are disclosed with an
// explicit revalidation flag and must not be inherited as settled facts.
func (s Store) RelatedDecisionRefs(projectUUID, targetCapabilityID string) ([]DecisionContextRef, error) {
	board, err := s.ReadProjectDecisionBoard(projectUUID)
	if err != nil {
		return nil, err
	}
	target := capabilityImpactContract(targetCapabilityID)
	targetDimensions := decisionUniqueStrings(append(append([]string(nil), target.Reads...), target.RecheckOn...))
	out := []DecisionContextRef{}
	for _, view := range board.Decisions {
		switch view.CurrentStatus {
		case DecisionVerified, DecisionNeedsReview, DecisionInconclusive:
		default:
			continue
		}
		relevant := decisionIntersectionValues(view.Impact.Writes, targetDimensions)
		if view.CapabilityID == targetCapabilityID && len(relevant) == 0 {
			relevant = []string{"same_capability_history"}
		}
		if len(relevant) == 0 {
			continue
		}
		out = append(out, DecisionContextRef{
			Ref: "mixboard-decision:" + view.RecordID, RecordID: view.RecordID,
			CapabilityID: view.CapabilityID, CurrentStatus: view.CurrentStatus,
			RelevantDimensions: relevant, RequiresRevalidation: view.CurrentStatus != DecisionVerified,
		})
	}
	return out, nil
}

func (s Store) BuildMixReport(req MixReportRequest) (MixReport, error) {
	projectUUID := strings.TrimSpace(req.ProjectUUID)
	if projectUUID == "" && req.CurrentProjectCut != nil {
		projectUUID = strings.TrimSpace(req.CurrentProjectCut.ProjectUUID)
	}
	if projectUUID == "" {
		return MixReport{}, fmt.Errorf("project uuid is required")
	}
	board, err := s.buildProjectDecisionBoard(projectUUID)
	if err != nil {
		return MixReport{}, err
	}
	cut := board.LatestProjectCut
	limitations := []string{}
	unresolved := append([]string(nil), board.OpenItems...)
	if req.CurrentProjectCut == nil {
		limitations = append(limitations, "current_project_cut_not_provided")
		unresolved = append(unresolved, "current_project_cut_not_provided")
	} else {
		if currentUUID := strings.TrimSpace(req.CurrentProjectCut.ProjectUUID); currentUUID != "" && currentUUID != projectUUID {
			return MixReport{}, fmt.Errorf("current project cut belongs to %s, not %s", currentUUID, projectUUID)
		}
		cut = *req.CurrentProjectCut
		if decisionProjectStateDiffers(board.LatestProjectCut, cut) {
			limitations = append(limitations, "current_project_cut_has_unrecorded_changes")
			unresolved = append(unresolved, "current_project_cut_has_unrecorded_changes")
		}
	}
	verificationSummary := map[string]int{}
	byCapability := map[string]map[string]any{}
	for _, decision := range board.Decisions {
		verificationSummary[decision.CurrentStatus]++
		row := byCapability[decision.CapabilityID]
		if row == nil {
			row = map[string]any{"capability_id": decision.CapabilityID, "decision_count": 0, "current_status": decision.CurrentStatus}
			byCapability[decision.CapabilityID] = row
		}
		row["decision_count"] = row["decision_count"].(int) + 1
		row["current_status"] = decision.CurrentStatus
		row["latest_record_id"] = decision.RecordID
	}
	capabilities := make([]string, 0, len(byCapability))
	for capabilityID := range byCapability {
		capabilities = append(capabilities, capabilityID)
	}
	sort.Strings(capabilities)
	capabilitySummary := make([]map[string]any, 0, len(capabilities))
	for _, capabilityID := range capabilities {
		capabilitySummary = append(capabilitySummary, byCapability[capabilityID])
	}
	readiness := "not_assessed"
	measurementStatus := strings.ToLower(strings.TrimSpace(cleanAnyString(req.FinalMeasurements["status"])))
	if len(req.FinalMeasurements) > 0 {
		if measurementStatus == "ready" && len(unresolved) == 0 {
			readiness = "ready"
		} else {
			readiness = "needs_review"
		}
	}
	return MixReport{
		SchemaVersion: MixReportSchemaVersion, ProjectUUID: projectUUID, ProjectCut: cut,
		GeneratedAt: s.now().UTC(), MixIntent: decisionCloneMap(req.MixIntent),
		CapabilitySummary: capabilitySummary, DecisionTimeline: board.Decisions,
		VerificationSummary: verificationSummary, UnresolvedItems: decisionUniqueStrings(unresolved),
		FinalMeasurements: decisionCloneMap(req.FinalMeasurements), ExportReadiness: readiness,
		Limitations: decisionUniqueStrings(limitations),
	}, nil
}

func (s Store) projectDecisionDir(projectUUID string) string {
	if strings.EqualFold(filepath.Base(filepath.Clean(s.Root)), "sessions") {
		return filepath.Join(filepath.Dir(filepath.Clean(s.Root)), "decisions")
	}
	return filepath.Join(s.Root, "_projects", safePathName(projectUUID))
}

func (s Store) buildProjectDecisionBoard(projectUUID string) (ProjectDecisionBoard, error) {
	records, err := s.readProjectDecisionRecords(projectUUID)
	if err != nil {
		return ProjectDecisionBoard{}, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CompletedAt.Equal(records[j].CompletedAt) {
			return records[i].RecordID < records[j].RecordID
		}
		return records[i].CompletedAt.Before(records[j].CompletedAt)
	})
	events, err := s.readProjectDecisionEvents(projectUUID)
	if err != nil {
		return ProjectDecisionBoard{}, err
	}
	latestEvent := map[string]DecisionStateEvent{}
	for _, event := range events {
		prior, ok := latestEvent[event.RecordID]
		if !ok || event.OccurredAt.After(prior.OccurredAt) || (event.OccurredAt.Equal(prior.OccurredAt) && event.EventID > prior.EventID) {
			latestEvent[event.RecordID] = event
		}
	}
	views := make([]DecisionView, len(records))
	for i, record := range records {
		views[i] = decisionViewFromRecord(record)
		if event, ok := latestEvent[record.RecordID]; ok {
			views[i].CurrentStatus = event.Status
			views[i].StateEventRefs = append(views[i].StateEventRefs, "mixboard-decision-event:"+event.EventID)
			views[i].Limitations = decisionAppendUnique(views[i].Limitations, event.Reason)
		}
		if views[i].CurrentStatus == DecisionReverted {
			continue
		}
		for j := i + 1; j < len(records); j++ {
			later := records[j]
			if event, ok := latestEvent[later.RecordID]; ok && event.Status == DecisionReverted {
				continue
			}
			if !later.Applied || !decisionScopesOverlap(record, later) {
				continue
			}
			if views[i].CurrentStatus == DecisionVerified && record.CapabilityID == later.CapabilityID && later.DecisionStatus == DecisionVerified {
				views[i].CurrentStatus = DecisionSuperseded
				views[i].SupersededBy = later.RecordID
				views[i].AffectedBy = decisionAppendUnique(views[i].AffectedBy, later.RecordID)
				continue
			}
			if decisionIntersects(record.Impact.RecheckOn, later.Impact.Writes) {
				if views[i].CurrentStatus == DecisionVerified {
					views[i].CurrentStatus = DecisionNeedsReview
				}
				views[i].AffectedBy = decisionAppendUnique(views[i].AffectedBy, later.RecordID)
			}
		}
	}
	board := ProjectDecisionBoard{
		SchemaVersion: DecisionBoardSchemaVersion, ProjectUUID: projectUUID,
		DecisionCount: len(views), Decisions: views, UpdatedAt: s.now().UTC(),
	}
	for index := len(records) - 1; index >= 0; index-- {
		if strings.TrimSpace(records[index].ProjectCut.ProjectUUID) != "" || strings.TrimSpace(records[index].ProjectCut.Hash) != "" {
			board.LatestProjectCut = records[index].ProjectCut
			break
		}
	}
	for _, view := range views {
		switch view.CurrentStatus {
		case DecisionVerified:
			board.VerifiedCount++
		case DecisionNeedsReview, DecisionInconclusive:
			board.NeedsReviewCount++
			board.OpenItems = append(board.OpenItems, view.RecordID+":"+view.CurrentStatus)
		case DecisionFailed, DecisionStale:
			board.OpenItems = append(board.OpenItems, view.RecordID+":"+view.CurrentStatus)
		}
	}
	board.OpenItems = decisionUniqueStrings(board.OpenItems)
	return board, nil
}

func (s Store) readProjectDecisionRecords(projectUUID string) ([]MixDecisionRecord, error) {
	dir := filepath.Join(s.projectDecisionDir(projectUUID), "decisions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	records := make([]MixDecisionRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var record MixDecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("read mix decision %s: %w", entry.Name(), err)
		}
		if record.SchemaVersion != DecisionRecordSchemaVersion || record.ProjectUUID != projectUUID {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func (s Store) readProjectDecisionEvents(projectUUID string) ([]DecisionStateEvent, error) {
	dir := filepath.Join(s.projectDecisionDir(projectUUID), "decision_events")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	events := make([]DecisionStateEvent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var event DecisionStateEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("read mix decision event %s: %w", entry.Name(), err)
		}
		if event.SchemaVersion == DecisionEventSchemaVersion && event.ProjectUUID == projectUUID && event.Status == DecisionReverted {
			events = append(events, event)
		}
	}
	return events, nil
}

func decisionRecordFromSession(session orchestration.PlanningSession, recordedAt time.Time) MixDecisionRecord {
	proposal := session.ActiveProposal
	if session.FrozenPlan != nil {
		proposal = &session.FrozenPlan.Proposal
	}
	targets := []string{}
	if proposal != nil {
		targets = append(targets, proposal.TargetScope...)
	}
	if session.FrozenPlan != nil {
		for _, action := range session.FrozenPlan.ActionSet.Actions {
			targets = append(targets, action.TargetRef)
		}
	}
	targets = decisionUniqueStrings(targets)
	refs := DecisionRefs{CapabilitySession: "capability-session:" + session.ID}
	cut := orchestration.ProjectCut{}
	if session.FrozenPlan != nil {
		cut = session.FrozenPlan.ProjectCut
		refs.ContextBundle = decisionPrefixRef("capability-pack:", session.FrozenPlan.ContextBundleID)
		refs.ActionSet = decisionPrefixRef("action-set:", session.FrozenPlan.ActionSet.ID)
		refs.BeforeObservation = decisionPrefixRef("mix.observe:", session.FrozenPlan.PreviousObservationID)
	}
	if proposal != nil {
		refs.Proposal = fmt.Sprintf("proposal:%s:%d", proposal.ID, proposal.Revision)
	}
	evidence := []string{}
	limitations := []string{}
	applied := false
	var verification *orchestration.VerificationResult
	if session.Execution != nil {
		refs.ProjectHistory = append([]string(nil), session.Execution.PersistenceRefs...)
		refs.Receipt = decisionPrefixRef("receipt:", session.Execution.ReceiptRef)
		verification = decisionCloneVerification(session.Execution.VerificationResult)
		if verification != nil {
			evidence = append(evidence, verification.EvidenceRefs...)
			refs.AfterObservation = decisionAfterObservationRef(verification.EvidenceRefs)
			if strings.TrimSpace(verification.Summary) != "" && !strings.EqualFold(verification.Status, "pass") {
				limitations = append(limitations, verification.Summary)
			}
		}
		for _, receipt := range session.Execution.Receipts {
			evidence = append(evidence, receipt.EvidenceRefs...)
			if decisionReceiptApplied(receipt) {
				applied = true
			}
		}
	}
	completedAt := session.UpdatedAt
	if completedAt.IsZero() {
		completedAt = recordedAt
	}
	return MixDecisionRecord{
		SchemaVersion: DecisionRecordSchemaVersion,
		RecordID:      "mixdec_" + safePathName(session.ID), ProjectUUID: session.ProjectUUID,
		CapabilityID: session.Invocation.CapabilityID, CapabilityVersion: session.Invocation.CapabilityVer,
		Goal: session.Goal, TargetScope: targets, ProjectCut: cut, TerminalStatus: session.Status,
		DecisionStatus: decisionStatusFromSession(session), Applied: applied,
		Impact: capabilityImpactContract(session.Invocation.CapabilityID), Refs: refs,
		EvidenceRefs: decisionUniqueStrings(evidence), Verification: verification,
		Limitations: decisionUniqueStrings(limitations), CreatedAt: session.CreatedAt,
		CompletedAt: completedAt, RecordedAt: recordedAt,
	}
}

func capabilityImpactContract(capabilityID string) CapabilityImpact {
	switch strings.TrimSpace(capabilityID) {
	case "static_mix.static_balance.v0":
		return CapabilityImpact{
			Reads:       []string{"track_roles", "static_levels", "headroom", "mix_style"},
			Writes:      []string{"static_levels", "static_level_relationship", "track_fader"},
			RecheckOn:   []string{"track_roles", "track_content", "track_fader", "clip_gain", "pan_layout", "static_eq", "dynamics", "spatial_depth", "automation", "routing"},
			ProjectWide: true,
		}
	case "static_mix.pan_layout.v0":
		return CapabilityImpact{
			Reads:       []string{"track_roles", "static_level_relationship", "stereo_relationship", "routing", "channel_layout", "mix_style"},
			Writes:      []string{"pan_layout", "track_pan"},
			RecheckOn:   []string{"track_roles", "track_content", "track_pan", "stereo_width", "routing"},
			ProjectWide: true,
		}
	case "static_mix.low_end_relation.v0":
		return CapabilityImpact{
			Reads:       []string{"track_roles", "static_levels", "low_end_relationship", "band_energy", "static_eq"},
			Writes:      []string{"low_end_relationship", "static_eq"},
			RecheckOn:   []string{"track_roles", "track_content", "track_fader", "clip_gain", "static_eq", "dynamics", "routing"},
			ProjectWide: true,
		}
	case "fine_mix.frequency_cleanup.v1":
		return CapabilityImpact{
			Reads:       []string{"frequency_relationship", "track_roles", "static_levels", "low_end_relationship", "band_energy", "static_eq", "plugin_chain"},
			Writes:      []string{"frequency_balance", "spectral_tone", "static_eq", "plugin_chain"},
			RecheckOn:   []string{"track_roles", "track_content", "track_fader", "clip_gain", "static_eq", "dynamics", "spatial_depth", "automation", "routing", "plugin_chain"},
			ProjectWide: true,
		}
	case "fine_mix.dynamic_control.v1":
		return CapabilityImpact{
			Reads:       []string{"track_roles", "time_dynamics", "peak_structure", "activity_structure", "frequency_time_events", "transient_structure", "band_dynamics", "plugin_chain"},
			Writes:      []string{"dynamics", "plugin_chain"},
			RecheckOn:   []string{"track_roles", "track_content", "track_fader", "clip_gain", "static_eq", "dynamics", "spatial_depth", "automation", "routing", "plugin_chain"},
			ProjectWide: true,
		}
	default:
		return CapabilityImpact{}
	}
}

func decisionStatusFromSession(session orchestration.PlanningSession) string {
	switch session.Status {
	case orchestration.StatusCompleted:
		if session.Execution != nil && session.Execution.VerificationResult != nil && strings.EqualFold(session.Execution.VerificationResult.Status, "pass") {
			return DecisionVerified
		}
		return DecisionInconclusive
	case orchestration.StatusNeedsReview:
		return DecisionNeedsReview
	case orchestration.StatusFailed:
		return DecisionFailed
	case orchestration.StatusStale:
		return DecisionStale
	case orchestration.StatusCancelled:
		return DecisionCancelled
	default:
		return DecisionInconclusive
	}
}

func decisionViewFromRecord(record MixDecisionRecord) DecisionView {
	return DecisionView{
		RecordID: record.RecordID, CapabilityID: record.CapabilityID,
		CapabilityVersion: record.CapabilityVersion, Goal: record.Goal,
		TargetScope: append([]string(nil), record.TargetScope...), CurrentStatus: record.DecisionStatus,
		TerminalStatus: record.TerminalStatus, ProjectCutHash: record.ProjectCut.Hash,
		Impact: record.Impact, Refs: record.Refs, Verification: decisionCloneVerification(record.Verification),
		Limitations: append([]string(nil), record.Limitations...), CompletedAt: record.CompletedAt,
	}
}

func decisionScopesOverlap(a, b MixDecisionRecord) bool {
	if a.Impact.ProjectWide || b.Impact.ProjectWide || len(a.TargetScope) == 0 || len(b.TargetScope) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, target := range a.TargetScope {
		set[strings.ToLower(strings.TrimSpace(target))] = true
	}
	for _, target := range b.TargetScope {
		if set[strings.ToLower(strings.TrimSpace(target))] {
			return true
		}
	}
	return false
}

func decisionIntersects(a, b []string) bool {
	set := map[string]bool{}
	for _, value := range a {
		set[strings.ToLower(strings.TrimSpace(value))] = true
	}
	for _, value := range b {
		if set[strings.ToLower(strings.TrimSpace(value))] {
			return true
		}
	}
	return false
}

func decisionIntersectionValues(a, b []string) []string {
	set := map[string]bool{}
	for _, value := range a {
		set[strings.ToLower(strings.TrimSpace(value))] = true
	}
	out := []string{}
	for _, value := range b {
		key := strings.ToLower(strings.TrimSpace(value))
		if key != "" && set[key] {
			out = append(out, key)
		}
	}
	return decisionUniqueStrings(out)
}

func decisionProjectStateDiffers(recorded, current orchestration.ProjectCut) bool {
	if strings.TrimSpace(recorded.ProjectUUID) == "" || strings.TrimSpace(current.ProjectUUID) == "" {
		return false
	}
	if recorded.ProjectUUID != current.ProjectUUID || recorded.ProjectEpoch != current.ProjectEpoch {
		return true
	}
	if strings.TrimSpace(recorded.BaseProjectRevision) != "" && strings.TrimSpace(current.BaseProjectRevision) != "" {
		return recorded.BaseProjectRevision != current.BaseProjectRevision
	}
	return recorded.Hash != "" && current.Hash != "" && recorded.Hash != current.Hash
}

func decisionRecordsEqual(a, b MixDecisionRecord) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func decisionStateEventMatchesRequest(event DecisionStateEvent, req DecisionReversionRequest, refs []string) bool {
	if event.SchemaVersion != DecisionEventSchemaVersion || event.Status != DecisionReverted ||
		event.EventID != strings.TrimSpace(req.EventID) || event.ProjectUUID != strings.TrimSpace(req.ProjectUUID) ||
		event.RecordID != strings.TrimSpace(req.RecordID) || event.Reason != strings.TrimSpace(req.Reason) {
		return false
	}
	left, _ := json.Marshal(event.EvidenceRefs)
	right, _ := json.Marshal(refs)
	if string(left) != string(right) {
		return false
	}
	return req.OccurredAt.IsZero() || event.OccurredAt.Equal(req.OccurredAt.UTC())
}

func decisionReceiptApplied(receipt orchestration.ActionReceipt) bool {
	switch strings.ToLower(strings.TrimSpace(receipt.Status)) {
	case "ok", "pass", "passed", "success", "succeeded", "applied", "completed":
		return true
	default:
		return false
	}
}

func decisionAfterObservationRef(refs []string) string {
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if strings.HasPrefix(ref, "mix.observe:") && !strings.HasPrefix(ref, "mix.observe.revision:") {
			return ref
		}
	}
	return ""
}

func decisionPrefixRef(prefix, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, prefix) {
		return value
	}
	return prefix + value
}

func decisionAppendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func decisionUniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func decisionCloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func decisionCloneVerification(in *orchestration.VerificationResult) *orchestration.VerificationResult {
	if in == nil {
		return nil
	}
	out := *in
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	return &out
}
