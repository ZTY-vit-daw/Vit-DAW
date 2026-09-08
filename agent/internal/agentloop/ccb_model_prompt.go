package agentloop

import (
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
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
		prefix += freeStateImprovementConvergenceGuidance()
		// Add pattern recognition guidance for improvement tasks
		prefix += freeStatePatternRecognitionGuidance()
	}
	if directive := messageLoopFreeStateContinuationBudgetDirective(state); directive != "" {
		prefix += directive
	}
	prefix += messageLoopFreeStateObservationSaturationDirective(state)
	prefix += messageLoopCandidateFrontierDirective(state)
	prefix += messageLoopFreeStatePhaseDeferredResubmissionDirective(state)
	// GLM ruling (D2-2): the single-round prohibitions stay byte-identical on
	// the default tier; only an explicitly admitted multi-round experiment
	// swaps in the next-round calibration wording. continue_once and a second
	// treatment within one round stay forbidden on every tier.
	multiRound := messageLoopFreeStateMultiRoundTier(state)
	experimentBudget := 1
	if multiRound {
		experimentBudget = messageLoopFreeStateEffectiveExperimentBudget(state)
	}
	return fmt.Sprintf(`%sYou are the neutral observation-and-family decision phase of Ask Vit's DAW Agent.
Return ONLY strict JSON in one of these shapes:
{"final":false,"reply":"short catalog discovery note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"why the available view IDs must be discovered","requested_view_ids":[]},"tool_calls":[{"tool":"ccb.observation_catalog","args":{},"reason":"why catalog discovery is needed"}]}
{"final":false,"reply":"short observation progress note","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"what evidence is missing","requested_view_ids":["<model-selected-view-id>"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["<model-selected-view-id>"],"target_ref":{"kind":"track","id":"<visible track id>","label":"<visible track name>"}},"reason":"why this target-specific evidence can change the decision"}]}
{"final":true,"reply":"short treatment handoff","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"what the returned evidence supports","remaining_intent":"the unresolved audible outcome","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"<model-selected-family>","intent":"<open acoustic intent>","required_coverage":["<model-selected-axis>"],"scope":"current_track|current_selection|project|track_group","control_mode":"semantic_loop|typed_control|observe_only","confidence":0.0,"evidence_refs":["<exact observation id>"]}},"tool_calls":[]}
%s
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
- %s
- %s
- Evidence status and problem status are different. Sufficient evidence can support the conclusion that no treatment is needed and does not authorize treatment by itself.
- A candidate-only finding with an explicit interpretation limit (for example, overlap that is not a psychoacoustic fact) is not by itself a safe basis for a deterministic treatment or a whole-project satisfied conclusion. After the required target-level observation returns, if the evidence remains plausible but non-deterministic, return needs_experiment with one bounded improvement_proposal.v1; do not convert that epistemic limit into blocked. Use blocked only for a concrete capability, freshness, authorization, or observation boundary.
- Preserve conditional authorization exactly. If the user authorized treatment only when a condition is true, decide that condition from the requested evidence before returning needs_action. Weak, natural, or within-control variation is not enough; return no_candidate_found with the bounded evidence and limitations when the condition is false.
- Authorization semantics under an open improvement contract: a diagnostic-phrased user goal (asking to check, inspect, or find problems in the mix) still authorizes bounded reversible improvement experiments, unless this turn is explicitly marked diagnostic-only. The needs_experiment user-confirmation gate is where any mutation is authorized; diagnostic wording alone neither selects a treatment nor refuses one. Do not return capability_blocked on authorization without a concrete runtime authorization boundary.
- Every CCB observation bundle carries read_only=true and mutation_authority=false. These fields describe the observation tool itself: CCB observes, never mutates, and neither grants nor denies modification authority. They are contract properties of the observation view, not statements about your task authorization, and must not be used alone as a capability_blocked authorization basis.
- During post_action_evaluation, when the loop context still carries requires_post_action_observation=true your FIRST action must be a ccb.observation_request for the admitted experiment's view set on the applied target; no runtime outcome, needs_action, or settle report is legal until that request has returned in this reasoning cycle. When requires_post_action_observation is false the fresh post-action evidence is already recorded on the round: cite it and settle the experiment from it without requesting the observation again. Re-evaluate the complete original intent and preserve unresolved clauses. The bounded experiment has already been user-confirmed and applied; authorization was settled at that confirmation. Re-litigating whether the original request authorized treatment is not a valid boundary in this phase — evaluate the applied experiment's outcome only from fresh post-action evidence.
- %s
- If post-action evidence is partial, inconclusive, stale, or otherwise insufficient to prove the target remains unmet, continue observing or report the concrete evidence limitation; never return needs_action and never write another processor action from inconclusive evidence. Return blocked only when the fresh post-action CCB observation itself cannot be obtained; blocked before that observation is not a legal terminal.
- For diagnostic_complete or no_candidate_found, fresh evidence must cover the declared bounded scope. For capability_blocked, state the concrete runtime boundary without naming a replacement family.

Available observation catalog:
%s

	Allowed tools:
%s`, prefix, freeStateImprovementProposalPromptShape(experimentBudget), freeStateD1AdmittedDomainRuleForBudget(experimentBudget), freeStateSubthresholdExperimentPromptRule(multiRound), freeStateRoundMutationLimitPromptRule(multiRound), catalog, allowed)
}

