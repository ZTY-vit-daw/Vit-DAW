package observationrouter

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

type Router struct{}

func New() Router {
	return Router{}
}

func RequestFromTool(tool string, args map[string]any, source agentprotocol.Source) (agentprotocol.ObservationRequest, bool) {
	name := normalizeTool(tool, args)
	if name != "mix.observe" && name != "mix.request_observation" {
		return agentprotocol.ObservationRequest{}, false
	}
	intent := firstNonEmpty(text(args["mom_intent"]), text(args["projection_intent"]), text(args["workflow_intent"]), text(args["intent"]), text(args["goal_text"]), text(args["goal"]), "mix_diagnosis")
	targetRef := "project"
	if trackID := firstNonEmpty(text(args["track_id"]), text(args["selected_track_id"])); trackID != "" {
		targetRef = "track:" + trackID
	}
	return agentprotocol.ObservationRequest{
		ID:              agentprotocol.NormalizeID("observation_request", source.ConversationID, source.GoalID, source.RunID, name, targetRef),
		Kind:            agentprotocol.KindObservationRequest,
		Intent:          intent,
		TargetRef:       targetRef,
		RequiredSources: []string{name, "project.shadow"},
		Status:          "requested",
		Source:          source,
	}, true
}

func ResultFromToolResult(tool string, args map[string]any, result map[string]any, source agentprotocol.Source) (agentprotocol.ObservationResult, bool) {
	request, ok := RequestFromTool(tool, args, source)
	if !ok {
		return agentprotocol.ObservationResult{}, false
	}
	observationID := firstNonEmpty(text(result["observation_id"]), text(mapValue(result["observation"])["observation_id"]), text(mapValue(result["digest"])["observation_id"]))
	contextPackID := firstNonEmpty(text(result["context_pack_id"]), text(result["context_pack_path"]), observationID)
	summary := firstNonEmpty(text(result["summary"]), text(mapValue(result["digest"])["summary"]), text(mapValue(result["observation"])["summary"]))
	if summary == "" && observationID != "" {
		summary = fmt.Sprintf("Observation %s completed.", observationID)
	}
	status := firstNonEmpty(text(result["status"]), "ok")
	metadata := map[string]any{
		"target_ref": request.TargetRef,
		"tool":       normalizeTool(tool, args),
	}
	if acousticStatus := mapValue(result["acoustic_package_status"]); len(acousticStatus) > 0 {
		metadata["acoustic_package_status"] = acousticStatus
	}
	if path := text(result["acoustic_package_status_path"]); path != "" {
		metadata["acoustic_package_status_path"] = path
	}
	return agentprotocol.ObservationResult{
		ID:            agentprotocol.NormalizeID("observation_result", request.ID, observationID, contextPackID),
		Kind:          agentprotocol.KindObservationResult,
		Intent:        request.Intent,
		ContextPackID: contextPackID,
		Summary:       summary,
		SourceRefs:    compactStrings(observationID, contextPackID),
		Status:        status,
		Source:        source,
		Metadata:      metadata,
	}, true
}

func normalizeTool(tool string, args map[string]any) string {
	name := strings.TrimSpace(tool)
	if name == "" {
		name = firstNonEmpty(text(args["tool"]), text(args["cmd"]), text(args["command"]))
	}
	switch strings.TrimSpace(name) {
	case "mix_observe":
		return "mix.observe"
	case "mix_request_observation":
		return "mix.request_observation"
	default:
		return strings.TrimSpace(name)
	}
}

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func text(value any) string {
	out := strings.TrimSpace(fmt.Sprint(value))
	if out == "" || out == "<nil>" {
		return ""
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func compactStrings(values ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
