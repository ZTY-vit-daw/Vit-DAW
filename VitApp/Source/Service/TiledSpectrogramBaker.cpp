#include "TiledSpectrogramBaker.h"
#include "AudioFeatureTypes.h"
#include "SharedMemorySegment.h"
#include "../Core/VitPaths.h"
#include <algorithm>
#include <chrono>
#include <cstdlib>
#include <memory>
#include <atomic>
#include <cmath>
#include <cstring>
#include <fstream>
#include <map>
#include <mutex>
#include <thread>

namespace vit {
namespace {

// ============================================================
// Constants
// 1 tile  = 500 frames = 5.0 seconds  (1 frame = 0.01 s)
// Texture layout: width=500 (time/X), height=336 (freq/Y)
// Index formula : (bin * kFrames + frame) * kCh
// ============================================================
constexpr int   kOrder   = 13;
constexpr int   kSize    = 1 << kOrder;   // 8192 samples per FFT window
constexpr int   kFftBins = kSize / 2;     // 4096 linear FFT bins
constexpr int   kUiBins  = 336;           // output frequency bins (log-mapped)
constexpr int   kFrames  = 500;           // frames per tile
constexpr int   kCh      = 4;            // RGBA channels
constexpr double kFrameSec = 0.01;       // seconds per frame (fixed physical time)
constexpr float  kMinFreq  = 20.0f;      // Hz
constexpr float  kMaxFreq  = 20000.0f;   // Hz
constexpr float  kEnvSilence = 1e-3f;    // align step9: gate when envelope below this
constexpr float  kGateNoisePercentile = 0.35f;
constexpr float  kGateAbsNoiseScale = 2.0f;
constexpr float  kGateDynDbMin = 55.0f;
constexpr float  kGateDynDbMax = 72.0f;
constexpr float  kGateDynDbSnrFactor = 0.8f;
constexpr float  kGateFloor = 1.0e-8f;
constexpr int    kDebugLogFrameStride = 25;
// Temporal smoothing on mapped spectrum bins (10 ms frame).
// Keep very light smoothing to avoid visible trail/lag.
constexpr float  kTemporalSmoothTauAttackSec  = 0.015f;
constexpr float  kTemporalSmoothTauReleaseSec = 0.040f;
constexpr int    kRetiredHandleGraceMs = 5000;

enum class BakeDebugMode
{
    Full = 0,      // gate + per-column normalize + sharpen (current production)
    Raw = 1,       // no gate, no normalize/sharpen
    GateOnly = 2,  // gate, but no normalize/sharpen
    NormOnly = 3,  // normalize/sharpen, but no gate
    NoNorm = 4,    // gate on, norm off
    NoGateNoNorm = 5 // gate off, norm off (pure mapped path)
};

enum class PoolMode
{
    Centroid = 0, // peak-gated centroid (default)
    Top3 = 1,     // top-3 average
    Max = 2       // pure max-pool
};

std::mutex gMutex;
std::mutex gDiagLogMutex;
std::map<std::string, std::vector<std::unique_ptr<ISharedMemorySegment>>> gHandles;
std::map<std::string, uint64_t>            gGen;

struct RetiredSegment
{
    std::string key;
    uint64_t gen = 0;
    std::unique_ptr<ISharedMemorySegment> segment;
    std::chrono::steady_clock::time_point releaseAt;
};

std::vector<RetiredSegment> gRetiredSegments;

size_t handleCountUnlocked (const std::string& key)
{
    const auto it = gHandles.find (key);
    return it == gHandles.end() ? 0u : it->second.size();
}

void cleanupRetiredSegmentsUnlocked()
{
    const auto now = std::chrono::steady_clock::now();
    for (auto it = gRetiredSegments.begin(); it != gRetiredSegments.end();)
    {
        if (it->releaseAt > now)
        {
            ++it;
            continue;
        }
        it = gRetiredSegments.erase (it);
    }
}

juce::String makeBakeKey (const juce::String& trackId, const juce::String& clipId)
{
    auto key = clipId.trim();
    return key.isNotEmpty() ? key : trackId.trim();
}

[[maybe_unused]] juce::String sanitiseBakeKeyForShm (juce::String key)
{
    key = key.trim();
    if (key.isEmpty())
        key = "track";
    return key.replaceCharacters ("\\/:.", "____");
}

void writeDiagLog(const juce::String& line)
{
    juce::Logger::writeToLog(line);
    std::lock_guard<std::mutex> lock(gDiagLogMutex);
    const auto logsDir = paths::getLogsDirectory();
    paths::ensureDirectoryExists(logsDir, "logs");
    const auto diagLogFile = logsDir.getChildFile("baker_diag.log");
    std::ofstream out(diagLogFile.getFullPathName().toStdString(), std::ios::app);
    if (out.is_open())
        out << line.toStdString() << std::endl;
}

// ----------------------------------------------------------
struct StereoRelationAcc
{
    double crossRe = 0.0;
    double crossIm = 0.0;
    double powerL = 0.0;
    double powerR = 0.0;
    int count = 0;
};

uint64_t beginGen(const juce::String& id)
{
    std::lock_guard<std::mutex> lock(gMutex);
    cleanupRetiredSegmentsUnlocked();
    auto key = id.toStdString();
    auto& gen = gGen[key]; ++gen;
    auto& handles = gHandles[key];
    const auto releasedCount = handles.size();
    const auto releaseAt = std::chrono::steady_clock::now() + std::chrono::milliseconds (kRetiredHandleGraceMs);
    for (auto& h : handles)
    {
        if (h)
            gRetiredSegments.push_back ({ key, gen - 1, std::move (h), releaseAt });
    }
    handles.clear();
    writeDiagLog ("[baker.lifecycle] begin_gen key=" + id
                  + " gen=" + juce::String ((int64) gen)
                  + " retired_handles=" + juce::String ((int) releasedCount)
                  + " retired_total=" + juce::String ((int) gRetiredSegments.size())
                  + " grace_ms=" + juce::String (kRetiredHandleGraceMs)
                  + " active_keys=" + juce::String ((int) gHandles.size()));
    return gen;
}

bool isGen(const juce::String& id, uint64_t gen)
{
    std::lock_guard<std::mutex> lock(gMutex);
    auto it = gGen.find(id.toStdString());
    return it != gGen.end() && it->second == gen;
}

[[maybe_unused]] bool storeSegment(const juce::String& id, uint64_t gen, std::unique_ptr<ISharedMemorySegment> segment)
{
    if (!segment)
        return false;
    std::lock_guard<std::mutex> lock(gMutex);
    cleanupRetiredSegmentsUnlocked();
    auto key = id.toStdString();
    auto it  = gGen.find(key);
    if (it == gGen.end() || it->second != gen)
    {
        writeDiagLog ("[baker.lifecycle] store_segment_reject key=" + id
                      + " gen=" + juce::String ((int64) gen)
                      + " current_gen=" + juce::String (it == gGen.end() ? -1 : (int64) it->second));
        return false;
    }
    gHandles[key].push_back(std::move(segment));
    writeDiagLog ("[baker.lifecycle] store_segment key=" + id
                  + " gen=" + juce::String ((int64) gen)
                  + " handle_count=" + juce::String ((int) gHandles[key].size()));
    return true;
}

[[maybe_unused]] size_t handleCountForGen (const juce::String& id, uint64_t gen)
{
    std::lock_guard<std::mutex> lock(gMutex);
    auto key = id.toStdString();
    auto it  = gGen.find(key);
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

[[maybe_unused]] QualityDecision decideSpectralQuality (const QualityStats& readerStats,
                                       const QualityStats& fftInputStats,
                                       const QualityStats& fftOutputStats,
                                       const QualityStats& tileStats,
                                       const QualityStats& shmStats,
                                       int64_t audioSampleCount,
                                       int frameCount,
                                       int activeFrameCount)
{
    if (audioSampleCount <= 0 || frameCount <= 0)
        return { "failed", "empty_coverage" };
    if (readerStats.nanInfCount > 0 || fftInputStats.nanInfCount > 0 || fftOutputStats.nanInfCount > 0
        || tileStats.nanInfCount > 0 || shmStats.nanInfCount > 0)
        return { "failed", "nan_or_inf_detected" };
    if (readerStats.nonzeroCount <= 0)
        return { "suspect", "reader_all_zero" };
    if (activeFrameCount <= 0)
        return { "failed", "no_active_frames_above_gate" };
    if (fftInputStats.nonzeroCount <= 0)
        return { "failed", "fft_input_all_zero" };
    if (fftOutputStats.nonzeroCount <= 0)
        return { "failed", "fft_output_all_zero" };
    if (tileStats.nonzeroCount <= 0)
        return { "failed", "tile_all_zero_before_write" };
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

float globalEnv(juce::AudioFormatReader& r)
{
    juce::AudioBuffer<float> b(2, 32768);
    float e = 0;
    for (int64_t p = 0; p < r.lengthInSamples; p += 32768)
    {
        auto n = (int)juce::jmin<int64_t>(32768, r.lengthInSamples - p);
        b.clear();
        r.read(&b, 0, n, p, true, true);
        for (int ch = 0; ch < juce::jmin(2, b.getNumChannels()); ++ch)
        {
            auto* d = b.getReadPointer(ch);
            for (int i = 0; i < n; ++i) e = juce::jmax(e, std::abs(d[i]));
        }
    }
    return e;
}

// Apply Hann window and copy src into dst (zero-padded to kSize)
void prepWindow(const float* src, int srcLen,
                juce::dsp::WindowingFunction<float>& win,
                std::vector<float>& dst)
{
    std::fill(dst.begin(), dst.end(), 0.0f);
    int copyLen = juce::jmin(srcLen, kSize);
    std::memcpy(dst.data(), src, (size_t)copyLen * sizeof(float));
    win.multiplyWithWindowingTable(dst.data(), kSize);
}

void ri(const std::vector<float>& d, int bin, float& re, float& im)
{
    if (bin == 0) { re = d[0]; im = 0; return; }
    re = d[2 * bin];
    im = d[2 * bin + 1];
}

float percentileNonZero(const std::vector<float>& v, float q)
{
    std::vector<float> nz;
    nz.reserve(v.size());
    for (auto x : v)
        if (x > 0.0f)
            nz.push_back(x);
    if (nz.empty())
        return 0.0f;

    const float qq = juce::jlimit(0.0f, 1.0f, q);
    const int k = juce::jlimit(0, (int)nz.size() - 1, (int)std::lround(qq * (float)(nz.size() - 1)));
    std::nth_element(nz.begin(), nz.begin() + k, nz.end());
    return nz[(size_t)k];
}

float adaptiveThreshold(float peak, float noise)
{
    if (peak <= 0.0f)
        return kGateFloor;
    const float eps = 1.0e-12f;
    const float snrDb = 20.0f * std::log10((peak + eps) / (noise + eps));
    const float dynDb = juce::jlimit(kGateDynDbMin, kGateDynDbMax, kGateDynDbSnrFactor * snrDb);
    const float thrRel = peak * std::pow(10.0f, -dynDb / 20.0f);
    const float thrAbs = noise * kGateAbsNoiseScale;
    return juce::jmax(kGateFloor, juce::jmax(thrRel, thrAbs));
}

float uiBinToHz(int uiBin)
{
    const float t = (float)uiBin / (float)(kUiBins - 1);
    return kMinFreq * std::pow(kMaxFreq / kMinFreq, t);
}

BakeDebugMode getBakeDebugMode()
{
    if (const char* v = std::getenv("VIT_BAKER_MODE"))
    {
        const juce::String s(v);
        if (s == "4" || s.equalsIgnoreCase("no_norm"))      return BakeDebugMode::NoNorm;
        if (s == "5" || s.equalsIgnoreCase("no_gate_no_norm")) return BakeDebugMode::NoGateNoNorm;
        if (s.equalsIgnoreCase("raw"))      return BakeDebugMode::Raw;
        if (s.equalsIgnoreCase("gate_only"))return BakeDebugMode::GateOnly;
        if (s.equalsIgnoreCase("norm_only"))return BakeDebugMode::NormOnly;
    }
    return BakeDebugMode::Full;
}

PoolMode getPoolMode()
{
    if (const char* v = std::getenv("VIT_BAKER_POOL"))
    {
        const juce::String s(v);
        if (s.equalsIgnoreCase("max"))  return PoolMode::Max;
        if (s.equalsIgnoreCase("top3")) return PoolMode::Top3;
        if (s.equalsIgnoreCase("centroid")) return PoolMode::Centroid;
    }
    return PoolMode::Centroid;
}

bool isTemporalSmoothEnabled()
{
    if (const char* v = std::getenv("VIT_BAKER_SMOOTH"))
    {
        const juce::String s(v);
        if (s == "0" || s.equalsIgnoreCase("off") || s.equalsIgnoreCase("false"))
            return false;
    }
    return true;
}

bool isBakeDebugLogEnabled()
{
    if (const char* v = std::getenv("VIT_BAKER_DEBUG"))
    {
        const juce::String s(v);
        return s == "1" || s.equalsIgnoreCase("true") || s.equalsIgnoreCase("on");
    }
    return false;
}

} // anonymous namespace

// ----------------------------------------------------------
// Build logarithmic frequency LUT: FFT bin -> UI bin index
// Maps kFftBins linear FFT bins onto kUiBins log-spaced slots
// ----------------------------------------------------------
std::vector<int> TiledSpectrogramBaker::buildFftToUiBin(double sr)
{
    std::vector<int> lut(kFftBins, 0);
    for (int i = 0; i < kFftBins; ++i)
    {
        float f = (float)i * (float)sr / (float)kSize;
        if (f < kMinFreq)
        {
            lut[i] = 0;
            continue;
        }
        if (f > kMaxFreq)
        {
            lut[i] = kUiBins - 1;
            continue;
        }
        // Logarithmic mapping: bin 0 -> kMinFreq, bin (kUiBins-1) -> kMaxFreq
        float t   = std::log2(f / kMinFreq) / std::log2(kMaxFreq / kMinFreq);
        int   bin = (int)std::lround(t * (kUiBins - 1));
        lut[i] = juce::jlimit(0, kUiBins - 1, bin);
    }
    return lut;
}

// ----------------------------------------------------------
// startBake: spawn background thread that fills shared memory
// ----------------------------------------------------------
void TiledSpectrogramBaker::startBake(juce::String filePath,
                                       juce::String trackId,
                                       juce::String clipId,
                                       PublishCallback publish,
                                       double sourceOffsetSeconds,
                                       double bakeLengthSeconds,
                                       juce::String sourceId,
                                       juce::String sourceRevision,
                                       juce::String clipRevision,
                                       juce::String renderRevision)
{
    const auto bakeKey = makeBakeKey (trackId, clipId);
    auto gen = beginGen (bakeKey);
    writeDiagLog ("[baker.lifecycle] start_bake key=" + bakeKey
                  + " gen=" + juce::String ((int64) gen)
                  + " track_id=" + trackId
                  + " clip_id=" + clipId
                  + " source_revision=" + sourceRevision
                  + " clip_revision=" + clipRevision
                  + " source_offset=" + juce::String (sourceOffsetSeconds, 4)
                  + " bake_length=" + juce::String (bakeLengthSeconds, 4)
                  + " file=" + filePath);
    std::thread([filePath = std::move(filePath),
                 trackId  = std::move(trackId),
                 clipId   = std::move(clipId),
                 bakeKey  = std::move(bakeKey),
                 publish  = std::move(publish),
                 sourceId = std::move(sourceId),
                 sourceRevision = std::move(sourceRevision),
                 clipRevision = std::move(clipRevision),
                 renderRevision = std::move(renderRevision),
                 sourceOffsetSeconds,
                 bakeLengthSeconds,
                 gen]() mutable
    {
        juce::AudioFormatManager fm;
        fm.registerBasicFormats();
        std::unique_ptr<juce::AudioFormatReader> reader(
            fm.createReaderFor(juce::File(filePath)));

        if (!reader || reader->lengthInSamples <= 0)
        {
            writeDiagLog(
                "[baker.lifecycle] reader_failed key=" + bakeKey
                + " gen=" + juce::String ((int64) gen)
                + " file=" + filePath);
            return;
        }

        const double sr = reader->sampleRate;
        const int64_t sourceStartSample = juce::jlimit<int64_t> (0, reader->lengthInSamples, (int64_t) std::floor (juce::jmax (0.0, sourceOffsetSeconds) * sr));
        const int64_t availableSamples = juce::jmax<int64_t> (0, reader->lengthInSamples - sourceStartSample);
        int64_t bakeTotalSamples = availableSamples;
        if (bakeLengthSeconds > 0.0)
            bakeTotalSamples = juce::jmin<int64_t> (availableSamples, (int64_t) std::floor (bakeLengthSeconds * sr));
        if (bakeTotalSamples <= 0)
        {
            writeDiagLog ("[baker.lifecycle] empty_bake key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + " track_id=" + trackId
                          + " clip_id=" + clipId);
            return;
        }

        // Fixed physical hop: 1 frame = 0.01 s exactly
        const int hopSize = juce::jmax(1, (int)(sr * kFrameSec));

        // Total tiles: each tile covers exactly 500 frames = 5 s
        const int64_t tileSpanSamples = (int64_t)hopSize * kFrames;
        const int totalTiles = (int)juce::jmax<int64_t>(
            1, (bakeTotalSamples + tileSpanSamples - 1) / tileSpanSamples);

        // Broadcast total duration immediately -- before any tile is baked
        // so Godot can update the scrollbar and accept_session without waiting
        // for the first tile_ready packet.
        const double totalDurationSecEarly = (double)bakeTotalSamples / sr;
        const auto sessionIdEarly = bakeKey + ":" + juce::String ((int64) gen);
        if (publish)
        {
            auto durObj = std::make_unique<juce::DynamicObject>();
            durObj->setProperty("command",        "track_duration_ready");
            durObj->setProperty("feature_family", "audio_feature");
            durObj->setProperty("feature_type",   audioFeatureTypeToString (AudioFeatureType::SpectralField));
            durObj->setProperty("feature_version", audioFeatureProductVersion (AudioFeatureType::SpectralField));
            durObj->setProperty("analysis_version", audioFeatureAnalysisVersion());
            durObj->setProperty("source_kind", clipId.isNotEmpty() ? "clip" : "file");
            stampIdentityProperties (*durObj,
                                     trackId,
                                     clipId,
                                     sourceId,
                                     sourceRevision,
                                     clipRevision,
                                     renderRevision,
                                     filePath,
                                     totalDurationSecEarly);
            durObj->setProperty("total_duration", totalDurationSecEarly);
            durObj->setProperty("session_id",     sessionIdEarly);
            durObj->setProperty("tile_count",     totalTiles);
            durObj->setProperty("tile_duration",  kFrameSec * kFrames);
            publish(juce::JSON::toString(juce::var(durObj.release())));
        }

        auto lut  = buildFftToUiBin(sr);
        const auto debugMode = getBakeDebugMode();
        const bool debugLog = isBakeDebugLogEnabled();
        const auto poolMode = getPoolMode();
        const bool smoothEnabled = isTemporalSmoothEnabled();
        (void)globalEnv(*reader); // gEnvNorm no longer written to tile (A = per-frame env)

        juce::dsp::FFT fft(kOrder);
        juce::dsp::WindowingFunction<float> win(
            kSize, juce::dsp::WindowingFunction<float>::hann, false);

        juce::AudioBuffer<float> rb(2, kSize);
        std::vector<float> fl((size_t)2 * kSize, 0.0f);
        std::vector<float> fr((size_t)2 * kSize, 0.0f);
        std::vector<float> tile((size_t)kFrames * kUiBins * kCh, 0.0f);
        std::vector<StereoRelationAcc> stereoAcc((size_t)kUiBins);

        // Per-frame raw FFT magnitudes for max-pool mapping
        std::vector<float> rawMagL((size_t)kFftBins, 0.0f);
        std::vector<float> rawMagR((size_t)kFftBins, 0.0f);
        // Per-frame log-mapped magnitudes (max-pool result)
        std::vector<float> mappedL((size_t)kUiBins, 0.0f);
        std::vector<float> mappedR((size_t)kUiBins, 0.0f);
        std::vector<float> smoothL((size_t)kUiBins, 0.0f);
        std::vector<float> smoothR((size_t)kUiBins, 0.0f);
        // LUT: for each UI bin, store [first_fft_bin, last_fft_bin)
        // Pre-build bin range LUT for max-pool
        std::vector<int> binRangeStart((size_t)kUiBins, 0);
        std::vector<int> binRangeEnd((size_t)kUiBins, 0);
        {
            // For each UI bin, find the range of FFT bins that map to it
            std::vector<int> firstSeen((size_t)kUiBins, kFftBins);
            std::vector<int> lastSeen((size_t)kUiBins, -1);
            for (int i = 0; i < kFftBins; ++i)
            {
                int uiBin = lut[(size_t)i];
                if (i < firstSeen[(size_t)uiBin]) firstSeen[(size_t)uiBin] = i;
                if (i > lastSeen[(size_t)uiBin])  lastSeen[(size_t)uiBin]  = i;
            }
            for (int b = 0; b < kUiBins; ++b)
            {
                binRangeStart[(size_t)b] = firstSeen[(size_t)b];
                binRangeEnd[(size_t)b]   = (lastSeen[(size_t)b] < 0) ? firstSeen[(size_t)b] : lastSeen[(size_t)b] + 1;
            }
        }

        for (int tileIndex = 0; tileIndex < totalTiles; ++tileIndex)
        {
            if (!isGen(bakeKey, gen)) return;

            // Zero the tile buffer (silent frames stay 0 for short audio)
            std::fill(tile.begin(), tile.end(), 0.0f);
            std::fill(smoothL.begin(), smoothL.end(), 0.0f);
            std::fill(smoothR.begin(), smoothR.end(), 0.0f);
            const float alphaAttack  = 1.0f - std::exp(-kFrameSec / juce::jmax(1.0e-4f, kTemporalSmoothTauAttackSec));
            const float alphaRelease = 1.0f - std::exp(-kFrameSec / juce::jmax(1.0e-4f, kTemporalSmoothTauReleaseSec));
            int prevPeakBinL = -1, prevPeakBinR = -1;
            float driftSumBinL = 0.0f, driftSumBinR = 0.0f;
            int driftCountL = 0, driftCountR = 0;
            [[maybe_unused]] int activeFrameCount = 0;
            [[maybe_unused]] int silentFrameCount = 0;
            QualityStats readerStats;
            QualityStats fftInputStats;
            QualityStats fftOutputStats;

            const int64_t tileStartSample = (int64_t)tileIndex * tileSpanSamples;

            for (int frame = 0; frame < kFrames; ++frame)
            {
                if (!isGen(bakeKey, gen)) return;

                // Absolute sample position locked to physical time
                const int64_t frameStartSample =
                    tileStartSample + (int64_t)frame * hopSize;

                // Past end of audio: remainder is silence (already zeroed)
                if (frameStartSample >= bakeTotalSamples) break;

                rb.clear();
                reader->read(&rb, 0, kSize, sourceStartSample + frameStartSample, true, true);

                auto* lPtr = rb.getReadPointer(0);
                auto* rPtr = rb.getReadPointer(
                    juce::jmin(1, rb.getNumChannels() - 1));
                const int frameReadableSamples = (int) juce::jlimit<int64_t> (
                    0,
                    (int64_t) kSize,
                    bakeTotalSamples - frameStartSample);
                // Step 1: frame-level envelope over the FFT input window.
                // The quality gate must follow the data actually sent into FFT;
                // otherwise nonzero reader samples can be discarded before evidence is produced.
                float envL = 0.0f, envR = 0.0f;
                for (int s = 0; s < frameReadableSamples; ++s)
                {
                    readerStats.observe (lPtr[s]);
                    readerStats.observe (rPtr[s]);
                    envL = juce::jmax(envL, std::abs(lPtr[s]));
                    envR = juce::jmax(envR, std::abs(rPtr[s]));
                }

                // Step 2: hard noise gate — skip silent frames entirely
                if (envL < kEnvSilence && envR < kEnvSilence)
                {
                    ++silentFrameCount;
                    continue; // tile already zeroed, skip FFT
                }
                ++activeFrameCount;

                // Apply Hann window and FFT
                prepWindow(lPtr, kSize, win, fl);
                prepWindow(rPtr, kSize, win, fr);
                for (int i = 0; i < kSize; ++i)
                {
                    fftInputStats.observe (fl[(size_t) i]);
                    fftInputStats.observe (fr[(size_t) i]);
                }
                fft.performRealOnlyForwardTransform(fl.data());
                fft.performRealOnlyForwardTransform(fr.data());

                // Extract raw linear magnitudes
                for (int i = 0; i < kFftBins; ++i)
                {
                    float lr = 0, li = 0, rr = 0, rih = 0;
                    ri(fl, i, lr, li);
                    ri(fr, i, rr, rih);
                    rawMagL[(size_t)i] = std::sqrt(lr * lr + li * li);
                    rawMagR[(size_t)i] = std::sqrt(rr * rr + rih * rih);
                    fftOutputStats.observe (rawMagL[(size_t)i]);
                    fftOutputStats.observe (rawMagR[(size_t)i]);
                }

                // Step 3: log UI bin aggregation (selectable by env for diagnosis).
                for (int b = 0; b < kUiBins; ++b)
                {
                    int bStart = binRangeStart[(size_t)b];
                    int bEnd   = binRangeEnd[(size_t)b];
                    if (bStart >= bEnd) bEnd = bStart + 1;
                    float outL = 0.0f, outR = 0.0f;
                    if (poolMode == PoolMode::Max)
                    {
                        for (int i = bStart; i < bEnd && i < kFftBins; ++i)
                        {
                            outL = juce::jmax(outL, rawMagL[(size_t)i]);
                            outR = juce::jmax(outR, rawMagR[(size_t)i]);
                        }
                    }
                    else if (poolMode == PoolMode::Top3)
                    {
                        float m1L = 0.0f, m2L = 0.0f, m3L = 0.0f;
                        float m1R = 0.0f, m2R = 0.0f, m3R = 0.0f;
                        for (int i = bStart; i < bEnd && i < kFftBins; ++i)
                        {
                            const float vL = rawMagL[(size_t)i];
                            const float vR = rawMagR[(size_t)i];
                            if (vL >= m1L) { m3L = m2L; m2L = m1L; m1L = vL; }
                            else if (vL >= m2L) { m3L = m2L; m2L = vL; }
                            else if (vL > m3L) { m3L = vL; }
                            if (vR >= m1R) { m3R = m2R; m2R = m1R; m1R = vR; }
                            else if (vR >= m2R) { m3R = m2R; m2R = vR; }
                            else if (vR > m3R) { m3R = vR; }
                        }
                        int kL = 0; if (m1L > 0.0f) ++kL; if (m2L > 0.0f) ++kL; if (m3L > 0.0f) ++kL;
                        int kR = 0; if (m1R > 0.0f) ++kR; if (m2R > 0.0f) ++kR; if (m3R > 0.0f) ++kR;
                        outL = (kL > 0) ? ((m1L + m2L + m3L) / (float)kL) : 0.0f;
                        outR = (kR > 0) ? ((m1R + m2R + m3R) / (float)kR) : 0.0f;
                    }
                    else // PoolMode::Centroid
                    {
                        float peakL = 0.0f, peakR = 0.0f;
                        for (int i = bStart; i < bEnd && i < kFftBins; ++i)
                        {
                            const float vL = rawMagL[(size_t)i];
                            const float vR = rawMagR[(size_t)i];
                            peakL = juce::jmax(peakL, vL);
                            peakR = juce::jmax(peakR, vR);
                        }
                        const float gateL = peakL * 0.45f;
                        const float gateR = peakR * 0.45f;
                        float sumWL = 0.0f, sumVL = 0.0f;
                        float sumWR = 0.0f, sumVR = 0.0f;
                        for (int i = bStart; i < bEnd && i < kFftBins; ++i)
                        {
                            const float vL = rawMagL[(size_t)i];
                            const float vR = rawMagR[(size_t)i];
                            if (vL >= gateL) { sumWL += vL; sumVL += vL * vL; }
                            if (vR >= gateR) { sumWR += vR; sumVR += vR * vR; }
                        }
                        outL = (sumWL > 1.0e-12f) ? (sumVL / sumWL) : peakL;
                        outR = (sumWR > 1.0e-12f) ? (sumVR / sumWR) : peakR;
                    }
                    mappedL[(size_t)b] = outL;
                    mappedR[(size_t)b] = outR;
                }

                // Step 3b: temporal smoothing in time direction (frame-by-frame per frequency bin).
                // Fast attack keeps motion responsive; slower release suppresses jitter/flicker.
                if (smoothEnabled)
                {
                    for (int b = 0; b < kUiBins; ++b)
                    {
                        const float curL = mappedL[(size_t)b];
                        const float curR = mappedR[(size_t)b];
                        const float prevL = smoothL[(size_t)b];
                        const float prevR = smoothR[(size_t)b];
                        const float aL = (curL >= prevL) ? alphaAttack : alphaRelease;
                        const float aR = (curR >= prevR) ? alphaAttack : alphaRelease;
                        smoothL[(size_t)b] = aL * curL + (1.0f - aL) * prevL;
                        smoothR[(size_t)b] = aR * curR + (1.0f - aR) * prevR;
                        mappedL[(size_t)b] = smoothL[(size_t)b];
                        mappedR[(size_t)b] = smoothR[(size_t)b];
                    }
                }

                // Step 4: column-level normalisation + power sharpening + envelope scale
                float colMaxL = 1e-10f, colMaxR = 1e-10f;
                for (int b = 0; b < kUiBins; ++b)
                {
                    colMaxL = juce::jmax(colMaxL, mappedL[(size_t)b]);
                    colMaxR = juce::jmax(colMaxR, mappedR[(size_t)b]);
                }

                // Step 4a: optional adaptive soft gate.
                const float noiseL = percentileNonZero(mappedL, kGateNoisePercentile);
                const float noiseR = percentileNonZero(mappedR, kGateNoisePercentile);
                const float thresholdL = adaptiveThreshold(colMaxL, noiseL);
                const float thresholdR = adaptiveThreshold(colMaxR, noiseR);
                const bool useGate = (debugMode == BakeDebugMode::Full || debugMode == BakeDebugMode::GateOnly || debugMode == BakeDebugMode::NoNorm);
                if (useGate)
                {
                    for (int b = 0; b < kUiBins; ++b)
                    {
                        if (mappedL[(size_t)b] < thresholdL)
                        {
                            const float ratioL = mappedL[(size_t)b] / juce::jmax(thresholdL, 1.0e-12f);
                            mappedL[(size_t)b] *= ratioL; // soft attenuation, preserve objective continuity
                        }
                        if (mappedR[(size_t)b] < thresholdR)
                        {
                            const float ratioR = mappedR[(size_t)b] / juce::jmax(thresholdR, 1.0e-12f);
                            mappedR[(size_t)b] *= ratioR;
                        }
                    }
                }

                int peakBinL = 0, peakBinR = 0;
                float peakValL = 0.0f, peakValR = 0.0f;
                for (int b = 0; b < kUiBins; ++b)
                {
                    if (mappedL[(size_t)b] > peakValL) { peakValL = mappedL[(size_t)b]; peakBinL = b; }
                    if (mappedR[(size_t)b] > peakValR) { peakValR = mappedR[(size_t)b]; peakBinR = b; }
                }
                if (prevPeakBinL >= 0) { driftSumBinL += std::abs((float)peakBinL - (float)prevPeakBinL); ++driftCountL; }
                if (prevPeakBinR >= 0) { driftSumBinR += std::abs((float)peakBinR - (float)prevPeakBinR); ++driftCountR; }
                prevPeakBinL = peakBinL;
                prevPeakBinR = peakBinR;

                if (debugLog && (frame % kDebugLogFrameStride == 0))
                {
                    writeDiagLog(
                        "BakerDbg mode=" + juce::String((int)debugMode) +
                        " pool=" + juce::String((int)poolMode) +
                        " smooth=" + juce::String(smoothEnabled ? 1 : 0) +
                        " tile=" + juce::String(tileIndex) +
                        " frame=" + juce::String(frame) +
                        " thL=" + juce::String(thresholdL, 8) +
                        " thR=" + juce::String(thresholdR, 8) +
                        " colMaxL=" + juce::String(colMaxL, 8) +
                        " colMaxR=" + juce::String(colMaxR, 8) +
                        " peakL_bin=" + juce::String(peakBinL) + " peakL_hz=" + juce::String(uiBinToHz(peakBinL), 3) +
                        " peakR_bin=" + juce::String(peakBinR) + " peakR_hz=" + juce::String(uiBinToHz(peakBinR), 3));
                }

                // Stereo relation: aggregate L * conj(R) per UI bin so B/A are
                // acoustic primitives, not a visual envelope surrogate.
                std::fill(stereoAcc.begin(), stereoAcc.end(), StereoRelationAcc{});
                for (int i = 0; i < kFftBins; ++i)
                {
                    float lr = 0, li = 0, rr = 0, rih = 0;
                    ri(fl, i, lr, li);
                    ri(fr, i, rr, rih);
                    auto& a = stereoAcc[(size_t)lut[(size_t)i]];
                    a.crossRe += (double) lr * (double) rr + (double) li * (double) rih;
                    a.crossIm += (double) li * (double) rr - (double) lr * (double) rih;
                    a.powerL += (double) lr * (double) lr + (double) li * (double) li;
                    a.powerR += (double) rr * (double) rr + (double) rih * (double) rih;
                    ++a.count;
                }

                // Write to tile: index (bin * kFrames + frame) * kCh — matches Godot
                // Image 500x336 row-major: pixel (x,y)=(frame,bin) -> y*500+x = bin*500+frame
                // UV.x = time, UV.y = frequency bin (shader samples data_matrix(UV))
                // Data (step9_ground_truth_baker parity, SFFT + max-pool instead of CQT):
                //   R = (magL/colMaxL)^1.5 * envL (0 if envL silent)
                //   G = (magR/colMaxR)^1.5 * envR
                //   B = cross-spectrum phase relation L*conj(R), radians [-pi, pi]
                //   A = display weight: energy * L/R balance * coherence
                for (int b = 0; b < kUiBins; ++b)
                {
                    auto base = (size_t)((b * kFrames + frame) * kCh);

                    const bool useNorm = (debugMode == BakeDebugMode::Full || debugMode == BakeDebugMode::NormOnly);
                    float outL = mappedL[(size_t)b];
                    float outR = mappedR[(size_t)b];
                    const float normL = mappedL[(size_t)b] / juce::jmax(colMaxL, 1.0e-12f);
                    const float normR = mappedR[(size_t)b] / juce::jmax(colMaxR, 1.0e-12f);
                    if (useNorm)
                    {
                        outL = std::pow(normL, 1.5f);
                        outR = std::pow(normR, 1.5f);
                    }
                    tile[base + 0] = (envL < kEnvSilence) ? 0.0f : (outL * envL);
                    tile[base + 1] = (envR < kEnvSilence) ? 0.0f : (outR * envR);

                    auto& a = stereoAcc[(size_t)b];
                    const double crossMag = std::sqrt(a.crossRe * a.crossRe + a.crossIm * a.crossIm);
                    const double powerProduct = a.powerL * a.powerR;
                    const double coherence = powerProduct > 1.0e-24
                        ? juce::jlimit(0.0, 1.0, crossMag / std::sqrt(powerProduct))
                        : 0.0;
                    const double maxPower = juce::jmax(a.powerL, a.powerR);
                    const double balance = maxPower > 1.0e-24
                        ? juce::jlimit(0.0, 1.0, juce::jmin(a.powerL, a.powerR) / maxPower)
                        : 0.0;
                    const float spectralWeight = juce::jlimit(0.0f, 1.0f, juce::jmax(normL, normR));
                    const float frameWeight = juce::jlimit(0.0f, 1.0f, juce::jmax(envL, envR));

                    tile[base + 2] = (a.count > 0 && crossMag > 1.0e-24)
                        ? (float) std::atan2(a.crossIm, a.crossRe)
                        : 0.0f;
                    tile[base + 3] = (float) juce::jlimit(0.0, 1.0,
                                                         coherence * balance * (double) spectralWeight * (double) frameWeight);
                }
            }

            if (debugLog)
            {
                const float avgJumpL = (driftCountL > 0) ? (driftSumBinL / (float)driftCountL) : 0.0f;
                const float avgJumpR = (driftCountR > 0) ? (driftSumBinR / (float)driftCountR) : 0.0f;
                writeDiagLog(
                    "BakerDiag tile=" + juce::String(tileIndex) +
                    " mode=" + juce::String((int)debugMode) +
                    " pool=" + juce::String((int)poolMode) +
                    " smooth=" + juce::String(smoothEnabled ? 1 : 0) +
                    " avg_peak_jump_bin_L=" + juce::String(avgJumpL, 3) +
                    " avg_peak_jump_bin_R=" + juce::String(avgJumpR, 3));
            }

            [[maybe_unused]] const double totalDurationSec = bakeTotalSamples > 0
                ? ((double) bakeTotalSamples / sr)
                : 0.0;
            const int64_t tileRemainingSamples = juce::jmax<int64_t> (0, bakeTotalSamples - tileStartSample);
            const int64_t tileValidSamples = juce::jmin<int64_t> (tileSpanSamples, tileRemainingSamples);
            [[maybe_unused]] const double tileDurationSec = tileValidSamples > 0
                ? ((double) tileValidSamples / sr)
                : 0.0;
            [[maybe_unused]] const double tileContentStartSeconds = sourceOffsetSeconds + ((double) tileStartSample / sr);
            [[maybe_unused]] const auto tileStats = collectQualityStats (tile.data(), tile.size());

            // Write tile to shared memory
            auto sessionId = bakeKey + ":" + juce::String ((int64) gen);
            auto descriptiveName = "Vit_Waveform_" + sanitiseBakeKeyForShm (bakeKey)
                + "_g" + juce::String ((int64) gen) + "_" + juce::String(tileIndex);
            auto shm = juce::String (ISharedMemorySegment::platformPublishedName (descriptiveName.toStdString()));
            auto bytes = tile.size() * sizeof(float);
            std::string segmentError;
            auto segment = ISharedMemorySegment::createAndMap (descriptiveName.toStdString(), bytes, segmentError);
            if (!segment)
            {
                writeDiagLog(
                    "[baker.lifecycle] segment_create_failed key=" + bakeKey
                    + " gen=" + juce::String ((int64) gen)
                    + " track_id=" + trackId
                    + " clip_id=" + clipId
                    + " tile_index=" + juce::String(tileIndex)
                    + " bytes=" + juce::String ((int64) bytes)
                    + " shm=" + shm
                    + " detail=" + segmentError);
                continue;
            }
            auto* mapped = (float*)segment->writableData();
            std::memset(mapped, 0, bytes);
            std::memcpy(mapped, tile.data(), tile.size() * sizeof(float));
            const auto shmStats = collectQualityStats (mapped, tile.size());
            segment->unmapView();

            if (!storeSegment(bakeKey, gen, std::move(segment))) return;
            const auto handleCount = handleCountForGen (bakeKey, gen);
            const auto quality = decideSpectralQuality (readerStats,
                                                        fftInputStats,
                                                        fftOutputStats,
                                                        tileStats,
                                                        shmStats,
                                                        tileValidSamples,
                                                        kFrames,
                                                        activeFrameCount);
            const double coverageRatio = bakeTotalSamples > 0
                ? juce::jlimit (0.0, 1.0, (double) tileValidSamples / (double) bakeTotalSamples)
                : 0.0;
            writeDiagLog ("[baker.lifecycle] publish_tile key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + " track_id=" + trackId
                          + " clip_id=" + clipId
                          + " tile_index=" + juce::String (tileIndex)
                          + " handle_count=" + juce::String ((int) handleCount)
                          + " quality_status=" + quality.status
                          + " quality_reason=" + quality.reason
                          + " active_frames=" + juce::String (activeFrameCount)
                          + " nonzero=" + juce::String ((int64) tileStats.nonzeroCount)
                          + " bytes=" + juce::String ((int64) bytes)
                          + " shm=" + shm);

            if (publish)
            {
                auto obj = std::make_unique<juce::DynamicObject>();
                obj->setProperty("command",       "tile_ready");
                obj->setProperty("feature_family", "audio_feature");
                obj->setProperty("feature_type",   audioFeatureTypeToString (AudioFeatureType::SpectralField));
                obj->setProperty("feature_version", audioFeatureProductVersion (AudioFeatureType::SpectralField));
                obj->setProperty("analysis_version", audioFeatureAnalysisVersion());
                obj->setProperty("channels_semantics", "r=left_energy,g=right_energy,b=cross_spectrum_phase_delta,a=phase_display_weight");
                obj->setProperty("source_kind", clipId.isNotEmpty() ? "clip" : "file");
                stampIdentityProperties (*obj,
                                         trackId,
                                         clipId,
                                         sourceId,
                                         sourceRevision,
                                         clipRevision,
                                         renderRevision,
                                         filePath,
                                         totalDurationSec);
                obj->setProperty("session_id",    sessionId);
                obj->setProperty("file_path",     filePath);
                obj->setProperty("tile_index",    tileIndex);
                obj->setProperty("tile_count",    totalTiles);
                obj->setProperty("tile_duration", tileDurationSec);
                obj->setProperty("tile_content_start_seconds", tileContentStartSeconds);
                obj->setProperty("range_source_offset_seconds", sourceOffsetSeconds);
                obj->setProperty("range_length_seconds", bakeLengthSeconds);
                obj->setProperty("coverage_seconds", tileDurationSec);
                obj->setProperty("coverage_ratio", coverageRatio);
                obj->setProperty("frame_count", kFrames);
                obj->setProperty("audio_sample_count", (int64) tileValidSamples);
                obj->setProperty("active_frame_count", activeFrameCount);
                obj->setProperty("silent_frame_count", silentFrameCount);
                obj->setProperty("resolution_frame_width", kFrames);
                obj->setProperty("resolution_frequency_bins", kUiBins);
                obj->setProperty("frame_duration_seconds", kFrameSec);
                obj->setProperty("total_duration", totalDurationSec);
                obj->setProperty("shared_memory", shm);
                obj->setProperty("float_count", (int64) tile.size());
                obj->setProperty("bake_key", bakeKey);
                obj->setProperty("generation", (int64) gen);
                obj->setProperty("handle_count", (int) handleCount);
                obj->setProperty("shm_bytes", (int64) bytes);
                obj->setProperty("quality_status", quality.status);
                obj->setProperty("quality_reason", quality.reason);
                obj->setProperty("ready", quality.status == "ready");
                setQualityStatsProperties (*obj, {}, tileStats);
                setQualityStatsProperties (*obj, "reader_", readerStats);
                setQualityStatsProperties (*obj, "fft_input_", fftInputStats);
                setQualityStatsProperties (*obj, "fft_output_", fftOutputStats);
                setQualityStatsProperties (*obj, "shm_postwrite_", shmStats);
                publish(juce::JSON::toString(juce::var(obj.release())));
            }
        }
    }).detach();
}

void TiledSpectrogramBaker::releaseTrackMappings(const juce::String& trackId)
{
    writeDiagLog ("[baker.lifecycle] release_track_mappings track_id=" + trackId
                  + " note=track_key_only_clip_scoped_bakes_use_clip_id_key");
    beginGen(trackId);
}

void TiledSpectrogramBaker::invalidateClipBake (const juce::String& clipId)
{
    const auto trimmed = clipId.trim();
    writeDiagLog ("[baker.lifecycle] invalidate_clip_bake clip_id=" + trimmed);
    if (trimmed.isNotEmpty())
        beginGen (trimmed);
}

} // namespace vit
