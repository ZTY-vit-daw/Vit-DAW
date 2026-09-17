#include "WaveformEnvelopeBaker.h"

#include "AudioFeatureTypes.h"
#include "OfflineAudioReadCoordinator.h"
#include "../Core/VitPaths.h"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <cstdlib>
#include <cstring>
#include <deque>
#include <fstream>
#include <functional>
#include <limits>
#include <map>
#include <mutex>
#include <string>
#include <thread>
#include <utility>
#include <vector>
#if defined(_WIN32)
#include <windows.h>
#endif

namespace vit
{
namespace
{

constexpr double kTileSeconds = 5.0;
constexpr int kDefaultFramesPerTile = 1024;
constexpr int kFeatureStride = 6; // L_min, L_max, R_min, R_max, L_rms, R_rms
// Godot drains audio_feature_data_ready on a small per-frame budget. Under
// visible-priority multitrack waveform preparation, old generations can arrive
// well after a newer request supersedes them, so retired SHM handles need to
// outlive short UI backlogs.
constexpr int kRetiredHandleGraceMs = 120000;

std::mutex gMutex;
std::mutex gDiagLogMutex;
std::map<std::string, std::vector<void*>> gHandles;
std::map<std::string, uint64_t> gGen;

struct BakeSummary
{
    juce::String bakeKey;
    uint64_t gen = 0;
    juce::String status = "missing";
    juce::String reason;
    juce::String trackId;
    juce::String clipId;
    juce::String filePath;
    juce::String sourceId;
    juce::String sourceRevision;
    juce::String clipRevision;
    juce::String renderRevision;
    int completedTiles = 0;
    int totalTiles = 0;
    int handleCount = 0;
    int frameCount = 0;
    int featureStride = kFeatureStride;
    double sampleRate = 0.0;
    double totalDurationSeconds = 0.0;
    int64_t metricFrameCount = 0;
    int64_t nonzeroCount = 0;
    int64_t nanInfCount = 0;
    double sumSquares = 0.0;
    double sumAbs = 0.0;
    double maxAbs = 0.0;
    double peakAbs = 0.0;
    juce::Time updatedAt;
};

std::map<std::string, BakeSummary> gBakeSummaries;

void writeDiagLog (const juce::String& line);

struct RetiredHandle
{
    std::string key;
    uint64_t gen = 0;
    void* handle = nullptr;
    std::chrono::steady_clock::time_point releaseAt;
};

std::vector<RetiredHandle> gRetiredHandles;

struct QueuedWaveformBake
{
    juce::String bakeKey;
    uint64_t gen = 0;
    int priorityRank = 100;
    uint64_t sequence = 0;
    std::function<void()> run;
};

std::mutex gQueueMutex;
std::condition_variable gQueueCondition;
std::deque<QueuedWaveformBake> gWaveformQueue;
std::vector<std::thread> gWaveformWorkers;
uint64_t gWaveformQueueSequence = 0;
bool gWaveformWorkersStarted = false;

constexpr int kDefaultWaveformQueueMax = 256;
constexpr int kDefaultWaveformWorkerMax = 4;

int envInt (const char* name, int fallback, int minValue, int maxValue)
{
    if (name == nullptr)
        return fallback;
    if (const auto* raw = std::getenv (name))
    {
        try
        {
            return juce::jlimit (minValue, maxValue, std::stoi (std::string (raw)));
        }
        catch (...) {}
    }
    return fallback;
}

int waveformPriorityRank (AudioFeaturePriority priority)
{
    switch (priority)
    {
        case AudioFeaturePriority::OnDemand:         return 0;
        case AudioFeaturePriority::ImportImmediate:  return 10;
        case AudioFeaturePriority::BackgroundWarm:   return 50;
    }

    return 100;
}

int waveformWorkerCount()
{
    const auto hardware = (int) std::thread::hardware_concurrency();
    const auto derived = juce::jlimit (1, kDefaultWaveformWorkerMax, hardware > 0 ? hardware / 2 : 2);
    return envInt ("VIT_WAVEFORM_BAKER_WORKERS", derived, 1, 16);
}

int waveformQueueMax()
{
    return envInt ("VIT_WAVEFORM_BAKER_QUEUE_MAX", kDefaultWaveformQueueMax, 16, 4096);
}

size_t bestQueuedBakeIndexUnlocked()
{
    size_t best = 0;
    for (size_t i = 1; i < gWaveformQueue.size(); ++i)
    {
        const auto& candidate = gWaveformQueue[i];
        const auto& current = gWaveformQueue[best];
        if (candidate.priorityRank < current.priorityRank
            || (candidate.priorityRank == current.priorityRank && candidate.sequence < current.sequence))
            best = i;
    }
    return best;
}

bool dropWorstQueuedBakeForUnlocked (int incomingPriorityRank)
{
    if (gWaveformQueue.empty())
        return true;

    size_t worst = 0;
    for (size_t i = 1; i < gWaveformQueue.size(); ++i)
    {
        const auto& candidate = gWaveformQueue[i];
        const auto& current = gWaveformQueue[worst];
        if (candidate.priorityRank > current.priorityRank
            || (candidate.priorityRank == current.priorityRank && candidate.sequence > current.sequence))
            worst = i;
    }

    if (gWaveformQueue[worst].priorityRank <= incomingPriorityRank)
        return false;

    writeDiagLog ("[waveform_envelope.queue] drop_queued key=" + gWaveformQueue[worst].bakeKey
                  + " gen=" + juce::String ((int64) gWaveformQueue[worst].gen)
                  + " priority_rank=" + juce::String (gWaveformQueue[worst].priorityRank)
                  + " reason=queue_full");
    gWaveformQueue.erase (gWaveformQueue.begin() + (std::ptrdiff_t) worst);
    return true;
}

void waveformWorkerLoop()
{
    for (;;)
    {
        QueuedWaveformBake job;
        {
            std::unique_lock<std::mutex> lock (gQueueMutex);
            gQueueCondition.wait (lock, [] { return ! gWaveformQueue.empty(); });
            const auto best = bestQueuedBakeIndexUnlocked();
            job = std::move (gWaveformQueue[best]);
            gWaveformQueue.erase (gWaveformQueue.begin() + (std::ptrdiff_t) best);
        }

        if (job.run)
            job.run();
    }
}

void ensureWaveformWorkersStartedUnlocked()
{
    if (gWaveformWorkersStarted)
        return;

    gWaveformWorkersStarted = true;
    const auto count = waveformWorkerCount();
    gWaveformWorkers.reserve ((size_t) count);
    for (int i = 0; i < count; ++i)
    {
        gWaveformWorkers.emplace_back ([] { waveformWorkerLoop(); });
        gWaveformWorkers.back().detach();
    }

    writeDiagLog ("[waveform_envelope.queue] workers_started count=" + juce::String (count)
                  + " queue_max=" + juce::String (waveformQueueMax()));
}

bool enqueueWaveformBake (QueuedWaveformBake job)
{
    std::lock_guard<std::mutex> lock (gQueueMutex);
    ensureWaveformWorkersStartedUnlocked();

    for (auto it = gWaveformQueue.begin(); it != gWaveformQueue.end();)
    {
        if (it->bakeKey == job.bakeKey)
        {
            writeDiagLog ("[waveform_envelope.queue] supersede_queued key=" + it->bakeKey
                          + " old_gen=" + juce::String ((int64) it->gen)
                          + " new_gen=" + juce::String ((int64) job.gen));
            it = gWaveformQueue.erase (it);
        }
        else
        {
            ++it;
        }
    }

    const auto queueMax = (size_t) waveformQueueMax();
    if (gWaveformQueue.size() >= queueMax && ! dropWorstQueuedBakeForUnlocked (job.priorityRank))
    {
        writeDiagLog ("[waveform_envelope.queue] reject key=" + job.bakeKey
                      + " gen=" + juce::String ((int64) job.gen)
                      + " priority_rank=" + juce::String (job.priorityRank)
                      + " queue_size=" + juce::String ((int) gWaveformQueue.size())
                      + " reason=queue_full");
        return false;
    }

    job.sequence = ++gWaveformQueueSequence;
    writeDiagLog ("[waveform_envelope.queue] enqueue key=" + job.bakeKey
                  + " gen=" + juce::String ((int64) job.gen)
                  + " priority_rank=" + juce::String (job.priorityRank)
                  + " queue_size=" + juce::String ((int) gWaveformQueue.size() + 1));
    gWaveformQueue.push_back (std::move (job));
    gQueueCondition.notify_one();
    return true;
}

juce::String makeBakeKey (const juce::String& trackId, const juce::String& clipId)
{
    auto key = clipId.trim();
    return key.isNotEmpty() ? key : trackId.trim();
}

double waveformDbFromLinear (double value)
{
    constexpr double kSilenceDb = -120.0;
    if (value <= 0.0 || ! std::isfinite (value))
        return kSilenceDb;
    return juce::jmax (kSilenceDb, 20.0 * std::log10 (value));
}

[[maybe_unused]] juce::String sanitiseBakeKeyForShm (juce::String key)
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
#if defined(_WIN32)
        if (it->handle)
            CloseHandle (it->handle);
#endif
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
    gBakeSummaries.erase (key);
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

[[maybe_unused]] bool storeHandle (const juce::String& id, uint64_t gen, void* h)
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

void recordBakeSummary (BakeSummary summary)
{
    std::lock_guard<std::mutex> lock (gMutex);
    auto key = summary.bakeKey.toStdString();
    const auto it = gGen.find (key);
    if (it == gGen.end() || it->second != summary.gen)
        return;

    summary.updatedAt = juce::Time::getCurrentTime();
    gBakeSummaries[key] = std::move (summary);
}

juce::var bakeSummaryToVar (const BakeSummary& summary)
{
    auto obj = std::make_unique<juce::DynamicObject>();
    obj->setProperty ("status", summary.status);
    obj->setProperty ("reason", summary.reason);
    obj->setProperty ("feature_type", audioFeatureTypeToString (AudioFeatureType::WaveformEnvelope));
    obj->setProperty ("feature_version", audioFeatureProductVersion (AudioFeatureType::WaveformEnvelope));
    obj->setProperty ("analysis_version", audioFeatureAnalysisVersion());
    obj->setProperty ("source_kind", "waveform_baker_status");
    obj->setProperty ("source", "waveform_envelope_baker");
    obj->setProperty ("bake_key", summary.bakeKey);
    obj->setProperty ("generation", (int64) summary.gen);
    obj->setProperty ("track_id", summary.trackId);
    obj->setProperty ("source_track_id", summary.trackId);
    if (summary.clipId.isNotEmpty())
        obj->setProperty ("clip_id", summary.clipId);
    obj->setProperty ("source_path", summary.filePath);
    obj->setProperty ("file_path", summary.filePath);
    if (summary.sourceId.isNotEmpty())
        obj->setProperty ("source_id", summary.sourceId);
    if (summary.sourceRevision.isNotEmpty())
    {
        obj->setProperty ("source_revision", summary.sourceRevision);
        obj->setProperty ("source_fingerprint", summary.sourceRevision);
    }
    if (summary.clipRevision.isNotEmpty())
        obj->setProperty ("clip_revision", summary.clipRevision);
    if (summary.renderRevision.isNotEmpty())
        obj->setProperty ("render_revision", summary.renderRevision);
    obj->setProperty ("tile_count_seen", summary.completedTiles);
    obj->setProperty ("tile_count_expected", summary.totalTiles);
    obj->setProperty ("completed_tiles", summary.completedTiles);
    obj->setProperty ("total_tiles", summary.totalTiles);
    obj->setProperty ("handle_count", summary.handleCount);
    obj->setProperty ("frame_count", summary.frameCount);
    obj->setProperty ("feature_stride", summary.featureStride);
    obj->setProperty ("sample_rate", summary.sampleRate);
    obj->setProperty ("duration_seconds", summary.totalDurationSeconds);
    obj->setProperty ("total_duration", summary.totalDurationSeconds);
    if (summary.metricFrameCount > 0)
    {
        const auto rms = std::sqrt (summary.sumSquares / (double) summary.metricFrameCount);
        const auto peakDb = waveformDbFromLinear (summary.peakAbs);
        const auto rmsDb = waveformDbFromLinear (rms);
        obj->setProperty ("rms", rms);
        obj->setProperty ("peak_abs", summary.peakAbs);
        obj->setProperty ("rms_dbfs", rms > 0.0 ? juce::var (rmsDb) : juce::var());
        obj->setProperty ("peak_dbfs", summary.peakAbs > 0.0 ? juce::var (peakDb) : juce::var());
        obj->setProperty ("headroom_db", summary.peakAbs > 0.0 ? juce::var (-peakDb) : juce::var());
        obj->setProperty ("crest_db", (summary.peakAbs > 0.0 && rms > 0.0) ? juce::var (peakDb - rmsDb) : juce::var());
        obj->setProperty ("metric_frame_count", (int64) summary.metricFrameCount);
        obj->setProperty ("sample_count", (int64) summary.metricFrameCount);
        obj->setProperty ("nonzero_count", (int64) summary.nonzeroCount);
        obj->setProperty ("sum_abs", summary.sumAbs);
        obj->setProperty ("max_abs", summary.maxAbs);
        obj->setProperty ("nan_inf_count", (int64) summary.nanInfCount);
    }
    obj->setProperty ("ready", summary.status == "ready");
    if (summary.updatedAt.toMilliseconds() > 0)
        obj->setProperty ("updated_at", summary.updatedAt.toISO8601 (true));
    return juce::var (obj.release());
}

BakeSummary baseBakeSummary (const juce::String& bakeKey,
                             uint64_t gen,
                             const juce::String& status,
                             const juce::String& reason,
                             const juce::String& trackId,
                             const juce::String& clipId,
                             const juce::String& filePath,
                             const juce::String& sourceId,
                             const juce::String& sourceRevision,
                             const juce::String& clipRevision,
                             const juce::String& renderRevision,
                             int frameCount)
{
    BakeSummary summary;
    summary.bakeKey = bakeKey;
    summary.gen = gen;
    summary.status = status;
    summary.reason = reason;
    summary.trackId = trackId;
    summary.clipId = clipId;
    summary.filePath = filePath;
    summary.sourceId = sourceId;
    summary.sourceRevision = sourceRevision;
    summary.clipRevision = clipRevision;
    summary.renderRevision = renderRevision;
    summary.frameCount = frameCount;
    return summary;
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

[[maybe_unused]] QualityDecision decideWaveformQuality (const QualityStats& inputStats,
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

[[maybe_unused]] void setQualityStatsProperties (juce::DynamicObject& obj,
                                const juce::String& prefix,
                                const QualityStats& stats)
{
    obj.setProperty (prefix + "sample_count", (int64) stats.sampleCount);
    obj.setProperty (prefix + "nonzero_count", (int64) stats.nonzeroCount);
    obj.setProperty (prefix + "sum_abs", stats.sumAbs);
    obj.setProperty (prefix + "max_abs", stats.maxAbs);
    obj.setProperty (prefix + "nan_inf_count", (int64) stats.nanInfCount);
}

[[maybe_unused]] void stampIdentityProperties (juce::DynamicObject& obj,
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
                                       juce::String renderRevision,
                                       AudioFeaturePriority priority)
{
    const auto bakeKey = makeBakeKey (trackId, clipId);
    const auto gen = beginGen (bakeKey);
    const int safeFramesPerTile = juce::jmax (16, framesPerTile > 0 ? framesPerTile : kDefaultFramesPerTile);
    const auto priorityRank = waveformPriorityRank (priority);

    writeDiagLog ("[waveform_envelope.lifecycle] start_bake key=" + bakeKey
                  + " gen=" + juce::String ((int64) gen)
                  + " priority_rank=" + juce::String (priorityRank)
                  + " track_id=" + trackId
                  + " clip_id=" + clipId
                  + " source_revision=" + sourceRevision
                  + " clip_revision=" + clipRevision
                  + " source_offset=" + juce::String (sourceOffsetSeconds, 4)
                  + " bake_length=" + juce::String (bakeLengthSeconds, 4)
                  + " frames_per_tile=" + juce::String (safeFramesPerTile)
                  + " file=" + filePath);

    recordBakeSummary (baseBakeSummary (bakeKey,
                                        gen,
                                        "building",
                                        "waveform_bake_queued",
                                        trackId,
                                        clipId,
                                        filePath,
                                        sourceId,
                                        sourceRevision,
                                        clipRevision,
                                        renderRevision,
                                        safeFramesPerTile));

    const auto dropFilePath = filePath;
    const auto dropTrackId = trackId;
    const auto dropClipId = clipId;
    const auto dropPublish = publish;

    QueuedWaveformBake queued;
    queued.bakeKey = bakeKey;
    queued.gen = gen;
    queued.priorityRank = priorityRank;
    queued.run = [filePath = std::move (filePath),
                  trackId = std::move (trackId),
                  clipId = std::move (clipId),
                  bakeKey,
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
        const auto leaseWaitStartMs = juce::Time::getMillisecondCounterHiRes();
        auto sourceReadLease = OfflineAudioReadCoordinator::acquire (filePath);
        logPerf ("source_read_lease",
                 juce::String ("wait_ms=")
                     + juce::String (juce::Time::getMillisecondCounterHiRes() - leaseWaitStartMs, 2));
        juce::AudioFormatManager fm;
        fm.registerBasicFormats();
        const auto readerStartMs = juce::Time::getMillisecondCounterHiRes();
        std::unique_ptr<juce::AudioFormatReader> reader (
            fm.createReaderFor (juce::File (filePath)));
        logPerf ("reader_open", juce::String ("stage_ms=") + juce::String (juce::Time::getMillisecondCounterHiRes() - readerStartMs, 2));

        if (! reader || reader->lengthInSamples <= 0 || reader->sampleRate <= 0.0)
        {
            recordBakeSummary (baseBakeSummary (bakeKey,
                                                gen,
                                                "failed",
                                                "reader_failed",
                                                trackId,
                                                clipId,
                                                filePath,
                                                sourceId,
                                                sourceRevision,
                                                clipRevision,
                                                renderRevision,
                                                safeFramesPerTile));
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
            recordBakeSummary (baseBakeSummary (bakeKey,
                                                gen,
                                                "failed",
                                                "empty_range",
                                                trackId,
                                                clipId,
                                                filePath,
                                                sourceId,
                                                sourceRevision,
                                                clipRevision,
                                                renderRevision,
                                                safeFramesPerTile));
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
        {
            auto summary = baseBakeSummary (bakeKey,
                                            gen,
                                            "partial",
                                            "waveform_tiles_pending",
                                            trackId,
                                            clipId,
                                            filePath,
                                            sourceId,
                                            sourceRevision,
                                            clipRevision,
                                            renderRevision,
                                            safeFramesPerTile);
            summary.totalTiles = totalTiles;
            summary.sampleRate = sr;
            summary.totalDurationSeconds = totalDurationSec;
            recordBakeSummary (std::move (summary));
        }
        int completedTiles = 0;
        int64_t aggregateMetricFrames = 0;
        int64_t aggregateNonzeroCount = 0;
        int64_t aggregateNanInfCount = 0;
        double aggregateSumSquares = 0.0;
        double aggregateSumAbs = 0.0;
        double aggregateMaxAbs = 0.0;
        double aggregatePeakAbs = 0.0;

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
            [[maybe_unused]] const double tileContentStartSeconds = sourceOffsetSeconds + ((double) tileStartSample / sr);
            juce::AudioBuffer<float> buffer (2, tileValidSamples);
            buffer.clear();
            const bool logTileRead = tileIndex == 0 || tileIndex + 1 == totalTiles || ((tileIndex + 1) % 25) == 0;
            if (logTileRead)
                logPerf ("tile_read_begin",
                         juce::String ("tile=") + juce::String (tileIndex + 1)
                         + "/" + juce::String (totalTiles)
                         + " source_sample=" + juce::String ((int64) (sourceStartSample + tileStartSample))
                         + " valid_samples=" + juce::String (tileValidSamples));
            reader->read (&buffer,
                          0,
                          tileValidSamples,
                          sourceStartSample + tileStartSample,
                          true,
                          true);
            if (logTileRead)
                logPerf ("tile_read_end",
                         juce::String ("tile=") + juce::String (tileIndex + 1)
                         + "/" + juce::String (totalTiles));

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

                const double framePeak = juce::jmax (juce::jmax (std::abs ((double) minL), std::abs ((double) maxL)),
                                                     juce::jmax (std::abs ((double) minR), std::abs ((double) maxR)));
                const double frameRmsL = (double) envelope[base + 4];
                const double frameRmsR = (double) envelope[base + 5];
                const double frameRms = std::sqrt ((frameRmsL * frameRmsL + frameRmsR * frameRmsR) / 2.0);
                aggregatePeakAbs = juce::jmax (aggregatePeakAbs, framePeak);
                aggregateSumSquares += frameRms * frameRms;
                ++aggregateMetricFrames;
            }

            const auto outputStats = collectQualityStats (envelope.data(), envelope.size());
            aggregateNonzeroCount += outputStats.nonzeroCount;
            aggregateNanInfCount += outputStats.nanInfCount;
            aggregateSumAbs += outputStats.sumAbs;
            aggregateMaxAbs = juce::jmax (aggregateMaxAbs, outputStats.maxAbs);
#if defined(_WIN32)
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
#else
            // PORT-A3: shm publishing uses the Windows mapping API; the POSIX
            // publisher is A1 scope. Skip the shared-memory write and the
            // audio_feature_data_ready event (it would advertise a segment
            // that does not exist on this platform).
            writeDiagLog ("[waveform_envelope.lifecycle] publish_skipped key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + " tile_index=" + juce::String (tileIndex)
                          + " reason=shm_unsupported_platform");
            const auto handleCount = handleCountForGen (bakeKey, gen);
#endif
            completedTiles = tileIndex + 1;
            {
                auto summary = baseBakeSummary (bakeKey,
                                                gen,
                                                completedTiles >= totalTiles ? "ready" : "partial",
                                                completedTiles >= totalTiles ? "ok" : "waveform_tiles_pending",
                                                trackId,
                                                clipId,
                                                filePath,
                                                sourceId,
                                                sourceRevision,
                                                clipRevision,
                                                renderRevision,
                                                safeFramesPerTile);
                summary.completedTiles = completedTiles;
                summary.totalTiles = totalTiles;
                summary.handleCount = (int) handleCount;
                summary.sampleRate = sr;
                summary.totalDurationSeconds = totalDurationSec;
                summary.metricFrameCount = aggregateMetricFrames;
                summary.nonzeroCount = aggregateNonzeroCount;
                summary.nanInfCount = aggregateNanInfCount;
                summary.sumSquares = aggregateSumSquares;
                summary.sumAbs = aggregateSumAbs;
                summary.maxAbs = aggregateMaxAbs;
                summary.peakAbs = aggregatePeakAbs;
                recordBakeSummary (std::move (summary));
            }
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
    };

    if (! enqueueWaveformBake (std::move (queued)))
        publishBakeStatus ("dropped", "waveform_queue_full", dropTrackId, dropClipId, dropFilePath, dropPublish);
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

juce::var WaveformEnvelopeBaker::getLatestBakeStatus (const juce::String& trackId,
                                                      const juce::String& clipId)
{
    const auto bakeKey = makeBakeKey (trackId, clipId);
    if (bakeKey.trim().isEmpty())
        return {};

    BakeSummary summary;
    {
        std::lock_guard<std::mutex> lock (gMutex);
        const auto key = bakeKey.toStdString();
        const auto genIt = gGen.find (key);
        const auto summaryIt = gBakeSummaries.find (key);
        if (genIt == gGen.end() || summaryIt == gBakeSummaries.end() || summaryIt->second.gen != genIt->second)
        {
            summary.bakeKey = bakeKey;
            summary.gen = genIt == gGen.end() ? 0 : genIt->second;
            summary.status = "missing";
            summary.reason = "waveform_bake_status_not_recorded";
            summary.trackId = trackId;
            summary.clipId = clipId;
            return bakeSummaryToVar (summary);
        }
        summary = summaryIt->second;
    }

    return bakeSummaryToVar (summary);
}

} // namespace vit
