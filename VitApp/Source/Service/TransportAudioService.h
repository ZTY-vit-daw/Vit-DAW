#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitProductionCoordinator;

class TransportAudioService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    TransportAudioService (EditGetter editGetter,
                           SaveProjectAction saveProjectAction,
                           VitProductionCoordinator* productionCoordinator);

    juce::String handlePlay (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStop (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleReturnToZero (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleTransportOptionStopReturnToStart (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleToggleClick (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetClick (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSeek (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetAudioDeviceTypes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetAudioDevices (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAudioDevice (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetWaveInputDevices (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRouteWaveInputToTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleArmTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStartRecording (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStopRecording (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleFreezeTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUnfreezeTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStartRender (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCancelRender (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
    VitProductionCoordinator* production = nullptr;
    mutable bool stopReturnsToZero = false;
};

} // namespace vit
