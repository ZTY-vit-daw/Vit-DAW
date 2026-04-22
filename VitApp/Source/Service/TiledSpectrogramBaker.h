#pragma once

#include <functional>

#include <JuceHeader.h>

namespace vit
{

class TiledSpectrogramBaker final
{
public:
    using PublishCallback = std::function<void(const juce::String&)>;

    static void startBake (juce::String filePath,
                           juce::String trackId,
                           juce::String clipId,
                           PublishCallback publishCallback,
                           double sourceOffsetSeconds = 0.0,
                           double bakeLengthSeconds = -1.0);

    static void releaseTrackMappings (const juce::String& trackId);

    /** Bump bake generation and close SHM handles registered for this clip key (stops in-flight bakes). */
    static void invalidateClipBake (const juce::String& clipId);

private:
    static std::vector<int> buildFftToUiBin (double sampleRate);
};

} // namespace vit
