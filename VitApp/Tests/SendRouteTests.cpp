// KERNEL-SENDROUTE-IMPL-1 (K1): SendRouteService acceptance tests.
//
// Three groups:
//  A. command face — auto_create single-transaction connect (undo-atomic /
//     idempotent resend / name-collision suffix), bus-pair allocation and
//     recycling, chain-order emission, list read-back, disconnect with
//     zero-source FX release, anti-loop gate rejections, set_fx_role,
//     sidechain (control-send) via the native Plugin sidechain face.
//  B. five audio legs inherited from the AuxMultiReturnProbe evidence
//     (MIX-SENDRETURN-DESIGN-1 experiment): topology is built through the
//     command face wherever it is reachable; the degenerate legs (single
//     return, muted source, reversed pair order) adjust live plugin state
//     directly because the command face makes them unreachable by design
//     (anti-loop gate 2).
//  C. fail-open / persistence boundary — tracks without vit_fx_role read as
//     ordinary tracks; saved state keeps the aux plugins engine-native
//     (PLUGIN type auxsend/auxreturn + busNum) so an older kernel meets its
//     existing unknown-plugin default semantics rather than VitApp schema.

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>
#include <tracktion_graph/tracktion_graph/tracktion_TestUtilities.h>
#include <tracktion_engine/utilities/tracktion_TestUtilities.h>
#include <tracktion_engine/playback/graph/tracktion_BenchmarkUtilities.h>

#include "../Source/Service/SendRouteService.h"

#include <cmath>
#include <cstdio>
#include <memory>
#include <string>

namespace te = tracktion;

using namespace tracktion;
using namespace tracktion::engine;

