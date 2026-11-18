# 409 product acceptance plan: shell condition, one attempt

Prepared 2026-11-18 12:48 +08:00. This plan alone is not a runtime lease. Start only after the supervisor reviews the two smoke corrections, full staged-project import succeeds, the shared PC reservation is confirmed remotely, and immediate clearance passes.

- One shell-condition attempt, at most 10 minutes including teardown. No unchanged retry, no automatic external leg. Any product assertion, harness failure, crash, missing identity, natural-condition miss or infrastructure failure stops this batch and preserves its evidence. Zero model calls in the product leg; nonempty agent LLM telemetry fails the zero-model claim.
- Success requires script exit 0, the classifier's defined native/message/polling assertions, project DOM and agent state, isolated process paths, normal shell exit, no active process/port residue, fixture-source integrity and hash-proven recents/authority restoration. `not_exercised`, `coverage_gap`, product failure and environment/harness failure remain distinct. A pass proves one path within the stated coverage; it does not prove stability, sampled gaps, rendered frames, export-template compatibility or the 391 hand-test gate.
- Source frontend `2d92d55` plus 16 preserved candidate paths; source main `90b42269` plus the four smoke files. The raw source state and hashes must be saved at execution. Stage is an archive of frontend HEAD with all 16 paths overlaid and the fresh Debug DLL; source tracked deployment binaries are untouched.
- Stage: `%TEMP%/shell409-product-stage-8c5d3d76b53c41258fbe600c7ab61313`; Godot project is its `frontend/`, runtime is its `runtime/`, host is its `host/`. `staging_manifest.json` records initial artifact hashes. Fresh extension SHA-256 `8323F43BFD6CD6C91404DDD120B6B10BA38FEFE27A34B8DFA256BC4678927EF4`; VitShell SHA-256 `0A9541F823E76D7B6C1105764AE1CAF9C4CC40AFCFB83F0A02DCF3D61770D73D`.
- Read-only native snapshots after initial `ShowWindow` and after any single correction are separate. Message counters cover the installed own-thread subclass interval; 10ms polling covers the surrounding sampled interval and must disclose gaps. No desktop input, activation forcing, global hooks or injection. Existing product startup focus behavior is part of the product path.
- Fixture source remains `D:/Vit_DAW/temp/f3_empty_project.vit`; the script copies it before opening. Snapshot recents before the real kernel, restore in `finally`, verify hashes even on failure. Close and cleanup only this run's exact PID/birth identities. A dirty teardown keeps the lease owned until addressed.

