// KERNEL-TIMESIG-1: project time-signature command face acceptance tests.
//
// Four groups:
//  A. command face — set_time_signature 2/4 happy path (reply + engine state
//     + music_context parity), fail-closed rejections (missing / non-int /
//     out-of-range fields leave the edit untouched), no-edit error, and the
//     auto-persist failure reply.
//  B. W01 contract string parse — the "N/D" wire form accepted by the
//     new_project creation payload (docs/LEGAME_DESIGN_CONTRACT_V0.md §4.6
//     workspace.timing.time_signature): valid meters parse, every malformed
//     variant is rejected with an error and untouched outputs.
//  C. persistence round-trip (AGENTS §11 hard requirement) — set 2/4, save to
//     a real file via EditFileOperations::saveAs, destroy the edit, reload
//     with loadEditFromFile, meter reads back 2/4; set-back to 4/4 round-trips
//     as well.
//  D. old-project compatibility (AGENTS §11) — a project file whose TIMESIG
//     node carries no numerator/denominator properties (the pre-timesig
//     legacy shape) loads as 4/4 through the engine's CachedValue defaults:
//     zero behavioural change for existing projects; the music_context
//     disclosure keys derive from the same values.

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>
#include <tracktion_engine/utilities/tracktion_TestUtilities.h>

#include "../Source/Service/TimeSignatureService.h"

#include <cstdio>
#include <memory>
#include <string>

namespace te = tracktion;

using namespace tracktion;
using namespace tracktion::engine;

namespace
{

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
    auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        return object->getProperty (key).toString();

    return {};
}

int replyInt (const juce::String& reply, const char* key)
{
    auto parsed = juce::JSON::parse (reply);

    if (auto* object = parsed.getDynamicObject())
        return (int) object->getProperty (key);

    return -1;
}

bool replyOk (const juce::String& reply)
{
    return replyString (reply, "status") == "ok";
}

// --- time-sig helpers ----------------------------------------------------------

int editNumerator (te::Edit& edit)
{
    auto* timeSig = edit.tempoSequence.getTimeSig (0);
    return timeSig != nullptr ? (int) timeSig->numerator.get() : -1;
}

int editDenominator (te::Edit& edit)
{
    auto* timeSig = edit.tempoSequence.getTimeSig (0);
    return timeSig != nullptr ? (int) timeSig->denominator.get() : -1;
}

juce::String editTimeSigString (te::Edit& edit)
{
    auto* timeSig = edit.tempoSequence.getTimeSig (0);
    return timeSig != nullptr ? timeSig->getStringTimeSig() : juce::String();
}

vit::TimeSignatureService makeService (te::Edit* edit)
{
    return vit::TimeSignatureService { [edit] { return edit; }, [] { return true; } };
}

// A fresh save target per round trip (never overwrite another run's artefacts).
juce::File tempProjectFile (const char* tag)
{
    static int counter = 0;
    ++counter;
    return juce::File::getSpecialLocation (juce::File::tempDirectory)
        .getChildFile ("VitTimeSigTests_" + juce::String (tag) + "_" + juce::String (counter) + ".vit");
}

