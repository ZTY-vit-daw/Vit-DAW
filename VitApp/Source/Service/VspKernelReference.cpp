#include "VspKernelReference.h"

#include <cstdint>
#include <functional>
#include <unordered_map>
#include <unordered_set>

namespace vit
{

namespace
{

struct LegacyCommandMapping
{
    const char* legacyCommand = "";
    bool writesProjectOrTransport = false;
};

struct ProjectStateSnapshot
{
    bool valid = false;
    int64_t revision = 0;
    juce::String hash;
    juce::String projectEpoch;
    juce::String scope;
    juce::var snapshot;
    juce::String errorCode;
    juce::String errorMessage;
};

struct StateSessionCursor
{
    bool valid = false;
    int64_t revision = 0;
    juce::String hash;
    juce::var snapshot;
};

struct StateStore
{
    int64_t revision = 0;
    juce::String hash;
    juce::var snapshot;
    std::unordered_map<std::string, StateSessionCursor> cursors;
    std::unordered_map<std::string, juce::String> commandReceipts;
};

struct RealtimeSubscription
{
    juce::String subscriptionId;
    juce::String sessionId;
    juce::Array<juce::var> streams;
};

struct EventSubscription
{
    juce::String subscriptionId;
    juce::String sessionId;
    juce::StringArray topics;
    int64_t sequence = 0;
};

struct AssetPrepareResult
{
    juce::String requestId;
    juce::String status = "skipped";
    juce::String message;
    bool ok = false;
};

struct Phase4Store
{
    std::unordered_map<std::string, RealtimeSubscription> realtimeSubscriptions;
    std::unordered_map<std::string, EventSubscription> eventSubscriptions;
    int64_t realtimeFrameIndex = 0;
    int64_t eventSequence = 0;
};

const std::unordered_map<std::string, LegacyCommandMapping>& canonicalCommandMap()
{
    static const std::unordered_map<std::string, LegacyCommandMapping> mappings {
        { "kernel.ping", { "ping", false } },
        { "transport.play", { "play", true } },
        { "transport.stop", { "stop", true } },
        { "project.snapshot.get", { "get_project_state", false } },
        { "project.tracks.list", { "list_tracks", false } },
        { "project.markers.list", { "project.markers.list", false } },
        { "project.markers.upsert", { "project.markers.upsert", true } },
        { "project.markers.rename", { "project.markers.rename", true } },
        { "project.markers.delete", { "project.markers.delete", true } },
        { "project.markers.apply_section_markers", { "project.markers.apply_section_markers", true } },
        { "track.group.list", { "track.group.list", false } },
        { "track.group.create", { "track.group.create", true } },
        { "track.group.update", { "track.group.update", true } },
        { "track.group.set_members", { "track.group.set_members", true } },
        { "track.group.delete", { "track.group.delete", true } },
        { "track.group.apply_control", { "track.group.apply_control", true } },
        { "project.import_audio_files", { "project.import_audio_files", true } },
        { "track.create", { "add_track", true } },
        { "track.delete", { "delete_track", true } },
        { "folder_track.create", { "folder_track.create", true } },
        { "track.folder.create", { "folder_track.create", true } },
        { "track.move_to_folder", { "track.move_to_folder", true } },
        { "folder_track.set_routing_bus_enabled", { "folder_track.set_routing_bus_enabled", true } },
        { "track.folder.set_routing_bus_enabled", { "folder_track.set_routing_bus_enabled", true } },
        { "project.apply_track_organization", { "project.apply_track_organization", true } },
        { "project.track_organization.apply", { "project.apply_track_organization", true } },
        { "clip.move", { "move_clip", true } },
        { "clip.resize", { "resize_clip", true } },
        { "clip.split", { "split_clip", true } },
        { "clip.remove", { "remove_clips", true } },
        { "clip.fade.set", { "clip.fade.set", true } },
        { "clip.fade.read", { "clip.fade.read", false } },
        { "clip.gain.set", { "clip.gain.set", true } },
        { "clip.gain.read", { "clip.gain.read", false } },
        { "project.undo", { "undo", true } },
        { "project.redo", { "redo", true } },
        { "plugin.parameters.get", { "get_plugin_parameters", false } },
        { "plugin.describe_parameters", { "get_plugin_parameters", false } },
        { "plugin.readback", { "get_plugin_parameters", false } },
        { "macro.create", { "control_add_macro", true } },
        { "macro.bind", { "control_add_binding", true } },
        { "macro.set_values", { "control_set_macro_values", true } },
        { "render.start", { "start_render", true } },
        { "render.cancel", { "cancel_render", false } },
        { "audition.prepare", { "audition.prepare", false } },
        { "audition.status", { "audition.status", false } },
        { "audition.ready", { "audition.ready", false } },
        { "audition.stale", { "audition.stale", false } },
        { "audition.failed", { "audition.failed", false } },
        { "audition.inspect_candidate", { "audition.inspect_candidate", false } },
        { "audition.apply_candidate", { "audition.apply_candidate", false } },
        { "audition.select", { "audition.select", false } },
        { "audition.position", { "audition.position", false } },
        { "audition.stop", { "audition.stop", false } },
    };

    return mappings;
}

const std::unordered_set<std::string>& legacyReadOnlyCommands()
{
    static const std::unordered_set<std::string> commands {
        "ping",
        "get_project_state",
        "list_tracks",
        "get_recent_projects",
        "get_midi_clip_notes",
        "get_midi_clip_data",
        "clip.fade.read",
        "clip_fade_read",
        "read_clip_fade",
        "clip.gain.read",
        "clip_gain_read",
        "read_clip_gain",
        "get_plugin_parameters",
        "l2_render_probe",
        "compressor_dual_tap_probe",
        "project_health_check",
        "project.audio_analysis_status",
        "track.group.list",
        "track_group_list",
        "get_audio_device_types",
        "get_audio_devices",
        "get_wave_input_devices",
        "audition.prepare",
        "audition.status",
        "audition.ready",
        "audition.stale",
        "audition.failed",
        "audition.select",
        "audition.position",
        "audition.stop",
        "audition.inspect_candidate",
        "audition.apply_candidate",
    };

    return commands;
}

juce::CriticalSection& stateStoreLock()
{
    static juce::CriticalSection lock;
    return lock;
}

StateStore& stateStore()
{
    static StateStore store;
    return store;
}

juce::CriticalSection& phase4StoreLock()
{
    static juce::CriticalSection lock;
    return lock;
}

Phase4Store& phase4Store()
{
    static Phase4Store store;
    return store;
}

juce::String trimStringProperty (const juce::DynamicObject& object, const juce::Identifier& name)
{
    return object.getProperty (name).toString().trim();
}

juce::String firstStringProperty (const juce::DynamicObject& object, const juce::StringArray& names)
{
    for (const auto& name : names)
    {
        const auto value = object.getProperty (juce::Identifier (name)).toString().trim();
        if (value.isNotEmpty())
            return value;
    }

    return {};
}

int64_t int64FromVar (const juce::var& value)
{
    if (value.isVoid())
        return 0;

    if (value.isString())
        return value.toString().trim().getLargeIntValue();

    return static_cast<int64_t> (value);
}

int64_t int64Property (const juce::DynamicObject& object, const juce::Identifier& name)
{
    return int64FromVar (object.getProperty (name));
}

juce::String stableHashString (const juce::String& text)
{
    const auto hashed = std::hash<std::string>{} (text.toStdString());
    return juce::String::toHexString (static_cast<juce::int64> (hashed));
}

juce::String stableHashVar (const juce::var& value)
{
    return stableHashString (juce::JSON::toString (value));
}

void copyPropertyIfPresent (const juce::DynamicObject& source,
                            juce::DynamicObject& target,
                            const juce::Identifier& sourceName,
                            const juce::Identifier& targetName)
{
    if (source.hasProperty (sourceName))
        target.setProperty (targetName, source.getProperty (sourceName));
}

void copyPropertyIfPresent (const juce::DynamicObject& source,
                            juce::DynamicObject& target,
                            const juce::Identifier& name)
{
    copyPropertyIfPresent (source, target, name, name);
}

void addUnique (juce::StringArray& values, const juce::String& value)
{
    if (value.isNotEmpty() && ! values.contains (value))
        values.add (value);
}

juce::String generatedId (const char* prefix)
{
    return juce::String (prefix)
        + juce::String (juce::Time::currentTimeMillis())
        + "_"
        + juce::String (juce::Random::getSystemRandom().nextInt (0x3fffffff));
}

juce::String generatedId (const juce::String& prefix)
{
    return generatedId (prefix.toRawUTF8());
}

juce::String nowIso8601()
{
    return juce::Time::getCurrentTime().toISO8601 (true);
}

void copyProperties (const juce::DynamicObject& source, juce::DynamicObject& target)
{
    const auto& properties = source.getProperties();

    for (int i = 0; i < properties.size(); ++i)
        target.setProperty (properties.getName (i), properties.getValueAt (i));
}

juce::StringArray capabilityList()
{
    return {
        "project.read",
        "project.write",
        "transport.control",
        "audition.preview",
        "track.edit",
        "clip.edit",
        "plugin.read",
        "plugin.control",
        "asset.read",
        "asset.reference",
        "asset.manifest",
        "realtime.subscribe",
        "state.subscribe",
        "event.subscribe",
        "render.offline",
        "audio.l2_render_probe",
        "audio.compressor_dual_tap_probe",
        "filesystem.import",
        "command.batch",
        "legacy.command",
    };
}

juce::Array<juce::var> toVarArray (const juce::StringArray& values)
{
    juce::Array<juce::var> result;

    for (const auto& value : values)
        result.add (value);

    return result;
}

juce::String trimStringVar (const juce::var& value)
{
    return value.toString().trim();
}

juce::StringArray stringArrayFromVar (const juce::var& value)
{
    juce::StringArray result;

    if (auto* values = value.getArray())
    {
        for (const auto& item : *values)
            addUnique (result, trimStringVar (item));
    }
    else
    {
        addUnique (result, trimStringVar (value));
    }

    return result;
}

int intFromVar (const juce::var& value, int fallback)
{
    if (value.isVoid())
        return fallback;

    if (value.isString())
    {
        const auto text = value.toString().trim();
        return text.isNotEmpty() ? text.getIntValue() : fallback;
    }

    return static_cast<int> (value);
}

double doubleFromVar (const juce::var& value, double fallback)
{
    if (value.isVoid())
        return fallback;

    if (value.isString())
    {
        const auto text = value.toString().trim();
        return text.isNotEmpty() ? text.getDoubleValue() : fallback;
    }

    return static_cast<double> (value);
}

bool boolFromVar (const juce::var& value, bool fallback)
{
    if (value.isVoid())
        return fallback;

    if (value.isBool())
        return static_cast<bool> (value);

    const auto text = value.toString().trim().toLowerCase();
    if (text == "true" || text == "1" || text == "yes")
        return true;
    if (text == "false" || text == "0" || text == "no")
        return false;

    return fallback;
}

juce::var makeStringArrayVar (const juce::StringArray& values)
{
    return juce::var (toVarArray (values));
}

juce::var buildTrackCore (const juce::var& trackVar)
{
    auto* track = trackVar.getDynamicObject();
    auto core = std::make_unique<juce::DynamicObject>();

    if (track == nullptr)
        return juce::var (core.release());

    const auto& properties = track->getProperties();
    for (int i = 0; i < properties.size(); ++i)
    {
        const auto name = properties.getName (i);
        if (name.toString() == "clips")
            continue;

        core->setProperty (name, properties.getValueAt (i));
    }

    return juce::var (core.release());
}

bool sameJsonValue (const juce::var& a, const juce::var& b)
{
    return juce::JSON::toString (a) == juce::JSON::toString (b);
}

bool isKernelInternalTimelineTrackId (const juce::String& trackId)
{
    const auto numericId = trackId.getLargeIntValue();
    return numericId >= 1002 && numericId <= 1006;
}

bool shouldExposeUserTimelineTracksOnly (const juce::String& requestedScope)
{
    const auto scope = requestedScope.trim().toLowerCase();
    return scope.isEmpty() || scope == "project.timeline";
}

juce::Array<juce::var> normaliseClipArray (const juce::var& clipsVar, const juce::String& trackId)
{
    juce::Array<juce::var> clips;
    auto* sourceClips = clipsVar.getArray();

    if (sourceClips == nullptr)
        return clips;

    for (const auto& clipVar : *sourceClips)
    {
        auto* clip = clipVar.getDynamicObject();
        if (clip == nullptr)
            continue;

        const auto clipId = firstStringProperty (*clip, { "clip_id", "id" });
        if (clipId.isEmpty())
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("clip_id", clipId);
        row->setProperty ("id", clipId);
        row->setProperty ("track_id", trackId);
        row->setProperty ("name", firstStringProperty (*clip, { "name", "clip_name" }));
        copyPropertyIfPresent (*clip, *row, "clip_type");
        copyPropertyIfPresent (*clip, *row, "start_seconds");
        copyPropertyIfPresent (*clip, *row, "end_seconds");
        copyPropertyIfPresent (*clip, *row, "length_seconds");
        copyPropertyIfPresent (*clip, *row, "offset_in_source_seconds");
        copyPropertyIfPresent (*clip, *row, "clip_gain_db");
        copyPropertyIfPresent (*clip, *row, "clip_pan");
        copyPropertyIfPresent (*clip, *row, "clip_mute");
        copyPropertyIfPresent (*clip, *row, "gain_db");
        copyPropertyIfPresent (*clip, *row, "pan");
        copyPropertyIfPresent (*clip, *row, "mute");
        copyPropertyIfPresent (*clip, *row, "fade_in_seconds");
        copyPropertyIfPresent (*clip, *row, "fade_out_seconds");
        copyPropertyIfPresent (*clip, *row, "fade_in_curve");
        copyPropertyIfPresent (*clip, *row, "fade_out_curve");
        copyPropertyIfPresent (*clip, *row, "fade_in_curve_type");
        copyPropertyIfPresent (*clip, *row, "fade_out_curve_type");
        copyPropertyIfPresent (*clip, *row, "fade_in_behaviour");
        copyPropertyIfPresent (*clip, *row, "fade_out_behaviour");
        copyPropertyIfPresent (*clip, *row, "fade_in_behaviour_type");
        copyPropertyIfPresent (*clip, *row, "fade_out_behaviour_type");
        copyPropertyIfPresent (*clip, *row, "auto_crossfade");
        copyPropertyIfPresent (*clip, *row, "asset_ref");
        copyPropertyIfPresent (*clip, *row, "asset_state");
        copyPropertyIfPresent (*clip, *row, "asset_job_id");
        copyPropertyIfPresent (*clip, *row, "warp_state");
        copyPropertyIfPresent (*clip, *row, "take_stack_id");
        copyPropertyIfPresent (*clip, *row, "active_take_id");
        copyPropertyIfPresent (*clip, *row, "ghost_state");
        copyPropertyIfPresent (*clip, *row, "current_source_path");
        copyPropertyIfPresent (*clip, *row, "clip_state_revision");
        copyPropertyIfPresent (*clip, *row, "playback_source_valid");

        clips.add (juce::var (row.release()));
    }

    return clips;
}

juce::Array<juce::var> normaliseRackNodeArray (const juce::var& nodesVar)
{
    juce::Array<juce::var> nodes;
    auto* sourceNodes = nodesVar.getArray();

    if (sourceNodes == nullptr)
        return nodes;

    for (const auto& nodeVar : *sourceNodes)
    {
        auto* node = nodeVar.getDynamicObject();
        if (node == nullptr)
            continue;

        const auto nodeId = firstStringProperty (*node,
                                                 { "plugin_item_id", "node_id", "item_id", "plugin_id", "id" });
        if (nodeId.isEmpty())
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("node_id", nodeId);
        row->setProperty ("plugin_item_id", nodeId);
        row->setProperty ("item_id", nodeId);
        row->setProperty ("id", nodeId);
        row->setProperty ("plugin_id", nodeId);

        const auto pluginName = firstStringProperty (*node, { "plugin_name", "name", "display_name" });
        if (pluginName.isNotEmpty())
        {
            row->setProperty ("name", pluginName);
            row->setProperty ("plugin_name", pluginName);
        }

        for (const auto* property : {
                 "type", "enabled", "x", "y", "zone_id", "clip_scope", "template_role",
                 "audio_reachable_from_rack_input", "vit_effective_in_output_path",
                 "vit_orphan_bypass_candidate", "supports_param_grabber", "manufacturer",
                 "vendor", "category", "is_instrument", "isInstrument", "plugin_identifier", "identifier"
             })
            copyPropertyIfPresent (*node, *row, juce::Identifier (property));

        const auto pluginPath = firstStringProperty (*node, { "plugin_path", "path", "file_path", "filename" });
        if (pluginPath.isNotEmpty())
        {
            row->setProperty ("plugin_path", pluginPath);
            row->setProperty ("path", pluginPath);
        }

        const auto pluginFormat = firstStringProperty (*node, { "plugin_format", "format" });
        if (pluginFormat.isNotEmpty())
        {
            row->setProperty ("plugin_format", pluginFormat);
            row->setProperty ("format", pluginFormat);
        }

        nodes.add (juce::var (row.release()));
    }

    return nodes;
}

juce::var normaliseRackState (const juce::var& rackVar)
{
    auto* rack = rackVar.getDynamicObject();
    if (rack == nullptr)
        return {};

    auto row = std::make_unique<juce::DynamicObject>();
    const auto rackItemId = firstStringProperty (*rack, { "rack_item_id", "item_id", "id" });
    if (rackItemId.isNotEmpty())
    {
        row->setProperty ("rack_item_id", rackItemId);
        row->setProperty ("item_id", rackItemId);
        row->setProperty ("id", rackItemId);
    }

    copyPropertyIfPresent (*rack, *row, "scope");
    const auto nodes = normaliseRackNodeArray (rack->getProperty ("nodes"));
    row->setProperty ("nodes", juce::var (nodes));
    row->setProperty ("node_count", nodes.size());

    // Edges and clip routing are already compact structural records generated
    // by createRackState. Preserve them so UI topology and agent verification
    // observe the same graph as the execution layer.
    for (const auto* property : { "edges", "clip_proxy_nodes", "clip_routes" })
        copyPropertyIfPresent (*rack, *row, juce::Identifier (property));

    if (! row->hasProperty ("edges"))
        row->setProperty ("edges", juce::var (juce::Array<juce::var>()));

    return juce::var (row.release());
}

juce::Array<juce::var> normaliseTrackArray (const juce::var& tracksVar, bool userTimelineOnly)
{
    juce::Array<juce::var> tracks;
    auto* sourceTracks = tracksVar.getArray();

    if (sourceTracks == nullptr)
        return tracks;

    for (const auto& trackVar : *sourceTracks)
    {
        auto* track = trackVar.getDynamicObject();
        if (track == nullptr)
            continue;

        const auto trackId = firstStringProperty (*track, { "track_id", "id" });
        if (trackId.isEmpty())
            continue;

        if (userTimelineOnly && isKernelInternalTimelineTrackId (trackId))
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("track_id", trackId);
        row->setProperty ("id", trackId);
        row->setProperty ("name", firstStringProperty (*track, { "track_name", "name" }));
        copyPropertyIfPresent (*track, *row, "track_type");
        copyPropertyIfPresent (*track, *row, "is_audio");
        copyPropertyIfPresent (*track, *row, "is_audio_track");
        copyPropertyIfPresent (*track, *row, "parent_track_id");
        copyPropertyIfPresent (*track, *row, "parent_folder_track_id");
        copyPropertyIfPresent (*track, *row, "depth");
        copyPropertyIfPresent (*track, *row, "track_depth");
        copyPropertyIfPresent (*track, *row, "child_track_ids");
        copyPropertyIfPresent (*track, *row, "direct_child_track_ids");
        copyPropertyIfPresent (*track, *row, "descendant_track_ids");
        copyPropertyIfPresent (*track, *row, "child_track_count");
        copyPropertyIfPresent (*track, *row, "descendant_track_count");
        copyPropertyIfPresent (*track, *row, "has_child_tracks");
        copyPropertyIfPresent (*track, *row, "is_folder_track");
        copyPropertyIfPresent (*track, *row, "is_folder_container");
        copyPropertyIfPresent (*track, *row, "is_submix_folder");
        copyPropertyIfPresent (*track, *row, "routing_bus_enabled");
        copyPropertyIfPresent (*track, *row, "folder_behavior");
        copyPropertyIfPresent (*track, *row, "can_contain_child_tracks");
        copyPropertyIfPresent (*track, *row, "vit_type");
        copyPropertyIfPresent (*track, *row, "vit_intent");
        copyPropertyIfPresent (*track, *row, "mute");
        copyPropertyIfPresent (*track, *row, "solo");
        copyPropertyIfPresent (*track, *row, "is_armed");
        copyPropertyIfPresent (*track, *row, "armed");
        copyPropertyIfPresent (*track, *row, "volume_db");
        copyPropertyIfPresent (*track, *row, "gain_db");
        copyPropertyIfPresent (*track, *row, "fader_db");
        copyPropertyIfPresent (*track, *row, "pan");
        copyPropertyIfPresent (*track, *row, "pan_value");
        copyPropertyIfPresent (*track, *row, "track_state_revision");

        const auto rack = normaliseRackState (track->getProperty ("rack"));
        if (rack.getDynamicObject() != nullptr)
            row->setProperty ("rack", rack);

        const auto clips = normaliseClipArray (track->getProperty ("clips"), trackId);
        row->setProperty ("clip_count", clips.size());
        row->setProperty ("clips", juce::var (clips));
        tracks.add (juce::var (row.release()));
    }

    return tracks;
}

juce::String projectEpochForSnapshot (const juce::var& snapshot)
{
    auto* snapshotObject = snapshot.getDynamicObject();
    if (snapshotObject == nullptr)
        return "epoch_project_current";

    const auto projectVar = snapshotObject->getProperty ("project");
    auto* project = projectVar.getDynamicObject();
    const auto projectPath = project != nullptr ? trimStringProperty (*project, "project_path") : juce::String();
    static const auto bootEpoch = generatedId ("kernel_boot_");
    const auto key = (projectPath.isNotEmpty() ? projectPath : "project_current") + "|" + bootEpoch;
    return "epoch_" + stableHashString (key);
}

juce::var makeCompactProjectSnapshot (const juce::DynamicObject& legacyState, const juce::String& requestedScope)
{
    auto snapshot = std::make_unique<juce::DynamicObject>();
    auto project = std::make_unique<juce::DynamicObject>();
    const auto tracks = normaliseTrackArray (legacyState.getProperty ("tracks"),
                                             shouldExposeUserTimelineTracksOnly (requestedScope));
    int clipCount = 0;

    for (const auto& trackVar : tracks)
    {
        if (auto* track = trackVar.getDynamicObject())
        {
            const auto clipsVar = track->getProperty ("clips");
            auto* clips = clipsVar.getArray();
            if (clips != nullptr)
                clipCount += clips->size();
        }
    }

    project->setProperty ("project_id", "project_current");
    copyPropertyIfPresent (legacyState, *project, "project_path");
    copyPropertyIfPresent (legacyState, *project, "project_uuid");
    copyPropertyIfPresent (legacyState, *project, "parent_project_uuid");
    copyPropertyIfPresent (legacyState, *project, "analysis_manifest");
    project->setProperty ("scope", requestedScope.isNotEmpty() ? requestedScope : trimStringProperty (legacyState, "scope"));
    project->setProperty ("track_count", tracks.size());
    project->setProperty ("clip_count", clipCount);
    copyPropertyIfPresent (legacyState, *project, "graph_revision", "legacy_graph_revision");
    copyPropertyIfPresent (legacyState, *project, "graph_active_revision", "legacy_graph_active_revision");
    copyPropertyIfPresent (legacyState, *project, "graph_pending_revision", "legacy_graph_pending_revision");
    copyPropertyIfPresent (legacyState, *project, "graph_last_diff_kind", "legacy_graph_last_diff_kind");
    copyPropertyIfPresent (legacyState, *project, "graph_publish_mode", "legacy_graph_publish_mode");
    copyPropertyIfPresent (legacyState, *project, "graph_lifecycle_state", "legacy_graph_lifecycle_state");

    snapshot->setProperty ("project", juce::var (project.release()));
    snapshot->setProperty ("tracks", juce::var (tracks));
    copyPropertyIfPresent (legacyState, *snapshot, "markers");
    return juce::var (snapshot.release());
}

juce::var buildLegacyStateRequest (const juce::DynamicObject& request, const juce::String& defaultScope)
{
    auto legacy = std::make_unique<juce::DynamicObject>();
    legacy->setProperty ("cmd", "get_project_state");

    const auto requestId = trimStringProperty (request, "request_id");
    if (requestId.isNotEmpty())
        legacy->setProperty ("request_id", requestId);

    auto* payload = request.getProperty ("payload").getDynamicObject();
    juce::String scope = defaultScope;

    if (payload != nullptr)
    {
        const auto payloadScope = firstStringProperty (*payload, { "scope", "clip_scope" });
        if (payloadScope.isNotEmpty())
            scope = payloadScope;
    }

    if (scope.isNotEmpty())
        legacy->setProperty ("scope", scope);

    legacy->setProperty ("vsp_adapter", "phase3_state_channel");
    return juce::var (legacy.release());
}

int64_t updateGlobalStateSnapshot (const juce::String& hash, const juce::var& snapshot)
{
    const juce::ScopedLock sl (stateStoreLock());
    auto& store = stateStore();

    if (store.revision <= 0)
        store.revision = 1;

    if (store.hash.isEmpty())
    {
        store.hash = hash;
        store.snapshot = snapshot;
    }
    else if (store.hash != hash)
    {
        ++store.revision;
        store.hash = hash;
        store.snapshot = snapshot;
    }

    return store.revision;
}

void updateSessionCursor (const juce::String& sessionId, const ProjectStateSnapshot& snapshot)
{
    const juce::ScopedLock sl (stateStoreLock());
    auto& cursor = stateStore().cursors[sessionId.toStdString()];
    cursor.valid = true;
    cursor.revision = snapshot.revision;
    cursor.hash = snapshot.hash;
    cursor.snapshot = snapshot.snapshot;
}

StateSessionCursor getSessionCursor (const juce::String& sessionId)
{
    const juce::ScopedLock sl (stateStoreLock());
    const auto found = stateStore().cursors.find (sessionId.toStdString());
    if (found == stateStore().cursors.end())
        return {};

    return found->second;
}

juce::var makeStateOp (const juce::String& op,
                       const juce::String& path,
                       const juce::var& value,
                       const juce::String& trackId,
                       const juce::String& clipId)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("op", op);
    object->setProperty ("path", path);

