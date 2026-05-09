#include "VitProductionCoordinator.h"

#include <cmath>

namespace vit
{

namespace
{
juce::String makeCoordinatorError (const juce::String& message)
{
    auto o = std::make_unique<juce::DynamicObject>();
    o->setProperty ("status", "error");
    o->setProperty ("message", message);
    return juce::JSON::toString (juce::var (o.release()));
}
}

VitProductionCoordinator::VitProductionCoordinator (PublishFn publish)
    : publishMessage (std::move (publish))
{
}

juce::String VitProductionCoordinator::startOfflineRender (te::Edit& edit,
                                                           const juce::File& destFile,
                                                           double rangeStartSeconds,
                                                           double rangeEndSeconds,
                                                           int bitDepth,
                                                           bool useMasterPlugins)
{
    if (rendering.load())
        return makeCoordinatorError ("A render job is already in progress");

    if (! destFile.getParentDirectory().exists())
        destFile.getParentDirectory().createDirectory();

    if (destFile.existsAsFile() && ! destFile.deleteFile())
        return makeCoordinatorError ("Cannot overwrite destination file");

    const auto startSec = juce::jmin (rangeStartSeconds, rangeEndSeconds);
    const auto endSec = juce::jmax (rangeStartSeconds, rangeEndSeconds);

    jobId = juce::Uuid().toString();
    rendering.store (true);
    lastPublishedProgress = -1.0f;

    te::Renderer::Parameters params (edit);
    params.destFile = destFile;
    params.time = te::TimeRange (te::TimePosition::fromSeconds (startSec),
                                 te::TimePosition::fromSeconds (endSec));
    params.audioFormat = edit.engine.getAudioFileFormatManager().getWavFormat();
    params.bitDepth = bitDepth > 0 ? bitDepth : 24;
    params.useMasterPlugins = useMasterPlugins;

    if (auto* device = edit.engine.getDeviceManager().deviceManager.getCurrentAudioDevice())
        params.sampleRateForAudio = device->getCurrentSampleRate();
    else
        params.sampleRateForAudio = 48000.0;

    renderHandle = te::EditRenderer::render (
        std::move (params),
        [this, destPath = destFile.getFullPathName(), jid = jobId] (tl::expected<juce::File, std::string> res)
        {
            juce::MessageManager::callAsync (
                [this, destPath, jid, res]
                {
                    rendering.store (false);
                    renderHandle.reset();
                    lastPublishedProgress = -1.0f;

                    auto obj = std::make_unique<juce::DynamicObject>();
                    obj->setProperty ("topic", "render");
                    obj->setProperty ("subtopic", res.has_value() ? "render_done" : "render_failed");
                    obj->setProperty ("job_id", jid);
                    if (res.has_value())
                    {
                        obj->setProperty ("file_path", destPath);
                        obj->setProperty ("status", "ok");
                    }
                    else
                    {
                        obj->setProperty ("status", "error");
                        obj->setProperty ("message", juce::String (res.error()));
                    }

                    if (publishMessage)
                        publishMessage (juce::JSON::toString (juce::var (obj.release())));
                });
        });

    auto reply = std::make_unique<juce::DynamicObject>();
    reply->setProperty ("status", "ok");
    reply->setProperty ("message", "Render started");
    reply->setProperty ("job_id", jobId);
    return juce::JSON::toString (juce::var (reply.release()));
}

void VitProductionCoordinator::cancelOfflineRender()
{
    if (renderHandle != nullptr)
        renderHandle->cancel();
}

void VitProductionCoordinator::tick()
{
    if (! rendering.load() || renderHandle == nullptr)
        return;

    const float p = renderHandle->getProgress();
    if (p >= 0.0f && std::abs (p - lastPublishedProgress) >= 0.01f)
    {
        lastPublishedProgress = p;
        auto obj = std::make_unique<juce::DynamicObject>();
        obj->setProperty ("topic", "render");
        obj->setProperty ("subtopic", "render_progress");
        obj->setProperty ("job_id", jobId);
        obj->setProperty ("progress", p);
        if (publishMessage)
            publishMessage (juce::JSON::toString (juce::var (obj.release())));
    }
}

} // namespace vit
