package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

// SEMANTIC-CARD-COPY-1 ①②③ acceptance pins: the semantic chain card copy
// keeps its honest-contract wording at the render layer — admission boundary
// instead of "唯一可加载", the load confirmation names its target track, and
// the typed-controller confirmation card speaks natural Chinese.

func semanticCardCopyAdmissionBoundary(admitted int, names ...string) map[string]any {
	return map[string]any{
		"catalog_search_type": "reverb",
		"catalog_face_count":  40,
		"admitted_count":      admitted,
		"admitted_names":      names,
	}
}

func TestPluginRecommendationAdmissionBoundaryLineCoversAllCountShapes(t *testing.T) {
	single := pluginRecommendationAdmissionBoundaryLine(semanticCardCopyAdmissionBoundary(1, "FabFilter Pro-R"))
	wantSingle := "准入边界：本机插件库检索到 40 款混响，当前通过 PCA 语义控制准入的只有 FabFilter Pro-R；其余 39 款可装载，但未接入本类语义控制，不在本次推荐面。"
	if single != wantSingle {
		t.Fatalf("single-admitted boundary line changed:\n got: %s\nwant: %s", single, wantSingle)
	}
	many := pluginRecommendationAdmissionBoundaryLine(semanticCardCopyAdmissionBoundary(3, "A", "B", "C"))
	if !strings.Contains(many, "其中 3 款已通过 PCA 语义控制准入") || !strings.Contains(many, "其余 37 款可装载，但未接入本类语义控制") {
		t.Fatalf("multi-admitted boundary line lost honest counts: %s", many)
	}
	allAdmitted := pluginRecommendationAdmissionBoundaryLine(semanticCardCopyAdmissionBoundary(40))
	if !strings.Contains(allAdmitted, "已全部通过 PCA 语义控制准入") || strings.Contains(allAdmitted, "其余") {
		t.Fatalf("all-admitted boundary line invented an excluded set: %s", allAdmitted)
	}
	if line := pluginRecommendationAdmissionBoundaryLine(nil); line != "" {
		t.Fatalf("absent disclosure rendered a line: %s", line)
	}
	if line := pluginRecommendationAdmissionBoundaryLine(map[string]any{"catalog_face_count": 0, "admitted_count": 0}); line != "" {
		t.Fatalf("degenerate disclosure rendered a line: %s", line)
	}
	dynamics := pluginRecommendationAdmissionBoundaryLine(map[string]any{
		"catalog_search_type": "dynamics", "catalog_face_count": 50, "admitted_count": 2, "admitted_names": []string{"A", "B"},
	})
	if !strings.Contains(dynamics, "50 款动态类效果器") {
		t.Fatalf("broad dynamics catalog face was narrowed to a single label: %s", dynamics)
	}
}

func TestPluginRecommendationSelectionCardDisclosesAdmissionBoundary(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-m", RunID: "run-m", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-m", agentModeDefault, manualTestContext(map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
		pluginRecommendationAdmissionBoundaryKey: semanticCardCopyAdmissionBoundary(1, "FabFilter Pro-R"),
	}), res, plan, pluginRecommendationTestCandidates())
	if !strings.Contains(resp.Reply, "准入边界：本机插件库检索到 40 款混响，当前通过 PCA 语义控制准入的只有 FabFilter Pro-R") {
		t.Fatalf("selection card omitted the admission boundary: %s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "唯一可加载") {
		t.Fatalf("selection card kept the dishonest loadability claim: %s", resp.Reply)
	}
	boundary := firstMapFromAny(resp.WorkflowData["admission_boundary"])
	if pluginRecommendationBoundaryCount(boundary["catalog_face_count"]) != 40 ||
		pluginRecommendationBoundaryCount(boundary["admitted_count"]) != 1 {
		t.Fatalf("selection payload omitted auditable boundary numbers: %#v", boundary)
	}
	card := resp.InteractionRequests[0]
	if !strings.Contains(card.Body, "准入边界") {
		t.Fatalf("parked interaction card body omitted the boundary line: %s", card.Body)
	}
}