    if (! value.isVoid())
        object->setProperty ("value", value);

    if (trackId.isNotEmpty())
        object->setProperty ("track_id", trackId);

    if (clipId.isNotEmpty())
        object->setProperty ("clip_id", clipId);

    return juce::var (object.release());
}

std::unordered_map<std::string, juce::var> mapTracksById (const juce::var& snapshot)
{
    std::unordered_map<std::string, juce::var> result;
    auto* snapshotObject = snapshot.getDynamicObject();

    if (snapshotObject == nullptr)
        return result;

    const auto tracksVar = snapshotObject->getProperty ("tracks");
    auto* tracks = tracksVar.getArray();

    if (tracks == nullptr)
        return result;

    for (const auto& trackVar : *tracks)
    {
        auto* track = trackVar.getDynamicObject();
        if (track == nullptr)
            continue;

        const auto trackId = trimStringProperty (*track, "track_id");
        if (trackId.isNotEmpty())
            result[trackId.toStdString()] = trackVar;
    }

    return result;
}

std::unordered_map<std::string, juce::var> mapClipsById (const juce::var& trackVar)
{
    std::unordered_map<std::string, juce::var> result;
    auto* track = trackVar.getDynamicObject();

    if (track == nullptr)
        return result;

    const auto clipsVar = track->getProperty ("clips");
    auto* clips = clipsVar.getArray();

    if (clips == nullptr)
        return result;

    for (const auto& clipVar : *clips)
    {
        auto* clip = clipVar.getDynamicObject();
        if (clip == nullptr)
            continue;

        const auto clipId = trimStringProperty (*clip, "clip_id");
        if (clipId.isNotEmpty())
            result[clipId.toStdString()] = clipVar;
    }

    return result;
}

void appendClipDeltaOps (const juce::String& trackId,
                         const juce::var& previousTrack,
                         const juce::var& currentTrack,
                         juce::Array<juce::var>& ops,
                         juce::StringArray& changedTracks,
                         juce::StringArray& changedClips)
{
    const auto previousClips = mapClipsById (previousTrack);
    const auto currentClips = mapClipsById (currentTrack);

    for (const auto& [clipKey, previousClip] : previousClips)
    {
        juce::ignoreUnused (previousClip);

        if (currentClips.find (clipKey) != currentClips.end())
            continue;

        const auto clipId = juce::String (clipKey);
        ops.add (makeStateOp ("remove", "/tracks/" + trackId + "/clips/" + clipId, {}, trackId, clipId));
        addUnique (changedTracks, trackId);
        addUnique (changedClips, clipId);
    }

    for (const auto& [clipKey, currentClip] : currentClips)
    {
        const auto clipId = juce::String (clipKey);
        const auto foundPrevious = previousClips.find (clipKey);

        if (foundPrevious == previousClips.end())
        {
            ops.add (makeStateOp ("add", "/tracks/" + trackId + "/clips/" + clipId, currentClip, trackId, clipId));
            addUnique (changedTracks, trackId);
            addUnique (changedClips, clipId);
            continue;
        }

        if (! sameJsonValue (foundPrevious->second, currentClip))
        {
            ops.add (makeStateOp ("replace", "/tracks/" + trackId + "/clips/" + clipId, currentClip, trackId, clipId));
            addUnique (changedTracks, trackId);
            addUnique (changedClips, clipId);
        }
    }
}

juce::Array<juce::var> buildStateDeltaOps (const juce::var& previousSnapshot,
                                           const juce::var& currentSnapshot,
                                           juce::StringArray& changedTracks,
                                           juce::StringArray& changedClips)
{
    juce::Array<juce::var> ops;
    auto* previousObject = previousSnapshot.getDynamicObject();
    auto* currentObject = currentSnapshot.getDynamicObject();

    if (previousObject != nullptr && currentObject != nullptr)
    {
        const auto previousProject = previousObject->getProperty ("project");
        const auto currentProject = currentObject->getProperty ("project");
        if (! sameJsonValue (previousProject, currentProject))
            ops.add (makeStateOp ("replace", "/project", currentProject, {}, {}));
    }

    const auto previousTracks = mapTracksById (previousSnapshot);
    const auto currentTracks = mapTracksById (currentSnapshot);

    for (const auto& [trackKey, previousTrack] : previousTracks)
    {
        juce::ignoreUnused (previousTrack);

        if (currentTracks.find (trackKey) != currentTracks.end())
            continue;

        const auto trackId = juce::String (trackKey);
        ops.add (makeStateOp ("remove", "/tracks/" + trackId, {}, trackId, {}));
        addUnique (changedTracks, trackId);
    }

    for (const auto& [trackKey, currentTrack] : currentTracks)
    {
        const auto trackId = juce::String (trackKey);
        const auto foundPrevious = previousTracks.find (trackKey);

        if (foundPrevious == previousTracks.end())
        {
            ops.add (makeStateOp ("add", "/tracks/" + trackId, currentTrack, trackId, {}));
            addUnique (changedTracks, trackId);
            continue;
        }

        const auto previousCore = buildTrackCore (foundPrevious->second);
        const auto currentCore = buildTrackCore (currentTrack);
        if (! sameJsonValue (previousCore, currentCore))
        {
            ops.add (makeStateOp ("replace", "/tracks/" + trackId, currentCore, trackId, {}));
            addUnique (changedTracks, trackId);
        }

        appendClipDeltaOps (trackId, foundPrevious->second, currentTrack, ops, changedTracks, changedClips);
    }

    return ops;
}

void setCommonEnvelopeFields (juce::DynamicObject& response,
                              const juce::DynamicObject& request,
                              const juce::String& schema,
                              const juce::String& channel,
                              const juce::String& type)
{
    response.setProperty ("vsp_version", "1.0");
    response.setProperty ("schema", schema);
    response.setProperty ("message_id", generatedId ("msg_kernel_"));
    response.setProperty ("session_id", trimStringProperty (request, "session_id").isNotEmpty()
                                             ? trimStringProperty (request, "session_id")
                                             : "session_unknown");
    response.setProperty ("client_id", "kernel.main");
    response.setProperty ("role", "kernel");
    response.setProperty ("channel", channel);
    response.setProperty ("type", type);
    response.setProperty ("created_at", nowIso8601());

    const auto requestId = trimStringProperty (request, "request_id");
    if (requestId.isNotEmpty())
        response.setProperty ("request_id", requestId);

    const auto messageId = trimStringProperty (request, "message_id");
    if (messageId.isNotEmpty())
        response.setProperty ("correlation_id", messageId);

    const auto traceId = trimStringProperty (request, "trace_id");
    if (traceId.isNotEmpty())
        response.setProperty ("trace_id", traceId);
}

juce::String makeVspError (const juce::DynamicObject& request,
                           const juce::String& channel,
                           const juce::String& type,
                           const juce::String& code,
                           const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp." + channel + ".error.v1", channel, type);

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "rejected");
    ack->setProperty ("message", message);
    response->setProperty ("ack", juce::var (ack.release()));

