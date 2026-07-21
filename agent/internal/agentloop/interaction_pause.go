package agentloop

import (
	"encoding/json"
	"strings"

	agentruntime "vit-daw-agent/internal/runtime"
)

// pendingInteractionPause describes a formal user-interaction card emitted by
// a tool. The loop must stop at this boundary instead of continuing with tool
// calls that happened to be returned in the same model turn.
type pendingInteractionPause struct {
	Status     agentruntime.GoalStatus
	StopReason string
	Reply      string
}

type toolInteractionRequest struct {
	Type   string `json:"type"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func pendingInteractionPauseForResult(result map[string]any) (pendingInteractionPause, bool) {
	raw, ok := result["interaction_requests"]
	if !ok || raw == nil {
		return pendingInteractionPause{}, false
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return pendingInteractionPause{}, false
	}
	var requests []toolInteractionRequest
	if err := json.Unmarshal(payload, &requests); err != nil {
		return pendingInteractionPause{}, false
	}
	for _, request := range requests {
		status := strings.ToLower(strings.TrimSpace(request.Status))
		if status != "waiting_for_user" && status != "waiting_user" && status != "pending_user" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(request.Kind + " " + request.Type))
		pause := pendingInteractionPause{
			Status:     agentruntime.StatusWaitingClarification,
			StopReason: StopReasonNeedsClarification,
			Reply:      firstNonEmpty(firstMapText(result, "reply", "message"), strings.TrimSpace(request.Body), strings.TrimSpace(request.Title)),
		}
		if strings.Contains(kind, "confirmation") {
			pause.Status = agentruntime.StatusWaitingConfirmation
			pause.StopReason = StopReasonNeedsConfirmation
		}
		return pause, true
	}
	return pendingInteractionPause{}, false
}