func TestPluginRecommendationSelectionCardWithoutDisclosureStaysBackCompatible(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-m", RunID: "run-m", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-m", agentModeDefault, manualTestContext(map[string]any{
		"selected_track_id": "1007",
	}), res, plan, pluginRecommendationTestCandidates())
	if strings.Contains(resp.Reply, "准入边界") {
		t.Fatalf("face without two-face numbers invented a boundary: %s", resp.Reply)
	}
	if firstMapFromAny(resp.WorkflowData["admission_boundary"]) != nil {
		t.Fatalf("face without disclosure fabricated payload numbers: %#v", resp.WorkflowData["admission_boundary"])
	}
}

// FULLACCESS-REPLY-COPY-2 ①: the autonomous selection reports its choice in
// the chat reply, but the honest admission boundary lives only on the
// structured faces — the workflow_data disclosure must survive verbatim.
func TestFullAccessSelectionReplyOmitsAdmissionBoundaryKeepsStructuredDisclosure(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-fa", RunID: "run-fa", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-fa", agentModeDefault, fullAccessTestContext(map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
		pluginRecommendationAdmissionBoundaryKey: semanticCardCopyAdmissionBoundary(1, "FabFilter Pro-R"),
	}), res, plan, pluginRecommendationTestCandidates())
	if !strings.Contains(resp.Reply, "主推荐") {
		t.Fatalf("autonomous selection did not report the chosen candidate: %s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "准入边界") {
		t.Fatalf("chat reply body must not carry the admission boundary: %s", resp.Reply)
	}
	postContext := firstMapFromAny(resp.WorkflowData["semantic_post_load_request_context"])
	disclosed := firstMapFromAny(postContext[pluginRecommendationAdmissionBoundaryKey])
	if pluginRecommendationBoundaryCount(disclosed["catalog_face_count"]) != 40 ||
		pluginRecommendationBoundaryCount(disclosed["admitted_count"]) != 1 {
		t.Fatalf("autonomous selection lost the structured boundary disclosure: %#v", postContext)
	}
}

