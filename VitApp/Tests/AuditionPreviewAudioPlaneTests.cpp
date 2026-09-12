#include "../Source/Service/AuditionPreviewAudioPlane.h"
#include "../Source/Service/AuditionPreviewService.h"

#include <cassert>
#include <cmath>
#include <memory>

namespace
{

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
    assert (stream != nullptr);
    auto writer = std::unique_ptr<juce::AudioFormatWriter> (
        juce::WavAudioFormat().createWriterFor (stream.release(), sampleRate, 1, 16, {}, 0));
    assert (writer != nullptr);
    assert (writer->writeFromAudioSampleBuffer (buffer, 0, buffer.getNumSamples()));
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
    assert (directory.createDirectory());
    const auto candidateA = writeConstantWave (directory, "candidate-a.wav", 0.20f);
    const auto candidateB = writeConstantWave (directory, "candidate-b.wav", -0.60f);

    vit::AuditionPreviewAudioPlane plane (nullptr);
    auto session = makeSession (candidateA, candidateB);
    const auto prepared = plane.prepare (session);
    assert (prepared.ok);
    assert (prepared.candidates.size() == 2);
    assert (plane.isCandidatePrepared (session.id, "candidate-a"));
    assert (plane.isCandidatePrepared (session.id, "candidate-b"));
    assert (! prepared.candidates[0].previewRef.empty());
    assert (! prepared.candidates[1].previewRevision.empty());

    assert (plane.select (session.id, "candidate-a", 0.0, true));
    const auto a = plane.renderTestBlock (session.id, "candidate-a", 0.0, 256);
    assert (a.previewActive);
    assert (a.candidateId == "candidate-a");
    assert (a.startPositionSamples == 0);
    assert (a.endPositionSamples == 256);
    assert (a.rms > 0.18 && a.rms < 0.22);

    assert (plane.select (session.id, "candidate-b", 256.0 / 44100.0, true));
    const auto b = plane.renderTestBlock (session.id, "candidate-b", 256.0 / 44100.0, 512);
    assert (b.previewActive);
    assert (b.candidateId == "candidate-b");
    assert (b.startPositionSamples == a.endPositionSamples);
    assert (b.endPositionSamples == a.endPositionSamples + 512);
    assert (b.crossfadeApplied);
    assert (b.rms > 0.40);
    assert (std::abs (a.rms - b.rms) > 0.15);

    const auto diagnostics = plane.diagnostics();
    assert (diagnostics.candidatesDecoded == 2);
    assert (diagnostics.sourceSwitchRequests == 2);
    assert (diagnostics.audioBlocksRendered == 2);
    assert (diagnostics.renderRequests == 0);
    assert (diagnostics.checkoutRequests == 0);
    assert (diagnostics.projectOpenRequests == 0);
    assert (diagnostics.kernelReloadRequests == 0);

    const auto snapshot = plane.snapshot (session.id);
    assert (snapshot.prepared);
    assert (snapshot.candidateId == "candidate-b");
    assert (snapshot.positionSeconds > 0.0);

    plane.invalidate (session.id);
    const auto invalidated = plane.snapshot (session.id);
    assert (! invalidated.previewActive);
    assert (! invalidated.isPlaying);

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
    assert (preparedObject != nullptr && preparedObject->getProperty ("status").toString() == "ok");
    auto* preparedSession = preparedObject->getProperty ("session").getDynamicObject();
    assert (preparedSession != nullptr && preparedSession->getProperty ("status").toString() == "ready");
    auto* preparedA = preparedSession->getProperty ("candidate_a").getDynamicObject();
    assert (preparedA != nullptr && preparedA->getProperty ("preview_ref").toString().startsWith ("audio-buffer://"));
    assert (preparedA->getProperty ("preview_revision").toString().isNotEmpty());
    assert (! events.isEmpty());
    const auto lastEventValue = juce::JSON::parse (events.getLast());
    auto* lastEvent = lastEventValue.getDynamicObject();
    assert (lastEvent != nullptr && lastEvent->getProperty ("type").toString() == "audition.ready");