namespace
{

constexpr double kSampleRate = 44100.0;
constexpr int kBlockSize = 256;
constexpr double kDuration = 3.0;
constexpr double kPi = 3.14159265358979323846;
constexpr double kTolerance = 0.05; // 5%, matching the probe evidence
constexpr float kFxWetMarkerDb = -6.0206f; // linear 0.5x wet marker from the probe

int gFailures = 0;

void logLine (const std::string& text)
{
    std::fputs (text.c_str(), stdout);
    std::fputc ('\n', stdout);
    std::fflush (stdout);
}

void checkFailed (const char* what)
{
    logLine (std::string ("CHECK_FAILED: ") + what);
    ++gFailures;
}

#define VIT_CHECK(expression) \
    ((expression) ? void() : checkFailed (#expression))

// --- payload / reply helpers --------------------------------------------------

using Payload = std::unique_ptr<juce::DynamicObject>;

Payload payload()
{
    return std::make_unique<juce::DynamicObject>();
}

juce::String replyString (const juce::String& reply, const char* key)
{
    // The parsed var must outlive the object pointer it hands out.
    auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        return object->getProperty (key).toString();

    return {};
}

double replyDouble (const juce::String& reply, const char* key)
{
    auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        return (double) object->getProperty (key);

    return 0.0;
}

int replyArraySize (const juce::String& reply, const char* key)
{
    auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        if (auto* array = object->getProperty (key).getArray())
            return array->size();

    return -1;
}

bool replyOk (const juce::String& reply)
{
    return replyString (reply, "status") == "ok";
}

// --- edit/service fixture ------------------------------------------------------

te::AudioTrack* audioTrack (te::Edit& edit, int index)
{
    auto tracks = getAudioTracks (edit);
    return index < tracks.size() ? tracks[index] : nullptr;
}

te::AudioTrack* trackByName (te::Edit& edit, const juce::String& name)
{
    for (auto* track : getAllTracks (edit))
        if (track->getName() == name)
            return dynamic_cast<te::AudioTrack*> (track);

    return nullptr;
}

vit::SendRouteService makeService (te::Edit* edit)
{
    return vit::SendRouteService { [edit] { return edit; }, [] { return true; } };
}

// --- live-state inspection ------------------------------------------------------

te::AuxSendPlugin* auxSendWithBus (te::AudioTrack& track, int bus)
{
    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* send = dynamic_cast<te::AuxSendPlugin*> (plugin))
            if (send->getBusNumber() == bus)
                return send;

    return nullptr;
}

te::AuxReturnPlugin* auxReturnWithBus (te::AudioTrack& track, int bus)
{
    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* ret = dynamic_cast<te::AuxReturnPlugin*> (plugin))
            if ((int) ret->busNumber == bus)
                return ret;

    return nullptr;
}

int pluginIndex (te::AudioTrack& track, te::Plugin* plugin)
{
    if (plugin == nullptr)
        return -1;

    const auto plugins = track.pluginList.getPlugins();

    for (int i = 0; i < plugins.size(); ++i)
        if (plugins[i].get() == plugin)
            return i;

    return -1;
}

bool hasFxRole (te::AudioTrack& track)
{
    return track.state.getProperty ("vit_fx_role").toString() == "send_return_fx";
}

int fxSendBus (te::AudioTrack& track) { return (int) track.state.getProperty ("vit_send_bus", -1); }
int fxReturnBus (te::AudioTrack& track) { return (int) track.state.getProperty ("vit_return_bus", -1); }

// --- audio-plane rendering (probe lineage) ---------------------------------------

double goertzelMagnitude (const float* data, int numSamples, double sampleRate, double freq)
{
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
    double amp440 = 0.0;
    double amp880 = 0.0;
    bool finite = true;
};

LegResult renderEdit (te::Edit& edit)
{
    tracktion::graph::test_utilities::TestSetup ts;
    ts.sampleRate = kSampleRate;
    ts.blockSize = kBlockSize;

    tracktion::graph::PlayHead playHead;
    tracktion::graph::PlayHeadState playHeadState { playHead };
    ProcessState processState { playHeadState, edit.tempoSequence };

    auto node = benchmark_utilities::createNode (edit, processState, ts.sampleRate, ts.blockSize);
    tracktion::graph::test_utilities::TestProcess<TracktionNodePlayer> testContext (
        std::make_unique<TracktionNodePlayer> (std::move (node), processState, ts.sampleRate, ts.blockSize,
                                               getPoolCreatorFunction (ThreadPoolStrategy::hybrid)),
        ts, 2, kDuration, true);
    testContext.getNodePlayer().setNumThreads (0);
    testContext.setPlayHead (&playHeadState.playHead);
    playHeadState.playHead.playSyncedToRange ({});

    auto result = testContext.processAll();
    LegResult leg;

    const auto& buffer = result->buffer;
    const int total = buffer.getNumSamples();
    const int start = (int) (0.5 * kSampleRate);
    const int count = (int) (2.0 * kSampleRate);
    VIT_CHECK (total >= start + count);

    const float* ch0 = buffer.getReadPointer (0);

    for (int i = start; i < start + count; ++i)
        if (! std::isfinite (ch0[i]))
            leg.finite = false;

    leg.amp440 = goertzelMagnitude (ch0 + start, count, kSampleRate, 440.0);
    leg.amp880 = goertzelMagnitude (ch0 + start, count, kSampleRate, 880.0);
    return leg;
}

void checkRatio (const char* leg, const char* what, double measured, double reference, double expectedRatio)
{
    const double actualRatio = reference > 1e-9 ? measured / reference : measured;
    const bool pass = std::abs (actualRatio - expectedRatio) <= kTolerance * std::max (1.0, expectedRatio);
    char buf[256];
    std::snprintf (buf, sizeof (buf), "ASSERT %-13s %s ratio=%.4f (ref=%.5f) expected=%.2f -> %s",
                   leg, what, actualRatio, reference, expectedRatio, pass ? "PASS" : "FAIL");
    logLine (buf);

    if (! pass)
        ++gFailures;
}

void checkNearZero (const char* leg, const char* what, double measured, double reference)
{
    const bool pass = measured <= 0.02 * reference;
    char buf[256];
    std::snprintf (buf, sizeof (buf), "ASSERT %-13s %s amp=%.6f (<=2%% of R=%.5f) -> %s",
                   leg, what, measured, reference, pass ? "PASS" : "FAIL");
    logLine (buf);

    if (! pass)
        ++gFailures;
}

void checkBounded (const char* leg, const char* what, double amp, double reference, double maxRatio)
{
    const bool pass = std::isfinite (amp) && amp <= maxRatio * reference;
    char buf[256];
    std::snprintf (buf, sizeof (buf), "ASSERT %-13s %s bounded amp=%.5f (<= %.1fxR=%.5f) -> %s",
                   leg, what, amp, maxRatio, maxRatio * reference, pass ? "PASS" : "FAIL");
    logLine (buf);

    if (! pass)
        ++gFailures;
}

/** Two sine tracks (440/880 Hz) at -12dB, matching the probe calibration; the
 *  FX track always comes from the command face (auto_create). */
struct LegEdit
{
    std::unique_ptr<te::Edit> edit;
    te::AudioTrack* t0 = nullptr;
    te::AudioTrack* t1 = nullptr;
    // getSinFile returns a scoped TemporaryFile whose destructor deletes the
    // wav — the holders must outlive the render or the clip nodes starve.
    std::unique_ptr<juce::TemporaryFile> sin440;
    std::unique_ptr<juce::TemporaryFile> sin880;
};

LegEdit makeLegEdit (te::Engine& engine)
{
    LegEdit leg;
    leg.edit = tracktion::engine::test_utilities::createTestEdit (engine);
    leg.edit->ensureNumberOfAudioTracks (2);

    leg.sin440 = tracktion::graph::test_utilities::getSinFile<juce::WavAudioFormat> (kSampleRate, kDuration, 2, 440.0f);
    leg.sin880 = tracktion::graph::test_utilities::getSinFile<juce::WavAudioFormat> (kSampleRate, kDuration, 2, 880.0f);

    leg.t0 = getAudioTracks (*leg.edit)[0];
    leg.t1 = getAudioTracks (*leg.edit)[1];
    leg.t0->insertWaveClip ({}, leg.sin440->getFile(), ClipPosition { { {}, TimeDuration::fromSeconds (kDuration) } }, false);
    leg.t1->insertWaveClip ({}, leg.sin880->getFile(), ClipPosition { { {}, TimeDuration::fromSeconds (kDuration) } }, false);
    leg.t0->setName ("targetA");
    leg.t1->setName ("targetB");
    leg.t0->getVolumePlugin()->setVolumeDb (-12.0f);
    leg.t1->getVolumePlugin()->setVolumeDb (-12.0f);
    return leg;
}

/** Wire both targets to a command-created FX track and set the probe's 0.5x
 *  wet marker on the FX-side return send (not exposed on the command face by
 *  design — the FX-side send is fixed at unity). Returns the FX track. */
te::AudioTrack* wireFullLeg (LegEdit& leg, vit::SendRouteService& service)
{
    auto connectA = payload();
    connectA->setProperty ("source_track_id", leg.t0->itemID.toString());
    connectA->setProperty ("auto_create_fx_track", "FX");
    connectA->setProperty ("amount_db", 0.0);
    VIT_CHECK (replyOk (service.handleConnect (*connectA, {})));

    auto* fx = trackByName (*leg.edit, "FX");
    VIT_CHECK (fx != nullptr);

    auto connectB = payload();
    connectB->setProperty ("source_track_id", leg.t1->itemID.toString());
    connectB->setProperty ("fx_track_id", fx->itemID.toString());
    connectB->setProperty ("amount_db", 0.0);
    VIT_CHECK (replyOk (service.handleConnect (*connectB, {})));

    auto* wetSend = auxSendWithBus (*fx, fxReturnBus (*fx));
    VIT_CHECK (wetSend != nullptr);
    wetSend->setGainDb (kFxWetMarkerDb);
    return fx;
}

} // namespace

