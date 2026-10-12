#include "TimefxDualTapEvidence.h"

#include "RefSchema.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <limits>
#include <numeric>
#include <sodium.h>
#include <vector>

namespace vit
{

namespace
{
constexpr double kSilenceDb = -160.0;
// Approximate integrated loudness offset shared with the L3 acoustic
// analyzer surface (L3AcousticAnalyzer.cpp "approximate_rms_lufs_v1"): the
// timefx artifact declares the same approximate LUFS derivation so agent-side
// consumers compare like with like.
constexpr double kApproxLufsOffsetDb = 0.691;

struct PCMData
{
    bool ok = false;
    double sampleRate = 0.0;
    int channelCount = 0;
    int64 frameCount = 0;
    int64 nonzeroSamples = 0;
    int64 nanInfSamples = 0;
    double sumSquares = 0.0;
    double peak = 0.0;
    std::vector<std::vector<float>> channels;
};

double dbFromLinear (double value)
{
    if (! std::isfinite (value) || value <= 0.0)
        return kSilenceDb;
    return juce::jmax (kSilenceDb, 20.0 * std::log10 (value));
}

PCMData readPCM (const juce::File& file)
{
    PCMData out;
    juce::AudioFormatManager formats;
    formats.registerBasicFormats();
    std::unique_ptr<juce::AudioFormatReader> reader (formats.createReaderFor (file));
    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0 || reader->numChannels <= 0)
        return out;

    out.sampleRate = reader->sampleRate;
    out.channelCount = (int) reader->numChannels;
    out.frameCount = reader->lengthInSamples;
    out.channels.resize ((size_t) out.channelCount);
    for (auto& channel : out.channels)
        channel.resize ((size_t) out.frameCount, 0.0f);

    constexpr int blockSize = 16384;
    juce::AudioBuffer<float> buffer (out.channelCount, blockSize);
    for (int64 pos = 0; pos < out.frameCount; pos += blockSize)
    {
        const int count = (int) juce::jmin<int64> (blockSize, out.frameCount - pos);
        buffer.clear();
        if (! reader->read (&buffer, 0, count, pos, true, true))
            return {};
        for (int channel = 0; channel < out.channelCount; ++channel)
        {
            const auto* source = buffer.getReadPointer (channel);
            auto* destination = out.channels[(size_t) channel].data() + pos;
            std::copy (source, source + count, destination);
            for (int i = 0; i < count; ++i)
            {
                const double value = source[i];
                if (! std::isfinite (value))
                {
                    ++out.nanInfSamples;
                    continue;
                }
                const auto magnitude = std::abs (value);
                if (magnitude > 0.0)
                    ++out.nonzeroSamples;
                out.peak = juce::jmax (out.peak, magnitude);
                out.sumSquares += value * value;
            }
        }
    }
    out.ok = true;
    return out;
}

std::vector<double> monoSignal (const PCMData& data)
{
    std::vector<double> mono ((size_t) data.frameCount, 0.0);
    for (int64 i = 0; i < data.frameCount; ++i)
    {
        double sum = 0.0;
        for (int channel = 0; channel < data.channelCount; ++channel)
        {
            const double value = data.channels[(size_t) channel][(size_t) i];
            if (std::isfinite (value))
                sum += value;
        }
        mono[(size_t) i] = sum / (double) juce::jmax (1, data.channelCount);
    }
    return mono;
}

double correlationAtLag (const std::vector<double>& input,
                         const std::vector<double>& output,
                         int lag,
                         int64 maxPoints)
{
    const int64 inputOffset = lag < 0 ? -lag : 0;
    const int64 outputOffset = lag > 0 ? lag : 0;
    const int64 available = juce::jmin<int64> ((int64) input.size() - inputOffset,
                                               (int64) output.size() - outputOffset);
    if (available < 64)
        return -1.0;

    const int64 stride = juce::jmax<int64> (1, available / juce::jmax<int64> (64, maxPoints));
    double inputSum = 0.0, outputSum = 0.0;
    int64 count = 0;
    for (int64 i = 0; i < available; i += stride)
    {
        inputSum += input[(size_t) (inputOffset + i)];
        outputSum += output[(size_t) (outputOffset + i)];
        ++count;
    }
    if (count < 64)
        return -1.0;
    const double inputMean = inputSum / (double) count;
    const double outputMean = outputSum / (double) count;
    double numerator = 0.0, inputEnergy = 0.0, outputEnergy = 0.0;
    for (int64 i = 0; i < available; i += stride)
    {
        const double in = input[(size_t) (inputOffset + i)] - inputMean;
        const double out = output[(size_t) (outputOffset + i)] - outputMean;
        numerator += in * out;
        inputEnergy += in * in;
        outputEnergy += out * out;
    }
    const auto denominator = std::sqrt (inputEnergy * outputEnergy);
    return denominator > 0.0 ? juce::jlimit (-1.0, 1.0, numerator / denominator) : -1.0;
}

TimefxTapMetrics tapMetrics (const PCMData& data, int64 expectedFrames)
{
    TimefxTapMetrics out;
    out.sampleFrames = data.frameCount;
    out.nonzeroSamples = data.nonzeroSamples;
    out.nanInfSamples = data.nanInfSamples;
    out.coverage = expectedFrames > 0 ? juce::jlimit (0.0, 1.0, (double) data.frameCount / (double) expectedFrames) : 0.0;
    out.peakDbfs = dbFromLinear (data.peak);
    const int64 scalarCount = data.frameCount * (int64) juce::jmax (1, data.channelCount);
    out.rmsDbfs = dbFromLinear (scalarCount > 0 ? std::sqrt (data.sumSquares / (double) scalarCount) : 0.0);
    out.approximateLufs = out.rmsDbfs - kApproxLufsOffsetDb;
    out.crestDb = out.peakDbfs - out.rmsDbfs;
    return out;
}

juce::var tapMetricsVar (const TimefxTapMetrics& metrics)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("sample_frames", metrics.sampleFrames);
    object->setProperty ("nonzero_samples", metrics.nonzeroSamples);
    object->setProperty ("nan_inf_samples", metrics.nanInfSamples);
    object->setProperty ("coverage", metrics.coverage);
    object->setProperty ("peak_dbfs", metrics.peakDbfs);
    object->setProperty ("rms_dbfs", metrics.rmsDbfs);
    object->setProperty ("approximate_lufs", metrics.approximateLufs);
    object->setProperty ("crest_db", metrics.crestDb);
    return juce::var (object.release());
}

