#include "VitTakeHistoryStack.h"

#include <unordered_map>
#include <vector>

namespace vit
{

namespace
{

struct StoredTakeStack
{
    VitTakeStackRecord info;
    std::vector<VitTakeRecord> takes;
};

juce::CriticalSection& getTakeLock()
{
    static juce::CriticalSection lock;
    return lock;
}

std::unordered_map<std::string, std::unordered_map<std::string, StoredTakeStack>>& getTakeStores()
{
    static std::unordered_map<std::string, std::unordered_map<std::string, StoredTakeStack>> stores;
    return stores;
}

std::string makeProjectKey (const juce::File& projectFile)
{
    return projectFile.getFullPathName().toStdString();
}

juce::var takeToVar (const VitTakeRecord& take)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("take_id", take.takeId);
    object->setProperty ("asset_ref", take.assetRef);
    object->setProperty ("absolute_path", take.absolutePath);
    object->setProperty ("clip_id", take.clipId);
    object->setProperty ("track_id", take.trackId);
    object->setProperty ("job_id", take.jobId);
    object->setProperty ("label", take.label);
    object->setProperty ("warp_state", take.warpState);
    return juce::var (object.release());
}

juce::var stackToVar (const StoredTakeStack& stack)
{
    auto object = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> takes;

    for (const auto& take : stack.takes)
        takes.add (takeToVar (take));

    object->setProperty ("take_stack_id", stack.info.stackId);
    object->setProperty ("active_take_id", stack.info.activeTakeId);
    object->setProperty ("clip_id", stack.info.clipId);
    object->setProperty ("track_id", stack.info.trackId);
    object->setProperty ("node_id", stack.info.nodeId);
    object->setProperty ("takes", juce::var (takes));
    return juce::var (object.release());
}

} // namespace

juce::String VitTakeHistoryStack::resolveStackId (const juce::DynamicObject& object,
                                                  const juce::String& nodeIdFallback,
                                                  const juce::String& clipIdFallback)
{
    auto stackId = object.getProperty ("take_stack_id").toString().trim();

    if (stackId.isNotEmpty())
        return stackId;

    if (nodeIdFallback.isNotEmpty())
        return "take_stack:node:" + nodeIdFallback;

    if (clipIdFallback.isNotEmpty())
        return "take_stack:clip:" + clipIdFallback;

    return "take_stack:" + juce::Uuid().toString();
}

juce::String VitTakeHistoryStack::addTake (const juce::File& projectFile,
                                           const VitTakeStackRecord& stack,
                                           const VitTakeRecord& take,
                                           bool makeActive)
{
    const juce::ScopedLock sl (getTakeLock());
    auto& projectStore = getTakeStores()[makeProjectKey (projectFile)];
    auto& stored = projectStore[stack.stackId.toStdString()];

    if (stored.info.stackId.isEmpty())
        stored.info = stack;

    stored.info.clipId = stack.clipId;
    stored.info.trackId = stack.trackId;
    stored.info.nodeId = stack.nodeId;

    for (auto& existing : stored.takes)
        if (existing.takeId == take.takeId)
        {
            existing = take;
            if (makeActive)
                stored.info.activeTakeId = take.takeId;
            return take.takeId;
        }

    stored.takes.push_back (take);

    if (makeActive || stored.info.activeTakeId.isEmpty())
        stored.info.activeTakeId = take.takeId;

    return take.takeId;
}

bool VitTakeHistoryStack::setActiveTake (const juce::File& projectFile,
                                         const juce::String& stackId,
                                         const juce::String& takeId)
{
    const juce::ScopedLock sl (getTakeLock());
    auto projectIt = getTakeStores().find (makeProjectKey (projectFile));

    if (projectIt == getTakeStores().end())
        return false;

    auto it = projectIt->second.find (stackId.toStdString());
    if (it == projectIt->second.end())
        return false;

    for (const auto& take : it->second.takes)
        if (take.takeId == takeId)
        {
            it->second.info.activeTakeId = takeId;
            return true;
        }

    return false;
}

bool VitTakeHistoryStack::getTake (const juce::File& projectFile,
                                   const juce::String& stackId,
                                   const juce::String& takeId,
                                   VitTakeRecord& outTake)
{
    const juce::ScopedLock sl (getTakeLock());
    auto projectIt = getTakeStores().find (makeProjectKey (projectFile));

    if (projectIt == getTakeStores().end())
        return false;

    auto it = projectIt->second.find (stackId.toStdString());
    if (it == projectIt->second.end())
        return false;

    for (const auto& take : it->second.takes)
        if (take.takeId == takeId)
        {
            outTake = take;
            return true;
        }

    return false;
}

juce::Array<juce::var> VitTakeHistoryStack::snapshotProjectStacks (const juce::File& projectFile)
{
    juce::Array<juce::var> stacks;
    const juce::ScopedLock sl (getTakeLock());

    if (const auto it = getTakeStores().find (makeProjectKey (projectFile)); it != getTakeStores().end())
        for (const auto& [stackId, stack] : it->second)
        {
            juce::ignoreUnused (stackId);
            stacks.add (stackToVar (stack));
        }

    return stacks;
}

} // namespace vit
