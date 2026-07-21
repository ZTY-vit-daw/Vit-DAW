package capabilityruntime

import (
	"sort"
	"strings"
)

const ReadinessSchemaVersion = "capability.readiness.v1"

const (
	ConditionReady   = "ready"
	ConditionWarning = "warning"
	ConditionBlocked = "blocked"
	ConditionMissing = "missing"
)

type Condition struct {
	ID           string   `json:"id"`
	Required     bool     `json:"required"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary,omitempty"`
	KnownCount   int      `json:"known_count,omitempty"`
	TotalCount   int      `json:"total_count,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Remediation  []string `json:"remediation,omitempty"`
}

type Readiness struct {
	SchemaVersion string      `json:"schema_version"`
	CapabilityID  string      `json:"capability_id"`
	Status        string      `json:"status"`
	CanProceed    bool        `json:"can_proceed"`
	CanSuggest    bool        `json:"can_suggest"`
	Conditions    []Condition `json:"conditions"`
	BlockedBy     []string    `json:"blocked_by,omitempty"`
	Warnings      []string    `json:"warnings,omitempty"`
	EvidenceRefs  []string    `json:"evidence_refs,omitempty"`
	Remediation   []string    `json:"remediation,omitempty"`
}

// Evaluate is deterministic. Capability readiness must never require an LLM call.
func Evaluate(capabilityID string, conditions []Condition) Readiness {
	out := Readiness{
		SchemaVersion: ReadinessSchemaVersion,
		CapabilityID:  strings.TrimSpace(capabilityID),
		Status:        ConditionReady,
		CanProceed:    true,
		CanSuggest:    true,
		Conditions:    append([]Condition(nil), conditions...),
	}
	for i := range out.Conditions {
		condition := &out.Conditions[i]
		condition.ID = strings.TrimSpace(condition.ID)
		condition.Status = normalizeStatus(condition.Status)
		condition.EvidenceRefs = unique(condition.EvidenceRefs)
		condition.Remediation = unique(condition.Remediation)
		out.EvidenceRefs = append(out.EvidenceRefs, condition.EvidenceRefs...)
		if condition.Required && (condition.Status == ConditionBlocked || condition.Status == ConditionMissing) {
			out.CanProceed = false
			out.BlockedBy = append(out.BlockedBy, condition.ID)
			out.Remediation = append(out.Remediation, condition.Remediation...)
		}
		if condition.Status == ConditionWarning {
			out.Warnings = append(out.Warnings, condition.ID)
		}
	}
	out.BlockedBy = unique(out.BlockedBy)
	out.Warnings = unique(out.Warnings)
	out.EvidenceRefs = unique(out.EvidenceRefs)
	out.Remediation = unique(out.Remediation)
	if !out.CanProceed {
		out.Status = ConditionBlocked
	} else if len(out.Warnings) > 0 {
		out.Status = ConditionWarning
	}
	return out
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ConditionReady:
		return ConditionReady
	case ConditionWarning, "partial":
		return ConditionWarning
	case ConditionMissing:
		return ConditionMissing
	default:
		return ConditionBlocked
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
