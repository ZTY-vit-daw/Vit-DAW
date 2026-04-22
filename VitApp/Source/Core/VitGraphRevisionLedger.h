#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

struct VitGraphChangeEvent
{
    juce::String kind;
    juce::String summary;
    juce::String trackId;
    juce::String rackItemId;
    juce::String nodeId;
    juce::String sourceId;
    juce::String destId;
};

struct VitGraphRevisionSnapshot
{
    int64_t revision = 0;
    int64_t activeRevision = 0;
    int64_t pendingRevision = 0;
    int64_t lastRetiredRevision = 0;
    int retiredSnapshotCount = 0;
    juce::String lastSummary;
    juce::String lastKind;
    juce::String publishMode;
    juce::String lifecycleState;
    juce::Array<juce::var> recentChanges;
};

class VitGraphRevisionLedger final
{
public:
    static void resetForEdit (te::Edit&, const juce::String& reason);
    static VitGraphRevisionSnapshot recordChange (te::Edit&, const VitGraphChangeEvent&, const juce::String& publishMode);
    static VitGraphRevisionSnapshot getSnapshot (te::Edit&);
};

} // namespace vit