// =============================================================================
// Group A — command face
// =============================================================================

static void runCommandFaceTests (te::Engine& engine)
{
    logLine ("== group A: command face ==");

    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (3);
        auto* t0 = audioTrack (*edit, 0);
        auto* t1 = audioTrack (*edit, 1);
        auto* t2 = audioTrack (*edit, 2);
        auto service = makeService (edit.get());

        // A1 — auto_create one-step connect (single command transaction).
        auto p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Reverb Bus");
        p->setProperty ("amount_db", -6.0);
        auto reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "created_fx_track") == 1.0);
        VIT_CHECK (replyString (reply, "fx_name") == "Reverb Bus");
        VIT_CHECK (replyDouble (reply, "send_bus") == 0.0);
        VIT_CHECK (replyDouble (reply, "return_bus") == 1.0);
        VIT_CHECK (replyDouble (reply, "amount_db") == -6.0);

        auto* fx = trackByName (*edit, "Reverb Bus");
        VIT_CHECK (fx != nullptr);
        VIT_CHECK (fx != t0 && fx != t1 && fx != t2);
        VIT_CHECK (hasFxRole (*fx));
        VIT_CHECK (fxSendBus (*fx) == 0 && fxReturnBus (*fx) == 1);

        // FX-side chain: AuxReturn(send) at the head, SendOut(return) before
        // the volume tail, volume pulled to the -96dB master killer.
        VIT_CHECK (auxReturnWithBus (*fx, 0) != nullptr);
        VIT_CHECK (auxSendWithBus (*fx, 1) != nullptr);
        VIT_CHECK (pluginIndex (*fx, auxReturnWithBus (*fx, 0)) == 0);
        VIT_CHECK (pluginIndex (*fx, auxSendWithBus (*fx, 1)) < pluginIndex (*fx, fx->getVolumePlugin()));
        VIT_CHECK (std::abs (fx->getVolumePlugin()->getVolumeDb() - (-96.0f)) < 0.01f);

        // Source-side pair: ordered, adjacent, before the monitoring tail.
        auto* so0 = auxSendWithBus (*t0, 0);
        auto* si0 = auxReturnWithBus (*t0, 1);
        VIT_CHECK (so0 != nullptr && si0 != nullptr);
        VIT_CHECK (pluginIndex (*t0, si0) == pluginIndex (*t0, so0) + 1);
        VIT_CHECK (pluginIndex (*t0, si0) < pluginIndex (*t0, t0->getVolumePlugin()));
        VIT_CHECK (std::abs (so0->getGainDb() - (-6.0f)) < 0.01f);

        // Read-back: one send row, one fx row with source_count 1.
        VIT_CHECK (replyArraySize (reply, "sends") == 1);
        VIT_CHECK (replyArraySize (reply, "fx_tracks") == 1);
        VIT_CHECK (replyArraySize (reply, "dangling") == 0);

        auto listPayload = payload();
        auto listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyOk (listReply));
        VIT_CHECK (replyArraySize (listReply, "sends") == 1);

        // A2 — idempotent resend: no second track, no duplicate plugins, only
        // the amount follows the new payload.
        const auto pluginCountBefore = t0->pluginList.getPlugins().size();
        const auto fxPluginCountBefore = fx->pluginList.getPlugins().size();
        const auto trackCountBefore = getAllTracks (*edit).size();

        p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Reverb Bus");
        p->setProperty ("amount_db", -3.0);
        reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "created_fx_track") == 0.0);
        VIT_CHECK (replyString (reply, "fx_track_id") == fx->itemID.toString());
        VIT_CHECK (getAllTracks (*edit).size() == trackCountBefore);
        VIT_CHECK (t0->pluginList.getPlugins().size() == pluginCountBefore);
        VIT_CHECK (fx->pluginList.getPlugins().size() == fxPluginCountBefore);
        VIT_CHECK (std::abs (so0->getGainDb() - (-3.0f)) < 0.01f);

        // A3 — second source joins the same FX track by id.
        p = payload();
        p->setProperty ("source_track_id", t1->itemID.toString());
        p->setProperty ("fx_track_id", fx->itemID.toString());
        p->setProperty ("amount_db", -6.0);
        reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "created_fx_track") == 0.0);
        VIT_CHECK (auxSendWithBus (*t1, 0) != nullptr && auxReturnWithBus (*t1, 1) != nullptr);
        VIT_CHECK (pluginIndex (*t1, auxReturnWithBus (*t1, 1)) == pluginIndex (*t1, auxSendWithBus (*t1, 0)) + 1);
        VIT_CHECK (replyArraySize (reply, "sends") == 2);

        listPayload = payload();
        listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyArraySize (listReply, "sends") == 2);

        // A4 — set_amount is the source-side primary control (裁定 2).
        p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("fx_track_id", fx->itemID.toString());
        p->setProperty ("amount_db", -12.0);
        reply = service.handleSetAmount (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (std::abs (auxSendWithBus (*t0, 0)->getGainDb() - (-12.0f)) < 0.01f);

        // A5 — name collision on a non-FX track appends a sequence suffix.
        t2->setName ("Delay Bus");
        p = payload();
        p->setProperty ("source_track_id", t2->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Delay Bus");
        reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "created_fx_track") == 1.0);
        VIT_CHECK (replyString (reply, "fx_name") == "Delay Bus 2");
        VIT_CHECK (replyDouble (reply, "send_bus") == 2.0);
        VIT_CHECK (replyDouble (reply, "return_bus") == 3.0);

        // A6 — anti-loop gate 1 rejections.
        p = payload();
        p->setProperty ("source_track_id", fx->itemID.toString());
        p->setProperty ("fx_track_id", fx->itemID.toString());
        VIT_CHECK (! replyOk (service.handleConnect (*p, {}))); // FX track as source + self-loop

        p = payload();
        p->setProperty ("source_track_id", t1->itemID.toString());
        p->setProperty ("fx_track_id", t1->itemID.toString());
        VIT_CHECK (! replyOk (service.handleConnect (*p, {}))); // plain self-loop

        p = payload(); // t2 is a send source (edge to "Delay Bus 2") — cannot become FX
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("fx_track_id", t2->itemID.toString());
        VIT_CHECK (! replyOk (service.handleConnect (*p, {})));

        p = payload(); // no such edge
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("fx_track_id", t2->itemID.toString());
        p->setProperty ("amount_db", -1.0);
        VIT_CHECK (! replyOk (service.handleSetAmount (*p, {})));

        p = payload(); // unknown fx id
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("fx_track_id", "999999");
        VIT_CHECK (! replyOk (service.handleConnect (*p, {})));

        // A7 — disconnect one source: pair removed, FX survives.
        p = payload();
        p->setProperty ("source_track_id", t1->itemID.toString());
        p->setProperty ("fx_track_id", fx->itemID.toString());
        reply = service.handleDisconnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "fx_released") == 0.0);
        VIT_CHECK (auxSendWithBus (*t1, 0) == nullptr && auxReturnWithBus (*t1, 1) == nullptr);
        VIT_CHECK (hasFxRole (*fx));
        VIT_CHECK (auxReturnWithBus (*fx, 0) != nullptr && auxSendWithBus (*fx, 1) != nullptr);

        // A8 — last disconnect releases the FX side and restores the volume.
        p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("fx_track_id", fx->itemID.toString());
        reply = service.handleDisconnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "fx_released") == 1.0);
        VIT_CHECK (! hasFxRole (*fx));
        VIT_CHECK (fxSendBus (*fx) == -1 && fxReturnBus (*fx) == -1);
        VIT_CHECK (auxReturnWithBus (*fx, 0) == nullptr && auxSendWithBus (*fx, 1) == nullptr);
        VIT_CHECK (std::abs (fx->getVolumePlugin()->getVolumeDb()) < 0.01f);
        VIT_CHECK (auxSendWithBus (*t0, 0) == nullptr && auxReturnWithBus (*t0, 1) == nullptr);

        listPayload = payload();
        listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyArraySize (listReply, "sends") == 1); // t2 → Delay Bus 2 remains

        // A9 — the released pair is recycled: next FX allocation takes (0,1).
        p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Reverb Bus 2nd");
        reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "send_bus") == 0.0);
        VIT_CHECK (replyDouble (reply, "return_bus") == 1.0);
    }

    // A10 — auto_create undo atomicity: one undo reverts track + plugins + edge.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (1);
        auto* t0 = audioTrack (*edit, 0);
        auto service = makeService (edit.get());

        auto p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Chorus Bus");
        VIT_CHECK (replyOk (service.handleConnect (*p, {})));
        VIT_CHECK (trackByName (*edit, "Chorus Bus") != nullptr);

        VIT_CHECK (edit->getUndoManager().undo());

        VIT_CHECK (trackByName (*edit, "Chorus Bus") == nullptr);
        VIT_CHECK (auxSendWithBus (*t0, 0) == nullptr && auxReturnWithBus (*t0, 1) == nullptr);

        auto listPayload = payload();
        const auto listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyArraySize (listReply, "sends") == 0);
        VIT_CHECK (replyArraySize (listReply, "fx_tracks") == 0);
    }

    // A11 — set_fx_role upgrade/downgrade as the manual primitive.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (2);
        auto* t0 = audioTrack (*edit, 0);
        auto service = makeService (edit.get());

        auto p = payload();
        p->setProperty ("track_id", t0->itemID.toString());
        p->setProperty ("role", "send_return_fx");
        auto reply = service.handleSetFxRole (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (hasFxRole (*t0));
        VIT_CHECK (fxSendBus (*t0) == 0 && fxReturnBus (*t0) == 1);
        VIT_CHECK (auxReturnWithBus (*t0, 0) != nullptr && auxSendWithBus (*t0, 1) != nullptr);

        p = payload();
        p->setProperty ("track_id", t0->itemID.toString());
        p->setProperty ("role", "none");
        reply = service.handleSetFxRole (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (! hasFxRole (*t0));
        VIT_CHECK (std::abs (t0->getVolumePlugin()->getVolumeDb()) < 0.01f);
    }

    // A12 — fail-open read + dangling bus reservation.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (2);
        auto* t0 = audioTrack (*edit, 0);
        auto* t1 = audioTrack (*edit, 1);
        auto service = makeService (edit.get());

        // Old-project shape: plain tracks, no vit_* annotations.
        auto listPayload = payload();
        auto listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyOk (listReply));
        VIT_CHECK (replyArraySize (listReply, "sends") == 0);
        VIT_CHECK (replyArraySize (listReply, "fx_tracks") == 0);
        VIT_CHECK (replyArraySize (listReply, "dangling") == 0);

        // A hand-added aux send with no live FX track reads back as dangling,
        // and its bus number stays reserved so a new FX pair skips it.
        auto orphan = edit->getPluginCache().createNewPlugin (te::AuxSendPlugin::xmlTypeName, {});
        orphan->state.setProperty ("busNum", 0, nullptr);
        t0->pluginList.insertPlugin (orphan, 0, nullptr);

        listPayload = payload();
        listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyArraySize (listReply, "dangling") == 1);
        VIT_CHECK (replyArraySize (listReply, "sends") == 0);

        auto p = payload();
        p->setProperty ("source_track_id", t1->itemID.toString());
        p->setProperty ("auto_create_fx_track", "Safe Bus");
        const auto reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyDouble (reply, "send_bus") == 2.0); // pair (0,1) reserved by the orphan
        VIT_CHECK (replyDouble (reply, "return_bus") == 3.0);
    }

    // A13 — sidechain (control send) via the native Plugin face.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        edit->ensureNumberOfAudioTracks (2);
        auto* t0 = audioTrack (*edit, 0);
        auto* t1 = audioTrack (*edit, 1);
        auto service = makeService (edit.get());

        auto* targetPlugin = t1->getVolumePlugin();
        VIT_CHECK (targetPlugin != nullptr);

        auto target = payload();
        target->setProperty ("kind", "sidechain");
        target->setProperty ("track_id", t1->itemID.toString());
        target->setProperty ("plugin_item_id", targetPlugin->itemID.toString());

        auto p = payload();
        p->setProperty ("source_track_id", t0->itemID.toString());
        p->setProperty ("target", juce::var (target.release()));
        auto reply = service.handleConnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (targetPlugin->getSidechainSourceID() == t0->itemID);
        VIT_CHECK (replyArraySize (reply, "sidechain") == 1);

        auto listPayload = payload();
        const auto listReply = service.handleList (*listPayload, {});
        VIT_CHECK (replyArraySize (listReply, "sidechain") == 1);

        auto clearTarget = payload();
        clearTarget->setProperty ("kind", "sidechain");
        clearTarget->setProperty ("plugin_item_id", targetPlugin->itemID.toString());

        p = payload();
        p->setProperty ("target", juce::var (clearTarget.release()));
        reply = service.handleDisconnect (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (! targetPlugin->getSidechainSourceID().isValid());
    }

    logLine ("group A complete");
}

