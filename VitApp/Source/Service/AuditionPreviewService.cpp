#include "AuditionPreviewService.h"

#include <cmath>
#include <utility>

namespace vit
{

namespace
{

juce::var candidateToVar (const audition::Candidate& candidate)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("id", juce::String (candidate.id.c_str()));
    object->setProperty ("label", juce::String (candidate.label.c_str()));
    object->setProperty ("source_kind", juce::String (candidate.sourceKind.c_str()));
    object->setProperty ("source_ref", juce::String (candidate.sourceRef.c_str()));
    object->setProperty ("preview_ref", juce::String (candidate.previewRef.c_str()));
    object->setProperty ("checkpoint_ref", juce::String (candidate.checkpointRef.c_str()));
    object->setProperty ("commit_id", juce::String (candidate.commitId.c_str()));
    object->setProperty ("branch_ref", juce::String (candidate.branchRef.c_str()));
    object->setProperty ("worktree_ref", juce::String (candidate.worktreeRef.c_str()));
    object->setProperty ("project_uuid", juce::String (candidate.projectUuid.c_str()));
    object->setProperty ("project_revision", juce::String (candidate.projectRevision.c_str()));
    object->setProperty ("render_revision", juce::String (candidate.renderRevision.c_str()));
    object->setProperty ("preview_revision", juce::String (candidate.previewRevision.c_str()));
    object->setProperty ("scope", juce::String (candidate.scope.c_str()));
    object->setProperty ("duration_seconds", candidate.durationSeconds);
    object->setProperty ("sample_rate", candidate.sampleRate);
    object->setProperty ("channel_count", candidate.channelCount);
    object->setProperty ("status", audition::toString (candidate.status));
    return juce::var (object.release());
}

bool appendCandidate (const juce::var& value,
                      std::vector<audition::Candidate>& output,
                      const std::string& sessionScope,
                      const std::string& activeProjectRef,
                      const std::string& activeProjectRevision,
                      juce::String& error)
{
    const auto* candidateObject = value.getDynamicObject();
    if (candidateObject == nullptr)
    {
        error = "audition.prepare candidate must be an object";
        return false;
    }

    audition::Candidate candidate;
    candidate.id = candidateObject->getProperty ("id").toString().trim().toStdString();
    candidate.label = candidateObject->getProperty ("label").toString().trim().toStdString();
    candidate.sourceKind = candidateObject->getProperty ("source_kind").toString().trim().toStdString();
    candidate.sourceRef = candidateObject->getProperty ("source_ref").toString().trim().toStdString();
    candidate.checkpointRef = candidateObject->getProperty ("checkpoint_ref").toString().trim().toStdString();
    candidate.commitId = candidateObject->getProperty ("commit_id").toString().trim().toStdString();
    candidate.branchRef = candidateObject->getProperty ("branch_ref").toString().trim().toStdString();
    candidate.worktreeRef = candidateObject->getProperty ("worktree_ref").toString().trim().toStdString();
    candidate.projectUuid = candidateObject->getProperty ("project_uuid").toString().trim().toStdString();
    candidate.projectRevision = candidateObject->getProperty ("project_revision").toString().trim().toStdString();
    candidate.renderRevision = candidateObject->getProperty ("render_revision").toString().trim().toStdString();
    candidate.previewRevision = candidateObject->getProperty ("preview_revision").toString().trim().toStdString();
    candidate.scope = candidateObject->getProperty ("scope").toString().trim().toStdString();
    if (candidate.scope.empty())
        candidate.scope = sessionScope;
    if (candidate.projectRevision.empty())
        candidate.projectRevision = activeProjectRevision;
    if (candidate.checkpointRef.empty() && candidate.sourceKind == "checkpoint")
        candidate.checkpointRef = candidate.sourceRef;
    if (candidate.commitId.empty() && candidate.sourceKind == "experiment")
        candidate.commitId = candidate.sourceRef;
    if (candidate.branchRef.empty() && candidate.worktreeRef.empty())
        candidate.branchRef = activeProjectRef;
    if (candidate.sourceKind.empty() || candidate.sourceRef.empty())
    {
        error = "candidate requires source_kind and source_ref";
        return false;
    }
    if (candidate.checkpointRef.empty() && candidate.commitId.empty())
    {
        error = "candidate requires checkpoint_ref or commit_id";
        return false;
    }
    if (candidate.branchRef.empty() && candidate.worktreeRef.empty())
    {
        error = "candidate requires branch_ref or worktree_ref";
        return false;
    }
    if (candidate.projectRevision.empty())
    {
        error = "candidate requires project_revision";
        return false;
    }
    if (candidate.scope != sessionScope)
    {
        error = "candidate scope must match audition session scope";
        return false;
    }
    if (candidate.renderRevision.empty())
        candidate.renderRevision = "direct-audio-file";
    // Client-provided preview refs do not establish readiness. The Kernel audio
    // plane replaces this after decode, resample, and warm-up have succeeded.
    candidate.previewRef.clear();
    output.push_back (std::move (candidate));
    return true;
}

bool readFiniteDouble (const juce::var& value, double& result)
{
    if (! value.isDouble() && ! value.isInt() && ! value.isInt64())
        return false;

    result = static_cast<double> (value);
    return std::isfinite (result);
}

} // namespace