// The TIMESIG ValueTree inside a saved file — used to shape the legacy fixture.
juce::ValueTree timeSigTreeInState (const juce::ValueTree& editState)
{
    auto tempoSequence = editState.getChildWithName (te::IDs::TEMPOSEQUENCE);
    return tempoSequence.getChildWithName (te::IDs::TIMESIG);
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
        auto service = makeService (edit.get());

        // A1 — happy path: 2/4 (the lesson meter, §13).
        auto p = payload();
        p->setProperty ("numerator", 2);
        p->setProperty ("denominator", 4);
        auto reply = service.handleSetTimeSignature (*p, {});
        VIT_CHECK (replyOk (reply));
        VIT_CHECK (replyString (reply, "time_signature") == "2/4");
        VIT_CHECK (replyInt (reply, "numerator") == 2);
        VIT_CHECK (replyInt (reply, "denominator") == 4);
        VIT_CHECK (editNumerator (*edit) == 2);
        VIT_CHECK (editDenominator (*edit) == 4);
        VIT_CHECK (editTimeSigString (*edit) == "2/4");

        // A2 — the markAsChanged half of the set_tempo precedent: the edit
        // must be flagged dirty so the persist funnel actually writes.
        VIT_CHECK (edit->hasChangedSinceSaved());

        // A3 — music_context parity: the disclosure keys read exactly the
        // engine values the command wrote (numerator / denominator /
        // "N/D" string form).
        auto* timeSig = edit->tempoSequence.getTimeSig (0);
        VIT_CHECK (timeSig != nullptr);
        VIT_CHECK (timeSig->getStringTimeSig() == "2/4");

        // A4 — set back to 4/4 (idempotent re-write of the default meter).
        auto p4 = payload();
        p4->setProperty ("numerator", 4);
        p4->setProperty ("denominator", 4);
        VIT_CHECK (replyOk (service.handleSetTimeSignature (*p4, {})));
        VIT_CHECK (editNumerator (*edit) == 4 && editDenominator (*edit) == 4);
    }

    // A5 — fail-closed rejections: every invalid payload is an error reply and
    // leaves the engine meter untouched (state pinned at 2/4 first).
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        auto service = makeService (edit.get());

        auto seed = payload();
        seed->setProperty ("numerator", 2);
        seed->setProperty ("denominator", 4);
        VIT_CHECK (replyOk (service.handleSetTimeSignature (*seed, {})));

        struct Case
        {
            const char* name;
            std::function<void (juce::DynamicObject&)> mutate;
        };

        // Helper lambdas below set one field to an invalid value while the
        // other stays valid, or remove a required field outright.
        const Case cases[] = {
            { "missing numerator", [] (juce::DynamicObject& o) { o.removeProperty ("numerator"); } },
            { "missing denominator", [] (juce::DynamicObject& o) { o.removeProperty ("denominator"); } },
            { "double numerator", [] (juce::DynamicObject& o) { o.setProperty ("numerator", 2.5); } },
            { "double denominator", [] (juce::DynamicObject& o) { o.setProperty ("denominator", 4.5); } },
            { "string numerator", [] (juce::DynamicObject& o) { o.setProperty ("numerator", "2"); } },
            { "bool denominator", [] (juce::DynamicObject& o) { o.setProperty ("denominator", true); } },
            { "zero numerator", [] (juce::DynamicObject& o) { o.setProperty ("numerator", 0); } },
            { "negative denominator", [] (juce::DynamicObject& o) { o.setProperty ("denominator", -4); } },
            { "numerator above 64", [] (juce::DynamicObject& o) { o.setProperty ("numerator", 65); } },
            { "denominator above 64", [] (juce::DynamicObject& o) { o.setProperty ("denominator", 128); } },
        };

        for (const auto& c : cases)
        {
            auto p = payload();
            p->setProperty ("numerator", 3);
            p->setProperty ("denominator", 4);
            c.mutate (*p);
            const auto reply = service.handleSetTimeSignature (*p, {});
            VIT_CHECK (! replyOk (reply));
            VIT_CHECK (replyString (reply, "status") == "error");
            // State untouched: still the seeded 2/4.
            VIT_CHECK (editNumerator (*edit) == 2);
            VIT_CHECK (editDenominator (*edit) == 4);
            logLine (std::string ("A5 reject ok: ") + c.name);
        }
    }

    // A6 — no active edit.
    {
        auto service = makeService (nullptr);
        auto p = payload();
        p->setProperty ("numerator", 2);
        p->setProperty ("denominator", 4);
        const auto reply = service.handleSetTimeSignature (*p, {});
        VIT_CHECK (! replyOk (reply));
        VIT_CHECK (replyString (reply, "message") == "No active edit loaded");
    }

    // A7 — persist funnel failure surfaces as an error (set_tempo wording
    // parity), state itself was applied in memory.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        auto service = vit::TimeSignatureService { [edit = edit.get()] { return edit; },
                                                   [] { return false; } };
        auto p = payload();
        p->setProperty ("numerator", 6);
        p->setProperty ("denominator", 8);
        const auto reply = service.handleSetTimeSignature (*p, {});
        VIT_CHECK (! replyOk (reply));
        VIT_CHECK (replyString (reply, "message").contains ("failed to save project"));
        VIT_CHECK (editNumerator (*edit) == 6 && editDenominator (*edit) == 8);
    }

    logLine ("group A complete");
}

// =============================================================================
// Group B — W01 contract string parse (creation-time payload form)
// =============================================================================

