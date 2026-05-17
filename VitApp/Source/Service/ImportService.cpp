#include "ImportService.h"

namespace vit
{

namespace
{

juce::String describeTransportState (te::Edit& edit)
{
    auto& transport = edit.getTransport();
    return "isPlaying=" + juce::String (transport.isPlaying() ? "true" : "false")
        + " isRecording=" + juce::String (transport.isRecording() ? "true" : "false")
        + " playContextActive=" + juce::String (transport.isPlayContextActive() ? "true" : "false")
        + " positionSeconds=" + juce::String (transport.getPosition().inSeconds(), 3)
        + " editLengthSeconds=" + juce::String (edit.getLength().inSeconds(), 3);
}

te::Clip* findClipByID (te::Edit& edit, const juce::String& clipId)
{
    if (clipId.isEmpty())
        return nullptr;

    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();
        for (int i = 0; i < n; ++i)
        {
            auto* item = track->getTrackItem (i);
            auto* clip = dynamic_cast<te::Clip*> (item);
            if (clip != nullptr && clip->itemID.toString() == clipId)
                return clip;
        }
    }
    return nullptr;
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
    {
        track.flushStateToValueTree();
        edit.getTransport().ensureContextAllocated (true);
        edit.dispatchPendingUpdatesSynchronously();
    }

    return changed;
}

} // namespace

