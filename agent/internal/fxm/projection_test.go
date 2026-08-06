package fxm

import "testing"

func number(value float64) *float64 { return &value }
func integer(value int) *int        { return &value }

func TestBuildProducesDeterministicChainEffectProjection(t *testing.T) {
	window := MeasurementWindow{SourceRevision: "source-1", StartSeconds: 0, EndSeconds: 10, SampleRate: 48000, ChannelCount: 2}
	quality := QualityEvidence{Deterministic: true, LatencyCompensated: true, Nonzero: true, Coverage: 1}
	projection := Build(Input{
		ObservationID: "obs-1", MixSessionID: "mix-1", CreatedAt: "2026-07-17T00:00:00Z",
		TargetRef: map[string]any{"kind": "track", "id": "1007"},
		Chain:     ChainIdentity{ChainHash: "chain-1", Plugins: []PluginIdentity{{Order: 1, Name: "TDR Nova", ProbeID: "probe-nova"}}}, Conditions: window,
		Baseline:  Measurement{ID: "baseline-1", Stage: "bypass_chain", Status: "ready", SourceRevision: "source-1", Window: window, RMSDBFS: number(-18), PeakDBFS: number(-6), LatencySamples: integer(0), Bands: []BandMetric{{ID: "presence", MinHz: 2000, MaxHz: 6000, EnergyDB: -22}}, Quality: quality, EvidenceRefs: []string{"render:baseline"}},
		Processed: Measurement{ID: "processed-1", Stage: "processed_chain", Status: "ready", SourceRevision: "source-1", Window: window, RMSDBFS: number(-16.5), PeakDBFS: number(-4), LatencySamples: integer(0), Bands: []BandMetric{{ID: "presence", MinHz: 2000, MaxHz: 6000, EnergyDB: -19.4}}, Quality: quality, EvidenceRefs: []string{"render:processed"}},
		ProbeRefs: []string{"probe:probe-nova"},
	})
	if projection.Status != StatusReady || !projection.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("projection not ready: %+v", projection)
	}
	if projection.EffectDelta.RMSDB == nil || *projection.EffectDelta.RMSDB != 1.5 || projection.EffectDelta.PeakDB == nil || *projection.EffectDelta.PeakDB != 2 {
		t.Fatalf("level delta = %+v", projection.EffectDelta)
	}
	if len(projection.EffectDelta.Bands) != 1 || projection.EffectDelta.Bands[0].DeltaDB != 2.6 {
		t.Fatalf("band delta = %+v", projection.EffectDelta.Bands)
	}
	if projection.ProjectionID == "" || !projection.LLMContext.DoNotIncludeRawPackage {
		t.Fatalf("projection identity/context = %+v", projection)
	}
}

func TestBuildFailsClosedOnMismatchedCounterfactual(t *testing.T) {
	quality := QualityEvidence{Deterministic: true, LatencyCompensated: true, Nonzero: true, Coverage: 1}
	projection := Build(Input{
		Baseline:  Measurement{ID: "a", Stage: "bypass_chain", Status: "ready", SourceRevision: "source-a", Window: MeasurementWindow{StartSeconds: 0, EndSeconds: 10, SampleRate: 48000, ChannelCount: 2}, RMSDBFS: number(-18), Quality: quality},
		Processed: Measurement{ID: "b", Stage: "processed_chain", Status: "ready", SourceRevision: "source-b", Window: MeasurementWindow{StartSeconds: 1, EndSeconds: 11, SampleRate: 44100, ChannelCount: 2}, RMSDBFS: number(-16), Quality: quality},
	})
	if projection.Status != StatusStale || projection.TrustQuality.CanSupportSuggestion || projection.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("mismatched projection should fail closed: %+v", projection.TrustQuality)
	}
}
