#include "PluginRackControlService.h"

#include "TiledSpectrogramBaker.h"

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
#include "../Core/VitPluginGrabberProjectProfile.h"
#include "../Core/VitPluginGrabberGlobalProfile.h"
#include "../Core/VitPluginGrabberProfileFormat.h"
#include "../Core/VitKernelUtils.h"
#include "../Core/VitParamSurface.h"
#include "../Core/VitPluginTemplateRegistry.h"
#include "../Core/VitProjectHealthCheck.h"
#include "../Core/VitTakeHistoryStack.h"
#include "../Core/VitZoneBufferAdapter.h"

#include <cctype>
#include <cmath>
#include <cstdlib>
#include <limits>
#include <optional>
#include <unordered_map>
#include <unordered_set>
#include <utility>
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

juce::Array<juce::var> stringArrayToVarArray (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return out;
}

juce::ValueTree findRackPluginInstanceState (te::RackType& rackType, te::EditItemID pluginItemId);
juce::Result resolveExternalPluginDescription (te::Edit& edit, const juce::String& pluginPath, juce::PluginDescription& outDesc);
te::Plugin* findPluginInEdit (te::Edit& edit, const juce::String& pluginIdStr);
juce::String describeExternalPluginLoadState (te::Edit& edit, te::ExternalPlugin& plugin);
bool ensureExternalPluginInstanceReady (te::Edit& edit,
                                        te::ExternalPlugin& plugin,
                                        const juce::String& context,
                                        int timeoutMs);
juce::String inferPluginFormatNameFromPath (const juce::String& pluginPath);
juce::var pluginDescriptionToJson (const juce::PluginDescription& desc);
bool pluginDescriptionMatchesQuery (const juce::PluginDescription& desc, const juce::String& query);
void normaliseAndRegisterExternalPluginDescription (te::Edit& edit,
                                                    juce::PluginDescription& desc,
                                                    const juce::String& fallbackPath,
                                                    const juce::String& context);
juce::var createControlGraphState (te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, te::Edit& edit);
void appendGraphRevisionProperties (juce::DynamicObject& object, const VitGraphRevisionSnapshot& snapshot);
juce::File getEffectiveProjectFile (const PluginRackControlService::CurrentProjectPathGetter& getter);
/** When RackType::addPlugin skips auto-connect (non-empty rack), chain new plugin after the unique tail feeding rack outputs. */
bool vitTryChainNewRackPluginSerial (te::RackType& rackType, te::Plugin& newPlugin);

constexpr int kPluginRackInsertReadyTimeoutMs = 0;
constexpr int kPluginOpenUiReadyTimeoutMs = 0;
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
	juce::Logger::writeToLog ("PluginRackControlService: " + label
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

bool readNumericVarLoose (const juce::var& value, double& out)
{
    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        out = static_cast<double> (value);
        return true;
    }

    if (value.isString())
    {
        const auto text = value.toString().trim();
        if (text.isEmpty())
            return false;

        const auto* raw = text.toRawUTF8();
        char* end = nullptr;
        const auto parsed = std::strtod (raw, &end);
        if (end != raw && std::isfinite (parsed))
        {
            out = parsed;
            return true;
        }
    }

    return false;
}

bool readNumericProperty (const juce::DynamicObject& object, const juce::StringArray& keys, double& out)
{
    for (const auto& k : keys)
    {
        const auto v = object.getProperty (k);

        if (readNumericVarLoose (v, out))
            return true;
    }

    return false;
}

bool readNumericPropertyWithKey (const juce::DynamicObject& object,
                                 const juce::StringArray& keys,
                                 double& out,
                                 juce::String& matchedKey)
{
    for (const auto& k : keys)
    {
        const auto v = object.getProperty (k);

        if (readNumericVarLoose (v, out))
        {
            matchedKey = k;
            return true;
        }
    }

    return false;
}

juce::StringArray stringArrayFromVarArray (const juce::var& value)
{
    juce::StringArray out;
    if (auto* array = value.getArray())
        for (const auto& item : *array)
            if (const auto text = item.toString().trim(); text.isNotEmpty())
                out.addIfNotAlreadyThere (text);

    return out;
}

juce::String normalisedResolverToken (juce::String text)
{
    text = text.trim().toLowerCase();
    return text.retainCharacters ("abcdefghijklmnopqrstuvwxyz0123456789");
}

juce::String firstNonEmptyProperty (const juce::DynamicObject& object, std::initializer_list<const char*> keys)
{
    for (const auto* key : keys)
    {
        const auto value = object.getProperty (key).toString().trim();
        if (value.isNotEmpty())
            return value;
    }

    return {};
}

bool boolProfileProperty (const juce::DynamicObject& object, const char* key)
{
    const auto value = object.getProperty (key);
    if (value.isBool())
        return static_cast<bool> (value);

    const auto text = value.toString().trim().toLowerCase();
    return text == "true" || text == "1" || text == "yes" || text == "on";
}

bool profileMappingAllowsRuntime (const juce::DynamicObject& object)
{
    const auto source = object.getProperty ("source").toString().trim().toLowerCase();
    if (boolProfileProperty (object, "locked") || source.contains ("user_demonstrated"))
        return true;

    const auto status = object.getProperty ("status").toString().trim().toLowerCase();
    if (status.isEmpty())
        return true;

    if (status != "active")
        return false;

    const auto confidenceVar = object.getProperty ("confidence");
    if (! confidenceVar.isDouble() && ! confidenceVar.isInt() && ! confidenceVar.isInt64())
        return true;

    return static_cast<double> (confidenceVar) >= 0.70;
}

juce::String paramIdFromProfileMapping (const juce::var& mapping)
{
    if (auto* object = mapping.getDynamicObject())
    {
        if (! profileMappingAllowsRuntime (*object))
            return {};
        return firstNonEmptyProperty (*object, { "param_id", "id", "raw_param_id", "parameter_id" });
    }

    return mapping.toString().trim();
}

juce::var lookupProfileParamMapping (const juce::var& paramsVar, std::initializer_list<const char*> aliases)
{
    auto* params = paramsVar.getDynamicObject();
    if (params == nullptr)
        return {};

    juce::StringArray normalisedAliases;
    for (const auto* alias : aliases)
        normalisedAliases.addIfNotAlreadyThere (normalisedResolverToken (alias));

    const auto& properties = params->getProperties();
    for (int i = 0; i < properties.size(); ++i)
    {
        const auto slot = normalisedResolverToken (properties.getName (i).toString());
        if (! normalisedAliases.contains (slot))
            continue;

        const auto paramId = paramIdFromProfileMapping (properties.getValueAt (i));
        if (paramId.isNotEmpty())
            return properties.getValueAt (i);
    }

    return {};
}

juce::String lookupProfileParamSlot (const juce::var& paramsVar, std::initializer_list<const char*> aliases)
{
    return paramIdFromProfileMapping (lookupProfileParamMapping (paramsVar, aliases));
}

juce::String lookupGroupParamSlot (const juce::var& groupVar, std::initializer_list<const char*> aliases)
{
    if (auto* group = groupVar.getDynamicObject())
        return lookupProfileParamSlot (group->getProperty (profileField::groupParams), aliases);

    return {};
}

juce::var lookupGroupParamMapping (const juce::var& groupVar, std::initializer_list<const char*> aliases)
{
    if (auto* group = groupVar.getDynamicObject())
        return lookupProfileParamMapping (group->getProperty (profileField::groupParams), aliases);

    return {};
}

juce::var findVirtualControlByName (const juce::Array<juce::var>& virtualControls, const juce::String& requestedControl)
{
    const auto requested = normalisedResolverToken (requestedControl);
    if (requested.isEmpty())
        return {};

    for (const auto& control : virtualControls)
    {
        auto* object = control.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto name = normalisedResolverToken (firstNonEmptyProperty (*object, { "name", "operation", "control" }));
        if (name == requested)
            return control;
    }

    return {};
}

bool isEqRuntimeControl (const juce::String& control)
{
    const auto clean = normalisedResolverToken (control);
    return clean.contains ("eqcut")
        || clean.contains ("eqboost")
        || clean.contains ("eqset")
        || clean.contains ("cutregion")
        || clean.contains ("boostregion")
        || clean.contains ("reducemud")
        || clean.contains ("mud")
        || clean.contains ("harsh")
        || clean.contains ("presence");
}

bool isBoostRuntimeControl (const juce::String& control)
{
    const auto clean = normalisedResolverToken (control);
    return clean.contains ("boost") || clean.contains ("add") || clean.contains ("presence");
}

juce::var findProfileGroupById (const juce::Array<juce::var>& groups, const juce::String& groupId)
{
    const auto requested = normalisedResolverToken (groupId);
    if (requested.isEmpty())
        return {};

    for (const auto& group : groups)
    {
        auto* object = group.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto id = normalisedResolverToken (firstNonEmptyProperty (*object, { "id", "component_id", "name", "label" }));
        if (id == requested)
            return group;
    }

    return {};
}

juce::var findParameterDescriptorById (const juce::Array<juce::var>& parameterDescriptors, const juce::String& paramId)
{
    for (const auto& parameter : parameterDescriptors)
        if (auto* object = parameter.getDynamicObject())
            if (object->getProperty ("id").toString().trim() == paramId)
                return parameter;

    return {};
}

bool parseDisplayNumber (juce::String text, double& out)
{
    const auto lower = text.toLowerCase();
    auto numeric = lower.retainCharacters ("0123456789.-");
    if (numeric.isEmpty() || numeric == "-" || numeric == ".")
        return false;

    out = numeric.getDoubleValue();
    if (lower.contains ("khz") || lower.contains (" k"))
        out *= 1000.0;

    return std::isfinite (out);
}

std::optional<double> currentDisplayNumberForParam (te::Plugin& plugin, const juce::String& paramId)
{
    auto param = resolvePluginParameterByID (plugin, paramId);
    if (param == nullptr)
        return {};

    double value = 0.0;
    if (! parseDisplayNumber (param->getCurrentValueAsString(), value))
        return {};

    return value;
}

