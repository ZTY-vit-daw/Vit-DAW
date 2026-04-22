#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitGeneratedAssetRecord
{
    juce::String assetRef;
    juce::String sourceHash;
    juce::String relativePath;
    juce::String absolutePath;
    juce::String assetKind = "audio";
    juce::String assetState = "present";
    juce::String lifecycleState = "kept";
    juce::String bucket = "Generated";
    juce::String trackId;
    juce::String clipId;
    juce::String jobId;
    double originBpm = 0.0;
    double warpTargetBpm = 0.0;
    juce::String originKey;
    juce::String warpMode = "bypassed";
    juce::String warpState = "bypassed";
};

class VitMediaPoolManager final
{
public:
    struct ProjectFolders
    {
        juce::File projectFile;
        juce::File projectDirectory;
        juce::File mediaRoot;
        juce::File cacheDirectory;
        juce::File generatedDirectory;
    };

    static ProjectFolders resolveProjectFolders (const juce::File& projectFile);
    static juce::Result ensureProjectFolders (const juce::File& projectFile);
    static juce::Result ingestGeneratedAsset (const juce::File& projectFile,
                                              const juce::File& sourceFile,
                                              const juce::String& bucketHint,
                                              VitGeneratedAssetRecord& outRecord);
    static void registerClipReference (const juce::File& projectFile, const VitGeneratedAssetRecord& record);
    static void syncProjectAssetsFromEdit (const juce::File& projectFile, te::Edit& edit);
    static juce::Array<juce::var> snapshotProjectAssets (const juce::File& projectFile);
};

} // namespace vit
