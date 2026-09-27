#pragma once

// TIM-KERNEL-DISCLOSE-1: pure classification helpers for the get_project_state
// disclosure extensions (GAPS Item 3 + Item 4). Deliberately dependency-free
// (std only) so both the tracktion-linked kernel sources and the plain-C++
// ctest target can include it.

#include <string>

namespace vit
{

// Item 4: per-instance external-plugin load state, structured form shared by
// the log line and the rack-node disclosure. loadState is one of
// "ready" / "async_pending" / "failed"; instanceReady is true only for
// "ready". asyncPending wins over the error string only in the sense that a
// still-initialising instance is transient, not failed (GAPS Item 4 risk:
// disclosure and consumption may straddle the async window).
struct ExternalPluginLoadState
{
    std::string loadState;
    bool instanceReady = false;
    std::string loadError;
};

inline ExternalPluginLoadState classifyExternalPluginLoadState (bool hasLoadError,
                                                                bool asyncPending)
{
    // RED-STUB for TIM-KERNEL-DISCLOSE-1: bodies stay wrong until the green
    // commit so the acceptance tests fail first.
    (void) hasLoadError;
    (void) asyncPending;
    return {};
}

// Item 3: the block size actually in use is only disclosable while an audio
// device is open; otherwise there is no "current" value at all. Returns -1
// for "omit the key" so consumers distinguish missing (not_evaluable) from a
// real zero.
inline int disclosedBlockSize (bool deviceOpen, int setupBufferSize)
{
    (void) deviceOpen;
    (void) setupBufferSize;
    return -1;
}

} // namespace vit
