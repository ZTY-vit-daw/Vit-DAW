# 406 runtime_01 — external observer path passed; synthetic condition not exercised

Execution owner /root/restore403; supervisor /root final acceptance pending. Shared-mainc6f3aa4b/card/resource read before RUNBOOK-runtime freeze. Reviewed frontend1a4ff1c and main script2d7eb8f2 remained unchanged; tested mainHEADe5f65625. No code/criterion modification, replacement run or additional condition occurred.

## Overall outcome and count
Configure/build exited0; runner/wrapper exited2 (coverage incomplete), not an overall PASS. Run2026-11-17T10:36:26.2905468Z–10:37:18.7414908Z, about52.45s within20min. Two probe host attempts, exactly one actual Godot child. No kernel/agent/product/user project or new geometry strategy ran.

| Condition | Probe PID | Godot child | Exit | Outcome |
|---|---:|---:|---:|---|
| synthetic |25808|not created|10|not_exercised: required owned-guard foreground absent at precondition check|
| external |19504|21460|0|candidate_pass: exact observer/restore/unload/natural-exit path completed|

Synthetic raw seq12 records baseline HWND10684046/PID30472, rather than the self-owned guard; seq13 returns not_exercised with child_pid0 beforeCreateProcess. Guard creation/show/foreground attempt did not leave the required foreground precondition at the check. SetForegroundWindow return/reason is not separately logged, so do not invent a focus-policy/user-action cause. The reviewed runner treats code10 as unavailable and continued to external; no synthetic child or retry was used. Supervisor should judge sufficiency with this explicit missing coverage, not convert it to synthetic PASS or manufacture a replacement budget.

## External identity, loading and recovery
Actual child21460/thread8356, exact Engine HWND1771826/generation1, nonce1b2b5230615141c884c2be9863e43799. Parent raw hook preceded suspended creation; the child set its own testenv and used official GDExtensionManager.load_extension. Engine records observer_load_result0/observer_loadedtrue and matching nonce/PID/native HWND. Read-only inventory and original CREATE/SHOW match accepted; no helper-first misbinding.

Native safe_armed at tick112387625 has childPID21460/TID8356/HWND1771826/generation1, oktrue/installedtrue/failures0; parent independently accepted it atseq32 before any recovery. preinit_coverage remainsfalse. Child was iconic and engineWindow/Displaymodes1, nonTOPMOST, with natural external baseline10684046/PID30472 retained and no child foreground event.

Only existing404 preparation/hide+singleShowWindow(SW_SHOWNOACTIVATE) ran. Immediate normal/noniconic/visible/nonTOPMOST/positivegeometry and engine mode0 synchronization within47ms were observed. Parent intervalseq104,61 samples, dual markers2/maxpumpgap16ms, >=2s stable loop; all samples retain native normal/engine0/focus0 and baselineFG. Raw child foreground count0. No405 work-area resize/z strategy was added and no product-size claim follows.

## Target-native WM evidence and independent pairing
Original self-process output is external.godot.project/transition_observer.jsonl, separate from the parent's Winevent/guard-WM stream. It contains54 continuous native recordseq: initialsnapshot1,26 entry/afterDefSubclassProc pairs52, remove-evidence1. Independently reconstructed LIFO stack has no mismatch/leftover and maxdepth2. All native/lifecycle rows match nonce/PID21460/TID8356/HWND1771826/generation1; failures/dropped/IOerror remain0. Exact message counts and the pairing/identity/normal-state crosscheck are in independent_native_crosscheck.json.

Observed messages include WINDOWPOSCHANGING/CHANGED, STYLECHANGING/CHANGED and nested nonclient/paint/system messages. Maxdepth2 is a measured nested-message case, not a claim that every synthetic nested-geometry failure was forced in runtime; those remain pure-policy counterexamples. Pending_changes briefly1 across a safe-point drain while awaiting the laterCHANGED, and returns0 before release; no unresolved transition atshutdown. Drain_begin/end and native timestamps preserve the distinct capture-vs-write chronology; local native seq applies to native_message records, lifecycle rows carry seq_boundary.

