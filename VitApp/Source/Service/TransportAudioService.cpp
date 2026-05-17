#include "TransportAudioService.h"

#include "VitProductionCoordinator.h"

namespace vit
{

namespace
{

juce::String buildTransportReply (te::Edit& edit, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    auto& transport = edit.getTransport();

    response->setProperty ("status", "ok");
    response->setProperty ("message", message);
    response->setProperty ("is_playing", transport.isPlaying());
    response->setProperty ("is_recording", transport.isRecording());
    response->setProperty ("position_seconds", transport.getPosition().inSeconds());
    response->setProperty ("click_track_enabled", static_cast<bool> (edit.clickTrackEnabled.get()));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String describeTransportState (te::Edit& edit)
{
    auto& transport = edit.getTransport();
    return "isPlaying=" + juce::String (transport.isPlaying() ? "true" : "false")
        + " isRecording=" + juce::String (transport.isRecording() ? "true" : "false")
        + " playContextActive=" + juce::String (transport.isPlayContextActive() ? "true" : "false")
        + " positionSeconds=" + juce::String (transport.getPosition().inSeconds(), 3)
        + " editLengthSeconds=" + juce::String (edit.getLength().inSeconds(), 3);
}

bool vitEditAnyInputsRecording (te::Edit& edit)
{
    for (auto* in : edit.getAllInputDevices())
        if (in != nullptr && in->isRecordingActive())
            return true;

    return false;
}

te::AudioTrack* findAudioTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack != nullptr && audioTrack->itemID.toString() == trackID)
            return audioTrack;
    }

    return nullptr;
}

juce::StringArray mergeUniqueDeviceNames (juce::AudioIODeviceType* dtype)
{
    juce::StringArray merged;

    if (dtype == nullptr)
        return merged;

    dtype->scanForDevices();

    for (auto wantInput : { false, true })
    {
        const juce::StringArray names = dtype->getDeviceNames (wantInput);

        for (int i = 0; i < names.size(); ++i)
        {
            const auto& n = names[i];

            if (n.isNotEmpty())
                merged.addIfNotAlreadyThere (n);
        }
    }

    return merged;
}

juce::String currentDeviceSummary (const juce::AudioDeviceManager::AudioDeviceSetup& setup)
{
    if (setup.outputDeviceName.isNotEmpty())
        return setup.outputDeviceName;

    return setup.inputDeviceName;
}

void appendBufferSizesFromDevice (juce::AudioIODevice* dev, juce::Array<juce::var>& out)
{
    if (dev == nullptr)
        return;

    const auto sizes = dev->getAvailableBufferSizes();

    for (int i = 0; i < sizes.size(); ++i)
        out.add (sizes[i]);

    const int def = dev->getDefaultBufferSize();

    if (def <= 0)
        return;

    bool has = false;

    for (const auto& v : out)
    {
        if (static_cast<int> (v) == def)
        {
            has = true;
            break;
        }
    }

    if (! has)
        out.add (def);
}

void appendSampleRatesFromDevice (juce::AudioIODevice* dev, juce::Array<juce::var>& out)
{
    if (dev == nullptr)
        return;

    const auto rates = dev->getAvailableSampleRates();

    for (int i = 0; i < rates.size(); ++i)
        out.add (rates[i]);
}

} // namespace

TransportAudioService::TransportAudioService (EditGetter editGetter,
                                              SaveProjectAction saveProjectAction,
                                              VitProductionCoordinator* productionCoordinator)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      production (productionCoordinator)
{
}

juce::String TransportAudioService::handlePlay (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("TransportAudioService::handlePlay: before " + describeTransportState (*edit));
    transport.ensureContextAllocated();
    transport.play (false);
    juce::Logger::writeToLog ("TransportAudioService::handlePlay: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport playing");
}

juce::String TransportAudioService::handleStop (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("TransportAudioService::handleStop: before " + describeTransportState (*edit));
    transport.stop (false, false);
    if (stopReturnsToZero)
        transport.setPosition (te::TimePosition::fromSeconds (0.0));

    transport.ensureContextAllocated();
    juce::Logger::writeToLog ("TransportAudioService::handleStop: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport stopped");
}

