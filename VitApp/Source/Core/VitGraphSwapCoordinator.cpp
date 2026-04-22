#include "VitGraphSwapCoordinator.h"

#include <deque>
#include <unordered_map>

namespace vit
{

namespace
{

struct RetiredSnapshot
{
    int64_t revision = 0;
    int64_t retireAfterMs = 0;
};

struct SwapState
{
    int64_t activeRevision = 0;
    int64_t pendingRevision = 0;
    int64_t lastRetiredRevision = 0;
    juce::String lifecycleState = "idle";
    std::deque<RetiredSnapshot> retiredSnapshots;
};

juce::CriticalSection& getSwapLock()
{
    static juce::CriticalSection lock;
    return lock;
}

std::unordered_map<std::string, SwapState>& getSwapStates()
{
    static std::unordered_map<std::string, SwapState> states;
    return states;
}

std::string makeEditKey (te::Edit& edit)
{
    return juce::String::toHexString (static_cast<juce::int64> (reinterpret_cast<std::uintptr_t> (&edit))).toStdString();
}

VitGraphRevisionSnapshot mergeSnapshot (const VitGraphRevisionSnapshot& ledgerSnapshot, const SwapState& swapState)
{
    auto merged = ledgerSnapshot;
    merged.activeRevision = swapState.activeRevision;
    merged.pendingRevision = swapState.pendingRevision;
    merged.lastRetiredRevision = swapState.lastRetiredRevision;
    merged.retiredSnapshotCount = static_cast<int> (swapState.retiredSnapshots.size());
    merged.lifecycleState = swapState.lifecycleState;
    return merged;
}

void enqueueRetiredSnapshot (SwapState& state, int64_t revision)
{
    if (revision <= 0)
        return;

    state.lastRetiredRevision = revision;
    state.retiredSnapshots.push_back ({ revision, juce::Time::currentTimeMillis() + 500 });
}

void pruneRetiredSnapshots (SwapState& state)
{
    const auto now = juce::Time::currentTimeMillis();

    while (! state.retiredSnapshots.empty()
           && (state.retiredSnapshots.front().retireAfterMs <= now || state.retiredSnapshots.size() > 8))
    {
        state.retiredSnapshots.pop_front();
    }
}

} // namespace

void VitGraphSwapCoordinator::resetForEdit (te::Edit& edit, const juce::String& reason)
{
    VitGraphRevisionLedger::resetForEdit (edit, reason);

    const juce::ScopedLock sl (getSwapLock());
    auto& state = getSwapStates()[makeEditKey (edit)];
    state = {};
    state.lifecycleState = "idle";
}

VitGraphRevisionSnapshot VitGraphSwapCoordinator::publishGraphChange (te::Edit& edit,
                                                                      const VitGraphChangeEvent& change)
{
    edit.dispatchPendingUpdatesSynchronously();
    auto& transport = edit.getTransport();
    transport.ensureContextAllocated (true);

    const bool live = transport.isPlaying();
    const auto ledgerSnapshot = VitGraphRevisionLedger::recordChange (edit,
                                                                      change,
                                                                      live ? "context_rebuild_live_pending"
                                                                           : "context_rebuild_idle");

    const juce::ScopedLock sl (getSwapLock());
    auto& state = getSwapStates()[makeEditKey (edit)];
    pruneRetiredSnapshots (state);

    if (live)
    {
        state.pendingRevision = ledgerSnapshot.revision;
        state.lifecycleState = "pending_live_swap";
    }
    else
    {
        if (state.activeRevision != 0 && state.activeRevision != ledgerSnapshot.revision)
            enqueueRetiredSnapshot (state, state.activeRevision);

        state.activeRevision = ledgerSnapshot.revision;
        state.pendingRevision = 0;
        state.lifecycleState = "active";
    }

    return mergeSnapshot (ledgerSnapshot, state);
}

VitGraphRevisionSnapshot VitGraphSwapCoordinator::serviceGraphLifecycle (te::Edit& edit)
{
    const juce::ScopedLock sl (getSwapLock());
    auto& state = getSwapStates()[makeEditKey (edit)];

    if (state.pendingRevision != 0)
    {
        if (state.activeRevision != 0 && state.activeRevision != state.pendingRevision)
            enqueueRetiredSnapshot (state, state.activeRevision);

        state.activeRevision = state.pendingRevision;
        state.pendingRevision = 0;
        state.lifecycleState = "active";
    }

    pruneRetiredSnapshots (state);
    auto snapshot = VitGraphRevisionLedger::getSnapshot (edit);

    if (snapshot.publishMode == "context_rebuild_live_pending" && state.pendingRevision == 0)
        snapshot.publishMode = "live_swap_activated";

    return mergeSnapshot (snapshot, state);
}

VitGraphRevisionSnapshot VitGraphSwapCoordinator::getSnapshot (te::Edit& edit)
{
    const juce::ScopedLock sl (getSwapLock());
    auto& state = getSwapStates()[makeEditKey (edit)];
    pruneRetiredSnapshots (state);
    return mergeSnapshot (VitGraphRevisionLedger::getSnapshot (edit), state);
}

} // namespace vit
