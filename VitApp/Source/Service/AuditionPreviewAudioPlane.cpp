#include "AuditionPreviewAudioPlane.h"

#include <algorithm>
#include <cmath>
#include <limits>
#include <utility>

namespace vit
{

namespace
{

constexpr double fallbackSampleRate = 44100.0;
constexpr int fallbackBlockSize = 512;
constexpr int crossfadeSamples = 128;

struct CandidateBuffer
{
    std::string id;
    std::string previewRef;
    std::string previewRevision;
    std::shared_ptr<const juce::AudioBuffer<float>> audio;
    double sampleRate = 0.0;
    std::int64_t numSamples = 0;
};

float sampleAt (const CandidateBuffer& candidate, std::int64_t position, int channel)
{
    if (candidate.audio == nullptr || candidate.numSamples <= 0)
        return 0.0f;

    if (position < 0 || position >= candidate.numSamples)
        return 0.0f;
    const auto& audio = *candidate.audio;
    const auto sourceChannel = std::min (channel, std::max (0, audio.getNumChannels() - 1));
    return audio.getSample (sourceChannel, static_cast<int> (position));
}

} // namespace

struct AuditionPreviewAudioPlane::RuntimeSession
{
    std::string id;
    CandidateBuffer candidates[2];
    double sampleRate = fallbackSampleRate;
    std::atomic<int> desiredCandidate { 0 };
    std::atomic<std::int64_t> requestedPositionSamples { 0 };
    std::atomic<std::uint64_t> positionRevision { 0 };
    std::atomic<bool> playing { false };
    std::atomic<bool> previewActive { false };
};

class AuditionPreviewAudioPlane::OutputProcessor final : public juce::AudioProcessor
{
public:
    explicit OutputProcessor (AuditionPreviewAudioPlane& owner)
        : ownerRef (owner)
    {
    }

    const juce::String getName() const override { return "Vit Audition Preview Output"; }
    void prepareToPlay (double sampleRate, int samplesPerBlock) override
    {
        currentSampleRate = sampleRate > 0.0 ? sampleRate : fallbackSampleRate;
        currentBlockSize = samplesPerBlock > 0 ? samplesPerBlock : fallbackBlockSize;
    }
    void releaseResources() override {}
    bool isBusesLayoutSupported (const BusesLayout& layouts) const override
    {
        return layouts.getMainOutputChannelSet() != juce::AudioChannelSet::disabled();
    }
    void processBlock (juce::AudioBuffer<float>& buffer, juce::MidiBuffer&) override
    {
        process (buffer);
    }
    void processBlockBypassed (juce::AudioBuffer<float>& buffer, juce::MidiBuffer&) override
    {
        process (buffer);
    }
    double getTailLengthSeconds() const override { return 0.0; }
    bool acceptsMidi() const override { return false; }
    bool producesMidi() const override { return false; }
    bool isMidiEffect() const override { return false; }
    juce::AudioProcessorEditor* createEditor() override { return nullptr; }
    bool hasEditor() const override { return false; }
    int getNumPrograms() override { return 1; }
    int getCurrentProgram() override { return 0; }
    void setCurrentProgram (int) override {}
    const juce::String getProgramName (int) override { return {}; }
    void changeProgramName (int, const juce::String&) override {}
    void getStateInformation (juce::MemoryBlock&) override {}
    void setStateInformation (const void*, int) override {}

