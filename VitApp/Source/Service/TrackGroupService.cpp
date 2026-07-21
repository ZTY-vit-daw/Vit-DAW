#include "TrackGroupService.h"

#include <algorithm>
#include <cmath>
#include <vector>

namespace vit
{

namespace
{

const juce::Identifier kGroupsTree ("VIT_TRACK_GROUPS");
const juce::Identifier kGroupTree ("VIT_TRACK_GROUP");
const juce::Identifier kSchemaVersion ("schema_version");
const juce::Identifier kGroupID ("group_id");
const juce::Identifier kName ("name");
const juce::Identifier kColor ("color");
const juce::Identifier kType ("type");
const juce::Identifier kOrigin ("origin");
const juce::Identifier kEnabled ("enabled");
const juce::Identifier kSuspended ("suspended");
const juce::Identifier kMemberTrackIDs ("member_track_ids");
const juce::Identifier kLinkVolume ("link_volume");
const juce::Identifier kLinkPan ("link_pan");
const juce::Identifier kLinkMute ("link_mute");
const juce::Identifier kLinkSolo ("link_solo");
const juce::Identifier kCreatedAt ("created_at");
const juce::Identifier kUpdatedAt ("updated_at");

constexpr double kMinimumVolumeDb = -100.0;
constexpr double kMaximumVolumeDb = 12.0;

juce::String nowStamp()
{
    return juce::String (juce::Time::getCurrentTime().toMilliseconds());
}

juce::ValueTree groupsTree (te::Edit& edit)
{
    return edit.state.getChildWithName (kGroupsTree);
}

juce::ValueTree ensureGroupsTree (te::Edit& edit, juce::UndoManager* undo)
{
    auto tree = groupsTree (edit);
    if (tree.isValid())
        return tree;

    tree = juce::ValueTree (kGroupsTree);
    tree.setProperty (kSchemaVersion, "vit_track_groups.v1", nullptr);
    edit.state.addChild (tree, -1, undo);
    return tree;
}

juce::String firstStringProperty (const juce::DynamicObject& object,
                                  std::initializer_list<const char*> keys)
{
    for (auto* key : keys)
    {
        const auto text = object.getProperty (key).toString().trim();
        if (text.isNotEmpty())
            return text;
    }

    return {};
}

bool boolFromVar (const juce::var& value, bool fallback)
{
    if (value.isVoid())
        return fallback;

    if (value.isBool())
        return static_cast<bool> (value);

    const auto text = value.toString().trim().toLowerCase();
    if (text == "true" || text == "1" || text == "yes" || text == "on")
        return true;
    if (text == "false" || text == "0" || text == "no" || text == "off")
        return false;

    return fallback;
}

bool boolPropertyOrDefault (const juce::DynamicObject& object,
                            std::initializer_list<const char*> keys,
                            bool fallback)
{
    for (auto* key : keys)
    {
        const auto value = object.getProperty (key);
        if (! value.isVoid())
            return boolFromVar (value, fallback);
    }

    return fallback;
}

bool numericVarToDouble (const juce::var& value, double& out)
{
    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        out = static_cast<double> (value);
        return std::isfinite (out);
    }

    const auto text = value.toString().trim();
    if (text.isEmpty())
        return false;

