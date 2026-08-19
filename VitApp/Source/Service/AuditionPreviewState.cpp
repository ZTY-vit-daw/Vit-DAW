#include "AuditionPreviewState.h"

#include <algorithm>
#include <cmath>
#include <utility>

namespace vit::audition
{

namespace
{

bool hasExactlyTwoCandidates (const Session& session)
{
    if (session.candidates.size() != 2)
        return false;

    return ! session.candidates[0].id.empty()
        && ! session.candidates[1].id.empty()
        && session.candidates[0].id != session.candidates[1].id;
}

} // namespace

const char* toString (CandidateStatus status) noexcept
{
    switch (status)
    {
        case CandidateStatus::preparing: return "preparing";
        case CandidateStatus::ready:     return "ready";
        case CandidateStatus::stale:     return "stale";
        case CandidateStatus::failed:    return "failed";
    }

    return "failed";
}

const char* toString (SessionStatus status) noexcept
{
    switch (status)
    {
        case SessionStatus::preparing: return "preparing";
        case SessionStatus::ready:     return "ready";
        case SessionStatus::playing:   return "playing";
        case SessionStatus::stopped:   return "stopped";
        case SessionStatus::stale:     return "stale";
        case SessionStatus::failed:    return "failed";
    }

    return "failed";
}

Result StateMachine::prepare (Session request)
{
    if (request.id.empty())
        return error ("validation_error", "audition.prepare requires session_id");

    if (! scopeSupported (request.scope))
        return error ("validation_error", "audition.prepare requires scope target, local_bus, or full_project");

    if (request.activeProjectRef.empty())
        return error ("validation_error", "audition.prepare requires active_project_ref");

    if (request.activeProjectRevision.empty())
        return error ("validation_error", "audition.prepare requires active_project_revision");

    if (request.transport.timelineRevision.empty())
        return error ("validation_error", "audition.prepare requires timeline_revision");

    if (! hasExactlyTwoCandidates (request))
        return error ("validation_error", "audition.prepare requires exactly two candidates with distinct ids");

    for (auto& candidate : request.candidates)
    {
        if (candidate.label.empty())
            candidate.label = candidate.id;

        if (candidate.sourceKind.empty())
            candidate.sourceKind = "edit";

        if (candidate.sourceRef.empty())
            return error ("validation_error", "audition.prepare candidate requires source_ref");

        // A pre-resolved preview is already playable. Otherwise a Kernel-side
        // preview preparer must call markCandidateReady after warming it.
        candidate.status = candidate.previewRef.empty()
            ? CandidateStatus::preparing
            : CandidateStatus::ready;
    }

    request.activeCandidateId.clear();
    request.status = SessionStatus::preparing;
    // audition.prepare receives the Kernel's authoritative transport anchor.
    // Preserve it; prepare must not reset the Active Project transport.
    request.stateRevision = 1;
    refreshReadiness (request);

    std::lock_guard lock (mutex);
    sessions[request.id] = request;
    return success (request, request.status == SessionStatus::ready
                               ? "audition.ready"
                               : "audition.prepare.started");
}

Result StateMachine::status (const std::string& sessionId) const
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    return success (found->second, "audition.status");
}

Result StateMachine::markCandidateReady (const std::string& sessionId,
                                          const std::string& candidateId,
                                          const std::string& previewRef)
{
    if (previewRef.empty())
        return error ("validation_error", "audition.ready requires preview_ref");

    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    if (session.status == SessionStatus::stale
        || session.status == SessionStatus::failed
        || session.status == SessionStatus::stopped)
        return error ("invalid_session_state", "Cannot ready a stale, failed, or stopped audition session");

    auto* candidate = findCandidate (session, candidateId);
    if (candidate == nullptr)
        return error ("candidate_not_found", "Unknown audition candidate");

    candidate->previewRef = previewRef;
    candidate->status = CandidateStatus::ready;
    ++session.stateRevision;
    refreshReadiness (session);
    return success (session, session.status == SessionStatus::ready
                               ? "audition.ready"
                               : "audition.candidate.ready");
}

Result StateMachine::setCandidateAudioMetadata (const std::string& sessionId,
                                                const std::string& candidateId,
                                                const std::string& previewRevision,
                                                double durationSeconds,
                                                double sampleRate,
                                                int channelCount)
{
    if (previewRevision.empty() || ! std::isfinite (durationSeconds) || durationSeconds <= 0.0
        || ! std::isfinite (sampleRate) || sampleRate <= 0.0 || channelCount <= 0)
        return error ("validation_error", "candidate audio metadata is invalid");

    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto* candidate = findCandidate (found->second, candidateId);
    if (candidate == nullptr)
        return error ("candidate_not_found", "Unknown audition candidate");

    candidate->previewRevision = previewRevision;
    candidate->durationSeconds = durationSeconds;
    candidate->sampleRate = sampleRate;
    candidate->channelCount = channelCount;
    ++found->second.stateRevision;
    return success (found->second, "audition.candidate.metadata");
}
Result StateMachine::markStale (const std::string& sessionId)
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    session.transport.isPlaying = false;
    session.status = SessionStatus::stale;
    for (auto& candidate : session.candidates)
        candidate.status = CandidateStatus::stale;
    ++session.stateRevision;
    return success (session, "audition.stale");
}

