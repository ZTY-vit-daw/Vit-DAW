#pragma once

#include <JuceHeader.h>

namespace vit
{

struct CompressorDualTapEvidenceRequest
{
    juce::String requestId;
    juce::String pairId;
    juce::String trackId;
    juce::String clipId;
    juce::String pluginInstanceId;
    juce::String pluginPosition;
    juce::String topologyClass;
    juce::String topologyGeneration;
    juce::String supportClass;
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
    int frameSizeSamples = 128;
    int hopSizeSamples = 64;
    int maxAlignmentSearchSamples = 256;
    juce::String analyzerVersion = "dad.compressor_dual_tap_analyzer.v1";
    bool deterministic = true;
};

struct CompressorTapQuality
{
    int64 sampleFrames = 0;
    int64 nonzeroSamples = 0;
    int64 nanInfSamples = 0;
    double coverage = 0.0;
    double peakDbfs = -160.0;
    double rmsDbfs = -160.0;
};

struct CompressorDualTapEvidenceResult
{
    juce::String status = "suspect";
    juce::String reason;
    double sampleRate = 0.0;
    int channelCount = 0;
    int64 alignedSampleFrames = 0;
    int measuredOffsetSamples = 0;
    int appliedOffsetSamples = 0;
    int residualErrorSamples = 0;
    double alignmentCorrelation = 0.0;
    bool alignmentReady = false;
    bool determinismVerified = false;
    double determinismMaxAbsDelta = 0.0;
    double determinismRMSDelta = 0.0;
    double determinismCorrelation = 0.0;
    double determinismPeakDBDelta = 0.0;
    double determinismRMSDBDelta = 0.0;
    CompressorTapQuality inputQuality;
    CompressorTapQuality outputQuality;
    int envelopeFrameCount = 0;
    int eventCandidateCount = 0;
    juce::String evidenceRef;
    juce::String artifactSha256;
    int64 artifactBytes = 0;
};

CompressorDualTapEvidenceResult analyseAndWriteCompressorDualTapEvidence (
    const juce::File& inputFile,
    const juce::File& outputFile,
    const juce::File& outputVerificationFile,
    const juce::File& artifactDirectory,
    const CompressorDualTapEvidenceRequest& request);

} // namespace vit
