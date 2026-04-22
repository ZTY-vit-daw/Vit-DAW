#include "VitProbeNode.h"

namespace vit
{

namespace
{

juce::var makeChannel (const juce::String& channelId, const juce::String& label)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("channel_id", channelId);
    object->setProperty ("label", label);
    return juce::var (object.release());
}

} // namespace

juce::Array<juce::var> VitProbeNode::createBuiltInProbeChannels()
{
    juce::Array<juce::var> channels;
    channels.add (makeChannel ("job_state", "Job State"));
    channels.add (makeChannel ("ghost_state", "Ghost State"));
    channels.add (makeChannel ("graph_revision", "Graph Revision"));
    channels.add (makeChannel ("active_take", "Active Take"));
    channels.add (makeChannel ("cache_hit", "Cache Hit"));
    channels.add (makeChannel ("output_source", "Output Source"));
    return channels;
}

} // namespace vit