Intended command, after independent reservation and final preflight (not executed by this document):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:/Users/timoz/.codex/worktrees/shell-zproduct-smoke-409/Vit_DAW/scripts/vitshell_zproduct_smoke.ps1 -RuntimeAuthorized -LeaseOwner '/root/01a1ece5-c755-7853-b14a-ee0e4058b636' -Condition shell -FrontendWorktree C:/Users/timoz/.codex/worktrees/shell-win-zproduct-409/vit-daw-frontend -GodotProjectStage C:/Users/timoz/AppData/Local/Temp/shell409-product-stage-8c5d3d76b53c41258fbe600c7ab61313/frontend -StageRoot C:/Users/timoz/AppData/Local/Temp/shell409-product-stage-8c5d3d76b53c41258fbe600c7ab61313 -ExpectedExtensionSha256 8323F43BFD6CD6C91404DDD120B6B10BA38FEFE27A34B8DFA256BC4678927EF4 -RunRoot C:/Users/timoz/.codex/worktrees/shell-zproduct-smoke-409/Vit_DAW/coord/runs/SHELL-WIN-ZPRODUCT-1/runtime_shell_01 -TimeoutSec 120
```

Set a run-private `WEBVIEW2_USER_DATA_FOLDER` in the launching shell so the product child inherits it. Preserve the full command, wrapper exit, `d1_smoke_report.json`, raw logs, native samples, classifier input/verdict, authoritative before/after hashes and teardown clearance. The supervisor must inspect the actual evidence before releasing the reservation or deciding another condition.

## Corrected harness attempt, 12:59 +08:00

Attempt `runtime_shell_01` stopped before any product process: script exit 2 because the lease parser rejected a reservation's teardown prose containing the word `释放`. The outer metadata wrapper then hung and was terminated at its exact PID/birth; its exit is -1 and the original artifacts are preserved. All six authoritative hashes were equal and the supervisor independently confirmed zero processes/active ports before releasing the first reservation (`08f49b2c`). No product behavior was exercised.

The parser now validates the timestamped event header's action separately from its prose. Nine offline cases using the captured actual reservation plus wrong-owner/release/no-start cases pass. Corrected script SHA-256 `C5B4B1EE91E66B66BE1A6F51C817EC1DC9E14AEA8203663710320B0B4ED9C6CF`. This is a changed harness, not an unchanged retry.

Authorize one corrected shell-condition attempt only, after its new shared reservation: owner is supervisor `/root/01a1eca3-e329-73f2-b43a-390e9fafbd30`, running the already independently implemented code for acceptance. Use `StageRoot C:/Users/timoz/AppData/Local/Temp/shell409-product-stage2-436f76ac63244f0f93a3ca6860e7c431`, its `frontend/` as Godot project, and fresh `RunRoot C:/Users/timoz/.codex/worktrees/shell-zproduct-smoke-409/Vit_DAW/coord/runs/SHELL-WIN-ZPRODUCT-1/runtime_shell_02`. The product source, DLL and host hashes are unchanged. This fresh copy retains imported caches and copied dependencies from the unused first stage, excludes its log override, and has fresh staged runtime timestamps. Call the script directly with plain-text command/exit capture; do not reuse the recursive metadata wrapper. All other success criteria, one-attempt limit, stop conditions and coverage boundaries remain as above. External remains unauthorized.

## Launch-contract correction, 13:05 +08:00

Attempt 02 started VitShell but did not start agent/kernel/Godot or exercise DAW display. The harness quoted flag names; the actual host's `ParseArgValue` ignored them, leaving the agent path empty. The harness also omitted the error-state body and treated unavailable shell ExitCode as a failure with a blank code. The reservation was released after independent clearance and verification of all six authority hashes (`670a1029`).

The changed harness leaves flags bare and quotes values, rejects embedded quotes/newlines, uses the normal visible product-host startup, saves the complete error/last status, pins the process handle and obtains an explicit numeric exit after waiting. Offline tests against the actual host parser prove old failure and acceptance of four spaced-path arguments; PS5.1 hidden ordinary child exits 0/7 are captured correctly. Script SHA-256 `C79D1EBD377929404D19A034CB93EB8A5A71E4F009629118F6B8B78AE332FECD`.

After a new reservation, allow one shell-condition invocation by the same supervisor owner, `StageRoot C:/Users/timoz/AppData/Local/Temp/shell409-product-stage3-444504f9831740cbb593cfc5b67d94f0`, project its `frontend/`, fresh run `runtime_shell_03` in the same smoke evidence directory. The product code, DLL and host remain unchanged; all previous limits and stop conditions apply. No external run or unchanged retry is authorized.

## Runtime readiness correction, 13:16 +08:00

Attempt 03 reached stage agent+hub and WebUI ready, then the harness prematurely required all three runtime processes before the asynchronous kernel spawn. The host exited normally with code 0; independent cleanup and all six hashes passed (`88e981f8`). The source contract confirms `Supervisor.Start` runs asynchronously, ensures hub first, then kernel (`agent/internal/runtimehost/runtimehost.go:351`).

Changed script SHA-256 `C3FECF67F57346B41F792475AF7D6F4F015349F89FB94669BD47D681E78311CA` now waits within TimeoutSec for all three owned stage processes and their listeners, rejects foreign/duplicate/unknown PID/birth/endpoint ownership immediately, and saves lightweight readiness and missing-item evidence on timeout. Offline progression and negative checks pass. Product/DLL/host hashes remain unchanged.

After a new shared reservation, allow only one `runtime_shell_04` invocation by the same supervisor owner, using fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage4-033a09f298834fb58eaf20f2a718da36` and its `frontend/`. Keep all previous success conditions, one-attempt limit, time bound, hygiene and stop rules. External remains unauthorized.

## UTF8 and child teardown correction, 13:21 +08:00

