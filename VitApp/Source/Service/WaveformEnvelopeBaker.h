#pragma once

#include <functional>

#include <JuceHeader.h>

#include "AudioFeatureTypes.h"

namespace vit
{

class WaveformEnvelopeBaker final
{
public:
    using PublishCallback = std::function<void(const juce::String&)>;

    static void startBake (juce::String filePath,
                           juce::String trackId,
                           juce::String clipId,
                           PublishCallback publishCallback,
                           double sourceOffsetSeconds = 0.0,
                           double bakeLengthSeconds = -1.0,
                           int framesPerTile = 1024,
                           juce::String sourceId = {},
                           juce::String sourceRevision = {},
                           juce::String clipRevision = {},
                           juce::String renderRevision = {},
                           AudioFeaturePriority priority = AudioFeaturePriority::OnDemand);

    static void releaseTrackMappings (const juce::String& trackId);
    static void invalidateClipBake (const juce::String& clipId);
    static juce::var getLatestBakeStatus (const juce::String& trackId,
                                          const juce::String& clipId);
};

} // namespace vit
