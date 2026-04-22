#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitConnectorPluginSpec final
{
public:
    static juce::Array<juce::var> createBuiltInSpecs();
    static juce::Array<juce::var> snapshotProfiles (const juce::File& projectFile);
    static juce::Result upsertProfile (const juce::File& projectFile,
                                       const juce::DynamicObject& object,
                                       juce::var& outProfile);
    static juce::Result removeProfile (const juce::File& projectFile,
                                       const juce::String& profileId,
                                       bool& removed);
};

} // namespace vit
