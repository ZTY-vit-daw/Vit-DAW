#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class TrackGroupService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    TrackGroupService (EditGetter editGetter,
                       SaveProjectAction saveProjectAction);

    static juce::var createGroupsSnapshot (te::Edit& edit);

    juce::String handleListGroups (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCreateGroup (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUpdateGroup (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteGroup (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetGroupMembers (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleApplyGroupControl (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
};

} // namespace vit
