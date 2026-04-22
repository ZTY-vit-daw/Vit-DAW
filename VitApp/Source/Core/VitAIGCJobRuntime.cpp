#include "VitAIGCJobRuntime.h"

#include <unordered_map>

namespace vit
{

namespace
{

struct StoredJob
{
    VitAIGCJobRecord record;
    int64_t updatedAtMs = 0;
};

juce::CriticalSection& getJobLock()
{
    static juce::CriticalSection lock;
    return lock;
}

std::unordered_map<std::string, std::unordered_map<std::string, StoredJob>>& getJobStores()
{
    static std::unordered_map<std::string, std::unordered_map<std::string, StoredJob>> stores;
    return stores;
}

std::string makeProjectKey (const juce::File& projectFile)
{
    return projectFile.getFullPathName().toStdString();
}

juce::var jobToVar (const StoredJob& job)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("job_id", job.record.jobId);
    object->setProperty ("node_id", job.record.nodeId);
    object->setProperty ("job_state", job.record.jobState);
    object->setProperty ("ghost_state", job.record.ghostState);
    object->setProperty ("status_message", job.record.statusMessage);
    object->setProperty ("request_hash", job.record.requestHash);
    object->setProperty ("source", job.record.source);
    object->setProperty ("asset_ref", job.record.assetRef);
    object->setProperty ("track_id", job.record.trackId);
    object->setProperty ("clip_id", job.record.clipId);
    object->setProperty ("updated_at_ms", job.updatedAtMs);
    return juce::var (object.release());
}

} // namespace

void VitAIGCJobRuntime::upsertJob (const juce::File& projectFile, const VitAIGCJobRecord& record)
{
    if (record.jobId.isEmpty())
        return;

    const juce::ScopedLock sl (getJobLock());
    auto& store = getJobStores()[makeProjectKey (projectFile)];
    auto& job = store[record.jobId.toStdString()];
    job.record = record;
    job.updatedAtMs = juce::Time::currentTimeMillis();
}

void VitAIGCJobRuntime::attachImportedAsset (const juce::File& projectFile,
                                             const juce::String& jobId,
                                             const juce::String& assetRef,
                                             const juce::String& trackId,
                                             const juce::String& clipId)
{
    if (jobId.isEmpty())
        return;

    const juce::ScopedLock sl (getJobLock());
    auto& store = getJobStores()[makeProjectKey (projectFile)];
    auto& job = store[jobId.toStdString()];
    job.record.jobId = jobId;
    job.record.assetRef = assetRef;
    job.record.trackId = trackId;
    job.record.clipId = clipId;
    job.record.jobState = "ready";
    job.updatedAtMs = juce::Time::currentTimeMillis();
}

void VitAIGCJobRuntime::setGhostState (const juce::File& projectFile,
                                       const juce::String& jobId,
                                       const juce::String& ghostState)
{
    if (jobId.isEmpty())
        return;

    const juce::ScopedLock sl (getJobLock());
    auto& store = getJobStores()[makeProjectKey (projectFile)];
    auto& job = store[jobId.toStdString()];
    job.record.jobId = jobId;
    job.record.ghostState = ghostState;
    job.updatedAtMs = juce::Time::currentTimeMillis();
}

juce::Array<juce::var> VitAIGCJobRuntime::snapshotProjectJobs (const juce::File& projectFile)
{
    juce::Array<juce::var> jobs;
    const juce::ScopedLock sl (getJobLock());

    if (const auto it = getJobStores().find (makeProjectKey (projectFile)); it != getJobStores().end())
        for (const auto& [jobId, job] : it->second)
        {
            juce::ignoreUnused (jobId);
            jobs.add (jobToVar (job));
        }

    return jobs;
}

} // namespace vit
