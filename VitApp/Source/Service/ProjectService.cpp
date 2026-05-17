#include "ProjectService.h"

namespace vit
{

ProjectService::ProjectService (BoolAction reloadProjectAction,
                                RecentProjectsReply recentProjectsReplyAction,
                                NewBlankProjectReply newBlankProjectReplyAction,
                                OpenProjectReply openProjectReplyAction,
                                SaveProjectReply saveProjectReplyAction,
                                SaveAsProjectReply saveAsProjectReplyAction)
    : reloadProject (std::move (reloadProjectAction)),
      recentProjectsReply (std::move (recentProjectsReplyAction)),
      newBlankProjectReply (std::move (newBlankProjectReplyAction)),
      openProjectReply (std::move (openProjectReplyAction)),
      saveProjectReply (std::move (saveProjectReplyAction)),
      saveAsProjectReply (std::move (saveAsProjectReplyAction))
{
}

juce::String ProjectService::handleReloadProject (const juce::DynamicObject&, const juce::String&) const
{
    if (! reloadProject)
        return makeErrorReply ("Reload action is unavailable");

    if (reloadProject())
        return makeStatusReply ("ok", "Project reloaded");

    return makeErrorReply ("Failed to reload project");
}

juce::String ProjectService::handleSaveProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! saveProjectReply)
        return makeErrorReply ("save_project is not available in this build");

    return saveProjectReply (object);
}

juce::String ProjectService::handleGetRecentProjects (const juce::DynamicObject&, const juce::String&) const
{
    if (! recentProjectsReply)
        return makeErrorReply ("get_recent_projects is not available in this build");

    return recentProjectsReply();
}

juce::String ProjectService::handleOpenProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! openProjectReply)
        return makeErrorReply ("open_project is not available in this build");

    const juce::String pathStr = object.getProperty ("file_path").toString().trim();

    if (pathStr.isEmpty())
        return makeErrorReply ("open_project requires a non-empty file_path");

    const juce::File target (pathStr);

    if (! target.existsAsFile())
        return makeErrorReply ("Project file does not exist: " + pathStr);

    return openProjectReply (object, target);
}

juce::String ProjectService::handleNewProject (const juce::DynamicObject&, const juce::String&) const
{
    if (! newBlankProjectReply)
        return makeErrorReply ("new_project is not available in this build");

    return newBlankProjectReply();
}

juce::String ProjectService::handleSaveAsProject (const juce::DynamicObject& object, const juce::String&) const
{
    if (! saveAsProjectReply)
        return makeErrorReply ("save_as_project is not available in this build");

    const juce::String pathStr = object.getProperty ("file_path").toString().trim();

    if (pathStr.isEmpty())
        return makeErrorReply ("save_as_project requires a non-empty file_path");

    const juce::File target (pathStr);

    if (target.isDirectory())
        return makeErrorReply ("save_as_project: file_path must be a file path, not a directory");

    return saveAsProjectReply (object, target);
}

juce::String ProjectService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

} // namespace vit
