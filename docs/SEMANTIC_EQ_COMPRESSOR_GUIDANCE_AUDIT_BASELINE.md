# EQ/Compressor Semantic Guidance Audit Baseline

Status: historical characterization baseline captured by TODO 1 before the
TODO 2 semantic-entry change. Entries marked `steering` or `automatic` record
the pre-change behavior and are not a description of the current entry path.
Current behavior is documented in `SEMANTIC_ENTRY_V1.md`.

## TODO 2 Delta

- New ordinary-Agent turns are classified by the model through
  `semantic_entry_decision.v1` before free-state startup.
- EQ, Compressor, and broad-treatment lexical predicates no longer start
  free-state.
- The selected EQ topology is not read or injected before a model-owned family
  handoff. The helper now requires an explicit downstream EQ authorization.
- The selected-Compressor lexical fast path and post-loop EQ lexical router are
  no longer called from the ordinary Agent entry path.
- Discussion and observation mutation barriers follow the verified model route
  rather than overriding it with request wording.
- The free-state prompt no longer maps source dynamics to Compressor, broad
  tonal evidence to EQ, or broad wording to a default observation call.
- Unknown semantic-entry fields, including family/plugin/parameter injection,
  are rejected. Client-provided semantic-entry context is stripped before the
  model classification.

## Scope

This audit covers the ordinary-Agent EQ and broadband-compressor path from
natural-language entry through free-state observation, processor-family
selection, loaded-instance arbitration, governed execution, and post-action
re-evaluation. Explicit parameter commands are recorded separately because
they intentionally use typed direct-control paths rather than abstract
semantic family selection.

## Current Call Chain

1. `runAgentLoopChat` calls `prepareFreeStateReasoningContext`.
2. `shouldStartFreeStateReasoningLoop` applies lexical entry predicates.
3. `agentLoopContextWithGenericEQTopology` may read and disclose EQ topology.
4. `messageLoopSystemPrompt` gives the model the free-state protocol.
5. The model returns `needs_observation`, `needs_action`, `satisfied`, or
   `blocked` in `FreeStateDecision`.
6. A structured observation decision is materialized through CCB.
7. `routeOrdinaryAgentTreatmentStrategy` binds the model-selected family and
   asks the model to choose a qualified existing instance or `load_required`.
8. The EQ or compressor planner creates a typed proposal; deterministic code
   validates, freezes, confirms, executes, reads back, restores on failure,
   and verifies.
9. `maybeContinueFreeStateAfterInteraction` records the action receipt and
   returns to the model for post-action observation and evaluation.

## Lexical Routing Inventory

| Location | Current behavior | Classification |
| --- | --- | --- |
| `free_state_reasoning_loop.go:shouldStartFreeStateReasoningLoop` | Starts free-state only when one of the EQ, broad-treatment, or compressor lexical predicates matches. Exact compressor parameter requests bypass it. | steering entry gate |
| `plugin_recommendation.go:ordinaryAgentPluginRecommendationIntent` | Treats an actionable EQ request on a selected track with no selected plugin as plugin recommendation, so it bypasses free-state before model-owned observation/family selection. | pre-free-state EQ handoff |
| `semantic_eq_planner.go:ordinaryAgentSemanticEQTextTopic` | Recognizes words including `mud`, `muddy`, `boxy`, `harsh`, `bright`, `dark`, `treble`, `bass`, `presence`, `eq`, `shelf`, `bell`, and `cut`. | family-specific lexical gate |
| `semantic_eq_planner.go:ordinaryAgentSemanticEQMutationIntent` | Uses discussion/action word lists to decide whether EQ language is actionable. | mutation lexical gate |
| `semantic_treatment_strategy.go:ordinaryAgentTreatmentStrategyIntent` | Uses broad audible-goal and action word lists to enter method arbitration. | generic lexical gate |
| `semantic_compressor_workflow.go:compressorExactParameterRequest` | Detects named compressor controls followed by a number and routes them away from free-state. | explicit-control exception |
| `semantic_compressor_workflow.go:ordinaryAgentSemanticCompressorPlanningRequest` | Uses compressor/dynamics/punch/transient terms plus semantic-action terms to enter the selected-compressor planner. | family-specific lexical gate |
| `agentloop/message_loop.go:messageLoopNaturalMixRequest` | Uses a broad mix/acoustic word list, including EQ- and dynamics-adjacent terms, to decide whether deterministic legacy observation is required outside free-state. | observation lexical gate |
| `agentloop/message_loop.go:messageLoopAudioObservationRequest` | Uses observation, spectrum, level, dynamics, waveform, and envelope terms to classify an audio-observation request. | observation lexical gate |