// =============================================================================
// Group B — five audio legs (AuxMultiReturnProbe lineage)
// =============================================================================

static void runAudioLegTests (te::Engine& engine)
{
    logLine ("== group B: five audio legs ==");

    // L1 control — calibration with no aux wiring.
    double R440 = 0.0, R880 = 0.0;
    {
        auto leg = makeLegEdit (engine);
        const auto l1 = renderEdit (*leg.edit);
        logLine ("LEG control amp440=" + std::to_string (l1.amp440) + " amp880=" + std::to_string (l1.amp880));
        R440 = l1.amp440;
        R880 = l1.amp880;
        checkRatio ("control", "amp440", R440, R440, 1.0);
        checkRatio ("control", "amp880", R880, R880, 1.0);
    }
    VIT_CHECK (R440 > 1e-3 && R880 > 1e-3);

    // L2 full — both targets wired through the command face: the 0.5x wet
    // marker injects into both target tracks → 2.0x at both frequencies.
    {
        auto leg = makeLegEdit (engine);
        auto service = makeService (leg.edit.get());
        wireFullLeg (leg, service);

        const auto l2 = renderEdit (*leg.edit);
        logLine ("LEG full amp440=" + std::to_string (l2.amp440) + " amp880=" + std::to_string (l2.amp880));
        checkRatio ("full", "amp440", l2.amp440, R440, 2.0);
        checkRatio ("full", "amp880", l2.amp880, R880, 2.0);
        VIT_CHECK (l2.finite);
    }

    // L3 single_return — degenerate topology (only targetA feeds wet back);
    // constructed by removing targetB's SendIn directly because the command
    // face always emits the pair.
    {
        auto leg = makeLegEdit (engine);
        auto service = makeService (leg.edit.get());
        auto* fx = wireFullLeg (leg, service);

        auto* sendIn = auxReturnWithBus (*leg.t1, fxReturnBus (*fx));
        VIT_CHECK (sendIn != nullptr);
        sendIn->removeFromParent();

        const auto l3 = renderEdit (*leg.edit);
        logLine ("LEG single_return amp440=" + std::to_string (l3.amp440) + " amp880=" + std::to_string (l3.amp880));
        checkRatio ("single_return", "amp440", l3.amp440, R440, 1.5);
        checkRatio ("single_return", "amp880", l3.amp880, R880, 1.5);
    }

    // L4 mute_T0 — the muted source's dry send is suppressed by the engine's
    // default mute-state behaviour; the other source's wet path survives.
    {
        auto leg = makeLegEdit (engine);
        auto service = makeService (leg.edit.get());
        wireFullLeg (leg, service);

        leg.t0->setMute (true);

        const auto l4 = renderEdit (*leg.edit);
        logLine ("LEG mute_T0 amp440=" + std::to_string (l4.amp440) + " amp880=" + std::to_string (l4.amp880));
        checkNearZero ("mute_T0", "amp440", l4.amp440, R440);
        checkRatio ("mute_T0", "amp880", l4.amp880, R880, 1.5);
    }

    // L5 loop_order — the reversed-pair config the command face cannot emit
    // (gate 2): engine graph culling must keep the render finite and bounded.
    {
        auto leg = makeLegEdit (engine);
        auto service = makeService (leg.edit.get());
        auto* fx = wireFullLeg (leg, service);

        auto* sendOut = auxSendWithBus (*leg.t0, fxSendBus (*fx));
        auto* sendIn = auxReturnWithBus (*leg.t0, fxReturnBus (*fx));
        VIT_CHECK (sendOut != nullptr && sendIn != nullptr);
        te::Plugin::Ptr sendInPtr (sendIn);
        sendIn->removeFromParent();
        leg.t0->pluginList.insertPlugin (sendInPtr, pluginIndex (*leg.t0, sendOut), nullptr);
        VIT_CHECK (pluginIndex (*leg.t0, sendIn) < pluginIndex (*leg.t0, sendOut));

        const auto l5 = renderEdit (*leg.edit);
        logLine ("LEG loop_order amp440=" + std::to_string (l5.amp440) + " amp880=" + std::to_string (l5.amp880));
        VIT_CHECK (l5.finite);
        checkBounded ("loop_order", "amp440", l5.amp440, R440, 2.5);
        checkBounded ("loop_order", "amp880", l5.amp880, R880, 2.5);
    }

    logLine ("group B complete");
}

