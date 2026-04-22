#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitGraphTrace final
{
public:
    static juce::var createObservabilitySnapshot (const juce::File& projectFile, te::Edit& edit);
};

} // namespace vit
