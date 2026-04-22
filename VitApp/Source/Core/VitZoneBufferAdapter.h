#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitZoneBufferAdapterAdvice
{
    juce::String mode = "native";
    bool requiresDummyAudioPadding = false;
    bool passthroughMidi = false;
    bool passthroughAudio = false;
    juce::String note;
};

class VitZoneBufferAdapter final
{
public:
    static VitZoneBufferAdapterAdvice describeConnection (te::Plugin* sourcePlugin,
                                                          te::Plugin* destPlugin,
                                                          const juce::String& sourceZoneId,
                                                          const juce::String& destZoneId);
};

} // namespace vit
