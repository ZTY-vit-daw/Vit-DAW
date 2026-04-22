#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitControlShell final
{
public:
    static juce::var buildShellDescriptor (const juce::String& templateRole,
                                           const juce::Array<juce::var>& recommendedGroups);
};

} // namespace vit
