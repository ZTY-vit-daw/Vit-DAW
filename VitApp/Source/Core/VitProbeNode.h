#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitProbeNode final
{
public:
    static juce::Array<juce::var> createBuiltInProbeChannels();
};

} // namespace vit
