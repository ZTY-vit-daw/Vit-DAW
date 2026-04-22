#include "VitNodeCapabilityManifest.h"

namespace vit
{

namespace
{

juce::Array<juce::var> toVarArray (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return out;
}

} // namespace

juce::var VitNodeCapabilityManifest::createManifest (const juce::String& nodeType,
                                                     const juce::String& executionDomain,
                                                     const juce::StringArray& supportedZones,
                                                     const juce::StringArray& acceptedAssetKinds,
                                                     const juce::StringArray& capabilityTags,
                                                     bool isAsync,
                                                     bool supportsBrowserLogin,
                                                     bool supportsDownloadIngest,
                                                     bool supportsControlGraph,
                                                     bool supportsParamGrabber)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("node_type", nodeType);
    object->setProperty ("execution_domain", executionDomain);
    object->setProperty ("supported_zones", juce::var (toVarArray (supportedZones)));
    object->setProperty ("accepted_asset_kinds", juce::var (toVarArray (acceptedAssetKinds)));
    object->setProperty ("capability_tags", juce::var (toVarArray (capabilityTags)));
    object->setProperty ("is_async", isAsync);
    object->setProperty ("supports_browser_login", supportsBrowserLogin);
    object->setProperty ("supports_download_ingest", supportsDownloadIngest);
    object->setProperty ("supports_control_graph", supportsControlGraph);
    object->setProperty ("supports_param_grabber", supportsParamGrabber);
    return juce::var (object.release());
}

juce::Array<juce::var> VitNodeCapabilityManifest::createBuiltInManifests()
{
    juce::Array<juce::var> manifests;
    manifests.add (createManifest ("plugin_external",
                                   "realtime_audio",
                                   { "Z1", "Z2", "Z3" },
                                   { "audio", "midi" },
                                   { "track_rack", "param_grabber", "plugin_host" },
                                   false,
                                   false,
                                   false,
                                   true,
                                   true));
    manifests.add (createManifest ("bridge_node",
                                   "async_ai",
                                   { "Z2", "Z3" },
                                   { "audio", "midi", "stems" },
                                   { "async_job", "download_ingest", "browser_connector" },
                                   true,
                                   true,
                                   true,
                                   false,
                                   false));
    manifests.add (createManifest ("audio_injector",
                                   "bridge_injector",
                                   { "Z3" },
                                   { "audio", "stems" },
                                   { "asset_backed", "warp_target" },
                                   true,
                                   false,
                                   true,
                                   false,
                                   false));
    manifests.add (createManifest ("control_node",
                                   "control_domain",
                                   { "Top" },
                                   {},
                                   { "control_graph", "macro_source" },
                                   false,
                                   false,
                                   false,
                                   true,
                                   false));
    manifests.add (createManifest ("connector_profile",
                                   "web_api",
                                   { "Z2", "Z3" },
                                   { "audio", "midi", "stems", "lyrics" },
                                   { "browser_connector", "profile_backed" },
                                   true,
                                   true,
                                   true,
                                   false,
                                   false));
    return manifests;
}

} // namespace vit
