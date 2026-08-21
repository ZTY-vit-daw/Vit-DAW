package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/shadow"
)

func capacityTestProject(trackCount int, duration float64, pluginsPerTrack, groups int, revision string) map[string]any {
	tracks := make([]any, 0, trackCount)
	members := make([][]any, groups)
	for index := 0; index < trackCount; index++ {
		trackID := fmt.Sprintf("track-%d", index+1)
		plugins := make([]any, 0, pluginsPerTrack)
		for pluginIndex := 0; pluginIndex < pluginsPerTrack; pluginIndex++ {
			plugins = append(plugins, map[string]any{"plugin_id": fmt.Sprintf("plugin-%d-%d", index+1, pluginIndex+1)})
		}
		groupsForTrack := []any{}
		if groups > 0 {
			groupIndex := index % groups
			groupID := fmt.Sprintf("group-%d", groupIndex+1)
			groupsForTrack = []any{groupID}
			members[groupIndex] = append(members[groupIndex], trackID)
		}
		tracks = append(tracks, map[string]any{
			"track_id": trackID, "track_name": "Track", "track_type": "hybrid", "is_audio_track": true,
			"plugins": plugins, "track_group_ids": groupsForTrack,
			"clips": []any{map[string]any{"start_seconds": 0.0, "duration_seconds": duration}},
		})
	}
	trackGroups := make([]any, 0, groups)
	for index := 0; index < groups; index++ {
		trackGroups = append(trackGroups, map[string]any{"group_id": fmt.Sprintf("group-%d", index+1), "member_track_ids": members[index]})
	}
	return map[string]any{
		"status": "ok", "project_uuid": "project-capacity", "project_revision": revision,
		"tracks": tracks, "track_groups": trackGroups,
	}
}

func TestCapacityAssessmentKeepsSmallProjectsInFreeState(t *testing.T) {
	for _, test := range []struct {
		name     string
		tracks   int
		duration float64
	}{
		{name: "six track stems", tracks: 6, duration: 20},
		{name: "two track overdub", tracks: 2, duration: 240},
		{name: "three track overdub", tracks: 3, duration: 240},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := capacityFactsFromProjectState(capacityTestProject(test.tracks, test.duration, 1, 0, "rev-small"), semanticEntryScopeProjectContext)
			assessment := assessFreeStateCapacity(facts, false, time.Unix(1, 0))
			if assessment.SelectedCapability != capabilityFreeState || assessment.Authority != "product_runtime" {
				t.Fatalf("small project was not retained in free-state: %+v", assessment)
			}
			if assessment.ObservedFacts.TrackCount != test.tracks || assessment.ObservedFacts.DurationSeconds != test.duration {
				t.Fatalf("assessment did not use observed structure: %+v", assessment.ObservedFacts)
			}
		})
	}
}

func TestCapacityObservationRetainsProcessorAutomationAndRoutingCounts(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{
		"project_uuid": "project-structure", "project_revision": "rev-structure",
		"tracks": []any{map[string]any{
			"track_id": "track-1", "track_name": "Audio", "track_type": "hybrid", "is_audio_track": true,
			"plugins":          []any{map[string]any{"plugin_id": "p1"}, map[string]any{"plugin_id": "p2"}},
			"automation_lanes": []any{map[string]any{"id": "a1"}, map[string]any{"id": "a2"}, map[string]any{"id": "a3"}},
			"sends":            []any{map[string]any{"id": "s1"}, map[string]any{"id": "s2"}},
			"clips":            []any{map[string]any{"start_seconds": 10.0, "duration_seconds": 30.0}},
		}},
	})
	server := New(nil, project, nil)
	facts := server.observeCapacityProjectFacts(context.Background(), semanticEntryScopeProjectContext)
	if facts.TrackCount != 1 || facts.ActiveAudioTrackCount != 1 || facts.PluginCount != 2 || facts.AutomationLaneCount != 3 || facts.RoutingEdgeCount != 2 || facts.DurationSeconds != 40 {
		t.Fatalf("structural capacity facts were lost by product observation: %+v", facts)
	}
}

