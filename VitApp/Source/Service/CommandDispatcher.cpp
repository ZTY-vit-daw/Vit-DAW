#include "CommandDispatcher.h"

#include "ClipService.h"
#include "GeneratedAssetService.h"
#include "ImportService.h"
#include "JobEventService.h"
#include "MidiService.h"
#include "ProjectService.h"
#include "PluginRackControlService.h"
#include "TiledSpectrogramBaker.h"
#include "TrackService.h"
#include "TransportAudioService.h"
#include "VitProductionCoordinator.h"

#include "../Core/VitAIGCJobRuntime.h"
#include "../Core/VitClipRouteRegistry.h"
#include "../Core/VitConnectorPluginSpec.h"
#include "../Core/VitDagChecker.h"
#include "../Core/VitGraphTrace.h"
#include "../Core/VitGraphValidator.h"
#include "../Core/VitMacroNode.h"
#include "../Core/VitParallelMergePlanner.h"
#include "../Core/VitNodeRegistry.h"
#include "../Core/VitPaths.h"
#include "../Core/VitControlShell.h"
#include "../Core/VitMediaPoolManager.h"
#include "../Core/VitParamBinding.h"
#include "../Core/VitParamLinkGraph.h"
#include "../Core/VitGraphSwapCoordinator.h"
#include "../Core/VitPluginGrabber.h"
#include "../Core/VitKernelUtils.h"
#include "../Core/VitParamSurface.h"
#include "../Core/VitPluginTemplateRegistry.h"
#include "../Core/VitProjectHealthCheck.h"
#include "../Core/VitTakeHistoryStack.h"
#include "../Core/VitZoneBufferAdapter.h"

#include <cmath>
#include <unordered_map>
#include <unordered_set>
#include <vector>