AuditionPreviewService::AuditionPreviewService (te::Engine* engine,
                                                EditGetter editGetter,
                                                PublishAction publishAction)
    : getEdit (std::move (editGetter)),
      audioPlane (engine, getEdit),
      publish (std::move (publishAction))
{
}

juce::String AuditionPreviewService::handlePrepare (const juce::DynamicObject& object,
                                                    const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);

    audition::Session session;
    juce::String error;
    session.id = requiredString (object, "session_id", "audition.prepare", error).toStdString();
    if (error.isNotEmpty())
        return errorReply ("audition.prepare", "validation_error", error);

    session.conversationId = object.getProperty ("conversation_id").toString().trim().toStdString();
    session.scope = requiredString (object, "scope", "audition.prepare", error).toStdString();
    if (error.isNotEmpty())
        return errorReply ("audition.prepare", "validation_error", error);

    session.activeProjectRef = requiredString (object, "active_project_ref", "audition.prepare", error).toStdString();
    if (error.isNotEmpty())
        return errorReply ("audition.prepare", "validation_error", error);

    session.activeProjectRevision = requiredString (object, "active_project_revision", "audition.prepare", error).toStdString();
    if (error.isNotEmpty())
        return errorReply ("audition.prepare", "validation_error", error);

    session.transport.timelineRevision = requiredString (object, "timeline_revision", "audition.prepare", error).toStdString();
    if (error.isNotEmpty())
        return errorReply ("audition.prepare", "validation_error", error);

    const auto candidateA = object.getProperty ("candidate_a");
    const auto candidateB = object.getProperty ("candidate_b");
    if (! candidateA.isVoid() || ! candidateB.isVoid())
    {
        if (candidateA.isVoid() || candidateB.isVoid())
            return errorReply ("audition.prepare", "validation_error", "audition.prepare requires candidate_a and candidate_b");

        if (! appendCandidate (candidateA, session.candidates, session.scope, session.activeProjectRef, session.activeProjectRevision, error)
            || ! appendCandidate (candidateB, session.candidates, session.scope, session.activeProjectRef, session.activeProjectRevision, error))
            return errorReply ("audition.prepare", "validation_error", error);
    }
    else
    {
        const auto* candidates = object.getProperty ("candidates").getArray();
        if (candidates == nullptr)
            return errorReply ("audition.prepare", "validation_error", "audition.prepare requires candidate_a and candidate_b");

        for (const auto& candidate : *candidates)
            if (! appendCandidate (candidate, session.candidates, session.scope, session.activeProjectRef, session.activeProjectRevision, error))
                return errorReply ("audition.prepare", "validation_error", error);
    }

    captureAuthoritativeTransport (session);
    auto result = state.prepare (session);
    if (! result.ok || ! result.session.has_value())
        return resultToReply ("audition.prepare", result);

    publishStateEvent (result, "audition.prepare.started");
    const auto audioResult = audioPlane.prepare (*result.session);
    if (! audioResult.ok)
    {
        auto failedState = state.markFailed (session.id);
        audition::Result failure { false, audioResult.code, audioResult.message, failedState.session };
        publishStateEvent (failure, "audition.failed");
        return resultToReply ("audition.prepare", failure);
    }

    for (const auto& prepared : audioResult.candidates)
    {
        state.setCandidateAudioMetadata (session.id, prepared.id, prepared.previewRevision,
                                         prepared.durationSeconds, prepared.sampleRate, prepared.channelCount);
        result = state.markCandidateReady (session.id, prepared.id, prepared.previewRef);
        publishStateEvent (result, result.message.c_str());
    }

    return resultToReply ("audition.prepare", result);
}

