# SIM-MUSIC-WIDGET-1：对话流谱面卡（music_score 控件挂载）

- 池序 441；P1；PC；发卡=lead（2026-11-20 深夜）；派发确认=用户 2026-11-20 23:2x（free 车道批量派发授权）；验收负责人=lead。
- 背景：SIM 线 435→438 已落地导师循环+简报卡+MIDI 材料装载卡；[368 裁定档 §二]登记的下一环=music_score 对话流控件（374 谱面工作台包装成卡内挂载组件）。真栈手测放行面=VitShell 产品入口。

## 目标（零栈，最小形态）

1. **谱面卡（ScoreCard）**：材料卡（MidiMaterialCard）扩展「查看谱面」交互——点击后在对话流内联展开谱面渲染，复用 374 SCORE-WORKBENCH-1 的 VexFlow beat-grid 视图组件（`agent/webui/src/score/` 域，领取时锚定实际组件名），数据源=已装载 MIDI 材料的音符（tutor workflow_data materials 已含 note_count/duration 摘要；谱面渲染所需音符序列从装载链既有内核回读或材料文件解析面取——领取时锚定，不发明新协议；若现有接缝拿不到音符序列，用装置侧材料文件只读解析，UI 不落新后端端点）。
2. **收起/展开**：默认收起，不撑爆对话流（与简报卡同模式）；普通工程/v0 实验零 UI 变化。
3. **播放器卡留位**：AudioPlayer 挂载接缝确认（不动实现——音频形态候内核渲染 MIDI→wav 后续卡）。

## 约束

- 文件域：`agent/webui/src/project/`、`agent/webui/src/score/`（只读复用为主）、`agent/webui/src/widgets/`（只读复用）；**不改 Go/server/内核**；437/438 既有测试只加不减。
- 与并行卡文件域互斥声明：440 在 `agent/internal/genapi`（Go），334 在 chat/harness——本卡全在 webui，无相交。
- 领取：worktree + 分支；领取记录提交 main 后开工；完成走 main 合入。
- 停止条件：374 组件无法在对话流上下文复用（强耦合工作台容器）→ 停卡上报锚点，不硬塞 iframe/复制大块代码。

## 验收（零栈）

- vitest：谱面卡展开渲染（音符数/拍号断言用小星星 42 音/48 拍/3-4 拍号装置 fixture）、收起态、无材料不渲染、437/438 用例不回归；`npm run test` + `npm run build` + `npm run test:render`（如装置在卷）exit 0。
- 真栈手测留 lead（VitShell 产品入口）。

## 领取（scorecard441，2026-11-20）

- 领取基线：main@`cd6032c70b64e62927db5fcc4cc68b438dc8b67b`；主仓工作树已有其他卡/运行态改动（YUE-440、VitApp workspace、coord/runs 工件等），与本卡文件域（`agent/webui/src/project/`、`scoreworkbench/` 只读）不相交，未触碰。
- worktree：`D:/Vit_DAW_wt_pc441score`；分支 `pc441-scorecard`（基于 main@cd6032c7，worktree 内 status 干净）。
- 374 复用锚点（实际组件名）：谱面渲染复用 `agent/webui/src/scoreworkbench/render.ts` 的 `renderScoreInto`（beat-grid 视图适配层，纯函数）+ `model.ts` 的 `ScoreModel`/`validateModel`；任务卡写的 `src/score/` 实际路径为 `src/scoreworkbench/`。`ScoreWorkbenchPanel`/`ScoreTrackWorkbench` 容器强耦合工作台编辑器/桥/target，不整挂——复用其渲染层不构成「复制大块代码」。
- fixture 实测锚定（node 解析亲证）：`theme_twinkle_v0.mid` = format 0 单轨 / division 480 / 42 noteOn / 48 拍 / 拍号 meta nn=4 dd=2（**4/4**，卡面「3-4 拍号」按实际装置 4/4 断言）/ tempo 600000us=100 BPM。
- 数据源裁定：`workflow_data.materials` 仅摘要（Go `tutorsim.go` 不下发音符序列，Go `midi.go` 的 `ParseMidi.Notes` 无网络面）→ 按卡面授权走「装置侧材料文件只读解析」（TS 解析器，纯函数，fail-closed）；live 装载链内核回读（`createAgentScoreBridge({clipID,trackID}).reload()`，374 既有协议）留为 ProjectSurface 可注入接缝，本卡不接线、后续真栈卡接入。
- 停止条件评估：未触发（渲染层可复用，无 iframe/无复制）。
