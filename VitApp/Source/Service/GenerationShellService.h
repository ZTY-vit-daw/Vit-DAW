#pragma once

#include <functional>
#include <memory>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

// GENSHELL-HOST-1: kernel-side generation shell service (件五卡2).
//
// Owns the submit queue, the GenShell instance registry and the downlink
// mirror. The T2 transport conclusion (verified on the live JUCE stack) is
// 降级 A (件一 §2): the raw controller IConnectionPoint is unreachable from
// te::ExternalPlugin/JUCE public surface — both plugin endpoints are occupied
// by JUCE's processor↔controller interconnect (single-peer, fail-closed
// connect) and the live IEditController* has no public accessor. Queue
// semantics are unchanged: submits land as queued jobs with ghost placement,
// gen_shell.pending serves jobs+providers, and job_progress/job_terminal ride
// the T1 parameter mirror (JobState/JobProgress/JobErrorClass).
class GenerationShellService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using CurrentProjectPathGetter = std::function<juce::String()>;

    GenerationShellService (EditGetter editGetter,
                            CurrentProjectPathGetter currentProjectPathGetter);
    ~GenerationShellService();

    // gen_shell.pending — the only new kernel command (件二 §4):
    //   answer { status, jobs: [{job_id, state, cancel_requested, request}],
    //            providers: [] }
    // Providers are empty in v1: the kernel has no genapi provider knowledge;
    // the agent runner (卡3) knows providers natively.
    // Smoke-only debug surface (absent in production payloads):
    //   debug_shell_state=true  -> adds "shells" registry/probe section
    //   debug_submit={request}  -> injects a submit through the internal path
    //   debug_cancel={job_id}   -> injects a cancel through the internal path
    juce::String handleGenShellPending (const juce::DynamicObject& object, const juce::String& raw);

private:
    struct Impl;
    std::unique_ptr<Impl> impl;
};

} // namespace vit
