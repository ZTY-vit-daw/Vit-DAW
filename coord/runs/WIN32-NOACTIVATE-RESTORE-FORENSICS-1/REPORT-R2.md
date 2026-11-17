# 403 R2 self-window evidence report — execution complete, supervisor acceptance pending

R1 formal_01 is preserved unchanged: apparatus failure, exit1, 0/6 window scenes; see REPORT.md. R2 was separately authorized in shared main5d1fd728 and frozen in RUNBOOK-R2.md before launch. No code/criteria changed between reviewed main8cca7d9cf0429499ada9bf01ec9a1e56036351d3 / frontendac3866cf38e4e6c87a4c36fd1e759e7890c3df3f and R2. No merge/cherry-pick of shared main occurred.

## Run and outcomes
Exact runner command is frozen in RUNBOOK-R2.md; full configure/build/native/probe commands and native exits are formal_02/commands.txt. Run formal_02 began 2026-11-17T06:26:52.3803426Z and finished06:27:54.7612015Z (62.38s; within20min). Configure0, build0, all six fresh probe exits0, wrapper/runner0; no replacement runs or code modifications.

| Scene | PID | Target HWND | Baseline FG HWND/PID | Interval ms | Result |
|---|---:|---:|---|---:|---|
| synthetic.restore |7212|1183300|1052260 /7212 owned guard|2016|control_expected_activation|
| synthetic.shownoactivate |22732|4001492|5311874 /22732 owned guard|2000|candidate_pass|
| synthetic.placement |30596|8127672|1248868 /30596 owned guard|2000|candidate_pass|
| external.restore |29320|1379940|394908 /25292 natural external|2016|control_expected_activation|
| external.shownoactivate |13436|4132612|394908 /25292 natural external|2000|candidate_pass|
| external.placement |20048|790150|394908 /25292 natural external|2000|candidate_pass|

Every scene used its own target process and same owned HWND through first show/minimize/hide/recovery. Synthetic guard is solely a test window, not real VitShell. External foreground was naturally present with a different PID; no API writes targeted it. Exactly one restoration API per scene. SW_RESTORE controls each produced one target raw FOREGROUND event within interval plus target WM_ACTIVATE activation messages, and restored normal. They are expected-activation controls, not nonactivation passes.

Both candidate APIs in both foreground conditions restored visible/non-iconic/non-zoomed normal windows, positive rect[240,200,960,680]/client[0,0,704,441], style349110272, exstyle256 (non-TOPMOST). Each had zero target foreground event/activation within interval and preserved baseline foreground throughout all64 samples and raw events. SetWindowPlacement returned1; ShowWindow returned0 (previous visibility, not failure). At hidden-minimized precondition every target had visible0/iconic1/showCmd2/style617545728 and iconic rect[-32000,-32000,-31724,-31955]. At restoration showCmd became1.

## First ShowWindow is empirical, not inferred
Every raw file records startup flags1 (STARTF_USESHOWWINDOW), startup_show0, before_first_show hidden/non-iconic, then the first explicit ShowWindow request4 (SW_SHOWNOACTIVATE), return_previous_visible0 and after-state visible1/non-iconic/normal at seq9. The observed first show therefore did NOT leave the target hidden despite startup_show0. Do not infer a hidden first show from the launcher flag or source. A second explicit show is logged at seq10 before minimization; the recovery API is later and cannot be that first explicit ShowWindow. This supports consumption of the probe's first explicit show, not a general explanation of all STARTUPINFO override behavior or of Godot startup.

## Raw event and message evidence
formal_02/*.jsonl are original unmodified probe logs, including all relevant WndProc messages, decoded WINDOWPOS, snapshots, and raw WinEvent native ticks/delivery ticks. raw_crosscheck.json independently re-parses originals; derived_summary.json is a convenience extraction, not a substitute. Each file's local sequence is contiguous and ends hook_removed after owned-window destruction and drain. Both bracketing own NAMECHANGE markers arrived in all scenes; every observed maximum pump gap16ms, with64 samples and duration>=2000ms. Candidate target WM_SIZE in interval is restored; control raw FG and WM_ACTIVATE are present.

Anchors: synthetic candidate files before_restore seq33, interval103, result104; external candidates before_restore seq20, interval90, result91. Controls synthetic before_restore33/interval112/result113, external before_restore20/interval97/result98. All first_show_consumed anchors seq9. Full decoded events/messages are in raw files; do not replace them with summary assertions.

Instrumentation boundary remains the frozen one: successful WinEvent hook, bracketing callback markers, sequential local logging, bounded pumping and message/sample corroboration. Windows does not expose a global event sequence to this probe; contiguous local seq alone is not an independent proof that the OS could never drop an event. The result supports the declared observation interval and apparatus; it is not a repeated reliability estimate or a guarantee for every environment.

## Provenance and hygiene
R2 source hashes in formal_02/source_hashes.json:
- runner 449DCE00C0D9930E76F4F86BD6E06D602A835DB1961C8DA8353DB69C9F212EF6
- CMake 91EC190AD9036994F3B0514127CF4C0BC60EE579C78B59F42E0112548AD44A68
- probe C5B3655CBDB11A9A217EE1A9C742DEC19FDFAE8E1830B8F34DCD8EA4F11920CF
Fresh R2 binary SHA256 BF338E5F65A34106D21D3CA9679A08AD67586CF6D8EADF97D7B87FF2E7154824, in binary_hash.json. The earlier compile-validation binary hash differs because R2 built a fresh executable; source hashes/revisions match. Binary/build caches are intentionally excluded from evidence commit. Both tested HEAD/status/diff snapshots are captured in formal_02; main only had untracked artifact root, frontend clean.

Remote resource-owner reread and immediate process/port checks preceded build and each probe launch; all fetch exits0. At teardown 2026-11-17T06:27:54.7597016Z: Vit/probe processes=[], TCP5555/5556/7878/7879/8787/9333=[], UDP4445=[] (after_teardown.idle.json). User recents before/after SHA256 both807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353. No process takeover, kernel/agent/Godot/product start, external-window write, project/fixture/config operation, CUA/keyboard/screenshot, or coordination-state edit occurred.

## Decision boundary
Evidence supports both candidate API paths for one owned-window scene per foreground condition on this PC. SW_SHOWNOACTIVATE is a direct single-API candidate for a separately authorized independent Godot experiment; placement provides no stronger conclusion here. It is now reasonable for supervisor to consider independent Godot-only restoration forensics, with fresh scope/budget and first-show/foreground/state evidence. No Godot run was authorized or performed in R2. This report does not unlock391/395, product modification, user handtest or end-to-end acceptance. Final acceptance and resource release remain supervisor decisions.

Evidence staging note: ordinary git diff --cached --check returned2 solely for preserved native-output trailing spaces/blank EOF in raw R1/R2 logs and the original R1 runbook/diff. Raw files were not trimmed to make the check green. Production/source diff check remains0; evidence check with only blank-at-eof/blank-at-eol diagnostics disabled returns0. Repository * text=auto may normalize line endings in Git; EVIDENCE_SHA256.json records original working-file bytes, and original raw working files remain on this PC.
