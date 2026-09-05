#include "VitProductionCoordinator.h"

#include <algorithm>
#include <array>
#include <chrono>
#include <cmath>
#include <limits>
#include <thread>
#include <vector>

namespace vit
{

namespace
{
constexpr double kSilenceDb = -160.0;

// KERNEL-RENDER-1 render watchdog: offline renders of Vit-DAW-scale projects
// finish in seconds; 120 s is a generous fixed budget plus the rendered range
// duration so long material keeps proportionally more headroom.
constexpr double kRenderWatchdogBaseSeconds = 120.0;

int64_t steadyClockNowMs()
{
    return (int64_t) std::chrono::duration_cast<std::chrono::milliseconds> (
        std::chrono::steady_clock::now().time_since_epoch()).count();
}

juce::String makeCoordinatorError (const juce::String& message)
{
    auto o = std::make_unique<juce::DynamicObject>();
    o->setProperty ("status", "error");
    o->setProperty ("message", message);
    return juce::JSON::toString (juce::var (o.release()));
}

double dbFromLinear (double value)
{
    if (! std::isfinite (value) || value <= 0.0)
        return kSilenceDb;

    return juce::jmax (kSilenceDb, 20.0 * std::log10 (value));
}

double dbFromEnergy (double value)
{
    if (! std::isfinite (value) || value <= 0.0)
        return kSilenceDb;

    return juce::jmax (kSilenceDb, 10.0 * std::log10 (value));
}

juce::String balanceStateForDb (double balanceDb)
{
    if (! std::isfinite (balanceDb))
        return "unknown";
    if (balanceDb > 1.5)
        return "right_heavy";
    if (balanceDb < -1.5)
        return "left_heavy";
    return "centered";
}

juce::String correlationStateForValue (double value)
{
    if (! std::isfinite (value))
        return "unknown";
    if (value < -0.2)
        return "phase_inverted";
    if (value < 0.25)
        return "decorrelated";
    if (value > 0.92)
        return "highly_correlated";
    return "coherent";
}

struct L2BandSummary
{
    juce::String name;
    double minHz = 0.0;
    double maxHz = 0.0;
    double energy = 0.0;
    int binHits = 0;
};

struct L2MaskingFrame
{
    double startSeconds = 0.0;
    double endSeconds = 0.0;
    std::array<double, 6> bandLevelsDbfs {{ kSilenceDb, kSilenceDb, kSilenceDb, kSilenceDb, kSilenceDb, kSilenceDb }};
};

struct L2ProbeAnalysis
{
    bool readerOk = false;
    int channelCount = 0;
    double sampleRate = 0.0;
    double durationSeconds = 0.0;
    int64 sampleCount = 0;
    int64 nanInfCount = 0;
    int64 nonzeroCount = 0;
    double sumAbs = 0.0;
    double maxAbs = 0.0;
    double peakAbs = 0.0;
    double rms = 0.0;
    double leftRms = 0.0;
    double rightRms = 0.0;
    double balanceDb = 0.0;
    double correlation = 1.0;
    double spectralEnergy = 0.0;
    double coverage = 0.0;
    int64 maskingFrameCountTotal = 0;
    int maskingFrameAggregationStride = 1;
    std::vector<L2MaskingFrame> maskingFrames;
    juce::String status = "suspect";
    juce::String reason;
    std::vector<L2BandSummary> bands {
        { "sub",      20.0,    60.0,    0.0, 0 },
        { "bass",     60.0,    250.0,   0.0, 0 },
        { "low_mid",  250.0,   500.0,   0.0, 0 },
        { "mid",      500.0,   2000.0,  0.0, 0 },
        { "presence", 2000.0,  6000.0,  0.0, 0 },
        { "air",      6000.0,  20000.0, 0.0, 0 }
    };
};

void stampL2ProbeIdentity (juce::DynamicObject& obj,
                           const VitProductionCoordinator::L2RenderProbeRequest& request)
{
    obj.setProperty ("command", "l2_render_probe_status");
    obj.setProperty ("feature_family", "audio_feature");
    obj.setProperty ("feature_type", "l2_render_probe");
    obj.setProperty ("schema_version", "dad_l2_render_probe.v1");
    obj.setProperty ("layer", "l2_realtime");
    obj.setProperty ("status", "building");
    obj.setProperty ("project_id", "current");
    obj.setProperty ("render_mode", request.renderMode);
    obj.setProperty ("tap_point", request.tapPoint);
    obj.setProperty ("tail_seconds", request.tailSeconds);
    obj.setProperty ("track_id", request.trackId);
    obj.setProperty ("clip_id", request.clipId);
    obj.setProperty ("source_path", request.sourcePath);
    obj.setProperty ("source_revision", request.sourceRevision);
    obj.setProperty ("clip_revision", request.clipRevision);
    obj.setProperty ("render_revision", request.renderRevision);
    obj.setProperty ("evidence_ref", "dad.l2_render_probe:" + request.renderRevision);
    if (request.analysisBandLowHz > 0.0 && request.analysisBandHighHz > request.analysisBandLowHz)
    {
        obj.setProperty ("analysis_band_low_hz", request.analysisBandLowHz);
        obj.setProperty ("analysis_band_high_hz", request.analysisBandHighHz);
        obj.setProperty ("analysis_band_id", request.analysisBandId);
    }
    if (request.requestId.isNotEmpty())
        obj.setProperty ("request_id", request.requestId);

    auto analyzedRange = std::make_unique<juce::DynamicObject>();
    analyzedRange->setProperty ("start_seconds", request.analyzedStartSeconds);
    analyzedRange->setProperty ("end_seconds", request.analyzedEndSeconds);
    analyzedRange->setProperty ("duration_seconds",
                                juce::jmax (0.0, request.analyzedEndSeconds - request.analyzedStartSeconds));
    obj.setProperty ("analyzed_range", juce::var (analyzedRange.release()));
}

void publishL2ProbeBuilding (const VitProductionCoordinator::PublishFn& publish,
                             const VitProductionCoordinator::L2RenderProbeRequest& request,
                             const juce::String& jobId)
{
    if (! publish || ! request.enabled)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampL2ProbeIdentity (*obj, request);
    obj->setProperty ("job_id", jobId);
    obj->setProperty ("status", "building");
    obj->setProperty ("quality_status", "building");
    obj->setProperty ("reason", "offline_render_in_progress");
    publish (juce::JSON::toString (juce::var (obj.release())));
}

void publishL2ProbeFailure (const VitProductionCoordinator::PublishFn& publish,
                            const VitProductionCoordinator::L2RenderProbeRequest& request,
                            const juce::String& jobId,
                            const juce::String& reason)
{
    if (! publish || ! request.enabled)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampL2ProbeIdentity (*obj, request);
    obj->setProperty ("job_id", jobId);
    obj->setProperty ("status", "suspect");
    obj->setProperty ("quality_status", "suspect");
    obj->setProperty ("reason", reason);
    auto evidence = std::make_unique<juce::DynamicObject>();
    evidence->setProperty ("nonzero", false);
    evidence->setProperty ("sum_abs", 0.0);
    evidence->setProperty ("max_abs", 0.0);
    evidence->setProperty ("nan_inf_count", 0);
    evidence->setProperty ("coverage", 0.0);
    evidence->setProperty ("latency_compensated", request.latencyCompensated);
    evidence->setProperty ("tail_captured", request.tailCaptured);
    evidence->setProperty ("deterministic", request.deterministic);
    obj->setProperty ("quality_evidence", juce::var (evidence.release()));
    publish (juce::JSON::toString (juce::var (obj.release())));
}

L2ProbeAnalysis analyseL2ProbeFile (const juce::File& renderFile,
                                    const VitProductionCoordinator::L2RenderProbeRequest& request)
{
    L2ProbeAnalysis out;
    if (request.analysisBandLowHz > 0.0 && request.analysisBandHighHz > request.analysisBandLowHz)
    {
        out.bands.push_back ({ request.analysisBandId.isNotEmpty() ? request.analysisBandId : juce::String ("target"),
                               request.analysisBandLowHz,
                               request.analysisBandHighHz,
                               0.0,
                               0 });
    }

    juce::AudioFormatManager fm;
    fm.registerBasicFormats();
    std::unique_ptr<juce::AudioFormatReader> reader (fm.createReaderFor (renderFile));

    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0)
    {
        out.reason = "reader_failed";
        return out;
    }

