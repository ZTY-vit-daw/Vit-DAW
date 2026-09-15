#include "../Source/Service/AuditionPreviewAudioPlane.h"
#include "../Source/Service/AuditionPreviewService.h"

#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <memory>

// AUDITION-REL-1: Release defines NDEBUG, which compiles plain assert() out.
// This test's setup side effects (directory creation, WAV data write) and its
// null-pointer guards lived inside assert(), so in Release the candidates were
// never written, prepare failed, and the service-reply derefs ran unguarded
// into a deterministic 0xC0000005 (NamedValueSet::operator[] on a null this).
// Checks here must stay active in every configuration: they abort with a
// message instead of vanishing under /DNDEBUG.
#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    std::abort();
}

juce::File writeConstantWave (const juce::File& directory, const juce::String& name, float value)
{
    constexpr double sampleRate = 44100.0;
    constexpr int numSamples = 44100;
    auto file = directory.getChildFile (name);
    file.deleteFile();

    juce::AudioBuffer<float> buffer (1, numSamples);
    buffer.clear();
    for (int sample = 0; sample < numSamples; ++sample)
        buffer.setSample (0, sample, value);

    auto stream = std::unique_ptr<juce::OutputStream> (file.createOutputStream());
    VIT_CHECK (stream != nullptr);
    auto writer = std::unique_ptr<juce::AudioFormatWriter> (
        juce::WavAudioFormat().createWriterFor (stream.release(), sampleRate, 1, 16, {}, 0));
    VIT_CHECK (writer != nullptr);
    const bool wrote = writer->writeFromAudioSampleBuffer (buffer, 0, buffer.getNumSamples());
    VIT_CHECK (wrote);
    return file;
}

vit::audition::Session makeSession (const juce::File& candidateA, const juce::File& candidateB)
{
    vit::audition::Session session;
    session.id = "audio-plane-session";
    session.scope = "target";
    session.activeProjectRef = "project:active";
    session.activeProjectRevision = "project-r17";
    session.transport.timelineRevision = "timeline-r17";
    session.transport.positionSeconds = 0.0;
    session.transport.isPlaying = true;
    vit::audition::Candidate a;
    a.id = "candidate-a"; a.label = "Candidate A"; a.sourceKind = "audio_file";
    a.sourceRef = candidateA.getFullPathName().toStdString(); a.checkpointRef = "checkpoint:a";
    a.branchRef = "branch:a"; a.projectRevision = "project-r17"; a.renderRevision = "direct-audio-file"; a.scope = "target";
    vit::audition::Candidate b;
    b.id = "candidate-b"; b.label = "Candidate B"; b.sourceKind = "audio_file";
    b.sourceRef = candidateB.getFullPathName().toStdString(); b.commitId = "commit-b";
    b.worktreeRef = "worktree:b"; b.projectRevision = "project-r17"; b.renderRevision = "direct-audio-file"; b.scope = "target";
    session.candidates = { a, b };
    return session;
}

} // namespace

int main()
{
    const auto directory = juce::File::getSpecialLocation (juce::File::tempDirectory)
        .getNonexistentChildFile ("vit-audition-audio-plane", {}, true);
    VIT_CHECK (directory.createDirectory());
    const auto candidateA = writeConstantWave (directory, "candidate-a.wav", 0.20f);
    const auto candidateB = writeConstantWave (directory, "candidate-b.wav", -0.60f);

    vit::AuditionPreviewAudioPlane plane (nullptr);
    auto session = makeSession (candidateA, candidateB);
    const auto prepared = plane.prepare (session);
    VIT_CHECK (prepared.ok);
    VIT_CHECK (prepared.candidates.size() == 2);
    VIT_CHECK (plane.isCandidatePrepared (session.id, "candidate-a"));
    VIT_CHECK (plane.isCandidatePrepared (session.id, "candidate-b"));
    VIT_CHECK (! prepared.candidates[0].previewRef.empty());
    VIT_CHECK (! prepared.candidates[1].previewRevision.empty());

    VIT_CHECK (plane.select (session.id, "candidate-a", 0.0, true));
    const auto a = plane.renderTestBlock (session.id, "candidate-a", 0.0, 256);
    VIT_CHECK (a.previewActive);
    VIT_CHECK (a.candidateId == "candidate-a");
    VIT_CHECK (a.startPositionSamples == 0);
    VIT_CHECK (a.endPositionSamples == 256);
    VIT_CHECK (a.rms > 0.18 && a.rms < 0.22);

    VIT_CHECK (plane.select (session.id, "candidate-b", 256.0 / 44100.0, true));
    const auto b = plane.renderTestBlock (session.id, "candidate-b", 256.0 / 44100.0, 512);
    VIT_CHECK (b.previewActive);
    VIT_CHECK (b.candidateId == "candidate-b");
    VIT_CHECK (b.startPositionSamples == a.endPositionSamples);
    VIT_CHECK (b.endPositionSamples == a.endPositionSamples + 512);
    VIT_CHECK (b.crossfadeApplied);
    VIT_CHECK (b.rms > 0.40);
    VIT_CHECK (std::abs (a.rms - b.rms) > 0.15);

    const auto diagnostics = plane.diagnostics();
    VIT_CHECK (diagnostics.candidatesDecoded == 2);
    VIT_CHECK (diagnostics.sourceSwitchRequests == 2);
    VIT_CHECK (diagnostics.audioBlocksRendered == 2);
    VIT_CHECK (diagnostics.renderRequests == 0);
    VIT_CHECK (diagnostics.checkoutRequests == 0);
    VIT_CHECK (diagnostics.projectOpenRequests == 0);
    VIT_CHECK (diagnostics.kernelReloadRequests == 0);

    const auto snapshot = plane.snapshot (session.id);
    VIT_CHECK (snapshot.prepared);
    VIT_CHECK (snapshot.candidateId == "candidate-b");
    VIT_CHECK (snapshot.positionSeconds > 0.0);

    plane.invalidate (session.id);
    const auto invalidated = plane.snapshot (session.id);
    VIT_CHECK (! invalidated.previewActive);
    VIT_CHECK (! invalidated.isPlaying);

    juce::Array<juce::String> events;
    vit::AuditionPreviewService service (nullptr, {}, [&events] (const juce::String& event) { events.add (event); });
    juce::DynamicObject request;
    request.setProperty ("session_id", "service-session");
    request.setProperty ("conversation_id", "conversation-audio");
    request.setProperty ("scope", "target");
    request.setProperty ("active_project_ref", "project:active");
    request.setProperty ("active_project_revision", "project-r17");
    request.setProperty ("timeline_revision", "timeline-r17");
    auto makeCandidate = [] (const char* id, const juce::File& file, const char* checkpoint, const char* branch, const char* worktree)
    {
        auto candidate = std::make_unique<juce::DynamicObject>();
        candidate->setProperty ("id", id);
        candidate->setProperty ("label", id);
        candidate->setProperty ("source_kind", "audio_file");
        candidate->setProperty ("source_ref", file.getFullPathName());
        candidate->setProperty ("checkpoint_ref", checkpoint);
        candidate->setProperty ("commit_id", juce::String (id) + "-commit");
        candidate->setProperty ("branch_ref", branch);
        candidate->setProperty ("worktree_ref", worktree);
        candidate->setProperty ("project_revision", "project-r17");
        candidate->setProperty ("render_revision", "direct-audio-file");
        candidate->setProperty ("scope", "target");
        return juce::var (candidate.release());
    };
    request.setProperty ("candidate_a", makeCandidate ("candidate-a", candidateA, "checkpoint:a", "branch:a", "worktree:a"));
    request.setProperty ("candidate_b", makeCandidate ("candidate-b", candidateB, "checkpoint:b", "branch:b", "worktree:b"));
    const auto preparedReply = juce::JSON::parse (service.handlePrepare (request, {}));
    auto* preparedObject = preparedReply.getDynamicObject();
    VIT_CHECK (preparedObject != nullptr && preparedObject->getProperty ("status").toString() == "ok");
    auto* preparedSession = preparedObject->getProperty ("session").getDynamicObject();
    VIT_CHECK (preparedSession != nullptr && preparedSession->getProperty ("status").toString() == "ready");
    auto* preparedA = preparedSession->getProperty ("candidate_a").getDynamicObject();
    VIT_CHECK (preparedA != nullptr && preparedA->getProperty ("preview_ref").toString().startsWith ("audio-buffer://"));
    VIT_CHECK (preparedA->getProperty ("preview_revision").toString().isNotEmpty());
    VIT_CHECK (! events.isEmpty());
    const auto lastEventValue = juce::JSON::parse (events.getLast());
    auto* lastEvent = lastEventValue.getDynamicObject();
    VIT_CHECK (lastEvent != nullptr && lastEvent->getProperty ("type").toString() == "audition.ready");

    juce::DynamicObject selectRequest;
    selectRequest.setProperty ("session_id", "service-session");
    selectRequest.setProperty ("candidate_id", "candidate-b");
    const auto selectReply = juce::JSON::parse (service.handleSelect (selectRequest, {}));
    auto* selectedSession = selectReply.getDynamicObject()->getProperty ("session").getDynamicObject();
    VIT_CHECK (selectedSession->getProperty ("active_candidate_id").toString() == "candidate-b");
    auto* activeProject = selectedSession->getProperty ("active_project_plane").getDynamicObject();
    VIT_CHECK (activeProject->getProperty ("project_ref").toString() == "project:active");
    VIT_CHECK (activeProject->getProperty ("project_revision").toString() == "project-r17");

// AUDITION-PLAY-1: `audition.select` is playback control for the preview plane.
// It must open the gate on its own; the project transport stays a position
// source only. `auto_start` explicitly false is the documented conservative
// opt-out and keeps the pre-card behaviour byte-for-byte.
constexpr bool auditionSelectAutoStarts = true;


    // AUDITION-PLAY-1 regression pin: the real-stack defect was a silent click.
    // The user pressed an A/B card while the project transport was stopped, so
    // every select arrived with isPlaying=false and the preview gate stayed
    // shut. The preview buffer is self-contained and the output processor
    // clears the buffer itself, so select must not inherit transport.
    VIT_CHECK (auditionSelectAutoStarts);
    juce::Array<juce::String> stoppedEvents;
    vit::AuditionPreviewService stoppedService (nullptr, {}, [&stoppedEvents] (const juce::String& event) { stoppedEvents.add (event); });
    juce::DynamicObject stoppedPrepare;
    stoppedPrepare.setProperty ("session_id", "stopped-session");
    stoppedPrepare.setProperty ("conversation_id", "conversation-stopped");
    stoppedPrepare.setProperty ("scope", "target");
    stoppedPrepare.setProperty ("active_project_ref", "project:active");
    stoppedPrepare.setProperty ("active_project_revision", "project-r17");
    stoppedPrepare.setProperty ("timeline_revision", "timeline-r17");
    stoppedPrepare.setProperty ("candidate_a", makeCandidate ("candidate-a", candidateA, "checkpoint:stopped-a", "branch:a", "worktree:a"));
    stoppedPrepare.setProperty ("candidate_b", makeCandidate ("candidate-b", candidateB, "checkpoint:stopped-b", "branch:b", "worktree:b"));
    const auto stoppedPrepared = juce::JSON::parse (stoppedService.handlePrepare (stoppedPrepare, {}));
    VIT_CHECK (stoppedPrepared.getDynamicObject()->getProperty ("status").toString() == "ok");
    VIT_CHECK (! stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session").previewActive);
    // Blind-telemetry pin at prepare: the preview ref names the Kernel buffer.
    // The source render file name is a physical-mapping token, so no candidate
    // row may echo it into the event stream.
    auto* stoppedPreparedSession = stoppedPrepared.getDynamicObject()->getProperty ("session").getDynamicObject();
    VIT_CHECK (stoppedPreparedSession != nullptr);
    for (const auto& row : *stoppedPreparedSession->getProperty ("candidates").getArray())
    {
        auto* candidateRow = row.getDynamicObject();
        VIT_CHECK (candidateRow != nullptr);
        const auto ref = candidateRow->getProperty ("preview_ref").toString();
        VIT_CHECK (ref.startsWith ("audio-buffer://"));
        VIT_CHECK (ref.contains ("candidate-a") || ref.contains ("candidate-b"));
        VIT_CHECK (! ref.contains ("before_revision") && ! ref.contains ("after_revision"));
        VIT_CHECK (! ref.contains (".wav"));
    }

    juce::DynamicObject stoppedSelectA;
    stoppedSelectA.setProperty ("session_id", "stopped-session");
    stoppedSelectA.setProperty ("candidate_id", "candidate-a");
    const auto stoppedSelectReply = juce::JSON::parse (stoppedService.handleSelect (stoppedSelectA, {}));
    VIT_CHECK (stoppedSelectReply.getDynamicObject()->getProperty ("status").toString() == "ok");
    // The select reply is what audition.select.changed republishes, so the same
    // neutrality pin applies to the select path.
    VIT_CHECK (! stoppedSelectReply.getDynamicObject()->getProperty ("session").toString().contains ("before_revision"));
    VIT_CHECK (! stoppedSelectReply.getDynamicObject()->getProperty ("session").toString().contains ("after_revision"));
    const auto stoppedPlayback = stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session");
    VIT_CHECK (stoppedPlayback.previewActive);
    VIT_CHECK (stoppedPlayback.isPlaying);
    VIT_CHECK (stoppedPlayback.candidateId == "candidate-a");
    const auto stoppedBlock = stoppedService.getAudioPlaneForTesting().renderTestBlock ("stopped-session", "candidate-a", 0.0, 256);
    VIT_CHECK (stoppedBlock.previewActive);
    VIT_CHECK (stoppedBlock.rms > 0.18 && stoppedBlock.rms < 0.22);

    juce::DynamicObject stoppedSelectB;
    stoppedSelectB.setProperty ("session_id", "stopped-session");
    stoppedSelectB.setProperty ("candidate_id", "candidate-b");
    VIT_CHECK (juce::JSON::parse (stoppedService.handleSelect (stoppedSelectB, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto stoppedCrossfade = stoppedService.getAudioPlaneForTesting().renderTestBlock ("stopped-session", "candidate-b", stoppedBlock.endPositionSamples / 44100.0, 512);
    VIT_CHECK (stoppedCrossfade.previewActive);
    VIT_CHECK (stoppedCrossfade.crossfadeApplied);
    VIT_CHECK (stoppedCrossfade.rms > 0.40);
    VIT_CHECK (std::abs (stoppedBlock.rms - stoppedCrossfade.rms) > 0.15);
    bool sawStoppedSelectEvent = false;
    for (const auto& event : stoppedEvents)
    {
        const auto value = juce::JSON::parse (event);
        auto* object = value.getDynamicObject();
        if (object != nullptr && object->getProperty ("type").toString() == "audition.select.changed")
            sawStoppedSelectEvent = true;
    }
    VIT_CHECK (sawStoppedSelectEvent);

    juce::DynamicObject manualSelect;
    manualSelect.setProperty ("session_id", "stopped-session");
    manualSelect.setProperty ("candidate_id", "candidate-b");
    manualSelect.setProperty ("auto_start", false);
    // Transport semantics stay anchored: an explicit position update can still
    // close the preview gate, and it leaves the session selectable.
    juce::DynamicObject silentPosition;
    silentPosition.setProperty ("session_id", "stopped-session");
    silentPosition.setProperty ("position_seconds", 0.0);
    silentPosition.setProperty ("is_playing", false);
    VIT_CHECK (juce::JSON::parse (stoppedService.handlePosition (silentPosition, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    VIT_CHECK (! stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session").previewActive);
    VIT_CHECK (juce::JSON::parse (stoppedService.handleSelect (manualSelect, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto manualPlayback = stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session");
    VIT_CHECK (! manualPlayback.previewActive);
    VIT_CHECK (! manualPlayback.isPlaying);

    // AUDITION-PLAY-1 pin (5): blind-tier telemetry must not carry the physical
    // render file name. The preview plane is Kernel-owned and the ref names the
    // Kernel buffer, never the source file.
    bool leakedRenderName = false;
    for (const auto& event : stoppedEvents)
        if (event.contains ("before_revision") || event.contains ("after_revision"))
            leakedRenderName = true;
    VIT_CHECK (! leakedRenderName);

    juce::DynamicObject playRequest;
    playRequest.setProperty ("session_id", "service-session");
    playRequest.setProperty ("position_seconds", 0.0);
    playRequest.setProperty ("is_playing", true);
    const auto playReply = juce::JSON::parse (service.handlePosition (playRequest, {}));
    VIT_CHECK (playReply.getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto servicePlayback = service.getAudioPlaneForTesting().snapshot ("service-session");
    VIT_CHECK (servicePlayback.previewActive && servicePlayback.isPlaying && servicePlayback.candidateId == "candidate-b");

    juce::DynamicObject staleRequest;
    staleRequest.setProperty ("session_id", "service-session");
    service.handleStale (staleRequest, {});
    VIT_CHECK (! service.getAudioPlaneForTesting().snapshot ("service-session").previewActive);
    VIT_CHECK (juce::JSON::parse (service.handleSelect (selectRequest, {})).getDynamicObject()->getProperty ("status").toString() == "error");

    juce::DynamicObject explicitRequest;
    explicitRequest.setProperty ("session_id", "service-session");
    explicitRequest.setProperty ("candidate_id", "candidate-b");
    VIT_CHECK (juce::JSON::parse (service.handleInspectCandidate (explicitRequest, {})).getDynamicObject()->getProperty ("code").toString() == "capability_not_supported");
    VIT_CHECK (juce::JSON::parse (service.handleApplyCandidate (explicitRequest, {})).getDynamicObject()->getProperty ("code").toString() == "capability_not_supported");

    const auto serviceDiagnostics = service.getAudioPlaneForTesting().diagnostics();
    VIT_CHECK (serviceDiagnostics.renderRequests == 0 && serviceDiagnostics.checkoutRequests == 0
            && serviceDiagnostics.projectOpenRequests == 0 && serviceDiagnostics.kernelReloadRequests == 0);

    directory.deleteRecursively();
    return 0;
}
