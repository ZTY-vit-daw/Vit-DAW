#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

/** Provides real VST3 plugin editor windows (DocumentWindow + AudioProcessorEditor). */
class VitPluginUIBehaviour final : public tracktion::engine::UIBehaviour
{
public:
    std::unique_ptr<juce::Component> createPluginWindow (tracktion::engine::PluginWindowState& pws) override;
    void recreatePluginWindowContentAsync (tracktion::engine::Plugin& p) override;
};

} // namespace vit
