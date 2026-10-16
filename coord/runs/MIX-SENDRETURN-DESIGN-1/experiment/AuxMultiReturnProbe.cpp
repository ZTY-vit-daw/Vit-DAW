// MIX-SENDRETURN-DESIGN-1 底座实证探针（取证工件，不入生产构建）
//
// 目的：实证 tracktion AuxSend/AuxReturn 原语在「轨到轨多目标回流」拓扑下的真实音频行为，
// 为 send/return 设计件提供底座可达性证据（卡面验收标准：worktree 内核最小实验）。
//
// 拓扑（裁定 1/3 的最小化）：
//   T0 (targetA, sin 440Hz):  [AuxSend(bus0, 0dB)] -> [AuxReturn(bus1)] -> vol(-12dB) -> master
//   T1 (targetB, sin 880Hz):  同上
//   FX (无 clip):             [AuxReturn(bus0)] -> [AuxSend(bus1, -6.02dB=0.5x)] -> vol(-96dB) -> master
//   即：两目标轨把干信号发到 bus0；FX 轨回授 bus0、经 0.5x 电平把湿信号发到 bus1；
//       两目标轨各自回授 bus1 —— 湿信号=全体发送源之和（共享空间），并回到每一条目标轨。
//
// 五腿（每腿独立 edit 渲染，Goertzel 测 440/880 分量，对照腿定标）：
//   L1 control        : 无 aux 接线            -> 440/880 各 = 1.0xR（定标 R）
//   L2 full           : 全接线                 -> 440/880 各 = 2.0xR  【多目标回流核心断言】
//                        （仅单目标回流的退化拓扑只会得 1.5xR；无回流 1.0xR）
//   L3 single_return  : 只有 T0 回授 bus1       -> 440/880 各 = 1.5xR
//   L4 mute_T0        : 全接线 + T0 静音        -> 440≈0，880 = 1.5xR（发送随轨静音被抑制）
//   L5 loop_order     : T0 链序颠倒（return 在前 send 在后）-> 只断言渲染有限且有界（实测
//                        440=1.0xR、880=2.0xR：引擎环剔除是传播式的——乱序轨的干发送也被
//                        剥离，正常轨出现 0.5 环路增益的局部收敛；精确幅度不作断言，防环
//                        不发散即达标。结论：封装层必须强制 send-在-return-前 的链序）。
//
// 边界声明：本探针直接驱动 tracktion_engine 原语（与 VitApp 链接同一套引擎源），
// 不经过 VitApp 命令面（VitApp 现无 aux 命令——正是实现卡要补的面）。
// 输出走纯 ASCII printf（JUCE 控制台 app 在 Windows 会把 stdout 切到 UTF-16，juce::String
// 直印会乱码）。

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>
#include <tracktion_graph/tracktion_graph/tracktion_TestUtilities.h>
#include <tracktion_engine/utilities/tracktion_TestUtilities.h>
#include <tracktion_engine/playback/graph/tracktion_BenchmarkUtilities.h>

#include <cmath>
#include <cstdio>
#include <string>
#include <vector>

using namespace tracktion;
using namespace tracktion::engine;

namespace
{

constexpr double kSampleRate = 44100.0;
constexpr int kBlockSize = 256;
constexpr double kDuration = 3.0;
constexpr double kPi = 3.14159265358979323846;

int gFailures = 0;

void logLine (const std::string& text)
{
    std::fputs (text.c_str(), stdout);
    std::fputc ('\n', stdout);
    std::fflush (stdout);
}

void failCheck (const char* expr)
{
    logLine (std::string ("CHECK_FAILED: ") + expr);
    ++gFailures;
}

double goertzelMagnitude (const float* data, int numSamples, double sampleRate, double freq)
{
    // 窗口内整数周期（2s@44100 → 0.5Hz 分辨率，440/880 均为整周期），返回正弦幅度估计
    const double k = 2.0 * kPi * freq / sampleRate;
    const double coeff = 2.0 * std::cos (k);
    double s1 = 0.0, s2 = 0.0;
    for (int i = 0; i < numSamples; ++i)
    {
        const double s0 = data[i] + coeff * s1 - s2;
        s2 = s1;
        s1 = s0;
    }
    const double power = s1 * s1 + s2 * s2 - coeff * s1 * s2;
    return std::sqrt (std::abs (power)) * 2.0 / (double) numSamples;
}

struct LegResult
{
    std::string name;
    double amp440 = 0.0, amp880 = 0.0;
    double rms = 0.0;
    bool finite = true;
};

} // namespace