    out = text.getDoubleValue();
    return std::isfinite (out);
}

bool firstNumericProperty (const juce::DynamicObject& object,
                           std::initializer_list<const char*> keys,
                           double& out)
{
    for (auto* key : keys)
    {
        const auto value = object.getProperty (key);
        if (! value.isVoid() && numericVarToDouble (value, out))
            return true;
    }

    return false;
}

juce::String normalizedColor (const juce::String& value, int index)
{
    static const juce::StringArray palette {
        "#4E8CFF", "#27BFA5", "#F2A33A", "#D85C7A",
        "#8D7CFF", "#60B15A", "#CF6B45", "#38A1C5"
    };

    const auto trimmed = value.trim();
    if (trimmed.startsWithChar ('#') && (trimmed.length() == 7 || trimmed.length() == 9))
        return trimmed;

    return palette[index % palette.size()];
}

juce::String makeGroupID (int index, const juce::String& name, const juce::StringArray& trackIDs)
{
    const auto key = juce::String (index) + "|" + name.trim() + "|" + trackIDs.joinIntoString (",");
    return "grp_" + juce::String::toHexString (static_cast<juce::int64> (key.hashCode64()));
}

juce::StringArray splitTrackIDs (const juce::String& text)
{
    juce::StringArray ids;
    ids.addTokens (text, ",", "");
    ids.trim();
    ids.removeEmptyStrings();
    ids.removeDuplicates (false);
    return ids;
}

juce::var trackIDArrayVar (const juce::StringArray& ids)
{
    juce::Array<juce::var> out;
    for (const auto& id : ids)
        if (id.trim().isNotEmpty())
            out.add (id.trim());
    return juce::var (out);
}

juce::StringArray trackIDsFromArray (const juce::Array<juce::var>* values)
{
    juce::StringArray ids;
    if (values == nullptr)
        return ids;

    for (const auto& value : *values)
    {
        if (auto* object = value.getDynamicObject())
        {
            const auto id = firstStringProperty (*object, { "track_id", "id", "member_track_id" });
            if (id.isNotEmpty())
                ids.addIfNotAlreadyThere (id);
            continue;
        }

        const auto id = value.toString().trim();
        if (id.isNotEmpty())
            ids.addIfNotAlreadyThere (id);
    }

    return ids;
}

juce::StringArray trackIDsFromObject (const juce::DynamicObject& object)
{
    if (auto* values = object.getProperty ("track_ids").getArray())
        return trackIDsFromArray (values);
    if (auto* values = object.getProperty ("member_track_ids").getArray())
        return trackIDsFromArray (values);
    if (auto* values = object.getProperty ("members").getArray())
        return trackIDsFromArray (values);

    for (auto* key : { "track_ids", "member_track_ids", "members" })
    {
        const auto text = object.getProperty (key).toString().trim();
        if (text.isNotEmpty())
            return splitTrackIDs (text);
    }

    juce::StringArray ids;
    const auto id = firstStringProperty (object, { "track_id", "member_track_id" });
    if (id.isNotEmpty())
        ids.add (id);
    return ids;
}

te::Track* findTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr && track->itemID.toString() == trackID)
            return track;

    return nullptr;
}

bool isControllableTrack (te::Track& track)
{
    return dynamic_cast<te::AudioTrack*> (&track) != nullptr
           || dynamic_cast<te::FolderTrack*> (&track) != nullptr;
}

te::VolumeAndPanPlugin* volumePluginForTrack (te::Track& track)
{
    if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (&track))
    {
        if (audioTrack->getVolumePlugin() == nullptr)
        {
            auto& edit = audioTrack->pluginList.getEdit();
            auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {});
            if (plugin != nullptr)
                audioTrack->pluginList.insertPlugin (plugin, -1, nullptr);
        }
        return audioTrack->getVolumePlugin();
    }

    if (auto* folderTrack = dynamic_cast<te::FolderTrack*> (&track))
        return folderTrack->getVolumePlugin();

    return nullptr;
}

bool readVolumeDb (te::Track& track, double& out)
{
    if (auto* volume = volumePluginForTrack (track))
    {
        out = volume->getVolumeDb();
        return std::isfinite (out);
    }

    return false;
}

float convertRequestedDbToPluginDb (double requestedDb)
{
    const auto clamped = std::clamp (requestedDb, kMinimumVolumeDb, kMaximumVolumeDb);
    const auto gain = juce::Decibels::decibelsToGain (static_cast<float> (clamped), static_cast<float> (kMinimumVolumeDb));
    return juce::Decibels::gainToDecibels (gain, static_cast<float> (kMinimumVolumeDb));
}

