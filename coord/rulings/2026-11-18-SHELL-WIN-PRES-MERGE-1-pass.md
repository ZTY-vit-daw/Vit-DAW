# SHELL-WIN-PRES-MERGE-1 验收：实现面 PASS（零栈），待组合真栈最终确认

- 卡：`2026-11-18-SHELL-WIN-PRES-MERGE-1`（池序 412）；主管=lead（PC 决策流，2026-11-18 交接后）。
- 执行：`lead/presmerge412`（DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-win-pres-412`，commit `c44c9039be1c2b74d334876d893c05ac6526619d`；基线 `738291f6`。
- 合入：主仓 main merge commit `668078fd07b14abdf2fc275ad769296cc64e9216`（已推 origin/main）。

## 主管独立复核（非仅读回执）

| 命令 | 结果 |
|---|---|
| `git show --name-status` | 仅 8 个卡面文件域文件：App/ConvergedLauncher/ProjectSurface/windowPresentation.tsx+css+test/render-window-presentation.mjs/README；无 coord/、scripts/、gallery、launcherData |
| `npm run test` | exit 0；60 files / 722 tests（含 windowPresentation.test.tsx 9 例） |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；checks=31 / failed=[] |
| `npm run test:render` | exit 0；FOUNDATION/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED 全 PASS |
| `git diff --check` | exit 0（执行侧；主管复核 diff 未见断言弱化） |

## 采纳与边界

- 采纳：WindowPresentationProvider 挂在 App；ConvergedLauncher/ProjectSurface 接线 launchBusy 门与反馈/焦点提示；消费 394-D 协议（由 410 在 409 基线提供 producer）。
- 冲突处理采纳：`launcherSurface.test.tsx`、`gallery/render-smoke.mjs` 保留 HEAD(389) 版本；排除 `launcherData.ts` 文案、`ProjectSurface.test.tsx` 两行断言与 coord/runs 工件（均在卡面文件域外）。
- **未落地项（如实登记）**：395-B 的 directive 占位文案（launcherData.ts，域外）未随本卡落地；如需该文案另开扩域小卡。不影响 391 的装载反馈/焦点提示主链。
- 本卡只证明本地逻辑与生产 React 装配渲染；真实 WebView 投递、Godot 时序、OS 焦点/z 由 414 组合真栈裁决。当前状态=实现面 PASS，最终端侧结论见 414。

## 后续

- 410/411 producer 完成后，413 重建 webui dist/VitShell；414 组合真栈同时断言 409 窗口面与 395 呈现面。