    AuditionPreviewAudioPlane::BlockEvidence renderTestBlock (const std::shared_ptr<RuntimeSession>& session,
                                                              const std::string& requestedCandidate,
                                                              double positionSeconds,
                                                              int numSamples)
    {
        if (session == nullptr)
            return {};

        session->desiredCandidate.store (requestedCandidate == session->candidates[1].id ? 1 : 0,
                                         std::memory_order_release);
        session->requestedPositionSamples.store (static_cast<std::int64_t> (std::llround (positionSeconds * currentSampleRate)),
                                                  std::memory_order_release);
        session->positionRevision.fetch_add (1, std::memory_order_release);
        session->playing.store (true, std::memory_order_release);
        session->previewActive.store (true, std::memory_order_release);

        juce::AudioBuffer<float> buffer (2, std::max (1, numSamples));
        buffer.clear();
        juce::MidiBuffer midi;
        activeSessionOverride = session;
        process (buffer);
        activeSessionOverride.reset();

        BlockEvidence evidence;
        evidence.previewActive = true;
        evidence.sessionId = session->id;
        evidence.candidateId = requestedCandidate;
        evidence.startPositionSamples = lastBlockStart;
        evidence.endPositionSamples = lastBlockEnd;
        evidence.firstSample = buffer.getSample (0, 0);
        evidence.crossfadeApplied = lastBlockCrossfade;
        double sum = 0.0;
        for (int sample = 0; sample < buffer.getNumSamples(); ++sample)
            sum += static_cast<double> (buffer.getSample (0, sample)) * buffer.getSample (0, sample);
        evidence.rms = std::sqrt (sum / std::max (1, buffer.getNumSamples()));
        return evidence;
    }

private:
    void process (juce::AudioBuffer<float>& buffer)
    {
        auto session = activeSessionOverride != nullptr
            ? activeSessionOverride
            : ownerRef.activeSession.load (std::memory_order_acquire);

        if (session == nullptr || ! session->previewActive.load (std::memory_order_acquire)
            || ! session->playing.load (std::memory_order_acquire))
            return;

        const auto desired = std::clamp (session->desiredCandidate.load (std::memory_order_acquire), 0, 1);
        const auto requested = session->requestedPositionSamples.load (std::memory_order_acquire);
        const auto requestedRevision = session->positionRevision.load (std::memory_order_acquire);
        if (lastRuntimeSession != session.get())
        {
            lastRuntimeSession = session.get();
            currentCandidate = desired;
            fadeFrom = desired;
            fadeTo = desired;
            fadeRemaining = 0;
            positionSamples = requested;
            lastPositionRevision = requestedRevision;
        }

        if (requestedRevision != lastPositionRevision)
        {
            positionSamples = requested;
            lastPositionRevision = requestedRevision;
        }

        lastBlockCrossfade = fadeRemaining > 0;
        if (desired != currentCandidate && fadeRemaining <= 0)
        {
            lastBlockCrossfade = true;
            fadeFrom = currentCandidate;
            fadeTo = desired;
            fadeRemaining = crossfadeSamples;
        }

        const auto startPosition = positionSamples;
        ownerRef.audioBlocksRendered.fetch_add (1, std::memory_order_relaxed);
        buffer.clear();
        for (int sample = 0; sample < buffer.getNumSamples(); ++sample)
        {
            const auto isCrossfading = fadeRemaining > 0;
            const auto progress = isCrossfading
                ? 1.0f - static_cast<float> (fadeRemaining) / static_cast<float> (crossfadeSamples)
                : 1.0f;
            for (int channel = 0; channel < buffer.getNumChannels(); ++channel)
            {
                const auto value = isCrossfading
                    ? sampleAt (session->candidates[fadeFrom], positionSamples, channel) * (1.0f - progress)
                        + sampleAt (session->candidates[fadeTo], positionSamples, channel) * progress
                    : sampleAt (session->candidates[currentCandidate], positionSamples, channel);
                buffer.setSample (channel, sample, value);
            }
            if (isCrossfading)
            {
                --fadeRemaining;
                if (fadeRemaining == 0)
                    currentCandidate = fadeTo;
            }
            ++positionSamples;
        }

        lastBlockStart = startPosition;
        lastBlockEnd = positionSamples;
        session->requestedPositionSamples.store (positionSamples, std::memory_order_release);
        if (positionSamples >= session->candidates[currentCandidate].numSamples)
        {
            session->playing.store (false, std::memory_order_release);
            session->previewActive.store (false, std::memory_order_release);
        }
    }

