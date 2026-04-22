#include "VitNodeProfiler.h"

namespace vit
{

juce::var VitNodeProfiler::createProjectProfile (te::Edit& edit,
                                                 const juce::Array<juce::var>& jobs,
                                                 const juce::Array<juce::var>& assets,
                                                 const juce::Array<juce::var>& takeHistories)
{
    int trackCount = 0;
    int clipCount = 0;

    for (auto* track : te::getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        ++trackCount;
        clipCount += track->getNumTrackItems();
    }

    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("track_count", trackCount);
    object->setProperty ("clip_count", clipCount);
    object->setProperty ("job_count", jobs.size());
    object->setProperty ("generated_asset_count", assets.size());
    object->setProperty ("take_stack_count", takeHistories.size());
    return juce::var (object.release());
}

} // namespace vit
