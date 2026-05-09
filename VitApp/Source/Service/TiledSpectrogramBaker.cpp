#include "TiledSpectrogramBaker.h"
#include "../Core/VitPaths.h"
#include <algorithm>
#include <chrono>
#include <cstdlib>
#include <windows.h>
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
std::map<std::string, std::vector<HANDLE>> gHandles;
std::map<std::string, uint64_t>            gGen;

struct RetiredHandle
{
    std::string key;
    uint64_t gen = 0;
    HANDLE handle = nullptr;
    std::chrono::steady_clock::time_point releaseAt;
};

std::vector<RetiredHandle> gRetiredHandles;

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
struct Acc { float l = 0, r = 0, p = 0; int c = 0; };

uint64_t beginGen(const juce::String& id)
{
    std::lock_guard<std::mutex> lock(gMutex);
    cleanupRetiredHandlesUnlocked();
    auto key = id.toStdString();
    auto& gen = gGen[key]; ++gen;
    auto& handles = gHandles[key];
    const auto releasedCount = handles.size();
    const auto releaseAt = std::chrono::steady_clock::now() + std::chrono::milliseconds (kRetiredHandleGraceMs);
    for (auto h : handles)
    {
        if (h)
            gRetiredHandles.push_back ({ key, gen - 1, h, releaseAt });
    }
    handles.clear();
    writeDiagLog ("[baker.lifecycle] begin_gen key=" + id
                  + " gen=" + juce::String ((int64) gen)
                  + " retired_handles=" + juce::String ((int) releasedCount)
                  + " retired_total=" + juce::String ((int) gRetiredHandles.size())
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

bool storeHandle(const juce::String& id, uint64_t gen, HANDLE h)
{
    std::lock_guard<std::mutex> lock(gMutex);
    cleanupRetiredHandlesUnlocked();
    auto key = id.toStdString();
    auto it  = gGen.find(key);
    if (it == gGen.end() || it->second != gen)
    {
        writeDiagLog ("[baker.lifecycle] store_handle_reject key=" + id
                      + " gen=" + juce::String ((int64) gen)
                      + " current_gen=" + juce::String (it == gGen.end() ? -1 : (int64) it->second));
        return false;
    }
    gHandles[key].push_back(h);
    writeDiagLog ("[baker.lifecycle] store_handle key=" + id
                  + " gen=" + juce::String ((int64) gen)
                  + " handle_count=" + juce::String ((int) gHandles[key].size()));
    return true;
}

size_t handleCountForGen (const juce::String& id, uint64_t gen)
{
    std::lock_guard<std::mutex> lock(gMutex);
    auto key = id.toStdString();
    auto it  = gGen.find(key);
    if (it == gGen.end() || it->second != gen)
        return 0u;
    return handleCountUnlocked (key);
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
                                       double bakeLengthSeconds)
{
    const auto bakeKey = makeBakeKey (trackId, clipId);
    auto gen = beginGen (bakeKey);
    writeDiagLog ("[baker.lifecycle] start_bake key=" + bakeKey
                  + " gen=" + juce::String ((int64) gen)
                  + " track_id=" + trackId
                  + " clip_id=" + clipId
                  + " source_offset=" + juce::String (sourceOffsetSeconds, 4)
                  + " bake_length=" + juce::String (bakeLengthSeconds, 4)
                  + " file=" + filePath);
    std::thread([filePath = std::move(filePath),
                 trackId  = std::move(trackId),
                 clipId   = std::move(clipId),
                 bakeKey  = std::move(bakeKey),
                 publish  = std::move(publish),
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
            durObj->setProperty("track_id",       trackId);
            durObj->setProperty("total_duration", totalDurationSecEarly);
            durObj->setProperty("session_id",     sessionIdEarly);
            if (clipId.isNotEmpty())
                durObj->setProperty("clip_id", clipId);
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
        std::vector<Acc>   acc((size_t)kUiBins);

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

                // Step 1: frame-level envelope (peak of hopSize window)
                float envL = 0.0f, envR = 0.0f;
                for (int s = 0; s < hopSize; ++s)
                {
                    envL = juce::jmax(envL, std::abs(lPtr[s]));
                    envR = juce::jmax(envR, std::abs(rPtr[s]));
                }

                // Step 2: hard noise gate — skip silent frames entirely
                if (envL < kEnvSilence && envR < kEnvSilence)
                    continue; // tile already zeroed, skip FFT

                // Apply Hann window and FFT
                prepWindow(lPtr, kSize, win, fl);
                prepWindow(rPtr, kSize, win, fr);
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

                // Phase: recompute per bin using accumulated avg (same as before)
                std::fill(acc.begin(), acc.end(), Acc{});
                for (int i = 0; i < kFftBins; ++i)
                {
                    float lr = 0, li = 0, rr = 0, rih = 0;
                    ri(fl, i, lr, li);
                    ri(fr, i, rr, rih);
                    auto lp = std::atan2(li, lr);
                    auto rp = std::atan2(rih, rr);
                    auto pd = std::remainder(
                        rp - lp, juce::MathConstants<float>::twoPi);
                    auto& a = acc[(size_t)lut[(size_t)i]];
                    a.p += pd; ++a.c;
                }

                // Write to tile: index (bin * kFrames + frame) * kCh — matches Godot
                // Image 500x336 row-major: pixel (x,y)=(frame,bin) -> y*500+x = bin*500+frame
                // UV.x = time, UV.y = frequency bin (shader samples data_matrix(UV))
                // Data (step9_ground_truth_baker parity, SFFT + max-pool instead of CQT):
                //   R = (magL/colMaxL)^1.5 * envL (0 if envL silent)
                //   G = (magR/colMaxR)^1.5 * envR
                //   B = phase difference (per UI bin)
                //   A = max(envL, envR) for TIME view silhouette
                for (int b = 0; b < kUiBins; ++b)
                {
                    auto base = (size_t)((b * kFrames + frame) * kCh);

                    const bool useNorm = (debugMode == BakeDebugMode::Full || debugMode == BakeDebugMode::NormOnly);
                    float outL = mappedL[(size_t)b];
                    float outR = mappedR[(size_t)b];
                    if (useNorm)
                    {
                        outL = std::pow(mappedL[(size_t)b] / colMaxL, 1.5f);
                        outR = std::pow(mappedR[(size_t)b] / colMaxR, 1.5f);
                    }
                    tile[base + 0] = (envL < kEnvSilence) ? 0.0f : (outL * envL);
                    tile[base + 1] = (envR < kEnvSilence) ? 0.0f : (outR * envR);

                    auto& a = acc[(size_t)b];
                    tile[base + 2] = (a.c > 0) ? a.p / (float)a.c : 0.0f;

                    tile[base + 3] = juce::jmax(envL, envR);
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

            const double totalDurationSec = bakeTotalSamples > 0
                ? ((double) bakeTotalSamples / sr)
                : 0.0;
            const int64_t tileRemainingSamples = juce::jmax<int64_t> (0, bakeTotalSamples - tileStartSample);
            const int64_t tileValidSamples = juce::jmin<int64_t> (tileSpanSamples, tileRemainingSamples);
            const double tileDurationSec = tileValidSamples > 0
                ? ((double) tileValidSamples / sr)
                : 0.0;
            const double tileContentStartSeconds = sourceOffsetSeconds + ((double) tileStartSample / sr);

            // Write tile to shared memory
            auto sessionId = bakeKey + ":" + juce::String ((int64) gen);
            auto shm = "Vit_Waveform_" + sanitiseBakeKeyForShm (bakeKey) + "_g" + juce::String ((int64) gen) + "_" + juce::String(tileIndex);
            auto bytes = (SIZE_T)(tile.size() * sizeof(float));
            HANDLE h   = CreateFileMappingA(
                INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE,
                0, (DWORD)bytes, shm.toRawUTF8());
            if (!h)
            {
                writeDiagLog(
                    "[baker.lifecycle] create_mapping_failed key=" + bakeKey
                    + " gen=" + juce::String ((int64) gen)
                    + " track_id=" + trackId
                    + " clip_id=" + clipId
                    + " tile_index=" + juce::String (tileIndex)
                    + " bytes=" + juce::String ((int64) bytes)
                    + " shm=" + shm
                    + " win_error=" + juce::String ((int) GetLastError()));
                continue;
            }
            auto* mapped = (float*)MapViewOfFile(h, FILE_MAP_ALL_ACCESS, 0, 0, bytes);
            if (!mapped)
            {
                writeDiagLog(
                    "[baker.lifecycle] map_view_failed key=" + bakeKey
                    + " gen=" + juce::String ((int64) gen)
                    + " track_id=" + trackId
                    + " clip_id=" + clipId
                    + " tile_index=" + juce::String (tileIndex)
                    + " bytes=" + juce::String ((int64) bytes)
                    + " shm=" + shm
                    + " win_error=" + juce::String ((int) GetLastError()));
                CloseHandle(h);
                continue;
            }
            std::memset(mapped, 0, bytes);
            std::memcpy(mapped, tile.data(), tile.size() * sizeof(float));
            UnmapViewOfFile(mapped);

            if (!storeHandle(bakeKey, gen, h)) { CloseHandle(h); return; }
            const auto handleCount = handleCountForGen (bakeKey, gen);
            writeDiagLog ("[baker.lifecycle] publish_tile key=" + bakeKey
                          + " gen=" + juce::String ((int64) gen)
                          + " track_id=" + trackId
                          + " clip_id=" + clipId
                          + " tile_index=" + juce::String (tileIndex)
                          + " handle_count=" + juce::String ((int) handleCount)
                          + " bytes=" + juce::String ((int64) bytes)
                          + " shm=" + shm);

            if (publish)
            {
                auto obj = std::make_unique<juce::DynamicObject>();
                obj->setProperty("command",       "tile_ready");
                obj->setProperty("track_id",      trackId);
                obj->setProperty("source_track_id", trackId);
                obj->setProperty("session_id",    sessionId);
                obj->setProperty("file_path",     filePath);
                obj->setProperty("tile_index",    tileIndex);
                obj->setProperty("tile_duration", tileDurationSec);
                obj->setProperty("tile_content_start_seconds", tileContentStartSeconds);
                obj->setProperty("total_duration", totalDurationSec);
                obj->setProperty("shared_memory", shm);
                obj->setProperty("bake_key", bakeKey);
                obj->setProperty("generation", (int64) gen);
                obj->setProperty("handle_count", (int) handleCount);
                obj->setProperty("shm_bytes", (int64) bytes);
                if (clipId.isNotEmpty())
                    obj->setProperty("clip_id", clipId);
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
