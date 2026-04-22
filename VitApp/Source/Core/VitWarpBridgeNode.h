#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitWarpDescriptor
{
    double originBpm = 0.0;
    juce::String originKey;
    double targetBpm = 0.0;
    juce::String warpMode = "bypassed";
    juce::String warpState = "bypassed";
};

class VitWarpBridgeNode final
{
public:
    static VitWarpDescriptor fromRequest (te::Edit&, const juce::DynamicObject&);
    static void applyClipProperties (juce::ValueTree&, const VitWarpDescriptor&, juce::UndoManager*);
};

} // namespace vit
