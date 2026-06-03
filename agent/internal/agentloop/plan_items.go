package agentloop

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/planner"
)

func mergePlanItems(existing, updates []planner.PlanItem) []planner.PlanItem {
	out := make([]planner.PlanItem, 0, len(existing)+len(updates))
	byID := map[string]int{}
	for _, item := range existing {
		item = normalizePlanItem(item, len(out)+1)
		if item.ID == "" {
			continue
		}
		byID[item.ID] = len(out)
		out = append(out, item)
	}
	for _, item := range updates {
		item = normalizePlanItem(item, len(out)+1)
		if item.ID == "" {
			continue
		}
		if idx, ok := byID[item.ID]; ok {
			current := out[idx]
			if item.Description != "" {
				current.Description = item.Description
			}
			if item.Evidence != "" {
				current.Evidence = item.Evidence
			}
			if len(item.Metadata) > 0 {
				current.Metadata = item.Metadata
			}
			if item.Status != "" && !(current.Status == "completed" && (item.Status == "pending" || item.Status == "running")) {
				current.Status = item.Status
			}
			out[idx] = current
			continue
		}
		byID[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}

func normalizePlanItem(item planner.PlanItem, fallbackIndex int) planner.PlanItem {
	item.ID = strings.TrimSpace(item.ID)
	item.Description = strings.TrimSpace(item.Description)
	item.Status = normalizePlanStatus(item.Status)
	item.Evidence = strings.TrimSpace(item.Evidence)
	if item.ID == "" && item.Description != "" {
		item.ID = fmt.Sprintf("plan_item_%d", fallbackIndex)
	}
	if item.Status == "" && (item.ID != "" || item.Description != "") {
		item.Status = "pending"
	}
	if len(item.Metadata) == 0 {
		item.Metadata = nil
	}
	return item
}

func normalizePlanStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "todo", "open":
		return "pending"
	case "in_progress", "started", "executing":
		return "running"
	case "done", "complete", "succeeded", "success", "verified":
		return "completed"
	case "needs_confirmation", "waiting-confirmation", "waiting confirmation":
		return "waiting_confirmation"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func markPlanItemStatus(items []planner.PlanItem, id, status, evidence string) []planner.PlanItem {
	id = strings.TrimSpace(id)
	if id == "" {
		return items
	}
	status = normalizePlanStatus(status)
	evidence = strings.TrimSpace(evidence)
	for i := range items {
		if items[i].ID != id {
			continue
		}
		if status != "" {
			items[i].Status = status
		}
		if evidence != "" {
			items[i].Evidence = evidence
		}
		return items
	}
	item := planner.PlanItem{ID: id, Status: firstNonEmpty(status, "pending"), Evidence: evidence}
	return append(items, item)
}

func openPlanItems(items []planner.PlanItem, completion *planner.Completion) []planner.PlanItem {
	var out []planner.PlanItem
	for _, item := range items {
		item = normalizePlanItem(item, 0)
		if item.ID == "" {
			continue
		}
		switch item.Status {
		case "completed", "skipped", "cancelled", "blocked", "failed":
			continue
		default:
			out = append(out, item)
		}
	}
	return out
}

func planUpdateEvent(items []planner.PlanItem, message string) planner.TraceEvent {
	return planner.TraceEvent{
		Kind:      "plan_update",
		Message:   strings.TrimSpace(message),
		PlanItems: append([]planner.PlanItem(nil), items...),
	}
}

func planEvidenceText(ver planner.VerificationResult) string {
	if strings.TrimSpace(ver.Message) != "" {
		return strings.TrimSpace(ver.Message)
	}
	if strings.TrimSpace(ver.Postcondition) != "" && strings.TrimSpace(ver.Status) != "" {
		return strings.TrimSpace(ver.Postcondition) + ": " + strings.TrimSpace(ver.Status)
	}
	return strings.TrimSpace(ver.Status)
}

func finalGateIssue(state *runState, out planner.Output) string {
	if out.Completion != nil && out.Completion.Uncertain {
		return "planner marked completion as uncertain; continue observing, repairing, or ask the user instead of claiming success"
	}
	open := openPlanItems(state.planItems, out.Completion)
	if len(open) == 0 {
		return ""
	}
	names := make([]string, 0, len(open))
	for _, item := range open {
		names = append(names, firstNonEmpty(item.ID, item.Description))
	}
	return "cannot finish yet; open plan items remain: " + strings.Join(names, ", ")
}
