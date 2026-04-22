#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitParamBinding final
{
public:
    static juce::ValueTree createBindingState (const juce::String& bindingId,
                                               const juce::String& sourceNodeId,
                                               const juce::String& sourceOutput,
                                               const juce::String& targetKind,
                                               const juce::String& targetPluginId,
                                               const juce::String& targetParamId,
                                               const juce::String& targetNodeId,
                                               const juce::String& targetInput,
                                               double minValue,
                                               double maxValue,
                                               const juce::String& curve);
    static juce::var createSnapshot (const juce::ValueTree& bindingState);
};

} // namespace vit