bool setVolumeDb (te::Track& track, double requestedDb)
{
    if (auto* volume = volumePluginForTrack (track))
    {
        volume->setVolumeDb (convertRequestedDbToPluginDb (requestedDb));
        track.flushStateToValueTree();
        return true;
    }

    return false;
}

juce::ValueTree findGroupByID (juce::ValueTree groups, const juce::String& groupID)
{
    for (int i = 0; i < groups.getNumChildren(); ++i)
    {
        auto child = groups.getChild (i);
        if (child.hasType (kGroupTree) && child.getProperty (kGroupID).toString() == groupID)
            return child;
    }

    return {};
}

juce::var linkedControlsVar (const juce::ValueTree& group)
{
    auto out = std::make_unique<juce::DynamicObject>();
    out->setProperty ("volume", boolFromVar (group.getProperty (kLinkVolume), true));
    out->setProperty ("pan", boolFromVar (group.getProperty (kLinkPan), false));
    out->setProperty ("mute", boolFromVar (group.getProperty (kLinkMute), false));
    out->setProperty ("solo", boolFromVar (group.getProperty (kLinkSolo), false));
    return juce::var (out.release());
}

void applyLinkedControlsFromObject (juce::ValueTree& group,
                                    const juce::DynamicObject& object,
                                    juce::UndoManager* undo,
                                    bool isCreate)
{
    auto applyOne = [&] (const juce::Identifier& prop, const char* key, bool fallback)
    {
        const auto value = object.getProperty (key);
        if (! value.isVoid())
            group.setProperty (prop, boolFromVar (value, fallback), undo);
        else if (isCreate)
            group.setProperty (prop, fallback, undo);
    };

    if (auto* linked = object.getProperty ("linked_controls").getDynamicObject())
    {
        applyOne (kLinkVolume, "volume", true);
        applyOne (kLinkPan, "pan", false);
        applyOne (kLinkMute, "mute", false);
        applyOne (kLinkSolo, "solo", false);
        const auto volume = linked->getProperty ("volume");
        const auto pan = linked->getProperty ("pan");
        const auto mute = linked->getProperty ("mute");
        const auto solo = linked->getProperty ("solo");
        if (! volume.isVoid()) group.setProperty (kLinkVolume, boolFromVar (volume, true), undo);
        if (! pan.isVoid()) group.setProperty (kLinkPan, boolFromVar (pan, false), undo);
        if (! mute.isVoid()) group.setProperty (kLinkMute, boolFromVar (mute, false), undo);
        if (! solo.isVoid()) group.setProperty (kLinkSolo, boolFromVar (solo, false), undo);
        return;
    }

    applyOne (kLinkVolume, "link_volume", true);
    applyOne (kLinkPan, "link_pan", false);
    applyOne (kLinkMute, "link_mute", false);
    applyOne (kLinkSolo, "link_solo", false);
}