juce::var findBestEqBandGroup (te::Plugin& plugin,
                               const juce::Array<juce::var>& groups,
                               const juce::String& requestedComponentId,
                               double targetFrequencyHz)
{
    if (requestedComponentId.isNotEmpty())
    {
        auto group = findProfileGroupById (groups, requestedComponentId);
        if (group.isObject())
            return group;
    }

    double bestScore = std::numeric_limits<double>::max();
    juce::var bestGroup;
    int fallbackOrder = 0;

    for (const auto& group : groups)
    {
        auto* object = group.getDynamicObject();
        if (object == nullptr)
            continue;

        const auto id = firstNonEmptyProperty (*object, { "id", "component_id", "name", "label" }).toLowerCase();
        const auto role = object->getProperty (profileField::groupRole).toString().trim().toLowerCase();
        const auto looksLikeEqBand = (role.contains ("eq") && role.contains ("band"))
                                  || id.contains ("band")
                                  || id.contains ("eq");
        if (! looksLikeEqBand)
            continue;

        const auto frequencyParamId = lookupGroupParamSlot (group, { "frequency", "freq", "freq_hz", "center_frequency", "center_freq", "cutoff" });
        const auto gainParamId = lookupGroupParamSlot (group, { "gain", "gain_db", "level", "amount" });
        if (frequencyParamId.isEmpty() || gainParamId.isEmpty())
            continue;

        double score = 1000000.0 + static_cast<double> (fallbackOrder++);
        if (targetFrequencyHz > 0.0)
        {
            if (const auto currentHz = currentDisplayNumberForParam (plugin, frequencyParamId))
                score = std::abs (std::log (juce::jmax (1.0, *currentHz)) - std::log (juce::jmax (1.0, targetFrequencyHz)));
        }

        if (! bestGroup.isObject() || score < bestScore)
        {
            bestScore = score;
            bestGroup = group;
        }
    }

    return bestGroup;
}

bool readNumericFromTarget (const juce::DynamicObject& command,
                            const juce::DynamicObject& target,
                            std::initializer_list<const char*> keys,
                            double& out)
{
    juce::StringArray keyArray;
    for (const auto* key : keys)
        keyArray.add (key);

    return readNumericProperty (target, keyArray, out)
        || readNumericProperty (command, keyArray, out);
}

bool readStringProperty (const juce::DynamicObject& object, const juce::StringArray& keys, juce::String& out)
{
    for (const auto& k : keys)
    {
        const auto text = object.getProperty (k).toString().trim();
        if (text.isNotEmpty())
        {
            out = text;
            return true;
        }
    }

    return false;
}

bool readStringFromTarget (const juce::DynamicObject& command,
                           const juce::DynamicObject& target,
                           std::initializer_list<const char*> keys,
                           juce::String& out)
{
    juce::StringArray keyArray;
    for (const auto* key : keys)
        keyArray.add (key);

    return readStringProperty (target, keyArray, out)
        || readStringProperty (command, keyArray, out);
}

double defaultEqGainDbForAmount (const juce::String& controlName, const juce::DynamicObject& target)
{
    const auto amount = firstNonEmptyProperty (target, { "amount", "strength", "intensity" }).toLowerCase();
    double magnitude = 2.5;
    if (amount.contains ("strong") || amount.contains ("hard") || amount.contains ("heavy"))
        magnitude = 6.0;
    else if (amount.contains ("medium") || amount.contains ("moderate"))
        magnitude = 4.0;
    else if (amount.contains ("tiny") || amount.contains ("subtle") || amount.contains ("light"))
        magnitude = 1.5;

    const auto control = controlName.toLowerCase();
    return control.contains ("boost") || control.contains ("add") ? magnitude : -magnitude;
}

double defaultEqQForWidth (const juce::DynamicObject& target)
{
    const auto width = firstNonEmptyProperty (target, { "width", "range", "bandwidth" }).toLowerCase();
    if (width.contains ("narrow"))
        return 2.4;
    if (width.contains ("wide"))
        return 0.7;
    return 1.1;
}

struct ResolvedApplyValue
{
    float value = 0.0f;
    bool normalised = false;
    juce::String mode;
    bool ok = true;
    juce::String error;
};

struct RuntimeDisplayDomain
{
    bool present = false;
    bool hasRange = false;
    bool confirmed = false;
    bool isEnum = false;
    double minValue = 0.0;
    double maxValue = 1.0;
    juce::String unit;
    juce::String scale;
    juce::String status;
    juce::String text;
    juce::StringArray enumLabels;
};

float normalisedLogValue (double value, double minValue, double maxValue)
{
    value = juce::jlimit (minValue, maxValue, value);
    return static_cast<float> (std::log (value / minValue) / std::log (maxValue / minValue));
}

float normalisedLinearValue (double value, double minValue, double maxValue)
{
    value = juce::jlimit (minValue, maxValue, value);
    return static_cast<float> ((value - minValue) / (maxValue - minValue));
}

bool readNumericVar (const juce::var& value, double& out)
{
    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        out = static_cast<double> (value);
        return true;
    }

    return false;
}

bool asciiDigitOrDot (char c)
{
    return (c >= '0' && c <= '9') || c == '.';
}

bool previousNonSpaceByteIsNumber (const char* raw, const char* current)
{
    auto* p = current;
    while (p > raw)
    {
        --p;
        const auto c = static_cast<unsigned char> (*p);
        if (std::isspace (c))
            continue;

        return asciiDigitOrDot (static_cast<char> (c));
    }

    return false;
}

bool parseDisplayDomainRangeText (const juce::String& text, double& minValue, double& maxValue)
{
    std::vector<double> values;
    const auto* raw = text.toRawUTF8();
    for (auto* p = raw; *p != 0 && values.size() < 2;)
    {
        const auto c = *p;
        const auto sign = c == '-' || c == '+';
        if (sign && previousNonSpaceByteIsNumber (raw, p))
        {
            ++p;
            continue;
        }

        if (asciiDigitOrDot (c) || sign)
        {
            char* end = nullptr;
            const auto parsed = std::strtod (p, &end);
            if (end != p && std::isfinite (parsed))
            {
                values.push_back (parsed);
                p = end;
                continue;
            }
        }

        ++p;
    }

    if (values.size() < 2)
        return false;

    minValue = juce::jmin (values[0], values[1]);
    maxValue = juce::jmax (values[0], values[1]);
    return std::abs (maxValue - minValue) > 0.000001;
}

void fillDisplayDomainFromText (RuntimeDisplayDomain& domain)
{
    const auto text = domain.text.trim();
    if (text.isEmpty())
        return;

    const auto lower = text.toLowerCase();
    if (domain.scale == "enum" || lower.startsWith ("enum:") || lower.startsWith ("enum "))
    {
        domain.isEnum = true;
        domain.scale = "enum";
        auto labelsText = text;
        if (lower.startsWith ("enum:"))
            labelsText = text.substring (5);
        else if (lower.startsWith ("enum "))
            labelsText = text.substring (5);

        for (const auto& label : juce::StringArray::fromTokens (labelsText, "/", ""))
        {
            const auto trimmed = label.trim();
            if (trimmed.isNotEmpty())
                domain.enumLabels.add (trimmed);
        }

        return;
    }

    if (domain.unit.isEmpty())
    {
        if (lower.contains ("db"))
            domain.unit = "dB";
        else if (lower.contains ("khz") || lower.contains ("hz"))
            domain.unit = "Hz";
        else if (text.contains ("%"))
            domain.unit = "%";
        else if (lower.contains ("ms"))
            domain.unit = "ms";
        else if (lower.contains ("sec") || lower.contains ("second"))
            domain.unit = "s";
    }

    if (domain.scale.isEmpty())
        domain.scale = domain.unit == "Hz" ? "log" : "linear";

    if (! domain.hasRange)
    {
        double minValue = 0.0;
        double maxValue = 0.0;
        if (parseDisplayDomainRangeText (text, minValue, maxValue))
        {
            domain.minValue = minValue;
            domain.maxValue = maxValue;
            domain.hasRange = true;
        }
    }
}

RuntimeDisplayDomain displayDomainFromProfileMapping (const juce::var& mapping)
{
    RuntimeDisplayDomain out;
    auto* mappingObject = mapping.getDynamicObject();
    if (mappingObject == nullptr)
        return out;

    out.confirmed = static_cast<bool> (mappingObject->getProperty ("confirmed"));
    if (auto* domain = mappingObject->getProperty ("display_domain").getDynamicObject())
    {
        out.present = true;
        out.text = domain->getProperty ("text").toString().trim();
        out.unit = domain->getProperty ("unit").toString().trim();
        out.scale = domain->getProperty ("scale").toString().trim().toLowerCase();
        out.status = domain->getProperty ("status").toString().trim().toLowerCase();
        double minValue = 0.0;
        double maxValue = 0.0;
        const auto hasMin = readNumericVar (domain->getProperty ("min"), minValue);
        const auto hasMax = readNumericVar (domain->getProperty ("max"), maxValue);
        if (hasMin && hasMax)
        {
            out.minValue = juce::jmin (minValue, maxValue);
            out.maxValue = juce::jmax (minValue, maxValue);
            out.hasRange = std::abs (out.maxValue - out.minValue) > 0.000001;
        }
    }

    const auto text = mappingObject->getProperty ("display_domain_text").toString().trim();
    if (out.text.isEmpty() && text.isNotEmpty())
    {
        out.present = true;
        out.text = text;
    }

    fillDisplayDomainFromText (out);
    if (out.confirmed && out.hasRange && (out.status.isEmpty() || out.status == "needs_confirmation" || out.status == "unknown"))
        out.status = "confirmed";

    return out;
}

bool displayDomainUsableForDisplayValue (const RuntimeDisplayDomain& domain)
{
    if (! domain.present || ! domain.hasRange)
        return false;

    if (domain.confirmed)
        return true;

    if (domain.status == "needs_confirmation" || domain.status == "unknown")
        return false;

    return true;
}

bool targetKeyImpliesDisplayValue (const juce::String& targetKey)
{
    const auto key = targetKey.trim().toLowerCase();
    return key.contains ("_db") || key.contains ("db")
        || key.contains ("_hz") || key.contains ("hz")
        || key.contains ("percent") || key.contains ("pct")
        || key.contains ("display");
}

