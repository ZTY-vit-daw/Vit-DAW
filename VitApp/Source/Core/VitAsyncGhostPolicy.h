#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitAsyncGhostPolicy final
{
public:
    static juce::String normalise (const juce::String& policy);
    static bool isValid (const juce::String& policy);
};

} // namespace vit
