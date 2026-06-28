#include "WaveformEnvelopeBaker.h"

#include "AudioFeatureTypes.h"
#include "../Core/VitPaths.h"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstring>
#include <fstream>
#include <limits>
#include <map>
#include <mutex>
#include <thread>
#include <windows.h>

namespace vit
{
namespace
{

constexpr double kTileSeconds = 5.0;
constexpr int kDefaultFramesPerTile = 1024;
constexpr int kFeatureStride = 6; // L_min, L_max, R_min, R_max, L_rms, R_rms
constexpr int kRetiredHandleGraceMs = 5000;

std::mutex gMutex;
std::mutex gDiagLogMutex;
std::map<std::string, std::vector<HANDLE>> gHandles;
std::map<std::string, uint64_t> gGen;

struct RetiredHandle
{
    std::string key;
    uint64_t gen = 0;
    HANDLE handle = nullptr;
    std::chrono::steady_clock::time_point releaseAt;
};

std::vector<RetiredHandle> gRetiredHandles;

juce::String makeBakeKey (const juce::String& trackId, const juce::String& clipId)
{
    auto key = clipId.trim();
    return key.isNotEmpty() ? key : trackId.trim();
}

juce::String sanitiseBakeKeyForShm (juce::String key)
{
    key = key.trim();
    if (key.isEmpty())
        key = "track";
    return key.replaceCharacters ("\\/:.", "____");
}

void writeDiagLog (const juce::String& line)
{
    juce::Logger::writeToLog (line);
    std::lock_guard<std::mutex> lock (gDiagLogMutex);
    const auto logsDir = paths::getLogsDirectory();
    paths::ensureDirectoryExists (logsDir, "logs");
    const auto diagLogFile = logsDir.getChildFile ("waveform_envelope_baker_diag.log");
    std::ofstream out (diagLogFile.getFullPathName().toStdString(), std::ios::app);
    if (out.is_open())
        out << line.toStdString() << std::endl;
}

size_t handleCountUnlocked (const std::string& key)
{
    const auto it = gHandles.find (key);
    return it == gHandles.end() ? 0u : it->second.size();
}

void cleanupRetiredHandlesUnlocked()
{
    const auto now = std::chrono::steady_clock::now();
    for (auto it = gRetiredHandles.begin(); it != gRetiredHandles.end();)
    {
        if (it->releaseAt > now)
        {
            ++it;
            continue;
        }
        if (it->handle)
            CloseHandle (it->handle);
        it = gRetiredHandles.erase (it);
    }
}

uint64_t beginGen (const juce::String& id)
{
    std::lock_guard<std::mutex> lock (gMutex);
    cleanupRetiredHandlesUnlocked();
    auto key = id.toStdString();
    auto& gen = gGen[key];
    ++gen;
    auto& handles = gHandles[key];
    const auto releaseAt = std::chrono::steady_clock::now() + std::chrono::milliseconds (kRetiredHandleGraceMs);
    const auto releasedCount = handles.size();
    for (auto h : handles)
    {
        if (h)
            gRetiredHandles.push_back ({ key, gen - 1, h, releaseAt });
    }
    handles.clear();
    writeDiagLog ("[waveform_envelope.lifecycle] begin_gen key=" + id
                  + " gen=" + juce::String ((int64) gen)
                  + " retired_handles=" + juce::String ((int) releasedCount));
    return gen;
}

bool isGen (const juce::String& id, uint64_t gen)
{
    std::lock_guard<std::mutex> lock (gMutex);
    const auto it = gGen.find (id.toStdString());
    return it != gGen.end() && it->second == gen;
}

bool storeHandle (const juce::String& id, uint64_t gen, HANDLE h)
{
    std::lock_guard<std::mutex> lock (gMutex);
    cleanupRetiredHandlesUnlocked();
    auto key = id.toStdString();
    const auto it = gGen.find (key);
    if (it == gGen.end() || it->second != gen)
        return false;
    gHandles[key].push_back (h);
    return true;
}

size_t handleCountForGen (const juce::String& id, uint64_t gen)
{
    std::lock_guard<std::mutex> lock (gMutex);
    auto key = id.toStdString();
    const auto it = gGen.find (key);
    if (it == gGen.end() || it->second != gen)
        return 0u;
    return handleCountUnlocked (key);
}

struct QualityStats
{
    int64_t sampleCount = 0;
    int64_t nonzeroCount = 0;
    int64_t nanInfCount = 0;
    double sumAbs = 0.0;
    double maxAbs = 0.0;

