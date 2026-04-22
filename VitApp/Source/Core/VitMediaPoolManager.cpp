#include "VitMediaPoolManager.h"

#include <unordered_map>

namespace vit
{

namespace
{

struct AssetStore
{
    std::unordered_map<std::string, VitGeneratedAssetRecord> assets;
};

juce::CriticalSection& getAssetLock()
{
    static juce::CriticalSection lock;
    return lock;
}

std::unordered_map<std::string, AssetStore>& getProjectStores()
{
    static std::unordered_map<std::string, AssetStore> stores;
    return stores;
}

std::string makeProjectKey (const juce::File& projectFile)
{
    return projectFile.getFullPathName().toStdString();
}

juce::String normaliseBucket (const juce::String& bucketHint)
{
    const auto bucket = bucketHint.trim().toLowerCase();

    if (bucket == "cache" || bucket == "aigc_cache")
        return "AIGC_Cache";

    return "Generated";
}

juce::String lifecycleForBucket (const juce::String& bucket)
{
    return bucket == "AIGC_Cache" ? "transient" : "kept";
}

juce::var assetToVar (const VitGeneratedAssetRecord& record)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("asset_ref", record.assetRef);
    object->setProperty ("hash", record.sourceHash);
    object->setProperty ("relative_path", record.relativePath);
    object->setProperty ("absolute_path", record.absolutePath);
    object->setProperty ("asset_kind", record.assetKind);
    object->setProperty ("asset_state", record.assetState);
    object->setProperty ("lifecycle_state", record.lifecycleState);
    object->setProperty ("bucket", record.bucket);
    object->setProperty ("track_id", record.trackId);
    object->setProperty ("clip_id", record.clipId);
    object->setProperty ("job_id", record.jobId);
    object->setProperty ("origin_bpm", record.originBpm);
    object->setProperty ("origin_key", record.originKey);
    object->setProperty ("warp_target_bpm", record.warpTargetBpm);
    object->setProperty ("warp_mode", record.warpMode);
    object->setProperty ("warp_state", record.warpState);
    return juce::var (object.release());
}

void registerAssetLocked (const juce::File& projectFile, const VitGeneratedAssetRecord& record)
{
    auto& store = getProjectStores()[makeProjectKey (projectFile)];
    store.assets[record.assetRef.toStdString()] = record;
}

} // namespace

VitMediaPoolManager::ProjectFolders VitMediaPoolManager::resolveProjectFolders (const juce::File& projectFile)
{
    ProjectFolders folders;
    folders.projectFile = projectFile;
    folders.projectDirectory = projectFile.getParentDirectory();

    auto baseName = projectFile.getFileNameWithoutExtension();
    if (baseName.isEmpty())
        baseName = "VitProject";

    folders.mediaRoot = folders.projectDirectory.getChildFile (baseName + "_Media");
    folders.cacheDirectory = folders.mediaRoot.getChildFile ("AIGC_Cache");
    folders.generatedDirectory = folders.mediaRoot.getChildFile ("Generated");
    return folders;
}

juce::Result VitMediaPoolManager::ensureProjectFolders (const juce::File& projectFile)
{
    const auto folders = resolveProjectFolders (projectFile);

    if (! folders.projectDirectory.exists() && ! folders.projectDirectory.createDirectory())
        return juce::Result::fail ("Failed to create project directory for media pool");

    if ((! folders.mediaRoot.exists() && ! folders.mediaRoot.createDirectory())
        || (! folders.cacheDirectory.exists() && ! folders.cacheDirectory.createDirectory())
        || (! folders.generatedDirectory.exists() && ! folders.generatedDirectory.createDirectory()))
        return juce::Result::fail ("Failed to create media pool directories");

    return juce::Result::ok();
}