func TestCapacityAssessmentRoutesOnlyCombinedLargeProjectPressure(t *testing.T) {
	largeFacts := capacityFactsFromProjectState(capacityTestProject(36, 600, 2, 10, "rev-large"), semanticEntryScopeProjectContext)
	large := assessFreeStateCapacity(largeFacts, false, time.Unix(2, 0))
	if large.SelectedCapability != capabilityProjectMix || large.CapacityScore < 50 || len(large.ReasonCodes) < 3 {
		t.Fatalf("large relational project was not routed to fixed mixing capability: %+v", large)
	}
	boundedFacts := largeFacts
	boundedFacts.RequestScope = semanticEntryScopeCurrentSelection
	bounded := assessFreeStateCapacity(boundedFacts, false, time.Unix(3, 0))
	if bounded.SelectedCapability != capabilityFreeState || bounded.ReasonCodes[0] != "bounded_request_scope" {
		t.Fatalf("bounded target was incorrectly expanded to whole-project workflow: %+v", bounded)
	}
	trackCountAlone := largeFacts
	trackCountAlone.DurationSeconds, trackCountAlone.PluginCount, trackCountAlone.RelationshipBreadth, trackCountAlone.RoutingEdgeCount = 30, 0, 0, 0
	singleFactor := assessFreeStateCapacity(trackCountAlone, false, time.Unix(4, 0))
	if singleFactor.SelectedCapability != capabilityFreeState {
		t.Fatalf("one project-size factor alone forced fixed workflow: %+v", singleFactor)
	}
}

func TestCapabilityEntryDefaultsToA2AndRespectsExplicitOrTaskHistory(t *testing.T) {
	assessment := assessFreeStateCapacity(capacityFactsFromProjectState(capacityTestProject(36, 600, 2, 10, "rev"), semanticEntryScopeProjectContext), false, time.Now())
	defaultPlan := capabilityEntryPlan("检查一下当前工程", CapabilityRouteRecord{}, assessment)
	if defaultPlan.Stage != "A2" || defaultPlan.CapabilityID != "project_prep.technical_integrity.v0" || defaultPlan.Source != "default_after_capacity_routing" {
		t.Fatalf("default entry is not A2: %+v", defaultPlan)
	}
	explicit := capabilityEntryPlan("请从 C2 开始", CapabilityRouteRecord{}, assessment)
	if explicit.Stage != "C2" || explicit.Source != "explicit_user_request" {
		t.Fatalf("explicit stage was not selected: %+v", explicit)
	}
	history := CapabilityRouteRecord{EntryPlan: &CapabilityEntryPlan{Stage: "A3", CapabilityID: "project_prep.track_organization.v0"}}
	resumed := capabilityEntryPlan("继续工程处理", history, assessment)
	if resumed.Stage != "A3" || resumed.Source != "task_history" {
		t.Fatalf("task stage history was not resumed: %+v", resumed)
	}
}

func TestExplicitCapabilityStageSelectsFixedLayerEvenForSmallProject(t *testing.T) {
	facts := capacityFactsFromProjectState(capacityTestProject(3, 120, 1, 0, "rev-explicit"), semanticEntryScopeProjectContext)
	assessment := assessFreeStateCapacity(facts, explicitCapabilityStage("请从 C2 开始") != "", time.Now())
	plan := capabilityEntryPlan("请从 C2 开始", CapabilityRouteRecord{}, assessment)
	if assessment.SelectedCapability != capabilityProjectMix || plan.Stage != "C2" || plan.Source != "explicit_user_request" {
		t.Fatalf("explicit capability layer request was not honored: assessment=%+v plan=%+v", assessment, plan)
	}
}

