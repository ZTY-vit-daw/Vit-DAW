#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitBridgeNode final
{
public:
    static juce::String defaultNodeRole() { return "bridge"; }
    static juce::String defaultAssetKind() { return "audio"; }
};

} // namespace vit
