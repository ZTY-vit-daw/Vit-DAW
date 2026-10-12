#pragma once

#include <JuceHeader.h>

namespace vit
{

// M2-TIMEFX-2: paired bypass/processed render evidence for the reverb/delay
// families. Same triple-render shape as the compressor dual-tap probe
// (bypass render + processed render + verification render), but the pair is
// measured over the identical window without latency alignment: for
// time-domain effects the temporal displacement IS the transformation, so
// aligning the taps would erase the effect under measurement. Both taps are
// reported as per-window metrics (RMS/Peak/approximate LUFS/Crest) plus the
// same-window differential.
struct TimefxDualTapEvidenceRequest
{
    juce::String requestId;
    juce::String pairId;
    juce::String trackId;
    juce::String clipId;
    juce::String pluginInstanceId;
    juce::String pluginPosition;
    juce::String family;            // "reverb" | "delay"
    juce::String topologyClass;
    juce::String topologyGeneration;
    juce::String chainHash;
    juce::String processorStateHash;
    juce::String scopeRevision;
    juce::String sourceRevision;
    juce::String clipRevision;
    juce::String renderRevision;
    int64 startSample = 0;
    int64 endSample = 0;
    double sampleRate = 0.0;
    juce::String channelLayout;
    int reportedLatencySamples = 0;
    juce::String analyzerVersion = "dad.timefx_dual_tap_analyzer.v1";
    bool deterministic = true;
};

struct TimefxTapMetrics
{
    int64 sampleFrames = 0;
    int64 nonzeroSamples = 0;
    int64 nanInfSamples = 0;
    double coverage = 0.0;
    double peakDbfs = -160.0;
    double rmsDbfs = -160.0;
    double approximateLufs = -160.0;
    double crestDb = 0.0;
};

struct TimefxDualTapEvidenceResult
{
    juce::String status = "suspect";
    juce::String reason;
    double sampleRate = 0.0;
    int channelCount = 0;
    bool determinismVerified = false;
    double determinismMaxAbsDelta = 0.0;
    double determinismRMSDelta = 0.0;
    double determinismCorrelation = 0.0;
    double determinismPeakDBDelta = 0.0;
    double determinismRMSDBDelta = 0.0;
    bool latencyCompensated = true;
    TimefxTapMetrics baselineMetrics;   // bypass tap
    TimefxTapMetrics processedMetrics;  // processed tap
    double rmsDeltaDb = 0.0;
    double lufsDeltaDb = 0.0;
    double peakDeltaDb = 0.0;
    double crestDeltaDb = 0.0;
    juce::String evidenceRef;
    juce::String artifactSha256;
    int64 artifactBytes = 0;
};

TimefxDualTapEvidenceResult analyseAndWriteTimefxDualTapEvidence (
    const juce::File& baselineRenderFile,
    const juce::File& processedRenderFile,
    const juce::File& processedVerificationRenderFile,
    const juce::File& artifactDirectory,
    const TimefxDualTapEvidenceRequest& request);

} // namespace vit
