#include "TrackService.h"

#include "AudioFeatureService.h"

#include "../Core/VitKernelUtils.h"

#include <vector>

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

void appendTrackTreeState (juce::DynamicObject& trackObject, te::Track& track)
{
    auto* parentTrack = track.getParentTrack();
    auto* parentFolder = track.getParentFolderTrack();
    auto* folderTrack = dynamic_cast<te::FolderTrack*> (&track);
    juce::Array<juce::var> directChildIds;
    juce::Array<juce::var> descendantTrackIds;

    for (auto* child : track.getAllSubTracks (false))
        if (child != nullptr)
            directChildIds.add (child->itemID.toString());

    for (auto* child : track.getAllSubTracks (true))
        if (child != nullptr)
            descendantTrackIds.add (child->itemID.toString());

    trackObject.setProperty ("parent_track_id",
                             parentTrack != nullptr ? juce::var (parentTrack->itemID.toString()) : juce::var());
    trackObject.setProperty ("parent_folder_track_id",
                             parentFolder != nullptr ? juce::var (parentFolder->itemID.toString()) : juce::var());
    trackObject.setProperty ("depth", track.getTrackDepth());
    trackObject.setProperty ("track_depth", track.getTrackDepth());
    trackObject.setProperty ("child_track_ids", juce::var (directChildIds));
    trackObject.setProperty ("direct_child_track_ids", juce::var (directChildIds));
    trackObject.setProperty ("descendant_track_ids", juce::var (descendantTrackIds));
    trackObject.setProperty ("child_track_count", directChildIds.size());
    trackObject.setProperty ("descendant_track_count", descendantTrackIds.size());
    trackObject.setProperty ("has_child_tracks", directChildIds.size() > 0);
    trackObject.setProperty ("is_folder_track", folderTrack != nullptr);
    trackObject.setProperty ("is_folder_container", folderTrack != nullptr);
    trackObject.setProperty ("can_contain_child_tracks", folderTrack != nullptr);

    if (folderTrack == nullptr)
    {
        trackObject.setProperty ("is_submix_folder", false);
        trackObject.setProperty ("routing_bus_enabled", false);
        trackObject.setProperty ("folder_behavior", juce::var());
        return;
    }

    const auto routingBusEnabled = folderTrack->isSubmixFolder();
    trackObject.setProperty ("is_submix_folder", routingBusEnabled);
    trackObject.setProperty ("routing_bus_enabled", routingBusEnabled);
    trackObject.setProperty ("folder_behavior", routingBusEnabled ? "routing_bus" : "container");
    trackObject.setProperty ("mute", folderTrack->isMuted (false));
    trackObject.setProperty ("solo", folderTrack->isSolo (false));

    if (auto* volumePlugin = folderTrack->getVolumePlugin())
    {
        const auto db = volumePlugin->getVolumeDb();
        trackObject.setProperty ("db", db);
        trackObject.setProperty ("volume_db", db);
        trackObject.setProperty ("gain_db", db);
        trackObject.setProperty ("fader_db", db);
        trackObject.setProperty ("pan", volumePlugin->getPan());
        trackObject.setProperty ("pan_value", volumePlugin->getPan());
    }
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
    appendTrackTreeState (*trackObject, track);

    if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (&track))
    {
        trackObject->setProperty ("mute", audioTrack->isMuted (false));
        trackObject->setProperty ("solo", audioTrack->isSolo (false));

        if (auto* volumePlugin = audioTrack->getVolumePlugin())
        {
            const auto db = volumePlugin->getVolumeDb();
            trackObject->setProperty ("db", db);
            trackObject->setProperty ("volume_db", db);
            trackObject->setProperty ("gain_db", db);
            trackObject->setProperty ("fader_db", db);
            trackObject->setProperty ("pan", volumePlugin->getPan());
            trackObject->setProperty ("pan_value", volumePlugin->getPan());
        }
    }

    return juce::var (trackObject.release());
}

te::Track* findTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr && track->itemID.toString() == trackID)
            return track;

    return nullptr;
}