juce::String TransportAudioService::handleReturnToZero (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("TransportAudioService::handleReturnToZero: before " + describeTransportState (*edit));
    transport.stop (false, false);
    transport.setPosition (te::TimePosition::fromSeconds (0.0));
    transport.ensureContextAllocated();
    juce::Logger::writeToLog ("TransportAudioService::handleReturnToZero: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport returned to zero");
}

juce::String TransportAudioService::handleTransportOptionStopReturnToStart (const juce::DynamicObject& object, const juce::String&) const
{
    const auto valueVar = object.getProperty ("value");

    if (! valueVar.isBool())
        return makeErrorReply ("transport_option_stop_return_to_start requires a boolean value field");

    stopReturnsToZero = static_cast<bool> (valueVar);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleToggleClick (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    edit->clickTrackEnabled = ! static_cast<bool> (edit->clickTrackEnabled.get());
    edit->getTransport().ensureContextAllocated();

    return buildTransportReply (*edit,
                                static_cast<bool> (edit->clickTrackEnabled.get())
                                    ? "Click track enabled"
                                    : "Click track disabled");
}

juce::String TransportAudioService::handleSetClick (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto enabledVar = object.getProperty ("enabled");

    if (! enabledVar.isBool())
        return makeErrorReply ("set_click requires a boolean enabled field");

    const auto enabled = static_cast<bool> (enabledVar);
    edit->clickTrackEnabled = enabled;
    edit->getTransport().ensureContextAllocated();

    return buildTransportReply (*edit, enabled ? "Click track enabled" : "Click track disabled");
}

juce::String TransportAudioService::handleSeek (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto timeVar = object.getProperty ("time");

    if (! timeVar.isDouble() && ! timeVar.isInt() && ! timeVar.isInt64())
        return makeErrorReply ("seek requires a numeric time field");

    const auto targetTime = juce::jmax (0.0, static_cast<double> (timeVar));
    auto& transport = edit->getTransport();
    transport.setPosition (te::TimePosition::fromSeconds (targetTime));
    juce::Logger::writeToLog ("TransportAudioService::handleSeek: position set to " + juce::String (targetTime, 4) + "s");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Seek executed");
    response->setProperty ("position_seconds", targetTime);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleGetAudioDeviceTypes (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& jdm = edit->engine.getDeviceManager().deviceManager;
    juce::Array<juce::var> types;
    const auto& available = jdm.getAvailableDeviceTypes();

    for (int i = 0; i < available.size(); ++i)
    {
        auto* t = available[i];

        if (t != nullptr && t->getTypeName().isNotEmpty())
            types.add (t->getTypeName());
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("types", juce::var (types));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleGetAudioDevices (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto typeStr = object.getProperty ("type").toString().trim();

    if (typeStr.isEmpty())
        return makeErrorReply ("get_audio_devices requires a non-empty type field");

    auto& jdm = edit->engine.getDeviceManager().deviceManager;

    if (jdm.getCurrentAudioDeviceType() != typeStr)
        jdm.setCurrentAudioDeviceType (typeStr, true);

    auto* dtype = jdm.getCurrentDeviceTypeObject();
    const juce::StringArray availableDevices = mergeUniqueDeviceNames (dtype);
    const juce::StringArray outputDeviceNames = dtype != nullptr ? dtype->getDeviceNames (false) : juce::StringArray();
    const juce::StringArray inputDeviceNames = dtype != nullptr ? dtype->getDeviceNames (true) : juce::StringArray();

    #if JUCE_DEBUG
    DBG ("[Vit] get_audio_devices type=" << typeStr << " inputs=" << inputDeviceNames.size() << " outputs=" << outputDeviceNames.size());
    #endif

    juce::Array<juce::var> devicesJson;
    juce::Array<juce::var> outNamesJson;
    juce::Array<juce::var> inNamesJson;

    for (int i = 0; i < availableDevices.size(); ++i)
        devicesJson.add (availableDevices[i]);

    for (int i = 0; i < outputDeviceNames.size(); ++i)
        outNamesJson.add (outputDeviceNames[i]);

    for (int i = 0; i < inputDeviceNames.size(); ++i)
        inNamesJson.add (inputDeviceNames[i]);

    const auto setup = jdm.getAudioDeviceSetup();

    juce::Array<juce::var> sampleRatesJson;
    juce::Array<juce::var> bufferSizesJson;

    if (auto* currentDevice = jdm.getCurrentAudioDevice())
    {
        appendSampleRatesFromDevice (currentDevice, sampleRatesJson);
        appendBufferSizesFromDevice (currentDevice, bufferSizesJson);
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("current_device", currentDeviceSummary (setup));
    response->setProperty ("current_output_device", setup.outputDeviceName);
    response->setProperty ("current_input_device", setup.inputDeviceName);
    response->setProperty ("current_sample_rate", setup.sampleRate);
    response->setProperty ("current_buffer_size", setup.bufferSize);
    response->setProperty ("available_devices", juce::var (devicesJson));
    response->setProperty ("available_output_devices", juce::var (outNamesJson));
    response->setProperty ("available_input_devices", juce::var (inNamesJson));
    response->setProperty ("available_sample_rates", juce::var (sampleRatesJson));
    response->setProperty ("available_buffer_sizes", juce::var (bufferSizesJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleSetAudioDevice (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto typeStr = object.getProperty ("type").toString().trim();
    const auto legacyName = object.getProperty ("device_name").toString().trim();
    const auto outNameProp = object.getProperty ("output_device_name").toString().trim();
    const auto inNameProp = object.getProperty ("input_device_name").toString().trim();
    const auto srVar = object.getProperty ("sample_rate");
    const auto bsVar = object.getProperty ("buffer_size");

    if (typeStr.isEmpty())
        return makeErrorReply ("set_audio_device requires a non-empty type field");

    if (! srVar.isDouble() && ! srVar.isInt() && ! srVar.isInt64())
        return makeErrorReply ("set_audio_device requires a numeric sample_rate field");

    if (! bsVar.isInt() && ! bsVar.isInt64() && ! bsVar.isDouble())
        return makeErrorReply ("set_audio_device requires a numeric buffer_size field");

    auto& jdm = edit->engine.getDeviceManager().deviceManager;
    const auto& availableTypes = jdm.getAvailableDeviceTypes();
    juce::AudioIODeviceType* matchedType = nullptr;

    for (int i = 0; i < availableTypes.size(); ++i)
    {
        auto* typeObj = availableTypes[i];

        if (typeObj != nullptr && typeObj->getTypeName() == typeStr)
        {
            matchedType = typeObj;
            break;
        }
    }

    if (matchedType == nullptr)
        return makeErrorReply ("Invalid device type");

    matchedType->scanForDevices();
    const auto outputDevices = matchedType->getDeviceNames (false);
    const auto inputDevices = matchedType->getDeviceNames (true);
    const auto availableDevices = mergeUniqueDeviceNames (matchedType);

    juce::String outputPick;
    juce::String inputPick;

    if (outNameProp.isNotEmpty() || inNameProp.isNotEmpty())
    {
        if (outNameProp.isNotEmpty())
        {
            if (! outputDevices.contains (outNameProp))
                return makeErrorReply ("output_device_name is not a valid output device for the specified type");

            outputPick = outNameProp;
        }

        if (inNameProp.isNotEmpty())
        {
            if (! inputDevices.contains (inNameProp))
                return makeErrorReply ("input_device_name is not a valid input device for the specified type");

            inputPick = inNameProp;
        }
    }
    else if (legacyName.isNotEmpty())
    {
        if (! availableDevices.contains (legacyName))
            return makeErrorReply ("Device name not found in the specified type");

        if (! outputDevices.contains (legacyName))
            return makeErrorReply ("Device name is not a valid output device for the specified type");

        outputPick = legacyName;

        if (inputDevices.contains (legacyName))
            inputPick = legacyName;
    }
    else
    {
        return makeErrorReply ("set_audio_device requires device_name or output_device_name and/or input_device_name");
    }

    auto setup = jdm.getAudioDeviceSetup();

    if (outputPick.isNotEmpty())
        setup.outputDeviceName = outputPick;

    if (inputPick.isNotEmpty())
        setup.inputDeviceName = inputPick;

    setup.sampleRate = static_cast<double> (srVar);
    setup.bufferSize = static_cast<int> (bsVar);

    te::Engine* enginePtr = &edit->engine;

    juce::MessageManager::callAsync ([enginePtr, typeStr, setup]() mutable
                                     {
                                         if (enginePtr == nullptr)
                                             return;

                                         auto& jdm = enginePtr->getDeviceManager().deviceManager;

                                         if (typeStr.isNotEmpty() && jdm.getCurrentAudioDeviceType() != typeStr)
                                             jdm.setCurrentAudioDeviceType (typeStr, true);

                                         const juce::String err = jdm.setAudioDeviceSetup (setup, true);

                                         if (err.isNotEmpty())
                                             juce::Logger::writeToLog ("set_audio_device (async): " + err);
                                     });

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Device change requested");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleGetWaveInputDevices (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    juce::Array<juce::var> arr;
    auto& dm = edit->engine.getDeviceManager();

    for (auto* w : dm.getWaveInputDevices())
    {
        if (w == nullptr || w->getDeviceType() != te::InputDevice::waveDevice)
            continue;

        auto* row = new juce::DynamicObject();
        row->setProperty ("device_id", w->getDeviceID());
        row->setProperty ("name", w->getName());
        row->setProperty ("alias", w->getAlias());
        arr.add (juce::var (row));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("devices", juce::var (arr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleRouteWaveInputToTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto deviceKey = object.getProperty ("device_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("route_wave_input_to_track requires track_id");

    auto* audioTrack = findAudioTrackByID (*edit, trackId);

    if (audioTrack == nullptr)
        return makeErrorReply ("route_wave_input_to_track: audio track not found");

    te::WaveInputDevice* chosen = nullptr;
    auto& dm = edit->engine.getDeviceManager();
    auto waveInputs = dm.getWaveInputDevices();

    for (auto* w : waveInputs)
    {
        if (w == nullptr || w->getDeviceType() != te::InputDevice::waveDevice)
            continue;

        if (deviceKey.isEmpty())
        {
            chosen = w;
            break;
        }

        if (w->getDeviceID() == deviceKey || w->getName() == deviceKey)
        {
            chosen = w;
            break;
        }
    }

    if (chosen == nullptr)
        return makeErrorReply ("route_wave_input_to_track: no matching hardware wave input (check device_id or open audio device)");

    chosen->setEnabled (true);

    auto& transport = edit->getTransport();
    transport.ensureContextAllocated (true);
    auto* ctx = edit->getCurrentPlaybackContext();

    if (ctx == nullptr)
        return makeErrorReply ("route_wave_input_to_track: no playback context");

    edit->getEditInputDevices().getInstanceStateForInputDevice (*chosen);

    auto* inst = ctx->getInputFor (chosen);

    if (inst == nullptr)
        return makeErrorReply ("route_wave_input_to_track: failed to resolve input instance");

    const bool preserveRecordArmed = inst->isRecordingEnabled (audioTrack->itemID);

    const auto targetResult = inst->setTarget (audioTrack->itemID, false, &edit->getUndoManager());

    if (! targetResult.has_value())
        return makeErrorReply ("route_wave_input_to_track: " + targetResult.error());

    if (preserveRecordArmed)
        inst->setRecordingEnabled (audioTrack->itemID, true);

    audioTrack->getWaveInputDevice().setEnabled (true);
    edit->dispatchPendingUpdatesSynchronously();
    transport.ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Routing saved in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("track_id", trackId);
    response->setProperty ("device_id", chosen->getDeviceID());
    response->setProperty ("message", "Wave input routed to track");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleArmTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("arm_track requires track_id");

    auto* audioTrack = findAudioTrackByID (*edit, trackId);

    if (audioTrack == nullptr)
        return makeErrorReply ("arm_track: audio track not found");

    const auto armedVar = object.getProperty ("is_armed");
    const bool armed = armedVar.isBool() ? static_cast<bool> (armedVar)
                                         : armedVar.toString() == "1";

    auto& transport = edit->getTransport();
    transport.ensureContextAllocated (true);
    auto* ctx = edit->getCurrentPlaybackContext();

    if (ctx == nullptr)
        return makeErrorReply ("arm_track: no playback context");

    auto& waveDev = audioTrack->getWaveInputDevice();

    if (ctx->getInputFor (&waveDev) == nullptr)
        ctx->addWaveInputDeviceInstance (waveDev);

    for (auto* in : edit->getAllInputDevices())
        if (in != nullptr)
            in->setRecordingEnabled (audioTrack->itemID, armed);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("track_id", trackId);
    response->setProperty ("is_armed", armed);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleStartRecording (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    transport.ensureContextAllocated (true);

    const bool anyInputsBefore = vitEditAnyInputsRecording (*edit);
    juce::Logger::writeToLog ("[Vit][record] start_recording (before transport.record): anyInputsRecording="
                              + juce::String (anyInputsBefore ? "true" : "false") + " "
                              + describeTransportState (*edit));

    transport.record (false, false);

    const bool anyInputsAfter = vitEditAnyInputsRecording (*edit);
    juce::Logger::writeToLog ("[Vit][record] start_recording (after transport.record): anyInputsRecording="
                              + juce::String (anyInputsAfter ? "true" : "false") + " "
                              + describeTransportState (*edit));

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Recording started");
    response->setProperty ("is_playing", transport.isPlaying());
    response->setProperty ("is_recording", transport.isRecording());
    response->setProperty ("position_seconds", transport.getPosition().inSeconds());
    response->setProperty ("click_track_enabled", static_cast<bool> (edit->clickTrackEnabled.get()));
    response->setProperty ("any_inputs_recording_active_before", anyInputsBefore);
    response->setProperty ("any_inputs_recording_active_after", anyInputsAfter);
    response->setProperty ("play_context_active", transport.isPlayContextActive());
    if (! anyInputsBefore)
        response->setProperty (
            "record_hint",
            "No input had record-enabled destinations when start_recording ran; arm_track / route_wave_input may be missing or mismatched.");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleStopRecording (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();

    juce::Logger::writeToLog ("[Vit][record] stop_recording (before): " + describeTransportState (*edit));

    if (transport.isRecording())
        transport.stopRecording (false);

    juce::Logger::writeToLog ("[Vit][record] stop_recording (after): " + describeTransportState (*edit));

    return buildTransportReply (*edit, "Recording stopped");
}

juce::String TransportAudioService::handleFreezeTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("freeze_track requires track_id");

    auto* audioTrack = findAudioTrackByID (*edit, trackId);

    if (audioTrack == nullptr)
        return makeErrorReply ("freeze_track: audio track not found");

    audioTrack->freezeTrackAsync();

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("track_id", trackId);
    response->setProperty ("message", "Freeze scheduled");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleUnfreezeTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("unfreeze_track requires track_id");

    auto* audioTrack = findAudioTrackByID (*edit, trackId);

    if (audioTrack == nullptr)
        return makeErrorReply ("unfreeze_track: audio track not found");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Unfreeze track");
    audioTrack->setFrozen (false, te::Track::anyFreeze);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("track_id", trackId);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::handleStartRender (const juce::DynamicObject& object, const juce::String&) const
{
    if (production == nullptr)
        return makeErrorReply ("Offline render coordinator unavailable");

    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto path = object.getProperty ("file_path").toString().trim();

    if (path.isEmpty())
        return makeErrorReply ("start_render requires file_path");

    double startSec = 0.0;
    double endSec = edit->getLength().inSeconds();
    const auto rangeVar = object.getProperty ("range");

    if (auto* arr = rangeVar.getArray())
    {
        if (arr->size() >= 2)
        {
            startSec = static_cast<double> (arr->getReference (0));
            endSec = static_cast<double> (arr->getReference (1));
        }
    }

    int bitDepth = 24;

    if (object.hasProperty ("bit_depth"))
        bitDepth = juce::jmax (16, static_cast<int> (object.getProperty ("bit_depth")));

    bool useMasterPlugins = true;

    if (object.hasProperty ("use_master_plugins"))
        useMasterPlugins = static_cast<bool> (object.getProperty ("use_master_plugins"));

    return production->startOfflineRender (*edit, juce::File (path), startSec, endSec, bitDepth, useMasterPlugins);
}

juce::String TransportAudioService::handleCancelRender (const juce::DynamicObject&, const juce::String&) const
{
    if (production == nullptr)
        return makeErrorReply ("Offline render coordinator unavailable");

    production->cancelOfflineRender();
    return makeStatusReply ("ok", "Render cancel requested");
}

juce::String TransportAudioService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TransportAudioService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
