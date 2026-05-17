#include "TrackService.h"

#include "TiledSpectrogramBaker.h"

#include "../Core/VitKernelUtils.h"

namespace vit
{

namespace
{

juce::String pluginItemIdString (const te::Plugin& plugin)
{
    const auto id = plugin.itemID.toString().trim();
    jassert (id.isNotEmpty());
    return id;
}

juce::var createPluginState (te::Plugin& plugin)
{
    auto pluginObject = std::make_unique<juce::DynamicObject>();
    const auto pluginItemId = pluginItemIdString (plugin);
    pluginObject->setProperty ("id", pluginItemId);
    pluginObject->setProperty ("plugin_item_id", pluginItemId);
    pluginObject->setProperty ("name", plugin.getName());
    pluginObject->setProperty ("type", plugin.getPluginType());
    pluginObject->setProperty ("enabled", plugin.isEnabled());
    return juce::var (pluginObject.release());
}

juce::Array<juce::var> createPluginArray (te::Track& track)
{
    juce::Array<juce::var> pluginsArray;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        pluginsArray.add (createPluginState (*plugin));
    }

    return pluginsArray;
}

juce::String getTrackKind (const te::Track& track)
{
    if (dynamic_cast<const te::MasterTrack*> (&track) != nullptr)
        return "master";

    if (dynamic_cast<const te::FolderTrack*> (&track) != nullptr)
        return "folder";

    if (dynamic_cast<const te::AudioTrack*> (&track) != nullptr)
        return "hybrid";

    return "track";
}

juce::var createTrackState (te::Track& track)
{
    auto trackObject = std::make_unique<juce::DynamicObject>();
    auto pluginsArray = createPluginArray (track);

    trackObject->setProperty ("track_id", track.itemID.toString());
    trackObject->setProperty ("name", track.getName());
    trackObject->setProperty ("track_type", getTrackKind (track));
    trackObject->setProperty ("is_audio", track.isAudioTrack());
    trackObject->setProperty ("is_audio_track", track.isAudioTrack());
    trackObject->setProperty ("vit_type", track.state.getProperty ("vit_type").toString());
    trackObject->setProperty ("vit_intent", track.state.getProperty ("vit_intent").toString());
    trackObject->setProperty ("plugin_count", pluginsArray.size());
    trackObject->setProperty ("plugins", juce::var (pluginsArray));
    return juce::var (trackObject.release());
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

juce::String buildTrackReply (te::AudioTrack& track, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();

    response->setProperty ("status", "ok");
    response->setProperty ("message", message);
    response->setProperty ("track_id", track.itemID.toString());
    response->setProperty ("mute", track.isMuted (false));

    if (auto* volumePlugin = track.getVolumePlugin())
        response->setProperty ("db", volumePlugin->getVolumeDb());

    return juce::JSON::toString (juce::var (response.release()));
}

float convertRequestedDbToPluginDb (double requestedDb)
{
    constexpr float minimumVolumeDb = -100.0f;
    const auto gain = juce::Decibels::decibelsToGain (static_cast<float> (requestedDb), minimumVolumeDb);
    return juce::Decibels::gainToDecibels (gain, minimumVolumeDb);
}

} // namespace

TrackService::TrackService (EditGetter editGetter,
                            SaveProjectAction saveProjectAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction))
{
}

juce::String TrackService::handleListTracks (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto response = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> tracksArray;

    for (auto* track : te::getAllTracks (*edit))
    {
        if (track == nullptr)
            continue;

        tracksArray.add (createTrackState (*track));
    }

    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track list fetched");
    response->setProperty ("tracks", juce::var (tracksArray));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleAppendGhostTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackName = object.getProperty ("track_name").toString().trim();
    const auto intent = object.getProperty ("intent").toString().trim();

    if (trackName.isEmpty())
        return makeErrorReply ("append_ghost_track requires a non-empty track_name");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Append ghost track");
    auto newTrack = edit->insertNewAudioTrack (te::TrackInsertPoint::getEndOfTracks (*edit), nullptr, true);

    if (newTrack == nullptr)
        return makeErrorReply ("Failed to insert new audio track");

    ensureMonitoringPlugins (*newTrack);
    ensureSingleRackForTrack (*newTrack);
    newTrack->setName (trackName);
    newTrack->state.setProperty ("vit_type", "ghost", &undo);
    newTrack->state.setProperty ("vit_intent", intent, &undo);
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Ghost track added in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Ghost track appended");
    response->setProperty ("track_name", trackName);
    response->setProperty ("track_id", newTrack->itemID.toString());
    response->setProperty ("intent", intent);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleAddTrack (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add track");
    auto newTrack = edit->insertNewAudioTrack (te::TrackInsertPoint::getEndOfTracks (*edit), nullptr, true);

    if (newTrack == nullptr)
        return makeErrorReply ("Failed to insert new track");

    ensureMonitoringPlugins (*newTrack);
    ensureSingleRackForTrack (*newTrack);

    int audioCount = 0;

    for (auto* track : te::getAllTracks (*edit))
        if (dynamic_cast<te::AudioTrack*> (track) != nullptr)
            ++audioCount;

    newTrack->setName ("Track " + juce::String (audioCount));

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Track added in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track added");
    response->setProperty ("track_id", newTrack->itemID.toString());
    response->setProperty ("track_name", newTrack->getName());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleDeleteTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();

    if (trackID.isEmpty())
        return makeErrorReply ("delete_track requires a non-empty track_id");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    int audioTrackCount = 0;

    for (auto* track : te::getAllTracks (*edit))
        if (dynamic_cast<te::AudioTrack*> (track) != nullptr)
            ++audioTrackCount;

    if (audioTrackCount <= 1)
        return makeErrorReply ("Cannot delete the last audio track");

    const auto releasedTrackID = targetTrack->itemID.toString();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete track");
    edit->deleteTrack (targetTrack);
    TiledSpectrogramBaker::releaseTrackMappings (releasedTrackID);
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Track removed in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Audio track deleted");
    response->setProperty ("track_id", releasedTrackID);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleSetVolume (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto dbVar = object.getProperty ("db");

    if (trackID.isEmpty())
        return makeErrorReply ("set_volume requires a non-empty track_id");

    if (! dbVar.isDouble() && ! dbVar.isInt() && ! dbVar.isInt64())
        return makeErrorReply ("set_volume requires a numeric db field");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    ensureMonitoringPlugins (*targetTrack);

    auto* volumePlugin = targetTrack->getVolumePlugin();

    if (volumePlugin == nullptr)
        return makeErrorReply ("Volume plugin unavailable for track_id: " + trackID);

    const auto requestedDb = static_cast<double> (dbVar);
    const auto appliedDb = convertRequestedDbToPluginDb (requestedDb);
    volumePlugin->setVolumeDb (appliedDb);
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();
    juce::Logger::writeToLog ("[INFO] Applied Volume DB: "
                              + juce::String (appliedDb, 1)
                              + " (Converted to Gain)");

    return buildTrackReply (*targetTrack, "Track volume updated");
}

juce::String TrackService::handleSetMute (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto muteVar = object.getProperty ("mute");

    if (trackID.isEmpty())
        return makeErrorReply ("set_mute requires a non-empty track_id");

    if (! muteVar.isBool())
        return makeErrorReply ("set_mute requires a boolean mute field");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    ensureMonitoringPlugins (*targetTrack);

    targetTrack->setMute (static_cast<bool> (muteVar));
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();

    return buildTrackReply (*targetTrack,
                            static_cast<bool> (muteVar) ? "Track muted" : "Track unmuted");
}

juce::String TrackService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
