package com

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

const PairedEvidenceSchemaVersion = "dad.compressor_dual_tap_receipt.v1"

type PairedEvidenceReceipt struct {
	Command                string                   `json:"command"`
	FeatureFamily          string                   `json:"feature_family"`
	FeatureType            string                   `json:"feature_type"`
	SchemaVersion          string                   `json:"schema_version"`
	Status                 string                   `json:"status"`
	QualityStatus          string                   `json:"quality_status"`
	Reason                 string                   `json:"reason"`
	JobID                  string                   `json:"job_id"`
	RequestID              string                   `json:"request_id,omitempty"`
	PairID                 string                   `json:"pair_id"`
	TrackID                string                   `json:"track_id"`
	ClipID                 string                   `json:"clip_id"`
	PluginInstanceID       string                   `json:"plugin_instance_id"`
	PluginPosition         string                   `json:"plugin_position"`
	TopologyClass          string                   `json:"topology_class"`
	TopologyGeneration     string                   `json:"topology_generation"`
	SupportClass           string                   `json:"support_class"`
	ChainHash              string                   `json:"chain_hash"`
	ProcessorStateHash     string                   `json:"processor_state_hash"`
	ScopeRevision          string                   `json:"scope_revision"`
	SourceRevision         string                   `json:"source_revision"`
	ClipRevision           string                   `json:"clip_revision"`
	RenderRevision         string                   `json:"render_revision"`
	StartSample            int64                    `json:"start_sample"`
	EndSample              int64                    `json:"end_sample"`
	SampleRate             float64                  `json:"sample_rate"`
	ChannelCount           int                      `json:"channel_count"`
	ChannelLayout          string                   `json:"channel_layout"`
	RenderMode             string                   `json:"render_mode"`
	Deterministic          bool                     `json:"deterministic"`
	InputTap               string                   `json:"input_tap"`
	OutputTap              string                   `json:"output_tap"`
	TailPolicy             string                   `json:"tail_policy"`
	AnalyzerVersion        string                   `json:"analyzer_version"`
	EvidenceRef            string                   `json:"evidence_ref"`
	AlignedSampleFrames    int64                    `json:"aligned_sample_frames"`
	EnvelopeFrameCount     int                      `json:"envelope_frame_count"`
	EventCandidateCount    int                      `json:"event_candidate_count"`
	ArtifactSHA256         string                   `json:"artifact_sha256"`
	ArtifactBytes          int64                    `json:"artifact_bytes"`
	DeterminismProofStatus string                   `json:"determinism_proof_status"`
	DeterminismMaxAbsDelta float64                  `json:"determinism_max_abs_delta"`
	DeterminismRMSDelta    float64                  `json:"determinism_rms_delta"`
	DeterminismCorrelation float64                  `json:"determinism_correlation"`
	DeterminismPeakDBDelta float64                  `json:"determinism_peak_db_delta"`
	DeterminismRMSDBDelta  float64                  `json:"determinism_rms_db_delta"`
	LatencyAlignment       EvidenceLatencyAlignment `json:"latency_alignment"`
	QualityEvidence        PairedQualityEvidence    `json:"quality_evidence"`
}

type EvidenceLatencyAlignment struct {
	Status                       string  `json:"status"`
	Method                       string  `json:"method"`
	PluginReportedLatencySamples int     `json:"plugin_reported_latency_samples"`
	MeasuredOffsetSamples        int     `json:"measured_offset_samples"`
	AppliedOffsetSamples         int     `json:"applied_offset_samples"`
	ResidualErrorSamples         int     `json:"residual_error_samples"`
	Correlation                  float64 `json:"correlation"`
	SearchRadiusSamples          int     `json:"search_radius_samples,omitempty"`
	AlignedSampleFrames          int64   `json:"aligned_sample_frames,omitempty"`
}

type PairedQualityEvidence struct {
	Input  TapQualityEvidence `json:"input"`
	Output TapQualityEvidence `json:"output"`
}

type TapQualityEvidence struct {
	SampleFrames   int64   `json:"sample_frames"`
	NonzeroSamples int64   `json:"nonzero_samples"`
	NaNInfSamples  int64   `json:"nan_inf_samples"`
	Coverage       float64 `json:"coverage"`
	PeakDBFS       float64 `json:"peak_dbfs"`
	RMSDBFS        float64 `json:"rms_dbfs"`
}

