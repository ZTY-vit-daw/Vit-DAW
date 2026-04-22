#pragma once

#include <JuceHeader.h>

namespace vit
{

struct VitTakeRecord
{
    juce::String takeId;
    juce::String assetRef;
    juce::String absolutePath;
    juce::String clipId;
    juce::String trackId;
    juce::String jobId;
    juce::String label;
    juce::String warpState;
};

struct VitTakeStackRecord
{
    juce::String stackId;
    juce::String activeTakeId;
    juce::String clipId;
    juce::String trackId;
    juce::String nodeId;
};

class VitTakeHistoryStack final
{
public:
    static juce::String resolveStackId (const juce::DynamicObject& object,
                                        const juce::String& nodeIdFallback,
                                        const juce::String& clipIdFallback);
    static juce::String addTake (const juce::File& projectFile,
                                 const VitTakeStackRecord& stack,
                                 const VitTakeRecord& take,
                                 bool makeActive);
    static bool setActiveTake (const juce::File& projectFile,
                               const juce::String& stackId,
                               const juce::String& takeId);
    static bool getTake (const juce::File& projectFile,
                         const juce::String& stackId,
                         const juce::String& takeId,
                         VitTakeRecord& outTake);
    static juce::Array<juce::var> snapshotProjectStacks (const juce::File& projectFile);
};

} // namespace vit
