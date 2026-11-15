#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

// KERNEL-TIMESIG-1: the project time-signature command face.
//
// Meter lives in the engine's tempo sequence (TimeSigSetting index 0) — the
// same store the music_context disclosure already reads — so persistence and
// old-project semantics come from the engine's own ValueTree:
//   * TimeSigSetting wires numerator/denominator as CachedValues with default
//     4 — a project file saved before any non-4/4 meter (i.e. every existing
//     Vit project) loads as 4/4 with zero behavioural change (AGENTS §11).
//   * Unknown out-of-range values are rejected on the command face before any
//     state is touched (fail-closed); persisted ints are engine-native.
//
// The write path follows the set_tempo precedent (CommandDispatcher
// handleSetTempo): assign the value, then Edit::markAsChanged() — the raw
// ValueTree/CachedValue write does not synchronously reach the undo change
// machinery, and without the explicit mark an open_project'd edit would skip
// the disk write while runAutoPersist still reports success — then the
// governance-aware auto-persist funnel saves the working copy.
//
// Creation-time application (new_project payload time_signature "N/D", the
// W01 contract string format from docs/LEGAME_DESIGN_CONTRACT_V0.md §4.6
// workspace.timing.time_signature — bridging note for the 373 coursetemplate
// assembly: this kernel accepts exactly that field's value) shares the same
// parse/validate/apply helpers so both faces enforce one validation domain.
class TimeSignatureService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using AutoPersist = std::function<bool()>;

    TimeSignatureService (EditGetter editGetter, AutoPersist autoPersistAction);

    // Command face: set_time_signature { numerator: int, denominator: int }.
    // Fail-closed: any invalid input is rejected without touching the edit.
    juce::String handleSetTimeSignature (const juce::DynamicObject&, const juce::String&) const;

    // Parse the W01 contract string form "N/D" (positive integers, each in
    // [kMinTimeSigValue, kMaxTimeSigValue], no extra characters). Returns
    // false with a specific error and leaves the outputs untouched.
    static bool parseTimeSignatureString (const juce::String& text, int& numerator, int& denominator, juce::String& error);

    // Validate (int bounds) and apply numerator/denominator to time sig 0 of
    // the edit's tempo sequence, then mark the edit changed. Returns false
    // with a specific error and leaves the edit untouched on rejection.
    static bool applyToEdit (te::Edit& edit, int numerator, int denominator, juce::String& error);

    static constexpr int kMinTimeSigValue = 1;
    static constexpr int kMaxTimeSigValue = 64;

private:
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    AutoPersist autoPersist;
};

} // namespace vit
