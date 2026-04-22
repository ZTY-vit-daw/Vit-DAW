#pragma once

#include <JuceHeader.h>

namespace vit
{

struct VitAIGCJobRecord
{
    juce::String jobId;
    juce::String nodeId;
    juce::String jobState = "idle";
    juce::String ghostState = "bypass";
    juce::String statusMessage;
    juce::String requestHash;
    juce::String source;
    juce::String assetRef;
    juce::String trackId;
    juce::String clipId;
};

class VitAIGCJobRuntime final
{
public:
    static void upsertJob (const juce::File& projectFile, const VitAIGCJobRecord& record);
    static void attachImportedAsset (const juce::File& projectFile,
                                     const juce::String& jobId,
                                     const juce::String& assetRef,
                                     const juce::String& trackId,
                                     const juce::String& clipId);
    static void setGhostState (const juce::File& projectFile,
                               const juce::String& jobId,
                               const juce::String& ghostState);
    static juce::Array<juce::var> snapshotProjectJobs (const juce::File& projectFile);
};

} // namespace vit
