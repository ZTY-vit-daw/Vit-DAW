#include "VitDagChecker.h"

#include <unordered_set>
#include <vector>

namespace vit
{

bool VitDagChecker::wouldCreateCycle (const te::RackType& rackType, te::EditItemID sourceId, te::EditItemID destId)
{
    if (! sourceId.isValid() || ! destId.isValid())
        return false;

    if (sourceId == destId)
        return true;

    std::vector<te::EditItemID> stack;
    std::unordered_set<juce::String> visited;
    stack.push_back (destId);

    while (! stack.empty())
    {
        const auto current = stack.back();
        stack.pop_back();

        const auto currentKey = current.toString();

        if (! visited.insert (currentKey.toStdString()).second)
            continue;

        if (current == sourceId)
            return true;

        for (auto* connection : rackType.getConnections())
        {
            if (connection == nullptr)
                continue;

            const auto nextSource = connection->sourceID.get();
            const auto nextDest = connection->destID.get();

            if (! nextSource.isValid() || ! nextDest.isValid())
                continue;

            if (nextSource == current)
                stack.push_back (nextDest);
        }
    }

    return false;
}

} // namespace vit
