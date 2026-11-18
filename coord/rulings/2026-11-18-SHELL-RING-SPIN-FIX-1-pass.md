# SHELL-RING-SPIN-FIX-1 验收：PASS（加载卡状态环原地自转，computed-style 亲证）

- 卡：`2026-11-18-SHELL-RING-SPIN-FIX-1`（池序 416）；主管=lead（PC 决策流）。
- 执行：`lead/ringfix416`（DeepSeek-V4.1-Flash，PC）；主仓分支 `codex/shell-ring-spin-fix-416`，commit `2160104f67ab58585ecc8ce939d7afbaa31baf5b`（父 186ca505）；已合入主仓 main `88f4f163` 并推送。
- 用户反馈：加载卡黄色圆环不是原地自转而是绕轴环绕；期望**原地自转**（已确认）。

## 修复

| 文件 | 变更 |
|---|---|
| `src/design/sprite.css` | `.v5-ring--idle/.v5-ring--busy` 的 `transform-origin: 12px 12px`（24px 坐标）→ `center`；busy 1.6s 与 reduced-motion 全局规则保留 |
| `src/project/loading/loading.css` | 删除 `.v5-loading-chip .v5-icon` 的 `v5-loading-advance`（translateY）覆盖、其 keyframes 与组件级 reduced-motion 块；进行中动效回归 `v5-ring-spin` 单一来源 |
| `tests/render-window-presentation.mjs` | pending 态新增 4 条 computed-style 断言（16px busy 环、spin 1.6s 未被覆盖、轴心居中、无 translateY） |
| `src/gallery/render-smoke.mjs` | `test:render` 新增 §4b：16/20/24 三档样张 `transform-origin` 必须落在自身几何中心（防空过） |

## 主管独立复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；60 files / 722 tests |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；35 checks，0 failed；`ringSpin` 原文：`animationName=v5-ring-spin`、`duration=1.6s`、`iteration=infinite`、`transformOrigin=8px 8px`、`transformBox=view-box`、`activeAnimations=["v5-ring-spin"]`、matrix 无位移 |
| `npm run test:render`（仓外工件目录） | exit 0；FOUNDATION/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED 全 PASS；FOUNDATION notes：`status-ring origins centered: 16px→8px 8px, 20px→10px 10px, 24px→12px 12px` |
| `git diff --check` | exit 0 |

- 执行侧反向对照：只还原旧 CSS、保留新断言时如实失败（`12px 12px`，exit 1），随后从 commit 还原，worktree clean。

## 产物与边界

- 主检出 `agent/webui/dist` 已按 416 重建：`dist/index.html` SHA256=`0C6A1691EB17323DBAEA13C4CBA3954D0DC70459ACBC5FB98315F3CC311FE7A9`（取代 413 时的 0886058E…；手测入口 `启动VitShell.cmd` 已含本修复）。
- 保留的同类既有行为（未改动、非本卡域）：`components.css` 对 `.v5-current-chip .v5-icon` 的 `v5-step-advance`（translateY）优先于 ring spin，StatusCard/trace 的 current-chip busy 环仍是上下步进。用户本次只反馈加载卡；若 SPEC 要求该处也原地自转，需另开扩域小卡。
- 边界：自动化只证 computed-style/渲染面；实际视觉仍待用户目检确认「原地自转」。
