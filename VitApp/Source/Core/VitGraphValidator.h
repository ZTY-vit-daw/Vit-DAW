#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitGraphValidationResult
{
    bool allowed = false;
    juce::String errorMessage;
    juce::String sourceZoneId;
    juce::String destZoneId;
};

class VitGraphValidator final
{
public:
    static juce::String normaliseZoneId (const juce::String& zoneId);
    static bool isZoneIdAllowedForNodeCreation (const juce::String& zoneId);
    static juce::String getZoneIdForNode (te::RackType&, te::EditItemID, te::Plugin* fallbackPlugin = nullptr);
    static VitGraphValidationResult validateConnection (te::RackType&, te::EditItemID sourceId, te::EditItemID destId);
};

} // namespace vit
