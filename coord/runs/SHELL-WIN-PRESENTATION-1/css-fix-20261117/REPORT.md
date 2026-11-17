# 395 CSS compact-sidebar review fix

Owner `/root/window391`, PC. Implementation worktree remains the independent
`shell-presentation-395/Vit_DAW`, branch `codex/shell-presentation-395`, clean
baseline `5bd0be40adc11d332d25a30c0f0b3fd88030d69c`.

The combined checkout `entry-final-validation/Vit_DAW` at
`2f45c5f1f90120ac501c808685d3ae5ea31cc45e` was read only. On inspection it
already contained another owner's edits in
`run_entry_management_acceptance.ps1`, `ui_entry_smoke.ps1`, and
`vitshell_full_smoke_cdp.mjs`; none were changed, staged or committed by this
worker. The supervisor corrected the initial combined-tree write location
before this worker had made any change there.

## Counterexample and change

The original combined render log and launcher JSON are copied unchanged as
`combined-before-render.log` and `combined-before-launcher.json`. The launcher
failure contains exactly three compact sidebar overflow observations:
`scrollWidth=214 > clientWidth=199`. Shadowbox aggregated the launcher failure;
the actual message alignment was not the failed target.

The read-only DOM rule-toggle diagnosis supplied by `/root/cleanup389` and
confirmed by the supervisor established the mechanism: a 200px compact
drawer with 12px padding on each side has 175px of content width. The new
395 direct-child `min-width:190px` forced a wider child and produced the
214px scroll width. Toggling that minimum to zero allowed a 175px child and
`scrollWidth=clientWidth=199`; the regular 280px drawer remained unchanged.
These geometry numbers are the diagnostic owner's measurements, not a
new browser measurement by this worker.

The only production change replaces that direct-child minimum with zero and
documents why. Content can now shrink/wrap to the actual rail width. The
existing overflow rule is unchanged; no new clipping or hidden-text workaround
was added. Collapse dimensions/transitions, visibility, inert/aria behavior,
reduced-motion, controls and message styles are unchanged. No render
assertion was edited or weakened.

## Local verification and remaining combination gate

Commands ran once from this independent worktree's `agent/webui`:

```powershell
$env:VIT_WINDOW_RENDER_OUTPUT = '<worktree>/agent/webui/output/playwright/395-css-fix/render'
node tests/render-window-presentation.mjs
npm run build
```

Both exited 0. The existing production-App render leg passed all 31 checks,
including keyboard sidebar collapse, inert hidden controls, state retention,
reduced-motion and 1024x680 no horizontal overflow. `independent-render.json`,
`render.log`, `build.log`, and `results.json` retain raw evidence. Vite's
existing bundle-size warning remains. `git diff --check` exited 0.

The exact new compact gallery cases are in the supervisor's combined 389/395
tree, not this older independent gallery baseline. The supervisor will merge
this one-file fix and run the unchanged combined eleven-segment render once
into a new directory to establish the red-to-green comparison for those cases.
This receipt does not claim that combined run already passed. No Go tests,
Godot, VitShell, agent, kernel, real-stack HTTP or desktop automation ran.
