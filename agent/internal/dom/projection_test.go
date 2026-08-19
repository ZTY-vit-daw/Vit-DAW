package dom

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceOnlyBuildsNeutralBoundedProjection(t *testing.T) {
	projection := Build(domTestInput())
	if projection.SchemaVersion != SchemaVersion || projection.DOMVersion != Version || projection.Mode != ModeSourceOnly {
		t.Fatalf("projection identity = %+v", projection)
	}
	if projection.Status != StatusPartial {
		t.Fatalf("status = %q", projection.Status)
	}
	if projection.ProcessorScope != nil {
		t.Fatalf("source-only projection disclosed processor scope: %+v", projection.ProcessorScope)
	}
	if projection.PeakStructure == nil || projection.PeakStructure.Status != StatusReady || projection.PeakStructure.PeakDBFS == nil {
		t.Fatalf("peak structure = %+v", projection.PeakStructure)
	}
	if projection.ActivityStructure == nil || projection.ActivityStructure.Status != StatusPartial || projection.ActivityStructure.NoiseFloorStatus != StatusMissing || projection.ActivityStructure.LowEnergyCount != 1 {
		t.Fatalf("activity structure without noise floor must stay partial: %+v", projection.ActivityStructure)
	}
	if projection.FrequencyTimeEvents == nil || projection.FrequencyTimeEvents.Status != StatusPartial || projection.FrequencyTimeEvents.TimeLocalized {
		t.Fatalf("frequency-time structure = %+v", projection.FrequencyTimeEvents)
	}
	if projection.TransientStructure == nil || projection.TransientStructure.Status != StatusPartial || projection.TransientStructure.OnsetEventsReady {
		t.Fatalf("transient structure = %+v", projection.TransientStructure)
	}
	if projection.BandDynamics == nil || projection.BandDynamics.Status != StatusPartial || projection.BandDynamics.TimeVaryingReady {
		t.Fatalf("band dynamics = %+v", projection.BandDynamics)
	}
	if !projection.TrustQuality.CanSupportFamilySelection || projection.TrustQuality.CanSupportBehaviorObservation || projection.TrustQuality.CanSupportPostActionEvaluation {
		t.Fatalf("trust boundary = %+v", projection.TrustQuality)
	}
	data, _ := json.Marshal(projection)
	text := string(data)
	for _, forbidden := range []string{
		"processor_family", "plugin_instance_id", "parameter_id", "recommended_processor",
		"limiter", "gate_threshold", "de_esser", "sibilance", "transient_shaper", "multiband_dynamics", "clipper",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("source-only projection leaked %q: %s", forbidden, text)
		}
	}
}

func TestSourceOnlyProjectionIDIsStable(t *testing.T) {
	input := domTestInput()
	first := Build(input)
	input.CreatedAt = "2027-01-01T00:00:00Z"
	second := Build(input)
	if first.ProjectionID == "" || first.ProjectionID != second.ProjectionID {
		t.Fatalf("projection ids = %q %q", first.ProjectionID, second.ProjectionID)
	}
}