juce::String artifactHash (const juce::File& file)
{
    juce::FileInputStream stream (file);
    if (! stream.openedOk())
        return {};
    crypto_hash_sha256_state state;
    crypto_hash_sha256_init (&state);
    std::array<unsigned char, 65536> buffer {};
    while (! stream.isExhausted())
    {
        const auto count = stream.read (buffer.data(), (int) buffer.size());
        if (count <= 0)
            break;
        crypto_hash_sha256_update (&state, buffer.data(), (unsigned long long) count);
    }
    unsigned char digest[crypto_hash_sha256_BYTES] {};
    crypto_hash_sha256_final (&state, digest);
    return juce::String::toHexString (digest, crypto_hash_sha256_BYTES, 0);
}
}

TimefxDualTapEvidenceResult analyseAndWriteTimefxDualTapEvidence (
    const juce::File& baselineRenderFile,
    const juce::File& processedRenderFile,
    const juce::File& processedVerificationRenderFile,
    const juce::File& artifactDirectory,
    const TimefxDualTapEvidenceRequest& request)
{
    TimefxDualTapEvidenceResult result;
    result.evidenceRef = refschema::makeTimefxDualTapRef (request.trackId, request.startSample,
                                                          request.endSample, request.pairId);
    result.latencyCompensated = request.reportedLatencySamples <= 0;
    const auto baseline = readPCM (baselineRenderFile);
    const auto processed = readPCM (processedRenderFile);
    const auto verification = readPCM (processedVerificationRenderFile);
    if (! baseline.ok || ! processed.ok || ! verification.ok)
    {
        result.reason = "render_reader_failed";
        return result;
    }

    result.sampleRate = baseline.sampleRate;
    result.channelCount = baseline.channelCount;
    const int64 expectedFrames = request.endSample - request.startSample;
    result.baselineMetrics = tapMetrics (baseline, expectedFrames);
    result.processedMetrics = tapMetrics (processed, expectedFrames);
    if (std::abs (baseline.sampleRate - request.sampleRate) > 0.01
        || std::abs (processed.sampleRate - request.sampleRate) > 0.01
        || std::abs (verification.sampleRate - request.sampleRate) > 0.01
        || baseline.channelCount != processed.channelCount)
    {
        result.reason = "sample_format_mismatch";
        return result;
    }
    if (verification.channelCount != processed.channelCount
        || verification.frameCount != processed.frameCount
        || verification.nanInfSamples > 0)
    {
        result.reason = "determinism_repeat_format_mismatch";
        return result;
    }
    double repeatSquares = 0.0;
    int64 repeatScalars = 0;
    for (int channel = 0; channel < processed.channelCount; ++channel)
        for (int64 sample = 0; sample < processed.frameCount; ++sample)
        {
            const double delta = (double) processed.channels[(size_t) channel][(size_t) sample]
                - (double) verification.channels[(size_t) channel][(size_t) sample];
            result.determinismMaxAbsDelta = juce::jmax (result.determinismMaxAbsDelta, std::abs (delta));
            repeatSquares += delta * delta;
            ++repeatScalars;
        }
    result.determinismRMSDelta = repeatScalars > 0 ? std::sqrt (repeatSquares / (double) repeatScalars) : 0.0;
    const auto processedScalarCount = processed.frameCount * (int64) juce::jmax (1, processed.channelCount);
    const auto verificationScalarCount = verification.frameCount * (int64) juce::jmax (1, verification.channelCount);
    const auto processedRMS = processedScalarCount > 0 ? std::sqrt (processed.sumSquares / (double) processedScalarCount) : 0.0;
    const auto verificationRMS = verificationScalarCount > 0
        ? std::sqrt (verification.sumSquares / (double) verificationScalarCount) : 0.0;
    result.determinismPeakDBDelta = dbFromLinear (verification.peak) - dbFromLinear (processed.peak);
    result.determinismRMSDBDelta = dbFromLinear (verificationRMS) - dbFromLinear (processedRMS);
    const auto processedMono = monoSignal (processed);
    const auto verificationMono = monoSignal (verification);
    result.determinismCorrelation = correlationAtLag (processedMono, verificationMono, 0, 65536);
    result.determinismVerified = std::isfinite (result.determinismCorrelation)
        && result.determinismCorrelation >= 0.999
        && std::abs (result.determinismPeakDBDelta) <= 0.10
        && std::abs (result.determinismRMSDBDelta) <= 0.10;
    // Time-domain effects with internal modulation (e.g. FabFilter Pro-R
    // diffusion) produce genuinely different repeat renders: the variance is
    // the plugin, not the measurement. The same-window bypass/processed pair
    // stays valid (one render per tap), so a failed repeat check downgrades
    // the evidence to partial with the variance disclosed instead of
    // rejecting it. Only the format/NaN repeat guards above stay hard.
    if (! result.determinismVerified)
        result.reason = "processed_render_repeat_variance";
    if (baseline.nanInfSamples > 0 || processed.nanInfSamples > 0)
    {
        result.reason = "nan_inf_detected";
        return result;
    }
    if (baseline.nonzeroSamples <= 0 || processed.nonzeroSamples <= 0)
    {
        result.reason = "zero_signal";
        return result;
    }
    if (result.baselineMetrics.coverage < 0.999 || result.processedMetrics.coverage < 0.999)
    {
        result.reason = "window_coverage_failed";
        return result;
    }

    result.rmsDeltaDb = result.processedMetrics.rmsDbfs - result.baselineMetrics.rmsDbfs;
    result.lufsDeltaDb = result.processedMetrics.approximateLufs - result.baselineMetrics.approximateLufs;
    result.peakDeltaDb = result.processedMetrics.peakDbfs - result.baselineMetrics.peakDbfs;
    result.crestDeltaDb = result.processedMetrics.crestDb - result.baselineMetrics.crestDb;

    auto root = std::make_unique<juce::DynamicObject>();
    root->setProperty ("schema_version", "dad.timefx_dual_tap_evidence.v1");
    root->setProperty ("pair_id", request.pairId);
    root->setProperty ("evidence_ref", result.evidenceRef);
    root->setProperty ("analyzer_version", request.analyzerVersion);

    auto scope = std::make_unique<juce::DynamicObject>();
    scope->setProperty ("track_id", request.trackId);
    scope->setProperty ("plugin_instance_id", request.pluginInstanceId);
    scope->setProperty ("plugin_position", request.pluginPosition);
    scope->setProperty ("family", request.family);
    scope->setProperty ("topology_class", request.topologyClass);
    scope->setProperty ("topology_generation", request.topologyGeneration);
    scope->setProperty ("chain_hash", request.chainHash);
    scope->setProperty ("processor_state_hash", request.processorStateHash);
    scope->setProperty ("scope_revision", request.scopeRevision);
    root->setProperty ("processor_scope", juce::var (scope.release()));

    auto conditions = std::make_unique<juce::DynamicObject>();
    conditions->setProperty ("source_revision", request.sourceRevision);
    conditions->setProperty ("clip_revision", request.clipRevision);
    conditions->setProperty ("render_revision", request.renderRevision);
    conditions->setProperty ("start_sample", request.startSample);
    conditions->setProperty ("end_sample", request.endSample);
    conditions->setProperty ("sample_rate", request.sampleRate);
    conditions->setProperty ("channel_count", baseline.channelCount);
    conditions->setProperty ("channel_layout", request.channelLayout);
    conditions->setProperty ("render_mode", "offline_probe");
    conditions->setProperty ("deterministic", request.deterministic);
    conditions->setProperty ("baseline_tap", "timefx_bypass");
    conditions->setProperty ("processed_tap", "timefx_processed");
    conditions->setProperty ("tail_policy", "same_window_pair");
    conditions->setProperty ("latency_compensated", result.latencyCompensated);
    conditions->setProperty ("reported_latency_samples", request.reportedLatencySamples);
    root->setProperty ("conditions", juce::var (conditions.release()));

    auto determinism = std::make_unique<juce::DynamicObject>();
    determinism->setProperty ("status", result.determinismVerified ? "ready" : "partial");
    determinism->setProperty ("method", "same_scope_repeat_render_envelope_tolerance_v1");
    determinism->setProperty ("repeat_count", 2);
    determinism->setProperty ("max_abs_delta", result.determinismMaxAbsDelta);
    determinism->setProperty ("rms_delta", result.determinismRMSDelta);
    determinism->setProperty ("correlation", result.determinismCorrelation);
    determinism->setProperty ("peak_db_delta", result.determinismPeakDBDelta);
    determinism->setProperty ("rms_db_delta", result.determinismRMSDBDelta);
    determinism->setProperty ("correlation_minimum", 0.999);
    determinism->setProperty ("level_delta_tolerance_db", 0.10);
    root->setProperty ("determinism_proof", juce::var (determinism.release()));

    auto quality = std::make_unique<juce::DynamicObject>();
    quality->setProperty ("baseline", tapMetricsVar (result.baselineMetrics));
    quality->setProperty ("processed", tapMetricsVar (result.processedMetrics));
    root->setProperty ("tap_metrics", juce::var (quality.release()));

    auto delta = std::make_unique<juce::DynamicObject>();
    delta->setProperty ("rms_delta_db", result.rmsDeltaDb);
    delta->setProperty ("lufs_delta_db", result.lufsDeltaDb);
    delta->setProperty ("peak_delta_db", result.peakDeltaDb);
    delta->setProperty ("crest_delta_db", result.crestDeltaDb);
    root->setProperty ("effect_delta", juce::var (delta.release()));

    if (! artifactDirectory.isDirectory() && ! artifactDirectory.createDirectory())
    {
        result.reason = "artifact_directory_failed";
        return result;
    }
    const auto artifactFile = artifactDirectory.getChildFile (request.pairId + ".json");
    if (! artifactFile.replaceWithText (juce::JSON::toString (juce::var (root.release()), true)))
    {
        result.reason = "artifact_write_failed";
        return result;
    }
    result.artifactSha256 = artifactHash (artifactFile);
    result.artifactBytes = artifactFile.getSize();
    if (result.artifactSha256.isEmpty() || result.artifactBytes <= 0)
    {
        result.reason = "artifact_integrity_failed";
        return result;
    }
    result.status = "ready";
    if (! result.determinismVerified)
    {
        result.status = "partial";
        if (result.reason == "ok")
            result.reason = "processed_render_repeat_variance";
    }
    return result;
}

} // namespace vit
