package com

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSourceOnlyReadyMacroProjection(t *testing.T) {
	projection := Build(readySourceInput())
	if projection.SchemaVersion != SchemaVersion || projection.COMVersion != Version || projection.Mode != ModeSourceOnly {
		t.Fatalf("projection identity = %+v", projection)
	}
	if projection.Status != StatusReady || projection.SourceDynamics == nil || projection.SourceDynamics.Status != StatusReady {
		t.Fatalf("source-only projection not ready: %+v", projection)
	}
	if projection.SourceDynamics.DeclaredScale != ScaleMacroProgram || projection.SourceDynamics.Activity.ValidSegmentCount != 4 {
		t.Fatalf("macro source projection = %+v", projection.SourceDynamics)
	}
	if projection.SourceDynamics.Activity.ActiveSegmentCount != 3 || projection.SourceDynamics.Activity.SilentSegmentCount != 1 {
		t.Fatalf("activity = %+v", projection.SourceDynamics.Activity)
	}
	if projection.SourceDynamics.MacroDynamics.RMSRangeDB == nil || *projection.SourceDynamics.MacroDynamics.RMSRangeDB != 6 {
		t.Fatalf("macro range = %+v", projection.SourceDynamics.MacroDynamics)
	}
	if !projection.TrustQuality.CanSupportSourceDescription {
		t.Fatalf("ready source cannot support description: %+v", projection.TrustQuality)
	}
	if projection.TrustQuality.CanSupportBehaviorObservation || projection.TrustQuality.CanSupportSemanticPlanning || projection.TrustQuality.CanSupportPostActionEvaluation {
		t.Fatalf("source-only promoted compressor authority: %+v", projection.TrustQuality)
	}
	for _, behavior := range []*BehaviorProjection{
		projection.GainAction, projection.TransientResponse, projection.RecoveryMotion,
		projection.LevelEffect, projection.StereoBehavior, projection.TriggerRelation,
	} {
		if behavior != nil {
			t.Fatalf("source-only populated behavior projection: %+v", behavior)
		}
	}
	for _, dimension := range projection.Identifiability.Dimensions {
		if dimension.Dimension == DimensionSourceDynamics {
			if dimension.Status != Identified || dimension.Resolution.WindowMS != 5000 || dimension.Resolution.HopMS != 5000 {
				t.Fatalf("source identifiability = %+v", dimension)
			}
			continue
		}
		if dimension.Status != NotIdentifiable || dimension.Confidence != 0 {
			t.Fatalf("behavior dimension was promoted: %+v", dimension)
		}
	}
	if len(projection.LLMContext.CompactFacts) > maxCompactFacts || !projection.LLMContext.DoNotIncludeRawPackage {
		t.Fatalf("context contract = %+v", projection.LLMContext)
	}
	if projection.LLMContext.CompactFacts[0]["layer"] != "trust_quality" || projection.LLMContext.CompactFacts[1]["layer"] != "identifiability" {
		t.Fatalf("trust/identifiability must lead context: %+v", projection.LLMContext.CompactFacts)
	}
}

func TestBuildSourceOnlyStableAcrossNonSemanticOrderingAndGeneratedTime(t *testing.T) {
	first := readySourceInput()
	second := readySourceInput()
	second.CreatedAt = "2027-01-01T00:00:00Z"
	reverseSegments(second.Source.TimeSegments)
	second.Source.EvidenceRefs = []string{"dad:source:2", "dad:source:1", "dad:source:1"}
	second.TargetRef = map[string]any{"label": "Vocal", "id": "track-1", "kind": "track"}

	left := Build(first)
	right := Build(second)
	if left.ProjectionID != right.ProjectionID {
		t.Fatalf("stable IDs differ: %s != %s", left.ProjectionID, right.ProjectionID)
	}
	if !reflect.DeepEqual(left.SourceDynamics, right.SourceDynamics) {
		t.Fatalf("normalized source projections differ:\nleft=%+v\nright=%+v", left.SourceDynamics, right.SourceDynamics)
	}
}

func TestBuildScalarOnlyIsPartialWithoutInventedTimeBehavior(t *testing.T) {
	input := readySourceInput()
	input.Source.TimeSegments = nil
	projection := Build(input)
	if projection.Status != StatusPartial || projection.SourceDynamics.DeclaredScale != ScaleWholeWindow {
		t.Fatalf("scalar-only status = %s source=%+v", projection.Status, projection.SourceDynamics)
	}
	macro := timeScale(projection.SourceDynamics.TimeScaleCoverage, ScaleMacroProgram)
	if macro.Status != StatusMissing || macro.WindowMS != 0 || macro.HopMS != 0 {
		t.Fatalf("missing macro evidence was approximated: %+v", macro)
	}
	for _, dimension := range projection.Identifiability.Dimensions {
		if dimension.Dimension != DimensionSourceDynamics && dimension.Status != NotIdentifiable {
			t.Fatalf("scalar-only behavior promoted: %+v", dimension)
		}
	}
}

