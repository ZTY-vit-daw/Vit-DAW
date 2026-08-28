package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/orchestrationcontroller"
)

const (
	capacityAssessmentSchema  = "free_state_capacity_assessment.v1"
	capabilityRouteSchema     = "capability_route_decision.v1"
	capabilityEntryPlanSchema = "capability_entry_plan.v1"

	capacityAssessmentContextKey  = "free_state_capacity_assessment"
	capabilityRouteContextKey     = "capability_route_decision"
	capabilityEntryPlanContextKey = "capability_entry_plan"

	capabilityFreeState  = "free_state"
	capabilityProjectMix = "mixing_capability_layer"
)

type CapacityObservedFacts struct {
	ProjectUUID             string   `json:"project_uuid,omitempty"`
	ProjectRevision         string   `json:"project_revision,omitempty"`
	TrackCount              int      `json:"track_count"`
	ActiveAudioTrackCount   int      `json:"active_audio_track_count"`
	DurationSeconds         float64  `json:"duration_seconds,omitempty"`
	PluginCount             int      `json:"plugin_count"`
	AutomationLaneCount     int      `json:"automation_lane_count"`
	TrackGroupCount         int      `json:"track_group_count"`
	RoutingEdgeCount        int      `json:"routing_edge_count"`
	RelationshipBreadth     int      `json:"relationship_breadth"`
	RequestScope            string   `json:"request_scope"`
	TaskHistoryAvailable    bool     `json:"task_history_available"`
	PriorCapabilityStage    string   `json:"prior_capability_stage,omitempty"`
	PriorAssessmentRevision string   `json:"prior_assessment_revision,omitempty"`
	EvidenceRefs            []string `json:"evidence_refs,omitempty"`
	MissingFacts            []string `json:"missing_facts,omitempty"`
}

type CapacityComplexityFactor struct {
	Code      string  `json:"code"`
	Observed  float64 `json:"observed"`
	Weight    int     `json:"weight"`
	Triggered bool    `json:"triggered"`
	Reason    string  `json:"reason"`
}

// FreeStateCapacityAssessment is the host-owned, durable decision input. It
// contains structural facts only; no processor, target track or acoustic
// diagnosis belongs at this boundary.
type FreeStateCapacityAssessment struct {
	SchemaVersion      string                     `json:"schema_version"`
	Authority          string                     `json:"authority"`
	ObservedFacts      CapacityObservedFacts      `json:"observed_facts"`
	ComplexityFactors  []CapacityComplexityFactor `json:"complexity_factors"`
	CapacityScore      int                        `json:"capacity_score"`
	CapacityLevel      string                     `json:"capacity_level"`
	SelectedCapability string                     `json:"selected_capability"`
	ReasonCodes        []string                   `json:"reason_codes"`
	Confidence         float64                    `json:"confidence"`
	ProjectRevision    string                     `json:"project_revision,omitempty"`
	ObservedAt         time.Time                  `json:"observed_at"`
}

type CapabilityEntryPlan struct {
	SchemaVersion string `json:"schema_version"`
	Capability    string `json:"capability"`
	Stage         string `json:"stage,omitempty"`
	CapabilityID  string `json:"capability_id,omitempty"`
	Source        string `json:"source"`
	Reason        string `json:"reason"`
}

