package experiment

import (
	"fmt"
	"math"
	"strings"
)

// D1S1DomainSpec describes one action domain admitted by the narrow bounded
// experiment gate (Phase D1-S1 and its D2-1 extension). Each domain carries
// its own equally tight parameter bounds; admitting another domain never
// relaxes the shared invariants (budget 1, one round, one forward mutation,
// observation-bound track target).
type D1S1DomainSpec struct {
	ActionDomain string
	ActionKind string
	// PromptParameterHint is the model-facing description of this domain's
	// parameter_bounds shape and absolute bounds. It is derived wording only:
	// admitting a domain or editing this hint never changes what
	// ValidateTypedAction/ValidateDoseBounds enforce.
	PromptParameterHint string
	// ValidateTypedAction checks the domain-specific typed action parameters.
	ValidateTypedAction func(a Admission) error
	// ValidateDoseBounds checks one dose-bounds map ("diagnostic" or
	// "retained") against the domain's acoustic parameter bounds.
	ValidateDoseBounds func(scope string, bounds map[string]any) error
}

var d1s1Domains = []D1S1DomainSpec{
	{
		ActionDomain: D1S1ActionDomain,
		ActionKind:   D1S1ActionKind,
		PromptParameterHint: `parameter_bounds={"delta_db":<nonzero number within +/-2>}`,
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			delta, ok := mapNumber(bounds, "delta_db")
			if !ok || delta == 0 || math.Abs(delta) > 2 {
				return fmt.Errorf("D1-S1 %s delta_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
	},
	{
		// static_eq adjusts one EQ band of one track. D2-1 admits it with the
		// same single-mutation tightness as track_gain: one band, one bounded
		// gain move, no frequency outside the audible range, no resonant Q.
		ActionDomain: "static_eq",
		ActionKind:   "static_eq_band_adjust",
		PromptParameterHint: `parameter_bounds={"gain_db":<nonzero number within +/-2>,"frequency_hz":<number within 20-20000>,"q":<optional number within 0.1-18>,"band_index":<optional non-negative integer>}`,
		ValidateTypedAction: func(a Admission) error {
			frequency, ok := mapNumber(a.TypedAction, "frequency_hz")
			if !ok || frequency < 20 || frequency > 20000 {
				return fmt.Errorf("D1-S1 static_eq frequency_hz must be within 20-20000 Hz")
			}
			if index, present := a.TypedAction["band_index"]; present {
				parsed, ok := mapNumber(map[string]any{"band_index": index}, "band_index")
				if !ok || parsed < 0 || parsed != math.Trunc(parsed) {
					return fmt.Errorf("D1-S1 static_eq band_index must be a non-negative integer")
				}
			}
			if value, present := a.TypedAction["q"]; present {
				parsed, ok := mapNumber(map[string]any{"q": value}, "q")
				if !ok || parsed < 0.1 || parsed > 18 {
					return fmt.Errorf("D1-S1 static_eq q must be within 0.1-18")
				}
			}
			return nil
		},
		ValidateDoseBounds: func(scope string, bounds map[string]any) error {
			gain, ok := mapNumber(bounds, "gain_db")
			if !ok || gain == 0 || math.Abs(gain) > 2 {
				return fmt.Errorf("D1-S1 %s gain_db must be non-zero and within +/-2 dB", scope)
			}
			return nil
		},
	},
}

// D1S1AdmittedDomains lists the action domains admitted by the bounded
// experiment gate, in registration order.
func D1S1AdmittedDomains() []string {
	out := make([]string, 0, len(d1s1Domains))
	for _, domain := range d1s1Domains {
		out = append(out, domain.ActionDomain)
	}
	return out
}

// D1S1DomainSpecs returns a copy of the admitted domain specs in registration
// order for read-only consumers (the model prompt layer derives its domain
// wording from this table; the validators remain the enforcement authority).
func D1S1DomainSpecs() []D1S1DomainSpec {
	return append([]D1S1DomainSpec(nil), d1s1Domains...)
}

// D1S1DomainSpecFor resolves the admitted domain spec for an admission's
// typed action. A domain must match on both action_domain and action_kind so
// a hybrid action cannot borrow a domain's bounds.
func D1S1DomainSpecFor(a Admission) (D1S1DomainSpec, bool) {
	domainValue := strings.ToLower(mapString(a.TypedAction, "action_domain", "domain"))
	kindValue := strings.ToLower(mapString(a.TypedAction, "action_kind", "kind"))
	for _, domain := range d1s1Domains {
		if domainValue == domain.ActionDomain && kindValue == domain.ActionKind {
			return domain, true
		}
	}
	return D1S1DomainSpec{}, false
}