juce::String AuditionPreviewService::handleStatus (const juce::DynamicObject& object,
                                                   const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.status", error);
    if (error.isNotEmpty())
        return errorReply ("audition.status", "validation_error", error);

    return resultToReply ("audition.status", state.status (sessionId.toStdString()));
}

juce::String AuditionPreviewService::handleReady (const juce::DynamicObject& object,
                                                  const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.ready", error);
    if (error.isNotEmpty())
        return errorReply ("audition.ready", "validation_error", error);
    const auto candidateId = requiredString (object, "candidate_id", "audition.ready", error);
    if (error.isNotEmpty())
        return errorReply ("audition.ready", "validation_error", error);

    if (! audioPlane.isCandidatePrepared (sessionId.toStdString(), candidateId.toStdString()))
        return errorReply ("audition.ready", "candidate_not_prepared", "Kernel audio source has not been prepared");

    const auto result = state.markCandidateReady (sessionId.toStdString(), candidateId.toStdString(),
                                                  audioPlane.preparedPreviewRef (sessionId.toStdString(), candidateId.toStdString()));
    publishStateEvent (result, "audition.ready");
    return resultToReply ("audition.ready", result);
}

juce::String AuditionPreviewService::handleStale (const juce::DynamicObject& object,
                                                  const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.stale", error);
    if (error.isNotEmpty())
        return errorReply ("audition.stale", "validation_error", error);
    audioPlane.invalidate (sessionId.toStdString());
    const auto result = state.markStale (sessionId.toStdString());
    publishStateEvent (result, "audition.stale");
    return resultToReply ("audition.stale", result);
}

juce::String AuditionPreviewService::handleFailed (const juce::DynamicObject& object,
                                                   const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.failed", error);
    if (error.isNotEmpty())
        return errorReply ("audition.failed", "validation_error", error);
    audioPlane.invalidate (sessionId.toStdString());
    const auto result = state.markFailed (sessionId.toStdString());
    publishStateEvent (result, "audition.failed");
    return resultToReply ("audition.failed", result);
}

juce::String AuditionPreviewService::handleSelect (const juce::DynamicObject& object,
                                                   const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.select", error);
    if (error.isNotEmpty())
        return errorReply ("audition.select", "validation_error", error);
    const auto candidateId = requiredString (object, "candidate_id", "audition.select", error);
    if (error.isNotEmpty())
        return errorReply ("audition.select", "validation_error", error);

    if (! audioPlane.isCandidatePrepared (sessionId.toStdString(), candidateId.toStdString()))
        return errorReply ("audition.select", "candidate_not_prepared", "audition.select requires a prepared Kernel audio source");

    auto result = state.select (sessionId.toStdString(), candidateId.toStdString());
    if (! result.ok || ! result.session.has_value())
        return resultToReply ("audition.select", result);

    auto liveSession = *result.session;
    captureAuthoritativeTransport (liveSession);
    auto positioned = state.position (liveSession.id, liveSession.transport.positionSeconds, liveSession.transport.isPlaying);
    if (! positioned.ok || ! positioned.session.has_value())
        return resultToReply ("audition.select", positioned);

    const auto& session = *positioned.session;
    if (! audioPlane.select (session.id, candidateId.toStdString(),
                             session.transport.positionSeconds, session.transport.isPlaying))
        return errorReply ("audition.select", "audio_source_switch_failed", "Kernel could not select the prepared audio source");

    result.session = positioned.session;
    result.message = "audition.select.changed";
    publishStateEvent (result, "audition.select.changed");
    return resultToReply ("audition.select", result);
}

juce::String AuditionPreviewService::handlePosition (const juce::DynamicObject& object,
                                                      const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.position", error);
    if (error.isNotEmpty())
        return errorReply ("audition.position", "validation_error", error);

    std::optional<double> positionSeconds;
    if (object.hasProperty ("position_seconds"))
    {
        double value = 0.0;
        if (! readFiniteDouble (object.getProperty ("position_seconds"), value) || value < 0.0)
            return errorReply ("audition.position", "validation_error", "position_seconds must be finite and non-negative");
        positionSeconds = value;
    }

    std::optional<bool> isPlaying;
    if (object.hasProperty ("is_playing"))
    {
        const auto value = object.getProperty ("is_playing");
        if (! value.isBool())
            return errorReply ("audition.position", "validation_error", "is_playing must be boolean");
        isPlaying = static_cast<bool> (value);
    }

    if (! positionSeconds.has_value() && ! isPlaying.has_value())
        return errorReply ("audition.position", "validation_error", "audition.position requires position_seconds or is_playing");

    const auto result = state.position (sessionId.toStdString(), positionSeconds, isPlaying);
    if (result.ok && ! audioPlane.position (sessionId.toStdString(), positionSeconds, isPlaying))
        return errorReply ("audition.position", "audio_session_not_found", "Kernel audio session is unavailable");
    if (result.ok)
        publishStateEvent (result, "audition.position.changed");
    return resultToReply ("audition.position", result);
}

juce::String AuditionPreviewService::handleStop (const juce::DynamicObject& object,
                                                  const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.stop", error);
    if (error.isNotEmpty())
        return errorReply ("audition.stop", "validation_error", error);

    audioPlane.stop (sessionId.toStdString());
    const auto result = state.stop (sessionId.toStdString());
    publishStateEvent (result, "audition.stopped");
    return resultToReply ("audition.stop", result);
}

juce::String AuditionPreviewService::handleInspectCandidate (const juce::DynamicObject& object,
                                                              const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    requiredString (object, "session_id", "audition.inspect_candidate", error);
    if (error.isNotEmpty())
        return errorReply ("audition.inspect_candidate", "validation_error", error);
    requiredString (object, "candidate_id", "audition.inspect_candidate", error);
    if (error.isNotEmpty())
        return errorReply ("audition.inspect_candidate", "validation_error", error);
    return errorReply ("audition.inspect_candidate", "capability_not_supported",
                       "inspect_candidate is an explicit Active Project Plane operation and is not implemented by the target audio spike");
}

juce::String AuditionPreviewService::handleApplyCandidate (const juce::DynamicObject& object,
                                                            const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    requiredString (object, "session_id", "audition.apply_candidate", error);
    if (error.isNotEmpty())
        return errorReply ("audition.apply_candidate", "validation_error", error);
    requiredString (object, "candidate_id", "audition.apply_candidate", error);
    if (error.isNotEmpty())
        return errorReply ("audition.apply_candidate", "validation_error", error);
    return errorReply ("audition.apply_candidate", "capability_not_supported",
                       "apply_candidate is an explicit project mutation and is not implemented by the target audio spike");
}

juce::String AuditionPreviewService::requiredString (const juce::DynamicObject& object,
                                                     const char* property,
                                                     const char* command,
                                                     juce::String& error)
{
    const auto value = object.getProperty (property).toString().trim();
    if (value.isEmpty())
        error = juce::String (command) + " requires " + property;
    else
        error.clear();
    return value;
}

juce::var AuditionPreviewService::sessionToVar (const audition::Session& session) const
{
    auto output = std::make_unique<juce::DynamicObject>();
    output->setProperty ("session_id", juce::String (session.id.c_str()));
    output->setProperty ("conversation_id", juce::String (session.conversationId.c_str()));
    output->setProperty ("scope", juce::String (session.scope.c_str()));
    output->setProperty ("status", audition::toString (session.status));
    output->setProperty ("state_revision", static_cast<juce::int64> (session.stateRevision));
    output->setProperty ("active_candidate_id", juce::String (session.activeCandidateId.c_str()));

    auto activeProject = std::make_unique<juce::DynamicObject>();
    activeProject->setProperty ("plane", "active_project");
    activeProject->setProperty ("project_ref", juce::String (session.activeProjectRef.c_str()));
    activeProject->setProperty ("project_revision", juce::String (session.activeProjectRevision.c_str()));
    output->setProperty ("active_project_plane", juce::var (activeProject.release()));

    const auto playback = audioPlane.snapshot (session.id);
    auto previewPlane = std::make_unique<juce::DynamicObject>();
    previewPlane->setProperty ("plane", "audition_preview");
    previewPlane->setProperty ("session_id", juce::String (session.id.c_str()));
    previewPlane->setProperty ("status", audition::toString (session.status));
    previewPlane->setProperty ("active_candidate_id", juce::String (session.activeCandidateId.c_str()));
    previewPlane->setProperty ("audio_source_prepared", playback.prepared);
    previewPlane->setProperty ("audio_source_active", playback.previewActive);
    previewPlane->setProperty ("audio_candidate_id", juce::String (playback.candidateId.c_str()));
    previewPlane->setProperty ("audio_position_seconds", playback.positionSeconds);
    previewPlane->setProperty ("audio_is_playing", playback.isPlaying);
    previewPlane->setProperty ("crossfade_samples", playback.crossfadeLengthSamples);
    output->setProperty ("audition_preview_plane", juce::var (previewPlane.release()));

    auto transport = std::make_unique<juce::DynamicObject>();
    transport->setProperty ("position_seconds", playback.previewActive ? playback.positionSeconds : session.transport.positionSeconds);
    transport->setProperty ("is_playing", playback.previewActive ? playback.isPlaying : session.transport.isPlaying);
    transport->setProperty ("sample_rate", session.transport.sampleRate);
    transport->setProperty ("timeline_revision", juce::String (session.transport.timelineRevision.c_str()));
    output->setProperty ("transport", juce::var (transport.release()));

    juce::Array<juce::var> candidates;
    for (const auto& candidate : session.candidates)
        candidates.add (candidateToVar (candidate));
    output->setProperty ("candidates", juce::var (candidates));
    if (session.candidates.size() == 2)
    {
        output->setProperty ("candidate_a", candidateToVar (session.candidates[0]));
        output->setProperty ("candidate_b", candidateToVar (session.candidates[1]));
    }

    return juce::var (output.release());
}

juce::String AuditionPreviewService::resultToReply (const char* command, const audition::Result& result) const
{
    auto output = std::make_unique<juce::DynamicObject>();
    output->setProperty ("status", result.ok ? "ok" : "error");
    output->setProperty ("command", command);
    output->setProperty ("message", juce::String (result.message.c_str()));
    if (! result.ok)
        output->setProperty ("code", juce::String (result.code.c_str()));
    if (result.session.has_value())
        output->setProperty ("session", sessionToVar (*result.session));
    return juce::JSON::toString (juce::var (output.release()));
}

juce::String AuditionPreviewService::errorReply (const char* command,
                                                  const juce::String& code,
                                                  const juce::String& message)
{
    auto output = std::make_unique<juce::DynamicObject>();
    output->setProperty ("status", "error");
    output->setProperty ("command", command);
    output->setProperty ("code", code);
    output->setProperty ("message", message);
    return juce::JSON::toString (juce::var (output.release()));
}

void AuditionPreviewService::publishStateEvent (const audition::Result& result, const char* fallbackType) const
{
    if (! publish || ! result.session.has_value())
        return;

    const auto& session = *result.session;
    auto type = result.ok ? juce::String (fallbackType) : juce::String ("audition.failed");
    if (result.ok && result.message == "audition.ready")
        type = "audition.ready";
    else if (result.ok && result.message == "audition.candidate.ready")
        type = "audition.candidate.ready";

    auto event = std::make_unique<juce::DynamicObject>();
    event->setProperty ("topic", "audition");
    event->setProperty ("subtopic", type);
    event->setProperty ("type", type);
    event->setProperty ("conversation_id", juce::String (session.conversationId.c_str()));
    event->setProperty ("session", sessionToVar (session));
    if (! result.ok)
    {
        event->setProperty ("code", juce::String (result.code.c_str()));
        event->setProperty ("message", juce::String (result.message.c_str()));
    }
    publish (juce::JSON::toString (juce::var (event.release())));
}

void AuditionPreviewService::captureAuthoritativeTransport (audition::Session& session) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;
    if (edit == nullptr)
        return;
    auto& transport = edit->getTransport();
    session.transport.positionSeconds = transport.getPosition().inSeconds();
    session.transport.isPlaying = transport.isPlaying();
    session.transport.sampleRate = edit->engine.getDeviceManager().getSampleRate();
}

} // namespace vit