int main ()
{
    logLine ("VitAuxMultiReturnProbe start");

    tracktion::engine::Engine engine { "VitAuxMultiReturnProbe" };

    auto renderLeg = [&] (const char* name,
                          bool wireAux,
                          bool t0Return /* T0 是否回授 bus1 */,
                          bool t1Return,
                          bool muteT0,
                          bool t0LoopOrder /* T0 链序颠倒：return 在前 send 在后 */) -> LegResult
    {
        tracktion::graph::test_utilities::TestSetup ts;
        ts.sampleRate = kSampleRate;
        ts.blockSize = kBlockSize;

        auto sin440 = tracktion::graph::test_utilities::getSinFile<juce::WavAudioFormat> (kSampleRate, kDuration, 2, 440.0f);
        auto sin880 = tracktion::graph::test_utilities::getSinFile<juce::WavAudioFormat> (kSampleRate, kDuration, 2, 880.0f);

        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (3);
        auto* t0 = getAudioTracks (*edit)[0];
        auto* t1 = getAudioTracks (*edit)[1];
        auto* fx = getAudioTracks (*edit)[2];

        t0->insertWaveClip ({}, sin440->getFile(), ClipPosition { { {}, TimeDuration::fromSeconds (kDuration) } }, false);
        t1->insertWaveClip ({}, sin880->getFile(), ClipPosition { { {}, TimeDuration::fromSeconds (kDuration) } }, false);

        t0->setName ("targetA");
        t1->setName ("targetB");
        fx->setName ("fx");

        // 目标轨输出电平 -12dB（避免叠加后满幅）；对照腿保持一致以定标
        t0->getVolumePlugin()->setVolumeDb (-12.0f);
        t1->getVolumePlugin()->setVolumeDb (-12.0f);

        if (wireAux)
        {
            auto& um = edit->getUndoManager();

            // T0：正常链序=干发送在前、湿回授在后（防环链序）；乱序腿反之
            if (t0LoopOrder)
            {
                auto r0 = edit->getPluginCache().createNewPlugin (AuxReturnPlugin::xmlTypeName, {});
                r0->state.setProperty ("busNum", 1, &um);
                t0->pluginList.insertPlugin (r0, 0, nullptr);

                auto s0 = edit->getPluginCache().createNewPlugin (AuxSendPlugin::xmlTypeName, {});
                s0->state.setProperty ("busNum", 0, &um);
                dynamic_cast<AuxSendPlugin*> (s0.get())->setGainDb (0.0f);
                t0->pluginList.insertPlugin (s0, 1, nullptr);
            }
            else
            {
                auto s0 = edit->getPluginCache().createNewPlugin (AuxSendPlugin::xmlTypeName, {});
                s0->state.setProperty ("busNum", 0, &um);
                dynamic_cast<AuxSendPlugin*> (s0.get())->setGainDb (0.0f);
                t0->pluginList.insertPlugin (s0, 0, nullptr);

                if (t0Return)
                {
                    auto r0 = edit->getPluginCache().createNewPlugin (AuxReturnPlugin::xmlTypeName, {});
                    r0->state.setProperty ("busNum", 1, &um);
                    t0->pluginList.insertPlugin (r0, 1, nullptr);
                }
            }

            // T1：正常链序（Send@0，可选 Return@1）
            {
                auto s1 = edit->getPluginCache().createNewPlugin (AuxSendPlugin::xmlTypeName, {});
                s1->state.setProperty ("busNum", 0, &um);
                dynamic_cast<AuxSendPlugin*> (s1.get())->setGainDb (0.0f);
                t1->pluginList.insertPlugin (s1, 0, nullptr);

                if (t1Return)
                {
                    auto r1 = edit->getPluginCache().createNewPlugin (AuxReturnPlugin::xmlTypeName, {});
                    r1->state.setProperty ("busNum", 1, &um);
                    t1->pluginList.insertPlugin (r1, 1, nullptr);
                }
            }

            // FX：回授 bus0 -> 0.5x 发送到 bus1；自身主输出压到 -96dB
            {
                auto rf = edit->getPluginCache().createNewPlugin (AuxReturnPlugin::xmlTypeName, {});
                rf->state.setProperty ("busNum", 0, &um);
                fx->pluginList.insertPlugin (rf, 0, nullptr);

                auto sf = edit->getPluginCache().createNewPlugin (AuxSendPlugin::xmlTypeName, {});
                sf->state.setProperty ("busNum", 1, &um);
                dynamic_cast<AuxSendPlugin*> (sf.get())->setGainDb (-6.0206f); // 线性 0.5x
                fx->pluginList.insertPlugin (sf, 1, nullptr);
            }
            fx->getVolumePlugin()->setVolumeDb (-96.0f);
        }

        if (muteT0)
            t0->setMute (true);

        // 渲染（与引擎自测同路径：createNode -> TracktionNodePlayer -> TestProcess）
        tracktion::graph::PlayHead playHead;
        tracktion::graph::PlayHeadState playHeadState { playHead };
        ProcessState processState { playHeadState, edit->tempoSequence };

        auto node = benchmark_utilities::createNode (*edit, processState, ts.sampleRate, ts.blockSize);
        tracktion::graph::test_utilities::TestProcess<TracktionNodePlayer> testContext (
            std::make_unique<TracktionNodePlayer> (std::move (node), processState, ts.sampleRate, ts.blockSize,
                                                   getPoolCreatorFunction (ThreadPoolStrategy::hybrid)),
            ts, 2, kDuration, true);
        testContext.getNodePlayer().setNumThreads (0);
        testContext.setPlayHead (&playHeadState.playHead);
        playHeadState.playHead.playSyncedToRange ({});
        auto result = testContext.processAll();

        LegResult leg;
        leg.name = name;

        const auto& buffer = result->buffer;
        const int total = buffer.getNumSamples();
        const int start = (int) (0.5 * kSampleRate);
        const int count = (int) (2.0 * kSampleRate);
        if (total < start + count)
        {
            logLine (std::string ("RENDER_TOO_SHORT leg=") + name);
            ++gFailures;
            return leg;
        }

        const float* ch0 = buffer.getReadPointer (0);
        leg.finite = true;
        double peak = 0.0, sumSquares = 0.0;
        for (int i = start; i < start + count; ++i)
        {
            if (! std::isfinite (ch0[i]))
                leg.finite = false;
            peak = std::max (peak, std::abs ((double) ch0[i]));
            sumSquares += (double) ch0[i] * ch0[i];
        }
        leg.rms = std::sqrt (sumSquares / count);
        leg.amp440 = goertzelMagnitude (ch0 + start, count, kSampleRate, 440.0);
        leg.amp880 = goertzelMagnitude (ch0 + start, count, kSampleRate, 880.0);

        char legLine[512];
        std::snprintf (legLine, sizeof (legLine),
                       "LEG %-13s amp440=%.5f amp880=%.5f rms=%.5f peak=%.5f finite=%d",
                       name, leg.amp440, leg.amp880, leg.rms, peak, (int) leg.finite);
        logLine (legLine);
        return leg;
    };

    const double tol = 0.05; // 5% 容差（含推子曲线换算残差）

    auto checkRatio = [&] (const char* leg, const char* what, double measured, double reference, double expectedRatio)
    {
        const double actualRatio = reference > 1e-9 ? measured / reference : measured;
        const bool pass = std::abs (actualRatio - expectedRatio) <= tol * std::max (1.0, expectedRatio);
        char buf[256];
        std::snprintf (buf, sizeof (buf), "ASSERT %-13s %s ratio=%.4f (ref=%.5f) expected=%.2f -> %s",
                       leg, what, actualRatio, reference, expectedRatio, pass ? "PASS" : "FAIL");
        logLine (buf);
        if (! pass)
            ++gFailures;
    };

    auto checkNearZero = [&] (const char* leg, const char* what, double measured, double reference)
    {
        const bool pass = measured <= 0.02 * reference;
        char buf[256];
        std::snprintf (buf, sizeof (buf), "ASSERT %-13s %s amp=%.6f (<=2%% of R=%.5f) -> %s",
                       leg, what, measured, reference, pass ? "PASS" : "FAIL");
        logLine (buf);
        if (! pass)
            ++gFailures;
    };

    auto checkBounded = [&] (const char* leg, double amp, double reference, double maxRatio)
    {
        const bool pass = std::isfinite (amp) && amp <= maxRatio * reference;
        char buf[256];
        std::snprintf (buf, sizeof (buf), "ASSERT %-13s bounded amp=%.5f (<= %.1fxR=%.5f) -> %s",
                       leg, amp, maxRatio, maxRatio * reference, pass ? "PASS" : "FAIL");
        logLine (buf);
        if (! pass)
            ++gFailures;
    };

    // L1 对照（定标）
    auto l1 = renderLeg ("control", false, false, false, false, false);
    const double R440 = l1.amp440, R880 = l1.amp880;
    char calib[128];
    std::snprintf (calib, sizeof (calib), "CALIBRATION R440=%.5f R880=%.5f", R440, R880);
    logLine (calib);

    // L2 全接线：多目标回流（核心断言）
    auto l2 = renderLeg ("full", true, true, true, false, false);
    checkRatio ("full", "amp440", l2.amp440, R440, 2.0);
    checkRatio ("full", "amp880", l2.amp880, R880, 2.0);
    if (! l2.finite)
        failCheck ("full leg finite samples");

    // L3 单回授退化对照
    auto l3 = renderLeg ("single_return", true, true, false, false, false);
    checkRatio ("single_return", "amp440", l3.amp440, R440, 1.5);
    checkRatio ("single_return", "amp880", l3.amp880, R880, 1.5);

    // L4 静音 T0
    auto l4 = renderLeg ("mute_T0", true, true, true, true, false);
    checkNearZero ("mute_T0", "amp440", l4.amp440, R440);
    checkRatio ("mute_T0", "amp880", l4.amp880, R880, 1.5);

    // L5 T0 链序颠倒（return 在 send 前）：引擎防环=渲染有限且有界（精确幅度记录不作断言）
    auto l5 = renderLeg ("loop_order", true, false, true, false, true);
    if (! l5.finite)
        failCheck ("loop_order leg finite samples");
    checkBounded ("loop_order", l5.amp440, R440, 2.5);
    checkBounded ("loop_order", l5.amp880, R880, 2.5);

    char verdict[96];
    std::snprintf (verdict, sizeof (verdict), "VERDICT %s (failures=%d)", gFailures == 0 ? "PASS" : "FAIL", gFailures);
    logLine (verdict);
    return gFailures == 0 ? 0 : 1;
}