    out.readerOk = true;
    out.sampleRate = reader->sampleRate;
    out.channelCount = juce::jmax (1, (int) reader->numChannels);
    out.durationSeconds = (double) reader->lengthInSamples / reader->sampleRate;

    constexpr int fftOrder = 12;
    constexpr int fftSize = 1 << fftOrder;
    constexpr int fftBins = fftSize / 2;
    constexpr int maxMaskingFrames = 256;
    juce::dsp::FFT fft (fftOrder);
    juce::dsp::WindowingFunction<float> window (fftSize,
                                                juce::dsp::WindowingFunction<float>::hann,
                                                false);
    juce::AudioBuffer<float> buffer (2, fftSize);
    std::vector<float> fftData ((size_t) fftSize * 2, 0.0f);

    double sumSquares = 0.0;
    double sumSquaresL = 0.0;
    double sumSquaresR = 0.0;
    double sumLR = 0.0;
    int64 frameCount = 0;
    const auto totalFFTFrames = juce::jmax<int64> (1, (reader->lengthInSamples + fftSize - 1) / fftSize);
    out.maskingFrameAggregationStride = juce::jmax (1, (int) ((totalFFTFrames + maxMaskingFrames - 1) / maxMaskingFrames));
    std::array<double, 6> maskingGroupEnergy {{ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 }};
    int maskingGroupFrames = 0;
    double maskingGroupStartSeconds = 0.0;
    double maskingGroupEndSeconds = 0.0;

    const auto flushMaskingGroup = [&]
    {
        if (maskingGroupFrames <= 0)
            return;
        L2MaskingFrame frame;
        frame.startSeconds = maskingGroupStartSeconds;
        frame.endSeconds = maskingGroupEndSeconds;
        for (int bandIndex = 0; bandIndex < 6; ++bandIndex)
            frame.bandLevelsDbfs[(size_t) bandIndex] = dbFromEnergy (
                maskingGroupEnergy[(size_t) bandIndex] / (double) maskingGroupFrames);
        out.maskingFrames.push_back (frame);
        maskingGroupEnergy.fill (0.0);
        maskingGroupFrames = 0;
    };