    auto error = std::make_unique<juce::DynamicObject>();
    error->setProperty ("code", code);
    error->setProperty ("message", message);
    error->setProperty ("retryable", code == "busy" || code == "timeout" || code == "transport_error");
    response->setProperty ("error", juce::var (error.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", "error");
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String makeSessionHelloAck (const juce::DynamicObject& request)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.session.hello_ack.v1", "session", "session.hello_ack");
    response->setProperty ("session_id", generatedId ("sess_kernel_"));
    response->setProperty ("capabilities", juce::var (toVarArray (capabilityList())));

    auto flags = std::make_unique<juce::DynamicObject>();
    flags->setProperty ("command.batch", true);
    flags->setProperty ("plugin.set_params_batch", true);
    flags->setProperty ("audio.l2_render_probe", true);
    flags->setProperty ("audio.compressor_dual_tap_probe", true);
    flags->setProperty ("audition.preview", true);
    flags->setProperty ("audition.preview.audio_file_target", true);
    flags->setProperty ("audition.preview.block_boundary_switch", true);
    flags->setProperty ("audition.preview.crossfade", true);
    flags->setProperty ("audition.preview.full_project", false);
    flags->setProperty ("command.idempotency", true);
    flags->setProperty ("command.base_revision_cas", true);
    flags->setProperty ("state.snapshot.scoped", true);
    flags->setProperty ("state.delta", true);
    flags->setProperty ("state.resync", true);
    flags->setProperty ("realtime.visible_tracks", true);
    flags->setProperty ("realtime.latest_only", true);
    flags->setProperty ("realtime.drop_old", true);
    flags->setProperty ("asset.reference", true);
    flags->setProperty ("asset.manifest", true);
    flags->setProperty ("asset.range_read", false);
    flags->setProperty ("event.job_progress", true);
    flags->setProperty ("legacy.ipc_adapter", true);
    response->setProperty ("feature_flags", juce::var (flags.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("accepted_protocol", "1.0");
    payload->setProperty ("project_epoch", "epoch_kernel_reference_v1");
    payload->setProperty ("server_name", "Vit Kernel");
    payload->setProperty ("server_version", "phase4-realtime-asset-reference");
    payload->setProperty ("session_mode", "phase4_reference");
    payload->setProperty ("transport_binding", "legacy.zmq_reqrep");
    response->setProperty ("payload", juce::var (payload.release()));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", "VSP session accepted");
    response->setProperty ("ack", juce::var (ack.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String errorCodeFromLegacyMessage (const juce::String& message)
{
    const auto lower = message.toLowerCase();

    if (lower.contains ("missing") || lower.contains ("requires") || lower.contains ("invalid"))
        return "validation_error";

    if (lower.contains ("not found") || lower.contains ("unknown command"))
        return "not_found";

    if (lower.contains ("busy"))
        return "busy";

    if (lower.contains ("permission"))
        return "permission_denied";

    if (lower.contains ("timeout"))
        return "timeout";

    return "legacy_adapter_error";
}

ProjectStateSnapshot captureProjectStateSnapshot (const juce::DynamicObject& request,
                                                  const VspKernelReference::LegacyCommandHandler& legacyHandler,
                                                  const juce::String& defaultScope)
{
    ProjectStateSnapshot result;

    if (! legacyHandler)
    {
        result.errorCode = "internal_error";
        result.errorMessage = "Legacy command handler unavailable";
        return result;
    }

    auto legacyRequest = buildLegacyStateRequest (request, defaultScope);
    const auto legacyRaw = juce::JSON::toString (legacyRequest);
    const auto legacyReplyRaw = legacyHandler (legacyRequest, legacyRaw);
    const auto parsed = juce::JSON::parse (legacyReplyRaw);
    auto* legacyState = parsed.getDynamicObject();

    if (legacyState == nullptr)
    {
        result.errorCode = "legacy_adapter_error";
        result.errorMessage = "get_project_state returned non-object JSON";
        return result;
    }

    const auto status = trimStringProperty (*legacyState, "status");
    if (status.equalsIgnoreCase ("error"))
    {
        result.errorCode = errorCodeFromLegacyMessage (trimStringProperty (*legacyState, "message"));
        result.errorMessage = trimStringProperty (*legacyState, "message");
        if (result.errorMessage.isEmpty())
            result.errorMessage = "get_project_state failed";
        return result;
    }

    result.scope = defaultScope.isNotEmpty() ? defaultScope : trimStringProperty (*legacyState, "scope");
    result.snapshot = makeCompactProjectSnapshot (*legacyState, result.scope);
    result.hash = stableHashVar (result.snapshot);
    result.revision = updateGlobalStateSnapshot (result.hash, result.snapshot);
    result.projectEpoch = projectEpochForSnapshot (result.snapshot);
    result.valid = true;
    return result;
}

constexpr int kRealtimeMaxVisibleTracks = 17;

int defaultRealtimeHzForStream (const juce::String& stream)
{
    if (stream == "spectrum.visible_tracks")
        return 12;

    if (stream == "meters.visible_tracks")
        return 20;

    return 30;
}

bool isSupportedRealtimeStream (const juce::String& stream)
{
    return stream == "transport.playhead"
        || stream == "meters.visible_tracks"
        || stream == "spectrum.visible_tracks"
        || stream == "recording.status";
}

bool streamUsesVisibleTracks (const juce::String& stream)
{
    return stream == "meters.visible_tracks" || stream == "spectrum.visible_tracks";
}

juce::StringArray limitedVisibleTrackIds (const juce::StringArray& requested)
{
    juce::StringArray limited;

    for (const auto& trackId : requested)
    {
        addUnique (limited, trackId);

        if (limited.size() >= kRealtimeMaxVisibleTracks)
            break;
    }

    return limited;
}

juce::var makeRealtimeTrackBudget (int requestedTrackCount, int acceptedTrackCount)
{
    auto budget = std::make_unique<juce::DynamicObject>();
    budget->setProperty ("max_visible_tracks", kRealtimeMaxVisibleTracks);
    budget->setProperty ("requested_track_count", requestedTrackCount);
    budget->setProperty ("accepted_track_count", acceptedTrackCount);
    budget->setProperty ("policy", "visible_tracks_only");
    return juce::var (budget.release());
}

juce::var makeRealtimeStreamDescriptor (const juce::var& streamVar, juce::String& error)
{
    auto descriptor = std::make_unique<juce::DynamicObject>();
    auto* streamObject = streamVar.getDynamicObject();
    const auto stream = streamObject != nullptr ? firstStringProperty (*streamObject, { "stream", "name" })
                                                : trimStringVar (streamVar);

    if (stream.isEmpty())
    {
        error = "realtime.subscribe stream entry requires stream";
        return juce::var (descriptor.release());
    }

    if (! isSupportedRealtimeStream (stream))
    {
        error = "Unsupported realtime stream: " + stream;
        return juce::var (descriptor.release());
    }

    const auto defaultHz = defaultRealtimeHzForStream (stream);
    const auto requestedMaxHz = streamObject != nullptr ? intFromVar (streamObject->getProperty ("max_hz"), defaultHz)
                                                       : defaultHz;
    const auto maxHz = juce::jlimit (1, 60, requestedMaxHz);
    const auto requestedMode = streamObject != nullptr ? trimStringProperty (*streamObject, "mode") : juce::String();
    juce::StringArray requestedTrackIds;

    if (streamObject != nullptr)
    {
        requestedTrackIds = stringArrayFromVar (streamObject->getProperty ("track_ids"));
        if (requestedTrackIds.isEmpty())
            requestedTrackIds = stringArrayFromVar (streamObject->getProperty ("visible_track_ids"));
    }

    const auto acceptedTrackIds = streamUsesVisibleTracks (stream) ? limitedVisibleTrackIds (requestedTrackIds)
                                                                  : requestedTrackIds;

    descriptor->setProperty ("stream_id", generatedId ("rt_stream_"));
    descriptor->setProperty ("stream", stream);
    descriptor->setProperty ("enabled", true);
    descriptor->setProperty ("mode", "latest_only");
    descriptor->setProperty ("latest_only", true);
    descriptor->setProperty ("drop_old", true);
    descriptor->setProperty ("queue_policy", "drop_old");
    descriptor->setProperty ("max_hz", maxHz);
    descriptor->setProperty ("min_interval_ms", 1000.0 / static_cast<double> (maxHz));

    if (requestedMode.isNotEmpty() && requestedMode != "latest_only")
        descriptor->setProperty ("requested_mode", requestedMode);

    if (streamUsesVisibleTracks (stream))
    {
        descriptor->setProperty ("visible_tracks_only", true);
        descriptor->setProperty ("track_ids", makeStringArrayVar (acceptedTrackIds));
        descriptor->setProperty ("track_count", acceptedTrackIds.size());
        descriptor->setProperty ("track_budget", makeRealtimeTrackBudget (requestedTrackIds.size(), acceptedTrackIds.size()));
    }

    if (streamObject != nullptr)
    {
        const auto visibleRange = streamObject->getProperty ("visible_range");
        if (! visibleRange.isVoid())
            descriptor->setProperty ("visible_range", visibleRange);

        const auto visibleWindow = streamObject->getProperty ("visible_window");
        if (! visibleWindow.isVoid())
            descriptor->setProperty ("visible_window", visibleWindow);
    }

    return juce::var (descriptor.release());
}

bool getRealtimeSubscription (const juce::String& subscriptionId, RealtimeSubscription& result)
{
    const juce::ScopedLock sl (phase4StoreLock());
    const auto found = phase4Store().realtimeSubscriptions.find (subscriptionId.toStdString());

    if (found == phase4Store().realtimeSubscriptions.end())
        return false;

    result = found->second;
    return true;
}

void putRealtimeSubscription (const RealtimeSubscription& subscription)
{
    const juce::ScopedLock sl (phase4StoreLock());
    phase4Store().realtimeSubscriptions[subscription.subscriptionId.toStdString()] = subscription;
}

bool removeRealtimeSubscription (const juce::String& subscriptionId)
{
    const juce::ScopedLock sl (phase4StoreLock());
    return phase4Store().realtimeSubscriptions.erase (subscriptionId.toStdString()) > 0;
}

int64_t nextRealtimeFrameIndex()
{
    const juce::ScopedLock sl (phase4StoreLock());
    return ++phase4Store().realtimeFrameIndex;
}

juce::String makeRealtimeStreamStatusResponse (const juce::DynamicObject& request,
                                               const RealtimeSubscription& subscription,
                                               const juce::String& status,
                                               const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.realtime.stream_status.v1", "realtime", "realtime.stream_status");

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", status == "unsubscribed" ? "completed" : "completed");
    ack->setProperty ("message", message);
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", status);
    payload->setProperty ("subscription_id", subscription.subscriptionId);
    payload->setProperty ("stream_count", subscription.streams.size());
    payload->setProperty ("streams", juce::var (subscription.streams));
    payload->setProperty ("default_mode", "latest_only");
    payload->setProperty ("queue_policy", "drop_old");
    payload->setProperty ("transport_binding", "legacy.zmq_reqrep.reference_pull");
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String handleRealtimeSubscribe (const juce::DynamicObject& request)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.subscribe requires payload object");

    juce::Array<juce::var> streams;
    const auto streamsVar = payload->getProperty ("streams");

    if (auto* requestedStreams = streamsVar.getArray())
    {
        for (const auto& streamVar : *requestedStreams)
        {
            juce::String error;
            auto descriptor = makeRealtimeStreamDescriptor (streamVar, error);
            if (error.isNotEmpty())
                return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", error);

            streams.add (descriptor);
        }
    }
    else
    {
        juce::String error;
        auto descriptor = makeRealtimeStreamDescriptor (juce::var (payload), error);
        if (error.isNotEmpty())
            return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", error);

        streams.add (descriptor);
    }

    if (streams.isEmpty())
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.subscribe requires at least one stream");

    RealtimeSubscription subscription;
    subscription.sessionId = trimStringProperty (request, "session_id");
    subscription.subscriptionId = firstStringProperty (*payload, { "subscription_id", "id" });
    if (subscription.subscriptionId.isEmpty())
        subscription.subscriptionId = generatedId ("sub_rt_");
    subscription.streams = streams;

    putRealtimeSubscription (subscription);
    return makeRealtimeStreamStatusResponse (request, subscription, "subscribed", "Realtime subscription accepted");
}

juce::String handleRealtimeUnsubscribe (const juce::DynamicObject& request)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.unsubscribe requires payload object");

    const auto subscriptionId = firstStringProperty (*payload, { "subscription_id", "id" });
    if (subscriptionId.isEmpty())
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.unsubscribe requires subscription_id");

    RealtimeSubscription subscription;
    if (! getRealtimeSubscription (subscriptionId, subscription))
    {
        subscription.subscriptionId = subscriptionId;
        subscription.sessionId = trimStringProperty (request, "session_id");
        return makeRealtimeStreamStatusResponse (request, subscription, "no_op", "Realtime subscription was not active");
    }

    removeRealtimeSubscription (subscriptionId);

    for (auto& streamVar : subscription.streams)
        if (auto* stream = streamVar.getDynamicObject())
            stream->setProperty ("enabled", false);

    return makeRealtimeStreamStatusResponse (request, subscription, "unsubscribed", "Realtime subscription removed");
}

juce::var makeRealtimeReferenceDataForStream (const juce::DynamicObject& stream)
{
    const auto streamName = trimStringProperty (stream, "stream");
    auto data = std::make_unique<juce::DynamicObject>();

    if (streamName == "transport.playhead")
    {
        data->setProperty ("position_seconds", 0.0);
        data->setProperty ("is_playing", false);
        data->setProperty ("is_recording", false);
        data->setProperty ("source", "reference_sample");
    }
    else if (streamName == "recording.status")
    {
        data->setProperty ("is_recording", false);
        data->setProperty ("armed_track_count", 0);
        data->setProperty ("source", "reference_sample");
    }
    else if (streamName == "meters.visible_tracks")
    {
        juce::Array<juce::var> meters;
        const auto trackIds = stringArrayFromVar (stream.getProperty ("track_ids"));

        for (const auto& trackId : trackIds)
        {
            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("track_id", trackId);
            row->setProperty ("peak_db", -120.0);
            row->setProperty ("rms_db", -120.0);
            row->setProperty ("clipped", false);
            meters.add (juce::var (row.release()));
        }

        data->setProperty ("tracks", juce::var (meters));
        data->setProperty ("visible_track_count", trackIds.size());
    }
    else if (streamName == "spectrum.visible_tracks")
    {
        juce::Array<juce::var> refs;
        const auto trackIds = stringArrayFromVar (stream.getProperty ("track_ids"));

        for (const auto& trackId : trackIds)
        {
            auto row = std::make_unique<juce::DynamicObject>();
            row->setProperty ("track_id", trackId);
            row->setProperty ("kind", "spectrum_tile");
            row->setProperty ("uri", "vit-cache://project_current/tracks/" + trackId + "/spectrum/latest");
            row->setProperty ("revision", static_cast<juce::int64> (nextRealtimeFrameIndex()));
            refs.add (juce::var (row.release()));
        }

        data->setProperty ("asset_refs", juce::var (refs));
        data->setProperty ("visible_track_count", trackIds.size());
        data->setProperty ("inline_bins", false);
    }

    return juce::var (data.release());
}

juce::var makeRealtimeDataForStream (const juce::DynamicObject& stream,
                                     const VspKernelReference::RealtimeDataProvider& realtimeDataProvider)
{
    if (realtimeDataProvider)
    {
        auto provided = realtimeDataProvider (stream);
        if (! provided.isVoid())
            return provided;
    }

    return makeRealtimeReferenceDataForStream (stream);
}

juce::var makeRealtimeFrame (const juce::var& streamVar,
                             const VspKernelReference::RealtimeDataProvider& realtimeDataProvider)
{
    auto frame = std::make_unique<juce::DynamicObject>();
    auto* stream = streamVar.getDynamicObject();
    const auto frameIndex = nextRealtimeFrameIndex();

    frame->setProperty ("frame_index", static_cast<juce::int64> (frameIndex));
    frame->setProperty ("timestamp_ms", static_cast<juce::int64> (juce::Time::currentTimeMillis()));
    frame->setProperty ("latest_only", true);
    frame->setProperty ("drop_old", true);

    if (stream != nullptr)
    {
        copyPropertyIfPresent (*stream, *frame, "stream_id");
        copyPropertyIfPresent (*stream, *frame, "stream");
        copyPropertyIfPresent (*stream, *frame, "mode");
        copyPropertyIfPresent (*stream, *frame, "max_hz");
        copyPropertyIfPresent (*stream, *frame, "track_ids");
        frame->setProperty ("data", makeRealtimeDataForStream (*stream, realtimeDataProvider));
    }

    return juce::var (frame.release());
}

juce::String handleRealtimeFrameRequest (const juce::DynamicObject& request,
                                         const VspKernelReference::RealtimeDataProvider& realtimeDataProvider)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.frame_request requires payload object");

    const auto subscriptionId = firstStringProperty (*payload, { "subscription_id", "id" });
    if (subscriptionId.isEmpty())
        return makeVspError (request, "realtime", "realtime.stream_status", "validation_error", "realtime.frame_request requires subscription_id");

    RealtimeSubscription subscription;
    if (! getRealtimeSubscription (subscriptionId, subscription))
        return makeVspError (request, "realtime", "realtime.stream_status", "not_found", "Realtime subscription not found: " + subscriptionId);

    const auto requestedStreamId = trimStringProperty (*payload, "stream_id");
    juce::Array<juce::var> frames;

    for (const auto& streamVar : subscription.streams)
    {
        auto* stream = streamVar.getDynamicObject();
        if (stream == nullptr)
            continue;

        if (requestedStreamId.isNotEmpty() && requestedStreamId != trimStringProperty (*stream, "stream_id"))
            continue;

        frames.add (makeRealtimeFrame (streamVar, realtimeDataProvider));

        if (requestedStreamId.isNotEmpty())
            break;
    }

    if (frames.isEmpty())
        return makeVspError (request, "realtime", "realtime.stream_status", "not_found", "Requested realtime stream not found");

    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.realtime.frame.v1", "realtime", "realtime.frame");

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", "Realtime latest frame sampled");
    response->setProperty ("ack", juce::var (ack.release()));

    auto payloadOut = std::make_unique<juce::DynamicObject>();
    payloadOut->setProperty ("status", "ok");
    payloadOut->setProperty ("subscription_id", subscription.subscriptionId);
    payloadOut->setProperty ("frame_count", frames.size());
    payloadOut->setProperty ("frames", juce::var (frames));

    if (frames.size() == 1)
    {
        if (auto* frame = frames.getReference (0).getDynamicObject())
        {
            copyPropertyIfPresent (*frame, *payloadOut, "stream_id");
            copyPropertyIfPresent (*frame, *payloadOut, "stream");
            copyPropertyIfPresent (*frame, *payloadOut, "frame_index");
            copyPropertyIfPresent (*frame, *payloadOut, "timestamp_ms");
            copyPropertyIfPresent (*frame, *payloadOut, "data");
        }
    }

    response->setProperty ("payload", juce::var (payloadOut.release()));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String handleRealtimeMessage (const juce::DynamicObject& request,
                                    const VspKernelReference::RealtimeDataProvider& realtimeDataProvider)
{
    const auto type = trimStringProperty (request, "type");

    if (type.equalsIgnoreCase ("realtime.subscribe"))
        return handleRealtimeSubscribe (request);

    if (type.equalsIgnoreCase ("realtime.unsubscribe"))
        return handleRealtimeUnsubscribe (request);

    if (type.equalsIgnoreCase ("realtime.frame_request") || type.equalsIgnoreCase ("realtime.poll"))
        return handleRealtimeFrameRequest (request, realtimeDataProvider);

    return makeVspError (request, "realtime", "realtime.stream_status", "capability_not_supported", "Unsupported realtime message type: " + type);
}

juce::String normaliseAssetKind (const juce::String& requested)
{
    const auto lower = requested.trim().toLowerCase();

    if (lower.isEmpty() || lower.contains ("waveform") || lower == "peak" || lower == "peaks")
        return "waveform_peak";

    if (lower.contains ("spectrum") || lower.contains ("spectral") || lower.contains ("spectrogram"))
        return "spectrum_tile";

    return lower;
}

juce::String assetRouteForKind (const juce::String& kind)
{
    if (kind == "spectrum_tile")
        return "spectrum/tiles";

    return "waveform/peaks";
}

juce::String assetFormatForKind (const juce::String& kind)
{
    if (kind == "spectrum_tile")
        return "f32_spectrum_tile";

    return "f32_peak_minmax";
}

juce::var findClipInSnapshot (const juce::var& snapshot, const juce::String& clipId, juce::String& trackId)
{
    auto* snapshotObject = snapshot.getDynamicObject();
    if (snapshotObject == nullptr)
        return {};

    const auto tracksVar = snapshotObject->getProperty ("tracks");
    auto* tracks = tracksVar.getArray();
    if (tracks == nullptr)
        return {};

    for (const auto& trackVar : *tracks)
    {
        auto* track = trackVar.getDynamicObject();
        if (track == nullptr)
            continue;

        const auto clipsVar = track->getProperty ("clips");
        auto* clips = clipsVar.getArray();
        if (clips == nullptr)
            continue;

        for (const auto& clipVar : *clips)
        {
            auto* clip = clipVar.getDynamicObject();
            if (clip == nullptr)
                continue;

            if (trimStringProperty (*clip, "clip_id") == clipId || trimStringProperty (*clip, "id") == clipId)
            {
                trackId = trimStringProperty (*track, "track_id");
                return clipVar;
            }
        }
    }

    return {};
}

bool stringFilterAllows (const juce::StringArray& acceptedValues, const juce::String& value)
{
    return acceptedValues.isEmpty() || acceptedValues.contains (value);
}

juce::var makeAssetReferencePayload (const ProjectStateSnapshot& snapshot,
                                     const juce::DynamicObject& requestPayload,
                                     const juce::DynamicObject& clip,
                                     const juce::String& clipId,
                                     const juce::String& trackId,
                                     const juce::String& kind)
{
    auto payload = std::make_unique<juce::DynamicObject>();
    auto asset = std::make_unique<juce::DynamicObject>();
    const auto assetRef = firstStringProperty (clip, { "asset_ref", "asset_id" });
    const auto assetState = firstStringProperty (clip, { "asset_state", "state" });
    const auto sourcePath = firstStringProperty (clip, { "current_source_path", "file_path" });
    const auto revision = static_cast<juce::int64> (snapshot.revision);
    const auto route = assetRouteForKind (kind);
    const auto uri = "vit-cache://project_current/clips/" + clipId + "/" + route + "/v1";
    const auto identity = kind + ":" + clipId + ":" + assetRef + ":" + juce::String (revision);

    asset->setProperty ("asset_id", assetRef.isNotEmpty() ? assetRef : "asset_" + stableHashString (identity));
    asset->setProperty ("kind", kind);
    asset->setProperty ("owner", clipId);
    asset->setProperty ("owner_type", "clip");
    asset->setProperty ("track_id", trackId);
    asset->setProperty ("uri", uri);
    asset->setProperty ("revision", revision);
    asset->setProperty ("format", assetFormatForKind (kind));
    asset->setProperty ("hash", "stable:" + stableHashString (identity + ":" + sourcePath));
    asset->setProperty ("state", assetState.isNotEmpty() ? assetState : "reference_only");
    asset->setProperty ("platform_neutral", true);

    copyPropertyIfPresent (clip, *asset, "length_seconds", "duration_seconds");
    copyPropertyIfPresent (clip, *asset, "start_seconds");
    copyPropertyIfPresent (clip, *asset, "end_seconds");

    const auto byteRange = requestPayload.getProperty ("byte_range");
    if (! byteRange.isVoid())
        asset->setProperty ("byte_range", byteRange);

    const auto range = requestPayload.getProperty ("range");
    if (! range.isVoid())
        asset->setProperty ("range", range);

    juce::Array<juce::var> bindings;

    auto cacheBinding = std::make_unique<juce::DynamicObject>();
    cacheBinding->setProperty ("binding", "cache_file");
    cacheBinding->setProperty ("platform", "any");
    cacheBinding->setProperty ("ttl_ms", 60000);
    cacheBinding->setProperty ("availability", "reference_only");
    bindings.add (juce::var (cacheBinding.release()));

    if (sourcePath.isNotEmpty())
    {
        auto sourceBinding = std::make_unique<juce::DynamicObject>();
        sourceBinding->setProperty ("binding", "legacy_source_file");
        sourceBinding->setProperty ("platform", "local");
        sourceBinding->setProperty ("optional", true);
        sourceBinding->setProperty ("path", sourcePath);
        sourceBinding->setProperty ("identity_role", "binding_metadata_only");
        bindings.add (juce::var (sourceBinding.release()));
    }

    payload->setProperty ("status", "ok");
    payload->setProperty ("asset", juce::var (asset.release()));
    payload->setProperty ("bindings", juce::var (bindings));
    payload->setProperty ("inline_payload", false);
    payload->setProperty ("no_big_json_payload", true);
    return juce::var (payload.release());
}

juce::var makeAssetManifestReferenceRow (const ProjectStateSnapshot& snapshot,
                                         const juce::DynamicObject& requestPayload,
                                         const juce::DynamicObject& clip,
                                         const juce::String& clipId,
                                         const juce::String& trackId,
                                         const juce::String& kind)
{
    auto row = std::make_unique<juce::DynamicObject>();
    const auto payloadVar = makeAssetReferencePayload (snapshot, requestPayload, clip, clipId, trackId, kind);
    auto* payload = payloadVar.getDynamicObject();

    row->setProperty ("clip_id", clipId);
    row->setProperty ("kernel_track_id", trackId);
    row->setProperty ("track_id", trackId);
    row->setProperty ("kind", kind);
    row->setProperty ("revision", static_cast<juce::int64> (snapshot.revision));
    row->setProperty ("project_epoch", snapshot.projectEpoch);
    row->setProperty ("source", "vsp.asset.manifest");

    if (payload != nullptr)
    {
        copyPropertyIfPresent (*payload, *row, "status");
        copyPropertyIfPresent (*payload, *row, "inline_payload");
        copyPropertyIfPresent (*payload, *row, "no_big_json_payload");
        row->setProperty ("asset", payload->getProperty ("asset"));
        row->setProperty ("bindings", payload->getProperty ("bindings"));
    }

    return juce::var (row.release());
}

bool assetManifestShouldPrepareWaveform (const juce::DynamicObject& requestPayload,
                                         const juce::String& kind)
{
    if (kind != "waveform_peak")
        return false;

    if (requestPayload.hasProperty ("prepare"))
        return boolFromVar (requestPayload.getProperty ("prepare"), false);

    if (requestPayload.hasProperty ("prepare_assets"))
        return boolFromVar (requestPayload.getProperty ("prepare_assets"), false);

    const auto priority = trimStringProperty (requestPayload, "priority").toLowerCase();
    return priority.contains ("visible")
        || priority.contains ("interactive")
        || priority.contains ("foreground")
        || priority.contains ("urgent")
        || priority.contains ("now");
}

juce::String assetManifestPreparePriority (const juce::DynamicObject& requestPayload)
{
    const auto priority = trimStringProperty (requestPayload, "priority").toLowerCase();
    if (priority.contains ("background"))
        return "background_warm";

    return "visible_warm";
}

int assetManifestPrepareLimit (const juce::DynamicObject& requestPayload, int maxAssets)
{
    const auto requested = intFromVar (requestPayload.getProperty ("max_prepares"), -1);
    if (requested >= 0)
        return juce::jlimit (0, 512, requested);

    return juce::jlimit (1, 512, juce::jmin (maxAssets, 32));
}

bool clipCanPrepareWaveformFromManifest (const juce::DynamicObject& clip)
{
    const auto clipType = firstStringProperty (clip, { "clip_type", "type" }).toLowerCase();
    return clipType.isEmpty()
        || (clipType != "midi" && clipType != "sequencer" && clipType != "step");
}

AssetPrepareResult requestWaveformPrepareFromManifestRow (
    const juce::DynamicObject& row,
    const juce::DynamicObject& requestPayload,
    const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    AssetPrepareResult result;
    result.requestId = generatedId ("asset_prepare_");

    if (! legacyHandler)
    {
        result.status = "error";
        result.message = "Legacy command handler unavailable";
        return result;
    }

    const auto clipId = trimStringProperty (row, "clip_id");
    const auto trackId = trimStringProperty (row, "kernel_track_id");
    if (clipId.isEmpty() || trackId.isEmpty())
    {
        result.status = "error";
        result.message = "Manifest row missing clip_id or kernel_track_id";
        return result;
    }

    auto legacy = std::make_unique<juce::DynamicObject>();
    legacy->setProperty ("cmd", "warm_waveform_bake");
    legacy->setProperty ("command", "asset.manifest.prepare_waveform");
    legacy->setProperty ("track_id", trackId);
    legacy->setProperty ("clip_id", clipId);
    legacy->setProperty ("feature_type", "waveform_envelope");
    legacy->setProperty ("priority", assetManifestPreparePriority (requestPayload));
    legacy->setProperty ("request_id", result.requestId);
    legacy->setProperty ("source", "vsp.asset.manifest");

    auto legacyVar = juce::var (legacy.release());
    const auto legacyRaw = juce::JSON::toString (legacyVar);
    const auto replyRaw = legacyHandler (legacyVar, legacyRaw);
    const auto parsed = juce::JSON::parse (replyRaw);
    auto* reply = parsed.getDynamicObject();
    if (reply == nullptr)
    {
        result.status = "error";
        result.message = "warm_waveform_bake returned non-object JSON";
        return result;
    }

    const auto status = trimStringProperty (*reply, "status");
    result.status = status.isNotEmpty() ? status : "unknown";
    result.message = trimStringProperty (*reply, "message");
    result.ok = status.equalsIgnoreCase ("ok");
    if (! result.ok && result.message.isEmpty())
        result.message = replyRaw;
    return result;
}

juce::var makeAssetManifestPayload (const ProjectStateSnapshot& snapshot,
                                    const juce::DynamicObject& requestPayload,
                                    const juce::String& kind,
                                    const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    auto payload = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> references;
    auto requestedTrackIds = stringArrayFromVar (requestPayload.getProperty ("track_ids"));
    if (requestedTrackIds.isEmpty())
        requestedTrackIds = stringArrayFromVar (requestPayload.getProperty ("visible_track_ids"));
    const auto requestedClipIds = stringArrayFromVar (requestPayload.getProperty ("clip_ids"));
    const auto maxAssets = juce::jlimit (1, 512, intFromVar (requestPayload.getProperty ("max_assets"), 128));
    const bool prepareWaveform = assetManifestShouldPrepareWaveform (requestPayload, kind);
    const auto prepareLimit = prepareWaveform ? assetManifestPrepareLimit (requestPayload, maxAssets) : 0;
    int prepareAttemptCount = 0;
    int prepareOkCount = 0;
    int prepareErrorCount = 0;
    bool prepareTruncated = false;

    auto* snapshotObject = snapshot.snapshot.getDynamicObject();
    auto* tracks = snapshotObject != nullptr ? snapshotObject->getProperty ("tracks").getArray() : nullptr;
    bool reachedLimit = false;

    if (tracks != nullptr)
    {
        for (const auto& trackVar : *tracks)
        {
            auto* track = trackVar.getDynamicObject();
            if (track == nullptr)
                continue;

            const auto trackId = firstStringProperty (*track, { "track_id", "id" });
            if (! stringFilterAllows (requestedTrackIds, trackId))
                continue;

            auto* clips = track->getProperty ("clips").getArray();
            if (clips == nullptr)
                continue;

            for (const auto& clipVar : *clips)
            {
                auto* clip = clipVar.getDynamicObject();
                if (clip == nullptr)
                    continue;

                const auto clipId = firstStringProperty (*clip, { "clip_id", "id" });
                if (clipId.isEmpty() || ! stringFilterAllows (requestedClipIds, clipId))
                    continue;

                if (kind == "waveform_peak" && ! clipCanPrepareWaveformFromManifest (*clip))
                    continue;

                auto rowVar = makeAssetManifestReferenceRow (snapshot, requestPayload, *clip, clipId, trackId, kind);
                auto* row = rowVar.getDynamicObject();
                if (prepareWaveform && row != nullptr)
                {
                    if (prepareAttemptCount < prepareLimit)
                    {
                        auto prepare = requestWaveformPrepareFromManifestRow (*row, requestPayload, legacyHandler);
                        row->setProperty ("prepare_request_id", prepare.requestId);
                        row->setProperty ("prepare_status", prepare.status);
                        if (prepare.message.isNotEmpty())
                            row->setProperty ("prepare_message", prepare.message);

                        ++prepareAttemptCount;
                        if (prepare.ok)
                            ++prepareOkCount;
                        else
                            ++prepareErrorCount;
                    }
                    else
                    {
                        prepareTruncated = true;
                        row->setProperty ("prepare_status", "skipped_prepare_limit");
                    }
                }

                references.add (rowVar);
                if (references.size() >= maxAssets)
                {
                    reachedLimit = true;
                    break;
                }
            }

            if (reachedLimit)
                break;
        }
    }

    payload->setProperty ("status", "ok");
    payload->setProperty ("manifest_id", generatedId ("asset_manifest_"));
    payload->setProperty ("kind", kind);
    payload->setProperty ("references", juce::var (references));
    payload->setProperty ("assets", juce::var (references));
    payload->setProperty ("asset_count", references.size());
    payload->setProperty ("truncated", reachedLimit);
    payload->setProperty ("max_assets", maxAssets);
    payload->setProperty ("prepare_requested", prepareWaveform);
    payload->setProperty ("prepare_count", prepareAttemptCount);
    payload->setProperty ("prepare_ok_count", prepareOkCount);
    payload->setProperty ("prepare_error_count", prepareErrorCount);
    payload->setProperty ("prepare_limit", prepareLimit);
    payload->setProperty ("prepare_truncated", prepareTruncated);
    payload->setProperty ("platform_neutral", true);
    payload->setProperty ("inline_payload", false);
    payload->setProperty ("no_big_json_payload", true);

    if (! requestedTrackIds.isEmpty())
        payload->setProperty ("track_ids", juce::var (toVarArray (requestedTrackIds)));
    if (! requestedClipIds.isEmpty())
        payload->setProperty ("clip_ids", juce::var (toVarArray (requestedClipIds)));

    copyPropertyIfPresent (requestPayload, *payload, "priority");
    copyPropertyIfPresent (requestPayload, *payload, "visible_range");
    return juce::var (payload.release());
}

juce::String handleAssetManifestRequest (const juce::DynamicObject& request,
                                         const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "asset", "asset.error", "validation_error", "asset.manifest_request requires payload object");

    const auto kind = normaliseAssetKind (firstStringProperty (*payload, { "kind", "asset_kind", "asset_type" }));
    if (kind.isEmpty())
        return makeVspError (request, "asset", "asset.error", "validation_error", "asset.manifest_request requires a supported asset kind");

    auto snapshot = captureProjectStateSnapshot (request, legacyHandler, "project.timeline");
    if (! snapshot.valid)
        return makeVspError (request,
                             "asset",
                             "asset.error",
                             snapshot.errorCode.isNotEmpty() ? snapshot.errorCode : "legacy_adapter_error",
                             snapshot.errorMessage.isNotEmpty() ? snapshot.errorMessage : "Unable to capture project state for asset manifest");

    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.asset.manifest.v1", "asset", "asset.manifest");
    response->setProperty ("project_epoch", snapshot.projectEpoch);
    response->setProperty ("revision", static_cast<juce::int64> (snapshot.revision));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", "Asset manifest resolved");
    response->setProperty ("ack", juce::var (ack.release()));
    response->setProperty ("payload", makeAssetManifestPayload (snapshot, *payload, kind, legacyHandler));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String handleAssetMessage (const juce::DynamicObject& request,
                                 const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    const auto type = trimStringProperty (request, "type");
    if (type.equalsIgnoreCase ("asset.manifest_request") || type.equalsIgnoreCase ("asset.manifest.request"))
        return handleAssetManifestRequest (request, legacyHandler);

    if (! type.equalsIgnoreCase ("asset.request"))
        return makeVspError (request, "asset", "asset.error", "capability_not_supported", "Unsupported asset message type: " + type);

    auto* payload = request.getProperty ("payload").getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "asset", "asset.error", "validation_error", "asset.request requires payload object");

    const auto clipId = firstStringProperty (*payload, { "clip_id", "owner", "owner_id" });
    if (clipId.isEmpty())
        return makeVspError (request, "asset", "asset.error", "validation_error", "asset.request requires clip_id");

    const auto kind = normaliseAssetKind (firstStringProperty (*payload, { "kind", "asset_kind", "asset_type" }));
    if (kind.isEmpty())
        return makeVspError (request, "asset", "asset.error", "validation_error", "asset.request requires a supported asset kind");

    auto snapshot = captureProjectStateSnapshot (request, legacyHandler, "project.timeline");
    if (! snapshot.valid)
        return makeVspError (request,
                             "asset",
                             "asset.error",
                             snapshot.errorCode.isNotEmpty() ? snapshot.errorCode : "legacy_adapter_error",
                             snapshot.errorMessage.isNotEmpty() ? snapshot.errorMessage : "Unable to capture project state for asset reference");

    juce::String trackId;
    const auto clipVar = findClipInSnapshot (snapshot.snapshot, clipId, trackId);
    auto* clip = clipVar.getDynamicObject();
    if (clip == nullptr)
        return makeVspError (request, "asset", "asset.error", "not_found", "Clip not found for asset reference: " + clipId);

    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.asset.reference.v1", "asset", "asset.reference");
    response->setProperty ("project_epoch", snapshot.projectEpoch);
    response->setProperty ("revision", static_cast<juce::int64> (snapshot.revision));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", "Asset reference resolved");
    response->setProperty ("ack", juce::var (ack.release()));
    response->setProperty ("payload", makeAssetReferencePayload (snapshot, *payload, *clip, clipId, trackId, kind));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::StringArray defaultEventTopics()
{
    return { "import.audio", "asset.bake", "render" };
}

juce::StringArray eventTopicsFromPayload (const juce::DynamicObject* payload)
{
    if (payload == nullptr)
        return defaultEventTopics();

    auto topics = stringArrayFromVar (payload->getProperty ("topics"));
    if (topics.isEmpty())
        addUnique (topics, trimStringProperty (*payload, "topic"));

    return topics.isEmpty() ? defaultEventTopics() : topics;
}

void putEventSubscription (const EventSubscription& subscription)
{
    const juce::ScopedLock sl (phase4StoreLock());
    phase4Store().eventSubscriptions[subscription.subscriptionId.toStdString()] = subscription;
}

bool getEventSubscription (const juce::String& subscriptionId, EventSubscription& subscription)
{
    const juce::ScopedLock sl (phase4StoreLock());
    const auto found = phase4Store().eventSubscriptions.find (subscriptionId.toStdString());

    if (found == phase4Store().eventSubscriptions.end())
        return false;

    subscription = found->second;
    return true;
}

int64_t nextEventSequence (const juce::String& subscriptionId)
{
    const juce::ScopedLock sl (phase4StoreLock());
    ++phase4Store().eventSequence;

    const auto found = phase4Store().eventSubscriptions.find (subscriptionId.toStdString());
    if (found == phase4Store().eventSubscriptions.end())
        return phase4Store().eventSequence;

    found->second.sequence += 1;
    return found->second.sequence;
}

bool topicsIncludeImportOrBake (const juce::StringArray& topics)
{
    for (const auto& topic : topics)
    {
        const auto lower = topic.toLowerCase();
        if (lower == "import.audio" || lower == "asset.bake" || lower.contains ("import") || lower.contains ("bake"))
            return true;
    }

    return false;
}

juce::var callAudioAnalysisStatus (const juce::DynamicObject& request,
                                   const VspKernelReference::LegacyCommandHandler& legacyHandler,
                                   const juce::String& jobId)
{
    juce::ignoreUnused (request);

    if (! legacyHandler)
        return {};

    auto legacy = std::make_unique<juce::DynamicObject>();
    legacy->setProperty ("cmd", "project.audio_analysis_status");
    legacy->setProperty ("latest", true);
    legacy->setProperty ("vsp_adapter", "phase4_event_channel");

    if (jobId.isNotEmpty())
    {
        legacy->setProperty ("job_id", jobId);
        legacy->setProperty ("analysis_job_id", jobId);
    }

    auto legacyVar = juce::var (legacy.release());
    const auto legacyRaw = juce::JSON::toString (legacyVar);
    return juce::JSON::parse (legacyHandler (legacyVar, legacyRaw));
}

juce::String makeEventNotificationResponse (const juce::DynamicObject& request,
                                            const juce::String& subscriptionId,
                                            int64_t sequence,
                                            const juce::StringArray& topics,
                                            const juce::String& status,
                                            const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.event.notification.v1", "event", "event.notification");

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", status == "no_op" ? "no_op" : "completed");
    ack->setProperty ("message", message);
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", status);
    payload->setProperty ("event_id", generatedId ("evt_notice_"));
    payload->setProperty ("subscription_id", subscriptionId);
    payload->setProperty ("sequence", static_cast<juce::int64> (sequence));
    payload->setProperty ("topics", makeStringArrayVar (topics));
    payload->setProperty ("message", message);
    payload->setProperty ("terminal", false);
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String makeEventProgressResponse (const juce::DynamicObject& request,
                                        const juce::String& subscriptionId,
                                        int64_t sequence,
                                        const juce::DynamicObject& analysisJob)
{
    const auto status = firstStringProperty (analysisJob, { "status", "analysis_queue_status" });
    const auto jobId = firstStringProperty (analysisJob, { "job_id", "analysis_job_id" });
    const auto progress = juce::jlimit (0.0, 1.0, doubleFromVar (analysisJob.getProperty ("progress"), 0.0));
    const auto terminal = status == "submitted" || status == "completed" || status == "cancelled" || status == "failed";

    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.event.progress.v1", "event", "event.progress");

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", "Job progress sampled");
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", "ok");
    payload->setProperty ("event_id", generatedId ("evt_import_"));
    payload->setProperty ("subscription_id", subscriptionId);
    payload->setProperty ("job_id", jobId.isNotEmpty() ? jobId : "job_unknown");
    payload->setProperty ("sequence", static_cast<juce::int64> (sequence));
    payload->setProperty ("topic", "import.audio");
    payload->setProperty ("progress", progress);
    payload->setProperty ("message", status.isNotEmpty() ? "Audio analysis " + status : "Audio analysis progress");
    payload->setProperty ("terminal", terminal);
    payload->setProperty ("legacy_status", status);

    auto analysisJobCopy = std::make_unique<juce::DynamicObject>();
    copyProperties (analysisJob, *analysisJobCopy);
    payload->setProperty ("analysis_job", juce::var (analysisJobCopy.release()));
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String handleEventSubscribe (const juce::DynamicObject& request)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    const auto topics = eventTopicsFromPayload (payload);

    EventSubscription subscription;
    subscription.sessionId = trimStringProperty (request, "session_id");
    subscription.subscriptionId = payload != nullptr ? firstStringProperty (*payload, { "subscription_id", "id" }) : juce::String();
    if (subscription.subscriptionId.isEmpty())
        subscription.subscriptionId = generatedId ("sub_evt_");
    subscription.topics = topics;
    subscription.sequence = 0;
    putEventSubscription (subscription);

    return makeEventNotificationResponse (request,
                                          subscription.subscriptionId,
                                          nextEventSequence (subscription.subscriptionId),
                                          topics,
                                          "subscribed",
                                          "Event subscription accepted");
}

juce::String handleEventPoll (const juce::DynamicObject& request,
                              const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();
    const auto subscriptionId = payload != nullptr ? firstStringProperty (*payload, { "subscription_id", "id" }) : juce::String();
    EventSubscription subscription;
    juce::StringArray topics = eventTopicsFromPayload (payload);

    if (subscriptionId.isNotEmpty() && getEventSubscription (subscriptionId, subscription))
        topics = subscription.topics;

    const auto sequence = nextEventSequence (subscriptionId);

    if (topicsIncludeImportOrBake (topics))
    {
        const auto jobId = payload != nullptr ? firstStringProperty (*payload, { "job_id", "analysis_job_id" }) : juce::String();
        const auto status = callAudioAnalysisStatus (request, legacyHandler, jobId);
        if (auto* statusObject = status.getDynamicObject())
        {
            const auto analysisJobVar = statusObject->getProperty ("analysis_job");
            auto* analysisJob = analysisJobVar.getDynamicObject();
            if (analysisJob != nullptr && ! trimStringProperty (*analysisJob, "job_id").isEmpty())
                return makeEventProgressResponse (request, subscriptionId, sequence, *analysisJob);
        }
    }

    return makeEventNotificationResponse (request,
                                          subscriptionId,
                                          sequence,
                                          topics,
                                          "no_op",
                                          "No active job progress is available");
}

juce::String handleEventMessage (const juce::DynamicObject& request,
                                 const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    const auto type = trimStringProperty (request, "type");

    if (type.equalsIgnoreCase ("event.subscribe"))
        return handleEventSubscribe (request);

    if (type.equalsIgnoreCase ("event.poll") || type.equalsIgnoreCase ("event.progress_request"))
        return handleEventPoll (request, legacyHandler);

    return makeVspError (request, "event", "event.error", "capability_not_supported", "Unsupported event message type: " + type);
}

void addSnapshotPayloadFields (juce::DynamicObject& payload, const ProjectStateSnapshot& snapshot)
{
    payload.setProperty ("scope", snapshot.scope);
    payload.setProperty ("snapshot_hash", snapshot.hash);
    payload.setProperty ("snapshot", snapshot.snapshot);

    if (auto* snapshotObject = snapshot.snapshot.getDynamicObject())
    {
        payload.setProperty ("project", snapshotObject->getProperty ("project"));
        payload.setProperty ("tracks", snapshotObject->getProperty ("tracks"));
    }
}

juce::String makeStateSnapshotResponse (const juce::DynamicObject& request,
                                        const ProjectStateSnapshot& snapshot,
                                        bool isResync,
                                        const juce::String& subscriptionId = {})
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.state.snapshot.v1", "state", "state.snapshot");
    response->setProperty ("project_epoch", snapshot.projectEpoch);
    response->setProperty ("revision", static_cast<juce::int64> (snapshot.revision));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "completed");
    ack->setProperty ("message", isResync ? "State resynchronised" : "State snapshot captured");
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", "ok");
    payload->setProperty ("resync", isResync);

    if (subscriptionId.isNotEmpty())
        payload->setProperty ("subscription_id", subscriptionId);

    addSnapshotPayloadFields (*payload, snapshot);
    response->setProperty ("payload", juce::var (payload.release()));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String makeStateDeltaResponse (const juce::DynamicObject& request,
                                     const ProjectStateSnapshot& current,
                                     int64_t baseRevision,
                                     const juce::Array<juce::var>& ops,
                                     const juce::StringArray& changedTracks,
                                     const juce::StringArray& changedClips)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.state.delta.v1", "state", "state.delta");
    response->setProperty ("project_epoch", current.projectEpoch);
    response->setProperty ("base_revision", static_cast<juce::int64> (baseRevision));
    response->setProperty ("revision", static_cast<juce::int64> (current.revision));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", ops.isEmpty() ? "no_op" : "completed");
    ack->setProperty ("message", ops.isEmpty() ? "No state changes since base_revision" : "State delta captured");
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", "ok");
    payload->setProperty ("scope", current.scope);
    payload->setProperty ("snapshot_hash", current.hash);
    payload->setProperty ("ops", juce::var (ops));
    payload->setProperty ("changed_tracks", juce::var (toVarArray (changedTracks)));
    payload->setProperty ("changed_clips", juce::var (toVarArray (changedClips)));
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String makeStateResyncRequired (const juce::DynamicObject& request,
                                      const ProjectStateSnapshot& current,
                                      int64_t requestedBaseRevision,
                                      int64_t expectedBaseRevision,
                                      const juce::String& reason)
{
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response, request, "vsp.state.resync_required.v1", "state", "state.resync_required");
    response->setProperty ("project_epoch", current.projectEpoch);
    response->setProperty ("base_revision", static_cast<juce::int64> (requestedBaseRevision));
    response->setProperty ("revision", static_cast<juce::int64> (current.revision));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", "rejected");
    ack->setProperty ("message", reason);
    response->setProperty ("ack", juce::var (ack.release()));

    auto error = std::make_unique<juce::DynamicObject>();
    error->setProperty ("code", "conflict");
    error->setProperty ("message", reason);
    error->setProperty ("retryable", false);
    response->setProperty ("error", juce::var (error.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("status", "resync_required");
    payload->setProperty ("resync_hint", true);
    payload->setProperty ("requested_base_revision", static_cast<juce::int64> (requestedBaseRevision));
    payload->setProperty ("expected_base_revision", static_cast<juce::int64> (expectedBaseRevision));
    payload->setProperty ("current_revision", static_cast<juce::int64> (current.revision));
    payload->setProperty ("snapshot_hash", current.hash);
    response->setProperty ("payload", juce::var (payload.release()));

    return juce::JSON::toString (juce::var (response.release()));
}

bool isLegacyCommandWriteLike (const juce::String& legacyCommand)
{
    return legacyReadOnlyCommands().find (legacyCommand.toStdString()) == legacyReadOnlyCommands().end();
}

juce::String transactionIdForRequest (const juce::DynamicObject& request,
                                      const juce::String& requestId,
                                      const juce::String& legacyCommand,
                                      bool writeLike)
{
    const auto explicitTransactionId = trimStringProperty (request, "transaction_id");
    if (explicitTransactionId.isNotEmpty())
        return explicitTransactionId;

    if (! writeLike)
        return {};

    if (requestId.isNotEmpty())
        return "tx_" + requestId;

    if (legacyCommand.isNotEmpty())
        return generatedId (juce::String ("tx_") + legacyCommand + "_");

    return generatedId ("tx_vsp_");
}

juce::String makeVspCommandResponse (const juce::DynamicObject& request,
                                     const juce::String& vspCommand,
                                     const juce::String& legacyCommand,
                                     bool writeLike,
                                     const juce::String& legacyReplyRaw)
{
    const auto parsedReply = juce::JSON::parse (legacyReplyRaw);
    auto* replyObject = parsedReply.getDynamicObject();

    if (replyObject == nullptr)
        return makeVspError (request,
                             "command",
                             "command.error",
                             "legacy_adapter_error",
                             "Legacy command returned non-object JSON");

    const auto legacyStatus = trimStringProperty (*replyObject, "status");
    const auto legacyMessage = trimStringProperty (*replyObject, "message");
    const bool isLegacyError = legacyStatus.equalsIgnoreCase ("error");

    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response,
                             request,
                             isLegacyError ? "vsp.command.error.v1" : "vsp.command.response.v1",
                             "command",
                             isLegacyError ? "command.error" : "command.response");

    const auto requestId = trimStringProperty (request, "request_id");
    const auto transactionId = transactionIdForRequest (request, requestId, legacyCommand, writeLike);
    if (transactionId.isNotEmpty())
        response->setProperty ("transaction_id", transactionId);

    if (replyObject->hasProperty ("revision"))
        response->setProperty ("revision", replyObject->getProperty ("revision"));

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", isLegacyError ? "rejected" : "completed");
    ack->setProperty ("message", legacyMessage.isNotEmpty() ? legacyMessage
                                                            : (isLegacyError ? "Legacy command failed"
                                                                             : "Legacy command completed"));
    response->setProperty ("ack", juce::var (ack.release()));

    auto payload = std::make_unique<juce::DynamicObject>();
    payload->setProperty ("command", vspCommand);
    payload->setProperty ("legacy_command", legacyCommand);
    payload->setProperty ("legacy_status", legacyStatus);
    payload->setProperty ("legacy_reply", parsedReply);

    if (writeLike && ! replyObject->hasProperty ("revision"))
    {
        payload->setProperty ("revision_status", "legacy_unknown");
        payload->setProperty ("resync_hint", true);
    }

    response->setProperty ("payload", juce::var (payload.release()));

    if (isLegacyError)
    {
        auto error = std::make_unique<juce::DynamicObject>();
        error->setProperty ("code", errorCodeFromLegacyMessage (legacyMessage));
        error->setProperty ("message", legacyMessage.isNotEmpty() ? legacyMessage : "Legacy command failed");
        error->setProperty ("retryable", false);
        response->setProperty ("error", juce::var (error.release()));
    }

    return juce::JSON::toString (juce::var (response.release()));
}

juce::var buildLegacyCommandFromArgs (const juce::String& legacyCommand,
                                      const juce::DynamicObject* args,
                                      const juce::DynamicObject& request)
{
    auto legacy = std::make_unique<juce::DynamicObject>();

    if (args != nullptr)
        copyProperties (*args, *legacy);

    legacy->setProperty ("cmd", legacyCommand);

    const auto requestId = trimStringProperty (request, "request_id");
    if (requestId.isNotEmpty())
        legacy->setProperty ("request_id", requestId);

    const auto transactionId = trimStringProperty (request, "transaction_id");
    if (transactionId.isNotEmpty())
        legacy->setProperty ("transaction_id", transactionId);

    legacy->setProperty ("vsp_adapter", "phase2_kernel_reference");
    return juce::var (legacy.release());
}

juce::String validateCommandBaseRevision (const juce::DynamicObject& request,
                                          const juce::DynamicObject* args,
                                          const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    int64_t requestedRevision = 0;
    bool hasRevision = false;

    if (request.hasProperty ("base_revision"))
    {
        requestedRevision = int64Property (request, "base_revision");
        hasRevision = true;
    }
    else if (args != nullptr && args->hasProperty ("base_revision"))
    {
        requestedRevision = int64Property (*args, "base_revision");
        hasRevision = true;
    }

    if (! hasRevision)
        return {};

    if (requestedRevision <= 0)
        return makeVspError (request,
                             "command",
                             "command.error",
                             "validation_error",
                             "base_revision must be positive for a mutation command");

    const auto current = captureProjectStateSnapshot (request, legacyHandler, "project.timeline");
    if (! current.valid)
        return makeVspError (request,
                             "command",
                             "command.error",
                             "snapshot_unavailable",
                             current.errorMessage.isNotEmpty() ? current.errorMessage : "Unable to validate project revision before mutation");

    if (requestedRevision != current.revision)
    {
        return makeVspError (request,
                             "command",
                             "command.error",
                             "stale_project_cut",
                             "Mutation rejected: base_revision does not match the current project snapshot");
    }

    return {};
}

juce::String dispatchLegacyCommand (const juce::DynamicObject& request,
                                    const juce::String& vspCommand,
                                    const juce::String& legacyCommand,
                                    const juce::DynamicObject* args,
                                    bool writeLike,
                                    const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    if (! legacyHandler)
        return makeVspError (request, "command", "command.error", "internal_error", "Legacy command handler unavailable");

    if (writeLike)
    {
        const auto preconditionError = validateCommandBaseRevision (request, args, legacyHandler);
        if (preconditionError.isNotEmpty())
            return preconditionError;
    }

    auto legacyCommandVar = buildLegacyCommandFromArgs (legacyCommand, args, request);
    const auto legacyRaw = juce::JSON::toString (legacyCommandVar);
    const auto legacyReply = legacyHandler (legacyCommandVar, legacyRaw);
    return makeVspCommandResponse (request, vspCommand, legacyCommand, writeLike, legacyReply);
}

juce::String dispatchCommandPayload (const juce::DynamicObject& request,
                                     const juce::DynamicObject& payload,
                                     const VspKernelReference::LegacyCommandHandler& legacyHandler);

bool isNumericVar (const juce::var& value)
{
    return value.isDouble() || value.isInt() || value.isInt64();
}

juce::String commandNameFromPayload (const juce::DynamicObject& payload)
{
    const auto command = trimStringProperty (payload, "command");
    if (command.isNotEmpty())
        return command;

    if (auto* legacyPayload = payload.getProperty ("legacy").getDynamicObject())
        return "legacy:" + trimStringProperty (*legacyPayload, "cmd");

    return {};
}

bool commandResponseFailed (const juce::DynamicObject& response)
{
    if (trimStringProperty (response, "type").equalsIgnoreCase ("command.error"))
        return true;

    if (response.hasProperty ("error"))
        return true;

    if (auto* ack = response.getProperty ("ack").getDynamicObject())
        return trimStringProperty (*ack, "stage").equalsIgnoreCase ("rejected");

    return false;
}

bool commandPayloadWriteLike (const juce::DynamicObject& payload)
{
    const auto command = trimStringProperty (payload, "command");
    if (command == "command.batch" || command == "plugin.set_params_batch")
        return true;

    if (command == "legacy.command")
    {
        if (auto* legacy = payload.getProperty ("legacy").getDynamicObject())
            return isLegacyCommandWriteLike (trimStringProperty (*legacy, "cmd"));
        return false;
    }

    const auto found = canonicalCommandMap().find (command.toStdString());
    return found != canonicalCommandMap().end() && found->second.writesProjectOrTransport;
}

juce::String commandReceiptCacheKey (const juce::DynamicObject& request)
{
    const auto requestId = trimStringProperty (request, "request_id");
    if (requestId.isEmpty())
        return {};

    const auto sessionId = trimStringProperty (request, "session_id");
    return sessionId + "|" + requestId;
}

juce::String cachedCommandReceipt (const juce::DynamicObject& request)
{
    const auto key = commandReceiptCacheKey (request);
    if (key.isEmpty())
        return {};

    const juce::ScopedLock sl (stateStoreLock());
    const auto found = stateStore().commandReceipts.find (key.toStdString());
    return found == stateStore().commandReceipts.end() ? juce::String() : found->second;
}

void rememberCommandReceipt (const juce::DynamicObject& request, const juce::String& response)
{
    const auto key = commandReceiptCacheKey (request);
    if (key.isEmpty() || response.isEmpty())
        return;

    const juce::ScopedLock sl (stateStoreLock());
    auto& receipts = stateStore().commandReceipts;
    receipts[key.toStdString()] = response;
    constexpr size_t maxReceipts = 512;
    while (receipts.size() > maxReceipts)
        receipts.erase (receipts.begin());
}

juce::String makeBatchTransactionId (const juce::DynamicObject& request, const juce::String& commandName)
{
    const auto requestId = trimStringProperty (request, "request_id");
    return transactionIdForRequest (request, requestId, commandName, true);
}

juce::String handlePluginSetParamsBatch (const juce::DynamicObject& request,
                                         const juce::DynamicObject& payload,
                                         const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    if (! legacyHandler)
        return makeVspError (request, "command", "command.error", "internal_error", "Legacy command handler unavailable");

    auto* args = payload.getProperty ("args").getDynamicObject();
    if (args == nullptr)
        return makeVspError (request, "command", "command.error", "validation_error", "plugin.set_params_batch requires payload.args");

    if (const auto preconditionError = validateCommandBaseRevision (request, args, legacyHandler); preconditionError.isNotEmpty())
        return preconditionError;

    const auto pluginId = trimStringProperty (*args, "plugin_id");
    if (pluginId.isEmpty())
        return makeVspError (request, "command", "command.error", "validation_error", "plugin.set_params_batch requires plugin_id");

    auto* parameters = args->getProperty ("parameters").getArray();
    if (parameters == nullptr || parameters->isEmpty())
        return makeVspError (request, "command", "command.error", "validation_error", "plugin.set_params_batch requires non-empty parameters array");

    juce::Array<juce::var> results;
    juce::Array<juce::var> changedParams;
    juce::String failedMessage;
    int failedIndex = -1;

    for (int i = 0; i < parameters->size(); ++i)
    {
        auto* parameter = parameters->getReference (i).getDynamicObject();
        if (parameter == nullptr)
        {
            failedIndex = i;
            failedMessage = "plugin.set_params_batch parameter must be an object";
            break;
        }

        const auto paramId = firstStringProperty (*parameter, { "parameter_id", "param_id", "id" });
        if (paramId.isEmpty())
        {
            failedIndex = i;
            failedMessage = "plugin.set_params_batch parameter requires parameter_id";
            break;
        }

        const auto normalisedValue = parameter->hasProperty ("normalized_value") ? parameter->getProperty ("normalized_value")
                                      : parameter->hasProperty ("normalised_value") ? parameter->getProperty ("normalised_value")
                                      : juce::var();
        const auto value = parameter->getProperty ("value");
        const auto valueText = firstStringProperty (*parameter, { "value_text", "display_value_text", "target_text", "text" });

        if (! isNumericVar (normalisedValue) && ! isNumericVar (value) && valueText.isEmpty())
        {
            failedIndex = i;
            failedMessage = "plugin.set_params_batch parameter requires normalized_value, value, or value_text";
            break;
        }

        auto legacyArgs = std::make_unique<juce::DynamicObject>();
        legacyArgs->setProperty ("plugin_id", pluginId);
        legacyArgs->setProperty ("param_id", paramId);
        copyPropertyIfPresent (*args, *legacyArgs, "track_id");
        copyPropertyIfPresent (*parameter, *legacyArgs, "unit");
        copyPropertyIfPresent (*parameter, *legacyArgs, "value_text");
        copyPropertyIfPresent (*parameter, *legacyArgs, "display_value_text");
        copyPropertyIfPresent (*parameter, *legacyArgs, "target_text");
        copyPropertyIfPresent (*parameter, *legacyArgs, "text");

        if (isNumericVar (normalisedValue))
            legacyArgs->setProperty ("normalized_value", normalisedValue);
        else if (isNumericVar (value))
            legacyArgs->setProperty ("value", value);

        auto legacyCommandVar = buildLegacyCommandFromArgs ("set_plugin_param", legacyArgs.get(), request);
        const auto legacyRaw = juce::JSON::toString (legacyCommandVar);
        const auto legacyReplyRaw = legacyHandler (legacyCommandVar, legacyRaw);
        const auto legacyReply = juce::JSON::parse (legacyReplyRaw);
        auto* legacyReplyObject = legacyReply.getDynamicObject();

        auto result = std::make_unique<juce::DynamicObject>();
        result->setProperty ("index", i);
        result->setProperty ("parameter_id", paramId);
        result->setProperty ("legacy_command", "set_plugin_param");
        result->setProperty ("legacy_reply", legacyReply);

        if (legacyReplyObject == nullptr)
        {
            result->setProperty ("status", "error");
            result->setProperty ("message", "set_plugin_param returned non-object JSON");
            results.add (juce::var (result.release()));
            failedIndex = i;
            failedMessage = "set_plugin_param returned non-object JSON";
            break;
        }

        const auto status = trimStringProperty (*legacyReplyObject, "status");
        const auto message = trimStringProperty (*legacyReplyObject, "message");
        result->setProperty ("status", status.isNotEmpty() ? status : "ok");

        if (message.isNotEmpty())
            result->setProperty ("message", message);

        if (status.equalsIgnoreCase ("error"))
        {
            results.add (juce::var (result.release()));
            failedIndex = i;
            failedMessage = message.isNotEmpty() ? message : "set_plugin_param failed";
            break;
        }

        auto changed = std::make_unique<juce::DynamicObject>();
        changed->setProperty ("parameter_id", paramId);
        changed->setProperty ("param_id", paramId);
        changed->setProperty ("status", "ok");
        copyPropertyIfPresent (*legacyReplyObject, *changed, "new_value", "actual_value");
        copyPropertyIfPresent (*legacyReplyObject, *changed, "new_normalised_value", "actual_normalised_value");
        copyPropertyIfPresent (*legacyReplyObject, *changed, "actual_normalized_value");
        copyPropertyIfPresent (*legacyReplyObject, *changed, "new_value_text", "display_text");
        copyPropertyIfPresent (*legacyReplyObject, *changed, "value_interpretation");
        changedParams.add (juce::var (changed.release()));
        results.add (juce::var (result.release()));
    }

    const auto failed = failedIndex >= 0;
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response,
                             request,
                             failed ? "vsp.command.error.v1" : "vsp.command.response.v1",
                             "command",
                             failed ? "command.error" : "command.response");

    const auto transactionId = makeBatchTransactionId (request, "plugin.set_params_batch");
    if (transactionId.isNotEmpty())
        response->setProperty ("transaction_id", transactionId);

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", failed ? "rejected" : "completed");
    ack->setProperty ("message", failed ? failedMessage : "Plugin parameter batch applied");
    response->setProperty ("ack", juce::var (ack.release()));

    auto responsePayload = std::make_unique<juce::DynamicObject>();
    responsePayload->setProperty ("command", "plugin.set_params_batch");
    responsePayload->setProperty ("plugin_id", pluginId);
    responsePayload->setProperty ("status", failed ? "partial_failure" : "ok");
    responsePayload->setProperty ("parameter_count", parameters->size());
    responsePayload->setProperty ("changed_params", juce::var (changedParams));
    responsePayload->setProperty ("results", juce::var (results));

    const auto trackId = firstStringProperty (*args, { "track_id", "track" });
    if (! failed && boolFromVar (args->getProperty ("readback"), false))
    {
        if (trackId.isEmpty())
        {
            responsePayload->setProperty ("readback_status", "track_id_required");
            responsePayload->setProperty ("readback_message", "get_plugin_parameters requires track_id for readback");
        }
        else
        {
            auto readbackArgs = std::make_unique<juce::DynamicObject>();
            readbackArgs->setProperty ("track_id", trackId);
            readbackArgs->setProperty ("plugin_id", pluginId);
            auto legacyReadbackVar = buildLegacyCommandFromArgs ("get_plugin_parameters", readbackArgs.get(), request);
            const auto legacyReadbackRaw = juce::JSON::toString (legacyReadbackVar);
            responsePayload->setProperty ("readback", juce::JSON::parse (legacyHandler (legacyReadbackVar, legacyReadbackRaw)));
        }
    }

    response->setProperty ("payload", juce::var (responsePayload.release()));

    if (failed)
    {
        auto error = std::make_unique<juce::DynamicObject>();
        error->setProperty ("code", "partial_failure");
        error->setProperty ("message", failedMessage);
        error->setProperty ("retryable", false);

        auto details = std::make_unique<juce::DynamicObject>();
        details->setProperty ("failed_index", failedIndex);
        details->setProperty ("plugin_id", pluginId);
        error->setProperty ("details", juce::var (details.release()));
        response->setProperty ("error", juce::var (error.release()));
    }

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String handleCommandBatch (const juce::DynamicObject& request,
                                 const juce::DynamicObject& payload,
                                 const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    auto* args = payload.getProperty ("args").getDynamicObject();
    const auto commandsVar = args != nullptr && args->hasProperty ("commands")
                                 ? args->getProperty ("commands")
                                 : payload.getProperty ("commands");
    auto* commands = commandsVar.getArray();

    if (commands == nullptr || commands->isEmpty())
        return makeVspError (request, "command", "command.error", "validation_error", "command.batch requires non-empty commands array");

    const auto continueOnError = args != nullptr && boolFromVar (args->getProperty ("continue_on_error"), false);
    juce::Array<juce::var> results;
    int completedCount = 0;
    int failedIndex = -1;
    juce::String failedMessage;

    for (int i = 0; i < commands->size(); ++i)
    {
        auto* commandObject = commands->getReference (i).getDynamicObject();
        if (commandObject == nullptr)
        {
            failedIndex = i;
            failedMessage = "command.batch child command must be an object";
            break;
        }

        auto* childPayload = commandObject->getProperty ("payload").getDynamicObject();
        if (childPayload == nullptr)
            childPayload = commandObject;

        const auto childCommandName = commandNameFromPayload (*childPayload);
        const auto childRaw = dispatchCommandPayload (request, *childPayload, legacyHandler);
        const auto childReply = juce::JSON::parse (childRaw);
        auto* childReplyObject = childReply.getDynamicObject();
        const auto childFailed = childReplyObject == nullptr || commandResponseFailed (*childReplyObject);

        auto result = std::make_unique<juce::DynamicObject>();
        result->setProperty ("index", i);
        result->setProperty ("command", childCommandName);
        result->setProperty ("status", childFailed ? "error" : "ok");
        result->setProperty ("response", childReply);
        results.add (juce::var (result.release()));

        if (childFailed)
        {
            failedIndex = i;
            if (childReplyObject != nullptr)
            {
                if (auto* error = childReplyObject->getProperty ("error").getDynamicObject())
                    failedMessage = trimStringProperty (*error, "message");
                if (failedMessage.isEmpty() && childReplyObject->getProperty ("ack").isObject())
                    failedMessage = trimStringProperty (*childReplyObject->getProperty ("ack").getDynamicObject(), "message");
            }

            if (failedMessage.isEmpty())
                failedMessage = "command.batch child command failed";

            if (! continueOnError)
                break;
        }
        else
        {
            ++completedCount;
        }
    }

    const auto failed = failedIndex >= 0;
    auto response = std::make_unique<juce::DynamicObject>();
    setCommonEnvelopeFields (*response,
                             request,
                             failed ? "vsp.command.error.v1" : "vsp.command.response.v1",
                             "command",
                             failed ? "command.error" : "command.response");

    const auto transactionId = makeBatchTransactionId (request, "command.batch");
    if (transactionId.isNotEmpty())
        response->setProperty ("transaction_id", transactionId);

    auto ack = std::make_unique<juce::DynamicObject>();
    ack->setProperty ("stage", failed ? "rejected" : "completed");
    ack->setProperty ("message", failed ? failedMessage : "Command batch completed");
    response->setProperty ("ack", juce::var (ack.release()));

    auto responsePayload = std::make_unique<juce::DynamicObject>();
    responsePayload->setProperty ("command", "command.batch");
    responsePayload->setProperty ("status", failed ? "partial_failure" : "ok");
    responsePayload->setProperty ("ordered", true);
    responsePayload->setProperty ("continue_on_error", continueOnError);
    responsePayload->setProperty ("command_count", commands->size());
    responsePayload->setProperty ("completed_count", completedCount);
    responsePayload->setProperty ("results", juce::var (results));
    if (failed)
        responsePayload->setProperty ("failed_index", failedIndex);
    response->setProperty ("payload", juce::var (responsePayload.release()));

    if (failed)
    {
        auto error = std::make_unique<juce::DynamicObject>();
        error->setProperty ("code", "partial_failure");
        error->setProperty ("message", failedMessage);
        error->setProperty ("retryable", false);
        response->setProperty ("error", juce::var (error.release()));
    }

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String dispatchCommandPayload (const juce::DynamicObject& request,
                                     const juce::DynamicObject& payload,
                                     const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    const auto vspCommand = trimStringProperty (payload, "command");
    if (vspCommand.isEmpty())
        return makeVspError (request, "command", "command.error", "validation_error", "command.request requires payload.command");

    if (vspCommand == "command.batch")
        return handleCommandBatch (request, payload, legacyHandler);

    if (vspCommand == "plugin.set_params_batch")
        return handlePluginSetParamsBatch (request, payload, legacyHandler);

    if (vspCommand == "legacy.command")
    {
        auto* legacyPayload = payload.getProperty ("legacy").getDynamicObject();
        if (legacyPayload == nullptr)
            return makeVspError (request, "command", "command.error", "validation_error", "legacy.command requires payload.legacy");

        const auto legacyCommand = trimStringProperty (*legacyPayload, "cmd");
        if (legacyCommand.isEmpty())
            return makeVspError (request, "command", "command.error", "validation_error", "legacy.command requires legacy.cmd");

        auto* args = legacyPayload->getProperty ("args").getDynamicObject();
        return dispatchLegacyCommand (request,
                                      vspCommand,
                                      legacyCommand,
                                      args != nullptr ? args : legacyPayload,
                                      isLegacyCommandWriteLike (legacyCommand),
                                      legacyHandler);
    }

    const auto& mappings = canonicalCommandMap();
    const auto mapping = mappings.find (vspCommand.toStdString());
    if (mapping == mappings.end())
        return makeVspError (request,
                             "command",
                             "command.error",
                             "capability_not_supported",
                             "Unsupported VSP command: " + vspCommand);

    auto* args = payload.getProperty ("args").getDynamicObject();
    return dispatchLegacyCommand (request,
                                  vspCommand,
                                  mapping->second.legacyCommand,
                                  args,
                                  mapping->second.writesProjectOrTransport,
                                  legacyHandler);
}

juce::String handleCommandRequest (const juce::DynamicObject& request,
                                   const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    if (! trimStringProperty (request, "type").equalsIgnoreCase ("command.request"))
        return makeVspError (request, "command", "command.error", "validation_error", "Unsupported command message type");

    const auto payloadVar = request.getProperty ("payload");
    auto* payload = payloadVar.getDynamicObject();
    if (payload == nullptr)
        return makeVspError (request, "command", "command.error", "validation_error", "command.request requires payload object");

    const auto writeLike = commandPayloadWriteLike (*payload);
    if (writeLike)
    {
        if (const auto cached = cachedCommandReceipt (request); cached.isNotEmpty())
            return cached;
    }

    const auto response = dispatchCommandPayload (request, *payload, legacyHandler);
    if (writeLike)
        rememberCommandReceipt (request, response);
    return response;
}

juce::String stateScopeForRequest (const juce::DynamicObject& request)
{
    auto* payload = request.getProperty ("payload").getDynamicObject();

    if (payload != nullptr)
    {
        const auto scope = firstStringProperty (*payload, { "scope", "clip_scope" });
        if (scope.isNotEmpty())
            return scope;
    }

    return "project.timeline";
}

bool hasBaseRevision (const juce::DynamicObject& request)
{
    if (request.hasProperty ("base_revision"))
        return true;

    auto* payload = request.getProperty ("payload").getDynamicObject();
    return payload != nullptr && payload->hasProperty ("base_revision");
}

int64_t baseRevisionForRequest (const juce::DynamicObject& request)
{
    if (request.hasProperty ("base_revision"))
        return int64Property (request, "base_revision");

    auto* payload = request.getProperty ("payload").getDynamicObject();
    return payload != nullptr ? int64Property (*payload, "base_revision") : 0;
}

juce::String handleStateMessage (const juce::DynamicObject& request,
                                 const VspKernelReference::LegacyCommandHandler& legacyHandler)
{
    const auto type = trimStringProperty (request, "type");
    const auto scope = stateScopeForRequest (request);

    if (type.equalsIgnoreCase ("state.snapshot_request")
        || type.equalsIgnoreCase ("state.subscribe")
        || type.equalsIgnoreCase ("state.resync_request"))
    {
        auto snapshot = captureProjectStateSnapshot (request, legacyHandler, scope);
        if (! snapshot.valid)
            return makeVspError (request,
                                 "state",
                                 "state.error",
                                 snapshot.errorCode.isNotEmpty() ? snapshot.errorCode : "legacy_adapter_error",
                                 snapshot.errorMessage.isNotEmpty() ? snapshot.errorMessage : "Unable to capture project state");

        const auto sessionId = trimStringProperty (request, "session_id");
        updateSessionCursor (sessionId.isNotEmpty() ? sessionId : "session_unknown", snapshot);

        const bool isResync = type.equalsIgnoreCase ("state.resync_request");
        const auto subscriptionId = type.equalsIgnoreCase ("state.subscribe") ? generatedId ("sub_state_") : juce::String();
        return makeStateSnapshotResponse (request, snapshot, isResync, subscriptionId);
    }

    if (type.equalsIgnoreCase ("state.delta_request"))
    {
        if (! hasBaseRevision (request))
            return makeVspError (request, "state", "state.error", "validation_error", "state.delta_request requires base_revision");

        const auto requestedBaseRevision = baseRevisionForRequest (request);
        const auto sessionId = trimStringProperty (request, "session_id");
        const auto cursor = getSessionCursor (sessionId.isNotEmpty() ? sessionId : "session_unknown");
        auto current = captureProjectStateSnapshot (request, legacyHandler, scope);

        if (! current.valid)
            return makeVspError (request,
                                 "state",
                                 "state.error",
                                 current.errorCode.isNotEmpty() ? current.errorCode : "legacy_adapter_error",
                                 current.errorMessage.isNotEmpty() ? current.errorMessage : "Unable to capture project state");

        if (! cursor.valid)
            return makeStateResyncRequired (request,
                                            current,
                                            requestedBaseRevision,
                                            0,
                                            "No state cursor for session; request state.resync_request");

        if (requestedBaseRevision != cursor.revision)
            return makeStateResyncRequired (request,
                                            current,
                                            requestedBaseRevision,
                                            cursor.revision,
                                            "base_revision does not match server cursor; request state.resync_request");

        juce::StringArray changedTracks;
        juce::StringArray changedClips;
        auto ops = buildStateDeltaOps (cursor.snapshot, current.snapshot, changedTracks, changedClips);
        updateSessionCursor (sessionId.isNotEmpty() ? sessionId : "session_unknown", current);
        return makeStateDeltaResponse (request, current, requestedBaseRevision, ops, changedTracks, changedClips);
    }

    return makeVspError (request, "state", "state.error", "capability_not_supported", "Unsupported state message type: " + type);
}

} // namespace

bool VspKernelReference::isVspEnvelope (const juce::DynamicObject& object)
{
    return object.hasProperty ("vsp_version")
        || (object.hasProperty ("channel") && object.hasProperty ("type") && object.hasProperty ("payload"));
}

juce::String VspKernelReference::dispatchEnvelope (const juce::DynamicObject& envelope,
                                                   const juce::String& rawPayload,
                                                   LegacyCommandHandler legacyHandler,
                                                   RealtimeDataProvider realtimeDataProvider)
{
    juce::ignoreUnused (rawPayload);

    const auto version = trimStringProperty (envelope, "vsp_version");
    if (version.isNotEmpty() && version != "1.0")
        return makeVspError (envelope, "session", "session.close", "capability_not_supported", "Unsupported VSP version: " + version);

    const auto channel = trimStringProperty (envelope, "channel");
    const auto type = trimStringProperty (envelope, "type");

    if (channel.equalsIgnoreCase ("session"))
    {
        if (type.equalsIgnoreCase ("session.hello"))
            return makeSessionHelloAck (envelope);

        if (type.equalsIgnoreCase ("session.heartbeat"))
        {
            auto response = std::make_unique<juce::DynamicObject>();
            setCommonEnvelopeFields (*response, envelope, "vsp.session.heartbeat.v1", "session", "session.heartbeat");

            auto ack = std::make_unique<juce::DynamicObject>();
            ack->setProperty ("stage", "completed");
            ack->setProperty ("message", "heartbeat");
            response->setProperty ("ack", juce::var (ack.release()));

            auto payload = std::make_unique<juce::DynamicObject>();
            payload->setProperty ("server_time", nowIso8601());
            response->setProperty ("payload", juce::var (payload.release()));

            return juce::JSON::toString (juce::var (response.release()));
        }

        return makeVspError (envelope, "session", "session.close", "validation_error", "Unsupported session message type");
    }

    if (channel.equalsIgnoreCase ("command"))
        return handleCommandRequest (envelope, legacyHandler);

    if (channel.equalsIgnoreCase ("state"))
        return handleStateMessage (envelope, legacyHandler);

    if (channel.equalsIgnoreCase ("realtime"))
        return handleRealtimeMessage (envelope, realtimeDataProvider);

    if (channel.equalsIgnoreCase ("asset"))
        return handleAssetMessage (envelope, legacyHandler);

    if (channel.equalsIgnoreCase ("event"))
        return handleEventMessage (envelope, legacyHandler);

    return makeVspError (envelope, channel.isNotEmpty() ? channel : "session", "session.close", "capability_not_supported", "Unsupported VSP channel: " + channel);
}

} // namespace vit
