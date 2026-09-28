// REFSCHEMA-D1 red-first acceptance tests for the kernel-side L0 evidence-ref
// grammar mirror (VitApp/Source/Service/RefSchema.h).
//
// T1-T3  golden samples -- the builders must serialize exactly the strings
//        the agent-side FormatRef (agentprotocol/refschema.go) produces for
//        the same structured fields, including the ruling #2/#3 explicit
//        "t=all" window and "#-" un-CAS-ed hash.
// T4     escaping -- the grammar delimiters (% / @ # :) percent-escape with
//        uppercase hex inside segments; everything else (spaces, UTF-8)
//        passes through byte-for-byte.
// T5     sample-bounded window -- "t=<start>..<end>" (the D2 shape, locked
//        now so the suspended migration lands on a stable format).
// T6     agent-parse acceptance -- a structural mirror of the agent ParseRef
//        acceptance surface; every golden sample must be a well-formed
//        registered vit:// ref (consumer compatibility).
// T7     legacy lock -- the suspended D2 sites keep emitting the exact legacy
//        string the agent-side com consumers assert against.
// T8-T9  integration through the real L3 analyzer -- BandEnergySummary and
//        SegmentationPrimitives publications must carry vit:// refs from the
//        D3/D4/D5 generation points (was: "dad.l3.<feature>:<rev>" strings
//        and bare "dad.l3.noise_floor" constants).
//
// Red state: before the migration, no published payload contains a vit://
// evidence_ref, so the T8/T9 lookups fail; T1-T6 fail until RefSchema
// serializes the L0 grammar.

#include "../Source/Service/RefSchema.h"
#include "../Source/Service/L3AcousticAnalyzer.h"

#include <chrono>
#include <cmath>
#include <cstdio>
#include <future>
#include <memory>
#include <string>

namespace
{

constexpr double kSampleRate = 44100.0;
constexpr double kDurationSeconds = 2.0;
constexpr int kAnalysisWaitTimeoutSeconds = 30;

int gFailures = 0;

void check (bool condition, const std::string& description)
{
    if (condition)
    {
        std::printf ("[refschema] PASS %s\n", description.c_str());
        std::fflush (stdout);
        return;
    }
    ++gFailures;
    std::printf ("[refschema] FAIL %s\n", description.c_str());
    std::fflush (stdout);
}

void checkEqual (const juce::String& actual, const juce::String& expected,
                 const std::string& description)
{
    check (actual == expected,
           description + " (got \"" + actual.toStdString() + "\", want \""
               + expected.toStdString() + "\")");
}

class CapturingLogger final : public juce::Logger
{
public:
    void logMessage (const juce::String& message) override
    {
        (void) message;
    }
};

// Structural mirror of the agent-side ParseRef acceptance surface for a
// registered-kind vit:// ref (agentprotocol/refschema.go): scheme prefix,
// three '/'-separated path segments (kind / scope / window), a ':' inside
// the scope segment, an '@' snapshot segment and a trailing '#'-hash
// segment. Percent-escapes are opaque at this layer -- full byte-level
// verification is the golden-sample suite above.
bool acceptedByAgentGrammar (const juce::String& ref)
{
    const auto prefix = juce::String (vit::refschema::kRefSchemePrefix);
    if (! ref.startsWith (prefix))
        return false;
    const auto body = ref.substring (prefix.length());
    const auto at = body.indexOfChar ('@');
    if (at <= 0)
        return false;
    const auto tail = body.substring (at + 1);
    const auto hash = tail.indexOfChar ('#');
    if (hash < 0)
        return false;
    const auto head = body.substring (0, at);
    int slashes = 0;
    for (int i = 0; i < head.length(); ++i)
        if (head[i] == '/')
            ++slashes;
    if (slashes != 2)
        return false;
    if (! head.contains (":"))
        return false;
    return head.contains ("/t=all") || head.contains ("/t=");
}

juce::File testWorkspaceDirectory()
{
    const auto directory = juce::File::getSpecialLocation (juce::File::tempDirectory)
                               .getChildFile ("Vit_DAW_RefSchemaTests");
    if (directory.exists())
        directory.deleteRecursively();
    directory.createDirectory();
    return directory;
}

bool writeSineWav (const juce::File& file, int64 sampleCount)
{
    juce::AudioBuffer<float> buffer (2, (int) sampleCount);
    for (int i = 0; i < (int) sampleCount; ++i)
    {
        const double t = (double) i / kSampleRate;
        const auto value = (float) (0.2 * std::sin (juce::MathConstants<double>::twoPi * 220.0 * t));
        buffer.setSample (0, i, value);
        buffer.setSample (1, i, value);
    }
    juce::WavAudioFormat wavFormat;
    std::unique_ptr<juce::OutputStream> stream (file.createOutputStream());
    if (stream == nullptr)
        return false;
    std::unique_ptr<juce::AudioFormatWriter> writer (
        wavFormat.createWriterFor (stream.release(), kSampleRate, 2, 16, {}, 0));
    if (writer == nullptr)
        return false;
    writer->writeFromAudioSampleBuffer (buffer, 0, (int) sampleCount);
    return true;
}

juce::String analyzeAndWait (const juce::String& filePath, vit::AudioFeatureType featureType)
{
    vit::AudioFeatureBakeRequest request;
    request.projectId = "refschema-test-project";
    request.filePath = filePath;
    request.trackId = "refschema-test-track";
    request.sourceId = "refschema-test-source";
    request.sourceRevision = "refschema-test-rev-1";
    request.featureType = featureType;
    request.priority = vit::AudioFeaturePriority::OnDemand;

    auto promise = std::make_shared<std::promise<juce::String>>();
    auto future = promise->get_future();
    vit::L3AcousticAnalyzer::startAnalyze (request,
        [promise] (const juce::String& payload)
        {
            promise->set_value (payload);
        });
    if (future.wait_for (std::chrono::seconds (kAnalysisWaitTimeoutSeconds)) != std::future_status::ready)
        return {};
    return future.get();
}

juce::String firstEvidenceRef (const juce::var& block)
{
    if (auto* object = block.getDynamicObject())
    {
        const auto refs = object->getProperty ("evidence_refs");
        if (refs.isArray() && refs.getArray()->size() > 0)
            return refs.getArray()->getReference (0).toString();
    }
    return {};
}

} // namespace

