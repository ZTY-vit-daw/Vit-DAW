#include "VitPluginGrabberGlobalProfile.h"

#include "VitPaths.h"

namespace vit
{

namespace
{

bool ensureGlobalProfilesDirectory()
{
    const auto dir = VitPluginGrabberGlobalProfile::getGlobalProfilesDirectory();
    if (dir.isDirectory())
        return true;

    if (dir.createDirectory())
    {
        juce::Logger::writeToLog ("VitPluginGrabberGlobalProfile: created directory: " + dir.getFullPathName());
        return true;
    }

    juce::Logger::writeToLog ("VitPluginGrabberGlobalProfile: failed to create directory: " + dir.getFullPathName());
    return false;
}

} // namespace

juce::File VitPluginGrabberGlobalProfile::getGlobalProfilesDirectory()
{
    return paths::getWorkspaceDirectory().getChildFile ("plugin_grabber_profiles");
}

juce::String VitPluginGrabberGlobalProfile::getProfileFilename (const juce::String& profileKey)
{
    auto clean = profileKey.trim().toLowerCase();
    if (clean.isEmpty())
        clean = "unknown";

    // Ensure it ends with .json
    if (! clean.endsWith (".json"))
        clean += ".json";

    return clean;
}

juce::var VitPluginGrabberGlobalProfile::findGlobalProfile (const juce::String& profileKey)
{
    if (profileKey.isEmpty())
        return {};

    const auto dir = getGlobalProfilesDirectory();
    const auto file = dir.getChildFile (getProfileFilename (profileKey));

    if (! file.existsAsFile())
        return {};

    const auto parsed = juce::JSON::parse (file.loadFileAsString());
    if (auto* obj = parsed.getDynamicObject())
    {
        // Quick sanity: if the stored profile_key doesn't match, discard.
        if (auto* identity = obj->getProperty ("plugin_identity").getDynamicObject())
            if (identity->getProperty ("profile_key").toString().trim().toLowerCase() == profileKey.trim().toLowerCase())
                return parsed;
    }

    juce::Logger::writeToLog ("VitPluginGrabberGlobalProfile: profile_key mismatch for " + profileKey
                              + ", discarding file " + file.getFullPathName());
    return {};
}

juce::Result VitPluginGrabberGlobalProfile::writeGlobalProfile (const juce::String& profileKey,
                                                                const juce::var& profileContent)
{
    if (profileKey.isEmpty())
        return juce::Result::fail ("Cannot write global profile with empty profileKey");

    if (! ensureGlobalProfilesDirectory())
        return juce::Result::fail ("Failed to create global profiles directory");

    const auto file = getGlobalProfilesDirectory().getChildFile (getProfileFilename (profileKey));

    juce::String json;
    if (auto* obj = profileContent.getDynamicObject())
    {
        // Ensure the profile has a schema_version
        if (! obj->hasProperty ("schema_version"))
            obj->setProperty ("schema_version", 1);

        // Ensure it has an updated_at timestamp
        obj->setProperty ("updated_at", juce::Time::getCurrentTime().toISO8601 (true));

        json = juce::JSON::toString (profileContent, true);
    }
    else
    {
        // If not a DynamicObject, stringify anyway
        json = juce::JSON::toString (profileContent, true);
    }

    if (file.replaceWithText (json))
    {
        juce::Logger::writeToLog ("VitPluginGrabberGlobalProfile: wrote profile " + profileKey
                                  + " -> " + file.getFullPathName());
        return juce::Result::ok();
    }

    return juce::Result::fail ("Failed to write global profile: " + profileKey);
}

bool VitPluginGrabberGlobalProfile::removeGlobalProfile (const juce::String& profileKey)
{
    if (profileKey.isEmpty())
        return false;

    const auto file = getGlobalProfilesDirectory().getChildFile (getProfileFilename (profileKey));
    if (! file.existsAsFile())
        return true; // already gone

    if (file.deleteFile())
    {
        juce::Logger::writeToLog ("VitPluginGrabberGlobalProfile: removed profile " + profileKey);
        return true;
    }

    return false;
}

juce::Array<juce::var> VitPluginGrabberGlobalProfile::snapshotGlobalProfiles()
{
    juce::Array<juce::var> profiles;
    const auto dir = getGlobalProfilesDirectory();

    if (! dir.isDirectory())
        return profiles;

    for (const auto& entry : juce::RangedDirectoryIterator (dir, false, "*.json"))
    {
        const auto file = entry.getFile();
        if (! file.existsAsFile())
            continue;

        const auto parsed = juce::JSON::parse (file.loadFileAsString());
        if (parsed.isObject())
            profiles.add (parsed);
    }

    return profiles;
}

} // namespace vit
