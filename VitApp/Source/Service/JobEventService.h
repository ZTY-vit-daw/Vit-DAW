#pragma once

#include <functional>

#include <JuceHeader.h>

namespace vit
{

class JobEventService final
{
public:
    using CurrentProjectPathGetter = std::function<juce::String()>;

    explicit JobEventService (CurrentProjectPathGetter currentProjectPathGetter);

    juce::String handleAigcRegisterJob (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    CurrentProjectPathGetter getCurrentProjectPath;
};

} // namespace vit