int main()
{
    CapturingLogger capturingLogger;
    juce::Logger::setCurrentLogger (&capturingLogger);

    using namespace vit::refschema;

    // --- T1 D1 golden samples -------------------------------------------
    {
        const auto ref = makeL2RenderProbeRef ("1007", "render-1");
        checkEqual (ref, "vit://dad.l2_render_probe/track:1007/t=all@render-1#-",
                    "T1a D1 track-scoped probe ref");
        check (acceptedByAgentGrammar (ref), "T1a accepted by agent grammar");
    }
    {
        const auto ref = makeL2RenderProbeRef ({}, "render-2");
        checkEqual (ref, "vit://dad.l2_render_probe/project:current/t=all@render-2#-",
                    "T1b D1 master-tap fallback scope");
        check (acceptedByAgentGrammar (ref), "T1b accepted by agent grammar");
    }

    // --- T2 D3/D5 golden samples ----------------------------------------
    {
        const auto ref = makeL3FeatureRef ("band_energy_summary", "rev_1", {});
        checkEqual (ref, "vit://dad.l3/feature:band_energy_summary/t=all@rev_1#-",
                    "T2a D3 revision-snapshot feature ref");
        check (acceptedByAgentGrammar (ref), "T2a accepted by agent grammar");
    }
    {
        // D3 path fallback with a Windows path: ':' and '/' escape, the
        // space passes through -- same bytes the agent escapeSegment emits.
        const auto ref = makeL3FeatureRef ("spectral_field", {}, "D:/audio/track 1.wav");
        checkEqual (ref, "vit://dad.l3/feature:spectral_field/t=all@D%3A%2Faudio%2Ftrack 1.wav#-",
                    "T2b D3 file-path fallback escapes reserved bytes only");
        check (acceptedByAgentGrammar (ref), "T2b accepted by agent grammar");
    }
    {
        // Dots are not reserved; a '%' inside the snapshot must escape to
        // %25 so the agent-side unescape closes over exactly one byte.
        const auto ref = makeL3FeatureRef ("segmentation_primitives.onset_events", "rev%2", {});
        checkEqual (ref, "vit://dad.l3/feature:segmentation_primitives.onset_events/t=all@rev%252#-",
                    "T2c D5 dotted feature name passes, snapshot '%' escapes");
        check (acceptedByAgentGrammar (ref), "T2c accepted by agent grammar");
    }

    // --- T3 D4 golden samples --------------------------------------------
    {
        const auto ref = makeL3BandRef ("sub", "rev_1", {});
        checkEqual (ref, "vit://dad.l3/band:sub/t=all@rev_1#-",
                    "T3a D4 band row ref");
        check (acceptedByAgentGrammar (ref), "T3a accepted by agent grammar");
    }

    // --- T4 escaping unit --------------------------------------------------
    checkEqual (escapeRefSegment ("a%b/c@d#e:f"), "a%25b%2Fc%40d%23e%3Af",
                "T4a every reserved delimiter escapes with uppercase hex");
    checkEqual (escapeRefSegment (juce::String::fromUTF8 ("h\xc3\xa9llo")),
                juce::String::fromUTF8 ("h\xc3\xa9llo"),
                "T4b non-ASCII passes through byte-for-byte");
    checkEqual (escapeRefSegment ({}), {},
                "T4c empty segment stays empty");

    // --- T5 sample-bounded window ------------------------------------------
    {
        const auto ref = formatEvidenceRef (kKindCompressorDualTap, kScopeKindTrack, "trk_9",
                                            false, 0, 44100, "pair_2f3e");
        checkEqual (ref, "vit://dad.compressor_dual_tap/track:trk_9/t=0..44100@pair_2f3e#-",
                    "T5 sample-bounded window serialization (D2 shape)");
        check (acceptedByAgentGrammar (ref), "T5 accepted by agent grammar");
    }

    // --- T6 legacy lock for the suspended D2 sites ---------------------------
    checkEqual (juce::String (kLegacyPrefixCompressorDualTap) + "pair_7",
                "dad.compressor_dual_tap:pair_7",
                "T7 suspended D2 keeps the exact legacy string (consumer lock)");

    // --- T8/T9 integration through the real analyzer -------------------------
    const auto workspace = testWorkspaceDirectory();
    const auto sine = workspace.getChildFile ("refschema_sine.wav");
    const int64 sampleCount = (int64) (kDurationSeconds * kSampleRate);
    if (! writeSineWav (sine, sampleCount))
    {
        std::printf ("[refschema] FAIL could not write test wav under %s\n",
                     workspace.getFullPathName().toStdString().c_str());
        return 1;
    }

    {
        const auto payload = analyzeAndWait (sine.getFullPathName(),
                                             vit::AudioFeatureType::BandEnergySummary);
        check (payload.isNotEmpty(), "T8 BandEnergySummary publishes a payload");
        auto parsed = juce::JSON::parse (payload);
        auto* object = parsed.getDynamicObject();
        if (object != nullptr)
        {
            // D3: stampCommon's top-level ref (was "dad.l3.band_energy_summary:<rev|path>").
            checkEqual (object->getProperty ("evidence_ref").toString(),
                        "vit://dad.l3/feature:band_energy_summary/t=all@refschema-test-rev-1#-",
                        "T8a D3 top-level evidence_ref migrated");
            // D4: per-band rows (was "dad.l3.band_energy_summary:<bandName>").
            if (auto* bands = object->getProperty ("bands").getDynamicObject())
            {
                const auto bandRef = bands->getProperty ("sub")
                                         .getDynamicObject()->getProperty ("evidence_ref").toString();
                checkEqual (bandRef, "vit://dad.l3/band:sub/t=all@refschema-test-rev-1#-",
                            "T8b D4 band evidence_ref migrated");
            }
            else
            {
                check (false, "T8b bands block missing");
            }
            // D5: bare constants (was "dad.l3.noise_floor" etc.) now carry
            // the same source snapshot identity as the D3 top-level ref.
            checkEqual (firstEvidenceRef (object->getProperty ("noise_floor_evidence")),
                        "vit://dad.l3/feature:noise_floor/t=all@refschema-test-rev-1#-",
                        "T8c D5 noise_floor ref migrated");
            checkEqual (firstEvidenceRef (object->getProperty ("frequency_time_events")),
                        "vit://dad.l3/feature:frequency_time_events/t=all@refschema-test-rev-1#-",
                        "T8d D5 frequency_time_events ref migrated");
            checkEqual (firstEvidenceRef (object->getProperty ("transient_events")),
                        "vit://dad.l3/feature:transient_events/t=all@refschema-test-rev-1#-",
                        "T8e D5 transient_events ref migrated");
            if (auto* bandDynamics = object->getProperty ("band_dynamics").getDynamicObject())
            {
                const auto rows = bandDynamics->getProperty ("bands");
                if (rows.isArray() && rows.getArray()->size() > 0)
                {
                    checkEqual (firstEvidenceRef (rows.getArray()->getReference (0)),
                                "vit://dad.l3/feature:band_dynamics/t=all@refschema-test-rev-1#-",
                                "T8f D5 band_dynamics ref migrated");
                }
                else
                {
                    check (false, "T8f band_dynamics rows missing");
                }
            }
            else
            {
                check (false, "T8f band_dynamics block missing");
            }
        }
        else
        {
            check (false, "T8 payload is not a JSON object");
        }
    }

    {
        const auto payload = analyzeAndWait (sine.getFullPathName(),
                                             vit::AudioFeatureType::SegmentationPrimitives);
        check (payload.isNotEmpty(), "T9 SegmentationPrimitives publishes a payload");
        auto parsed = juce::JSON::parse (payload);
        if (auto* object = parsed.getDynamicObject())
        {
            checkEqual (firstEvidenceRef (object->getProperty ("onset_events")),
                        "vit://dad.l3/feature:segmentation_primitives.onset_events/t=all@refschema-test-rev-1#-",
                        "T9a D5 segmentation onset_events ref migrated");
            checkEqual (firstEvidenceRef (object->getProperty ("onset_density")),
                        "vit://dad.l3/feature:segmentation_primitives.onset_density/t=all@refschema-test-rev-1#-",
                        "T9b D5 segmentation onset_density ref migrated");
            checkEqual (firstEvidenceRef (object->getProperty ("energy_novelty")),
                        "vit://dad.l3/feature:segmentation_primitives.energy_novelty/t=all@refschema-test-rev-1#-",
                        "T9c D5 segmentation energy_novelty ref migrated");
        }
        else
        {
            check (false, "T9 payload is not a JSON object");
        }
    }

    if (gFailures > 0)
    {
        std::printf ("[refschema] %d check(s) FAILED\n", gFailures);
        return 1;
    }
    std::printf ("[refschema] all checks passed\n");
    return 0;
}
