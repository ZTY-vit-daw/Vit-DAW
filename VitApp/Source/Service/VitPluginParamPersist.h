#pragma once

// RACK-PARAM-PERSIST-FIX-1: explicit-parameter override channel for plugin
// state persistence.
//
// Background (card forensics + engine source read):
//  - Rack-internal plugins hang off the RackType's own plugin list and are
//    never reached by Track-level flush (Track::flushStateToValueTree only
//    cascades the track's own pluginList) nor by the Edit-level
//    getAllPlugins() walk (it visits the track tree). Both save entry points
//    in VitHeadlessService therefore serialize stale chunks for rack members.
//  - VST3 chunk absorption of host-side parameter writes is plugin-dependent:
//    the getStateInformation() blob may not contain values the host just set,
//    so even a flushed chunk can round-trip a stale parameter.
//  - The engine already defines an explicit-parameter override property on
//    the plugin state ("parameters", tracktion_Identifiers.h
//    DECLARE_ID(parameters)), written by AutomatableEditItem::
//    saveChangedParametersToState() and replayed by
//    restoreChangedParametersFromState(). Replay runs AFTER the chunk restore
//    when the instance is rebuilt (ExternalPlugin::
//    completePluginInstanceCreation: restorePluginStateFromValueTree() then
//    buildParameterList() -> restoreChangedParametersFromState()), so an
//    override persisted here wins over the chunk on reload.
//  - The engine only writes that channel for parameters whose current value
//    deviates from the explicit value (automation drift), which misses the
//    plain "host wrote a new value, plugin accepted it" case. The helpers
//    below persist the applied value explicitly in the same wire format.
//
// Wire format (must stay byte-compatible with the engine pair):
//   repeat { juce::OutputStream::writeString(paramID); juce::writeFloat(value); }
// Replay is positional sequential, last entry for an id wins.

#include <cstddef>
#include <utility>
#include <vector>

#include <juce_core/juce_core.h>
#include <juce_data_structures/juce_data_structures.h>

namespace vit
{

struct ExplicitParamOverrideEntry
{
    juce::String paramID;
    float value = 0.0f;
};

// Decodes an override stream in the engine wire format. Tolerates a torn
// tail (string without its float) by stopping at the last complete pair; a
// stream that starts mid-record is not a supported input because the format
// has no resync marker.
inline std::vector<ExplicitParamOverrideEntry> decodeExplicitParamOverrides (const void* data,
                                                                             std::size_t sizeInBytes)
{
    std::vector<ExplicitParamOverrideEntry> entries;

    if (data == nullptr || sizeInBytes == 0)
        return entries;

    const auto* bytes = static_cast<const char*> (data);
    juce::MemoryInputStream stream (bytes, static_cast<int> (sizeInBytes), false);

    while (! stream.isExhausted())
    {
        const auto id = stream.readString();

        if (stream.isExhausted())
            break; // torn tail: string without its float

        const auto value = stream.readFloat();

        if (id.isEmpty())
            continue;

        entries.push_back ({ id, value });
    }

    return entries;
}

// Encodes entries in the engine wire format. Empty input produces an empty
// block (callers remove the property instead when there is nothing to keep).
inline void encodeExplicitParamOverrides (const std::vector<ExplicitParamOverrideEntry>& entries,
                                          juce::MemoryBlock& destination)
{
    destination.reset();
    juce::MemoryOutputStream stream (destination, false);

    for (const auto& entry : entries)
    {
        stream.writeString (entry.paramID);
        stream.writeFloat (entry.value);
    }

    stream.flush();
}

// Reads the override table from a plugin state ValueTree, replaces the value
// for paramID (or appends it), drops duplicates for the same id keeping the
// incoming one, and writes the property back. The property name is the engine
// identifier "parameters" (tracktion_Identifiers.h DECLARE_ID(parameters)) --
// kept as a literal here so this header does not depend on engine headers.
inline void upsertExplicitParamOverride (juce::ValueTree& pluginState,
                                         const juce::String& paramID,
                                         float value)
{
    std::vector<ExplicitParamOverrideEntry> entries;

    if (auto* existing = pluginState.getProperty (juce::Identifier ("parameters")).getBinaryData())
        entries = decodeExplicitParamOverrides (existing->getData(), existing->getSize());

    juce::MemoryOutputStream stream;

    for (const auto& entry : entries)
    {
        if (entry.paramID == paramID)
            continue; // replaced by the incoming entry below

        stream.writeString (entry.paramID);
        stream.writeFloat (entry.value);
    }

    stream.writeString (paramID);
    stream.writeFloat (value);
    stream.flush();

    pluginState.setProperty (juce::Identifier ("parameters"),
                             stream.getMemoryBlock(),
                             nullptr);
}

} // namespace vit
