// M3-TRACK-PLUGIN-DISCLOSE-1 unit tests for the compact-face track-level
// plugin disclosure (TrackPluginDisclosure.h).
//
// T1 minimal-set compaction: full-face plugin entries (createPluginState
//    shape and rack-node flavour) compact to id/name/type only - enabled
//    flags and any parameter payload must not travel.
// T2 legacy schema stability (S11): a missing, null or non-array "plugins"
//    value compacts to an empty array without erroring, so old track states
//    keep a stable compact schema.
// T3 wire round trip: the disclosed entries survive a JSON serialise+parse
//    cycle with exactly the minimal keys, and a legacy snapshot without
//    plugins still parses to an empty array.

#include "../Source/Service/TrackPluginDisclosure.h"

#include <cstdio>
#include <cstdlib>
#include <string>

#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    std::_Exit (134);
}

juce::var parseJsonOrDie (const juce::String& text)
{
    const auto parsed = juce::JSON::parse (text);

    if (parsed.isVoid())
    {
        std::fprintf (stderr, "JSON round trip produced an unparseable payload: %s\n", text.toRawUTF8());
        std::_Exit (134);
    }

    return parsed;
}

void runMinimalSetCompactionTest()
{
    // Full-face shape (createPluginState): id/plugin_item_id/name/type/enabled.
    auto fullFace = std::make_unique<juce::DynamicObject>();
    fullFace->setProperty ("id", " 1055 ");
    fullFace->setProperty ("plugin_item_id", "1055");
    fullFace->setProperty ("name", "Volume & Pan");
    fullFace->setProperty ("type", "volume");
    fullFace->setProperty ("enabled", true);

    // Rack-node flavour: plugin_id/plugin_name/plugin_type plus a parameter
    // payload that must never reach the compact face.
    auto rackFlavour = std::make_unique<juce::DynamicObject>();
    rackFlavour->setProperty ("plugin_id", "p-7");
    rackFlavour->setProperty ("plugin_name", "Pro-Q 3");
    rackFlavour->setProperty ("plugin_type", "vst3");
    rackFlavour->setProperty ("params", juce::var (juce::Array<juce::var> { juce::var (1.25), juce::var (-3.0) }));
    rackFlavour->setProperty ("param_blob", juce::String::repeatedString ("x", 512));

    juce::Array<juce::var> source;
    source.add (juce::var (fullFace.release()));
    source.add (juce::var (rackFlavour.release()));

    const auto disclosed = vit::compactTrackPluginDisclosure (juce::var (source));
    VIT_CHECK (disclosed.size() == 2);

    const auto* first = disclosed[0].getDynamicObject();
    VIT_CHECK (first != nullptr);
    VIT_CHECK (first->getProperty ("id").toString() == "1055"); // trimmed
    VIT_CHECK (first->getProperty ("name").toString() == "Volume & Pan");
    VIT_CHECK (first->getProperty ("type").toString() == "volume");
    VIT_CHECK (! first->hasProperty ("enabled"));
    VIT_CHECK (! first->hasProperty ("plugin_item_id"));

    const auto* second = disclosed[1].getDynamicObject();
    VIT_CHECK (second != nullptr);
    VIT_CHECK (second->getProperty ("id").toString() == "p-7");
    VIT_CHECK (second->getProperty ("name").toString() == "Pro-Q 3");
    VIT_CHECK (second->getProperty ("type").toString() == "vst3");
    VIT_CHECK (! second->hasProperty ("params"));
    VIT_CHECK (! second->hasProperty ("param_blob"));

    // Serialised form carries exactly the minimal keys - no parameter payloads.
    const auto text = juce::JSON::toString (juce::var (disclosed), true);
    VIT_CHECK (! text.contains ("param_blob"));
    VIT_CHECK (! text.contains ("enabled"));
    VIT_CHECK (text.contains ("\"id\""));
    VIT_CHECK (text.contains ("\"name\""));
    VIT_CHECK (text.contains ("\"type\""));

    // Non-object entries are skipped without erroring.
    juce::Array<juce::var> withNoise;
    withNoise.add (juce::var (juce::String ("not-an-object")));
    withNoise.add (juce::var (42));
    const auto noiseDisclosed = vit::compactTrackPluginDisclosure (juce::var (withNoise));
    VIT_CHECK (noiseDisclosed.isEmpty());
}

