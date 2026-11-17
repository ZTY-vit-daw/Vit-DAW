# 403 B2 — coverage insufficient before restoration

Owner /root/restore403; supervisor /root. B2 authorization in shared main2c44f2e9 was reread with the resource record at15:01:33+0800. RUNBOOK-B2 was frozen before configure. Reviewed runnerf9792767 and probebb2a401 remained unchanged; tested mainHEAD831fe811 includes that runner. B1 original files and ZIP remain intact. No third B candidate or replacement run occurred.

## Execution and provenance
formal_B2 ran from2026-11-17T07:02:12.8822299Z to07:02:34.6835024Z, within15minutes. Configure/build each exited0; host probe28544 exited20 (coverage_insufficient), runner/wrapper exited1. One of two permitted children started; external condition stopped. Full commands, hashes, HEAD/status/diff, owner and hygiene evidence are underformal_B2.

Godot child10448/thread17836 used nonce8d7bbffa50164b65ace552d587618d3f. Native hook preceded explicit CreateProcess; PID was known before ResumeThread. STARTUPINFO flags1/show0 and creation flags134218756 were logged. Direct GUI binary D:/Godot/Godot_v4.6.1-stable_win64.exe has SHA2561E5EFE381F62EE1CEA6BC18CAAC71C0C74BD6E68E6AF5C6EFB3DBE76628F61C7. Runtime observer reports4.6.1 stable official, hash14d19694e0c88a3f9e82d899a0400f27a24c176e. No wrapper, import, version, kernel, agent or product process was launched. The generated project has no autoload/plugin/extension and uses child-only isolated user directories.

Tested hashes are recorded in source_hashes.json and binary_hash.json:
- Runner4629D87DE7FDD72A3AF4E27ADA011F2BE6F15A26743ACE90471A23AD85D10811
- ProbeE44B224A99B11B3B6E28D17269FAC7C230B7142A02A7BC8F4536AB6EB0BFE4DB
- CMake91EC190AD9036994F3B0514127CF4C0BC60EE579C78B59F42E0112548AD44A68
- Fresh probe binary4B02B40957620BA06CCEBEFCD74041422E6F56A31D6FA55EC6776142371CD963
The build has the previously recorded C4244 ASCII nonce conversion warning; no build error. No code changes were made during B2.

## Exact failure and reachability
The BOM correction allowed the scene and observer to load. Godot's configured log contains version/rendering information and no parse error. The three engine rows record scene_ready, first_process_ready and sample with matching nonce/PID10448, both modes1, focused0, size720x480 and native HWND2034330. Raw child SHOW at seq31 points to that same HWND. No child raw foreground event was observed before the stop. This proves startup reached the observer's first process callback; it does not expose the engine's exact first ShowWindow arguments.

The native findChild function instead selects the first EnumWindows result belonging to the child with no owner, then stops enumeration. At seq34 it selected HWND4656690, hidden and noniconic, different from Godot's reported HWND2034330. A first unowned HWND is insufficient to identify the Godot main window. The probe checked whether that selected HWND had a child SHOW event and correctly rejected coverage at seq35; only HWND2034330 had SHOW. The subsequent explicit engine-HWND comparison would also disagree. These are observations of different windows, not evidence that Godot and Win32 disagree about one window's mode.

There are zero safe_hidden_minimized, before_restore or restore_api_return rows: no child hide/preparation or candidate API call was reached. No native restoration, mode0 synchronization or two-second observation exists. The synthetic guard HWND721802/PID28544 retained foreground through the failure. Child10448 was still alive and was then terminated through its exact owned handle: seq36 records exit0 with forced_teardown1, terminate_return1 and wait0. That exit0 is teardown, not application success or natural exit.

All45 raw rows are sequential and end hook_removed. diagnostic_crosscheck_B2.json derives the window mismatch, ready data and zero-call facts from originals. The second marker and recovery interval were not reached, so successful-interval continuity cannot be claimed. Host guard WndProc messages are not child WM messages; no injection or subclassing was used. The Godot script only observes state.

## Hygiene and evidence boundaries
Remote owner fetch and immediate idle checks passed before build and launch. At07:02:34.6809897Z all Vit/Godot/probe processes and TCP5555/5556/7878/7879/8787/9333 plus UDP4445 were absent. Full recents before.json was saved; before/after SHA256 both807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353. No foreign process/window, project, fixture or user configuration was modified. Build binaries and isolated shader caches remain excluded from commits and ZIP.

Complete configured Godot log, raw WinEvent and passive engine observations are preserved. Independent stdout/stderr remain unavailable, as accepted for B2; no complete-standard-stream claim is made. ZIP/SHA preserve original bytes despite Git text normalization, and expanded files remain available. StageA and B1 artifacts are unchanged.

## Can and cannot prove; next gap
B2 proves that the revised generated fixture reaches Godot's first process callback and reports minimized mode1 without focus, with a matching raw SHOW for its native main HWND. It proves that first-unowned-window enumeration cannot establish the native main-window premise.

It cannot prove the real main HWND's native safe-iconic state, hide/restoration, nonactivation after restoration, mode0 synchronization or two-second retention. The outcome is a coverage defect, not an API pass or fail. StageA conclusions remain unchanged. Product391/395 and user handtest are not unlocked.

The B2 budget is closed. A separate future investigation would need to take the native HWND from matching nonce/PID engine readiness, validate IsWindow/GetWindowThreadProcessId, and correlate that exact HWND with raw events and read-only enumeration of all child windows. This report does not implement that change or authorize another run. Supervisor acceptance and resource closure remain pending.
