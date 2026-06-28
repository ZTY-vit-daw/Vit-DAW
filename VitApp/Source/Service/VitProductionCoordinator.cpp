#include "VitProductionCoordinator.h"

#include <array>
#include <cmath>
#include <limits>
#include <thread>
#include <vector>

namespace vit
{

namespace
{
constexpr double kSilenceDb = -160.0;

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
    const char* name = "";
    double minHz = 0.0;
    double maxHz = 0.0;
    double energy = 0.0;
    int binHits = 0;
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
    juce::String status = "suspect";
    juce::String reason;
    std::array<L2BandSummary, 6> bands {{
        { "sub",      20.0,    60.0,    0.0, 0 },
        { "bass",     60.0,    250.0,   0.0, 0 },
        { "low_mid",  250.0,   500.0,   0.0, 0 },
        { "mid",      500.0,   2000.0,  0.0, 0 },
        { "presence", 2000.0,  6000.0,  0.0, 0 },
        { "air",      6000.0,  20000.0, 0.0, 0 }
    }};
};

int bandIndexForFrequency (double hz, const std::array<L2BandSummary, 6>& bands)
{
    for (int i = 0; i < (int) bands.size(); ++i)
        if (hz >= bands[(size_t) i].minHz && hz < bands[(size_t) i].maxHz)
            return i;

    return -1;
}

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
    obj.setProperty ("track_id", request.trackId);
    obj.setProperty ("clip_id", request.clipId);
    obj.setProperty ("source_path", request.sourcePath);
    obj.setProperty ("source_revision", request.sourceRevision);
    obj.setProperty ("clip_revision", request.clipRevision);
    obj.setProperty ("render_revision", request.renderRevision);
    obj.setProperty ("evidence_ref", "dad.l2_render_probe:" + request.renderRevision);
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

        for (int bin = 1; bin < fftBins; ++bin)
        {
            const auto re = fftData[(size_t) bin * 2];
            const auto im = fftData[(size_t) bin * 2 + 1];
            const double mag2 = (double) re * (double) re + (double) im * (double) im;
            if (! std::isfinite (mag2) || mag2 <= 0.0)
                continue;

            const double hz = ((double) bin * out.sampleRate) / (double) fftSize;
            const int bandIndex = bandIndexForFrequency (hz, out.bands);
            if (bandIndex < 0)
                continue;

            out.bands[(size_t) bandIndex].energy += mag2;
            out.bands[(size_t) bandIndex].binHits += 1;
            out.spectralEnergy += mag2;
        }
    }

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
        bandsObject->setProperty (band.name, juce::var (bandObject.release()));
    }
    obj->setProperty ("bands", juce::var (bandsObject.release()));

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
        reply->setProperty ("probe_status", "building");
    }
    return juce::JSON::toString (juce::var (reply.release()));
}

void VitProductionCoordinator::cancelOfflineRender()
{
    if (renderHandle != nullptr)
        renderHandle->cancel();
}

void VitProductionCoordinator::tick()
{
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