    for (int64 pos = 0; pos < reader->lengthInSamples; pos += fftSize)
    {
        const int valid = (int) juce::jmin<int64> ((int64) fftSize, reader->lengthInSamples - pos);
        if (valid <= 0)
            break;

        buffer.clear();
        reader->read (&buffer, 0, valid, pos, true, true);

        const auto* left = buffer.getReadPointer (0);
        const auto* right = buffer.getReadPointer (1);

        std::fill (fftData.begin(), fftData.end(), 0.0f);

        for (int i = 0; i < valid; ++i)
        {
            const float lv = left[i];
            const float rv = out.channelCount > 1 ? right[i] : lv;

            for (auto v : { lv, rv })
            {
                ++out.sampleCount;
                if (! std::isfinite (v))
                {
                    ++out.nanInfCount;
                    continue;
                }

                const auto absValue = std::abs ((double) v);
                if (absValue > 0.0)
                    ++out.nonzeroCount;

                out.sumAbs += absValue;
                out.maxAbs = juce::jmax (out.maxAbs, absValue);
                out.peakAbs = juce::jmax (out.peakAbs, absValue);
                sumSquares += (double) v * (double) v;
            }

            if (std::isfinite (lv) && std::isfinite (rv))
            {
                sumSquaresL += (double) lv * (double) lv;
                sumSquaresR += (double) rv * (double) rv;
                sumLR += (double) lv * (double) rv;
                ++frameCount;
            }

            fftData[(size_t) i] = 0.5f * (lv + rv);
        }

        window.multiplyWithWindowingTable (fftData.data(), fftSize);
        fft.performRealOnlyForwardTransform (fftData.data());

        std::array<double, 6> maskingFrameEnergy {{ 0.0, 0.0, 0.0, 0.0, 0.0, 0.0 }};

        for (int bin = 1; bin < fftBins; ++bin)
        {
            const auto re = fftData[(size_t) bin * 2];
            const auto im = fftData[(size_t) bin * 2 + 1];
            const double mag2 = (double) re * (double) re + (double) im * (double) im;
            if (! std::isfinite (mag2) || mag2 <= 0.0)
                continue;

            const double hz = ((double) bin * out.sampleRate) / (double) fftSize;
            bool covered = false;
            for (int bandIndex = 0; bandIndex < (int) out.bands.size(); ++bandIndex)
            {
                auto& band = out.bands[(size_t) bandIndex];
                if (hz >= band.minHz && hz < band.maxHz)
                {
                    band.energy += mag2;
                    ++band.binHits;
                    if (bandIndex < 6)
                        maskingFrameEnergy[(size_t) bandIndex] += mag2;
                    covered = true;
                }
            }
            if (covered)
                out.spectralEnergy += mag2;
        }

        if (maskingGroupFrames == 0)
            maskingGroupStartSeconds = (double) pos / out.sampleRate;
        maskingGroupEndSeconds = (double) (pos + valid) / out.sampleRate;
        for (int bandIndex = 0; bandIndex < 6; ++bandIndex)
            maskingGroupEnergy[(size_t) bandIndex] += maskingFrameEnergy[(size_t) bandIndex];
        ++maskingGroupFrames;
        ++out.maskingFrameCountTotal;
        if (maskingGroupFrames >= out.maskingFrameAggregationStride)
            flushMaskingGroup();
    }
    flushMaskingGroup();

    if (out.sampleCount > 0)
        out.rms = std::sqrt (sumSquares / (double) out.sampleCount);

    if (frameCount > 0)
    {
        out.leftRms = std::sqrt (sumSquaresL / (double) frameCount);
        out.rightRms = std::sqrt (sumSquaresR / (double) frameCount);
        out.balanceDb = dbFromLinear (out.rightRms) - dbFromLinear (out.leftRms);
        const auto denom = std::sqrt (sumSquaresL * sumSquaresR);
        out.correlation = denom > 0.0 ? juce::jlimit (-1.0, 1.0, sumLR / denom) : 1.0;
    }

    const double requestedDuration = juce::jmax (0.0, request.analyzedEndSeconds - request.analyzedStartSeconds);
    out.coverage = requestedDuration > 0.0
        ? juce::jlimit (0.0, 1.0, out.durationSeconds / requestedDuration)
        : (out.durationSeconds > 0.0 ? 1.0 : 0.0);

    if (out.nonzeroCount > 0 && out.sumAbs > 0.0 && out.maxAbs > 0.0 && out.nanInfCount == 0 && out.coverage > 0.0)
    {
        out.status = "ready";
        out.reason = "ok";
    }
    else
    {
        out.status = "suspect";
        out.reason = out.nonzeroCount <= 0 ? "all_zero_render" : "quality_evidence_failed";
    }

