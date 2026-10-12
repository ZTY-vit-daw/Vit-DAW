package fxm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// M2-TIMEFX-2 timefx differential producer face: decode the kernel
// dad.timefx_dual_tap_evidence.v1 artifact (written by the VitApp
// timefx_dual_tap_probe triple render) and assemble the fxm.Input the
// projection builder consumes. The two taps share one artifact identity
// (source/clip revision, window, format, chain), so a faithful artifact
// yields same-source/same-window/same-format by construction; the contract
// tests tamper with each dimension separately to pin the blocked/stale gate.

const (
	TimefxEvidenceSchema        = "dad.timefx_dual_tap_evidence.v1"
	TimefxBaselineStage         = "timefx_bypass"
	TimefxProcessedStage        = "timefx_processed"
	timefxEvidenceRefKindPrefix = "vit://dad.timefx_dual_tap/"
)

// TimefxEvidenceArtifact mirrors the kernel artifact JSON. Decoding is
// fail-closed against unknown fields (com paired-artifact precedent).
type TimefxEvidenceArtifact struct {
	SchemaVersion   string                 `json:"schema_version"`
	PairID          string                 `json:"pair_id"`
	EvidenceRef     string                 `json:"evidence_ref"`
	AnalyzerVersion string                 `json:"analyzer_version"`
	ProcessorScope  TimefxProcessorScope   `json:"processor_scope"`
	Conditions      TimefxConditions       `json:"conditions"`
	Determinism     TimefxDeterminismProof `json:"determinism_proof"`
	TapMetrics      TimefxTapMetricsPair   `json:"tap_metrics"`
	EffectDelta     TimefxArtifactDelta    `json:"effect_delta"`
}

type TimefxProcessorScope struct {
	TrackID            string `json:"track_id"`
	PluginInstanceID   string `json:"plugin_instance_id"`
	PluginPosition     string `json:"plugin_position"`
	Family             string `json:"family"`
	TopologyClass      string `json:"topology_class"`
	TopologyGeneration string `json:"topology_generation"`
	ChainHash          string `json:"chain_hash"`
	ProcessorStateHash string `json:"processor_state_hash"`
	ScopeRevision      string `json:"scope_revision"`
}

type TimefxConditions struct {
	SourceRevision       string  `json:"source_revision"`
	ClipRevision         string  `json:"clip_revision"`
	RenderRevision       string  `json:"render_revision"`
	StartSample          int64   `json:"start_sample"`
	EndSample            int64   `json:"end_sample"`
	SampleRate           float64 `json:"sample_rate"`
	ChannelCount         int     `json:"channel_count"`
	ChannelLayout        string  `json:"channel_layout"`
	RenderMode           string  `json:"render_mode"`
	Deterministic        bool    `json:"deterministic"`
	BaselineTap          string  `json:"baseline_tap"`
	ProcessedTap         string  `json:"processed_tap"`
	TailPolicy           string  `json:"tail_policy"`
	LatencyCompensated   bool    `json:"latency_compensated"`
	ReportedLatencySamps int     `json:"reported_latency_samples"`
}

type TimefxDeterminismProof struct {
	Status      string  `json:"status"`
	Method      string  `json:"method"`
	RepeatCount int     `json:"repeat_count"`
	MaxAbsDelta float64 `json:"max_abs_delta"`
	RMSDelta    float64 `json:"rms_delta"`
	Correlation float64 `json:"correlation"`
	PeakDBDelta float64 `json:"peak_db_delta"`
	RMSDBDelta  float64 `json:"rms_db_delta"`
}

type TimefxTapMetricsPair struct {
	Baseline  TimefxTapMetricsJSON `json:"baseline"`
	Processed TimefxTapMetricsJSON `json:"processed"`
}

type TimefxTapMetricsJSON struct {
	SampleFrames    int64   `json:"sample_frames"`
	NonzeroSamples  int64   `json:"nonzero_samples"`
	NaNInfSamples   int64   `json:"nan_inf_samples"`
	Coverage        float64 `json:"coverage"`
	PeakDBFS        float64 `json:"peak_dbfs"`
	RMSDBFS         float64 `json:"rms_dbfs"`
	ApproximateLUFS float64 `json:"approximate_lufs"`
	CrestDB         float64 `json:"crest_db"`
}