type CapabilityRouteRecord struct {
	SchemaVersion   string                       `json:"schema_version"`
	TaskID          string                       `json:"task_id"`
	GoalID          string                       `json:"goal_id"`
	RunID           string                       `json:"run_id"`
	ConversationID  string                       `json:"conversation_id"`
	OriginalIntent  string                       `json:"original_intent"`
	SemanticEntry   map[string]any               `json:"semantic_entry"`
	Assessment      *FreeStateCapacityAssessment `json:"capacity_assessment,omitempty"`
	Controller      string                       `json:"controller"`
	EntryPlan       *CapabilityEntryPlan         `json:"entry_plan,omitempty"`
	ProjectRevision string                       `json:"project_revision,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
}

func (s *Server) ensureCapabilityRoutingTask(userText string, requestContext map[string]any) (map[string]any, CapabilityRouteRecord) {
	goalID, runID := s.freshAgentLoopGoalIDs(requestContext)
	goal := s.harness.EnsureGoal(goalID, runID, userText)
	record := CapabilityRouteRecord{GoalID: goal.GoalID, RunID: goal.RunID, OriginalIntent: strings.TrimSpace(userText)}
	if goal.Task != nil {
		record.TaskID = goal.Task.TaskID
		record.OriginalIntent = firstNonEmpty(goal.Task.OriginalIntent, record.OriginalIntent)
	}
	return mergeContext(requestContext, map[string]any{
		"task_id": record.TaskID, "goal_id": record.GoalID, "run_id": record.RunID,
		"original_intent": record.OriginalIntent,
	}), record
}

func (s *Server) planObservationFirstCapabilityRoute(ctx context.Context, conversationID, userText string, requestContext map[string]any,
	entry semanticEntryDecision, identity CapabilityRouteRecord) (semanticEntryDecision, CapabilityRouteRecord, error) {
	now := time.Now().UTC()
	record := identity
	record.SchemaVersion = capabilityRouteSchema
	record.ConversationID = conversationID
	record.SemanticEntry = semanticEntryDecisionMap(entry)
	record.CreatedAt, record.UpdatedAt = now, now

	if entry.Route == semanticEntryRouteObservation || entry.Route == semanticEntryRouteOpenSemantic {
		facts := s.observeCapacityProjectFacts(ctx, entry.TargetScope)
		previous := s.previousCapabilityRoute(record.TaskID, conversationID)
		if previous.SchemaVersion == capabilityRouteSchema {
			facts.TaskHistoryAvailable = true
			facts.PriorAssessmentRevision = previous.ProjectRevision
			if previous.EntryPlan != nil {
				facts.PriorCapabilityStage = previous.EntryPlan.Stage
			}
		}
		explicitFixed := explicitProjectMixInvocation(userText) || explicitCapabilityStage(userText) != "" || capabilityRouteWasExplicit(previous)
		assessment := assessFreeStateCapacity(facts, explicitFixed, now)
		record.Assessment = &assessment
		record.ProjectRevision = assessment.ProjectRevision
		if assessment.SelectedCapability == capabilityProjectMix {
			entry.Controller = string(orchestrationcontroller.ProjectMixWorkflow)
			record.EntryPlan = capabilityEntryPlan(userText, previous, assessment)
		} else {
			// An open problem-finding request is observation-first, but it is
			// still an improvement task once product capacity confirms that the
			// free-state controller owns it. Promote only this narrow intent;
			// explicit read-only status requests remain diagnostic-only.
			entry = promoteOpenImprovementEntry(userText, entry)
			entry.Controller = string(orchestrationcontroller.MinimalAudioClosure)
		}
		record.Controller = entry.Controller
		entry.Reason = firstNonEmpty(capabilityRouteReason(assessment), entry.Reason)
		// Persist the post-capacity contract. Observation is only the
		// semantic entry point; an untargeted improvement request becomes an
		// open semantic loop after structural capacity admits free-state.
		record.SemanticEntry = semanticEntryDecisionMap(entry)
	} else {
		entry.Controller = defaultSemanticEntryController(entry.Route)
		record.Controller = entry.Controller
	}
	if _, err := orchestrationControllerDecision(entry); err != nil {
		return semanticEntryDecision{}, CapabilityRouteRecord{}, err
	}
	if record.Assessment != nil {
		s.storeCapabilityRoute(record)
	}
	return entry, record, nil
}

// capabilityRouteSemanticEntry returns the host-authoritative semantic entry
// for a persisted capability route. Older snapshots recorded the pre-promotion
// observation entry even after an open improvement task was admitted. The
// migration is deliberately narrow: it requires a validated free-state
// assessment, the minimal audio closure, and the original task intent to be an
// open improvement request. It never turns a read-only status task into an
// action task and never calls the semantic-entry model again.
func capabilityRouteSemanticEntry(record CapabilityRouteRecord) (semanticEntryDecision, bool) {
	if record.SchemaVersion != capabilityRouteSchema || record.Assessment == nil ||
		record.Assessment.SelectedCapability != capabilityFreeState ||
		record.Controller != string(orchestrationcontroller.MinimalAudioClosure) ||
		!openProjectImprovementIntent(record.OriginalIntent) {
		return semanticEntryDecision{}, false
	}
	row := record.SemanticEntry
	entry := semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema,
		Route:         firstStringFromMap(row, "route"), TargetScope: firstStringFromMap(row, "target_scope"),
		ControlMode: firstStringFromMap(row, "control_mode"), UserAuthorization: firstStringFromMap(row, "user_authorization"),
		Confidence: semanticEntryFloatValue(row["confidence"]), Reason: firstStringFromMap(row, "reason"),
		Controller: record.Controller,
	}
	entry = promoteOpenImprovementEntry(record.OriginalIntent, entry)
	entry.Controller = record.Controller
	entry.Reason = firstNonEmpty(capabilityRouteReason(*record.Assessment), entry.Reason)
	if entry.TargetScope == "" {
		entry.TargetScope = semanticEntryScopeProjectContext
	}
	if _, err := orchestrationControllerDecision(entry); err != nil {
		return semanticEntryDecision{}, false
	}
	return entry, true
}

func promoteOpenImprovementEntry(userText string, entry semanticEntryDecision) semanticEntryDecision {
	if entry.Route != semanticEntryRouteObservation || !openProjectImprovementIntent(userText) {
		return entry
	}
	entry.Route = semanticEntryRouteOpenSemantic
	entry.ControlMode = semanticEntryControlSemanticLoop
	entry.UserAuthorization = semanticEntryAuthorizationAction
	return entry
}

func openProjectImprovementIntent(userText string) bool {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" || agentLoopTextHasAny(lower, "状态汇报", "状态报告", "progress report", "blackboard status") {
		return false
	}
	hasProject := agentLoopTextHasAny(lower, "工程", "项目", "project")
	hasOpenProblem := agentLoopTextHasAny(lower,
		"有什么问题", "哪些问题", "哪里有问题", "问题", "风险", "混音检查", "检查并改善", "可以改善",
		"what is wrong", "problems", "issues", "mix check", "improve", "improvement")
	return hasProject && hasOpenProblem
}

func capabilityRouteWasExplicit(record CapabilityRouteRecord) bool {
	if record.Assessment == nil {
		return false
	}
	for _, reason := range record.Assessment.ReasonCodes {
		if reason == "explicit_fixed_workflow_request" {
			return true
		}
	}
	return false
}

// refreshCapabilityRouteForRevision re-observes structure inside the same
// Task. It never calls semantic entry again and never changes original intent.
func (s *Server) refreshCapabilityRouteForRevision(ctx context.Context, conversationID string, requestContext map[string]any) (map[string]any, error) {
	previous := s.previousCapabilityRoute(firstStringFromMap(requestContext, "task_id"), conversationID)
	if previous.SchemaVersion != capabilityRouteSchema || previous.Assessment == nil {
		return requestContext, nil
	}
	semantic := semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema,
		Route:         firstStringFromMap(previous.SemanticEntry, "route"), Controller: previous.Controller,
		TargetScope:       firstStringFromMap(previous.SemanticEntry, "target_scope"),
		ControlMode:       firstStringFromMap(previous.SemanticEntry, "control_mode"),
		UserAuthorization: firstStringFromMap(previous.SemanticEntry, "user_authorization"),
		Confidence:        semanticEntryFloatValue(previous.SemanticEntry["confidence"]),
		Reason:            firstStringFromMap(previous.SemanticEntry, "reason"),
	}
	facts := s.observeCapacityProjectFacts(ctx, semantic.TargetScope)
	if facts.ProjectRevision == previous.ProjectRevision {
		return contextWithCapabilityRoute(requestContext, previous), nil
	}
	facts.TaskHistoryAvailable = true
	facts.PriorAssessmentRevision = previous.ProjectRevision
	if previous.EntryPlan != nil {
		facts.PriorCapabilityStage = previous.EntryPlan.Stage
	}
	explicitFixed := false
	for _, reason := range previous.Assessment.ReasonCodes {
		if reason == "explicit_fixed_workflow_request" {
			explicitFixed = true
			break
		}
	}
	assessment := assessFreeStateCapacity(facts, explicitFixed, time.Now().UTC())
	nextController := string(orchestrationcontroller.MinimalAudioClosure)
	if assessment.SelectedCapability == capabilityProjectMix {
		nextController = string(orchestrationcontroller.ProjectMixWorkflow)
	}
	if s.controllerOwners != nil {
		if owner, active := s.controllerOwners.Active(conversationID); active && string(owner.Controller) != nextController {
			if err := s.settleControllerForCapacityReroute(conversationID, owner, assessment.ProjectRevision); err != nil {
				return requestContext, err
			}
		}
	}
	semantic.Controller = nextController
	semantic.Reason = capabilityRouteReason(assessment)
	previous.Assessment = &assessment
	previous.Controller = nextController
	previous.ProjectRevision = assessment.ProjectRevision
	previous.UpdatedAt = time.Now().UTC()
	if nextController == string(orchestrationcontroller.ProjectMixWorkflow) {
		previous.EntryPlan = capabilityEntryPlan("", previous, assessment)
	} else {
		previous.EntryPlan = nil
	}
	s.storeCapabilityRoute(previous)
	out := contextWithSemanticEntryDecision(requestContext, semantic)
	return contextWithCapabilityRoute(out, previous), nil
}

func (s *Server) settleControllerForCapacityReroute(conversationID string, owner orchestrationcontroller.Owner, projectRevision string) error {
	now := time.Now().UTC()
	if owner.Controller == orchestrationcontroller.MinimalAudioClosure && s.audioClosures != nil {
		if closure, active := s.audioClosures.ActiveForConversation(conversationID); active && !closure.Terminal() {
			settled, err := (audioclosure.Driver{}).Settle(closure, closure.Revision, audioclosure.StopProjectRevisionStale,
				"project revision changed the capacity route to a different controller", false, now)
			if err != nil {
				return err
			}
			if err := s.audioClosures.Save(settled, closure.Revision); err != nil {
				return err
			}
			s.settleAudioClosureOwner(settled)
			return nil
		}
	}
	if s.controllerOwners == nil {
		return nil
	}
	current, active := s.controllerOwners.Active(conversationID)
	if !active || current.ControllerID != owner.ControllerID {
		return nil
	}
	_, err := s.controllerOwners.Settle(conversationID, owner.ControllerID, current.Revision,
		"project_revision_capacity_reroute:"+strings.TrimSpace(projectRevision), now)
	return err
}

func (s *Server) observeCapacityProjectFacts(ctx context.Context, requestScope string) CapacityObservedFacts {
	state := s.harness.UserStateSummary(ctx)
	// Refresh structural state through the read-only product command when a
	// live kernel exists. Harness.RefreshShadow prefers the authoritative VSP
	// snapshot and only falls back to the legacy state when VSP is unavailable;
	// this prevents a legacy get_project_state response (which may carry the
	// historical zero/absent revision) from overwriting canonical binding.
	if s.kernel != nil {
		s.harness.RefreshShadow(ctx, "capacity_project_facts")
		state = s.harness.UserStateSummary(ctx)
	}
	facts := capacityFactsFromProjectState(state, requestScope)
	return facts
}

func capacityFactsFromProjectState(state map[string]any, requestScope string) CapacityObservedFacts {
	tracks := capacityRows(state["tracks"])
	facts := CapacityObservedFacts{
		ProjectUUID:     firstStringFromMap(state, "project_uuid", "project_id"),
		ProjectRevision: firstNonEmpty(firstStringFromMap(state, "project_revision", "graph_revision", "snapshot_hash"), "unknown"),
		TrackCount:      intNumber(state["track_count"]), TrackGroupCount: intNumber(state["track_group_count"]),
		RequestScope: strings.TrimSpace(requestScope),
	}
	if facts.TrackCount == 0 {
		facts.TrackCount = len(tracks)
	}
	if facts.TrackGroupCount == 0 {
		facts.TrackGroupCount = len(capacityRows(firstNonNilCapacityValue(state["track_groups"], state["groups"])))
	}
	for _, track := range tracks {
		if contextBool(track, "is_audio_track") || strings.Contains(strings.ToLower(firstStringFromMap(track, "track_type", "type")), "audio") || strings.Contains(strings.ToLower(firstStringFromMap(track, "track_type", "type")), "hybrid") {
			facts.ActiveAudioTrackCount++
		}
		facts.PluginCount += capacityMaxInt(len(capacityRows(firstNonNilCapacityValue(track["plugins"], track["rack"]))), intNumber(track["plugin_count"]))
		facts.AutomationLaneCount += capacityCountCollection(firstNonNilCapacityValue(track["automation_lanes"], track["automation"], track["envelopes"], track["automation_lane_count"]))
		facts.RoutingEdgeCount += capacityCountCollection(firstNonNilCapacityValue(track["sends"], track["outputs"], track["routes"], track["routing"], track["routing_edge_count"]))
		groupMemberships := capacityCountCollection(firstNonNilCapacityValue(track["track_group_ids"], track["group_ids"], track["track_groups"]))
		facts.RoutingEdgeCount += groupMemberships
		for _, clip := range capacityRows(track["clips"]) {
			start := capacityFloat(firstNonNilCapacityValue(clip["start_seconds"], clip["start"], clip["position_seconds"]))
			end := capacityFloat(firstNonNilCapacityValue(clip["end_seconds"], clip["end"]))
			if end <= 0 {
				end = start + capacityFloat(firstNonNilCapacityValue(clip["duration_seconds"], clip["length_seconds"], clip["length"]))
			}
			if end > facts.DurationSeconds {
				facts.DurationSeconds = end
			}
		}
	}
	if duration := capacityFloat(firstNonNilCapacityValue(state["duration_seconds"], state["project_duration_seconds"], state["length_seconds"])); duration > facts.DurationSeconds {
		facts.DurationSeconds = duration
	}
	facts.RelationshipBreadth = facts.TrackGroupCount + facts.RoutingEdgeCount
	evidenceID := "project.state"
	if facts.ProjectUUID != "" {
		evidenceID += ":" + facts.ProjectUUID
	}
	if facts.ProjectRevision != "" {
		evidenceID += ":" + facts.ProjectRevision
	}
	facts.EvidenceRefs = []string{evidenceID}
	if facts.ProjectUUID == "" {
		facts.MissingFacts = append(facts.MissingFacts, "project_uuid")
	}
	if facts.ProjectRevision == "unknown" {
		facts.MissingFacts = append(facts.MissingFacts, "project_revision")
	}
	if facts.DurationSeconds <= 0 {
		facts.MissingFacts = append(facts.MissingFacts, "duration_seconds")
	}
	sort.Strings(facts.MissingFacts)
	return facts
}

func assessFreeStateCapacity(facts CapacityObservedFacts, explicitFixedWorkflow bool, now time.Time) FreeStateCapacityAssessment {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	factors := []CapacityComplexityFactor{
		capacityFactor("large_track_set", float64(facts.TrackCount), 25, facts.TrackCount >= 20, "twenty or more user tracks expand whole-project coverage"),
		capacityFactor("long_timeline", facts.DurationSeconds, 15, facts.DurationSeconds >= 300, "five minutes or more increases observation span"),
		capacityFactor("dense_processing_graph", float64(facts.PluginCount), 20, facts.PluginCount >= 40, "forty or more processors increase context pressure"),
		capacityFactor("dense_routing_graph", float64(facts.RelationshipBreadth), 20, facts.RelationshipBreadth >= 12, "routing and grouping relationships require project-wide coordination"),
		capacityFactor("automation_heavy", float64(facts.AutomationLaneCount), 15, facts.AutomationLaneCount >= 24, "automation breadth increases cross-stage state"),
	}
	score, triggered := 0, 0
	reasons := []string{}
	for _, factor := range factors {
		if factor.Triggered {
			score += factor.Weight
			triggered++
			reasons = append(reasons, factor.Code)
		}
	}
	selected := capabilityFreeState
	level := "within_free_state"
	switch {
	case explicitFixedWorkflow && facts.RequestScope == semanticEntryScopeProjectContext:
		selected, level = capabilityProjectMix, "fixed_capability_requested"
		reasons = append([]string{"explicit_fixed_workflow_request"}, reasons...)
	case facts.RequestScope == semanticEntryScopeCurrentSelection:
		reasons = append([]string{"bounded_request_scope"}, reasons...)
	case score >= 45 && triggered >= 2:
		selected, level = capabilityProjectMix, "exceeds_free_state"
	case score >= 35:
		level = "near_free_state_limit"
	}
	if len(reasons) == 0 {
		reasons = []string{"small_or_moderate_project"}
	}
	confidence := 0.94
	if len(facts.MissingFacts) > 0 {
		confidence = 0.78
	}
	return FreeStateCapacityAssessment{
		SchemaVersion: capacityAssessmentSchema, Authority: "product_runtime",
		ObservedFacts: facts, ComplexityFactors: factors, CapacityScore: score, CapacityLevel: level,
		SelectedCapability: selected, ReasonCodes: reasons, Confidence: confidence,
		ProjectRevision: facts.ProjectRevision, ObservedAt: now.UTC(),
	}
}

func capabilityEntryPlan(userText string, previous CapabilityRouteRecord, assessment FreeStateCapacityAssessment) *CapabilityEntryPlan {
	stage, source := explicitCapabilityStage(userText), "explicit_user_request"
	if stage == "" && previous.EntryPlan != nil && previous.EntryPlan.Stage != "" {
		stage, source = previous.EntryPlan.Stage, "task_history"
	}
	if stage == "" {
		stage, source = "A2", "default_after_capacity_routing"
	}
	capabilityID := capabilityIDForStage(stage)
	return &CapabilityEntryPlan{SchemaVersion: capabilityEntryPlanSchema, Capability: assessment.SelectedCapability,
		Stage: stage, CapabilityID: capabilityID, Source: source,
		Reason: "entry is selected after structural capacity assessment; capability stages remain non-linear"}
}

var explicitCapabilityStagePattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([a-f][1-9])(?:[^a-z0-9]|$)`)

