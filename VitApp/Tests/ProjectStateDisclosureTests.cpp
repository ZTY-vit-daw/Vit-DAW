// TIM-KERNEL-DISCLOSE-1 red-first acceptance tests for the two pure
// disclosure classifiers (GAPS Item 3 + Item 4).
//
// T1 load-state classification - no error and not async => ready with
//    instanceReady=true; async pending => async_pending, not ready, never
//    failed; a load error => failed with instanceReady=false.
// T2 block-size disclosure gate - device open discloses the setup value
//    (including 0 as a real value); device closed omits (-1).
//
// Red state: the header ships stub bodies, so every check fails and the
// test exits non-zero.

#include "../Source/Service/PluginLoadState.h"

#include <cstdio>
#include <cstdlib>
#include <string>

#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    std::_Exit (134);
}

void runLoadStateClassificationTest()
{
    const auto ready = vit::classifyExternalPluginLoadState (false, false);
    VIT_CHECK (ready.loadState == "ready");
    VIT_CHECK (ready.instanceReady == true);
    VIT_CHECK (ready.loadError.empty());

    const auto pending = vit::classifyExternalPluginLoadState (false, true);
    VIT_CHECK (pending.loadState == "async_pending");
    VIT_CHECK (pending.instanceReady == false);

    const auto failed = vit::classifyExternalPluginLoadState (true, false);
    VIT_CHECK (failed.loadState == "failed");
    VIT_CHECK (failed.instanceReady == false);

    // An instance that is still initialising is transient, not failed, even
    // if an error string sticks around from a retry.
    const auto pendingWithError = vit::classifyExternalPluginLoadState (true, true);
    VIT_CHECK (pendingWithError.loadState == "async_pending");
    VIT_CHECK (pendingWithError.instanceReady == false);
}

void runBlockSizeDisclosureTest()
{
    VIT_CHECK (vit::disclosedBlockSize (true, 512) == 512);
    VIT_CHECK (vit::disclosedBlockSize (true, 256) == 256);
    // A real zero is still a value: disclose it, let consumers judge.
    VIT_CHECK (vit::disclosedBlockSize (true, 0) == 0);
    // No device open: there is no "current" block size at all.
    VIT_CHECK (vit::disclosedBlockSize (false, 512) == -1);
}

} // namespace

int main()
{
    runLoadStateClassificationTest();
    runBlockSizeDisclosureTest();
    std::printf ("ProjectStateDisclosureTests: all checks passed\n");
    return 0;
}
