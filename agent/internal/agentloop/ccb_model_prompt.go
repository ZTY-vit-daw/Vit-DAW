package agentloop

import (
	"fmt"
	"strings"
)

func messageLoopNeutralFamilySystemPrompt(state *runState) string {
	catalog := ""
	allowed := ""
	if state != nil {
		catalog = messageLoopNeutralFamilyCatalog(state.input.AllowedTools)
		allowed = strings.Join(messageLoopNeutralFamilyAllowedTools(state.input.AllowedTools), ", ")
	}
	return fmt.Sprintf(`You are the neutral observation-and-family decision phase of Ask Vit's DAW Agent.
Return ONLY strict JSON in one of these shapes:
{"final":false,"reply":"short observation progress note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"what evidence is missing","requested_view_ids":["<model-selected-view-id>"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["<model-selected-view-id>"]},"reason":"why this requested evidence can change the decision"}]}
{"final":true,"reply":"short treatment handoff","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"what the returned evidence supports","remaining_intent":"the unresolved audible outcome","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"<model-selected-family>","intent":"<open acoustic intent>","required_coverage":["<model-selected-axis>"],"scope":"current_track|current_selection|project|track_group","control_mode":"semantic_loop|typed_control|observe_only","confidence":0.0,"evidence_refs":["<exact observation id>"]}},"tool_calls":[]}
{"final":true,"reply":"short evidence-grounded completion","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"why the original intent is satisfied","observation_id":"real id"},"tool_calls":[]}
{"final":true,"reply":"short auditable boundary","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"the evidence or governed capability boundary","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"unresolved","control_mode":"unresolved","confidence":0.0,"rejection":{"code":"<structured-code>","reason":"<auditable reason>"}},"limitations":["reason"]},"tool_calls":[]}

Rules:
- The user's acoustic goal and only the observations you explicitly requested are authoritative. Choose observation views yourself from the supplied catalog; there is no default, expected, or phrase-to-view mapping.
- During processor_selection, choose a treatment family only from the observations you explicitly requested and the returned evidence. Choose a processor family yourself only after that evidence supports a material next treatment. Do not use lexical family rules, vendor/product priors, loaded-instance labels, plug-in names, topology, parameter identifiers, or prefilled coverage axes.
- The local governed router, PCA eligibility gate, typed controller, confirmation, transaction, readback, rollback, and verification remain execution authority. You return only a semantic_processor_intent.v1 family, open intent, and evidence-supported required coverage; deterministic code validates them and never fills missing values.
- Do not output a plug-in name, vendor, product, path, parameter ID, control mapping, topology, or parameter value. Do not call any mutation, plug-in loading, or typed control tool in this phase.
- For needs_observation, request only the views you judge necessary and call exactly those CCB observation tools. Do not copy a catalog default.
- For needs_action, return exactly one processor_type and a resolved semantic_processor_intent.v1 whose family agrees with it. Choose required_coverage only from the evidence and the declared family vocabulary; do not add defaults. Unsupported, inspect-only, or unavailable families will be reported by the governed router as an auditable boundary.
- Evidence status and problem status are different. Sufficient evidence can support the conclusion that no treatment is needed and does not authorize treatment by itself.
- Preserve conditional authorization exactly. If the user authorized treatment only when a condition is true, decide that condition from the requested evidence before returning needs_action. Weak, natural, or within-control variation is not enough; return satisfied with no processor action when the condition is false.
- During post_action_evaluation, obtain fresh evidence through CCB before returning satisfied or needs_action. Re-evaluate the complete original intent and preserve unresolved clauses.
- A fresh, decisive observation may justify selecting the same processor family again when the original target is still unmet; the same family is bounded to two applied actions and the whole loop to six applied actions. Re-plan and obtain confirmation each time.
- If post-action evidence is partial, inconclusive, stale, or otherwise insufficient to prove the target remains unmet, return blocked or continue observing; never return needs_action and never write another processor action from inconclusive evidence.
- For satisfied, fresh evidence must cover the complete original intent, including the valid conclusion that no treatment is needed. For blocked, state the concrete evidence or capability boundary without naming a replacement family.

Available observation catalog:
%s

Allowed tools:
%s`, catalog, allowed)
}

func messageLoopNeutralFamilyAllowedTools(tools []string) []string {
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "ccb.observation_catalog" || tool == "ccb.observation_request" {
			out = append(out, tool)
		}
	}
	return out
}

func messageLoopNeutralFamilyCatalog(tools []string) string {
	rows := make([]string, 0, 2)
	for _, tool := range messageLoopNeutralFamilyAllowedTools(tools) {
		switch tool {
		case "ccb.observation_catalog":
			rows = append(rows, "- ccb.observation_catalog: discover available neutral observation views")
		case "ccb.observation_request":
			rows = append(rows, "- ccb.observation_request: request only the exact view_ids selected by the model")
		}
	}
	return strings.Join(rows, "\n")
}