void runLegacySchemaStabilityTest()
{
    // Missing property.
    juce::var missing;
    VIT_CHECK (vit::compactTrackPluginDisclosure (missing).isEmpty());

    // Explicit null.
    VIT_CHECK (vit::compactTrackPluginDisclosure (juce::var()).isEmpty());

    // Non-array legacy value (a string where an array would be): no error,
    // empty disclosure.
    VIT_CHECK (vit::compactTrackPluginDisclosure (juce::var (juce::String ("legacy"))).isEmpty());

    // Empty array stays empty.
    VIT_CHECK (vit::compactTrackPluginDisclosure (juce::var (juce::Array<juce::var>())).isEmpty());
}

void runWireRoundTripTest()
{
    // A compact track row exactly as normaliseTrackArray emits it for a
    // track with a Volume & Pan built-in.
    auto plugin = std::make_unique<juce::DynamicObject>();
    plugin->setProperty ("id", "2001");
    plugin->setProperty ("name", "Volume & Pan");
    plugin->setProperty ("type", "volume");

    auto track = std::make_unique<juce::DynamicObject>();
    track->setProperty ("track_id", "t-1");
    track->setProperty ("volume_db", -6.0);
    track->setProperty ("pan", 0.25);
    const auto disclosed = vit::compactTrackPluginDisclosure (juce::var (juce::Array<juce::var> { juce::var (plugin.release()) }));
    track->setProperty ("plugins", juce::var (disclosed));
    track->setProperty ("plugin_count", disclosed.size());

    const auto wire = juce::JSON::toString (juce::var (track.release()), true);
    const auto parsed = parseJsonOrDie (wire);
    auto* parsedTrack = parsed.getDynamicObject();
    VIT_CHECK (parsedTrack != nullptr);

    auto* parsedPlugins = parsedTrack->getProperty ("plugins").getArray();
    VIT_CHECK (parsedPlugins != nullptr);
    VIT_CHECK (parsedPlugins->size() == 1);
    VIT_CHECK (parsedTrack->getProperty ("plugin_count") == juce::var (1));

    auto* parsedPlugin = (*parsedPlugins)[0].getDynamicObject();
    VIT_CHECK (parsedPlugin != nullptr);
    VIT_CHECK (parsedPlugin->getProperty ("id").toString() == "2001");
    VIT_CHECK (parsedPlugin->getProperty ("name").toString() == "Volume & Pan");
    VIT_CHECK (parsedPlugin->getProperty ("type").toString() == "volume");
    VIT_CHECK (parsedPlugin->getProperties().size() == 3);

    // Legacy wire form (S11): a track row from before this disclosure
    // carries no "plugins" key at all. It must keep loading, and the
    // re-compacted disclosure is the empty default.
    const auto legacyParsed = parseJsonOrDie (R"({"track_id":"t-2","volume_db":-3.0,"pan":0.0})");
    auto* legacyTrack = legacyParsed.getDynamicObject();
    VIT_CHECK (legacyTrack != nullptr);
    VIT_CHECK (! legacyTrack->hasProperty ("plugins"));
    const auto legacyDisclosed = vit::compactTrackPluginDisclosure (legacyTrack->getProperty ("plugins"));
    VIT_CHECK (legacyDisclosed.isEmpty());
    const auto legacyWire = juce::JSON::toString (juce::var ([&] {
        auto row = std::make_unique<juce::DynamicObject>();
        row->setProperty ("track_id", legacyTrack->getProperty ("track_id"));
        row->setProperty ("plugins", juce::var (legacyDisclosed));
        row->setProperty ("plugin_count", legacyDisclosed.size());
        return row.release();
    }()), true);
    const auto legacyReloded = parseJsonOrDie (legacyWire);
    VIT_CHECK (legacyReloded.getDynamicObject() != nullptr);
    VIT_CHECK (legacyReloded.getDynamicObject()->getProperty ("plugin_count") == juce::var (0));
    auto* legacyPluginsArray = legacyReloded.getDynamicObject()->getProperty ("plugins").getArray();
    VIT_CHECK (legacyPluginsArray != nullptr && legacyPluginsArray->isEmpty());
}

} // namespace

int main()
{
    runMinimalSetCompactionTest();
    runLegacySchemaStabilityTest();
    runWireRoundTripTest();
    std::printf ("TrackPluginDisclosureTests: all checks passed\n");
    return 0;
}
