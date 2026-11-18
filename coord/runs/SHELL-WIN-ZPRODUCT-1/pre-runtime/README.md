# 409 pre-runtime evidence

No product stack or product window was started for these checks. This directory contains byte-identical copies of the raw logs referenced by the stage reports.

- Godot 4.6.1 isolated scene-tree probe: synchronous add from `_ready` exits 32 (`tree-immediate.log`); deferring to `process_frame` exits 0 and records `SCENE_CHANGED_PERSISTED=true` (`tree-deferred.log`).
- Official Godot 4.6.1 isolated extension-load probe exits 0 (`godot46-extension-load.log`). It registers and instantiates the candidate guard and recognizes the custom feature without calling native window methods. Tested DLL SHA-256 `6C7852D75B2B4E7FC6C64911CFB6FF44239E3E9C735E7AF497D28455F6DBA595`.
- Narrow VitShell rebuild and selected no-window CTest both exit 0 (`narrow-vitshell-build.log`, `narrow-ctest.log`). Win32 resume failure was reviewed, not fault-injected.
- Smoke worktree `shell-zproduct-smoke-409/Vit_DAW@90b42269`: `npm ci --no-audit --no-fund`, `go build -o ./bin/VitAgent.exe ./cmd/vitagent` from `agent/`, and `npm run build` from `agent/webui/` all exit 0. Logs: `npm-ci.log`, `agent-build.log`, `webui-build.log`.

Prepared smoke artifacts, not yet executed:

| Artifact | SHA-256 |
|---|---|
| Agent | `411C7C1B22EFCEB5D51FC7434F0A06AC9621C56AF1C38574096CE157C2CC9C9D` |
| WebUI `dist/index.html` | `A31C539E65B27F9B0768FB3F5D7D77189F7AACEA56FDF4F99ACA7671A7FF9F0A` |
| Borrowed unchanged kernel | `A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E` |
| Borrowed unchanged hub | `B10CC41295066C48BB446B255D12D1A58D2D8D64F164D875083667B464C71F68` |
| Authoritative fixture source | `AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D` |

The frontend worktree's tracked deployment DLL is old; runtime must stage the candidate DLL in an isolated project and record its fresh hash. These preparatory artifacts do not satisfy first-display, focus, z-order, geometry, external-foreground or teardown acceptance.
