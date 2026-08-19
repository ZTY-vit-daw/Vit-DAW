package com

import (
	"testing"
)

func transientFixture() *TransientEventEvidence {
	return &TransientEventEvidence{
		Status:       StatusReady,
		Coverage:     0.9,
		WindowMS:     85.33,
		HopMS:        85.33,
		EvidenceRefs: []string{"dad.l3.transient_events"},
		Events: []TransientEvent{
			{OnsetSeconds: 1.0, BodyEndSeconds: 1.08, SustainEndSeconds: 1.35, OnsetDBFS: number(-8), BodyDBFS: number(-15), SustainDBFS: number(-20), AttackBodyContrastDB: number(7), SustainDecayDB: number(5)},
			{OnsetSeconds: 2.5, BodyEndSeconds: 2.58, SustainEndSeconds: 2.85, OnsetDBFS: number(-10), BodyDBFS: number(-17), SustainDBFS: number(-22), AttackBodyContrastDB: number(7), SustainDecayDB: number(5)},
			{OnsetSeconds: 4.0, BodyEndSeconds: 4.08, SustainEndSeconds: 4.35, OnsetDBFS: number(-6), BodyDBFS: number(-14), SustainDBFS: number(-19), AttackBodyContrastDB: number(8), SustainDecayDB: number(5)},
		},
	}
}

func TestSourceOnlyMicroTransientFromFrameEvidence(t *testing.T) {
	input := readySourceInput()
	input.Source.TransientEvents = transientFixture()
	projection := Build(input)
	dynamics := projection.SourceDynamics
	if dynamics == nil {
		t.Fatal("source dynamics missing")
	}
	scale := timeScale(dynamics.TimeScaleCoverage, ScaleMicroTransient)
	if scale.Status != StatusPartial {
		t.Fatalf("micro transient scale = %q, want partial (frame-level resolution): %+v", scale.Status, dynamics.TimeScaleCoverage)
	}
	if scale.SegmentCount != 3 || scale.WindowMS != 85.33 || scale.HopMS != 85.33 {
		t.Fatalf("micro transient scale resolution not declared honestly: %+v", scale)
	}
	if scale.Reason != "frame_level_fft_transient_resolution" {
		t.Fatalf("micro transient reason = %q", scale.Reason)
	}
	events := dynamics.Events
	if events.EventCount != 3 {
		t.Fatalf("event count = %d, want 3", events.EventCount)
	}
	if events.EventDensityPerSecond == nil || *events.EventDensityPerSecond < 0.14 || *events.EventDensityPerSecond > 0.16 {
		t.Fatalf("event density = %+v", events.EventDensityPerSecond)
	}
	if events.InterEventIntervalMS == nil || events.InterEventIntervalMS.Count != 2 {
		t.Fatalf("interval distribution = %+v", events.InterEventIntervalMS)
	}
	if events.TransientContrastDB == nil || events.TransientContrastDB.Count != 3 || events.TransientContrastDB.Min != 7 {
		t.Fatalf("contrast distribution = %+v", events.TransientContrastDB)
	}
	if !contains(dynamics.EvidenceRefs, "dad.l3.transient_events") {
		t.Fatalf("transient evidence refs missing: %+v", dynamics.EvidenceRefs)
	}
	// Source-only behavior authority is unchanged: transient statistics are
	// descriptive, never compressor-behavior conclusions.
	if projection.GainAction != nil || projection.TransientResponse != nil {
		t.Fatalf("source-only promoted behavior from transient statistics")
	}
	if projection.TrustQuality.CanSupportBehaviorObservation {
		t.Fatalf("source-only transient evidence granted behavior observation")
	}
}

func TestSourceOnlyMicroTransientMissingWithoutEvidence(t *testing.T) {
	input := readySourceInput()
	projection := Build(input)
	scale := timeScale(projection.SourceDynamics.TimeScaleCoverage, ScaleMicroTransient)
	if scale.Status != StatusMissing {
		t.Fatalf("micro transient without evidence = %q, want missing", scale.Status)
	}
	if projection.SourceDynamics.Events.EventCount != 0 {
		t.Fatalf("events fabricated without evidence: %+v", projection.SourceDynamics.Events)
	}
}

func TestSourceOnlyMacroStatusUnaffectedByTransientEvidence(t *testing.T) {
	input := readySourceInput()
	input.Source.TransientEvents = transientFixture()
	projection := Build(input)
	if projection.Status != StatusReady || projection.SourceDynamics.Status != StatusReady {
		t.Fatalf("macro readiness changed by transient evidence: %+v", projection)
	}
	scale := timeScale(projection.SourceDynamics.TimeScaleCoverage, ScaleMacroProgram)
	if scale.Status != StatusReady || scale.SegmentCount != 4 {
		t.Fatalf("macro scale changed: %+v", scale)
	}
}

func TestSourceOnlyMicroTransientStableAcrossOrdering(t *testing.T) {
	left := readySourceInput()
	left.Source.TransientEvents = transientFixture()
	right := readySourceInput()
	right.Source.TransientEvents = &TransientEventEvidence{
		Status:       StatusReady,
		Coverage:     0.9,
		WindowMS:     85.33,
		HopMS:        85.33,
		EvidenceRefs: []string{"dad.l3.transient_events", "dad.l3.transient_events"},
		Events: []TransientEvent{
			{OnsetSeconds: 1.0, BodyEndSeconds: 1.08, SustainEndSeconds: 1.35, OnsetDBFS: number(-8), BodyDBFS: number(-15), SustainDBFS: number(-20), AttackBodyContrastDB: number(7), SustainDecayDB: number(5)},
			{OnsetSeconds: 4.0, BodyEndSeconds: 4.08, SustainEndSeconds: 4.35, OnsetDBFS: number(-6), BodyDBFS: number(-14), SustainDBFS: number(-19), AttackBodyContrastDB: number(8), SustainDecayDB: number(5)},
			{OnsetSeconds: 2.5, BodyEndSeconds: 2.58, SustainEndSeconds: 2.85, OnsetDBFS: number(-10), BodyDBFS: number(-17), SustainDBFS: number(-22), AttackBodyContrastDB: number(7), SustainDecayDB: number(5)},
		},
	}
	if !sameSourceDynamics(Build(left).SourceDynamics, Build(right).SourceDynamics) {
		t.Fatal("transient-derived dynamics differ under ordering")
	}
}

func sameSourceDynamics(left, right *SourceDynamics) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Events.EventCount != right.Events.EventCount {
		return false
	}
	if (left.Events.InterEventIntervalMS == nil) != (right.Events.InterEventIntervalMS == nil) {
		return false
	}
	if left.Events.InterEventIntervalMS != nil && left.Events.InterEventIntervalMS.Count != right.Events.InterEventIntervalMS.Count {
		return false
	}
	return true
}
