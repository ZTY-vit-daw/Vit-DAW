#include "VitHeadlessService.h"

#include "TiledSpectrogramBaker.h"

#include "../Core/VitEncryptionCore.h"
#include "../Core/VitGraphSwapCoordinator.h"
#include "../Core/VitKernelUtils.h"
#include "../Core/VitPaths.h"
#include "../Core/VitProjectFile.h"

#include <unordered_set>

namespace vit
{

namespace
{

constexpr float minimumTelemetryDb = -100.0f;
constexpr float maximumTelemetryDb = 0.0f;

float normaliseLevelDbForTelemetry (float rawLevelDb)
{
    const auto gain = juce::Decibels::decibelsToGain (rawLevelDb, minimumTelemetryDb);
    const auto dbfs = juce::Decibels::gainToDecibels (gain, minimumTelemetryDb);
    return juce::jlimit (minimumTelemetryDb, maximumTelemetryDb, dbfs);
}

bool ensureMonitoringPlugins (te::AudioTrack& track)
{
    auto& edit = track.pluginList.getEdit();
    bool changed = false;

    if (track.getVolumePlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (track.getLevelMeterPlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::LevelMeterPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (changed)
        track.flushStateToValueTree();

    return changed;
}

bool ensureMonitoringPluginsForEdit (te::Edit& edit)
{
    bool changed = false;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack != nullptr)
            changed = ensureMonitoringPlugins (*audioTrack) || changed;
    }

    if (changed)
    {
        edit.dispatchPendingUpdatesSynchronously();
        edit.getTransport().ensureContextAllocated (true);
    }

    return changed;
}

/**
 * createNodeForEdit 会跳过「解析不到 OutputDevice」或「设备未启用」的轨道；默认输出 ID 与工程里保存的
 * 路由在冷启动/设备列表刷新后可能短暂不一致。先校验默认设备，再把无路可走的音轨指回默认 wave out。
 */
void ensureAudioOutputsAndDefaultsForPlayback (te::Edit& loadedEdit)
{
    auto& dm = loadedEdit.engine.getDeviceManager();
    dm.checkDefaultDevicesAreValid();

    for (auto* at : te::getAudioTracks (loadedEdit))
    {
        if (at == nullptr)
            continue;

        auto& rout = at->getOutput();
        auto* dev = rout.getOutputDevice (false);

        if (dev == nullptr || ! dev->isEnabled())
        {
            rout.setOutputToDefaultDevice (false);
            at->flushStateToValueTree();
            juce::Logger::writeToLog ("VitHeadlessService: audio track " + at->itemID.toString()
                                      + " had no enabled output device; reset routing to default wave out");
        }
    }
}

/** 新 Edit 进内存后必须同步状态并强制重建播放节点，否则仍可能沿用空/旧图导致无声音输出。 */
void primeEditPlaybackGraph (te::Edit& loadedEdit)
{
    prepareTracktionAudioHardwareForPlayback (loadedEdit.engine);
    ensureAudioOutputsAndDefaultsForPlayback (loadedEdit);
    rebindAllWaveClipSourcesToDirectFiles (loadedEdit);
    loadedEdit.dispatchPendingUpdatesSynchronously();
    loadedEdit.getTransport().ensureContextAllocated (true);
}

static juce::Array<juce::var> collectLiveRecordingWaveformEntries (te::Edit& edit)
{
    juce::Array<juce::var> items;

    for (auto* idi : edit.getAllInputDevices())
    {
        if (idi == nullptr)
            continue;

        if (idi->getInputDevice().getDeviceType() != te::InputDevice::waveDevice)
            continue;

        for (auto targetID : idi->getTargets())
        {
            if (! idi->isRecording (targetID))
                continue;

            const auto recFile = idi->getRecordingFile (targetID);

            if (! recFile.existsAsFile())
                continue;

            auto thumbPtr = edit.engine.getRecordingThumbnailManager().getThumbnailFor (recFile);

            if (thumbPtr == nullptr || thumbPtr->thumb == nullptr)
                continue;

            auto* thumbBase = thumbPtr->thumb.get();
            const auto totalLen = thumbBase->getTotalLength();

            if (totalLen < 0.001)
                continue;

            const auto numBuckets = juce::jlimit (32, 900, juce::roundToInt (totalLen * 100.0));
            juce::Array<juce::var> peaksFlat;
            peaksFlat.ensureStorageAllocated (numBuckets * 2);

            constexpr int kMinMaxSubRanges = 8;
            for (int i = 0; i < numBuckets; ++i)
            {
                const auto t0 = (double) i * totalLen / (double) numBuckets;
                const auto t1 = (double) (i + 1) * totalLen / (double) numBuckets;
                float ch0Min = 0.0f, ch0Max = 0.0f;
                bool have0 = false;

                for (int s = 0; s < kMinMaxSubRanges; ++s)
                {
                    const auto st = t0 + (t1 - t0) * ((double) s / (double) kMinMaxSubRanges);
                    const auto en = t0 + (t1 - t0) * ((double) (s + 1) / (double) kMinMaxSubRanges);
                    float lo = 0.0f, hi = 0.0f;
                    thumbBase->getApproximateMinMax (st, en, 0, lo, hi);
                    if (! have0)
                    {
                        ch0Min = lo;
                        ch0Max = hi;
                        have0 = true;
                    }
                    else
                    {
                        ch0Min = juce::jmin (ch0Min, lo);
                        ch0Max = juce::jmax (ch0Max, hi);
                    }

                    if (thumbBase->getNumChannels() > 1)
                    {
                        thumbBase->getApproximateMinMax (st, en, 1, lo, hi);
                        ch0Min = juce::jmin (ch0Min, lo);
                        ch0Max = juce::jmax (ch0Max, hi);
                    }
                }

                peaksFlat.add (ch0Min);
                peaksFlat.add (ch0Max);
            }

            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("track_id", targetID.toString());
            row->setProperty ("duration_seconds", totalLen);
            row->setProperty ("peaks", juce::var (peaksFlat));
            items.add (juce::var (row.release()));
        }
    }

    return items;
}

static juce::String buildTransportTelemetryPayload (te::Edit& edit, bool includeRecordingWaveformDetail)
{
    auto response = std::make_unique<juce::DynamicObject>();
    auto& transport = edit.getTransport();

    response->setProperty ("topic", "transport");
    response->setProperty ("is_playing", transport.isPlaying());
    response->setProperty ("is_recording", transport.isRecording());
    response->setProperty ("position_seconds", transport.getPosition().inSeconds());

    if (transport.isRecording() && includeRecordingWaveformDetail)
        response->setProperty ("recording_waveforms", juce::var (collectLiveRecordingWaveformEntries (edit)));

    return juce::JSON::toString (juce::var (response.release()));
}

bool isAppBoundEncryption (const juce::DynamicObject& object)
{
    const auto enc = object.getProperty ("encryption");

    if (auto* encObj = enc.getDynamicObject())
        return encObj->getProperty ("mode").toString() == "app_bound_aes";

    return false;
}

} // namespace

VitHeadlessService::VitHeadlessService (juce::String applicationName)
    : engineDevice (std::move (applicationName))
{
    globalProjectConfig = std::make_unique<VitGlobalProjectConfig>();

    productionCoordinator = std::make_unique<VitProductionCoordinator> (
        [this](const juce::String& payload)
        {
            if (zmqGateway != nullptr)
                zmqGateway->publishMessage (payload);
        });

    commandDispatcher = std::make_unique<CommandDispatcher> (
        [this]() -> te::Edit*
        {
            return edit.get();
        },
        [this]()
        {
            return reloadProjectFromDefaultXml();
        },
        [this]()
        {
            return saveProjectToDefaultXml();
        },
        [this](const juce::String& payload)
        {
            if (zmqGateway != nullptr)
                zmqGateway->publishMessage (payload);
        },
        [this]()
        {
            return ipcGetRecentProjects();
        },
        [this]()
        {
            return ipcNewBlankProject();
        },
        [this](const juce::DynamicObject& object, const juce::File& projectFile)
        {
            return ipcOpenProjectAt (object, projectFile);
        },
        [this](const juce::DynamicObject& object)
        {
            return ipcSaveProjectWithPayload (object);
        },
        [this](const juce::DynamicObject& object, const juce::File& targetFile)
        {
            return ipcSaveAsProjectAt (object, targetFile);
        },
        [this]()
        {
            return currentProjectPath.getFullPathName();
        },
        productionCoordinator.get());
}

bool VitHeadlessService::start()
{
    callbackGuard->store (true);

    if (! reloadProjectFromDefaultXml())
        return false;

    zmqGateway = std::make_unique<ZmqGateway> ([this, guard = callbackGuard] (const juce::var& command, const juce::String& payload)
    {
        if (! guard->load())
            return CommandDispatcher::makeErrorReply ("Service is shutting down");

        return handleIncomingCommandOnMessageThread (command, payload);
    },
                                              &deltaHub.getBuffer());

    if (! zmqGateway->startGateway())
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to start ZeroMQ gateway.");
        zmqGateway.reset();
        edit.reset();
        return false;
    }