type PairedEvidenceExpectation struct {
	TrackID            string
	ClipID             string
	PluginInstanceID   string
	TopologyGeneration string
	ScopeRevision      string
	SourceRevision     string
	ClipRevision       string
	StartSample        int64
	EndSample          int64
	SampleRate         float64
	ChannelCount       int
	PriorPairID        string
	PriorJobID         string
}

type PairedEvidenceValidation struct {
	Ready   bool     `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func DecodePairedEvidenceReceipt(raw []byte) (PairedEvidenceReceipt, error) {
	if reason := pairedEvidenceRawLeakReason(raw); reason != "" {
		return PairedEvidenceReceipt{}, fmt.Errorf("raw_evidence_leak: %s", reason)
	}
	var receipt PairedEvidenceReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return PairedEvidenceReceipt{}, fmt.Errorf("decode paired evidence receipt: %w", err)
	}
	return receipt, nil
}

func ValidatePairedEvidenceReceipt(receipt PairedEvidenceReceipt, expected PairedEvidenceExpectation) PairedEvidenceValidation {
	reasons := make([]string, 0)
	require := func(ok bool, reason string) {
		if !ok {
			reasons = append(reasons, reason)
		}
	}
	require(receipt.SchemaVersion == PairedEvidenceSchemaVersion, "schema_version_mismatch")
	require(receipt.Command == "compressor_dual_tap_probe_ready" && receipt.FeatureType == "compressor_dual_tap_probe", "feature_identity_mismatch")
	require(receipt.Status == StatusReady && receipt.QualityStatus == StatusReady && receipt.Reason == "ok", "receipt_not_ready")
	require(strings.TrimSpace(receipt.PairID) != "" && strings.TrimSpace(receipt.JobID) != "", "capture_identity_missing")
	require(strings.TrimSpace(receipt.TrackID) != "" && strings.TrimSpace(receipt.ClipID) != "" && strings.TrimSpace(receipt.PluginInstanceID) != "", "target_identity_missing")
	require(strings.TrimSpace(receipt.PluginPosition) != "" && strings.TrimSpace(receipt.ChainHash) != "" && strings.TrimSpace(receipt.ProcessorStateHash) != "" && strings.TrimSpace(receipt.ScopeRevision) != "", "processor_scope_identity_missing")
	require(strings.TrimSpace(receipt.TopologyClass) != "" && strings.TrimSpace(receipt.TopologyGeneration) != "", "topology_identity_missing")
	require(receipt.SupportClass == "single_band_broadband", "unsupported_processor_scope")
	require(strings.TrimSpace(receipt.SourceRevision) != "" && strings.TrimSpace(receipt.ClipRevision) != "" && strings.TrimSpace(receipt.RenderRevision) != "", "revision_identity_missing")
	require(receipt.StartSample >= 0 && receipt.EndSample > receipt.StartSample, "sample_window_invalid")
	require(finite(receipt.SampleRate) && receipt.SampleRate > 7000 && receipt.ChannelCount > 0 && strings.TrimSpace(receipt.ChannelLayout) != "", "sample_format_invalid")
	require(receipt.RenderMode == "offline_probe" && receipt.Deterministic, "render_not_deterministic")
	require(receipt.DeterminismProofStatus == StatusReady && finite(receipt.DeterminismCorrelation) && receipt.DeterminismCorrelation >= .999 && finite(receipt.DeterminismPeakDBDelta) && math.Abs(receipt.DeterminismPeakDBDelta) <= .10 && finite(receipt.DeterminismRMSDBDelta) && math.Abs(receipt.DeterminismRMSDBDelta) <= .10, "determinism_proof_failed")
	require(receipt.InputTap == "compressor_input" && receipt.OutputTap == "compressor_output", "tap_order_invalid")
	require(receipt.TailPolicy == "exact_window_no_tail" && strings.TrimSpace(receipt.AnalyzerVersion) != "", "analysis_conditions_missing")
	require(receipt.EvidenceRef == "dad.compressor_dual_tap:"+receipt.PairID, "evidence_ref_invalid")
	require(sha256Pattern.MatchString(receipt.ArtifactSHA256) && receipt.ArtifactBytes > 0, "artifact_integrity_invalid")
	require(receipt.EnvelopeFrameCount > 0 && receipt.AlignedSampleFrames > 0, "fine_envelope_missing")
	validateTapQuality := func(label string, quality TapQualityEvidence) {
		require(quality.SampleFrames >= receipt.EndSample-receipt.StartSample, label+"_window_coverage_failed")
		require(quality.NonzeroSamples > 0 && quality.NaNInfSamples == 0, label+"_signal_quality_failed")
		require(finite(quality.Coverage) && quality.Coverage >= 0.999, label+"_coverage_failed")
		require(finite(quality.PeakDBFS) && finite(quality.RMSDBFS), label+"_level_quality_failed")
	}
	validateTapQuality("input", receipt.QualityEvidence.Input)
	validateTapQuality("output", receipt.QualityEvidence.Output)
	alignment := receipt.LatencyAlignment
	require(alignment.Status == StatusReady && alignment.Method == "offline_pdc_plus_integer_cross_correlation_v1", "latency_alignment_missing")
	require(alignment.AppliedOffsetSamples == alignment.MeasuredOffsetSamples, "latency_offset_not_applied")
	require(absInt(alignment.ResidualErrorSamples) <= 1, "latency_residual_too_large")
	require(finite(alignment.Correlation) && alignment.Correlation >= 0.25, "latency_correlation_failed")

	compareString := func(actual, wanted, reason string) {
		if strings.TrimSpace(wanted) != "" {
			require(actual == wanted, reason)
		}
	}
	compareString(receipt.TrackID, expected.TrackID, "track_identity_mismatch")
	compareString(receipt.ClipID, expected.ClipID, "clip_identity_mismatch")
	compareString(receipt.PluginInstanceID, expected.PluginInstanceID, "plugin_identity_mismatch")
	compareString(receipt.TopologyGeneration, expected.TopologyGeneration, "topology_generation_mismatch")
	compareString(receipt.ScopeRevision, expected.ScopeRevision, "scope_revision_stale")
	compareString(receipt.SourceRevision, expected.SourceRevision, "source_revision_mismatch")
	compareString(receipt.ClipRevision, expected.ClipRevision, "clip_revision_mismatch")
	if expected.EndSample > expected.StartSample {
		require(receipt.StartSample == expected.StartSample && receipt.EndSample == expected.EndSample, "sample_window_mismatch")
	}
	if expected.SampleRate > 0 {
		require(math.Abs(receipt.SampleRate-expected.SampleRate) < 0.01, "sample_rate_mismatch")
	}
	if expected.ChannelCount > 0 {
		require(receipt.ChannelCount == expected.ChannelCount, "channel_count_mismatch")
	}
	if strings.TrimSpace(expected.PriorPairID) != "" {
		require(receipt.PairID != expected.PriorPairID, "pair_id_stale")
	}
	if strings.TrimSpace(expected.PriorJobID) != "" {
		require(receipt.JobID != expected.PriorJobID, "job_id_stale")
	}
	sort.Strings(reasons)
	return PairedEvidenceValidation{Ready: len(reasons) == 0, Reasons: uniqueSorted(reasons)}
}

func pairedEvidenceRawLeakReason(raw []byte) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "invalid_json"
	}
	forbidden := map[string]bool{
		"shared_memory": true, "time_segments": true, "spectral_tiles": true,
		"raw_ranges": true, "raw_waveform": true, "raw_samples": true,
		"render_file_path": true, "render_path": true, "file_path": true,
		"aligned_envelope_frames": true, "input_event_candidates": true,
		"frames": true, "events": true,
	}
	var walk func(any) string
	walk = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[strings.ToLower(strings.TrimSpace(key))] {
					return key
				}
				if reason := walk(child); reason != "" {
					return reason
				}
			}
		case []any:
			for _, child := range typed {
				if reason := walk(child); reason != "" {
					return reason
				}
			}
		case string:
			lower := strings.ToLower(strings.TrimSpace(typed))
			if regexp.MustCompile(`^[a-z]:[\\/]`).MatchString(lower) || strings.HasPrefix(lower, "/") {
				return "absolute_path"
			}
		}
		return ""
	}
	return walk(value)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
