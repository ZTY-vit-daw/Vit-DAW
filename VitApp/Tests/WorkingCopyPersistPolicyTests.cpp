// B15: true manual-save semantics - the auto-persist gate.
//
// Forensics this pins (artifacts/mantest3/20260911 + kernel log
// VitHeadlessServer2026-09-11_19-53-47.log:52426):
//   * an agent-governed mutation reaches the kernel as a VSP envelope
//     (channel/type/payload) and is unwrapped into a legacy command whose
//     handler finishes with the auto-persist callback
//     (CommandDispatcher::saveProject -> VitHeadlessService::saveProjectToCurrentPathOrDefaultXml);
//   * a Godot UI edit reaches the same handler as a raw command payload with no
//     envelope, which is the user's own in-session edit;
//   * history.Checkpoint hashes project.snapshot_export (in-memory XML) and
//     never touches the live .vit, so the version repository keeps collecting
//     blobs while the working copy stays put.
//
// Four nails, mapping 1:1 onto the card's acceptance list.

#include "../Source/Service/VitWorkingCopyPersistPolicy.h"

#include <cassert>
#include <iostream>
#include <string>
#include <vector>

namespace
{

struct PersistProbe
{
    vit::VitWorkingCopyPersistPolicy policy;
    std::vector<std::string> writes;      // real working-copy writes
    std::vector<std::string> deferred;    // auto-persists routed away from the working copy

    /** Mirrors CommandDispatcher::runAutoPersist(). */
    bool autoPersist (const std::string& origin)
    {
        if (policy.shouldDeferWorkingCopyWrite())
        {
            policy.noteDeferredWorkingCopyWrite();
            deferred.push_back (origin);
            return true;
        }

        writes.push_back (origin);
        return true;
    }

    /** Mirrors VitHeadlessService::ipcSaveEncryptedProjectToPath (explicit save). */
    void explicitSave (const std::string& origin)
    {
        writes.push_back (origin);
    }
};

int failures = 0;

void check (bool condition, const std::string& label)
{
    if (! condition)
    {
        ++failures;
        std::cerr << "FAIL: " << label << "\n";
    }
    else
    {
        std::cout << "ok: " << label << "\n";
    }
}

/** Nail 1 - governance mutation: the auto-persist is deferred, the working copy is untouched. */
void nailGovernanceMutationDefersWorkingCopy()
{
    PersistProbe probe;

    {
        const vit::VitWorkingCopyPersistPolicy::GovernanceScope governed (probe.policy);
        check (probe.policy.insideGovernedMutation(), "nail1: inside governed mutation");
        // The agent's D1 StaticEQ load (rack/instantiate path) ends in the callback.
        check (probe.autoPersist ("instantiate_plugin"), "nail1: governed load reports success");
        check (probe.autoPersist ("set_plugin_param_aliases"), "nail1: governed alias write reports success");
    }

    check (probe.writes.empty(), "nail1: no working-copy write during governance");
    check (probe.deferred.size() == 2, "nail1: both auto-persists deferred");
    check (probe.policy.deferredWrites() == 2, "nail1: deferral counter visible for audit");
    check (! probe.policy.insideGovernedMutation(), "nail1: scope released");

    // The version repository still receives the state: the harness hashes the
    // in-memory snapshot (project.snapshot_export), which is independent of the
    // working-copy gate. Deferral must not mean "the step was lost".
    check (probe.autoPersist ("post-governance raw command") && probe.writes.size() == 1,
           "nail1: repository-carrying path still available after the scope closes");
}

/** Nail 3 - the user's manual save path is unchanged, including from inside a governed turn. */
void nailUserSavePathUnchanged()
{
    PersistProbe probe;

    // Explicit Lifecycle command in a plain session.
    probe.explicitSave ("save_project");
    check (probe.writes.size() == 1 && probe.writes[0] == "save_project",
           "nail3: explicit save_project writes the working copy");

    // Explicit save raised while a governed mutation is in flight must still write.
    {
        const vit::VitWorkingCopyPersistPolicy::GovernanceScope governed (probe.policy);
        probe.explicitSave ("save_project");
        probe.explicitSave ("save_as_project");
    }

    check (probe.writes.size() == 3, "nail3: explicit save path ignores the governance gate");
    check (probe.deferred.empty(), "nail3: explicit save never counted as deferred");
    check (probe.policy.deferredWrites() == 0, "nail3: explicit save leaves the deferral counter untouched");
}

/** Nail 4 - a bare (non-governed) mutation keeps persisting exactly as before. */
void nailBareOperationUnchanged()
{
    PersistProbe probe;

    check (! probe.policy.insideGovernedMutation(), "nail4: a bare command is not governed");
    check (probe.autoPersist ("rack_add_node (Godot UI drag)"), "nail4: bare rack_add_node persists");
    check (probe.autoPersist ("set_tempo"), "nail4: bare set_tempo persists");
    check (probe.autoPersist ("clear_project"), "nail4: bare clear_project persists");

    check (probe.writes.size() == 3, "nail4: three working-copy writes, no regression");
    check (probe.deferred.empty(), "nail4: nothing deferred without governance");
}

/** Nail 2 - reopening lands on the last manual save: the gate never fabricates a write. */
void nailReopenReturnsToSavePoint()
{
    PersistProbe probe;

    probe.explicitSave ("manual save (user)");            // working copy == save point
    const auto savePointWrites = probe.writes.size();

    {
        const vit::VitWorkingCopyPersistPolicy::GovernanceScope governed (probe.policy);
        probe.autoPersist ("governed EQ apply");          // in-memory only
    }

    check (probe.writes.size() == savePointWrites,
           "nail2: the governed step did not move the save point (reopen == save point)");
    check (probe.deferred.size() == 1, "nail2: the governed step is recorded as deferred");

    // Nested scopes (an envelope may unwrap into another dispatcher level) must
    // not leak the governance marker when the inner scope exits.
    {
        const vit::VitWorkingCopyPersistPolicy::GovernanceScope outer (probe.policy);
        {
            const vit::VitWorkingCopyPersistPolicy::GovernanceScope inner (probe.policy);
            check (probe.policy.insideGovernedMutation(), "nail2: nested scope still governed");
        }
        check (probe.policy.insideGovernedMutation(), "nail2: inner scope exit keeps outer governance");
        probe.autoPersist ("nested governed step");
    }

    check (probe.policy.insideGovernedMutation() == false, "nail2: governance fully released");
    probe.autoPersist ("user edit after reopen");
    check (probe.writes.size() == savePointWrites + 1,
           "nail2: post-governance user edit persists again");
}

} // namespace

int main()
{
    nailGovernanceMutationDefersWorkingCopy();
    nailReopenReturnsToSavePoint();
    nailUserSavePathUnchanged();
    nailBareOperationUnchanged();

    if (failures != 0)
    {
        std::cerr << failures << " B15 persist-gate check(s) failed\n";
        return 1;
    }

    std::cout << "VitWorkingCopyPersistPolicyTests: all B15 persist-gate checks passed\n";
    return 0;
}