    void observe (float value)
    {
        ++sampleCount;
        if (! std::isfinite (value))
        {
            ++nanInfCount;
            return;
        }

        const auto absValue = std::abs ((double) value);
        if (absValue > 0.0)
            ++nonzeroCount;
        sumAbs += absValue;
        maxAbs = juce::jmax (maxAbs, absValue);
    }
};

struct QualityDecision
{
    juce::String status = "failed";
    juce::String reason = "not_evaluated";
};

QualityStats collectQualityStats (const float* data, size_t count)
{
    QualityStats stats;
    if (data == nullptr)
        return stats;

    for (size_t i = 0; i < count; ++i)
        stats.observe (data[i]);

    return stats;
}

QualityDecision decideWaveformQuality (const QualityStats& inputStats,
                                       const QualityStats& outputStats,
                                       const QualityStats& shmStats,
                                       int64_t audioSampleCount,
                                       int frameCount)
{
    if (audioSampleCount <= 0 || frameCount <= 0)
        return { "failed", "empty_coverage" };
    if (inputStats.nanInfCount > 0 || outputStats.nanInfCount > 0 || shmStats.nanInfCount > 0)
        return { "failed", "nan_or_inf_detected" };
    if (outputStats.nonzeroCount <= 0)
        return { inputStats.nonzeroCount > 0 ? "failed" : "suspect",
                 inputStats.nonzeroCount > 0 ? "output_all_zero" : "input_all_zero" };
    if (shmStats.nonzeroCount <= 0)
        return { "failed", "shared_memory_all_zero_after_write" };
    return { "ready", "ok" };
}

void setQualityStatsProperties (juce::DynamicObject& obj,
                                const juce::String& prefix,
                                const QualityStats& stats)
{
    obj.setProperty (prefix + "sample_count", (int64) stats.sampleCount);
    obj.setProperty (prefix + "nonzero_count", (int64) stats.nonzeroCount);
    obj.setProperty (prefix + "sum_abs", stats.sumAbs);
    obj.setProperty (prefix + "max_abs", stats.maxAbs);
    obj.setProperty (prefix + "nan_inf_count", (int64) stats.nanInfCount);
}

void stampIdentityProperties (juce::DynamicObject& obj,
                              const juce::String& trackId,
                              const juce::String& clipId,
                              const juce::String& sourceId,
                              const juce::String& sourceRevision,
                              const juce::String& clipRevision,
                              const juce::String& renderRevision,
                              const juce::String& filePath,
                              double totalDurationSec)
{
    obj.setProperty ("project_id", "current");
    obj.setProperty ("track_id", trackId);
    obj.setProperty ("source_track_id", trackId);
    obj.setProperty ("source_path", filePath);
    obj.setProperty ("duration_seconds", totalDurationSec);
    if (clipId.isNotEmpty())
        obj.setProperty ("clip_id", clipId);
    if (sourceId.isNotEmpty())
        obj.setProperty ("source_id", sourceId);
    if (sourceRevision.isNotEmpty())
    {
        obj.setProperty ("source_revision", sourceRevision);
        obj.setProperty ("source_fingerprint", sourceRevision);
    }
    if (clipRevision.isNotEmpty())
        obj.setProperty ("clip_revision", clipRevision);
    if (renderRevision.isNotEmpty())
        obj.setProperty ("render_revision", renderRevision);
    obj.setProperty ("analyzer_revision", audioFeatureAnalysisVersion());
}

void publishBakeStatus (const juce::String& status,
                        const juce::String& reason,
                        const juce::String& trackId,
                        const juce::String& clipId,
                        const juce::String& filePath,
                        const WaveformEnvelopeBaker::PublishCallback& publish)
{
    if (! publish)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    obj->setProperty ("command", "audio_feature_bake_status");
    obj->setProperty ("feature_family", "audio_feature");
    obj->setProperty ("feature_type", audioFeatureTypeToString (AudioFeatureType::WaveformEnvelope));
    obj->setProperty ("feature_version", audioFeatureProductVersion (AudioFeatureType::WaveformEnvelope));
    obj->setProperty ("analysis_version", audioFeatureAnalysisVersion());
    obj->setProperty ("status", status);
    obj->setProperty ("reason", reason);
    obj->setProperty ("track_id", trackId);
    obj->setProperty ("file_path", filePath);
    if (clipId.isNotEmpty())
        obj->setProperty ("clip_id", clipId);
    publish (juce::JSON::toString (juce::var (obj.release())));
}

} // namespace

void WaveformEnvelopeBaker::startBake (juce::String filePath,
                                       juce::String trackId,
                                       juce::String clipId,
                                       PublishCallback publish,
                                       double sourceOffsetSeconds,
                                       double bakeLengthSeconds,
                                       int framesPerTile,
                                       juce::String sourceId,
                                       juce::String sourceRevision,
                                       juce::String clipRevision,
                                       juce::String renderRevision)
{
    const auto bakeKey = makeBakeKey (trackId, clipId);
    const auto gen = beginGen (bakeKey);
    const int safeFramesPerTile = juce::jmax (16, framesPerTile > 0 ? framesPerTile : kDefaultFramesPerTile);

    writeDiagLog ("[waveform_envelope.lifecycle] start_bake key=" + bakeKey
                  + " gen=" + juce::String ((int64) gen)
                  + " track_id=" + trackId
                  + " clip_id=" + clipId
                  + " source_revision=" + sourceRevision
                  + " clip_revision=" + clipRevision
                  + " source_offset=" + juce::String (sourceOffsetSeconds, 4)
                  + " bake_length=" + juce::String (bakeLengthSeconds, 4)
                  + " frames_per_tile=" + juce::String (safeFramesPerTile)
                  + " file=" + filePath);

    std::thread ([filePath = std::move (filePath),
                  trackId = std::move (trackId),
                  clipId = std::move (clipId),
                  bakeKey = std::move (bakeKey),
                  publish = std::move (publish),
                  sourceId = std::move (sourceId),
                  sourceRevision = std::move (sourceRevision),
                  clipRevision = std::move (clipRevision),
                  renderRevision = std::move (renderRevision),
                  sourceOffsetSeconds,
                  bakeLengthSeconds,
                   safeFramesPerTile,
                   gen]() mutable
    {
        const auto bakeStartMs = juce::Time::getMillisecondCounterHiRes();
        auto logPerf = [&] (const juce::String& stage, const juce::String& details = {})
        {
            const auto nowMs = juce::Time::getMillisecondCounterHiRes();
            writeDiagLog (juce::String ("[VitImportPerf] op=waveform_envelope")
                          + " stage=" + stage
                          + " total_ms=" + juce::String (nowMs - bakeStartMs, 2)
                          + " key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + (details.isNotEmpty() ? " " + details : juce::String()));
        };
        juce::AudioFormatManager fm;
        fm.registerBasicFormats();
        const auto readerStartMs = juce::Time::getMillisecondCounterHiRes();
        std::unique_ptr<juce::AudioFormatReader> reader (
            fm.createReaderFor (juce::File (filePath)));
        logPerf ("reader_open", juce::String ("stage_ms=") + juce::String (juce::Time::getMillisecondCounterHiRes() - readerStartMs, 2));

        if (! reader || reader->lengthInSamples <= 0 || reader->sampleRate <= 0.0)
        {
            writeDiagLog ("[waveform_envelope.lifecycle] reader_failed key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + " file=" + filePath);
            publishBakeStatus ("error", "reader_failed", trackId, clipId, filePath, publish);
            logPerf ("finish", "status=error reason=reader_failed");
            return;
        }

        const double sr = reader->sampleRate;
        const int64_t sourceStartSample = juce::jlimit<int64_t> (
            0,
            reader->lengthInSamples,
            (int64_t) std::floor (juce::jmax (0.0, sourceOffsetSeconds) * sr));
        const int64_t availableSamples = juce::jmax<int64_t> (0, reader->lengthInSamples - sourceStartSample);
        int64_t bakeTotalSamples = availableSamples;
        if (bakeLengthSeconds > 0.0)
            bakeTotalSamples = juce::jmin<int64_t> (availableSamples, (int64_t) std::ceil (bakeLengthSeconds * sr));

        if (bakeTotalSamples <= 0)
        {
            publishBakeStatus ("error", "empty_range", trackId, clipId, filePath, publish);
            logPerf ("finish", "status=error reason=empty_range");
            return;
        }

        const int64_t tileSpanSamples = juce::jmax<int64_t> (1, (int64_t) std::llround (kTileSeconds * sr));
        const int totalTiles = (int) juce::jmax<int64_t> (
            1,
            (bakeTotalSamples + tileSpanSamples - 1) / tileSpanSamples);
        const double totalDurationSec = (double) bakeTotalSamples / sr;
        std::vector<float> envelope ((size_t) safeFramesPerTile * kFeatureStride, 0.0f);
        logPerf ("range_ready",
                 juce::String ("sample_rate=") + juce::String (sr, 2)
                 + " total_tiles=" + juce::String (totalTiles)
                 + " total_duration=" + juce::String (totalDurationSec, 4)
                 + " frames_per_tile=" + juce::String (safeFramesPerTile));
        int completedTiles = 0;

        for (int tileIndex = 0; tileIndex < totalTiles; ++tileIndex)
        {
            if (! isGen (bakeKey, gen))
            {
                logPerf ("finish",
                         juce::String ("status=canceled completed_tiles=") + juce::String (completedTiles)
                         + " total_tiles=" + juce::String (totalTiles));
                return;
            }

            const auto tileStartMs = juce::Time::getMillisecondCounterHiRes();
            const int64_t tileStartSample = (int64_t) tileIndex * tileSpanSamples;
            const int64_t tileRemainingSamples = juce::jmax<int64_t> (0, bakeTotalSamples - tileStartSample);
            const int64_t tileValidSamples64 = juce::jmin<int64_t> (tileSpanSamples, tileRemainingSamples);
            if (tileValidSamples64 <= 0)
                break;

            const int tileValidSamples = (int) juce::jmin<int64_t> (
                tileValidSamples64,
                (int64_t) (std::numeric_limits<int>::max)());
            const double tileDurationSec = (double) tileValidSamples / sr;
            const double tileContentStartSeconds = sourceOffsetSeconds + ((double) tileStartSample / sr);
            juce::AudioBuffer<float> buffer (2, tileValidSamples);
            buffer.clear();
            reader->read (&buffer,
                          0,
                          tileValidSamples,
                          sourceStartSample + tileStartSample,
                          true,
                          true);

            const float* left = buffer.getReadPointer (0);
            const float* right = buffer.getReadPointer (juce::jmin (1, buffer.getNumChannels() - 1));
            std::fill (envelope.begin(), envelope.end(), 0.0f);
            QualityStats inputStats;

            for (int frame = 0; frame < safeFramesPerTile; ++frame)
            {
                const int s0 = (int) std::floor ((double) frame * (double) tileValidSamples / (double) safeFramesPerTile);
                int s1 = (int) std::floor ((double) (frame + 1) * (double) tileValidSamples / (double) safeFramesPerTile);
                s1 = juce::jlimit (s0 + 1, tileValidSamples, s1);

                float minL = 1.0f;
                float maxL = -1.0f;
                float minR = 1.0f;
                float maxR = -1.0f;
                double sumL = 0.0;
                double sumR = 0.0;
                int count = 0;

                for (int s = s0; s < s1; ++s)
                {
                    const float lv = left[s];
                    const float rv = right[s];
                    inputStats.observe (lv);
                    inputStats.observe (rv);
                    minL = juce::jmin (minL, lv);
                    maxL = juce::jmax (maxL, lv);
                    minR = juce::jmin (minR, rv);
                    maxR = juce::jmax (maxR, rv);
                    sumL += (double) lv * (double) lv;
                    sumR += (double) rv * (double) rv;
                    ++count;
                }

                if (count <= 0)
                {
                    minL = maxL = minR = maxR = 0.0f;
                }

                const auto base = (size_t) frame * kFeatureStride;
                envelope[base + 0] = minL;
                envelope[base + 1] = maxL;
                envelope[base + 2] = minR;
                envelope[base + 3] = maxR;
                envelope[base + 4] = count > 0 ? std::sqrt ((float) (sumL / (double) count)) : 0.0f;
                envelope[base + 5] = count > 0 ? std::sqrt ((float) (sumR / (double) count)) : 0.0f;
            }

            const auto outputStats = collectQualityStats (envelope.data(), envelope.size());
            const auto sessionId = bakeKey + ":" + juce::String ((int64) gen);
            const auto shm = "Vit_AudioFeature_waveform_"
                + sanitiseBakeKeyForShm (bakeKey)
                + "_g" + juce::String ((int64) gen)
                + "_" + juce::String (tileIndex);
            const auto bytes = (SIZE_T) (envelope.size() * sizeof (float));
            HANDLE h = CreateFileMappingA (INVALID_HANDLE_VALUE,
                                           nullptr,
                                           PAGE_READWRITE,
                                           0,
                                           (DWORD) bytes,
                                           shm.toRawUTF8());
            if (! h)
            {
                writeDiagLog ("[waveform_envelope.lifecycle] create_mapping_failed key=" + bakeKey
                              + " gen=" + juce::String ((int64) gen)
                              + " tile_index=" + juce::String (tileIndex)
                              + " win_error=" + juce::String ((int) GetLastError()));
                continue;
            }

            auto* mapped = (float*) MapViewOfFile (h, FILE_MAP_ALL_ACCESS, 0, 0, bytes);
            if (! mapped)
            {
                writeDiagLog ("[waveform_envelope.lifecycle] map_view_failed key=" + bakeKey
                              + " gen=" + juce::String ((int64) gen)
                              + " tile_index=" + juce::String (tileIndex)
                              + " win_error=" + juce::String ((int) GetLastError()));
                CloseHandle (h);
                continue;
            }

            std::memcpy (mapped, envelope.data(), envelope.size() * sizeof (float));
            const auto shmStats = collectQualityStats (mapped, envelope.size());
            UnmapViewOfFile (mapped);

            if (! storeHandle (bakeKey, gen, h))
            {
                CloseHandle (h);
                return;
            }

            const auto handleCount = handleCountForGen (bakeKey, gen);
            const auto quality = decideWaveformQuality (inputStats,
                                                        outputStats,
                                                        shmStats,
                                                        tileValidSamples64,
                                                        safeFramesPerTile);
            const double coverageRatio = bakeTotalSamples > 0
                ? juce::jlimit (0.0, 1.0, (double) tileValidSamples64 / (double) bakeTotalSamples)
                : 0.0;
            if (publish)
            {
                auto obj = std::make_unique<juce::DynamicObject>();
                obj->setProperty ("command", "audio_feature_data_ready");
                obj->setProperty ("feature_family", "audio_feature");
                obj->setProperty ("feature_type", audioFeatureTypeToString (AudioFeatureType::WaveformEnvelope));
                obj->setProperty ("feature_version", audioFeatureProductVersion (AudioFeatureType::WaveformEnvelope));
                obj->setProperty ("analysis_version", audioFeatureAnalysisVersion());
                obj->setProperty ("channels_semantics", "l_min,l_max,r_min,r_max,l_rms,r_rms");
                obj->setProperty ("source_kind", clipId.isNotEmpty() ? "clip" : "file");
                stampIdentityProperties (*obj,
                                         trackId,
                                         clipId,
                                         sourceId,
                                         sourceRevision,
                                         clipRevision,
                                         renderRevision,
                                         filePath,
                                         totalDurationSec);
                obj->setProperty ("session_id", sessionId);
                obj->setProperty ("file_path", filePath);
                obj->setProperty ("tile_index", tileIndex);
                obj->setProperty ("tile_duration", tileDurationSec);
                obj->setProperty ("tile_content_start_seconds", tileContentStartSeconds);
                obj->setProperty ("range_source_offset_seconds", sourceOffsetSeconds);
                obj->setProperty ("range_length_seconds", bakeLengthSeconds);
                obj->setProperty ("coverage_seconds", tileDurationSec);
                obj->setProperty ("coverage_ratio", coverageRatio);
                obj->setProperty ("frame_count", safeFramesPerTile);
                obj->setProperty ("audio_sample_count", (int64) tileValidSamples64);
                obj->setProperty ("resolution_frame_count", safeFramesPerTile);
                obj->setProperty ("feature_stride", kFeatureStride);
                obj->setProperty ("frame_duration_seconds", tileDurationSec / (double) safeFramesPerTile);
                obj->setProperty ("total_duration", totalDurationSec);
                obj->setProperty ("shared_memory", shm);
                obj->setProperty ("float_count", (int64) envelope.size());
                obj->setProperty ("bake_key", bakeKey);
                obj->setProperty ("generation", (int64) gen);
                obj->setProperty ("handle_count", (int) handleCount);
                obj->setProperty ("shm_bytes", (int64) bytes);
                obj->setProperty ("quality_status", quality.status);
                obj->setProperty ("quality_reason", quality.reason);
                obj->setProperty ("ready", quality.status == "ready");
                setQualityStatsProperties (*obj, {}, outputStats);
                setQualityStatsProperties (*obj, "reader_", inputStats);
                setQualityStatsProperties (*obj, "shm_postwrite_", shmStats);
                publish (juce::JSON::toString (juce::var (obj.release())));
            }
            completedTiles = tileIndex + 1;
            if (tileIndex == 0 || completedTiles == totalTiles || (completedTiles % 25) == 0)
            {
                logPerf ("tile_progress",
                         juce::String ("tile=") + juce::String (completedTiles)
                         + "/" + juce::String (totalTiles)
                         + " tile_ms=" + juce::String (juce::Time::getMillisecondCounterHiRes() - tileStartMs, 2)
                         + " tile_duration=" + juce::String (tileDurationSec, 4)
                         + " handles=" + juce::String ((int) handleCount));
            }
        }
        logPerf ("finish",
                 juce::String ("status=ok completed_tiles=") + juce::String (completedTiles)
                 + " total_tiles=" + juce::String (totalTiles));
    }).detach();
}

void WaveformEnvelopeBaker::releaseTrackMappings (const juce::String& trackId)
{
    beginGen (trackId);
}

void WaveformEnvelopeBaker::invalidateClipBake (const juce::String& clipId)
{
    const auto trimmed = clipId.trim();
    if (trimmed.isNotEmpty())
        beginGen (trimmed);
}

} // namespace vit
