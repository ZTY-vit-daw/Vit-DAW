#include "VitAsyncGhostPolicy.h"

namespace vit
{

juce::String VitAsyncGhostPolicy::normalise (const juce::String& policy)
{
    const auto value = policy.trim().toLowerCase();

    if (value == "mute")
        return "mute";

    if (value == "play_cache" || value == "play-cache" || value == "cache")
        return "play_cache";

    return "bypass";
}

bool VitAsyncGhostPolicy::isValid (const juce::String& policy)
{
    const auto value = normalise (policy);
    return value == "bypass" || value == "play_cache" || value == "mute";
}

} // namespace vit
