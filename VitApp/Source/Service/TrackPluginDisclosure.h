#pragma once

#include <JuceHeader.h>

#include <memory>

namespace vit
{

// M3-TRACK-PLUGIN-DISCLOSE-1 (2026-10-31): minimal track-level plugin
// disclosure for the compact project snapshot. Track-level built-ins such as
// Volume & Pan are not rack nodes, so the compact face used to carry only
// their volume_db/pan numbers plus the plugin_param_digest hash -- the
// plugin identity itself was invisible to observation consumers (card 222
// secondary surface: the model saw "track has no plugins" while the kernel
// truth had track-level Volume & Pan in place). Each released entry keeps
// the observation-minimal set (id/name/type); parameters, enabled flags and
// every other plugin field stay on the full serialisation face
// (createTrackState) so the compact payload cannot bloat. Read-only
// disclosure -- this is not an execution surface.

namespace TrackPluginDisclosureDetail
{
inline juce::String firstStringProperty (const juce::DynamicObject& object,
                                         std::initializer_list<const char*> names)
{
    for (const auto* name : names)
    {
        const auto value = object.getProperty (juce::Identifier (name)).toString().trim();
        if (value.isNotEmpty())
            return value;
    }

    return {};
}
} // namespace TrackPluginDisclosureDetail

// Compacts a full-face track "plugins" array into the minimal disclosure
// entries. Accepts both the createPluginState shape (id/plugin_item_id/name/
// type/enabled) and rack-node-flavoured shapes (plugin_id/plugin_name/
// plugin_type). A missing, null or non-array source compacts to an empty
// array: legacy track states keep a stable schema and never error (AGENTS
// S11 compat).
inline juce::Array<juce::var> compactTrackPluginDisclosure (const juce::var& pluginsVar)
{
    juce::Array<juce::var> entries;
    auto* source = pluginsVar.getArray();

    if (source == nullptr)
        return entries;

    for (const auto& entryVar : *source)
    {
        auto* entry = entryVar.getDynamicObject();

        if (entry == nullptr)
            continue;

        auto row = std::make_unique<juce::DynamicObject>();
        const auto id = TrackPluginDisclosureDetail::firstStringProperty (*entry, { "plugin_id", "plugin_item_id", "id" });

        if (id.isNotEmpty())
            row->setProperty ("id", id);

        const auto name = TrackPluginDisclosureDetail::firstStringProperty (*entry, { "plugin_name", "name" });

        if (name.isNotEmpty())
            row->setProperty ("name", name);

        const auto type = TrackPluginDisclosureDetail::firstStringProperty (*entry, { "plugin_type", "type" });

        if (type.isNotEmpty())
            row->setProperty ("type", type);

        if (row->getProperties().isEmpty())
            continue;

        entries.add (juce::var (row.release()));
    }

    return entries;
}

} // namespace vit