    return out;
}

void publishL2ProbeAnalysis (const VitProductionCoordinator::PublishFn& publish,
                             const VitProductionCoordinator::L2RenderProbeRequest& request,
                             const juce::String& jobId,
                             const L2ProbeAnalysis& analysis)
{
    if (! publish || ! request.enabled)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampL2ProbeIdentity (*obj, request);
    obj->setProperty ("command", "l2_render_probe_ready");
    obj->setProperty ("job_id", jobId);
    obj->setProperty ("status", analysis.status);
    obj->setProperty ("quality_status", analysis.status);
    obj->setProperty ("reason", analysis.reason);
    obj->setProperty ("duration_seconds", analysis.durationSeconds);
    obj->setProperty ("sample_rate", analysis.sampleRate);
    obj->setProperty ("channel_count", analysis.channelCount);
    obj->setProperty ("peak_abs", analysis.peakAbs);
    obj->setProperty ("peak_dbfs", dbFromLinear (analysis.peakAbs));
    obj->setProperty ("rms", analysis.rms);
    obj->setProperty ("rms_dbfs", dbFromLinear (analysis.rms));
    obj->setProperty ("headroom_db", analysis.peakAbs > 0.0 ? -dbFromLinear (analysis.peakAbs) : 0.0);
    obj->setProperty ("crest_db", dbFromLinear (analysis.peakAbs) - dbFromLinear (analysis.rms));
    obj->setProperty ("left_level_db", dbFromLinear (analysis.leftRms));
    obj->setProperty ("right_level_db", dbFromLinear (analysis.rightRms));
    obj->setProperty ("balance_db", analysis.balanceDb);
    obj->setProperty ("balance_state", balanceStateForDb (analysis.balanceDb));
    obj->setProperty ("correlation_estimate", analysis.correlation);
    obj->setProperty ("correlation_state", correlationStateForValue (analysis.correlation));

    auto bandsObject = std::make_unique<juce::DynamicObject>();
    for (const auto& band : analysis.bands)
    {
        auto bandObject = std::make_unique<juce::DynamicObject>();
        const double unitEnergy = analysis.spectralEnergy > 0.0
            ? juce::jlimit (0.0, 1.0, band.energy / analysis.spectralEnergy)
            : 0.0;
        bandObject->setProperty ("unit_energy", unitEnergy);
        bandObject->setProperty ("energy_db", dbFromEnergy (band.energy));
        bandObject->setProperty ("min_hz", band.minHz);
        bandObject->setProperty ("max_hz", band.maxHz);
        bandObject->setProperty ("coverage_ratio", band.binHits > 0 ? analysis.coverage : 0.0);
        bandsObject->setProperty (juce::Identifier (band.name), juce::var (bandObject.release()));
    }
    obj->setProperty ("bands", juce::var (bandsObject.release()));

    auto masking = std::make_unique<juce::DynamicObject>();
    masking->setProperty ("schema_version", "dad.l2_masking_frames.v1");
    masking->setProperty ("status", analysis.status);
    masking->setProperty ("analyzer_revision", "dad_l2_render_probe.masking_frames.v1");
    masking->setProperty ("band_model", "vit_broad_frequency_bands.v1");
    masking->setProperty ("frame_count_total", analysis.maskingFrameCountTotal);
    masking->setProperty ("frame_count_disclosed", (int) analysis.maskingFrames.size());
    masking->setProperty ("aggregation_stride", analysis.maskingFrameAggregationStride);
    masking->setProperty ("sample_rate", analysis.sampleRate);

    juce::Array<juce::var> maskingBands;
    for (int bandIndex = 0; bandIndex < 6; ++bandIndex)
    {
        const auto& band = analysis.bands[(size_t) bandIndex];
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("id", band.name);
        row->setProperty ("min_hz", band.minHz);
        row->setProperty ("max_hz", band.maxHz);
        maskingBands.add (juce::var (row.release()));
    }
    masking->setProperty ("bands", juce::var (maskingBands));

    juce::Array<juce::var> maskingFrames;
    for (const auto& frame : analysis.maskingFrames)
    {
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("start_seconds", frame.startSeconds + request.analyzedStartSeconds);
        row->setProperty ("end_seconds", frame.endSeconds + request.analyzedStartSeconds);
        auto levels = std::make_unique<juce::DynamicObject>();
        for (int bandIndex = 0; bandIndex < 6; ++bandIndex)
            levels->setProperty (juce::Identifier (analysis.bands[(size_t) bandIndex].name),
                                 frame.bandLevelsDbfs[(size_t) bandIndex]);
        row->setProperty ("levels_dbfs", juce::var (levels.release()));
        maskingFrames.add (juce::var (row.release()));
    }
    masking->setProperty ("frames", juce::var (maskingFrames));
    obj->setProperty ("masking_frames", juce::var (masking.release()));

    auto evidence = std::make_unique<juce::DynamicObject>();
    evidence->setProperty ("nonzero", analysis.nonzeroCount > 0);
    evidence->setProperty ("sum_abs", analysis.sumAbs);
    evidence->setProperty ("max_abs", analysis.maxAbs);
    evidence->setProperty ("nan_inf_count", analysis.nanInfCount);
    evidence->setProperty ("coverage", analysis.coverage);
    evidence->setProperty ("latency_compensated", request.latencyCompensated);
    evidence->setProperty ("tail_captured", request.tailCaptured);
    evidence->setProperty ("deterministic", request.deterministic);
    obj->setProperty ("quality_evidence", juce::var (evidence.release()));

    publish (juce::JSON::toString (juce::var (obj.release())));
}

void stampCompressorDualTapIdentity (juce::DynamicObject& object,
                                     const CompressorDualTapEvidenceRequest& request)
{
    object.setProperty ("feature_family", "audio_feature");
    object.setProperty ("feature_type", "compressor_dual_tap_probe");
    object.setProperty ("schema_version", "dad.compressor_dual_tap_receipt.v1");
    object.setProperty ("pair_id", request.pairId);
    object.setProperty ("request_id", request.requestId);
    object.setProperty ("track_id", request.trackId);
    object.setProperty ("clip_id", request.clipId);
    object.setProperty ("plugin_instance_id", request.pluginInstanceId);
    object.setProperty ("plugin_position", request.pluginPosition);
    object.setProperty ("topology_class", request.topologyClass);
    object.setProperty ("topology_generation", request.topologyGeneration);
    object.setProperty ("support_class", request.supportClass);
    object.setProperty ("chain_hash", request.chainHash);
    object.setProperty ("processor_state_hash", request.processorStateHash);
    object.setProperty ("scope_revision", request.scopeRevision);
    object.setProperty ("source_revision", request.sourceRevision);
    object.setProperty ("clip_revision", request.clipRevision);
    object.setProperty ("render_revision", request.renderRevision);
    object.setProperty ("start_sample", request.startSample);
    object.setProperty ("end_sample", request.endSample);
    object.setProperty ("sample_rate", request.sampleRate);
    object.setProperty ("channel_layout", request.channelLayout);
    object.setProperty ("render_mode", "offline_probe");
    object.setProperty ("deterministic", request.deterministic);
    object.setProperty ("input_tap", "compressor_input");
    object.setProperty ("output_tap", "compressor_output");
    object.setProperty ("tail_policy", "exact_window_no_tail");
    object.setProperty ("analyzer_version", request.analyzerVersion);
    object.setProperty ("evidence_ref", "dad.compressor_dual_tap:" + request.pairId);
}

void publishCompressorDualTapBuilding (const VitProductionCoordinator::PublishFn& publish,
                                       const CompressorDualTapEvidenceRequest& request,
                                       const juce::String& jobId,
                                       const juce::String& phase)
{
    if (! publish)
        return;
    auto object = std::make_unique<juce::DynamicObject>();
    stampCompressorDualTapIdentity (*object, request);
    object->setProperty ("command", "compressor_dual_tap_probe_status");
    object->setProperty ("status", "building");
    object->setProperty ("quality_status", "building");
    object->setProperty ("job_id", jobId);
    object->setProperty ("phase", phase);
    publish (juce::JSON::toString (juce::var (object.release())));
}

void publishCompressorDualTapFailure (const VitProductionCoordinator::PublishFn& publish,
                                      const CompressorDualTapEvidenceRequest& request,
                                      const juce::String& jobId,
                                      const juce::String& reason)
{
    if (! publish)
        return;
    auto object = std::make_unique<juce::DynamicObject>();
    stampCompressorDualTapIdentity (*object, request);
    object->setProperty ("command", "compressor_dual_tap_probe_ready");
    object->setProperty ("status", "suspect");
    object->setProperty ("quality_status", "suspect");
    object->setProperty ("job_id", jobId);
    object->setProperty ("reason", reason);
    publish (juce::JSON::toString (juce::var (object.release())));
}

juce::var compressorTapQualityVar (const CompressorTapQuality& quality)
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

void publishCompressorDualTapResult (const VitProductionCoordinator::PublishFn& publish,
                                     const CompressorDualTapEvidenceRequest& request,
                                     const juce::String& jobId,
                                     const CompressorDualTapEvidenceResult& result)
{
    if (! publish)
        return;
    auto object = std::make_unique<juce::DynamicObject>();
    stampCompressorDualTapIdentity (*object, request);
    object->setProperty ("command", "compressor_dual_tap_probe_ready");
    object->setProperty ("status", result.status);
    object->setProperty ("quality_status", result.status);
    object->setProperty ("job_id", jobId);
    object->setProperty ("reason", result.reason);
    object->setProperty ("channel_count", result.channelCount);
    object->setProperty ("aligned_sample_frames", result.alignedSampleFrames);
    object->setProperty ("envelope_frame_count", result.envelopeFrameCount);
    object->setProperty ("event_candidate_count", result.eventCandidateCount);
    object->setProperty ("artifact_sha256", result.artifactSha256);
    object->setProperty ("artifact_bytes", result.artifactBytes);
    object->setProperty ("determinism_proof_status", result.determinismVerified ? "ready" : "suspect");
    object->setProperty ("determinism_max_abs_delta", result.determinismMaxAbsDelta);
    object->setProperty ("determinism_rms_delta", result.determinismRMSDelta);
    object->setProperty ("determinism_correlation", result.determinismCorrelation);
    object->setProperty ("determinism_peak_db_delta", result.determinismPeakDBDelta);
    object->setProperty ("determinism_rms_db_delta", result.determinismRMSDBDelta);

    auto alignment = std::make_unique<juce::DynamicObject>();
    alignment->setProperty ("status", result.alignmentReady ? "ready" : "suspect");
    alignment->setProperty ("method", "offline_pdc_plus_integer_cross_correlation_v1");
    alignment->setProperty ("plugin_reported_latency_samples", request.reportedLatencySamples);
    alignment->setProperty ("measured_offset_samples", result.measuredOffsetSamples);
    alignment->setProperty ("applied_offset_samples", result.appliedOffsetSamples);
    alignment->setProperty ("residual_error_samples", result.residualErrorSamples);
    alignment->setProperty ("correlation", result.alignmentCorrelation);
    object->setProperty ("latency_alignment", juce::var (alignment.release()));

    auto quality = std::make_unique<juce::DynamicObject>();
    quality->setProperty ("input", compressorTapQualityVar (result.inputQuality));
    quality->setProperty ("output", compressorTapQualityVar (result.outputQuality));
    object->setProperty ("quality_evidence", juce::var (quality.release()));
    publish (juce::JSON::toString (juce::var (object.release())));
}
}

