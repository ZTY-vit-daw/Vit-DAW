#include "VitGraphTrace.h"

#include "VitAIGCJobRuntime.h"
#include "VitGraphSwapCoordinator.h"
#include "VitMediaPoolManager.h"
#include "VitNodeProfiler.h"
#include "VitProbeNode.h"
#include "VitTakeHistoryStack.h"

namespace vit
{

namespace
{

bool isPendingJobState (const juce::String& state)
{
    const auto lowered = state.trim().toLowerCase();
    return lowered == "pending" || lowered == "queued" || lowered == "running" || lowered == "processing";
}

} // namespace

juce::var VitGraphTrace::createObservabilitySnapshot (const juce::File& projectFile, te::Edit& edit)
{
    const auto jobs = VitAIGCJobRuntime::snapshotProjectJobs (projectFile);
    const auto assets = VitMediaPoolManager::snapshotProjectAssets (projectFile);
    const auto takeHistories = VitTakeHistoryStack::snapshotProjectStacks (projectFile);
    const auto graphSnapshot = VitGraphSwapCoordinator::getSnapshot (edit);

    int pendingJobs = 0;
    int cacheAssets = 0;
    int activeTakeCount = 0;

    for (const auto& job : jobs)
        if (auto* object = job.getDynamicObject())
            if (isPendingJobState (object->getProperty ("job_state").toString()))
                ++pendingJobs;

    for (const auto& asset : assets)
        if (auto* object = asset.getDynamicObject())
            if (object->getProperty ("bucket").toString() == "AIGC_Cache")
                ++cacheAssets;

    for (const auto& stack : takeHistories)
        if (auto* object = stack.getDynamicObject())
            if (object->getProperty ("active_take_id").toString().isNotEmpty())
                ++activeTakeCount;

    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("probe_channels", juce::var (VitProbeNode::createBuiltInProbeChannels()));
    object->setProperty ("graph_revision", graphSnapshot.revision);
    object->setProperty ("graph_lifecycle_state", graphSnapshot.lifecycleState);
    object->setProperty ("pending_job_count", pendingJobs);
    object->setProperty ("active_take_count", activeTakeCount);
    object->setProperty ("cache_asset_count", cacheAssets);
    object->setProperty ("output_source_hint", cacheAssets > 0 ? "generated_assets" : "project_media");
    object->setProperty ("profile", VitNodeProfiler::createProjectProfile (edit, jobs, assets, takeHistories));
    return juce::var (object.release());
}

} // namespace vit