Attempt 04 proved all three stage runtime processes and owned listeners ready, but PS5.1 decoded the valid UTF8 Chinese launcher JSON as ANSI and stopped before the open action. The shell exited 0; a still-exiting owned child was immediately forced, so the final report retained environment failure. Independent clearance and six authority hashes passed; reservation released (`d60a3022`).

Changed script SHA-256 `E7E1571C45E34A95E4219B41DD5CF219353D57EBE9F7A792587C9BBE68329C92` reads JSON/Godot logs explicitly as UTF8 and waits within the existing 20-second cleanup deadline for all exact owned PID/birth identities. Residual forced cleanup still fails. The original run04 JSON parses correctly and delayed-exit/timeout/PID-reuse offline cases pass. Allow one new `runtime_shell_05` after reservation, using fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage5-601925cf1c0e42a6b54ff87968e7a0cb`, same supervisor owner and unchanged product artifacts. Previous success criteria, limits and no-external/no-unchanged-retry rules remain.

## Independent external condition, 13:24 +08:00

Attempt 05 completed full runtime/launcher DOM and normal cleanup but the natural foreground was Edge (PID27352), not VitShell (PID20040). It correctly exited 3 `not_exercised` before the open action; shell exit 0, zero LLM, all six hashes and independent clearance passed. The shell case is still uncovered and is not a product PASS.

The supervisor now separately authorizes the already card-scoped external condition, rather than forcing shell foreground or repeating the same absent condition. One fresh `runtime_external_01`, same script/product hashes and supervisor owner, stage `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage6-61ef83da76fc4148a9e7fc53f11e24f2` and its `frontend/`, `-Condition external`. Require the naturally observed external HWND/PID to remain unchanged, all native/DOM assertions and hygiene. This changes the condition, not the criteria; both conditions remain required for full 409 acceptance. No further run is automatically authorized.

## Bound log capture and user foreground setup, 13:33 +08:00

The external attempt actually opened the project and passed DOM/state; native output landed in the default Godot log, so the original script exited 4. Correlated supplemental evidence passed the unchanged classifier (854 samples, maximum gap 16ms, native first show below reference, zero activation/focus messages, no correction); this does not rewrite the original exit. Independent cleanup passed and the reservation was released (`b358387e`).

Changed script `D7B5CD0319F15B32A744894D61A4E6AB67CAA387EC46DB2226F779C2E5FF3576` snapshots the default log before open and captures raw output with changed-hash, current Godot/reference PID+HWND and monotonic-tick bindings, rejecting stale/unbound evidence. Override-path root cause remains unresolved and is disclosed. It adds a read-only, bounded wait for the natural foreground. Offline acceptance/rejection checks pass; production code is unchanged.

The user explicitly agreed to bring the next VitShell window to foreground for the automated assertions, not to perform functional hand testing. After reservation, allow one shell invocation `runtime_shell_06`, same supervisor owner, fresh stage `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage7-482398c68f6547559eb9fda612cab54e` and its `frontend/`, `-NaturalConditionWaitSec 60`. Missing natural foreground remains exit 3. Keep all criteria, hygiene, 10-minute total bound and stop rules. No further run is automatically authorized.

## Final external condition, 13:36 +08:00

Shell06 script/wrapper exit 0, host exit 0, zero LLM, 792 samples, native initial ordering correct with no correction, zero activation/focus messages in the installed interval, 16 DIP geometry at 168 DPI. Independent process/active-port clearance and six authority hashes passed. This is one successful shell path within the defined coverage, not a stability claim.

Separately allow one external invocation `runtime_external_02` using the same final script/product hashes and supervisor owner, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage8-500b2f8ce8c44a129c72180d0475f1f1`, its `frontend/`, `-Condition external -NaturalConditionWaitSec 20`. The user need not interact with VitShell. Require an actual independent natural external foreground and unchanged identity, plus all frozen assertions and hygiene. Stop on any anomaly; no automatic additional run.

## Explicit external foreground setup, 13:39 +08:00

External02 exited 3 before open: its natural foreground was the VitShell itself (PID5340). No product assertion failed; host exit 0, no teardown error, zero LLM, independent clearance and six hashes passed. The user is now instructed to switch naturally back to Edge after the next product window appears, using the already agreed foreground setup cooperation; assertions remain automated and no agent desktop control is used.