namespace vit
{

namespace
{

juce::String pluginItemIdString (const te::Plugin& plugin);
juce::String parseZoneIdOrDefault (const juce::DynamicObject& object, te::Plugin* plugin = nullptr);
juce::String parseClipScopeOrDefault (const juce::DynamicObject& object);
juce::Result validateRequestedZoneId (const juce::DynamicObject& object);
juce::Result validateRequestedClipScope (te::Track& track, const juce::DynamicObject& object);
te::RackInstance* findRackInstanceOnTrack (te::Track& track, const juce::String& rackItemId = {});
te::RackInstance* findUsableRackInstanceOnTrack (te::Track& track, const juce::String& rackItemId = {});
int fallbackRackInsertionIndex (te::Track& track);
te::RackInstance* ensureRackInstanceOnTrack (te::Track& track);
juce::ValueTree findRackPluginInstanceState (te::RackType& rackType, te::EditItemID pluginItemId);
juce::Result resolveExternalPluginDescription (te::Edit& edit, const juce::String& pluginPath, juce::PluginDescription& outDesc);
te::Plugin* findPluginInEdit (te::Edit& edit, const juce::String& pluginIdStr);
juce::String describeExternalPluginLoadState (te::Edit& edit, te::ExternalPlugin& plugin);
bool ensureExternalPluginInstanceReady (te::Edit& edit,
                                        te::ExternalPlugin& plugin,
                                        const juce::String& context,
                                        int timeoutMs);
juce::String inferPluginFormatNameFromPath (const juce::String& pluginPath);
void normaliseAndRegisterExternalPluginDescription (te::Edit& edit,
                                                    juce::PluginDescription& desc,
                                                    const juce::String& fallbackPath,
                                                    const juce::String& context);
juce::var createRackState (te::Track& track, const juce::String& requestScope);
juce::var createControlGraphState (te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, const VitGraphRevisionSnapshot& snapshot);
juce::File getEffectiveProjectFile (const CommandDispatcher::CurrentProjectPathGetter& getter);
/** When RackType::addPlugin skips auto-connect (non-empty rack), chain new plugin after the unique tail feeding rack outputs. */
bool vitTryChainNewRackPluginSerial (te::RackType& rackType, te::Plugin& newPlugin);

enum class OverlapPolicy
{
    trim,
    layer,
    crossfade,
};

constexpr double kMinimumSurvivingClipLengthSeconds = 0.01;
constexpr int kPluginRackInsertReadyTimeoutMs = 0;
constexpr int kPluginOpenUiReadyTimeoutMs = 0;
constexpr double kClipEditEpsilonSeconds = 0.0005;
constexpr auto kVitParamAliasesProperty = "vit_param_aliases";

/** Empty string or "RACK_INPUT" selects te::RackType bus input (invalid source EditItemID). */
bool isRackBusInputSourceToken (const juce::String& trimmedSourceId)
{
    if (trimmedSourceId.isEmpty())
        return true;

    return trimmedSourceId.equalsIgnoreCase ("RACK_INPUT");
}

/** Empty string or "RACK_OUTPUT" selects te::RackType bus output (invalid dest EditItemID). */
bool isRackBusOutputDestToken (const juce::String& trimmedDestId)
{
    if (trimmedDestId.isEmpty())
        return true;

    return trimmedDestId.equalsIgnoreCase ("RACK_OUTPUT");
}

juce::String rackEndpointLabel (te::EditItemID id, bool isSource)
{
	return id.isValid() ? id.toString()
	                    : juce::String (isSource ? "RACK_INPUT" : "RACK_OUTPUT");
}

juce::String describeRackConnections (te::RackType& rackType)
{
	juce::StringArray parts;

	for (auto* connection : rackType.getConnections())
	{
		if (connection == nullptr)
			continue;

		parts.add (rackEndpointLabel (connection->sourceID.get(), true)
		           + ":" + juce::String (connection->sourcePin.get())
		           + "->"
		           + rackEndpointLabel (connection->destID.get(), false)
		           + ":" + juce::String (connection->destPin.get()));
	}

	return "[" + parts.joinIntoString (", ") + "]";
}

void logRackConnections (const juce::String& label, te::RackType& rackType)
{
	juce::Logger::writeToLog ("CommandDispatcher: " + label
	                          + " connections=" + describeRackConnections (rackType));
}

bool rackConnectionExists (te::RackType& rackType,
                           te::EditItemID sourceId,
                           int sourcePin,
                           te::EditItemID destId,
                           int destPin)
{
	for (auto* connection : rackType.getConnections())
	{
		if (connection == nullptr)
			continue;

		if (connection->sourceID.get() == sourceId
		    && connection->destID.get() == destId
		    && connection->sourcePin.get() == sourcePin
		    && connection->destPin.get() == destPin)
			return true;
	}

	return false;
}

int addLogicalStereoAudioConnections (te::RackType& rackType,
                                      te::EditItemID sourceId,
                                      te::EditItemID destId)
{
	int accepted = 0;

	if (rackConnectionExists (rackType, sourceId, 0, destId, 0))
		return 1;

	if (rackType.isConnectionLegal (sourceId, 0, destId, 0)
	    && rackType.addConnection (sourceId, 0, destId, 0))
		return 1;

	for (int pin = 1; pin <= 2; ++pin)
	{
		if (rackConnectionExists (rackType, sourceId, pin, destId, pin))
		{
			++accepted;
			continue;
		}

		if (rackType.isConnectionLegal (sourceId, pin, destId, pin)
		    && rackType.addConnection (sourceId, pin, destId, pin))
			++accepted;
	}

	return accepted;
}

int removeLogicalAudioConnections (te::RackType& rackType,
                                   te::EditItemID sourceId,
                                   te::EditItemID destId)
{
	struct PinPair
	{
		int sourcePin = 0;
		int destPin = 0;
	};

	std::vector<PinPair> toRemove;

	for (auto* connection : rackType.getConnections())
	{
		if (connection == nullptr)
			continue;

		if (connection->sourceID.get() == sourceId
		    && connection->destID.get() == destId
		    && connection->sourcePin.get() >= 0
		    && connection->destPin.get() >= 0)
		{
			toRemove.push_back ({ connection->sourcePin.get(), connection->destPin.get() });
		}
	}

	int removed = 0;

	for (const auto& pins : toRemove)
		if (rackType.removeConnection (sourceId, pins.sourcePin, destId, pins.destPin))
			++removed;

	return removed;
}

std::unordered_set<std::string> collectAudioReachableRackNodeIds (te::RackType& rackType)
{
	std::unordered_map<std::string, std::vector<te::EditItemID>> adjacency;
	std::vector<te::EditItemID> queue;
	std::unordered_set<std::string> reachable;

	for (auto* connection : rackType.getConnections())
	{
		if (connection == nullptr)
			continue;

		const auto sourceId = connection->sourceID.get();
		const auto destId = connection->destID.get();

		if (connection->sourcePin.get() < 0 || connection->destPin.get() < 0)
			continue;

		if (! destId.isValid())
			continue;

		if (! sourceId.isValid())
		{
			const auto key = destId.toString().toStdString();

			if (reachable.insert (key).second)
				queue.push_back (destId);

			continue;
		}

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

std::unordered_map<std::string, juce::String> readPluginParamAliases (const te::Plugin& plugin)
{
    std::unordered_map<std::string, juce::String> aliases;
    const auto raw = plugin.state.getProperty (kVitParamAliasesProperty).toString().trim();

    if (raw.isEmpty())
        return aliases;

    const auto parsed = juce::JSON::parse (raw);
    auto* object = parsed.getDynamicObject();
    if (object == nullptr)
        return aliases;

    const auto& properties = object->getProperties();

    for (int i = 0; i < properties.size(); ++i)
    {
        const auto key = properties.getName (i).toString().trim();
        const auto value = properties.getValueAt (i).toString().trim();

        if (key.isNotEmpty() && value.isNotEmpty())
            aliases[key.toStdString()] = value;
    }

    return aliases;
}

juce::var createAliasMapVar (const std::unordered_map<std::string, juce::String>& aliases)
{
    auto object = std::make_unique<juce::DynamicObject>();

    for (const auto& [key, value] : aliases)
        object->setProperty (juce::Identifier (key), value);

    return juce::var (object.release());
}

juce::String serialisePluginParamAliases (const std::unordered_map<std::string, juce::String>& aliases)
{
    if (aliases.empty())
        return {};

    return juce::JSON::toString (createAliasMapVar (aliases));
}

std::unordered_map<std::string, juce::String> extractPluginParamAliasUpdates (const juce::DynamicObject& object)
{
    std::unordered_map<std::string, juce::String> updates;

    auto readAliasObject = [&updates] (const juce::var& value)
    {
        auto* aliasObject = value.getDynamicObject();
        if (aliasObject == nullptr)
            return;

        const auto& properties = aliasObject->getProperties();

        for (int i = 0; i < properties.size(); ++i)
        {
            const auto key = properties.getName (i).toString().trim();
            if (key.isEmpty())
                continue;

            updates[key.toStdString()] = properties.getValueAt (i).toString().trim();
        }
    };

    readAliasObject (object.getProperty ("aliases"));
    readAliasObject (object.getProperty ("param_aliases"));

    const auto singleParamId = object.getProperty ("param_id").toString().trim();
    if (singleParamId.isNotEmpty())
        updates[singleParamId.toStdString()] = object.getProperty ("alias").toString().trim();

    return updates;
}

void applyAliasesToParameterDescriptors (juce::Array<juce::var>& parameterDescriptors,
                                         const std::unordered_map<std::string, juce::String>& aliases)
{
    for (auto& parameter : parameterDescriptors)
    {
        auto* object = parameter.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto rawParamId = object->getProperty ("raw_param_id").toString();
        const auto normalizedRole = object->getProperty ("normalized_role").toString();
        const auto rawParamName = object->getProperty ("raw_param_name").toString();

        auto alias = rawParamName;
        auto aliasSource = juce::String ("raw_param_name");

        if (const auto rawAliasIt = aliases.find (rawParamId.toStdString()); rawAliasIt != aliases.end() && rawAliasIt->second.isNotEmpty())
        {
            alias = rawAliasIt->second;
            aliasSource = "param_id";
        }
        else if (const auto roleAliasIt = aliases.find (normalizedRole.toStdString()); roleAliasIt != aliases.end() && roleAliasIt->second.isNotEmpty())
        {
            alias = roleAliasIt->second;
            aliasSource = "normalized_role";
        }

        object->setProperty ("alias", alias);
        object->setProperty ("alias_source", aliasSource);
    }
}

te::AutomatableParameter::Ptr resolvePluginParameterByID (te::Plugin& plugin, const juce::String& paramIdRaw)
{
    te::AutomatableParameter::Ptr param = plugin.getAutomatableParameterByID (paramIdRaw);

    if (param == nullptr && paramIdRaw.equalsIgnoreCase ("volume"))
        param = plugin.getAutomatableParameterByID ("master volume");

    if (param == nullptr && paramIdRaw.equalsIgnoreCase ("pan"))
        param = plugin.getAutomatableParameterByID ("master pan");

    return param;
}

void flushPluginOrOwnerState (te::Plugin& plugin)
{
    if (auto* owner = plugin.getOwnerTrack())
        owner->flushStateToValueTree();
    else
        plugin.flushPluginStateToValueTree();
}

double mapControlValueThroughBinding (double sourceValue,
                                      const juce::ValueTree& sourceNodeState,
                                      const juce::ValueTree& bindingState)
{
    const auto sourceMin = static_cast<double> (sourceNodeState.getProperty ("min"));
    const auto sourceMax = static_cast<double> (sourceNodeState.getProperty ("max"));
    const auto bindingMin = static_cast<double> (bindingState.getProperty ("range_min"));
    const auto bindingMax = static_cast<double> (bindingState.getProperty ("range_max"));
    const auto curve = bindingState.getProperty ("curve").toString().trim().toLowerCase();

    double normalized = 0.0;
    if (sourceMax > sourceMin)
        normalized = juce::jlimit (0.0, 1.0, (sourceValue - sourceMin) / (sourceMax - sourceMin));

    if (curve == "invert")
        normalized = 1.0 - normalized;
    else if (curve == "ease_in")
        normalized = normalized * normalized;
    else if (curve == "ease_out")
        normalized = std::sqrt (normalized);

    return bindingMin + (bindingMax - bindingMin) * normalized;
}

void appendBindingApplyRecord (juce::Array<juce::var>& appliedBindings,
                               const juce::String& bindingId,
                               const juce::String& targetKind,
                               const juce::String& targetPluginId,
                               const juce::String& targetParamId,
                               const juce::String& targetNodeId,
                               const juce::String& targetInput,
                               double appliedValue)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("binding_id", bindingId);
    row->setProperty ("target_kind", targetKind);
    row->setProperty ("target_plugin_id", targetPluginId);
    row->setProperty ("target_param_id", targetParamId);
    row->setProperty ("target_node_id", targetNodeId);
    row->setProperty ("target_input", targetInput);
    row->setProperty ("applied_value", appliedValue);
    appliedBindings.add (juce::var (row.release()));
}

void applyControlBindingsFromOutput (te::Edit& edit,
                                     const juce::String& sourceNodeId,
                                     const juce::String& sourceOutput,
                                     juce::UndoManager* undoManager,
                                     juce::Array<juce::var>& appliedBindings,
                                     std::unordered_set<std::string>& visitedOutputs)
{
    const auto visitKey = (sourceNodeId + "::" + (sourceOutput.trim().isNotEmpty() ? sourceOutput.trim() : "value")).toStdString();
    if (visitedOutputs.contains (visitKey))
        return;

    visitedOutputs.insert (visitKey);

    auto sourceNodeState = VitParamLinkGraph::findControlNode (edit, sourceNodeId);
    if (! sourceNodeState.isValid())
        return;

    const auto normalizedSourceOutput = sourceOutput.trim().isNotEmpty() ? sourceOutput.trim() : "value";
    const auto sourceValue = VitParamLinkGraph::getNodeOutputValue (sourceNodeState, normalizedSourceOutput);
    const auto bindings = VitParamLinkGraph::getBindingsForSource (edit, sourceNodeId, normalizedSourceOutput);

    for (const auto& bindingState : bindings)
    {
        if (! static_cast<bool> (bindingState.getProperty ("enabled")))
            continue;

        const auto targetKind = bindingState.getProperty ("target_kind").toString();
        const auto bindingId = bindingState.getProperty ("binding_id").toString();
        const auto appliedValue = mapControlValueThroughBinding (sourceValue, sourceNodeState, bindingState);

        if (targetKind == "plugin_param")
        {
            const auto targetPluginId = bindingState.getProperty ("target_plugin_id").toString();
            const auto targetParamId = bindingState.getProperty ("target_param_id").toString();
            auto* targetPlugin = findPluginInEdit (edit, targetPluginId);
            if (targetPlugin == nullptr)
                continue;

            auto targetParam = resolvePluginParameterByID (*targetPlugin, targetParamId);
            if (targetParam == nullptr)
                continue;

            const auto vr = targetParam->getValueRange();
            const auto clampedValue = juce::jlimit (static_cast<double> (vr.getStart()),
                                                    static_cast<double> (vr.getEnd()),
                                                    appliedValue);
            targetParam->setParameter (static_cast<float> (clampedValue), juce::sendNotification);
            flushPluginOrOwnerState (*targetPlugin);
            appendBindingApplyRecord (appliedBindings,
                                      bindingId,
                                      targetKind,
                                      targetPluginId,
                                      targetParamId,
                                      {},
                                      {},
                                      clampedValue);
        }
        else if (targetKind == "control_port")
        {
            const auto targetNodeId = bindingState.getProperty ("target_node_id").toString();
            const auto targetInput = bindingState.getProperty ("target_input").toString();
            auto targetNodeState = VitParamLinkGraph::findControlNode (edit, targetNodeId);
            if (! targetNodeState.isValid())
                continue;

            VitParamLinkGraph::setNodeOutputValue (targetNodeState, targetInput, appliedValue, undoManager);
            appendBindingApplyRecord (appliedBindings,
                                      bindingId,
                                      targetKind,
                                      {},
                                      {},
                                      targetNodeId,
                                      targetInput,
                                      appliedValue);
            applyControlBindingsFromOutput (edit,
                                            targetNodeId,
                                            targetInput,
                                            undoManager,
                                            appliedBindings,
                                            visitedOutputs);
        }
    }
}

juce::var createResolvedPluginParamInfo (te::Plugin& plugin, const juce::String& paramIdRaw)
{
    auto targetParam = resolvePluginParameterByID (plugin, paramIdRaw);
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("plugin_id", pluginItemIdString (plugin));
    object->setProperty ("plugin_name", plugin.getName());
    object->setProperty ("plugin_type", plugin.getPluginType());
    object->setProperty ("template_role", plugin.state.getProperty ("vit_template_role").toString());
    object->setProperty ("param_id", paramIdRaw);

    if (targetParam != nullptr)
    {
        object->setProperty ("resolved", true);
        object->setProperty ("param_name", targetParam->getParameterName());
        object->setProperty ("current_value", targetParam->getCurrentValue());
        const auto range = targetParam->getValueRange();
        object->setProperty ("min", range.getStart());
        object->setProperty ("max", range.getEnd());
    }
    else
    {
        object->setProperty ("resolved", false);
        object->setProperty ("param_name", {});
    }

    return juce::var (object.release());
}

juce::var createResolvedBindingTargetInfo (te::Edit& edit, const juce::ValueTree& bindingState)
{
    const auto targetKind = bindingState.getProperty ("target_kind").toString();
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("target_kind", targetKind);

    if (targetKind == "plugin_param")
    {
        const auto targetPluginId = bindingState.getProperty ("target_plugin_id").toString();
        const auto targetParamId = bindingState.getProperty ("target_param_id").toString();
        if (auto* targetPlugin = findPluginInEdit (edit, targetPluginId))
            return createResolvedPluginParamInfo (*targetPlugin, targetParamId);

        object->setProperty ("resolved", false);
        object->setProperty ("plugin_id", targetPluginId);
        object->setProperty ("param_id", targetParamId);
    }
    else if (targetKind == "control_port")
    {
        const auto targetNodeId = bindingState.getProperty ("target_node_id").toString();
        const auto targetInput = bindingState.getProperty ("target_input").toString();
        auto targetNodeState = VitParamLinkGraph::findControlNode (edit, targetNodeId);
        object->setProperty ("resolved", targetNodeState.isValid());
        object->setProperty ("target_node_id", targetNodeId);
        object->setProperty ("target_input", targetInput);

        if (targetNodeState.isValid())
        {
            object->setProperty ("target_node_name", targetNodeState.getProperty ("name").toString());
            object->setProperty ("target_node_kind", targetNodeState.getProperty ("kind").toString());
        }
    }
    else
    {
        object->setProperty ("resolved", false);
    }

    return juce::var (object.release());
}

juce::var createBindingSnapshot (te::Edit& edit, const juce::ValueTree& bindingState)
{
    auto base = VitParamBinding::createSnapshot (bindingState);
    if (auto* object = base.getDynamicObject())
    {
        const auto resolvedInfo = createResolvedBindingTargetInfo (edit, bindingState);
        object->setProperty ("resolved_target_info", resolvedInfo);

        if (bindingState.getProperty ("target_kind").toString() == "plugin_param")
            object->setProperty ("resolved_param_info", resolvedInfo);
    }
    return base;
}

juce::Array<juce::var> createControlBindingsSnapshot (te::Edit& edit)
{
    juce::Array<juce::var> bindings;
    for (const auto& bindingState : VitParamLinkGraph::getAllBindings (edit))
        bindings.add (createBindingSnapshot (edit, bindingState));
    return bindings;
}

void appendBindingTargetPropertiesToParameterDescriptors (juce::Array<juce::var>& parameterDescriptors,
                                                          te::Plugin& plugin)
{
    const auto pluginId = pluginItemIdString (plugin);
    const auto templateRole = plugin.state.getProperty ("vit_template_role").toString();

    for (const auto& parameter : parameterDescriptors)
    {
        auto* object = parameter.getDynamicObject();
        if (object == nullptr)
            continue;

        auto target = std::make_unique<juce::DynamicObject>();
        target->setProperty ("target_kind", "plugin_param");
        target->setProperty ("plugin_id", pluginId);
        target->setProperty ("plugin_name", plugin.getName());
        target->setProperty ("plugin_type", plugin.getPluginType());
        target->setProperty ("template_role", templateRole);
        target->setProperty ("param_id", object->getProperty ("raw_param_id"));
        target->setProperty ("param_name", object->getProperty ("raw_param_name"));
        target->setProperty ("normalized_role", object->getProperty ("normalized_role"));
        target->setProperty ("alias", object->getProperty ("alias"));
        object->setProperty ("binding_target", juce::var (target.release()));
    }
}

OverlapPolicy parseOverlapPolicy (const juce::DynamicObject& object)
{
    auto raw = object.getProperty ("overlap_policy").toString().trim().toLowerCase();

    if (raw.isEmpty())
        raw = object.getProperty ("overlap_mode").toString().trim().toLowerCase();

    if (raw.isEmpty() || raw == "cut" || raw == "trim")
        return OverlapPolicy::trim;

    if (raw == "layer" || raw == "stack")
        return OverlapPolicy::layer;

    if (raw == "crossfade" || raw == "xfade")
        return OverlapPolicy::crossfade;

    return OverlapPolicy::trim;
}

juce::String overlapPolicyToString (OverlapPolicy policy)
{
    switch (policy)
    {
        case OverlapPolicy::layer:     return "layer";
        case OverlapPolicy::crossfade: return "crossfade";
        case OverlapPolicy::trim:      break;
    }

    return "trim";
}

bool isTrimPolicy (OverlapPolicy policy)
{
    return policy == OverlapPolicy::trim;
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

juce::Array<juce::var> createPluginNameArray (te::Track& track)
{
    juce::Array<juce::var> plugins;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        auto item = std::make_unique<juce::DynamicObject>();
        const auto pluginItemId = pluginItemIdString (*plugin);
        item->setProperty ("id", pluginItemId);
        item->setProperty ("plugin_item_id", pluginItemId);
        item->setProperty ("name", plugin->getName().trim());
        plugins.add (juce::var (item.release()));
    }

    return plugins;
}

/** get_project_state：与 createPluginNameArray 兼容，并补充 type/enabled，供前端机架与调试�?*/
juce::Array<juce::var> createProjectStatePluginsArray (te::Track& track)
{
    juce::Array<juce::var> plugins;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        auto item = std::make_unique<juce::DynamicObject>();
        const auto pluginItemId = pluginItemIdString (*plugin);
        item->setProperty ("id", pluginItemId);
        item->setProperty ("item_id", pluginItemId);
        item->setProperty ("plugin_item_id", pluginItemId);
        item->setProperty ("name", plugin->getName().trim());
        item->setProperty ("type", plugin->getPluginType());
        item->setProperty ("enabled", plugin->isEnabled());
        plugins.add (juce::var (item.release()));
    }

    return plugins;
}

/** 遍历 ClipTrack 上的 Clip �?TrackItem，输出时间线与标识（供前�?影子树对齐）�?*/
juce::Array<juce::var> createProjectStateClipsArray (te::Track& track)
{
    juce::Array<juce::var> clips;
    const int n = track.getNumTrackItems();

    for (int i = 0; i < n; ++i)
    {
        auto* item = track.getTrackItem (i);

        if (item == nullptr)
            continue;

        auto* clip = dynamic_cast<te::Clip*> (item);

        if (clip == nullptr)
            continue;

        const auto pos = clip->getPosition();
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", clip->itemID.toString());
        row->setProperty ("name", clip->getName());
        row->setProperty ("clip_type", juce::String (te::TrackItem::typeToString (clip->type)));
        row->setProperty ("start_seconds", pos.getStart().inSeconds());
        row->setProperty ("end_seconds", pos.getEnd().inSeconds());
        row->setProperty ("length_seconds", pos.getLength().inSeconds());
        row->setProperty ("offset_in_source_seconds", pos.getOffset().inSeconds());
        row->setProperty ("asset_ref", clip->state.getProperty ("vit_asset_ref").toString());
        row->setProperty ("asset_state", clip->state.getProperty ("vit_asset_state").toString());
        row->setProperty ("asset_job_id", clip->state.getProperty ("vit_asset_job_id").toString());
        row->setProperty ("warp_state", clip->state.getProperty ("vit_warp_state").toString());
        row->setProperty ("take_stack_id", clip->state.getProperty ("vit_take_stack_id").toString());
        row->setProperty ("active_take_id", clip->state.getProperty ("vit_active_take_id").toString());
        row->setProperty ("ghost_state", clip->state.getProperty ("vit_async_ghost_state").toString());

        if (clip->type == te::TrackItem::Type::wave)
        {
            if (auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip))
            {
                const auto srcFile = audioClip->getOriginalFile();
                const auto curFile = audioClip->getCurrentSourceFile();

                if (srcFile.existsAsFile())
                    row->setProperty ("file_path", srcFile.getFullPathName());

                row->setProperty ("current_source_path", curFile.getFullPathName());
                row->setProperty ("playback_source_valid",
                                  te::AudioFile (track.pluginList.getEdit().engine, curFile).isValid());
            }
        }

        clips.add (juce::var (row.release()));
    }

