#include "GeneratedAssetService.h"

#include "ImportService.h"

#include "../Core/VitAIGCJobRuntime.h"
#include "../Core/VitAudioInjectorNode.h"
#include "../Core/VitAsyncGhostPolicy.h"
#include "../Core/VitBridgeNode.h"
#include "../Core/VitGraphSwapCoordinator.h"
#include "../Core/VitMediaPoolManager.h"
#include "../Core/VitPaths.h"
#include "../Core/VitTakeHistoryStack.h"
#include "../Core/VitWarpBridgeNode.h"

namespace vit
{

namespace
{

juce::File getEffectiveProjectFile (const GeneratedAssetService::CurrentProjectPathGetter& getter)
{
    if (getter != nullptr)
    {
        const auto currentPath = getter();
        if (currentPath.isNotEmpty())
            return juce::File (currentPath);
    }

    return paths::ensureDefaultProjectXmlFileExists();
}

te::Clip* findClipByID (te::Edit& edit, const juce::String& clipId)
{
    if (clipId.isEmpty())
        return nullptr;

    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();
        for (int i = 0; i < n; ++i)
        {
            auto* item = track->getTrackItem (i);
            auto* clip = dynamic_cast<te::Clip*> (item);
            if (clip != nullptr && clip->itemID.toString() == clipId)
                return clip;
        }
    }
    return nullptr;
}

te::AudioTrack* findAudioTrackByID (te::Edit& edit, const juce::String& trackID)
{
    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack != nullptr && audioTrack->itemID.toString() == trackID)
            return audioTrack;
    }

    return nullptr;
}

bool ensureMonitoringPlugins (te::AudioTrack& track)
{
    auto& edit = track.pluginList.getEdit();
    bool changed = false;

    if (track.getVolumePlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (track.getLevelMeterPlugin() == nullptr)
    {
        auto plugin = edit.getPluginCache().createNewPlugin (te::LevelMeterPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            track.pluginList.insertPlugin (plugin, -1, nullptr);
            changed = true;
        }
    }

    if (changed)
    {
        track.flushStateToValueTree();
        edit.getTransport().ensureContextAllocated (true);
        edit.dispatchPendingUpdatesSynchronously();
    }

    return changed;
}

void appendGraphRevisionProperties (juce::DynamicObject& object, const VitGraphRevisionSnapshot& snapshot)
{
    object.setProperty ("graph_revision", snapshot.revision);
    object.setProperty ("graph_active_revision", snapshot.activeRevision);
    object.setProperty ("graph_pending_revision", snapshot.pendingRevision);
    object.setProperty ("graph_last_retired_revision", snapshot.lastRetiredRevision);
    object.setProperty ("graph_retired_snapshot_count", snapshot.retiredSnapshotCount);
    object.setProperty ("graph_last_diff_kind", snapshot.lastKind);
    object.setProperty ("graph_last_diff_summary", snapshot.lastSummary);
    object.setProperty ("graph_publish_mode", snapshot.publishMode);
    object.setProperty ("graph_lifecycle_state", snapshot.lifecycleState);
    object.setProperty ("graph_recent_changes", juce::var (snapshot.recentChanges));
}

void appendGeneratedAssetPropertiesToClip (juce::ValueTree& clipState,
                                           const VitGeneratedAssetRecord& assetRecord,
                                           const VitWarpDescriptor& warpDescriptor,
                                           juce::UndoManager* undoManager)
{
    clipState.setProperty ("vit_asset_ref", assetRecord.assetRef, undoManager);
    clipState.setProperty ("vit_asset_hash", assetRecord.sourceHash, undoManager);
    clipState.setProperty ("vit_asset_relative_path", assetRecord.relativePath, undoManager);
    clipState.setProperty ("vit_asset_bucket", assetRecord.bucket, undoManager);
    clipState.setProperty ("vit_asset_kind", assetRecord.assetKind, undoManager);
    clipState.setProperty ("vit_asset_state", assetRecord.assetState, undoManager);
    clipState.setProperty ("vit_asset_lifecycle_state", assetRecord.lifecycleState, undoManager);
    clipState.setProperty ("vit_asset_job_id", assetRecord.jobId, undoManager);
    clipState.setProperty ("vit_node_role", VitAudioInjectorNode::defaultNodeRole(), undoManager);
    VitWarpBridgeNode::applyClipProperties (clipState, warpDescriptor, undoManager);
}

void appendTakePropertiesToClip (juce::ValueTree& clipState,
                                 const juce::String& takeStackId,
                                 const juce::String& activeTakeId,
                                 const juce::String& ghostState,
                                 juce::UndoManager* undoManager)
{
    clipState.setProperty ("vit_take_stack_id", takeStackId, undoManager);
    clipState.setProperty ("vit_active_take_id", activeTakeId, undoManager);
    clipState.setProperty ("vit_async_ghost_state", VitAsyncGhostPolicy::normalise (ghostState), undoManager);
}

} // namespace

