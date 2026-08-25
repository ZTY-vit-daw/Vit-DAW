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
			prefix += `A usable non-structural CCB observation is already visible in this request. You MUST NOT call a tool or return needs_observation. Return final=true now. Use free_state.status=diagnostic_complete with evidence_status=sufficient for an evidence-supported confirmed diagnostic, no_candidate_found with a ruled_out diagnostic when the bounded search found no credible candidate, or capability_blocked with evidence_status=insufficient for an unresolved capability boundary. The diagnostic statement remains entirely your judgment.

`
		}
	}
	if messageLoopFreeStateCatalogAlreadyObserved(state) {
		prefix += `A successful ccb.observation_catalog result is already present in this loop. Do not call ccb.observation_catalog again. Use one of the returned exact view_id values with ccb.observation_request, or return a concrete capability/evidence boundary.

`
	}
	if strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
		prefix += `This is an open improvement contract. A local diagnosis or evidence-sufficient dimension does not complete the user's task. You MUST NOT return satisfied. After observation, return needs_experiment with one bounded evidence-backed improvement_proposal, no_candidate_found with a diagnostic covering the bounded search and its limitations, or capability_blocked with a concrete runtime boundary. The product runtime alone settles the task after the experiment contract.

`
	}
	prefix += messageLoopCandidateFrontierDirective(state)
	return fmt.Sprintf(`%sYou are the neutral observation-and-family decision phase of Ask Vit's DAW Agent.
Return ONLY strict JSON in one of these shapes:
{"final":false,"reply":"short catalog discovery note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"why the available view IDs must be discovered","requested_view_ids":[]},"tool_calls":[{"tool":"ccb.observation_catalog","args":{},"reason":"why catalog discovery is needed"}]}
{"final":false,"reply":"short observation progress note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"what evidence is missing","requested_view_ids":["<model-selected-view-id>"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["<model-selected-view-id>"],"target_ref":{"kind":"track","id":"<visible track id>","label":"<visible track name>"}},"reason":"why this target-specific evidence can change the decision"}]}
{"final":true,"reply":"short treatment handoff","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"what the returned evidence supports","remaining_intent":"the unresolved audible outcome","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"<model-selected-family>","intent":"<open acoustic intent>","required_coverage":["<model-selected-axis>"],"scope":"current_track|current_selection|project|track_group","control_mode":"semantic_loop|typed_control|observe_only","confidence":0.0,"evidence_refs":["<exact observation id>"]}},"tool_calls":[]}
{"final":true,"reply":"short improvement proposal","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"what the evidence plausibly relates to","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"<visible track id>"},"evidence_refs":["<exact observation id>"],"improvement_intent":"<desired listening improvement>","hypothesis":"<bounded non-deterministic improvement hypothesis>","expected_effect":"<what should be compared after the change>","action_domain":"track_gain","action_kind":"track_gain_adjust","parameter_bounds":{"delta_db":0.5},"verification_plan":{"experiment_budget":1,"max_action_attempts":1},"confidence":0.0,"limitations":["<optional limitation>"],"needs_resolution":["<optional missing typed detail>"]}},"tool_calls":[]}
{"final":true,"reply":"short experiment evaluation","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible|sufficient","summary":"what the latest fresh evidence says","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track|clip|bus|relationship","id":"<visible id>"},"evidence_refs":["<exact observation id>"],"improvement_intent":"<original improvement>","hypothesis":"<same bounded hypothesis>","expected_effect":"<comparison target>","action_domain":"<same governed domain>","action_kind":"<same bounded action>","parameter_bounds":{"delta_db":0.5},"confidence":0.0},"experiment_materiality":{"state":"none|subthreshold|material","evaluation":"not_ready|insufficient_dose|agent_evaluable|ambiguous","attempt":1,"evidence_refs":["<fresh evidence ref>"]},"experiment_target_response":{"response":"absent|directional|sufficient|ambiguous","outcome":"agent_evaluable|human_audition_ready|human_confirmed|ambiguous|unsupported_hypothesis","evidence_refs":["<fresh evidence ref>"],"summary":"<bounded response>"},"experiment_round_decision":"next_round|retained|rolled_back|user_judgment_pending|plateau|blocked_by_observation|blocked_by_capability|stopped"},"tool_calls":[]}
{"final":true,"reply":"short evidence-grounded diagnosis","free_state":{"schema_version":"free_state_decision.v1","status":"diagnostic_complete","evidence_status":"sufficient","summary":"the bounded diagnosis","observation_id":"real id","diagnostic":{"schema_version":"free_state_diagnostic.v1","status":"confirmed","findings":[{"statement":"bounded finding","evidence_refs":["real id"],"confidence":0.0}]}},"tool_calls":[]}
{"final":true,"reply":"no credible candidate in the bounded search","free_state":{"schema_version":"free_state_decision.v1","status":"no_candidate_found","evidence_status":"sufficient","summary":"what was checked and what remains outside the evidence boundary","diagnostic":{"schema_version":"free_state_diagnostic.v1","status":"ruled_out","findings":[{"statement":"no candidate was supported in the checked scope","evidence_refs":["real id"],"confidence":0.0,"limitation":"unchecked boundary"}],"limitations":["unchecked boundary"]}},"tool_calls":[]}
{"final":true,"reply":"short auditable boundary","free_state":{"schema_version":"free_state_decision.v1","status":"capability_blocked","evidence_status":"insufficient","summary":"the concrete governed capability boundary","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"unresolved","control_mode":"unresolved","confidence":0.0,"rejection":{"code":"<structured-code>","reason":"<auditable reason>"}},"limitations":["reason"]},"tool_calls":[]}

Rules:
- The user's acoustic goal and only the observations you explicitly requested are authoritative. Choose observation views yourself from the supplied catalog; there is no default, expected, or phrase-to-view mapping.
- During processor_selection, choose a treatment family only from the observations you explicitly requested and the returned evidence. Choose a processor family yourself only after that evidence supports a material next treatment. Do not use lexical family rules, vendor/product priors, loaded-instance labels, plug-in names, topology, parameter identifiers, or prefilled coverage axes.
- L3 improvement proposals are different from deterministic engineering actions. When the evidence plausibly relates to a user-stated listening goal but cannot prove an objective defect, return needs_experiment with improvement_proposal.v1. Do not force a unique processor family or claim that the existing mix is wrong.
- The local governed router, PCA eligibility gate, typed controller, confirmation, transaction, readback, rollback, and verification remain execution authority. You return only a semantic_processor_intent.v1 family, open intent, and evidence-supported required coverage; deterministic code validates them and never fills missing values.
- Do not output a plug-in name, vendor, product, path, plug-in parameter ID, control mapping, topology, or plug-in parameter value. Do not call any mutation, plug-in loading, or typed control tool in this phase.
- For needs_observation, request only the views you judge necessary and call exactly those CCB observation tools. Do not copy a catalog default.
- When revisiting the same target/view set, include priority_reason="contradiction" and a genuinely new unresolved_questions entry, or set declared_contradiction=true only when the prior receipt is stale/partial. A changed project revision is admitted automatically. These fields explain evidence freshness; they never override a rejected exact view-set.
- A catalog-only discovery turn uses ccb.observation_catalog with requested_view_ids=[] because no view set is executed yet. A ccb.observation_request turn must use a non-empty requested_view_ids that exactly matches args.view_ids.
- A track.* view observes one track, not the whole project. Its view_id MUST be copied verbatim from the CCB catalog (for example, "track.band_dynamics"); never prefix or namespace it with a track ID such as "track:1012::track.band_dynamics". Choose its target yourself from track IDs and names already disclosed by project.structure or another model-visible project view, and send args.target_ref={"kind":"track","id":"<that exact visible id>","label":"<visible name>"}. Do not invent a target. If no track identity is visible yet, request a project view that discloses it. A request combining mix.* and track.* views uses the same explicit track as its focus while retaining full-project relationship scope.
- Evidence for one track does not establish that every track or the complete project is problem-free. A project-wide diagnostic_complete or no_candidate_found decision must account for the disclosed project tracks and all unresolved candidate findings; otherwise continue with target-specific observations or return the concrete evidence boundary.
- If a CCB observation returns rejected, unavailable, or deferred evidence for a requested view set, do not retry the same view set. Choose a different executable catalog view set, or return blocked with the concrete limitation when no safe alternative exists.
- A rejection with rejection_scope "exact_view_set" applies only to that exact requested set. Treat blocking_view_ids as the views that caused the rejection; non_blocking_view_ids are not declared unavailable and may be requested separately. Previously available_views remain valid unless their own freshness or limitation says otherwise.
- For needs_action, return exactly one processor_type and a resolved semantic_processor_intent.v1 whose family agrees with it. Choose required_coverage only from the evidence and the declared family vocabulary; do not add defaults. Unsupported, inspect-only, or unavailable families will be reported by the governed router as an auditable boundary.
- For needs_experiment in D1-S1, return exactly one track target and action_domain=track_gain, action_kind=track_gain_adjust, parameter_bounds={"delta_db":<nonzero number within +/-2>}, and verification_plan with experiment_budget=1 and max_action_attempts=1. Other domains or action kinds are unsupported in this phase; return capability_blocked instead of inventing a fallback.
- During an admitted D1-S1 experiment, preserve the same proposal and report experiment_materiality only from the fresh Agent-selected post-action CCB evidence. A subthreshold state MUST use evaluation=insufficient_dose and does not disprove the hypothesis, but it MUST NOT request next_round or another mutation; it may only proceed to an ambiguous human A/B evaluation. A material result may carry a separate experiment_target_response. Target response must never be inferred from parameter readback alone. Leave processor_type empty for this native track-gain experiment.
- Evidence status and problem status are different. Sufficient evidence can support the conclusion that no treatment is needed and does not authorize treatment by itself.
- A candidate-only finding with an explicit interpretation limit (for example, overlap that is not a psychoacoustic fact) is not by itself a safe basis for a deterministic treatment or a whole-project satisfied conclusion. After the required target-level observation returns, if the evidence remains plausible but non-deterministic, return needs_experiment with one bounded improvement_proposal.v1; do not convert that epistemic limit into blocked. Use blocked only for a concrete capability, freshness, authorization, or observation boundary.
- Preserve conditional authorization exactly. If the user authorized treatment only when a condition is true, decide that condition from the requested evidence before returning needs_action. Weak, natural, or within-control variation is not enough; return no_candidate_found with the bounded evidence and limitations when the condition is false.
- During post_action_evaluation, obtain fresh evidence through CCB before returning a runtime outcome or needs_action. Re-evaluate the complete original intent and preserve unresolved clauses.
- D1-S1 permits one experiment round and one forward mutation. Never return next_round, continue_once, or a second treatment after the action has been applied; only retain, rollback, ambiguous human judgment, or a concrete blocked boundary may follow.
- If post-action evidence is partial, inconclusive, stale, or otherwise insufficient to prove the target remains unmet, return blocked or continue observing; never return needs_action and never write another processor action from inconclusive evidence.
- For diagnostic_complete or no_candidate_found, fresh evidence must cover the declared bounded scope. For capability_blocked, state the concrete runtime boundary without naming a replacement family.

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
		if messageLoopFreeStateCandidateTargetEvidencePartial(state, allowedTracks) {
			if messageLoopFreeStatePartialTargetEvidenceAlreadyAvailable(state, allowedTracks) {
				return "The selected closure candidate already has bounded target-level evidence, but its quality is partial. This is sufficient for an open improvement hypothesis even though it does not prove an objective defect. You MUST return final=true with free_state.status=needs_experiment and exactly one bounded improvement_proposal.v1 citing the exact target observation_id or evidence_refs. Do not request another observation, do not repeat any previously observed view, and do not return no_candidate_found, diagnostic_complete, satisfied, or capability_blocked merely because the evidence is partial.\n\n"
			}
			return "The selected closure candidate has returned only partial target evidence. This does not rule out the candidate and does not complete the open improvement task. Do not return no_candidate_found, diagnostic_complete, or satisfied. Request a different cataloged target-level track.* view or another unresolved candidate track; if the evidence supports only a bounded hypothesis, return needs_experiment with one improvement_proposal.v1 citing the returned observation.\n\n"
		}
		return "A target-level observation for the closure candidate has just returned in this turn. Return final=true now: use needs_action only when the returned target evidence supports a deterministic governed action; when it plausibly relates to the user's listening goal but cannot prove an objective defect, use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation boundary. Do not request another observation.\n\n"
	}
	if selected == "" {
		return "A closure candidate frontier is now available: " + strings.Join(rows, "; ") + ". You MUST select one candidate by requesting only track.* observation(s) targeted at one listed track ID. Do not request a catalog, project.*, or mix.* view while this frontier is unresolved. Candidate evidence is not itself permission to select a processor.\n\n"
	}
	return "The closure selected candidate " + selected + " and has already recorded its target-level observation. The minimal observation loop is closed: return final=true now. Use needs_action only when that evidence supports a deterministic governed action; when it supports only a bounded improvement hypothesis, use needs_experiment with one improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation boundary. Do not request another observation or restart project-level inspection.\n\n"
}
