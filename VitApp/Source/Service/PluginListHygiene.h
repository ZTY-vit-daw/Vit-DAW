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
    // TIM-KERNEL-HYGIENE-1 (Item 5): wall-clock stamp of the cleanup run,
    // ISO-8601 with timezone; set on every run, including zero-removal runs.
    juce::String completedAtISO;
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

// TIM-KERNEL-HYGIENE-1 (Item 5): flood-safe summary of the removed paths
// (at most three shown, then "+ N more"), shared by the log line and the
// project-state disclosure block so the two can never drift apart.
juce::String pluginListHygieneRemovedSummary (const PluginListHygieneReport& report);

// TIM-KERNEL-HYGIENE-1 (Item 5): serialize the last hygiene report into the
// get_project_state disclosure block. Counts are always present (zero
// cleanup = explicit 0); cleanupRan=false emits the never-ran form with an
// empty completed_at so consumers can distinguish "ran, removed nothing"
// from "never ran" (agent side reports not_evaluable for the latter).
juce::DynamicObject* pluginListHygieneStateObject (const PluginListHygieneReport& report,
                                                   bool cleanupRan);

} // namespace vit