// freeStateImprovementProposalPromptShape renders the needs_experiment
// proposal example for the effective experiment budget. The budget is
// runtime-controlled (server-side tier injection): the model's
// verification_plan echoes it but never upgrades it.
func freeStateImprovementProposalPromptShape(experimentBudget int) string {
	return fmt.Sprintf(`{"final":true,"reply":"short improvement proposal","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"what the evidence plausibly relates to","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"<visible track id>"},"evidence_refs":["<exact observation id>"],"improvement_intent":"<desired listening improvement>","hypothesis":"<bounded non-deterministic improvement hypothesis>","expected_effect":"<what should be compared after the change>","action_domain":"track_gain|static_eq|broadband_compression","action_kind":"track_gain_adjust|static_eq_band_adjust|broadband_threshold_adjust","parameter_bounds":{"delta_db":0.5}|{"gain_db":-0.5,"frequency_hz":400,"q":1.0,"band_index":0}|{"threshold_db":-1.0},"verification_plan":{"experiment_budget":%d,"max_action_attempts":1},"confidence":0.0,"limitations":["<optional limitation>"],"needs_resolution":["<optional missing typed detail>"]}},"tool_calls":[]}`, experimentBudget)
}

// freeStateSubthresholdExperimentPromptRule is the post-action materiality
// rule. The single-round wording is byte-identical to the historical prompt;
// the multi-round tier additionally allows the runtime-opened next calibration
// round after insufficient_dose (GLM ruling 1: insufficient dose is not
// ambiguity and never disproves the hypothesis).
func freeStateSubthresholdExperimentPromptRule(multiRound bool) string {
	if multiRound {
		return `During an admitted multi-round experiment, preserve the same proposal and report experiment_materiality only from the fresh Agent-selected post-action CCB evidence of the current round. A subthreshold state MUST use evaluation=insufficient_dose and does not disprove the hypothesis; the runtime may then open one next calibration round, and every round is evaluated only from its own fresh post-action evidence. Never request another mutation inside the round that just applied one. A material or ambiguous result settles through the human A/B evaluation: report experiment_target_response with outcome="human_audition_ready" together with experiment_round_decision="user_judgment_pending". Target response must never be inferred from parameter readback alone. Leave processor_type empty for this native bounded experiment.`
	}
	return `During an admitted D1-S1 experiment, preserve the same proposal and report experiment_materiality only from the fresh Agent-selected post-action CCB evidence. A subthreshold state MUST use evaluation=insufficient_dose and does not disprove the hypothesis, but it MUST NOT request next_round or another mutation; it settles only through the ambiguous human A/B evaluation: report experiment_target_response with response="ambiguous" and outcome="human_audition_ready" together with experiment_round_decision="user_judgment_pending" (any other target-response pair is rejected for a subthreshold round). A material result reports experiment_target_response with outcome="human_audition_ready" and the response value the fresh evidence shows (for example "directional"), together with the same user_judgment_pending boundary. Target response must never be inferred from parameter readback alone. Leave processor_type empty for this native bounded experiment.`
}

