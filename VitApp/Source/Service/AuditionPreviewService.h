#pragma once

#include <functional>
#include <memory>
#include <string>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "AuditionPreviewAudioPlane.h"
#include "AuditionPreviewState.h"

namespace vit
{

namespace te = tracktion;

/** Kernel/JUCE command adapter for the real target-scope Audition Preview Plane. */
class AuditionPreviewService final
{
public:
    using PublishAction = std::function<void (const juce::String&)>;
    using EditGetter = std::function<te::Edit*()>;

    AuditionPreviewService (te::Engine* engine,
                            EditGetter editGetter,
                            PublishAction publishAction = {});

    juce::String handlePrepare (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStatus (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleReady (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStale (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleFailed (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleSelect (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handlePosition (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleStop (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleInspectCandidate (const juce::DynamicObject& object, const juce::String& rawPayload);
    juce::String handleApplyCandidate (const juce::DynamicObject& object, const juce::String& rawPayload);

    AuditionPreviewAudioPlane& getAudioPlaneForTesting() noexcept { return audioPlane; }

private:
    static juce::String requiredString (const juce::DynamicObject& object,
                                        const char* property,
                                        const char* command,
                                        juce::String& error);
    juce::var sessionToVar (const audition::Session& session) const;
    juce::String resultToReply (const char* command, const audition::Result& result) const;
    static juce::String errorReply (const char* command, const juce::String& code, const juce::String& message);

    void publishStateEvent (const audition::Result& result, const char* fallbackType) const;
    void captureAuthoritativeTransport (audition::Session& session) const;

    EditGetter getEdit;
    audition::StateMachine state;
    AuditionPreviewAudioPlane audioPlane;
    PublishAction publish;
};

} // namespace vit