    return clips;
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

te::AudioTrack* findAudioTrackByIndex (te::Edit& edit, int trackIndex)
{
    if (trackIndex <= 0)
        return nullptr;

    int currentIndex = 0;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        ++currentIndex;

        if (currentIndex == trackIndex)
            return audioTrack;
    }

    return nullptr;
}

te::Track* findTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr && track->itemID.toString() == trackID)
            return track;

    return nullptr;
}

enum class CommandTimeUnit
{
    seconds,
    beats
};

bool parseTimeUnit (const juce::DynamicObject& object, CommandTimeUnit& out, juce::String& error)
{
    const auto raw = object.getProperty ("time_unit").toString().trim().toLowerCase();

    if (raw.isEmpty() || raw == "seconds" || raw == "second" || raw == "sec" || raw == "s")
    {
        out = CommandTimeUnit::seconds;
        return true;
    }

    if (raw == "beat" || raw == "beats")
    {
        out = CommandTimeUnit::beats;
        return true;
    }

    error = "time_unit must be one of: seconds, beats";
    return false;
}

bool readNumericProperty (const juce::DynamicObject& object, const juce::StringArray& keys, double& out)
{
    for (const auto& k : keys)
    {
        const auto v = object.getProperty (k);

        if (v.isDouble() || v.isInt() || v.isInt64())
        {
            out = static_cast<double> (v);
            return true;
        }
    }

    return false;
}

/** Remap every declared EditItemID in a detached clip tree (MIDI notes, automation, nested nodes). */
void regenerateValueTreeIDs (juce::ValueTree& vt, te::Edit& edit, juce::UndoManager* undoManager)
{
    if (! vt.isValid())
        return;

    te::EditItemID::remapIDs (vt, undoManager, edit, nullptr);
}

