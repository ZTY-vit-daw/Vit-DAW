package fxm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func Build(in Input) Projection {
	limitations := []string{}
	blocked := []string{}
	baselineReady := measurementReady(in.Baseline)
	processedReady := measurementReady(in.Processed)
	if !baselineReady {
		blocked = append(blocked, "baseline_measurement_missing")
	}
	if !processedReady {
		blocked = append(blocked, "processed_measurement_missing")
	}
	sameSource := sameNonEmpty(firstNonEmpty(in.Baseline.SourceRevision, in.Baseline.Window.SourceRevision, in.Conditions.SourceRevision), firstNonEmpty(in.Processed.SourceRevision, in.Processed.Window.SourceRevision, in.Conditions.SourceRevision))
	if baselineReady && processedReady && !sameSource {
		blocked = append(blocked, "source_revision_mismatch")
	}
	sameWindow := windowsMatch(in.Baseline.Window, in.Processed.Window, in.Conditions)
	if baselineReady && processedReady && !sameWindow {
		blocked = append(blocked, "measurement_window_mismatch")
	}
	sameFormat := formatsMatch(in.Baseline.Window, in.Processed.Window, in.Conditions)
	if baselineReady && processedReady && !sameFormat {
		blocked = append(blocked, "sample_format_mismatch")
	}
	deterministic := in.Baseline.Quality.Deterministic && in.Processed.Quality.Deterministic
	latencyCompensated := in.Baseline.Quality.LatencyCompensated && in.Processed.Quality.LatencyCompensated
	if baselineReady && processedReady && !deterministic {
		limitations = append(limitations, "one or both renders are not declared deterministic")
	}
	if baselineReady && processedReady && !latencyCompensated {
		limitations = append(limitations, "one or both renders are not latency compensated")
	}
	if in.Baseline.Quality.NaNInfCount > 0 || in.Processed.Quality.NaNInfCount > 0 {
		blocked = append(blocked, "render_contains_nan_or_inf")
	}

	delta, metricCount, deltaLimits := buildDelta(in.Baseline, in.Processed)
	limitations = append(limitations, deltaLimits...)
	status := StatusReady
	if !baselineReady || !processedReady {
		status = StatusMissing
	} else if !sameSource || !sameWindow || !sameFormat || len(blocked) > 0 {
		status = StatusStale
	} else if metricCount == 0 || !deterministic || !latencyCompensated || len(limitations) > 0 {
		status = StatusPartial
	}
	canObserve := (status == StatusReady || status == StatusPartial) && metricCount > 0
	canSuggest := canObserve && sameSource && sameWindow && sameFormat
	canPreflight := status == StatusReady && deterministic && latencyCompensated
	trust := TrustQuality{
		OverallStatus: status, SameSource: sameSource, SameWindow: sameWindow, SameFormat: sameFormat,
		Deterministic: deterministic, LatencyCompensated: latencyCompensated,
		CanSupportObservation: canObserve, CanSupportSuggestion: canSuggest, CanSupportActionPreflight: canPreflight,
		BlockedReasons: uniqueSorted(blocked), Limitations: uniqueSorted(limitations),
	}
	evidence := uniqueSorted(append(append(append([]string{}, in.Baseline.EvidenceRefs...), in.Processed.EvidenceRefs...), in.ProbeRefs...))
	generatedAt := strings.TrimSpace(in.CreatedAt)
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	projection := Projection{
		SchemaVersion: SchemaVersion, FXMVersion: Version, ObservationID: strings.TrimSpace(in.ObservationID), MixSessionID: strings.TrimSpace(in.MixSessionID),
		Status: status, TargetRef: cloneMap(in.TargetRef), Chain: in.Chain, Conditions: effectiveConditions(in),
		BaselineRef: measurementRef(in.Baseline), ProcessedRef: measurementRef(in.Processed), EffectDelta: delta,
		TrustQuality: trust, EvidenceRefs: evidence, Limitations: trust.Limitations, GeneratedAt: generatedAt,
	}
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func ContextProjection(projection Projection) map[string]any {
	return map[string]any{
		"schema_version": projection.SchemaVersion,
		"fxm_version":    projection.FXMVersion,
		"projection_id":  projection.ProjectionID,
		"status":         projection.Status,
		"target_ref":     projection.TargetRef,
		"chain_hash":     projection.Chain.ChainHash,
		"baseline_stage": projection.BaselineRef.Stage,
		"effect_delta":   projection.EffectDelta,
		"trust_quality":  projection.TrustQuality,
		"llm_context":    projection.LLMContext,
		"evidence_refs":  projection.EvidenceRefs,
	}
}

func buildDelta(before, after Measurement) (EffectDelta, int, []string) {
	var out EffectDelta
	count := 0
	out.RMSDB, count = subtractFloat(before.RMSDBFS, after.RMSDBFS, count)
	out.LUFSDB, count = subtractFloat(before.LUFS, after.LUFS, count)
	out.PeakDB, count = subtractFloat(before.PeakDBFS, after.PeakDBFS, count)
	out.CrestDB, count = subtractFloat(before.CrestDB, after.CrestDB, count)
	if before.LatencySamples != nil && after.LatencySamples != nil {
		value := *after.LatencySamples - *before.LatencySamples
		out.Latency = &value
		count++
	}
	limits := []string{}
	beforeBands := map[string]BandMetric{}
	for _, band := range before.Bands {
		beforeBands[bandKey(band)] = band
	}
	for _, band := range after.Bands {
		key := bandKey(band)
		base, ok := beforeBands[key]
		if !ok {
			limits = append(limits, "baseline omits processed spectral band "+key)
			continue
		}
		out.Bands = append(out.Bands, BandDelta{ID: firstNonEmpty(band.ID, base.ID), MinHz: band.MinHz, MaxHz: band.MaxHz, DeltaDB: round(band.EnergyDB - base.EnergyDB)})
		count++
	}
	sort.Slice(out.Bands, func(i, j int) bool { return out.Bands[i].MinHz < out.Bands[j].MinHz })
	return out, count, uniqueSorted(limits)
}

func buildLLMContext(p Projection) LLMContext {
	facts := []map[string]any{{"projection_id": p.ProjectionID, "status": p.Status, "baseline_stage": p.BaselineRef.Stage, "chain_hash": p.Chain.ChainHash}}
	if p.EffectDelta.RMSDB != nil {
		facts = append(facts, map[string]any{"metric": "rms_delta_db", "value": *p.EffectDelta.RMSDB})
	}
	if p.EffectDelta.LUFSDB != nil {
		facts = append(facts, map[string]any{"metric": "lufs_delta_db", "value": *p.EffectDelta.LUFSDB})
	}
	if p.EffectDelta.PeakDB != nil {
		facts = append(facts, map[string]any{"metric": "peak_delta_db", "value": *p.EffectDelta.PeakDB})
	}
	for i, band := range p.EffectDelta.Bands {
		if i >= 8 {
			break
		}
		facts = append(facts, map[string]any{"metric": "spectral_delta_db", "band": band.ID, "min_hz": band.MinHz, "max_hz": band.MaxHz, "value": band.DeltaDB})
	}
	summary := fmt.Sprintf("FXM %s projection %s compares %s against processed chain state.", p.Status, p.ProjectionID, firstNonEmpty(p.BaselineRef.Stage, "baseline"))
	next := "Do not use this projection for action preflight until matching baseline and processed renders are available."
	if p.TrustQuality.CanSupportActionPreflight {
		next = "Use the compact deltas as one evidence source; do not treat them as a musical target or load raw render payloads into context."
	}
	return LLMContext{SummaryMD: summary, CompactFacts: facts, DoNotIncludeRawPackage: true, EvidenceRefs: p.EvidenceRefs, LimitationNotes: p.Limitations, SuggestedNextStep: next}
}

func measurementReady(value Measurement) bool {
	status := strings.ToLower(strings.TrimSpace(value.Status))
	return (status == "ready" || status == "conformed") && strings.TrimSpace(value.ID) != ""
}

func windowsMatch(a, b, fallback MeasurementWindow) bool {
	aStart, aEnd := effectiveWindow(a, fallback)
	bStart, bEnd := effectiveWindow(b, fallback)
	return math.Abs(aStart-bStart) <= 0.0001 && math.Abs(aEnd-bEnd) <= 0.0001 && aEnd > aStart
}

func formatsMatch(a, b, fallback MeasurementWindow) bool {
	aRate := firstPositive(a.SampleRate, fallback.SampleRate)
	bRate := firstPositive(b.SampleRate, fallback.SampleRate)
	aChannels := firstPositiveInt(a.ChannelCount, fallback.ChannelCount)
	bChannels := firstPositiveInt(b.ChannelCount, fallback.ChannelCount)
	return aRate > 0 && bRate > 0 && math.Abs(aRate-bRate) <= 0.01 && aChannels > 0 && aChannels == bChannels
}

func effectiveWindow(value, fallback MeasurementWindow) (float64, float64) {
	start, end := value.StartSeconds, value.EndSeconds
	if end <= start {
		start, end = fallback.StartSeconds, fallback.EndSeconds
	}
	return start, end
}

func effectiveConditions(in Input) MeasurementWindow {
	out := in.Conditions
	out.SourceRevision = firstNonEmpty(out.SourceRevision, in.Baseline.SourceRevision, in.Baseline.Window.SourceRevision, in.Processed.SourceRevision, in.Processed.Window.SourceRevision)
	out.ClipRevision = firstNonEmpty(out.ClipRevision, in.Baseline.ClipRevision, in.Baseline.Window.ClipRevision, in.Processed.ClipRevision, in.Processed.Window.ClipRevision)
	if out.EndSeconds <= out.StartSeconds {
		out.StartSeconds, out.EndSeconds = effectiveWindow(in.Processed.Window, in.Baseline.Window)
	}
	out.SampleRate = firstPositive(out.SampleRate, in.Processed.Window.SampleRate, in.Baseline.Window.SampleRate)
	out.ChannelCount = firstPositiveInt(out.ChannelCount, in.Processed.Window.ChannelCount, in.Baseline.Window.ChannelCount)
	return out
}

func measurementRef(value Measurement) MeasurementRef {
	return MeasurementRef{ID: strings.TrimSpace(value.ID), Stage: strings.TrimSpace(value.Stage), RenderRevision: strings.TrimSpace(value.RenderRevision), ChainStateHash: strings.TrimSpace(value.ChainStateHash)}
}

func subtractFloat(before, after *float64, count int) (*float64, int) {
	if before == nil || after == nil || !finite(*before) || !finite(*after) {
		return nil, count
	}
	value := round(*after - *before)
	return &value, count + 1
}

func stableProjectionID(value Projection) string {
	copy := value
	copy.ProjectionID = ""
	copy.LLMContext = LLMContext{}
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return "fxm_" + hex.EncodeToString(sum[:])[:20]
}

func bandKey(value BandMetric) string {
	if id := strings.TrimSpace(value.ID); id != "" {
		return id
	}
	return fmt.Sprintf("%.3f-%.3f", value.MinHz, value.MaxHz)
}

func sameNonEmpty(a, b string) bool {
	return strings.TrimSpace(a) != "" && strings.TrimSpace(a) == strings.TrimSpace(b)
}
func finite(v float64) bool   { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func round(v float64) float64 { return math.Round(v*1000) / 1000 }
func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
func firstPositiveInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}
