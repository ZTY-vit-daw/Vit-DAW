// TIM-KERNEL-HYGIENE-1 (Item 2) red-first acceptance tests for the L3
// evidence DC-offset disclosure.
//
// T1 signed mean       - a constant +0.10 bias under a zero-mean sine must
//    publish dc_offset ~= +0.10 and dc_offset_ratio ~= 0.10 in the feature
//    snapshot passthrough keys and in the quality_evidence block.
// T2 negative bias     - a -0.05 bias must publish a negative dc_offset
//    (signedness is the point of the field).
// T3 zero-mean source  - a plain sine must publish |dc_offset| ~ 0.
// T4 all_zero interaction - a silent file keeps the all_zero_audio quality
//    reason AND publishes dc_offset = 0 exactly (the :442 interaction the
//    GAPS report flags).
//
// Red state: before the analyzer computes the field, no dc_offset key exists
// in any published payload, so the property lookups fail and the test exits
// non-zero.

#include "../Source/Service/L3AcousticAnalyzer.h"

#include <atomic>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <future>
#include <memory>
#include <string>

namespace
{

constexpr double kSampleRate = 44100.0;
constexpr double kDurationSeconds = 2.0;
constexpr double kSineHz = 220.0;
constexpr double kSineAmplitude = 0.20;
constexpr int kAnalysisWaitTimeoutSeconds = 30;

int gFailures = 0;

void check (bool condition, const std::string& description)
{
    if (condition)
    {
        std::printf ("[l3evidence] PASS %s\n", description.c_str());
        std::fflush (stdout);
        return;
    }
    ++gFailures;
    std::printf ("[l3evidence] FAIL %s\n", description.c_str());
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
                               .getChildFile ("Vit_DAW_L3EvidenceTests");
    if (directory.exists())
        directory.deleteRecursively();
    directory.createDirectory();
    return directory;
}

// dcBias is added to a zero-mean sine; integer sample positions and standard
// libm only, so the expected mean is exactly the bias (sine sums to ~0 over
// an even number of full cycles; tolerance below absorbs the residual).
bool writeBiasedSineWav (const juce::File& file, double dcBias, int64 sampleCount)
{
    juce::AudioBuffer<float> buffer (2, (int) sampleCount);
    for (int i = 0; i < (int) sampleCount; ++i)
    {
        const double t = (double) i / kSampleRate;
        const double sample = dcBias + kSineAmplitude * std::sin (juce::MathConstants<double>::twoPi * kSineHz * t);
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

bool writeSilenceWav (const juce::File& file, int64 sampleCount)
{
    juce::AudioBuffer<float> buffer (2, (int) sampleCount);
    buffer.clear();

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

juce::String analyzeAndWait (const juce::String& filePath)
{
    vit::AudioFeatureBakeRequest request;
    request.projectId = "l3evidence-test-project";
    request.filePath = filePath;
    request.trackId = "l3evidence-test-track";
    request.sourceId = "l3evidence-test-source";
    request.sourceRevision = "l3evidence-test-rev-1";
    request.featureType = vit::AudioFeatureType::BandEnergySummary;
    request.priority = vit::AudioFeaturePriority::OnDemand;

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

bool hasDCOffsetKeys (const juce::String& payload, double expectedOffset, double tolerance,
                      const std::string& tag)
{
    auto parsed = juce::JSON::parse (payload);
    auto* object = parsed.getDynamicObject();
    if (object == nullptr)
    {
        check (false, tag + ": payload is not a JSON object");
        return false;
    }

    const auto offsetVar = object->getProperty ("dc_offset");
    if (!offsetVar.isDouble())
    {
        check (false, tag + ": feature snapshot passthrough key dc_offset missing (not a double)");
        return false;
    }
    const auto ratioVar = object->getProperty ("dc_offset_ratio");
    if (!ratioVar.isDouble())
    {
        check (false, tag + ": feature snapshot passthrough key dc_offset_ratio missing (not a double)");
        return false;
    }
    const double offset = offsetVar;
    const double ratio = ratioVar;

    check (std::abs (offset - expectedOffset) <= tolerance,
           tag + ": dc_offset ~= " + std::to_string (expectedOffset));
    check (std::abs (ratio - std::abs (expectedOffset)) <= tolerance,
           tag + ": dc_offset_ratio ~= |" + std::to_string (expectedOffset) + "|");

    if (auto* evidence = object->getProperty ("quality_evidence").getDynamicObject())
    {
        const auto evidenceOffset = evidence->getProperty ("dc_offset");
        const auto evidenceRatio = evidence->getProperty ("dc_offset_ratio");
        check (evidenceOffset.isDouble() && std::abs ((double) evidenceOffset - expectedOffset) <= tolerance,
               tag + ": quality_evidence.dc_offset ~= expected");
        check (evidenceRatio.isDouble() && std::abs ((double) evidenceRatio - std::abs (expectedOffset)) <= tolerance,
               tag + ": quality_evidence.dc_offset_ratio ~= expected");
    }
    else
    {
        check (false, tag + ": quality_evidence block missing");
    }
    return true;
}

} // namespace

int main()
{
    CapturingLogger capturingLogger;
    juce::Logger::setCurrentLogger (&capturingLogger);

    const auto workspace = testWorkspaceDirectory();
    const auto biasedPositive = workspace.getChildFile ("l3evidence_bias_plus.wav");
    const auto biasedNegative = workspace.getChildFile ("l3evidence_bias_minus.wav");
    const auto plainSine = workspace.getChildFile ("l3evidence_plain.wav");
    const auto silence = workspace.getChildFile ("l3evidence_silence.wav");
    const int64 sampleCount = (int64) (kDurationSeconds * kSampleRate);

    if (! writeBiasedSineWav (biasedPositive, 0.10, sampleCount)
        || ! writeBiasedSineWav (biasedNegative, -0.05, sampleCount)
        || ! writeBiasedSineWav (plainSine, 0.0, sampleCount)
        || ! writeSilenceWav (silence, sampleCount))
    {
        std::printf ("[l3evidence] FAIL could not write test wav files under %s\n",
                     workspace.getFullPathName().toStdString().c_str());
        return 1;
    }

    // 16-bit quantization noise dominates the residual: 1 LSB at these
    // amplitudes is ~3e-5, so 5e-3 tolerances are generous but still pin the
    // sign and magnitude of the bias.
    {
        const auto payload = analyzeAndWait (biasedPositive.getFullPathName());
        check (payload.isNotEmpty(), "T1 biased source publishes a payload");
        if (payload.isNotEmpty())
            hasDCOffsetKeys (payload, 0.10, 5e-3, "T1 positive bias");
    }
    {
        const auto payload = analyzeAndWait (biasedNegative.getFullPathName());
        check (payload.isNotEmpty(), "T2 negative-bias source publishes a payload");
        if (payload.isNotEmpty())
            hasDCOffsetKeys (payload, -0.05, 5e-3, "T2 negative bias");
    }
    {
        const auto payload = analyzeAndWait (plainSine.getFullPathName());
        check (payload.isNotEmpty(), "T3 zero-mean source publishes a payload");
        if (payload.isNotEmpty())
            hasDCOffsetKeys (payload, 0.0, 5e-3, "T3 zero mean");
    }
    {
        const auto payload = analyzeAndWait (silence.getFullPathName());
        check (payload.isNotEmpty(), "T4 silence publishes a payload");
        if (payload.isNotEmpty())
        {
            auto parsed = juce::JSON::parse (payload);
            if (auto* object = parsed.getDynamicObject())
            {
                const auto offsetVar = object->getProperty ("dc_offset");
                check (offsetVar.isDouble() && (double) offsetVar == 0.0,
                       "T4 silence publishes dc_offset exactly 0");
                if (auto* evidence = object->getProperty ("quality_evidence").getDynamicObject())
                {
                    const auto reasons = evidence->getProperty ("quality_reasons").toString();
                    check (reasons.contains ("all_zero_audio"),
                           "T4 silence keeps the all_zero_audio quality reason");
                }
                else
                {
                    check (false, "T4 quality_evidence block missing");
                }
            }
            else
            {
                check (false, "T4 payload is not a JSON object");
            }
        }
    }

    if (gFailures > 0)
    {
        std::printf ("[l3evidence] %d check(s) FAILED\n", gFailures);
        return 1;
    }
    std::printf ("[l3evidence] all checks passed\n");
    return 0;
}