bool convertTimeValueToSeconds (te::Edit& edit, CommandTimeUnit unit, double value, double& outSeconds, juce::String& error)
{
    if (value < 0.0)
    {
        error = "time value must be greater than or equal to zero";
        return false;
    }

    if (unit == CommandTimeUnit::seconds)
    {
        outSeconds = value;
        return true;
    }

    outSeconds = edit.tempoSequence.toTime (te::BeatPosition::fromBeats (value)).inSeconds();
    return true;
}

bool convertDurationValueToSeconds (te::Edit& edit,
                                    CommandTimeUnit unit,
                                    double atStartSeconds,
                                    double durationValue,
                                    double& outSeconds,
                                    juce::String& error)
{
    if (durationValue <= 0.0)
    {
        error = "duration must be greater than zero";
        return false;
    }

    if (unit == CommandTimeUnit::seconds)
    {
        outSeconds = durationValue;
        return true;
    }

    const auto startTime = te::TimePosition::fromSeconds (atStartSeconds);
    const auto startBeat = edit.tempoSequence.toBeats (startTime);
    const auto endBeat = startBeat + te::BeatDuration::fromBeats (durationValue);
    const auto endTime = edit.tempoSequence.toTime (endBeat);
    outSeconds = (endTime - startTime).inSeconds();

    if (outSeconds <= 0.0)
    {
        error = "duration conversion produced non-positive length";
        return false;
    }

    return true;
}

bool applyCutOverlapPolicy (te::ClipTrack& track, te::Clip& movedOrResizedClip)
{
    const auto targetRange = movedOrResizedClip.getPosition().time;

    for (auto* clip : track.getClips())
    {
        if (clip == nullptr || clip == &movedOrResizedClip)
            continue;

        if (clip->getPosition().time.overlaps (targetRange))
            clip->trimAwayOverlap (targetRange);
    }

    return true;
}

struct OverlapEditResult
{
    juce::StringArray touchedOriginalClipIds;
    juce::StringArray createdClipIds;
    juce::StringArray removedClipIds;
};

juce::var stringArrayToVar (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return juce::var (out);
}

bool trackContainsClipId (te::ClipTrack& track, const juce::String& clipId)
{
    for (auto* clip : track.getClips())
        if (clip != nullptr && clip->itemID.toString() == clipId)
            return true;
    return false;
}

void applyMicroFadeIfNeeded (te::Clip& clip)
{
    if (auto* audioClip = dynamic_cast<te::AudioClipBase*> (&clip))
    {
        const auto clipLengthSeconds = clip.getPosition().getLength().inSeconds();
        const auto fadeSeconds = juce::jlimit (0.0, 0.005, juce::jmin (0.003, clipLengthSeconds * 0.5));

        if (fadeSeconds > 0.0)
        {
            const auto fadeDuration = te::TimeDuration::fromSeconds (fadeSeconds);
            audioClip->setFadeIn (fadeDuration);
            audioClip->setFadeOut (fadeDuration);
        }
    }
}

bool isBelowMinimumSurvivingClipLength (const te::Clip& clip)
{
    return clip.getPosition().getLength().inSeconds() < (kMinimumSurvivingClipLengthSeconds - kClipEditEpsilonSeconds);
}

void pruneTinyEditedClips (te::ClipTrack& track, OverlapEditResult* result)
{
    te::Clip::Array clipsToRemove;

    for (auto* clip : track.getClips())
    {
        if (clip == nullptr)
            continue;

        const auto clipId = clip->itemID.toString();
        const auto wasTouched = result != nullptr && result->touchedOriginalClipIds.contains (clipId);
        const auto wasCreated = result != nullptr && result->createdClipIds.contains (clipId);

        if (! wasTouched && ! wasCreated)
            continue;

        if (isBelowMinimumSurvivingClipLength (*clip))
            clipsToRemove.add (clip);
    }

    for (int i = clipsToRemove.size(); --i >= 0;)
    {
        auto clip = clipsToRemove.getUnchecked (i);
        if (clip != nullptr)
        {
            const auto clipId = clip->itemID.toString();
            const auto wasCreated = result != nullptr && result->createdClipIds.contains (clipId);

            clip->removeFromParent();

            if (result != nullptr)
            {
                if (wasCreated)
                    result->createdClipIds.removeString (clipId);
                else
                    result->removedClipIds.addIfNotAlreadyThere (clipId);
            }
        }
    }
}

bool applyCutOverlapPolicy (te::ClipTrack& track, te::Clip& movedOrResizedClip, bool allowSplit, OverlapEditResult* result = nullptr)
{
    const auto targetRange = movedOrResizedClip.getPosition().time;
    te::Clip::Array victims;

    for (auto* clip : track.getClips())
    {
        if (clip == nullptr || clip == &movedOrResizedClip)
            continue;

        if (clip->getPosition().time.overlaps (targetRange))
            victims.add (clip);
    }

    for (int i = victims.size(); --i >= 0;)
    {
        auto victim = victims.getUnchecked (i);

        if (victim == nullptr)
            continue;

        const auto victimId = victim->itemID.toString();

        if (result != nullptr)
            result->touchedOriginalClipIds.addIfNotAlreadyThere (victimId);

        if (allowSplit)
        {
            const auto newClips = te::deleteRegion (*victim, targetRange);

            if (result != nullptr)
                for (int newClipIndex = 0; newClipIndex < newClips.size(); ++newClipIndex)
                    if (auto* newClip = newClips.getUnchecked (newClipIndex))
                    if (newClip != nullptr)
                        result->createdClipIds.addIfNotAlreadyThere (newClip->itemID.toString());
        }
        else
        {
            victim->trimAwayOverlap (targetRange);
        }
    }

    pruneTinyEditedClips (track, result);

    if (result != nullptr)
    {
        for (const auto& victimId : result->touchedOriginalClipIds)
            if (! trackContainsClipId (track, victimId))
                result->removedClipIds.addIfNotAlreadyThere (victimId);

        for (auto* clip : track.getClips())
        {
            if (clip == nullptr)
                continue;

            const auto clipId = clip->itemID.toString();

            if (result->touchedOriginalClipIds.contains (clipId)
                || result->createdClipIds.contains (clipId))
            {
                applyMicroFadeIfNeeded (*clip);
            }
        }
    }

    return true;
}

juce::String pluginItemIdString (const te::Plugin& plugin)
{
    const auto id = plugin.itemID.toString().trim();
    jassert (id.isNotEmpty());
    return id;
}

juce::String parseZoneIdOrDefault (const juce::DynamicObject& object, te::Plugin* plugin)
{
    auto zone = VitGraphValidator::normaliseZoneId (object.getProperty ("zone_id").toString());

    if (VitGraphValidator::isZoneIdAllowedForNodeCreation (zone))
        return zone;

    if (plugin != nullptr && plugin->isSynth())
        return "Z2";

    return "Z3";
}

juce::String parseClipScopeOrDefault (const juce::DynamicObject& object)
{
    return VitClipRouteRegistry::normaliseClipScope (object.getProperty ("clip_scope").toString());
}

juce::Result validateRequestedZoneId (const juce::DynamicObject& object)
{
    const auto zoneId = VitGraphValidator::normaliseZoneId (object.getProperty ("zone_id").toString());

    if (zoneId.isEmpty())
        return juce::Result::ok();

    if (zoneId == "TOP")
        return juce::Result::fail ("Top is mapping-only and cannot host rack nodes");

    if (! VitGraphValidator::isZoneIdAllowedForNodeCreation (zoneId))
        return juce::Result::fail ("zone_id must be one of Z1, Z2, or Z3");

    return juce::Result::ok();
}

juce::Result validateRequestedClipScope (te::Track& track, const juce::DynamicObject& object)
{
    const auto clipScope = VitClipRouteRegistry::normaliseClipScope (object.getProperty ("clip_scope").toString());

    if (clipScope == "debug_global")
        return juce::Result::fail ("clip_scope for a node must be track or clip:<clip_id>");

    return VitClipRouteRegistry::validateClipScope (track, clipScope);
}

te::Plugin* findPluginByID (te::Track& track, const juce::String& pluginID)
{
    const auto parsedId = te::EditItemID::fromString (pluginID.trim());

    if (! parsedId.isValid())
        return nullptr;

    for (auto* plugin : track.pluginList.getPlugins())
        if (plugin != nullptr && plugin->itemID == parsedId)
            return plugin;

    return nullptr;
}

/** True if plugin lives on targetTrack's linear pluginList or inside a RackInstance on that track. */
bool pluginBelongsToTrackGraph (te::Track& targetTrack, te::Plugin& plugin)
{
    const auto pid = plugin.itemID;

    for (auto* slot : targetTrack.pluginList.getPlugins())
    {
        if (slot == nullptr)
            continue;

        if (slot == &plugin)
            return true;

        if (auto* rack = dynamic_cast<te::RackInstance*> (slot))
            if (rack->type != nullptr && rack->type->getPluginForID (pid) == &plugin)
                return true;
    }

    return false;
}

