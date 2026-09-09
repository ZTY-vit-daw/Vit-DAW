package agentloop

import (
	"encoding/json"
	"strings"
	"testing"
)

// TIMING-2 (2026-09-09, advisory #6 v1.1): three disclosure changes, all
// machine-state driven and content-blind — ① the GATE PATH pre-disclosure
// (which admission-gate evidence the ledger still lacks, naming the two
// qualified mix scan views and disqualifying project-level structure views),
// ② the locked-turn duplicate-fingerprint honesty sentence, ③ the G3 gap
// binding naming the qualified views. Advisory #6 v1.1 guards: the disclosure
// never adjudicates (guard ①), the wording stays content-blind (guard ②),
// and the trigger is ledger/accounting state only with zero phase conditions
// (guard ③). The fixtures reuse the shared gateTestState green control
// (target track 1007, candidate tracks 1007/1012, target-level observation
// obs-target at rev-7).

// timing2WithoutMixScanReceipts builds the green control minus every mix scan
// receipt, so exactly G3 fails (the BEHAVIOR-1 p03 ledger shape: observations
// exist, but none of them is a qualified mix scan).
func timing2WithoutMixScanReceipts(mutate func(ctx map[string]any)) *runState {
	state := gateTestState(mutate)
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	kept := make([]any, 0, 4)
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		scan := false
		for _, viewID := range messageLoopStringList(row["requested_views"]) {
			for _, qualified := range freeStateProjectScanViews {
				if strings.EqualFold(strings.TrimSpace(viewID), qualified) {
					scan = true
				}
			}
		}
		if !scan {
			kept = append(kept, row)
		}
	}
	ledger["receipts"] = kept
	return state
}

// timing2AddMixScanReceipt appends a ready, revision-bound mix scan receipt
// (the host-side ledger advance between slices).
func timing2AddMixScanReceipt(state *runState) {
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	ledger["receipts"] = append(messageLoopMapRows(ledger["receipts"]), map[string]any{
		"status": "ready", "observation_id": "obs-mix-scan", "requested_views": []any{"mix.multitrack_relationship"},
		"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
	})
}