// freeStateRoundMutationLimitPromptRule is the round/mutation limit rule. The
// single-round wording is byte-identical to the historical prompt; the
// multi-round tier opens runtime-controlled next rounds but keeps continue_once
// and the per-round single-mutation prohibition (GLM ruling 1 + ADR §8: an
// ambiguous judgment is terminal and never authorizes another dose).
func freeStateRoundMutationLimitPromptRule(multiRound bool) string {
	if multiRound {
		return `This multi-round experiment permits exactly one forward mutation per round within the runtime-controlled experiment budget; never apply or request a second treatment within the same round and never return continue_once. Only retain, rollback, ambiguous human judgment, or a concrete blocked boundary may follow an applied action. An ambiguous human judgment (a difference was heard but neither candidate is preferred) is terminal: never propose another round, mutation, or dose change after it. Once the fresh post-action evidence is recorded, proposing another mix tick, suggestion, or improvement is ILLEGAL in post_action_evaluation: your only legal output is the settle report (experiment_materiality + experiment_target_response + experiment_round_decision=user_judgment_pending on the preserved proposal), or a concrete blocked boundary stating why the settle evidence itself cannot be obtained.`
	}
	return `D1-S1 permits one experiment round and one forward mutation. Never return next_round, continue_once, or a second treatment after the action has been applied; only retain, rollback, ambiguous human judgment, or a concrete blocked boundary may follow. Once the fresh post-action evidence is recorded, proposing another mix tick, suggestion, or improvement is ILLEGAL in post_action_evaluation: your only legal output is the settle report (experiment_materiality + experiment_target_response + experiment_round_decision=user_judgment_pending on the preserved proposal), or a concrete blocked boundary stating why the settle evidence itself cannot be obtained.`
}

// freeStateD1AdmittedDomainRule renders the needs_experiment domain rule from
// the experiment D1-S1 domain table so the prompt and the admission gate share
// one source of truth. Adding a domain row updates the wording; the bounds
// themselves are always enforced by the table's validators, not by this text.
func freeStateD1AdmittedDomainRule() string {
	return freeStateD1AdmittedDomainRuleForBudget(1)
}

func freeStateD1AdmittedDomainRuleForBudget(experimentBudget int) string {
	specs := experiment.D1S1DomainSpecs()
	if len(specs) == 0 {
		return "For needs_experiment in D1-S1, no action domain is admitted in this phase; return capability_blocked instead of inventing a fallback."
	}
	options := make([]string, 0, len(specs))
	for _, spec := range specs {
		options = append(options, fmt.Sprintf("action_domain=%s, action_kind=%s, %s", spec.ActionDomain, spec.ActionKind, spec.PromptParameterHint))
	}
	budgetClause := "In every case include verification_plan with experiment_budget=1 and max_action_attempts=1."
	if experimentBudget > 1 {
		budgetClause = fmt.Sprintf("In every case include verification_plan with experiment_budget=%d (runtime-controlled) and max_action_attempts=1.", experimentBudget)
	}
	return "For needs_experiment in D1-S1, return exactly one track target and one of the admitted domains: " + strings.Join(options, "; or ") +
		". Every listed domain is admitted and executable end-to-end by the experiment runtime in this phase; choose the one the requested evidence supports. " + budgetClause + " Domains or action kinds outside that list are unsupported: return capability_blocked instead of inventing a fallback."
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
		return "A closure candidate frontier is now available: " + strings.Join(rows, "; ") + ". The runtime resolves this frontier only through target-level evidence: request track.* observation(s) targeted at one listed track ID. Which candidate track and which track.* view(s) you request remain your own evidence-driven choice — a track-level view from any diagnostic dimension is admissible on a candidate track. Candidate evidence is not itself permission to select a processor.\n\n"
	}
	return "The closure selected candidate " + selected + " and has already recorded its target-level observation. The minimal observation loop is closed: return final=true now. Use needs_action only when that evidence supports a deterministic governed action; when it supports only a bounded improvement hypothesis, use needs_experiment with one improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation boundary. Do not request another observation or restart project-level inspection.\n\n"
}

