#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitNodeProfiler final
{
public:
    static juce::var createProjectProfile (te::Edit& edit,
                                           const juce::Array<juce::var>& jobs,
                                           const juce::Array<juce::var>& assets,
                                           const juce::Array<juce::var>& takeHistories);
};

} // namespace vit
