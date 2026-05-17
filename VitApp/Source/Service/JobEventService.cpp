#include "JobEventService.h"

#include "../Core/VitAIGCJobRuntime.h"
#include "../Core/VitAsyncGhostPolicy.h"
#include "../Core/VitPaths.h"

namespace vit
{

namespace
{

juce::File getEffectiveProjectFile (const JobEventService::CurrentProjectPathGetter& getter)
{
    if (getter != nullptr)
    {
        const auto currentPath = getter();
        if (currentPath.isNotEmpty())
            return juce::File (currentPath);
    }

    return paths::ensureDefaultProjectXmlFileExists();
}

} // namespace

JobEventService::JobEventService (CurrentProjectPathGetter currentProjectPathGetter)
    : getCurrentProjectPath (std::move (currentProjectPathGetter))
{
}

juce::String JobEventService::handleAigcRegisterJob (const juce::DynamicObject& object, const juce::String&) const
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

juce::String JobEventService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String JobEventService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