func TestContextProjectionOmitsRawAndBoundIdentity(t *testing.T) {
	projection := Build(domTestInput())
	data, _ := json.Marshal(ContextProjection(projection))
	text := string(data)
	for _, forbidden := range []string{"time_segments", "raw_samples", "plugin_instance_id", "topology_generation", "processor_state_hash"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("context projection leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"do_not_include_raw_package":true`) {
		t.Fatalf("raw package guard missing: %s", text)
	}
}

func TestReservedBehaviorModesFailClosed(t *testing.T) {
	input := domTestInput()
	input.Mode = ModePairedIO
	input.ProcessorScope = &ProcessorScope{Family: FamilyLimiter, TrackID: "track-1", PluginInstanceID: "plugin-1"}
	paired := Build(input)
	if paired.Status != StatusMissing || paired.TrustQuality.CanSupportBehaviorObservation {
		t.Fatalf("reserved paired mode widened: %+v", paired)
	}
	if paired.ProcessorScope == nil || paired.ProcessorScope.Family != FamilyLimiter {
		t.Fatalf("bound family missing after selection: %+v", paired.ProcessorScope)
	}

	input.ProcessorScope.Family = "compressor"
	unsupported := Build(input)
	if unsupported.Status != StatusUnsupported || unsupported.TrustQuality.CanSupportBehaviorObservation {
		t.Fatalf("COM-owned family entered DOM: %+v", unsupported)
	}
}

func TestSourceOnlyUsesBoundedFineEvidenceWithoutRoutingSemantics(t *testing.T) {
	input := domTestInput()
	noise := -52.0
	p10 := -55.0
	p50 := -48.0
	level := -18.0
	contrast := 8.0
	onset := -9.0
	body := -16.0
	sustain := -20.0
	attackBody := 7.0
	decay := 4.0
	input.Source.NoiseFloor = &NoiseFloorEvidence{Status: StatusReady, EstimateDBFS: &noise, P10DBFS: &p10, P50DBFS: &p50, Method: "bounded_rms_percentile_100ms", Confidence: "medium", WindowCount: 20}
	input.Source.Frequency = &FrequencyEventEvidence{Status: StatusReady, Coverage: 1, EventCountAvailable: true, Events: []FrequencyEvent{{StartSeconds: 1, EndSeconds: 1.02, BandID: "presence_high", MinHz: 4500, MaxHz: 11000, LevelDBFS: &level, ContrastDB: &contrast}}}
	input.Source.Transient = &TransientEventEvidence{Status: StatusReady, Coverage: 1, Events: []TransientEvent{{OnsetSeconds: 1, BodyEndSeconds: 1.08, SustainEndSeconds: 1.35, OnsetDBFS: &onset, BodyDBFS: &body, SustainDBFS: &sustain, AttackBodyContrastDB: &attackBody, SustainDecayDB: &decay}}}
	input.Source.BandDynamicsEvidence = []BandDynamicsEvidence{
		{ID: "bass", Status: StatusReady, TimeDistribution: &Distribution{Count: 20, Min: -40, P50: -22, P90: -10, Max: -5}, CrestDistribution: &Distribution{Count: 20, Min: 8, P50: 12, P90: 16, Max: 20}},
		{ID: "mid", Status: StatusReady, TimeDistribution: &Distribution{Count: 20, Min: -38, P50: -24, P90: -14, Max: -7}, CrestDistribution: &Distribution{Count: 20, Min: 7, P50: 11, P90: 15, Max: 19}},
		{ID: "presence", Status: StatusReady, TimeDistribution: &Distribution{Count: 20, Min: -45, P50: -30, P90: -18, Max: -9}, CrestDistribution: &Distribution{Count: 20, Min: 6, P50: 10, P90: 14, Max: 18}},
	}
	projection := Build(input)
	if projection.Status != StatusReady {
		t.Fatalf("complete bounded evidence remained non-ready: %+v", projection)
	}
	if projection.ActivityStructure.NoiseFloorStatus != StatusReady || projection.FrequencyTimeEvents.Status != StatusReady || !projection.FrequencyTimeEvents.TimeLocalized || projection.TransientStructure.Status != StatusReady || !projection.TransientStructure.AttackBodyReady || !projection.TransientStructure.SustainDecayReady || projection.BandDynamics.Status != StatusReady || !projection.BandDynamics.CrossBandReady {
		t.Fatalf("fine evidence readiness = activity=%+v frequency=%+v transient=%+v bands=%+v", projection.ActivityStructure, projection.FrequencyTimeEvents, projection.TransientStructure, projection.BandDynamics)
	}
	data, _ := json.Marshal(projection)
	text := strings.ToLower(string(data))
	for _, forbidden := range []string{"processor_family", "plugin_instance_id", "parameter_id", "limiter", "de_esser", "transient_shaper", "multiband_dynamics"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("fine evidence introduced routing identity %q: %s", forbidden, text)
		}
	}
}

func TestFineEvidenceStatusCannotWidenWithoutBoundedFacts(t *testing.T) {
	input := domTestInput()
	input.Source.NoiseFloor = &NoiseFloorEvidence{Status: StatusReady}
	input.Source.Frequency = &FrequencyEventEvidence{Status: StatusReady, Coverage: 1}
	input.Source.Transient = &TransientEventEvidence{Status: StatusReady}
	input.Source.BandDynamicsEvidence = []BandDynamicsEvidence{{ID: "bass", Status: StatusReady}}
	projection := Build(input)
	if projection.ActivityStructure.NoiseFloorStatus == StatusReady || projection.FrequencyTimeEvents.Status == StatusReady || projection.TransientStructure.Status == StatusReady || projection.BandDynamics.Status == StatusReady {
		t.Fatalf("status-only fine evidence widened readiness: activity=%+v frequency=%+v transient=%+v bands=%+v", projection.ActivityStructure, projection.FrequencyTimeEvents, projection.TransientStructure, projection.BandDynamics)
	}
}