bool targetKeyIsGenericAmountValue (const juce::String& targetKey)
{
    const auto key = targetKey.trim().toLowerCase();
    return key == "amount" || key == "value" || key == "target_value";
}

bool slotUsuallyUsesDisplayValue (const juce::String& slot, double requestedValue)
{
    const auto cleanSlot = normalisedResolverToken (slot);
    if (requestedValue >= 0.0 && requestedValue <= 1.0)
        return false;

    return cleanSlot.contains ("freq") || cleanSlot.contains ("cutoff")
        || cleanSlot.contains ("gain") || cleanSlot.contains ("level")
        || cleanSlot.contains ("threshold")
        || cleanSlot == "q" || cleanSlot.contains ("quality") || cleanSlot.contains ("width");
}

bool requestedModeImpliesDisplayValue (const juce::String& requestedMode)
{
    const auto mode = requestedMode.trim().toLowerCase();
    return mode == "display" || mode == "display_value" || mode == "semantic"
        || mode == "relative_delta" || mode == "display_delta";
}

double displayValueFromNormalised (float normalised, const RuntimeDisplayDomain& domain)
{
    const auto value = juce::jlimit (0.0, 1.0, static_cast<double> (normalised));
    if (domain.scale == "log" && domain.minValue > 0.0 && domain.maxValue > domain.minValue)
        return domain.minValue * std::pow (domain.maxValue / domain.minValue, value);

    return domain.minValue + (domain.maxValue - domain.minValue) * value;
}

float normalisedFromDisplayValue (double displayValue, const RuntimeDisplayDomain& domain)
{
    if (domain.scale == "log" && domain.minValue > 0.0 && domain.maxValue > domain.minValue)
        return normalisedLogValue (displayValue, domain.minValue, domain.maxValue);

    return normalisedLinearValue (displayValue, domain.minValue, domain.maxValue);
}

ResolvedApplyValue failedResolvedApplyValue (const juce::String& message)
{
    ResolvedApplyValue out;
    out.ok = false;
    out.error = message;
    return out;
}

ResolvedApplyValue resolveEnumApplyValue (te::AutomatableParameter& param,
                                          const juce::String& requestedText,
                                          const RuntimeDisplayDomain& domain)
{
    const auto requested = normalisedResolverToken (requestedText);
    if (requested.isEmpty())
        return failedResolvedApplyValue ("enum apply control requires a non-empty label for " + param.getParameterName());

    const auto range = param.getValueRange();
    const auto direct = param.stringToValue (requestedText);
    if (std::isfinite (direct) && direct >= range.getStart() && direct <= range.getEnd())
    {
        // Some plug-ins return 0.0 for text they cannot parse.  A range check
        // alone therefore turns an unknown enum label into a valid state-zero
        // write.  Treat the plug-in conversion as usable only when it can
        // round-trip back to the requested label; verified profile mappings
        // and discrete labels remain the authoritative fallbacks below.
        const auto roundTrip = normalisedResolverToken (param.valueToString (direct));
        if (roundTrip == requested)
            return { direct, false, "enum_plugin_text_roundtrip" };
    }

    if (param.isDiscrete())
    {
        const auto numStates = param.getNumberOfStates();
        for (int state = 0; state < numStates; ++state)
        {
            const auto value = param.getValueForState (state);
            const auto label = normalisedResolverToken (param.getLabelForValue (value));
            if (label == requested)
                return { value, false, "enum_discrete_label_match" };
        }
    }

    for (int i = 0; i < domain.enumLabels.size(); ++i)
    {
        if (normalisedResolverToken (domain.enumLabels[i]) != requested)
            continue;

        if (param.isDiscrete() && i < param.getNumberOfStates())
            return { param.getValueForState (i), false, "enum_profile_label_match" };

        if (domain.enumLabels.size() > 1)
        {
            const auto normalised = static_cast<float> (i) / static_cast<float> (domain.enumLabels.size() - 1);
            return { normalised, true, "enum_profile_label_normalised" };
        }
    }

    return failedResolvedApplyValue ("enum apply control could not match \"" + requestedText
                                     + "\" against known labels for " + param.getParameterName());
}

juce::String displayDomainClarificationMessage (const juce::String& slot,
                                                const juce::String& paramName,
                                                const juce::String& targetKey,
                                                const RuntimeDisplayDomain& domain)
{
    juce::String current = "目前这个抓手只掌握后台 0~1 的参数范围";
    if (domain.present && domain.text.isNotEmpty())
        current += "，已记录的显示域是“" + domain.text + "”，但还不足以可靠换算";

    const auto requested = targetKey.isNotEmpty() ? targetKey : slot;
    return current + "。你这次使用的是 " + requested
        + " 这类显示单位控制。请告诉我你在插件 UI 上看到或希望使用的显示范围和单位，例如：-20~+20 dB、0~100%、20~20000 Hz 对数。参数："
        + paramName;
}

ResolvedApplyValue resolveSemanticApplyValue (te::AutomatableParameter& param,
                                               const juce::String& slot,
                                               double requestedValue,
                                               const juce::String& requestedMode,
                                               const juce::var& profileMapping,
                                               const juce::String& targetKey)
{
    const auto cleanMode = requestedMode.trim().toLowerCase();
    const auto cleanSlot = normalisedResolverToken (slot);
    const auto range = param.getValueRange();
    const auto rangeStart = static_cast<double> (range.getStart());
    const auto rangeEnd = static_cast<double> (range.getEnd());
    const auto normalisedRange = rangeStart >= -0.0001 && rangeEnd <= 1.0001;
    const auto displayDomain = displayDomainFromProfileMapping (profileMapping);

    if (cleanMode == "normalised" || cleanMode == "normalized")
        return { juce::jlimit (0.0f, 1.0f, static_cast<float> (requestedValue)), true, "requested_normalised" };

    if (cleanMode == "raw")
        return { juce::jlimit (range.getStart(), range.getEnd(), static_cast<float> (requestedValue)), false, "requested_raw" };

    const auto requestIsDisplayValue = requestedModeImpliesDisplayValue (requestedMode)
        || targetKeyImpliesDisplayValue (targetKey)
        || (displayDomainUsableForDisplayValue (displayDomain) && targetKeyIsGenericAmountValue (targetKey))
        || (normalisedRange && slotUsuallyUsesDisplayValue (slot, requestedValue));
    if (normalisedRange && requestIsDisplayValue)
    {
        if (! displayDomainUsableForDisplayValue (displayDomain))
            return failedResolvedApplyValue ("display_domain_clarification:" + displayDomainClarificationMessage (slot, param.getParameterName(), targetKey, displayDomain));

        auto displayValue = requestedValue;
        if (cleanMode == "relative_delta" || cleanMode == "display_delta")
            displayValue = displayValueFromNormalised (param.getCurrentNormalisedValue(), displayDomain) + requestedValue;

        return { normalisedFromDisplayValue (displayValue, displayDomain), true,
                 cleanMode == "relative_delta" || cleanMode == "display_delta" ? "display_domain_relative_delta" : "display_domain_absolute" };
    }

    if (normalisedRange && targetKeyIsGenericAmountValue (targetKey) && (requestedValue < 0.0 || requestedValue > 1.0))
        return failedResolvedApplyValue ("display_domain_clarification:" + displayDomainClarificationMessage (slot, param.getParameterName(), targetKey, displayDomain));

    if (normalisedRange)
    {
        if (cleanSlot.contains ("freq") || cleanSlot.contains ("cutoff"))
            return { normalisedLogValue (requestedValue, 10.0, 40000.0), true, "semantic_frequency_log_10_40000" };

        if (cleanSlot == "q" || cleanSlot.contains ("quality") || cleanSlot.contains ("width"))
            return { normalisedLogValue (requestedValue, 0.1, 6.0), true, "semantic_q_log_0p1_6" };

        if (cleanSlot.contains ("gain") || cleanSlot.contains ("level") || cleanSlot.contains ("amount"))
            return { normalisedLinearValue (requestedValue, -18.0, 18.0), true, "semantic_gain_db_linear_-18_18" };

        if (cleanSlot.contains ("threshold"))
            return { normalisedLinearValue (requestedValue, -50.0, 0.0), true, "semantic_threshold_db_linear_-50_0" };
    }

    for (const auto& text : { juce::String (requestedValue, 4),
                              juce::String (requestedValue, 4) + " Hz",
                              juce::String (requestedValue, 4) + " dB" })
    {
        const auto converted = param.stringToValue (text);
        if (std::isfinite (converted) && converted >= range.getStart() && converted <= range.getEnd())
            return { converted, false, "plugin_text_conversion" };
    }

    if (requestedValue >= rangeStart && requestedValue <= rangeEnd)
        return { static_cast<float> (requestedValue), false, "semantic_raw_within_range" };

    if (requestedValue >= 0.0 && requestedValue <= 1.0)
        return { static_cast<float> (requestedValue), true, "semantic_normalised_fallback" };

    return { juce::jlimit (range.getStart(), range.getEnd(), static_cast<float> (requestedValue)), false, "semantic_clamped_raw_fallback" };
}

juce::var makeAppliedParameterRecord (const juce::String& slot,
                                      const juce::String& paramId,
                                      te::AutomatableParameter& param,
                                      double requestedValue,
                                      const ResolvedApplyValue& resolved,
                                      float oldValue)
{
    auto row = std::make_unique<juce::DynamicObject>();
    row->setProperty ("slot", slot);
    row->setProperty ("param_id", paramId);
    row->setProperty ("param_name", param.getParameterName());
    row->setProperty ("requested_value", requestedValue);
    row->setProperty ("value_mode", resolved.mode);
    row->setProperty ("applied_value", resolved.value);
    row->setProperty ("applied_as_normalised", resolved.normalised);
    row->setProperty ("old_value", oldValue);
    row->setProperty ("new_value", param.getCurrentValue());
    row->setProperty ("new_normalised_value", param.getCurrentNormalisedValue());
    row->setProperty ("new_value_text", param.getCurrentValueAsString());
    return juce::var (row.release());
}