bool vitTryChainNewRackPluginSerial (te::RackType& rackType, te::Plugin& newPlugin)
{
    const auto newId = newPlugin.itemID;
    std::unordered_set<std::string> tailSourceKeys;
    juce::Array<te::EditItemID> tailSources;

    for (auto* rc : rackType.getConnections())
    {
        if (rc == nullptr)
            continue;

        if (rc->destID.get().isValid())
            continue;

        const auto srcId = rc->sourceID.get();

        if (! srcId.isValid())
            continue;

        if (srcId == newId)
            continue;

        const auto key = srcId.toString().toStdString();

        if (tailSourceKeys.insert (key).second)
            tailSources.add (srcId);
    }

    if (tailSources.size() != 1)
        return false;

    const auto tailId = tailSources.getFirst();

    struct TailOutPinPair
    {
        int sourcePin = 0;
        int rackOutPin = 0;
    };

    juce::Array<TailOutPinPair> edges;

    for (auto* rc : rackType.getConnections())
    {
        if (rc == nullptr)
            continue;

        if (rc->destID.get().isValid())
            continue;

        if (rc->sourceID.get() != tailId)
            continue;

        edges.add ({ rc->sourcePin.get(), rc->destPin.get() });
    }

    if (edges.isEmpty())
        return false;

    const te::EditItemID rackBus {};

    // Tracktion uses pin 0 for MIDI and pins 1..N for audio. If any tail->rack pin cannot reach the new
    // plugin (e.g. MIDI pin while the new FX has no MIDI input), aborting the whole chain left audio
    // disconnected too; only chain pins that are legal end-to-end.
    juce::Array<TailOutPinPair> legalEdges;

    for (const auto& e : edges)
        if (rackType.isConnectionLegal (tailId, e.sourcePin, newId, e.sourcePin)
            && rackType.isConnectionLegal (newId, e.sourcePin, rackBus, e.rackOutPin))
            legalEdges.add (e);

    if (legalEdges.isEmpty())
    {
        juce::Logger::writeToLog ("CommandDispatcher: rack serial chain no legal pins tail="
                                  + tailId.toString()
                                  + " -> "
                                  + newPlugin.getName()
                                  + " (total tail→rack pins="
                                  + juce::String (edges.size())
                                  + ")");
        return false;
    }

    if (legalEdges.size() < edges.size())
        juce::Logger::writeToLog ("CommandDispatcher: rack serial chain partial pins tail="
                                  + tailId.toString()
                                  + " -> "
                                  + newPlugin.getName()
                                  + " chaining "
                                  + juce::String (legalEdges.size())
                                  + " of "
                                  + juce::String (edges.size())
                                  + " tail→rack pins");

    for (const auto& e : legalEdges)
        if (! rackType.removeConnection (tailId, e.sourcePin, rackBus, e.rackOutPin))
            return false;

    for (const auto& e : legalEdges)
    {
        if (! rackType.addConnection (tailId, e.sourcePin, newId, e.sourcePin))
            return false;

        if (! rackType.addConnection (newId, e.sourcePin, rackBus, e.rackOutPin))
            return false;
    }

    return true;
}

te::RackInstance* findRackInstanceOnTrack (te::Track& track, const juce::String& rackItemId)
{
    const auto trimmed = rackItemId.trim();
    te::EditItemID parsedId;

    if (trimmed.isNotEmpty())
        parsedId = te::EditItemID::fromString (trimmed);

    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* rack = dynamic_cast<te::RackInstance*> (plugin))
            if (! parsedId.isValid() || rack->itemID == parsedId)
                return rack;

    return nullptr;
}

te::RackInstance* findUsableRackInstanceOnTrack (te::Track& track, const juce::String& rackItemId)
{
    const auto trimmed = rackItemId.trim();
    te::EditItemID parsedId;

    if (trimmed.isNotEmpty())
        parsedId = te::EditItemID::fromString (trimmed);

    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* rack = dynamic_cast<te::RackInstance*> (plugin))
            if ((! parsedId.isValid() || rack->itemID == parsedId) && rack->type != nullptr)
                return rack;

    return nullptr;
}

int fallbackRackInsertionIndex (te::Track& track)
{
    int index = 0;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        if (dynamic_cast<te::VolumeAndPanPlugin*> (plugin) != nullptr
            || dynamic_cast<te::LevelMeterPlugin*> (plugin) != nullptr)
            return index;

        ++index;
    }

    return -1;
}

te::RackInstance* ensureRackInstanceOnTrack (te::Track& track)
{
    auto& edit = track.pluginList.getEdit();

    auto settleRack = [&]() -> te::RackInstance*
    {
        edit.dispatchPendingUpdatesSynchronously();
        edit.getTransport().ensureContextAllocated (true);
        return findUsableRackInstanceOnTrack (track, {});
    };

    if (auto* existingRack = findUsableRackInstanceOnTrack (track, {}))
        return existingRack;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        auto* rack = dynamic_cast<te::RackInstance*> (plugin);
        if (rack == nullptr || rack->type != nullptr)
            continue;

        rack->removeFromParent();
        juce::Logger::writeToLog ("CommandDispatcher: removed broken rack instance without rack type on track "
                                  + track.itemID.toString());
    }

    if (auto* settledRack = settleRack())
        return settledRack;

    if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (&track))
    {
        if (ensureSingleRackForTrack (*audioTrack))
            if (auto* settledRack = settleRack())
                return settledRack;

        if (auto rackType = edit.getRackList().addNewRack())
        {
            if (audioTrack->pluginList.insertPlugin (te::RackInstance::create (*rackType),
                                                     fallbackRackInsertionIndex (*audioTrack)))
            {
                audioTrack->flushStateToValueTree();
                juce::Logger::writeToLog ("CommandDispatcher: fallback inserted rack instance on track "
                                          + track.itemID.toString() + " rack=" + rackType->itemID.toString());
                return settleRack();
            }

            juce::Logger::writeToLog ("CommandDispatcher: fallback failed to insert rack plugin on track "
                                      + track.itemID.toString());
        }
        else
        {
            juce::Logger::writeToLog ("CommandDispatcher: fallback failed to create rack type for track "
                                      + track.itemID.toString());
        }
    }

    return nullptr;
}

juce::ValueTree findRackPluginInstanceState (te::RackType& rackType, te::EditItemID pluginItemId)
{
    if (! pluginItemId.isValid())
        return {};

    for (auto child : rackType.state)
        if (child.hasType (te::IDs::PLUGININSTANCE))
            if (auto pluginState = child.getChildWithName (te::IDs::PLUGIN); pluginState.isValid())
                if (te::EditItemID::fromID (pluginState) == pluginItemId)
                    return child;

    return {};
}

juce::Result resolveExternalPluginDescription (te::Edit& edit, const juce::String& pluginPath, juce::PluginDescription& outDesc)
{
    juce::File pluginFile (pluginPath);

    if (! pluginFile.exists())
        return juce::Result::fail ("plugin_path does not exist: " + pluginPath);

    auto& pluginManager = edit.engine.getPluginManager();
    auto& knownPluginList = pluginManager.knownPluginList;
    const auto targetCanon = pluginFile.getFullPathName();

    for (const auto& desc : knownPluginList.getTypes())
    {
        if (desc.fileOrIdentifier.equalsIgnoreCase (targetCanon))
        {
            outDesc = desc;
            normaliseAndRegisterExternalPluginDescription (edit, outDesc, targetCanon, "resolveExternalPluginDescription/cache-exact");
            return juce::Result::ok();
        }

        juce::File descFile (desc.fileOrIdentifier);

        if (descFile.getFullPathName().equalsIgnoreCase (targetCanon))
        {
            outDesc = desc;
            normaliseAndRegisterExternalPluginDescription (edit, outDesc, targetCanon, "resolveExternalPluginDescription/cache-canon");
            return juce::Result::ok();
        }
    }

    juce::AudioPluginFormat* vst3Format = nullptr;

    for (int i = 0; i < pluginManager.pluginFormatManager.getNumFormats(); ++i)
    {
        auto* f = pluginManager.pluginFormatManager.getFormat (i);

        if (f != nullptr && f->getName() == "VST3")
        {
            vst3Format = f;
            break;
        }
    }

    if (vst3Format == nullptr)
        return juce::Result::fail ("VST3 format not available");

    juce::OwnedArray<juce::PluginDescription> discovered;
    vst3Format->findAllTypesForFile (discovered, pluginFile.getFullPathName());

    if (discovered.isEmpty())
        return juce::Result::fail ("Plugin not in cache and VST3 introspection failed (check path). CLAP is not implemented in rack_add_node yet.");

    if (auto* first = discovered.getFirst())
    {
        outDesc = *first;
        normaliseAndRegisterExternalPluginDescription (edit, outDesc, targetCanon, "resolveExternalPluginDescription/introspection");
        return juce::Result::ok();
    }

    return juce::Result::fail ("Failed to resolve plugin description");
}

juce::String inferPluginFormatNameFromPath (const juce::String& pluginPath)
{
    const auto lower = pluginPath.toLowerCase();

    if (lower.endsWith (".vst3"))
        return "VST3";

    if (lower.endsWith (".vst") || lower.endsWith (".dll"))
        return "VST";

    if (lower.startsWith ("audiounit:"))
        return "AudioUnit";

    return {};
}

void normaliseAndRegisterExternalPluginDescription (te::Edit& edit,
                                                    juce::PluginDescription& desc,
                                                    const juce::String& fallbackPath,
                                                    const juce::String& context)
{
    if (desc.fileOrIdentifier.isEmpty())
        desc.fileOrIdentifier = fallbackPath;

    if (desc.pluginFormatName.isEmpty())
        desc.pluginFormatName = inferPluginFormatNameFromPath (desc.fileOrIdentifier);

    auto& knownPluginList = edit.engine.getPluginManager().knownPluginList;
    const auto beforeCount = knownPluginList.getNumTypes();
    const auto added = knownPluginList.addType (desc);
    const auto identifier = te::createIdentifierString (desc);

    juce::Logger::writeToLog ("CommandDispatcher: " + context
                              + " plugin desc name=" + desc.name
                              + " format=" + desc.pluginFormatName
                              + " path=" + desc.fileOrIdentifier
                              + " uniqueId=" + juce::String (desc.uniqueId)
                              + " uid=" + juce::String (desc.deprecatedUid)
                              + " identifier=" + identifier
                              + " known_before=" + juce::String (beforeCount)
                              + " known_after=" + juce::String (knownPluginList.getNumTypes())
                              + " added=" + juce::String (added ? "true" : "false"));
}