func TestUntargetedNaturalLanguageEntriesObserveBeforeRuntimeRouting(t *testing.T) {
	var prompts []string
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		prompts = append(prompts, string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":\"semantic_entry_decision.v1\",\"route\":\"observation\",\"target_scope\":\"project_context\",\"control_mode\":\"observe_only\",\"user_authorization\":\"observe_only\",\"confidence\":0.95,\"reason\":\"inspect current project\"}"}}]}`))
	}))
	defer model.Close()

	for _, request := range []string{"检查一下当前工程有什么问题？", "你能帮我对当前工程进行混音检查吗？"} {
		project := shadow.New(nil)
		project.Initialize(capacityTestProject(6, 20, 1, 0, "rev-natural"))
		server := New(nil, project, nil)
		server.llm = &llm.Client{HTTPClient: model.Client()}
		contextWithTask, identity := server.ensureCapabilityRoutingTask(request, nil)
		entry, err := server.planSemanticEntry(context.Background(), "conversation-natural", request, contextWithTask,
			config.EngineConfig{BaseURL: model.URL + "/v1", APIKey: "test", DefaultModel: "test"})
		if err != nil || entry.Controller != "" {
			t.Fatalf("semantic entry selected capability before observation: entry=%+v err=%v", entry, err)
		}
		entry, record, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-natural", request, contextWithTask, entry, identity)
		if err != nil || entry.Controller != string(orchestrationcontroller.MinimalAudioClosure) || record.Assessment == nil || record.Assessment.SelectedCapability != capabilityFreeState {
			t.Fatalf("observation-first route failed: entry=%+v record=%+v err=%v", entry, record, err)
		}
		encoded, _ := json.Marshal(record)
		for _, forbidden := range []string{"vocals", "bass", "清晰", "稳定", "靠前", "processor_type", "plugin_id", "requested_view"} {
			if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
				t.Fatalf("untargeted route invented %q: %s", forbidden, encoded)
			}
		}
	}
	if len(prompts) != 2 {
		t.Fatalf("semantic entry was not called once per natural request: %d", len(prompts))
	}
}

func TestProductChatPathRoutesLargeUntargetedInspectionToA2Boundary(t *testing.T) {
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":\"semantic_entry_decision.v1\",\"route\":\"observation\",\"target_scope\":\"project_context\",\"control_mode\":\"observe_only\",\"user_authorization\":\"observe_only\",\"confidence\":0.95,\"reason\":\"inspect current project\"}"}}]}`))
	}))
	defer model.Close()
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(36, 600, 2, 10, "rev-product"))
	server := New(nil, project, nil)
	server.llm = &llm.Client{HTTPClient: model.Client()}
	response, handled := server.runAgentLoopChat(context.Background(), "conversation-product", ChatRequest{
		Message: "你能帮我对当前工程进行混音检查吗？",
		Context: map[string]any{
			capacityAssessmentContextKey: map[string]any{"authority": "test_harness", "selected_capability": capabilityFreeState},
		},
	}, config.EngineConfig{BaseURL: model.URL + "/v1", APIKey: "test", DefaultModel: "test"})
	if !handled || response.Workflow != "project_mix_workflow" || response.StopReason != "capability_unavailable" {
		t.Fatalf("product path did not stop at selected fixed capability boundary: %+v", response)
	}
	if response.TaskID == "" || response.GoalID == "" || response.RunID == "" || response.OriginalIntent != "你能帮我对当前工程进行混音检查吗？" {
		t.Fatalf("product route omitted task identity: %+v", response)
	}
	assessment := capacityAssessmentFromAny(response.WorkflowData[capacityAssessmentContextKey])
	plan := capabilityEntryPlanFromAny(response.WorkflowData[capabilityEntryPlanContextKey])
	if assessment == nil || assessment.Authority != "product_runtime" || assessment.SelectedCapability != capabilityProjectMix || plan == nil || plan.Stage != "A2" {
		t.Fatalf("product route did not expose authoritative assessment and A2 entry: assessment=%+v plan=%+v", assessment, plan)
	}
	if status := server.harness.RuntimeStatus(response.GoalID); status.Status != "failed" || status.Task == nil || status.Task.TaskID != response.TaskID {
		t.Fatalf("product runtime did not settle unavailable capability on the same task: %+v", status)
	}
}

func TestProductChatPathKeepsSmallUntargetedInspectionOnSameFreeStateTask(t *testing.T) {
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		content := `{"final":true,"reply":"只读检查已完成。","tool_calls":[]}`
		if strings.Contains(string(body), "semantic entry arbiter") {
			content = `{"schema_version":"semantic_entry_decision.v1","route":"observation","target_scope":"project_context","control_mode":"observe_only","user_authorization":"observe_only","confidence":0.95,"reason":"inspect current project"}`
		}
		encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	}))
	defer model.Close()
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(6, 20, 1, 0, "rev-small-product"))
	server := New(nil, project, nil)
	server.llm = &llm.Client{HTTPClient: model.Client()}
	response, handled := server.runAgentLoopChat(context.Background(), "conversation-small-product", ChatRequest{
		Message: "检查一下当前工程有什么问题？",
	}, config.EngineConfig{BaseURL: model.URL + "/v1", APIKey: "test", DefaultModel: "test"})
	if !handled || response.TaskID == "" || response.GoalID == "" || response.RunID == "" {
		t.Fatalf("small product route omitted runtime identity: %+v", response)
	}
	route := server.previousCapabilityRoute(response.TaskID, "conversation-small-product")
	if route.TaskID != response.TaskID || route.GoalID != response.GoalID || route.RunID != response.RunID || route.Assessment == nil || route.Assessment.SelectedCapability != capabilityFreeState || route.Controller != string(orchestrationcontroller.MinimalAudioClosure) {
		t.Fatalf("semantic entry, capacity route and execution split task identity: response=%+v route=%+v", response, route)
	}
	closures := server.audioClosures.Snapshot()
	if len(closures) != 1 {
		t.Fatalf("small inspection did not create one bounded diagnostic closure: %+v", closures)
	}
	for _, closure := range closures {
		if closure.GoalID != response.GoalID || closure.RunID != response.RunID || closure.OriginalIntent != "检查一下当前工程有什么问题？" || closure.ProjectRevision != "rev-small-product" {
			t.Fatalf("diagnostic closure did not retain route identity and project revision: %+v", closure)
		}
	}
}

