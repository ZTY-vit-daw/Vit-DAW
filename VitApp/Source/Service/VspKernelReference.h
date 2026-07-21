#pragma once

#include <functional>

#include <JuceHeader.h>

namespace vit
{

class VspKernelReference final
{
public:
    using LegacyCommandHandler = std::function<juce::String (const juce::var&, const juce::String&)>;
    using RealtimeDataProvider = std::function<juce::var (const juce::DynamicObject&)>;

    static bool isVspEnvelope (const juce::DynamicObject& object);
    static juce::String dispatchEnvelope (const juce::DynamicObject& envelope,
                                          const juce::String& rawPayload,
                                          LegacyCommandHandler legacyHandler,
                                          RealtimeDataProvider realtimeDataProvider = {});

private:
    VspKernelReference() = delete;
};

} // namespace vit
