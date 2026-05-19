#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class TrackService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    TrackService (EditGetter editGetter,
                  SaveProjectAction saveProjectAction);

    juce::String handleListTracks (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAppendGhostTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRenameTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetVolume (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetMute (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetSolo (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
};

} // namespace vit