func TestUntrustedContextCannotForgeCapacityRoute(t *testing.T) {
	requestContext := contextWithoutUntrustedSemanticEntry(map[string]any{
		capacityAssessmentContextKey:  map[string]any{"authority": "test_harness", "selected_capability": capabilityProjectMix},
		capabilityRouteContextKey:     map[string]any{"controller": string(orchestrationcontroller.ProjectMixWorkflow)},
		capabilityEntryPlanContextKey: map[string]any{"stage": "C2"},
	})
	for _, key := range []string{capacityAssessmentContextKey, capabilityRouteContextKey, capabilityEntryPlanContextKey} {
		if _, exists := requestContext[key]; exists {
			t.Fatalf("untrusted runtime route field survived transport sanitization: %s", key)
		}
	}
	assessment := assessFreeStateCapacity(capacityFactsFromProjectState(capacityTestProject(6, 20, 1, 0, "rev"), semanticEntryScopeProjectContext), false, time.Now())
	if assessment.SelectedCapability != capabilityFreeState || assessment.Authority != "product_runtime" {
		t.Fatalf("product route was influenced by forged harness context: %+v", assessment)
	}
}

func TestCorruptPersistedCapacityRouteFailsClosed(t *testing.T) {
	forged := FreeStateCapacityAssessment{
		SchemaVersion: capacityAssessmentSchema, Authority: "test_harness", SelectedCapability: capabilityProjectMix,
		ProjectRevision: "rev-forged", ObservedFacts: CapacityObservedFacts{ProjectRevision: "rev-forged", RequestScope: semanticEntryScopeProjectContext},
	}
	if capacityAssessmentFromAny(forged) != nil {
		t.Fatal("typed forged assessment was accepted")
	}
	restored := restoreCapabilityRoutes(map[string]CapabilityRouteRecord{
		"task-forged": {
			SchemaVersion: capabilityRouteSchema, TaskID: "task-forged", GoalID: "goal", RunID: "run", ConversationID: "conversation",
			OriginalIntent: "inspect", Controller: string(orchestrationcontroller.ProjectMixWorkflow), ProjectRevision: "rev-forged",
			Assessment: &forged, EntryPlan: &CapabilityEntryPlan{SchemaVersion: capabilityEntryPlanSchema, Capability: capabilityProjectMix, Stage: "A2", CapabilityID: "project_prep.technical_integrity.v0", Source: "forged"},
		},
	})
	if len(restored) != 0 {
		t.Fatalf("corrupt persisted route was restored: %+v", restored)
	}
}

