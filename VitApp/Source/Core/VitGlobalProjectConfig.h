#pragma once

#include <JuceHeader.h>

namespace vit
{

/** Persists cross-session data (recent .vit paths) under the OS user application data folder. */
class VitGlobalProjectConfig final
{
public:
    VitGlobalProjectConfig();

    [[nodiscard]] juce::String buildGetRecentProjectsReply() const;
    void prependRecentProject (const juce::File& projectFile);

private:
    static constexpr int maxRecentEntries = 10;

    [[nodiscard]] juce::File getConfigFile() const;
    [[nodiscard]] juce::StringArray loadRecentPathsFromDisk() const;
    bool writeRecentPathsToDisk (const juce::StringArray& paths) const;
    static void normaliseAndDedupe (juce::StringArray& paths, const juce::String& insertFirst);

    juce::File configFile;
};

} // namespace vit