juce::Result VitMediaPoolManager::ingestGeneratedAsset (const juce::File& projectFile,
                                                        const juce::File& sourceFile,
                                                        const juce::String& bucketHint,
                                                        VitGeneratedAssetRecord& outRecord)
{
    if (! sourceFile.existsAsFile())
        return juce::Result::fail ("Generated asset source file does not exist");

    if (const auto ensureResult = ensureProjectFolders (projectFile); ensureResult.failed())
        return ensureResult;

    const auto folders = resolveProjectFolders (projectFile);
    const auto bucket = normaliseBucket (bucketHint);
    const auto hash = juce::String::toHexString (static_cast<juce::int64> (sourceFile.hashCode64()));
    const auto extension = sourceFile.getFileExtension();
    const auto targetName = hash + extension;
    const auto destFile = (bucket == "AIGC_Cache" ? folders.cacheDirectory : folders.generatedDirectory).getChildFile (targetName);

    if (! destFile.existsAsFile() && ! sourceFile.copyFileTo (destFile))
        return juce::Result::fail ("Failed to copy generated asset into media pool");

    outRecord.assetRef = "asset:" + hash;
    outRecord.sourceHash = hash;
    outRecord.relativePath = destFile.getRelativePathFrom (folders.projectDirectory);
    outRecord.absolutePath = destFile.getFullPathName();
    outRecord.bucket = bucket;
    outRecord.lifecycleState = lifecycleForBucket (bucket);
    outRecord.assetState = destFile.existsAsFile() ? "present" : "missing";
    return juce::Result::ok();
}

void VitMediaPoolManager::registerClipReference (const juce::File& projectFile, const VitGeneratedAssetRecord& record)
{
    const juce::ScopedLock sl (getAssetLock());
    registerAssetLocked (projectFile, record);
}

void VitMediaPoolManager::syncProjectAssetsFromEdit (const juce::File& projectFile, te::Edit& edit)
{
    const auto folders = resolveProjectFolders (projectFile);
    const juce::ScopedLock sl (getAssetLock());

    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();

        for (int i = 0; i < n; ++i)
        {
            auto* item = track->getTrackItem (i);
            auto* clip = dynamic_cast<te::Clip*> (item);

            if (clip == nullptr)
                continue;

            const auto assetRef = clip->state.getProperty ("vit_asset_ref").toString().trim();

            if (assetRef.isEmpty())
                continue;

            VitGeneratedAssetRecord record;
            record.assetRef = assetRef;
            record.sourceHash = clip->state.getProperty ("vit_asset_hash").toString();
            record.relativePath = clip->state.getProperty ("vit_asset_relative_path").toString();
            record.assetKind = clip->state.getProperty ("vit_asset_kind").toString().isNotEmpty()
                                   ? clip->state.getProperty ("vit_asset_kind").toString()
                                   : "audio";
            record.bucket = clip->state.getProperty ("vit_asset_bucket").toString().isNotEmpty()
                                ? clip->state.getProperty ("vit_asset_bucket").toString()
                                : "Generated";
            record.lifecycleState = clip->state.getProperty ("vit_asset_lifecycle_state").toString().isNotEmpty()
                                        ? clip->state.getProperty ("vit_asset_lifecycle_state").toString()
                                        : lifecycleForBucket (record.bucket);
            record.trackId = track->itemID.toString();
            record.clipId = clip->itemID.toString();
            record.jobId = clip->state.getProperty ("vit_asset_job_id").toString();
            record.originBpm = static_cast<double> (clip->state.getProperty ("vit_origin_bpm"));
            record.originKey = clip->state.getProperty ("vit_origin_key").toString();
            record.warpTargetBpm = static_cast<double> (clip->state.getProperty ("vit_warp_target_bpm"));
            record.warpMode = clip->state.getProperty ("vit_warp_mode").toString();
            record.warpState = clip->state.getProperty ("vit_warp_state").toString();

            juce::File absolutePath;

            if (record.relativePath.isNotEmpty())
                absolutePath = folders.projectDirectory.getChildFile (record.relativePath);

            if ((! absolutePath.existsAsFile()) && dynamic_cast<te::AudioClipBase*> (clip) != nullptr)
                if (auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip))
                    absolutePath = audioClip->getCurrentSourceFile();

            record.absolutePath = absolutePath.getFullPathName();
            record.assetState = absolutePath.existsAsFile() ? "present" : "missing";
            registerAssetLocked (projectFile, record);
        }
    }
}

juce::Array<juce::var> VitMediaPoolManager::snapshotProjectAssets (const juce::File& projectFile)
{
    juce::Array<juce::var> assets;
    const juce::ScopedLock sl (getAssetLock());

    if (const auto it = getProjectStores().find (makeProjectKey (projectFile)); it != getProjectStores().end())
        for (const auto& [assetRef, record] : it->second.assets)
        {
            juce::ignoreUnused (assetRef);
            assets.add (assetToVar (record));
        }

    return assets;
}

} // namespace vit
