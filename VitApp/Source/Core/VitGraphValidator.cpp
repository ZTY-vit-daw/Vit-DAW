#include "VitGraphValidator.h"

namespace vit
{

namespace
{

int zoneRank (const juce::String& zoneId)
{
    if (zoneId == "Z1")
        return 1;

    if (zoneId == "Z2")
        return 2;

    if (zoneId == "Z3")
        return 3;

    return 0;
}

} // namespace

juce::String VitGraphValidator::normaliseZoneId (const juce::String& zoneId)
{
    return zoneId.trim().toUpperCase();
}

bool VitGraphValidator::isZoneIdAllowedForNodeCreation (const juce::String& zoneId)
{
    const auto normalised = normaliseZoneId (zoneId);
    return normalised == "Z1" || normalised == "Z2" || normalised == "Z3";
}

juce::String VitGraphValidator::getZoneIdForNode (te::RackType& rackType, te::EditItemID nodeId, te::Plugin* fallbackPlugin)
{
    if (nodeId.isValid())
        for (auto child : rackType.state)
            if (child.hasType (te::IDs::PLUGININSTANCE))
                if (auto pluginState = child.getChildWithName (te::IDs::PLUGIN); pluginState.isValid())
                    if (te::EditItemID::fromID (pluginState) == nodeId)
                    {
                        const auto zoneId = normaliseZoneId (child.getProperty ("vit_zone_id").toString());

                        if (zoneId.isNotEmpty())
                            return zoneId;

                        break;
                    }

    if (fallbackPlugin != nullptr && fallbackPlugin->isSynth())
        return "Z2";

    return "Z3";
}

VitGraphValidationResult VitGraphValidator::validateConnection (te::RackType& rackType,
                                                                te::EditItemID sourceId,
                                                                te::EditItemID destId)
{
    VitGraphValidationResult result;
    auto* sourcePlugin = rackType.getPluginForID (sourceId);
    auto* destPlugin = rackType.getPluginForID (destId);

    if (sourcePlugin == nullptr || destPlugin == nullptr)
    {
        result.errorMessage = "Both source_id and dest_id must resolve to rack nodes";
        return result;
    }

    result.sourceZoneId = getZoneIdForNode (rackType, sourceId, sourcePlugin);
    result.destZoneId = getZoneIdForNode (rackType, destId, destPlugin);

    if (sourceId == destId)
    {
        result.errorMessage = "A rack node cannot connect to itself";
        return result;
    }

    if (result.sourceZoneId == "TOP" || result.destZoneId == "TOP")
    {
        result.errorMessage = "Top is mapping-only and cannot participate in rack execution edges";
        return result;
    }

    const auto sourceRank = zoneRank (result.sourceZoneId);
    const auto destRank = zoneRank (result.destZoneId);

    if (sourceRank > 0 && destRank > 0 && sourceRank > destRank)
    {
        result.errorMessage = "Reverse zone connection is forbidden: "
                              + result.sourceZoneId + " -> " + result.destZoneId;
        return result;
    }

    result.allowed = true;
    return result;
}

} // namespace vit
