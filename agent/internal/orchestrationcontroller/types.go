// Package orchestrationcontroller selects and leases exactly one top-level
// controller for a conversation. Selection is model-proposed and host-validated;
// controllers may share lower layers but cannot concurrently own continuation.
package orchestrationcontroller

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const DecisionSchema = "orchestration_controller_decision.v1"

type Kind string

const (
	OrdinaryConversation Kind = "ordinary_conversation"
	DirectTypedAction    Kind = "direct_typed_action"
	MinimalAudioClosure  Kind = "minimal_audio_closure"
	ProjectMixWorkflow   Kind = "project_mix_workflow"
)

type Decision struct {
	SchemaVersion string `json:"schema_version"`
	Controller    Kind   `json:"controller"`
	TargetScope   string `json:"target_scope"`
	Authorization string `json:"authorization"`
	SourceRoute   string `json:"source_route"`
	Reason        string `json:"reason"`
}

type SelectionInput struct {
	SemanticRoute      string
	TargetScope        string
	ControlMode        string
	Authorization      string
	ProposedController Kind
	Reason             string
}

func Select(input SelectionInput) (Decision, error) {
	route := strings.ToLower(strings.TrimSpace(input.SemanticRoute))
	scope := strings.ToLower(strings.TrimSpace(input.TargetScope))
	proposed := Kind(strings.ToLower(strings.TrimSpace(string(input.ProposedController))))
	controller := proposed
	if controller == "" {
		switch route {
		case "discussion", "other":
			controller = OrdinaryConversation
		case "explicit_control":
			controller = DirectTypedAction
		case "observation", "open_semantic":
			controller = MinimalAudioClosure
		default:
			return Decision{}, fmt.Errorf("semantic route %q cannot select a controller", route)
		}
	}
	decision := Decision{
		SchemaVersion: DecisionSchema, Controller: controller, TargetScope: scope,
		Authorization: strings.ToLower(strings.TrimSpace(input.Authorization)), SourceRoute: route, Reason: strings.TrimSpace(input.Reason),
	}
	if err := Validate(decision, input.ControlMode); err != nil {
		return Decision{}, err
	}
	return decision, nil
}

func Validate(decision Decision, controlMode string) error {
	if decision.SchemaVersion != DecisionSchema {
		return fmt.Errorf("schema_version must be %s", DecisionSchema)
	}
	if strings.TrimSpace(decision.Reason) == "" {
		return fmt.Errorf("controller reason is required")
	}
	mode := strings.ToLower(strings.TrimSpace(controlMode))
	switch decision.Controller {
	case OrdinaryConversation:
		if decision.SourceRoute != "discussion" && decision.SourceRoute != "other" {
			return fmt.Errorf("ordinary_conversation is incompatible with route %s", decision.SourceRoute)
		}
	case DirectTypedAction:
		if decision.SourceRoute != "explicit_control" || mode != "typed_control" || decision.Authorization != "action_requested" {
			return fmt.Errorf("direct_typed_action requires explicit typed control authorization")
		}
	case MinimalAudioClosure:
		if decision.SourceRoute != "observation" && decision.SourceRoute != "open_semantic" {
			return fmt.Errorf("minimal_audio_closure is incompatible with route %s", decision.SourceRoute)
		}
		if decision.TargetScope == "none" {
			return fmt.Errorf("minimal_audio_closure requires a target scope")
		}
		if decision.SourceRoute == "observation" && (mode != "observe_only" || decision.Authorization != "observe_only") {
			return fmt.Errorf("diagnostic closure requires observe-only authorization")
		}
		if decision.SourceRoute == "open_semantic" && (mode != "semantic_loop" || decision.Authorization != "action_requested") {
			return fmt.Errorf("treatment closure requires semantic-loop authorization")
		}
	case ProjectMixWorkflow:
		if decision.SourceRoute != "open_semantic" || decision.TargetScope != "project_context" || mode != "semantic_loop" || decision.Authorization != "action_requested" {
			return fmt.Errorf("project_mix_workflow requires an explicit full-project semantic request")
		}
	default:
		return fmt.Errorf("unknown controller %q", decision.Controller)
	}
	return nil
}