VitProductionCoordinator::VitProductionCoordinator (PublishFn publish)
    : publishMessage (std::move (publish))
{
}

juce::String VitProductionCoordinator::startOfflineRender (te::Edit& edit,
                                                           const juce::File& destFile,
                                                           double rangeStartSeconds,
                                                           double rangeEndSeconds,
                                                           int bitDepth,
                                                           bool useMasterPlugins,
                                                           const juce::BigInteger& tracksToDo,
                                                           L2RenderProbeRequest probeRequest)
{
    if (rendering.load())
        return makeCoordinatorError ("A render job is already in progress");

    if (! destFile.getParentDirectory().exists())
        destFile.getParentDirectory().createDirectory();

    if (destFile.existsAsFile() && ! destFile.deleteFile())
        return makeCoordinatorError ("Cannot overwrite destination file");

    const auto startSec = juce::jmin (rangeStartSeconds, rangeEndSeconds);
    const auto endSec = juce::jmax (rangeStartSeconds, rangeEndSeconds);

    jobId = juce::Uuid().toString();
    const auto activeJobId = jobId;
    rendering.store (true);
    lastPublishedProgress = -1.0f;
    armRenderWatchdog (kRenderWatchdogBaseSeconds + (endSec - startSec));

    te::Renderer::Parameters params (edit);
    params.destFile = destFile;
    params.time = te::TimeRange (te::TimePosition::fromSeconds (startSec),
                                 te::TimePosition::fromSeconds (endSec));
    params.audioFormat = edit.engine.getAudioFileFormatManager().getWavFormat();
    params.bitDepth = bitDepth > 0 ? bitDepth : 24;
    params.useMasterPlugins = useMasterPlugins;
    params.tracksToDo = tracksToDo;
    if (probeRequest.enabled && probeRequest.tailSeconds > 0.0)
        params.endAllowance = te::TimeDuration::fromSeconds (probeRequest.tailSeconds);

    if (auto* device = edit.engine.getDeviceManager().deviceManager.getCurrentAudioDevice())
        params.sampleRateForAudio = device->getCurrentSampleRate();
    else
        params.sampleRateForAudio = 48000.0;

    publishL2ProbeBuilding (publishMessage, probeRequest, activeJobId);

    renderHandle = te::EditRenderer::render (
        std::move (params),
        [this,
         destPath = destFile.getFullPathName(),
         destFile,
         jid = activeJobId,
         probeRequest] (tl::expected<juce::File, std::string> res)
        {
            juce::MessageManager::callAsync (
                [this, destPath, destFile, jid, probeRequest, res]
                {
                    if (jid != jobId)
                    {
                        // Stale completion (render watchdog already force-cleared
                        // this job, or a newer job replaced it): drop its parked
                        // handle and ignore the late result.
                        releaseWedgedRenderHandle (jid);
                        return;
                    }

                    rendering.store (false);
                    renderHandle.reset();
                    lastPublishedProgress = -1.0f;

                    if (probeRequest.enabled)
                    {
                        if (res.has_value())
                        {
                            auto publish = publishMessage;
                            std::thread ([publish, probeRequest, destFile, jid]
                            {
                                const auto analysis = analyseL2ProbeFile (destFile, probeRequest);
                                publishL2ProbeAnalysis (publish, probeRequest, jid, analysis);
                                destFile.deleteFile();
                            }).detach();
                        }
                        else
                        {
                            publishL2ProbeFailure (publishMessage,
                                                   probeRequest,
                                                   jid,
                                                   juce::String (res.error()));
                            destFile.deleteFile();
                        }
                    }
                    else
                    {
                        auto obj = std::make_unique<juce::DynamicObject>();
                        obj->setProperty ("topic", "render");
                        obj->setProperty ("subtopic", res.has_value() ? "render_done" : "render_failed");
                        obj->setProperty ("job_id", jid);
                        if (res.has_value())
                        {
                            obj->setProperty ("file_path", destPath);
                            obj->setProperty ("status", "ok");
                        }
                        else
                        {
                            obj->setProperty ("status", "error");
                            obj->setProperty ("message", juce::String (res.error()));
                        }

                        if (publishMessage)
                            publishMessage (juce::JSON::toString (juce::var (obj.release())));
                    }
                });
        });

    auto reply = std::make_unique<juce::DynamicObject>();
    reply->setProperty ("status", "ok");
    reply->setProperty ("message", probeRequest.enabled ? "L2 render probe started" : "Render started");
    reply->setProperty ("job_id", jobId);
    if (probeRequest.enabled)
    {
        reply->setProperty ("cmd", "l2_render_probe");
        reply->setProperty ("feature_type", "l2_render_probe");
        reply->setProperty ("tap_point", probeRequest.tapPoint);
        reply->setProperty ("render_mode", probeRequest.renderMode);
        reply->setProperty ("track_id", probeRequest.trackId);
        reply->setProperty ("clip_id", probeRequest.clipId);
        reply->setProperty ("source_revision", probeRequest.sourceRevision);
        reply->setProperty ("clip_revision", probeRequest.clipRevision);
        reply->setProperty ("render_revision", probeRequest.renderRevision);
        reply->setProperty ("evidence_ref", "dad.l2_render_probe:" + probeRequest.renderRevision);
        if (probeRequest.analysisBandLowHz > 0.0 && probeRequest.analysisBandHighHz > probeRequest.analysisBandLowHz)
        {
            reply->setProperty ("analysis_band_low_hz", probeRequest.analysisBandLowHz);
            reply->setProperty ("analysis_band_high_hz", probeRequest.analysisBandHighHz);
            reply->setProperty ("analysis_band_id", probeRequest.analysisBandId);
        }
        reply->setProperty ("probe_status", "building");
    }
    return juce::JSON::toString (juce::var (reply.release()));
}