std::unordered_set<std::string> collectCurrentParamIds (const juce::Array<juce::var>& parameterDescriptors)
{
    std::unordered_set<std::string> validParamIds;
    for (const auto& parameter : parameterDescriptors)
        if (auto* object = parameter.getDynamicObject())
            if (const auto id = object->getProperty ("id").toString().trim(); id.isNotEmpty())
                validParamIds.insert (id.toStdString());

    return validParamIds;
}

juce::Result applyRuntimeProfileParam (te::Plugin& plugin,
                                       const std::unordered_set<std::string>& validParamIds,
                                       const juce::StringArray& staleParamIds,
                                       const juce::String& slot,
                                       const juce::String& paramId,
                                       double requestedValue,
                                       const juce::String& requestedMode,
                                       const juce::var& profileMapping,
                                       const juce::String& targetKey,
                                       juce::Array<juce::var>& applied)
{
    if (paramId.isEmpty())
        return juce::Result::ok();

    if (! validParamIds.contains (paramId.toStdString()))
        return juce::Result::fail ("runtime profile mapped " + slot + " to param_id not in current snapshot: " + paramId);

    if (staleParamIds.contains (paramId))
        return juce::Result::fail ("runtime profile mapped " + slot + " to stale param_id: " + paramId);

    auto param = resolvePluginParameterByID (plugin, paramId);
    if (param == nullptr)
        return juce::Result::fail ("runtime profile mapped " + slot + " to unresolved param_id: " + paramId);

    const auto previousValue = param->getCurrentValue();
    const auto resolved = resolveSemanticApplyValue (*param, slot, requestedValue, requestedMode, profileMapping, targetKey);
    if (! resolved.ok)
        return juce::Result::fail (resolved.error);

    param->parameterChangeGestureBegin();
    if (resolved.normalised)
        param->setNormalisedParameter (resolved.value, juce::sendNotification);
    else
        param->setParameter (resolved.value, juce::sendNotification);
    param->parameterChangeGestureEnd();

    applied.add (makeAppliedParameterRecord (slot, paramId, *param, requestedValue, resolved, previousValue));
    return juce::Result::ok();
}

juce::Result applyRuntimeProfileEnumParam (te::Plugin& plugin,
                                           const std::unordered_set<std::string>& validParamIds,
                                           const juce::StringArray& staleParamIds,
                                           const juce::String& slot,
                                           const juce::String& paramId,
                                           const juce::String& requestedText,
                                           const RuntimeDisplayDomain& domain,
                                           juce::Array<juce::var>& applied)
{
    if (paramId.isEmpty())
        return juce::Result::ok();

    if (! validParamIds.contains (paramId.toStdString()))
        return juce::Result::fail ("runtime profile mapped " + slot + " to param_id not in current snapshot: " + paramId);

    if (staleParamIds.contains (paramId))
        return juce::Result::fail ("runtime profile mapped " + slot + " to stale param_id: " + paramId);

    auto param = resolvePluginParameterByID (plugin, paramId);
    if (param == nullptr)
        return juce::Result::fail ("runtime profile mapped " + slot + " to unresolved param_id: " + paramId);

    const auto previousValue = param->getCurrentValue();
    const auto resolved = resolveEnumApplyValue (*param, requestedText, domain);
    if (! resolved.ok)
        return juce::Result::fail (resolved.error);

    param->parameterChangeGestureBegin();
    if (resolved.normalised)
        param->setNormalisedParameter (resolved.value, juce::sendNotification);
    else
        param->setParameter (resolved.value, juce::sendNotification);
    param->parameterChangeGestureEnd();

    applied.add (makeAppliedParameterRecord (slot, paramId, *param, 0.0, resolved, previousValue));
    return juce::Result::ok();
}

juce::var cloneVarObject (const juce::var& value)
{
    if (auto* object = value.getDynamicObject())
        return juce::var (object->clone().release());

    return {};
}

bool hasUsableRuntimeProfile (const VitPluginGrabberProjectProfile::MergeResult& profileMerge)
{
    return profileMerge.profileApplied || profileMerge.globalProfileApplied || profileMerge.hasExtendedFields();
}

double clampBySafety (double value,
                      const juce::var& safety,
                      const char* minKey,
                      const char* maxKey)
{
    auto* object = safety.getDynamicObject();
    if (object == nullptr)
        return value;

    double minValue = 0.0;
    juce::StringArray minKeys;
    minKeys.add (minKey);
    if (readNumericProperty (*object, minKeys, minValue))
        value = juce::jmax (minValue, value);

    double maxValue = 0.0;
    juce::StringArray maxKeys;
    maxKeys.add (maxKey);
    if (readNumericProperty (*object, maxKeys, maxValue))
        value = juce::jmin (maxValue, value);

    return value;
}

double clampGainBySafety (double gainDb, const juce::var& safety)
{
    auto* object = safety.getDynamicObject();
    if (object == nullptr)
        return gainDb;

    double maxAbsGainDb = 0.0;
    juce::StringArray keys;
    keys.add (profileField::safetyMaxGainDb);
    if (! readNumericProperty (*object, keys, maxAbsGainDb) || maxAbsGainDb <= 0.0)
        return gainDb;

    return juce::jlimit (-std::abs (maxAbsGainDb), std::abs (maxAbsGainDb), gainDb);
}

void addStringKeys (juce::StringArray& keys, std::initializer_list<const char*> values)
{
    for (const auto* value : values)
        keys.addIfNotAlreadyThere (value);
}

juce::var makeControlResolutionRecord (const juce::String& resolver,
                                       const juce::String& componentId,
                                       const juce::var& virtualControl,
                                       const juce::var& group)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("resolver", resolver);
    object->setProperty ("component_id", componentId);
    if (virtualControl.isObject())
        object->setProperty ("virtual_control", cloneVarObject (virtualControl));
    if (group.isObject())
        object->setProperty ("group", cloneVarObject (group));
    return juce::var (object.release());
}

bool readNumericTargetValue (const juce::DynamicObject& command,
                             const juce::DynamicObject& target,
                             const juce::String& slot,
                             const juce::StringArray& inputKeys,
                             double& out,
                             juce::String& matchedKey)
{
    juce::StringArray keys;
    keys.add (slot);
    keys.add (slot.toLowerCase());

    const auto normalisedSlot = normalisedResolverToken (slot);
    bool semanticSlot = false;
    if (normalisedSlot.contains ("freq"))
    {
        addStringKeys (keys, { "freq_hz", "frequency_hz", "frequency", "freq", "center_frequency", "center_freq", "cutoff" });
        semanticSlot = true;
    }
    else if (normalisedSlot == "q" || normalisedSlot.contains ("quality") || normalisedSlot.contains ("width"))
    {
        addStringKeys (keys, { "q", "quality", "width", "bandwidth" });
        semanticSlot = true;
    }
    else if (normalisedSlot.contains ("gain") || normalisedSlot.contains ("level") || normalisedSlot.contains ("amount"))
    {
        addStringKeys (keys, { "gain_db", "gain", "level_db", "level", "amount_db", "amount" });
        semanticSlot = true;
    }
    else if (normalisedSlot.contains ("threshold"))
    {
        addStringKeys (keys, { "threshold_db", "threshold" });
        semanticSlot = true;
    }

    if (! semanticSlot)
    {
        addStringKeys (keys, { "value", "target_value", "display_value" });
        for (const auto& inputKey : inputKeys)
        {
            const auto key = inputKey.trim();
            if (key.isEmpty())
                continue;

            keys.addIfNotAlreadyThere (key);
            keys.addIfNotAlreadyThere (key.toLowerCase());
            const auto cleanKey = normalisedResolverToken (key);
            if (cleanKey.contains ("amount") || cleanKey.contains ("value"))
                addStringKeys (keys, { "amount", "amount_percent", "percent", "pct" });
        }
    }

    return readNumericPropertyWithKey (target, keys, out, matchedKey)
        || readNumericPropertyWithKey (command, keys, out, matchedKey);
}

bool readEnumTargetValue (const juce::DynamicObject& command,
                          const juce::DynamicObject& target,
                          const juce::String& slot,
                          juce::String& out)
{
    juce::StringArray keys;
    keys.add (slot);
    keys.add (slot.toLowerCase());

    const auto normalisedSlot = normalisedResolverToken (slot);
    if (normalisedSlot.contains ("type") || normalisedSlot.contains ("shape"))
        addStringKeys (keys, { "type", "shape", "filter_type", "response_shape", "band_type" });
    else if (normalisedSlot.contains ("enable"))
        addStringKeys (keys, { "enabled", "enable" });

    return readStringProperty (target, keys, out) || readStringProperty (command, keys, out);
}

