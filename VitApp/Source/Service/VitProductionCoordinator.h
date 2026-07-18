#pragma once

#include <JuceHeader.h>
#include <atomic>
#include <functional>
#include <memory>
#include <tracktion_engine/tracktion_engine.h>

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

    void cancelOfflineRender();

    /** Call from message thread (e.g. VitHeadlessService timer). Publishes render_progress. */
    void tick();

private:
    PublishFn publishMessage;
    std::atomic<bool> rendering { false };
    juce::String jobId;
    std::shared_ptr<te::EditRenderer::Handle> renderHandle;
    float lastPublishedProgress = -1.0f;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitProductionCoordinator)
};

} // namespace vit
