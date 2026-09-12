#pragma once

// Deliberately dependency-free (no JUCE): the B15 persist gate is a pure
// decision, so it stays unit-testable without the JUCE/Tracktion build graph.

namespace vit
{

/** B15: true manual-save semantics for the on-disk working copy.
 *
 * Product ruling (2026-09-11): the .vit file on disk is the user's last manual
 * save. Agent-governed mutations (VSP version advance) belong to the version
 * repository (blob + commit) only and must not advance the working copy.
 *
 * Kernel wiring truth (forensics 2026-09-11/12, artifacts/mantest3/20260911):
 *   - Godot UI commands arrive as RAW payloads ({"cmd":"rack_add_node",...})
 *     on the command channel and are the user's own in-session edits.
 *   - Agent-governed mutations arrive as VSP envelopes
 *     (isVspEnvelope == true) and are unwrapped into the same legacy command
 *     handlers, which each end in the auto-persist callback
 *     (CommandDispatcher::saveProject -> VitHeadlessService::saveProjectToCurrentPathOrDefaultXml).
 *     Log proof: VitHeadlessServer2026-09-11_19-53-47.log lines 52426-52432
 *     ("Command executed: instantiate_plugin") is the agent's D1 StaticEQ load.
 *
 * So the origin of the governance is carried by the envelope shape, and the
 * policy needs a dispatch-scoped marker rather than a command allowlist.
 *
 * Scope of the gate (deliberately narrow):
 *   - Suppressed: the auto-persist callback fired from inside a governed
 *     mutation (governanceFlush, tempo, clear_project, plugin rack
 *     add/connect/remove, track add/move/delete, marker edits, import, ...).
 *   - Untouched: the explicit project.save lifecycle commands
 *     (save_project / save_as_project / save_project_copy) do NOT route through
 *     the auto-persist callback at all - they reach
 *     VitHeadlessService::ipcSaveEncryptedProjectToPath /
 *     ipcSaveProjectToCurrentPath / ipcSaveProjectAsProjectAt directly. The
 *     user's manual save therefore keeps writing the working copy unchanged.
 *   - Untouched: every path with no Project History / VSP governance (the bare
 *     kernel command channel) still persists exactly as before.
 *
 * Not yet deferred (reported, not silently changed): a Godot UI plugin load is
 * a raw command and is still persisted by the kernel. Deferring that as well
 * would require an explicit-save path on the UI side, which is out of this
 * card's file domain.
 */
class VitWorkingCopyPersistPolicy final
{
public:
    /** RAII marker for "we are executing a governed (VSP) mutation". */
    class GovernanceScope final
    {
    public:
        explicit GovernanceScope (VitWorkingCopyPersistPolicy& target) noexcept
            : policy (target)
        {
            ++policy.governanceDepth;
        }

        ~GovernanceScope() noexcept
        {
            --policy.governanceDepth;
        }

        GovernanceScope (const GovernanceScope&) = delete;
        GovernanceScope& operator= (const GovernanceScope&) = delete;

    private:
        VitWorkingCopyPersistPolicy& policy;
    };

    /** True while the kernel is running an agent-governed mutation. */
    bool insideGovernedMutation() const noexcept { return governanceDepth > 0; }

    /** True when an auto-persist fired now must be deferred to the version repository. */
    bool shouldDeferWorkingCopyWrite() const noexcept { return governanceDepth > 0; }

    /** Number of auto-persists deferred since the last reset (diagnostics). */
    int deferredWrites() const noexcept { return deferredWorkingCopyWrites; }

    void noteDeferredWorkingCopyWrite() noexcept { ++deferredWorkingCopyWrites; }

private:
    int governanceDepth = 0;
    int deferredWorkingCopyWrites = 0;
};

} // namespace vit
