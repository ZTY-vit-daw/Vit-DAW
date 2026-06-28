#pragma once

#include <JuceHeader.h>

namespace vit
{

enum class AudioFeatureType
{
    WaveformEnvelope,
    TimeEnergy,
    SpectralField,
    StereoRelationField,
    BandEnergySummary,
    StereoRelationSummary,
    LoudnessSummary,
    L3AcousticSummary,
    SegmentationPrimitives,
    ThreeDField,
    MasterOutputPreview
};

enum class AudioFeaturePriority
{
    ImportImmediate,
    BackgroundWarm,
    OnDemand
};

struct AudioFeatureRange
{
    double sourceOffsetSeconds = 0.0;
    double lengthSeconds = -1.0;
};

struct AudioFeatureResolution
{
    int frameWidth = 500;
    int frequencyBins = 336;
    double frameSeconds = 0.01;
};

struct AudioFeatureBakeRequest
{
    juce::String filePath;
    juce::String trackId;
    juce::String clipId;
    juce::String sourceId;
    juce::String sourceRevision;
    juce::String clipRevision;
    juce::String renderRevision;
    juce::String requestId;
    AudioFeatureType featureType = AudioFeatureType::SpectralField;
    AudioFeaturePriority priority = AudioFeaturePriority::OnDemand;
    AudioFeatureRange range;
    AudioFeatureResolution resolution;
};

juce::String audioFeatureTypeToString (AudioFeatureType type);
AudioFeatureType audioFeatureTypeFromString (const juce::String& text,
                                             AudioFeatureType fallback = AudioFeatureType::SpectralField);
juce::String audioFeaturePriorityToString (AudioFeaturePriority priority);
juce::String audioFeatureAnalysisVersion();
juce::String audioFeatureProductVersion (AudioFeatureType type);
bool audioFeatureUsesSpectralTextureTile (AudioFeatureType type);

} // namespace vit
