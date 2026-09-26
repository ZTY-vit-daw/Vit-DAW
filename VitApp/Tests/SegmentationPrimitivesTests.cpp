// L2-2-SEG-1: segmentation primitives v1 acceptance tests.
//
// T1 deterministic replay  - the same source analyzed twice must publish a
//    byte-identical primitives payload (only updated_at, a wall-clock stamp,
//    is excluded; every numeric field including all array elements must match).
// T2 structural            - onset event sequence and energy novelty curve are
//    non-empty, aligned to the existing L3 frame grid, and reflect the
//    transients/energy step baked into the test signal.
// T3 not_evaluable         - missing file / zero-sample source must still
//    publish, with explicit not_evaluable sub-states rather than a silent
//    empty or missing payload.
//
// Red state: before the feature exists, no segmentation_primitives payload is
// published for these requests, so analyzeAndWait times out and the test
// exits non-zero.

#include "../Source/Service/L3AcousticAnalyzer.h"

#include <atomic>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <future>
#include <memory>
#include <string>
#include <vector>

namespace
{

constexpr double kSampleRate = 44100.0;
constexpr double kDurationSeconds = 3.0;
constexpr double kSineHz = 220.0;
constexpr double kBaseLevelFirstHalf = 0.05;
constexpr double kBaseLevelSecondHalf = 0.12;
constexpr double kEnergyStepSeconds = 1.5;
constexpr double kPulseStartSeconds = 0.15;
constexpr double kPulseSpacingSeconds = 0.30;
constexpr double kPulseDecayTauSeconds = 0.008;
constexpr double kPulsePeakAmplitude = 0.90;
constexpr int kAnalysisWaitTimeoutSeconds = 30;

int gFailures = 0;

void check (bool condition, const std::string& description)
{
    if (condition)
    {
        std::printf ("[segprims] PASS %s\n", description.c_str());
        std::fflush (stdout);
        return;
    }
    ++gFailures;
    std::printf ("[segprims] FAIL %s\n", description.c_str());
    std::fflush (stdout);
}

class CapturingLogger final : public juce::Logger
{
public:
    void logMessage (const juce::String& message) override
    {
        (void) message;
    }
};

juce::File testWorkspaceDirectory()
{
    const auto directory = juce::File::getSpecialLocation (juce::File::tempDirectory)
                               .getChildFile ("Vit_DAW_SegmentationPrimitivesTests");
    if (directory.exists())
        directory.deleteRecursively();
    directory.createDirectory();
    return directory;
}

// Deterministic test signal: quiet 220 Hz sine bed that steps up in energy at
// 1.5 s, plus a decaying transient pulse every 0.3 s. Integer sample loops and
// standard libm only, so the written bytes are reproducible.
bool writeTestWav (const juce::File& file, int64 sampleCount)
{
    juce::AudioBuffer<float> buffer (2, (int) sampleCount);
    for (int i = 0; i < (int) sampleCount; ++i)
    {
        const double t = (double) i / kSampleRate;
        const double bed = t < kEnergyStepSeconds ? kBaseLevelFirstHalf : kBaseLevelSecondHalf;
        double sample = bed * std::sin (juce::MathConstants<double>::twoPi * kSineHz * t);
        const double sincePulse = std::fmod (t - kPulseStartSeconds, kPulseSpacingSeconds);
        if (t >= kPulseStartSeconds && sincePulse >= 0.0 && sincePulse < 4.0 * kPulseDecayTauSeconds)
            sample += kPulsePeakAmplitude * std::exp (-sincePulse / kPulseDecayTauSeconds);
        const float value = (float) juce::jlimit (-1.0, 1.0, sample);
        buffer.setSample (0, i, value);
        buffer.setSample (1, i, value);
    }

    juce::WavAudioFormat wavFormat;
    std::unique_ptr<juce::OutputStream> stream (file.createOutputStream());
    if (stream == nullptr)
        return false;
    std::unique_ptr<juce::AudioFormatWriter> writer (
        wavFormat.createWriterFor (stream.release(), kSampleRate, 2, 16, {}, 0));
    if (writer == nullptr)
        return false;
    writer->writeFromAudioSampleBuffer (buffer, 0, (int) sampleCount);
    return true;
}

// L3AcousticAnalyzer publishes from its serialized worker pool; block until the
// payload arrives or the red-state timeout expires (no producer => timeout).
juce::String analyzeAndWait (const vit::AudioFeatureBakeRequest& request)
{
    auto promise = std::make_shared<std::promise<juce::String>>();
    auto future = promise->get_future();
    vit::L3AcousticAnalyzer::startAnalyze (request,
        [promise] (const juce::String& payload)
        {
            promise->set_value (payload);
        });
    if (future.wait_for (std::chrono::seconds (kAnalysisWaitTimeoutSeconds)) != std::future_status::ready)
        return {};
    return future.get();
}

vit::AudioFeatureBakeRequest makeRequest (const juce::String& filePath)
{
    vit::AudioFeatureBakeRequest request;
    request.projectId = "segprims-test-project";
    request.filePath = filePath;
    request.trackId = "segprims-test-track";
    request.sourceId = "segprims-test-source";
    request.sourceRevision = "segprims-test-rev-1";
    request.featureType = vit::AudioFeatureType::SegmentationPrimitives;
    request.priority = vit::AudioFeaturePriority::OnDemand;
    return request;
}

// Hold the parsed var in the caller's scope: the DynamicObject it owns is
// reference-counted by the var, so returning a bare pointer from a local var
// would dangle.
juce::var parsedPayload (const juce::String& payload)
{
    return juce::JSON::parse (payload);
}

// Canonical form for the byte-equality replay check: everything except the
// wall-clock updated_at stamp.
juce::String canonicalPayload (const juce::String& payload)
{
    auto parsed = juce::JSON::parse (payload);
    if (auto* object = parsed.getDynamicObject())
        object->removeProperty ("updated_at");
    return juce::JSON::toString (parsed, true);
}

const juce::DynamicObject* subObject (const juce::DynamicObject* parent, const char* name)
{
    if (parent == nullptr)
        return nullptr;
    return parent->getProperty (name).getDynamicObject();
}

} // namespace

