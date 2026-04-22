#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitParamSurface final
{
public:
    static juce::String normaliseControlKind (const juce::String& kind);
    static juce::StringArray supportedControlKinds();
    static juce::Identifier valuePropertyForOutput (const juce::String& outputName);
    static juce::ValueTree createControlNodeState (const juce::String& nodeId,
                                                   const juce::String& kind,
                                                   const juce::String& name,
                                                   float x,
                                                   float y);
    static juce::var createNodeSnapshot (const juce::ValueTree& nodeState);
};

} // namespace vit
