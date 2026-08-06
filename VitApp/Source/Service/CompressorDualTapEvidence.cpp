#include "CompressorDualTapEvidence.h"

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

std::pair<int, double> measureOffset (const PCMData& input, const PCMData& output, int maxSearch)
{
    const auto inputMono = monoSignal (input);
    const auto outputMono = monoSignal (output);
    int bestLag = 0;
    double bestCorrelation = -1.0;
    for (int lag = -maxSearch; lag <= maxSearch; ++lag)
    {
        const auto correlation = correlationAtLag (inputMono, outputMono, lag, 32768);
        if (correlation > bestCorrelation)
        {
            bestCorrelation = correlation;
            bestLag = lag;
        }
    }
    return { bestLag, bestCorrelation };
}

juce::Array<juce::var> channelFrameValues (const PCMData& data,
                                           int64 offset,
                                           int64 start,
                                           int count,
                                           bool peak)
{
    juce::Array<juce::var> values;
    for (int channel = 0; channel < data.channelCount; ++channel)
    {
        double accumulator = 0.0;
        double maximum = 0.0;
        for (int i = 0; i < count; ++i)
        {
            const double value = data.channels[(size_t) channel][(size_t) (offset + start + i)];
            if (! std::isfinite (value))
                continue;
            maximum = juce::jmax (maximum, std::abs (value));
            accumulator += value * value;
        }
        values.add (dbFromLinear (peak ? maximum : std::sqrt (accumulator / (double) juce::jmax (1, count))));
    }
    return values;
}

juce::var tapQualityVar (const CompressorTapQuality& quality)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("sample_frames", quality.sampleFrames);
    object->setProperty ("nonzero_samples", quality.nonzeroSamples);
    object->setProperty ("nan_inf_samples", quality.nanInfSamples);
    object->setProperty ("coverage", quality.coverage);
    object->setProperty ("peak_dbfs", quality.peakDbfs);
    object->setProperty ("rms_dbfs", quality.rmsDbfs);
    return juce::var (object.release());
}

CompressorTapQuality tapQuality (const PCMData& data, int64 expectedFrames)
{
    CompressorTapQuality out;
    out.sampleFrames = data.frameCount;
    out.nonzeroSamples = data.nonzeroSamples;
    out.nanInfSamples = data.nanInfSamples;
    out.coverage = expectedFrames > 0 ? juce::jlimit (0.0, 1.0, (double) data.frameCount / (double) expectedFrames) : 0.0;
    out.peakDbfs = dbFromLinear (data.peak);
    const int64 scalarCount = data.frameCount * (int64) juce::jmax (1, data.channelCount);
    out.rmsDbfs = dbFromLinear (scalarCount > 0 ? std::sqrt (data.sumSquares / (double) scalarCount) : 0.0);
    return out;
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

CompressorDualTapEvidenceResult analyseAndWriteCompressorDualTapEvidence (
    const juce::File& inputFile,
    const juce::File& outputFile,
    const juce::File& outputVerificationFile,
    const juce::File& artifactDirectory,
    const CompressorDualTapEvidenceRequest& request)
{
    CompressorDualTapEvidenceResult result;
    result.evidenceRef = "dad.compressor_dual_tap:" + request.pairId;
    const auto input = readPCM (inputFile);
    const auto output = readPCM (outputFile);
    const auto outputVerification = readPCM (outputVerificationFile);
    if (! input.ok || ! output.ok || ! outputVerification.ok)
    {
        result.reason = "render_reader_failed";
        return result;
    }

    result.sampleRate = input.sampleRate;
    result.channelCount = input.channelCount;
    const int64 expectedFrames = request.endSample - request.startSample;
    result.inputQuality = tapQuality (input, expectedFrames);
    result.outputQuality = tapQuality (output, expectedFrames);
    if (std::abs (input.sampleRate - request.sampleRate) > 0.01
        || std::abs (output.sampleRate - request.sampleRate) > 0.01
        || std::abs (outputVerification.sampleRate - request.sampleRate) > 0.01
        || input.channelCount != output.channelCount)
    {
        result.reason = "sample_format_mismatch";
        return result;
    }
    if (outputVerification.channelCount != output.channelCount
        || outputVerification.frameCount != output.frameCount
        || outputVerification.nanInfSamples > 0)
    {
        result.reason = "determinism_repeat_format_mismatch";
        return result;
    }
    double repeatSquares = 0.0;
    int64 repeatScalars = 0;
    for (int channel = 0; channel < output.channelCount; ++channel)
        for (int64 sample = 0; sample < output.frameCount; ++sample)
        {
            const double delta = (double) output.channels[(size_t) channel][(size_t) sample]
                - (double) outputVerification.channels[(size_t) channel][(size_t) sample];
            result.determinismMaxAbsDelta = juce::jmax (result.determinismMaxAbsDelta, std::abs (delta));
            repeatSquares += delta * delta;
            ++repeatScalars;
        }
    result.determinismRMSDelta = repeatScalars > 0 ? std::sqrt (repeatSquares / (double) repeatScalars) : 0.0;
    const auto outputScalarCount = output.frameCount * (int64) juce::jmax (1, output.channelCount);
    const auto verificationScalarCount = outputVerification.frameCount * (int64) juce::jmax (1, outputVerification.channelCount);
    const auto outputRMS = outputScalarCount > 0 ? std::sqrt (output.sumSquares / (double) outputScalarCount) : 0.0;
    const auto verificationRMS = verificationScalarCount > 0
        ? std::sqrt (outputVerification.sumSquares / (double) verificationScalarCount) : 0.0;
    result.determinismPeakDBDelta = dbFromLinear (outputVerification.peak) - dbFromLinear (output.peak);
    result.determinismRMSDBDelta = dbFromLinear (verificationRMS) - dbFromLinear (outputRMS);
    const auto outputMono = monoSignal (output);
    const auto verificationMono = monoSignal (outputVerification);
    result.determinismCorrelation = correlationAtLag (outputMono, verificationMono, 0, 65536);
    result.determinismVerified = std::isfinite (result.determinismCorrelation)
        && result.determinismCorrelation >= 0.999
        && std::abs (result.determinismPeakDBDelta) <= 0.10
        && std::abs (result.determinismRMSDBDelta) <= 0.10;
    if (! result.determinismVerified)
    {
        result.reason = "processed_render_not_deterministic";
        return result;
    }
    if (input.nanInfSamples > 0 || output.nanInfSamples > 0)
    {
        result.reason = "nan_inf_detected";
        return result;
    }
    if (input.nonzeroSamples <= 0 || output.nonzeroSamples <= 0)
    {
        result.reason = "zero_signal";
        return result;
    }
    if (result.inputQuality.coverage < 0.999 || result.outputQuality.coverage < 0.999)
    {
        result.reason = "window_coverage_failed";
        return result;
    }

    const int maxSearch = juce::jmax (1, request.maxAlignmentSearchSamples);
    const auto [measuredOffset, correlation] = measureOffset (input, output, maxSearch);
    result.measuredOffsetSamples = measuredOffset;
    result.appliedOffsetSamples = measuredOffset;
    result.alignmentCorrelation = correlation;
    result.alignmentReady = std::isfinite (correlation) && correlation >= 0.25 && std::abs (measuredOffset) < maxSearch;
    if (! result.alignmentReady)
    {
        result.reason = std::abs (measuredOffset) >= maxSearch ? "alignment_search_boundary" : "alignment_correlation_failed";
        return result;
    }

    const int64 inputOffset = measuredOffset < 0 ? -measuredOffset : 0;
    const int64 outputOffset = measuredOffset > 0 ? measuredOffset : 0;
    result.alignedSampleFrames = juce::jmin<int64> (input.frameCount - inputOffset, output.frameCount - outputOffset);
    if (result.alignedSampleFrames < request.frameSizeSamples)
    {
        result.reason = "aligned_window_too_short";
        return result;
    }

    auto root = std::make_unique<juce::DynamicObject>();
    root->setProperty ("schema_version", "dad.compressor_dual_tap_evidence.v1");
    root->setProperty ("pair_id", request.pairId);
    root->setProperty ("evidence_ref", result.evidenceRef);
    root->setProperty ("analyzer_version", request.analyzerVersion);

    auto scope = std::make_unique<juce::DynamicObject>();
    scope->setProperty ("track_id", request.trackId);
    scope->setProperty ("plugin_instance_id", request.pluginInstanceId);
    scope->setProperty ("plugin_position", request.pluginPosition);
    scope->setProperty ("topology_class", request.topologyClass);
    scope->setProperty ("topology_generation", request.topologyGeneration);
    scope->setProperty ("support_class", request.supportClass);
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
    conditions->setProperty ("channel_count", input.channelCount);
    conditions->setProperty ("channel_layout", request.channelLayout);
    conditions->setProperty ("render_mode", "offline_probe");
    conditions->setProperty ("deterministic", request.deterministic);
    conditions->setProperty ("input_tap", "compressor_input");
    conditions->setProperty ("output_tap", "compressor_output");
    conditions->setProperty ("tail_policy", "exact_window_no_tail");
    conditions->setProperty ("frame_size_samples", request.frameSizeSamples);
    conditions->setProperty ("hop_size_samples", request.hopSizeSamples);
    root->setProperty ("conditions", juce::var (conditions.release()));

    auto alignment = std::make_unique<juce::DynamicObject>();
    alignment->setProperty ("method", "offline_pdc_plus_integer_cross_correlation_v1");
    alignment->setProperty ("plugin_reported_latency_samples", request.reportedLatencySamples);
    alignment->setProperty ("measured_offset_samples", measuredOffset);
    alignment->setProperty ("applied_offset_samples", measuredOffset);
    alignment->setProperty ("residual_error_samples", result.residualErrorSamples);
    alignment->setProperty ("correlation", correlation);
    alignment->setProperty ("search_radius_samples", maxSearch);
    alignment->setProperty ("aligned_sample_frames", result.alignedSampleFrames);
    alignment->setProperty ("status", "ready");
    root->setProperty ("latency_alignment", juce::var (alignment.release()));

    auto determinism = std::make_unique<juce::DynamicObject>();
    determinism->setProperty ("status", "ready");
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
    quality->setProperty ("input", tapQualityVar (result.inputQuality));
    quality->setProperty ("output", tapQualityVar (result.outputQuality));
    root->setProperty ("quality_evidence", juce::var (quality.release()));

    juce::Array<juce::var> frames;
    std::vector<double> inputFramePeaks;
    const int frameSize = juce::jmax (16, request.frameSizeSamples);
    const int hopSize = juce::jmax (1, request.hopSizeSamples);
    for (int64 start = 0; start + frameSize <= result.alignedSampleFrames; start += hopSize)
    {
        auto frame = std::make_unique<juce::DynamicObject>();
        frame->setProperty ("start_sample", request.startSample + start);
        frame->setProperty ("end_sample", request.startSample + start + frameSize);
        const auto inputPeaks = channelFrameValues (input, inputOffset, start, frameSize, true);
        frame->setProperty ("input_peak_dbfs", inputPeaks);
        frame->setProperty ("input_rms_dbfs", channelFrameValues (input, inputOffset, start, frameSize, false));
        frame->setProperty ("output_peak_dbfs", channelFrameValues (output, outputOffset, start, frameSize, true));
        frame->setProperty ("output_rms_dbfs", channelFrameValues (output, outputOffset, start, frameSize, false));
        double peak = kSilenceDb;
        for (const auto& value : inputPeaks)
            peak = juce::jmax (peak, (double) value);
        inputFramePeaks.push_back (peak);
        frames.add (juce::var (frame.release()));
    }
    result.envelopeFrameCount = frames.size();
    root->setProperty ("aligned_envelope_frames", frames);

    juce::Array<juce::var> events;
    const int refractoryFrames = juce::jmax (1, (int) std::ceil (0.020 * request.sampleRate / (double) hopSize));
    int lastEvent = -refractoryFrames;
    for (int i = 1; i + 1 < (int) inputFramePeaks.size(); ++i)
    {
        if (inputFramePeaks[(size_t) i] < -48.0
            || inputFramePeaks[(size_t) i] < inputFramePeaks[(size_t) i - 1]
            || inputFramePeaks[(size_t) i] <= inputFramePeaks[(size_t) i + 1]
            || i - lastEvent < refractoryFrames)
            continue;
        auto event = std::make_unique<juce::DynamicObject>();
        event->setProperty ("sample", request.startSample + (int64) i * hopSize);
        event->setProperty ("input_peak_dbfs", inputFramePeaks[(size_t) i]);
        event->setProperty ("kind", "input_onset_candidate");
        events.add (juce::var (event.release()));
        lastEvent = i;
    }
    result.eventCandidateCount = events.size();
    root->setProperty ("input_event_candidates", events);

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
    result.reason = "ok";
    return result;
}

} // namespace vit
