package orchestration

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

// ContextWindowBudget is the admission budget for one model call. It is
// intentionally separate from capability computation budgets: a capability
// may analyze the full project while this envelope discloses only bounded
// decision material.
type ContextWindowBudget struct {
	MaxWindowTokens      int `json:"max_window_tokens"`
	OutputReserveTokens  int `json:"output_reserve_tokens"`
	MaxRegistryEntries   int `json:"max_registry_entries"`
	MaxToolSchemas       int `json:"max_tool_schemas"`
	MaxConversationTurns int `json:"max_conversation_turns"`
}

type ContextEntry struct {
	ID           string         `json:"id"`
	Content      string         `json:"content,omitempty"`
	Reference    string         `json:"reference,omitempty"`
	Priority     int            `json:"priority,omitempty"`
	Availability OmissionStatus `json:"availability,omitempty"`
}

type ContextEnvelopeRequest struct {
	System            string              `json:"system"`
	Registry          []ContextEntry      `json:"registry,omitempty"`
	ToolSchemas       []ContextEntry      `json:"tool_schemas,omitempty"`
	Session           PlanningSession     `json:"session"`
	Bundle            ContextBundle       `json:"bundle"`
	ConversationTurns []ContextEntry      `json:"conversation_turns,omitempty"`
	Budget            ContextWindowBudget `json:"budget"`
}

// ContextEnvelope is the only material admitted to a v1 model call. Full
// derived models and old conversation turns remain addressable by reference.
type ContextEnvelope struct {
	System               string                    `json:"system"`
	Registry             []ContextEntry            `json:"registry,omitempty"`
	ToolSchemas          []ContextEntry            `json:"tool_schemas,omitempty"`
	TypedSession         string                    `json:"typed_session"`
	BundleDisclosure     string                    `json:"bundle_disclosure,omitempty"`
	BundleRefs           []string                  `json:"bundle_refs,omitempty"`
	ConversationTurns    []ContextEntry            `json:"conversation_turns,omitempty"`
	EstimatedInputTokens int                       `json:"estimated_input_tokens"`
	MaxInputTokens       int                       `json:"max_input_tokens"`
	OutputReserveTokens  int                       `json:"output_reserve_tokens"`
	Omissions            map[string]OmissionStatus `json:"omissions,omitempty"`
	Valid                bool                      `json:"valid"`
	Blockers             []string                  `json:"blockers,omitempty"`
}

func DefaultContextWindowBudget() ContextWindowBudget {
	return ContextWindowBudget{
		MaxWindowTokens:      32_000,
		OutputReserveTokens:  4_000,
		MaxRegistryEntries:   8,
		MaxToolSchemas:       12,
		MaxConversationTurns: 8,
	}
}

func BuildContextEnvelope(request ContextEnvelopeRequest) ContextEnvelope {
	budget := normalizeContextWindowBudget(request.Budget)
	out := ContextEnvelope{
		System:              strings.TrimSpace(request.System),
		BundleRefs:          append([]string(nil), request.Bundle.ArtifactRefs...),
		MaxInputTokens:      budget.MaxWindowTokens - budget.OutputReserveTokens,
		OutputReserveTokens: budget.OutputReserveTokens,
		Omissions:           map[string]OmissionStatus{},
		Valid:               true,
	}
	for key, status := range request.Bundle.Omissions {
		out.Omissions["bundle."+key] = status
	}

	sessionJSON, _ := json.Marshal(compactSessionForContext(request.Session))
	out.TypedSession = string(sessionJSON)
	used := estimatedContextTokens(out.System) + estimatedContextTokens(out.TypedSession)
	if out.System == "" {
		out.Valid = false
		out.Omissions["system"] = OmissionUnavailable
		out.Blockers = append(out.Blockers, "system_context_missing")
	}
	if strings.TrimSpace(request.Session.ID) == "" {
		out.Valid = false
		out.Omissions["typed_session"] = OmissionUnavailable
		out.Blockers = append(out.Blockers, "typed_session_missing")
	}
	if used > out.MaxInputTokens {
		out.Valid = false
		out.Blockers = append(out.Blockers, "required_system_and_session_exceed_context_budget")
	}

	bundle := strings.TrimSpace(request.Bundle.Disclosure)
	bundleCost := estimatedContextTokens(bundle)
	if bundle == "" {
		if len(out.BundleRefs) > 0 {
			out.Omissions["bundle.disclosure"] = OmissionByReference
		} else {
			out.Omissions["bundle.disclosure"] = OmissionUnavailable
		}
	} else if used+bundleCost <= out.MaxInputTokens {
		out.BundleDisclosure = bundle
		used += bundleCost
	} else if len(out.BundleRefs) > 0 {
		out.Omissions["bundle.disclosure"] = OmissionByReference
	} else {
		out.Omissions["bundle.disclosure"] = OmissionBudget
		out.Valid = false
		out.Blockers = append(out.Blockers, "bundle_disclosure_exceeds_context_budget_without_reference")
	}

	remaining := func() int { return out.MaxInputTokens - used }
	out.Registry, used = admitRankedEntries(request.Registry, budget.MaxRegistryEntries, "registry", remaining(), used, out.Omissions)
	out.ToolSchemas, used = admitRankedEntries(request.ToolSchemas, budget.MaxToolSchemas, "tool_schemas", remaining(), used, out.Omissions)
	out.ConversationTurns, used = admitRecentTurns(request.ConversationTurns, budget.MaxConversationTurns, remaining(), used, out.Omissions)
	out.EstimatedInputTokens = used
	if used > out.MaxInputTokens {
		out.Valid = false
		out.Blockers = append(out.Blockers, "assembled_context_exceeds_budget")
	}
	return out
}

