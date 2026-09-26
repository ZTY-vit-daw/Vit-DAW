#include "PluginListHygiene.h"

namespace vit
{

bool pluginListEntryIsPathValidated (const juce::PluginDescription& description)
{
    // Only the VST3 scan surface carries absolute file paths. Built-in and
    // other non-path identifiers must never be existence-checked: a rename or
    // identifier scheme change would otherwise mass-delete valid entries.
    return description.pluginFormatName == "VST3"
        && juce::File::isAbsolutePath (description.fileOrIdentifier);
}

bool defaultPluginPathExists (const juce::String& fileOrIdentifier)
{
    // mac VST3s are bundle DIRECTORIES (.vst3/), Windows ones single files;
    // existence must accept either form or every mac entry looks stale
    // (FIX-KERNEL-HYGIENE-BUNDLE-1: startup wiped 719/719 valid entries).
    return juce::File (fileOrIdentifier).exists();
}

PluginListHygieneReport cleanStalePluginListEntries (
    juce::KnownPluginList& list,
    const std::function<bool (const juce::String&)>& pathExists)
{
    PluginListHygieneReport report;
    report.typesBefore = list.getNumTypes();
    report.blacklistBefore = list.getBlacklistedFiles().size();

    // Snapshot first: removeType mutates the array getTypes() hands out.
    juce::Array<juce::PluginDescription> staleTypes;
    for (const auto& description : list.getTypes())
        if (pluginListEntryIsPathValidated (description) && ! pathExists (description.fileOrIdentifier))
            staleTypes.add (description);

    for (const auto& description : staleTypes)
    {
        report.removedTypePaths.add (description.fileOrIdentifier);
        list.removeType (description);
    }
    report.typesRemoved = report.removedTypePaths.size();

    // Blacklist entries are always file paths on the scan surface; anything
    // that is not an absolute path is left alone.
    const auto blacklistSnapshot = list.getBlacklistedFiles();
    for (const auto& blacklisted : blacklistSnapshot)
        if (juce::File::isAbsolutePath (blacklisted) && ! pathExists (blacklisted))
        {
            report.removedBlacklistPaths.add (blacklisted);
            list.removeFromBlacklist (blacklisted);
        }
    report.blacklistRemoved = report.removedBlacklistPaths.size();

    if (report.typesRemoved > 0 || report.blacklistRemoved > 0)
    {
        // One summary line per run, never one line per entry: a transplanted
        // Settings.xml can carry ~1000 ghost entries and must not flood the log.
        auto paths = report.removedTypePaths;
        paths.addArray (report.removedBlacklistPaths);
        const auto summary = paths.size() <= 3
            ? paths.joinIntoString (", ")
            : paths[0] + ", " + paths[1] + ", ... +" + juce::String (paths.size() - 2) + " more";
        juce::Logger::writeToLog ("PluginListHygiene: removed "
                                  + juce::String (report.typesRemoved) + " stale type entries (of "
                                  + juce::String (report.typesBefore) + ") and "
                                  + juce::String (report.blacklistRemoved) + " stale blacklist entries (of "
                                  + juce::String (report.blacklistBefore) + "): " + summary);
    }

    return report;
}

juce::StringArray applyUserCancelledScanCleanup (
    juce::KnownPluginList& list,
    const juce::StringArray& blacklistBeforeScan,
    const juce::File& deadMansPedalFile)
{
    // The cancel/crash dichotomy lives in the coordinator, never in JUCE
    // layer signals: the custom scanner returns false for aborts and crashes
    // alike, so juce::KnownPluginList::scanAndAddFile has already
    // unconditionally blacklisted the file being probed at cancel time.
    deadMansPedalFile.deleteFile();

    // Roll back only entries added after the scan snapshot was taken; the
    // snapshot is captured after the scanner consumed any pre-existing crash
    // pedal, so crash-defence blacklistings survive a later cancel.
    juce::StringArray rolledBack;
    const auto blacklistSnapshot = list.getBlacklistedFiles();
    for (const auto& blacklisted : blacklistSnapshot)
        if (! blacklistBeforeScan.contains (blacklisted))
            rolledBack.add (blacklisted);

    for (const auto& blacklisted : rolledBack)
        list.removeFromBlacklist (blacklisted);

    return rolledBack;
}

} // namespace vit
