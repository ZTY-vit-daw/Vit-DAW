#include "VitPortablePropertyStorage.h"

#include "VitPaths.h"

namespace vit
{

VitPortablePropertyStorage::VitPortablePropertyStorage (juce::String applicationName)
    : te::PropertyStorage (std::move (applicationName))
{
}

juce::File VitPortablePropertyStorage::getAppCacheFolder()
{
    const auto cacheDirectory = paths::getCacheDirectory();
    paths::ensureDirectoryExists (paths::getWorkspaceDirectory(), "workspace");
    paths::ensureDirectoryExists (cacheDirectory, "cache");
    return cacheDirectory;
}

juce::File VitPortablePropertyStorage::getAppPrefsFolder()
{
    const auto settingsDirectory = paths::getSettingsDirectory();
    paths::ensureDirectoryExists (paths::getWorkspaceDirectory(), "workspace");
    paths::ensureDirectoryExists (settingsDirectory, "settings");
    return settingsDirectory;
}

void VitPortablePropertyStorage::flushSettingsToDisk()
{
    getPropertiesFile().saveIfNeeded();
}

juce::File VitPortablePropertyStorage::getDefaultLoadSaveDirectory (juce::StringRef)
{
    const auto workspaceDirectory = paths::getWorkspaceDirectory();
    paths::ensureDirectoryExists (workspaceDirectory, "workspace");
    return workspaceDirectory;
}

juce::File VitPortablePropertyStorage::getDefaultLoadSaveDirectory (te::ProjectItem::Category)
{
    return getDefaultLoadSaveDirectory (juce::StringRef());
}

} // namespace vit
