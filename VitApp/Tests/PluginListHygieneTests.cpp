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
    // JUCE's File::isAbsolutePath recognises "X:" drive letters only under
    // JUCE_WINDOWS, so a PC-transplant path is path-validated (and therefore
    // removable) on Windows only; on mac the gate conservatively keeps the
    // identifier it cannot classify. Production semantics are unchanged —
    // these expectations just make the suite platform-accurate (the suite
    // was authored and first run on PC, FIX-KERNEL-HYGIENE-BUNDLE-1 mac run).
    VIT_CHECK (vit::pluginListEntryIsPathValidated (keptReal));
    VIT_CHECK (! vit::pluginListEntryIsPathValidated (builtIn));
    VIT_CHECK (! vit::pluginListEntryIsPathValidated (vst3NonPath));
   #if JUCE_WINDOWS
    VIT_CHECK (vit::pluginListEntryIsPathValidated (ghostPcPath));
    const int expectedTypesRemoved = 2;
    const int expectedTypesAfter = 3;
   #else
    VIT_CHECK (! vit::pluginListEntryIsPathValidated (ghostPcPath));
    const int expectedTypesRemoved = 1;
    const int expectedTypesAfter = 4;
   #endif

    const auto report = vit::cleanStalePluginListEntries (list);

    VIT_CHECK (report.typesBefore == 5);
    VIT_CHECK (report.typesRemoved == expectedTypesRemoved);
    VIT_CHECK (report.blacklistBefore == 3);
    VIT_CHECK (report.blacklistRemoved == 1);
    VIT_CHECK (report.removedTypePaths.contains (directory.getChildFile ("ghost.vst3").getFullPathName()));
   #if JUCE_WINDOWS
    VIT_CHECK (report.removedTypePaths.contains ("C:\\Program Files\\Ghost\\g.vst3"));
   #endif
    VIT_CHECK (report.removedBlacklistPaths.contains (directory.getChildFile ("ghost.vst3").getFullPathName()));

    VIT_CHECK (list.getNumTypes() == expectedTypesAfter);
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
    VIT_CHECK (list.getNumTypes() == expectedTypesAfter && list.getBlacklistedFiles().size() == 2);
}

// ---------------------------------------------------------------------------
// FIX-KERNEL-HYGIENE-BUNDLE-1: on mac every VST3 is a bundle DIRECTORY
// (Foo.vst3/Contents/MacOS/Foo), so the existence predicate must be
// form-agnostic. A .vst3 directory (mac bundle) and a .vst3 single file
// (Windows) both survive startup cleanup; only paths that exist as neither
// file nor directory are removed. The blacklist walks the same predicate.
// ---------------------------------------------------------------------------
void runBundleFormCleanupTest (const juce::File& directory)
{
    VIT_CHECK (directory.createDirectory());
    const auto macBundle = directory.getChildFile ("MacBundle.vst3");
    VIT_CHECK (macBundle.createDirectory());
    const auto windowsFile = directory.getChildFile ("WindowsSingle.vst3");
    VIT_CHECK (windowsFile.create());
    const auto ghostBundle = directory.getChildFile ("GhostBundle.vst3").getFullPathName();

    juce::KnownPluginList list;
    list.addType (makeVst3Entry ("Mac Bundle Directory", macBundle.getFullPathName()));
    list.addType (makeVst3Entry ("Windows Single File", windowsFile.getFullPathName()));
    list.addType (makeVst3Entry ("Ghost Bundle Path", ghostBundle));
    list.addToBlacklist (macBundle.getFullPathName());
    list.addToBlacklist (ghostBundle);

    const auto report = vit::cleanStalePluginListEntries (list);

    VIT_CHECK (report.typesRemoved == 1);
    VIT_CHECK (report.removedTypePaths.contains (ghostBundle));
    VIT_CHECK (list.getTypeForFile (macBundle.getFullPathName()) != nullptr);
    VIT_CHECK (list.getTypeForFile (windowsFile.getFullPathName()) != nullptr);
    VIT_CHECK (list.getTypeForFile (ghostBundle) == nullptr);

    VIT_CHECK (report.blacklistRemoved == 1);
    VIT_CHECK (list.getBlacklistedFiles().contains (macBundle.getFullPathName()));
    VIT_CHECK (! list.getBlacklistedFiles().contains (ghostBundle));
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

// ---------------------------------------------------------------------------
// TIM-KERNEL-HYGIENE-1 (Item 5): the startup cleanup report must be
// serializable into the get_project_state disclosure block — counts always
// present (zero cleanup = explicit 0), flood-safe path summary shared with
// the log line, ISO-8601 completed_at on every run, and an explicit
// never-ran form so consumers can tell "ran, removed nothing" apart from
// "never ran".
// ---------------------------------------------------------------------------
void runHygieneStateObjectTest (const juce::File& directory)
{
    VIT_CHECK (directory.createDirectory());
    const auto realFile = directory.getChildFile ("real.vst3");
    VIT_CHECK (realFile.create());

    juce::KnownPluginList list;
    list.addType (makeVst3Entry ("Real One", realFile.getFullPathName()));
    list.addToBlacklist (directory.getChildFile ("gone.vst3").getFullPathName());

    // Ran with something to remove.
    const auto report = vit::cleanStalePluginListEntries (list);
    VIT_CHECK (report.typesRemoved == 0);
    VIT_CHECK (report.blacklistRemoved == 1);
    VIT_CHECK (report.completedAtISO.isNotEmpty());
    VIT_CHECK (report.completedAtISO.contains ("T")); // ISO-8601 shape

    auto* object = vit::pluginListHygieneStateObject (report, true);
    VIT_CHECK (object != nullptr);
    if (object != nullptr)
    {
        VIT_CHECK ((bool) object->getProperty ("cleanup_ran") == true);
        VIT_CHECK ((int) object->getProperty ("types_before") == 1);
        VIT_CHECK ((int) object->getProperty ("types_removed") == 0);
        VIT_CHECK ((int) object->getProperty ("blacklist_before") == 1);
        VIT_CHECK ((int) object->getProperty ("blacklist_removed") == 1);
        VIT_CHECK ((int) object->getProperty ("removed_total") == 1);
        VIT_CHECK (object->getProperty ("completed_at").toString() == report.completedAtISO);
        VIT_CHECK (object->getProperty ("removed_summary").toString().isNotEmpty());
    }

    // Ran again over the already-clean list: explicit zero form.
    const auto cleanReport = vit::cleanStalePluginListEntries (list);
    auto* cleanObject = vit::pluginListHygieneStateObject (cleanReport, true);
    VIT_CHECK (cleanObject != nullptr);
    if (cleanObject != nullptr)
    {
        VIT_CHECK ((bool) cleanObject->getProperty ("cleanup_ran") == true);
        VIT_CHECK ((int) cleanObject->getProperty ("removed_total") == 0);
        VIT_CHECK ((int) cleanObject->getProperty ("types_removed") == 0);
        VIT_CHECK ((int) cleanObject->getProperty ("blacklist_removed") == 0);
        VIT_CHECK (cleanObject->getProperty ("completed_at").toString() == cleanReport.completedAtISO);
    }

    // Flood-safe summary: more than three removed paths collapses to the
    // first two plus "+ N more" (the log-line policy, now shared).
    vit::PluginListHygieneReport flood;
    flood.typesRemoved = 5;
    for (int i = 0; i < 5; ++i)
        flood.removedTypePaths.add ("/ghost/" + juce::String (i) + ".vst3");
    const auto summary = vit::pluginListHygieneRemovedSummary (flood);
    VIT_CHECK (summary.contains ("/ghost/0.vst3"));
    VIT_CHECK (summary.contains ("/ghost/1.vst3"));
    VIT_CHECK (summary.contains ("+ 3 more"));
    VIT_CHECK (! summary.contains ("/ghost/2.vst3"));

    // Never-ran form: empty stamp, explicit zeros, cleanup_ran=false.
    vit::PluginListHygieneReport neverRan;
    auto* neverObject = vit::pluginListHygieneStateObject (neverRan, false);
    VIT_CHECK (neverObject != nullptr);
    if (neverObject != nullptr)
    {
        VIT_CHECK ((bool) neverObject->getProperty ("cleanup_ran") == false);
        VIT_CHECK (neverObject->getProperty ("completed_at").toString().isEmpty());
        VIT_CHECK ((int) neverObject->getProperty ("removed_total") == 0);
    }
}

int main()
{
    const auto directory = juce::File::getSpecialLocation (juce::File::tempDirectory)
                               .getChildFile ("vit-pluginlist-hygiene-tests-" + juce::Uuid().toString());
    VIT_CHECK (directory.createDirectory());

    runStartupCleanupTest (directory.getChildFile ("startup-cleanup"));
    runBundleFormCleanupTest (directory.getChildFile ("bundle-form"));
    runUserCancelCleanupTest (directory.getChildFile ("user-cancel"));
    runCrashContrastTest (directory.getChildFile ("crash-contrast"));
    runHygieneStateObjectTest (directory.getChildFile ("hygiene-state-object"));

    directory.deleteRecursively();
    std::printf ("PluginListHygieneTests: all checks passed\n");
    return 0;
}
