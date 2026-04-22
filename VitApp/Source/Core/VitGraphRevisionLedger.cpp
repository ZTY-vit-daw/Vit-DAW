#include "VitGraphRevisionLedger.h"

#include <cstdint>
#include <deque>
#include <unordered_map>

namespace vit
{

namespace
{

struct LedgerState
{
    int64_t revision = 0;
    juce::String lastSummary;
    juce::String lastKind;
    juce::String publishMode = "idle";
    std::deque<juce::var> recentChanges;
};

juce::CriticalSection& getLedgerLock()
{
    static juce::CriticalSection lock;
    return lock;
}

std::unordered_map<std::string, LedgerState>& getLedgerStates()
{
    static std::unordered_map<std::string, LedgerState> states;
    return states;
}

std::string makeEditKey (te::Edit& edit)
{
    return juce::String::toHexString (static_cast<juce::int64> (reinterpret_cast<std::uintptr_t> (&edit))).toStdString();
}

juce::var createEventVar (int64_t revision, const VitGraphChangeEvent& event)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("revision", revision);
    object->setProperty ("kind", event.kind);
    object->setProperty ("summary", event.summary);
    object->setProperty ("track_id", event.trackId);
    object->setProperty ("rack_item_id", event.rackItemId);
    object->setProperty ("node_id", event.nodeId);
    object->setProperty ("source_id", event.sourceId);
    object->setProperty ("dest_id", event.destId);
    object->setProperty ("timestamp_ms", static_cast<int64_t> (juce::Time::currentTimeMillis()));
    return juce::var (object.release());
}

VitGraphRevisionSnapshot toSnapshot (const LedgerState& state)
{
    VitGraphRevisionSnapshot snapshot;
    snapshot.revision = state.revision;
    snapshot.lastSummary = state.lastSummary;
    snapshot.lastKind = state.lastKind;
    snapshot.publishMode = state.publishMode;

    for (const auto& event : state.recentChanges)
        snapshot.recentChanges.add (event);

    return snapshot;
}

} // namespace

void VitGraphRevisionLedger::resetForEdit (te::Edit& edit, const juce::String& reason)
{
    const juce::ScopedLock sl (getLedgerLock());
    auto& state = getLedgerStates()[makeEditKey (edit)];
    state = {};
    state.lastKind = "edit_loaded";
    state.lastSummary = reason.isNotEmpty() ? reason : "Edit loaded";
    state.publishMode = "initialised";
}

VitGraphRevisionSnapshot VitGraphRevisionLedger::recordChange (te::Edit& edit,
                                                               const VitGraphChangeEvent& event,
                                                               const juce::String& publishMode)
{
    const juce::ScopedLock sl (getLedgerLock());
    auto& state = getLedgerStates()[makeEditKey (edit)];
    state.revision += 1;
    state.lastKind = event.kind;
    state.lastSummary = event.summary;
    state.publishMode = publishMode;
    state.recentChanges.push_front (createEventVar (state.revision, event));

    while (state.recentChanges.size() > 16)
        state.recentChanges.pop_back();

    return toSnapshot (state);
}

VitGraphRevisionSnapshot VitGraphRevisionLedger::getSnapshot (te::Edit& edit)
{
    const juce::ScopedLock sl (getLedgerLock());
    auto& state = getLedgerStates()[makeEditKey (edit)];
    return toSnapshot (state);
}

} // namespace vit