This establishes real target-thread User32 callback entry/downstream-return observation for this one condition. It does not cover beforeSCENE installation, prove OS global event-loss absence, or equate WM events to the first DWM composited pixel frame. Parent guard messages are not misrepresented as child messages.

## Natural graceful stop and removal
Parent stop_request seq105 usedCREATE_NEW with exact noncebody, reused0/ok1. Fixture_process matched nonce, calledunload_extension, read own native removed_verified/shutdown and wrote unload_ack.json. Ack nonce/PID/HWND matches, nonce_matchestrue/unload_result0/still_loadedfalse/removed_verifiedtrue/shutdowntrue. Native cleanup records show depth0/pending0/failed0/drop0/IOok, installedfalse; message-only drain window/class removedtrue. Parent graceful_teardownseq116 confirms all checks, actual child exit0 and natural_exit1. owned_child_exitseq118 confirms forced_teardown0/wait0. No TerminateProcess was needed for this child and no forced0 is counted as success.

The actual successful unload closes this observer instance's callback/helper lifecycle; it is not evidence that hypothetical wrong-thread/remove-failure paths were dynamically exercised. Pure tests cover those policy branches. Engine was not asked to restore another window or mutate production state.

## New DPI observation relevant to405, not a new geometry result
For this exact child/handle, target UI-thread native recordseq38 reports DPI168, normal outerrect[908,428,1652,972] (744×544), client720×480. The same-window parent after-call snapshot reports outer425×311/client411×274; engine logical size remains720×480. This measured1.75-scale contrast supports the405 hypothesis that prior “small” readings were from differing DPI observation contexts. The recorded DPI context43025 is opaque; no awareness enum is invented from that numeric handle. This is not proof of a new work-area target, DPI conversion contract for all monitors, or first-large-visible presentation;405 still needs its own scope/decision.

## Provenance and hygiene
Exact frozen outer command is RUNBOOK-runtime.md; all native configure/build/probe commands and exits under runtime_01/commands.txt. Main/runtime HEAD/status/diff/source snapshots and ABI/DLL hashes retained. Direct GUI Godot4.6.1 official hash14d19694e0c88a3f9e82d899a0400f27a24c176e, binarySHA1E5EFE381F62EE1CEA6BC18CAAC71C0C74BD6E68E6AF5C6EFB3DBE76628F61C7. Runtime observer DLLSHA21B349E80244461A7ECB015C3455761D044364379ECF1C92E10EB2AA85E17447; probebinarySHA AFFBD8535B22587A6492DD42B7350EA2BFCADD4F7BC36C5FAEE42F8FB98EFFE6; officialABI headerSHA D21AD8AB1362BE6C10E656E19A5DD8A5EC742154991F713D16BCA325F460C660. DLLcopyhash and source hashes are inobserver_hashes.json and fixture manifests; no production DLL was loaded. Existing old-probe C4244 warning remains in build, no buildfailure.

Current406 exact-owner reservation was fetched and matched before build and both host attempts, with immediate process/port checks; longbuild followed by recheck. Final10:37:18.7395753Z snapshot: allVit/Godot/probeprocesses=[], TCP5555/5556/7878/7879/8787/9333=[], UDP4445=[]. Recents fullbefore.json saved; before/afterSHA both807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353. Original source/engine/native/parent/loader/stop/ack/exit logs plus byteZIP/SHA preserved; binary/build/shadercache excluded from commits/ZIP and expanded originals remain. No other processes/windows/configs were controlled, no CUA/input/screenshot orLLM.

## Supported and unproven
Supported: external foreground minimalGodot official-loader/self-UI-thread observer, native entry/after nested recording and pairing, original404 safe restoration, and actual natural unload/removal/exit0 closed-loop. Missing: synthetic-owned-guard condition. Overall exit2 is retained. Native no-error evidence does not erase unexercised coverage.

405 first-large-normal geometry/z, productionbootstrap/projectready/395/complete matrix and user handtest remain unvalidated. No new product behavior or production mechanism was implemented. Runtime budget is exhausted/stopped without further attempts; supervisor owns acceptance, any next scope and resource release.