    startTimer (telemetryIntervalMs);
    juce::Logger::writeToLog ("VitHeadlessService: service started successfully.");
    return true;
}

void VitHeadlessService::stop()
{
    juce::Logger::writeToLog ("VitHeadlessService: stopping service.");

    callbackGuard->store (false);
    stopTimer();

    if (edit != nullptr)
        for (auto* track : te::getAllTracks (*edit))
            if (track != nullptr)
                TiledSpectrogramBaker::releaseTrackMappings (track->itemID.toString());

    if (zmqGateway != nullptr)
    {
        zmqGateway->stopGateway();
        zmqGateway.reset();
    }

    deltaHub.detach();
    clearLevelMeterClients();
    edit.reset();
}

bool VitHeadlessService::reloadProjectFromDefaultXml()
{
    const auto xmlFile = paths::ensureDefaultProjectXmlFileExists();

    if (! xmlFile.existsAsFile())
    {
        juce::Logger::writeToLog ("VitHeadlessService: project XML not found: " + xmlFile.getFullPathName());
        return false;
    }

    auto loadedEdit = loadEditFromXmlFile (engineDevice.getEngine(), xmlFile, 1, this);

    if (loadedEdit == nullptr)
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to load edit from XML.");
        return false;
    }

    clearLevelMeterClients();
    loadedEdit->playInStopEnabled = true;
    const auto monitoringPluginsAdded = ensureMonitoringPluginsForEdit (*loadedEdit);
    const auto trackRackChanged = ensureTrackRackGraphForEdit (*loadedEdit);
    primeEditPlaybackGraph (*loadedEdit);

    deltaHub.detach();
    edit = std::move (loadedEdit);
    lastTransportRecording = (edit != nullptr && edit->getTransport().isRecording());

    syncLevelMeterClients();

    if (edit != nullptr)
        deltaHub.attach (edit->state);

    if (monitoringPluginsAdded || trackRackChanged)
        saveProjectToDefaultXml();

    currentProjectPath = juce::File();
    juce::Logger::writeToLog ("VitHeadlessService: project reload complete.");
    return true;
}

