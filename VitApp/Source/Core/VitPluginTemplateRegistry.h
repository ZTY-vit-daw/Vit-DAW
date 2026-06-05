#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitPluginTemplateRegistry final
{
public:
    static juce::String inferTemplateRole (te::Plugin&);
    static juce::String inferTemplateRoleFromParameterNames (const juce::StringArray& parameterNames,
                                                             const juce::String& fallbackRole);
    static juce::String inferNormalizedRole (const juce::String& parameterName,
                                             const juce::String& templateRole);
    static juce::String inferDisplayGroup (const juce::String& normalizedRole,
                                           const juce::String& templateRole);
    static juce::String inferControlRelevance (const juce::String& parameterName,
                                               const juce::String& normalizedRole);
    static int controlPriorityForRelevance (const juce::String& controlRelevance);
};

} // namespace vit