// =============================================================================
// Group C — fail-open persistence boundary
// =============================================================================

static void runPersistenceBoundaryTests (te::Engine& engine)
{
    logLine ("== group C: persistence boundary ==");

    auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
    edit->ensureNumberOfAudioTracks (1);
    auto* t0 = audioTrack (*edit, 0);
    auto service = makeService (edit.get());

    auto p = payload();
    p->setProperty ("source_track_id", t0->itemID.toString());
    p->setProperty ("auto_create_fx_track", "Persist Bus");
    VIT_CHECK (replyOk (service.handleConnect (*p, {})));

    auto* fx = trackByName (*edit, "Persist Bus");
    VIT_CHECK (fx != nullptr);

    // Fail-open read side: a source track carries no FX annotation — old
    // readers that don't know the properties simply ignore them.
    VIT_CHECK (t0->state.getProperty ("vit_fx_role").isVoid());
    VIT_CHECK (fx->state.getProperty ("vit_fx_role").toString() == "send_return_fx");

    const auto stateXml = edit->state.toXmlString();

    // The aux pair persists as engine-native PLUGIN nodes (type + busNum) —
    // exactly what an older kernel sees; its existing unknown-plugin default
    // semantics apply. New projects are not expected to keep working on old
    // kernels (design §1.2 boundary — recorded, not worked around).
    VIT_CHECK (stateXml.contains ("auxsend"));
    VIT_CHECK (stateXml.contains ("auxreturn"));
    VIT_CHECK (stateXml.contains ("busNum"));
    VIT_CHECK (stateXml.contains ("vit_fx_role"));
    VIT_CHECK (stateXml.contains ("vit_send_bus"));
    VIT_CHECK (stateXml.contains ("vit_return_bus"));

    logLine ("group C complete");
}

// =============================================================================

int main()
{
    logLine ("VitSendRouteTests start");

    tracktion::engine::Engine engine { "VitSendRouteTests" };

    runCommandFaceTests (engine);
    runAudioLegTests (engine);
    runPersistenceBoundaryTests (engine);

    char verdict[96];
    std::snprintf (verdict, sizeof (verdict), "VERDICT %s (failures=%d)", gFailures == 0 ? "PASS" : "FAIL", gFailures);
    logLine (verdict);
    return gFailures == 0 ? 0 : 1;
}
