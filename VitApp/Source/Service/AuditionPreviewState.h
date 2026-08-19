#pragma once

#include <cstdint>
#include <mutex>
#include <optional>
#include <string>
#include <unordered_map>
#include <vector>

namespace vit::audition
{

enum class CandidateStatus
{
    preparing,
    ready,
    stale,
    failed,
};

enum class SessionStatus
{
    preparing,
    ready,
    playing,
    stopped,
    stale,
    failed,
};

struct Candidate
{
    std::string id;
    std::string label;
    std::string sourceKind;
    std::string sourceRef;
    std::string previewRef;
    CandidateStatus status = CandidateStatus::preparing;
};

struct TransportAnchor
{
    double positionSeconds = 0.0;
    bool isPlaying = false;
    std::string timelineRevision;
};

struct Session
{
    std::string id;
    // Optional AgentEvent routing context; it does not alter preview identity.
    std::string conversationId;
    std::string scope;

    // Active Project Plane identity. This is observational context only: the
    // audition state machine never checks out, renders, reloads, or mutates it.
    std::string activeProjectRef;
    std::string activeProjectRevision;

    // Audition Preview Plane state.
    SessionStatus status = SessionStatus::preparing;
    std::string activeCandidateId;
    TransportAnchor transport;
    std::vector<Candidate> candidates;
    std::uint64_t stateRevision = 0;
};

struct Result
{
    bool ok = false;
    std::string code;
    std::string message;
    std::optional<Session> session;
};

/**
    G4 spike state machine for the Kernel-owned Audition Preview Plane.

    The state machine deliberately has no Edit, renderer, checkout, reload, or
    audio callback dependency. Candidate selection only changes session-local
    Audition Preview Plane state; Active Project Plane identity is copied into
    the session and remains immutable after prepare().
*/
class StateMachine final
{
public:
    Result prepare (Session request);
    Result status (const std::string& sessionId) const;
    Result markCandidateReady (const std::string& sessionId,
                               const std::string& candidateId,
                               const std::string& previewRef);
    Result markStale (const std::string& sessionId);
    Result markFailed (const std::string& sessionId);
    Result select (const std::string& sessionId, const std::string& candidateId);
    Result position (const std::string& sessionId,
                     std::optional<double> positionSeconds,
                     std::optional<bool> isPlaying);
    Result stop (const std::string& sessionId);

private:
    static Result error (std::string code, std::string message);
    static Result success (const Session& session, std::string message);
    static bool scopeSupported (const std::string& scope);
    static Candidate* findCandidate (Session& session, const std::string& candidateId);
    static const Candidate* findCandidate (const Session& session, const std::string& candidateId);
    static void refreshReadiness (Session& session);

    mutable std::mutex mutex;
    std::unordered_map<std::string, Session> sessions;
};

const char* toString (CandidateStatus status) noexcept;
const char* toString (SessionStatus status) noexcept;

} // namespace vit::audition