type OwnerStatus string

const (
	OwnerActive  OwnerStatus = "active"
	OwnerSettled OwnerStatus = "settled"
)

type Owner struct {
	ConversationID   string      `json:"conversation_id"`
	ControllerID     string      `json:"controller_id"`
	Controller       Kind        `json:"controller"`
	TargetScope      string      `json:"target_scope"`
	Status           OwnerStatus `json:"status"`
	Revision         uint64      `json:"revision"`
	SettlementReason string      `json:"settlement_reason,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type Registry struct {
	mu     sync.RWMutex
	owners map[string]Owner
}

func NewRegistry() *Registry { return &Registry{owners: map[string]Owner{}} }

func (r *Registry) Acquire(conversationID, controllerID string, decision Decision, now time.Time) (Owner, bool, error) {
	conversationID, controllerID = strings.TrimSpace(conversationID), strings.TrimSpace(controllerID)
	if conversationID == "" || controllerID == "" {
		return Owner{}, false, fmt.Errorf("conversation_id and controller_id are required")
	}
	if decision.SchemaVersion != DecisionSchema {
		return Owner{}, false, fmt.Errorf("controller decision is not validated")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, exists := r.owners[conversationID]; exists && current.Status == OwnerActive {
		if current.ControllerID == controllerID && current.Controller == decision.Controller {
			return current, false, nil
		}
		return Owner{}, false, fmt.Errorf("conversation %s is owned by %s controller %s", conversationID, current.Controller, current.ControllerID)
	}
	owner := Owner{ConversationID: conversationID, ControllerID: controllerID, Controller: decision.Controller, TargetScope: decision.TargetScope, Status: OwnerActive, Revision: 1, CreatedAt: now, UpdatedAt: now}
	r.owners[conversationID] = owner
	return owner, true, nil
}

func (r *Registry) Settle(conversationID, controllerID string, expectedRevision uint64, reason string, now time.Time) (Owner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	owner, exists := r.owners[strings.TrimSpace(conversationID)]
	if !exists {
		return Owner{}, fmt.Errorf("conversation has no controller owner")
	}
	if owner.ControllerID != strings.TrimSpace(controllerID) {
		return Owner{}, fmt.Errorf("controller %s does not own the conversation", controllerID)
	}
	if owner.Revision != expectedRevision {
		return Owner{}, fmt.Errorf("owner revision conflict: expected %d, current %d", expectedRevision, owner.Revision)
	}
	if owner.Status == OwnerSettled {
		return owner, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	owner.Status, owner.Revision, owner.SettlementReason, owner.UpdatedAt = OwnerSettled, owner.Revision+1, strings.TrimSpace(reason), now.UTC()
	r.owners[owner.ConversationID] = owner
	return owner, nil
}

func (r *Registry) Active(conversationID string) (Owner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	owner, exists := r.owners[strings.TrimSpace(conversationID)]
	return owner, exists && owner.Status == OwnerActive
}

func (r *Registry) Snapshot() map[string]Owner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Owner, len(r.owners))
	for id, owner := range r.owners {
		out[id] = owner
	}
	return out
}

func (r *Registry) Restore(owners map[string]Owner) error {
	next := make(map[string]Owner, len(owners))
	for conversationID, owner := range owners {
		if strings.TrimSpace(conversationID) == "" || conversationID != owner.ConversationID || owner.ControllerID == "" || owner.Revision == 0 {
			return fmt.Errorf("invalid controller owner snapshot for %q", conversationID)
		}
		if owner.Status != OwnerActive && owner.Status != OwnerSettled {
			return fmt.Errorf("invalid owner status %q", owner.Status)
		}
		next[conversationID] = owner
	}
	r.mu.Lock()
	r.owners = next
	r.mu.Unlock()
	return nil
}