type TimefxArtifactDelta struct {
	RMSDeltaDB   float64 `json:"rms_delta_db"`
	LUFSDeltaDB  float64 `json:"lufs_delta_db"`
	PeakDeltaDB  float64 `json:"peak_delta_db"`
	CrestDeltaDB float64 `json:"crest_delta_db"`
}

// DecodeTimefxEvidenceArtifact decodes one kernel artifact payload. Unknown
// fields are rejected: the artifact schema is a kernel-owned contract.
func DecodeTimefxEvidenceArtifact(raw []byte) (TimefxEvidenceArtifact, error) {
	var artifact TimefxEvidenceArtifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return TimefxEvidenceArtifact{}, fmt.Errorf("decode timefx dual-tap artifact: %w", err)
	}
	return artifact, nil
}

// ValidateTimefxEvidenceArtifact applies the fail-closed artifact gates the
// producer requires before an fxm.Input may be built: schema, family, taps,
// determinism proof, render mode, window and metric finiteness. It returns
// the rejection reasons in kernel vocabulary.
func ValidateTimefxEvidenceArtifact(artifact TimefxEvidenceArtifact) []string {
	reasons := []string{}
	require := func(ok bool, reason string) {
		if !ok {
			reasons = append(reasons, reason)
		}
	}
	require(artifact.SchemaVersion == TimefxEvidenceSchema, "timefx_schema_mismatch")
	scope := artifact.ProcessorScope
	require(strings.TrimSpace(scope.Family) == "reverb" || strings.TrimSpace(scope.Family) == "delay", "timefx_family_unsupported")
	require(strings.TrimSpace(scope.TrackID) != "" && strings.TrimSpace(scope.PluginInstanceID) != "", "timefx_processor_target_missing")
	require(strings.TrimSpace(scope.ChainHash) != "" && strings.TrimSpace(scope.ProcessorStateHash) != "" && strings.TrimSpace(scope.ScopeRevision) != "", "timefx_processor_scope_identity_missing")
	c := artifact.Conditions
	require(strings.TrimSpace(c.SourceRevision) != "" && strings.TrimSpace(c.ClipRevision) != "" && strings.TrimSpace(c.RenderRevision) != "", "timefx_revision_identity_missing")
	require(c.StartSample >= 0 && c.EndSample > c.StartSample, "timefx_sample_window_invalid")
	require(finite(c.SampleRate) && c.SampleRate > 7000 && c.ChannelCount > 0, "timefx_sample_format_invalid")
	require(c.RenderMode == "offline_probe" && c.Deterministic, "timefx_render_not_deterministic")
	require(c.BaselineTap == TimefxBaselineStage && c.ProcessedTap == TimefxProcessedStage, "timefx_tap_order_invalid")
	require(c.TailPolicy == "same_window_pair", "timefx_tail_policy_invalid")
	require(artifact.Determinism.Status == StatusReady, "timefx_determinism_proof_failed")
	for _, metrics := range []TimefxTapMetricsJSON{artifact.TapMetrics.Baseline, artifact.TapMetrics.Processed} {
		require(metrics.SampleFrames >= c.EndSample-c.StartSample, "timefx_window_coverage_failed")
		require(metrics.NonzeroSamples > 0, "timefx_active_signal_missing")
		require(metrics.NaNInfSamples == 0, "timefx_nan_inf_detected")
		require(metrics.Coverage >= .999, "timefx_quality_coverage_failed")
		require(finite(metrics.PeakDBFS) && finite(metrics.RMSDBFS) && finite(metrics.ApproximateLUFS) && finite(metrics.CrestDB), "timefx_metric_not_finite")
	}
	require(strings.HasPrefix(strings.TrimSpace(artifact.EvidenceRef), timefxEvidenceRefKindPrefix), "timefx_evidence_ref_invalid")
	require(strings.TrimSpace(artifact.PairID) != "" && strings.Contains(strings.TrimSpace(artifact.EvidenceRef), "@"+strings.TrimSpace(artifact.PairID)+"#"), "timefx_pair_identity_invalid")
	return reasons
}

