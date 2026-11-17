# 403 execution report — BLOCKED before window execution

Owner /root/restore403; final acceptance reserved for supervisor /root. No claim of API semantic success or product acceptance.

## Revisions and file boundary
Main e30319e64ef1f22bb024e88f0f88b64fba7b6b54 -> 8cca7d9c (scripts/vitshell_smoke.ps1 only).
Frontend d05043e11a79452b9155a89f3405668994eafdcb -> ac3866c (VitShell/tests/CMakeLists.txt and win32_noactivate_restore_probe.cpp only).
Both implementation branches codex/win32-restore-forensics-403 pushed to their own origin; no main push, no production/coord-card/resource edits. Frontend final status clean; main only this untracked artifact directory. Build outputs remain untracked under compile_validation_01/build.

## Frozen attempt and failure
RUNBOOK.md was written before execution. The single frozen runner invocation was:
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/vitshell_smoke.ps1 -Win32RestoreForensics -FrontendWorktree C:/Users/timoz/.codex/worktrees/win32-restore-403-fe/vit-daw-frontend -RunRoot C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WIN32-NOACTIVATE-RESTORE-FORENSICS-1/formal_01
Wrapper/runner exit=1, began 2026-11-17T06:17:01.7943649Z, finished 06:17:11.6328374Z.
Exact failing operation (original script line56):
& git -C $MainRoot fetch origin main 2>&1 | Out-File (Join-Path $RunRoot "$Label.fetch.log") -Encoding utf8
Windows PowerShell 5 with ErrorActionPreference=Stop promoted native stderr `From github.com:ZTY-vit-daw/Vit-DAW` to terminating NativeCommandError before checking LASTEXITCODE. The same original helper failed again in finally. The raw original errors are formal_01/failure.txt and teardown.failure.txt; neither was replaced.
The failed fetch native exit was not observed before the pipeline terminated; do not assign it a made-up exit0. A later read-only fetch through the corrected helper returned0.
No configure/build or Start-Process branch was reached in formal_01. scenes=[] in its report; no scene.process.json, raw scene JSONL, build tree or probe binary exists there. Thus zero of six formal scenes started, and no probe launch evidence exists. Historical OS-wide process creation auditing was not enabled, so the statement rests on preserved control flow and artifacts rather than an independent historical OS audit.

## Repair and permitted verification
Supervisor explicitly stopped all further window execution and allowed stderr repair plus compilation/static verification (message in current chat). No replacement runner or window scene was invoked.
Native subprocess capture now handles Windows PowerShell stderr under a function-local Continue preference and checks actual native exit. Parent Stop preference is preserved. Corrected script also records exact created PID/start time before wait, retains its process handle, and never kills by enumeration.
PowerShell parse=0; staged git diff --check=0 in both repositories.
Regression check extracted only Invoke-403Native via PowerShell AST (no script top-level execution), then ran:
cmd.exe /c "echo benign-stderr 1>&2 & exit /b 0" -> helper return0
cmd.exe /c "echo failing-stderr 1>&2 & exit /b 7" -> helper return7
Artifacts: compile_validation_01/native_stderr_exit{0,7}.log and static_and_native_helper_checks.json (parent Stop enum serialized as1).
Standalone cmake configure=0 and cmake --build --config Release=0. Full commands/exits in compile_validation_01/commands.txt; build log has no compiler warnings/errors. This compiled only the standalone owned-window test, without launching it. Final source/binary SHA256 recorded in compile_validation_01/final_hashes.json; formal initial-source hashes preserved separately in formal_01/source_hashes.json.
Two git commits, git show/status and two branch pushes all exit0. Final git evidence in final_git_evidence.txt.

## Safety and evidence boundary
Read-only postfailure resource fetch=0; top record still owner /root/restore403. 14:19:57+0800 process snapshot: Vit/probe processes=[], TCP5555/5556/7878/7879/8787/9333=[], UDP4445=[]. Artifact compile_validation_01/post_failure.idle.json. No process was stopped because none was created by this attempt. No kernel/agent/Godot/product, project or fixture was started/opened/written.
User recents before/after/postfailure SHA256 all 807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353. No user config write/restore was performed.
Buildability and the native stderr repair are supported. First ShowWindow semantics, hidden-minimized restoration, activation/foreground preservation and event continuity remain UNEXERCISED; no API candidate gets PASS or FAIL. The code's first-show logging is preparation only, not empirical first-show evidence.
Do not proceed to independent Godot yet. Suggested next supervisor decision: inspect the two commits, preserve formal_01 as apparatus failure, and separately register any renewed self-window experiment budget before execution. No new budget has been granted. 391/395/user handtest remain locked by their existing decisions.