Allow one external03 with the changed explicit foreground setup, 60-second read-only condition wait, same final script/product/owner, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage9-6afacba69fb44d85be224cdfe5702f88` and its `frontend/`, fresh `runtime_external_03`. This is not another run against the same unfulfilled foreground setup. All criteria and stop limits remain; no automatic additional run.

## Confirmed live external setup, 13:43 +08:00

External03 remained not exercised and exited 3 before open; independent clearance and all six authority hashes passed. The user has now explicitly replied “现在开始，我保持 Codex 前台”. After the new host appears, the user will bring the existing Codex window back to foreground; no automated focus control is allowed.

Authorize only one external04, final script/product hashes and supervisor owner unchanged, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage10-583d6a6ead3647d8b110fa5306048a83`, its `frontend/`, fresh `runtime_external_04`, read-only natural-condition wait 60 seconds. Criteria, hygiene and stop limits remain unchanged; missing condition still exits 3 and no automatic further run follows.

## User explicitly requested another setup attempt, 13:49 +08:00

External04 did not reach an independent foreground and exited 3; normal host exit, zero LLM, independent clearance and all authority hashes passed. Runs stopped and the reservation was released (`ef6e34cf`). Asked about desktop visibility, the user explained “刚才没在看桌面” and explicitly requested another attempt. No desktop/session defect is established by the missing condition.

Authorize exactly one external05 for that request, with the same frozen criteria, final script/product hashes and supervisor owner, 60-second read-only condition wait, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage11-1465f91b9cf64a479fde2d24fffcdc10`, its `frontend/`, fresh `runtime_external_05`. The user will naturally select Codex or the browser after the host appears; all assertions remain automatic. Stop on any anomaly and do not automatically retry.

## Bound stable Codex setup, 14:01 +08:00

External05 opened successfully with native first-show/geometry/guard correct and no sampled Godot foreground/focus or activation/focus messages; the unchanged external-foreground assertion failed because multiple other windows became foreground. Original exit 1 remains. The user reports one switch to Codex and a WeChat update popup/confirmation during this interval; no crash or script error is in the native log, the host exited 0 and all cleanup/hashes passed (`c55fc851`). The foreground failure does not establish that the DAW stole focus.

Changed script `11289805B11CCDA874FBA90907212C7896D09FA16343163A97FCE72BC45CBC12` adds optional `ExpectedExternalPid` (default 0 retains the existing behavior). When set for the external condition, the same independent HWND/PID must remain natural foreground for two seconds before the unchanged observation/open assertions begin. Transient/zero/owned/other-PID windows reset readiness; original run05 transition replay and rejection cases pass offline.

The user explicitly replied “现在补测，我保持Codex前台”. Allow exactly one external06, supervisor owner unchanged, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage12-0d4418371bc24932a66f8a98af321422`, its `frontend/`, fresh `runtime_external_06`, `-ExpectedExternalPid 26840 -NaturalConditionWaitSec 60`. PID26840 is verified as the current ChatGPT/Codex UI process; product/DLL/host and all functional criteria remain unchanged. No automatic further run follows.

## Prompt after actual readiness, 14:10 +08:00

External06 exited 3 without opening because the expected UI PID was not foreground during the preparation interval. Host exit 0, no teardown error, independent clearance and six authority hashes passed. A later read-only observation confirmed the actual Codex foreground PID26840/HWND460986, so there is no evidence of a different-session or wrong-PID defect. Earlier notices preceded actual process readiness and did not establish aligned user setup.

Only the parameter validation maximum was changed from 60 to 180 seconds (default 0 unchanged), script `8C1AD902CD9282251997127AB99636CA2620779138DB4F027EB002CF05DBBF35`; no production code or functional predicate changed. The supervisor will start first, verify that runtime/launcher DOM readiness artifacts exist, then prompt for natural foreground. Allow one external07, same owner/product/criteria, fresh `C:/Users/timoz/AppData/Local/Temp/shell409-product-stage13-976ec685a51d4a52a2045b59f5eef09d`, its `frontend/`, fresh `runtime_external_07`, `-ExpectedExternalPid 26840 -NaturalConditionWaitSec 180`. Total run remains bounded to ten minutes; no automatic further run.
