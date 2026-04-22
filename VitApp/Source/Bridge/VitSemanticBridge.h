#pragma once

#include <JuceHeader.h>

namespace vit
{

struct GhostTrackDescriptor
{
    juce::String trackName;
    juce::String intent;
};

class VitSemanticBridge
{
public:
    virtual ~VitSemanticBridge() = default;

    virtual void onEditReloaded (const juce::File&) {}
    virtual void onGhostTrackDetected (const GhostTrackDescriptor&) {}
};

} // namespace vit