juce::Result applyVirtualControlParams (te::Plugin& plugin,
                                        const std::unordered_set<std::string>& validParamIds,
                                        const juce::StringArray& staleParamIds,
                                        const juce::var& virtualControl,
                                        const juce::DynamicObject& command,
                                        const juce::DynamicObject& target,
                                        const juce::var& safety,
                                        juce::Array<juce::var>& applied)
{
    auto* control = virtualControl.getDynamicObject();
    if (control == nullptr)
        return juce::Result::fail ("virtual control is missing from runtime profile");

    auto* params = control->getProperty (profileField::groupParams).getDynamicObject();
    if (params == nullptr)
        params = control->getProperty ("params").getDynamicObject();
    if (params == nullptr)
        return juce::Result::fail ("virtual control has no params mapping");

    const auto requestedMode = command.getProperty ("value_mode").toString().trim();
    const auto inputKeys = stringArrayFromVarArray (control->getProperty ("inputs"));
    const auto& properties = params->getProperties();
    for (int i = 0; i < properties.size(); ++i)
    {
        const auto slot = properties.getName (i).toString().trim();
        const auto profileMapping = properties.getValueAt (i);
        const auto paramId = paramIdFromProfileMapping (profileMapping);
        if (slot.isEmpty() || paramId.isEmpty())
            continue;

        double value = 0.0;
        juce::String targetKey;
        if (! readNumericTargetValue (command, target, slot, inputKeys, value, targetKey))
        {
            const auto domain = displayDomainFromProfileMapping (profileMapping);
            juce::String enumText;
            if (! domain.isEnum || ! readEnumTargetValue (command, target, slot, enumText))
                continue;

            auto param = resolvePluginParameterByID (plugin, paramId);
            if (param == nullptr)
                return juce::Result::fail ("runtime profile mapped " + slot + " to unresolved param_id: " + paramId);

            const auto previousValue = param->getCurrentValue();
            const auto resolved = resolveEnumApplyValue (*param, enumText, domain);
            if (! resolved.ok)
                return juce::Result::fail (resolved.error);

            param->parameterChangeGestureBegin();
            if (resolved.normalised)
                param->setNormalisedParameter (resolved.value, juce::sendNotification);
            else
                param->setParameter (resolved.value, juce::sendNotification);
            param->parameterChangeGestureEnd();

            applied.add (makeAppliedParameterRecord (slot, paramId, *param, 0.0, resolved, previousValue));
            continue;
        }

        const auto cleanSlot = normalisedResolverToken (slot);
        if (cleanSlot.contains ("gain") || cleanSlot.contains ("level") || cleanSlot.contains ("amount"))
            value = clampGainBySafety (value, safety);
        else if (cleanSlot == "q" || cleanSlot.contains ("quality") || cleanSlot.contains ("width"))
            value = clampBySafety (value, safety, profileField::safetyMinQ, profileField::safetyMaxQ);
        else if (cleanSlot.contains ("threshold"))
            value = clampBySafety (value, safety, profileField::safetyMinThreshold, profileField::safetyMaxThreshold);

        if (const auto result = applyRuntimeProfileParam (plugin,
                                                          validParamIds,
                                                          staleParamIds,
                                                          slot,
                                                           paramId,
                                                           value,
                                                           requestedMode,
                                                           profileMapping,
                                                           targetKey,
                                                           applied); result.failed())
            return result;
    }

    if (applied.isEmpty())
        return juce::Result::fail ("display_domain_clarification:我已经找到了这个插件抓手，但这次请求没有包含可执行的目标数值。请告诉我要把它设置到多少，最好带上你看到的显示单位，例如 50%、-6 dB 或 1200 Hz。");

    return juce::Result::ok();
}

