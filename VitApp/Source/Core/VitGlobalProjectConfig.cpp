#include "VitGlobalProjectConfig.h"

namespace vit
{

VitGlobalProjectConfig::VitGlobalProjectConfig()
    : configFile (getConfigFile())
{
}

juce::File VitGlobalProjectConfig::getConfigFile() const
{
    const auto dir = juce::File::getSpecialLocation (juce::File::userApplicationDataDirectory)
                         .getChildFile ("Vit-DAW");

    if (! dir.isDirectory())
        dir.createDirectory();

    return dir.getChildFile ("global_project_config.json");
}

juce::StringArray VitGlobalProjectConfig::loadRecentPathsFromDisk() const
{
    juce::StringArray out;

    if (! configFile.existsAsFile())
        return out;

    const auto text = configFile.loadFileAsString();

    if (text.isEmpty())
        return out;

    const auto parsed = juce::JSON::parse (text);

    if (parsed.isVoid())
        return out;

    auto* root = parsed.getDynamicObject();

    if (root == nullptr)
        return out;

    const auto recentVar = root->getProperty ("recent_projects");

    if (auto* arr = recentVar.getArray())
    {
        for (const auto& v : *arr)
        {
            const auto s = v.toString().trim();

            if (s.isNotEmpty())
                out.addIfNotAlreadyThere (s);
        }
    }

    return out;
}

bool VitGlobalProjectConfig::writeRecentPathsToDisk (const juce::StringArray& paths) const
{
    auto root = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> arr;

    for (int i = 0; i < paths.size(); ++i)
        arr.add (paths[i]);

    root->setProperty ("recent_projects", juce::var (arr));

    const juce::String json = juce::JSON::toString (juce::var (root.release()), true);

    if (json.isEmpty())
        return false;

    if (! configFile.getParentDirectory().isDirectory())
        configFile.getParentDirectory().createDirectory();

    return configFile.replaceWithText (json);
}

void VitGlobalProjectConfig::normaliseAndDedupe (juce::StringArray& paths, const juce::String& insertFirst)
{
    juce::StringArray next;
    juce::StringArray seen;

    auto addOne = [&] (const juce::String& raw)
    {
        const auto p = juce::File (raw).getFullPathName();

        if (p.isEmpty())
            return;

        if (seen.contains (p, true))
            return;

        seen.add (p);
        next.add (p);
    };

    addOne (insertFirst);

    for (int i = 0; i < paths.size(); ++i)
        addOne (paths[i]);

    while (next.size() > maxRecentEntries)
        next.remove (next.size() - 1);

    paths = next;
}

void VitGlobalProjectConfig::prependRecentProject (const juce::File& projectFile)
{
    if (! projectFile.existsAsFile())
        return;

    const auto canonical = projectFile.getFullPathName();
    auto paths = loadRecentPathsFromDisk();
    normaliseAndDedupe (paths, canonical);

    if (! writeRecentPathsToDisk (paths))
        juce::Logger::writeToLog ("VitGlobalProjectConfig: failed to write " + configFile.getFullPathName());
}

juce::String VitGlobalProjectConfig::buildGetRecentProjectsReply() const
{
    const auto paths = loadRecentPathsFromDisk();
    auto response = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> arr;

    for (int i = 0; i < paths.size(); ++i)
        arr.add (paths[i]);

    response->setProperty ("status", "ok");
    response->setProperty ("recent_projects", juce::var (arr));
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit
