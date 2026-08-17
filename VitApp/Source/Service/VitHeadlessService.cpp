#include "VitHeadlessService.h"

#include "AudioFeatureService.h"
#include "ProjectAudioSettingsService.h"

#include "../Core/VitEncryptionCore.h"
#include "../Core/VitGraphSwapCoordinator.h"
#include "../Core/VitKernelUtils.h"
#include "../Core/VitPaths.h"
#include "../Core/VitProjectFile.h"

#include <unordered_map>
#include <unordered_set>
#include <vector>

namespace vit
{

namespace
{

constexpr float minimumTelemetryDb = -100.0f;
constexpr float maximumTelemetryDb = 0.0f;
constexpr int maxVspVisibleTracks = 17;
constexpr auto vitProjectUUIDProperty = "vit_project_uuid";
constexpr auto vitProjectParentUUIDProperty = "vit_project_parent_uuid";
constexpr auto vitAgentHistoryGenerationProperty = "vit_agent_history_generation";
constexpr auto vitAnalysisManifestProperty = "vit_analysis_manifest_json";

juce::String normaliseLegacyProjectID (const juce::String& projectID)
{
    auto safe = projectID.trim().toLowerCase().retainCharacters ("abcdefghijklmnopqrstuvwxyz0123456789");
    return safe.isNotEmpty() ? "vitproj_legacy_" + safe : juce::String();
}

juce::String ensureVitProjectUUID (te::Edit& edit, bool forceNew = false)
{
    auto projectUUID = edit.state.getProperty (vitProjectUUIDProperty).toString().trim();

    if (forceNew || projectUUID.isEmpty())
    {
        if (forceNew)
            projectUUID.clear();
        else
            projectUUID = normaliseLegacyProjectID (edit.state.getProperty ("projectID").toString());

        if (projectUUID.isEmpty())
            projectUUID = "vitproj_" + juce::Uuid().toString().toLowerCase();

        edit.state.setProperty (vitProjectUUIDProperty, projectUUID, nullptr);
    }

    return projectUUID;
}

juce::String makeProjectLifecycleReply (const juce::String& message,
                                        const juce::String& lifecycle,
                                        const juce::File& projectPath,
                                        const juce::String& projectUUID,
                                        const juce::String& sourceProjectUUID = {},
                                        const juce::String& parentProjectUUID = {},
                                        const juce::String& agentHistoryGeneration = {},
                                        const juce::String& historyPrepareID = {})
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", message);
    response->setProperty ("project_lifecycle", lifecycle);
    response->setProperty ("project_path", projectPath.getFullPathName());
    response->setProperty ("current_project_path", projectPath.getFullPathName());
    response->setProperty ("project_uuid", projectUUID);
    response->setProperty ("project_id", projectUUID);
    if (sourceProjectUUID.isNotEmpty())
        response->setProperty ("source_project_uuid", sourceProjectUUID);
    const auto effectiveParentUUID = parentProjectUUID.isNotEmpty()
                                       ? parentProjectUUID
                                       : (lifecycle == "save_as" ? sourceProjectUUID : juce::String());
    if (effectiveParentUUID.isNotEmpty())
        response->setProperty ("parent_project_uuid", effectiveParentUUID);
    if (agentHistoryGeneration.isNotEmpty())
        response->setProperty ("agent_history_generation", agentHistoryGeneration);
    if (historyPrepareID.isNotEmpty())
        response->setProperty ("history_prepare_id", historyPrepareID);
    return juce::JSON::toString (juce::var (response.release()));
}

void embedDerivedAnalysisManifest (te::Edit& edit, const juce::File& projectFile)
{
    if (projectFile.getFullPathName().isEmpty())
        return;
    const auto projectUUID = ensureVitProjectUUID (edit);
    const auto manifestFile = projectFile.getParentDirectory()
                                         .getChildFile (".vit_derived")
                                         .getChildFile (projectUUID)
                                         .getChildFile ("analysis_manifest.json");
    if (! manifestFile.existsAsFile())
        return;
    const auto parsed = juce::JSON::parse (manifestFile.loadFileAsString());
    auto* manifest = parsed.getDynamicObject();
    if (manifest == nullptr || manifest->getProperty ("project_uuid").toString() != projectUUID)
        return;
    edit.state.setProperty (vitAnalysisManifestProperty, juce::JSON::toString (parsed, true), nullptr);
}

void rebindEmbeddedAnalysisManifest (te::Edit& edit, const juce::String& projectUUID, const juce::File& projectFile)
{
    const auto parsed = juce::JSON::parse (edit.state.getProperty (vitAnalysisManifestProperty).toString());
    if (auto* manifest = parsed.getDynamicObject())
    {
        manifest->setProperty ("project_uuid", projectUUID);
        if (projectFile.getFullPathName().isNotEmpty())
            manifest->setProperty ("project_path", projectFile.getFullPathName());
        edit.state.setProperty (vitAnalysisManifestProperty, juce::JSON::toString (parsed, true), nullptr);
    }
}

float normaliseLevelDbForTelemetry (float rawLevelDb)
{
    const auto gain = juce::Decibels::decibelsToGain (rawLevelDb, minimumTelemetryDb);
    const auto dbfs = juce::Decibels::gainToDecibels (gain, minimumTelemetryDb);
    return juce::jlimit (minimumTelemetryDb, maximumTelemetryDb, dbfs);
}

juce::var spectrumArrayToVar (const float* values, int count)
{
    juce::Array<juce::var> out;
    out.ensureStorageAllocated (count);

    for (int i = 0; i < count; ++i)
        out.add (juce::var ((double) juce::jlimit (0.0f, 1.0f, values[i])));

    return juce::var (out);
}

juce::StringArray stringArrayFromVar (const juce::var& value)
{
    juce::StringArray result;

    if (auto* values = value.getArray())
    {
        for (const auto& item : *values)
        {
            const auto text = item.toString().trim();
            if (text.isNotEmpty())
                result.addIfNotAlreadyThere (text);
        }
    }
    else
    {
        const auto text = value.toString().trim();
        if (text.isNotEmpty())
            result.addIfNotAlreadyThere (text);
    }

    return result;
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

static juce::String buildTransportTelemetryPayload (te::Edit& edit,
                                                    bool includeRecordingWaveformDetail,
                                                    juce::int64 sequence)
{
    auto response = std::make_unique<juce::DynamicObject>();
    auto& transport = edit.getTransport();

    response->setProperty ("topic", "transport");
    response->setProperty ("is_playing", transport.isPlaying());
    response->setProperty ("is_recording", transport.isRecording());
    response->setProperty ("position_seconds", transport.getPosition().inSeconds());
    response->setProperty ("timestamp_ms", static_cast<juce::int64> (juce::Time::currentTimeMillis()));
    response->setProperty ("sequence", sequence);

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

bool looksLikeEncryptedProjectFile (const juce::File& projectFile)
{
    juce::FileInputStream stream (projectFile);
    if (! stream.openedOk())
        return false;

    char magic[4] {};
    if (stream.read (magic, sizeof (magic)) != sizeof (magic))
        return false;

    return magic[0] == 'V' && magic[1] == 'I' && magic[2] == 'T' && magic[3] == '1';
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
            return saveProjectToCurrentPathOrDefaultXml();
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
        [this](const juce::DynamicObject& object)
        {
            return ipcNewBlankProject (object);
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
        [this](const juce::DynamicObject& object, const juce::File& targetFile)
        {
            return ipcSaveProjectCopyAt (object, targetFile);
        },
        [this]()
        {
            return currentProjectPath.getFullPathName();
        },
        [this](const juce::DynamicObject& stream)
        {
            return buildVspRealtimeDataForStream (stream);
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
                AudioFeatureService::releaseTrackMappings (track->itemID.toString());

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
    const auto audioSettingsChanged = ProjectAudioSettingsService::ensureDefaultAudioSettings (*loadedEdit, "reload_project").changed;
    const auto monitoringPluginsAdded = ensureMonitoringPluginsForEdit (*loadedEdit);
    const auto trackRackChanged = ensureTrackRackGraphForEdit (*loadedEdit);
    primeEditPlaybackGraph (*loadedEdit);

    deltaHub.detach();
    edit = std::move (loadedEdit);
    lastTransportRecording = (edit != nullptr && edit->getTransport().isRecording());

    syncLevelMeterClients();

    if (edit != nullptr)
        deltaHub.attach (edit->state);

    if (audioSettingsChanged || monitoringPluginsAdded || trackRackChanged)
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
                AudioFeatureService::releaseTrackMappings (track->itemID.toString());
    }

    clearLevelMeterClients();
    edit.reset();

    loadedEdit->playInStopEnabled = true;
    ensureVitProjectUUID (*loadedEdit);
    const auto audioSettingsChanged = ProjectAudioSettingsService::ensureDefaultAudioSettings (*loadedEdit, "open_project").changed;
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

    if (audioSettingsChanged || monitoringPluginsAdded || trackRackChanged)
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

    ensureVitProjectUUID (*edit);
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

bool VitHeadlessService::saveProjectToCurrentPathOrDefaultXml()
{
    if (currentProjectPath.getFullPathName().isEmpty())
        return saveProjectToDefaultXml();

    const auto reply = ipcSaveProjectToCurrentPath();
    const auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        if (object->getProperty ("status").toString() == "ok")
            return true;

    juce::Logger::writeToLog ("VitHeadlessService: failed to save current project: " + reply);
    return false;
}

void VitHeadlessService::clearLevelMeterClients()
{
    for (auto& [trackID, registration] : trackLevelClients)
    {
        juce::ignoreUnused (trackID);

        if (registration.plugin != nullptr && registration.client != nullptr)
            registration.plugin->measurer.removeClient (*registration.client);

        if (registration.plugin != nullptr && registration.vspClient != nullptr)
            registration.plugin->measurer.removeClient (*registration.vspClient);
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

        if (registration.plugin == meterPlugin
            && registration.client != nullptr
            && registration.vspClient != nullptr)
            continue;

        if (registration.plugin != nullptr && registration.client != nullptr)
            registration.plugin->measurer.removeClient (*registration.client);

        if (registration.plugin != nullptr && registration.vspClient != nullptr)
            registration.plugin->measurer.removeClient (*registration.vspClient);

        registration.plugin = meterPlugin;
        registration.client = std::make_unique<te::LevelMeasurer::Client>();
        registration.vspClient = std::make_unique<te::LevelMeasurer::Client>();
        meterPlugin->measurer.addClient (*registration.client);
        meterPlugin->measurer.addClient (*registration.vspClient);
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

        if (it->second.plugin != nullptr && it->second.vspClient != nullptr)
            it->second.plugin->measurer.removeClient (*it->second.vspClient);

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
    zmqGateway->publishMessage (buildTransportTelemetryPayload (*edit,
                                                               includeLiveWaveform,
                                                               static_cast<juce::int64> (transportTelemetryTick)));
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
        float leftLevelDb = -100.0f;
        float rightLevelDb = -100.0f;
        te::SpectrumFrame spectrumFrame;
        bool hasSpectrumFrame = false;

        if (const auto it = trackLevelClients.find (audioTrack->itemID.toString().toStdString());
            it != trackLevelClients.end() && it->second.client != nullptr)
        {
            const auto numChannels = juce::jmax (1, it->second.client->getNumChannelsUsed());

            for (int channel = 0; channel < juce::jmin (numChannels, 2); ++channel)
            {
                const auto channelDb = normaliseLevelDbForTelemetry (it->second.client->getAndClearAudioLevel (channel).dB);
                levelDb = juce::jmax (levelDb, channelDb);

                if (channel == 0)
                    leftLevelDb = channelDb;
                else
                    rightLevelDb = channelDb;
            }

            if (numChannels == 1)
                rightLevelDb = leftLevelDb;

            hasSpectrumFrame = it->second.client->getAndClearSpectrumFrame (spectrumFrame);
        }

        auto trackObject = std::make_unique<juce::DynamicObject>();
        trackObject->setProperty ("id", audioTrack->itemID.toString());
        trackObject->setProperty ("level_db", levelDb);
        trackObject->setProperty ("left_level_db", leftLevelDb);
        trackObject->setProperty ("right_level_db", rightLevelDb);

        if (hasSpectrumFrame)
        {
            trackObject->setProperty ("spectrum_bin_count", te::SpectrumFrame::numBins);
            trackObject->setProperty ("spectrum_min_hz", 20.0);
            trackObject->setProperty ("spectrum_max_hz", 20000.0);
            trackObject->setProperty ("spectrum_input_peak", spectrumFrame.inputPeak);
            trackObject->setProperty ("spectrum_output_peak", spectrumFrame.outputPeak);
            trackObject->setProperty ("spectrum_left", spectrumArrayToVar (spectrumFrame.left, te::SpectrumFrame::numBins));
            trackObject->setProperty ("spectrum_right", spectrumArrayToVar (spectrumFrame.right, te::SpectrumFrame::numBins));
            trackObject->setProperty ("spectrum_phase", spectrumArrayToVar (spectrumFrame.phase, te::SpectrumFrame::numBins));
            trackObject->setProperty ("spectrum_weight", spectrumArrayToVar (spectrumFrame.weight, te::SpectrumFrame::numBins));
        }

        tracksArray.add (juce::var (trackObject.release()));
    }

    response->setProperty ("topic", "levels");
    response->setProperty ("tracks", juce::var (tracksArray));
    zmqGateway->publishMessage (juce::JSON::toString (juce::var (response.release())));
}

juce::var VitHeadlessService::buildVspRealtimeDataForStream (const juce::DynamicObject& stream)
{
    const auto streamName = stream.getProperty ("stream").toString().trim();
    auto data = std::make_unique<juce::DynamicObject>();
    data->setProperty ("source", "engine_realtime");
    data->setProperty ("timestamp_ms", static_cast<juce::int64> (juce::Time::currentTimeMillis()));

    if (edit == nullptr)
    {
        data->setProperty ("source", "engine_unavailable");
        return juce::var (data.release());
    }

    if (streamName == "transport.playhead")
    {
        auto& transport = edit->getTransport();
        data->setProperty ("position_seconds", transport.getPosition().inSeconds());
        data->setProperty ("is_playing", transport.isPlaying());
        data->setProperty ("is_recording", transport.isRecording());
        return juce::var (data.release());
    }

    if (streamName == "recording.status")
    {
        auto& transport = edit->getTransport();
        data->setProperty ("is_recording", transport.isRecording());
        data->setProperty ("position_seconds", transport.getPosition().inSeconds());
        data->setProperty ("armed_track_count", 0);
        return juce::var (data.release());
    }

    if (streamName != "meters.visible_tracks" && streamName != "spectrum.visible_tracks")
        return juce::var();

    syncLevelMeterClients();

    const auto requestedTrackIds = stringArrayFromVar (stream.getProperty ("track_ids"));
    std::unordered_set<std::string> requestedTrackSet;
    for (const auto& trackId : requestedTrackIds)
        requestedTrackSet.insert (trackId.toStdString());

    juce::Array<juce::var> tracksArray;
    juce::Array<juce::var> assetRefs;
    int editIndex = 0;

    for (auto* track : te::getAllTracks (*edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        const auto trackId = audioTrack->itemID.toString();
        const auto trackIdKey = trackId.toStdString();

        if (! requestedTrackSet.empty() && ! requestedTrackSet.contains (trackIdKey))
        {
            ++editIndex;
            continue;
        }

        if (tracksArray.size() >= maxVspVisibleTracks)
            break;

        if (streamName == "meters.visible_tracks")
        {
            float levelDb = minimumTelemetryDb;
            float leftLevelDb = minimumTelemetryDb;
            float rightLevelDb = minimumTelemetryDb;
            bool clipped = false;
            bool peakHeld = false;
            int numChannels = 0;

            if (const auto it = trackLevelClients.find (trackIdKey);
                it != trackLevelClients.end() && it->second.vspClient != nullptr)
            {
                numChannels = juce::jmax (1, it->second.vspClient->getNumChannelsUsed());

                for (int channel = 0; channel < juce::jmin (numChannels, 2); ++channel)
                {
                    const auto channelDb = normaliseLevelDbForTelemetry (it->second.vspClient->getAndClearAudioLevel (channel).dB);
                    levelDb = juce::jmax (levelDb, channelDb);

                    if (channel == 0)
                        leftLevelDb = channelDb;
                    else
                        rightLevelDb = channelDb;
                }

                if (numChannels == 1)
                    rightLevelDb = leftLevelDb;

                clipped = it->second.vspClient->getAndClearOverload();
                peakHeld = it->second.vspClient->getAndClearPeak();
            }

            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("track_id", trackId);
            row->setProperty ("id", trackId);
            row->setProperty ("name", audioTrack->getName());
            row->setProperty ("track_name", audioTrack->getName());
            row->setProperty ("edit_index", editIndex);
            row->setProperty ("level_db", levelDb);
            row->setProperty ("peak_db", levelDb);
            row->setProperty ("rms_db", levelDb);
            row->setProperty ("left_level_db", leftLevelDb);
            row->setProperty ("right_level_db", rightLevelDb);
            row->setProperty ("left_peak_db", leftLevelDb);
            row->setProperty ("right_peak_db", rightLevelDb);
            row->setProperty ("clipped", clipped);
            row->setProperty ("peak_held", peakHeld);
            row->setProperty ("channel_count", numChannels);
            row->setProperty ("source", "engine_level_meter");
            tracksArray.add (juce::var (row.release()));
        }
        else
        {
            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("track_id", trackId);
            row->setProperty ("id", trackId);
            row->setProperty ("name", audioTrack->getName());
            row->setProperty ("track_name", audioTrack->getName());
            row->setProperty ("edit_index", editIndex);
            row->setProperty ("kind", "spectrum_frame");
            row->setProperty ("source", "engine_level_meter");

            te::SpectrumFrame spectrumFrame;
            bool hasSpectrumFrame = false;
            if (const auto it = trackLevelClients.find (trackIdKey);
                it != trackLevelClients.end() && it->second.vspClient != nullptr)
            {
                hasSpectrumFrame = it->second.vspClient->getAndClearSpectrumFrame (spectrumFrame);
            }

            if (hasSpectrumFrame)
            {
                row->setProperty ("spectrum_bin_count", te::SpectrumFrame::numBins);
                row->setProperty ("spectrum_min_hz", 20.0);
                row->setProperty ("spectrum_max_hz", 20000.0);
                row->setProperty ("spectrum_input_peak", spectrumFrame.inputPeak);
                row->setProperty ("spectrum_output_peak", spectrumFrame.outputPeak);
                row->setProperty ("spectrum_left", spectrumArrayToVar (spectrumFrame.left, te::SpectrumFrame::numBins));
                row->setProperty ("spectrum_right", spectrumArrayToVar (spectrumFrame.right, te::SpectrumFrame::numBins));
                row->setProperty ("spectrum_phase", spectrumArrayToVar (spectrumFrame.phase, te::SpectrumFrame::numBins));
                row->setProperty ("spectrum_weight", spectrumArrayToVar (spectrumFrame.weight, te::SpectrumFrame::numBins));
            }
            else
            {
                row->setProperty ("status", "no_frame");
            }

            auto ref = std::make_unique<juce::DynamicObject>();
            ref->setProperty ("track_id", trackId);
            ref->setProperty ("kind", "spectrum_frame");
            ref->setProperty ("uri", "vit-cache://project_current/tracks/" + trackId + "/spectrum/latest");
            ref->setProperty ("source", hasSpectrumFrame ? "engine_level_meter" : "engine_level_meter_no_frame");

            assetRefs.add (juce::var (ref.release()));
            tracksArray.add (juce::var (row.release()));
        }

        ++editIndex;
    }

    if (streamName == "meters.visible_tracks")
    {
        data->setProperty ("tracks", juce::var (tracksArray));
        data->setProperty ("visible_track_count", tracksArray.size());
    }
    else
    {
        data->setProperty ("tracks", juce::var (tracksArray));
        data->setProperty ("asset_refs", juce::var (assetRefs));
        data->setProperty ("visible_track_count", tracksArray.size());
        data->setProperty ("inline_bins", true);
    }

    return juce::var (data.release());
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

juce::String VitHeadlessService::ipcNewBlankProject (const juce::DynamicObject& object)
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
                AudioFeatureService::releaseTrackMappings (track->itemID.toString());

    clearLevelMeterClients();
    edit.reset();

    loadedEdit->playInStopEnabled = true;
    bool audioSettingsChanged = ProjectAudioSettingsService::ensureDefaultAudioSettings (*loadedEdit, "new_project").changed;
    juce::Array<juce::var> audioSettingsWarnings;
    if (auto* audioSettingsObject = object.getProperty ("audio_settings").getDynamicObject())
    {
        ProjectAudioSettingsService::applyAudioSettingsToEdit (*loadedEdit,
                                                               *audioSettingsObject,
                                                               "new_project_payload",
                                                               false,
                                                               &audioSettingsWarnings);
        audioSettingsChanged = true;
    }
    const auto monitoringPluginsAdded = ensureMonitoringPluginsForEdit (*loadedEdit);
    const auto trackRackChanged = ensureTrackRackGraphForEdit (*loadedEdit);
    primeEditPlaybackGraph (*loadedEdit);

    edit = std::move (loadedEdit);
    lastTransportRecording = (edit != nullptr && edit->getTransport().isRecording());

    const auto projectUUID = ensureVitProjectUUID (*edit, true);
    edit->state.removeProperty (vitProjectParentUUIDProperty, nullptr);
    edit->state.removeProperty (vitAnalysisManifestProperty, nullptr);

    if (edit != nullptr)
        VitGraphSwapCoordinator::resetForEdit (*edit, "Blank project created");

    syncLevelMeterClients();

    if (edit != nullptr)
        deltaHub.attach (edit->state);

    if (audioSettingsChanged || monitoringPluginsAdded || trackRackChanged)
        saveProjectToDefaultXml();

    edit->getUndoManager().clearUndoHistory();
    currentProjectPath = juce::File();

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Blank project created");
    response->setProperty ("project_lifecycle", "new");
    response->setProperty ("project_uuid", projectUUID);
    response->setProperty ("project_id", projectUUID);
    response->setProperty ("project_path", "");
    response->setProperty ("current_project_path", "");
    if (! audioSettingsWarnings.isEmpty())
        response->setProperty ("warnings", juce::var (audioSettingsWarnings));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String VitHeadlessService::ipcOpenProjectAt (const juce::DynamicObject& object, const juce::File& projectFile)
{
    const bool encryptedFile = looksLikeEncryptedProjectFile (projectFile);
    const bool encryptionRequested = isAppBoundEncryption (object);
    if (encryptedFile && ! encryptionRequested)
        juce::Logger::writeToLog ("VitHeadlessService: auto-detected encrypted project " + projectFile.getFullPathName());
    else if (encryptionRequested && ! encryptedFile)
        juce::Logger::writeToLog ("VitHeadlessService: app-bound open requested for legacy clear XML project; "
                                  "loading clear XML for migration: " + projectFile.getFullPathName());

    const bool ok = encryptedFile ? loadEncryptedProjectFromFile (projectFile)
                                  : loadProjectFromFile (projectFile);

    if (! ok)
        return CommandDispatcher::makeErrorReply ("open_project failed for " + projectFile.getFullPathName());

    currentProjectPath = juce::File (projectFile.getFullPathName());

    if (globalProjectConfig != nullptr)
        globalProjectConfig->prependRecentProject (currentProjectPath);

    return makeProjectLifecycleReply ("Project opened",
                                      "open",
                                      currentProjectPath,
                                      ensureVitProjectUUID (*edit),
                                      {},
                                      edit->state.getProperty (vitProjectParentUUIDProperty).toString().trim(),
                                      edit->state.getProperty (vitAgentHistoryGenerationProperty).toString().trim());
}

juce::String VitHeadlessService::ipcSaveProjectWithPayload (const juce::DynamicObject& object)
{
    const juce::String requestedPath = object.getProperty ("file_path").toString().trim();
    const auto target = requestedPath.isNotEmpty() ? normalizeProjectPathForSave (juce::File (requestedPath))
                                                   : normalizeProjectPathForSave (currentProjectPath);
    const bool savesVitProject = target.getFileExtension().equalsIgnoreCase (".vit");

    // A .vit file is always an app-bound encrypted container.  Do not let a
    // stale GUI repository snapshot or a direct Agent save silently downgrade
    // the active project to clear Tracktion XML.
    if (isAppBoundEncryption (object) || savesVitProject)
    {
        if (target.getFullPathName().isEmpty())
            return CommandDispatcher::makeStatusReply ("require_path", "Please prompt Save As");

        const auto current = normalizeProjectPathForSave (currentProjectPath);
        const bool createsNewProject = currentProjectPath.getFullPathName().isEmpty()
                                    || current.getFullPathName() != target.getFullPathName();
        return ipcSaveEncryptedProjectToPath (target, createsNewProject, object);
    }

    return ipcSaveProjectToCurrentPath (&object);
}

juce::String VitHeadlessService::ipcSaveEncryptedProjectToPath (const juce::File& fileFromPayload,
                                                                bool createNewProjectIdentity,
                                                                const juce::DynamicObject& object)
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

    const auto sourceProjectUUID = ensureVitProjectUUID (*activeEdit);
    embedDerivedAnalysisManifest (*activeEdit, currentProjectPath);
    const auto sourceParentUUID = activeEdit->state.getProperty (vitProjectParentUUIDProperty);
    const auto sourceAgentHistoryGeneration = activeEdit->state.getProperty (vitAgentHistoryGenerationProperty);
    const auto agentHistoryGeneration = object.getProperty ("agent_history_generation").toString().trim();
    const auto historyPrepareID = object.getProperty ("history_prepare_id").toString().trim();
    if (agentHistoryGeneration.isNotEmpty())
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, agentHistoryGeneration, nullptr);
    const auto projectUUID = createNewProjectIdentity ? ensureVitProjectUUID (*activeEdit, true)
                                                      : sourceProjectUUID;
    if (createNewProjectIdentity)
    {
        activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceProjectUUID, nullptr);
        rebindEmbeddedAnalysisManifest (*activeEdit, projectUUID, saveTarget);
    }
    struct SaveAsMediaBinding
    {
        te::AudioClipBase* clip = nullptr;
        juce::String originalReference;
    };
    std::vector<SaveAsMediaBinding> saveAsMediaBindings;
    if (createNewProjectIdentity)
    {
        for (auto* track : te::getAllTracks (*activeEdit))
        {
            if (track == nullptr)
                continue;
            for (int index = 0; index < track->getNumTrackItems(); ++index)
            {
                auto* clip = dynamic_cast<te::AudioClipBase*> (track->getTrackItem (index));
                if (clip == nullptr)
                    continue;
                auto& reference = clip->getSourceFileReference();
                auto sourceFile = reference.getFile();
                if (sourceFile.getFullPathName().isEmpty())
                    sourceFile = clip->getOriginalFile();
                if (sourceFile.getFullPathName().isEmpty())
                    continue;
                saveAsMediaBindings.push_back ({ clip, reference.source.get() });
                // Save As creates a new project identity but does not collect
                // media. Stabilise references against the source project so a
                // target in another directory does not reinterpret a relative
                // token and immediately go offline.
                reference.source = sourceFile.getFullPathName();
            }
        }
    }
    const auto restoreSourceIdentity = [&]
    {
        for (const auto& binding : saveAsMediaBindings)
            if (binding.clip != nullptr)
                binding.clip->getSourceFileReference().source = binding.originalReference;
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
        if (createNewProjectIdentity)
        {
            activeEdit->state.setProperty (vitProjectUUIDProperty, sourceProjectUUID, nullptr);
            activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceParentUUID, nullptr);
            rebindEmbeddedAnalysisManifest (*activeEdit, sourceProjectUUID, currentProjectPath);
        }
    };

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    if (auto xml = activeEdit->state.createXml())
    {
        const auto blob = VitEncryptionCore::encryptProject (xml->toString());

        if (blob.empty())
        {
            restoreSourceIdentity();
            return CommandDispatcher::makeErrorReply ("save_project: encryption failed");
        }

        if (! saveTarget.replaceWithData (blob.data(), blob.size()))
        {
            restoreSourceIdentity();
            return CommandDispatcher::makeErrorReply ("save_project: failed to write " + saveTarget.getFullPathName());
        }

        currentProjectPath = juce::File (saveTarget.getFullPathName());

        if (globalProjectConfig != nullptr)
            globalProjectConfig->prependRecentProject (currentProjectPath);

        return makeProjectLifecycleReply ("Project saved",
                                          createNewProjectIdentity ? "save_as" : "save",
                                          currentProjectPath,
                                          projectUUID,
                                          createNewProjectIdentity ? sourceProjectUUID : juce::String(),
                                          {},
                                          agentHistoryGeneration,
                                          historyPrepareID);
    }

    restoreSourceIdentity();
    return CommandDispatcher::makeErrorReply ("save_project: failed to serialize edit state");
}

juce::String VitHeadlessService::ipcSaveProjectToCurrentPath (const juce::DynamicObject* object)
{
    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    const auto projectUUID = ensureVitProjectUUID (*activeEdit);
    embedDerivedAnalysisManifest (*activeEdit, currentProjectPath);
    const auto sourceAgentHistoryGeneration = activeEdit->state.getProperty (vitAgentHistoryGenerationProperty);
    const auto agentHistoryGeneration = object != nullptr ? object->getProperty ("agent_history_generation").toString().trim()
                                                          : juce::String();
    const auto historyPrepareID = object != nullptr ? object->getProperty ("history_prepare_id").toString().trim()
                                                    : juce::String();
    if (agentHistoryGeneration.isNotEmpty())
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, agentHistoryGeneration, nullptr);

    if (currentProjectPath.getFullPathName().isEmpty())
    {
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
        return CommandDispatcher::makeStatusReply ("require_path", "Please prompt Save As");
    }

    const auto parentDir = currentProjectPath.getParentDirectory();

    if (! parentDir.isDirectory())
        if (! parentDir.createDirectory())
        {
            activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
            return CommandDispatcher::makeErrorReply ("save_project: cannot create directory " + parentDir.getFullPathName());
        }

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    const auto saveTarget = normalizeProjectPathForSave (currentProjectPath);

    if (! te::EditFileOperations (*activeEdit).saveAs (saveTarget, true))
    {
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
        return CommandDispatcher::makeErrorReply ("save_project: failed to write " + saveTarget.getFullPathName());
    }

    currentProjectPath = saveTarget;

    if (isVitDumpClearXmlEnabled())
        if (auto xml = activeEdit->state.createXml())
            writeClearXmlSidecarSilently (saveTarget, *xml);

    return makeProjectLifecycleReply ("Project saved", "save", currentProjectPath, projectUUID,
                                      {}, {}, agentHistoryGeneration, historyPrepareID);
}

juce::String VitHeadlessService::ipcSaveAsProjectAt (const juce::DynamicObject& object, const juce::File& targetFile)
{
    if (isAppBoundEncryption (object) || targetFile.getFileExtension().equalsIgnoreCase (".vit"))
        return ipcSaveEncryptedProjectToPath (targetFile, true, object);

    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    const auto sourceProjectUUID = ensureVitProjectUUID (*activeEdit);
    embedDerivedAnalysisManifest (*activeEdit, currentProjectPath);
    const auto sourceParentUUID = activeEdit->state.getProperty (vitProjectParentUUIDProperty);
    const auto sourceAgentHistoryGeneration = activeEdit->state.getProperty (vitAgentHistoryGenerationProperty);
    const auto agentHistoryGeneration = object.getProperty ("agent_history_generation").toString().trim();
    const auto historyPrepareID = object.getProperty ("history_prepare_id").toString().trim();
    if (agentHistoryGeneration.isNotEmpty())
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, agentHistoryGeneration, nullptr);
    const auto projectUUID = ensureVitProjectUUID (*activeEdit, true);
    activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceProjectUUID, nullptr);
    rebindEmbeddedAnalysisManifest (*activeEdit, projectUUID, targetFile);

