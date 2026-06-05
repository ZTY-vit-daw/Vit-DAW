#include "VitClipRouteRegistry.h"

#include "VitGraphValidator.h"

#include <unordered_map>
#include <unordered_set>
#include <vector>

namespace vit
{

namespace
{

struct ClipDescriptor
{
    juce::String clipId;
    juce::String name;
    bool isMidi = false;
};

struct ParsedScope
{
    juce::String raw = "track";
    juce::String selectedClipId;
    bool isTrack = true;
    bool isClip = false;
    bool isDebugGlobal = false;
};

constexpr auto kVitInternalDefaultAudioSourcesProperty = "vit_internal_default_audio_sources";

std::vector<ClipDescriptor> collectTrackClips (te::Track& track)
{
    std::vector<ClipDescriptor> clips;
    const int n = track.getNumTrackItems();

    for (int i = 0; i < n; ++i)
    {
        auto* item = track.getTrackItem (i);
        auto* clip = dynamic_cast<te::Clip*> (item);

        if (clip == nullptr)
            continue;

        clips.push_back ({ clip->itemID.toString(), clip->getName(), clip->isMidi() });
    }

    return clips;
}

ParsedScope parseScope (const juce::String& requestedScope)
{
    ParsedScope scope;
    scope.raw = requestedScope.trim();

    if (scope.raw.isEmpty() || scope.raw.equalsIgnoreCase ("track"))
        return scope;

    if (scope.raw.equalsIgnoreCase ("debug_global"))
    {
        scope.isTrack = false;
        scope.isDebugGlobal = true;
        return scope;
    }

    if (scope.raw.startsWithIgnoreCase ("clip:"))
    {
        scope.isTrack = false;
        scope.isClip = true;
        scope.selectedClipId = scope.raw.fromFirstOccurrenceOf (":", false, false).trim();
        return scope;
    }

    scope.raw = "track";
    return scope;
}

juce::StringArray toClipIdsArray (const std::vector<ClipDescriptor>& clips)
{
    juce::StringArray ids;

    for (const auto& clip : clips)
        ids.add (clip.clipId);

    return ids;
}

juce::StringArray parseNodeClipIds (const juce::String& clipScope, const juce::StringArray& allClipIds)
{
    if (clipScope.equalsIgnoreCase ("track"))
        return allClipIds;

    if (clipScope.startsWithIgnoreCase ("clip:"))
    {
        juce::StringArray ids;
        ids.add (clipScope.fromFirstOccurrenceOf (":", false, false).trim());
        return ids;
    }

    return allClipIds;
}

juce::StringArray intersectClipSets (const juce::StringArray& a, const juce::StringArray& b)
{
    juce::StringArray result;

    for (const auto& item : a)
        if (b.contains (item) && ! result.contains (item))
            result.add (item);

    return result;
}

juce::var stringArrayToVar (const juce::StringArray& values)
{
    juce::Array<juce::var> array;

    for (const auto& value : values)
        array.add (value);

    return juce::var (array);
}

juce::String pickSourceClipId (const juce::StringArray& clipIds)
{
    return clipIds.size() == 1 ? clipIds[0] : juce::String();
}

juce::String colorHintForClipId (const juce::String& clipId)
{
    if (clipId.isEmpty())
        return "#9AA4B2";

    const auto hash = static_cast<uint32_t> (clipId.hashCode());
    const auto hue = static_cast<float> (hash % 360) / 360.0f;
    const auto colour = juce::Colour::fromHSV (hue, 0.65f, 0.9f, 1.0f);
    return colour.toDisplayString (true);
}

bool isHiddenInScope (const juce::StringArray& clipIds, const ParsedScope& scope)
{
    if (scope.isTrack || scope.isDebugGlobal)
        return false;

    if (! scope.isClip)
        return false;

    if (clipIds.isEmpty())
        return false;

    return ! clipIds.contains (scope.selectedClipId);
}

juce::String makeEdgeKey (const juce::String& sourceId, int sourcePin, const juce::String& destId, int destPin)
{
    return sourceId + "|" + juce::String (sourcePin) + "|" + destId + "|" + juce::String (destPin);
}

juce::StringArray parseInternalDefaultAudioSources (const juce::ValueTree& pluginInstanceState)
{
    juce::StringArray result;
    auto raw = pluginInstanceState.getProperty (kVitInternalDefaultAudioSourcesProperty).toString().trim();

    if (raw.isEmpty())
        return result;

    raw = raw.replaceCharacter ('|', ',');
    juce::StringArray tokens;
    tokens.addTokens (raw, ",", juce::String());

    for (auto token : tokens)
    {
        token = token.trim();

        if (token.isNotEmpty())
            result.addIfNotAlreadyThere (token);
    }

    return result;
}

juce::ValueTree findRackPluginInstanceState (te::RackType& rackType, te::EditItemID nodeId)
{
    for (auto child : rackType.state)
        if (child.hasType (te::IDs::PLUGININSTANCE))
            if (auto pluginState = child.getChildWithName (te::IDs::PLUGIN); pluginState.isValid())
                if (te::EditItemID::fromID (pluginState) == nodeId)
                    return child;

    return {};
}

std::unordered_map<std::string, std::unordered_set<std::string>> collectInternalDefaultAudioSourcesByDest (te::RackType& rackType)
{
    std::unordered_map<std::string, std::unordered_set<std::string>> result;

    for (auto child : rackType.state)
    {
        if (! child.hasType (te::IDs::PLUGININSTANCE))
            continue;

        auto pluginState = child.getChildWithName (te::IDs::PLUGIN);

        if (! pluginState.isValid())
            continue;

        const auto destId = te::EditItemID::fromID (pluginState);

        if (! destId.isValid())
            continue;

        auto sources = parseInternalDefaultAudioSources (child);

        if (sources.isEmpty())
            continue;

        auto& destSources = result[destId.toString().toStdString()];

        for (const auto& source : sources)
            destSources.insert (source.toStdString());
    }

    return result;
}

bool isMainAudioPinPair (int sourcePin, int destPin)
{
    return sourcePin >= 1 && destPin >= 1;
}

bool isInternalDefaultAudioEdge (const std::unordered_map<std::string, std::unordered_set<std::string>>& internalSourcesByDest,
                                 const juce::String& sourceId,
                                 int sourcePin,
                                 const juce::String& destId,
                                 int destPin)
{
    if (! isMainAudioPinPair (sourcePin, destPin))
        return false;

    const auto foundDest = internalSourcesByDest.find (destId.toStdString());

    if (foundDest == internalSourcesByDest.end())
        return false;

    return foundDest->second.count (sourceId.toStdString()) > 0;
}

std::unordered_set<std::string> collectNodesReachableFromRackInput (te::RackType& rackType)
{
    std::unordered_map<std::string, std::vector<te::EditItemID>> adjacency;
    std::vector<te::EditItemID> queue;
    std::unordered_set<std::string> reachable;

    for (auto* connection : rackType.getConnections())
    {
        if (connection == nullptr)
            continue;

        if (connection->sourcePin.get() < 0 || connection->destPin.get() < 0)
            continue;

        const auto sourceId = connection->sourceID.get();
        const auto destId = connection->destID.get();

        if (! destId.isValid())
            continue;

        if (! sourceId.isValid())
        {
            const bool isRackAudioInput = isMainAudioPinPair (connection->sourcePin.get(), connection->destPin.get());
            const bool isRackMidiToZ2 = connection->sourcePin.get() == 0
                                        && connection->destPin.get() == 0
                                        && VitGraphValidator::getZoneIdForNode (rackType, destId) == "Z2";

            if (isRackAudioInput || isRackMidiToZ2)
            {
                const auto key = destId.toString().toStdString();

                if (reachable.insert (key).second)
                    queue.push_back (destId);
            }

            continue;
        }

        if (isMainAudioPinPair (connection->sourcePin.get(), connection->destPin.get()))
            adjacency[sourceId.toString().toStdString()].push_back (destId);
    }

    for (size_t i = 0; i < queue.size(); ++i)
    {
        const auto sourceId = queue[i];
        const auto found = adjacency.find (sourceId.toString().toStdString());

        if (found == adjacency.end())
            continue;

        for (const auto& destId : found->second)
        {
            const auto key = destId.toString().toStdString();

            if (reachable.insert (key).second)
                queue.push_back (destId);
        }
    }

    return reachable;
}

std::unordered_set<std::string> collectNodesThatCanReachRackOutput (te::RackType& rackType)
{
    std::unordered_map<std::string, std::vector<te::EditItemID>> reverseAdjacency;
    std::vector<te::EditItemID> queue;
    std::unordered_set<std::string> canReachOutput;

    for (auto* connection : rackType.getConnections())
    {
        if (connection == nullptr)
            continue;

        if (! isMainAudioPinPair (connection->sourcePin.get(), connection->destPin.get()))
            continue;

        const auto sourceId = connection->sourceID.get();
        const auto destId = connection->destID.get();

        if (! sourceId.isValid())
            continue;

        if (! destId.isValid())
        {
            const auto key = sourceId.toString().toStdString();

            if (canReachOutput.insert (key).second)
                queue.push_back (sourceId);

            continue;
        }

        reverseAdjacency[destId.toString().toStdString()].push_back (sourceId);
    }

    for (size_t i = 0; i < queue.size(); ++i)
    {
        const auto destId = queue[i];
        const auto found = reverseAdjacency.find (destId.toString().toStdString());

        if (found == reverseAdjacency.end())
            continue;

        for (const auto& sourceId : found->second)
        {
            const auto key = sourceId.toString().toStdString();

            if (canReachOutput.insert (key).second)
                queue.push_back (sourceId);
        }
    }

    return canReachOutput;
}

} // namespace

juce::String VitClipRouteRegistry::normaliseClipScope (const juce::String& clipScope)
{
    const auto trimmed = clipScope.trim();

    if (trimmed.isEmpty())
        return "track";

    if (trimmed.equalsIgnoreCase ("track"))
        return "track";

    if (trimmed.equalsIgnoreCase ("debug_global"))
        return "debug_global";

    if (trimmed.startsWithIgnoreCase ("clip:"))
        return "clip:" + trimmed.fromFirstOccurrenceOf (":", false, false).trim();

    return "track";
}

juce::Result VitClipRouteRegistry::validateClipScope (te::Track& track, const juce::String& clipScope)
{
    const auto normalised = normaliseClipScope (clipScope);

    if (normalised == "track" || normalised == "debug_global")
        return juce::Result::ok();

    if (! normalised.startsWithIgnoreCase ("clip:"))
        return juce::Result::fail ("clip_scope must be track or clip:<clip_id>");

    const auto clipId = normalised.fromFirstOccurrenceOf (":", false, false).trim();

    if (clipId.isEmpty())
        return juce::Result::fail ("clip_scope requires a non-empty clip id");

    for (const auto& clip : collectTrackClips (track))
        if (clip.clipId == clipId)
            return juce::Result::ok();

    return juce::Result::fail ("clip_scope references a clip that is not on the specified track");
}

juce::String VitClipRouteRegistry::getNodeClipScope (te::RackType& rackType, te::EditItemID nodeId)
{
    const auto pluginInstanceState = findRackPluginInstanceState (rackType, nodeId);

    if (! pluginInstanceState.isValid())
        return "track";

    return normaliseClipScope (pluginInstanceState.getProperty ("vit_clip_scope").toString());
}

juce::Array<juce::var> VitClipRouteRegistry::createClipProxyNodes (te::Track& track, const juce::String& requestScope)
{
    juce::Array<juce::var> proxies;
    const auto scope = parseScope (requestScope);

    for (const auto& clip : collectTrackClips (track))
    {
        auto object = std::make_unique<juce::DynamicObject>();
        const juce::StringArray clipIds { clip.clipId };
        object->setProperty ("node_id", "clip_proxy:" + clip.clipId);
        object->setProperty ("clip_id", clip.clipId);
        object->setProperty ("source_clip_id", clip.clipId);
        object->setProperty ("name", clip.name);
        object->setProperty ("node_role", "clip_proxy");
        object->setProperty ("clip_type", clip.isMidi ? "midi" : "audio");
        object->setProperty ("line_color_hint", colorHintForClipId (clip.clipId));
        object->setProperty ("shared_by_clip_ids", stringArrayToVar (clipIds));
        object->setProperty ("is_scope_hidden", isHiddenInScope (clipIds, scope));
        proxies.add (juce::var (object.release()));
    }

    return proxies;
}

juce::Array<juce::var> VitClipRouteRegistry::createClipRoutes (te::Track& track,
                                                               te::RackType& rackType,
                                                               const juce::String& requestScope)
{
    juce::Array<juce::var> routes;
    const auto clips = collectTrackClips (track);
    const auto allClipIds = toClipIdsArray (clips);
    const auto scope = parseScope (requestScope);

    for (auto* plugin : rackType.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        const auto nodeId = plugin->itemID.toString();
        const auto clipScope = getNodeClipScope (rackType, plugin->itemID);
        const auto owningClipIds = parseNodeClipIds (clipScope, allClipIds);

        for (const auto& clipId : owningClipIds)
        {
            auto route = std::make_unique<juce::DynamicObject>();
            const juce::StringArray routeClipIds { clipId };
            route->setProperty ("clip_id", clipId);
            route->setProperty ("source_clip_id", clipId);
            route->setProperty ("proxy_node_id", "clip_proxy:" + clipId);
            route->setProperty ("dest_node_id", nodeId);
            route->setProperty ("shared_by_clip_ids", stringArrayToVar (owningClipIds));
            route->setProperty ("line_color_hint", colorHintForClipId (clipId));
            route->setProperty ("is_scope_hidden", isHiddenInScope (routeClipIds, scope));
            routes.add (juce::var (route.release()));
        }
    }

    return routes;
}

void VitClipRouteRegistry::annotateNodesAndEdges (te::Track& track,
                                                  te::RackType& rackType,
                                                  juce::Array<juce::var>& nodes,
                                                  juce::Array<juce::var>& edges,
                                                  const juce::String& requestScope)
{
    const auto clips = collectTrackClips (track);
    const auto allClipIds = toClipIdsArray (clips);
    const auto scope = parseScope (requestScope);
    const auto internalSourcesByDest = collectInternalDefaultAudioSourcesByDest (rackType);
    const auto reachableFromInput = collectNodesReachableFromRackInput (rackType);
    const auto canReachOutput = collectNodesThatCanReachRackOutput (rackType);
    std::unordered_map<std::string, juce::StringArray> nodeClipSets;

    for (auto& nodeVar : nodes)
    {
        auto* object = nodeVar.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto nodeId = object->getProperty ("node_id").toString();
        const auto clipScope = normaliseClipScope (object->getProperty ("clip_scope").toString());
        const auto clipIds = parseNodeClipIds (clipScope, allClipIds);
        const auto nodeKey = nodeId.toStdString();
        const bool reachable = reachableFromInput.count (nodeKey) > 0;
        const bool effectiveInOutputPath = reachable && canReachOutput.count (nodeKey) > 0;
        const auto zoneId = object->getProperty ("zone_id").toString().trim().toUpperCase();
        nodeClipSets[nodeId.toStdString()] = clipIds;

        object->setProperty ("shared_by_clip_ids", stringArrayToVar (clipIds));
        object->setProperty ("source_clip_id", pickSourceClipId (clipIds));
        object->setProperty ("line_color_hint", colorHintForClipId (pickSourceClipId (clipIds)));
        object->setProperty ("is_scope_hidden", isHiddenInScope (clipIds, scope));
        object->setProperty ("audio_reachable_from_rack_input", reachable);
        object->setProperty ("vit_effective_in_output_path", effectiveInOutputPath);

        if (zoneId == "Z3")
            object->setProperty ("vit_orphan_bypass_candidate", ! effectiveInOutputPath);
    }

    juce::Array<juce::var> visibleEdges;

    for (auto& edgeVar : edges)
    {
        auto* object = edgeVar.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto sourceId = object->getProperty ("source_id").toString();
        const auto destId = object->getProperty ("dest_id").toString();
        const auto sourcePin = static_cast<int> (object->getProperty ("source_pin"));
        const auto destPin = static_cast<int> (object->getProperty ("dest_pin"));
        const auto sourceClipIds = nodeClipSets[sourceId.toStdString()];
        const auto destClipIds = nodeClipSets[destId.toStdString()];
        const bool internalDefaultAudio = isInternalDefaultAudioEdge (internalSourcesByDest, sourceId, sourcePin, destId, destPin);
        auto sharedClipIds = intersectClipSets (sourceClipIds, destClipIds);

        if (sharedClipIds.isEmpty())
            sharedClipIds = sourceClipIds;

        object->setProperty ("edge_id", makeEdgeKey (sourceId, sourcePin, destId, destPin));
        object->setProperty ("shared_by_clip_ids", stringArrayToVar (sharedClipIds));
        object->setProperty ("source_clip_id", pickSourceClipId (sharedClipIds));
        object->setProperty ("line_color_hint", colorHintForClipId (pickSourceClipId (sharedClipIds)));
        object->setProperty ("is_scope_hidden", isHiddenInScope (sharedClipIds, scope));
        object->setProperty ("vit_internal_default_audio", internalDefaultAudio);

        if (! internalDefaultAudio)
            visibleEdges.add (edgeVar);
    }

    edges = visibleEdges;
}

} // namespace vit