func timing2DeepCloneContext(state *runState) map[string]any {
	data, err := json.Marshal(state.input.Context)
	if err != nil {
		panic(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func timing2GateVerdicts(state *runState, decision *FreeStateDecision) map[string]bool {
	return map[string]bool{
		freeStateGateG1: gateG1(state), freeStateGateG2: gateG2(state),
		freeStateGateG3: gateG3(state), freeStateGateG4: gateG4(state),
		freeStateGateG5: gateG5(state), freeStateGateG6: gateG6(state),
		freeStateGateG7: gateG7(state, decision.ImprovementProposal.EvidenceRefs),
		freeStateGateG8: gateG8(state, decision.ImprovementProposal),
	}
}

func timing2SameVerdicts(left, right map[string]bool, except string) bool {
	for gate, value := range left {
		if gate == except {
			continue
		}
		if right[gate] != value {
			return false
		}
	}
	return true
}

// ① Appearance: with no usable mix scan receipt the standing GATE PATH row is
// present and names both qualified views plus the project-structure contrast
// (the 7/7 semantic-mismatch root cause BEHAVIOR-1 measured).
func TestTIMING2GatePathDirectiveAppearsWhenMixScanMissing(t *testing.T) {
	state := timing2WithoutMixScanReceipts(nil)
	directive := messageLoopFreeStateGatePathDirective(state)
	if directive == "" {
		t.Fatal("the GATE PATH disclosure is absent while G3 has no qualified mix scan receipt")
	}
	for _, needle := range []string{
		"GATE PATH (mechanical runtime state)", "G3_project_scan",
		"mix.multitrack_relationship", "mix.frequency_relationship",
		"project.structure",
	} {
		if !strings.Contains(directive, needle) {
			t.Fatalf("GATE PATH disclosure missing %q: %q", needle, directive)
		}
	}
	// On the green control (G3/G5/G6 all satisfied) the directive stays silent:
	// nothing is missing, so there is no path left to disclose.
	if got := messageLoopFreeStateGatePathDirective(gateTestState(nil)); got != "" {
		t.Fatalf("fully satisfied gate must not render a GATE PATH row: %q", got)
	}
}

// ① Per-row exit: each row follows its own gate state — removing the frontier
// adds the G5 row, removing the target-level observation adds the G6 row, and
// restoring the mix scan receipt exits the G3 row.
func TestTIMING2GatePathDirectiveRowsFollowOwnGateState(t *testing.T) {
	noFrontier := timing2WithoutMixScanReceipts(func(ctx map[string]any) {
		delete(messageLoopMapValue(ctx["minimal_audio_closure"]), "hypothesis_frontier")
	})
	directive := messageLoopFreeStateGatePathDirective(noFrontier)
	if !strings.Contains(directive, "G5_frontier_established") {
		t.Fatalf("missing frontier must render the G5 row: %q", directive)
	}
	if !strings.Contains(directive, "G3_project_scan") {
		t.Fatalf("G3 row must stay while the mix scan receipt is still missing: %q", directive)
	}
	if strings.Contains(directive, "G6_target_evidence") {
		t.Fatalf("G6 row rendered without a selected candidate: %q", directive)
	}

	noTargetEvidence := timing2WithoutMixScanReceipts(func(ctx map[string]any) {
		ledger := messageLoopMapValue(messageLoopMapValue(ctx["free_state_reasoning_loop"])["observation_ledger"])
		delete(messageLoopMapValue(ledger["available_views"]), "track:1007::track.timbre_frequency")
		for _, row := range messageLoopMapRows(ledger["receipts"]) {
			if messageLoopText(row["observation_id"]) == "obs-target" {
				row["status"] = "rejected"
			}
		}
	})
	directive = messageLoopFreeStateGatePathDirective(noTargetEvidence)
	if !strings.Contains(directive, "G6_target_evidence") {
		t.Fatalf("missing target-level observation must render the G6 row: %q", directive)
	}

	// Ledger advance: the mix scan receipt lands and the G3 row exits while
	// the G6 row (still failing) stays — the rows exit independently.
	advanced := timing2WithoutMixScanReceipts(func(ctx map[string]any) {
		ledger := messageLoopMapValue(messageLoopMapValue(ctx["free_state_reasoning_loop"])["observation_ledger"])
		delete(messageLoopMapValue(ledger["available_views"]), "track:1007::track.timbre_frequency")
	})
	timing2AddMixScanReceipt(advanced)
	directive = messageLoopFreeStateGatePathDirective(advanced)
	if strings.Contains(directive, "G3_project_scan") {
		t.Fatalf("G3 row did not exit after the qualified receipt landed: %q", directive)
	}
	if !strings.Contains(directive, "G6_target_evidence") {
		t.Fatalf("G6 row must survive the G3 exit: %q", directive)
	}
}

// ③ Zero phase conditions: the disclosure trigger reads no phase field — the
// directive output is byte-identical across every phase value (and phase-free).
func TestTIMING2GatePathDirectiveZeroPhaseConditions(t *testing.T) {
	base := timing2WithoutMixScanReceipts(nil)
	want := messageLoopFreeStateGatePathDirective(base)
	for _, phase := range []string{"", "fs0_semantic_entry", "fs2_capacity_assessed", "fs3_project_scan", "fs4_diagnostic_round", "fs5_candidate_frontier", "fs9_terminal"} {
		state := timing2WithoutMixScanReceipts(func(ctx map[string]any) {
			if phase == "" {
				delete(ctx, "free_state_phase")
				delete(messageLoopMapValue(ctx["minimal_audio_closure"]), "phase")
				return
			}
			ctx["free_state_phase"] = phase
			messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = phase
		})
		if got := messageLoopFreeStateGatePathDirective(state); got != want {
			t.Fatalf("phase %q changed the GATE PATH disclosure:\n got %q\nwant %q", phase, got, want)
		}
	}
}

// ① "Disclosure only, never adjudicates" (v1.1 guard ①): the disclosure's
// presence/absence never changes a G1–G8 verdict. The terminal lock (a pure
// disclosure trigger, not a gate input) toggles the directive off while every
// gate verdict stays identical; the ledger advance (a gate input) flips G3 and
// exits the G3 row together. Rendering the full prompt mutates no state, and
// the gate/phase refusal surfaces carry no GATE PATH text.
func TestTIMING2GatePathDisclosureNeverAdjudicates(t *testing.T) {
	proposal := gateTestProposal(nil).FreeStateDecision

	missing := timing2WithoutMixScanReceipts(nil)
	if messageLoopFreeStateGatePathDirective(missing) == "" {
		t.Fatal("control: the disclosure must be present on the G3-missing state")
	}
	verdictsMissing := timing2GateVerdicts(missing, proposal)

	// ±disclosure via a non-gate input: the terminal lock silences the GATE
	// PATH directive (the terminal directive owns the locked turn) and leaves
	// all eight gate verdicts byte-identical.
	locked := timing2WithoutMixScanReceipts(func(ctx map[string]any) {
		messageLoopMapValue(ctx["free_state_reasoning_loop"])[freeStateTerminalTurnLockedKey] = true
	})
	if got := messageLoopFreeStateGatePathDirective(locked); got != "" {
		t.Fatalf("a locked turn must not render the GATE PATH row: %q", got)
	}
	if verdicts := timing2GateVerdicts(locked, proposal); !timing2SameVerdicts(verdictsMissing, verdicts, "") {
		t.Fatalf("the disclosure toggle changed gate verdicts: %v vs %v", verdictsMissing, verdicts)
	}

	// The ledger advance is the only lever that flips G3 — and it exits the
	// disclosure row at the same time (appearance/exit is ledger-driven).
	advanced := timing2WithoutMixScanReceipts(nil)
	timing2AddMixScanReceipt(advanced)
	verdictsAdvanced := timing2GateVerdicts(advanced, proposal)
	if verdictsAdvanced[freeStateGateG3] == verdictsMissing[freeStateGateG3] {
		t.Fatal("control: the mix scan receipt must flip G3")
	}
	if !timing2SameVerdicts(verdictsMissing, verdictsAdvanced, freeStateGateG3) {
		t.Fatalf("the ledger advance moved a verdict other than G3: %v vs %v", verdictsMissing, verdictsAdvanced)
	}
	if messageLoopFreeStateGatePathDirective(advanced) != "" {
		t.Fatal("the fully satisfied gate must not render a GATE PATH row")
	}

	// Read-only: rendering the full system prompt leaves the context untouched.
	before, _ := json.Marshal(timing2DeepCloneContext(missing))
	_ = messageLoopNeutralFamilySystemPrompt(missing)
	after, _ := json.Marshal(timing2DeepCloneContext(missing))
	if string(before) != string(after) {
		t.Fatal("rendering the prompt mutated the loop context")
	}

	// Wiring: the refusal surfaces never carry the disclosure text.
	gateMessage := freeStateNeedsExperimentGateFailureMessage(missing, gateTestProposal([]string{"obs-ghost"}).FreeStateDecision,
		evaluateFreeStateNeedsExperimentGate(missing, gateTestProposal([]string{"obs-ghost"}).FreeStateDecision))
	if strings.Contains(gateMessage, "GATE PATH (mechanical runtime state)") {
		t.Fatalf("the gate refusal message carries prompt-side disclosure text: %q", gateMessage)
	}
	if issue := messageLoopFreeStatePhaseDecisionIssue(missing, "needs_action"); strings.Contains(issue, "GATE PATH (mechanical runtime state)") {
		t.Fatalf("the phase refusal carries prompt-side disclosure text: %q", issue)
	}
}

// ② Content-blind negative assertions (v1.1 guard ②): the view IDs appear
// only as structural gate-condition vocabulary — no domain, track, plug-in,
// or dosage semantics ride the disclosure.
func TestTIMING2GatePathDirectiveContentBlind(t *testing.T) {
	directive := messageLoopFreeStateGatePathDirective(timing2WithoutMixScanReceipts(nil))
	assertTiming2ContentBlind(t, directive)
	// The view IDs may not co-occur with treatment-plan vocabulary.
	for _, banned := range []string{"gain", "threshold", "dose", "dosage", "dB", "treat", "fix", "boost", "cut "} {
		if strings.Contains(strings.ToLower(directive), strings.ToLower(banned)) {
			t.Fatalf("GATE PATH disclosure carries treatment-plan vocabulary %q: %q", banned, directive)
		}
	}
}

// ② Locked-turn duplicate-fingerprint disclosure trigger: locked ∧ fingerprint
// latched ∧ revision match — with the fingerprint slot and the legal-output
// set in the sentence. A new evidence revision, an unlocked loop, an empty
// fingerprint, or (for the gate-message variant) a different proposal all
// suppress it.
func TestTIMING2LockedDuplicateDisclosureTriggerConditions(t *testing.T) {
	lockWithFingerprint := func(revision string) *runState {
		return gateTestState(func(ctx map[string]any) {
			loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
			loop[freeStateTerminalTurnLockedKey] = true
			loop["terminal_turn_reason"] = freeStateTerminalReasonDuplicateFingerprint
			loop[freeStateLastRejectedFingerprintKey] = "proposal:abc123def4567890abcdef"
			loop[freeStateLastRejectedEvidenceRevisionKey] = revision
		})
	}
	matching := lockWithFingerprint("rev-7")
	directive := messageLoopFreeStateTerminalTurnDirective(matching)
	if !strings.Contains(directive, "DUPLICATE FINGERPRINT") || !strings.Contains(directive, "proposal:abc123def4567890abcdef") {
		t.Fatalf("terminal directive missing the duplicate-fingerprint disclosure: %q", directive)
	}
	for _, needle := range []string{"blocked", "no_candidate_found", "limitations"} {
		if !strings.Contains(directive, needle) {
			t.Fatalf("disclosure missing the legal-output %q: %q", needle, directive)
		}
	}
	// The frozen BOUNDARY-1 sentence stays embedded exactly once.
	if strings.Count(directive, freeStateTerminalTurnSentence) != 1 {
		t.Fatalf("terminal directive lost the frozen sentence: %q", directive)
	}

	// Not-trigger family: new revision in play / unlocked / empty fingerprint.
	if got := messageLoopFreeStateTerminalTurnDirective(lockWithFingerprint("rev-8")); strings.Contains(got, "DUPLICATE FINGERPRINT") {
		t.Fatalf("a new evidence revision must suppress the disclosure: %q", got)
	}
	unlocked := gateTestState(func(ctx map[string]any) {
		loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
		loop[freeStateLastRejectedFingerprintKey] = "proposal:abc123def4567890abcdef"
		loop[freeStateLastRejectedEvidenceRevisionKey] = "rev-7"
	})
	if got := messageLoopFreeStateTerminalTurnDirective(unlocked); got != "" {
		t.Fatalf("an unlocked loop must not render the terminal directive: %q", got)
	}
	emptyFingerprint := gateTestState(func(ctx map[string]any) {
		loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
		loop[freeStateTerminalTurnLockedKey] = true
		loop["terminal_turn_reason"] = "budget_critical"
	})
	if got := messageLoopFreeStateTerminalTurnDirective(emptyFingerprint); strings.Contains(got, "DUPLICATE FINGERPRINT") {
		t.Fatalf("an empty fingerprint must suppress the disclosure: %q", got)
	}

	// Gate-message variant: the disclosure rides the bounced refusal the model
	// receives only when the bounced decision IS the latched same-fingerprint
	// resubmission. It is appended at the output-issue wrapper, never inside
	// the pure gap template — the anti-abuse identity comparison recomputes
	// the pure template and must stay stable across the fingerprint latch.
	bounced := gateTestProposal([]string{"obs-ghost"}).FreeStateDecision
	latched := freeStateProposalFingerprint(bounced.ImprovementProposal)
	lockedState := gateTestState(func(ctx map[string]any) {
		loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
		loop[freeStateTerminalTurnLockedKey] = true
		loop["terminal_turn_reason"] = freeStateTerminalReasonDuplicateFingerprint
		loop[freeStateLastRejectedFingerprintKey] = latched
		loop[freeStateLastRejectedEvidenceRevisionKey] = "rev-7"
	})
	failed := evaluateFreeStateNeedsExperimentGate(lockedState, bounced)
	if strings.Contains(freeStateNeedsExperimentGateFailureMessage(lockedState, bounced, failed), "DUPLICATE FINGERPRINT") {
		t.Fatal("the pure gap template must stay free of the disclosure (anti-abuse identity stability)")
	}
	issue := messageLoopFreeStateOutputIssue(lockedState, gateTestProposal([]string{"obs-ghost"}))
	if !strings.Contains(issue, "DUPLICATE FINGERPRINT") || !strings.Contains(issue, latched) {
		t.Fatalf("the same-fingerprint bounce lost its disclosure: %q", issue)
	}
	// A different failing proposal on the same locked turn gets no disclosure
	// (it is a repair attempt, not a resubmission).
	different := gateTestProposal([]string{"obs-ghost-2"})
	if issue = messageLoopFreeStateOutputIssue(lockedState, different); strings.Contains(issue, "DUPLICATE FINGERPRINT") {
		t.Fatalf("a different proposal must not carry the duplicate disclosure: %q", issue)
	}
	// The ordinary (unlocked) loop never sees it — the gap feedback semantics
	// of TIMING-1 stay byte-identical there.
	ordinary := gateTestState(nil)
	ordinaryBounce := gateTestProposal([]string{"obs-ghost"}).FreeStateDecision
	ordinaryMessage := freeStateNeedsExperimentGateFailureMessage(ordinary, ordinaryBounce, evaluateFreeStateNeedsExperimentGate(ordinary, ordinaryBounce))
	if strings.Contains(ordinaryMessage, "DUPLICATE FINGERPRINT") {
		t.Fatalf("the ordinary gap feedback gained disclosure text: %q", ordinaryMessage)
	}
	// The identity comparison of the anti-abuse accounting still matches the
	// recomputed message on ordinary turns.
	if issue := messageLoopFreeStateOutputIssue(ordinary, gateTestProposal([]string{"obs-ghost"})); issue != ordinaryMessage {
		t.Fatalf("ordinary bounce message drifted from the recomputed identity: %q vs %q", issue, ordinaryMessage)
	}
	// Recording the same bounce twice under a pre-existing reservation lock
	// (the TIMING-1 identity pattern) still accounts — the latched fingerprint
	// does not destabilize the recomputed pure message.
	reserved := gateTestState(func(ctx map[string]any) {
		loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
		loop[freeStateTerminalTurnLockedKey] = true
		loop["terminal_turn_reason"] = "budget_critical"
	})
	reservedIssue := messageLoopFreeStateOutputIssue(reserved, gateTestProposal([]string{"obs-ghost"}))
	messageLoopFreeStateNoteAdmissionRejection(reserved, gateTestProposal([]string{"obs-ghost"}), reservedIssue)
	messageLoopFreeStateNoteAdmissionRejection(reserved, gateTestProposal([]string{"obs-ghost"}), reservedIssue)
	if count := freeStateAdmissionRejectionCount(reserved); count != 2 {
		t.Fatalf("the disclosure broke the accounting identity under a reservation lock: count=%d", count)
	}
}

func TestTIMING2LockedDisclosureContentBlind(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
		loop[freeStateTerminalTurnLockedKey] = true
		loop["terminal_turn_reason"] = freeStateTerminalReasonDuplicateFingerprint
		loop[freeStateLastRejectedFingerprintKey] = "proposal:abc123def4567890abcdef"
		loop[freeStateLastRejectedEvidenceRevisionKey] = "rev-7"
	})
	assertTiming2ContentBlind(t, messageLoopFreeStateTerminalTurnDirective(state))
}

// ③ The G3 gap binding names both qualified views and disqualifies
// project-level structure views — the semantic mismatch BEHAVIOR-1 measured
// (7/7 model scans chose project.structure) is now named in the bounce
// itself. Content-blind apart from the structural view vocabulary.
func TestTIMING2G3GapBindingNamesQualifiedViews(t *testing.T) {
	state := timing2WithoutMixScanReceipts(nil)
	out := gateTestProposal(nil)
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "G3_project_scan") {
		t.Fatalf("control: the proposal must fail G3 on the scan-less ledger: %q", issue)
	}
	gap := decodeAdmissionGap(t, issue)
	found := false
	for _, missing := range gap.Missing {
		if missing.GateID != freeStateGateG3 {
			continue
		}
		found = true
		for _, needle := range []string{"mix.multitrack_relationship", "mix.frequency_relationship", "project.structure"} {
			if !strings.Contains(missing.Binding, needle) {
				t.Fatalf("G3 binding missing %q: %q", needle, missing.Binding)
			}
		}
	}
	if !found {
		t.Fatalf("gap missing the G3 condition: %+v", gap.Missing)
	}
	assertTiming2ContentBlind(t, issue)
}

// assertTiming2ContentBlind enforces the red line on the TIMING-2 disclosure
// surfaces: no track identity, domain/family, plug-in identity, or dosage may
// ride the text (view ids and gate ids are structural vocabulary, advisory #6
// question 2).
func assertTiming2ContentBlind(t *testing.T, text string) {
	t.Helper()
	for _, banned := range []string{
		"1007", "1012", "1032", // track identities from the fixture
		"track_gain", "static_eq", "broadband_compression", "compressor", "limiter", "de_esser", "transient_shaper", "multiband_dynamics", // domains/families
		"delta_db", "-0.5", "0.5", // dose
		"plugin", "plug-in", "vendor", // plug-in identity
	} {
		if strings.Contains(text, banned) {
			t.Fatalf("disclosure leaked the content hint %q: %q", banned, text)
		}
	}
}
