package capabilityinteraction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
)

var ErrRevisionNeedsClarification = errors.New("proposal revision needs clarification")

func RequiresReplan(decision orchestration.ApprovalDecision) bool {
	for _, adjustment := range decision.Adjustments {
		if adjustment.Kind == "select_candidate" {
			return true
		}
	}
	return false
}

// ReviseFrozenPlan creates a new immutable Proposal revision from supported,
// bounded conversational adjustments. It never writes the project.
func ReviseFrozenPlan(session orchestration.PlanningSession, decision orchestration.ApprovalDecision) (orchestration.FrozenPlan, error) {
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return orchestration.FrozenPlan{}, errors.New("active frozen proposal is required")
	}
	if decision.Kind != orchestration.ApprovalRevise && decision.Kind != orchestration.ApprovalNarrow {
		return orchestration.FrozenPlan{}, errors.New("revision decision is required")
	}
	if len(decision.Unresolved) > 0 || len(decision.Adjustments) == 0 || RequiresReplan(decision) {
		return orchestration.FrozenPlan{}, ErrRevisionNeedsClarification
	}
	plan := cloneFrozenPlan(*session.FrozenPlan)
	presentation := clonePresentation(plan.Proposal.Presentation)
	if presentation == nil {
		return orchestration.FrozenPlan{}, errors.New("proposal presentation is required for safe revision")
	}
	actions := append([]orchestration.Action(nil), plan.ActionSet.Actions...)
	previews := append([]orchestration.ProposalActionPreview(nil), presentation.Actions...)
	for _, adjustment := range decision.Adjustments {
		var err error
		actions, previews, err = applyAdjustment(actions, previews, adjustment)
		if err != nil {
			return orchestration.FrozenPlan{}, err
		}
	}
	if len(actions) == 0 {
		return orchestration.FrozenPlan{}, errors.New("revision removed every proposal action")
	}
	if len(actions) != len(previews) {
		return orchestration.FrozenPlan{}, errors.New("revised action and presentation coverage diverged")
	}
	nextRevision := session.ActiveProposal.Revision + 1
	for index := range actions {
		actions[index].ID = revisedActionID(actions[index].ID, nextRevision)
		previews[index].ActionID = actions[index].ID
	}
	plan.ActionSet.Actions = actions
	plan.ActionSet.ID = fmt.Sprintf("%s.r%d", strings.TrimSpace(plan.ActionSet.ID), nextRevision)
	plan.ActionSet.Hash = plan.ActionSet.ComputeHash()
	plan.Proposal.Revision = nextRevision
	plan.Proposal.ActionSetHash = plan.ActionSet.Hash
	plan.Proposal.TargetScope = actionScope(actions)
	plan.Proposal.ID = revisedProposalID(plan.Proposal.CapabilityID, nextRevision, plan.ActionSet.Hash)
	plan.Proposal.Summary = fmt.Sprintf("%s（对话修订 r%d，%d 个动作）", firstNonEmpty(plan.Proposal.Summary, presentation.Title), nextRevision, len(actions))
	plan.Proposal.CreatedAt = time.Now().UTC()
	presentation.ProposalID = plan.Proposal.ID
	presentation.ProposalRevision = nextRevision
	presentation.ActionCount = len(actions)
	presentation.Actions = previews
	presentation.ChangeGroups = summarizePreviewGroups(previews)
	presentation.Recommendation = fmt.Sprintf("已按你的自然语言要求生成 revision %d；旧 Proposal 不再可授权。", nextRevision)
	presentation.ApprovalPrompt = "请检查更新后的修改预览；可以直接回复“执行这个方案”、继续调整、提问或取消。"
	plan.Proposal.Presentation = presentation
	plan.FrozenAt = time.Now().UTC()
	return plan, nil
}

