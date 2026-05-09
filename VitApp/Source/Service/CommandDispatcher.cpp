#include "CommandDispatcher.h"

#include "TiledSpectrogramBaker.h"

#include "../Core/VitAIGCJobRuntime.h"
#include "../Core/VitAudioInjectorNode.h"
#include "../Core/VitAsyncGhostPolicy.h"
#include "../Core/VitBridgeNode.h"
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
#include "../Core/VitWarpBridgeNode.h"
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
juce::var createRackState (te::Track& track, const juce::String& requestScope);
juce::var createControlGraphState (te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, const VitGraphRevisionSnapshot& snapshot);
juce::File getEffectiveProjectFile (const CommandDispatcher::CurrentProjectPathGetter& getter);
void appendGeneratedAssetPropertiesToClip (juce::ValueTree& clipState,
                                           const VitGeneratedAssetRecord& assetRecord,
                                           const VitWarpDescriptor& warpDescriptor,
                                           juce::UndoManager* undoManager);
void appendTakePropertiesToClip (juce::ValueTree& clipState,
                                 const juce::String& takeStackId,
                                 const juce::String& activeTakeId,
                                 const juce::String& ghostState,
                                 juce::UndoManager* undoManager);
/** When RackType::addPlugin skips auto-connect (non-empty rack), chain new plugin after the unique tail feeding rack outputs. */
bool vitTryChainNewRackPluginSerial (te::RackType& rackType, te::Plugin& newPlugin);

enum class OverlapPolicy
{
    trim,
    layer,
    crossfade,
};

constexpr double kMinimumSurvivingClipLengthSeconds = 0.01;
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
		    && connection->sourcePin.get() >= 1
		    && connection->destPin.get() >= 1)
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

		if (connection->sourcePin.get() < 1 || connection->destPin.get() < 1)
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

/** get_project_state：与 createPluginNameArray 兼容，并补充 type/enabled，供前端机架与调试。 */
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

/** 遍历 ClipTrack 上的 Clip 类 TrackItem，输出时间线与标识（供前端/影子树对齐）。 */
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

struct AudioImportInsertResult
{
    bool ok = false;
    juce::String errorMessage;
    juce::String clipId;
    juce::String clipName;
    juce::String trackItemId;
    double startTimeSeconds = 0.0;
    double audioLengthSeconds = 0.0;
    double editLengthSeconds = 0.0;
};

/** Shared by import_audio / import_media_to_track: Undo transaction, insertWaveClip, spectrogram bake. */
AudioImportInsertResult insertWaveClipWithUndoAndStartBake (
    te::Edit& edit,
    te::AudioTrack& targetTrack,
    const juce::File& sourceFile,
    double startTimeSeconds,
    double audioLengthSeconds,
    bool deleteExistingClips,
    const juce::String& appliedMode,
    const TiledSpectrogramBaker::PublishCallback& publish,
    const CommandDispatcher::BoolAction& saveProjectAction)
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
                                      publish);

    if (saveProjectAction && ! saveProjectAction())
    {
        out.errorMessage = "Audio clip added in memory but failed to save project";
        return out;
    }

    out.ok = true;
    juce::ignoreUnused (appliedMode);
    return out;
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
            return juce::Result::ok();
        }

        juce::File descFile (desc.fileOrIdentifier);

        if (descFile.getFullPathName().equalsIgnoreCase (targetCanon))
        {
            outDesc = desc;
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
        return juce::Result::ok();
    }

    return juce::Result::fail ("Failed to resolve plugin description");
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

void appendGeneratedAssetPropertiesToClip (juce::ValueTree& clipState,
                                           const VitGeneratedAssetRecord& assetRecord,
                                           const VitWarpDescriptor& warpDescriptor,
                                           juce::UndoManager* undoManager)
{
    clipState.setProperty ("vit_asset_ref", assetRecord.assetRef, undoManager);
    clipState.setProperty ("vit_asset_hash", assetRecord.sourceHash, undoManager);
    clipState.setProperty ("vit_asset_relative_path", assetRecord.relativePath, undoManager);
    clipState.setProperty ("vit_asset_bucket", assetRecord.bucket, undoManager);
    clipState.setProperty ("vit_asset_kind", assetRecord.assetKind, undoManager);
    clipState.setProperty ("vit_asset_state", assetRecord.assetState, undoManager);
    clipState.setProperty ("vit_asset_lifecycle_state", assetRecord.lifecycleState, undoManager);
    clipState.setProperty ("vit_asset_job_id", assetRecord.jobId, undoManager);
    clipState.setProperty ("vit_node_role", VitAudioInjectorNode::defaultNodeRole(), undoManager);
    VitWarpBridgeNode::applyClipProperties (clipState, warpDescriptor, undoManager);
}

void appendTakePropertiesToClip (juce::ValueTree& clipState,
                                 const juce::String& takeStackId,
                                 const juce::String& activeTakeId,
                                 const juce::String& ghostState,
                                 juce::UndoManager* undoManager)
{
    clipState.setProperty ("vit_take_stack_id", takeStackId, undoManager);
    clipState.setProperty ("vit_active_take_id", activeTakeId, undoManager);
    clipState.setProperty ("vit_async_ghost_state", VitAsyncGhostPolicy::normalise (ghostState), undoManager);
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
                                      CurrentProjectPathGetter currentProjectPathGetterAction)
    : getEdit (std::move (editGetter)),
      reloadProject (std::move (reloadProjectAction)),
      saveProject (std::move (saveProjectAction)),
      publishMessage (std::move (publishAction)),
      recentProjectsReply (std::move (recentProjectsReplyAction)),
      newBlankProjectReply (std::move (newBlankProjectReplyAction)),
      openProjectReply (std::move (openProjectReplyAction)),
      saveProjectReply (std::move (saveProjectReplyAction)),
      saveAsProjectReply (std::move (saveAsProjectReplyAction)),
      getCurrentProjectPath (std::move (currentProjectPathGetterAction))
{
    registerBuiltinCommands();
}

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
        return handleReloadProject (object, raw);
    });

    handlers.emplace ("new_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleNewProject (object, raw);
    });

    handlers.emplace ("get_recent_projects", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetRecentProjects (object, raw);
    });

    handlers.emplace ("open_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleOpenProject (object, raw);
    });

    handlers.emplace ("list_tracks", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleListTracks (object, raw);
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
        return handleAppendGhostTrack (object, raw);
    });

    handlers.emplace ("add_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAddTrack (object, raw);
    });

    handlers.emplace ("add_audio_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAddTrack (object, raw);
    });

    handlers.emplace ("delete_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleDeleteTrack (object, raw);
    });

    handlers.emplace ("add_audio_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAddAudioClip (object, raw);
    });

    handlers.emplace ("move_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleMoveClip (object, raw);
    });

    handlers.emplace ("clone_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleCloneClip (object, raw);
    });

    handlers.emplace ("resize_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleResizeClip (object, raw);
    });

    handlers.emplace ("add_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAddMidiNotes (object, raw);
    });

    handlers.emplace ("add_midi_notes_bulk", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAddMidiNotesBulk (object, raw);
    });

    handlers.emplace ("mutate_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleMutateMidiNotes (object, raw);
    });

    handlers.emplace ("delete_midi_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleDeleteMidiNotes (object, raw);
    });

    handlers.emplace ("get_midi_clip_notes", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetMidiClipNotes (object, raw);
    });

    handlers.emplace ("get_midi_clip_data", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetMidiClipData (object, raw);
    });

    handlers.emplace ("insert_midi_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleInsertMidiClip (object, raw);
    });

    handlers.emplace ("create_midi_clip", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleInsertMidiClip (object, raw);
    });

    handlers.emplace ("remove_clips", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRemoveClips (object, raw);
    });

    handlers.emplace ("import_audio", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleImportAudio (object, raw);
    });
    handlers.emplace ("import_media_to_track", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleImportMediaToTrack (object, raw);
    });
    handlers.emplace ("warm_waveform_bake", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleWarmWaveformBake (object, raw);
    });
    handlers.emplace ("aigc_register_job", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleAigcRegisterJob (object, raw);
    });
    handlers.emplace ("bridge_ingest_generated_asset", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleBridgeIngestGeneratedAsset (object, raw);
    });
    handlers.emplace ("switch_asset_take", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSwitchAssetTake (object, raw);
    });
    handlers.emplace ("set_async_ghost_state", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetAsyncGhostState (object, raw);
    });

    handlers.emplace ("set_volume", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetVolume (object, raw);
    });

    handlers.emplace ("set_mute", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetMute (object, raw);
    });
    handlers.emplace ("set_plugin_param", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetPluginParam (object, raw);
    });
    handlers.emplace ("set_plugin_param_aliases", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetPluginParamAliases (object, raw);
    });
    handlers.emplace ("control_add_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlAddNode (object, raw);
    });
    handlers.emplace ("control_add_macro", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlAddMacro (object, raw);
    });
    handlers.emplace ("control_add_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlAddBinding (object, raw);
    });
    handlers.emplace ("control_update_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlUpdateNode (object, raw);
    });
    handlers.emplace ("control_set_node_value", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlSetNodeValue (object, raw);
    });
    handlers.emplace ("control_update_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlUpdateBinding (object, raw);
    });
    handlers.emplace ("control_remove_binding", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlRemoveBinding (object, raw);
    });
    handlers.emplace ("control_remove_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlRemoveNode (object, raw);
    });
    handlers.emplace ("control_set_macro_values", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleControlSetMacroValues (object, raw);
    });
    handlers.emplace ("connector_upsert_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleConnectorUpsertProfile (object, raw);
    });
    handlers.emplace ("connector_remove_profile", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleConnectorRemoveProfile (object, raw);
    });
    handlers.emplace ("project_health_check", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleProjectHealthCheck (object, raw);
    });

    handlers.emplace ("play", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handlePlay (object, raw);
    });

    handlers.emplace ("stop", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleStop (object, raw);
    });

    handlers.emplace ("return_to_zero", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleReturnToZero (object, raw);
    });

    handlers.emplace ("transport_option_stop_return_to_start", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleTransportOptionStopReturnToStart (object, raw);
    });

    handlers.emplace ("toggle_click", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleToggleClick (object, raw);
    });

    handlers.emplace ("set_click", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetClick (object, raw);
    });

    handlers.emplace ("seek", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSeek (object, raw);
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
        return handleSaveProject (object, raw);
    });

    handlers.emplace ("load_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleOpenProject (object, raw);
    });

    handlers.emplace ("save_as_project", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSaveAsProject (object, raw);
    });

    handlers.emplace ("get_audio_device_types", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetAudioDeviceTypes (object, raw);
    });

    handlers.emplace ("get_audio_devices", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetAudioDevices (object, raw);
    });

    handlers.emplace ("set_audio_device", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleSetAudioDevice (object, raw);
    });

    handlers.emplace ("scan_plugins", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleScanPlugins (object, raw);
    });

    handlers.emplace ("instantiate_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleInstantiatePlugin (object, raw);
    });

    handlers.emplace ("show_plugin_editor", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleOpenPluginUI (object, raw);
    });

    handlers.emplace ("open_plugin_ui", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleOpenPluginUI (object, raw);
    });

    handlers.emplace ("get_plugin_parameters", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleGetPluginParameters (object, raw);
    });

    handlers.emplace ("delete_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleDeletePlugin (object, raw);
    });

    handlers.emplace ("move_plugin", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleMovePlugin (object, raw);
    });

    handlers.emplace ("rack_add_node", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackAddNode (object, raw);
    });

    handlers.emplace ("rack_connect_pins", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackConnectPins (object, raw);
    });

    handlers.emplace ("rack_remove_connection", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackRemoveConnection (object, raw);
    });

    handlers.emplace ("rack_set_node_clip_scope", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackSetNodeClipScope (object, raw);
    });

    handlers.emplace ("rack_add_edge", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackConnectPins (object, raw);
    });

    handlers.emplace ("rack_remove_edge", [this] (const juce::DynamicObject& object, const juce::String& raw)
    {
        return handleRackRemoveConnection (object, raw);
    });
}

juce::String CommandDispatcher::handlePing (const juce::DynamicObject&, const juce::String&) const
{
    std::cout << "[ZMQ] Received ping, system is alive." << std::endl;
    juce::Logger::writeToLog ("CommandDispatcher: handled ping command.");
    return makeStatusReply ("ok", "pong");
}

juce::String CommandDispatcher::handleReloadProject (const juce::DynamicObject&, const juce::String&) const
{
    if (! reloadProject)
        return makeErrorReply ("Reload action is unavailable");

    if (reloadProject())
        return makeStatusReply ("ok", "Project reloaded");

    return makeErrorReply ("Failed to reload project");
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

juce::String CommandDispatcher::handleSaveProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! saveProjectReply)
        return makeErrorReply ("save_project is not available in this build");

    return saveProjectReply (object);
}

