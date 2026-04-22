#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitParallelMergeAdvice
{
    bool requiresAttention = false;
    int incomingBranchCount = 0;
    juce::String summary;
};

class VitParallelMergePlanner final
{
public:
    static VitParallelMergeAdvice analyseDestination (const te::RackType& rackType,
                                                      te::EditItemID destId,
                                                      te::Plugin* destPlugin = nullptr,
                                                      te::EditItemID pendingSourceId = {});
};

} // namespace vit
