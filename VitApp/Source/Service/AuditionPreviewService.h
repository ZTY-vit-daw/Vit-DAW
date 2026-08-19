#pragma once

#include <functional>
#include <string>

#include <JuceHeader.h>

#include "AuditionPreviewState.h"

namespace vit
{

/** Kernel/JUCE command adapter for the G4 Audition Preview Plane spike. */
class AuditionPreviewService final
{
public:
    using PublishAction = std::function<void (const juce::String&)>;

    explicit AuditionPreviewService (PublishAction publishAction = {});

    juce::String handlePrepare (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStatus (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleReady (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStale (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleFailed (const juce::DynamicObject& object, const juce::String& rawPayload);

    juce::String handleSelect (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handlePosition (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStop (const juce::DynamicObject& object, const juce::String& rawPayload);

private:
    static juce::String requiredString (const juce::DynamicObject& object,
                                        const char* property,
                                        const char* command,
                                        juce::String& error);
    static juce::var sessionToVar (const audition::Session& session);
    static juce::String resultToReply (const char* command, const audition::Result& result);
    static juce::String errorReply (const char* command, const juce::String& code, const juce::String& message);

    void publishStateEvent (const audition::Result& result, const char* fallbackType) const;

    audition::StateMachine state;
    PublishAction publish;
};

} // namespace vit