juce::Result applyEqRuntimeControl (te::Plugin& plugin,
                                    const std::unordered_set<std::string>& validParamIds,
                                    const juce::StringArray& staleParamIds,
                                    const juce::Array<juce::var>& groups,
                                    const juce::DynamicObject& command,
                                    const juce::DynamicObject& target,
                                    const juce::String& preferredComponentId,
                                    const juce::var& safety,
                                    juce::Array<juce::var>& applied,
                                    juce::var& selectedGroup)
{
    double targetFrequencyHz = 0.0;
    const auto hasFrequency = readNumericFromTarget (command,
                                                     target,
                                                     { "freq_hz", "frequency_hz", "frequency", "freq", "center_frequency", "center_freq", "cutoff" },
                                                     targetFrequencyHz);
    if (! hasFrequency)
        return juce::Result::fail ("EQ apply control requires target.freq_hz");

    auto requestedComponentId = firstNonEmptyProperty (target, { "component_id", "group_id", "band_id" });
    if (requestedComponentId.isEmpty())
        requestedComponentId = preferredComponentId;
    selectedGroup = findBestEqBandGroup (plugin, groups, requestedComponentId, targetFrequencyHz);
    if (! selectedGroup.isObject())
        return juce::Result::fail ("runtime profile has no usable EQ band group");

    auto* group = selectedGroup.getDynamicObject();
    const auto frequencyMapping = lookupGroupParamMapping (selectedGroup, { "frequency", "freq", "freq_hz", "center_frequency", "center_freq", "cutoff" });
    const auto gainMapping = lookupGroupParamMapping (selectedGroup, { "gain", "gain_db", "level", "amount" });
    const auto qMapping = lookupGroupParamMapping (selectedGroup, { "q", "quality", "width", "bandwidth" });
    const auto enableMapping = lookupGroupParamMapping (selectedGroup, { "enable", "enabled", "active", "band_enable", "on" });
    const auto thresholdMapping = lookupGroupParamMapping (selectedGroup, { "threshold", "threshold_db" });
    const auto typeMapping = lookupGroupParamMapping (selectedGroup, { "type", "shape", "filter_type", "band_type", "response_shape" });
    const auto dynEnableMapping = lookupGroupParamMapping (selectedGroup, { "dyn_enable", "dynamic_enable", "dynamics_enabled", "dynamics_enable" });
    const auto frequencyParamId = paramIdFromProfileMapping (frequencyMapping);
    const auto gainParamId = paramIdFromProfileMapping (gainMapping);
    const auto qParamId = paramIdFromProfileMapping (qMapping);
    const auto enableParamId = paramIdFromProfileMapping (enableMapping);
    const auto thresholdParamId = paramIdFromProfileMapping (thresholdMapping);
    const auto typeParamId = paramIdFromProfileMapping (typeMapping);
    const auto dynEnableParamId = paramIdFromProfileMapping (dynEnableMapping);

    const auto requestedMode = command.getProperty ("value_mode").toString().trim();
    const auto groupId = group != nullptr ? firstNonEmptyProperty (*group, { "id", "component_id", "name", "label" }) : juce::String();

    juce::String requestedType;
    if (typeParamId.isNotEmpty() && readEnumTargetValue (command, target, "type", requestedType))
    {
        const auto typeDomain = displayDomainFromProfileMapping (typeMapping);
        if (const auto result = applyRuntimeProfileEnumParam (plugin,
                                                              validParamIds,
                                                              staleParamIds,
                                                              "type",
                                                               typeParamId,
                                                               requestedType,
                                                               typeDomain,
                                                               applied); result.failed())
            return result;
    }

    if (const auto result = applyRuntimeProfileParam (plugin,
                                                      validParamIds,
                                                      staleParamIds,
                                                      "frequency",
                                                       frequencyParamId,
                                                       targetFrequencyHz,
                                                       requestedMode,
                                                       frequencyMapping,
                                                       "freq_hz",
                                                       applied); result.failed())
        return result;

    double gainDb = 0.0;
    const auto controlName = firstNonEmptyProperty (command, { "control", "operation", "name" });
    if (! readNumericFromTarget (command, target, { "gain_db", "gain", "amount_db", "amount" }, gainDb))
        gainDb = defaultEqGainDbForAmount (controlName, target);

    if (! isBoostRuntimeControl (controlName) && gainDb > 0.0)
        gainDb = -gainDb;

    gainDb = clampGainBySafety (gainDb, safety);
    if (const auto result = applyRuntimeProfileParam (plugin,
                                                      validParamIds,
                                                      staleParamIds,
                                                      "gain",
                                                       gainParamId,
                                                       gainDb,
                                                       requestedMode,
                                                       gainMapping,
                                                       "gain_db",
                                                       applied); result.failed())
        return result;

    double q = 0.0;
    if (! readNumericFromTarget (command, target, { "q", "quality", "width", "bandwidth" }, q))
        q = defaultEqQForWidth (target);
    q = clampBySafety (q, safety, profileField::safetyMinQ, profileField::safetyMaxQ);
    if (const auto result = applyRuntimeProfileParam (plugin,
                                                      validParamIds,
                                                      staleParamIds,
                                                      "q",
                                                       qParamId,
                                                       q,
                                                       requestedMode,
                                                       qMapping,
                                                       "q",
                                                       applied); result.failed())
        return result;

    double thresholdDb = 0.0;
    if (readNumericFromTarget (command, target, { "threshold_db", "threshold" }, thresholdDb))
    {
        thresholdDb = clampBySafety (thresholdDb, safety, profileField::safetyMinThreshold, profileField::safetyMaxThreshold);
        if (const auto result = applyRuntimeProfileParam (plugin,
                                                          validParamIds,
                                                          staleParamIds,
                                                          "threshold",
                                                           thresholdParamId,
                                                           thresholdDb,
                                                           requestedMode,
                                                           thresholdMapping,
                                                           "threshold_db",
                                                           applied); result.failed())
            return result;
    }

    if (enableParamId.isNotEmpty())
    {
        double enableValue = 1.0;
        readNumericFromTarget (command, target, { "enabled", "enable", "band_enable" }, enableValue);
        if (const auto result = applyRuntimeProfileParam (plugin,
                                                          validParamIds,
                                                          staleParamIds,
                                                          "enable",
                                                           enableParamId,
                                                           enableValue,
                                                           requestedMode,
                                                           enableMapping,
                                                           "enable",
                                                           applied); result.failed())
            return result;
    }

    double dynEnableValue = 0.0;
    if (dynEnableParamId.isNotEmpty()
        && readNumericFromTarget (command, target, { "dyn_enable", "dynamic_enable", "dynamics_enabled" }, dynEnableValue))
    {
        if (const auto result = applyRuntimeProfileParam (plugin,
                                                          validParamIds,
                                                          staleParamIds,
                                                          "dyn_enable",
                                                           dynEnableParamId,
                                                           dynEnableValue,
                                                           requestedMode,
                                                           dynEnableMapping,
                                                           "dyn_enable",
                                                           applied); result.failed())
            return result;
    }

    if (applied.isEmpty())
        return juce::Result::fail ("EQ runtime control resolved no writable parameters for group: " + groupId);

    return juce::Result::ok();
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
        juce::Logger::writeToLog ("PluginRackControlService: rack serial chain no legal pins tail="
                                  + tailId.toString()
                                  + " -> "
                                  + newPlugin.getName()
                                  + " (total tail鈫抮ack pins="
                                  + juce::String (edges.size())
                                  + ")");
        return false;
    }

    if (legalEdges.size() < edges.size())
        juce::Logger::writeToLog ("PluginRackControlService: rack serial chain partial pins tail="
                                  + tailId.toString()
                                  + " -> "
                                  + newPlugin.getName()
                                  + " chaining "
                                  + juce::String (legalEdges.size())
                                  + " of "
                                  + juce::String (edges.size())
                                  + " tail鈫抮ack pins");

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
        juce::Logger::writeToLog ("PluginRackControlService: removed broken rack instance without rack type on track "
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
                juce::Logger::writeToLog ("PluginRackControlService: fallback inserted rack instance on track "
                                          + track.itemID.toString() + " rack=" + rackType->itemID.toString());
                return settleRack();
            }

            juce::Logger::writeToLog ("PluginRackControlService: fallback failed to insert rack plugin on track "
                                      + track.itemID.toString());
        }
        else
        {
            juce::Logger::writeToLog ("PluginRackControlService: fallback failed to create rack type for track "
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

juce::var pluginDescriptionToJson (const juce::PluginDescription& desc)
{
    auto entry = std::make_unique<juce::DynamicObject>();
    const auto identifier = desc.createIdentifierString();
    const auto fileOrIdentifier = desc.fileOrIdentifier.isNotEmpty() ? desc.fileOrIdentifier : identifier;

    entry->setProperty ("name", desc.name);
    entry->setProperty ("descriptive_name", desc.descriptiveName);
    entry->setProperty ("manufacturer", desc.manufacturerName);
    entry->setProperty ("format", desc.pluginFormatName);
    entry->setProperty ("category", desc.category);
    entry->setProperty ("identifier", identifier);
    entry->setProperty ("file_or_identifier", fileOrIdentifier);
    entry->setProperty ("plugin_path", fileOrIdentifier);
    entry->setProperty ("path", fileOrIdentifier);
    entry->setProperty ("uid", juce::String (desc.uniqueId));
    entry->setProperty ("deprecated_uid", juce::String (desc.deprecatedUid));
    entry->setProperty ("is_instrument", desc.isInstrument);
    return juce::var (entry.release());
}

bool pluginDescriptionMatchesQuery (const juce::PluginDescription& desc, const juce::String& query)
{
    const auto q = query.trim().toLowerCase();

    if (q.isEmpty())
        return true;

    const auto haystack = (desc.name + " "
                           + desc.descriptiveName + " "
                           + desc.manufacturerName + " "
                           + desc.category + " "
                           + desc.pluginFormatName + " "
                           + desc.fileOrIdentifier + " "
                           + desc.createIdentifierString()).toLowerCase();

    juce::StringArray tokens;
    tokens.addTokens (q, " \t\r\n", "");

    for (const auto& token : tokens)
        if (token.trim().isNotEmpty() && ! haystack.contains (token.trim()))
            return false;

    return true;
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

    juce::Logger::writeToLog ("PluginRackControlService: " + context
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

    juce::Logger::writeToLog ("PluginRackControlService: " + context + " ensure instance begin "
                              + describeExternalPluginLoadState (edit, plugin));

    plugin.setProcessingEnabled (true);
    plugin.initialiseFully();
    edit.dispatchPendingUpdatesSynchronously();
    edit.getTransport().ensureContextAllocated (true);

    const bool ready = plugin.getAudioPluginInstance() != nullptr;
    juce::Logger::writeToLog ("PluginRackControlService: " + context
                              + (ready ? " ensure instance ready " : " ensure instance failed ")
                              + describeExternalPluginLoadState (edit, plugin));
    return ready;
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

juce::File getEffectiveProjectFile (const PluginRackControlService::CurrentProjectPathGetter& getter)
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

float convertRequestedDbToPluginDb (double requestedDb)
{
    constexpr float minimumVolumeDb = -100.0f;
    const auto gain = juce::Decibels::decibelsToGain (static_cast<float> (requestedDb), minimumVolumeDb);
    return juce::Decibels::gainToDecibels (gain, minimumVolumeDb);
}


} // namespace

PluginRackControlService::PluginRackControlService (EditGetter editGetter,
                                                    SaveProjectAction saveProjectAction,
                                                    CurrentProjectPathGetter currentProjectPathGetter)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      getCurrentProjectPath (std::move (currentProjectPathGetter))
{
}
juce::String PluginRackControlService::handleSetPluginParam (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto pluginIdStr = object.getProperty ("plugin_id").toString().trim();
    const auto paramIdRaw  = object.getProperty ("param_id").toString().trim();
    const auto valueVar    = object.getProperty ("value");
    const auto normalisedValueVar = object.hasProperty ("normalized_value") ? object.getProperty ("normalized_value")
                                      : object.hasProperty ("normalised_value") ? object.getProperty ("normalised_value")
                                      : juce::var();
    const auto valueText   = [&]
    {
        for (const auto& key : { "value_text", "display_value_text", "target_text", "text" })
        {
            const auto text = object.getProperty (key).toString().trim();
            if (text.isNotEmpty())
                return text;
        }
        return juce::String();
    }();
    const auto unit        = object.getProperty ("unit").toString().trim().toLowerCase();
    const auto hasNumericValue = valueVar.isDouble() || valueVar.isInt() || valueVar.isInt64();
    const auto hasNormalisedValue = normalisedValueVar.isDouble() || normalisedValueVar.isInt() || normalisedValueVar.isInt64();

    if (pluginIdStr.isEmpty())
        return makeErrorReply ("set_plugin_param requires a non-empty plugin_id");

    if (paramIdRaw.isEmpty())
        return makeErrorReply ("set_plugin_param requires a non-empty param_id");

    if (! hasNumericValue && ! hasNormalisedValue && valueText.isEmpty())
        return makeErrorReply ("set_plugin_param requires a numeric value, normalized_value, or value_text");

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

    auto valueInterpretation = juce::String ("raw");
    float valueToApply = hasNumericValue ? static_cast<float> (static_cast<double> (valueVar))
                                         : param->getCurrentValue();
    const auto vr = param->getValueRange();

    if (hasNormalisedValue)
    {
        valueToApply = juce::jlimit (0.0f, 1.0f, static_cast<float> (static_cast<double> (normalisedValueVar)));
        valueInterpretation = "normalised";
    }
    else if (valueText.isNotEmpty())
    {
        const auto converted = param->stringToValue (valueText);
        if (! std::isfinite (converted) || converted < vr.getStart() || converted > vr.getEnd())
            return makeErrorReply ("set_plugin_param could not convert value_text for param_id: " + paramIdRaw);

        valueToApply = converted;
        valueInterpretation = "plugin_display_text";
    }

    const bool volumeAsDb = unit == "db"
                            && (paramIdRaw.equalsIgnoreCase ("volume")
                                || paramIdRaw.equalsIgnoreCase ("master volume"))
                            && valueText.isEmpty();

    if (volumeAsDb)
    {
        const auto db = convertRequestedDbToPluginDb (static_cast<double> (valueToApply));
        valueToApply = te::decibelsToVolumeFaderPosition (db);
    }

    // Godot / IPC sends pan as 0..1 normalised for Tracktion Volume+Pan [-1,+1].
    const bool panAsNormalisedUi = (! volumeAsDb)
                                   && (paramIdRaw.equalsIgnoreCase ("pan")
                                       || paramIdRaw.equalsIgnoreCase ("master pan"));

    if (hasNormalisedValue)
    {
        param->setNormalisedParameter (valueToApply, juce::sendNotification);
    }
    else if (panAsNormalisedUi)
    {
        const float clampedNorm = juce::jlimit (0.0f, 1.0f, valueToApply);
        param->setNormalisedParameter (clampedNorm, juce::sendNotification);
    }
    else
    {
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
    response->setProperty ("new_normalised_value", param->getCurrentNormalisedValue());
    response->setProperty ("actual_normalized_value", param->getCurrentNormalisedValue());
    response->setProperty ("new_value_text", param->getCurrentValueAsString());
    response->setProperty ("display_text", param->getCurrentValueAsString());
    response->setProperty ("value_interpretation", valueInterpretation);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleSetPluginParamAliases (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlAddNode (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlAddMacro (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlAddBinding (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlUpdateNode (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlSetNodeValue (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlUpdateBinding (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlRemoveBinding (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlRemoveNode (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleControlSetMacroValues (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleConnectorUpsertProfile (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleConnectorRemoveProfile (const juce::DynamicObject& object, const juce::String&) const
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



juce::String PluginRackControlService::handlePluginGrabberGetProjectProfiles (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("plugin_grabber_profiles", juce::var (VitPluginGrabberProjectProfile::snapshotProfiles (projectFile)));
    return juce::JSON::toString (juce::var (response.release()));
}


juce::String PluginRackControlService::handlePluginGrabberUpsertProjectProfile (const juce::DynamicObject& object,
                                                                                const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginID = object.getProperty ("plugin_id").toString().trim();
    if (trackID.isEmpty() || pluginID.isEmpty())
        return makeErrorReply ("plugin_grabber_upsert_project_profile requires track_id + plugin_id");

    auto* targetTrack = findTrackByID (*edit, trackID);
    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* plugin = findPluginInEdit (*edit, pluginID);
    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginID);

    if (! pluginBelongsToTrackGraph (*targetTrack, *plugin))
        return makeErrorReply ("Plugin is not on the specified track");

    auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
    if (ext == nullptr)
        return makeErrorReply ("Plugin grabber profiles are only available for external plugins");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    const auto identity = VitPluginGrabberProjectProfile::createPluginIdentity (*ext, pluginID);
    juce::var profile;
    if (const auto result = VitPluginGrabberProjectProfile::upsertProjectDefault (projectFile, object, identity, profile); result.failed())
        return makeErrorReply (result.getErrorMessage());

    // Layer 2: write to global profile store when global: true flag is set
    const auto globalFlag = object.getProperty ("global").toString().trim().toLowerCase();
    if (globalFlag == "true" || globalFlag == "1")
    {
        const auto gpKey = [&]() -> juce::String {
            if (auto* idObj = identity.getDynamicObject())
                return idObj->getProperty ("profile_key").toString().trim();
            return {};
        }();
        if (gpKey.isNotEmpty())
        {
            juce::var globalContent = profile;
            if (auto* gcObj = globalContent.getDynamicObject())
            {
                if (object.hasProperty ("class"))
                    gcObj->setProperty ("class", object.getProperty ("class"));
                if (object.hasProperty ("groups"))
                    gcObj->setProperty ("groups", object.getProperty ("groups"));
                if (object.hasProperty ("virtual_controls"))
                    gcObj->setProperty ("virtual_controls", object.getProperty ("virtual_controls"));
                if (object.hasProperty ("safety"))
                    gcObj->setProperty ("safety", object.getProperty ("safety"));
                if (object.hasProperty ("param_signature_hash"))
                    gcObj->setProperty ("param_signature_hash", object.getProperty ("param_signature_hash"));
                if (object.hasProperty ("parameter_snapshot"))
                    gcObj->setProperty ("parameter_snapshot", object.getProperty ("parameter_snapshot"));
                if (object.hasProperty ("plugin_skill"))
                    gcObj->setProperty ("plugin_skill", object.getProperty ("plugin_skill"));
                if (object.hasProperty ("plugin_skill_validator_warnings"))
                    gcObj->setProperty ("plugin_skill_validator_warnings", object.getProperty ("plugin_skill_validator_warnings"));
            }
            VitPluginGrabberGlobalProfile::writeGlobalProfile (gpKey, globalContent);
        }
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin grabber project profile saved");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_id", pluginID);
    response->setProperty ("plugin_identity", identity);
    response->setProperty ("profile", profile);
    response->setProperty ("plugin_grabber_profiles", juce::var (VitPluginGrabberProjectProfile::snapshotProfiles (projectFile)));
    return juce::JSON::toString (juce::var (response.release()));
}


juce::String PluginRackControlService::handlePluginGrabberRemoveProjectProfile (const juce::DynamicObject& object,
                                                                                const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto profileId = object.getProperty ("profile_id").toString().trim();
    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginID = object.getProperty ("plugin_id").toString().trim();

    if (profileId.isEmpty())
    {
        if (trackID.isEmpty() || pluginID.isEmpty())
            return makeErrorReply ("plugin_grabber_remove_project_profile requires profile_id or track_id + plugin_id");

        auto* targetTrack = findTrackByID (*edit, trackID);
        if (targetTrack == nullptr)
            return makeErrorReply ("Track not found for track_id: " + trackID);

        auto* plugin = findPluginInEdit (*edit, pluginID);
        if (plugin == nullptr)
            return makeErrorReply ("Plugin not found for plugin_id: " + pluginID);

        if (! pluginBelongsToTrackGraph (*targetTrack, *plugin))
            return makeErrorReply ("Plugin is not on the specified track");

        auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
        if (ext == nullptr)
            return makeErrorReply ("Plugin grabber profiles are only available for external plugins");

        const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
        const auto identity = VitPluginGrabberProjectProfile::createPluginIdentity (*ext, pluginID);

        // Also remove global profile if it exists
        if (auto* idObj = identity.getDynamicObject())
            if (const auto key = idObj->getProperty ("profile_key").toString().trim(); key.isNotEmpty())
                VitPluginGrabberGlobalProfile::removeGlobalProfile (key);

        profileId = [&]() -> juce::String {
            if (auto* idObj = identity.getDynamicObject())
                return idObj->getProperty ("profile_key").toString().trim();
            return {};
        }();

        bool removed = false;
        if (const auto result = VitPluginGrabberProjectProfile::removeProfile (projectFile, profileId, removed); result.failed())
            return makeErrorReply (result.getErrorMessage());

        if (! removed)
            return makeErrorReply ("Profile not found");

        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("message", "Plugin grabber profile removed");
        response->setProperty ("track_id", trackID);
        response->setProperty ("plugin_id", pluginID);
        response->setProperty ("profile_id", profileId);
        response->setProperty ("plugin_grabber_profiles", juce::var (VitPluginGrabberProjectProfile::snapshotProfiles (projectFile)));
        return juce::JSON::toString (juce::var (response.release()));
    }

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    // Also remove global profile
    if (const auto key = profileId.trim().toLowerCase(); key.isNotEmpty())
        VitPluginGrabberGlobalProfile::removeGlobalProfile (key);

    bool removed = false;
    if (const auto result = VitPluginGrabberProjectProfile::removeProfile (projectFile, profileId, removed); result.failed())
        return makeErrorReply (result.getErrorMessage());

    if (! removed)
        return makeErrorReply ("Profile not found");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin grabber profile removed");
    response->setProperty ("profile_id", profileId);
    response->setProperty ("plugin_grabber_profiles", juce::var (VitPluginGrabberProjectProfile::snapshotProfiles (projectFile)));
    return juce::JSON::toString (juce::var (response.release()));
}


juce::String PluginRackControlService::handlePluginGrabberApplyControl (const juce::DynamicObject& object,
                                                                        const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto pluginID = object.getProperty ("plugin_id").toString().trim();
    const auto controlName = firstNonEmptyProperty (object, { "control", "operation", "name" });
    if (trackID.isEmpty() || pluginID.isEmpty() || controlName.isEmpty())
        return makeErrorReply ("plugin_grabber_apply_control requires track_id, plugin_id, and control");

    const auto targetVar = object.getProperty ("target");
    const auto* targetObject = targetVar.getDynamicObject();
    if (targetObject == nullptr)
        targetObject = &object;

    auto* targetTrack = findTrackByID (*edit, trackID);
    if (targetTrack == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto* plugin = findPluginInEdit (*edit, pluginID);
    if (plugin == nullptr)
        return makeErrorReply ("Plugin not found for plugin_id: " + pluginID);

    if (! pluginBelongsToTrackGraph (*targetTrack, *plugin))
        return makeErrorReply ("Plugin is not on the specified track");

    auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
    if (ext == nullptr)
        return makeErrorReply ("Plugin grabber controls are only available for external plugins");

    if (! ensureExternalPluginInstanceReady (*edit, *ext, "plugin_grabber_apply_control", kPluginOpenUiReadyTimeoutMs))
        return makeErrorReply ("Plugin grabber controls require an instantiated external plugin: "
                               + describeExternalPluginLoadState (*edit, *ext));

    const auto storedTemplateRole = plugin->state.getProperty ("vit_template_role").toString().trim().toLowerCase();
    const auto templateRole = storedTemplateRole.isNotEmpty() ? storedTemplateRole
                                                              : VitPluginTemplateRegistry::inferTemplateRole (*plugin);
    auto parametersArray = VitPluginGrabber::buildParameterDescriptors (*ext, templateRole);
    const auto aliasMap = readPluginParamAliases (*plugin);
    applyAliasesToParameterDescriptors (parametersArray, aliasMap);

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    const auto pluginIdentity = VitPluginGrabberProjectProfile::createPluginIdentity (*ext, pluginID);
    auto profileMerge = VitPluginGrabberProjectProfile::applyProjectDefault (projectFile, pluginIdentity, parametersArray);
    const auto currentSignatureHash = paramSignature::computeHash (parametersArray);
    const auto storedSignatureHash = paramSignature::loadFromProfile (profileMerge.profile.isObject() ? profileMerge.profile
                                                                                                      : profileMerge.globalProfile);

    if (! hasUsableRuntimeProfile (profileMerge))
        return makeErrorReply ("plugin_grabber_apply_control requires a learned runtime profile; run get_plugin_parameters, then Learn/Generate Controls");

    if (storedSignatureHash.isNotEmpty() && storedSignatureHash != currentSignatureHash)
        return makeErrorReply ("plugin_grabber_apply_control profile signature is stale; run get_plugin_parameters and refresh the profile");

    const auto validParamIds = collectCurrentParamIds (parametersArray);
    juce::Array<juce::var> applied;
    juce::var selectedVirtualControl = findVirtualControlByName (profileMerge.virtualControls, controlName);
    juce::var selectedGroup;
    juce::String resolver = "unknown";
    juce::String componentId;

    juce::Result applyResult = juce::Result::fail ("unsupported plugin grabber control: " + controlName);

    if (selectedVirtualControl.isObject())
    {
        if (auto* controlObject = selectedVirtualControl.getDynamicObject())
        {
            resolver = firstNonEmptyProperty (*controlObject, { "resolver", "mode" });
            componentId = firstNonEmptyProperty (*controlObject, { "component_id", "component", "group_id", "band_id" });

            if (controlObject->getProperty ("params").isObject())
                applyResult = applyVirtualControlParams (*plugin,
                                                         validParamIds,
                                                         profileMerge.staleParamIds,
                                                         selectedVirtualControl,
                                                         object,
                                                         *targetObject,
                                                         profileMerge.safety,
                                                         applied);
        }
    }

    if (applyResult.failed() && isEqRuntimeControl (controlName))
    {
        resolver = resolver.isNotEmpty() ? resolver : juce::String ("choose_nearest_or_free_band");
        applyResult = applyEqRuntimeControl (*plugin,
                                             validParamIds,
                                             profileMerge.staleParamIds,
                                             profileMerge.groups,
                                             object,
                                             *targetObject,
                                             componentId,
                                             profileMerge.safety,
                                             applied,
                                             selectedGroup);
    }

    if (applyResult.failed())
    {
        const auto message = applyResult.getErrorMessage();
        const juce::String clarificationPrefix = "display_domain_clarification:";
        if (message.startsWith (clarificationPrefix))
            return makeStatusReply ("needs_clarification", message.substring (clarificationPrefix.length()).trim());

        return makeErrorReply (message);
    }

    flushPluginOrOwnerState (*plugin);
    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Plugin grabber control applied");
    response->setProperty ("track_id", trackID);
    response->setProperty ("plugin_id", pluginID);
    response->setProperty ("control", controlName);
    response->setProperty ("plugin_identity", pluginIdentity);
    response->setProperty ("profile_source", profileMerge.profileSource);
    response->setProperty ("profile_applied", profileMerge.profileApplied);
    response->setProperty ("global_profile_applied", profileMerge.globalProfileApplied);
    response->setProperty ("current_param_signature_hash", currentSignatureHash);
    if (storedSignatureHash.isNotEmpty())
        response->setProperty ("profile_param_signature_hash", storedSignatureHash);
    response->setProperty ("profile_stale_param_ids", juce::var (stringArrayToVarArray (profileMerge.staleParamIds)));
    response->setProperty ("resolution", makeControlResolutionRecord (resolver, componentId, selectedVirtualControl, selectedGroup));
    response->setProperty ("applied_parameters", juce::var (applied));
    return juce::JSON::toString (juce::var (response.release()));
}



juce::String PluginRackControlService::handleScanPlugins (const juce::DynamicObject& object, const juce::String&) const
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

        pluginsJson.add (pluginDescriptionToJson (desc));
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    juce::StringArray scannedPaths;
    for (int i = 0; i < searchPath.getNumPaths(); ++i)
        scannedPaths.add (searchPath[i].getFullPathName());
    response->setProperty ("scanned_paths", juce::var (stringArrayToVarArray (scannedPaths)));
    response->setProperty ("plugins", juce::var (pluginsJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleListPlugins (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto limit = juce::jmax (0, object.getProperty ("limit").toString().getIntValue());
    auto& knownPluginList = edit->engine.getPluginManager().knownPluginList;
    juce::Array<juce::var> pluginsJson;

    for (const auto& desc : knownPluginList.getTypes())
    {
        if (desc.pluginFormatName != "VST3")
            continue;
        pluginsJson.add (pluginDescriptionToJson (desc));
        if (limit > 0 && pluginsJson.size() >= limit)
            break;
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("source", "knownPluginList");
    response->setProperty ("plugin_count", pluginsJson.size());
    response->setProperty ("plugins", juce::var (pluginsJson));
    response->setProperty ("entries", juce::var (pluginsJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleSearchPlugins (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto query = object.getProperty ("query").toString().trim();
    const auto limit = juce::jmax (1, object.getProperty ("limit").toString().getIntValue());
    auto& knownPluginList = edit->engine.getPluginManager().knownPluginList;
    juce::Array<juce::var> pluginsJson;

    for (const auto& desc : knownPluginList.getTypes())
    {
        if (desc.pluginFormatName != "VST3")
            continue;
        if (! pluginDescriptionMatchesQuery (desc, query))
            continue;

        pluginsJson.add (pluginDescriptionToJson (desc));
        if (pluginsJson.size() >= limit)
            break;
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("source", "knownPluginList");
    response->setProperty ("query", query);
    response->setProperty ("plugin_count", pluginsJson.size());
    response->setProperty ("plugins", juce::var (pluginsJson));
    response->setProperty ("entries", juce::var (pluginsJson));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleInstantiatePlugin (const juce::DynamicObject& object, const juce::String&) const
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
            normaliseAndRegisterExternalPluginDescription (*edit, chosenDesc, targetCanon, "instantiate_plugin/cache-exact");
            haveDesc = true;
            break;
        }

        juce::File descFile (desc.fileOrIdentifier);

        if (descFile.getFullPathName().equalsIgnoreCase (targetCanon))
        {
            chosenDesc = desc;
            normaliseAndRegisterExternalPluginDescription (*edit, chosenDesc, targetCanon, "instantiate_plugin/cache-canon");
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

        normaliseAndRegisterExternalPluginDescription (*edit, chosenDesc, targetCanon, "instantiate_plugin/introspection");
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

juce::String PluginRackControlService::handleOpenPluginUI (const juce::DynamicObject& object, const juce::String&) const
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
    if (! ensureExternalPluginInstanceReady (*edit, *ext, "open_plugin_ui", kPluginOpenUiReadyTimeoutMs))
        return makeErrorReply ("Plugin instance is not ready yet: "
                               + describeExternalPluginLoadState (*edit, *ext));
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

juce::String PluginRackControlService::handleGetPluginParameters (const juce::DynamicObject& object, const juce::String&) const
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
    if (ext == nullptr)
        return makeErrorReply ("Plugin parameters are only available for external plugins");

    if (! ensureExternalPluginInstanceReady (*edit, *ext, "get_plugin_parameters", kPluginOpenUiReadyTimeoutMs))
        return makeErrorReply ("Plugin parameters are only available for instantiated external plugins: "
                               + describeExternalPluginLoadState (*edit, *ext));

    const auto storedTemplateRole = plugin->state.getProperty ("vit_template_role").toString().trim().toLowerCase();
    const auto templateRole = storedTemplateRole.isNotEmpty() ? storedTemplateRole
                                                              : VitPluginTemplateRegistry::inferTemplateRole (*plugin);
    auto parametersArray = VitPluginGrabber::buildParameterDescriptors (*ext, templateRole);
    const auto aliasMap = readPluginParamAliases (*plugin);
    applyAliasesToParameterDescriptors (parametersArray, aliasMap);
    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    const auto pluginIdentity = VitPluginGrabberProjectProfile::createPluginIdentity (*ext, pluginID);
    auto profileMerge = VitPluginGrabberProjectProfile::applyProjectDefault (projectFile, pluginIdentity, parametersArray);
    // Also load global profile info for extended fields
    const auto profileKey = [&]() -> juce::String {
        if (auto* idObj = pluginIdentity.getDynamicObject())
            return idObj->getProperty ("profile_key").toString().trim();
        return {};
    }();
    // Inline global profile load (replaces populateGlobalInfo)
    if (profileKey.isNotEmpty())
    {
        const auto gpDir = paths::getWorkspaceDirectory().getChildFile ("plugin_grabber_profiles");
        const auto gpFile = gpDir.getChildFile (profileKey.toLowerCase() + ".json");
        if (gpFile.existsAsFile())
        {
            const auto gParsed = juce::JSON::parse (gpFile.loadFileAsString());
            if (auto* gObj = gParsed.getDynamicObject())
            {
                profileMerge.globalProfileApplied = true;
                profileMerge.globalProfileSource = "global_profile";
                profileMerge.globalProfile = gParsed;
                profileMerge.pluginClass = gObj->getProperty ("class").toString().trim();
                if (auto* ga = gObj->getProperty ("groups").getArray())
                    profileMerge.groups = *ga;
                if (auto* va = gObj->getProperty ("virtual_controls").getArray())
                    profileMerge.virtualControls = *va;
                if (auto* sObj = gObj->getProperty ("safety").getDynamicObject())
                    profileMerge.safety = juce::var (sObj->clone().release());
            }
        }
    }
    appendBindingTargetPropertiesToParameterDescriptors (parametersArray, *plugin);
    const auto currentSignatureHash = paramSignature::computeHash (parametersArray);
    const auto storedSignatureHash = paramSignature::loadFromProfile (profileMerge.profile.isObject() ? profileMerge.profile
                                                                                                      : profileMerge.globalProfile);
    const auto recommendedGroups = VitPluginGrabber::buildRecommendedGroups (parametersArray);
    const auto quickControls = profileMerge.quickControlIds.isEmpty()
                                   ? VitPluginGrabber::buildQuickControls (parametersArray, templateRole)
                                   : VitPluginGrabberProjectProfile::buildQuickControlsForIds (parametersArray, profileMerge.quickControlIds);
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
    response->setProperty ("plugin_identity", pluginIdentity);
    response->setProperty ("profile_applied", profileMerge.profileApplied);
    response->setProperty ("profile_source", profileMerge.profileSource);
    response->setProperty ("profile_stale_param_ids", juce::var (stringArrayToVarArray (profileMerge.staleParamIds)));
    response->setProperty ("project_profile", profileMerge.profile);
    response->setProperty ("current_param_signature_hash", currentSignatureHash);
    if (storedSignatureHash.isNotEmpty())
        response->setProperty ("profile_param_signature_hash", storedSignatureHash);
    response->setProperty ("global_profile_applied", profileMerge.globalProfileApplied);
    response->setProperty ("global_profile_source", profileMerge.globalProfileSource);
    if (profileMerge.globalProfile.isObject())
        response->setProperty ("global_profile", profileMerge.globalProfile);
    if (profileMerge.pluginClass.isNotEmpty())
        response->setProperty ("plugin_class", profileMerge.pluginClass);
    if (profileMerge.groups.size() > 0)
        response->setProperty ("plugin_groups", juce::var (profileMerge.groups));
    if (profileMerge.virtualControls.size() > 0)
        response->setProperty ("virtual_controls", juce::var (profileMerge.virtualControls));
    if (profileMerge.safety.isObject())
        response->setProperty ("safety_limits", profileMerge.safety);
    if (auto* profileObject = profileMerge.profile.getDynamicObject())
        if (profileObject->getProperty (profileField::pluginSkill).isObject())
            response->setProperty ("plugin_skill", profileObject->getProperty (profileField::pluginSkill));
    if (! response->hasProperty ("plugin_skill"))
        if (auto* globalObject = profileMerge.globalProfile.getDynamicObject())
            if (globalObject->getProperty (profileField::pluginSkill).isObject())
                response->setProperty ("plugin_skill", globalObject->getProperty (profileField::pluginSkill));
    response->setProperty ("binding_targets", juce::var (bindingTargets));
    response->setProperty ("recommended_groups", juce::var (recommendedGroups));
    response->setProperty ("quick_controls", juce::var (quickControls));
    response->setProperty ("control_shell", VitControlShell::buildShellDescriptor (templateRole, recommendedGroups));
    response->setProperty ("capability_manifest", juce::var (capabilityTags));
    response->setProperty ("parameters", juce::var (parametersArray));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleDeletePlugin (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleMovePlugin (const juce::DynamicObject& object, const juce::String&) const
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
    juce::Logger::writeToLog ("PluginRackControlService: rejected deprecated move_plugin track="
                              + trackID + " plugin=" + pluginItemIdStr + " new_index=" + juce::String (newIndex));
    return makeErrorReply ("move_plugin deprecated in DAG rack mode");
}

juce::String PluginRackControlService::handleRackAddNode (const juce::DynamicObject& object, const juce::String&) const
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

    plugin->setProcessingEnabled (true);

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

    bool pluginInstanceReady = false;
    juce::String pluginLoadState;

    if (auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin.get()))
    {
        pluginInstanceReady = ensureExternalPluginInstanceReady (*edit,
                                                                 *ext,
                                                                 "rack_add_node",
                                                                 kPluginRackInsertReadyTimeoutMs);
        pluginLoadState = describeExternalPluginLoadState (*edit, *ext);
    }

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
            juce::Logger::writeToLog ("PluginRackControlService: rack_add_node serial auto-chain skipped for "
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
    response->setProperty ("plugin_instance_ready", pluginInstanceReady);
    response->setProperty ("plugin_load_state", pluginLoadState);
    response->setProperty ("x", x);
    response->setProperty ("y", y);
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::handleRackConnectPins (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleRackRemoveConnection (const juce::DynamicObject& object, const juce::String&) const
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

juce::String PluginRackControlService::handleRackSetNodeClipScope (const juce::DynamicObject& object, const juce::String&) const
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


juce::String PluginRackControlService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String PluginRackControlService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
