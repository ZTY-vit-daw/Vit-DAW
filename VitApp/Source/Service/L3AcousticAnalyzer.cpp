#include "L3AcousticAnalyzer.h"

#include "../Core/VitPaths.h"
#include "OfflineAudioReadCoordinator.h"

#include <array>
#include <cmath>
#include <fstream>
#include <limits>
#include <vector>

namespace vit
{
namespace
{

constexpr const char* kAnalyzerVersion = "dad_l3_offline_analyzer.v1";
constexpr double kSilenceDb = -160.0;

void writeDiagLog (const juce::String& line);

struct BandDefinition
{
    const char* name = "";
    double minHz = 0.0;
    double maxHz = 0.0;
};

struct BandAccumulator
{
    BandDefinition definition;
    double energy = 0.0;
    int64 frameCount = 0;
};

struct L3Evidence
{
    double sampleRate = 0.0;
    int channels = 0;
    double durationSeconds = 0.0;
    int64 expectedSampleCount = 0;
    int64 analyzedSampleCount = 0;
    double coverageRatio = 0.0;
    int64 nonzeroCount = 0;
    double sumAbs = 0.0;
    double maxAbs = 0.0;
    int64 nanCount = 0;
    int64 infCount = 0;
    juce::String status = "suspect";
    juce::StringArray reasons;