func explicitCapabilityStage(text string) string {
	match := explicitCapabilityStagePattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) < 2 {
		return ""
	}
	return strings.ToUpper(match[1])
}

func capabilityIDForStage(stage string) string {
	switch strings.ToUpper(strings.TrimSpace(stage)) {
	case "A2":
		return "project_prep.technical_integrity.v0"
	case "A3":
		return "project_prep.track_organization.v0"
	case "C1":
		return "fine_mix.frequency_cleanup.v0"
	case "C2":
		return "fine_mix.dynamic_control.v0"
	case "C3":
		return "fine_mix.space_depth.v0"
	default:
		return "mixing_capability_stage." + strings.ToLower(strings.TrimSpace(stage))
	}
}

func capabilityRouteReason(assessment FreeStateCapacityAssessment) string {
	return fmt.Sprintf("product runtime selected %s after project structure observation (score=%d, reasons=%s)",
		assessment.SelectedCapability, assessment.CapacityScore, strings.Join(assessment.ReasonCodes, ","))
}

func contextWithCapabilityRoute(requestContext map[string]any, record CapabilityRouteRecord) map[string]any {
	values := map[string]any{capabilityRouteContextKey: capabilityRouteRecordMap(record)}
	if record.Assessment != nil {
		values[capacityAssessmentContextKey] = *record.Assessment
		if record.Assessment.ProjectRevision != "" {
			values["project_revision"] = record.Assessment.ProjectRevision
		}
		if record.Assessment.ObservedFacts.ProjectUUID != "" {
			values["project_uuid"] = record.Assessment.ObservedFacts.ProjectUUID
		}
	}
	if record.EntryPlan != nil {
		values[capabilityEntryPlanContextKey] = *record.EntryPlan
	}
	return mergeContext(requestContext, values)
}