te::Plugin* findPluginInEdit (te::Edit& edit, const juce::String& pluginIdStr)
{
    const auto trimmed = pluginIdStr.trim();
    const auto parsedId = te::EditItemID::fromString (trimmed);

    if (! parsedId.isValid())
        return nullptr;

    if (auto ptr = edit.getPluginCache().getPluginFor (parsedId))
        return ptr.get();

    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr)
            if (auto* found = findPluginByID (*track, trimmed))
                return found;

    return nullptr;
}

juce::String describeExternalPluginLoadState (te::Edit& edit, te::ExternalPlugin& plugin)
{
    const auto& desc = plugin.desc;
    auto loadError = plugin.getLoadError();

    if (loadError.isEmpty() && plugin.isInitialisingAsync())
        loadError = "async initialisation pending";

    return "name=" + plugin.getName()
           + " id=" + pluginItemIdString (plugin)
           + " format=" + desc.pluginFormatName
           + " path=" + desc.fileOrIdentifier
           + " enabled=" + juce::String (plugin.isEnabled() ? "true" : "false")
           + " processing=" + juce::String (plugin.isProcessingEnabled() ? "true" : "false")
           + " edit_should_load_plugins=" + juce::String (edit.shouldLoadPlugins() ? "true" : "false")
           + " async=" + juce::String (plugin.isInitialisingAsync() ? "true" : "false")
           + " load_error=" + loadError;
}

bool ensureExternalPluginInstanceReady (te::Edit& edit,
                                        te::ExternalPlugin& plugin,
                                        const juce::String& context,
                                        int timeoutMs)
{
    juce::ignoreUnused (timeoutMs);

    if (plugin.getAudioPluginInstance() != nullptr)
        return true;

    juce::Logger::writeToLog ("CommandDispatcher: " + context + " ensure instance begin "
                              + describeExternalPluginLoadState (edit, plugin));

    plugin.setProcessingEnabled (true);
    plugin.initialiseFully();
    edit.dispatchPendingUpdatesSynchronously();
    edit.getTransport().ensureContextAllocated (true);

    const bool ready = plugin.getAudioPluginInstance() != nullptr;
    juce::Logger::writeToLog ("CommandDispatcher: " + context
                              + (ready ? " ensure instance ready " : " ensure instance failed ")
                              + describeExternalPluginLoadState (edit, plugin));
    return ready;
}

juce::var createRackState (te::Track& track, const juce::String& requestScope)
{
    auto* rack = findUsableRackInstanceOnTrack (track, {});

    if (rack == nullptr || rack->type == nullptr)
        return juce::var();

    auto rackObject = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> nodes;
    juce::Array<juce::var> edges;
	const auto audioReachableNodeIds = collectAudioReachableRackNodeIds (*rack->type);

    for (auto* plugin : rack->type->getPlugins())
    {
        if (plugin == nullptr)
            continue;

        auto node = std::make_unique<juce::DynamicObject>();
        const auto nodeId = pluginItemIdString (*plugin);
        const auto pos = rack->type->getPluginPosition (te::Plugin::Ptr (plugin));
        const auto zoneId = VitGraphValidator::getZoneIdForNode (*rack->type, plugin->itemID, plugin);
        const auto clipScope = VitClipRouteRegistry::getNodeClipScope (*rack->type, plugin->itemID);
        const auto storedTemplateRole = plugin->state.getProperty ("vit_template_role").toString().trim().toLowerCase();
        const auto templateRole = storedTemplateRole.isNotEmpty() ? storedTemplateRole
                                                                  : VitPluginTemplateRegistry::inferTemplateRole (*plugin);
		const auto audioReachable = audioReachableNodeIds.count (nodeId.toStdString()) > 0;

        node->setProperty ("node_id", nodeId);
        node->setProperty ("plugin_item_id", nodeId);
        node->setProperty ("name", plugin->getName().trim());
        node->setProperty ("type", plugin->getPluginType());
        node->setProperty ("enabled", plugin->isEnabled());
        node->setProperty ("x", pos.x);
        node->setProperty ("y", pos.y);
        node->setProperty ("zone_id", zoneId);
        node->setProperty ("clip_scope", clipScope);
        node->setProperty ("template_role", templateRole);
		node->setProperty ("audio_reachable_from_rack_input", audioReachable);
		node->setProperty ("vit_orphan_bypass_candidate", zoneId == "Z3" && ! audioReachable);
        node->setProperty ("supports_param_grabber", dynamic_cast<te::ExternalPlugin*> (plugin) != nullptr);
        nodes.add (juce::var (node.release()));
    }

    for (auto* connection : rack->type->getConnections())
    {
        if (connection == nullptr)
            continue;

        const auto srcRaw = connection->sourceID.get();
        const auto dstRaw = connection->destID.get();
        const bool srcBus = ! srcRaw.isValid();
        const bool dstBus = ! dstRaw.isValid();

        if (srcBus && dstBus)
            continue;

        const juce::String sourceId = srcBus ? juce::String ("RACK_INPUT") : srcRaw.toString();
        const juce::String destId = dstBus ? juce::String ("RACK_OUTPUT") : dstRaw.toString();

        auto edge = std::make_unique<juce::DynamicObject>();
        const auto sourcePin = connection->sourcePin.get();
        const auto destPin = connection->destPin.get();
        edge->setProperty ("source_id", sourceId);
        edge->setProperty ("source_pin", connection->sourcePin.get());
        edge->setProperty ("dest_id", destId);
        edge->setProperty ("dest_pin", connection->destPin.get());
        edge->setProperty ("edge_id", sourceId + "|" + juce::String (sourcePin) + "|" + destId + "|" + juce::String (destPin));
        edges.add (juce::var (edge.release()));
    }

    VitClipRouteRegistry::annotateNodesAndEdges (track, *rack->type, nodes, edges, requestScope);

    rackObject->setProperty ("rack_item_id", rack->itemID.toString());
    rackObject->setProperty ("scope", requestScope);
    rackObject->setProperty ("nodes", juce::var (nodes));
    rackObject->setProperty ("edges", juce::var (edges));
    rackObject->setProperty ("clip_proxy_nodes", juce::var (VitClipRouteRegistry::createClipProxyNodes (track, requestScope)));
    rackObject->setProperty ("clip_routes", juce::var (VitClipRouteRegistry::createClipRoutes (track, *rack->type, requestScope)));
    return juce::var (rackObject.release());
}

juce::var createControlGraphState (te::Edit& edit)
{
    auto snapshot = VitParamLinkGraph::createSnapshot (edit);
    if (auto* object = snapshot.getDynamicObject())
        object->setProperty ("bindings", juce::var (createControlBindingsSnapshot (edit)));
    return snapshot;
}

void appendGraphRevisionProperties (juce::DynamicObject& object, te::Edit& edit)
{
    appendGraphRevisionProperties (object, VitGraphSwapCoordinator::getSnapshot (edit));
}

void appendGraphRevisionProperties (juce::DynamicObject& object, const VitGraphRevisionSnapshot& snapshot)
{
    object.setProperty ("graph_revision", snapshot.revision);
    object.setProperty ("graph_active_revision", snapshot.activeRevision);
    object.setProperty ("graph_pending_revision", snapshot.pendingRevision);
    object.setProperty ("graph_last_retired_revision", snapshot.lastRetiredRevision);
    object.setProperty ("graph_retired_snapshot_count", snapshot.retiredSnapshotCount);
    object.setProperty ("graph_last_diff_kind", snapshot.lastKind);
    object.setProperty ("graph_last_diff_summary", snapshot.lastSummary);
    object.setProperty ("graph_publish_mode", snapshot.publishMode);
    object.setProperty ("graph_lifecycle_state", snapshot.lifecycleState);
    object.setProperty ("graph_recent_changes", juce::var (snapshot.recentChanges));
}

juce::File getEffectiveProjectFile (const CommandDispatcher::CurrentProjectPathGetter& getter)
{
    if (getter != nullptr)
    {
        const auto currentPath = getter();
        if (currentPath.isNotEmpty())
            return juce::File (currentPath);
    }

    return paths::ensureDefaultProjectXmlFileExists();
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
    }

    return juce::JSON::toString (juce::var (response.release()));
}

float convertRequestedDbToPluginDb (double requestedDb)
{
    constexpr float minimumVolumeDb = -100.0f;
    const auto gain = juce::Decibels::decibelsToGain (static_cast<float> (requestedDb), minimumVolumeDb);
    return juce::Decibels::gainToDecibels (gain, minimumVolumeDb);
}

static const juce::Identifier kVitNoteId ("vit_note_id");

te::MidiNote* findMidiNoteByVitId (te::MidiList& list, const juce::String& id)
{
    if (id.isEmpty())
        return nullptr;

    for (auto* note : list.getNotes())
        if (note != nullptr && note->state.getProperty (kVitNoteId).toString() == id)
            return note;

    return nullptr;
}

bool tryReadMidiInt (const juce::DynamicObject& o, const juce::String& key, int& out)
{
    if (! o.hasProperty (key))
        return false;

    const auto v = o.getProperty (key);

    if (v.isInt() || v.isInt64())
    {
        out = static_cast<int> (v);
        return true;
    }

    if (v.isDouble())
    {
        out = static_cast<int> (std::lround (static_cast<double> (v)));
        return true;
    }

    return false;
}