func applyAdjustment(actions []orchestration.Action, previews []orchestration.ProposalActionPreview, adjustment orchestration.ProposalAdjustment) ([]orchestration.Action, []orchestration.ProposalActionPreview, error) {
	switch adjustment.Kind {
	case "exclude_targets", "include_only":
		selected := make(map[string]bool, len(adjustment.TargetRefs))
		for _, target := range adjustment.TargetRefs {
			selected[strings.TrimSpace(target)] = true
		}
		if len(selected) == 0 {
			return nil, nil, fmt.Errorf("%w: no proposal targets matched %q", ErrRevisionNeedsClarification, adjustment.TargetQuery)
		}
		outActions := []orchestration.Action{}
		outPreviews := []orchestration.ProposalActionPreview{}
		for index, action := range actions {
			include := selected[action.TargetRef]
			if adjustment.Kind == "exclude_targets" {
				include = !include
			}
			if include {
				outActions = append(outActions, action)
				outPreviews = append(outPreviews, previews[index])
			}
		}
		return outActions, outPreviews, nil
	case "max_abs_delta":
		if adjustment.Value <= 0 {
			return nil, nil, fmt.Errorf("%w: maximum change must be positive", ErrRevisionNeedsClarification)
		}
		for index := range actions {
			if !adjustmentTargets(adjustment, actions[index].TargetRef) {
				continue
			}
			applyBoundedDelta(&actions[index], &previews[index], math.Copysign(math.Min(math.Abs(previews[index].Delta), adjustment.Value), previews[index].Delta))
		}
		return actions, previews, nil
	case "replace_delta":
		matched := 0
		for index := range actions {
			if !adjustmentTargets(adjustment, actions[index].TargetRef) || math.Abs(previews[index].Delta-adjustment.OriginalValue) > .011 {
				continue
			}
			applyBoundedDelta(&actions[index], &previews[index], adjustment.Value)
			matched++
		}
		if matched == 0 {
			return nil, nil, fmt.Errorf("%w: no action used the requested original value", ErrRevisionNeedsClarification)
		}
		return actions, previews, nil
	case "set_delta":
		if len(adjustment.TargetRefs) == 0 {
			return nil, nil, fmt.Errorf("%w: set_delta requires an explicit target or group", ErrRevisionNeedsClarification)
		}
		for index := range actions {
			if adjustmentTargets(adjustment, actions[index].TargetRef) {
				applyBoundedDelta(&actions[index], &previews[index], adjustment.Value)
			}
		}
		return actions, previews, nil
	default:
		return nil, nil, fmt.Errorf("%w: unsupported adjustment %s", ErrRevisionNeedsClarification, adjustment.Kind)
	}
}

func applyBoundedDelta(action *orchestration.Action, preview *orchestration.ProposalActionPreview, delta float64) {
	if action == nil || preview == nil {
		return
	}
	preview.Delta = delta
	preview.Target = preview.Before + delta
	if action.Args == nil {
		action.Args = map[string]any{}
	}
	if strings.EqualFold(preview.Unit, "dB") {
		action.Args["delta_db"] = delta
		action.Args["target_db"] = preview.Target
	} else {
		preview.Target = math.Max(-1, math.Min(1, preview.Target))
		preview.Delta = preview.Target - preview.Before
		action.Args["delta_pan"] = preview.Delta
		action.Args["target_pan"] = preview.Target
	}
}

func adjustmentTargets(adjustment orchestration.ProposalAdjustment, target string) bool {
	if len(adjustment.TargetRefs) == 0 {
		return true
	}
	for _, candidate := range adjustment.TargetRefs {
		if strings.TrimSpace(candidate) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func summarizePreviewGroups(previews []orchestration.ProposalActionPreview) []orchestration.ProposalChangeGroup {
	groups := map[string][]orchestration.ProposalActionPreview{}
	for _, preview := range previews {
		key := firstNonEmpty(preview.Function, preview.Role, "other")
		groups[key] = append(groups[key], preview)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]orchestration.ProposalChangeGroup, 0, len(keys))
	for _, key := range keys {
		rows := groups[key]
		minimum, maximum := rows[0].Delta, rows[0].Delta
		for _, row := range rows[1:] {
			minimum = math.Min(minimum, row.Delta)
			maximum = math.Max(maximum, row.Delta)
		}
		out = append(out, orchestration.ProposalChangeGroup{
			ID: "revised:" + key, Label: strings.ReplaceAll(key, "_", " "), Role: rows[0].Role, Function: rows[0].Function,
			TrackCount: len(rows), MoveCount: len(rows), MinValue: minimum, MaxValue: maximum, Unit: rows[0].Unit,
		})
	}
	return out
}

func actionScope(actions []orchestration.Action) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		if target := strings.TrimSpace(action.TargetRef); target != "" {
			out = append(out, target)
		}
	}
	sort.Strings(out)
	return out
}

func revisedActionID(id string, revision int64) string {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "action"
	}
	return fmt.Sprintf("%s:r%d", id, revision)
}

func revisedProposalID(capabilityID string, revision int64, actionSetHash string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", capabilityID, revision, actionSetHash)))
	return "proposal_" + hex.EncodeToString(sum[:8])
}

func cloneFrozenPlan(in orchestration.FrozenPlan) orchestration.FrozenPlan {
	data, _ := json.Marshal(in)
	out := orchestration.FrozenPlan{}
	_ = json.Unmarshal(data, &out)
	return out
}

func clonePresentation(in *orchestration.ProposalPresentation) *orchestration.ProposalPresentation {
	if in == nil {
		return nil
	}
	data, _ := json.Marshal(in)
	out := &orchestration.ProposalPresentation{}
	_ = json.Unmarshal(data, out)
	return out
}