func normalizeContextWindowBudget(value ContextWindowBudget) ContextWindowBudget {
	defaults := DefaultContextWindowBudget()
	if value.MaxWindowTokens <= 0 {
		value.MaxWindowTokens = defaults.MaxWindowTokens
	}
	if value.OutputReserveTokens <= 0 || value.OutputReserveTokens >= value.MaxWindowTokens {
		value.OutputReserveTokens = defaults.OutputReserveTokens
		if value.OutputReserveTokens >= value.MaxWindowTokens {
			value.OutputReserveTokens = value.MaxWindowTokens / 4
		}
	}
	if value.MaxRegistryEntries <= 0 {
		value.MaxRegistryEntries = defaults.MaxRegistryEntries
	}
	if value.MaxToolSchemas <= 0 {
		value.MaxToolSchemas = defaults.MaxToolSchemas
	}
	if value.MaxConversationTurns <= 0 {
		value.MaxConversationTurns = defaults.MaxConversationTurns
	}
	return value
}

func admitRankedEntries(input []ContextEntry, limit int, category string, available, used int, omissions map[string]OmissionStatus) ([]ContextEntry, int) {
	if len(input) == 0 {
		omissions[category] = OmissionNotRequested
		return nil, used
	}
	entries := append([]ContextEntry(nil), input...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Priority == entries[j].Priority {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Priority > entries[j].Priority
	})
	if len(entries) > limit {
		omissions[category+".remainder"] = OmissionByReference
		entries = entries[:limit]
	}
	out := make([]ContextEntry, 0, len(entries))
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Content = strings.TrimSpace(entry.Content)
		if entry.Availability != "" {
			omissions[category+"."+entry.ID] = entry.Availability
			continue
		}
		if entry.Content == "" {
			if strings.TrimSpace(entry.Reference) != "" {
				omissions[category+"."+entry.ID] = OmissionByReference
			} else {
				omissions[category+"."+entry.ID] = OmissionUnavailable
			}
			continue
		}
		cost := estimatedContextTokens(entry.ID) + estimatedContextTokens(entry.Content) + 4
		if cost > available {
			omissions[category+".remainder"] = OmissionBudget
			continue
		}
		out = append(out, entry)
		available -= cost
		used += cost
	}
	return out, used
}

func admitRecentTurns(input []ContextEntry, limit, available, used int, omissions map[string]OmissionStatus) ([]ContextEntry, int) {
	if len(input) == 0 {
		omissions["conversation"] = OmissionNotRequested
		return nil, used
	}
	start := 0
	if len(input) > limit {
		start = len(input) - limit
		omissions["conversation.older_turns"] = OmissionByReference
	}
	candidates := append([]ContextEntry(nil), input[start:]...)
	selectedReverse := make([]ContextEntry, 0, len(candidates))
	for index := len(candidates) - 1; index >= 0; index-- {
		entry := candidates[index]
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Content = strings.TrimSpace(entry.Content)
		if entry.Availability != "" || entry.Content == "" {
			continue
		}
		cost := estimatedContextTokens(entry.ID) + estimatedContextTokens(entry.Content) + 4
		if cost > available {
			omissions["conversation.older_turns"] = OmissionBudget
			continue
		}
		selectedReverse = append(selectedReverse, entry)
		available -= cost
		used += cost
	}
	out := make([]ContextEntry, len(selectedReverse))
	for index := range selectedReverse {
		out[len(selectedReverse)-1-index] = selectedReverse[index]
	}
	return out, used
}

func compactSessionForContext(session PlanningSession) any {
	return struct {
		SchemaVersion  string               `json:"schema_version"`
		ID             string               `json:"id"`
		ProjectUUID    string               `json:"project_uuid"`
		EngineOwner    EngineOwner          `json:"engine_owner"`
		Revision       uint64               `json:"revision"`
		Status         SessionStatus        `json:"status"`
		Goal           string               `json:"goal"`
		Invocation     CapabilityInvocation `json:"invocation"`
		ActiveProposal *Proposal            `json:"active_proposal,omitempty"`
		Authorization  *Authorization       `json:"authorization,omitempty"`
		ExecutionState string               `json:"execution_state,omitempty"`
	}{
		SchemaVersion: session.SchemaVersion, ID: session.ID, ProjectUUID: session.ProjectUUID,
		EngineOwner: session.EngineOwner, Revision: session.Revision, Status: session.Status,
		Goal: session.Goal, Invocation: session.Invocation, ActiveProposal: session.ActiveProposal,
		Authorization: session.Authorization, ExecutionState: executionState(session.Execution),
	}
}

func executionState(execution *ExecutionRecord) string {
	if execution == nil {
		return ""
	}
	return execution.Status
}

// estimatedContextTokens uses an intentionally conservative tokenizer-free
// estimate: ASCII is charged at four bytes/token and non-ASCII at one
// rune/token. The admission invariant is boundedness, not billing precision.
func estimatedContextTokens(value string) int {
	ascii := 0
	nonASCII := 0
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		if r <= 0x7f {
			ascii++
		} else {
			nonASCII++
		}
	}
	return (ascii+3)/4 + nonASCII
}
