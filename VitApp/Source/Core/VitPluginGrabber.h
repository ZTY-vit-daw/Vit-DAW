#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitPluginGrabber final
{
public:
    static juce::String inferTemplateRoleForParameters (te::ExternalPlugin&,
                                                        const juce::String& fallbackRole);
    static juce::Array<juce::var> buildParameterDescriptors (te::ExternalPlugin&,
                                                             const juce::String& templateRole);
    static juce::Array<juce::var> buildRecommendedGroups (const juce::Array<juce::var>& parameterDescriptors);
    static juce::Array<juce::var> buildQuickControls (const juce::Array<juce::var>& parameterDescriptors,
                                                      const juce::String& templateRole);
};

} // namespace vit