juce::String VitProductionCoordinator::startCompressorDualTapProbe (
    te::Edit& edit,
    CompressorDualTapProbeRequest request)
{
    if (rendering.load())
        return makeCoordinatorError ("A render job is already in progress");

    const auto& evidence = request.evidence;
    if (evidence.sampleRate <= 0.0 || evidence.endSample <= evidence.startSample
        || request.tracksToDo.countNumberOfSetBits() != 1)
        return makeCoordinatorError ("Invalid compressor dual-tap render request");

    request.inputRenderFile.getParentDirectory().createDirectory();
    request.outputRenderFile.getParentDirectory().createDirectory();
    request.artifactDirectory.createDirectory();
    for (const auto& file : { request.inputRenderFile, request.outputRenderFile, request.outputVerificationRenderFile })
        if (file.existsAsFile() && ! file.deleteFile())
            return makeCoordinatorError ("Cannot overwrite compressor dual-tap temporary render");

    jobId = juce::Uuid().toString();
    const auto activeJobId = jobId;
    rendering.store (true);
    lastPublishedProgress = -1.0f;
    armRenderWatchdog (kRenderWatchdogBaseSeconds
                       + ((double) (evidence.endSample - evidence.startSample) / evidence.sampleRate));

    auto makeParameters = [&edit, &request] (const juce::File& destination, bool usePlugins)
    {
        te::Renderer::Parameters params (edit);
        params.destFile = destination;
        params.time = te::TimeRange (
            te::TimePosition::fromSeconds ((double) request.evidence.startSample / request.evidence.sampleRate),
            te::TimePosition::fromSeconds ((double) request.evidence.endSample / request.evidence.sampleRate));
        params.audioFormat = edit.engine.getAudioFileFormatManager().getWavFormat();
        params.bitDepth = 32;
        params.sampleRateForAudio = request.evidence.sampleRate;
        params.blockSizeForAudio = 512;
        params.tracksToDo = request.tracksToDo;
        params.usePlugins = usePlugins;
        params.useMasterPlugins = false;
        params.canRenderInMono = false;
        params.mustRenderInMono = false;
        params.trimSilenceAtEnds = false;
        params.shouldNormalise = false;
        params.shouldNormaliseByRMS = false;
        params.ditheringEnabled = false;
        return params;
    };

    publishCompressorDualTapBuilding (publishMessage, evidence, activeJobId, "render_input");
    renderHandle = te::EditRenderer::render (
        makeParameters (request.inputRenderFile, false),
        [this, &edit, request, activeJobId] (tl::expected<juce::File, std::string> inputResult)
        {
            juce::MessageManager::callAsync (
                [this, &edit, request, activeJobId, inputResult]
                {
                    if (! inputResult.has_value())
                    {
                        rendering.store (false);
                        renderHandle.reset();
                        lastPublishedProgress = -1.0f;
                        publishCompressorDualTapFailure (publishMessage, request.evidence, activeJobId,
                                                         "input_render_failed:" + juce::String (inputResult.error()));
                        request.inputRenderFile.deleteFile();
                        request.outputRenderFile.deleteFile();
                        request.outputVerificationRenderFile.deleteFile();
                        return;
                    }

                    publishCompressorDualTapBuilding (publishMessage, request.evidence, activeJobId, "render_output");
                    te::Renderer::Parameters params (edit);
                    params.destFile = request.outputRenderFile;
                    params.time = te::TimeRange (
                        te::TimePosition::fromSeconds ((double) request.evidence.startSample / request.evidence.sampleRate),
                        te::TimePosition::fromSeconds ((double) request.evidence.endSample / request.evidence.sampleRate));
                    params.audioFormat = edit.engine.getAudioFileFormatManager().getWavFormat();
                    params.bitDepth = 32;
                    params.sampleRateForAudio = request.evidence.sampleRate;
                    params.blockSizeForAudio = 512;
                    params.tracksToDo = request.tracksToDo;
                    params.usePlugins = true;
                    params.useMasterPlugins = false;
                    params.canRenderInMono = false;
                    params.mustRenderInMono = false;
                    params.trimSilenceAtEnds = false;
                    params.shouldNormalise = false;
                    params.shouldNormaliseByRMS = false;
                    params.ditheringEnabled = false;

                    renderHandle = te::EditRenderer::render (
                        std::move (params),
                        [this, &edit, request, activeJobId] (tl::expected<juce::File, std::string> outputResult)
                        {
                            juce::MessageManager::callAsync (
                                [this, &edit, request, activeJobId, outputResult]
                                {
                                    if (! outputResult.has_value())
                                    {
                                        rendering.store (false);
                                        renderHandle.reset();
                                        lastPublishedProgress = -1.0f;
                                        publishCompressorDualTapFailure (publishMessage, request.evidence, activeJobId,
                                                                         "output_render_failed:" + juce::String (outputResult.error()));
                                        request.inputRenderFile.deleteFile();
                                        request.outputRenderFile.deleteFile();
                                        request.outputVerificationRenderFile.deleteFile();
                                        return;
                                    }

                                    publishCompressorDualTapBuilding (publishMessage, request.evidence, activeJobId, "render_verify");
                                    te::Renderer::Parameters verifyParams (edit);
                                    verifyParams.destFile = request.outputVerificationRenderFile;
                                    verifyParams.time = te::TimeRange (
                                        te::TimePosition::fromSeconds ((double) request.evidence.startSample / request.evidence.sampleRate),
                                        te::TimePosition::fromSeconds ((double) request.evidence.endSample / request.evidence.sampleRate));
                                    verifyParams.audioFormat = edit.engine.getAudioFileFormatManager().getWavFormat();
                                    verifyParams.bitDepth = 32;
                                    verifyParams.sampleRateForAudio = request.evidence.sampleRate;
                                    verifyParams.blockSizeForAudio = 512;
                                    verifyParams.tracksToDo = request.tracksToDo;
                                    verifyParams.usePlugins = true;
                                    verifyParams.useMasterPlugins = false;
                                    verifyParams.canRenderInMono = false;
                                    verifyParams.mustRenderInMono = false;
                                    verifyParams.trimSilenceAtEnds = false;
                                    verifyParams.shouldNormalise = false;
                                    verifyParams.shouldNormaliseByRMS = false;
                                    verifyParams.ditheringEnabled = false;

                                    renderHandle = te::EditRenderer::render (
                                        std::move (verifyParams),
                                        [this, request, activeJobId] (tl::expected<juce::File, std::string> verificationResult)
                                        {
                                            juce::MessageManager::callAsync (
                                                [this, request, activeJobId, verificationResult]
                                                {
                                                    rendering.store (false);
                                                    renderHandle.reset();
                                                    lastPublishedProgress = -1.0f;
                                                    if (! verificationResult.has_value())
                                                    {
                                                        publishCompressorDualTapFailure (publishMessage, request.evidence, activeJobId,
                                                                                         "verification_render_failed:" + juce::String (verificationResult.error()));
                                                        request.inputRenderFile.deleteFile();
                                                        request.outputRenderFile.deleteFile();
                                                        request.outputVerificationRenderFile.deleteFile();
                                                        return;
                                                    }
                                                    const auto currentRevision = request.readCurrentScopeRevision != nullptr
                                                        ? request.readCurrentScopeRevision() : juce::String();
                                                    if (currentRevision.isEmpty() || currentRevision != request.evidence.scopeRevision)
                                                    {
                                                        publishCompressorDualTapFailure (publishMessage, request.evidence, activeJobId,
                                                                                         "scope_revision_changed_during_capture");
                                                        request.inputRenderFile.deleteFile();
                                                        request.outputRenderFile.deleteFile();
                                                        request.outputVerificationRenderFile.deleteFile();
                                                        return;
                                                    }
                                                    auto publish = publishMessage;
                                                    std::thread ([publish, request, activeJobId]
                                                    {
                                                        const auto result = analyseAndWriteCompressorDualTapEvidence (
                                                            request.inputRenderFile,
                                                            request.outputRenderFile,
                                                            request.outputVerificationRenderFile,
                                                            request.artifactDirectory,
                                                            request.evidence);
                                                        publishCompressorDualTapResult (publish, request.evidence, activeJobId, result);
                                                        request.inputRenderFile.deleteFile();
                                                        request.outputRenderFile.deleteFile();
                                                        request.outputVerificationRenderFile.deleteFile();
                                                    }).detach();
                                                });
                                        });
                                });
                        });
                });
        });

    auto reply = std::make_unique<juce::DynamicObject>();
    reply->setProperty ("status", "ok");
    reply->setProperty ("message", "Compressor dual-tap probe started");
    reply->setProperty ("cmd", "compressor_dual_tap_probe");
    reply->setProperty ("feature_type", "compressor_dual_tap_probe");
    reply->setProperty ("job_id", activeJobId);
    reply->setProperty ("pair_id", evidence.pairId);
    reply->setProperty ("evidence_ref", "dad.compressor_dual_tap:" + evidence.pairId);
    reply->setProperty ("probe_status", "building");
    return juce::JSON::toString (juce::var (reply.release()));
}

