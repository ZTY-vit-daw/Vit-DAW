package chat

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
)

type capabilityAuthorityBinding struct {
	CapabilityID   string                    `json:"capability_id"`
	ConversationID string                    `json:"conversation_id,omitempty"`
	SessionID      string                    `json:"session_id,omitempty"`
	Owner          orchestration.EngineOwner `json:"owner"`
	Status         string                    `json:"status"`
}

type capabilityAuthorityReport struct {
	GeneratedAt             string                       `json:"generated_at"`
	RolloutV1               []string                     `json:"rollout_v1,omitempty"`
	LiveVerified            bool                         `json:"live_verified"`
	LegacyCreationClosed    bool                         `json:"legacy_creation_closed"`
	LegacyPending           []capabilityAuthorityBinding `json:"legacy_pending,omitempty"`
	V1Active                []capabilityAuthorityBinding `json:"v1_active,omitempty"`
	V1TerminalCount         int                          `json:"v1_terminal_count"`
	Conflicts               []capabilityAuthorityBinding `json:"conflicts,omitempty"`
	SafeToDisableLegacyCode bool                         `json:"safe_to_disable_legacy_code"`
	StoreError              string                       `json:"store_error,omitempty"`
	Blockers                []string                     `json:"blockers,omitempty"`
}

func (s *Server) capabilityAuthorityReport() capabilityAuthorityReport {
	report := capabilityAuthorityReport{
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		LiveVerified:         environmentTrue("VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED"),
		LegacyCreationClosed: true,
		RolloutV1:            []string{staticBalanceCapabilityID, panLayoutCapabilityID},
	}
	if s != nil {
		if s.orchestrationRuntime != nil && s.orchestrationRuntime.Store != nil {
			sessions, listErr := orchestration.ListStoreSessions(s.orchestrationRuntime.Store)
			if listErr != nil {
				report.StoreError = listErr.Error()
			}
			for _, session := range sessions {
				if session.EngineOwner != orchestration.EngineV1 || (session.Invocation.CapabilityID != staticBalanceCapabilityID && session.Invocation.CapabilityID != panLayoutCapabilityID) {
					continue
				}
				if session.Terminal() {
					report.V1TerminalCount++
					continue
				}
				report.V1Active = append(report.V1Active, capabilityAuthorityBinding{
					CapabilityID: session.Invocation.CapabilityID, ConversationID: session.Invocation.ConversationID,
					SessionID: session.ID, Owner: session.EngineOwner, Status: string(session.Status),
				})
			}
		}
	}
	for _, legacy := range report.LegacyPending {
		for _, active := range report.V1Active {
			if legacy.CapabilityID == active.CapabilityID && legacy.ConversationID == active.ConversationID {
				report.Conflicts = append(report.Conflicts, active)
			}
		}
	}
	sortAuthorityBindings(report.LegacyPending)
	sortAuthorityBindings(report.V1Active)
	sortAuthorityBindings(report.Conflicts)
	if len(report.RolloutV1) < 2 {
		report.Blockers = append(report.Blockers, "B2/B3 rollout policy is not fully v1")
	}
	if report.StoreError != "" {
		report.Blockers = append(report.Blockers, "orchestration Store inventory failed: "+report.StoreError)
	}
	if !report.LiveVerified {
		report.Blockers = append(report.Blockers, "live VSP execution has not been marked verified")
	}
	if !report.LegacyCreationClosed {
		report.Blockers = append(report.Blockers, "legacy B2/B3 creation gate remains open")
	}
	if len(report.LegacyPending) > 0 {
		report.Blockers = append(report.Blockers, fmt.Sprintf("%d legacy pending Session(s) still require drain", len(report.LegacyPending)))
	}
	if len(report.Conflicts) > 0 {
		report.Blockers = append(report.Blockers, fmt.Sprintf("%d dual-authority conflict(s) require quarantine", len(report.Conflicts)))
	}
	report.SafeToDisableLegacyCode = len(report.Blockers) == 0
	return report
}

func sortAuthorityBindings(values []capabilityAuthorityBinding) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].CapabilityID == values[j].CapabilityID {
			if values[i].ConversationID == values[j].ConversationID {
				return values[i].SessionID < values[j].SessionID
			}
			return values[i].ConversationID < values[j].ConversationID
		}
		return values[i].CapabilityID < values[j].CapabilityID
	})
}

func environmentTrue(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func legacyCapabilityCreationClosed() bool {
	return true
}

// applyLegacyCapabilityCreationGate is a final safety net for requests that
// escaped deterministic routing after full cutover. It never converts a
// legacy candidate into v1 execution input.
func (s *Server) applyLegacyCapabilityCreationGate(res agentloop.Result) agentloop.Result {
	blocked := []string{}
	if res.ExecutionMemory.PendingStaticBalancePlan != nil {
		res.ExecutionMemory.PendingStaticBalancePlan = nil
		blocked = append(blocked, "B2")
	}
	if res.ExecutionMemory.PendingPanLayoutPlan != nil {
		res.ExecutionMemory.PendingPanLayoutPlan = nil
		blocked = append(blocked, "B3")
	}
	if len(blocked) == 0 {
		return res
	}
	res.Status = agentruntime.StatusFailed
	res.StopReason = "legacy_capability_creation_closed"
	res.Reply = "该能力已由 Project-aware Capability Runtime v1 接管；legacy AgentLoop 候选已被永久拒绝。请重新发起 " + strings.Join(blocked, "/") + " 请求。"
	res.Error = "legacy B2/B3 authority has been retired"
	return res
}

func capabilityAuthorityReportReply(report capabilityAuthorityReport) string {
	lines := []string{
		"Capability Runtime v1 authority report",
		fmt.Sprintf("- rollout_v1: %s", strings.Join(report.RolloutV1, ", ")),
		fmt.Sprintf("- live_verified: %t", report.LiveVerified),
		fmt.Sprintf("- legacy_creation_closed: %t", report.LegacyCreationClosed),
		fmt.Sprintf("- legacy_pending: %d", len(report.LegacyPending)),
		fmt.Sprintf("- v1_active: %d", len(report.V1Active)),
		fmt.Sprintf("- authority_conflicts: %d", len(report.Conflicts)),
		fmt.Sprintf("- safe_to_disable_legacy_code: %t", report.SafeToDisableLegacyCode),
	}
	for _, blocker := range report.Blockers {
		lines = append(lines, "- blocker: "+blocker)
	}
	return strings.Join(lines, "\n")
}
