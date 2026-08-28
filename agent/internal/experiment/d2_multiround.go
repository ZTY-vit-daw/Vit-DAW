package experiment

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// MaxD2MultiRoundBudget is the sealed upper bound of the D2-2 multi-round
// experiment budget. The bound is locked by a sealed assertion
// (TestD2MultiRoundAdmissionBudgetBounds): raising it is a deliberate,
// reviewed decision, never an accident of a wider budget field.
const MaxD2MultiRoundBudget = 4

// d2MultiRoundCumulativeDeltaBoundDB is the experiment-lifetime cumulative
// displacement bound of the D2-2 multi-round tier. Per GLM ruling 2 it equals
// the domain-table single-action absolute bound (2 dB for every admitted
// domain); TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound seals
// that equality against the live domain table.
const d2MultiRoundCumulativeDeltaBoundDB = 2.0

// ErrJudgmentPending is returned by StartRound, ApplyIntervention, and the
// non-settle DecideRound family while an earlier round of this experiment has
// a user judgment requested but not landed (GLM ruling 3: the judgment
// boundary persists at experiment scope, not per round).
var ErrJudgmentPending = errors.New("judgment pending for this experiment")

// IsD2MultiRound reports whether the admission opted into the D2-2 multi-round
// tier: an admitted domain member whose experiment_budget is within
// 2..MaxD2MultiRoundBudget. It is the explicit tier predicate; IsD1S1 stays a
// pure domain-membership test that never consults the budget (the tier trap:
// track_gain with budget 3 still hits every IsD1S1 domain guard), and
// ValidateD1S1 keeps rejecting budget>1 on its own.
func (a Admission) IsD2MultiRound() bool {
	if _, ok := D1S1DomainSpecFor(a); !ok {
		return false
	}
	return a.ExperimentBudget > 1 && a.ExperimentBudget <= MaxD2MultiRoundBudget
}

// isD1S1SingleRound reports whether the admission runs under the original
// D1-S1 single-round guard set. Budgets above MaxD2MultiRoundBudget fall back
// to the single-round guards, so an unknown tier fails closed instead of
// bypassing the D1-S1 guards.
func (a Admission) isD1S1SingleRound() bool {
	return a.IsD1S1() && !a.IsD2MultiRound()
}

// ValidateD2MultiRound is the multi-round admission variant of ValidateD1S1.
// It reuses the same admitted domain table (D1S1DomainSpecFor), the same
// observation-bound track target, the same per-domain typed-action and dose
// bounds validation, and the same per-scope max_action_attempts==1; only the
// budget differs (2..MaxD2MultiRoundBudget) and the admission must carry the
// experiment baseline fingerprint whose revision anchors cumulative dose
// accounting. The domain-table bounds validators word their errors with the
// D1-S1 prefix because they are the shared domain bounds, not tier policy.
func (a Admission) ValidateD2MultiRound() error {
	if err := a.Validate(); err != nil {
		return err
	}
	domain, ok := D1S1DomainSpecFor(a)
	if !ok {
		return fmt.Errorf("D2-2 action_domain/action_kind must be one of the admitted domains: %s", strings.Join(D1S1AdmittedDomains(), ", "))
	}
	if a.ExperimentBudget <= 1 || a.ExperimentBudget > MaxD2MultiRoundBudget {
		return fmt.Errorf("D2-2 experiment_budget must be within 2-%d", MaxD2MultiRoundBudget)
	}
	if strings.ToLower(mapString(a.TargetRef, "kind")) != "track" || mapString(a.TargetRef, "id", "track_id") == "" {
		return fmt.Errorf("D2-2 target must be an observation-bound track")
	}
	if len(a.BaselineFingerprint) == 0 || strings.TrimSpace(mapString(a.BaselineFingerprint, "revision", "project_revision")) == "" {
		return fmt.Errorf("D2-2 baseline fingerprint with a project revision is required for cumulative dose accounting")
	}
	if domain.ValidateTypedAction != nil {
		if err := domain.ValidateTypedAction(a); err != nil {
			return err
		}
	}
	for name, bounds := range map[string]map[string]any{"diagnostic": a.DiagnosticDoseBounds, "retained": a.RetainedDoseBounds} {
		attempts, ok := mapNumber(bounds, "max_action_attempts")
		if !ok || attempts != 1 {
			return fmt.Errorf("D2-2 %s max_action_attempts must be 1", name)
		}
		if err := domain.ValidateDoseBounds(name, bounds); err != nil {
			return err
		}
	}
	return nil
}

// experimentJudgmentPending implements GLM ruling 3: the human-judgment
// boundary persists at experiment scope. Any earlier round that requested a
// user judgment without the judgment landing (no judgment evidence recorded)
// parks the whole experiment; only a settle-family decision or an explicit
// settlement continues past it. Per-round state bits are not moved.
func (t *Turn) experimentJudgmentPending() bool {
	for _, round := range t.Rounds {
		if round.UserJudgmentRequested && len(round.UserJudgmentEvidence) == 0 {
			return true
		}
	}
	return false
}

// checkD2MultiRoundCumulativeDisplacement enforces GLM ruling 2: the
// experiment-lifetime cumulative displacement relative to the experiment
// baseline never exceeds the domain's single-action absolute bound, while the
// per-action bound itself is untouched. Displacement accumulates in signed dB
// from the receipt chain (each applied intervention's AchievedDelta at the
// domain's value key), so reciprocal moves cancel. One admission moves one
// admission-pinned parameter (for static_eq, the band pinned in its typed
// action), so per-band accumulation coincides with per-experiment
// accumulation inside an admission. Failed or ambiguous attempts spend budget
// but record no trusted displacement and are not accumulated.
func (t *Turn) checkD2MultiRoundCumulativeDisplacement(intervention Intervention) error {
	domain, ok := D1S1DomainSpecFor(t.Admission)
	if !ok {
		// Unreachable through IsD2MultiRound; fail closed if the domain table
		// ever changes under a persisted admission.
		return fmt.Errorf("D2-2 action_domain/action_kind must be one of the admitted domains: %s", strings.Join(D1S1AdmittedDomains(), ", "))
	}
	if intervention.TechnicalApplication != TechnicalApplied {
		return nil
	}
	delta, ok := mapNumber(intervention.AchievedDelta, domain.AdmissionValueKey)
	if !ok {
		return fmt.Errorf("D2-2 applied intervention must record achieved %s for cumulative dose accounting", domain.AdmissionValueKey)
	}
	cumulative := delta
	for _, round := range t.Rounds {
		for _, prior := range round.Interventions {
			if prior.TechnicalApplication != TechnicalApplied {
				continue
			}
			priorDelta, ok := mapNumber(prior.AchievedDelta, domain.AdmissionValueKey)
			if !ok {
				return fmt.Errorf("D2-2 prior applied intervention %s lacks achieved %s; cumulative dose accounting is incomplete", prior.ID, domain.AdmissionValueKey)
			}
			cumulative += priorDelta
		}
	}
	if math.Abs(cumulative) > d2MultiRoundCumulativeDeltaBoundDB {
		return fmt.Errorf("D2-2 cumulative %s displacement %.4g dB exceeds the experiment-lifetime bound %.4g dB; the single-action bound is unchanged", domain.AdmissionValueKey, cumulative, d2MultiRoundCumulativeDeltaBoundDB)
	}
	return nil
}