// messageLoopFreeStateContinuationBudgetDirective escalates convergence
// pressure as the continuation budget drains while the loop is still in the
// pre-proposal observation phase. The directive is phase-aware: a
// needs_experiment decision is only admissible once the closure host has
// confirmed a target (fs6+), so before that the pressure text steers the model
// onto the fastest gate-legal observation path instead of demanding an early
// proposal the host would bounce. It is steering text only: no validator,
// admission gate, or runtime criterion reads it.
func messageLoopFreeStateContinuationBudgetDirective(state *runState) string {
	if state == nil || !strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
		return ""
	}
	if messageLoopFreeStateDiagnosticOnly(state) {
		return ""
	}
	ctx := messageLoopFreeStateContext(state)
	budget := messageLoopFreeStatePositiveInt(ctx["continuation_budget"])
	used := messageLoopFreeStatePositiveInt(ctx["continuation_used"])
	if budget <= 0 || used <= 0 {
		return ""
	}
	// Only the pre-proposal observation phase needs convergence pressure; an
	// admitted experiment legitimately spends continuations on post-action
	// observation and evaluation.
	if decision := messageLoopMapValue(ctx["latest_decision"]); decision != nil {
		switch strings.ToLower(strings.TrimSpace(messageLoopText(decision["status"]))) {
		case FreeStateNeedsExperiment, FreeStateImprovementProposal, FreeStateNeedsAction:
			return ""
		}
	}
	phase := messageLoopFreeStateHostPhase(state)
	remaining := budget - used
	switch {
	case remaining <= 1 && phaseAtLeastTargetConfirmed(phase):
		return fmt.Sprintf(`CONTINUATION BUDGET CRITICAL: %d of %d checkpoints are already consumed and this is (one of) the last usable turn(s) before the runtime settles the task at the observation boundary without ever exercising an improvement. The closure host has confirmed a target, so a bounded proposal is now admissible. You MUST return final=true on this turn with needs_experiment and one bounded improvement_proposal.v1 citing the target-level observation you already hold; plausible, reversible, and evidence-cited is enough. Do not request another observation.

`, used, budget)
	case remaining <= 1:
		return fmt.Sprintf(`CONTINUATION BUDGET CRITICAL: %d of %d checkpoints are already consumed and this is (one of) the last usable turn(s). The closure host is still in phase %s, which does not admit needs_experiment yet: a bounced proposal would only waste the turn. Follow the fastest gate-legal path instead: call ccb.observation_catalog if you have not discovered the available views yet; otherwise request the one view whose facts disclose track-level candidates with track IDs (skip it if it already returned), then the target-level track.* view on the single most plausible candidate track. Do not re-request any view that already returned.

`, used, budget, phase)
	case used*2 >= budget:
		return fmt.Sprintf(`Continuation budget warning: %d of %d checkpoints consumed while the closure host is still in phase %s. A needs_experiment decision is only admissible after the host confirms a target, so early proposals get bounced. The fastest gate-legal path is: (1) ccb.observation_catalog discovery if you have not seen the available views, then the view whose facts disclose candidates with track IDs; (2) the target-level track.* view on the single most plausible candidate track to confirm the target; (3) then final=true needs_experiment. Every checkpoint should buy evidence you do not already hold; do not re-request views that already returned.

`, used, budget, phase)
	}
	return ""
}

