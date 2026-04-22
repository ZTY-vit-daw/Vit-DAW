#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitAudioInjectorNode final
{
public:
    static juce::String defaultNodeRole() { return "audio_injector"; }
    static juce::String defaultZoneId()   { return "Z3"; }
};

} // namespace vit