juce::String CommandDispatcher::handleGetRecentProjects (const juce::DynamicObject&, const juce::String&) const
{
    if (! recentProjectsReply)
        return makeErrorReply ("get_recent_projects is not available in this build");

    return recentProjectsReply();
}

juce::String CommandDispatcher::handleOpenProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! openProjectReply)
        return makeErrorReply ("open_project is not available in this build");

    const juce::String pathStr = object.getProperty ("file_path").toString().trim();

    if (pathStr.isEmpty())
        return makeErrorReply ("open_project requires a non-empty file_path");

    const juce::File target (pathStr);

    if (! target.existsAsFile())
        return makeErrorReply ("Project file does not exist: " + pathStr);

    return openProjectReply (object, target);
}

juce::String CommandDispatcher::handleNewProject (const juce::DynamicObject&, const juce::String&) const
{
    if (! newBlankProjectReply)
        return makeErrorReply ("new_project is not available in this build");

    return newBlankProjectReply();
}

juce::String CommandDispatcher::handleSaveAsProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! saveAsProjectReply)
        return makeErrorReply ("save_as_project is not available in this build");

    const juce::String pathStr = object.getProperty ("file_path").toString().trim();

    if (pathStr.isEmpty())
        return makeErrorReply ("save_as_project requires a non-empty file_path");

    const juce::File target (pathStr);

    if (target.isDirectory())
        return makeErrorReply ("save_as_project: file_path must be a file path, not a directory");

    return saveAsProjectReply (object, target);
}

juce::String CommandDispatcher::handleListTracks (const juce::DynamicObject&, const juce::String&) const
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

