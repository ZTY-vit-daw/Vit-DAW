#pragma once

#include <atomic>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <optional>
#include <string>
#include <unordered_map>
#include <vector>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "AuditionPreviewState.h"

namespace vit
{

namespace te = tracktion;

/**
    Kernel-owned target-scope audio plane for the G4 gap closure.

    Preparation is message-thread work: source files are decoded and resampled
    into immutable, Kernel-owned buffers. Playback is audio-thread work through
    Tracktion's DeviceManager global output processor. Select/position/stop
    publish only atomics and never touch the active Edit or project lifecycle.
*/
class AuditionPreviewAudioPlane final
{
public:
    using EditGetter = std::function<te::Edit*()>;

    struct PreparedCandidate
    {
        std::string id;
        std::string previewRef;
        std::string previewRevision;
        double durationSeconds = 0.0;
        double sampleRate = 0.0;
        int channelCount = 0;
        std::int64_t numSamples = 0;
    };

    struct PrepareResult
    {
        bool ok = false;
        std::string code;
        std::string message;
        std::vector<PreparedCandidate> candidates;
    };

    struct PlaybackSnapshot
    {
        bool prepared = false;
        bool previewActive = false;
        bool isPlaying = false;
        std::string candidateId;
        double positionSeconds = 0.0;
        double sampleRate = 0.0;
        int crossfadeLengthSamples = 128;
    };

    struct Diagnostics
    {
        std::uint64_t candidatesDecoded = 0;
        std::uint64_t sourceSwitchRequests = 0;
        std::uint64_t audioBlocksRendered = 0;
        std::uint64_t renderRequests = 0;
        std::uint64_t checkoutRequests = 0;
        std::uint64_t projectOpenRequests = 0;
        std::uint64_t kernelReloadRequests = 0;
    };

    struct BlockEvidence
    {
        bool previewActive = false;
        std::string sessionId;
        std::string candidateId;
        std::int64_t startPositionSamples = 0;
        std::int64_t endPositionSamples = 0;
        double rms = 0.0;
        float firstSample = 0.0f;
        bool crossfadeApplied = false;
    };

    AuditionPreviewAudioPlane (te::Engine* engine, EditGetter editGetter = {});
    ~AuditionPreviewAudioPlane();

    PrepareResult prepare (const audition::Session& session);
    bool isCandidatePrepared (const std::string& sessionId, const std::string& candidateId) const;
    std::string preparedPreviewRef (const std::string& sessionId, const std::string& candidateId) const;
    PlaybackSnapshot snapshot (const std::string& sessionId) const;

    bool select (const std::string& sessionId,
                 const std::string& candidateId,
                 double positionSeconds,
                 bool isPlaying);
    bool position (const std::string& sessionId,
                   std::optional<double> positionSeconds,
                   std::optional<bool> isPlaying);
    bool stop (const std::string& sessionId);
    void invalidate (const std::string& sessionId);

    // Uses the exact same processor path installed into DeviceManager. This is
    // intentionally public so deterministic C++ tests can prove source change
    // without requiring a physical audio device.
    BlockEvidence renderTestBlock (const std::string& sessionId,
                                   const std::string& candidateId,
                                   double positionSeconds,
                                   int numSamples = 256);

    bool ownsDeviceManagerOutputProcessor() const noexcept { return ownsOutputProcessor; }
    Diagnostics diagnostics() const noexcept;

private:
    struct RuntimeSession;
    class OutputProcessor;

    static std::string normaliseSourcePath (const std::string& sourceRef);
    static std::shared_ptr<juce::AudioBuffer<float>> decodeAndResample (const juce::File& file,
                                                                         double targetSampleRate,
                                                                         std::string& error);
    static std::string makePreviewRef (const std::string& sessionId,
                                       const std::string& candidateId,
                                       double sampleRate,
                                       int channelCount);
    static std::string makePreviewRevision (const juce::File& source,
                                             double sampleRate,
                                             int channelCount,
                                             std::int64_t numSamples);

    std::shared_ptr<RuntimeSession> findSession (const std::string& sessionId) const;
    bool installOutputProcessor();
    void uninstallOutputProcessor();

    te::Engine* engine = nullptr;
    EditGetter getEdit;
    std::unique_ptr<OutputProcessor> outputProcessorOwner;
    OutputProcessor* outputProcessor = nullptr;
    bool ownsOutputProcessor = false;

    mutable std::mutex sessionsMutex;
    std::unordered_map<std::string, std::shared_ptr<RuntimeSession>> sessions;
    std::atomic<std::shared_ptr<RuntimeSession>> activeSession;
    std::atomic<std::uint64_t> candidatesDecoded { 0 };
    std::atomic<std::uint64_t> sourceSwitchRequests { 0 };
    std::atomic<std::uint64_t> audioBlocksRendered { 0 };
};

} // namespace vit
