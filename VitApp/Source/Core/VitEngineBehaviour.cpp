#include "VitEngineBehaviour.h"

#include "VitPaths.h"

namespace vit
{

juce::File VitEngineBehaviour::getDefaultFolderForAudioRecordings (te::Edit& edit)
{
    juce::ignoreUnused (edit);
    const auto dir = paths::getCacheDirectory().getChildFile ("Recordings");
    paths::ensureDirectoryExists (paths::getWorkspaceDirectory(), "workspace");
    paths::ensureDirectoryExists (paths::getCacheDirectory(), "cache");
    paths::ensureDirectoryExists (dir, "recordings");
    return dir;
}

juce::File VitEngineBehaviour::getFileForNewAudioRecording (te::Track& track, const juce::String& fileExtension)
{
    const auto ext = fileExtension.isNotEmpty() ? fileExtension : juce::String ("wav");
    auto folder = getDefaultFolderForAudioRecordings (track.edit);
    const auto name = juce::String ("rec_") + juce::Uuid().toString().removeCharacters ("-").substring (0, 12)
                      + juce::String (".") + ext;
    return folder.getChildFile (name);
}

} // namespace vit