Result StateMachine::markFailed (const std::string& sessionId)
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    session.transport.isPlaying = false;
    session.status = SessionStatus::failed;
    for (auto& candidate : session.candidates)
        candidate.status = CandidateStatus::failed;
    ++session.stateRevision;
    return success (session, "audition.failed");
}

Result StateMachine::select (const std::string& sessionId, const std::string& candidateId)
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    if (session.status != SessionStatus::ready && session.status != SessionStatus::playing)
        return error ("audition_not_ready", "audition.select requires a ready or playing session");

    auto* candidate = findCandidate (session, candidateId);
    if (candidate == nullptr)
        return error ("candidate_not_found", "Unknown audition candidate");

    if (candidate->status != CandidateStatus::ready || candidate->previewRef.empty())
        return error ("candidate_not_ready", "audition.select requires a ready candidate");

    // This is the only mutation performed by select: session-local preview
    // selection. Active Project Plane identity remains untouched.
    session.activeCandidateId = candidate->id;
    session.status = session.transport.isPlaying ? SessionStatus::playing : SessionStatus::ready;
    ++session.stateRevision;
    return success (session, "audition.select.changed");
}

Result StateMachine::position (const std::string& sessionId,
                               std::optional<double> positionSeconds,
                               std::optional<bool> isPlaying)
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    if (session.status != SessionStatus::ready && session.status != SessionStatus::playing)
        return error ("audition_not_ready", "audition.position requires a ready or playing session");

    if (positionSeconds.has_value())
    {
        if (! std::isfinite (*positionSeconds) || *positionSeconds < 0.0)
            return error ("validation_error", "audition.position requires a finite non-negative position_seconds");
        session.transport.positionSeconds = *positionSeconds;
    }

    if (isPlaying.has_value())
    {
        if (*isPlaying && session.activeCandidateId.empty())
            return error ("candidate_not_selected", "audition.position cannot play before audition.select");
        session.transport.isPlaying = *isPlaying;
    }

    session.status = session.transport.isPlaying ? SessionStatus::playing : SessionStatus::ready;
    ++session.stateRevision;
    return success (session, "audition.position.changed");
}

Result StateMachine::stop (const std::string& sessionId)
{
    std::lock_guard lock (mutex);
    const auto found = sessions.find (sessionId);
    if (found == sessions.end())
        return error ("session_not_found", "Unknown audition session");

    auto& session = found->second;
    if (session.status == SessionStatus::stale || session.status == SessionStatus::failed)
        return error ("invalid_session_state", "Cannot stop a stale or failed audition session");

    session.transport.isPlaying = false;
    session.status = SessionStatus::stopped;
    ++session.stateRevision;
    return success (session, "audition.stopped");
}

Result StateMachine::error (std::string code, std::string message)
{
    return { false, std::move (code), std::move (message), std::nullopt };
}

Result StateMachine::success (const Session& session, std::string message)
{
    return { true, {}, std::move (message), session };
}

bool StateMachine::scopeSupported (const std::string& scope)
{
    return scope == "target" || scope == "local_bus" || scope == "full_project";
}

Candidate* StateMachine::findCandidate (Session& session, const std::string& candidateId)
{
    const auto found = std::find_if (session.candidates.begin(), session.candidates.end(),
                                     [&] (const Candidate& candidate) { return candidate.id == candidateId; });
    return found == session.candidates.end() ? nullptr : &*found;
}

const Candidate* StateMachine::findCandidate (const Session& session, const std::string& candidateId)
{
    const auto found = std::find_if (session.candidates.begin(), session.candidates.end(),
                                     [&] (const Candidate& candidate) { return candidate.id == candidateId; });
    return found == session.candidates.end() ? nullptr : &*found;
}

void StateMachine::refreshReadiness (Session& session)
{
    const auto allReady = std::all_of (session.candidates.begin(), session.candidates.end(),
                                       [] (const Candidate& candidate)
                                       {
                                           return candidate.status == CandidateStatus::ready
                                               && ! candidate.previewRef.empty();
                                       });

    if (allReady && session.status != SessionStatus::playing)
        session.status = SessionStatus::ready;
    else if (! allReady)
        session.status = SessionStatus::preparing;
}

} // namespace vit::audition