te::Track::Ptr findTrackPtrByID (te::Edit& edit, const juce::String& trackID)
{
    return te::Track::Ptr (findTrackByID (edit, trackID));
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

te::FolderTrack* findFolderTrackByID (te::Edit& edit, const juce::String& trackID)
{
    return dynamic_cast<te::FolderTrack*> (findTrackByID (edit, trackID));
}

juce::String firstNonEmptyProperty (const juce::DynamicObject& object,
                                    std::initializer_list<const char*> keys)
{
    for (const auto* key : keys)
    {
        auto text = object.getProperty (key).toString().trim();

        if (text.isNotEmpty())
            return text;
    }

    return {};
}

juce::var firstPresentProperty (const juce::DynamicObject& object,
                                std::initializer_list<const char*> keys)
{
    for (const auto* key : keys)
    {
        auto value = object.getProperty (key);

        if (! value.isVoid())
            return value;
    }

    return {};
}

bool boolPropertyOrDefault (const juce::DynamicObject& object,
                            std::initializer_list<const char*> keys,
                            bool defaultValue)
{
    auto value = firstPresentProperty (object, keys);
    return value.isBool() ? static_cast<bool> (value) : defaultValue;
}

te::Track* resolveOptionalTrack (te::Edit& edit,
                                 const juce::String& trackID,
                                 const juce::String& fieldName,
                                 juce::String& error)
{
    if (trackID.isEmpty())
        return nullptr;

    auto* track = findTrackByID (edit, trackID);

    if (track == nullptr)
        error = fieldName + " not found: " + trackID;

    return track;
}

bool folderHasUserBusPlugins (te::FolderTrack& folder)
{
    for (auto* plugin : folder.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        if (dynamic_cast<te::VCAPlugin*> (plugin) != nullptr
            || dynamic_cast<te::TextPlugin*> (plugin) != nullptr
            || dynamic_cast<te::VolumeAndPanPlugin*> (plugin) != nullptr
            || dynamic_cast<te::LevelMeterPlugin*> (plugin) != nullptr)
            continue;

        return true;
    }

    return false;
}

bool ensureFolderRoutingBusState (te::FolderTrack& folder,
                                  bool enabled,
                                  bool forceClearPlugins,
                                  juce::String& error)
{
    auto& edit = folder.pluginList.getEdit();

    if (enabled)
    {
        if (! folder.isSubmixFolder())
        {
            folder.pluginList.clear();
            folder.pluginList.addDefaultTrackPlugins (false);
        }
        else
        {
            if (folder.getVolumePlugin() == nullptr)
            {
                auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {});

                if (plugin != nullptr)
                    folder.pluginList.insertPlugin (plugin, -1, nullptr);
            }

            if (folder.pluginList.findFirstPluginOfType<te::LevelMeterPlugin>() == nullptr)
            {
                auto plugin = edit.getPluginCache().createNewPlugin (te::LevelMeterPlugin::xmlTypeName, {});

                if (plugin != nullptr)
                    folder.pluginList.insertPlugin (plugin, -1, nullptr);
            }
        }

        folder.flushStateToValueTree();
        return true;
    }

    if (! folder.isSubmixFolder())
        return false;

    if (folderHasUserBusPlugins (folder) && ! forceClearPlugins)
    {
        error = "Folder bus has user plugins; pass force_clear_plugins=true only after explicit user confirmation";
        return false;
    }

    folder.pluginList.clear();
    folder.pluginList.addDefaultTrackPlugins (true);
    folder.flushStateToValueTree();
    return true;
}

juce::Array<juce::var> stringArrayToVarArray (const juce::StringArray& strings)
{
    juce::Array<juce::var> out;

    for (const auto& text : strings)
        out.add (text);

    return out;
}

