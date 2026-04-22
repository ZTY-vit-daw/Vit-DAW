#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitProjectHealthCheck final
{
public:
    static juce::var createReport (const juce::File& projectFile, te::Edit& edit);
    static juce::var createExportPolicy (const juce::File& projectFile, te::Edit& edit);
};

} // namespace vit