void VitProductionCoordinator::cancelOfflineRender()
{
    if (renderHandle != nullptr)
        renderHandle->cancel();
}

void VitProductionCoordinator::armRenderWatchdog (double timeoutSeconds)
{
    const auto deadlineMs = steadyClockNowMs()
        + (int64_t) (juce::jmax (0.05, timeoutSeconds) * 1000.0);
    renderWatchdogDeadlineMs.store (deadlineMs);
}

void VitProductionCoordinator::releaseWedgedRenderHandle (const juce::String& handleJobId)
{
    wedgedRenderHandles.erase (
        std::remove_if (wedgedRenderHandles.begin(), wedgedRenderHandles.end(),
                        [&handleJobId] (const auto& entry) { return entry.first == handleJobId; }),
        wedgedRenderHandles.end());
}

void VitProductionCoordinator::checkRenderWatchdog()
{
    const auto deadlineMs = renderWatchdogDeadlineMs.load();
    if (deadlineMs <= 0 || ! rendering.load())
        return;

    if (steadyClockNowMs() < deadlineMs)
        return;

    // Timeout: the EditRenderer completion callback will never arrive (e.g. a
    // blocked third-party plugin wedged the render thread). Cancel the job,
    // force the rendering flag down and publish so later render commands are
    // accepted again (self-heal). The wedged handle is parked, never destroyed:
    // its destructor joins the render thread, which would hang the message
    // thread. Clearing jobId makes any late completion callback a stale no-op.
    const auto wedgedJobId = jobId;
    const auto overshootSeconds = (steadyClockNowMs() - deadlineMs) / 1000.0;
    renderWatchdogDeadlineMs.store (0);
    jobId = juce::String();
    if (renderHandle != nullptr)
    {
        renderHandle->cancel();
        wedgedRenderHandles.emplace_back (wedgedJobId, std::move (renderHandle));
    }
    lastPublishedProgress = -1.0f;
    rendering.store (false);

    juce::Logger::writeToLog (
        "WARN [render_watchdog] render job " + wedgedJobId + " missed its deadline by "
        + juce::String (overshootSeconds, 1) + "s: cancel requested, rendering flag force-cleared, "
          "subsequent render commands are accepted again");

    if (publishMessage)
    {
        auto object = std::make_unique<juce::DynamicObject>();
        object->setProperty ("topic", "render");
        object->setProperty ("subtopic", "render_failed");
        object->setProperty ("job_id", wedgedJobId);
        object->setProperty ("status", "error");
        object->setProperty ("source", "render_watchdog");
        object->setProperty ("message", "render watchdog timeout: render cancelled, engine render "
                                         "state force-cleared (self-heal)");
        publishMessage (juce::JSON::toString (juce::var (object.release())));
    }
}

void VitProductionCoordinator::simulateWedgedRenderForTest (double watchdogTimeoutSeconds)
{
    jobId = juce::Uuid().toString();
    renderHandle.reset();
    lastPublishedProgress = -1.0f;
    rendering.store (true);
    armRenderWatchdog (watchdogTimeoutSeconds);
}

void VitProductionCoordinator::tick()
{
    checkRenderWatchdog();

    if (! rendering.load() || renderHandle == nullptr)
        return;

    const float p = renderHandle->getProgress();
    if (p >= 0.0f && std::abs (p - lastPublishedProgress) >= 0.01f)
    {
        lastPublishedProgress = p;
        auto obj = std::make_unique<juce::DynamicObject>();
        obj->setProperty ("topic", "render");
        obj->setProperty ("subtopic", "render_progress");
        obj->setProperty ("job_id", jobId);
        obj->setProperty ("progress", p);
        if (publishMessage)
            publishMessage (juce::JSON::toString (juce::var (obj.release())));
    }
}

} // namespace vit
