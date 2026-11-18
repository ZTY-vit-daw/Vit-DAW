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