juce::String CommandDispatcher::handleAppendGhostTrack (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleAddTrack (const juce::DynamicObject&, const juce::String&) const
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

juce::String CommandDispatcher::handleDeleteTrack (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleAddAudioClip (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleMoveClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto sourceTrackId = object.getProperty ("source_track_id").toString().trim();
    const auto targetTrackId = object.getProperty ("target_track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (sourceTrackId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty source_track_id");

    if (targetTrackId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty target_track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("move_clip requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("move_clip: " + parseError);

    double startValue = 0.0;
    const juce::StringArray startKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_start_beat", "new_start_beats", "new_start" }
                                            : juce::StringArray { "new_start_seconds", "new_start" };

    if (! readNumericProperty (object, startKeys, startValue))
        return makeErrorReply ("move_clip requires numeric new_start (or unit-specific alias)");

    double newStartSeconds = 0.0;
    if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
        return makeErrorReply ("move_clip: " + parseError);

    auto* sourceTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, sourceTrackId));
    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, targetTrackId));

    if (sourceTrack == nullptr)
        return makeErrorReply ("move_clip source track not found or is not a clip track: " + sourceTrackId);
    if (targetTrack == nullptr)
        return makeErrorReply ("move_clip target track not found or is not a clip track: " + targetTrackId);

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("move_clip clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();
    if (clipTrack == nullptr)
        return makeErrorReply ("move_clip clip has no parent clip track");
    if (clipTrack->itemID.toString() != sourceTrackId)
        return makeErrorReply ("move_clip source_track_id does not match clip's current parent track");

    const auto overlapPolicy = parseOverlapPolicy (object);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Move clip");
    OverlapEditResult overlapResult;

    if (sourceTrack != targetTrack)
    {
        if (! clip->moveTo (*targetTrack))
            return makeErrorReply ("move_clip failed to move clip to target track");
    }

    clip->setStart (te::TimePosition::fromSeconds (newStartSeconds), false, true);

    if (isTrimPolicy (overlapPolicy))
    {
        const auto allowSplit = true;
        if (! applyCutOverlapPolicy (*targetTrack, *clip, allowSplit, &overlapResult))
            return makeErrorReply ("move_clip failed to apply trim overlap policy");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip moved");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("source_track_id", sourceTrackId);
    response->setProperty ("target_track_id", targetTrackId);
    response->setProperty ("affected_track_id", targetTrackId);
    response->setProperty ("new_start_seconds", clip->getPosition().getStart().inSeconds());
    response->setProperty ("overlap_mode", overlapPolicyToString (overlapPolicy));
    response->setProperty ("created_clip_ids", stringArrayToVar (overlapResult.createdClipIds));
    response->setProperty ("removed_clip_ids", stringArrayToVar (overlapResult.removedClipIds));
    response->setProperty ("affected_clip_ids", stringArrayToVar (overlapResult.touchedOriginalClipIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleCloneClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto timeUnitRaw = object.getProperty ("time_unit").toString().trim();

    if (timeUnitRaw.isEmpty())
        return makeErrorReply ("clone_clip requires time_unit (e.g. beats or seconds)");

    const auto sourceClipId = object.getProperty ("source_clip_id").toString().trim();

    if (sourceClipId.isEmpty())
        return makeErrorReply ("clone_clip requires a non-empty source_clip_id");

    const auto targetTrackId = object.getProperty ("target_track_id").toString().trim();

    if (targetTrackId.isEmpty())
        return makeErrorReply ("clone_clip requires a non-empty target_track_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("clone_clip: " + parseError);

    double startValue = 0.0;

    if (! readNumericProperty (object, { "new_start" }, startValue))
        return makeErrorReply ("clone_clip requires numeric new_start");

    double newStartSeconds = 0.0;

    if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
        return makeErrorReply ("clone_clip: " + parseError);

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, targetTrackId));

    if (targetTrack == nullptr)
        return makeErrorReply ("clone_clip target track not found or is not a clip track: " + targetTrackId);

    auto* sourceClip = findClipByID (*edit, sourceClipId);

    if (sourceClip == nullptr)
        return makeErrorReply ("clone_clip source clip not found for source_clip_id: " + sourceClipId);

    if (! sourceClip->canBeAddedTo (*targetTrack))
        return makeErrorReply ("clone_clip: clip type cannot be added to target track");

    sourceClip->flushStateToValueTree();

    juce::ValueTree clipState = sourceClip->state.createCopy();

    if (! clipState.isValid())
        return makeErrorReply ("clone_clip: failed to copy clip state");

    jassert (! clipState.getParent().isValid());

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Clone clip");

    regenerateValueTreeIDs (clipState, *edit, &undo);
    clipState.setProperty (te::IDs::start, newStartSeconds, &undo);

    auto* newClip = targetTrack->insertClipWithState (clipState);

    if (newClip == nullptr)
        return makeErrorReply ("clone_clip: engine refused to insert cloned clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleResizeClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("resize_clip requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);
    if (clip == nullptr)
        return makeErrorReply ("resize_clip clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();
    if (clipTrack == nullptr)
        return makeErrorReply ("resize_clip clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();
    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("resize_clip track_id does not match clip's current parent track");

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));
    if (targetTrack == nullptr)
        return makeErrorReply ("resize_clip track not found or is not a clip track: " + trackId);

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("resize_clip: " + parseError);

    const juce::StringArray startKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_start_beat", "new_start_beats", "new_start" }
                                            : juce::StringArray { "new_start_seconds", "new_start" };
    const juce::StringArray lengthKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "new_length_beats", "new_length_beat", "new_length" }
                                            : juce::StringArray { "new_length_seconds", "new_length" };
    const juce::StringArray offsetKeys = (timeUnit == CommandTimeUnit::beats)
                                            ? juce::StringArray { "offset_in_source_beats", "offset_in_source_beat", "offset_in_source" }
                                            : juce::StringArray { "offset_in_source_seconds", "offset_in_source" };

    double startValue = 0.0;
    double lengthValue = 0.0;
    double offsetValue = 0.0;

    const bool haveStart = readNumericProperty (object, startKeys, startValue);
    const bool haveLength = readNumericProperty (object, lengthKeys, lengthValue);
    const bool haveOffset = readNumericProperty (object, offsetKeys, offsetValue);

    if (! haveLength)
        return makeErrorReply ("resize_clip requires numeric new_length (or unit-specific alias)");

    double newStartSeconds = clip->getPosition().getStart().inSeconds();
    if (haveStart)
    {
        if (! convertTimeValueToSeconds (*edit, timeUnit, startValue, newStartSeconds, parseError))
            return makeErrorReply ("resize_clip: " + parseError);
    }

    double newLengthSeconds = 0.0;
    if (! convertDurationValueToSeconds (*edit, timeUnit, newStartSeconds, lengthValue, newLengthSeconds, parseError))
        return makeErrorReply ("resize_clip: " + parseError);
    newLengthSeconds = juce::jmax (newLengthSeconds, kMinimumSurvivingClipLengthSeconds);

    double newOffsetSeconds = clip->getPosition().getOffset().inSeconds();
    if (haveOffset)
    {
        if (! convertTimeValueToSeconds (*edit, timeUnit, offsetValue, newOffsetSeconds, parseError))
            return makeErrorReply ("resize_clip: " + parseError);
    }

    const auto overlapPolicy = parseOverlapPolicy (object);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Resize clip");
    OverlapEditResult overlapResult;

    // Left trim semantics: start, length and source offset are applied in one transaction.
    clip->setStart (te::TimePosition::fromSeconds (newStartSeconds), false, true);
    clip->setLength (te::TimeDuration::fromSeconds (newLengthSeconds), false);
    clip->setOffset (te::TimeDuration::fromSeconds (newOffsetSeconds));

    if (isTrimPolicy (overlapPolicy))
    {
        if (! applyCutOverlapPolicy (*targetTrack, *clip, true, &overlapResult))
            return makeErrorReply ("resize_clip failed to apply trim overlap policy");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clip resized");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("affected_track_id", trackId);
    response->setProperty ("new_start_seconds", clip->getPosition().getStart().inSeconds());
    response->setProperty ("new_length_seconds", clip->getPosition().getLength().inSeconds());
    response->setProperty ("offset_in_source_seconds", clip->getPosition().getOffset().inSeconds());
    response->setProperty ("overlap_mode", overlapPolicyToString (overlapPolicy));
    response->setProperty ("created_clip_ids", stringArrayToVar (overlapResult.createdClipIds));
    response->setProperty ("removed_clip_ids", stringArrayToVar (overlapResult.removedClipIds));
    response->setProperty ("affected_clip_ids", stringArrayToVar (overlapResult.touchedOriginalClipIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleAddMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("add_midi_notes: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("add_midi_notes requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("add_midi_notes requires a non-empty notes array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("add_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("add_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("add_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("add_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add MIDI notes");
    int added = 0;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes: each note must be an object");

        const auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            return makeErrorReply ("add_midi_notes: each note requires a non-empty id");

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start" }, rawStart))
            return makeErrorReply ("add_midi_notes: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length" }, rawLen))
            return makeErrorReply ("add_midi_notes: note missing numeric length: " + noteId);

        double startBeats = 0.0;
        double lenBeats = 0.0;

        if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
            return makeErrorReply ("add_midi_notes: " + parseError);

        int pitch = 60;

        if (! tryReadMidiInt (*noteObj, "pitch", pitch))
            return makeErrorReply ("add_midi_notes: note missing pitch: " + noteId);

        pitch = juce::jlimit (0, 127, pitch);

        int velocity = 100;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            velocity = juce::jlimit (1, 127, velocity);

        auto* created = seq.addNote (pitch,
                                     te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     velocity,
                                     0,
                                     &undo);

        if (created == nullptr)
            return makeErrorReply ("add_midi_notes: engine refused note: " + noteId);

        created->state.setProperty (kVitNoteId, noteId, &undo);
        ++added;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes added");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("added_count", added);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleAddMidiNotesBulk (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("add_midi_notes_bulk requires a non-empty clip_id");

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip not found for clip_id: " + clipId);

    auto* clipTrack = clip->getClipTrack();

    if (clipTrack == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip has no parent clip track");

    juce::String trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        trackId = clipTrack->itemID.toString();
    else if (clipTrack->itemID.toString() != trackId)
        return makeErrorReply ("add_midi_notes_bulk track_id does not match clip's current parent track");

    auto* targetTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (targetTrack == nullptr)
        return makeErrorReply ("add_midi_notes_bulk track not found or is not a clip track: " + trackId);

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("add_midi_notes_bulk clip is not a MIDI clip");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("add_midi_notes_bulk: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("add_midi_notes_bulk requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("add_midi_notes_bulk requires a non-empty notes array");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Add MIDI Block");
    int added = 0;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("add_midi_notes_bulk: each note must be an object");

        auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            noteId = juce::String ("blk_") + juce::Uuid().toString();

        if (findMidiNoteByVitId (seq, noteId) != nullptr)
            return makeErrorReply ("add_midi_notes_bulk: duplicate note id: " + noteId);

        double rawStart = 0.0;
        double rawLen = 0.0;

        if (! readNumericProperty (*noteObj, { "start" }, rawStart))
            return makeErrorReply ("add_midi_notes_bulk: note missing numeric start: " + noteId);

        if (! readNumericProperty (*noteObj, { "length" }, rawLen))
            return makeErrorReply ("add_midi_notes_bulk: note missing numeric length: " + noteId);

        double startBeats = 0.0;
        double lenBeats = 0.0;

        if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
            return makeErrorReply ("add_midi_notes_bulk: " + parseError);

        int pitch = 60;

        if (! tryReadMidiInt (*noteObj, "pitch", pitch))
            return makeErrorReply ("add_midi_notes_bulk: note missing pitch: " + noteId);

        pitch = juce::jlimit (0, 127, pitch);

        int velocity = 100;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            velocity = juce::jlimit (1, 127, velocity);

        auto* created = seq.addNote (pitch,
                                     te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     velocity,
                                     0,
                                     &undo);

        if (created == nullptr)
            return makeErrorReply ("add_midi_notes_bulk: engine refused note: " + noteId);

        created->state.setProperty (kVitNoteId, noteId, &undo);
        ++added;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI block notes added");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("added_count", added);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleMutateMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty clip_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::seconds;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("mutate_midi_notes: " + parseError);

    const auto notesVar = object.getProperty ("notes");

    if (! notesVar.isArray())
        return makeErrorReply ("mutate_midi_notes requires notes array");

    auto* arr = notesVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("mutate_midi_notes requires a non-empty notes array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("mutate_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("mutate_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("mutate_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("mutate_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Mutate MIDI notes");
    int mutated = 0;
    juce::StringArray missingIds;

    for (const auto& item : *arr)
    {
        auto* noteObj = item.getDynamicObject();

        if (noteObj == nullptr)
            return makeErrorReply ("mutate_midi_notes: each note must be an object");

        const auto noteId = noteObj->getProperty ("id").toString().trim();

        if (noteId.isEmpty())
            return makeErrorReply ("mutate_midi_notes: each note requires a non-empty id");

        auto* note = findMidiNoteByVitId (seq, noteId);

        if (note == nullptr)
        {
            missingIds.addIfNotAlreadyThere (noteId);
            continue;
        }

        double startBeats = note->getStartBeat().inBeats();
        double lenBeats = note->getLengthBeats().inBeats();
        double rawStart = 0.0;
        double rawLen = 0.0;
        const bool hasStart = readNumericProperty (*noteObj, { "start" }, rawStart);
        const bool hasLength = readNumericProperty (*noteObj, { "length" }, rawLen);

        if (hasStart || hasLength)
        {
            if (timeUnit == CommandTimeUnit::beats)
            {
                if (hasStart)
                    startBeats = rawStart;

                if (hasLength)
                    lenBeats = rawLen;

                if (lenBeats <= 0.0)
                    return makeErrorReply ("mutate_midi_notes: length must be greater than zero");
            }
            else
            {
                if (! hasStart || ! hasLength)
                    return makeErrorReply ("mutate_midi_notes: with time_unit seconds, both start and length are required when changing note geometry");

                if (! clipLocalNoteGeometryToBeats (*edit, *midiClip, timeUnit, rawStart, rawLen, startBeats, lenBeats, parseError))
                    return makeErrorReply ("mutate_midi_notes: " + parseError);
            }

            note->setStartAndLength (te::BeatPosition::fromBeats (startBeats),
                                     te::BeatDuration::fromBeats (lenBeats),
                                     &undo);
        }

        int pitch = 0;

        if (tryReadMidiInt (*noteObj, "pitch", pitch))
            note->setNoteNumber (juce::jlimit (0, 127, pitch), &undo);

        int velocity = 0;

        if (tryReadMidiInt (*noteObj, "velocity", velocity))
            note->setVelocity (juce::jlimit (1, 127, velocity), &undo);

        ++mutated;
    }

    if (missingIds.size() == static_cast<int> (arr->size()))
    {
        return makeErrorReply ("mutate_midi_notes: no matching notes (missing vit_note_id? ids: "
                               + missingIds.joinIntoString (", ") + ")");
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes mutated");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("mutated_count", mutated);
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleDeleteMidiNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty track_id");

    if (clipId.isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty clip_id");

    const auto idsVar = object.getProperty ("note_ids");

    if (! idsVar.isArray())
        return makeErrorReply ("delete_midi_notes requires note_ids array");

    auto* arr = idsVar.getArray();

    if (arr == nullptr || arr->isEmpty())
        return makeErrorReply ("delete_midi_notes requires a non-empty note_ids array");

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("delete_midi_notes track not found or is not a clip track: " + trackId);

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("delete_midi_notes clip not found for clip_id: " + clipId);

    if (clip->getClipTrack() == nullptr || clip->getClipTrack()->itemID.toString() != trackId)
        return makeErrorReply ("delete_midi_notes track_id does not match clip's current parent track");

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("delete_midi_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete MIDI notes");
    int removed = 0;
    juce::StringArray missingIds;

    for (const auto& item : *arr)
    {
        const auto noteId = item.toString().trim();

        if (noteId.isEmpty())
            continue;

        auto* note = findMidiNoteByVitId (seq, noteId);

        if (note == nullptr)
        {
            missingIds.addIfNotAlreadyThere (noteId);
            continue;
        }

        seq.removeNote (*note, &undo);
        ++removed;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI notes deleted");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("removed_count", removed);
    response->setProperty ("missing_note_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleGetMidiClipNotes (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("get_midi_clip_notes requires clip_id");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("get_midi_clip_notes clip not found for clip_id: " + clipId);

    if (trackId.isNotEmpty())
    {
        auto* clipTrack = clip->getClipTrack();

        if (clipTrack == nullptr || clipTrack->itemID.toString() != trackId)
            return makeErrorReply ("get_midi_clip_notes track_id does not match clip's current parent track");
    }

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    if (midiClip == nullptr)
        return makeErrorReply ("get_midi_clip_notes clip is not a MIDI clip");

    auto& seq = midiClip->getSequence();
    juce::Array<juce::var> notesArr;

    for (auto* n : seq.getNotes())
    {
        if (n == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", n->state.getProperty (kVitNoteId).toString());
        row->setProperty ("start", n->getStartBeat().inBeats());
        row->setProperty ("length", n->getLengthBeats().inBeats());
        row->setProperty ("pitch", n->getNoteNumber());
        row->setProperty ("velocity", n->getVelocity());
        notesArr.add (juce::var (row.release()));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI clip notes");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("track_id", trackId);
    response->setProperty ("time_unit", "beats");
    response->setProperty ("notes", juce::var (notesArr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleGetMidiClipData (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();

    if (clipId.isEmpty())
        return makeErrorReply ("get_midi_clip_data requires clip_id");

    auto* clip = findClipByID (*edit, clipId);

    if (clip == nullptr)
        return makeErrorReply ("get_midi_clip_data clip not found for clip_id: " + clipId);

    auto* midiClip = dynamic_cast<te::MidiClip*> (clip);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("time_unit", "beats");

    if (midiClip == nullptr)
    {
        response->setProperty ("notes", juce::var (juce::Array<juce::var>()));
        return juce::JSON::toString (juce::var (response.release()));
    }

    const auto pos = clip->getPosition();
    response->setProperty ("start_seconds", pos.getStart().inSeconds());
    response->setProperty ("length_seconds", pos.getLength().inSeconds());
    response->setProperty ("offset_in_source_seconds", pos.getOffset().inSeconds());

    auto& seq = midiClip->getSequence();
    juce::Array<juce::var> notesArr;

    for (auto* n : seq.getNotes())
    {
        if (n == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", n->state.getProperty (kVitNoteId).toString());
        row->setProperty ("start", n->getStartBeat().inBeats());
        row->setProperty ("length", n->getLengthBeats().inBeats());
        row->setProperty ("pitch", n->getNoteNumber());
        row->setProperty ("velocity", n->getVelocity());
        notesArr.add (juce::var (row.release()));
    }

    response->setProperty ("notes", juce::var (notesArr));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleInsertMidiClip (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackId = object.getProperty ("track_id").toString().trim();

    if (trackId.isEmpty())
        return makeErrorReply ("insert_midi_clip requires track_id");

    CommandTimeUnit timeUnit = CommandTimeUnit::beats;
    juce::String parseError;

    if (! parseTimeUnit (object, timeUnit, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    double startVal = 0.0;
    readNumericProperty (object, { "start", "start_beat", "start_beats" }, startVal);

    double lengthVal = 4.0;

    if (! readNumericProperty (object, { "length", "length_beat", "length_beats", "initial_length" }, lengthVal) || lengthVal <= 0.0)
        lengthVal = 4.0;

    double startSeconds = 0.0;

    if (! convertTimeValueToSeconds (*edit, timeUnit, startVal, startSeconds, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    double lengthSeconds = 0.0;

    if (! convertDurationValueToSeconds (*edit, timeUnit, startSeconds, lengthVal, lengthSeconds, parseError))
        return makeErrorReply ("insert_midi_clip: " + parseError);

    lengthSeconds = juce::jmax (lengthSeconds, kMinimumSurvivingClipLengthSeconds);

    auto* clipTrack = dynamic_cast<te::ClipTrack*> (findTrackByID (*edit, trackId));

    if (clipTrack == nullptr)
        return makeErrorReply ("insert_midi_clip track not found or cannot host clips: " + trackId);

    const auto tr = te::TimeRange (te::TimePosition::fromSeconds (startSeconds),
                                   te::TimePosition::fromSeconds (startSeconds + lengthSeconds));

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Insert MIDI clip");

    const auto newClip = clipTrack->insertMIDIClip (tr, nullptr);

    if (newClip == nullptr)
        return makeErrorReply ("insert_midi_clip: engine refused to create clip");

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "MIDI clip inserted");
    response->setProperty ("clip_id", newClip->itemID.toString());
    response->setProperty ("new_clip_id", newClip->itemID.toString());
    response->setProperty ("track_id", trackId);
    response->setProperty ("start_seconds", newClip->getPosition().getStart().inSeconds());
    response->setProperty ("length_seconds", newClip->getPosition().getLength().inSeconds());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleRemoveClips (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto idsVar = object.getProperty ("clip_ids");

    if (! idsVar.isArray())
        return makeErrorReply ("remove_clips requires clip_ids array");

    auto* arr = idsVar.getArray();

    if (arr == nullptr)
        return makeErrorReply ("remove_clips requires clip_ids array");

    std::vector<juce::String> uniqueOrdered;
    std::unordered_set<std::string> seen;

    for (const auto& item : *arr)
    {
        const auto id = item.toString().trim();

        if (id.isEmpty())
            continue;

        const auto key = id.toStdString();

        if (seen.insert (key).second)
            uniqueOrdered.push_back (id);
    }

    if (uniqueOrdered.empty())
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("message", "No clip ids to remove");
        response->setProperty ("removed_count", 0);
        response->setProperty ("missing_ids", stringArrayToVar (juce::StringArray()));
        return juce::JSON::toString (juce::var (response.release()));
    }

    juce::StringArray missingIds;
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Remove clips");
    int removedCount = 0;

    for (const auto& clipId : uniqueOrdered)
    {
        auto* clip = findClipByID (*edit, clipId);

        if (clip == nullptr)
        {
            missingIds.add (clipId);
            continue;
        }

        if (dynamic_cast<te::AudioClipBase*> (clip) != nullptr)
            TiledSpectrogramBaker::invalidateClipBake (clip->itemID.toString());

        clip->deselect();
        clip->removeFromParent();
        ++removedCount;
    }

    edit->invalidateStoredLength();
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (removedCount == 0)
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "error");
        response->setProperty ("message", "remove_clips: no matching clips for supplied clip_ids");
        response->setProperty ("removed_count", 0);
        response->setProperty ("missing_ids", stringArrayToVar (missingIds));
        return juce::JSON::toString (juce::var (response.release()));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Clips removed");
    response->setProperty ("removed_count", removedCount);
    response->setProperty ("missing_ids", stringArrayToVar (missingIds));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleImportMediaToTrack (const juce::DynamicObject& object, const juce::String&) const
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
                                                              appliedMode,
                                                              publishMessage,
                                                              saveProject);

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    juce::Logger::writeToLog ("CommandDispatcher::handleImportMediaToTrack: clipId=\""
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

juce::String CommandDispatcher::handleImportAudio (const juce::DynamicObject& object, const juce::String&) const
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
                                                              "append",
                                                              publishMessage,
                                                              saveProject);

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    juce::Logger::writeToLog ("CommandDispatcher::handleImportAudio: imported clip=\""
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

juce::String CommandDispatcher::handleWarmWaveformBake (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleAigcRegisterJob (const juce::DynamicObject& object, const juce::String&) const
{
    const auto jobId = object.getProperty ("job_id").toString().trim();

    if (jobId.isEmpty())
        return makeErrorReply ("aigc_register_job requires a non-empty job_id");

    VitAIGCJobRecord record;
    record.jobId = jobId;
    record.nodeId = object.getProperty ("node_id").toString().trim();
    record.jobState = object.getProperty ("job_state").toString().trim();
    record.ghostState = VitAsyncGhostPolicy::normalise (object.getProperty ("ghost_state").toString());
    record.statusMessage = object.getProperty ("status_message").toString().trim();
    record.requestHash = object.getProperty ("request_hash").toString().trim();
    record.source = object.getProperty ("source").toString().trim();

    if (record.jobState.isEmpty())
        record.jobState = "pending";

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    VitAIGCJobRuntime::upsertJob (projectFile, record);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "AIGC job registered");
    response->setProperty ("job_id", record.jobId);
    response->setProperty ("job_state", record.jobState);
    response->setProperty ("ghost_state", record.ghostState);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleBridgeIngestGeneratedAsset (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto bucketHint = object.getProperty ("bucket").toString().trim();
    const auto jobId = object.getProperty ("job_id").toString().trim();
    const auto startTimeVar = object.getProperty ("start_time");

    if (filePath.isEmpty())
        return makeErrorReply ("bridge_ingest_generated_asset requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("bridge_ingest_generated_asset requires a non-empty track_id");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("bridge_ingest_generated_asset requires numeric start_time");

    auto* targetTrack = findAudioTrackByID (*edit, trackId);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    juce::File sourceFile (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("Generated asset source file does not exist: " + filePath);

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
        return makeErrorReply ("Unsupported or unreadable generated audio file: " + filePath);

    const auto audioLengthSeconds = audioFile.getLength();

    if (audioLengthSeconds <= 0.0)
        return makeErrorReply ("Generated audio file has zero length: " + filePath);

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    VitGeneratedAssetRecord assetRecord;
    assetRecord.assetKind = object.getProperty ("asset_kind").toString().trim().isNotEmpty()
                                ? object.getProperty ("asset_kind").toString().trim()
                                : VitBridgeNode::defaultAssetKind();
    assetRecord.jobId = jobId;
    const auto nodeId = object.getProperty ("node_id").toString().trim();
    const auto requestedGhostState = VitAsyncGhostPolicy::normalise (object.getProperty ("ghost_state").toString());

    if (const auto ingestResult = VitMediaPoolManager::ingestGeneratedAsset (projectFile, sourceFile, bucketHint, assetRecord); ingestResult.failed())
        return makeErrorReply (ingestResult.getErrorMessage());

    ensureMonitoringPlugins (*targetTrack);
    const auto startTimeSeconds = juce::jmax (0.0, static_cast<double> (startTimeVar));
    const auto inserted = insertWaveClipWithUndoAndStartBake (*edit,
                                                              *targetTrack,
                                                              juce::File (assetRecord.absolutePath),
                                                              startTimeSeconds,
                                                              audioLengthSeconds,
                                                              false,
                                                              "bridge_ingest",
                                                              publishMessage,
                                                              saveProject);

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    auto* clip = findClipByID (*edit, inserted.clipId);
    if (clip == nullptr)
        return makeErrorReply ("Generated asset clip was inserted but could not be resolved by clip_id");

    assetRecord.trackId = inserted.trackItemId;
    assetRecord.clipId = inserted.clipId;

    const auto warpDescriptor = VitWarpBridgeNode::fromRequest (*edit, object);
    assetRecord.originBpm = warpDescriptor.originBpm;
    assetRecord.originKey = warpDescriptor.originKey;
    assetRecord.warpTargetBpm = warpDescriptor.targetBpm;
    assetRecord.warpMode = warpDescriptor.warpMode;
    assetRecord.warpState = warpDescriptor.warpState;

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Annotate generated asset");
    appendGeneratedAssetPropertiesToClip (clip->state, assetRecord, warpDescriptor, &undo);
    const auto takeStackId = VitTakeHistoryStack::resolveStackId (object, nodeId, inserted.clipId);
    const auto takeId = object.getProperty ("take_id").toString().trim().isNotEmpty()
                            ? object.getProperty ("take_id").toString().trim()
                            : "take:" + juce::Uuid().toString();
    appendTakePropertiesToClip (clip->state, takeStackId, takeId, requestedGhostState, &undo);

    if (auto* clipTrack = clip->getClipTrack())
        clipTrack->flushStateToValueTree();

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Generated asset imported but failed to save annotated project state");

    VitMediaPoolManager::registerClipReference (projectFile, assetRecord);

    VitTakeHistoryStack::addTake (projectFile,
                                  { takeStackId, takeId, inserted.clipId, inserted.trackItemId, nodeId },
                                  { takeId,
                                    assetRecord.assetRef,
                                    assetRecord.absolutePath,
                                    inserted.clipId,
                                    inserted.trackItemId,
                                    jobId,
                                    object.getProperty ("take_label").toString().trim(),
                                    warpDescriptor.warpState },
                                  true);

    if (jobId.isNotEmpty())
        VitAIGCJobRuntime::attachImportedAsset (projectFile, jobId, assetRecord.assetRef, assetRecord.trackId, assetRecord.clipId);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Generated asset ingested");
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("asset_ref", assetRecord.assetRef);
    response->setProperty ("asset_state", assetRecord.assetState);
    response->setProperty ("asset_bucket", assetRecord.bucket);
    response->setProperty ("asset_relative_path", assetRecord.relativePath);
    response->setProperty ("job_id", jobId);
    response->setProperty ("job_state", jobId.isNotEmpty() ? "ready" : "idle");
    response->setProperty ("take_stack_id", takeStackId);
    response->setProperty ("active_take_id", takeId);
    response->setProperty ("ghost_state", requestedGhostState);
    response->setProperty ("bridge_node_role", VitBridgeNode::defaultNodeRole());
    response->setProperty ("injector_node_role", VitAudioInjectorNode::defaultNodeRole());
    response->setProperty ("injector_zone_id", VitAudioInjectorNode::defaultZoneId());
    response->setProperty ("warp_mode", warpDescriptor.warpMode);
    response->setProperty ("warp_state", warpDescriptor.warpState);
    response->setProperty ("origin_bpm", warpDescriptor.originBpm);
    response->setProperty ("origin_key", warpDescriptor.originKey);
    response->setProperty ("warp_target_bpm", warpDescriptor.targetBpm);
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("clip_length_seconds", inserted.audioLengthSeconds);
    response->setProperty ("generated_assets", juce::var (VitMediaPoolManager::snapshotProjectAssets (projectFile)));
    response->setProperty ("jobs", juce::var (VitAIGCJobRuntime::snapshotProjectJobs (projectFile)));
    response->setProperty ("take_histories", juce::var (VitTakeHistoryStack::snapshotProjectStacks (projectFile)));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSwitchAssetTake (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();
    const auto takeId = object.getProperty ("take_id").toString().trim();

    if (clipId.isEmpty() || takeId.isEmpty())
        return makeErrorReply ("switch_asset_take requires non-empty clip_id and take_id");

    auto* clip = findClipByID (*edit, clipId);
    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);

    if (audioClip == nullptr)
        return makeErrorReply ("switch_asset_take clip not found or is not audio");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    const auto stackId = object.getProperty ("take_stack_id").toString().trim().isNotEmpty()
                             ? object.getProperty ("take_stack_id").toString().trim()
                             : clip->state.getProperty ("vit_take_stack_id").toString().trim();

    if (stackId.isEmpty())
        return makeErrorReply ("switch_asset_take could not resolve take stack id");

    VitTakeRecord take;
    if (! VitTakeHistoryStack::getTake (projectFile, stackId, takeId, take))
        return makeErrorReply ("switch_asset_take take not found in stack");

    juce::File takeFile (take.absolutePath);

    if (! takeFile.existsAsFile())
        return makeErrorReply ("switch_asset_take target take file is missing");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Switch asset take");
    audioClip->getSourceFileReference().setToDirectFileReference (takeFile, false);
    audioClip->sourceMediaChanged();
    clip->state.setProperty ("vit_asset_ref", take.assetRef, &undo);
    clip->state.setProperty ("vit_active_take_id", take.takeId, &undo);
    clip->state.setProperty ("vit_asset_state", "present", &undo);
    clip->state.setProperty ("vit_warp_state", take.warpState, &undo);

    if (auto* clipTrack = clip->getClipTrack())
        clipTrack->flushStateToValueTree();

    edit->dispatchPendingUpdatesSynchronously();
    const auto trackId = clip->getClipTrack() != nullptr ? clip->getClipTrack()->itemID.toString() : juce::String();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "take_switch",
                                                                              "Switched active take for clip " + clipId + " to " + takeId,
                                                                              trackId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (! VitTakeHistoryStack::setActiveTake (projectFile, stackId, takeId))
        return makeErrorReply ("switch_asset_take failed to activate take in stack");

    if (saveProject && ! saveProject())
        return makeErrorReply ("Active take switched but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Active take switched");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("take_stack_id", stackId);
    response->setProperty ("active_take_id", takeId);
    response->setProperty ("asset_ref", take.assetRef);
    response->setProperty ("take_histories", juce::var (VitTakeHistoryStack::snapshotProjectStacks (projectFile)));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSetAsyncGhostState (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();
    const auto jobId = object.getProperty ("job_id").toString().trim();
    const auto ghostState = VitAsyncGhostPolicy::normalise (object.getProperty ("ghost_state").toString());

    if (clipId.isEmpty() && jobId.isEmpty())
        return makeErrorReply ("set_async_ghost_state requires clip_id or job_id");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    juce::String trackId;

    if (clipId.isNotEmpty())
    {
        auto* clip = findClipByID (*edit, clipId);

        if (clip == nullptr)
            return makeErrorReply ("set_async_ghost_state clip not found");

        auto& undo = edit->getUndoManager();
        undo.beginNewTransaction ("Set async ghost state");
        clip->state.setProperty ("vit_async_ghost_state", ghostState, &undo);

        if (auto* clipTrack = clip->getClipTrack())
        {
            clipTrack->flushStateToValueTree();
            trackId = clipTrack->itemID.toString();
        }

        edit->dispatchPendingUpdatesSynchronously();
    }

    if (jobId.isNotEmpty())
        VitAIGCJobRuntime::setGhostState (projectFile, jobId, ghostState);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "ghost_policy_change",
                                                                              "Set async ghost state to " + ghostState,
                                                                              trackId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Ghost state updated but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Async ghost state updated");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("job_id", jobId);
    response->setProperty ("ghost_state", ghostState);
    response->setProperty ("jobs", juce::var (VitAIGCJobRuntime::snapshotProjectJobs (projectFile)));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSetVolume (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleSetMute (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleSetPluginParam (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto pluginIdStr = object.getProperty ("plugin_id").toString().trim();
    const auto paramIdRaw  = object.getProperty ("param_id").toString().trim();
    const auto valueVar    = object.getProperty ("value");
    const auto unit        = object.getProperty ("unit").toString().trim().toLowerCase();

    if (pluginIdStr.isEmpty())
        return makeErrorReply ("set_plugin_param requires a non-empty plugin_id");

    if (paramIdRaw.isEmpty())
        return makeErrorReply ("set_plugin_param requires a non-empty param_id");

    if (! valueVar.isDouble() && ! valueVar.isInt() && ! valueVar.isInt64())
        return makeErrorReply ("set_plugin_param requires a numeric value field");

    auto* plugin = findPluginInEdit (*edit, pluginIdStr);

    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginIdStr);

    te::AutomatableParameter::Ptr param = plugin->getAutomatableParameterByID (paramIdRaw);

    if (param == nullptr && paramIdRaw.equalsIgnoreCase ("volume"))
        param = plugin->getAutomatableParameterByID ("master volume");

    if (param == nullptr && paramIdRaw.equalsIgnoreCase ("pan"))
        param = plugin->getAutomatableParameterByID ("master pan");

    if (param == nullptr)
        return makeErrorReply ("Parameter not found for param_id: " + paramIdRaw);

    float valueToApply = static_cast<float> (static_cast<double> (valueVar));

    const bool volumeAsDb = unit == "db"
                            && (paramIdRaw.equalsIgnoreCase ("volume")
                                || paramIdRaw.equalsIgnoreCase ("master volume"));

    if (volumeAsDb)
    {
        const auto db = convertRequestedDbToPluginDb (static_cast<double> (valueToApply));
        valueToApply = te::decibelsToVolumeFaderPosition (db);
    }

    // Godot / IPC sends pan as 0..1 normalised for Tracktion Volume+Pan [-1,+1].
    const bool panAsNormalisedUi = (! volumeAsDb)
                                   && (paramIdRaw.equalsIgnoreCase ("pan")
                                       || paramIdRaw.equalsIgnoreCase ("master pan"));

    if (panAsNormalisedUi)
    {
        const float clampedNorm = juce::jlimit (0.0f, 1.0f, valueToApply);
        param->setNormalisedParameter (clampedNorm, juce::sendNotification);
    }
    else
    {
        const auto vr = param->getValueRange();
        valueToApply = juce::jlimit (vr.getStart(), vr.getEnd(), valueToApply);
        param->setParameter (valueToApply, juce::sendNotification);
    }

    if (auto* owner = plugin->getOwnerTrack())
        owner->flushStateToValueTree();
    else
        plugin->flushPluginStateToValueTree();

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("plugin_id", pluginIdStr);
    response->setProperty ("param_id", paramIdRaw);
    response->setProperty ("new_value", param->getCurrentValue());
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSetPluginParamAliases (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto pluginIdStr = object.getProperty ("plugin_id").toString().trim();
    if (pluginIdStr.isEmpty())
        return makeErrorReply ("set_plugin_param_aliases requires a non-empty plugin_id");

    auto updates = extractPluginParamAliasUpdates (object);
    if (updates.empty())
        return makeErrorReply ("set_plugin_param_aliases requires aliases object or param_id + alias");

    auto* plugin = findPluginInEdit (*edit, pluginIdStr);
    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginIdStr);

    auto aliases = readPluginParamAliases (*plugin);

    for (const auto& [key, value] : updates)
    {
        if (value.isEmpty())
            aliases.erase (key);
        else
            aliases[key] = value;
    }

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set plugin param aliases");

    const auto serialisedAliases = serialisePluginParamAliases (aliases);
    if (serialisedAliases.isEmpty())
        plugin->state.removeProperty (kVitParamAliasesProperty, &undo);
    else
        plugin->state.setProperty (kVitParamAliasesProperty, serialisedAliases, &undo);

    if (auto* owner = plugin->getOwnerTrack())
        owner->flushStateToValueTree();
    else
        plugin->flushPluginStateToValueTree();

    if (saveProject && ! saveProject())
        return makeErrorReply ("Plugin aliases updated but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("plugin_id", pluginIdStr);
    response->setProperty ("param_aliases", createAliasMapVar (aliases));
    response->setProperty ("alias_count", static_cast<int> (aliases.size()));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlAddNode (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto kind = VitParamSurface::normaliseControlKind (object.getProperty ("kind").toString());
    const auto name = object.getProperty ("name").toString().trim();
    const auto xVar = object.getProperty ("x");
    const auto yVar = object.getProperty ("y");

    if ((! xVar.isDouble() && ! xVar.isInt() && ! xVar.isInt64())
        || (! yVar.isDouble() && ! yVar.isInt() && ! yVar.isInt64()))
        return makeErrorReply ("control_add_node requires numeric x and y");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control add node");
    const auto nodeState = VitParamLinkGraph::addControlNode (*edit,
                                                             kind,
                                                             name,
                                                             static_cast<float> (xVar),
                                                             static_cast<float> (yVar),
                                                             &undo);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_node_add",
                                                                              "Added control node " + nodeState.getProperty ("node_id").toString(),
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control node added but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control node added");
    response->setProperty ("node", VitParamSurface::createNodeSnapshot (nodeState));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlAddMacro (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto name = object.getProperty ("name").toString().trim();
    const auto xVar = object.getProperty ("x");
    const auto yVar = object.getProperty ("y");
    const auto macroCountVar = object.getProperty ("macro_count");

    if ((! xVar.isDouble() && ! xVar.isInt() && ! xVar.isInt64())
        || (! yVar.isDouble() && ! yVar.isInt() && ! yVar.isInt64()))
        return makeErrorReply ("control_add_macro requires numeric x and y");

    const auto macroCount = macroCountVar.isInt() || macroCountVar.isInt64() || macroCountVar.isDouble()
                                ? static_cast<int> (macroCountVar)
                                : 8;

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control add macro");
    const auto nodeState = VitParamLinkGraph::addMacroNode (*edit,
                                                           name,
                                                           static_cast<float> (xVar),
                                                           static_cast<float> (yVar),
                                                           macroCount,
                                                           &undo);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "macro_node_add",
                                                                              "Added macro node " + nodeState.getProperty ("node_id").toString(),
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Macro node added but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Macro node added");
    response->setProperty ("node", VitMacroNode::createSnapshot (nodeState));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlAddBinding (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto sourceNodeId = object.getProperty ("source_node_id").toString().trim();
    const auto sourceOutput = object.getProperty ("source_output").toString().trim();
    const auto targetKind = object.getProperty ("target_kind").toString().trim().toLowerCase();
    const auto targetPluginId = object.getProperty ("target_plugin_id").toString().trim();
    const auto targetParamId = object.getProperty ("target_param_id").toString().trim();
    const auto targetNodeId = object.getProperty ("target_node_id").toString().trim();
    const auto targetInput = object.getProperty ("target_input").toString().trim();

    if (sourceNodeId.isEmpty())
        return makeErrorReply ("control_add_binding requires source_node_id");

    if (! VitParamLinkGraph::hasControlNode (*edit, sourceNodeId))
        return makeErrorReply ("control_add_binding source_node_id not found");

    if (targetKind.isEmpty())
        return makeErrorReply ("control_add_binding requires target_kind");

    if (targetKind == "plugin_param")
    {
        if (targetPluginId.isEmpty() || targetParamId.isEmpty())
            return makeErrorReply ("control_add_binding plugin_param target requires target_plugin_id + target_param_id");

        if (findPluginInEdit (*edit, targetPluginId) == nullptr)
            return makeErrorReply ("control_add_binding target_plugin_id not found");
    }
    else if (targetKind == "control_port")
    {
        if (targetNodeId.isEmpty())
            return makeErrorReply ("control_add_binding control_port target requires target_node_id");

        if (! VitParamLinkGraph::hasControlNode (*edit, targetNodeId))
            return makeErrorReply ("control_add_binding target_node_id not found");
    }
    else
    {
        return makeErrorReply ("control_add_binding target_kind must be plugin_param or control_port");
    }

    double minValue = 0.0;
    double maxValue = 1.0;
    readNumericProperty (object, { "min", "range_min" }, minValue);
    readNumericProperty (object, { "max", "range_max" }, maxValue);
    const auto curve = object.getProperty ("curve").toString().trim().toLowerCase();

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control add binding");
    const auto bindingState = VitParamLinkGraph::addBinding (*edit,
                                                            sourceNodeId,
                                                            sourceOutput,
                                                            targetKind,
                                                            targetPluginId,
                                                            targetParamId,
                                                            targetNodeId,
                                                            targetInput,
                                                            minValue,
                                                            maxValue,
                                                            curve,
                                                            &undo);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_binding_add",
                                                                              "Added control binding " + bindingState.getProperty ("binding_id").toString(),
                                                                              {},
                                                                              {},
                                                                              targetPluginId,
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control binding added but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control binding added");
    response->setProperty ("binding", createBindingSnapshot (*edit, bindingState));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlUpdateNode (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto nodeId = object.getProperty ("node_id").toString().trim();
    if (nodeId.isEmpty())
        return makeErrorReply ("control_update_node requires node_id");

    auto nodeState = VitParamLinkGraph::findControlNode (*edit, nodeId);
    if (! nodeState.isValid())
        return makeErrorReply ("control_update_node node_id not found");

    juce::NamedValueSet updates;

    const auto name = object.getProperty ("name").toString().trim();
    if (name.isNotEmpty())
        updates.set ("name", name);

    const auto xVar = object.getProperty ("x");
    if (xVar.isDouble() || xVar.isInt() || xVar.isInt64())
        updates.set ("x", static_cast<float> (xVar));

    const auto yVar = object.getProperty ("y");
    if (yVar.isDouble() || yVar.isInt() || yVar.isInt64())
        updates.set ("y", static_cast<float> (yVar));

    const auto minVar = object.getProperty ("min");
    if (minVar.isDouble() || minVar.isInt() || minVar.isInt64())
        updates.set ("min", static_cast<double> (minVar));

    const auto maxVar = object.getProperty ("max");
    if (maxVar.isDouble() || maxVar.isInt() || maxVar.isInt64())
        updates.set ("max", static_cast<double> (maxVar));

    const auto defaultValueVar = object.getProperty ("default_value");
    if (defaultValueVar.isDouble() || defaultValueVar.isInt() || defaultValueVar.isInt64())
        updates.set ("default_value", static_cast<double> (defaultValueVar));

    const auto enabledVar = object.getProperty ("enabled");
    if (enabledVar.isBool())
        updates.set ("enabled", static_cast<bool> (enabledVar));

    if (updates.size() == 0)
        return makeErrorReply ("control_update_node requires at least one mutable field");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control update node");
    VitParamLinkGraph::updateNodeLayoutAndRange (nodeState, updates, &undo);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_node_update",
                                                                              "Updated control node " + nodeId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control node updated but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control node updated");
    response->setProperty ("node", VitParamSurface::createNodeSnapshot (nodeState));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlSetNodeValue (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto nodeId = object.getProperty ("node_id").toString().trim();
    const auto outputName = object.getProperty ("output").toString().trim();
    const auto valueVar = object.getProperty ("value");

    if (nodeId.isEmpty())
        return makeErrorReply ("control_set_node_value requires node_id");

    if (! valueVar.isDouble() && ! valueVar.isInt() && ! valueVar.isInt64())
        return makeErrorReply ("control_set_node_value requires numeric value");

    auto nodeState = VitParamLinkGraph::findControlNode (*edit, nodeId);
    if (! nodeState.isValid())
        return makeErrorReply ("control_set_node_value node_id not found");

    const auto normalizedOutput = outputName.isNotEmpty() ? outputName : "value";
    const auto minValue = static_cast<double> (nodeState.getProperty ("min"));
    const auto maxValue = static_cast<double> (nodeState.getProperty ("max"));
    const auto incomingValue = static_cast<double> (valueVar);
    const auto clampedValue = juce::jlimit (minValue, maxValue, incomingValue);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control set node value");
    VitParamLinkGraph::setNodeOutputValue (nodeState, normalizedOutput, clampedValue, &undo);

    juce::Array<juce::var> appliedBindings;
    std::unordered_set<std::string> visitedOutputs;
    applyControlBindingsFromOutput (*edit,
                                    nodeId,
                                    normalizedOutput,
                                    &undo,
                                    appliedBindings,
                                    visitedOutputs);

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_value_set",
                                                                              "Set control output " + nodeId + ":" + normalizedOutput,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control node value set but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control node value set");
    response->setProperty ("node_id", nodeId);
    response->setProperty ("output", normalizedOutput);
    response->setProperty ("value", clampedValue);
    response->setProperty ("node", VitParamSurface::createNodeSnapshot (nodeState));
    response->setProperty ("applied_bindings", juce::var (appliedBindings));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlUpdateBinding (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto bindingId = object.getProperty ("binding_id").toString().trim();
    if (bindingId.isEmpty())
        return makeErrorReply ("control_update_binding requires binding_id");

    auto bindingState = VitParamLinkGraph::findBinding (*edit, bindingId);
    if (! bindingState.isValid())
        return makeErrorReply ("control_update_binding binding_id not found");

    juce::NamedValueSet updates;

    const auto enabledVar = object.getProperty ("enabled");
    if (enabledVar.isBool())
        updates.set ("enabled", static_cast<bool> (enabledVar));

    const auto curve = object.getProperty ("curve").toString().trim().toLowerCase();
    if (curve.isNotEmpty())
        updates.set ("curve", curve);

    const auto minVar = object.getProperty ("min");
    if (minVar.isDouble() || minVar.isInt() || minVar.isInt64())
        updates.set ("range_min", static_cast<double> (minVar));

    const auto maxVar = object.getProperty ("max");
    if (maxVar.isDouble() || maxVar.isInt() || maxVar.isInt64())
        updates.set ("range_max", static_cast<double> (maxVar));

    const auto targetInput = object.getProperty ("target_input").toString().trim();
    if (targetInput.isNotEmpty())
        updates.set ("target_input", targetInput);

    if (updates.size() == 0)
        return makeErrorReply ("control_update_binding requires mutable fields");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control update binding");
    VitParamLinkGraph::updateBinding (bindingState, updates, &undo);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_binding_update",
                                                                              "Updated control binding " + bindingId,
                                                                              {},
                                                                              {},
                                                                              bindingState.getProperty ("target_plugin_id").toString(),
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control binding updated but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control binding updated");
    response->setProperty ("binding", createBindingSnapshot (*edit, bindingState));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlRemoveBinding (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto bindingId = object.getProperty ("binding_id").toString().trim();
    if (bindingId.isEmpty())
        return makeErrorReply ("control_remove_binding requires binding_id");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control remove binding");
    if (! VitParamLinkGraph::removeBinding (*edit, bindingId, &undo))
        return makeErrorReply ("control_remove_binding binding_id not found");

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_binding_remove",
                                                                              "Removed control binding " + bindingId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control binding removed but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control binding removed");
    response->setProperty ("binding_id", bindingId);
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlRemoveNode (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto nodeId = object.getProperty ("node_id").toString().trim();
    if (nodeId.isEmpty())
        return makeErrorReply ("control_remove_node requires node_id");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control remove node");
    const auto removedCount = VitParamLinkGraph::removeControlNodeAndBindings (*edit, nodeId, &undo);
    if (removedCount == 0)
        return makeErrorReply ("control_remove_node node_id not found");

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "control_node_remove",
                                                                              "Removed control node " + nodeId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Control node removed but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Control node removed");
    response->setProperty ("node_id", nodeId);
    response->setProperty ("removed_item_count", removedCount);
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleControlSetMacroValues (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto nodeId = object.getProperty ("node_id").toString().trim();
    if (nodeId.isEmpty())
        return makeErrorReply ("control_set_macro_values requires node_id");

    auto nodeState = VitParamLinkGraph::findControlNode (*edit, nodeId);
    if (! nodeState.isValid())
        return makeErrorReply ("control_set_macro_values node_id not found");

    if (nodeState.getProperty ("kind").toString() != "macropanel")
        return makeErrorReply ("control_set_macro_values requires macropanel node");

    auto updates = object.getProperty ("values");
    auto* updateObject = updates.getDynamicObject();
    if (updateObject == nullptr)
        return makeErrorReply ("control_set_macro_values requires values object");

    const auto minValue = static_cast<double> (nodeState.getProperty ("min"));
    const auto maxValue = static_cast<double> (nodeState.getProperty ("max"));
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Control set macro values");

    juce::Array<juce::var> appliedBindings;
    std::unordered_set<std::string> visitedOutputs;
    const auto& properties = updateObject->getProperties();

    for (int i = 0; i < properties.size(); ++i)
    {
        const auto outputName = properties.getName (i).toString().trim();
        const auto valueVar = properties.getValueAt (i);
        if (outputName.isEmpty() || (! valueVar.isDouble() && ! valueVar.isInt() && ! valueVar.isInt64()))
            continue;

        const auto clampedValue = juce::jlimit (minValue, maxValue, static_cast<double> (valueVar));
        VitParamLinkGraph::setNodeOutputValue (nodeState, outputName, clampedValue, &undo);
        applyControlBindingsFromOutput (*edit, nodeId, outputName, &undo, appliedBindings, visitedOutputs);
    }

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "macro_values_set",
                                                                              "Set macro outputs for " + nodeId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Macro values updated but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Macro values updated");
    response->setProperty ("node", VitParamSurface::createNodeSnapshot (nodeState));
    response->setProperty ("applied_bindings", juce::var (appliedBindings));
    response->setProperty ("control_graph", createControlGraphState (*edit));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleConnectorUpsertProfile (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    juce::var profile;
    if (const auto result = VitConnectorPluginSpec::upsertProfile (projectFile, object, profile); result.failed())
        return makeErrorReply (result.getErrorMessage());

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Connector profile upserted");
    response->setProperty ("profile", profile);
    response->setProperty ("connector_profiles", juce::var (VitConnectorPluginSpec::snapshotProfiles (projectFile)));
    response->setProperty ("node_registry", VitNodeRegistry::createSnapshot (projectFile));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleConnectorRemoveProfile (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto profileId = object.getProperty ("profile_id").toString().trim();
    if (profileId.isEmpty())
        return makeErrorReply ("connector_remove_profile requires profile_id");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    bool removed = false;
    if (const auto result = VitConnectorPluginSpec::removeProfile (projectFile, profileId, removed); result.failed())
        return makeErrorReply (result.getErrorMessage());

    if (! removed)
        return makeErrorReply ("connector_remove_profile profile_id not found");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Connector profile removed");
    response->setProperty ("profile_id", profileId);
    response->setProperty ("connector_profiles", juce::var (VitConnectorPluginSpec::snapshotProfiles (projectFile)));
    response->setProperty ("node_registry", VitNodeRegistry::createSnapshot (projectFile));
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

juce::String CommandDispatcher::handlePlay (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("CommandDispatcher::handlePlay: before " + describeTransportState (*edit));
    transport.ensureContextAllocated();
    transport.play (false);
    juce::Logger::writeToLog ("CommandDispatcher::handlePlay: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport playing");
}

juce::String CommandDispatcher::handleStop (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("CommandDispatcher::handleStop: before " + describeTransportState (*edit));
    // Tracktion stop(discardRecordings, clearDevices): second arg clears device graph, not timeline rewind.
    transport.stop (false, false);
    if (stopReturnsToZero)
        transport.setPosition (te::TimePosition::fromSeconds (0.0));

    transport.ensureContextAllocated();
    juce::Logger::writeToLog ("CommandDispatcher::handleStop: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport stopped");
}

juce::String CommandDispatcher::handleReturnToZero (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto& transport = edit->getTransport();
    juce::Logger::writeToLog ("CommandDispatcher::handleReturnToZero: before " + describeTransportState (*edit));
    transport.stop (false, false);
    transport.setPosition (te::TimePosition::fromSeconds (0.0));
    transport.ensureContextAllocated();
    juce::Logger::writeToLog ("CommandDispatcher::handleReturnToZero: after " + describeTransportState (*edit));
    return buildTransportReply (*edit, "Transport returned to zero");
}

juce::String CommandDispatcher::handleTransportOptionStopReturnToStart (const juce::DynamicObject& object, const juce::String&) const
{
    const auto valueVar = object.getProperty ("value");

    if (! valueVar.isBool())
        return makeErrorReply ("transport_option_stop_return_to_start requires a boolean value field");

    stopReturnsToZero = static_cast<bool> (valueVar);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleToggleClick (const juce::DynamicObject&, const juce::String&) const
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

juce::String CommandDispatcher::handleSetClick (const juce::DynamicObject& object, const juce::String&) const
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

juce::String CommandDispatcher::handleSeek (const juce::DynamicObject& object, const juce::String&) const
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
    juce::Logger::writeToLog ("CommandDispatcher::handleSeek: position set to " + juce::String (targetTime, 4) + "s");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Seek executed");
    response->setProperty ("position_seconds", targetTime);
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

juce::String CommandDispatcher::handleGetAudioDeviceTypes (const juce::DynamicObject&, const juce::String&) const
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

juce::String CommandDispatcher::handleGetAudioDevices (const juce::DynamicObject& object, const juce::String&) const
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

    juce::Array<juce::var> devicesJson;

    for (int i = 0; i < availableDevices.size(); ++i)
        devicesJson.add (availableDevices[i]);

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
    response->setProperty ("current_sample_rate", setup.sampleRate);
    response->setProperty ("current_buffer_size", setup.bufferSize);
    response->setProperty ("available_devices", juce::var (devicesJson));
    response->setProperty ("available_sample_rates", juce::var (sampleRatesJson));
    response->setProperty ("available_buffer_sizes", juce::var (bufferSizesJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleSetAudioDevice (const juce::DynamicObject& object, const juce::String&) const
{
    // Audio device setup is global JUCE device manager state, not Edit ValueTree / UndoManager.
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto typeStr = object.getProperty ("type").toString().trim();
    const auto deviceName = object.getProperty ("device_name").toString().trim();
    const auto srVar = object.getProperty ("sample_rate");
    const auto bsVar = object.getProperty ("buffer_size");

    if (typeStr.isEmpty())
        return makeErrorReply ("set_audio_device requires a non-empty type field");

    if (deviceName.isEmpty())
        return makeErrorReply ("set_audio_device requires a non-empty device_name field");

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

    if (! availableDevices.contains (deviceName))
        return makeErrorReply ("Device name not found in the specified type");

    if (! outputDevices.contains (deviceName))
        return makeErrorReply ("Device name is not a valid output device for the specified type");

    auto setup = jdm.getAudioDeviceSetup();
    setup.outputDeviceName = deviceName;
    if (inputDevices.contains (deviceName))
        setup.inputDeviceName = deviceName;
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

juce::String CommandDispatcher::handleScanPlugins (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto pathsVar = object.getProperty ("paths");

    if (! pathsVar.isArray())
        return makeErrorReply ("scan_plugins requires paths array");

    auto* arr = pathsVar.getArray();

    if (arr == nullptr)
        return makeErrorReply ("scan_plugins requires paths array");

    juce::FileSearchPath searchPath;

    for (const auto& item : *arr)
    {
        const auto path = item.toString().trim();

        if (path.isNotEmpty())
            searchPath.add (juce::File (path));
    }

    searchPath.removeRedundantPaths();

    auto& pluginManager = edit->engine.getPluginManager();
    auto& formatManager = pluginManager.pluginFormatManager;
    auto& knownPluginList = pluginManager.knownPluginList;

    juce::Array<juce::var> pluginsJson;

    if (searchPath.getNumPaths() == 0)
    {
        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("plugins", juce::var (pluginsJson));
        return juce::JSON::toString (juce::var (response.release()));
    }

    juce::AudioPluginFormat* vst3Format = nullptr;

    for (int i = 0; i < formatManager.getNumFormats(); ++i)
    {
        auto* f = formatManager.getFormat (i);

        if (f != nullptr && f->getName() == "VST3")
        {
            vst3Format = f;
            break;
        }
    }

    if (vst3Format == nullptr)
        return makeErrorReply ("VST3 plugin format is not available in this build");

    {
        juce::PluginDirectoryScanner scanner (knownPluginList,
                                              *vst3Format,
                                              searchPath,
                                              true,
                                              juce::File(),
                                              false);
        juce::String pluginName;

        while (scanner.scanNextFile (true, pluginName))
        {
        }
    }

    for (const auto& desc : knownPluginList.getTypes())
    {
        if (desc.pluginFormatName != "VST3")
            continue;

        auto entry = std::make_unique<juce::DynamicObject>();
        entry->setProperty ("name", desc.name);
        entry->setProperty ("format", desc.pluginFormatName);
        entry->setProperty ("identifier", desc.createIdentifierString());
        pluginsJson.add (juce::var (entry.release()));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("plugins", juce::var (pluginsJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleInstantiatePlugin (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto trackID = object.getProperty ("track_id").toString().trim();
    if (trackID.isEmpty())
    {
        const auto trackIndexVar = object.getProperty ("track_index");
        if (trackIndexVar.isInt() || trackIndexVar.isInt64())
        {
            const auto trackIndex = static_cast<int> (trackIndexVar);
            if (auto* legacyTrack = findAudioTrackByIndex (*edit, trackIndex))
                trackID = legacyTrack->itemID.toString();
        }
    }
    if (trackID.isEmpty())
        return makeErrorReply ("instantiate_plugin requires non-empty track_id (or legacy track_index)");

    const auto pluginPath = object.getProperty ("plugin_path").toString().trim();

    if (pluginPath.isEmpty())
        return makeErrorReply ("instantiate_plugin requires plugin_path");

    juce::File pluginFile (pluginPath);

    if (! pluginFile.exists())
        return makeErrorReply ("plugin_path does not exist: " + pluginPath);

    auto* targetTrack = findTrackByID (*edit, trackID);
    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (targetTrack))
        ensureMonitoringPlugins (*audioTrack);

    auto& pluginManager = edit->engine.getPluginManager();
    auto& knownPluginList = pluginManager.knownPluginList;

    juce::PluginDescription chosenDesc;
    bool haveDesc = false;
    const auto targetCanon = pluginFile.getFullPathName();

    for (const auto& desc : knownPluginList.getTypes())
    {
        if (desc.fileOrIdentifier.equalsIgnoreCase (targetCanon))
        {
            chosenDesc = desc;
            haveDesc = true;
            break;
        }

        juce::File descFile (desc.fileOrIdentifier);

        if (descFile.getFullPathName().equalsIgnoreCase (targetCanon))
        {
            chosenDesc = desc;
            haveDesc = true;
            break;
        }
    }

    if (! haveDesc)
    {
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
            return makeErrorReply ("VST3 format not available");

        juce::OwnedArray<juce::PluginDescription> discovered;
        vst3Format->findAllTypesForFile (discovered, pluginFile.getFullPathName());

        if (discovered.isEmpty())
            return makeErrorReply ("Plugin not in cache and VST3 introspection failed (check path). CLAP is not implemented in instantiate_plugin yet.");

        if (auto* first = discovered.getFirst())
            chosenDesc = *first;

        haveDesc = true;
    }

    auto plugin = edit->getPluginCache().createNewPlugin (te::ExternalPlugin::xmlTypeName, chosenDesc);

    if (plugin == nullptr)
        return makeErrorReply ("Failed to create plugin instance");

    const auto templateRole = VitPluginTemplateRegistry::inferTemplateRole (*plugin);
    plugin->state.setProperty ("vit_template_role", templateRole, nullptr);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Instantiate plugin");
    targetTrack->pluginList.insertPlugin (plugin, 0, nullptr);
    targetTrack->flushStateToValueTree();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "node_add",
                                                                              "Inserted track plugin " + plugin->getName(),
                                                                              trackID,
                                                                              {},
                                                                              pluginItemIdString (*plugin),
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Plugin inserted but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin instantiated");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_id", pluginItemIdString (*plugin));
    response->setProperty ("plugin_item_id", pluginItemIdString (*plugin));
    response->setProperty ("template_role", templateRole);
    response->setProperty ("plugin_name", plugin->getName());
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleOpenPluginUI (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto trackID = object.getProperty ("track_id").toString().trim();
    auto pluginID = object.getProperty ("plugin_id").toString().trim();
    const auto pluginPath = object.getProperty ("plugin_path").toString().trim();
    if (trackID.isEmpty())
    {
        const auto trackIndexVar = object.getProperty ("track_index");
        if (trackIndexVar.isInt() || trackIndexVar.isInt64())
        {
            const auto trackIndex = static_cast<int> (trackIndexVar);
            if (auto* legacyTrack = findAudioTrackByIndex (*edit, trackIndex))
                trackID = legacyTrack->itemID.toString();
        }
    }
    if (trackID.isEmpty() || (pluginID.isEmpty() && pluginPath.isEmpty()))
        return makeErrorReply ("open_plugin_ui requires track_id + plugin_id (legacy: track_index/track_id + plugin_path)");

    auto* targetTrack = findTrackByID (*edit, trackID);
    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    te::Plugin* plugin = nullptr;
    if (pluginID.isNotEmpty())
    {
        plugin = findPluginInEdit (*edit, pluginID);

        if (plugin != nullptr && ! pluginBelongsToTrackGraph (*targetTrack, *plugin))
            return makeErrorReply ("Plugin is not on the specified track");
    }
    else
        for (auto* cand : targetTrack->pluginList.getPlugins())
            if (auto* extCand = dynamic_cast<te::ExternalPlugin*> (cand))
                if (extCand->desc.fileOrIdentifier.equalsIgnoreCase (pluginPath)
                    || juce::File (extCand->desc.fileOrIdentifier).getFullPathName().equalsIgnoreCase (juce::File (pluginPath).getFullPathName()))
                {
                    plugin = extCand;
                    pluginID = pluginItemIdString (*extCand);
                    break;
                }
    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginID);

    auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
    if (ext == nullptr)
        return makeErrorReply ("Plugin is not an external VST/AU plugin");
    if (ext->getAudioPluginInstance() == nullptr)
        return makeErrorReply ("Plugin instance is not ready yet");
    if (! ext->getAudioPluginInstance()->hasEditor())
        return makeErrorReply ("Plugin has no editor");

    ext->showWindowExplicitly();

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin UI opened");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_id", pluginID);
    response->setProperty ("plugin_item_id", pluginID);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleGetPluginParameters (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginID = object.getProperty ("plugin_id").toString().trim();
    if (trackID.isEmpty() || pluginID.isEmpty())
        return makeErrorReply ("get_plugin_parameters requires non-empty track_id and plugin_id");

    auto* targetTrack = findTrackByID (*edit, trackID);
    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* plugin = findPluginInEdit (*edit, pluginID);
    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginID);

    if (! pluginBelongsToTrackGraph (*targetTrack, *plugin))
        return makeErrorReply ("Plugin is not on the specified track");

    auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
    if (ext == nullptr || ext->getAudioPluginInstance() == nullptr)
        return makeErrorReply ("Plugin parameters are only available for instantiated external plugins");

    const auto storedTemplateRole = plugin->state.getProperty ("vit_template_role").toString().trim().toLowerCase();
    const auto templateRole = storedTemplateRole.isNotEmpty() ? storedTemplateRole
                                                              : VitPluginTemplateRegistry::inferTemplateRole (*plugin);
    auto parametersArray = VitPluginGrabber::buildParameterDescriptors (*ext, templateRole);
    const auto aliasMap = readPluginParamAliases (*plugin);
    applyAliasesToParameterDescriptors (parametersArray, aliasMap);
    appendBindingTargetPropertiesToParameterDescriptors (parametersArray, *plugin);
    const auto recommendedGroups = VitPluginGrabber::buildRecommendedGroups (parametersArray);
    const auto quickControls = VitPluginGrabber::buildQuickControls (parametersArray, templateRole);
    juce::Array<juce::var> bindingTargets;
    for (const auto& parameter : parametersArray)
        if (auto* parameterObject = parameter.getDynamicObject())
            bindingTargets.add (parameterObject->getProperty ("binding_target"));

    juce::Array<juce::var> capabilityTags;
    capabilityTags.add ("open_ui");
    capabilityTags.add ("get_param");
    capabilityTags.add ("template_shell");
    capabilityTags.add ("parameter_aliases");
    capabilityTags.add ("quick_controls");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_id", pluginID);
    response->setProperty ("plugin_item_id", pluginID);
    response->setProperty ("template_role", templateRole);
    response->setProperty ("supports_param_grabber", true);
    response->setProperty ("param_aliases", createAliasMapVar (aliasMap));
    response->setProperty ("binding_targets", juce::var (bindingTargets));
    response->setProperty ("recommended_groups", juce::var (recommendedGroups));
    response->setProperty ("quick_controls", juce::var (quickControls));
    response->setProperty ("control_shell", VitControlShell::buildShellDescriptor (templateRole, recommendedGroups));
    response->setProperty ("capability_manifest", juce::var (capabilityTags));
    response->setProperty ("parameters", juce::var (parametersArray));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleDeletePlugin (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginItemIdStr = object.getProperty ("plugin_item_id").toString().trim();

    if (trackID.isEmpty())
        return makeErrorReply ("delete_plugin requires track_id");

    if (pluginItemIdStr.isEmpty())
        return makeErrorReply ("delete_plugin requires plugin_item_id");

    auto* track = findTrackByID (*edit, trackID);

    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* plugin = findPluginInEdit (*edit, pluginItemIdStr);

    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_item_id: " + pluginItemIdStr);

    if (! pluginBelongsToTrackGraph (*track, *plugin))
        return makeErrorReply ("Plugin is not on the specified track");

    if (dynamic_cast<te::VolumeAndPanPlugin*> (plugin) != nullptr)
        return makeErrorReply ("Cannot delete built-in volume/pan plugin");

    if (dynamic_cast<te::LevelMeterPlugin*> (plugin) != nullptr)
        return makeErrorReply ("Cannot delete built-in level meter plugin");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete plugin");
    plugin->removeFromParent();
    track->flushStateToValueTree();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "node_remove",
                                                                              "Removed track plugin " + pluginItemIdStr,
                                                                              trackID,
                                                                              {},
                                                                              pluginItemIdStr,
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Plugin removed but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin deleted");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_item_id", pluginItemIdStr);
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleMovePlugin (const juce::DynamicObject& object, const juce::String&) const
{
    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginItemIdStr = object.getProperty ("plugin_item_id").toString().trim();
    const auto newIndexVar = object.getProperty ("new_index");

    if (trackID.isEmpty())
        return makeErrorReply ("move_plugin requires track_id");

    if (pluginItemIdStr.isEmpty())
        return makeErrorReply ("move_plugin requires plugin_item_id");

    if (! newIndexVar.isInt() && ! newIndexVar.isInt64())
        return makeErrorReply ("move_plugin requires integer new_index");

    const int newIndex = static_cast<int> (newIndexVar);
    juce::Logger::writeToLog ("CommandDispatcher: rejected deprecated move_plugin track="
                              + trackID + " plugin=" + pluginItemIdStr + " new_index=" + juce::String (newIndex));
    return makeErrorReply ("move_plugin deprecated in DAG rack mode");
}

juce::String CommandDispatcher::handleRackAddNode (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    if (ensureTrackRackGraphForEdit (*edit))
    {
        edit->dispatchPendingUpdatesSynchronously();
        edit->getTransport().ensureContextAllocated (true);
    }

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto rackItemId = object.getProperty ("rack_item_id").toString().trim();
    const auto pluginPath = object.getProperty ("plugin_path").toString().trim();
    const auto xVar = object.getProperty ("x");
    const auto yVar = object.getProperty ("y");
    const auto autoConnectVar = object.getProperty ("auto_connect");

    if (trackID.isEmpty())
        return makeErrorReply ("rack_add_node requires track_id");

    if (pluginPath.isEmpty())
        return makeErrorReply ("rack_add_node requires plugin_path");

    if ((! xVar.isDouble() && ! xVar.isInt() && ! xVar.isInt64())
        || (! yVar.isDouble() && ! yVar.isInt() && ! yVar.isInt64()))
        return makeErrorReply ("rack_add_node requires numeric x and y");

    if (const auto zoneValidation = validateRequestedZoneId (object); zoneValidation.failed())
        return makeErrorReply (zoneValidation.getErrorMessage());

    auto* track = findTrackByID (*edit, trackID);
    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    if (const auto clipScopeValidation = validateRequestedClipScope (*track, object); clipScopeValidation.failed())
        return makeErrorReply (clipScopeValidation.getErrorMessage());

    auto* rack = findRackInstanceOnTrack (*track, rackItemId);
    if (rack == nullptr || rack->type == nullptr)
        rack = ensureRackInstanceOnTrack (*track);

    if (rack == nullptr || rack->type == nullptr)
        return makeErrorReply ("No rack instance found on the specified track");

    juce::PluginDescription chosenDesc;
    if (const auto result = resolveExternalPluginDescription (*edit, pluginPath, chosenDesc); result.failed())
        return makeErrorReply (result.getErrorMessage());

    auto plugin = edit->getPluginCache().createNewPlugin (te::ExternalPlugin::xmlTypeName, chosenDesc);
    if (plugin == nullptr)
        return makeErrorReply ("Failed to create plugin instance");

    const auto x = static_cast<float> (xVar);
    const auto y = static_cast<float> (yVar);
    const bool autoConnect = autoConnectVar.isBool() ? static_cast<bool> (autoConnectVar) : true;
    const auto zoneId = parseZoneIdOrDefault (object, plugin.get());
    const auto clipScope = parseClipScopeOrDefault (object);
    const auto templateRole = VitPluginTemplateRegistry::inferTemplateRole (*plugin);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rack add node");

    if (! rack->type->addPlugin (plugin, { x, y }, autoConnect))
        return makeErrorReply ("Rack rejected plugin insertion");

	logRackConnections ("rack_add_node after addPlugin auto_connect="
	                    + juce::String (autoConnect ? "true" : "false")
	                    + " plugin=" + plugin->getName(),
	                    *rack->type);

    if (auto pluginInstanceState = findRackPluginInstanceState (*rack->type, plugin->itemID); pluginInstanceState.isValid())
    {
        pluginInstanceState.setProperty ("vit_zone_id", zoneId, &undo);
        pluginInstanceState.setProperty ("vit_clip_scope", clipScope, &undo);
        pluginInstanceState.setProperty ("vit_template_role", templateRole, &undo);
    }

    if (autoConnect && rack->type->getPlugins().size() > 1)
    {
        if (! vitTryChainNewRackPluginSerial (*rack->type, *plugin))
            juce::Logger::writeToLog ("CommandDispatcher: rack_add_node serial auto-chain skipped for "
                                      + plugin->getName()
                                      + " (parallel rack tail, illegal pins, or ambiguous graph)");
		logRackConnections ("rack_add_node after serial-chain plugin=" + plugin->getName(), *rack->type);
    }

    rack->type->flushStateToValueTree();
    track->flushStateToValueTree();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "node_add",
                                                                              "Added rack node " + plugin->getName(),
                                                                              trackID,
                                                                              rack->itemID.toString(),
                                                                              pluginItemIdString (*plugin),
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Rack node inserted but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Rack node added");
    response->setProperty ("track_id", trackID);
    response->setProperty ("rack_item_id", rack->itemID.toString());
    response->setProperty ("plugin_id", pluginItemIdString (*plugin));
    response->setProperty ("plugin_item_id", pluginItemIdString (*plugin));
    response->setProperty ("zone_id", zoneId);
    response->setProperty ("clip_scope", clipScope);
    response->setProperty ("template_role", templateRole);
    response->setProperty ("x", x);
    response->setProperty ("y", y);
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleRackConnectPins (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto rackItemId = object.getProperty ("rack_item_id").toString().trim();
    const auto sourceIdStr = object.getProperty ("source_id").toString().trim();
    const auto destIdStr = object.getProperty ("dest_id").toString().trim();
    const auto sourcePinVar = object.getProperty ("source_pin");
    const auto destPinVar = object.getProperty ("dest_pin");

    if (trackID.isEmpty())
        return makeErrorReply ("rack_connect_pins requires track_id");

    const bool sourceIsRackBusInput = isRackBusInputSourceToken (sourceIdStr);
    const bool destIsRackBusOutput = isRackBusOutputDestToken (destIdStr);

    if (! destIsRackBusOutput && destIdStr.isEmpty())
        return makeErrorReply ("rack_connect_pins requires non-empty dest_id");

    if (sourceIsRackBusInput && destIsRackBusOutput)
        return makeErrorReply ("Cannot connect rack input directly to rack output");

    if ((! sourcePinVar.isInt() && ! sourcePinVar.isInt64())
        || (! destPinVar.isInt() && ! destPinVar.isInt64()))
        return makeErrorReply ("rack_connect_pins requires integer source_pin and dest_pin");

    auto* track = findTrackByID (*edit, trackID);
    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* rack = findRackInstanceOnTrack (*track, rackItemId);
    if (rack == nullptr || rack->type == nullptr)
        return makeErrorReply ("No rack instance found on the specified track");

    te::EditItemID destId;
    te::Plugin* destPlugin = nullptr;

    if (destIsRackBusOutput)
    {
        destId = {};
    }
    else
    {
        destId = te::EditItemID::fromString (destIdStr);

        if (! destId.isValid())
            return makeErrorReply ("dest_id must be a valid EditItemID string");

        destPlugin = rack->type->getPluginForID (destId);

        if (destPlugin == nullptr)
            return makeErrorReply ("dest_id is not present in the specified rack");
    }

    te::EditItemID sourceId;

    if (sourceIsRackBusInput)
        sourceId = {};
    else
        sourceId = te::EditItemID::fromString (sourceIdStr);

    if (! sourceIsRackBusInput && ! sourceId.isValid())
        return makeErrorReply ("source_id must be a valid EditItemID string, empty, or RACK_INPUT");

    auto* sourcePlugin = sourceIsRackBusInput ? nullptr : rack->type->getPluginForID (sourceId);

    if (! sourceIsRackBusInput && sourcePlugin == nullptr)
        return makeErrorReply ("source_id is not present in the specified rack");

    const int sourcePin = static_cast<int> (sourcePinVar);
    const int destPin = static_cast<int> (destPinVar);
    const bool audioPinGroup = static_cast<bool> (object.getProperty ("audio_pin_group"))
                               && sourcePin >= 1
                               && destPin >= 1;

    VitGraphValidationResult structuralValidation;

    if (sourceIsRackBusInput && ! destIsRackBusOutput)
    {
        structuralValidation.allowed = true;
        structuralValidation.sourceZoneId = "RACK_INPUT";
        structuralValidation.destZoneId = VitGraphValidator::getZoneIdForNode (*rack->type, destId, destPlugin);

        if (structuralValidation.destZoneId == "TOP")
            return makeErrorReply ("Top is mapping-only and cannot participate in rack execution edges");
    }
    else if (! sourceIsRackBusInput && destIsRackBusOutput)
    {
        structuralValidation.allowed = true;
        structuralValidation.sourceZoneId = VitGraphValidator::getZoneIdForNode (*rack->type, sourceId, sourcePlugin);
        structuralValidation.destZoneId = "RACK_OUTPUT";

        if (structuralValidation.sourceZoneId == "TOP")
            return makeErrorReply ("Top is mapping-only and cannot participate in rack execution edges");
    }
    else if (! sourceIsRackBusInput && ! destIsRackBusOutput)
    {
        structuralValidation = VitGraphValidator::validateConnection (*rack->type, sourceId, destId);
    }
    else
    {
        return makeErrorReply ("Invalid rack connection endpoints");
    }

    if (! structuralValidation.allowed)
        return makeErrorReply (structuralValidation.errorMessage);

    if (! sourceIsRackBusInput && ! destIsRackBusOutput)
    {
        if (VitDagChecker::wouldCreateCycle (*rack->type, sourceId, destId))
            return makeErrorReply ("Connection would create a cycle in the rack DAG");
    }

    if (! audioPinGroup && ! rack->type->isConnectionLegal (sourceId, sourcePin, destId, destPin))
        return makeErrorReply ("Rack connection rejected as illegal (likely loop or incompatible pins)");

    const auto adapterAdvice = VitZoneBufferAdapter::describeConnection (sourcePlugin,
                                                                         destPlugin,
                                                                         structuralValidation.sourceZoneId,
                                                                         structuralValidation.destZoneId);
    const auto mergeAdvice = destIsRackBusOutput
                                 ? VitParallelMergeAdvice{}
                                 : VitParallelMergePlanner::analyseDestination (*rack->type, destId, destPlugin, sourceId);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rack connect pins");
	const auto effectiveSourceLabel = sourceIsRackBusInput ? juce::String ("RACK_INPUT") : sourceIdStr;
	const auto effectiveDestLabel = destIsRackBusOutput ? juce::String ("RACK_OUTPUT") : destIdStr;
	logRackConnections ("rack_connect_pins before", *rack->type);

    int connectionsChanged = 0;

    if (audioPinGroup)
    {
        connectionsChanged = addLogicalStereoAudioConnections (*rack->type, sourceId, destId);

        if (connectionsChanged <= 0)
            return makeErrorReply ("Failed to add any logical audio rack connections");
    }
    else
    {
        if (! rack->type->addConnection (sourceId, sourcePin, destId, destPin))
            return makeErrorReply ("Failed to add rack connection");

        connectionsChanged = 1;
    }

	logRackConnections ("rack_connect_pins after "
	                    + effectiveSourceLabel + ":" + juce::String (sourcePin)
	                    + "->" + effectiveDestLabel + ":" + juce::String (destPin),
	                    *rack->type);

    rack->type->flushStateToValueTree();
    track->flushStateToValueTree();

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "edge_add",
                                                                              "Connected rack edge " + effectiveSourceLabel + " -> " + effectiveDestLabel,
                                                                              trackID,
                                                                              rack->itemID.toString(),
                                                                              {},
                                                                              effectiveSourceLabel,
                                                                              effectiveDestLabel });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Rack connection created but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Rack connection added");
    response->setProperty ("track_id", trackID);
    response->setProperty ("rack_item_id", rack->itemID.toString());
    response->setProperty ("source_id", effectiveSourceLabel);
    response->setProperty ("source_pin", sourcePin);
    response->setProperty ("dest_id", effectiveDestLabel);
    response->setProperty ("dest_pin", destPin);
    response->setProperty ("audio_pin_group", audioPinGroup);
    response->setProperty ("connections_changed", connectionsChanged);
    response->setProperty ("source_zone_id", structuralValidation.sourceZoneId);
    response->setProperty ("dest_zone_id", structuralValidation.destZoneId);
    response->setProperty ("buffer_adapter_mode", adapterAdvice.mode);
    response->setProperty ("requires_dummy_audio_padding", adapterAdvice.requiresDummyAudioPadding);
    response->setProperty ("passthrough_midi", adapterAdvice.passthroughMidi);
    response->setProperty ("passthrough_audio", adapterAdvice.passthroughAudio);
    response->setProperty ("incoming_branch_count", mergeAdvice.incomingBranchCount);

    if (adapterAdvice.note.isNotEmpty())
        response->setProperty ("buffer_adapter_note", adapterAdvice.note);

    if (mergeAdvice.requiresAttention)
        response->setProperty ("merge_warning", mergeAdvice.summary);
    appendGraphRevisionProperties (*response, graphSnapshot);

    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleRackRemoveConnection (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto rackItemId = object.getProperty ("rack_item_id").toString().trim();
    const auto sourceIdStr = object.getProperty ("source_id").toString().trim();
    const auto destIdStr = object.getProperty ("dest_id").toString().trim();
    const auto sourcePinVar = object.getProperty ("source_pin");
    const auto destPinVar = object.getProperty ("dest_pin");

    if (trackID.isEmpty())
        return makeErrorReply ("rack_remove_connection requires track_id");

    const bool sourceIsRackBusInput = isRackBusInputSourceToken (sourceIdStr);
    const bool destIsRackBusOutput = isRackBusOutputDestToken (destIdStr);

    if (! destIsRackBusOutput && destIdStr.isEmpty())
        return makeErrorReply ("rack_remove_connection requires non-empty dest_id");

    if ((! sourcePinVar.isInt() && ! sourcePinVar.isInt64())
        || (! destPinVar.isInt() && ! destPinVar.isInt64()))
        return makeErrorReply ("rack_remove_connection requires integer source_pin and dest_pin");

    auto* track = findTrackByID (*edit, trackID);
    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* rack = findRackInstanceOnTrack (*track, rackItemId);
    if (rack == nullptr || rack->type == nullptr)
        return makeErrorReply ("No rack instance found on the specified track");

    te::EditItemID destId;

    if (destIsRackBusOutput)
        destId = {};
    else
    {
        destId = te::EditItemID::fromString (destIdStr);

        if (! destId.isValid())
            return makeErrorReply ("dest_id must be a valid EditItemID string");

        if (rack->type->getPluginForID (destId) == nullptr)
            return makeErrorReply ("dest_id is not present in the specified rack");
    }

    te::EditItemID sourceId;

    if (sourceIsRackBusInput)
        sourceId = {};
    else
        sourceId = te::EditItemID::fromString (sourceIdStr);

    if (! sourceIsRackBusInput && ! sourceId.isValid())
        return makeErrorReply ("source_id must be a valid EditItemID string, empty, or RACK_INPUT");

    if (! sourceIsRackBusInput && rack->type->getPluginForID (sourceId) == nullptr)
        return makeErrorReply ("source_id is not present in the specified rack");

    const int sourcePin = static_cast<int> (sourcePinVar);
    const int destPin = static_cast<int> (destPinVar);
    const bool audioPinGroup = static_cast<bool> (object.getProperty ("audio_pin_group"))
                               && sourcePin >= 1
                               && destPin >= 1;

    const auto effectiveSourceLabel = sourceIsRackBusInput ? juce::String ("RACK_INPUT") : sourceIdStr;
    const auto effectiveDestLabel = destIsRackBusOutput ? juce::String ("RACK_OUTPUT") : destIdStr;

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rack remove connection");
	logRackConnections ("rack_remove_connection before", *rack->type);

    int connectionsChanged = 0;

    if (audioPinGroup)
    {
        connectionsChanged = removeLogicalAudioConnections (*rack->type, sourceId, destId);

        if (connectionsChanged <= 0)
            return makeErrorReply ("Rack connection not found");
    }
    else
    {
        if (! rack->type->removeConnection (sourceId, sourcePin, destId, destPin))
            return makeErrorReply ("Rack connection not found");

        connectionsChanged = 1;
    }

	logRackConnections ("rack_remove_connection after "
	                    + effectiveSourceLabel + ":" + juce::String (sourcePin)
	                    + "->" + effectiveDestLabel + ":" + juce::String (destPin),
	                    *rack->type);

    rack->type->flushStateToValueTree();
    track->flushStateToValueTree();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "edge_remove",
                                                                              "Removed rack edge " + effectiveSourceLabel + " -> " + effectiveDestLabel,
                                                                              trackID,
                                                                              rack->itemID.toString(),
                                                                              {},
                                                                              effectiveSourceLabel,
                                                                              effectiveDestLabel });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Rack connection removed but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Rack connection removed");
    response->setProperty ("track_id", trackID);
    response->setProperty ("rack_item_id", rack->itemID.toString());
    response->setProperty ("source_id", effectiveSourceLabel);
    response->setProperty ("source_pin", sourcePin);
    response->setProperty ("dest_id", effectiveDestLabel);
    response->setProperty ("dest_pin", destPin);
    response->setProperty ("audio_pin_group", audioPinGroup);
    response->setProperty ("connections_changed", connectionsChanged);
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String CommandDispatcher::handleRackSetNodeClipScope (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto rackItemId = object.getProperty ("rack_item_id").toString().trim();
    const auto pluginItemIdStr = object.getProperty ("plugin_item_id").toString().trim();

    if (trackID.isEmpty())
        return makeErrorReply ("rack_set_node_clip_scope requires track_id");

    if (pluginItemIdStr.isEmpty())
        return makeErrorReply ("rack_set_node_clip_scope requires plugin_item_id");

    auto* track = findTrackByID (*edit, trackID);
    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    const auto clipScope = VitClipRouteRegistry::normaliseClipScope (object.getProperty ("clip_scope").toString());

    if (clipScope == "debug_global")
        return makeErrorReply ("clip_scope for a node must be track or clip:<clip_id>");

    if (const auto clipValidation = VitClipRouteRegistry::validateClipScope (*track, clipScope); clipValidation.failed())
        return makeErrorReply (clipValidation.getErrorMessage());

    auto* rack = findRackInstanceOnTrack (*track, rackItemId);
    if (rack == nullptr || rack->type == nullptr)
        return makeErrorReply ("No rack instance found on the specified track");

    const auto pluginItemId = te::EditItemID::fromString (pluginItemIdStr);

    if (! pluginItemId.isValid())
        return makeErrorReply ("plugin_item_id must be a valid EditItemID string");

    if (rack->type->getPluginForID (pluginItemId) == nullptr)
        return makeErrorReply ("plugin_item_id is not present in the specified rack");

    auto pluginInstanceState = findRackPluginInstanceState (*rack->type, pluginItemId);

    if (! pluginInstanceState.isValid())
        return makeErrorReply ("Rack plugin instance state not found");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rack set node clip scope");
    pluginInstanceState.setProperty ("vit_clip_scope", clipScope, &undo);

    rack->type->flushStateToValueTree();
    track->flushStateToValueTree();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "rack_clip_scope",
                                                                              "Set rack node clip scope",
                                                                              trackID,
                                                                              rack->itemID.toString(),
                                                                              pluginItemIdStr,
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Clip scope updated but project save failed");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Rack node clip scope updated");
    response->setProperty ("track_id", trackID);
    response->setProperty ("rack_item_id", rack->itemID.toString());
    response->setProperty ("plugin_item_id", pluginItemIdStr);
    response->setProperty ("clip_scope", clipScope);
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
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