The lexical predicates decide entry rather than directly assigning the final
`FreeStateDecision.ProcessorType`. They still affect which requests receive
open observation and which context is present when the model chooses a family.

## Prompt and Early-Disclosure Inventory

| Location | Current behavior | Classification |
| --- | --- | --- |
| `goalrunner_chat.go:agentLoopContextWithGenericEQTopology` | Reads live EQ parameters and injects `generic_eq_topology` before the message loop whenever the selected plugin and EQ lexical topic match. | pre-family topology injection |
| `agentloop/message_loop.go:messageLoopSystemPrompt` | States that macro dynamics may support compression and broad tonal evidence may support EQ. | evidence-to-family soft steering |
| `agentloop/message_loop.go:messageLoopSystemPrompt` | Gives `track.time_dynamics` as the concrete `needs_observation` example. | concrete observation-view example |
| `agentloop/message_loop.go:messageLoopSystemPrompt` | Includes a full concrete EQ action example and EQ-specific planning rules in the same prompt used for free-state family selection. | asymmetric family disclosure |
| `agentloop/message_loop.go:messageLoopSystemPrompt` | Suggests `mix.frequency_relationship` and `track.timbre_frequency` when masking, muddiness, clarity, or accompaniment coverage remains after a dynamics action. | intent-to-view soft steering |
| `semantic_treatment_strategy.go:planSemanticTreatment` | In non-free-state arbitration, discloses loaded instance classifications, allowed load-required families, and examples of materially different EQ/compression/saturation/space strategies. In free-state, the upstream family is a hard constraint. | method-arbitration disclosure |
| `semantic_eq_planner.go:planOrdinaryAgentSemanticEQWithFeedback` | Allows `user_report/not_needed` when observation evidence is absent or unsuitable. | observation-optional family planner |
| `semantic_compressor_planner.go:planOrdinaryAgentCompressorIntent` | Discloses the identity card before acoustic observation and explicitly allows product-name knowledge as a prior. | identity prior |

## Observation Materialization Inventory

| Location | Current behavior | Classification |
| --- | --- | --- |
| `agentloop/free_state_reasoning.go:coerceMessageLoopFreeStateObservationOutput` | Converts a model-produced `requested_view_ids` list into one `ccb.observation_request` and preserves the list exactly when no CCB call was emitted. | model-request materialization |
| `agentloop/free_state_reasoning.go:coerceMessageLoopFreeStateObservationOutput` | Leaves an explicit CCB tool call untouched and does not cross-check its `view_ids` against `FreeStateDecision.RequestedViewIDs`. | audit gap |
| `agentloop/free_state_reasoning.go:messageLoopFreeStateOutputIssue` | Requires fresh CCB observation after an applied action, but permits the first `needs_action` without any successful observation. | initial-observation gap |
| `agentloop/message_loop.go:preflightNaturalMixObservation` | Automatically executes `mix.observe` for qualifying natural mix requests, but explicitly skips active free-state loops. | automatic legacy/non-free-state observation |
| `agentloop/message_loop.go:messageLoopDeterministicMixObservationCall` | Derives observation scope and selected/focus target from lexical intent and current state for the automatic preflight. | automatic observation argument binding |
| `agentloop/message_loop.go:coerceWaveformBakeToMixObservation` | Rewrites qualifying or malformed waveform-bake calls to the preferred mix observation tool. | automatic tool rewrite |
| `agentloop/message_loop.go:coerceMixObservationCall` | Rewrites `mix.request_observation` to `mix.observe` when available and deterministically fills goal/session/scope arguments. | automatic tool/argument rewrite |
| `semantic_compressor_workflow.go:observeSemanticCompressorContext` | Materializes the LLM-selected compressor evidence mode and deterministic scope through `mix.observe`, including bounded fallback/retry behavior. | model-selected, deterministic observation |
| `harness/ccb_observation.go:invokeCCBObservationRequest` | Materializes dependencies required by the exact requested semantic views. | deterministic view execution |