// messageLoopFreeStateObservationSaturationDirective renders the DIAG3-3
// mechanical runtime notice (chat-side injection at the closure boundary)
// into the neutral-family prompt — the same mechanical message channel family
// as the continuation-budget directive and the admission-gate refusal text.
// The notice is pure runtime state disclosure: coverage status, the queue
// record's open dimension names, frontier size, and continuation budget. It
// never names a track, a processor domain or family, or a view to request —
// the observation and proposal choices stay the model's own.
func messageLoopFreeStateObservationSaturationDirective(state *runState) string {
	if state == nil {
		return ""
	}
	notice := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_saturation_notice"])
	if len(notice) == 0 {
		return ""
	}
	used := messageLoopFreeStatePositiveInt(notice["continuation_used"])
	budget := messageLoopFreeStatePositiveInt(notice["continuation_budget"])
	frontier := messageLoopFreeStatePositiveInt(notice["frontier_candidates"])
	coverage := strings.TrimSpace(messageLoopText(notice["coverage_status"]))
	openDimensions := messageLoopStringList(notice["open_dimensions"])
	facts := fmt.Sprintf("coverage_status=%s; open_dimensions=[%s]; frontier_candidates=%d; continuation checkpoints used=%d/%d",
		coverage, strings.Join(openDimensions, ", "), frontier, used, budget)
	return fmt.Sprintf(`OBSERVATION SATURATION (mechanical runtime state): %s. The per-track primary observation duty is closed: observing covered targets again adds no new coverage. The legal exits are unchanged — a needs_experiment turn with one bounded improvement_proposal citing evidence already in the ledger, or an honest terminal boundary (no_candidate_found / capability_blocked). This notice is runtime state only; the judgment and the choice stay yours.

`, facts)
}

// freeStatePhaseDeferredResubmissionHint is the closed, fixed resubmission
// sentence. It cites one procedural fact only — an earlier final candidate
// was deferred by the closure phase, which now admits final candidates — and
// never carries domain, track, plug-in, dosage, or view content. The exact
// text is pinned by test.
const freeStatePhaseDeferredResubmissionHint = "An earlier final candidate was deferred only because the closure phase did not admit it at that time; the current phase now admits final candidates. Independently re-evaluate whether to submit a final candidate based on the evidence you hold, or end the turn honestly."

// messageLoopFreeStatePhaseDeferredResubmissionDirective renders the neutral
// resubmission hint when a prior needs_experiment decision was rejected only
// by the host phase gate (phase_deferred_final_candidate loop marker) and the
// current host phase now admits final candidates. Phase admission is read
// from the same audioclosure decision policy the phase gate itself enforces,
// so the hint can never surface while the phase would bounce the proposal
// again. Steering text only: no validator, gate, or runtime criterion reads
// it, and the re-evaluation stays entirely the model's own.
func messageLoopFreeStatePhaseDeferredResubmissionDirective(state *runState) string {
	if state == nil {
		return ""
	}
	loop := messageLoopFreeStateContext(state)
	if !freeStateBool(loop[freeStatePhaseDeferredFinalCandidateKey]) {
		return ""
	}
	phase, ok := audioclosure.ParsePhase(messageLoopFreeStateHostPhase(state))
	if !ok || !audioclosure.IsFSPhase(phase) {
		return ""
	}
	if !audioclosure.AllowsDecisionStatus(phase, FreeStateNeedsExperiment) {
		return ""
	}
	return freeStatePhaseDeferredResubmissionHint + "\n\n"
}

// messageLoopFreeStateHostPhase reads the closure host phase from the prompt
// context (empty string when not disclosed).
func messageLoopFreeStateHostPhase(state *runState) string {
	if state == nil {
		return ""
	}
	phase := strings.TrimSpace(firstMapText(state.input.Context, "free_state_phase"))
	if phase == "" {
		phase = strings.TrimSpace(messageLoopText(messageLoopMapValue(state.input.Context["minimal_audio_closure"])["phase"]))
	}
	return phase
}

// phaseAtLeastTargetConfirmed reports whether the host phase admits a bounded
// improvement proposal (fs6_target_confirmed or later, excluding fs9 where the
// loop is already settled).
func phaseAtLeastTargetConfirmed(phase string) bool {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "fs6_target_confirmed", "fs7_improvement_proposal", "fs8_experiment_verification":
		return true
	}
	return false
}

func messageLoopFreeStatePositiveInt(raw any) int {
	value := 0
	switch v := raw.(type) {
	case int:
		value = v
	case int64:
		value = int(v)
	case float64:
		value = int(v)
	case json.Number:
		parsed, _ := v.Int64()
		value = int(parsed)
	}
	return value
}