bool VitHeadlessService::loadProjectFromFile (const juce::File& projectFile)
{
    if (! projectFile.existsAsFile())
    {
        juce::Logger::writeToLog ("VitHeadlessService: project file not found: " + projectFile.getFullPathName());
        return false;
    }

    if (! isRecognizedProjectExtension (projectFile))
        juce::Logger::writeToLog ("VitHeadlessService: 未在白名单内的工程扩展名，仍尝试按内容解析: "
                                  + projectFile.getFullPathName());

    std::unique_ptr<te::Edit> loadedEdit = te::loadEditFromFile (engineDevice.getEngine(), projectFile);

    if (loadedEdit == nullptr)
        loadedEdit = loadEditFromXmlFile (engineDevice.getEngine(), projectFile, 1, this);

    if (loadedEdit == nullptr)
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to load edit from " + projectFile.getFullPathName());
        return false;
    }

    return applyLoadedEdit (std::move (loadedEdit), juce::File (projectFile.getFullPathName()));
}

bool VitHeadlessService::loadEncryptedProjectFromFile (const juce::File& projectFile)
{
    if (! projectFile.existsAsFile())
    {
        juce::Logger::writeToLog ("VitHeadlessService: project file not found: " + projectFile.getFullPathName());
        return false;
    }

    juce::MemoryBlock mb;

    if (! projectFile.loadFileAsData (mb))
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to read encrypted project: " + projectFile.getFullPathName());
        return false;
    }

    const auto* bytes = static_cast<const uint8_t*> (mb.getData());
    std::vector<uint8_t> data (bytes, bytes + static_cast<size_t> (mb.getSize()));

    const auto xmlStr = VitEncryptionCore::decryptProject (data);

    if (xmlStr.isEmpty())
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to decrypt project " + projectFile.getFullPathName());
        return false;
    }

    auto loadedEdit = loadEditFromXmlString (engineDevice.getEngine(), xmlStr, 1, this, projectFile);

    if (loadedEdit == nullptr)
    {
        juce::Logger::writeToLog ("VitHeadlessService: failed to parse decrypted XML for " + projectFile.getFullPathName());
        return false;
    }

    return applyLoadedEdit (std::move (loadedEdit), juce::File (projectFile.getFullPathName()));
}