static void runStringParseTests()
{
    logLine ("== group B: W01 string parse ==");

    struct ValidCase
    {
        const char* text;
        int numerator;
        int denominator;
    };

    const ValidCase valid[] = {
        { "2/4", 2, 4 },  // lesson meter (§13: 工程拍号 2/4)
        { "4/4", 4, 4 },
        { "3/4", 3, 4 },
        { "6/8", 6, 8 },
        { " 2/4 ", 2, 4 }, // outer whitespace tolerated (YAML edge)
    };

    for (const auto& c : valid)
    {
        int numerator = -1, denominator = -1;
        juce::String error;
        VIT_CHECK (vit::TimeSignatureService::parseTimeSignatureString (c.text, numerator, denominator, error));
        VIT_CHECK (numerator == c.numerator && denominator == c.denominator);
    }

    const char* invalid[] = {
        "",      "2",     "/4",     "2/",     "2/4/8",  "a/b",    "2.5/4",
        "-2/4",  "+2/4",  "0/4",    "2/0",    "2/65",   "65/4",   "2 / 4",
        "2/-4",  "2/x",   "99999/4", "12.0/8",
    };

    for (const auto* text : invalid)
    {
        int numerator = 111, denominator = 111;
        juce::String error;
        const bool ok = vit::TimeSignatureService::parseTimeSignatureString (text, numerator, denominator, error);
        VIT_CHECK (! ok);
        VIT_CHECK (error.isNotEmpty());
        // Outputs untouched on rejection.
        VIT_CHECK (numerator == 111 && denominator == 111);
        logLine (std::string ("B reject ok: '") + text + "'");
    }

    logLine ("group B complete");
}

// =============================================================================
// Group C — persistence round-trip (set 2/4 → save → reopen → read back 2/4)
// =============================================================================

static void runRoundTripTests (te::Engine& engine)
{
    logLine ("== group C: persistence round-trip ==");

    // C1 — set 2/4, save to a real file, destroy, reload: meter reads 2/4.
    {
        auto file = tempProjectFile ("roundtrip24");
        juce::ValueTree savedState;

        {
            auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
            auto service = makeService (edit.get());
            auto p = payload();
            p->setProperty ("numerator", 2);
            p->setProperty ("denominator", 4);
            VIT_CHECK (replyOk (service.handleSetTimeSignature (*p, {})));
            VIT_CHECK (te::EditFileOperations (*edit).saveAs (file, true));
            savedState = edit->state.createCopy();
        }

        // The saved XML carries the meter as engine-native TIMESIG properties.
        VIT_CHECK (timeSigTreeInState (savedState).hasProperty (te::IDs::numerator));

        {
            auto reloaded = te::loadEditFromFile (engine, file);
            VIT_CHECK (reloaded != nullptr);
            VIT_CHECK (editNumerator (*reloaded) == 2);
            VIT_CHECK (editDenominator (*reloaded) == 4);
            VIT_CHECK (editTimeSigString (*reloaded) == "2/4");
        }
        file.deleteFile();
    }

    // C2 — set back to 4/4 and round-trip again (re-defaulting persists too).
    {
        auto file = tempProjectFile ("roundtrip44");
        {
            auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
            auto service = makeService (edit.get());
            auto seed = payload();
            seed->setProperty ("numerator", 2);
            seed->setProperty ("denominator", 4);
            VIT_CHECK (replyOk (service.handleSetTimeSignature (*seed, {})));
            auto back = payload();
            back->setProperty ("numerator", 4);
            back->setProperty ("denominator", 4);
            VIT_CHECK (replyOk (service.handleSetTimeSignature (*back, {})));
            VIT_CHECK (te::EditFileOperations (*edit).saveAs (file, true));
        }
        {
            auto reloaded = te::loadEditFromFile (engine, file);
            VIT_CHECK (reloaded != nullptr);
            VIT_CHECK (editNumerator (*reloaded) == 4 && editDenominator (*reloaded) == 4);
        }
        file.deleteFile();
    }

    logLine ("group C complete");
}

// =============================================================================
// Group D — old-project compatibility (AGENTS §11)
// =============================================================================

