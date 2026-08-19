#include "../Source/Service/AuditionPreviewState.h"

#include <cassert>
#include <cmath>
#include <string>

namespace
{

vit::audition::Session makeSession()
{
    vit::audition::Session session;
    session.id = "audition-test";
    session.scope = "target";
    session.activeProjectRef = "project:active";
    session.activeProjectRevision = "project-r17";
    session.transport.timelineRevision = "timeline-r17";
    session.candidates = {
        { "candidate-a", "Candidate A", "edit", "edit:a", {}, {} },
        { "candidate-b", "Candidate B", "edit", "edit:b", {}, {} },
    };
    return session;
}

void requireOk (const vit::audition::Result& result)
{
    assert (result.ok);
    assert (result.session.has_value());
}

} // namespace

int main()
{
    vit::audition::StateMachine machine;

    const auto prepared = machine.prepare (makeSession());
    requireOk (prepared);
    assert (prepared.session->status == vit::audition::SessionStatus::preparing);
    assert (prepared.message == "audition.prepare.started");

    const auto beforeReadySelect = machine.select ("audition-test", "candidate-a");
    assert (! beforeReadySelect.ok);
    assert (beforeReadySelect.code == "audition_not_ready");

    requireOk (machine.markCandidateReady ("audition-test", "candidate-a", "preview:a"));
    const auto oneReady = machine.status ("audition-test");
    requireOk (oneReady);
    assert (oneReady.session->status == vit::audition::SessionStatus::preparing);

    const auto allReady = machine.markCandidateReady ("audition-test", "candidate-b", "preview:b");
    requireOk (allReady);
    assert (allReady.session->status == vit::audition::SessionStatus::ready);
    assert (allReady.message == "audition.ready");

    const auto selected = machine.select ("audition-test", "candidate-b");
    requireOk (selected);
    assert (selected.session->activeCandidateId == "candidate-b");
    assert (selected.session->activeProjectRef == "project:active");
    assert (selected.session->activeProjectRevision == "project-r17");
    assert (selected.session->transport.timelineRevision == "timeline-r17");

    const auto playing = machine.position ("audition-test", 12.5, true);
    requireOk (playing);
    assert (playing.session->status == vit::audition::SessionStatus::playing);
    assert (std::abs (playing.session->transport.positionSeconds - 12.5) < 1.0e-9);

    const auto stopped = machine.stop ("audition-test");
    requireOk (stopped);
    assert (stopped.session->status == vit::audition::SessionStatus::stopped);
    assert (! stopped.session->transport.isPlaying);
    assert (stopped.session->activeCandidateId == "candidate-b");
    assert (stopped.session->activeProjectRef == "project:active");
    assert (stopped.session->activeProjectRevision == "project-r17");

    const auto positionAfterStop = machine.position ("audition-test", 1.0, std::nullopt);
    assert (! positionAfterStop.ok);
    assert (positionAfterStop.code == "audition_not_ready");

    return 0;
}