juce::var groupValueTreeToVar (te::Edit& edit, const juce::ValueTree& group)
{
    auto out = std::make_unique<juce::DynamicObject>();
    const auto groupID = group.getProperty (kGroupID).toString().trim();
    const auto name = group.getProperty (kName).toString().trim();
    const auto trackIDs = splitTrackIDs (group.getProperty (kMemberTrackIDs).toString());
    juce::Array<juce::var> missing;
    juce::Array<juce::var> memberNames;

    for (const auto& trackID : trackIDs)
    {
        if (auto* track = findTrackByID (edit, trackID))
            memberNames.add (track->getName());
        else
            missing.add (trackID);
    }

    out->setProperty ("group_id", groupID);
    out->setProperty ("id", groupID);
    out->setProperty ("name", name.isNotEmpty() ? name : groupID);
    out->setProperty ("color", group.getProperty (kColor).toString());
    out->setProperty ("type", group.getProperty (kType).toString().isNotEmpty() ? group.getProperty (kType) : juce::var ("control"));
    out->setProperty ("origin", group.getProperty (kOrigin).toString().isNotEmpty() ? group.getProperty (kOrigin) : juce::var ("user"));
    out->setProperty ("enabled", boolFromVar (group.getProperty (kEnabled), true));
    out->setProperty ("suspended", boolFromVar (group.getProperty (kSuspended), false));
    out->setProperty ("track_ids", trackIDArrayVar (trackIDs));
    out->setProperty ("member_track_ids", trackIDArrayVar (trackIDs));
    out->setProperty ("member_track_names", juce::var (memberNames));
    out->setProperty ("member_count", trackIDs.size());
    out->setProperty ("missing_member_ids", juce::var (missing));
    out->setProperty ("linked_controls", linkedControlsVar (group));
    out->setProperty ("link_volume", boolFromVar (group.getProperty (kLinkVolume), true));
    out->setProperty ("link_pan", boolFromVar (group.getProperty (kLinkPan), false));
    out->setProperty ("link_mute", boolFromVar (group.getProperty (kLinkMute), false));
    out->setProperty ("link_solo", boolFromVar (group.getProperty (kLinkSolo), false));
    if (group.hasProperty (kCreatedAt))
        out->setProperty ("created_at", group.getProperty (kCreatedAt));
    if (group.hasProperty (kUpdatedAt))
        out->setProperty ("updated_at", group.getProperty (kUpdatedAt));
    return juce::var (out.release());
}

juce::Array<juce::var> snapshotArrayFromTree (te::Edit& edit, const juce::ValueTree& tree)
{
    std::vector<juce::ValueTree> source;
    for (int i = 0; i < tree.getNumChildren(); ++i)
    {
        const auto child = tree.getChild (i);
        if (child.hasType (kGroupTree))
            source.push_back (child);
    }

    std::sort (source.begin(), source.end(), [] (const juce::ValueTree& a, const juce::ValueTree& b)
    {
        return a.getProperty (kName).toString().compareIgnoreCase (b.getProperty (kName).toString()) < 0;
    });

    juce::Array<juce::var> groups;
    for (const auto& group : source)
        groups.add (groupValueTreeToVar (edit, group));
    return groups;
}

juce::var groupListSnapshotVar (te::Edit& edit)
{
    const auto tree = groupsTree (edit);
    if (! tree.isValid())
        return juce::var (juce::Array<juce::var>());
    return juce::var (snapshotArrayFromTree (edit, tree));
}

void writeMembers (juce::ValueTree& group, const juce::StringArray& trackIDs, juce::UndoManager* undo)
{
    group.setProperty (kMemberTrackIDs, trackIDs.joinIntoString (","), undo);
}

juce::String validateTrackIDs (te::Edit& edit, const juce::StringArray& trackIDs)
{
    if (trackIDs.isEmpty())
        return "track group requires at least one member track_id";

    for (const auto& trackID : trackIDs)
    {
        auto* track = findTrackByID (edit, trackID);
        if (track == nullptr)
            return "Track not found for track group member: " + trackID;
        if (dynamic_cast<te::MasterTrack*> (track) != nullptr)
            return "Master track cannot be a control group member: " + trackID;
    }

    return {};
}

} // namespace

TrackGroupService::TrackGroupService (EditGetter editGetter,
                                      SaveProjectAction saveProjectAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction))
{
}

juce::String TrackGroupService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

juce::var TrackGroupService::createGroupsSnapshot (te::Edit& edit)
{
    return groupListSnapshotVar (edit);
}