    juce::DynamicObject selectRequest;
    selectRequest.setProperty ("session_id", "service-session");
    selectRequest.setProperty ("candidate_id", "candidate-b");
    const auto selectReply = juce::JSON::parse (service.handleSelect (selectRequest, {}));
    auto* selectedSession = selectReply.getDynamicObject()->getProperty ("session").getDynamicObject();
    assert (selectedSession->getProperty ("active_candidate_id").toString() == "candidate-b");
    auto* activeProject = selectedSession->getProperty ("active_project_plane").getDynamicObject();
    assert (activeProject->getProperty ("project_ref").toString() == "project:active");
    assert (activeProject->getProperty ("project_revision").toString() == "project-r17");

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
    assert (auditionSelectAutoStarts);
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
    assert (stoppedPrepared.getDynamicObject()->getProperty ("status").toString() == "ok");
    assert (! stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session").previewActive);
    // Blind-telemetry pin at prepare: the preview ref names the Kernel buffer.
    // The source render file name is a physical-mapping token, so no candidate
    // row may echo it into the event stream.
    auto* stoppedPreparedSession = stoppedPrepared.getDynamicObject()->getProperty ("session").getDynamicObject();
    assert (stoppedPreparedSession != nullptr);
    for (const auto& row : *stoppedPreparedSession->getProperty ("candidates").getArray())
    {
        auto* candidateRow = row.getDynamicObject();
        assert (candidateRow != nullptr);
        const auto ref = candidateRow->getProperty ("preview_ref").toString();
        assert (ref.startsWith ("audio-buffer://"));
        assert (ref.contains ("candidate-a") || ref.contains ("candidate-b"));
        assert (! ref.contains ("before_revision") && ! ref.contains ("after_revision"));
        assert (! ref.contains (".wav"));
    }

    juce::DynamicObject stoppedSelectA;
    stoppedSelectA.setProperty ("session_id", "stopped-session");
    stoppedSelectA.setProperty ("candidate_id", "candidate-a");
    const auto stoppedSelectReply = juce::JSON::parse (stoppedService.handleSelect (stoppedSelectA, {}));
    assert (stoppedSelectReply.getDynamicObject()->getProperty ("status").toString() == "ok");
    // The select reply is what audition.select.changed republishes, so the same
    // neutrality pin applies to the select path.
    assert (! stoppedSelectReply.getDynamicObject()->getProperty ("session").toString().contains ("before_revision"));
    assert (! stoppedSelectReply.getDynamicObject()->getProperty ("session").toString().contains ("after_revision"));
    const auto stoppedPlayback = stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session");
    assert (stoppedPlayback.previewActive);
    assert (stoppedPlayback.isPlaying);
    assert (stoppedPlayback.candidateId == "candidate-a");
    const auto stoppedBlock = stoppedService.getAudioPlaneForTesting().renderTestBlock ("stopped-session", "candidate-a", 0.0, 256);
    assert (stoppedBlock.previewActive);
    assert (stoppedBlock.rms > 0.18 && stoppedBlock.rms < 0.22);

    juce::DynamicObject stoppedSelectB;
    stoppedSelectB.setProperty ("session_id", "stopped-session");
    stoppedSelectB.setProperty ("candidate_id", "candidate-b");
    assert (juce::JSON::parse (stoppedService.handleSelect (stoppedSelectB, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto stoppedCrossfade = stoppedService.getAudioPlaneForTesting().renderTestBlock ("stopped-session", "candidate-b", stoppedBlock.endPositionSamples / 44100.0, 512);
    assert (stoppedCrossfade.previewActive);
    assert (stoppedCrossfade.crossfadeApplied);
    assert (stoppedCrossfade.rms > 0.40);
    assert (std::abs (stoppedBlock.rms - stoppedCrossfade.rms) > 0.15);
    bool sawStoppedSelectEvent = false;
    for (const auto& event : stoppedEvents)
    {
        const auto value = juce::JSON::parse (event);
        auto* object = value.getDynamicObject();
        if (object != nullptr && object->getProperty ("type").toString() == "audition.select.changed")
            sawStoppedSelectEvent = true;
    }
    assert (sawStoppedSelectEvent);

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
    assert (juce::JSON::parse (stoppedService.handlePosition (silentPosition, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    assert (! stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session").previewActive);
    assert (juce::JSON::parse (stoppedService.handleSelect (manualSelect, {})).getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto manualPlayback = stoppedService.getAudioPlaneForTesting().snapshot ("stopped-session");
    assert (! manualPlayback.previewActive);
    assert (! manualPlayback.isPlaying);

    // AUDITION-PLAY-1 pin (5): blind-tier telemetry must not carry the physical
    // render file name. The preview plane is Kernel-owned and the ref names the
    // Kernel buffer, never the source file.
    bool leakedRenderName = false;
    for (const auto& event : stoppedEvents)
        if (event.contains ("before_revision") || event.contains ("after_revision"))
            leakedRenderName = true;
    assert (! leakedRenderName);

    juce::DynamicObject playRequest;
    playRequest.setProperty ("session_id", "service-session");
    playRequest.setProperty ("position_seconds", 0.0);
    playRequest.setProperty ("is_playing", true);
    const auto playReply = juce::JSON::parse (service.handlePosition (playRequest, {}));
    assert (playReply.getDynamicObject()->getProperty ("status").toString() == "ok");
    const auto servicePlayback = service.getAudioPlaneForTesting().snapshot ("service-session");
    assert (servicePlayback.previewActive && servicePlayback.isPlaying && servicePlayback.candidateId == "candidate-b");

    juce::DynamicObject staleRequest;
    staleRequest.setProperty ("session_id", "service-session");
    service.handleStale (staleRequest, {});
    assert (! service.getAudioPlaneForTesting().snapshot ("service-session").previewActive);
    assert (juce::JSON::parse (service.handleSelect (selectRequest, {})).getDynamicObject()->getProperty ("status").toString() == "error");

    juce::DynamicObject explicitRequest;
    explicitRequest.setProperty ("session_id", "service-session");
    explicitRequest.setProperty ("candidate_id", "candidate-b");
    assert (juce::JSON::parse (service.handleInspectCandidate (explicitRequest, {})).getDynamicObject()->getProperty ("code").toString() == "capability_not_supported");
    assert (juce::JSON::parse (service.handleApplyCandidate (explicitRequest, {})).getDynamicObject()->getProperty ("code").toString() == "capability_not_supported");

    const auto serviceDiagnostics = service.getAudioPlaneForTesting().diagnostics();
    assert (serviceDiagnostics.renderRequests == 0 && serviceDiagnostics.checkoutRequests == 0
            && serviceDiagnostics.projectOpenRequests == 0 && serviceDiagnostics.kernelReloadRequests == 0);

    directory.deleteRecursively();
    return 0;
}
