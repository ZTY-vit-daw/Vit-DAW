#pragma once

#include <functional>

#include <JuceHeader.h>

namespace vit
{

class ProjectService final
{
public:
    using BoolAction = std::function<bool()>;
    using RecentProjectsReply = std::function<juce::String()>;
    using NewBlankProjectReply = std::function<juce::String (const juce::DynamicObject&)>;
    using OpenProjectReply = std::function<juce::String (const juce::DynamicObject&, const juce::File&)>;
    using SaveProjectReply = std::function<juce::String (const juce::DynamicObject&)>;
    using SaveAsProjectReply = std::function<juce::String (const juce::DynamicObject&, const juce::File&)>;

    ProjectService (BoolAction reloadProjectAction,
                    RecentProjectsReply recentProjectsReply,
                    NewBlankProjectReply newBlankProjectReply,
                    OpenProjectReply openProjectReply,
                    SaveProjectReply saveProjectReply,
                    SaveAsProjectReply saveAsProjectReply);

    juce::String handleReloadProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSaveProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetRecentProjects (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleOpenProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleNewProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSaveAsProject (const juce::DynamicObject&, const juce::String&) const;

private:
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

    BoolAction reloadProject;
    RecentProjectsReply recentProjectsReply;
    NewBlankProjectReply newBlankProjectReply;
    OpenProjectReply openProjectReply;
    SaveProjectReply saveProjectReply;
    SaveAsProjectReply saveAsProjectReply;
};

} // namespace vit
