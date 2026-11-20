# SIM-MUSIC-WIDGET-1 执行回执（scorecard441，2026-11-20）

## 四件套

### 1. 执行的命令 + 退出码（worktree D:/Vit_DAW_wt_pc441score/agent/webui）

| 命令 | 退出码 | 结果 |
|---|---|---|
| `npx tsc --noEmit` | 0 | 无类型错误 |
| `npm run test`（vitest run 全量） | 0 | 821 passed / 67 files（含本卡新增 19 例） |
| `npm run build`（tsc + vite build） | 0 | built in 2.6s（chunk 大小告警为既有现象） |
| `npm run test:render` | 0 | 11 项全 PASS：APPENTRY/ENTRY388/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED |
| `npx vitest run src/project/` | 0 | 143 passed（存量 124 + 新增 19：midiScore 13 + TutorWidgets 谱面 4 + ProjectSurface 谱面 2） |

git：分支 `pc441-scorecard` 实现 commit `d9daabf3`（7 文件 +813/-6）；main merge `08f24c7b`（--no-ff）；领取记录 main `a56a8b89`。

### 2. 产物路径

- 实现：`agent/webui/src/project/midiScore.ts`、`midiScore.test.ts`（新增）；`TutorWidgets.tsx`、`TutorWidgets.css`、`TutorWidgets.test.tsx`、`ProjectSurface.tsx`、`ProjectSurface.tutor.test.tsx`（扩展）。
- 本目录：PROGRESS.md（全程步骤/命令/断点）、receipt.md（本文件）。
- 卡面：`coord/cards/done/2026-11-20-SIM-MUSIC-WIDGET-1.md`（领取节+执行完成节）。

### 3. 遇到的问题

- **环境中断（已修复，非本卡代码回归）**：worktree 初用 junction 共享主仓 node_modules，`npm run test:render` ENTRY388 连续 2 次 FAIL（`waitForSelector("[data-launcher-recent-open]")` 30s 超时）。诊断（`C:/Users/timoz/AppData/Local/Temp/entry388_diag.mjs`）：vite dev 对 `/app/@fs/D:/Vit_DAW/.../vite/dist/client/env.mjs` 响应 JSON——junction 使 node_modules 物理路径落在主仓，超出 worktree dev server 的 server.fs 白名单 → 模块图 MIME 失败白屏。主仓同诊断 PASS（排除代码回归）。处置：删 junction，robocopy 真实副本（172M，exit 1=成功）→ 三门复跑全绿。
- 烟测工件覆盖：test:render 依既有装置行为覆写 `coord/runs/UI-*`、`CONVERGE-*` 的报告/截图（未提交、未混入本卡 commit）。
- 仓库窄声明约束：vite-env.d.ts 只声明 `readFileSync(path, encoding): string`（不引 @types/node）——装置 fixture 用 latin1 读（字节↔码位无损映射）转 Uint8Array。

### 4. 断了几次

- 无重派；单会话全程完成（首轮开工至收尾无中断）。

## 复用与数据源锚点（供后续卡）

- 374 复用：`scoreworkbench/render.ts renderScoreInto`（beat-grid 渲染层）+ `model.ts ScoreModel/validateModel`；`ScoreWorkbenchPanel` 容器强耦合工作台编辑器，未整挂。
- 音符数据源：本轮=装置侧解析（`parseMidiScore`：SMF format 0/1、VLQ+running status、note on/off 配对、0x58 拍号/0x51 tempo；fail-closed 同 Go tutorsim/midi.go）；live 链=ProjectSurface `tutorScoreLoader` 接缝（装载回执 trackID/clipID → 后续真栈卡接 `createAgentScoreBridge.reload()`）。
- 装置 fixture 实测：`theme_twinkle_v0.mid` = 42 音 / 48 拍 / 4/4（nn=4 dd=2） / 100 BPM / division 480 / format 0 单轨。
