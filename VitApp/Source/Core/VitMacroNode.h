#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitMacroNode final
{
public:
    static juce::ValueTree createMacroNodeState (const juce::String& nodeId,
                                                 const juce::String& name,
                                                 float x,
                                                 float y,
                                                 int macroCount);
    static juce::var createSnapshot (const juce::ValueTree& nodeState);
};

} // namespace vit
