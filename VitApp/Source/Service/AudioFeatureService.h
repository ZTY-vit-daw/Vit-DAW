#pragma once

#include <functional>

#include <JuceHeader.h>

#include "AudioFeatureTypes.h"

namespace vit
{

class AudioFeatureService final
{
public:
    using PublishCallback = std::function<void(const juce::String&)>;

    static void requestBake (AudioFeatureBakeRequest request,
                             PublishCallback publishCallback);

    static void requestLegacySpectralFieldBake (juce::String filePath,
                                                juce::String trackId,
                                                juce::String clipId,
                                                PublishCallback publishCallback,
                                                double sourceOffsetSeconds = 0.0,
                                                double bakeLengthSeconds = -1.0);

    static void releaseTrackMappings (const juce::String& trackId);
    static void invalidateClipBake (const juce::String& clipId);
};

} // namespace vit
