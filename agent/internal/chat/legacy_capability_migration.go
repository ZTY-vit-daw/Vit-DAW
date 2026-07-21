package chat

import (
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
)

const legacyCapabilityRetirementReason = "legacy B2/B3 authority retired after capability runtime v1 cutover"

// The functions in this file are deliberately one-way migration readers.
// They recognize persisted legacy B2/B3 authority and turn it into terminal
// audit data. They never reconstruct an executable confirmation handler.
func retireLegacyCapabilityExecutionMemory(memory map[string]agentloop.ExecutionMemory) map[string]agentloop.ExecutionMemory {
	for conversationID, value := range memory {
		value.PendingStaticBalancePlan = nil
		value.PendingPanLayoutPlan = nil
		if hasAgentLoopExecutionMemory(value) {
			memory[conversationID] = value
		} else {
			delete(memory, conversationID)
		}
	}
	return memory
}

func retireLegacyCapabilityPendingPlans(plans map[string]PendingPlan) map[string]PendingPlan {
	for id, plan := range plans {
		if legacyCapabilityWorkflow(plan.Workflow) {
			delete(plans, id)
		}
	}
	return plans
}

func retireLegacyCapabilityInteractions(interactions map[string]PendingInteraction) map[string]PendingInteraction {
	for id, interaction := range interactions {
		if strings.EqualFold(strings.TrimSpace(interaction.Workflow), "capability_runtime_v1") {
			continue
		}
		if legacyCapabilityWorkflow(interaction.Workflow) ||
			legacyCapabilityInteractionKind(interaction.Kind) ||
			legacyCapabilityInteractionKind(interaction.Type) {
			delete(interactions, id)
		}
	}
	return interactions
}

func retireLegacyCapabilityCandidates(state projectAgentRuntimeState) []agentprotocol.PendingCandidate {
	byID := make(map[string]agentprotocol.PendingCandidate, len(state.PendingCandidates)+len(state.PendingStaticBalancePlans)+len(state.PendingPanLayoutPlans))
	for _, candidate := range state.PendingCandidates {
		if legacyCapabilityCandidate(candidate) && !legacyCapabilityCandidateTerminal(candidate.Status) {
			candidate = retiredLegacyCapabilityCandidate(candidate, state.SavedAt)
		}
		if strings.TrimSpace(candidate.ID) != "" {
			mergeLegacyCapabilityCandidate(byID, candidate)
		}
	}
	createdAt := ""
	if !state.SavedAt.IsZero() {
		createdAt = state.SavedAt.UTC().Format(time.RFC3339Nano)
	}
	for conversationID, plan := range state.PendingStaticBalancePlans {
		candidate := plan.ToPendingCandidate(conversationID, "", "", createdAt)
		candidate = retiredLegacyCapabilityCandidate(candidate, state.SavedAt)
		mergeLegacyCapabilityCandidate(byID, candidate)
	}
	for conversationID, plan := range state.PendingPanLayoutPlans {
		candidate := plan.ToPendingCandidate(conversationID, "", "", createdAt)
		candidate = retiredLegacyCapabilityCandidate(candidate, state.SavedAt)
		mergeLegacyCapabilityCandidate(byID, candidate)
	}
	for conversationID, memory := range state.ConversationMemory {
		if memory.PendingStaticBalancePlan != nil {
			candidate := memory.PendingStaticBalancePlan.ToPendingCandidate(conversationID, "", "", createdAt)
			candidate = retiredLegacyCapabilityCandidate(candidate, state.SavedAt)
			mergeLegacyCapabilityCandidate(byID, candidate)
		}
		if memory.PendingPanLayoutPlan != nil {
			candidate := memory.PendingPanLayoutPlan.ToPendingCandidate(conversationID, "", "", createdAt)
			candidate = retiredLegacyCapabilityCandidate(candidate, state.SavedAt)
			mergeLegacyCapabilityCandidate(byID, candidate)
		}
	}
	out := make([]agentprotocol.PendingCandidate, 0, len(byID))
	for _, candidate := range byID {
		out = append(out, candidate)
	}
	return out
}

// Existing terminal audit truth always wins over a duplicate pending plan
// reconstructed from legacy maps or ExecutionMemory. This is important for
// projects upgraded after an already verified legacy execution: cutover must
// retire executable authority without rewriting history from verified to
// rejected.
func mergeLegacyCapabilityCandidate(byID map[string]agentprotocol.PendingCandidate, candidate agentprotocol.PendingCandidate) {
	if strings.TrimSpace(candidate.ID) == "" {
		return
	}
	identity := legacyCapabilityCandidateIdentity(candidate)
	for _, existing := range byID {
		if !legacyCapabilityCandidateTerminal(existing.Status) {
			continue
		}
		if existing.ID == candidate.ID || (identity != "" && legacyCapabilityCandidateIdentity(existing) == identity) {
			return
		}
	}
	byID[candidate.ID] = candidate
}

func legacyCapabilityCandidateIdentity(candidate agentprotocol.PendingCandidate) string {
	if !legacyCapabilityCandidate(candidate) {
		return ""
	}
	parts := []string{
		strings.ToLower(strings.TrimSpace(candidate.CandidateType)),
		strings.TrimSpace(candidate.Source.ConversationID),
		legacyCapabilityMetadataText(candidate.Source.Metadata, "plan_id"),
		legacyCapabilityMetadataText(candidate.Source.Metadata, "context_pack_id"),
		legacyCapabilityMetadataText(candidate.Source.Metadata, "candidate_plan_id"),
	}
	if parts[0] == "" || parts[1] == "" || (parts[2] == "" && parts[3] == "" && parts[4] == "") {
		return ""
	}
	return strings.Join(parts, "|")
}

func legacyCapabilityMetadataText(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func legacyCapabilityCandidateTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case agentprotocol.PendingStatusRejected,
		agentprotocol.PendingStatusCommitted,
		agentprotocol.PendingStatusVerified,
		agentprotocol.PendingStatusVerificationFailed,
		agentprotocol.PendingStatusFailed,
		agentprotocol.PendingStatusBlocked:
		return true
	default:
		return false
	}
}

func retiredLegacyCapabilityCandidate(candidate agentprotocol.PendingCandidate, retiredAt time.Time) agentprotocol.PendingCandidate {
	candidate.Status = agentprotocol.PendingStatusRejected
	if candidate.Source.Metadata == nil {
		candidate.Source.Metadata = map[string]any{}
	}
	candidate.Source.Metadata["transition_reason"] = legacyCapabilityRetirementReason
	if !retiredAt.IsZero() {
		candidate.Source.Metadata["retired_at"] = retiredAt.UTC().Format(time.RFC3339Nano)
	}
	return candidate
}

func legacyCapabilityCandidate(candidate agentprotocol.PendingCandidate) bool {
	switch strings.ToLower(strings.TrimSpace(candidate.CandidateType)) {
	case "static_balance", "static_balance_plan", "pan_layout", "pan_layout_plan":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(candidate.Source.LegacyKind)) {
	case "pendingstaticbalanceplan", "pendingpanlayoutplan":
		return true
	}
	return false
}

func legacyCapabilityWorkflow(workflow string) bool {
	switch strings.ToLower(strings.TrimSpace(workflow)) {
	case "static_balance", "pan_layout":
		return true
	default:
		return false
	}
}

func legacyCapabilityInteractionKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "static_balance_confirmation", "pan_layout_confirmation":
		return true
	default:
		return false
	}
}