func TestBuildSourceOnlyStatusPropagation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
		want   string
	}{
		{name: "stale", mutate: func(in *Input) { in.Source.Status = StatusStale }, want: StatusStale},
		{name: "freshness stale", mutate: func(in *Input) { in.Source.Freshness = StatusStale }, want: StatusStale},
		{name: "suspect", mutate: func(in *Input) { in.Source.Quality.NaNInfCount = 1 }, want: StatusSuspect},
		{name: "direct nan", mutate: func(in *Input) { in.Source.TimeSegments[0].RMSDBFS = number(math.NaN()) }, want: StatusSuspect},
		{name: "approximate", mutate: func(in *Input) { in.Source.Status = StatusApproximate }, want: StatusApproximate},
		{name: "missing", mutate: func(in *Input) {
			in.Source.Status = StatusMissing
			in.Source.RMSDBFS, in.Source.ActiveRMSDBFS, in.Source.PeakDBFS, in.Source.CrestDB = nil, nil, nil, nil
			in.Source.TimeSegments = nil
		}, want: StatusMissing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := readySourceInput()
			test.mutate(&input)
			projection := Build(input)
			if projection.Status != test.want || projection.SourceDynamics.Status != test.want {
				t.Fatalf("status = %s/%s, want %s", projection.Status, projection.SourceDynamics.Status, test.want)
			}
		})
	}
}

func TestBuildSilentSourceDoesNotInventEventsOrBehavior(t *testing.T) {
	input := readySourceInput()
	input.Source.Quality.Nonzero = false
	input.Source.RMSDBFS = number(-120)
	input.Source.PeakDBFS = number(-110)
	input.Source.CrestDB = nil
	for index := range input.Source.TimeSegments {
		input.Source.TimeSegments[index].EnergyState = "silent"
		input.Source.TimeSegments[index].RMSDBFS = number(-120)
		input.Source.TimeSegments[index].PeakDBFS = number(-110)
		input.Source.TimeSegments[index].CrestDB = nil
	}
	projection := Build(input)
	if projection.Status != StatusPartial || projection.SourceDynamics.Activity.ActiveSegmentCount != 0 || projection.SourceDynamics.Activity.ActiveRatio != 0 {
		t.Fatalf("silent source = status %s activity %+v", projection.Status, projection.SourceDynamics.Activity)
	}
	if projection.SourceDynamics.MacroDynamics.RMSDistribution != nil || projection.SourceDynamics.MacroDynamics.RMSRangeDB != nil {
		t.Fatalf("silent segments produced active distribution: %+v", projection.SourceDynamics.MacroDynamics)
	}
	if projection.TrustQuality.CanSupportBehaviorObservation {
		t.Fatal("silent source promoted behavior observation")
	}
}

func TestBuildRejectsUnsupportedModesWithoutDroppingSourceFacts(t *testing.T) {
	for _, mode := range []string{"unknown_future_mode"} {
		input := readySourceInput()
		input.Mode = mode
		projection := Build(input)
		if projection.Status != StatusMissing || projection.SourceDynamics == nil || projection.SourceDynamics.Status != StatusReady {
			t.Fatalf("mode %s boundary = projection %s source %+v", mode, projection.Status, projection.SourceDynamics)
		}
		if !contains(projection.TrustQuality.BlockedReasons, "com_1_supports_source_only_mode") {
			t.Fatalf("mode %s missing boundary reason: %+v", mode, projection.TrustQuality)
		}
	}
}

func TestMissingSampleFormatDowngradesReadySource(t *testing.T) {
	input := readySourceInput()
	input.Conditions.SampleRate = 0
	input.Conditions.ChannelCount = 0
	projection := Build(input)
	if projection.Status != StatusPartial || projection.TrustQuality.SameFormat {
		t.Fatalf("missing sample format = status %s trust %+v", projection.Status, projection.TrustQuality)
	}
	if !contains(projection.TrustQuality.BlockedReasons, "sample_format_missing") {
		t.Fatalf("sample-format gate missing: %+v", projection.TrustQuality.ModeGates)
	}
}

