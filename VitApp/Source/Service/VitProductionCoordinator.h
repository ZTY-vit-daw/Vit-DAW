#pragma once

#include <JuceHeader.h>
#include <atomic>
#include <functional>
#include <memory>
#include <utility>
#include <vector>
#include <tracktion_engine/tracktion_engine.h>

#include "CompressorDualTapEvidence.h"

namespace vit
{

namespace te = tracktion;

/**
 * Offline render job + guard flag used by CommandDispatcher (render lock).
 */
class VitProductionCoordinator final
{
public:
    using PublishFn = std::function<void (const juce::String&)>;

    struct L2RenderProbeRequest
    {
        bool enabled = false;
        juce::String requestId;
        juce::String trackId;
        juce::String clipId;
        juce::String sourcePath;
        juce::String sourceRevision;
        juce::String clipRevision;
        juce::String renderRevision;
        juce::String tapPoint = "track_post_fader";
        juce::String renderMode = "offline_probe";
        double analyzedStartSeconds = 0.0;
        double analyzedEndSeconds = 0.0;
        double tailSeconds = 0.25;
        double analysisBandLowHz = 0.0;
        double analysisBandHighHz = 0.0;
        juce::String analysisBandId;
        bool deterministic = true;
        bool latencyCompensated = true;
        bool tailCaptured = true;
    };

    struct CompressorDualTapProbeRequest
    {
        CompressorDualTapEvidenceRequest evidence;
        juce::File inputRenderFile;
        juce::File outputRenderFile;
        juce::File outputVerificationRenderFile;
        juce::File artifactDirectory;
        juce::BigInteger tracksToDo;
        std::function<juce::String()> readCurrentScopeRevision;
    };

    explicit VitProductionCoordinator (PublishFn publish);

    bool isRendering() const noexcept { return rendering.load(); }

    juce::String startOfflineRender (te::Edit& edit,
                                     const juce::File& destFile,
                                     double rangeStartSeconds,
                                     double rangeEndSeconds,
                                     int bitDepth,
                                     bool useMasterPlugins,
                                     const juce::BigInteger& tracksToDo = {},
                                     L2RenderProbeRequest probeRequest = {});

    juce::String startCompressorDualTapProbe (te::Edit& edit,
                                              CompressorDualTapProbeRequest request);

    void cancelOfflineRender();

    /** Call from message thread (e.g. VitHeadlessService timer). Publishes render_progress. */
    void tick();

    /**
     * Test-only hook (VitApp/Tests/RenderWatchdogTests): simulate a wedged
     * offline render — rendering flag set, watchdog armed, no engine handle,
     * and no completion callback ever arriving.
     */
    void simulateWedgedRenderForTest (double watchdogTimeoutSeconds);

private:
    PublishFn publishMessage;
    std::atomic<bool> rendering { false };
    juce::String jobId;
    std::shared_ptr<te::EditRenderer::Handle> renderHandle;
    float lastPublishedProgress = -1.0f;

    // Render watchdog (KERNEL-RENDER-1): steady-clock deadline in ms for the
    // current render job; 0 = disarmed. Message thread only in practice.
    std::atomic<int64_t> renderWatchdogDeadlineMs { 0 };
    // Handles of timed-out (wedged) renders. They are parked, never destroyed:
    // ~Handle joins the render thread, which would hang the message thread on a
    // blocked render. Cleared when the stale completion callback finally runs.
    std::vector<std::pair<juce::String, std::shared_ptr<te::EditRenderer::Handle>>> wedgedRenderHandles;

    void armRenderWatchdog (double timeoutSeconds);
    void checkRenderWatchdog();
    void releaseWedgedRenderHandle (const juce::String& handleJobId);

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitProductionCoordinator)
};

} // namespace vit