/** Clip-local start/length: either beats (relative to clip content) or seconds along the clip timeline. */
bool clipLocalNoteGeometryToBeats (te::Edit& edit,
                                   te::MidiClip& clip,
                                   CommandTimeUnit unit,
                                   double startVal,
                                   double lengthVal,
                                   double& outStartBeats,
                                   double& outLenBeats,
                                   juce::String& error)
{
    if (unit == CommandTimeUnit::beats)
    {
        if (lengthVal <= 0.0)
        {
            error = "note length must be greater than zero";
            return false;
        }

        outStartBeats = startVal;
        outLenBeats = lengthVal;
        return true;
    }

    if (lengthVal <= 0.0)
    {
        error = "note length in seconds must be greater than zero";
        return false;
    }

    const auto clipStart = clip.getPosition().getStart();
    const auto clipStartBeat = edit.tempoSequence.toBeats (clipStart);
    const auto noteStartTime = clipStart + te::TimeDuration::fromSeconds (startVal);
    const auto noteEndTime = noteStartTime + te::TimeDuration::fromSeconds (lengthVal);
    const auto noteStartBeat = edit.tempoSequence.toBeats (noteStartTime);
    const auto noteEndBeat = edit.tempoSequence.toBeats (noteEndTime);
    outStartBeats = noteStartBeat.inBeats() - clipStartBeat.inBeats();
    outLenBeats = noteEndBeat.inBeats() - noteStartBeat.inBeats();

    if (outLenBeats <= 0.0)
    {
        error = "note length conversion produced non-positive length";
        return false;
    }

    return true;
}

} // namespace

CommandDispatcher::CommandDispatcher (EditGetter editGetter,
                                      BoolAction reloadProjectAction,
                                      BoolAction saveProjectAction,
                                      PublishAction publishAction,
                                      RecentProjectsReply recentProjectsReplyAction,
                                      NewBlankProjectReply newBlankProjectReplyAction,
                                      OpenProjectReply openProjectReplyAction,
                                      SaveProjectReply saveProjectReplyAction,
                                      SaveAsProjectReply saveAsProjectReplyAction,
                                      CurrentProjectPathGetter currentProjectPathGetterAction,
                                      VitProductionCoordinator* productionCoordinator)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      publishMessage (std::move (publishAction)),
      getCurrentProjectPath (std::move (currentProjectPathGetterAction)),
      production (productionCoordinator)
{
    importService = std::make_unique<ImportService> (getEdit, saveProject, publishMessage);
    generatedAssetService = std::make_unique<GeneratedAssetService> (getEdit,
                                                                     saveProject,
                                                                     getCurrentProjectPath,
                                                                     importService.get());
    jobEventService = std::make_unique<JobEventService> (getCurrentProjectPath);
    projectService = std::make_unique<ProjectService> (std::move (reloadProjectAction),
                                                       std::move (recentProjectsReplyAction),
                                                       std::move (newBlankProjectReplyAction),
                                                       std::move (openProjectReplyAction),
                                                       std::move (saveProjectReplyAction),
                                                       std::move (saveAsProjectReplyAction));
    trackService = std::make_unique<TrackService> (getEdit, saveProject);
    clipService = std::make_unique<ClipService> (getEdit);
    midiService = std::make_unique<MidiService> (getEdit);
    transportAudioService = std::make_unique<TransportAudioService> (getEdit, saveProject, production);
    pluginRackControlService = std::make_unique<PluginRackControlService> (getEdit, saveProject, getCurrentProjectPath);
    registerBuiltinCommands();
}

CommandDispatcher::~CommandDispatcher() = default;

juce::String CommandDispatcher::dispatch (const juce::var& command, const juce::String& rawPayload) const
{
    juce::Logger::writeToLog ("Command received: " + rawPayload);

    juce::ignoreUnused (rawPayload);

    auto* object = command.getDynamicObject();

    if (object == nullptr)
        return makeErrorReply ("JSON payload must be an object with a cmd field");

    auto cmd = object->getProperty ("cmd").toString().trim();

    if (cmd.isEmpty())
        cmd = object->getProperty ("action").toString().trim();

    if (cmd.isEmpty())
        cmd = object->getProperty ("command").toString().trim();

    if (cmd.isEmpty())
        return makeErrorReply ("Missing cmd/action field");

    if (production != nullptr && production->isRendering())
    {
        static const std::unordered_set<std::string> renderAllowlist {
            "ping",
            "get_project_state",
            "list_tracks",
            "get_recent_projects",
            "get_midi_clip_notes",
            "get_midi_clip_data",
            "get_plugin_parameters",
            "project_health_check",
            "get_audio_device_types",
            "get_audio_devices",
            "get_wave_input_devices",
            "cancel_render",
        };

        if (renderAllowlist.find (cmd.toStdString()) == renderAllowlist.end())
            return makeErrorReply ("Engine is busy rendering");
    }

    if (const auto it = handlers.find (cmd.toStdString()); it != handlers.end())
    {
        auto result = it->second (*object, rawPayload);
        juce::Logger::writeToLog ("Command executed: " + cmd + ", result: " + result);
        return result;
    }

    return makeErrorReply ("Unknown command: " + cmd);
}