ImportService::ImportService (EditGetter editGetter,
                              SaveProjectAction saveProjectAction,
                              PublishAction publishAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      publishMessage (std::move (publishAction))
{
}

ImportService::AudioImportInsertResult ImportService::insertWaveClipWithUndoAndStartBake (
    te::Edit& edit,
    te::AudioTrack& targetTrack,
    const juce::File& sourceFile,
    double startTimeSeconds,
    double audioLengthSeconds,
    bool deleteExistingClips,
    const juce::String& appliedMode) const
{
    AudioImportInsertResult out;
    edit.getUndoManager().beginNewTransaction (
        "Import Media: " + sourceFile.getFileName());

    const auto clipStart = te::TimePosition::fromSeconds (startTimeSeconds);
    const auto clipEnd   = te::TimePosition::fromSeconds (startTimeSeconds + audioLengthSeconds);
    auto newClip = targetTrack.insertWaveClip (sourceFile.getFileNameWithoutExtension(),
                                               sourceFile,
                                               {{ clipStart, clipEnd }},
                                               deleteExistingClips);

    if (newClip == nullptr)
    {
        out.errorMessage = "Failed to insert audio clip";
        return out;
    }

    out.clipId = newClip->itemID.toString();
    out.clipName = newClip->getName();
    out.trackItemId = targetTrack.itemID.toString();
    out.startTimeSeconds = startTimeSeconds;
    out.audioLengthSeconds = audioLengthSeconds;

    targetTrack.flushStateToValueTree();
    edit.invalidateStoredLength();
    edit.dispatchPendingUpdatesSynchronously();
    out.editLengthSeconds = edit.getLength().inSeconds();
    edit.getTransport().ensureContextAllocated (true);

    TiledSpectrogramBaker::startBake (sourceFile.getFullPathName(),
                                      out.trackItemId,
                                      out.clipId,
                                      publishMessage);

    if (saveProject && ! saveProject())
    {
        out.errorMessage = "Audio clip added in memory but failed to save project";
        return out;
    }

    out.ok = true;
    juce::ignoreUnused (appliedMode);
    return out;
}

juce::String ImportService::handleAddAudioClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto startTimeVar = object.getProperty ("start_time");

    if (trackID.isEmpty())
        return makeErrorReply ("add_audio_clip requires a non-empty track_id");

    if (filePath.isEmpty())
        return makeErrorReply ("add_audio_clip requires a non-empty file_path");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("add_audio_clip requires a numeric start_time field");

    const auto startTimeSeconds = static_cast<double> (startTimeVar);

    if (startTimeSeconds < 0.0)
        return makeErrorReply ("start_time must be greater than or equal to zero");

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("Audio file does not exist: " + filePath);

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);

    const auto audioLengthSeconds = audioFile.getLength();

    if (audioLengthSeconds <= 0.0)
        return makeErrorReply ("Audio file has zero length: " + filePath);

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    ensureMonitoringPlugins (*targetTrack);

    const auto clipStart = te::TimePosition::fromSeconds (startTimeSeconds);
    const auto clipEnd = te::TimePosition::fromSeconds (startTimeSeconds + audioLengthSeconds);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add audio clip");
    auto newClip = targetTrack->insertWaveClip (sourceFile.getFileNameWithoutExtension(),
                                                sourceFile,
                                                {{ clipStart, clipEnd }},
                                                false);

    if (newClip == nullptr)
        return makeErrorReply ("Failed to insert audio clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    const auto editLengthSeconds = edit->getLength().inSeconds();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Audio clip added in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Audio clip added");
    response->setProperty ("track_id", trackID);
    response->setProperty ("clip_name", newClip->getName());
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("start_time", startTimeSeconds);
    response->setProperty ("clip_length_seconds", audioLengthSeconds);
    response->setProperty ("edit_length_seconds", editLengthSeconds);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleImportMediaToTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto mediaType = object.getProperty ("media_type").toString().trim().toLowerCase();
    const auto mode = object.getProperty ("mode").toString().trim().toLowerCase();
    const auto startTimeVar = object.getProperty ("start_time");

    if (filePath.isEmpty())
        return makeErrorReply ("import_media_to_track requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("import_media_to_track requires a non-empty track_id");

    if (mediaType.isEmpty() || mediaType != juce::String ("audio"))
        return makeErrorReply ("import_media_to_track: unsupported or missing media_type (expected \"audio\")");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("import_media_to_track requires a numeric start_time field");

    const double startTimeSeconds = juce::jmax (0.0, static_cast<double> (startTimeVar));

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("Audio file does not exist: " + filePath);

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);

    const auto audioLengthSeconds = audioFile.getLength();

    if (audioLengthSeconds <= 0.0)
        return makeErrorReply ("Audio file has zero length: " + filePath);

    auto* targetTrack = findAudioTrackByID (*edit, trackId);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    ensureMonitoringPlugins (*targetTrack);
    const bool deleteExistingClips = (mode.isEmpty() || mode == juce::String ("destructive"));
    const auto appliedMode = deleteExistingClips ? juce::String ("destructive")
                                                  : juce::String ("non_destructive");

    const auto inserted = insertWaveClipWithUndoAndStartBake (*edit,
                                                              *targetTrack,
                                                              sourceFile,
                                                              startTimeSeconds,
                                                              audioLengthSeconds,
                                                              deleteExistingClips,
                                                              appliedMode);

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    juce::Logger::writeToLog ("ImportService::handleImportMediaToTrack: clipId=\""
                            + inserted.clipId
                            + "\" clip=\""
                            + inserted.clipName
                            + "\" trackId=\""
                            + inserted.trackItemId
                            + "\" source=\""
                            + sourceFile.getFullPathName()
                            + "\" lengthSec="
                            + juce::String (inserted.audioLengthSeconds, 3)
                            + " "
                            + describeTransportState (*edit));

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "success");
    response->setProperty ("action", "import_media_to_track");
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("resolved_start_time", inserted.startTimeSeconds);
    response->setProperty ("length", inserted.audioLengthSeconds);
    response->setProperty ("applied_mode", appliedMode);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("edit_length_seconds", inserted.editLengthSeconds);
    response->setProperty ("baking_status", "baking_started");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleImportAudio (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (filePath.isEmpty())
        return makeErrorReply ("import_audio requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("import_audio requires a non-empty track_id");

    auto sourceFile = juce::File (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("Audio file does not exist: " + filePath);

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
        return makeErrorReply ("Unsupported or unreadable audio file: " + filePath);

    const auto audioLengthSeconds = audioFile.getLength();

    if (audioLengthSeconds <= 0.0)
        return makeErrorReply ("Audio file has zero length: " + filePath);

    auto* targetTrack = findAudioTrackByID (*edit, trackId);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    ensureMonitoringPlugins (*targetTrack);

    const auto offsetTimeVar = object.getProperty ("offset_time");
    double startTimeSeconds = 0.0;

    if (offsetTimeVar.isDouble() || offsetTimeVar.isInt() || offsetTimeVar.isInt64())
    {
        startTimeSeconds = juce::jmax (0.0, static_cast<double> (offsetTimeVar));
    }
    else
    {
        if (auto* clipTrack = dynamic_cast<te::ClipTrack*> (targetTrack))
        {
            for (auto* clip : clipTrack->getClips())
            {
                if (clip == nullptr)
                    continue;

                const auto clipEnd = clip->getEditTimeRange().getEnd().inSeconds();

                if (clipEnd > startTimeSeconds)
                    startTimeSeconds = clipEnd;
            }
        }
    }

    const auto inserted = insertWaveClipWithUndoAndStartBake (*edit,
                                                              *targetTrack,
                                                              sourceFile,
                                                              startTimeSeconds,
                                                              audioLengthSeconds,
                                                              false,
                                                              "append");

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    juce::Logger::writeToLog ("ImportService::handleImportAudio: imported clip=\""
                              + inserted.clipName
                              + "\" clipId=\""
                              + inserted.clipId
                              + "\" trackId=\""
                              + inserted.trackItemId
                              + "\" source=\""
                              + sourceFile.getFullPathName()
                              + "\" clipLengthSeconds="
                              + juce::String (inserted.audioLengthSeconds, 3)
                              + " "
                              + describeTransportState (*edit));

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("action", "import_audio");
    response->setProperty ("message", "Audio clip imported");
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("clip_length_seconds", inserted.audioLengthSeconds);
    response->setProperty ("length", inserted.audioLengthSeconds);
    response->setProperty ("edit_length_seconds", inserted.editLengthSeconds);
    response->setProperty ("baking_status", "baking_started");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::handleWarmWaveformBake (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId  = object.getProperty ("track_id").toString().trim();
    const auto clipId   = object.getProperty ("clip_id").toString().trim();
    if (trackId.isEmpty())
        return makeErrorReply ("warm_waveform_bake requires a non-empty track_id");
    if (findAudioTrackByID (*edit, trackId) == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    juce::File sourceFile;
    double sourceOffsetSeconds = 0.0;
    double bakeLengthSeconds = -1.0;

    if (clipId.isNotEmpty())
    {
        auto* clip = findClipByID (*edit, clipId);
        auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);
        if (audioClip == nullptr)
            return makeErrorReply ("warm_waveform_bake clip_id not found or not audio: " + clipId);
        sourceFile = audioClip->getCurrentSourceFile();
        if (! sourceFile.existsAsFile())
            sourceFile = audioClip->getOriginalFile();
        if (! sourceFile.existsAsFile())
            return makeErrorReply ("warm_waveform_bake clip source file missing for clip_id: " + clipId);
        sourceOffsetSeconds = clip->getPosition().getOffset().inSeconds();
        bakeLengthSeconds = juce::jmax (0.0, clip->getPosition().getLength().inSeconds());
    }
    else
    {
        if (filePath.isEmpty())
            return makeErrorReply ("warm_waveform_bake requires clip_id or file_path");
        sourceFile = juce::File (filePath);
        if (! sourceFile.existsAsFile())
            return makeErrorReply ("Audio file does not exist: " + filePath);
    }

    TiledSpectrogramBaker::startBake (sourceFile.getFullPathName(),
                                      trackId,
                                      clipId,
                                      publishMessage,
                                      sourceOffsetSeconds,
                                      bakeLengthSeconds);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("cmd", "warm_waveform_bake");
    response->setProperty ("track_id", trackId);
    response->setProperty ("clip_id", clipId);
    response->setProperty ("file_path", sourceFile.getFullPathName());
    response->setProperty ("message", "Waveform bake started (no new clip inserted)");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ImportService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