    void observe (float value)
    {
        ++analyzedSampleCount;
        if (std::isnan (value))
        {
            ++nanCount;
            return;
        }
        if (std::isinf (value))
        {
            ++infCount;
            return;
        }

        const auto absValue = std::abs ((double) value);
        if (absValue > 0.0)
            ++nonzeroCount;
        sumAbs += absValue;
        maxAbs = juce::jmax (maxAbs, absValue);
    }
};

struct L3Analysis
{
    L3Evidence evidence;
    std::array<BandAccumulator, 6> bands {{
        { { "sub",      20.0,    60.0    }, 0.0, 0 },
        { { "bass",     60.0,    250.0   }, 0.0, 0 },
        { { "low_mid",  250.0,   500.0   }, 0.0, 0 },
        { { "mid",      500.0,   2000.0  }, 0.0, 0 },
        { { "presence", 2000.0,  6000.0  }, 0.0, 0 },
        { { "air",      6000.0,  20000.0 }, 0.0, 0 }
    }};
    double totalBandEnergy = 0.0;
    int64 fftFrameCount = 0;
    int64 audioFrameCount = 0;
    double leftEnergy = 0.0;
    double rightEnergy = 0.0;
    double sumLR = 0.0;
    double peakAbs = 0.0;
    double sumSquares = 0.0;
    double rms = 0.0;
    double leftRms = 0.0;
    double rightRms = 0.0;
    double balanceDb = 0.0;
    double correlation = 1.0;
};

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

int bandIndexForHz (double hz, const std::array<BandAccumulator, 6>& bands)
{
    for (int i = 0; i < (int) bands.size(); ++i)
        if (hz >= bands[(size_t) i].definition.minHz && hz < bands[(size_t) i].definition.maxHz)
            return i;
    return -1;
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

juce::String phaseRiskForCorrelation (double value)
{
    if (! std::isfinite (value))
        return "unknown";
    if (value < -0.2)
        return "high";
    if (value < 0.25)
        return "medium";
    return "low";
}

juce::String monoCompatibilityRisk (double value)
{
    if (! std::isfinite (value))
        return "unknown";
    if (value < 0.0)
        return "high";
    if (value < 0.35)
        return "medium";
    return "low";
}

void writeDiagLog (const juce::String& line)
{
    juce::Logger::writeToLog (line);
    const auto logsDir = paths::getLogsDirectory();
    paths::ensureDirectoryExists (logsDir, "logs");
    const auto diagLogFile = logsDir.getChildFile ("l3_acoustic_analyzer_diag.log");
    std::ofstream out (diagLogFile.getFullPathName().toStdString(), std::ios::app);
    if (out.is_open())
        out << line.toStdString() << std::endl;
}

void finishEvidence (L3Analysis& analysis, const juce::String& extraReason = {})
{
    auto& e = analysis.evidence;
    e.coverageRatio = e.expectedSampleCount > 0
        ? juce::jlimit (0.0, 1.0, (double) e.analyzedSampleCount / (double) e.expectedSampleCount)
        : 0.0;

    if (extraReason.isNotEmpty())
        e.reasons.addIfNotAlreadyThere (extraReason);
    if (e.expectedSampleCount <= 0 || e.analyzedSampleCount <= 0)
        e.reasons.addIfNotAlreadyThere ("empty_coverage");
    if (e.nanCount > 0 || e.infCount > 0)
        e.reasons.addIfNotAlreadyThere ("nan_or_inf_detected");
    if (e.nonzeroCount <= 0 || e.sumAbs <= 0.0 || e.maxAbs <= 0.0)
        e.reasons.addIfNotAlreadyThere ("all_zero_audio");
    if (e.coverageRatio < 0.95)
        e.reasons.addIfNotAlreadyThere ("coverage_below_threshold");

    e.status = e.reasons.isEmpty() ? "ready" : "suspect";
    if (e.reasons.isEmpty())
        e.reasons.add ("ok");

    if (analysis.audioFrameCount > 0)
    {
        const auto denom = (double) analysis.audioFrameCount;
        analysis.rms = std::sqrt (analysis.sumSquares / (denom * (double) juce::jmax (1, e.channels)));
        analysis.leftRms = std::sqrt (analysis.leftEnergy / denom);
        analysis.rightRms = std::sqrt (analysis.rightEnergy / denom);
        analysis.balanceDb = dbFromLinear (analysis.rightRms) - dbFromLinear (analysis.leftRms);
        const auto corrDenom = std::sqrt (analysis.leftEnergy * analysis.rightEnergy);
        analysis.correlation = corrDenom > 0.0 ? juce::jlimit (-1.0, 1.0, analysis.sumLR / corrDenom) : 1.0;
    }
}

std::unique_ptr<juce::DynamicObject> makeSourceIdentity (const AudioFeatureBakeRequest& request,
                                                         const L3Evidence& evidence)
{
    auto identity = std::make_unique<juce::DynamicObject>();
	identity->setProperty ("project_id", request.projectId.isNotEmpty() ? request.projectId : "current");
	if (request.projectId.isNotEmpty())
		identity->setProperty ("project_uuid", request.projectId);
    identity->setProperty ("track_id", request.trackId);
    identity->setProperty ("clip_id", request.clipId);
    identity->setProperty ("source_path", request.filePath);
    if (request.sourceId.isNotEmpty())
        identity->setProperty ("source_id", request.sourceId);
    if (request.sourceRevision.isNotEmpty())
    {
        identity->setProperty ("source_revision", request.sourceRevision);
        identity->setProperty ("source_fingerprint", request.sourceRevision);
    }
    if (request.clipRevision.isNotEmpty())
        identity->setProperty ("clip_revision", request.clipRevision);
    if (request.renderRevision.isNotEmpty())
        identity->setProperty ("render_revision", request.renderRevision);
    identity->setProperty ("analyzer_version", kAnalyzerVersion);
    identity->setProperty ("analyzer_revision", kAnalyzerVersion);
    identity->setProperty ("duration_seconds", evidence.durationSeconds);
    identity->setProperty ("sample_rate", evidence.sampleRate);
    identity->setProperty ("channels", evidence.channels);
    return identity;
}

juce::Array<juce::var> qualityReasonsVar (const L3Evidence& evidence)
{
    juce::Array<juce::var> reasons;
    for (const auto& reason : evidence.reasons)
        reasons.add (reason);
    return reasons;
}

std::unique_ptr<juce::DynamicObject> makeEvidenceObject (const L3Evidence& evidence,
                                                         const juce::String& evidenceRef)
{
    auto obj = std::make_unique<juce::DynamicObject>();
    obj->setProperty ("source_identity", "source_revision+clip_revision+optional_render_revision");
    obj->setProperty ("analyzer_version", kAnalyzerVersion);
    obj->setProperty ("sample_rate", evidence.sampleRate);
    obj->setProperty ("channels", evidence.channels);
    obj->setProperty ("duration_seconds", evidence.durationSeconds);
    obj->setProperty ("expected_sample_count", (int64) evidence.expectedSampleCount);
    obj->setProperty ("analyzed_sample_count", (int64) evidence.analyzedSampleCount);
    obj->setProperty ("coverage_ratio", evidence.coverageRatio);
    obj->setProperty ("nonzero_count", (int64) evidence.nonzeroCount);
    obj->setProperty ("sum_abs", evidence.sumAbs);
    obj->setProperty ("max_abs", evidence.maxAbs);
    obj->setProperty ("nan_count", (int64) evidence.nanCount);
    obj->setProperty ("inf_count", (int64) evidence.infCount);
    obj->setProperty ("nan_inf_count", (int64) (evidence.nanCount + evidence.infCount));
    obj->setProperty ("quality_status", evidence.status);
    obj->setProperty ("quality_reasons", juce::var (qualityReasonsVar (evidence)));
    obj->setProperty ("evidence_ref", evidenceRef);
    return obj;
}

void stampCommon (juce::DynamicObject& obj,
                  const AudioFeatureBakeRequest& request,
                  const L3Analysis& analysis,
                  AudioFeatureType featureType)
{
    const auto featureName = audioFeatureTypeToString (featureType);
    const auto evidenceRef = "dad.l3." + featureName + ":"
        + (request.sourceRevision.isNotEmpty() ? request.sourceRevision : request.filePath);

    obj.setProperty ("command", "audio_feature_data_ready");
    obj.setProperty ("schema_version", "dad_l3_" + featureName + ".v1");
    obj.setProperty ("feature_family", "audio_feature");
    obj.setProperty ("feature_type", featureName);
    obj.setProperty ("feature_version", audioFeatureProductVersion (featureType));
    obj.setProperty ("analysis_version", audioFeatureAnalysisVersion());
    obj.setProperty ("layer", "l3_deep");
    obj.setProperty ("source", "kernel_l3_offline_analyzer");
    obj.setProperty ("source_kind", request.clipId.isNotEmpty() ? "clip" : "file");
    obj.setProperty ("capture_mode", "offline_full_song");
    obj.setProperty ("time_basis", "source_samples");
    obj.setProperty ("status", analysis.evidence.status);
    obj.setProperty ("quality_status", analysis.evidence.status);
    obj.setProperty ("quality_reason", analysis.evidence.reasons[0]);
    obj.setProperty ("quality_reasons", juce::var (qualityReasonsVar (analysis.evidence)));
    obj.setProperty ("ready", analysis.evidence.status == "ready");
	obj.setProperty ("project_id", request.projectId.isNotEmpty() ? request.projectId : "current");
	if (request.projectId.isNotEmpty())
		obj.setProperty ("project_uuid", request.projectId);
	if (request.projectPath.isNotEmpty())
		obj.setProperty ("project_path", request.projectPath);
    obj.setProperty ("track_id", request.trackId);
    obj.setProperty ("source_track_id", request.trackId);
    obj.setProperty ("clip_id", request.clipId);
    obj.setProperty ("source_path", request.filePath);
    obj.setProperty ("file_path", request.filePath);
    if (request.sourceId.isNotEmpty())
        obj.setProperty ("source_id", request.sourceId);
    if (request.sourceRevision.isNotEmpty())
    {
        obj.setProperty ("source_revision", request.sourceRevision);
        obj.setProperty ("source_fingerprint", request.sourceRevision);
    }
    if (request.clipRevision.isNotEmpty())
        obj.setProperty ("clip_revision", request.clipRevision);
    if (request.renderRevision.isNotEmpty())
        obj.setProperty ("render_revision", request.renderRevision);
    if (request.requestId.isNotEmpty())
        obj.setProperty ("request_id", request.requestId);
    obj.setProperty ("analyzer_version", kAnalyzerVersion);
    obj.setProperty ("analyzer_revision", kAnalyzerVersion);
    obj.setProperty ("sample_rate", analysis.evidence.sampleRate);
    obj.setProperty ("channels", analysis.evidence.channels);
    obj.setProperty ("channel_count", analysis.evidence.channels);
    obj.setProperty ("duration_seconds", analysis.evidence.durationSeconds);
    obj.setProperty ("expected_sample_count", (int64) analysis.evidence.expectedSampleCount);
    obj.setProperty ("analyzed_sample_count", (int64) analysis.evidence.analyzedSampleCount);
    obj.setProperty ("sample_count", (int64) analysis.evidence.analyzedSampleCount);
    obj.setProperty ("coverage_ratio", analysis.evidence.coverageRatio);
    obj.setProperty ("coverage_seconds", analysis.evidence.durationSeconds * analysis.evidence.coverageRatio);
    obj.setProperty ("nonzero_count", (int64) analysis.evidence.nonzeroCount);
    obj.setProperty ("sum_abs", analysis.evidence.sumAbs);
    obj.setProperty ("max_abs", analysis.evidence.maxAbs);
    obj.setProperty ("nan_count", (int64) analysis.evidence.nanCount);
    obj.setProperty ("inf_count", (int64) analysis.evidence.infCount);
    obj.setProperty ("nan_inf_count", (int64) (analysis.evidence.nanCount + analysis.evidence.infCount));
    obj.setProperty ("source_identity", juce::var (makeSourceIdentity (request, analysis.evidence).release()));
    obj.setProperty ("quality_evidence", juce::var (makeEvidenceObject (analysis.evidence, evidenceRef).release()));
    obj.setProperty ("evidence_ref", evidenceRef);
    obj.setProperty ("updated_at", juce::Time::getCurrentTime().toISO8601 (true));
}

void publishBandSummary (const AudioFeatureBakeRequest& request,
                         const L3Analysis& analysis,
                         const L3AcousticAnalyzer::PublishCallback& publish)
{
    if (! publish)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampCommon (*obj, request, analysis, AudioFeatureType::BandEnergySummary);
    obj->setProperty ("frame_count", (int64) analysis.fftFrameCount);
    obj->setProperty ("frequency_basis", "fft_hann_mono_sum");

    auto bandsObject = std::make_unique<juce::DynamicObject>();
    for (const auto& band : analysis.bands)
    {
        auto bandObject = std::make_unique<juce::DynamicObject>();
        const double relative = analysis.totalBandEnergy > 0.0
            ? juce::jlimit (0.0, 1.0, band.energy / analysis.totalBandEnergy)
            : 0.0;
        bandObject->setProperty ("status", band.frameCount > 0 ? analysis.evidence.status : "missing");
        bandObject->setProperty ("energy", band.energy);
        bandObject->setProperty ("energy_db", dbFromEnergy (band.energy));
        bandObject->setProperty ("unit_energy", relative);
        bandObject->setProperty ("relative_distribution", relative);
        bandObject->setProperty ("coverage", analysis.evidence.coverageRatio);
        bandObject->setProperty ("coverage_ratio", analysis.evidence.coverageRatio);
        bandObject->setProperty ("frame_count", (int64) band.frameCount);
        bandObject->setProperty ("sample_count", (int64) analysis.evidence.analyzedSampleCount);
        bandObject->setProperty ("min_hz", band.definition.minHz);
        bandObject->setProperty ("max_hz", band.definition.maxHz);
        bandObject->setProperty ("quality_status", analysis.evidence.status);
        bandObject->setProperty ("evidence_ref", "dad.l3.band_energy_summary:" + juce::String (band.definition.name));
        bandsObject->setProperty (band.definition.name, juce::var (bandObject.release()));
    }
    obj->setProperty ("bands", juce::var (bandsObject.release()));
    obj->setProperty ("band_count", (int) analysis.bands.size());

	const auto payload = juce::JSON::toString (juce::var (obj.release()), true);
	publish (payload);
}

void publishStereoSummary (const AudioFeatureBakeRequest& request,
                           const L3Analysis& analysis,
                           const L3AcousticAnalyzer::PublishCallback& publish)
{
    if (! publish)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampCommon (*obj, request, analysis, AudioFeatureType::StereoRelationSummary);
    obj->setProperty ("left_energy", analysis.leftEnergy);
    obj->setProperty ("right_energy", analysis.rightEnergy);
    obj->setProperty ("left_level_db", dbFromLinear (analysis.leftRms));
    obj->setProperty ("right_level_db", dbFromLinear (analysis.rightRms));
    obj->setProperty ("balance_db", analysis.balanceDb);
    obj->setProperty ("balance_state", balanceStateForDb (analysis.balanceDb));
    obj->setProperty ("correlation_estimate", analysis.correlation);
    obj->setProperty ("correlation_state", correlationStateForValue (analysis.correlation));
    obj->setProperty ("phase_risk", phaseRiskForCorrelation (analysis.correlation));
    obj->setProperty ("mono_compatibility_risk", monoCompatibilityRisk (analysis.correlation));
    obj->setProperty ("frame_count", (int64) analysis.audioFrameCount);

	const auto payload = juce::JSON::toString (juce::var (obj.release()), true);
	publish (payload);
}

void publishLoudnessSummary (const AudioFeatureBakeRequest& request,
                             const L3Analysis& analysis,
                             const L3AcousticAnalyzer::PublishCallback& publish)
{
    if (! publish)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    stampCommon (*obj, request, analysis, AudioFeatureType::LoudnessSummary);
    const auto peakDb = dbFromLinear (analysis.peakAbs);
    const auto rmsDb = dbFromLinear (analysis.rms);
    obj->setProperty ("peak", analysis.peakAbs);
    obj->setProperty ("peak_abs", analysis.peakAbs);
    obj->setProperty ("peak_dbfs", peakDb);
    obj->setProperty ("rms", analysis.rms);
    obj->setProperty ("rms_dbfs", rmsDb);
    obj->setProperty ("integrated_lufs", rmsDb - 0.691);
    obj->setProperty ("approximate_lufs", rmsDb - 0.691);
    obj->setProperty ("approximate", true);
    obj->setProperty ("algorithm", "approximate_rms_lufs_v1");
    obj->setProperty ("crest_factor", analysis.rms > 0.0 ? analysis.peakAbs / analysis.rms : 0.0);
    obj->setProperty ("crest_db", peakDb - rmsDb);

	const auto payload = juce::JSON::toString (juce::var (obj.release()), true);
	publish (payload);
}

L3Analysis analyzeFile (const AudioFeatureBakeRequest& request)
{
    L3Analysis analysis;

    const auto leaseWaitStartMs = juce::Time::getMillisecondCounterHiRes();
    auto sourceReadLease = OfflineAudioReadCoordinator::acquire (request.filePath);
    writeDiagLog ("[dad_l3] source_read_lease track_id=" + request.trackId
                  + " wait_ms=" + juce::String (juce::Time::getMillisecondCounterHiRes() - leaseWaitStartMs, 2)
                  + " file=" + request.filePath);

    juce::AudioFormatManager fm;
    fm.registerBasicFormats();
    const auto readerOpenStartMs = juce::Time::getMillisecondCounterHiRes();
    std::unique_ptr<juce::AudioFormatReader> reader (fm.createReaderFor (juce::File (request.filePath)));
    writeDiagLog ("[dad_l3] reader_open track_id=" + request.trackId
                  + " elapsed_ms=" + juce::String (juce::Time::getMillisecondCounterHiRes() - readerOpenStartMs, 2)
                  + " reader_ok=" + juce::String (reader != nullptr ? "true" : "false")
                  + " file=" + request.filePath);
    if (reader == nullptr || reader->sampleRate <= 0.0 || reader->lengthInSamples <= 0)
    {
        finishEvidence (analysis, "reader_failed");
        return analysis;
    }

    const double sr = reader->sampleRate;
    const int channelCount = juce::jmax (1, (int) reader->numChannels);
    const int channelsForEvidence = juce::jmin (2, channelCount);
    const int64 sourceStartSample = juce::jlimit<int64> (
        0,
        reader->lengthInSamples,
        (int64) std::floor (juce::jmax (0.0, request.range.sourceOffsetSeconds) * sr));
    const int64 availableSamples = juce::jmax<int64> (0, reader->lengthInSamples - sourceStartSample);
    int64 totalSamples = availableSamples;
    if (request.range.lengthSeconds > 0.0)
        totalSamples = juce::jmin<int64> (availableSamples, (int64) std::ceil (request.range.lengthSeconds * sr));

    analysis.evidence.sampleRate = sr;
    analysis.evidence.channels = channelCount;
    analysis.evidence.durationSeconds = totalSamples > 0 ? (double) totalSamples / sr : 0.0;
    analysis.evidence.expectedSampleCount = totalSamples * (int64) channelsForEvidence;

    if (totalSamples <= 0)
    {
        finishEvidence (analysis, "empty_range");
        return analysis;
    }

    constexpr int fftOrder = 12;
    constexpr int fftSize = 1 << fftOrder;
    constexpr int fftBins = fftSize / 2;
    juce::dsp::FFT fft (fftOrder);
    juce::dsp::WindowingFunction<float> window (fftSize,
                                                juce::dsp::WindowingFunction<float>::hann,
                                                false);
    juce::AudioBuffer<float> buffer (2, fftSize);
    std::vector<float> fftData ((size_t) fftSize * 2, 0.0f);

    const auto progressIntervalSamples = juce::jmax<int64> (fftSize, (int64) std::llround (sr * 60.0));
    int64 nextProgressSample = 0;
    for (int64 pos = 0; pos < totalSamples; pos += fftSize)
    {
        const int valid = (int) juce::jmin<int64> ((int64) fftSize, totalSamples - pos);
        if (valid <= 0)
            break;

        const bool logReadProgress = pos >= nextProgressSample || pos + valid >= totalSamples;
        if (logReadProgress)
        {
            writeDiagLog ("[dad_l3] reader_read_begin track_id=" + request.trackId
                          + " sample=" + juce::String ((int64) (sourceStartSample + pos))
                          + " valid=" + juce::String (valid)
                          + " total_samples=" + juce::String ((int64) totalSamples));
            nextProgressSample = pos + progressIntervalSamples;
        }

        buffer.clear();
        reader->read (&buffer, 0, valid, sourceStartSample + pos, true, true);
        if (logReadProgress)
            writeDiagLog ("[dad_l3] reader_read_end track_id=" + request.trackId
                          + " sample=" + juce::String ((int64) (sourceStartSample + pos)));
        const auto* left = buffer.getReadPointer (0);
        const auto* right = buffer.getReadPointer (juce::jmin (1, buffer.getNumChannels() - 1));

        std::fill (fftData.begin(), fftData.end(), 0.0f);
        for (int i = 0; i < valid; ++i)
        {
            const float lv = left[i];
            const float rv = channelCount > 1 ? right[i] : lv;

            analysis.evidence.observe (lv);
            if (channelsForEvidence > 1)
                analysis.evidence.observe (rv);

            if (std::isfinite (lv) && std::isfinite (rv))
            {
                const auto dl = (double) lv;
                const auto dr = (double) rv;
                analysis.leftEnergy += dl * dl;
                analysis.rightEnergy += dr * dr;
                analysis.sumLR += dl * dr;
                analysis.sumSquares += dl * dl;
                if (channelsForEvidence > 1)
                    analysis.sumSquares += dr * dr;
                analysis.peakAbs = juce::jmax (analysis.peakAbs, juce::jmax (std::abs (dl), std::abs (dr)));
                ++analysis.audioFrameCount;
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

            const double hz = ((double) bin * sr) / (double) fftSize;
            const int bandIndex = bandIndexForHz (hz, analysis.bands);
            if (bandIndex < 0)
                continue;

            auto& band = analysis.bands[(size_t) bandIndex];
            band.energy += mag2;
            ++band.frameCount;
            analysis.totalBandEnergy += mag2;
        }
        ++analysis.fftFrameCount;
    }

    finishEvidence (analysis);
    return analysis;
}

void publishSummaries (const AudioFeatureBakeRequest& request,
                       const L3Analysis& analysis,
                       const L3AcousticAnalyzer::PublishCallback& publish)
{
    const auto requested = request.featureType;
    if (requested == AudioFeatureType::BandEnergySummary || requested == AudioFeatureType::L3AcousticSummary)
        publishBandSummary (request, analysis, publish);
    if (requested == AudioFeatureType::StereoRelationSummary || requested == AudioFeatureType::L3AcousticSummary)
        publishStereoSummary (request, analysis, publish);
    if (requested == AudioFeatureType::LoudnessSummary || requested == AudioFeatureType::L3AcousticSummary)
        publishLoudnessSummary (request, analysis, publish);
}

juce::ThreadPool& l3AnalysisPool()
{
    // Full-source FFT analysis is intentionally serialized. The import queue
    // may enqueue a large stems project faster than individual files finish;
    // spawning one detached reader per track made completion non-deterministic
    // on repeated 4 GB project smokes.
    static juce::ThreadPool pool (juce::ThreadPool::Options()
                                      .withThreadName ("Vit L3 Acoustic")
                                      .withNumberOfThreads (1));
    return pool;
}

} // namespace

void L3AcousticAnalyzer::startAnalyze (AudioFeatureBakeRequest request,
                                       PublishCallback publishCallback)
{
    writeDiagLog ("[dad_l3] queued track_id=" + request.trackId
                  + " clip_id=" + request.clipId
                  + " feature_type=" + audioFeatureTypeToString (request.featureType)
                  + " source_revision=" + request.sourceRevision
                  + " clip_revision=" + request.clipRevision
                  + " file=" + request.filePath);

    l3AnalysisPool().addJob ([request = std::move (request),
                              publish = std::move (publishCallback)]() mutable
    {
        writeDiagLog ("[dad_l3] start track_id=" + request.trackId
                      + " clip_id=" + request.clipId
                      + " feature_type=" + audioFeatureTypeToString (request.featureType)
                      + " source_revision=" + request.sourceRevision
                      + " clip_revision=" + request.clipRevision
                      + " file=" + request.filePath);
        const auto analysis = analyzeFile (request);
        writeDiagLog ("[dad_l3] finish track_id=" + request.trackId
                      + " clip_id=" + request.clipId
                      + " quality_status=" + analysis.evidence.status
                      + " analyzed_sample_count=" + juce::String ((int64) analysis.evidence.analyzedSampleCount)
                      + " coverage=" + juce::String (analysis.evidence.coverageRatio, 4));
        publishSummaries (request, analysis, publish);
    });
}

} // namespace vit
