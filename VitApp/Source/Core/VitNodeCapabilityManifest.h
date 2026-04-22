#pragma once

#include <JuceHeader.h>

namespace vit
{

class VitNodeCapabilityManifest final
{
public:
    static juce::var createManifest (const juce::String& nodeType,
                                     const juce::String& executionDomain,
                                     const juce::StringArray& supportedZones,
                                     const juce::StringArray& acceptedAssetKinds,
                                     const juce::StringArray& capabilityTags,
                                     bool isAsync,
                                     bool supportsBrowserLogin,
                                     bool supportsDownloadIngest,
                                     bool supportsControlGraph,
                                     bool supportsParamGrabber);

    static juce::Array<juce::var> createBuiltInManifests();
};

} // namespace vit