func domTestInput() Input {
	rms, peak, headroom, crest := -20.0, -1.0, 1.0, 12.0
	lowRMS, lowPeak, lowCrest := -46.0, -30.0, 16.0
	activeRMS, activePeak, activeCrest := -18.0, -2.0, 16.0
	bass, presence := .2, .35
	bassDB, presenceDB := -14.0, -9.1
	return Input{
		Mode:          ModeSourceOnly,
		ObservationID: "obs-1",
		MixSessionID:  "mix-1",
		CreatedAt:     "2026-08-08T00:00:00Z",
		TargetRef:     map[string]any{"kind": "track", "id": "track-1"},
		Conditions: Conditions{SourceRevision: "source-1", ClipRevision: "clip-1", StartSample: 0, EndSample: 192000,
			StartSeconds: 0, EndSeconds: 4, SampleRate: 48000, ChannelCount: 2, AnalyzerVersion: "fixture-v1"},
		Source: SourceEvidence{
			SchemaVersion: "dad.dynamic_source_evidence.v1", Status: StatusReady, Freshness: "fresh", Duration: 4,
			RMSDBFS: &rms, PeakDBFS: &peak, HeadroomDB: &headroom, CrestDB: &crest,
			TimeSegments: []TimeSegment{
				{StartSeconds: 0, EndSeconds: 2, RMSDBFS: &activeRMS, PeakDBFS: &activePeak, CrestDB: &activeCrest, EnergyState: "active"},
				{StartSeconds: 2, EndSeconds: 4, RMSDBFS: &lowRMS, PeakDBFS: &lowPeak, CrestDB: &lowCrest, EnergyState: "low"},
			},
			Bands: []BandEvidence{
				{ID: "bass", Status: StatusReady, MinHz: 60, MaxHz: 160, UnitEnergy: &bass, EnergyDB: &bassDB},
				{ID: "mid", Status: StatusReady, MinHz: 500, MaxHz: 2000},
				{ID: "presence", Status: StatusReady, MinHz: 2000, MaxHz: 6000, UnitEnergy: &presence, EnergyDB: &presenceDB},
			},
			Quality: Quality{Status: StatusReady, Nonzero: true, Coverage: 1}, EvidenceRefs: []string{"dad.waveform:one", "dad.band:one"},
		},
	}
}

func TestFrequencyEventsOutsideDeclaredWindowStayPartial(t *testing.T) {
	input := domTestInput()
	level, contrast := -18.0, 8.0
	input.Source.Frequency = &FrequencyEventEvidence{Status: StatusReady, Coverage: 1, EventCountAvailable: true, Events: []FrequencyEvent{{StartSeconds: 5, EndSeconds: 5.2, BandID: "presence_high", LevelDBFS: &level, ContrastDB: &contrast}}}
	projection := Build(input)
	if projection.FrequencyTimeEvents.Status != StatusPartial || !projection.FrequencyTimeEvents.TimeLocalized {
		t.Fatalf("events outside the declared window were promoted: %+v", projection.FrequencyTimeEvents)
	}
}

func TestIncompleteTransientEventsStayPartial(t *testing.T) {
	input := domTestInput()
	onset, body := -9.0, -16.0
	input.Source.Transient = &TransientEventEvidence{Status: StatusReady, Coverage: 1, Events: []TransientEvent{{OnsetSeconds: 1, BodyEndSeconds: 1.08, SustainEndSeconds: 1.35, OnsetDBFS: &onset, BodyDBFS: &body}}}
	projection := Build(input)
	if projection.TransientStructure.Status != StatusPartial || projection.TransientStructure.OnsetEventsReady || projection.TransientStructure.AttackBodyReady || projection.TransientStructure.SustainDecayReady {
		t.Fatalf("incomplete transient facts were promoted: %+v", projection.TransientStructure)
	}
}

func TestBandDynamicsMissingDeclaredBandStaysPartial(t *testing.T) {
	input := domTestInput()
	input.Source.BandDynamicsEvidence = []BandDynamicsEvidence{{ID: "bass", Status: StatusReady, TimeDistribution: &Distribution{Count: 20, Min: -40, P50: -22, P90: -10, Max: -5}, CrestDistribution: &Distribution{Count: 20, Min: 8, P50: 12, P90: 16, Max: 20}}}
	projection := Build(input)
	if projection.BandDynamics.Status != StatusPartial || projection.BandDynamics.CrossBandReady {
		t.Fatalf("missing declared band was promoted: %+v", projection.BandDynamics)
	}
}

func TestMeasurementConditionsKeyIsStableAndSensitive(t *testing.T) {
	first := Build(domTestInput())
	second := Build(domTestInput())
	if first.Conditions.MeasurementKey == "" || first.Conditions.MeasurementKey != second.Conditions.MeasurementKey {
		t.Fatalf("measurement key was not stable: %q %q", first.Conditions.MeasurementKey, second.Conditions.MeasurementKey)
	}
	changed := domTestInput()
	changed.Conditions.TapPoint = "track_post_fader"
	third := Build(changed)
	if third.Conditions.MeasurementKey == first.Conditions.MeasurementKey {
		t.Fatalf("measurement key ignored tap point: %q", third.Conditions.MeasurementKey)
	}
}

func TestStaleOrSuspectSourceEvidenceCannotSupportFamilySelection(t *testing.T) {
	for _, status := range []string{StatusStale, StatusSuspect} {
		input := domTestInput()
		input.Source.Status = status
		projection := Build(input)
		if projection.TrustQuality.CanSupportFamilySelection || projection.Status == StatusReady {
			t.Fatalf("status %q was promoted: %+v", status, projection.TrustQuality)
		}
	}
}