func (s *Server) storeCapabilityRoute(record CapabilityRouteRecord) {
	if s == nil || record.TaskID == "" {
		return
	}
	s.mu.Lock()
	if s.capabilityRoutes == nil {
		s.capabilityRoutes = map[string]CapabilityRouteRecord{}
	}
	if previous, ok := s.capabilityRoutes[record.TaskID]; ok && !previous.CreatedAt.IsZero() {
		record.CreatedAt = previous.CreatedAt
	}
	s.capabilityRoutes[record.TaskID] = record
	s.mu.Unlock()
	// The route stays authoritative in memory when the save is contended; the
	// request's exit sync persists it. A persistent failure must be visible:
	// a route that only ever lived in memory is wiped by the next disk reload
	// and the task's continuation then fails closed as unrecoverable.
	if err := s.persistCurrentProjectWorkspaceChecked(); err != nil && s.logger != nil {
		s.logger.Warn("[capability.route] runtime state save deferred task=%s error=%v", record.TaskID, err)
	}
}

func (s *Server) previousCapabilityRoute(taskID, conversationID string) CapabilityRouteRecord {
	if s == nil {
		return CapabilityRouteRecord{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if taskID = strings.TrimSpace(taskID); taskID != "" {
		if record, ok := s.capabilityRoutes[taskID]; ok {
			return record
		}
		return CapabilityRouteRecord{}
	}
	var latest CapabilityRouteRecord
	for _, record := range s.capabilityRoutes {
		if record.ConversationID == conversationID && (latest.UpdatedAt.IsZero() || record.UpdatedAt.After(latest.UpdatedAt)) {
			latest = record
		}
	}
	return latest
}

func (s *Server) capabilityRouteProjection() []map[string]any {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	records := make([]CapabilityRouteRecord, 0, len(s.capabilityRoutes))
	for _, record := range s.capabilityRoutes {
		records = append(records, record)
	}
	s.mu.Unlock()
	sort.Slice(records, func(i, j int) bool { return records[i].UpdatedAt.After(records[j].UpdatedAt) })
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, capabilityRouteRecordMap(record))
	}
	return out
}