// FULLACCESS-REPLY-COPY-2 ②: the typed-controller executed completion face is
// Chinese, and its WorkflowData contract is unchanged.
func TestSemanticDynamicExecutedCompletionReplyIsChinese(t *testing.T) {
	receipt := map[string]any{"schema_version": semanticDynamicReceipt, "status": "executed", "ticket_id": "dyn-scc",
		"family": processorintent.FamilyReverb, "processor_type": "reverb", "track_id": "1007", "plugin_id": "plugin-1",
		"topology_generation": "generation-scc", "controller_result": map[string]any{"status": "exact", "atomic": true},
		"rollback":                 map[string]any{"status": "not_needed", "verified": true},
		"post_action_verification": map[string]any{"status": "pending_model_observation", "requires_fresh_observation": true, "view_selection": "model_owned", "server_injected_view": false}}
	resp := semanticDynamicExecutedResponse(PendingInteraction{ConversationID: "conversation-scc", GoalID: "goal-scc", RunID: "run-scc"}, receipt)
	if resp.Reply != semanticDynamicExecutedReply {
		t.Fatalf("executed completion reply drifted:\n got: %s\nwant: %s", resp.Reply, semanticDynamicExecutedReply)
	}
	if resp.Reply != "类型化语义处理器调整已执行完成，带回读与事务证据。原目标标记完成前需要一次新的动作后观察。" {
		t.Fatalf("executed completion reply is not the pinned Chinese copy: %s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "typed semantic processor") || strings.Contains(resp.Reply, "fresh post-action observation") {
		t.Fatalf("executed completion reply fell back to English: %s", resp.Reply)
	}
	if resp.Workflow != semanticDynamicWorkflow || resp.GoalStatus != string(agentruntime.StatusCompleted) ||
		resp.StopReason != "semantic_dynamic_execution_completed" {
		t.Fatalf("executed completion semantics changed: %#v", resp)
	}
	wantScalars := map[string]any{"schema_version": semanticDynamicSchema, "status": "executed", "planning_only": false, "mutation_authorized": true, "mutation_performed": true}
	for key, want := range wantScalars {
		if resp.WorkflowData[key] != want {
			t.Fatalf("executed workflow_data %s changed: got %#v want %#v", key, resp.WorkflowData[key], want)
		}
	}
	if len(resp.WorkflowData) != 7 {
		t.Fatalf("executed workflow_data field set drifted: %#v", resp.WorkflowData)
	}
	executed := firstMapFromAny(resp.WorkflowData["execution_receipt"])
	if executed["status"] != "executed" || executed["ticket_id"] != "dyn-scc" || executed["schema_version"] != semanticDynamicReceipt {
		t.Fatalf("executed workflow_data lost the execution receipt: %#v", executed)
	}
	post := firstMapFromAny(resp.WorkflowData["post_action_verification"])
	if firstStringFromMap(post, "status") != "pending_model_observation" || firstStringFromMap(post, "view_selection") != "model_owned" ||
		!boolValue(post["requires_fresh_observation"]) || boolValue(post["server_injected_view"]) {
		t.Fatalf("executed workflow_data post-action contract drifted: %#v", post)
	}
}

func TestPluginLoadConfirmationNamesTargetTrack(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := PendingInteraction{
		ConversationID: "conversation-m", GoalID: "goal-m", RunID: "run-m",
		RequestContext: manualTestContext(map[string]any{"selected_track_id": "1007"}),
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema, "processor_type": "reverb",
			"listening_goal": "加一点短衰减混响",
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
			"recommendations": []map[string]any{{
				"candidate_key": "plugin_candidate_1", "name": "FabFilter Pro-R", "plugin_path": `C:\VST3\FabFilter Pro-R.vst3`,
				"identifier": "VST3-FabFilter-Pro-R", "role": "recommended",
			}},
		},
	}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_1")
	if resp.Error != "" || !resp.NeedsConfirmation {
		t.Fatalf("manual load face changed: %#v", resp)
	}
	// No harness is attached, so no honest track name exists: the card must
	// degrade to the bare track ID, never back to the unnamed "当前轨道".
	if !strings.Contains(resp.Reply, "到轨道 1007") {
		t.Fatalf("load confirmation did not name the target track: %s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "到当前轨道") {
		t.Fatalf("load confirmation kept the unnamed track wording: %s", resp.Reply)
	}
	if !strings.Contains(resp.Reply, "本次只装载插件，不会写入任何参数") {
		t.Fatalf("load confirmation omitted the no-parameter contract: %s", resp.Reply)
	}
	plan, ok := server.pending[resp.PlanID]
	if !ok || firstStringFromMap(plan.WorkflowData, "track_id") != "1007" {
		t.Fatalf("load plan lost the target track id: %#v", plan)
	}
}

func TestPluginLoadTrackDisplayNamePrefersStateRowAndDistrustsMismatchedSelection(t *testing.T) {
	state := map[string]any{"tracks": []any{
		map[string]any{"track_id": "1007", "track_name": "Bass"},
		map[string]any{"track_id": "1008", "track_name": "Lead"},
	}}
	if got := pluginLoadTrackDisplayNameFromState(state, "1007", nil); got != "Bass" {
		t.Fatalf("state row resolution changed: %q", got)
	}
	mismatched := map[string]any{"selected_track_id": "1008", "selected_track_name": "Lead"}
	if got := pluginLoadTrackDisplayNameFromState(nil, "1007", mismatched); got != "" {
		t.Fatalf("mismatched selected name leaked into track 1007: %q", got)
	}
	matched := map[string]any{"selected_track_id": "1007", "selected_track_name": "Bass"}
	if got := pluginLoadTrackDisplayNameFromState(nil, "1007", matched); got != "Bass" {
		t.Fatalf("id-matched selected name was ignored: %q", got)
	}
	if got := pluginLoadTrackDisplayNameFromState(state, "", matched); got != "" {
		t.Fatalf("empty track id produced a name: %q", got)
	}
}

func semanticCardCopyDynamicTicket(t *testing.T) semanticDynamicTicket {
	t.Helper()
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyReverb)
	if !ok {
		t.Fatal("reverb dynamic spec missing")
	}
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "generation-scc"},
		"reverb_stage": map[string]any{
			"stage_key": "reverb_main",
			"tail":      []map[string]any{dynamicTestBinding("decay", "r-decay")},
		},
	}
	_, bindings, err := semanticDynamicBriefFor(spec, summary, []string{"decay"})
	if err != nil {
		t.Fatal(err)
	}
	path := "stage/reverb_main/tail/decay/0"
	binding, ok := bindings[path]
	if !ok {
		t.Fatalf("reverb decay binding missing: %#v", bindings)
	}
	intent := processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
		Family: spec.Family, Intent: "short decay reverb", RequiredCoverage: []string{"decay"},
		Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9}
	return semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn-scc", Family: spec.Family,
		ProcessorType: spec.Processor, TrackID: "1007", PluginID: "plugin-1", TopologyGeneration: "generation-scc",
		Intent: intent, Controls: []semanticDynamicTicketRow{{Axis: "decay", PathKey: path, Role: binding.Role,
			ControlRef: binding.ControlRef, Target: map[string]any{"time_seconds": 0.9}, Purpose: "short decay", Confidence: "high"}}}
}

