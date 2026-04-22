#include "VitParallelMergePlanner.h"

#include <unordered_set>

namespace vit
{

namespace
{

bool isLikelyExplicitMergeNode (te::Plugin* plugin)
{
    if (plugin == nullptr)
        return false;

    const auto label = (plugin->getName() + " " + plugin->getPluginType()).toLowerCase();

    return label.contains ("merge")
        || label.contains ("sum")
        || label.contains ("mix")
        || label.contains ("bus")
        || label.contains ("aux")
        || label.contains ("return");
}

} // namespace

VitParallelMergeAdvice VitParallelMergePlanner::analyseDestination (const te::RackType& rackType,
                                                                    te::EditItemID destId,
                                                                    te::Plugin* destPlugin,
                                                                    te::EditItemID pendingSourceId)
{
    VitParallelMergeAdvice advice;

    if (! destId.isValid())
        return advice;

    std::unordered_set<std::string> uniqueSources;

    for (auto* connection : rackType.getConnections())
    {
        if (connection == nullptr || connection->destID.get() != destId)
            continue;

        uniqueSources.insert (connection->sourceID.get().toString().toStdString());
    }

    if (pendingSourceId.isValid())
        uniqueSources.insert (pendingSourceId.toString().toStdString());

    advice.incomingBranchCount = static_cast<int> (uniqueSources.size());

    if (advice.incomingBranchCount <= 1 || isLikelyExplicitMergeNode (destPlugin))
        return advice;

    advice.requiresAttention = true;
    advice.summary = "Multiple upstream branches converge on this node. Consider inserting an explicit Merge/Sum/Bus node to make gain and latency intent obvious.";
    return advice;
}

} // namespace vit