func TestContextProjectionRecursivelyExcludesRawEvidenceAndPaths(t *testing.T) {
	input := readySourceInput()
	input.TargetRef = map[string]any{
		"kind": "track", "id": "track-1", "file_path": `D:\audio\vocal.wav`,
		"nested": map[string]any{"render_path": `/tmp/render.wav`, "safe": "lead vocal"},
		"notes":  []any{"safe note", `C:\audio\secret.flac`, "/var/audio/secret.aiff"},
	}
	input.Source.EvidenceRefs = append(input.Source.EvidenceRefs, `D:\audio\vocal.wav`, "/tmp/render.wav")
	projection := Build(input)
	context := ContextProjection(projection)
	assertNoContextLeak(t, context)
	data, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	for _, forbidden := range []string{"file_path", "render_path", "time_segments", "gain_trace", "d:\\audio", "/tmp/", ".wav", ".flac", ".aiff"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("context contains %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(text, "lead vocal") || !strings.Contains(text, "dad:source:1") {
		t.Fatalf("safe compact context was over-sanitized: %s", data)
	}
}

func TestProjectionJSONContainsNoSourceOnlyBehaviorOrNumericParameterInference(t *testing.T) {
	projection := Build(readySourceInput())
	data, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation", "behavior_change"} {
		if _, ok := object[key]; ok {
			t.Fatalf("source-only JSON includes %s: %s", key, data)
		}
	}
	for _, key := range []string{"threshold_db", "ratio_value", "attack_ms", "release_ms", "makeup_gain_db", "sidechain_filter_hz"} {
		if _, ok := findJSONKey(object, key); ok {
			t.Fatalf("projection inferred parameter key %q", key)
		}
	}
}

func readySourceInput() Input {
	return Input{
		Mode:           ModeSourceOnly,
		ObservationID:  "obs-1",
		MixSessionID:   "mix-1",
		CreatedAt:      "2026-08-04T00:00:00Z",
		TargetRef:      map[string]any{"kind": "track", "id": "track-1", "label": "Vocal"},
		ProcessorScope: ProcessorScope{TrackID: "track-1"},
		Conditions: Conditions{
			ProjectCutRef: "cut-1", SourceRevision: "source-1", ClipRevision: "clip-1", MaterialRef: "material-1",
			StartSample: 1, EndSample: 960001, StartSeconds: 0, EndSeconds: 20,
			SampleRate: 48000, ChannelCount: 1, ChannelLayout: "mono", AnalyzerVersion: "dad.waveform.v1",
			AnalysisResolutions: []AnalysisResolution{{Scale: ScaleMacroProgram, WindowMS: 5000, HopMS: 5000}},
		},
		Source: SourceEvidence{
			ID: "source-evidence-1", SchemaVersion: "dad.waveform_envelope.v1", Status: StatusReady, Freshness: "fresh", DurationSeconds: 20,
			RMSDBFS: number(-24), ActiveRMSDBFS: number(-20), PeakDBFS: number(-6), CrestDB: number(18),
			TimeSegments: []TimeSegment{
				{StartSeconds: 0, EndSeconds: 5, RMSDBFS: number(-24), PeakDBFS: number(-8), CrestDB: number(16), EnergyState: "medium"},
				{StartSeconds: 5, EndSeconds: 10, RMSDBFS: number(-18), PeakDBFS: number(-5), CrestDB: number(13), EnergyState: "high"},
				{StartSeconds: 10, EndSeconds: 15, RMSDBFS: number(-21), PeakDBFS: number(-7), CrestDB: number(14), EnergyState: "medium"},
				{StartSeconds: 15, EndSeconds: 20, RMSDBFS: number(-120), PeakDBFS: number(-110), EnergyState: "silent"},
			},
			Quality:      QualityEvidence{QualityStatus: StatusReady, Nonzero: true, Coverage: 1},
			EvidenceRefs: []string{"dad:source:1", "dad:source:2"},
		},
	}
}

func number(value float64) *float64 { return &value }

func reverseSegments(segments []TimeSegment) {
	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}
}

func timeScale(scales []TimeScaleCoverage, name string) TimeScaleCoverage {
	for _, scale := range scales {
		if scale.Scale == name {
			return scale
		}
	}
	return TimeScaleCoverage{}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertNoContextLeak(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if contextForbiddenKey(key) {
				t.Fatalf("forbidden context key %q", key)
			}
			assertNoContextLeak(t, child)
		}
	case []any:
		for _, child := range typed {
			assertNoContextLeak(t, child)
		}
	case []map[string]any:
		for _, child := range typed {
			assertNoContextLeak(t, child)
		}
	case string:
		if _, ok := sanitizeContextString(typed); !ok {
			t.Fatalf("unsafe context string %q", typed)
		}
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			t.Fatalf("non-finite context number %v", typed)
		}
	}
}

func findJSONKey(value any, target string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == target {
				return child, true
			}
			if found, ok := findJSONKey(child, target); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range typed {
			if found, ok := findJSONKey(child, target); ok {
				return found, true
			}
		}
	}
	return nil, false
}