static void runLegacyCompatTests (te::Engine& engine)
{
    logLine ("== group D: legacy compatibility ==");

    // D1 — a project saved before any non-4/4 meter: TIMESIG present with
    // explicit 4/4 (every existing Vit project shape) reloads as 4/4 and a
    // later 2/4 write still works on it.
    {
        auto file = tempProjectFile ("legacy44");
        {
            auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
            VIT_CHECK (editNumerator (*edit) == 4 && editDenominator (*edit) == 4);
            VIT_CHECK (te::EditFileOperations (*edit).saveAs (file, true));
        }
        {
            auto reloaded = te::loadEditFromFile (engine, file);
            VIT_CHECK (reloaded != nullptr);
            VIT_CHECK (editNumerator (*reloaded) == 4 && editDenominator (*reloaded) == 4);
            // Old project + new command interop: set 2/4 on the legacy file.
            auto service = makeService (reloaded.get());
            auto p = payload();
            p->setProperty ("numerator", 2);
            p->setProperty ("denominator", 4);
            VIT_CHECK (replyOk (service.handleSetTimeSignature (*p, {})));
            VIT_CHECK (editNumerator (*reloaded) == 2 && editDenominator (*reloaded) == 4);
            // Save-as round trip on the same legacy file.
            VIT_CHECK (te::EditFileOperations (*reloaded).saveAs (file, true));
        }
        {
            auto reopened = te::loadEditFromFile (engine, file);
            VIT_CHECK (reopened != nullptr);
            VIT_CHECK (editNumerator (*reopened) == 2 && editDenominator (*reopened) == 4);
        }
        file.deleteFile();
    }

    // D2 — the strictly-older shape: TIMESIG node with numerator/denominator
    // properties absent entirely. The engine's CachedValue defaults (4/4) own
    // the semantics — zero change, no fabricated properties, no migration.
    {
        auto file = tempProjectFile ("legacybare");

        {
            auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
            // Strip the meter properties to simulate the pre-timesig shape.
            auto timeSig = edit->tempoSequence.getTimeSig (0);
            VIT_CHECK (timeSig != nullptr);
            timeSig->state.removeProperty (te::IDs::numerator, nullptr);
            timeSig->state.removeProperty (te::IDs::denominator, nullptr);
            VIT_CHECK (! timeSig->state.hasProperty (te::IDs::numerator));
            VIT_CHECK (te::EditFileOperations (*edit).saveAs (file, true));
        }

        // The saved file itself must not carry the removed properties.
        {
            auto parsedState = te::loadEditFromFile (engine, file); // loads via engine path
            VIT_CHECK (parsedState != nullptr);
            VIT_CHECK (! timeSigTreeInState (parsedState->state).hasProperty (te::IDs::numerator));

            // Loaded values resolve through the CachedValue defaults: 4/4.
            VIT_CHECK (editNumerator (*parsedState) == 4);
            VIT_CHECK (editDenominator (*parsedState) == 4);
            VIT_CHECK (editTimeSigString (*parsedState) == "4/4");
        }
        file.deleteFile();
    }

    // D3 — unknown persisted values: the command face stays fail-closed; an
    // engine state holding an out-of-range meter (hand-edited file) is not
    // laundered by the command — re-setting a valid meter is explicit.
    {
        auto edit = tracktion::engine::test_utilities::createTestEdit (engine);
        auto* timeSig = edit->tempoSequence.getTimeSig (0);
        VIT_CHECK (timeSig != nullptr);
        // Hand-corrupted persisted meter (outside the command domain).
        timeSig->state.setProperty (te::IDs::numerator, 99, nullptr);
        timeSig->state.setProperty (te::IDs::denominator, 4, nullptr);
        VIT_CHECK (editNumerator (*edit) == 99); // engine reads what the file says

        auto service = makeService (edit.get());
        auto bad = payload();
        bad->setProperty ("numerator", 100); // still rejected on the face
        bad->setProperty ("denominator", 4);
        VIT_CHECK (! replyOk (service.handleSetTimeSignature (*bad, {})));
        VIT_CHECK (editNumerator (*edit) == 99); // untouched by the rejection

        auto repair = payload();
        repair->setProperty ("numerator", 2);
        repair->setProperty ("denominator", 4);
        VIT_CHECK (replyOk (service.handleSetTimeSignature (*repair, {})));
        VIT_CHECK (editNumerator (*edit) == 2);
    }

    logLine ("group D complete");
}

// =============================================================================
// main
// =============================================================================

int main()
{
    logLine ("VitTimeSignatureTests start");

    tracktion::engine::Engine engine { "VitTimeSignatureTests" };

    runCommandFaceTests (engine);
    runStringParseTests();
    runRoundTripTests (engine);
    runLegacyCompatTests (engine);

    char verdict[96];
    std::snprintf (verdict, sizeof (verdict), "VERDICT %s (failures=%d)", gFailures == 0 ? "PASS" : "FAIL", gFailures);
    logLine (verdict);
    return gFailures == 0 ? 0 : 1;
}
