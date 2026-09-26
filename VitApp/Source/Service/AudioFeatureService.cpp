#include "AudioFeatureService.h"

#include "L3AcousticAnalyzer.h"
#include "TiledSpectrogramBaker.h"
#include "WaveformEnvelopeBaker.h"

#include <map>
#include <utility>

namespace vit
{
namespace
{
constexpr juce::int64 kAudioFeatureRequestMergeWindowMs = 30000;
juce::CriticalSection gRecentAudioFeatureRequestLock;

struct RecentAudioFeatureRequest
{
    juce::int64 lastMs = 0;
    int priorityRank = 100;
};

std::map<std::string, RecentAudioFeatureRequest> gRecentAudioFeatureRequests;

int audioFeaturePriorityRank (AudioFeaturePriority priority)
{
    switch (priority)
    {
        case AudioFeaturePriority::OnDemand:         return 0;
        case AudioFeaturePriority::ImportImmediate:  return 10;
        case AudioFeaturePriority::BackgroundWarm:   return 50;
    }

    return 100;
}

juce::String audioFeatureRequestMergeKey (const AudioFeatureBakeRequest& request)
{
    const auto featureType = audioFeatureTypeToString (request.featureType);
    auto subject = request.clipId.trim();
    if (subject.isEmpty())
        subject = request.sourceId.trim();
    if (subject.isEmpty())
        subject = request.filePath.trim();

    auto revision = request.clipRevision.trim();
    if (revision.isEmpty())
        revision = request.renderRevision.trim();
    if (revision.isEmpty())
        revision = request.sourceRevision.trim();
    if (revision.isEmpty())
        revision = request.filePath.trim();

    return "track=" + request.trackId.trim()
        + "|subject=" + subject
        + "|feature=" + featureType
        + "|offset=" + juce::String (request.range.sourceOffsetSeconds, 4)
        + "|length=" + juce::String (request.range.lengthSeconds, 4)
        + "|frame_width=" + juce::String (request.featureType == AudioFeatureType::WaveformEnvelope
                                           ? 0
                                           : request.resolution.frameWidth)
        + "|revision=" + revision;
}

void pruneRecentAudioFeatureRequests (juce::int64 nowMs)
{
    for (auto it = gRecentAudioFeatureRequests.begin(); it != gRecentAudioFeatureRequests.end();)
    {
        if (nowMs - it->second.lastMs > kAudioFeatureRequestMergeWindowMs)
            it = gRecentAudioFeatureRequests.erase (it);
        else
            ++it;
    }
}

bool shouldMergeRecentAudioFeatureRequest (const AudioFeatureBakeRequest& request)
{
    const auto key = audioFeatureRequestMergeKey (request);
    if (key.trim().isEmpty())
        return false;

    const auto nowMs = juce::Time::currentTimeMillis();
    const juce::ScopedLock lock (gRecentAudioFeatureRequestLock);
    pruneRecentAudioFeatureRequests (nowMs);

    const auto keyText = key.toStdString();
    const auto incomingPriorityRank = audioFeaturePriorityRank (request.priority);
    auto existing = gRecentAudioFeatureRequests.find (keyText);
    if (existing != gRecentAudioFeatureRequests.end())
    {
        existing->second.lastMs = nowMs;
        if (incomingPriorityRank < existing->second.priorityRank)
        {
            existing->second.priorityRank = incomingPriorityRank;
            return false;
        }
        return true;
    }

    gRecentAudioFeatureRequests[keyText] = { nowMs, incomingPriorityRank };
    return false;
}

void forgetRecentAudioFeatureRequestsContaining (const juce::String& token)
{
    const auto needle = token.trim();
    if (needle.isEmpty())
        return;

    const juce::ScopedLock lock (gRecentAudioFeatureRequestLock);
    const auto text = needle.toStdString();
    for (auto it = gRecentAudioFeatureRequests.begin(); it != gRecentAudioFeatureRequests.end();)
    {
        if (it->first.find (text) != std::string::npos)
            it = gRecentAudioFeatureRequests.erase (it);
        else
            ++it;
    }
}

void publishDeferredStatus (const AudioFeatureBakeRequest& request,
                            const AudioFeatureService::PublishCallback& publish)
{
    if (! publish)
        return;

    auto obj = std::make_unique<juce::DynamicObject>();
    obj->setProperty ("command", "audio_feature_bake_status");
    obj->setProperty ("feature_family", "audio_feature");
    obj->setProperty ("feature_type", audioFeatureTypeToString (request.featureType));
    obj->setProperty ("feature_version", audioFeatureProductVersion (request.featureType));
    obj->setProperty ("analysis_version", audioFeatureAnalysisVersion());
    obj->setProperty ("status", "deferred");
    obj->setProperty ("reason", "feature_baker_not_yet_promoted");
    obj->setProperty ("project_id", "current");
    obj->setProperty ("track_id", request.trackId);
    obj->setProperty ("source_path", request.filePath);
    if (request.sourceId.isNotEmpty())
        obj->setProperty ("source_id", request.sourceId);
    if (request.sourceRevision.isNotEmpty())
        obj->setProperty ("source_revision", request.sourceRevision);
    if (request.clipRevision.isNotEmpty())
        obj->setProperty ("clip_revision", request.clipRevision);
    if (request.renderRevision.isNotEmpty())
        obj->setProperty ("render_revision", request.renderRevision);
    if (request.requestId.isNotEmpty())
        obj->setProperty ("request_id", request.requestId);
    if (request.clipId.isNotEmpty())
        obj->setProperty ("clip_id", request.clipId);
    if (request.filePath.isNotEmpty())
        obj->setProperty ("file_path", request.filePath);

    publish (juce::JSON::toString (juce::var (obj.release())));
}

} // namespace

void AudioFeatureService::requestBake (AudioFeatureBakeRequest request,
                                       PublishCallback publishCallback)
{
    if (shouldMergeRecentAudioFeatureRequest (request))
        return;

    if (audioFeatureUsesSpectralTextureTile (request.featureType))
    {
        TiledSpectrogramBaker::startBake (request.filePath,
                                          request.trackId,
                                          request.clipId,
                                          std::move (publishCallback),
                                          request.range.sourceOffsetSeconds,
                                          request.range.lengthSeconds,
                                          request.sourceId,
                                          request.sourceRevision,
                                          request.clipRevision,
                                          request.renderRevision);
        return;
    }

    if (request.featureType == AudioFeatureType::WaveformEnvelope)
    {
        WaveformEnvelopeBaker::startBake (request.filePath,
                                          request.trackId,
                                          request.clipId,
                                          std::move (publishCallback),
                                          request.range.sourceOffsetSeconds,
                                          request.range.lengthSeconds,
                                          request.resolution.frameWidth > 0 ? request.resolution.frameWidth : 1024,
                                          request.sourceId,
                                          request.sourceRevision,
                                          request.clipRevision,
                                          request.renderRevision,
                                          request.priority);
        return;
    }

    if (request.featureType == AudioFeatureType::BandEnergySummary
        || request.featureType == AudioFeatureType::StereoRelationSummary
        || request.featureType == AudioFeatureType::LoudnessSummary
        || request.featureType == AudioFeatureType::L3AcousticSummary
        || request.featureType == AudioFeatureType::SegmentationPrimitives)
    {
        L3AcousticAnalyzer::startAnalyze (std::move (request), std::move (publishCallback));
        return;
    }

    // The unified service owns the routing contract now; individual primitive bakers
    // are promoted behind it incrementally.
    publishDeferredStatus (request, publishCallback);
}

void AudioFeatureService::requestLegacySpectralFieldBake (juce::String filePath,
                                                          juce::String trackId,
                                                          juce::String clipId,
                                                          PublishCallback publishCallback,
                                                          double sourceOffsetSeconds,
                                                          double bakeLengthSeconds)
{
    AudioFeatureBakeRequest request;
    request.filePath = std::move (filePath);
    request.trackId = std::move (trackId);
    request.clipId = std::move (clipId);
    request.featureType = AudioFeatureType::SpectralField;
    request.priority = AudioFeaturePriority::BackgroundWarm;
    request.range.sourceOffsetSeconds = sourceOffsetSeconds;
    request.range.lengthSeconds = bakeLengthSeconds;
    requestBake (std::move (request), std::move (publishCallback));
}

void AudioFeatureService::releaseTrackMappings (const juce::String& trackId)
{
    forgetRecentAudioFeatureRequestsContaining (juce::String ("track=") + trackId.trim() + "|");
    TiledSpectrogramBaker::releaseTrackMappings (trackId);
    WaveformEnvelopeBaker::releaseTrackMappings (trackId);
}

void AudioFeatureService::invalidateClipBake (const juce::String& clipId)
{
    forgetRecentAudioFeatureRequestsContaining (juce::String ("subject=") + clipId.trim() + "|");
    TiledSpectrogramBaker::invalidateClipBake (clipId);
    WaveformEnvelopeBaker::invalidateClipBake (clipId);
}

} // namespace vit
