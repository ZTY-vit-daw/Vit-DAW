# 395-B copy review revision (2026-11-17)

Baseline `d2254f369e1442b13ec00ef8cd9c3bc525f3d28b`, branch
`codex/shell-presentation-395`, owner `/root/window391`.

Only three production files changed. `launcherData.ts` now describes an
operation request being submitted and points to progress below; it no longer
exposes the bridge code id, governance ownership or host implementation in
the directive/card placeholder notes. The current launcher and project
confirmation card descriptions explain the three choices in user terms.
Placeholder titles remain “正在打开/正在新建”; no history row claims success.
No button, protocol, gate, automatic-execution action or styling changed.

Two helper-note assertions in `launcherSurface.test.tsx` now check the user
copy and absence of implementation terms. An added ConvergedLauncher
confirmation case keeps request association and all three button anchors.
The existing ProjectSurface confirmation case checks the new description
while preserving its button assertions. These test edits overlap 389's
completed test migration only at the corresponding expectations; supervisor
combination preserves both migration and copy behavior.

Validation in `agent/webui`:

- `npx vitest run src/launcher/launcherSurface.test.tsx src/project/ProjectSurface.test.tsx`
  final exit 0, 2 files / 48 tests.
- `npm run build` exit 0 (TypeScript + Vite). Production files did not change
  after that build; the final edit only moved the confirmation-copy assertion
  from the wrong titlebar fixture to the actual confirmation-card test.
- `git diff --check` exit 0.

First focused attempt exited 1 because the added confirmation description
assertion was attached to the titlebar test rather than the confirmation-card
case. It was corrected before the final focused run; the original output and
both result records are retained in `raw-copy-logs.zip`. No full unit/render
suite was repeated for this copy-only revision, and no real stack or Godot
process was started.

After the copy commit was already pushed, the supervisor authorized the
adjacent render-script alignment. The 389 checkout was read-only verified at
`0371a1d0ea41e37bd5303d5189f2ba73eb96a8ec`; only its ignored/untracked render
artifacts remained, and the supervisor confirmed no active writer. The shared
`gallery/render-smoke.mjs` assertion and its comment now require “工程操作请求已提交”
and absence of `UI-BRIDGE-1`, retaining automatic state, no manual execute and
no false-success checks. `node --check src/gallery/render-smoke.mjs` exited 0.
No eleven-segment render rerun was performed; the supervisor's final combined
run verifies that script together with 389's new-fixture migration. The
follow-up is an adjacent commit to avoid rewriting the already-pushed copy
commit. Production copy and its focused/build evidence did not change.