func capabilityRouteRecordMap(record CapabilityRouteRecord) map[string]any {
	out := map[string]any{
		"schema_version": record.SchemaVersion, "task_id": record.TaskID, "goal_id": record.GoalID,
		"run_id": record.RunID, "conversation_id": record.ConversationID, "original_intent": record.OriginalIntent,
		"semantic_entry": record.SemanticEntry, "controller": record.Controller,
		"project_revision": record.ProjectRevision, "created_at": record.CreatedAt, "updated_at": record.UpdatedAt,
	}
	if record.Assessment != nil {
		out["capacity_assessment"] = *record.Assessment
	}
	if record.EntryPlan != nil {
		out["entry_plan"] = *record.EntryPlan
	}
	return out
}

func capacityFactor(code string, observed float64, weight int, triggered bool, reason string) CapacityComplexityFactor {
	return CapacityComplexityFactor{Code: code, Observed: observed, Weight: weight, Triggered: triggered, Reason: reason}
}

func capacityRows(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func capacityCountCollection(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case []map[string]any:
		return len(typed)
	case []string:
		return len(typed)
	case map[string]any:
		return len(typed)
	case string:
		if strings.TrimSpace(typed) != "" {
			return 1
		}
		return 0
	default:
		return intNumber(value)
	}
}

