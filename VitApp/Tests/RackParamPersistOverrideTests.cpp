// RACK-PARAM-PERSIST-FIX-1: explicit-parameter override channel tests.
//
// The override table must stay byte-compatible with the engine pair
// (AutomatableEditItem::saveChangedParametersToState /
// restoreChangedParametersFromState, tracktion_AutomatableEditItem.cpp):
// repeat { writeString(paramID); writeFloat(explicitValue); }. These tests
// pin the wire format, the upsert (replace-or-append) semantics used by the
// set_plugin_param write path, and torn-tail tolerance of the decoder.
//
// No tracktion linkage: the engine side of the format is asserted by
// constructing reference streams with the same juce stream calls the engine
// makes, and by decoding them with the Vit-side codec under test.

#include "../Source/Service/VitPluginParamPersist.h"

#include <cstdio>
#include <cstdlib>
#include <vector>

#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    std::_Exit (134);
}

void runEngineReferenceStreamDecodes()
{
    // Reference stream written exactly the way the engine writes it.
    juce::MemoryOutputStream reference;
    reference.writeString (juce::String ("3"));
    reference.writeFloat (0.7f);
    reference.writeString (juce::String ("master volume"));
    reference.writeFloat (-6.0f);
    reference.flush();

    const auto entries = vit::decodeExplicitParamOverrides (reference.getData(),
                                                             reference.getDataSize());

    VIT_CHECK (entries.size() == 2);
    VIT_CHECK (entries[0].paramID == "3");
    VIT_CHECK (std::abs (entries[0].value - 0.7f) < 1.0e-6f);
    VIT_CHECK (entries[1].paramID == "master volume");
    VIT_CHECK (std::abs (entries[1].value - (-6.0f)) < 1.0e-6f);
}

void runEncodeDecodeRoundTrip()
{
    std::vector<vit::ExplicitParamOverrideEntry> entries;
    entries.push_back ({ "3", 0.7f });
    entries.push_back ({ "7", -18.0794563f });
    entries.push_back ({ "Band 1 Gain", 0.0f });

    juce::MemoryBlock block;
    vit::encodeExplicitParamOverrides (entries, block);

    const auto decoded = vit::decodeExplicitParamOverrides (block.getData(), block.getSize());

    VIT_CHECK (decoded.size() == entries.size());

    for (std::size_t i = 0; i < entries.size(); ++i)
    {
        VIT_CHECK (decoded[i].paramID == entries[i].paramID);
        VIT_CHECK (std::abs (decoded[i].value - entries[i].value) < 1.0e-6f);
    }
}

void runTornTailIsDropped()
{
    juce::MemoryOutputStream torn;
    torn.writeString (juce::String ("3"));
    torn.writeFloat (0.7f);
    torn.writeString (juce::String ("orphan")); // string with no float after it
    torn.flush();

    const auto entries = vit::decodeExplicitParamOverrides (torn.getData(), torn.getDataSize());

    VIT_CHECK (entries.size() == 1);
    VIT_CHECK (entries[0].paramID == "3");
    VIT_CHECK (std::abs (entries[0].value - 0.7f) < 1.0e-6f);
}

void runUpsertReplacesAppendsAndSurvivesValueTree()
{
    juce::ValueTree pluginState (juce::Identifier ("PLUGIN"));

    // First write creates the property in engine wire format.
    vit::upsertExplicitParamOverride (pluginState, "3", 0.7f);

    auto* stored = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData();
    VIT_CHECK (stored != nullptr);

    auto entries = vit::decodeExplicitParamOverrides (stored->getData(), stored->getSize());
    VIT_CHECK (entries.size() == 1);
    VIT_CHECK (entries[0].paramID == "3");
    VIT_CHECK (std::abs (entries[0].value - 0.7f) < 1.0e-6f);

    // Same id upsert replaces instead of duplicating.
    vit::upsertExplicitParamOverride (pluginState, "3", 0.5f);

    stored = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData();
    entries = vit::decodeExplicitParamOverrides (stored->getData(), stored->getSize());
    VIT_CHECK (entries.size() == 1);
    VIT_CHECK (std::abs (entries[0].value - 0.5f) < 1.0e-6f);

    // Different id appends and preserves existing entries.
    vit::upsertExplicitParamOverride (pluginState, "7", -18.0794563f);

    stored = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData();
    entries = vit::decodeExplicitParamOverrides (stored->getData(), stored->getSize());
    VIT_CHECK (entries.size() == 2);
    VIT_CHECK (entries[0].paramID == "3");
    VIT_CHECK (entries[1].paramID == "7");
    VIT_CHECK (std::abs (entries[1].value - (-18.0794563f)) < 1.0e-4f);

    // The engine replay (MemoryInputStream readString/readFloat loop) parses
    // the stored block positionally -- pinned here so a juce stream change
    // that breaks the pair surfaces in this suite.
    auto* block = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData();
    juce::MemoryInputStream replay (*block, false);
    const auto firstId = replay.readString();
    const auto firstValue = replay.readFloat();
    const auto secondId = replay.readString();
    const auto secondValue = replay.readFloat();
    VIT_CHECK (firstId == "3");
    VIT_CHECK (std::abs (firstValue - 0.5f) < 1.0e-6f);
    VIT_CHECK (secondId == "7");
    VIT_CHECK (std::abs (secondValue - (-18.0794563f)) < 1.0e-4f);
    VIT_CHECK (replay.isExhausted());
}

void runUpsertFromEngineWrittenProperty()
{
    // Simulate a property previously written by the engine
    // (saveChangedParametersToState writes only deviated parameters).
    juce::ValueTree pluginState (juce::Identifier ("PLUGIN"));
    juce::MemoryOutputStream engineWritten;
    engineWritten.writeString (juce::String ("automation-written"));
    engineWritten.writeFloat (0.25f);
    engineWritten.flush();
    pluginState.setProperty (juce::Identifier ("parameters"),
                             engineWritten.getMemoryBlock(), nullptr);

    vit::upsertExplicitParamOverride (pluginState, "3", 0.7f);

    auto* stored = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData();
    const auto entries = vit::decodeExplicitParamOverrides (stored->getData(), stored->getSize());
    VIT_CHECK (entries.size() == 2);
    VIT_CHECK (entries[0].paramID == "automation-written");
    VIT_CHECK (std::abs (entries[0].value - 0.25f) < 1.0e-6f);
    VIT_CHECK (entries[1].paramID == "3");
    VIT_CHECK (std::abs (entries[1].value - 0.7f) < 1.0e-6f);
}

void runEmptyInputs()
{
    VIT_CHECK (vit::decodeExplicitParamOverrides (nullptr, 0).empty());
    VIT_CHECK (vit::decodeExplicitParamOverrides ("", 0).empty());

    juce::MemoryBlock emptyBlock;
    vit::encodeExplicitParamOverrides ({}, emptyBlock);
    VIT_CHECK (emptyBlock.getSize() == 0);
}

} // namespace

int main()
{
    runEngineReferenceStreamDecodes();
    runEncodeDecodeRoundTrip();
    runTornTailIsDropped();
    runUpsertReplacesAppendsAndSurvivesValueTree();
    runUpsertFromEngineWrittenProperty();
    runEmptyInputs();

    std::printf ("RackParamPersistOverrideTests: all checks passed\n");
    return 0;
}
