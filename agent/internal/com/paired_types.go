package com

// PairedEvidenceArtifact is the typed, read-only form of a COM-2 artifact.
// Raw frame/event arrays are consumed only during derivation and are never
// copied into Projection or LLM context.
type PairedEvidenceArtifact struct {
	SchemaVersion         string                      `json:"schema_version"`
	PairID                string                      `json:"pair_id"`
	EvidenceRef           string                      `json:"evidence_ref"`
	AnalyzerVersion       string                      `json:"analyzer_version"`
	ProcessorScope        PairedProcessorScope        `json:"processor_scope"`
	Conditions            PairedConditions            `json:"conditions"`
	LatencyAlignment      EvidenceLatencyAlignment    `json:"latency_alignment"`
	DeterminismProof      PairedDeterminismProof      `json:"determinism_proof"`
	QualityEvidence       PairedQualityEvidence       `json:"quality_evidence"`
	AlignedEnvelopeFrames []PairedEnvelopeFrame       `json:"aligned_envelope_frames"`
	InputEventCandidates  []PairedInputEventCandidate `json:"input_event_candidates"`
}

type PairedProcessorScope struct {
	TrackID            string `json:"track_id"`
	PluginInstanceID   string `json:"plugin_instance_id"`
	PluginPosition     string `json:"plugin_position"`
	TopologyClass      string `json:"topology_class"`
	TopologyGeneration string `json:"topology_generation"`
	SupportClass       string `json:"support_class"`
	ChainHash          string `json:"chain_hash"`
	ProcessorStateHash string `json:"processor_state_hash"`
	ScopeRevision      string `json:"scope_revision"`
}

type PairedConditions struct {
	SourceRevision   string  `json:"source_revision"`
	ClipRevision     string  `json:"clip_revision"`
	RenderRevision   string  `json:"render_revision"`
	StartSample      int64   `json:"start_sample"`
	EndSample        int64   `json:"end_sample"`
	SampleRate       float64 `json:"sample_rate"`
	ChannelCount     int     `json:"channel_count"`
	ChannelLayout    string  `json:"channel_layout"`
	RenderMode       string  `json:"render_mode"`
	Deterministic    bool    `json:"deterministic"`
	InputTap         string  `json:"input_tap"`
	OutputTap        string  `json:"output_tap"`
	TailPolicy       string  `json:"tail_policy"`
	FrameSizeSamples int     `json:"frame_size_samples"`
	HopSizeSamples   int     `json:"hop_size_samples"`
}

type PairedDeterminismProof struct {
	Status                string  `json:"status"`
	Method                string  `json:"method"`
	RepeatCount           int     `json:"repeat_count"`
	MaxAbsDelta           float64 `json:"max_abs_delta"`
	RMSDelta              float64 `json:"rms_delta"`
	Correlation           float64 `json:"correlation"`
	PeakDBDelta           float64 `json:"peak_db_delta"`
	RMSDBDelta            float64 `json:"rms_db_delta"`
	CorrelationMinimum    float64 `json:"correlation_minimum"`
	LevelDeltaToleranceDB float64 `json:"level_delta_tolerance_db"`
}

type PairedEnvelopeFrame struct {
	StartSample    int64     `json:"start_sample"`
	EndSample      int64     `json:"end_sample"`
	InputPeakDBFS  []float64 `json:"input_peak_dbfs"`
	InputRMSDBFS   []float64 `json:"input_rms_dbfs"`
	OutputPeakDBFS []float64 `json:"output_peak_dbfs"`
	OutputRMSDBFS  []float64 `json:"output_rms_dbfs"`
}

type PairedInputEventCandidate struct {
	Sample        int64   `json:"sample"`
	InputPeakDBFS float64 `json:"input_peak_dbfs"`
	Kind          string  `json:"kind"`
}

// PairedDerivationConstraints contains independently known interpretation
// limits. It contains no control values. COM does not guess these conditions
// from audio.
type PairedDerivationConstraints struct {
	ParallelBlendKnown  bool
	ParallelBlendActive bool
	GainStageAmbiguous  bool
}