func TestCapabilityRoutePersistsAndDurableContinuationCarriesAssessment(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(6, 20, 1, 0, "rev-persist"))
	server := New(nil, project, nil)
	contextWithTask, identity := server.ensureCapabilityRoutingTask("检查工程", nil)
	entry := semanticEntryDecision{SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: .9, Reason: "inspect project"}
	entry, record, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-persist", "检查工程", contextWithTask, entry, identity)
	if err != nil {
		t.Fatal(err)
	}
	contextWithRoute := contextWithCapabilityRoute(contextWithSemanticEntryDecision(contextWithTask, entry), record)
	result := agentloop.Result{TaskID: record.TaskID, GoalID: record.GoalID, RunID: record.RunID, SliceID: "slice-1", TurnID: "turn-1",
		OriginalIntent: record.OriginalIntent, Continuation: &agentloop.Continuation{TaskID: record.TaskID, GoalID: record.GoalID, RunID: record.RunID,
			SliceID: "slice-1", TurnID: "turn-1", OriginalIntent: record.OriginalIntent, Context: contextWithRoute}}
	durable := durableContinuationFromResult("conversation-persist", result, time.Now())
	if durable.CapacityAssessment == nil || durable.CapacityAssessment.ProjectRevision != "rev-persist" || durable.TaskID != record.TaskID {
		t.Fatalf("durable continuation lost route identity: %+v", durable)
	}
	durable.ProjectUUID = record.Assessment.ObservedFacts.ProjectUUID
	server.durableContinuations[durable.ContinuationID] = durable
	server.conversationGoals["conversation-persist"] = record.GoalID
	routes := server.capabilityRouteProjection()
	continuations := server.continuationRuntimeProjection()
	if len(routes) != 1 || routes[0]["task_id"] != record.TaskID || routes[0]["capacity_assessment"] == nil {
		t.Fatalf("runtime route projection omitted assessment: %+v", routes)
	}
	if len(continuations) != 1 || continuations[0]["task_id"] != record.TaskID || continuations[0]["capacity_assessment"] == nil {
		t.Fatalf("runtime continuation projection omitted assessment: %+v", continuations)
	}

	snapshot := server.projectAgentRuntimeStateLocked()
	restored := New(nil, project, nil)
	restored.restoreProjectAgentRuntimeStateLocked(snapshot)
	recovered := restored.previousCapabilityRoute(record.TaskID, "conversation-persist")
	if recovered.TaskID != record.TaskID || recovered.GoalID != record.GoalID || recovered.RunID != record.RunID || recovered.OriginalIntent != record.OriginalIntent || recovered.Assessment == nil {
		t.Fatalf("route did not survive project runtime restore: %+v", recovered)
	}
	restoredDurable, ok := restored.durableContinuations[durable.ContinuationID]
	if !ok || restoredDurable.TaskID != record.TaskID || restoredDurable.GoalID != record.GoalID || restoredDurable.RunID != record.RunID || restoredDurable.OriginalIntent != record.OriginalIntent || restoredDurable.CapacityAssessment == nil || restoredDurable.CapacityAssessment.ProjectRevision != "rev-persist" {
		t.Fatalf("durable continuation did not restore the same route assessment: %+v ok=%v", restoredDurable, ok)
	}
	if nested := capacityAssessmentFromAny(restoredDurable.Continuation.Context[capacityAssessmentContextKey]); nested == nil || nested.ProjectRevision != "rev-persist" {
		t.Fatalf("durable continuation context was not rebound to the validated route: %+v", restoredDurable.Continuation.Context)
	}
	mismatched := durable
	mismatched.Status = ContinuationPending
	wrongAssessment := *durable.CapacityAssessment
	wrongAssessment.SelectedCapability = capabilityProjectMix
	mismatched.CapacityAssessment = &wrongAssessment
	reconciled := reconcileDurableCapabilityRoutes(map[string]DurableContinuation{mismatched.ContinuationID: mismatched}, map[string]CapabilityRouteRecord{record.TaskID: record})
	if got := reconciled[mismatched.ContinuationID]; got.Status != ContinuationWaitingInteraction || firstStringFromMap(got.PendingInteraction, "status") != "recovery_validation_required" {
		t.Fatalf("mismatched durable route did not fail closed: %+v", got)
	}
}

func TestSameTaskHistoryIsObservedAndRestoresExplicitStage(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(3, 120, 1, 0, "rev-history"))
	server := New(nil, project, nil)
	ctx, identity := server.ensureCapabilityRoutingTask("请从 C2 开始", nil)
	entry := semanticEntryDecision{SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: .9, Reason: "explicit stage"}
	_, first, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-history", "请从 C2 开始", ctx, entry, identity)
	if err != nil || first.EntryPlan == nil || first.EntryPlan.Stage != "C2" {
		t.Fatalf("first explicit route failed: %+v err=%v", first, err)
	}
	ctx["goal_id"], ctx["run_id"], ctx["task_id"] = first.GoalID, first.RunID, first.TaskID
	_, second, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-history", "继续", ctx, entry, first)
	if err != nil || second.EntryPlan == nil || second.EntryPlan.Stage != "C2" || second.EntryPlan.Source != "task_history" {
		t.Fatalf("same-task stage history was not restored: %+v err=%v", second, err)
	}
	if second.Assessment == nil || !second.Assessment.ObservedFacts.TaskHistoryAvailable || second.Assessment.ObservedFacts.PriorCapabilityStage != "C2" || second.Assessment.ObservedFacts.PriorAssessmentRevision != "rev-history" {
		t.Fatalf("task history was absent from observed capacity facts: %+v", second.Assessment)
	}
	if leaked := server.previousCapabilityRoute("different-task", "conversation-history"); leaked.SchemaVersion != "" {
		t.Fatalf("conversation history leaked across task identity: %+v", leaked)
	}
}

