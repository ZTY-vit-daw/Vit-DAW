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
        candidate->setProperty ("engineering_source_kind", "worktree");
        candidate->setProperty ("engineering_source_ref", juce::String ("worktree:") + worktree);
        candidate->setProperty ("checkpoint_ref", checkpoint);
        candidate->setProperty ("commit_id", juce::String (id) + "-commit");
        candidate->setProperty ("branch_ref", branch);
        candidate->setProperty ("worktree_ref", worktree);
        candidate->setProperty ("project_revision", "project-r17");
        candidate->setProperty ("owner_agent_id", juce::String ("agent:") + id);
        candidate->setProperty ("reservation_id", juce::String ("reservation:") + id);
        candidate->setProperty ("artifact_ref", juce::String ("artifact:") + id);
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
    assert (preparedA->getProperty ("engineering_source_kind").toString() == "worktree");
    assert (preparedA->getProperty ("engineering_source_ref").toString() == "worktree:worktree:a");
    assert (preparedA->getProperty ("owner_agent_id").toString() == "agent:candidate-a");
    assert (preparedA->getProperty ("reservation_id").toString() == "reservation:candidate-a");
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