juce::StringArray trackIDsFromGroupObject (const juce::DynamicObject& group)
{
    juce::StringArray ids;

    if (auto* trackIds = group.getProperty ("track_ids").getArray())
        for (const auto& value : *trackIds)
            if (auto text = value.toString().trim(); text.isNotEmpty())
                ids.addIfNotAlreadyThere (text);

    if (auto* assignments = group.getProperty ("assignments").getArray())
    {
        for (const auto& assignmentVar : *assignments)
        {
            if (auto* assignment = assignmentVar.getDynamicObject())
            {
                auto text = firstNonEmptyProperty (*assignment, { "track_id", "id" });

                if (text.isNotEmpty())
                    ids.addIfNotAlreadyThere (text);
            }
        }
    }

    return ids;
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
    response->setProperty ("track_name", track.getName());
    response->setProperty ("mute", track.isMuted (false));
    response->setProperty ("solo", track.isSolo (false));

    if (auto* volumePlugin = track.getVolumePlugin())
    {
        const auto db = volumePlugin->getVolumeDb();
        response->setProperty ("db", db);
        response->setProperty ("volume_db", db);
        response->setProperty ("gain_db", db);
        response->setProperty ("fader_db", db);
        response->setProperty ("pan", volumePlugin->getPan());
        response->setProperty ("pan_value", volumePlugin->getPan());
    }

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String buildGenericTrackReply (te::Track& track, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    auto pluginsArray = createPluginArray (track);

    response->setProperty ("status", "ok");
    response->setProperty ("message", message);
    response->setProperty ("track_id", track.itemID.toString());
    response->setProperty ("track_name", track.getName());
    response->setProperty ("name", track.getName());
    response->setProperty ("track_type", getTrackKind (track));
    response->setProperty ("plugin_count", pluginsArray.size());
    response->setProperty ("plugins", juce::var (pluginsArray));
    appendTrackTreeState (*response, track);
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

juce::String TrackService::handleCreateFolderTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto folderName = firstNonEmptyProperty (object, { "folder_name", "track_name", "name" });
    const auto parentTrackID = firstNonEmptyProperty (object, { "parent_track_id", "parent_folder_track_id" });
    const auto precedingTrackID = firstNonEmptyProperty (object, { "preceding_track_id", "after_track_id" });
    const auto asSubmix = boolPropertyOrDefault (object, { "routing_bus_enabled", "as_submix", "bus_enabled" }, false);
    juce::String error;

    auto* parentTrack = resolveOptionalTrack (*edit, parentTrackID, "parent_track_id", error);

    if (error.isNotEmpty())
        return makeErrorReply (error);

    if (parentTrack != nullptr && dynamic_cast<te::FolderTrack*> (parentTrack) == nullptr)
        return makeErrorReply ("parent_track_id must refer to a folder container track");

    auto* precedingTrack = resolveOptionalTrack (*edit, precedingTrackID, "preceding_track_id", error);

    if (error.isNotEmpty())
        return makeErrorReply (error);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Create folder track");
    auto folderTrack = edit->insertNewFolderTrack (te::TrackInsertPoint (parentTrack, precedingTrack), nullptr, asSubmix);

    if (folderTrack == nullptr)
        return makeErrorReply ("Failed to insert folder track");

    if (folderName.isEmpty())
        folderName = folderTrack->getName();
    else
        folderTrack->setName (folderName);

    folderTrack->state.setProperty ("vit_type", "folder_container", &undo);
    folderTrack->state.setProperty ("vit_folder_kind", asSubmix ? "routing_bus" : "container", &undo);
    folderTrack->flushStateToValueTree();
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Folder track created in memory but failed to save project");

    return buildGenericTrackReply (*folderTrack, "Folder track created");
}

juce::String TrackService::handleMoveTrackToFolder (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = firstNonEmptyProperty (object, { "track_id", "child_track_id" });
    const auto folderTrackID = firstNonEmptyProperty (object, { "folder_track_id", "parent_track_id", "parent_folder_track_id" });
    const auto precedingTrackID = firstNonEmptyProperty (object, { "preceding_track_id", "after_track_id" });

    if (trackID.isEmpty())
        return makeErrorReply ("track.move_to_folder requires a non-empty track_id");

    auto targetTrack = findTrackPtrByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    if (dynamic_cast<te::MasterTrack*> (targetTrack.get()) != nullptr)
        return makeErrorReply ("Master track cannot be moved into a folder");

    if (! targetTrack->isMovable())
        return makeErrorReply ("Track type cannot be moved into a folder: " + trackID);

    te::FolderTrack* folderTrack = nullptr;

    if (folderTrackID.isNotEmpty())
    {
        folderTrack = findFolderTrackByID (*edit, folderTrackID);

        if (folderTrack == nullptr)
            return makeErrorReply ("Folder track not found for folder_track_id: " + folderTrackID);

        if (targetTrack.get() == folderTrack || folderTrack->isAChildOf (*targetTrack))
            return makeErrorReply ("Cannot move a track into itself or one of its descendants");
    }

    juce::String error;
    auto* precedingTrack = resolveOptionalTrack (*edit, precedingTrackID, "preceding_track_id", error);

    if (error.isNotEmpty())
        return makeErrorReply (error);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Move track to folder");
    const auto movedTrackID = targetTrack->itemID.toString();
    const auto movedTrackName = targetTrack->getName();
    edit->moveTrack (targetTrack, te::TrackInsertPoint (folderTrack, precedingTrack));
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Track moved in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", folderTrack != nullptr ? "Track moved to folder" : "Track moved to top level");
    response->setProperty ("track_id", movedTrackID);
    response->setProperty ("track_name", movedTrackName);
    response->setProperty ("folder_track_id", folderTrack != nullptr ? juce::var (folderTrack->itemID.toString()) : juce::var());

    if (auto* movedTrack = findTrackByID (*edit, movedTrackID))
        appendTrackTreeState (*response, *movedTrack);

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleSetFolderRoutingBus (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto folderTrackID = firstNonEmptyProperty (object, { "folder_track_id", "track_id" });

    if (folderTrackID.isEmpty())
        return makeErrorReply ("folder_track.set_routing_bus_enabled requires a non-empty folder_track_id or track_id");

    auto* folderTrack = findFolderTrackByID (*edit, folderTrackID);

    if (folderTrack == nullptr)
        return makeErrorReply ("Folder track not found for folder_track_id: " + folderTrackID);

    const auto enabledVar = firstPresentProperty (object, { "routing_bus_enabled", "enabled", "bus_enabled", "as_submix" });

    if (! enabledVar.isBool())
        return makeErrorReply ("folder_track.set_routing_bus_enabled requires a boolean routing_bus_enabled field");

    const auto enabled = static_cast<bool> (enabledVar);
    const auto forceClearPlugins = boolPropertyOrDefault (object, { "force_clear_plugins" }, false);
    juce::String error;
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set folder routing bus");

    if (! ensureFolderRoutingBusState (*folderTrack, enabled, forceClearPlugins, error))
    {
        if (error.isNotEmpty())
            return makeErrorReply (error);
    }

    folderTrack->state.setProperty ("vit_folder_kind", enabled ? "routing_bus" : "container", &undo);
    folderTrack->flushStateToValueTree();
    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Folder routing changed in memory but failed to save project");

    return buildGenericTrackReply (*folderTrack, enabled ? "Folder bus routing enabled" : "Folder bus routing disabled");
}

juce::String TrackService::handleApplyTrackOrganization (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto* groups = object.getProperty ("groups").getArray();

    if (groups == nullptr)
        groups = object.getProperty ("group_proposals").getArray();

    if (groups == nullptr || groups->isEmpty())
        return makeErrorReply ("project.apply_track_organization requires a non-empty groups array");

    struct GroupPlan
    {
        juce::String folderName;
        juce::String groupID;
        juce::String label;
        bool routingBusEnabled = false;
        juce::StringArray trackIDs;
    };

    std::vector<GroupPlan> plans;
    juce::StringArray allTrackIDs;

    for (const auto& groupVar : *groups)
    {
        auto* group = groupVar.getDynamicObject();

        if (group == nullptr)
            continue;

        GroupPlan plan;
        plan.folderName = firstNonEmptyProperty (*group, { "folder_name", "proposed_folder", "label", "name", "group_label", "group_id" });
        plan.groupID = firstNonEmptyProperty (*group, { "group_id", "id" });
        plan.label = firstNonEmptyProperty (*group, { "label", "group_label", "name" });
        plan.routingBusEnabled = boolPropertyOrDefault (*group, { "routing_bus_enabled", "as_submix", "bus_enabled" }, false);
        plan.trackIDs = trackIDsFromGroupObject (*group);

        if (plan.folderName.isEmpty())
            plan.folderName = "Folder";

        if (plan.trackIDs.isEmpty())
            continue;

        for (const auto& trackID : plan.trackIDs)
        {
            if (allTrackIDs.contains (trackID))
                return makeErrorReply ("Track appears in more than one organization group: " + trackID);

            auto* track = findTrackByID (*edit, trackID);

            if (track == nullptr)
                return makeErrorReply ("Track not found for organization track_id: " + trackID);

            if (dynamic_cast<te::MasterTrack*> (track) != nullptr)
                return makeErrorReply ("Master track cannot be organized into a folder: " + trackID);

            if (! track->isMovable())
                return makeErrorReply ("Track type cannot be organized into a folder: " + trackID);

            allTrackIDs.add (trackID);
        }

        plans.push_back (std::move (plan));
    }

    if (plans.empty())
        return makeErrorReply ("project.apply_track_organization found no groups with track_ids or assignments");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Apply track organization");
    juce::Array<juce::var> groupResults;
    juce::StringArray createdFolderIDs;
    juce::StringArray movedTrackIDs;
    te::EditItemID previousTopLevelTrackID;

    if (auto* lastTopLevelTrack = te::getTopLevelTracks (*edit).getLast())
        previousTopLevelTrackID = lastTopLevelTrack->itemID;

    for (const auto& plan : plans)
    {
        auto folderTrack = edit->insertNewFolderTrack (te::TrackInsertPoint (te::EditItemID(), previousTopLevelTrackID), nullptr, plan.routingBusEnabled);

        if (folderTrack == nullptr)
            return makeErrorReply ("Failed to create folder for group: " + plan.folderName);

        folderTrack->setName (plan.folderName);
        folderTrack->state.setProperty ("vit_type", "folder_container", &undo);
        folderTrack->state.setProperty ("vit_folder_kind", plan.routingBusEnabled ? "routing_bus" : "container", &undo);

        if (plan.groupID.isNotEmpty())
            folderTrack->state.setProperty ("vit_tom_group_id", plan.groupID, &undo);

        if (plan.label.isNotEmpty())
            folderTrack->state.setProperty ("vit_tom_group_label", plan.label, &undo);

        folderTrack->flushStateToValueTree();
        createdFolderIDs.add (folderTrack->itemID.toString());

        te::EditItemID previousChildID;
        juce::StringArray groupMovedTrackIDs;

        for (const auto& trackID : plan.trackIDs)
        {
            auto track = findTrackPtrByID (*edit, trackID);

            if (track == nullptr)
                continue;

            const auto movedTrackID = track->itemID.toString();
            edit->moveTrack (track, te::TrackInsertPoint (folderTrack->itemID, previousChildID));
            previousChildID = track->itemID;
            movedTrackIDs.add (movedTrackID);
            groupMovedTrackIDs.add (movedTrackID);
        }

        auto groupResult = std::make_unique<juce::DynamicObject>();
        groupResult->setProperty ("folder_track_id", folderTrack->itemID.toString());
        groupResult->setProperty ("folder_name", folderTrack->getName());
        groupResult->setProperty ("routing_bus_enabled", folderTrack->isSubmixFolder());
        groupResult->setProperty ("moved_track_ids", juce::var (stringArrayToVarArray (groupMovedTrackIDs)));
        groupResult->setProperty ("moved_track_count", groupMovedTrackIDs.size());
        groupResults.add (juce::var (groupResult.release()));
        previousTopLevelTrackID = folderTrack->itemID;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Track organization applied in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track organization applied");
    response->setProperty ("created_folder_ids", juce::var (stringArrayToVarArray (createdFolderIDs)));
    response->setProperty ("created_folder_count", createdFolderIDs.size());
    response->setProperty ("moved_track_ids", juce::var (stringArrayToVarArray (movedTrackIDs)));
    response->setProperty ("moved_track_count", movedTrackIDs.size());
    response->setProperty ("groups", juce::var (groupResults));
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
    AudioFeatureService::releaseTrackMappings (releasedTrackID);
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

juce::String TrackService::handleRenameTrack (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    auto newName = object.getProperty ("name").toString().trim();

    if (newName.isEmpty())
        newName = object.getProperty ("track_name").toString().trim();

    if (trackID.isEmpty())
        return makeErrorReply ("rename_track requires a non-empty track_id");

    if (newName.isEmpty())
        return makeErrorReply ("rename_track requires a non-empty name or track_name field");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    const auto oldName = targetTrack->getName();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rename track");
    targetTrack->setName (newName);
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track renamed");
    response->setProperty ("track_id", targetTrack->itemID.toString());
    response->setProperty ("old_name", oldName);
    response->setProperty ("track_name", targetTrack->getName());
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

juce::String TrackService::handleSetVolumeBatch (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto actionsVar = object.getProperty ("actions");
    if (! actionsVar.isArray())
        actionsVar = object.getProperty ("pending_actions");

    auto* actions = actionsVar.getArray();
    if (actions == nullptr || actions->isEmpty())
        return makeErrorReply ("track.volume.set_batch requires actions");

    struct Target
    {
        te::AudioTrack* track = nullptr;
        juce::String trackId;
        double requestedDb = 0.0;
        float appliedDb = 0.0f;
    };

    std::vector<Target> targets;
    targets.reserve (static_cast<size_t> (actions->size()));
    juce::StringArray seenTrackIds;

    for (const auto& actionVar : *actions)
    {
        auto* action = actionVar.getDynamicObject();
        if (action == nullptr)
            return makeErrorReply ("track.volume.set_batch contains a non-object action");

        auto* args = action;
        if (auto* nestedArgs = action->getProperty ("args").getDynamicObject())
            args = nestedArgs;

        const auto trackId = args->getProperty ("track_id").toString().trim().isNotEmpty()
                                 ? args->getProperty ("track_id").toString().trim()
                                 : action->getProperty ("track_id").toString().trim();
        if (trackId.isEmpty())
            return makeErrorReply ("track.volume.set_batch action requires track_id");
        if (seenTrackIds.contains (trackId))
            return makeErrorReply ("track.volume.set_batch contains duplicate track_id: " + trackId);

        auto dbVar = args->getProperty ("db");
        if (dbVar.isVoid())
            dbVar = args->getProperty ("target_db");
        if (dbVar.isVoid())
            dbVar = args->getProperty ("volume_db");
        if (dbVar.isVoid())
            dbVar = action->getProperty ("target_db");
        if (! dbVar.isDouble() && ! dbVar.isInt() && ! dbVar.isInt64())
            return makeErrorReply ("track.volume.set_batch action requires numeric db: " + trackId);

        auto* targetTrack = findAudioTrackByID (*edit, trackId);
        if (targetTrack == nullptr)
            return makeErrorReply ("Audio track not found for track_id: " + trackId);

        ensureMonitoringPlugins (*targetTrack);
        if (targetTrack->getVolumePlugin() == nullptr)
            return makeErrorReply ("Volume plugin unavailable for track_id: " + trackId);

        const auto requestedDb = static_cast<double> (dbVar);
        targets.push_back ({ targetTrack, trackId, requestedDb, convertRequestedDbToPluginDb (requestedDb) });
        seenTrackIds.add (trackId);
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set track volume batch");

    juce::Array<juce::var> results;
    results.ensureStorageAllocated (static_cast<int> (targets.size()));
    for (const auto& target : targets)
    {
        target.track->getVolumePlugin()->setVolumeDb (target.appliedDb);
        target.track->flushStateToValueTree();

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("status", "ok");
        row->setProperty ("track_id", target.trackId);
        row->setProperty ("track_name", target.track->getName());
        row->setProperty ("requested_db", target.requestedDb);
        row->setProperty ("volume_db", static_cast<double> (target.track->getVolumePlugin()->getVolumeDb()));
        results.add (juce::var (row.release()));
    }

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track volume batch applied");
    response->setProperty ("schema_version", "track.volume.set_batch.v1");
    response->setProperty ("applied_count", static_cast<int> (targets.size()));
    response->setProperty ("failed_count", 0);
    response->setProperty ("actions", results);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackService::handleSetPan (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    auto panVar = object.getProperty ("pan");

    if (panVar.isVoid())
        panVar = object.getProperty ("pan_value");

    if (trackID.isEmpty())
        return makeErrorReply ("set_pan requires a non-empty track_id");

    if (! panVar.isDouble() && ! panVar.isInt() && ! panVar.isInt64())
        return makeErrorReply ("set_pan requires a numeric pan field");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    ensureMonitoringPlugins (*targetTrack);

    auto* volumePlugin = targetTrack->getVolumePlugin();

    if (volumePlugin == nullptr)
        return makeErrorReply ("Volume/pan plugin unavailable for track_id: " + trackID);

    const auto pan = juce::jlimit (-1.0f, 1.0f, static_cast<float> (static_cast<double> (panVar)));
    volumePlugin->setPan (pan);
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();

    return buildTrackReply (*targetTrack, "Track pan updated");
}

juce::String TrackService::handleSetPanBatch (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto actionsVar = object.getProperty ("actions");
    if (! actionsVar.isArray())
        actionsVar = object.getProperty ("pending_actions");
    auto* actions = actionsVar.getArray();
    if (actions == nullptr || actions->isEmpty())
        return makeErrorReply ("track.pan.set_batch requires actions");

    struct Target
    {
        te::AudioTrack* track = nullptr;
        juce::String trackId;
        float requestedPan = 0.0f;
    };
    std::vector<Target> targets;
    targets.reserve (static_cast<size_t> (actions->size()));
    juce::StringArray seenTrackIds;

    for (const auto& actionVar : *actions)
    {
        auto* action = actionVar.getDynamicObject();
        if (action == nullptr)
            return makeErrorReply ("track.pan.set_batch contains a non-object action");
        auto* args = action;
        if (auto* nestedArgs = action->getProperty ("args").getDynamicObject())
            args = nestedArgs;
        const auto trackId = args->getProperty ("track_id").toString().trim().isNotEmpty()
                                 ? args->getProperty ("track_id").toString().trim()
                                 : action->getProperty ("track_id").toString().trim();
        if (trackId.isEmpty())
            return makeErrorReply ("track.pan.set_batch action requires track_id");
        if (seenTrackIds.contains (trackId))
            return makeErrorReply ("track.pan.set_batch contains duplicate track_id: " + trackId);

        auto panVar = args->getProperty ("target_pan");
        if (panVar.isVoid())
            panVar = args->getProperty ("pan");
        if (panVar.isVoid())
            panVar = args->getProperty ("pan_value");
        if (panVar.isVoid())
            panVar = action->getProperty ("target_pan");
        if (! panVar.isDouble() && ! panVar.isInt() && ! panVar.isInt64())
            return makeErrorReply ("track.pan.set_batch action requires numeric target_pan: " + trackId);
        const auto requestedPan = static_cast<float> (static_cast<double> (panVar));
        if (requestedPan < -1.0f || requestedPan > 1.0f)
            return makeErrorReply ("track.pan.set_batch target_pan outside [-1,1]: " + trackId);

        auto* targetTrack = findAudioTrackByID (*edit, trackId);
        if (targetTrack == nullptr)
            return makeErrorReply ("Audio track not found for track_id: " + trackId);
        ensureMonitoringPlugins (*targetTrack);
        if (targetTrack->getVolumePlugin() == nullptr)
            return makeErrorReply ("Volume/pan plugin unavailable for track_id: " + trackId);
        targets.push_back ({ targetTrack, trackId, requestedPan });
        seenTrackIds.add (trackId);
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set track pan batch");
    juce::Array<juce::var> results;
    results.ensureStorageAllocated (static_cast<int> (targets.size()));
    for (const auto& target : targets)
    {
        target.track->getVolumePlugin()->setPan (target.requestedPan);
        target.track->flushStateToValueTree();
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("status", "ok");
        row->setProperty ("track_id", target.trackId);
        row->setProperty ("track_name", target.track->getName());
        row->setProperty ("pan", static_cast<double> (target.track->getVolumePlugin()->getPan()));
        results.add (juce::var (row.release()));
    }
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track pan batch applied");
    response->setProperty ("schema_version", "track.pan.set_batch.v1");
    response->setProperty ("applied_count", static_cast<int> (targets.size()));
    response->setProperty ("failed_count", 0);
    response->setProperty ("actions", results);
    return juce::JSON::toString (juce::var (response.release()));
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

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set track mute");
    ensureMonitoringPlugins (*targetTrack);

    targetTrack->state.setProperty (te::IDs::mute, static_cast<bool> (muteVar), &undo);
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();

    return buildTrackReply (*targetTrack,
                            static_cast<bool> (muteVar) ? "Track muted" : "Track unmuted");
}

juce::String TrackService::handleSetSolo (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    auto soloVar = object.getProperty ("solo");

    if (! soloVar.isBool())
        soloVar = object.getProperty ("is_solo");

    if (trackID.isEmpty())
        return makeErrorReply ("set_solo requires a non-empty track_id");

    if (! soloVar.isBool())
        return makeErrorReply ("set_solo requires a boolean solo or is_solo field");

    auto* targetTrack = findAudioTrackByID (*edit, trackID);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set track solo");
    targetTrack->state.setProperty (te::IDs::solo, static_cast<bool> (soloVar), &undo);
    targetTrack->flushStateToValueTree();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated();

    return buildTrackReply (*targetTrack,
                            static_cast<bool> (soloVar) ? "Track soloed" : "Track unsoloed");
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
