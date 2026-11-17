# 395 window presentation verification

Run from `agent/webui` in its own checkout (no writable dependency junction):

```powershell
npm ci
npm run test
npm run build
$env:VIT_WINDOW_RENDER_OUTPUT = Join-Path (Get-Location) 'output/playwright/395-review-run'
node tests/render-window-presentation.mjs
```

The independent render script starts Vite on a random local port and headless
Chromium. It loads the real `index.html -> main.tsx -> App` production tree;
the loading card, launcher, project page, provider and sidebar are production
components. Every `/agent/**` request is intercepted and fulfilled with a
fixture. Host input is injected through the same `chrome.webview` message
listener API, with captured outgoing bridge requests. It does not start or
contact VitShell, Godot, VitApp, agent, native dialogs or the running stack.
It creates no new gallery entry and does not edit the shared gallery runner.

The script verifies request and sequence association, duplicate/old rejection,
late replay with a listener registered before the handshake, unknown stage,
legacy acceptance, failure, cancellation, explicit workspace readiness,
input gating, file-dialog failure with no unrelated retry, actual App surface
switching, management/settings/input preservation, independent OS focus,
keyboard sidebar collapse, inert hidden controls, reduced motion, minimum
native viewport width, visible current steps and new-text contrast. Screens,
HTTP request paths and the 31-check report are written to the output directory.
Fixture names are marked; the successful report is not real-stack evidence.

The nine reducer tests cover schema/shape validation, timestamp ordering,
known phase mapping, unknown progress, <=3 steps, request/seq/replay rules,
legacy/unknown opened results, terminal failure/cancel and next-request reset.
No clock drives a progress value. Progress remains `null`; the existing
LoadingCard handles its unknown tape frame.

The App provider is retained across launcher/project surface changes and the
existing project page remains mounted when the management view hides it.
Request events and OS window observations are separate: `release_reason` is
not a result; missing native state is unknown. `vit_launcher_result opened`
without a matching explicit `workspace_ready` progress event is unconfirmed.
`scene_accepted` remains a limited accepted transition. Missing steps are not
filled in. The sidebar is the existing recent-projects slot; the future 393
conversation list is not implemented by this change.

Native contract reference: frontend combo commit
`7bd812f1bf2042bc349bdf32c2f734bbb5309c4c` (`VitShell/tests/README.md`).
Register listener first, then send
`ask_vit_shell/window_presentation_ready` with a nonempty subscription id.
Accept matching replay wrappers, known `vitshell.launch_progress.v1` phases,
request id and monotonically increasing bridge-instance sequence. The native
fresh replay supplies window state separately. There is no browser-focus
fallback, native polling, percentage timer, or extra Go event protocol.

The existing full render suite can also run without touching a real stack:

```powershell
$env:VIT_RENDER_SMOKE_OUTPUT_ROOT = Join-Path (Get-Location) 'output/playwright/395-gallery-review'
$env:VITE_AGENT_PROXY = 'http://127.0.0.1:9'
npm run test:render
```

The dead proxy ensures its deliberately unavailable-host checks cannot reach
another owner's agent. Expected refused-proxy messages are retained in the
raw log; its own report assertions remain authoritative for that suite.
The real-stack combination runbook is in
`coord/runs/SHELL-WIN-PRESENTATION-1/zero-stack-20261117/RUNBOOK.md`.
