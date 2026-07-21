#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class ProjectMarkerService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    ProjectMarkerService (EditGetter editGetter,
                          SaveProjectAction saveProjectAction);

    static juce::var createMarkersSnapshot (te::Edit& edit);

    juce::String handleListMarkers (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUpsertMarker (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleApplySectionMarkers (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRenameMarker (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteMarker (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
};

} // namespace vit
