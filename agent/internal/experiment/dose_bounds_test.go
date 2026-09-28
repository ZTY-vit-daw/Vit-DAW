package experiment

import (
	"strings"
	"testing"
)

// DOSE-AUDIBLE-1 boundary seals: the audible-priority dose ceiling
// (2026-09-28 user ruling) must hold at its endpoints on every dose axis of
// the domain table — the authority layer both chat and agentloop mirror.

// Every dB-family row accepts the ceiling endpoints and refuses everything
// past them, zero, missing keys, and non-numeric values.
func TestD1S1DBDoseBoundsHoldAudibleCeilingEndpoints(t *testing.T) {
	for _, spec := range D1S1DomainSpecs() {
		key := spec.AdmissionValueKey
		if !strings.HasSuffix(key, "_db") {
			continue
		}
		t.Run(spec.ActionDomain, func(t *testing.T) {
			for _, edge := range []float64{D1S1MaxAbsDeltaDB, -D1S1MaxAbsDeltaDB} {
				if err := spec.ValidateDoseBounds("diagnostic", map[string]any{key: edge}); err != nil {
					t.Fatalf("ceiling endpoint %g refused on %s: %v", edge, key, err)
				}
			}
			for _, invalid := range []map[string]any{
				{key: D1S1MaxAbsDeltaDB + 0.01},
				{key: -D1S1MaxAbsDeltaDB - 0.01},
				{key: 0},
				{key: "loud"},
				{},
			} {
				if err := spec.ValidateDoseBounds("diagnostic", invalid); err == nil {
					t.Fatalf("invalid %s dose accepted: %+v", key, invalid)
				}
			}
		})
	}
}

// The pan row is isomorphic to the dB family: endpoints admitted, beyond,
// zero, missing, and non-numeric refused.
func TestD1S1PanDoseBoundsHoldAudibleCeilingEndpoints(t *testing.T) {
	for _, spec := range D1S1DomainSpecs() {
		if spec.AdmissionValueKey != "delta_pan" {
			continue
		}
		for _, edge := range []float64{D1S1MaxAbsDeltaPan, -D1S1MaxAbsDeltaPan} {
			if err := spec.ValidateDoseBounds("diagnostic", map[string]any{"delta_pan": edge}); err != nil {
				t.Fatalf("pan ceiling endpoint %g refused: %v", edge, err)
			}
		}
		for _, invalid := range []map[string]any{
			{"delta_pan": D1S1MaxAbsDeltaPan + 0.01},
			{"delta_pan": -D1S1MaxAbsDeltaPan - 0.01},
			{"delta_pan": 0},
			{"delta_pan": "left"},
			{},
		} {
			if err := spec.ValidateDoseBounds("diagnostic", invalid); err == nil {
				t.Fatalf("invalid pan dose accepted: %+v", invalid)
			}
		}
	}
}

// Hint and error wording must be generated from the constants, so no layer
// can restate a stale ceiling in prose while the validators move on.
func TestD1S1DoseWordingDerivesFromCeilingConstants(t *testing.T) {
	for _, spec := range D1S1DomainSpecs() {
		if spec.AdmissionValueKey == "delta_pan" {
			if !strings.Contains(spec.PromptParameterHint, D1S1PanDoseHintFragment()) {
				t.Fatalf("pan hint lost the constant-derived bound: %q", spec.PromptParameterHint)
			}
			err := spec.ValidateDoseBounds("diagnostic", map[string]any{"delta_pan": D1S1MaxAbsDeltaPan + 0.01})
			if err == nil || !strings.Contains(err.Error(), D1S1PanDoseRangeText()) {
				t.Fatalf("pan error wording lost the constant-derived bound: %v", err)
			}
			continue
		}
		if !strings.Contains(spec.PromptParameterHint, D1S1DBDoseHintFragment()) {
			t.Fatalf("domain %s hint lost the constant-derived bound: %q", spec.ActionDomain, spec.PromptParameterHint)
		}
		err := spec.ValidateDoseBounds("diagnostic", map[string]any{spec.AdmissionValueKey: D1S1MaxAbsDeltaDB + 0.01})
		if err == nil || !strings.Contains(err.Error(), D1S1DBDoseRangeText()) {
			t.Fatalf("domain %s error wording lost the constant-derived bound: %v", spec.ActionDomain, err)
		}
	}
}

// The D2-2 experiment-lifetime cumulative bound stays derived from the same
// ceiling (the sealed equality in d2_multiround_test.go pins it to the live
// table); widening the single-step ceiling without moving the cumulative
// bound would silently re-tighten every multi-round experiment.
func TestD2MultiRoundCumulativeBoundFollowsAudibleCeiling(t *testing.T) {
	if d2MultiRoundCumulativeDeltaBoundDB != D1S1MaxAbsDeltaDB {
		t.Fatalf("cumulative bound %g drifted from the audible single-step ceiling %g", d2MultiRoundCumulativeDeltaBoundDB, D1S1MaxAbsDeltaDB)
	}
}