func capacityFloat(value any) float64 { return semanticEntryFloatValue(value) }

func capacityMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonNilCapacityValue(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func capacityAssessmentFromAny(value any) *FreeStateCapacityAssessment {
	if value == nil {
		return nil
	}
	if assessment, ok := value.(FreeStateCapacityAssessment); ok {
		if !validCapacityAssessment(assessment) {
			return nil
		}
		copy := assessment
		return &copy
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var assessment FreeStateCapacityAssessment
	if json.Unmarshal(data, &assessment) != nil || !validCapacityAssessment(assessment) {
		return nil
	}
	return &assessment
}

func capabilityEntryPlanFromAny(value any) *CapabilityEntryPlan {
	if value == nil {
		return nil
	}
	if plan, ok := value.(CapabilityEntryPlan); ok {
		if !validCapabilityEntryPlan(plan) {
			return nil
		}
		copy := plan
		return &copy
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var plan CapabilityEntryPlan
	if json.Unmarshal(data, &plan) != nil || !validCapabilityEntryPlan(plan) {
		return nil
	}
	return &plan
}

func capacityAssessmentRevision(assessment *FreeStateCapacityAssessment) string {
	if assessment == nil {
		return ""
	}
	return assessment.ProjectRevision
}

func validCapacityAssessment(assessment FreeStateCapacityAssessment) bool {
	if assessment.SchemaVersion != capacityAssessmentSchema || assessment.Authority != "product_runtime" ||
		assessment.ProjectRevision == "" || assessment.ProjectRevision != assessment.ObservedFacts.ProjectRevision {
		return false
	}
	if assessment.SelectedCapability != capabilityFreeState && assessment.SelectedCapability != capabilityProjectMix {
		return false
	}
	return assessment.ObservedFacts.RequestScope == semanticEntryScopeProjectContext ||
		assessment.ObservedFacts.RequestScope == semanticEntryScopeCurrentSelection
}

func validCapabilityEntryPlan(plan CapabilityEntryPlan) bool {
	return plan.SchemaVersion == capabilityEntryPlanSchema && plan.Capability == capabilityProjectMix &&
		strings.TrimSpace(plan.Stage) != "" && strings.TrimSpace(plan.CapabilityID) != "" && strings.TrimSpace(plan.Source) != ""
}

func restoreCapabilityRoutes(routes map[string]CapabilityRouteRecord) map[string]CapabilityRouteRecord {
	out := map[string]CapabilityRouteRecord{}
	for taskID, record := range routes {
		if taskID == "" || record.TaskID != taskID || record.SchemaVersion != capabilityRouteSchema ||
			record.GoalID == "" || record.RunID == "" || record.ConversationID == "" || record.OriginalIntent == "" || record.Assessment == nil ||
			!validCapacityAssessment(*record.Assessment) || record.ProjectRevision != record.Assessment.ProjectRevision {
			continue
		}
		if record.Controller != string(orchestrationcontroller.MinimalAudioClosure) && record.Controller != string(orchestrationcontroller.ProjectMixWorkflow) {
			continue
		}
		if firstStringFromMap(record.SemanticEntry, "controller") != "" ||
			(firstStringFromMap(record.SemanticEntry, "route") != semanticEntryRouteObservation && firstStringFromMap(record.SemanticEntry, "route") != semanticEntryRouteOpenSemantic) {
			continue
		}
		if record.Controller == string(orchestrationcontroller.ProjectMixWorkflow) {
			if record.EntryPlan == nil || !validCapabilityEntryPlan(*record.EntryPlan) {
				continue
			}
		} else if record.Assessment.SelectedCapability != capabilityFreeState {
			continue
		}
		out[taskID] = record
	}
	return out
}

func reconcileDurableCapabilityRoutes(items map[string]DurableContinuation, routes map[string]CapabilityRouteRecord) map[string]DurableContinuation {
	for id, item := range items {
		contextAssessment := capacityAssessmentFromAny(item.Continuation.Context[capacityAssessmentContextKey])
		hasCapacityState := item.CapacityAssessment != nil || contextAssessment != nil || item.CapabilityEntryPlan != nil || item.Continuation.Context[capabilityRouteContextKey] != nil
		route, hasRoute := routes[item.TaskID]
		if !hasRoute {
			if hasCapacityState && item.Status != ContinuationCompleted && item.Status != ContinuationCancelled && item.Status != ContinuationFailed {
				item = failClosedDurableCapabilityRoute(item, "durable capacity state has no validated task route")
				items[id] = item
			}
			continue
		}
		if route.GoalID != item.GoalID || route.RunID != item.RunID || route.ConversationID != item.ConversationID || route.OriginalIntent != item.OriginalIntent ||
			item.CapacityAssessment != nil && !sameCapacityAssessmentIdentity(*item.CapacityAssessment, *route.Assessment) ||
			contextAssessment != nil && !sameCapacityAssessmentIdentity(*contextAssessment, *route.Assessment) {
			if item.Status != ContinuationCompleted && item.Status != ContinuationCancelled && item.Status != ContinuationFailed {
				item = failClosedDurableCapabilityRoute(item, "durable continuation does not match its validated task route")
				items[id] = item
			}
			continue
		}
		item.CapacityAssessment = capacityAssessmentFromAny(*route.Assessment)
		item.CapabilityEntryPlan = capabilityEntryPlanFromAny(route.EntryPlan)
		item.ProjectRevision = route.ProjectRevision
		semantic, semanticOK := capabilityRouteSemanticEntry(route)
		if !semanticOK {
			semantic = semanticEntryDecision{
				SchemaVersion: semanticEntryDecisionSchema, Route: firstStringFromMap(route.SemanticEntry, "route"),
				Controller: route.Controller, TargetScope: firstStringFromMap(route.SemanticEntry, "target_scope"),
				ControlMode: firstStringFromMap(route.SemanticEntry, "control_mode"), UserAuthorization: firstStringFromMap(route.SemanticEntry, "user_authorization"),
				Confidence: semanticEntryFloatValue(route.SemanticEntry["confidence"]), Reason: capabilityRouteReason(*route.Assessment),
			}
		}
		context := contextWithSemanticEntryDecision(cloneContext(item.Continuation.Context), semantic)
		item.Continuation.Context = contextWithCapabilityRoute(context, route)
		items[id] = cloneDurableContinuation(item)
	}
	return items
}

func sameCapacityAssessment(left, right FreeStateCapacityAssessment) bool {
	return validCapacityAssessment(left) && validCapacityAssessment(right) &&
		left.ProjectRevision == right.ProjectRevision && left.SelectedCapability == right.SelectedCapability &&
		left.ObservedFacts.ProjectUUID == right.ObservedFacts.ProjectUUID && left.ObservedFacts.RequestScope == right.ObservedFacts.RequestScope
}

// sameCapacityAssessmentIdentity compares only the assessment's task identity
// (project, capability, request scope). The project revision is deliberately
// excluded: a durable checkpoint embeds the capacity assessment snapshot from
// the slice that created it, and an applied mutation legitimately advances the
// project revision before the task's route is revalidated, so a post-apply
// checkpoint at the pre-apply revision must not be fail-closed as a route
// identity mismatch (2026-08-28 11:06 smoke: the post-apply continuation at
// revision 5 was parked recovery_validation_required against its own route at
// revision 7, permanently stalling the mandatory observation slice).
func sameCapacityAssessmentIdentity(left, right FreeStateCapacityAssessment) bool {
	return validCapacityAssessment(left) && validCapacityAssessment(right) &&
		left.SelectedCapability == right.SelectedCapability &&
		left.ObservedFacts.ProjectUUID == right.ObservedFacts.ProjectUUID &&
		left.ObservedFacts.RequestScope == right.ObservedFacts.RequestScope
}

func failClosedDurableCapabilityRoute(item DurableContinuation, reason string) DurableContinuation {
	item.Status = ContinuationWaitingInteraction
	item.LeaseOwner = ""
	item.LeaseExpiresAt = time.Time{}
	item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": reason}
	item.UpdatedAt = time.Now().UTC()
	return cloneDurableContinuation(item)
}
