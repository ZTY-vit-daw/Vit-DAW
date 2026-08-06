package orchestration

import (
	"os"
	"strings"
)

type OwnerDecisionSource string

const (
	OwnerExistingSession OwnerDecisionSource = "existing_session"
	OwnerLegacyDrain     OwnerDecisionSource = "legacy_pending_drain"
	OwnerExplicitCanary  OwnerDecisionSource = "explicit_canary"
	OwnerRolloutPolicy   OwnerDecisionSource = "rollout_policy"
	OwnerLegacyDefault   OwnerDecisionSource = "legacy_default"
	OwnerConflict        OwnerDecisionSource = "authority_conflict"
)

type OwnerDecision struct {
	Owner      EngineOwner         `json:"owner"`
	Source     OwnerDecisionSource `json:"source"`
	Conflict   bool                `json:"conflict"`
	Capability string              `json:"capability_id"`
	SessionID  string              `json:"session_id,omitempty"`
}

type OwnerPolicy struct {
	V1Capabilities map[string]bool
}

func OwnerPolicyFromEnvironment() OwnerPolicy {
	return ParseOwnerPolicy(os.Getenv("VIT_CAPABILITY_RUNTIME_V1_ROLLOUT"))
}

func ParseOwnerPolicy(value string) OwnerPolicy {
	policy := OwnerPolicy{V1Capabilities: map[string]bool{}}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		switch strings.TrimSpace(token) {
		case "all", "v1":
			policy.V1Capabilities["*"] = true
		case "b2", "static_balance", "static_mix.static_balance.v0":
			policy.V1Capabilities["static_mix.static_balance.v0"] = true
		case "b3", "pan_layout", "static_mix.pan_layout.v0":
			policy.V1Capabilities["static_mix.pan_layout.v0"] = true
		}
	}
	return policy
}

func (p OwnerPolicy) Select(capabilityID string, explicitV1, legacyPending bool, existing *PlanningSession) OwnerDecision {
	capabilityID = strings.TrimSpace(capabilityID)
	decision := OwnerDecision{Owner: EngineLegacy, Source: OwnerLegacyDefault, Capability: capabilityID}
	if existing != nil {
		decision.Owner = existing.EngineOwner
		decision.Source = OwnerExistingSession
		decision.SessionID = existing.ID
		if legacyPending && existing.EngineOwner == EngineV1 {
			decision.Conflict = true
			decision.Source = OwnerConflict
		}
		return decision
	}
	if legacyPending {
		decision.Source = OwnerLegacyDrain
		return decision
	}
	if explicitV1 {
		decision.Owner = EngineV1
		decision.Source = OwnerExplicitCanary
		return decision
	}
	if p.V1Capabilities["*"] || p.V1Capabilities[capabilityID] {
		decision.Owner = EngineV1
		decision.Source = OwnerRolloutPolicy
	}
	return decision
}