int main()
{
    CapturingLogger capturingLogger;
    juce::Logger::setCurrentLogger (&capturingLogger);

    const auto workspace = testWorkspaceDirectory();
    const auto wavFile = workspace.getChildFile ("segprims_signal.wav");
    const auto emptyWavFile = workspace.getChildFile ("segprims_empty.wav");
    const auto missingFile = workspace.getChildFile ("segprims_missing.wav");
    if (! writeTestWav (wavFile, (int64) (kDurationSeconds * kSampleRate))
        || ! writeTestWav (emptyWavFile, 0))
    {
        std::printf ("[segprims] FAIL could not write test wav files under %s\n",
                     workspace.getFullPathName().toStdString().c_str());
        return 1;
    }

    // --- T1: deterministic replay -------------------------------------------------
    const juce::String firstRun = analyzeAndWait (makeRequest (wavFile.getFullPathName()));
    check (firstRun.isNotEmpty(), "T1 first analysis publishes a payload (no producer timeout)");
    const juce::String secondRun = analyzeAndWait (makeRequest (wavFile.getFullPathName()));
    check (secondRun.isNotEmpty(), "T1 second analysis publishes a payload (no producer timeout)");
    if (firstRun.isNotEmpty() && secondRun.isNotEmpty())
    {
        const auto firstCanonical = canonicalPayload (firstRun);
        const auto secondCanonical = canonicalPayload (secondRun);
        const bool identical = firstCanonical == secondCanonical;
        check (identical, "T1 replay payload is byte-identical excluding updated_at");
        if (! identical)
        {
            std::printf ("[segprims] first:  %.400s\n", firstCanonical.toStdString().c_str());
            std::printf ("[segprims] second: %.400s\n", secondCanonical.toStdString().c_str());
        }
    }

    // --- T2: structural contents ---------------------------------------------------
    const auto firstParsed = parsedPayload (firstRun);
    if (const auto* payload = firstParsed.getDynamicObject())
    {
        check (payload->getProperty ("feature_type").toString() == "segmentation_primitives",
               "T2 feature_type is segmentation_primitives");
        check (payload->getProperty ("schema_version").toString() == "dad_l3_segmentation_primitives.v1",
               "T2 schema_version is dad_l3_segmentation_primitives.v1");
        check (payload->getProperty ("feature_version").toString() == "segmentation_primitives.v1",
               "T2 feature_version is segmentation_primitives.v1");
        check ((bool) payload->getProperty ("ready"), "T2 healthy source reports ready=true");

        const auto frameCount = (int) payload->getProperty ("frame_count");
        check (frameCount >= 30, "T2 frame_count covers the full 3 s source");

        const double expectedHopMs = 4096.0 / kSampleRate * 1000.0;
        const double hopMs = payload->getProperty ("hop_ms");
        const double windowMs = payload->getProperty ("window_ms");
        check (std::abs (hopMs - expectedHopMs) < 0.05 && std::abs (windowMs - expectedHopMs) < 0.05,
               "T2 frame grid aligns with the existing L3 non-overlapping FFT hop");

        const auto* onsets = subObject (payload, "onset_events");
        check (onsets != nullptr, "T2 onset_events object present");
        if (onsets != nullptr)
        {
            const auto* onsetList = onsets->getProperty ("events").getArray();
            check (onsetList != nullptr, "T2 onset event array present");
            const int detectedCount = onsets->getProperty ("detected_count");
            check (detectedCount >= 5 && onsetList != nullptr && onsetList->size() >= 5,
                   "T2 onset event sequence is non-empty (>=5 events from 10 baked transients)");
            if (onsetList != nullptr && onsetList->size() > 0)
            {
                const auto* firstOnset = (*onsetList)[0].getDynamicObject();
                const double onsetSeconds = firstOnset->getProperty ("onset_seconds");
                const double riseDb = firstOnset->getProperty ("rise_db");
                check (onsetSeconds > 0.0 && onsetSeconds < kDurationSeconds
                           && riseDb >= 3.0,
                       "T2 onset event carries time and >=3dB rise against the previous frame");
                std::printf ("[segprims] sample onset event: %s\n",
                             juce::JSON::toString ((*onsetList)[0], true).toStdString().c_str());
            }
            check (onsets->getProperty ("status").toString() == "ready", "T2 onset_events status ready");
        }

        const auto* density = subObject (payload, "onset_density");
        check (density != nullptr, "T2 onset_density object present");
        if (density != nullptr)
        {
            const auto* densityFrames = density->getProperty ("frames").getArray();
            check (densityFrames != nullptr && densityFrames->size() == frameCount,
                   "T2 onset density curve has one entry per L3 frame");
            if (densityFrames != nullptr)
            {
                int framesWithOnsets = 0;
                for (const auto& entry : *densityFrames)
                    if (auto* row = entry.getDynamicObject();
                        row != nullptr && (int) row->getProperty ("onset_count") > 0)
                        ++framesWithOnsets;
                check (framesWithOnsets > 0, "T2 onset density curve is non-zero somewhere");
                if (densityFrames->size() > 15)
                    std::printf ("[segprims] sample density frame: %s\n",
                                 juce::JSON::toString ((*densityFrames)[15], true).toStdString().c_str());
            }
        }

        const auto* novelty = subObject (payload, "energy_novelty");
        check (novelty != nullptr, "T2 energy_novelty object present");
        if (novelty != nullptr)
        {
            const auto* noveltyFrames = novelty->getProperty ("frames").getArray();
            check (noveltyFrames != nullptr && noveltyFrames->size() == frameCount,
                   "T2 energy novelty curve has one entry per L3 frame");
            const double maxNovelty = novelty->getProperty ("max_novelty");
            check (maxNovelty > 0.01, "T2 energy novelty reacts to the 1.5 s energy step");
            check (novelty->getProperty ("status").toString() == "ready", "T2 energy_novelty status ready");
            if (noveltyFrames != nullptr && noveltyFrames->size() > 16)
                std::printf ("[segprims] sample novelty frame: %s\n",
                             juce::JSON::toString ((*noveltyFrames)[16], true).toStdString().c_str());
        }
    }
    else
    {
        check (false, "T2 first analysis payload parses as a JSON object");
    }

    // --- T3: missing / empty sources stay explicit ----------------------------------
    const juce::StringArray degenerateSources { missingFile.getFullPathName(), emptyWavFile.getFullPathName() };
    for (const auto& source : degenerateSources)
    {
        const juce::String payload = analyzeAndWait (makeRequest (source));
        const bool published = payload.isNotEmpty();
        check (published, "T3 degenerate source still publishes a payload");
        if (! published)
            continue;
        const auto degenerateParsed = parsedPayload (payload);
        const auto* object = degenerateParsed.getDynamicObject();
        check (object != nullptr, "T3 degenerate payload parses as a JSON object");
        if (object == nullptr)
            continue;
        check (object->getProperty ("status").toString() == "suspect"
                   && ! (bool) object->getProperty ("ready"),
               "T3 degenerate source reports suspect / ready=false");
        for (const char* section : { "onset_events", "onset_density", "energy_novelty" })
        {
            const auto* subsection = subObject (object, section);
            const bool explicitState = subsection != nullptr
                && (subsection->getProperty ("status").toString() == "not_evaluable"
                    || subsection->getProperty ("status").toString() == "missing");
            check (explicitState, std::string ("T3 ") + section + " carries an explicit not_evaluable/missing state");
            if (subsection != nullptr)
            {
                const auto* rows = subsection->getProperty ("events").getArray();
                const auto* densityRows = subsection->getProperty ("frames").getArray();
                const auto* noveltyRows = subsection->getProperty ("frames").getArray();
                const bool noRows = (rows == nullptr || rows->isEmpty())
                    && (densityRows == nullptr || densityRows->isEmpty())
                    && (noveltyRows == nullptr || noveltyRows->isEmpty());
                check (noRows, std::string ("T3 ") + section + " publishes empty rows, not silence");
            }
        }
    }

    juce::Logger::setCurrentLogger (nullptr);
    if (gFailures != 0)
    {
        std::printf ("[segprims] %d check(s) FAILED\n", gFailures);
        return 1;
    }
    std::printf ("[segprims] all checks passed\n");
    return 0;
}