    const auto parentDir = targetFile.getParentDirectory();

    if (! parentDir.isDirectory())
        if (! parentDir.createDirectory())
        {
            activeEdit->state.setProperty (vitProjectUUIDProperty, sourceProjectUUID, nullptr);
            activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceParentUUID, nullptr);
            activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
            rebindEmbeddedAnalysisManifest (*activeEdit, sourceProjectUUID, currentProjectPath);
            return CommandDispatcher::makeErrorReply ("save_as_project: cannot create directory " + parentDir.getFullPathName());
        }

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    const auto saveTarget = normalizeProjectPathForSave (targetFile);

    if (! te::EditFileOperations (*activeEdit).saveAs (saveTarget, true))
    {
        activeEdit->state.setProperty (vitProjectUUIDProperty, sourceProjectUUID, nullptr);
        activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceParentUUID, nullptr);
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
        rebindEmbeddedAnalysisManifest (*activeEdit, sourceProjectUUID, currentProjectPath);
        return CommandDispatcher::makeErrorReply ("save_as_project: failed to write " + saveTarget.getFullPathName());
    }

    currentProjectPath = juce::File (saveTarget.getFullPathName());

    if (globalProjectConfig != nullptr)
        globalProjectConfig->prependRecentProject (currentProjectPath);

    if (isVitDumpClearXmlEnabled())
        if (auto xml = activeEdit->state.createXml())
            writeClearXmlSidecarSilently (saveTarget, *xml);

    return makeProjectLifecycleReply ("Project saved",
                                      "save_as",
                                      currentProjectPath,
                                      projectUUID,
                                      sourceProjectUUID,
                                      {},
                                      agentHistoryGeneration,
                                      historyPrepareID);
}

