#include "VitProjectHealthCheck.h"

#include "VitAIGCJobRuntime.h"
#include "VitConnectorPluginSpec.h"
#include "VitMediaPoolManager.h"

namespace vit
{

namespace
{

juce::var makeIssue (const juce::String& code,
                     const juce::String& severity,
                     const juce::String& message,
                     const juce::String& entityId = {})
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("code", code);
    object->setProperty ("severity", severity);
    object->setProperty ("message", message);
    object->setProperty ("entity_id", entityId);
    return juce::var (object.release());
}

bool isPendingJobState (const juce::String& state)
{
    const auto lowered = state.trim().toLowerCase();
    return lowered == "pending" || lowered == "queued" || lowered == "running" || lowered == "processing";
}

bool hasBuiltInSpecId (const juce::String& specId)
{
    for (const auto& spec : VitConnectorPluginSpec::createBuiltInSpecs())
        if (auto* object = spec.getDynamicObject())
            if (object->getProperty ("spec_id").toString() == specId)
                return true;

    return false;
}

} // namespace

juce::var VitProjectHealthCheck::createReport (const juce::File& projectFile, te::Edit& edit)
{
    juce::ignoreUnused (edit);

    const auto assets = VitMediaPoolManager::snapshotProjectAssets (projectFile);
    const auto jobs = VitAIGCJobRuntime::snapshotProjectJobs (projectFile);
    const auto profiles = VitConnectorPluginSpec::snapshotProfiles (projectFile);
    juce::Array<juce::var> issues;

    for (const auto& asset : assets)
        if (auto* object = asset.getDynamicObject())
        {
            const auto assetState = object->getProperty ("asset_state").toString();
            if (assetState == "missing")
                issues.add (makeIssue ("missing_asset",
                                       "error",
                                       "Generated asset file is missing",
                                       object->getProperty ("asset_ref").toString()));

            const auto warpState = object->getProperty ("warp_state").toString();
            if (warpState.isNotEmpty() && warpState != "aligned" && warpState != "bypassed")
                issues.add (makeIssue ("warp_unaligned",
                                       "warning",
                                       "Generated asset warp state is not aligned",
                                       object->getProperty ("asset_ref").toString()));
        }

    for (const auto& job : jobs)
        if (auto* object = job.getDynamicObject())
            if (isPendingJobState (object->getProperty ("job_state").toString()))
                issues.add (makeIssue ("pending_job",
                                       "info",
                                       "AIGC job is still pending",
                                       object->getProperty ("job_id").toString()));

    for (const auto& profile : profiles)
        if (auto* object = profile.getDynamicObject())
        {
            const auto specId = object->getProperty ("spec_id").toString();
            if (! hasBuiltInSpecId (specId))
                issues.add (makeIssue ("invalid_connector_profile",
                                       "warning",
                                       "Connector profile references unknown spec",
                                       object->getProperty ("profile_id").toString()));
        }

    auto report = std::make_unique<juce::DynamicObject>();
    report->setProperty ("issue_count", issues.size());
    report->setProperty ("issues", juce::var (issues));
    return juce::var (report.release());
}

juce::var VitProjectHealthCheck::createExportPolicy (const juce::File& projectFile, te::Edit& edit)
{
    juce::ignoreUnused (edit);

    const auto jobs = VitAIGCJobRuntime::snapshotProjectJobs (projectFile);
    const auto assets = VitMediaPoolManager::snapshotProjectAssets (projectFile);
    bool hasPendingJobs = false;
    bool hasMissingAssets = false;
    bool hasCacheAssets = false;

    for (const auto& job : jobs)
        if (auto* object = job.getDynamicObject())
            if (isPendingJobState (object->getProperty ("job_state").toString()))
                hasPendingJobs = true;

    for (const auto& asset : assets)
        if (auto* object = asset.getDynamicObject())
        {
            if (object->getProperty ("asset_state").toString() == "missing")
                hasMissingAssets = true;
            if (object->getProperty ("bucket").toString() == "AIGC_Cache")
                hasCacheAssets = true;
        }

    auto object = std::make_unique<juce::DynamicObject>();
    juce::String recommended = "wait";

    if (hasMissingAssets)
        recommended = "fail_export";
    else if (hasPendingJobs && hasCacheAssets)
        recommended = "use_cache";
    else if (hasPendingJobs)
        recommended = "bypass";

    object->setProperty ("recommended_mode", recommended);
    object->setProperty ("supported_modes", juce::var (juce::Array<juce::var> { "wait", "use_cache", "bypass", "fail_export" }));
    object->setProperty ("has_pending_jobs", hasPendingJobs);
    object->setProperty ("has_missing_assets", hasMissingAssets);
    object->setProperty ("has_cache_assets", hasCacheAssets);
    return juce::var (object.release());
}

} // namespace vit