// InputFromTimefxEvidence assembles the fxm.Input from a validated kernel
// artifact. Both measurements inherit the single artifact identity, so
// Build's same-source/same-window/same-format gates hold for faithful
// artifacts. Latency compensation is declared per the artifact: the pair is
// same-window by construction, and a plugin reporting nonzero latency marks
// the pair uncompensated (the displacement is part of the measured
// transformation, so it must not be silently forgiven).
func InputFromTimefxEvidence(artifact TimefxEvidenceArtifact) (Input, error) {
	if reasons := ValidateTimefxEvidenceArtifact(artifact); len(reasons) > 0 {
		return Input{}, fmt.Errorf("timefx artifact rejected: %s", strings.Join(reasons, ","))
	}
	c := artifact.Conditions
	scope := artifact.ProcessorScope
	startSeconds := float64(c.StartSample) / c.SampleRate
	endSeconds := float64(c.EndSample) / c.SampleRate
	window := MeasurementWindow{
		SourceRevision: c.SourceRevision,
		ClipRevision:   c.ClipRevision,
		StartSeconds:   startSeconds,
		EndSeconds:     endSeconds,
		SampleRate:     c.SampleRate,
		ChannelCount:   c.ChannelCount,
	}
	measurement := func(stage string, metrics TimefxTapMetricsJSON) Measurement {
		peak, rms, lufs, crest := metrics.PeakDBFS, metrics.RMSDBFS, metrics.ApproximateLUFS, metrics.CrestDB
		return Measurement{
			ID: artifact.PairID + "_" + stage, Stage: stage, Status: StatusReady,
			SourceRevision: c.SourceRevision, ClipRevision: c.ClipRevision, RenderRevision: c.RenderRevision,
			ChainStateHash: scope.ProcessorStateHash, Window: window,
			RMSDBFS: &rms, LUFS: &lufs, PeakDBFS: &peak, CrestDB: &crest,
			Quality: QualityEvidence{
				Deterministic:      c.Deterministic && artifact.Determinism.Status == StatusReady,
				LatencyCompensated: c.LatencyCompensated,
				Nonzero:            metrics.NonzeroSamples > 0,
				Coverage:           metrics.Coverage,
				NaNInfCount:        int(metrics.NaNInfSamples),
			},
			EvidenceRefs: []string{artifact.EvidenceRef},
		}
	}
	return Input{
		TargetRef: map[string]any{"kind": "track", "id": scope.TrackID, "family": scope.Family, "plugin_id": scope.PluginInstanceID},
		Chain: ChainIdentity{
			ChainHash:   scope.ChainHash,
			RoutingHash: scope.ChainHash,
			Plugins: []PluginIdentity{{
				Order: 1, PluginID: scope.PluginInstanceID, StateHash: scope.ProcessorStateHash,
			}},
		},
		Conditions: window,
		Baseline:   measurement(TimefxBaselineStage, artifact.TapMetrics.Baseline),
		Processed:  measurement(TimefxProcessedStage, artifact.TapMetrics.Processed),
		ProbeRefs:  []string{artifact.EvidenceRef},
	}, nil
}

// TimefxDeltaIsMeaningful reports whether the measured differential is a
// real, finite, non-nil observation (the values the FXM projection and the
// acceptance gate key on). A zero delta is not an error: it means the
// processor measurably did not transform the material in this window.
func TimefxDeltaIsMeaningful(delta EffectDelta) bool {
	if delta.RMSDB == nil || delta.LUFSDB == nil || delta.PeakDB == nil || delta.CrestDB == nil {
		return false
	}
	for _, value := range []float64{*delta.RMSDB, *delta.LUFSDB, *delta.PeakDB, *delta.CrestDB} {
		if !finite(value) {
			return false
		}
	}
	return true
}