void CommandDispatcher::registerBuiltinCommands()
{
    handlers.emplace ("ping", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handlePing (object, raw);
    });

    handlers.emplace ("reload_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleReloadProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("new_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleNewProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("get_recent_projects", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleGetRecentProjects (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("open_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleOpenProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("list_tracks", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleListTracks (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("get_project_state", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetProjectState (object, raw);
    });

    handlers.emplace ("set_tempo", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetTempo (object, raw);
    });

    handlers.emplace ("append_ghost_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleAppendGhostTrack (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("add_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleAddTrack (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("add_audio_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleAddTrack (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("delete_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleDeleteTrack (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });
    handlers.emplace ("rename_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleRenameTrack (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("add_audio_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return importService != nullptr ? importService->handleAddAudioClip (object, raw)
                                        : makeErrorReply ("Import service unavailable");
    });

    handlers.emplace ("move_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return clipService != nullptr ? clipService->handleMoveClip (object, raw)
                                      : makeErrorReply ("Clip service unavailable");
    });

    handlers.emplace ("clone_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return clipService != nullptr ? clipService->handleCloneClip (object, raw)
                                      : makeErrorReply ("Clip service unavailable");
    });

    handlers.emplace ("resize_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return clipService != nullptr ? clipService->handleResizeClip (object, raw)
                                      : makeErrorReply ("Clip service unavailable");
    });
    handlers.emplace ("split_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return clipService != nullptr ? clipService->handleSplitClip (object, raw)
                                      : makeErrorReply ("Clip service unavailable");
    });

    handlers.emplace ("add_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleAddMidiNotes (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("add_midi_notes_bulk", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleAddMidiNotesBulk (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("mutate_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleMutateMidiNotes (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("delete_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleDeleteMidiNotes (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("get_midi_clip_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleGetMidiClipNotes (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("get_midi_clip_data", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleGetMidiClipData (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("insert_midi_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleInsertMidiClip (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("create_midi_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return midiService != nullptr ? midiService->handleInsertMidiClip (object, raw)
                                      : makeErrorReply ("MIDI service unavailable");
    });

    handlers.emplace ("remove_clips", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return clipService != nullptr ? clipService->handleRemoveClips (object, raw)
                                      : makeErrorReply ("Clip service unavailable");
    });

    handlers.emplace ("import_audio", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return importService != nullptr ? importService->handleImportAudio (object, raw)
                                        : makeErrorReply ("Import service unavailable");
    });
    handlers.emplace ("import_media_to_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return importService != nullptr ? importService->handleImportMediaToTrack (object, raw)
                                        : makeErrorReply ("Import service unavailable");
    });
    handlers.emplace ("warm_waveform_bake", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return importService != nullptr ? importService->handleWarmWaveformBake (object, raw)
                                        : makeErrorReply ("Import service unavailable");
    });
    handlers.emplace ("aigc_register_job", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return jobEventService != nullptr ? jobEventService->handleAigcRegisterJob (object, raw)
                                          : makeErrorReply ("Job event service unavailable");
    });
    handlers.emplace ("bridge_ingest_generated_asset", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return generatedAssetService != nullptr ? generatedAssetService->handleBridgeIngestGeneratedAsset (object, raw)
                                                : makeErrorReply ("Generated asset service unavailable");
    });
    handlers.emplace ("switch_asset_take", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return generatedAssetService != nullptr ? generatedAssetService->handleSwitchAssetTake (object, raw)
                                                : makeErrorReply ("Generated asset service unavailable");
    });
    handlers.emplace ("set_async_ghost_state", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return generatedAssetService != nullptr ? generatedAssetService->handleSetAsyncGhostState (object, raw)
                                                : makeErrorReply ("Generated asset service unavailable");
    });

    handlers.emplace ("set_volume", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleSetVolume (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });
    handlers.emplace ("set_pan", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleSetPan (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });

    handlers.emplace ("set_mute", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleSetMute (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });
    handlers.emplace ("set_solo", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return trackService != nullptr ? trackService->handleSetSolo (object, raw)
                                       : makeErrorReply ("Track service unavailable");
    });
    handlers.emplace ("set_plugin_param", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleSetPluginParam (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("set_plugin_param_aliases", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleSetPluginParamAliases (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_add_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlAddNode (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_add_macro", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlAddMacro (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_add_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlAddBinding (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_update_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlUpdateNode (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_set_node_value", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlSetNodeValue (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_update_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlUpdateBinding (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_remove_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlRemoveBinding (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_remove_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlRemoveNode (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("control_set_macro_values", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleControlSetMacroValues (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("connector_upsert_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleConnectorUpsertProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("connector_remove_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleConnectorRemoveProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
    handlers.emplace ("project_health_check", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleProjectHealthCheck (object, raw);
    });

    handlers.emplace ("play", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handlePlay (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("stop", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleStop (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("return_to_zero", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleReturnToZero (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("transport_set_loop", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleSetLoop (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("transport_clear_loop", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleClearLoop (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("transport_option_stop_return_to_start", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleTransportOptionStopReturnToStart (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("toggle_click", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleToggleClick (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("set_click", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleSetClick (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("seek", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleSeek (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("clear_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleClearProject (object, raw);
    });

    handlers.emplace ("undo", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleUndo (object, raw);
    });

    handlers.emplace ("redo", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRedo (object, raw);
    });

    handlers.emplace ("save_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleSaveProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("load_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleOpenProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("save_as_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return projectService != nullptr ? projectService->handleSaveAsProject (object, raw)
                                         : makeErrorReply ("Project service unavailable");
    });

    handlers.emplace ("get_audio_device_types", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleGetAudioDeviceTypes (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("get_audio_devices", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleGetAudioDevices (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("set_audio_device", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleSetAudioDevice (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("get_wave_input_devices", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleGetWaveInputDevices (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("route_wave_input_to_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleRouteWaveInputToTrack (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("scan_plugins", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleScanPlugins (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_list_available", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleListPlugins (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_search", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleSearchPlugins (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("instantiate_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleInstantiatePlugin (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("show_plugin_editor", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleOpenPluginUI (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("open_plugin_ui", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleOpenPluginUI (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("get_plugin_parameters", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleGetPluginParameters (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("delete_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleDeletePlugin (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("move_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleMovePlugin (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_add_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackAddNode (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_connect_pins", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackConnectPins (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_remove_connection", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackRemoveConnection (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_set_node_clip_scope", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackSetNodeClipScope (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_add_edge", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackConnectPins (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("rack_remove_edge", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handleRackRemoveConnection (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("arm_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleArmTrack (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("start_recording", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleStartRecording (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("stop_recording", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleStopRecording (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("freeze_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleFreezeTrack (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("unfreeze_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleUnfreezeTrack (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("start_render", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleStartRender (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("l2_render_probe", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleL2RenderProbe (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("cancel_render", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return transportAudioService != nullptr ? transportAudioService->handleCancelRender (object, raw)
                                             : makeErrorReply ("Transport/audio service unavailable");
    });

    handlers.emplace ("n_project_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberUpsertProjectProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_grabber_upsert_project_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberUpsertProjectProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("n_get_project_profiles", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberGetProjectProfiles (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_grabber_get_project_profiles", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberGetProjectProfiles (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("n_remove_project_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberRemoveProjectProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_grabber_remove_project_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberRemoveProjectProfile (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("plugin_grabber_apply_control", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberApplyControl (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });

    handlers.emplace ("n_apply_control", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return pluginRackControlService != nullptr ? pluginRackControlService->handlePluginGrabberApplyControl (object, raw)
                                             : makeErrorReply ("Plugin/rack/control service unavailable");
    });
}

juce::String CommandDispatcher::handlePing (const juce::DynamicObject&, const juce::String&) const
{
    std::cout << "[ZMQ] Received ping, system is alive." << std::endl;
    juce::Logger::writeToLog ("CommandDispatcher: handled ping command.");
    return makeStatusReply ("ok", "pong");
}

juce::String CommandDispatcher::handleUndo (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& um = edit->getUndoManager();

    if (! um.canUndo())
        return makeErrorReply ("Nothing to undo");

    um.undo();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);
    return makeStatusReply ("ok", "Undo executed");
}

juce::String CommandDispatcher::handleRedo (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& um = edit->getUndoManager();

    if (! um.canRedo())
        return makeErrorReply ("Nothing to redo");

    um.redo();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);
    return makeStatusReply ("ok", "Redo executed");
}

juce::String CommandDispatcher::handleGetProjectState (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active project");

    if (ensureTrackRackGraphForEdit (*edit))
    {
        edit->dispatchPendingUpdatesSynchronously();
        edit->getTransport().ensureContextAllocated (true);
    }

    juce::String projectPath;

    if (getCurrentProjectPath != nullptr)
        projectPath = getCurrentProjectPath();

    const auto effectiveProjectFile = getEffectiveProjectFile (getCurrentProjectPath);
    VitMediaPoolManager::syncProjectAssetsFromEdit (effectiveProjectFile, *edit);

    const auto requestScope = VitClipRouteRegistry::normaliseClipScope (object.getProperty ("scope").toString().trim().isNotEmpty()
                                                                            ? object.getProperty ("scope").toString()
                                                                            : object.getProperty ("clip_scope").toString());
    auto response = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> tracksArray;

    for (auto* track : te::getAllTracks (*edit))
    {
        if (track == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("track_id", track->itemID.toString());
        row->setProperty ("track_name", track->getName());
        row->setProperty ("track_type", getTrackKind (*track));
        row->setProperty ("is_audio_track", track->isAudioTrack());
        row->setProperty ("is_audio", track->isAudioTrack());
        row->setProperty ("plugins", juce::var (createProjectStatePluginsArray (*track)));
        row->setProperty ("clips", juce::var (createProjectStateClipsArray (*track)));
        row->setProperty ("rack", createRackState (*track, requestScope));

        if (auto* at = dynamic_cast<te::AudioTrack*> (track))
        {
            row->setProperty ("mute", at->isMuted (false));
            row->setProperty ("solo", at->isSolo (false));

            if (auto* volumePlugin = at->getVolumePlugin())
            {
                const auto db = volumePlugin->getVolumeDb();
                row->setProperty ("db", db);
                row->setProperty ("volume_db", db);
                row->setProperty ("gain_db", db);
                row->setProperty ("fader_db", db);
                row->setProperty ("pan", volumePlugin->getPan());
                row->setProperty ("pan_value", volumePlugin->getPan());
            }

            bool isArmed = false;
            for (auto* input : edit->getAllInputDevices())
            {
                if (input != nullptr && input->isRecordingEnabled (at->itemID))
                {
                    isArmed = true;
                    break;
                }
            }

            row->setProperty ("is_armed", isArmed);
            row->setProperty ("armed", isArmed);

            auto& rout = at->getOutput();

            if (auto* dev = rout.getOutputDevice (false))
            {
                row->setProperty ("output_to_device_resolved", true);
                row->setProperty ("output_device_enabled", dev->isEnabled());
                row->setProperty ("output_device_id", dev->getDeviceID());
            }
            else
            {
                row->setProperty ("output_to_device_resolved", false);
                row->setProperty ("output_device_enabled", false);
                row->setProperty ("output_device_id", juce::var());
            }

            row->setProperty ("output_routing_name", rout.getOutputName());
        }

        tracksArray.add (juce::var (row.release()));
    }

    response->setProperty ("status", "ok");
    response->setProperty ("project_path", projectPath);
    response->setProperty ("scope", requestScope);
    response->setProperty ("tracks", juce::var (tracksArray));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    response->setProperty ("node_registry", VitNodeRegistry::createSnapshot (effectiveProjectFile));
    response->setProperty ("connector_profiles", juce::var (VitConnectorPluginSpec::snapshotProfiles (effectiveProjectFile)));
    response->setProperty ("connector_specs", juce::var (VitConnectorPluginSpec::createBuiltInSpecs()));
    response->setProperty ("generated_assets", juce::var (VitMediaPoolManager::snapshotProjectAssets (effectiveProjectFile)));
    response->setProperty ("jobs", juce::var (VitAIGCJobRuntime::snapshotProjectJobs (effectiveProjectFile)));
    response->setProperty ("take_histories", juce::var (VitTakeHistoryStack::snapshotProjectStacks (effectiveProjectFile)));
    response->setProperty ("observability", VitGraphTrace::createObservabilitySnapshot (effectiveProjectFile, *edit));
    response->setProperty ("project_health", VitProjectHealthCheck::createReport (effectiveProjectFile, *edit));
    response->setProperty ("export_policy", VitProjectHealthCheck::createExportPolicy (effectiveProjectFile, *edit));
    appendGraphRevisionProperties (*response, *edit);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSetTempo (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto bpmVar = object.getProperty ("bpm");

    if (! bpmVar.isDouble() && ! bpmVar.isInt() && ! bpmVar.isInt64())
        return makeErrorReply ("set_tempo requires a numeric bpm field");

    const auto bpm = static_cast<double> (bpmVar);

    if (bpm <= 0.0)
        return makeErrorReply ("bpm must be greater than zero");

    if (auto* tempo = edit->tempoSequence.getTempo (0))
        tempo->setBpm (bpm);
    else
        return makeErrorReply ("Unable to access master tempo");

    if (saveProject && ! saveProject())
        return makeErrorReply ("Tempo updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Tempo updated");
    response->setProperty ("bpm", bpm);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleProjectHealthCheck (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("observability", VitGraphTrace::createObservabilitySnapshot (projectFile, *edit));
    response->setProperty ("project_health", VitProjectHealthCheck::createReport (projectFile, *edit));
    response->setProperty ("export_policy", VitProjectHealthCheck::createExportPolicy (projectFile, *edit));
    appendGraphRevisionProperties (*response, *edit);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleClearProject (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    // 1. Stop transport and return to zero
    auto& transport = edit->getTransport();
    transport.stop (false, false);
    transport.setPosition (te::TimePosition::fromSeconds (0.0));

    // 2. Remove all clips from every audio track
    for (auto* track : te::getAllTracks (*edit))
    {
        auto* clipTrack = dynamic_cast<te::ClipTrack*> (track);
        if (clipTrack == nullptr)
            continue;

        auto clips = clipTrack->getClips();
        for (int i = clips.size() - 1; i >= 0; --i)
            clips[i]->removeFromParent();
    }

    // 3. Release all shared memory tile mappings
    for (auto* track : te::getAllTracks (*edit))
        if (track != nullptr)
            TiledSpectrogramBaker::releaseTrackMappings (track->itemID.toString());

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    transport.ensureContextAllocated (true);

    if (saveProject)
        saveProject();

    juce::Logger::writeToLog ("CommandDispatcher: clear_project executed - all clips removed, transport reset.");
    return makeStatusReply ("ok", "Project cleared");
}

juce::String CommandDispatcher::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
