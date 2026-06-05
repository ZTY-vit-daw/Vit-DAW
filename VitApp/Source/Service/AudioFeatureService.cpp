#include "AudioFeatureService.h"

#include "TiledSpectrogramBaker.h"
#include "WaveformEnvelopeBaker.h"

#include <utility>

namespace vit
{
namespace
{

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
    obj->setProperty ("track_id", request.trackId);
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
    if (audioFeatureUsesSpectralTextureTile (request.featureType))
    {
        TiledSpectrogramBaker::startBake (request.filePath,
                                          request.trackId,
                                          request.clipId,
                                          std::move (publishCallback),
                                          request.range.sourceOffsetSeconds,
                                          request.range.lengthSeconds);
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
                                          request.resolution.frameWidth > 0 ? request.resolution.frameWidth : 1024);
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
    TiledSpectrogramBaker::releaseTrackMappings (trackId);
    WaveformEnvelopeBaker::releaseTrackMappings (trackId);
}

void AudioFeatureService::invalidateClipBake (const juce::String& clipId)
{
    TiledSpectrogramBaker::invalidateClipBake (clipId);
    WaveformEnvelopeBaker::invalidateClipBake (clipId);
}

} // namespace vit
