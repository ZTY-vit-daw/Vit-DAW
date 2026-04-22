#include "VitNodeRegistry.h"

#include "VitConnectorPluginSpec.h"
#include "VitNodeCapabilityManifest.h"

namespace vit
{

juce::var VitNodeRegistry::createSnapshot (const juce::File& projectFile)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("built_in_manifests", juce::var (VitNodeCapabilityManifest::createBuiltInManifests()));
    object->setProperty ("connector_specs", juce::var (VitConnectorPluginSpec::createBuiltInSpecs()));
    object->setProperty ("connector_profiles", juce::var (VitConnectorPluginSpec::snapshotProfiles (projectFile)));
    return juce::var (object.release());
}

} // namespace vit
