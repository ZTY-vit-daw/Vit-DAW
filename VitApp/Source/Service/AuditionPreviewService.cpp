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
    object->setProperty ("status", audition::toString (candidate.status));
    return juce::var (object.release());
}

bool appendCandidate (const juce::var& value,
                     std::vector<audition::Candidate>& output,
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
    candidate.previewRef = candidateObject->getProperty ("preview_ref").toString().trim().toStdString();
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

AuditionPreviewService::AuditionPreviewService (PublishAction publishAction)
    : publish (std::move (publishAction))
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

        if (! appendCandidate (candidateA, session.candidates, error)
            || ! appendCandidate (candidateB, session.candidates, error))
            return errorReply ("audition.prepare", "validation_error", error);
    }
    else
    {
        const auto candidatesValue = object.getProperty ("candidates");
        const auto* candidates = candidatesValue.getArray();
        if (candidates == nullptr)
            return errorReply ("audition.prepare", "validation_error", "audition.prepare requires candidate_a and candidate_b");

        for (const auto& candidateValue : *candidates)
            if (! appendCandidate (candidateValue, session.candidates, error))
                return errorReply ("audition.prepare", "validation_error", error);
    }

    const auto result = state.prepare (std::move (session));
    publishReadyEvent (result);
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

    return resultToReply ("audition.select", state.select (sessionId.toStdString(), candidateId.toStdString()));
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

    return resultToReply ("audition.position", state.position (sessionId.toStdString(), positionSeconds, isPlaying));
}

juce::String AuditionPreviewService::handleStop (const juce::DynamicObject& object,
                                                  const juce::String& rawPayload)
{
    juce::ignoreUnused (rawPayload);
    juce::String error;
    const auto sessionId = requiredString (object, "session_id", "audition.stop", error);
    if (error.isNotEmpty())
        return errorReply ("audition.stop", "validation_error", error);

    return resultToReply ("audition.stop", state.stop (sessionId.toStdString()));
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

juce::var AuditionPreviewService::sessionToVar (const audition::Session& session)
{
    auto output = std::make_unique<juce::DynamicObject>();
    output->setProperty ("session_id", juce::String (session.id.c_str()));
    output->setProperty ("scope", juce::String (session.scope.c_str()));
    output->setProperty ("status", audition::toString (session.status));
    output->setProperty ("state_revision", static_cast<juce::int64> (session.stateRevision));
    output->setProperty ("active_candidate_id", juce::String (session.activeCandidateId.c_str()));

    auto activeProject = std::make_unique<juce::DynamicObject>();
    activeProject->setProperty ("plane", "active_project");
    activeProject->setProperty ("project_ref", juce::String (session.activeProjectRef.c_str()));
    activeProject->setProperty ("project_revision", juce::String (session.activeProjectRevision.c_str()));
    output->setProperty ("active_project_plane", juce::var (activeProject.release()));

    auto previewPlane = std::make_unique<juce::DynamicObject>();
    previewPlane->setProperty ("plane", "audition_preview");
    previewPlane->setProperty ("session_id", juce::String (session.id.c_str()));
    previewPlane->setProperty ("status", audition::toString (session.status));
    previewPlane->setProperty ("active_candidate_id", juce::String (session.activeCandidateId.c_str()));
    output->setProperty ("audition_preview_plane", juce::var (previewPlane.release()));

    auto transport = std::make_unique<juce::DynamicObject>();
    transport->setProperty ("position_seconds", session.transport.positionSeconds);
    transport->setProperty ("is_playing", session.transport.isPlaying);
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

juce::String AuditionPreviewService::resultToReply (const char* command, const audition::Result& result)
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
    audition::Result result;
    result.ok = false;
    result.code = code.toStdString();
    result.message = message.toStdString();
    return resultToReply (command, result);
}

void AuditionPreviewService::publishReadyEvent (const audition::Result& result) const
{
    if (! publish || ! result.ok || (result.message != "audition.ready" && result.message != "audition.candidate.ready") || ! result.session.has_value())
        return;

    auto event = std::make_unique<juce::DynamicObject>();
    event->setProperty ("topic", "audition");
    event->setProperty ("subtopic", "audition.ready");
    event->setProperty ("type", "audition.ready");
    event->setProperty ("session", sessionToVar (*result.session));
    publish (juce::JSON::toString (juce::var (event.release())));
}

} // namespace vit