func TestProjectRevisionReassessesSameTaskWithoutSemanticReentry(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(6, 20, 1, 0, "rev-1"))
	server := New(nil, project, nil)
	contextWithTask, identity := server.ensureCapabilityRoutingTask("检查工程", nil)
	entry := semanticEntryDecision{SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: .9, Reason: "inspect project"}
	entry, first, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-revision", "检查工程", contextWithTask, entry, identity)
	if err != nil {
		t.Fatal(err)
	}
	ctx := contextWithCapabilityRoute(contextWithSemanticEntryDecision(contextWithTask, entry), first)
	project.Initialize(capacityTestProject(6, 25, 1, 0, "rev-2"))
	refreshedContext, err := server.refreshCapabilityRouteForRevision(context.Background(), "conversation-revision", ctx)
	if err != nil {
		t.Fatal(err)
	}
	refreshed := server.previousCapabilityRoute(first.TaskID, "conversation-revision")
	if refreshed.TaskID != first.TaskID || refreshed.GoalID != first.GoalID || refreshed.RunID != first.RunID || refreshed.OriginalIntent != first.OriginalIntent {
		t.Fatalf("revision refresh changed task identity: before=%+v after=%+v", first, refreshed)
	}
	if refreshed.ProjectRevision != "rev-2" || firstStringFromMap(refreshedContext, "original_intent") != first.OriginalIntent {
		t.Fatalf("revision refresh did not replace capacity evidence in place: %+v context=%+v", refreshed, refreshedContext)
	}
	if refreshed.Assessment == nil || !refreshed.Assessment.ObservedFacts.TaskHistoryAvailable || refreshed.Assessment.ObservedFacts.PriorAssessmentRevision != "rev-1" {
		t.Fatalf("revision refresh omitted prior task assessment history: %+v", refreshed.Assessment)
	}
}

func TestProjectRevisionCapacityChangeHandsOffControllerWithinSameTask(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(6, 20, 1, 0, "rev-small"))
	server := New(nil, project, nil)
	contextWithTask, identity := server.ensureCapabilityRoutingTask("检查并改善当前工程", nil)
	entry := semanticEntryDecision{SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: .9, Reason: "open project improvement"}
	entry, first, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-handoff", "检查并改善当前工程", contextWithTask, entry, identity)
	if err != nil || first.Controller != string(orchestrationcontroller.MinimalAudioClosure) {
		t.Fatalf("initial free-state route failed: record=%+v err=%v", first, err)
	}
	ctx := contextWithCapabilityRoute(contextWithSemanticEntryDecision(contextWithTask, entry), first)
	ctx, closure, active, err := server.prepareAudioClosureContext("conversation-handoff", first.OriginalIntent, ctx)
	if err != nil || !active || closure.Terminal() {
		t.Fatalf("initial closure did not acquire controller: closure=%+v err=%v", closure, err)
	}
	project.Initialize(capacityTestProject(36, 600, 2, 10, "rev-large"))
	refreshedContext, err := server.refreshCapabilityRouteForRevision(context.Background(), "conversation-handoff", ctx)
	if err != nil {
		t.Fatal(err)
	}
	refreshed := server.previousCapabilityRoute(first.TaskID, "conversation-handoff")
	if refreshed.TaskID != first.TaskID || refreshed.GoalID != first.GoalID || refreshed.RunID != first.RunID || refreshed.Controller != string(orchestrationcontroller.ProjectMixWorkflow) || refreshed.EntryPlan == nil || refreshed.EntryPlan.Stage != "A2" {
		t.Fatalf("capacity reroute did not preserve task identity and select A2: %+v", refreshed)
	}
	settledClosure, ok := server.audioClosures.Load(closure.ClosureID)
	if !ok || !settledClosure.Terminal() {
		t.Fatalf("old closure remained active after capacity handoff: %+v", settledClosure)
	}
	_, owner, routed, err := server.prepareProjectMixController("conversation-handoff", refreshedContext)
	if err != nil || !routed || owner.Controller != orchestrationcontroller.ProjectMixWorkflow {
		t.Fatalf("fixed capability controller did not acquire after handoff: owner=%+v routed=%v err=%v", owner, routed, err)
	}
}
