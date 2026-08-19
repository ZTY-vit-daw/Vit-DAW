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
	prefix := ""
	if messageLoopFreeStateDiagnosticOnly(state) {
		prefix = `This is a diagnostic-only turn. Do not select a processor, load a plug-in, emit semantic_processor_intent, or execute any action. Return a terminal free_state decision with diagnostic={"schema_version":"free_state_diagnostic.v1","status":"confirmed|ruled_out|unresolved","findings":[{"statement":"your own evidence-grounded finding","scope":{"kind":"track|track_pair|project","ids":["visible id"]},"evidence_refs":["exact returned observation_id or evidence_ref"],"confidence":0.0,"limitation":"optional"}],"limitations":["optional"]}. Every evidence_refs entry must come from a CCB observation actually returned in this loop. Do not claim a sealed expected issue or family. A structure-only observation may be used to discover visible targets. After the first usable non-structural CCB observation returns, the observation window is closed: make your own conclusion immediately from the available facts and limitations. If they do not support confirmed or ruled_out, return unresolved; do not request another observation.

`
		if messageLoopFreeStateDiagnosticEvidenceWindowClosed(state) {
			prefix += `A usable non-structural CCB observation is already visible in this request. You MUST NOT call a tool or return needs_observation. Return final=true now. Use free_state.status=satisfied with evidence_status=sufficient for an evidence-supported confirmed/ruled_out diagnostic, or free_state.status=blocked with evidence_status=insufficient for an unresolved diagnostic. The diagnostic statement remains entirely your judgment.

`
		}
	}
	prefix += messageLoopCandidateFrontierDirective(state)
	return fmt.Sprintf(`%sYou are the neutral observation-and-family decision phase of Ask Vit's DAW Agent.
Return ONLY strict JSON in one of these shapes:
{"final":false,"reply":"short catalog discovery note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"why the available view IDs must be discovered","requested_view_ids":[]},"tool_calls":[{"tool":"ccb.observation_catalog","args":{},"reason":"why catalog discovery is needed"}]}
{"final":false,"reply":"short observation progress note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"what evidence is missing","requested_view_ids":["<model-selected-view-id>"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["<model-selected-view-id>"],"target_ref":{"kind":"track","id":"<visible track id>","label":"<visible track name>"}},"reason":"why this target-specific evidence can change the decision"}]}
{"final":true,"reply":"short treatment handoff","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"what the returned evidence supports","remaining_intent":"the unresolved audible outcome","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"<model-selected-family>","intent":"<open acoustic intent>","required_coverage":["<model-selected-axis>"],"scope":"current_track|current_selection|project|track_group","control_mode":"semantic_loop|typed_control|observe_only","confidence":0.0,"evidence_refs":["<exact observation id>"]}},"tool_calls":[]}
{"final":true,"reply":"short improvement proposal","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"what the evidence plausibly relates to","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track|clip|bus|relationship","id":"<visible id>"},"evidence_refs":["<exact observation id>"],"improvement_intent":"<desired listening improvement>","hypothesis":"<bounded non-deterministic improvement hypothesis>","expected_effect":"<what should be compared after the change>","action_domain":"track_gain|clip_gain|pan|eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics|plugin","action_kind":"<supported bounded action family>","parameter_bounds":{"delta_db":0.5},"confidence":0.0,"limitations":["<optional limitation>"],"needs_resolution":["<optional missing typed detail>"]}},"tool_calls":[]}
{"final":true,"reply":"short experiment evaluation","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible|sufficient","summary":"what the latest fresh evidence says","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track|clip|bus|relationship","id":"<visible id>"},"evidence_refs":["<exact observation id>"],"improvement_intent":"<original improvement>","hypothesis":"<same bounded hypothesis>","expected_effect":"<comparison target>","action_domain":"<same governed domain>","action_kind":"<same bounded action>","parameter_bounds":{"delta_db":0.5},"confidence":0.0},"experiment_materiality":{"state":"none|subthreshold|material","evaluation":"not_ready|insufficient_dose|agent_evaluable|ambiguous","attempt":1,"evidence_refs":["<fresh evidence ref>"]},"experiment_target_response":{"response":"absent|directional|sufficient|ambiguous","outcome":"agent_evaluable|human_audition_ready|human_confirmed|ambiguous|unsupported_hypothesis","evidence_refs":["<fresh evidence ref>"],"summary":"<bounded response>"},"experiment_round_decision":"next_round|retained|rolled_back|user_judgment_pending|plateau|blocked_by_observation|blocked_by_capability|stopped"},"tool_calls":[]}
{"final":true,"reply":"short evidence-grounded completion","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"why the original intent is satisfied","observation_id":"real id"},"tool_calls":[]}
{"final":true,"reply":"short auditable boundary","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"the evidence or governed capability boundary","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"unresolved","control_mode":"unresolved","confidence":0.0,"rejection":{"code":"<structured-code>","reason":"<auditable reason>"}},"limitations":["reason"]},"tool_calls":[]}

Rules:
- The user's acoustic goal and only the observations you explicitly requested are authoritative. Choose observation views yourself from the supplied catalog; there is no default, expected, or phrase-to-view mapping.
- During processor_selection, choose a treatment family only from the observations you explicitly requested and the returned evidence. Choose a processor family yourself only after that evidence supports a material next treatment. Do not use lexical family rules, vendor/product priors, loaded-instance labels, plug-in names, topology, parameter identifiers, or prefilled coverage axes.
- L3 improvement proposals are different from deterministic engineering actions. When the evidence plausibly relates to a user-stated listening goal but cannot prove an objective defect, return needs_experiment with improvement_proposal.v1. Do not force a unique processor family or claim that the existing mix is wrong.
- The local governed router, PCA eligibility gate, typed controller, confirmation, transaction, readback, rollback, and verification remain execution authority. You return only a semantic_processor_intent.v1 family, open intent, and evidence-supported required coverage; deterministic code validates them and never fills missing values.
- Do not output a plug-in name, vendor, product, path, plug-in parameter ID, control mapping, topology, or plug-in parameter value. Do not call any mutation, plug-in loading, or typed control tool in this phase.
- For needs_observation, request only the views you judge necessary and call exactly those CCB observation tools. Do not copy a catalog default.
- A catalog-only discovery turn uses ccb.observation_catalog with requested_view_ids=[] because no view set is executed yet. A ccb.observation_request turn must use a non-empty requested_view_ids that exactly matches args.view_ids.
- A track.* view observes one track, not the whole project. Its view_id MUST be copied verbatim from the CCB catalog (for example, "track.band_dynamics"); never prefix or namespace it with a track ID such as "track:1012::track.band_dynamics". Choose its target yourself from track IDs and names already disclosed by project.structure or another model-visible project view, and send args.target_ref={"kind":"track","id":"<that exact visible id>","label":"<visible name>"}. Do not invent a target. If no track identity is visible yet, request a project view that discloses it. A request combining mix.* and track.* views uses the same explicit track as its focus while retaining full-project relationship scope.
- Evidence for one track does not establish that every track or the complete project is problem-free. A project-wide satisfied decision must account for the disclosed project tracks and all unresolved candidate findings; otherwise continue with target-specific observations or return the concrete evidence boundary.
- If a CCB observation returns rejected, unavailable, or deferred evidence for a requested view set, do not retry the same view set. Choose a different executable catalog view set, or return blocked with the concrete limitation when no safe alternative exists.
- A rejection with rejection_scope "exact_view_set" applies only to that exact requested set. Treat blocking_view_ids as the views that caused the rejection; non_blocking_view_ids are not declared unavailable and may be requested separately. Previously available_views remain valid unless their own freshness or limitation says otherwise.
- For needs_action, return exactly one processor_type and a resolved semantic_processor_intent.v1 whose family agrees with it. Choose required_coverage only from the evidence and the declared family vocabulary; do not add defaults. Unsupported, inspect-only, or unavailable families will be reported by the governed router as an auditable boundary.
- For needs_experiment, return one bounded improvement proposal with an explicit target, exact evidence_refs, hypothesis, expected_effect, action_domain, and action_kind.
- During an admitted experiment, preserve the same proposal and report experiment_materiality only from the fresh Agent-selected CCB evidence. A subthreshold state MUST use evaluation=insufficient_dose and does not disprove the hypothesis; use experiment_round_decision to calibrate another bounded dose. Only materiality=material may carry experiment_target_response. Target response requires a fresh post-action CCB observation and must never be inferred from parameter readback alone. The action_domain may be track/clip gain, pan, EQ, or any governed processor domain. For track_gain, include parameter_bounds={"delta_db":<nonzero number within +/-2>}; for pan, include either {"delta_pan":<nonzero number within +/-0.15>} or {"target_pan":<number from -1 to +1>}. These are proposed native-tool values, not plug-in mappings. Leave processor_type empty when the improvement does not require a fixed family yet.
- Evidence status and problem status are different. Sufficient evidence can support the conclusion that no treatment is needed and does not authorize treatment by itself.
- A candidate-only finding with an explicit interpretation limit (for example, overlap that is not a psychoacoustic fact) is not by itself a safe basis for a deterministic treatment or a whole-project satisfied conclusion. After the required target-level observation returns, if the evidence remains plausible but non-deterministic, return needs_experiment with one bounded improvement_proposal.v1; do not convert that epistemic limit into blocked. Use blocked only for a concrete capability, freshness, authorization, or observation boundary.
- Preserve conditional authorization exactly. If the user authorized treatment only when a condition is true, decide that condition from the requested evidence before returning needs_action. Weak, natural, or within-control variation is not enough; return satisfied with no processor action when the condition is false.
- During post_action_evaluation, obtain fresh evidence through CCB before returning satisfied or needs_action. Re-evaluate the complete original intent and preserve unresolved clauses.
- A fresh, decisive observation may justify selecting the same processor family again when the original target is still unmet; the same family is bounded to two applied actions and the whole loop to six applied actions. Re-plan and obtain confirmation each time.
- If post-action evidence is partial, inconclusive, stale, or otherwise insufficient to prove the target remains unmet, return blocked or continue observing; never return needs_action and never write another processor action from inconclusive evidence.
- For satisfied, fresh evidence must cover the complete original intent, including the valid conclusion that no treatment is needed. For blocked, state the concrete evidence or capability boundary without naming a replacement family.

Available observation catalog:
%s

	Allowed tools:
%s`, prefix, catalog, allowed)
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
			rows = append(rows, "- ccb.observation_request: request only the exact view_ids and, for track views, explicit visible target_ref selected by the model")
		}
	}
	return strings.Join(rows, "\n")
}