juce::String TrackGroupService::handleListGroups (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto response = std::make_unique<juce::DynamicObject>();
    const auto groups = createGroupsSnapshot (*edit);
    const auto* groupArray = groups.getArray();
    response->setProperty ("status", "ok");
    response->setProperty ("track_groups", groups);
    response->setProperty ("groups", groups);
    response->setProperty ("group_count", groupArray != nullptr ? groupArray->size() : 0);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::handleCreateGroup (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto trackIDs = trackIDsFromObject (object);
    trackIDs.removeDuplicates (false);
    if (auto error = validateTrackIDs (*edit, trackIDs); error.isNotEmpty())
        return makeErrorReply (error);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Create track group");
    auto groups = ensureGroupsTree (*edit, &undo);
    auto name = firstStringProperty (object, { "name", "group_name", "label" });
    if (name.isEmpty())
        name = "Group " + juce::String (groups.getNumChildren() + 1);
    auto groupID = firstStringProperty (object, { "group_id", "id" });
    if (groupID.isEmpty())
        groupID = makeGroupID (groups.getNumChildren(), name, trackIDs);
    if (findGroupByID (groups, groupID).isValid())
        return makeErrorReply ("Track group already exists: " + groupID);

    auto group = juce::ValueTree (kGroupTree);
    const auto stamp = nowStamp();
    group.setProperty (kGroupID, groupID, nullptr);
    group.setProperty (kName, name, &undo);
    group.setProperty (kColor, normalizedColor (firstStringProperty (object, { "color", "colour" }), groups.getNumChildren()), &undo);
    group.setProperty (kType, firstStringProperty (object, { "type", "group_type" }).isNotEmpty() ? firstStringProperty (object, { "type", "group_type" }) : "control", &undo);
    group.setProperty (kOrigin, firstStringProperty (object, { "origin", "created_by" }).isNotEmpty() ? firstStringProperty (object, { "origin", "created_by" }) : "user", &undo);
    group.setProperty (kEnabled, boolPropertyOrDefault (object, { "enabled" }, true), &undo);
    group.setProperty (kSuspended, boolPropertyOrDefault (object, { "suspended" }, false), &undo);
    group.setProperty (kCreatedAt, stamp, &undo);
    group.setProperty (kUpdatedAt, stamp, &undo);
    applyLinkedControlsFromObject (group, object, &undo, true);
    writeMembers (group, trackIDs, &undo);
    groups.addChild (group, -1, &undo);

    edit->dispatchPendingUpdatesSynchronously();
    if (saveProject && ! saveProject())
        return makeErrorReply ("Track group created in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track group created");
    response->setProperty ("created", true);
    response->setProperty ("group_id", groupID);
    response->setProperty ("track_group", groupValueTreeToVar (*edit, group));
    response->setProperty ("track_groups", createGroupsSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::handleUpdateGroup (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto groupID = firstStringProperty (object, { "group_id", "id" });
    if (groupID.isEmpty())
        return makeErrorReply ("track.group.update requires group_id");

    auto groups = groupsTree (*edit);
    auto group = groups.isValid() ? findGroupByID (groups, groupID) : juce::ValueTree();
    if (! group.isValid())
        return makeErrorReply ("Track group not found: " + groupID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Update track group");
    const auto name = firstStringProperty (object, { "name", "group_name", "label" });
    if (name.isNotEmpty())
        group.setProperty (kName, name, &undo);
    const auto color = firstStringProperty (object, { "color", "colour" });
    if (color.isNotEmpty())
        group.setProperty (kColor, normalizedColor (color, 0), &undo);
    const auto type = firstStringProperty (object, { "type", "group_type" });
    if (type.isNotEmpty())
        group.setProperty (kType, type, &undo);
    const auto origin = firstStringProperty (object, { "origin" });
    if (origin.isNotEmpty())
        group.setProperty (kOrigin, origin, &undo);
    if (! object.getProperty ("enabled").isVoid())
        group.setProperty (kEnabled, boolPropertyOrDefault (object, { "enabled" }, true), &undo);
    if (! object.getProperty ("suspended").isVoid())
        group.setProperty (kSuspended, boolPropertyOrDefault (object, { "suspended" }, false), &undo);
    applyLinkedControlsFromObject (group, object, &undo, false);
    const auto trackIDs = trackIDsFromObject (object);
    if (! trackIDs.isEmpty())
    {
        if (auto error = validateTrackIDs (*edit, trackIDs); error.isNotEmpty())
            return makeErrorReply (error);
        writeMembers (group, trackIDs, &undo);
    }
    group.setProperty (kUpdatedAt, nowStamp(), &undo);

    edit->dispatchPendingUpdatesSynchronously();
    if (saveProject && ! saveProject())
        return makeErrorReply ("Track group updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track group updated");
    response->setProperty ("group_id", groupID);
    response->setProperty ("track_group", groupValueTreeToVar (*edit, group));
    response->setProperty ("track_groups", createGroupsSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::handleSetGroupMembers (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto groupID = firstStringProperty (object, { "group_id", "id" });
    if (groupID.isEmpty())
        return makeErrorReply ("track.group.set_members requires group_id");

    auto trackIDs = trackIDsFromObject (object);
    trackIDs.removeDuplicates (false);
    if (auto error = validateTrackIDs (*edit, trackIDs); error.isNotEmpty())
        return makeErrorReply (error);

    auto groups = groupsTree (*edit);
    auto group = groups.isValid() ? findGroupByID (groups, groupID) : juce::ValueTree();
    if (! group.isValid())
        return makeErrorReply ("Track group not found: " + groupID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set track group members");
    writeMembers (group, trackIDs, &undo);
    group.setProperty (kUpdatedAt, nowStamp(), &undo);
    edit->dispatchPendingUpdatesSynchronously();
    if (saveProject && ! saveProject())
        return makeErrorReply ("Track group members updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track group members updated");
    response->setProperty ("group_id", groupID);
    response->setProperty ("track_group", groupValueTreeToVar (*edit, group));
    response->setProperty ("track_groups", createGroupsSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::handleDeleteGroup (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto groupID = firstStringProperty (object, { "group_id", "id" });
    if (groupID.isEmpty())
        return makeErrorReply ("track.group.delete requires group_id");

    auto groups = groupsTree (*edit);
    if (! groups.isValid())
        return makeErrorReply ("Track group not found: " + groupID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete track group");
    bool removed = false;
    for (int i = groups.getNumChildren() - 1; i >= 0; --i)
    {
        auto child = groups.getChild (i);
        if (child.hasType (kGroupTree) && child.getProperty (kGroupID).toString() == groupID)
        {
            groups.removeChild (i, &undo);
            removed = true;
            break;
        }
    }
    if (! removed)
        return makeErrorReply ("Track group not found: " + groupID);

    edit->dispatchPendingUpdatesSynchronously();
    if (saveProject && ! saveProject())
        return makeErrorReply ("Track group deleted in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Track group deleted");
    response->setProperty ("deleted_count", 1);
    response->setProperty ("group_id", groupID);
    response->setProperty ("track_groups", createGroupsSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String TrackGroupService::handleApplyGroupControl (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto groupID = firstStringProperty (object, { "group_id", "id" });
    auto requestedTrackIDs = trackIDsFromObject (object);
    requestedTrackIDs.removeDuplicates (false);

    auto control = firstStringProperty (object, { "control", "param", "parameter" }).toLowerCase();
    if (control.isEmpty())
        control = "volume";
    if (control != "volume" && control != "track.volume")
        return makeErrorReply ("track.group.apply_control v1 supports volume only");

    auto groups = groupsTree (*edit);
    auto group = groups.isValid() ? findGroupByID (groups, groupID) : juce::ValueTree();
    const bool createGroupIfMissing = boolPropertyOrDefault (object, { "create_group_if_missing", "ensure_group", "create_group" }, false);
    const bool replaceMembers = boolPropertyOrDefault (object, { "replace_members", "set_members", "sync_members" }, false);
    if (! group.isValid())
    {
        if (groupID.isEmpty() && requestedTrackIDs.isEmpty())
            return makeErrorReply ("track.group.apply_control requires group_id or member_track_ids");
        if (! createGroupIfMissing && requestedTrackIDs.isEmpty())
            return makeErrorReply ("Track group not found: " + groupID);
        if (! createGroupIfMissing)
            return makeErrorReply ("Track group not found: " + groupID + " (set create_group_if_missing to create it from member_track_ids)");
        if (auto error = validateTrackIDs (*edit, requestedTrackIDs); error.isNotEmpty())
            return makeErrorReply (error);
    }

    const bool force = boolPropertyOrDefault (object, { "force" }, false);
    if (! force && ! boolFromVar (group.getProperty (kEnabled), true))
        return makeErrorReply ("Track group is disabled: " + groupID);
    if (! force && boolFromVar (group.getProperty (kSuspended), false))
        return makeErrorReply ("Track group is suspended: " + groupID);
    if (! force && ! boolFromVar (group.getProperty (kLinkVolume), true))
        return makeErrorReply ("Track group volume link is disabled: " + groupID);
    if (group.isValid() && replaceMembers && ! requestedTrackIDs.isEmpty())
        if (auto error = validateTrackIDs (*edit, requestedTrackIDs); error.isNotEmpty())
            return makeErrorReply (error);

    auto mode = firstStringProperty (object, { "mode", "operation" }).toLowerCase();
    if (mode.isEmpty())
        mode = object.getProperty ("delta_db").isVoid() ? "absolute" : "relative";

    const bool relative = mode == "relative" || mode == "delta" || mode == "volume_relative";
    const bool absolute = mode == "absolute" || mode == "set" || mode == "volume_absolute";
    if (! relative && ! absolute)
        return makeErrorReply ("track.group.apply_control mode must be absolute or relative");

    double requested = 0.0;
    if (relative)
    {
        if (! firstNumericProperty (object, { "delta_db", "db_delta", "amount_db" }, requested))
            return makeErrorReply ("relative group volume control requires delta_db");
    }
    else if (! firstNumericProperty (object, { "db", "target_db", "value_db", "volume_db" }, requested))
    {
        return makeErrorReply ("absolute group volume control requires db");
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Apply track group volume");
    bool groupCreated = false;
    bool membersReplaced = false;
    if (! group.isValid())
    {
        groups = ensureGroupsTree (*edit, &undo);
        auto name = firstStringProperty (object, { "name", "group_name", "label" });
        if (name.isEmpty())
            name = "B1 Fader Reset";
        if (groupID.isEmpty())
            groupID = makeGroupID (groups.getNumChildren(), name, requestedTrackIDs);
        auto existing = findGroupByID (groups, groupID);
        if (existing.isValid())
        {
            group = existing;
            if (! requestedTrackIDs.isEmpty())
            {
                writeMembers (group, requestedTrackIDs, &undo);
                membersReplaced = true;
            }
        }
        else
        {
            group = juce::ValueTree (kGroupTree);
            const auto stamp = nowStamp();
            group.setProperty (kGroupID, groupID, nullptr);
            group.setProperty (kName, name, &undo);
            group.setProperty (kColor, normalizedColor (firstStringProperty (object, { "color", "colour" }), groups.getNumChildren()), &undo);
            group.setProperty (kType, firstStringProperty (object, { "type", "group_type" }).isNotEmpty() ? firstStringProperty (object, { "type", "group_type" }) : "control", &undo);
            group.setProperty (kOrigin, firstStringProperty (object, { "origin", "created_by" }).isNotEmpty() ? firstStringProperty (object, { "origin", "created_by" }) : "b1_gain_staging", &undo);
            group.setProperty (kEnabled, boolPropertyOrDefault (object, { "enabled" }, true), &undo);
            group.setProperty (kSuspended, boolPropertyOrDefault (object, { "suspended" }, false), &undo);
            group.setProperty (kCreatedAt, stamp, &undo);
            group.setProperty (kUpdatedAt, stamp, &undo);
            applyLinkedControlsFromObject (group, object, &undo, true);
            writeMembers (group, requestedTrackIDs, &undo);
            groups.addChild (group, -1, &undo);
            groupCreated = true;
        }
    }
    else if (replaceMembers && ! requestedTrackIDs.isEmpty())
    {
        writeMembers (group, requestedTrackIDs, &undo);
        group.setProperty (kUpdatedAt, nowStamp(), &undo);
        membersReplaced = true;
    }

    const auto trackIDs = splitTrackIDs (group.getProperty (kMemberTrackIDs).toString());
    if (trackIDs.isEmpty())
        return makeErrorReply ("Track group has no members: " + groupID);

    juce::Array<juce::var> memberResults;
    int appliedCount = 0;
    int failedCount = 0;
    int verifiedCount = 0;

    for (const auto& trackID : trackIDs)
    {
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("track_id", trackID);
        auto* track = findTrackByID (*edit, trackID);
        if (track == nullptr)
        {
            row->setProperty ("status", "error");
            row->setProperty ("error", "track not found");
            ++failedCount;
            memberResults.add (juce::var (row.release()));
            continue;
        }

        row->setProperty ("track_name", track->getName());
        if (! isControllableTrack (*track))
        {
            row->setProperty ("status", "error");
            row->setProperty ("error", "track type has no group volume control");
            ++failedCount;
            memberResults.add (juce::var (row.release()));
            continue;
        }

        double before = 0.0;
        if (! readVolumeDb (*track, before))
        {
            row->setProperty ("status", "error");
            row->setProperty ("error", "volume plugin unavailable");
            ++failedCount;
            memberResults.add (juce::var (row.release()));
            continue;
        }

        const double target = std::clamp (relative ? before + requested : requested, kMinimumVolumeDb, kMaximumVolumeDb);
        if (! setVolumeDb (*track, target))
        {
            row->setProperty ("status", "error");
            row->setProperty ("error", "failed to set volume");
            row->setProperty ("before_db", before);
            row->setProperty ("target_db", target);
            ++failedCount;
            memberResults.add (juce::var (row.release()));
            continue;
        }

        double after = target;
        readVolumeDb (*track, after);
        const bool verified = std::abs (after - target) <= 0.01;
        row->setProperty ("status", "ok");
        row->setProperty ("before_db", before);
        row->setProperty ("target_db", target);
        row->setProperty ("after_db", after);
        row->setProperty ("verified", verified);
        if (relative)
            row->setProperty ("delta_db", target - before);
        ++appliedCount;
        if (verified)
            ++verifiedCount;
        memberResults.add (juce::var (row.release()));
    }

    group.setProperty (kUpdatedAt, nowStamp(), &undo);
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (appliedCount > 0 && saveProject && ! saveProject())
        return makeErrorReply ("Track group control applied in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", failedCount == 0 ? "ok" : (appliedCount > 0 ? "partial" : "error"));
    response->setProperty ("message", "Track group volume control applied");
    response->setProperty ("group_id", groupID);
    response->setProperty ("control", "volume");
    response->setProperty ("mode", relative ? "relative" : "absolute");
    if (relative)
        response->setProperty ("delta_db", requested);
    else
        response->setProperty ("db", requested);
    response->setProperty ("member_count", trackIDs.size());
    response->setProperty ("applied_count", appliedCount);
    response->setProperty ("failed_count", failedCount);
    response->setProperty ("verified_count", verifiedCount);
    response->setProperty ("group_created", groupCreated);
    response->setProperty ("members_replaced", membersReplaced);
    response->setProperty ("members", juce::var (memberResults));
    response->setProperty ("track_group", groupValueTreeToVar (*edit, group));
    response->setProperty ("track_groups", createGroupsSnapshot (*edit));
    response->setProperty ("requires_refresh", true);
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit
