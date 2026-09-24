#pragma once

#include <JuceHeader.h>

#include <functional>

namespace vit
{

// FIX-KERNEL-PLUGINLIST-HYGIENE-1: kernel plugin-list hygiene primitives.
// Decision + IO layer for (1) lazy startup cleanup of stale plugin entries and
// (2) the user-cancelled scan epilogue. Crash isolation is deliberately NOT
// touched here: the dead-man's-pedal residue -> blacklist path inside
// juce::PluginDirectoryScanner is the crash defence and must stay intact.

struct PluginListHygieneReport
{
    int typesBefore = 0;
    int typesRemoved = 0;
    int blacklistBefore = 0;
    int blacklistRemoved = 0;
    juce::StringArray removedTypePaths;
    juce::StringArray removedBlacklistPaths;
};

// Only entries that actually carry an absolute file-system path are validated
// (VST3 scan surface). Built-in/non-path identifiers are never touched, so a
// format rename or identifier scheme can never mass-delete user entries.
bool pluginListEntryIsPathValidated (const juce::PluginDescription& description);

bool defaultPluginPathExists (const juce::String& fileOrIdentifier);

// Lazy startup cleanup: removes type entries and blacklist entries whose file
// path no longer exists (e.g. Settings.xml transplanted across platforms).
// Idempotent; logs one summary line, never one line per removed entry.
PluginListHygieneReport cleanStalePluginListEntries (
    juce::KnownPluginList& list,
    const std::function<bool (const juce::String&)>& pathExists = &defaultPluginPathExists);

// User-cancel epilogue (coordinator-level cancel/crash dichotomy): deletes the
// dead-man's-pedal file and rolls back blacklist entries added during this
// scan (the file being scanned at cancel time is blacklisted unconditionally
// by juce::KnownPluginList::scanAndAddFile because the custom scanner returns
// false for aborts too). Entries already blacklisted before the scan stay.
// Types discovered before the cancel stay in the list (partial results are
// legal). Returns the rolled-back entries for logging.
juce::StringArray applyUserCancelledScanCleanup (
    juce::KnownPluginList& list,
    const juce::StringArray& blacklistBeforeScan,
    const juce::File& deadMansPedalFile);

} // namespace vit