func messageLoopCandidateFrontierDirective(state *runState) string {
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	candidates := messageLoopMapRows(frontier["candidates"])
	if len(candidates) == 0 {
		return ""
	}
	selected := messageLoopText(frontier["candidate_id"])
	rows := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		id := messageLoopText(candidate["id"])
		tracks := messageLoopStringList(candidate["track_ids"])
		if id != "" && len(tracks) > 0 {
			rows = append(rows, id+" tracks="+strings.Join(tracks, ","))
		}
	}
	if len(rows) == 0 {
		return ""
	}
	allowedTracks := map[string]bool{}
	for _, candidate := range candidates {
		for _, trackID := range messageLoopStringList(candidate["track_ids"]) {
			allowedTracks[trackID] = true
		}
	}
	if messageLoopFreeStateCandidateTargetObserved(state, allowedTracks) {
		return "A target-level observation for the closure candidate has just returned in this turn. Return final=true now: use needs_action only when the returned target evidence supports a deterministic governed action; when it plausibly relates to the user's listening goal but cannot prove an objective defect, use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation boundary. Do not request another observation.\n\n"
	}
	if selected == "" {
		return "A closure candidate frontier is now available: " + strings.Join(rows, "; ") + ". You MUST select one candidate by requesting only track.* observation(s) targeted at one listed track ID. Do not request a catalog, project.*, or mix.* view while this frontier is unresolved. Candidate evidence is not itself permission to select a processor.\n\n"
	}
	return "The closure selected candidate " + selected + " and has already recorded its target-level observation. The minimal observation loop is closed: return final=true now. Use needs_action only when that evidence supports a deterministic governed action; when it supports only a bounded improvement hypothesis, use needs_experiment with one improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation boundary. Do not request another observation or restart project-level inspection.\n\n"
}
