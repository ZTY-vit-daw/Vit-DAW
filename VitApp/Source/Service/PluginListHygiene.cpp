#include "PluginListHygiene.h"

namespace vit
{

namespace
{
// juce::File::isAbsolutePath is platform-dependent: outside JUCE_WINDOWS it
// rejects drive-letter paths, so entries transplanted from a PC Settings.xml
// would never be classified as paths and would survive every startup cleanup
// (the 2026-09-28 mac stack kept 994 C:\ ghost entries this way). Drive-letter
// forms are paths on every platform; off Windows they simply never exist.
bool pluginListCarriesAbsolutePath (const juce::String& fileOrIdentifier)
{
    if (juce::File::isAbsolutePath (fileOrIdentifier))
        return true;

    return fileOrIdentifier.length() > 1
        && juce::CharacterFunctions::isLetter (fileOrIdentifier[0])
        && fileOrIdentifier[1] == ':';
}
} // namespace

bool pluginListEntryIsPathValidated (const juce::PluginDescription& description)
{
    // Only the VST3 scan surface carries absolute file paths. Built-in and
    // other non-path identifiers must never be existence-checked: a rename or
    // identifier scheme change would otherwise mass-delete valid entries.
    return description.pluginFormatName == "VST3"
        && pluginListCarriesAbsolutePath (description.fileOrIdentifier);
}

bool defaultPluginPathExists (const juce::String& fileOrIdentifier)
{
    // mac VST3s are bundle DIRECTORIES (.vst3/), Windows ones single files;
    // existence must accept either form or every mac entry looks stale
    // (FIX-KERNEL-HYGIENE-BUNDLE-1: startup wiped 719/719 valid entries).
    // Drive-letter paths can never exist off Windows, and juce::File would
    // log debug assertions for what it classifies as relative paths there.
   #if ! JUCE_WINDOWS
    if (pluginListCarriesAbsolutePath (fileOrIdentifier)
        && ! juce::File::isAbsolutePath (fileOrIdentifier))
        return false;
   #endif
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
        if (pluginListCarriesAbsolutePath (blacklisted) && ! pathExists (blacklisted))
        {
            report.removedBlacklistPaths.add (blacklisted);
            list.removeFromBlacklist (blacklisted);
        }
    report.blacklistRemoved = report.removedBlacklistPaths.size();

    // TIM-KERNEL-HYGIENE-1 (Item 5): stamp every run — including zero-removal
    // runs — so the disclosure block can distinguish "ran, removed nothing"
    // from "never ran" without inferring from counts.
    report.completedAtISO = juce::Time::getCurrentTime().toISO8601 (true);

    if (report.typesRemoved > 0 || report.blacklistRemoved > 0)
    {
        // One summary line per run, never one line per entry: a transplanted
        // Settings.xml can carry ~1000 ghost entries and must not flood the log.
        juce::Logger::writeToLog ("PluginListHygiene: removed "
                                  + juce::String (report.typesRemoved) + " stale type entries (of "
                                  + juce::String (report.typesBefore) + ") and "
                                  + juce::String (report.blacklistRemoved) + " stale blacklist entries (of "
                                  + juce::String (report.blacklistBefore) + "): " + pluginListHygieneRemovedSummary (report));
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

// TIM-KERNEL-HYGIENE-1 (Item 5): flood-safe removed-paths summary shared by
// the log line and the disclosure block — at most three entries, then
// "+ N more", so a transplanted Settings.xml with ~1000 ghosts cannot flood
// either surface.
juce::String pluginListHygieneRemovedSummary (const PluginListHygieneReport& report)
{
    auto paths = report.removedTypePaths;
    paths.addArray (report.removedBlacklistPaths);
    if (paths.isEmpty())
        return {};
    if (paths.size() <= 3)
        return paths.joinIntoString (", ");
    return paths[0] + ", " + paths[1] + ", ... +" + juce::String (paths.size() - 2) + " more";
}

juce::DynamicObject* pluginListHygieneStateObject (const PluginListHygieneReport& report,
                                                   bool cleanupRan)
{
    auto obj = std::make_unique<juce::DynamicObject>();
    obj->setProperty ("cleanup_ran", cleanupRan);
    obj->setProperty ("types_before", report.typesBefore);
    obj->setProperty ("types_removed", report.typesRemoved);
    obj->setProperty ("blacklist_before", report.blacklistBefore);
    obj->setProperty ("blacklist_removed", report.blacklistRemoved);
    obj->setProperty ("removed_total", report.typesRemoved + report.blacklistRemoved);
    obj->setProperty ("completed_at", cleanupRan ? report.completedAtISO : juce::String());
    obj->setProperty ("removed_summary", pluginListHygieneRemovedSummary (report));
    return obj.release();
}

} // namespace vit
