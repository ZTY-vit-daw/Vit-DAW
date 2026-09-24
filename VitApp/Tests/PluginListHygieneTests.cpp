#include "../Source/Service/PluginListHygiene.h"

#include <cstdio>
#include <cstdlib>

// FIX-KERNEL-PLUGINLIST-HYGIENE-1 red-first acceptance tests. Checks must stay
// active in every configuration (plain assert() vanishes under NDEBUG), so we
// reuse the VIT_CHECK abort pattern from AuditionPreviewAudioPlaneTests.
#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    // std::_Exit, not abort(): a Debug MSVC abort() can raise a JIT-debug
    // dialog and hang unattended ctest runs. Unbuffered exit keeps the
    // failure observable through the process exit code.
    std::_Exit (134);
}

juce::PluginDescription makeVst3Entry (juce::String name, juce::String fileOrIdentifier)
{
    juce::PluginDescription description;
    description.name = std::move (name);
    description.pluginFormatName = "VST3";
    description.fileOrIdentifier = std::move (fileOrIdentifier);
    return description;
}

// ---------------------------------------------------------------------------
// Acceptance (1): Settings constructed with non-existent paths -> startup
// lazy cleanup removes exactly those entries (types AND blacklist), keeps
// existing files and non-path identifiers, and is idempotent.
// ---------------------------------------------------------------------------
void runStartupCleanupTest (const juce::File& directory)
{
    VIT_CHECK (directory.createDirectory());
    const auto realFile = directory.getChildFile ("real.vst3");
    VIT_CHECK (realFile.create());

    juce::KnownPluginList list;

    const auto keptReal = makeVst3Entry ("Real One", realFile.getFullPathName());
    const auto ghostLocal = makeVst3Entry ("Ghost Local", directory.getChildFile ("ghost.vst3").getFullPathName());
    const auto ghostPcPath = makeVst3Entry ("Ghost PC Transplant", "C:\\Program Files\\Ghost\\g.vst3");

    juce::PluginDescription builtIn;
    builtIn.name = "BuiltIn Volume";
    builtIn.pluginFormatName = "Tracktion";
    builtIn.fileOrIdentifier = "volume";

    const auto vst3NonPath = makeVst3Entry ("VST3 NonPath", "not-a-path");

    list.addType (keptReal);
    list.addType (ghostLocal);
    list.addType (ghostPcPath);
    list.addType (builtIn);
    list.addType (vst3NonPath);

    list.addToBlacklist (directory.getChildFile ("ghost.vst3").getFullPathName());
    list.addToBlacklist (realFile.getFullPathName());
    list.addToBlacklist ("weird-blacklist-entry");

    VIT_CHECK (list.getNumTypes() == 5);
    VIT_CHECK (list.getBlacklistedFiles().size() == 3);

    // Decision-level guard: only absolute-path VST3 entries are validated.
    VIT_CHECK (vit::pluginListEntryIsPathValidated (keptReal));
    VIT_CHECK (vit::pluginListEntryIsPathValidated (ghostPcPath));
    VIT_CHECK (! vit::pluginListEntryIsPathValidated (builtIn));
    VIT_CHECK (! vit::pluginListEntryIsPathValidated (vst3NonPath));

    const auto report = vit::cleanStalePluginListEntries (list);

    VIT_CHECK (report.typesBefore == 5);
    VIT_CHECK (report.typesRemoved == 2);
    VIT_CHECK (report.blacklistBefore == 3);
    VIT_CHECK (report.blacklistRemoved == 1);
    VIT_CHECK (report.removedTypePaths.contains (directory.getChildFile ("ghost.vst3").getFullPathName()));
    VIT_CHECK (report.removedTypePaths.contains ("C:\\Program Files\\Ghost\\g.vst3"));
    VIT_CHECK (report.removedBlacklistPaths.contains (directory.getChildFile ("ghost.vst3").getFullPathName()));

    VIT_CHECK (list.getNumTypes() == 3);
    VIT_CHECK (list.getTypeForFile (realFile.getFullPathName()) != nullptr);
    VIT_CHECK (list.getTypeForFile ("volume") != nullptr);
    VIT_CHECK (list.getTypeForFile ("not-a-path") != nullptr);
    VIT_CHECK (list.getTypeForFile (directory.getChildFile ("ghost.vst3").getFullPathName()) == nullptr);

    const auto& blacklist = list.getBlacklistedFiles();
    VIT_CHECK (blacklist.size() == 2);
    VIT_CHECK (blacklist.contains (realFile.getFullPathName()));
    VIT_CHECK (blacklist.contains ("weird-blacklist-entry"));
    VIT_CHECK (! blacklist.contains (directory.getChildFile ("ghost.vst3").getFullPathName()));

    // Idempotent: a second pass over the cleaned list changes nothing.
    const auto secondPass = vit::cleanStalePluginListEntries (list);
    VIT_CHECK (secondPass.typesRemoved == 0 && secondPass.blacklistRemoved == 0);
    VIT_CHECK (list.getNumTypes() == 3 && list.getBlacklistedFiles().size() == 2);
}

// ---------------------------------------------------------------------------
// Acceptance (2): user cancel -> dead-man's-pedal file is deleted, blacklist
// additions from this scan are rolled back, pre-existing blacklist entries
// and types discovered during the scan are preserved.
// ---------------------------------------------------------------------------
void runUserCancelCleanupTest (const juce::File& directory)
{
    VIT_CHECK (directory.createDirectory());
    const auto realFile = directory.getChildFile ("real.vst3");
    VIT_CHECK (realFile.create());

    juce::KnownPluginList list;

    // Partial scan result already in the table before the cancel epilogue.
    const auto discovered = makeVst3Entry ("Discovered Before Cancel", realFile.getFullPathName());
    list.addType (discovered);

    // Blacklist state before the scan started.
    const auto preExistingBlacklistEntry = directory.getChildFile ("pre-existing-bad.vst3").getFullPathName();
    list.addToBlacklist (preExistingBlacklistEntry);

    juce::StringArray blacklistBeforeScan;
    blacklistBeforeScan.add (preExistingBlacklistEntry);

    // During the scan: the file being probed at cancel time is blacklisted
    // unconditionally by juce (custom scanner returns false for aborts too).
    const auto cancelledProbedFile = directory.getChildFile ("cancelled-file.vst3").getFullPathName();
    list.addToBlacklist (cancelledProbedFile);
    VIT_CHECK (list.getBlacklistedFiles().size() == 2);

    // Damaged dead-man's-pedal residue left behind by the aborted scan.
    const auto pedalFile = directory.getChildFile ("plugin_scan_dead_mans_pedal.txt");
    pedalFile.replaceWithText (cancelledProbedFile + "\n" + directory.getChildFile ("another.vst3").getFullPathName());
    VIT_CHECK (pedalFile.existsAsFile());

    const auto rolledBack = vit::applyUserCancelledScanCleanup (list, blacklistBeforeScan, pedalFile);

    VIT_CHECK (rolledBack.size() == 1);
    VIT_CHECK (rolledBack.contains (cancelledProbedFile));

    VIT_CHECK (! pedalFile.existsAsFile());

    const auto& blacklist = list.getBlacklistedFiles();
    VIT_CHECK (blacklist.size() == 1);
    VIT_CHECK (blacklist.contains (preExistingBlacklistEntry));
    VIT_CHECK (! blacklist.contains (cancelledProbedFile));

    // Partial results survive the cancel: they are legal table members.
    VIT_CHECK (list.getNumTypes() == 1);
    VIT_CHECK (list.getTypeForFile (realFile.getFullPathName()) != nullptr);
}

// ---------------------------------------------------------------------------
// Acceptance (3): crash contrast — a dead-man's-pedal file left behind by a
// killed process still blacklists its entries when the next scanner is
// constructed. The crash defence must not be weakened by the cancel epilogue.
// ---------------------------------------------------------------------------
void runCrashContrastTest (const juce::File& directory)
{
    VIT_CHECK (directory.createDirectory());
    const auto pedalFile = directory.getChildFile ("crashed_pedal.txt");
    const auto crashedPlugin = juce::String ("C:/crashed/plugin.vst3");
    pedalFile.replaceWithText (crashedPlugin);
    VIT_CHECK (pedalFile.existsAsFile());

    juce::KnownPluginList list;
    juce::VST3PluginFormat format;
    juce::FileSearchPath emptySearchPath;

    {
        juce::PluginDirectoryScanner scanner (list, format, emptySearchPath, true, pedalFile, false);
        juce::ignoreUnused (scanner);
    }

    VIT_CHECK (list.getBlacklistedFiles().contains (crashedPlugin));
}

} // namespace

int main()
{
    const auto directory = juce::File::getSpecialLocation (juce::File::tempDirectory)
                               .getChildFile ("vit-pluginlist-hygiene-tests-" + juce::Uuid().toString());
    VIT_CHECK (directory.createDirectory());

    runStartupCleanupTest (directory.getChildFile ("startup-cleanup"));
    runUserCancelCleanupTest (directory.getChildFile ("user-cancel"));
    runCrashContrastTest (directory.getChildFile ("crash-contrast"));

    directory.deleteRecursively();
    std::printf ("PluginListHygieneTests: all checks passed\n");
    return 0;
}