juce::String VitHeadlessService::ipcSaveProjectCopyAt (const juce::DynamicObject& object,
                                                       const juce::File& logicalTargetFile)
{
    auto* activeEdit = edit.get();

    if (activeEdit == nullptr)
        return CommandDispatcher::makeErrorReply ("No active edit loaded");

    const auto logicalTarget = normalizeProjectPathForSave (logicalTargetFile);
    const auto writePathText = object.getProperty ("write_path").toString().trim();
    const auto writeTarget = normalizeProjectPathForSave (writePathText.isNotEmpty() ? juce::File (writePathText)
                                                                                     : logicalTarget);
    if (! logicalTarget.getFileExtension().equalsIgnoreCase (".vit")
        || ! writeTarget.getFileExtension().equalsIgnoreCase (".vit"))
        return CommandDispatcher::makeErrorReply ("save_project_copy requires .vit file_path and write_path");

    struct ClipSourceBinding
    {
        te::AudioClipBase* clip = nullptr;
        juce::String originalReference;
        juce::File sourceFile;
        juce::String sourceKey;
    };

    struct MediaSource
    {
        juce::File sourceFile;
        juce::File logicalTargetFile;
        juce::File physicalTargetFile;
        juce::String relativeTargetPath;
        bool exists = false;
        bool external = false;
    };

    std::vector<ClipSourceBinding> clipBindings;
    std::vector<MediaSource> mediaSources;
    std::unordered_map<std::string, size_t> mediaIndexBySource;
    std::unordered_map<std::string, juce::String> sourceByTargetName;
    const auto projectBase = logicalTarget.getFileNameWithoutExtension();
    const auto logicalMediaDir = logicalTarget.getSiblingFile (projectBase + "_Media").getChildFile ("Audio");
    const auto physicalMediaDir = writeTarget.getSiblingFile (projectBase + "_Media").getChildFile ("Audio");

    for (auto* track : te::getAllTracks (*activeEdit))
    {
        if (track == nullptr)
            continue;

        const int itemCount = track->getNumTrackItems();
        for (int index = 0; index < itemCount; ++index)
        {
            auto* clip = dynamic_cast<te::AudioClipBase*> (track->getTrackItem (index));
            if (clip == nullptr)
                continue;

            auto& reference = clip->getSourceFileReference();
            auto sourceFile = reference.getFile();
            if (sourceFile.getFullPathName().isEmpty())
                sourceFile = clip->getOriginalFile();
            const auto sourceKey = sourceFile.getFullPathName().toLowerCase().toStdString();
            clipBindings.push_back ({ clip, reference.source.get(), sourceFile, juce::String (sourceKey) });

            if (sourceKey.empty() || mediaIndexBySource.find (sourceKey) != mediaIndexBySource.end())
                continue;

            auto targetName = sourceFile.getFileName();
            const auto targetKey = targetName.toLowerCase().toStdString();
            const auto collision = sourceByTargetName.find (targetKey);
            if (collision != sourceByTargetName.end()
                && ! collision->second.equalsIgnoreCase (sourceFile.getFullPathName()))
            {
                const auto suffix = juce::String::toHexString (static_cast<juce::int64> (sourceFile.getFullPathName().hashCode64())).substring (0, 8);
                targetName = sourceFile.getFileNameWithoutExtension() + "_" + suffix + sourceFile.getFileExtension();
            }
            sourceByTargetName[targetName.toLowerCase().toStdString()] = sourceFile.getFullPathName();
            MediaSource media;
            media.sourceFile = sourceFile;
            media.logicalTargetFile = logicalMediaDir.getChildFile (targetName);
            media.physicalTargetFile = physicalMediaDir.getChildFile (targetName);
            media.relativeTargetPath = media.logicalTargetFile.getRelativePathFrom (logicalTarget.getParentDirectory()).replaceCharacter ('\\', '/');
            media.exists = sourceFile.existsAsFile();
            media.external = ! sourceFile.isAChildOf (currentProjectPath.getParentDirectory());
            mediaIndexBySource[sourceKey] = mediaSources.size();
            mediaSources.push_back (std::move (media));
        }
    }

    int64 estimatedCopyBytes = 0;
    int missingAudioCount = 0;
    int externalAudioCount = 0;
    for (const auto& media : mediaSources)
    {
        if (media.exists)
            estimatedCopyBytes += media.sourceFile.getSize();
        else
            ++missingAudioCount;
        if (media.external)
            ++externalAudioCount;
    }

    const auto mediaPolicy = object.getProperty ("media_policy").toString().trim().toLowerCase();
    const auto buildMediaReply = [&] (const juce::String& status, const juce::String& message)
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", status);
        response->setProperty ("message", message);
        response->setProperty ("referenced_audio_count", static_cast<int> (mediaSources.size()));
        response->setProperty ("external_audio_count", externalAudioCount);
        response->setProperty ("missing_audio_count", missingAudioCount);
        response->setProperty ("estimated_copy_bytes", estimatedCopyBytes);
        response->setProperty ("media_policy", mediaPolicy);
        return response;
    };

    if (! mediaSources.empty() && mediaPolicy.isEmpty())
        return juce::JSON::toString (juce::var (buildMediaReply ("require_media_policy", "Please choose whether to package referenced audio").release()));

    if (mediaPolicy.isNotEmpty() && mediaPolicy != "reference_only" && mediaPolicy != "copy_referenced_audio")
        return CommandDispatcher::makeErrorReply ("Unsupported media_policy: " + mediaPolicy);

    if (static_cast<bool> (object.getProperty ("preflight_only")))
        return juce::JSON::toString (juce::var (buildMediaReply ("media_preflight", "Project folder media preflight complete").release()));

    if (mediaPolicy == "copy_referenced_audio" && missingAudioCount > 0)
        return juce::JSON::toString (juce::var (buildMediaReply ("media_missing", "Referenced audio is missing; complete package was not created").release()));

    const auto sourceProjectUUID = ensureVitProjectUUID (*activeEdit);
    const auto sourceParentUUID = activeEdit->state.getProperty (vitProjectParentUUIDProperty);
    const auto sourceAgentHistoryGeneration = activeEdit->state.getProperty (vitAgentHistoryGenerationProperty);
    const auto sourceProjectPath = currentProjectPath;
    embedDerivedAnalysisManifest (*activeEdit, sourceProjectPath);
    const auto agentHistoryGeneration = object.getProperty ("agent_history_generation").toString().trim();
    const auto historyPrepareID = object.getProperty ("history_prepare_id").toString().trim();
    if (agentHistoryGeneration.isNotEmpty())
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, agentHistoryGeneration, nullptr);
    const auto targetProjectUUID = ensureVitProjectUUID (*activeEdit, true);
    activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceProjectUUID, nullptr);
    rebindEmbeddedAnalysisManifest (*activeEdit, targetProjectUUID, logicalTarget);

    const auto restoreSourceState = [&]
    {
        for (const auto& binding : clipBindings)
            if (binding.clip != nullptr)
                binding.clip->getSourceFileReference().source = binding.originalReference;
        activeEdit->state.setProperty (vitProjectUUIDProperty, sourceProjectUUID, nullptr);
        activeEdit->state.setProperty (vitProjectParentUUIDProperty, sourceParentUUID, nullptr);
        activeEdit->state.setProperty (vitAgentHistoryGenerationProperty, sourceAgentHistoryGeneration, nullptr);
        rebindEmbeddedAnalysisManifest (*activeEdit, sourceProjectUUID, sourceProjectPath);
    };

    for (const auto& binding : clipBindings)
    {
        if (binding.clip == nullptr || binding.sourceKey.isEmpty())
            continue;
        const auto found = mediaIndexBySource.find (binding.sourceKey.toStdString());
        if (found == mediaIndexBySource.end())
            continue;
        const auto& media = mediaSources[found->second];
        binding.clip->getSourceFileReference().source = mediaPolicy == "copy_referenced_audio"
                                                     ? media.relativeTargetPath
                                                     : media.sourceFile.getFullPathName();
    }

    for (auto* track : te::getAllTracks (*activeEdit))
        if (track != nullptr)
            track->flushStateToValueTree();

    const auto parentDir = writeTarget.getParentDirectory();
    if (! parentDir.isDirectory() && ! parentDir.createDirectory())
    {
        restoreSourceState();
        return CommandDispatcher::makeErrorReply ("save_project_copy: cannot create directory " + parentDir.getFullPathName());
    }

    if (auto xml = activeEdit->state.createXml())
    {
        const auto blob = VitEncryptionCore::encryptProject (xml->toString());
        if (blob.empty() || ! writeTarget.replaceWithData (blob.data(), blob.size()))
        {
            restoreSourceState();
            return CommandDispatcher::makeErrorReply ("save_project_copy: failed to write " + writeTarget.getFullPathName());
        }
    }
    else
    {
        restoreSourceState();
        return CommandDispatcher::makeErrorReply ("save_project_copy: failed to serialize edit state");
    }

    auto response = buildMediaReply ("ok", "Project folder snapshot created");
    response->setProperty ("project_lifecycle", "save_as_folder");
    response->setProperty ("project_path", logicalTarget.getFullPathName());
    response->setProperty ("snapshot_path", writeTarget.getFullPathName());
    response->setProperty ("project_uuid", targetProjectUUID);
    response->setProperty ("source_project_path", sourceProjectPath.getFullPathName());
    response->setProperty ("source_project_uuid", sourceProjectUUID);
    response->setProperty ("active_project_path", sourceProjectPath.getFullPathName());
    response->setProperty ("active_project_uuid", sourceProjectUUID);
    response->setProperty ("active_project_unchanged", true);
    response->setProperty ("agent_history_generation", agentHistoryGeneration);
    response->setProperty ("history_prepare_id", historyPrepareID);
    juce::Array<juce::var> mediaRows;
    for (const auto& media : mediaSources)
    {
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("kind", "audio");
        row->setProperty ("source_path", media.sourceFile.getFullPathName());
        row->setProperty ("target_path", mediaPolicy == "copy_referenced_audio" ? media.relativeTargetPath : juce::String());
        row->setProperty ("copy_path", mediaPolicy == "copy_referenced_audio" ? media.physicalTargetFile.getFullPathName() : juce::String());
        row->setProperty ("size", media.exists ? media.sourceFile.getSize() : 0);
        row->setProperty ("status", media.exists ? (mediaPolicy == "copy_referenced_audio" ? "pending_copy" : "referenced") : "missing");
        mediaRows.add (juce::var (row.release()));
    }
    response->setProperty ("media", juce::var (mediaRows));
    restoreSourceState();
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit
