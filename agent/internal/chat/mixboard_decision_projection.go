package chat

import (
	"encoding/json"
	"strings"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

func mixboardDecisionContextForState(state map[string]any, targetCapabilityID string) ([]mixboard.DecisionContextRef, error) {
	projectUUID := firstStringFromMap(state, "project_uuid", "project_id", "id")
	if projectUUID == "" {
		projectUUID = firstStringFromMap(firstMapFromAny(state["project"]), "project_uuid", "project_id", "id")
	}
	if projectUUID == "" {
		return nil, nil
	}
	return mixboard.NewStore("").RelatedDecisionRefs(projectUUID, targetCapabilityID)
}

func mixboardDecisionArtifactRefs(refs []mixboard.DecisionContextRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = appendUniqueStrings(out, ref.Ref)
	}
	return out
}

// attachMixboardDecisionContext adds only bounded decision references and
// freshness state to the model-facing disclosure. Original facts are still
// read from their owning authority. An unavailable derived board is recorded
// as an omission and never blocks the capability's primary evidence path.
func attachMixboardDecisionContext(bundle *orchestration.ContextBundle, refs []mixboard.DecisionContextRef, readErr error) {
	if bundle == nil {
		return
	}
	if readErr != nil {
		bundle.OmissionReasons = appendUniqueStrings(bundle.OmissionReasons, "mixboard_decisions_unavailable")
		if bundle.Omissions == nil {
			bundle.Omissions = map[string]orchestration.OmissionStatus{}
		}
		bundle.Omissions["mixboard_decisions"] = orchestration.OmissionUnavailable
		return
	}
	if len(refs) == 0 {
		return
	}
	bundle.ArtifactRefs = appendUniqueStrings(bundle.ArtifactRefs, mixboardDecisionArtifactRefs(refs)...)
	disclosure := map[string]any{}
	if strings.TrimSpace(bundle.Disclosure) != "" {
		if err := json.Unmarshal([]byte(bundle.Disclosure), &disclosure); err != nil {
			bundle.OmissionReasons = appendUniqueStrings(bundle.OmissionReasons, "mixboard_decisions_disclosure_unavailable")
			if bundle.Omissions == nil {
				bundle.Omissions = map[string]orchestration.OmissionStatus{}
			}
			bundle.Omissions["mixboard_decisions"] = orchestration.OmissionUnavailable
			return
		}
	}
	disclosure["mixboard_decision_refs"] = refs
	data, err := json.Marshal(disclosure)
	if err != nil {
		return
	}
	bundle.Disclosure = string(data)
}

// attachMixboardDecisionProjection publishes only references from a terminal
// capability session. Execution success/failure remains owned by the runtime;
// a projection failure is surfaced as an audit warning and never rewritten as
// a false project-mutation failure.
func attachMixboardDecisionProjection(response *ChatResponse, session orchestration.PlanningSession) {
	if response == nil || !session.Terminal() || strings.TrimSpace(session.ProjectUUID) == "" {
		return
	}
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	if session.Invocation.CapabilityID == frequencyCleanupCapabilityID && session.FrozenPlan == nil {
		response.WorkflowData["mixboard_decision_status"] = "read_only_or_no_action_not_recorded"
		return
	}
	// C1 plug-in loading is a separately confirmed control-plane prerequisite,
	// not the frequency-cleanup decision itself. The following parameter batch
	// publishes the specialist decision after actual static-EQ execution.
	if session.Invocation.CapabilityID == frequencyCleanupCapabilityID && session.FrozenPlan != nil && len(session.FrozenPlan.ActionSet.Actions) == 1 && session.FrozenPlan.ActionSet.Actions[0].Command == c1LoadBatchCommand {
		response.WorkflowData["mixboard_decision_status"] = "deferred_until_c1_parameter_batch"
		return
	}
	result, err := mixboard.NewStore("").RecordCapabilitySession(session)
	if err != nil {
		response.WorkflowData["mixboard_decision_status"] = "persistence_failed"
		response.WorkflowData["mixboard_decision_error"] = err.Error()
		return
	}
	response.WorkflowData["mixboard_decision_status"] = "recorded"
	response.WorkflowData["mixboard_decision_record_id"] = result.Record.RecordID
	response.WorkflowData["mixboard_decision_record_ref"] = "mixboard-decision:" + result.Record.RecordID
	response.WorkflowData["mixboard_decision_board_ref"] = "mixboard-project:" + result.Board.ProjectUUID
	response.WorkflowData["mixboard_decision_current_status"] = decisionProjectionCurrentStatus(result.Board, result.Record.RecordID)
	response.WorkflowData["mixboard_needs_review_count"] = result.Board.NeedsReviewCount
}

func decisionProjectionCurrentStatus(board mixboard.ProjectDecisionBoard, recordID string) string {
	for _, decision := range board.Decisions {
		if decision.RecordID == recordID {
			return decision.CurrentStatus
		}
	}
	return ""
}