GeneratedAssetService::GeneratedAssetService (EditGetter editGetter,
                                              SaveProjectAction saveProjectAction,
                                              CurrentProjectPathGetter currentProjectPathGetter,
                                              ImportService* importServicePtr)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction)),
      getCurrentProjectPath (std::move (currentProjectPathGetter)),
      importService (importServicePtr)
{
}

juce::String GeneratedAssetService::handleBridgeIngestGeneratedAsset (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto filePath = object.getProperty ("file_path").toString().trim();
    const auto trackId = object.getProperty ("track_id").toString().trim();
    const auto bucketHint = object.getProperty ("bucket").toString().trim();
    const auto jobId = object.getProperty ("job_id").toString().trim();
    const auto startTimeVar = object.getProperty ("start_time");

    if (filePath.isEmpty())
        return makeErrorReply ("bridge_ingest_generated_asset requires a non-empty file_path");

    if (trackId.isEmpty())
        return makeErrorReply ("bridge_ingest_generated_asset requires a non-empty track_id");

    if (! startTimeVar.isDouble() && ! startTimeVar.isInt() && ! startTimeVar.isInt64())
        return makeErrorReply ("bridge_ingest_generated_asset requires numeric start_time");

    auto* targetTrack = findAudioTrackByID (*edit, trackId);

    if (targetTrack == nullptr)
        return makeErrorReply ("Audio track not found for track_id: " + trackId);

    juce::File sourceFile (filePath);

    if (! sourceFile.existsAsFile())
        return makeErrorReply ("Generated asset source file does not exist: " + filePath);

    auto audioFile = te::AudioFile (edit->engine, sourceFile);

    if (! audioFile.isValid())
        return makeErrorReply ("Unsupported or unreadable generated audio file: " + filePath);

    const auto audioLengthSeconds = audioFile.getLength();

    if (audioLengthSeconds <= 0.0)
        return makeErrorReply ("Generated audio file has zero length: " + filePath);

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    VitGeneratedAssetRecord assetRecord;
    assetRecord.assetKind = object.getProperty ("asset_kind").toString().trim().isNotEmpty()
                                ? object.getProperty ("asset_kind").toString().trim()
                                : VitBridgeNode::defaultAssetKind();
    assetRecord.jobId = jobId;
    const auto nodeId = object.getProperty ("node_id").toString().trim();
    const auto requestedGhostState = VitAsyncGhostPolicy::normalise (object.getProperty ("ghost_state").toString());

    if (const auto ingestResult = VitMediaPoolManager::ingestGeneratedAsset (projectFile, sourceFile, bucketHint, assetRecord); ingestResult.failed())
        return makeErrorReply (ingestResult.getErrorMessage());

    ensureMonitoringPlugins (*targetTrack);
    const auto startTimeSeconds = juce::jmax (0.0, static_cast<double> (startTimeVar));
    if (importService == nullptr)
        return makeErrorReply ("Import service unavailable");

    const auto inserted = importService->insertWaveClipWithUndoAndStartBake (*edit,
                                                                             *targetTrack,
                                                                             juce::File (assetRecord.absolutePath),
                                                                             startTimeSeconds,
                                                                             audioLengthSeconds,
                                                                             false,
                                                                             "bridge_ingest");

    if (! inserted.ok)
        return makeErrorReply (inserted.errorMessage);

    auto* clip = findClipByID (*edit, inserted.clipId);
    if (clip == nullptr)
        return makeErrorReply ("Generated asset clip was inserted but could not be resolved by clip_id");

    assetRecord.trackId = inserted.trackItemId;
    assetRecord.clipId = inserted.clipId;

    const auto warpDescriptor = VitWarpBridgeNode::fromRequest (*edit, object);
    assetRecord.originBpm = warpDescriptor.originBpm;
    assetRecord.originKey = warpDescriptor.originKey;
    assetRecord.warpTargetBpm = warpDescriptor.targetBpm;
    assetRecord.warpMode = warpDescriptor.warpMode;
    assetRecord.warpState = warpDescriptor.warpState;

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Annotate generated asset");
    appendGeneratedAssetPropertiesToClip (clip->state, assetRecord, warpDescriptor, &undo);
    const auto takeStackId = VitTakeHistoryStack::resolveStackId (object, nodeId, inserted.clipId);
    const auto takeId = object.getProperty ("take_id").toString().trim().isNotEmpty()
                            ? object.getProperty ("take_id").toString().trim()
                            : "take:" + juce::Uuid().toString();
    appendTakePropertiesToClip (clip->state, takeStackId, takeId, requestedGhostState, &undo);

    if (auto* clipTrack = clip->getClipTrack())
        clipTrack->flushStateToValueTree();

    edit->dispatchPendingUpdatesSynchronously();
    edit->getTransport().ensureContextAllocated (true);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Generated asset imported but failed to save annotated project state");

    VitMediaPoolManager::registerClipReference (projectFile, assetRecord);

    VitTakeHistoryStack::addTake (projectFile,
                                  { takeStackId, takeId, inserted.clipId, inserted.trackItemId, nodeId },
                                  { takeId,
                                    assetRecord.assetRef,
                                    assetRecord.absolutePath,
                                    inserted.clipId,
                                    inserted.trackItemId,
                                    jobId,
                                    object.getProperty ("take_label").toString().trim(),
                                    warpDescriptor.warpState },
                                  true);

    if (jobId.isNotEmpty())
        VitAIGCJobRuntime::attachImportedAsset (projectFile, jobId, assetRecord.assetRef, assetRecord.trackId, assetRecord.clipId);

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Generated asset ingested");
    response->setProperty ("track_id", inserted.trackItemId);
    response->setProperty ("clip_id", inserted.clipId);
    response->setProperty ("clip_name", inserted.clipName);
    response->setProperty ("asset_ref", assetRecord.assetRef);
    response->setProperty ("asset_state", assetRecord.assetState);
    response->setProperty ("asset_bucket", assetRecord.bucket);
    response->setProperty ("asset_relative_path", assetRecord.relativePath);
    response->setProperty ("job_id", jobId);
    response->setProperty ("job_state", jobId.isNotEmpty() ? "ready" : "idle");
    response->setProperty ("take_stack_id", takeStackId);
    response->setProperty ("active_take_id", takeId);
    response->setProperty ("ghost_state", requestedGhostState);
    response->setProperty ("bridge_node_role", VitBridgeNode::defaultNodeRole());
    response->setProperty ("injector_node_role", VitAudioInjectorNode::defaultNodeRole());
    response->setProperty ("injector_zone_id", VitAudioInjectorNode::defaultZoneId());
    response->setProperty ("warp_mode", warpDescriptor.warpMode);
    response->setProperty ("warp_state", warpDescriptor.warpState);
    response->setProperty ("origin_bpm", warpDescriptor.originBpm);
    response->setProperty ("origin_key", warpDescriptor.originKey);
    response->setProperty ("warp_target_bpm", warpDescriptor.targetBpm);
    response->setProperty ("start_time", inserted.startTimeSeconds);
    response->setProperty ("clip_length_seconds", inserted.audioLengthSeconds);
    response->setProperty ("generated_assets", juce::var (VitMediaPoolManager::snapshotProjectAssets (projectFile)));
    response->setProperty ("jobs", juce::var (VitAIGCJobRuntime::snapshotProjectJobs (projectFile)));
    response->setProperty ("take_histories", juce::var (VitTakeHistoryStack::snapshotProjectStacks (projectFile)));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String GeneratedAssetService::handleSwitchAssetTake (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();
    const auto takeId = object.getProperty ("take_id").toString().trim();

    if (clipId.isEmpty() || takeId.isEmpty())
        return makeErrorReply ("switch_asset_take requires non-empty clip_id and take_id");

    auto* clip = findClipByID (*edit, clipId);
    auto* audioClip = dynamic_cast<te::AudioClipBase*> (clip);

    if (audioClip == nullptr)
        return makeErrorReply ("switch_asset_take clip not found or is not audio");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    const auto stackId = object.getProperty ("take_stack_id").toString().trim().isNotEmpty()
                             ? object.getProperty ("take_stack_id").toString().trim()
                             : clip->state.getProperty ("vit_take_stack_id").toString().trim();

    if (stackId.isEmpty())
        return makeErrorReply ("switch_asset_take could not resolve take stack id");

    VitTakeRecord take;
    if (! VitTakeHistoryStack::getTake (projectFile, stackId, takeId, take))
        return makeErrorReply ("switch_asset_take take not found in stack");

    juce::File takeFile (take.absolutePath);

    if (! takeFile.existsAsFile())
        return makeErrorReply ("switch_asset_take target take file is missing");

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Switch asset take");
    audioClip->getSourceFileReference().setToDirectFileReference (takeFile, false);
    audioClip->sourceMediaChanged();
    clip->state.setProperty ("vit_asset_ref", take.assetRef, &undo);
    clip->state.setProperty ("vit_active_take_id", take.takeId, &undo);
    clip->state.setProperty ("vit_asset_state", "present", &undo);
    clip->state.setProperty ("vit_warp_state", take.warpState, &undo);

    if (auto* clipTrack = clip->getClipTrack())
        clipTrack->flushStateToValueTree();

    edit->dispatchPendingUpdatesSynchronously();
    const auto trackId = clip->getClipTrack() != nullptr ? clip->getClipTrack()->itemID.toString() : juce::String();
    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "take_switch",
                                                                              "Switched active take for clip " + clipId + " to " + takeId,
                                                                              trackId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (! VitTakeHistoryStack::setActiveTake (projectFile, stackId, takeId))
        return makeErrorReply ("switch_asset_take failed to activate take in stack");

    if (saveProject && ! saveProject())
        return makeErrorReply ("Active take switched but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Active take switched");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("take_stack_id", stackId);
    response->setProperty ("active_take_id", takeId);
    response->setProperty ("asset_ref", take.assetRef);
    response->setProperty ("take_histories", juce::var (VitTakeHistoryStack::snapshotProjectStacks (projectFile)));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String GeneratedAssetService::handleSetAsyncGhostState (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto clipId = object.getProperty ("clip_id").toString().trim();
    const auto jobId = object.getProperty ("job_id").toString().trim();
    const auto ghostState = VitAsyncGhostPolicy::normalise (object.getProperty ("ghost_state").toString());

    if (clipId.isEmpty() && jobId.isEmpty())
        return makeErrorReply ("set_async_ghost_state requires clip_id or job_id");

    const auto projectFile = getEffectiveProjectFile (getCurrentProjectPath);
    juce::String trackId;

    if (clipId.isNotEmpty())
    {
        auto* clip = findClipByID (*edit, clipId);

        if (clip == nullptr)
            return makeErrorReply ("set_async_ghost_state clip not found");

        auto& undo = edit->getUndoManager();
        undo.beginNewTransaction ("Set async ghost state");
        clip->state.setProperty ("vit_async_ghost_state", ghostState, &undo);

        if (auto* clipTrack = clip->getClipTrack())
        {
            clipTrack->flushStateToValueTree();
            trackId = clipTrack->itemID.toString();
        }

        edit->dispatchPendingUpdatesSynchronously();
    }

    if (jobId.isNotEmpty())
        VitAIGCJobRuntime::setGhostState (projectFile, jobId, ghostState);

    const auto graphSnapshot = VitGraphSwapCoordinator::publishGraphChange (*edit,
                                                                            { "ghost_policy_change",
                                                                              "Set async ghost state to " + ghostState,
                                                                              trackId,
                                                                              {},
                                                                              {},
                                                                              {},
                                                                              {} });

    if (saveProject && ! saveProject())
        return makeErrorReply ("Ghost state updated but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Async ghost state updated");
    response->setProperty ("clip_id", clipId);
    response->setProperty ("job_id", jobId);
    response->setProperty ("ghost_state", ghostState);
    response->setProperty ("jobs", juce::var (VitAIGCJobRuntime::snapshotProjectJobs (projectFile)));
    appendGraphRevisionProperties (*response, graphSnapshot);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String GeneratedAssetService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String GeneratedAssetService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