bool VitHeadlessService::applyLoadedEdit (std::unique_ptr<te::Edit> loadedEdit, const juce::File& openedProject)
{
    if (loadedEdit == nullptr)
        return false;

    if (! loadedEdit->editFileRetriever)
        loadedEdit->editFileRetriever = [openedProject]
        {
            return openedProject;
        };

    deltaHub.detach();

    if (edit != nullptr)
    {
        auto& oldTransport = edit->getTransport();

        if (oldTransport.isPlaying())
            oldTransport.stop (false, true);

        for (auto* track : te::getAllTracks (*edit))
            if (track != nullptr)
                TiledSpectrogramBaker::releaseTrackMappings (track->itemID.toString());
    }

    clearLevelMeterClients();
    edit.reset();

    loadedEdit->playInStopEnabled = true;
    const auto monitoringPluginsAdded = ensureMonitoringPluginsForEdit (*loadedEdit);
    const auto trackRackChanged = ensureTrackRackGraphForEdit (*loadedEdit);
    primeEditPlaybackGraph (*loadedEdit);

    edit = std::move (loadedEdit);
    lastTransportRecording = (edit != nullptr && edit->getTransport().isRecording());

    if (edit != nullptr)
    {
        const auto loadSummary = juce::String ("Edit loaded from ") + openedProject.getFullPathName();
        VitGraphSwapCoordinator::resetForEdit (*edit, loadSummary);
    }

    syncLevelMeterClients();

    if (edit != nullptr)
        deltaHub.attach (edit->state);

    if (monitoringPluginsAdded || trackRackChanged)
        saveProjectToDefaultXml();

    juce::Logger::writeToLog ("VitHeadlessService: loaded project from " + openedProject.getFullPathName());
    return true;
}

bool VitHeadlessService::saveProjectToDefaultXml()
{
    if (edit == nullptr)
    {
        juce::Logger::writeToLog ("VitHeadlessService: no active edit to save.");
        return false;
    }

    const auto xmlFile = paths::ensureDefaultProjectXmlFileExists();

    for (auto* track : te::getAllTracks (*edit))
        if (track != nullptr)
            track->flushStateToValueTree();

    if (auto xml = edit->state.createXml())
    {
        if (xmlFile.replaceWithText (xml->toString()))
        {
            juce::Logger::writeToLog ("VitHeadlessService: project saved to " + xmlFile.getFullPathName());
            return true;
        }
    }

    juce::Logger::writeToLog ("VitHeadlessService: failed to save project XML to " + xmlFile.getFullPathName());
    return false;
}

