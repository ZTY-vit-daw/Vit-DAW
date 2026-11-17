# 395 zero-stack implementation receipt (pending supervisor acceptance)

Owner `/root/window391`, PC, Sol/xhigh. Worktree
`C:/Users/timoz/.codex/worktrees/shell-presentation-395/Vit_DAW`, branch
`codex/shell-presentation-395`, clean claim baseline
`66e951d99f10747809ef10f4f82334b1fd4d678e`. No card status, shared-main,
frontend-combo, 389 domain, or production-runtime write was performed.

The production change is exactly five files: App, ConvergedLauncher,
ProjectSurface, `windowPresentation.tsx` and `windowPresentation.css`.
The App-owned provider retains observations across surface changes. It
registers the host listener before replay handshake, validates versioned
request/sequence events, displays only observed steps, keeps progress unknown,
and distinguishes explicit workspace readiness from legacy/unconfirmed opened.
Failures/cancellation retain honest reasons; retry only reuses the actual
request directive. The existing recent-projects sidebar can collapse via
keyboard with aria-expanded and inert contents. Native DAW foreground only
changes the header; other/none/missing observations remain neutral. The 388
management, settings and retained project input behavior is preserved.

## Commands and observed results

All frontend commands ran in `agent/webui` with an independent `npm ci`
installation; no writable junction or shared node_modules was used.

| Command | Result |
| --- | --- |
| `npm ci` | exit 0; 170 packages installed |
| `npx vitest run src/project/loading/windowPresentation.test.tsx` | exit 0; 9 new cases |
| `npm run test` (final source) | exit 0; 60 files, 722 cases |
| `npm run build` (final source) | exit 0; TypeScript + Vite |
| `node tests/render-window-presentation.mjs` (final attempt3) | exit 0; 31 checks |
| `npm run test:render` | exit 0; every existing segment PASS |
| `git diff --check` | exit 0 |

The independent render used
`VIT_WINDOW_RENDER_OUTPUT=<worktree>/agent/webui/output/playwright/395-attempt3`.
The existing render suite used
`VIT_RENDER_SMOKE_OUTPUT_ROOT=<worktree>/agent/webui/output/playwright/395-validation/gallery`
and `VITE_AGENT_PROXY=http://127.0.0.1:9`. Refused dead-proxy messages in that
suite's unavailable-host cases were retained; it did not contact the running
agent. Its assertions reported FOUNDATION, LAUNCHER, PROJECT, LOADING,
FIDELITY, SHADOWBOX, REC-TAPE, WIDGETS, CONVERGED and ENTRY388 PASS.

`render-report.json` records all 31 production-App checks and request paths.
The browser entry is the real index/main/App, with only HTTP and host input
mocked. It verifies phase/seq/replay association and rejection, late listener,
failure/cancel/legacy/unknown-stage negatives, completed-state retention on
actual launcher unmount/project mount, independent focus facts, management
settings/draft-input roundtrip, in-project directive, unrelated file-dialog
retry prevention, current step inside the thread viewport, reduced motion,
1024x680 no horizontal overflow and no page exceptions/external requests.
New header/ready text contrast minimum is 5.4967:1, with minimum font 13px.

Representative screenshots are `launcher-pending.png`,
`project-ready-daw-foreground.png`, and
`project-failed-reduced-motion.png`. Visual inspection found the first
pending card below the thread's visible area; the final implementation scrolls
the current receipt into that actual thread on state change and the final
render asserts current-step visibility. Finite entry animation was settled
before screenshots; it never drives progress.

Raw final unit/build/gallery stdout and result metadata are in
`raw-final-logs.zip`, SHA256
`083C60B9E2301DCD1C81E040D7B7D868DF56D261817DAE38BBC072B732171C6B`.
The full ignored output directories for attempts1/2/3 and validation remain
in the worktree for supervisor review. Earlier render attempts passed 25 and
30 checks; final31 includes the additional unrelated-retry negative. A local
backslash-regex edit briefly failed TypeScript syntax checking and was fixed
before the final test/build; it was not counted as a passing source version.

## Boundaries

This receipt proves local logic, actual React production assembly and mocked
browser input/output. It does not prove real WebView delivery, Godot show
timing, OS foreground/z-order, kernel snapshot readiness or the full native
journey. No Godot/agent/kernel/VitShell, GUI/computer-use or live-stack HTTP
was run. Existing unit stderr includes jsdom canvas/text-metrics limitations;
Vite retains its existing bundle-size warning. These did not fail assertions
and are not claimed resolved.

The authoritative upcoming combined-device plan belongs to `/root/entry388`
under `coord/runs/SHELL-WIN-BEHAVIOR-1/RUNBOOK.md`. This directory's
`RUNBOOK.md` only identifies protocol consumption and remaining 395
assertions. 271 retains the next runtime slot. Real-stack exit-zero and final
supervisor review remain required before 391/395 delivery or user hand test.