func TestSemanticDynamicConfirmationCardIsChinese(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	ticket := semanticCardCopyDynamicTicket(t)
	brief := semanticDynamicBrief{}
	decision := semanticDynamicDecision{SchemaVersion: semanticDynamicDecisionSchema, Controls: []semanticDynamicDecisionRow{
		{Axis: "decay", PathKey: "stage/reverb_main/tail/decay/0", Role: "decay", Target: map[string]any{"time_seconds": 0.9}, Purpose: "short decay", Confidence: "high"},
	}}
	resp := server.semanticDynamicWaitingResponse("conversation-scc", map[string]any{"goal_id": "goal-scc", "run_id": "run-scc"}, ticket, brief, decision, plugingrabber.ParameterDigest{})
	if resp.InteractionRequests == nil || len(resp.InteractionRequests) != 1 {
		t.Fatalf("confirmation interaction was not created: %#v", resp.InteractionRequests)
	}
	card := resp.InteractionRequests[0]
	if card.Title != "确认执行语义参数控制器" {
		t.Fatalf("confirmation card title is not Chinese: %q", card.Title)
	}
	if card.Actions[0].Label != "确认执行" || card.Actions[1].Label != "取消" {
		t.Fatalf("confirmation actions are not Chinese: %#v", card.Actions)
	}
	wantReply := "语义控制计划已就绪，尚未写入任何参数；确认后才会执行参数控制器，取消则不会改动工程。"
	if resp.Reply != wantReply {
		t.Fatalf("confirmation reply changed:\n got: %s\nwant: %s", resp.Reply, wantReply)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || resp.StopReason != "semantic_dynamic_confirmation_required" {
		t.Fatalf("confirmation semantics changed: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}
}

func TestSemanticDynamicCancellationReplyIsChinese(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	ticket := semanticCardCopyDynamicTicket(t)
	interaction := PendingInteraction{
		ConversationID: "conversation-scc", GoalID: "goal-scc", RunID: "run-scc",
		RequestContext: map[string]any{"goal_id": "goal-scc", "run_id": "run-scc"},
		Data:           map[string]any{"ticket": ticket},
	}
	resp := server.continueSemanticDynamicExecutionInteraction(context.Background(), interaction, "cancel")
	if resp.GoalStatus != string(agentruntime.StatusCancelled) || boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("cancel semantics changed: %#v", resp)
	}
	if resp.Reply != "已取消语义参数控制；没有修改任何参数。" {
		t.Fatalf("cancel reply is not Chinese: %q", resp.Reply)
	}
}