// freeStateImprovementConvergenceGuidance keeps the model budget-aware during
// an open improvement contract. The key constraint is that a needs_experiment
// decision is only admissible once the closure host has established a
// candidate frontier and confirmed a target: an early proposal gets bounced
// by the host phase gate and the turn is wasted. Since D2-NEUTRAL3 the
// guidance presents observation views fairly: catalog discovery is the first
// step, view dimensions are chosen by the model from the material at hand,
// and cross-dimension touring inside the budget is legal — no named fast-path
// view sequence and no anti-breadth prohibition. Steering only: no validator,
// gate, or runtime criterion reads it.
func freeStateImprovementConvergenceGuidance() string {
	return `Continuation Budget and Fair Observation Selection for Open Improvement Contracts:

- The free-state loop context discloses continuation_budget and continuation_used. The budget is small (typically 6 checkpoints total for the whole task, not per dimension). Every needs_observation turn you return consumes one checkpoint, and exhausting it settles the task at the observation boundary without ever exercising an improvement.
- The closure host admits needs_experiment only after a candidate frontier is established and a target is confirmed. A proposal sent before that is bounced and the turn is wasted. Do not rush the proposal; rush the GATE PATH instead.
- Make catalog discovery your first observation step: call ccb.observation_catalog before your first ccb.observation_request, and choose view dimensions yourself from the returned catalog and the material at hand — visible track identities and names, the user's listening goal, and any disclosed still-unobserved diagnostic dimensions. There is no default view sequence and no privileged dimension.
- Choose dimensions by evidence, not by habit: visiting more than one diagnostic dimension within the budget is legal and is often necessary, because a single-dimension reading cannot show whether the audible problem lives elsewhere. What wastes the budget is re-requesting views that already returned, or observing after the target-level evidence you need is already in hand.
- The gate path itself, whatever views you choose: a candidate frontier is built from observation facts that disclose track-level candidates, and a target is confirmed by a target-level track.* observation of one concrete candidate track. Once the target-level evidence for your chosen candidate is in hand, return final=true needs_experiment with one bounded improvement_proposal.v1 citing it.
- Plausible, reversible, and evidence-cited is enough for the proposal; proof of an objective defect is NOT required.

`
}

func freeStatePatternRecognitionGuidance() string {
	return `Evidence Pattern Recognition for Open Improvement Tasks:

When interpreting CCB observation results, consider these common patterns:

Level & Headroom Dimension:
- mix.masking_relationship with large consistent margins (median >20dB, coverage >0.9) across multiple bands
  → often indicates level-imbalance improvement candidates → consider track_gain domain
- mix.multitrack_relationship showing consistent level differences
  → suggests gain adjustment candidates
- track.basic_energy showing extreme headroom or crest differences
  → may indicate level normalization opportunities

Frequency Dimension:
- mix.masking_relationship with band-specific patterns (margin concentrated in 1-2 bands)
  → may indicate frequency-domain considerations → the static_eq domain is admitted for a bounded static_eq band adjustment
- mix.frequency_relationship showing band overlap between tracks
  → suggests frequency separation candidates
- track.timbre_frequency showing band imbalance within one track
  → may indicate tonal adjustment opportunities

Dynamics Dimension:
- track.time_dynamics showing extreme crest or envelope variation
  → suggests dynamics control candidates → the broadband_compression domain is admitted for a bounded compressor threshold adjustment
- processor.behavior showing excessive gain reduction or pumping
  → may indicate dynamics recalibration needs

Stereo Dimension:
- track.stereo_space showing extreme correlation or imbalance
  → suggests stereo width or balance candidates

Transient Dimension:
- track.transient_structure showing onset/sustain imbalance
  → suggests transient shaping candidates

Important Epistemic Notes:
- These patterns are guidance for hypothesis formation, not deterministic rules
- Partial or bounded evidence supporting a plausible improvement hypothesis is sufficient for needs_experiment
- You are NOT required to prove an objective defect before proposing a bounded reversible experiment
- When evidence plausibly relates to the user's listening goal but cannot prove a defect, return needs_experiment with improvement_proposal
- Large consistent patterns across time and bands are stronger signals than isolated or brief variations

`
}