## Family and Instance Authority

- `agentloop/free_state_reasoning.go:FreeStateDecision` describes the family
  choice as model-owned and carries no execution authority.
- `messageLoopFreeStateProcessorGoverned` currently admits only EQ and
  compressor into open-semantic execution.
- `semantic_treatment_strategy.go:planSemanticTreatment` receives the upstream
  `required_processor_type` as a hard constraint. The treatment model may
  choose an eligible existing instance or `load_required`, but cannot replace
  that family.
- Existing-plugin choices must use an exact supplied `instance_key`.
- `semanticTreatmentInstances` currently collapses each loaded instance to one
  qualification. A qualified broadband compressor takes precedence over an
  embedded EQ surface; otherwise a generic static EQ may qualify.
- The EQ lexical fast path can automatically bind the only qualified loaded EQ
  without a model instance-arbitration turn. The free-state treatment path uses
  model selection.
- The treatment state token rejects a choice after the live plugin graph has
  changed.

## Execution and Re-evaluation Authority

- EQ and compressor semantic plans have no direct mutation authority.
- Deterministic code binds live topology, freezes a proposal, requests
  confirmation, performs typed atomic apply, reads back actual values, audits
  non-target parameters, and restores on failure.
- `freeStateAcousticActionOutcome` currently recognizes only EQ and compressor
  action receipts.
- `maybeContinueFreeStateAfterInteraction` requires a fresh post-action CCB
  observation before satisfaction or another processor action.
- A successfully applied family cannot currently be selected again in the same
  free-state loop. The total action safety limit is six.

## Characterization Test Matrix

| Concern | Test evidence |
| --- | --- |
| Current lexical entry and exact-parameter exception | `TestSemanticGuidanceAuditBaselineCurrentLexicalEntryRouting` |
| Current pre-family EQ topology injection | `TestSemanticGuidanceAuditBaselineCurrentPreFamilyEQTopologyInjection` |
| Current prompt family/view/EQ disclosures | `TestSemanticGuidanceAuditBaselineCurrentPromptDisclosures` |
| Initial action currently accepted without observation | `TestSemanticGuidanceAuditBaselineCurrentInitialActionNeedsNoObservation` |
| Model-requested views preserved during CCB materialization | `TestSemanticGuidanceAuditBaselineModelRequestedViewsArePreserved` |
| Explicit CCB/decision view mismatch currently not rejected | `TestSemanticGuidanceAuditBaselineCurrentCCBCallWinsWithoutViewCrossCheck` |
| Upstream family and exact qualified instance binding | `TestSemanticGuidanceAuditBaselineCurrentFamilyAndInstanceBinding` |
| Unknown/identity-only instance rejection | `TestSemanticTreatmentCannotEscalateIdentityOnlyInstanceToExecutablePlanner` |
| Stale instance-selection rejection | `TestSemanticTreatmentStateTokenExpiresWhenPluginGraphChanges` |
| Post-action fresh observation requirement | `TestFreeStateLoopRequiresCCBObservationBeforePostActionSatisfaction` |
| Cross-family continuation after fresh observation | `TestFreeStateLoopRequiresCCBObservationBeforeAnotherPostActionProcessor` |
| Same-family automatic repeat rejection | `TestFreeStateRejectsAutomaticRepeatOfAppliedProcessorFamily` |
| Compressor progressive disclosure | `TestCompressorIntentPlannerSeesIdentityCardBeforeCOM`, `TestCompressorControlPlannerUsesProgressiveBriefAndNoExecutionIdentity` |
| EQ typed planning and target binding | `TestPlanOrdinaryAgentSemanticEQReturnsTypedAction`, `TestPlanOrdinaryAgentSemanticEQRejectsChangedExactTarget` |

## Follow-up Neutralization Targets

The subsequent TODOs should replace, rather than silently delete, the
characterization assertions for:

1. lexical family entry;
2. pre-family EQ topology disclosure;
3. family-specific evidence hints and concrete view examples in the free-state
   prompt;
4. first-action observation optionality;
5. product-name prior use;
6. structured decision/tool-call view mismatch;
7. the single-family loaded-instance representation and EQ-only auto-binding.

No behavior is changed by this baseline.