    AuditionPreviewAudioPlane& ownerRef;
    double currentSampleRate = fallbackSampleRate;
    int currentBlockSize = fallbackBlockSize;
    std::shared_ptr<RuntimeSession> activeSessionOverride;
    RuntimeSession* lastRuntimeSession = nullptr;
    int currentCandidate = 0;
    int fadeFrom = 0;
    int fadeTo = 0;
    int fadeRemaining = 0;
    std::int64_t positionSamples = 0;
    std::uint64_t lastPositionRevision = 0;
    std::int64_t lastBlockStart = 0;
    std::int64_t lastBlockEnd = 0;
    bool lastBlockCrossfade = false;
};

AuditionPreviewAudioPlane::AuditionPreviewAudioPlane (te::Engine* engineToUse, EditGetter editGetter)
    : engine (engineToUse),
      getEdit (std::move (editGetter))
{
    outputProcessorOwner = std::make_unique<OutputProcessor> (*this);
    outputProcessor = outputProcessorOwner.get();
    installOutputProcessor();
}

AuditionPreviewAudioPlane::~AuditionPreviewAudioPlane()
{
    uninstallOutputProcessor();
}

std::string AuditionPreviewAudioPlane::normaliseSourcePath (const std::string& sourceRef)
{
    constexpr const char* filePrefix = "file://";
    if (sourceRef.rfind (filePrefix, 0) == 0)
        return sourceRef.substr (std::char_traits<char>::length (filePrefix));
    return sourceRef;
}

std::shared_ptr<juce::AudioBuffer<float>> AuditionPreviewAudioPlane::decodeAndResample (const juce::File& file,
                                                                                         double targetSampleRate,
                                                                                         std::string& error)
{
    juce::AudioFormatManager formats;
    formats.registerBasicFormats();
    std::unique_ptr<juce::AudioFormatReader> reader (formats.createReaderFor (file));
    if (reader == nullptr)
    {
        error = "candidate_audio_unreadable";
        return {};
    }

    const auto sourceLength = static_cast<std::int64_t> (reader->lengthInSamples);
    const auto sourceChannels = static_cast<int> (reader->numChannels);
    if (sourceLength <= 0 || sourceChannels <= 0 || reader->sampleRate <= 0.0)
    {
        error = "candidate_audio_empty";
        return {};
    }

    juce::AudioBuffer<float> source (sourceChannels, static_cast<int> (sourceLength));
    source.clear();
    if (! reader->read (&source, 0, static_cast<int> (sourceLength), 0, true, true))
    {
        error = "candidate_audio_read_failed";
        return {};
    }

    const auto outputLength = static_cast<std::int64_t> (std::max (1.0,
        std::ceil (sourceLength * targetSampleRate / reader->sampleRate)));
    auto output = std::make_shared<juce::AudioBuffer<float>> (sourceChannels, static_cast<int> (outputLength));
    for (int channel = 0; channel < sourceChannels; ++channel)
    {
        auto* destination = output->getWritePointer (channel);
        const auto* input = source.getReadPointer (channel);
        for (std::int64_t i = 0; i < outputLength; ++i)
        {
            const auto sourcePosition = static_cast<double> (i) * reader->sampleRate / targetSampleRate;
            const auto left = std::clamp<std::int64_t> (static_cast<std::int64_t> (sourcePosition), 0, sourceLength - 1);
            const auto right = std::min<std::int64_t> (left + 1, sourceLength - 1);
            const auto fraction = static_cast<float> (sourcePosition - static_cast<double> (left));
            destination[i] = input[left] + (input[right] - input[left]) * fraction;
        }
    }
    return output;
}

std::string AuditionPreviewAudioPlane::makePreviewRef (const std::string& sessionId,
                                                       const std::string& candidateId,
                                                       const juce::File& source)
{
    return "audio-buffer://" + sessionId + "/" + candidateId + "/" + source.getFileName().toStdString();
}

std::string AuditionPreviewAudioPlane::makePreviewRevision (const juce::File& source,
                                                            double sampleRate,
                                                            int channelCount,
                                                            std::int64_t numSamples)
{
    return source.getFullPathName().toStdString() + "|" + std::to_string (source.getSize()) + "|"
         + std::to_string (source.getLastModificationTime().toMilliseconds()) + "|"
         + std::to_string (sampleRate) + "|" + std::to_string (channelCount) + "|" + std::to_string (numSamples);
}

AuditionPreviewAudioPlane::PrepareResult AuditionPreviewAudioPlane::prepare (const audition::Session& session)
{
    PrepareResult result;
    if (session.scope != "target")
    {
        result.code = "preview_scope_unsupported";
        result.message = "real audio spike currently supports target scope only";
        return result;
    }

    const auto sampleRate = engine != nullptr && engine->getDeviceManager().getSampleRate() > 0.0
        ? engine->getDeviceManager().getSampleRate() : fallbackSampleRate;
    auto runtime = std::make_shared<RuntimeSession>();
    runtime->id = session.id;
    runtime->sampleRate = sampleRate;

    for (std::size_t i = 0; i < session.candidates.size() && i < 2; ++i)
    {
        const auto& source = session.candidates[i];
        if (source.sourceKind != "audio_file")
        {
            result.code = "candidate_source_unsupported";
            result.message = "real audio spike requires source_kind=audio_file for target candidates";
            return result;
        }

        const juce::File sourceFile (juce::String (normaliseSourcePath (source.sourceRef)));
        if (! sourceFile.existsAsFile())
        {
            result.code = "candidate_audio_missing";
            result.message = "candidate source file does not exist: " + sourceFile.getFullPathName().toStdString();
            return result;
        }

        std::string decodeError;
        auto decoded = decodeAndResample (sourceFile, sampleRate, decodeError);
        if (decoded == nullptr)
        {
            result.code = decodeError;
            result.message = "unable to prepare candidate audio: " + sourceFile.getFullPathName().toStdString();
            return result;
        }

        auto& prepared = runtime->candidates[i];
        prepared.id = source.id;
        prepared.audio = std::static_pointer_cast<const juce::AudioBuffer<float>> (decoded);
        prepared.sampleRate = sampleRate;
        prepared.numSamples = decoded->getNumSamples();
        prepared.previewRef = makePreviewRef (session.id, source.id, sourceFile);
        prepared.previewRevision = makePreviewRevision (sourceFile, sampleRate, decoded->getNumChannels(), prepared.numSamples);
        candidatesDecoded.fetch_add (1, std::memory_order_relaxed);
        result.candidates.push_back ({ prepared.id, prepared.previewRef, prepared.previewRevision,
                                       prepared.numSamples / sampleRate, sampleRate,
                                       decoded->getNumChannels(), prepared.numSamples });
    }

    if (result.candidates.size() != 2)
    {
        result.code = "validation_error";
        result.message = "audition.prepare requires exactly two audio candidates";
        return result;
    }

    std::lock_guard lock (sessionsMutex);
    sessions[session.id] = runtime;
    result.ok = true;
    result.message = "candidate audio prepared";
    return result;
}

std::shared_ptr<AuditionPreviewAudioPlane::RuntimeSession> AuditionPreviewAudioPlane::findSession (const std::string& sessionId) const
{
    std::lock_guard lock (sessionsMutex);
    const auto found = sessions.find (sessionId);
    return found == sessions.end() ? nullptr : found->second;
}

bool AuditionPreviewAudioPlane::isCandidatePrepared (const std::string& sessionId, const std::string& candidateId) const
{
    const auto session = findSession (sessionId);
    return session != nullptr && (session->candidates[0].id == candidateId || session->candidates[1].id == candidateId)
        && (session->candidates[0].id == candidateId ? session->candidates[0].audio : session->candidates[1].audio) != nullptr;
}

std::string AuditionPreviewAudioPlane::preparedPreviewRef (const std::string& sessionId, const std::string& candidateId) const
{
    const auto session = findSession (sessionId);
    if (session == nullptr)
        return {};
    for (const auto& candidate : session->candidates)
        if (candidate.id == candidateId)
            return candidate.previewRef;
    return {};
}

AuditionPreviewAudioPlane::PlaybackSnapshot AuditionPreviewAudioPlane::snapshot (const std::string& sessionId) const
{
    PlaybackSnapshot result;
    const auto session = findSession (sessionId);
    if (session == nullptr)
        return result;
    const auto index = std::clamp (session->desiredCandidate.load (std::memory_order_acquire), 0, 1);
    result.prepared = session->candidates[0].audio != nullptr && session->candidates[1].audio != nullptr;
    result.previewActive = session->previewActive.load (std::memory_order_acquire);
    result.isPlaying = session->playing.load (std::memory_order_acquire);
    result.candidateId = session->candidates[index].id;
    result.sampleRate = session->sampleRate;
    result.positionSeconds = session->sampleRate > 0.0
        ? session->requestedPositionSamples.load (std::memory_order_acquire) / session->sampleRate : 0.0;
    return result;
}

bool AuditionPreviewAudioPlane::select (const std::string& sessionId,
                                        const std::string& candidateId,
                                        double positionSeconds,
                                        bool isPlaying)
{
    const auto session = findSession (sessionId);
    if (session == nullptr || ! isCandidatePrepared (sessionId, candidateId))
        return false;
    const auto index = session->candidates[1].id == candidateId ? 1 : 0;
    sourceSwitchRequests.fetch_add (1, std::memory_order_relaxed);
    session->desiredCandidate.store (index, std::memory_order_release);
    session->requestedPositionSamples.store (static_cast<std::int64_t> (std::llround (positionSeconds * session->sampleRate)), std::memory_order_release);
    session->positionRevision.fetch_add (1, std::memory_order_release);
    session->playing.store (isPlaying, std::memory_order_release);
    session->previewActive.store (isPlaying, std::memory_order_release);
    activeSession.store (session, std::memory_order_release);
    return true;
}

bool AuditionPreviewAudioPlane::position (const std::string& sessionId,
                                          std::optional<double> positionSeconds,
                                          std::optional<bool> isPlaying)
{
    const auto session = findSession (sessionId);
    if (session == nullptr)
        return false;
    if (positionSeconds.has_value())
    {
        session->requestedPositionSamples.store (static_cast<std::int64_t> (std::llround (*positionSeconds * session->sampleRate)), std::memory_order_release);
        session->positionRevision.fetch_add (1, std::memory_order_release);
    }
    if (isPlaying.has_value())
    {
        session->playing.store (*isPlaying, std::memory_order_release);
        session->previewActive.store (*isPlaying, std::memory_order_release);
    }
    activeSession.store (session, std::memory_order_release);
    return true;
}

bool AuditionPreviewAudioPlane::stop (const std::string& sessionId)
{
    const auto session = findSession (sessionId);
    if (session == nullptr)
        return false;
    session->playing.store (false, std::memory_order_release);
    session->previewActive.store (false, std::memory_order_release);
    return true;
}

void AuditionPreviewAudioPlane::invalidate (const std::string& sessionId)
{
    const auto session = findSession (sessionId);
    if (session != nullptr)
    {
        session->playing.store (false, std::memory_order_release);
        session->previewActive.store (false, std::memory_order_release);
    }
    auto current = activeSession.load (std::memory_order_acquire);
    if (current != nullptr && current->id == sessionId)
        activeSession.store (std::shared_ptr<RuntimeSession> {}, std::memory_order_release);
}

AuditionPreviewAudioPlane::BlockEvidence AuditionPreviewAudioPlane::renderTestBlock (const std::string& sessionId,
                                                                                     const std::string& candidateId,
                                                                                     double positionSeconds,
                                                                                     int numSamples)
{
    auto session = findSession (sessionId);
    if (session == nullptr || outputProcessor == nullptr)
        return {};
    return outputProcessor->renderTestBlock (session, candidateId, positionSeconds, numSamples);
}

AuditionPreviewAudioPlane::Diagnostics AuditionPreviewAudioPlane::diagnostics() const noexcept
{
    Diagnostics result;
    result.candidatesDecoded = candidatesDecoded.load (std::memory_order_relaxed);
    result.sourceSwitchRequests = sourceSwitchRequests.load (std::memory_order_relaxed);
    result.audioBlocksRendered = audioBlocksRendered.load (std::memory_order_relaxed);
    return result;
}

bool AuditionPreviewAudioPlane::installOutputProcessor()
{
    if (engine == nullptr)
        return false;
    auto& devices = engine->getDeviceManager();
    if (devices.getGlobalOutputAudioProcessor() != nullptr)
    {
        juce::Logger::writeToLog ("AuditionPreviewAudioPlane: global output processor already occupied; audio plane disabled.");
        return false;
    }
    devices.setGlobalOutputAudioProcessor (std::move (outputProcessorOwner));
    ownsOutputProcessor = true;
    return true;
}

void AuditionPreviewAudioPlane::uninstallOutputProcessor()
{
    if (engine != nullptr && ownsOutputProcessor)
    {
        engine->getDeviceManager().setGlobalOutputAudioProcessor (nullptr);
        outputProcessor = nullptr;
        ownsOutputProcessor = false;
    }
}

} // namespace vit