void VitHeadlessService::clearLevelMeterClients()
{
    for (auto& [trackID, registration] : trackLevelClients)
    {
        juce::ignoreUnused (trackID);

        if (registration.plugin != nullptr && registration.client != nullptr)
            registration.plugin->measurer.removeClient (*registration.client);
    }

    trackLevelClients.clear();
}

void VitHeadlessService::syncLevelMeterClients()
{
    if (edit == nullptr)
        return;

    std::unordered_set<std::string> activeTrackIDs;

    for (auto* track : te::getAllTracks (*edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        ensureMonitoringPlugins (*audioTrack);

        auto* meterPlugin = audioTrack->getLevelMeterPlugin();

        if (meterPlugin == nullptr)
            continue;

        const auto trackID = audioTrack->itemID.toString().toStdString();
        activeTrackIDs.insert (trackID);
        auto& registration = trackLevelClients[trackID];

        if (registration.plugin == meterPlugin && registration.client != nullptr)
            continue;

        if (registration.plugin != nullptr && registration.client != nullptr)
            registration.plugin->measurer.removeClient (*registration.client);

        registration.plugin = meterPlugin;
        registration.client = std::make_unique<te::LevelMeasurer::Client>();
        meterPlugin->measurer.addClient (*registration.client);
    }

    for (auto it = trackLevelClients.begin(); it != trackLevelClients.end();)
    {
        if (activeTrackIDs.contains (it->first))
        {
            ++it;
            continue;
        }

        if (it->second.plugin != nullptr && it->second.client != nullptr)
            it->second.plugin->measurer.removeClient (*it->second.client);

        it = trackLevelClients.erase (it);
    }
}

void VitHeadlessService::timerCallback()
{
    if (edit != nullptr)
        VitGraphSwapCoordinator::serviceGraphLifecycle (*edit);

    if (productionCoordinator != nullptr)
        productionCoordinator->tick();

    broadcastTelemetry();
}

void VitHeadlessService::broadcastTelemetry()
{
    if (zmqGateway == nullptr || edit == nullptr)
        return;

    broadcastTransportTelemetry();
    broadcastLevelsTelemetry();
}

void VitHeadlessService::broadcastTransportTelemetry()
{
    if (zmqGateway == nullptr || edit == nullptr)
        return;

    auto& transport = edit->getTransport();
    const bool nowRecording = transport.isRecording();

    if (lastTransportRecording && ! nowRecording)
    {
        auto stopped = std::make_unique<juce::DynamicObject>();
        stopped->setProperty ("topic", "recording");
        stopped->setProperty ("subtopic", "recording_stopped");
        stopped->setProperty ("position_seconds", transport.getPosition().inSeconds());
        zmqGateway->publishMessage (juce::JSON::toString (juce::var (stopped.release())));
    }

    lastTransportRecording = nowRecording;
    ++transportTelemetryTick;
    const bool includeLiveWaveform = ! nowRecording
                                    || (transportTelemetryTick % recordingWaveformTelemetryStride == 0);
    zmqGateway->publishMessage (buildTransportTelemetryPayload (*edit, includeLiveWaveform));
}

void VitHeadlessService::broadcastLevelsTelemetry()
{
    if (zmqGateway == nullptr || edit == nullptr)
        return;

    syncLevelMeterClients();

    auto response = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> tracksArray;

    for (auto* track : te::getAllTracks (*edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        float levelDb = -100.0f;

        if (const auto it = trackLevelClients.find (audioTrack->itemID.toString().toStdString());
            it != trackLevelClients.end() && it->second.client != nullptr)
        {
            const auto numChannels = juce::jmax (1, it->second.client->getNumChannelsUsed());

            for (int channel = 0; channel < juce::jmin (numChannels, 2); ++channel)
                levelDb = juce::jmax (levelDb,
                                      normaliseLevelDbForTelemetry (it->second.client->getAndClearAudioLevel (channel).dB));
        }

        auto trackObject = std::make_unique<juce::DynamicObject>();
        trackObject->setProperty ("id", audioTrack->itemID.toString());
        trackObject->setProperty ("level_db", levelDb);
        tracksArray.add (juce::var (trackObject.release()));
    }

    response->setProperty ("topic", "levels");
    response->setProperty ("tracks", juce::var (tracksArray));
    zmqGateway->publishMessage (juce::JSON::toString (juce::var (response.release())));
}

juce::String VitHeadlessService::handleIncomingCommandOnMessageThread (const juce::var& command, const juce::String& payload)
{
    jassert (juce::MessageManager::getInstance()->isThisTheMessageThread());
    juce::Logger::writeToLog ("VitHeadlessService: processing queued command on JUCE message thread: " + payload);
    const auto reply = commandDispatcher != nullptr
        ? commandDispatcher->dispatch (command, payload)
        : CommandDispatcher::makeErrorReply ("Command dispatcher unavailable");
    const auto preview = reply.substring (0, juce::jmin (220, reply.length()));
    juce::Logger::writeToLog ("VitHeadlessService: command reply (truncated): " + preview);
    return reply;
}

void VitHeadlessService::onEditReloaded (const juce::File& xmlFile)
{
    juce::Logger::writeToLog ("VitHeadlessService: edit loaded from " + xmlFile.getFullPathName());
}

void VitHeadlessService::onGhostTrackDetected (const GhostTrackDescriptor& descriptor)
{
    const auto intent = descriptor.intent.isNotEmpty() ? descriptor.intent : "(none)";
    juce::Logger::writeToLog ("VitHeadlessService: ghost track detected name=\""
                              + descriptor.trackName + "\" intent=\"" + intent + "\"");
}

juce::String VitHeadlessService::ipcGetRecentProjects()
{
    if (globalProjectConfig == nullptr)
        return CommandDispatcher::makeErrorReply ("Global project config unavailable");

    return globalProjectConfig->buildGetRecentProjectsReply();
}

juce::String VitHeadlessService::ipcNewBlankProject()
{
    auto loadedEdit = loadEditFromXmlString (engineDevice.getEngine(),
                                             paths::detail::getBlankProjectXmlTemplate(),
                                             0,
                                             this);

    if (loadedEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("new_project: failed to create blank edit");

    deltaHub.detach();

    if (edit != nullptr)
        for (auto* track : te::getAllTracks (*edit))
            if (track != nullptr)
                TiledSpectrogramBaker::releaseTrackMappings (track->itemID.toString());

    clearLevelMeterClients();
    edit.reset();

    loadedEdit->playInStopEnabled = true;
    const auto monitoringPluginsAdded = ensureMonitoringPluginsForEdit (*loadedEdit);
    const auto trackRackChanged = ensureTrackRackGraphForEdit (*loadedEdit);
    primeEditPlaybackGraph (*loadedEdit);

    edit = std::move (loadedEdit);
    lastTransportRecording = (edit != nullptr && edit->getTransport().isRecording());

    if (edit != nullptr)
        VitGraphSwapCoordinator::resetForEdit (*edit, "Blank project created");

    syncLevelMeterClients();

    if (edit != nullptr)
        deltaHub.attach (edit->state);

    if (monitoringPluginsAdded || trackRackChanged)
        saveProjectToDefaultXml();

    edit->getUndoManager().clearUndoHistory();
    currentProjectPath = juce::File();

    return CommandDispatcher::makeStatusReply ("ok", "Blank project created");
}

juce::String VitHeadlessService::ipcOpenProjectAt (const juce::DynamicObject& object, const juce::File& projectFile)
{
    const bool ok = isAppBoundEncryption (object) ? loadEncryptedProjectFromFile (projectFile)
                                                  : loadProjectFromFile (projectFile);

    if (! ok)
        return CommandDispatcher::makeErrorReply ("open_project failed for " + projectFile.getFullPathName());

    currentProjectPath = juce::File (projectFile.getFullPathName());

    if (globalProjectConfig != nullptr)
        globalProjectConfig->prependRecentProject (currentProjectPath);

    return CommandDispatcher::makeStatusReply ("ok", "Project opened");
}

juce::String VitHeadlessService::ipcSaveProjectWithPayload (const juce::DynamicObject& object)
{
    if (isAppBoundEncryption (object))
    {
        const juce::String pathStr = object.getProperty ("file_path").toString().trim();

        if (pathStr.isEmpty())
            return CommandDispatcher::makeErrorReply ("save_project with app_bound_aes requires a non-empty file_path");

        return ipcSaveEncryptedProjectToPath (juce::File (pathStr));
    }

    return ipcSaveProjectToCurrentPath();
}

juce::String VitHeadlessService::ipcSaveEncryptedProjectToPath (const juce::File& fileFromPayload)
{
    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    const auto saveTarget = normalizeProjectPathForSave (fileFromPayload);

    if (! saveTarget.getFileExtension().equalsIgnoreCase (".vit"))
        return CommandDispatcher::makeErrorReply ("Encrypted project must be saved with a .vit file extension");

    const auto parentDir = saveTarget.getParentDirectory();

    if (! parentDir.isDirectory())
        if (! parentDir.createDirectory())
            return CommandDispatcher::makeErrorReply ("save_project: cannot create directory " + parentDir.getFullPathName());

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    if (auto xml = activeEdit->state.createXml())
    {
        const auto blob = VitEncryptionCore::encryptProject (xml->toString());

        if (blob.empty())
            return CommandDispatcher::makeErrorReply ("save_project: encryption failed");

        if (! saveTarget.replaceWithData (blob.data(), blob.size()))
            return CommandDispatcher::makeErrorReply ("save_project: failed to write " + saveTarget.getFullPathName());

        currentProjectPath = juce::File (saveTarget.getFullPathName());

        if (globalProjectConfig != nullptr)
            globalProjectConfig->prependRecentProject (currentProjectPath);

        return CommandDispatcher::makeStatusReply ("ok", "Project saved");
    }

    return CommandDispatcher::makeErrorReply ("save_project: failed to serialize edit state");
}

juce::String VitHeadlessService::ipcSaveProjectToCurrentPath()
{
    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    if (currentProjectPath.getFullPathName().isEmpty())
        return CommandDispatcher::makeStatusReply ("require_path", "Please prompt Save As");

    const auto parentDir = currentProjectPath.getParentDirectory();

    if (! parentDir.isDirectory())
        if (! parentDir.createDirectory())
            return CommandDispatcher::makeErrorReply ("save_project: cannot create directory " + parentDir.getFullPathName());

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    const auto saveTarget = normalizeProjectPathForSave (currentProjectPath);

    if (! te::EditFileOperations (*activeEdit).saveAs (saveTarget, true))
        return CommandDispatcher::makeErrorReply ("save_project: failed to write " + saveTarget.getFullPathName());

    currentProjectPath = saveTarget;

    if (isVitDumpClearXmlEnabled())
        if (auto xml = activeEdit->state.createXml())
            writeClearXmlSidecarSilently (saveTarget, *xml);

    return CommandDispatcher::makeStatusReply ("ok", "Project saved");
}

juce::String VitHeadlessService::ipcSaveAsProjectAt (const juce::DynamicObject& object, const juce::File& targetFile)
{
    if (isAppBoundEncryption (object))
        return ipcSaveEncryptedProjectToPath (targetFile);

    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    const auto parentDir = targetFile.getParentDirectory();

    if (! parentDir.isDirectory())
        if (! parentDir.createDirectory())
            return CommandDispatcher::makeErrorReply ("save_as_project: cannot create directory " + parentDir.getFullPathName());

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    const auto saveTarget = normalizeProjectPathForSave (targetFile);

    if (! te::EditFileOperations (*activeEdit).saveAs (saveTarget, true))
        return CommandDispatcher::makeErrorReply ("save_as_project: failed to write " + saveTarget.getFullPathName());

    currentProjectPath = juce::File (saveTarget.getFullPathName());

    if (globalProjectConfig != nullptr)
        globalProjectConfig->prependRecentProject (currentProjectPath);

    if (isVitDumpClearXmlEnabled())
        if (auto xml = activeEdit->state.createXml())
            writeClearXmlSidecarSilently (saveTarget, *xml);

    return CommandDispatcher::makeStatusReply ("ok", "Project saved");
}

} // namespace vit
