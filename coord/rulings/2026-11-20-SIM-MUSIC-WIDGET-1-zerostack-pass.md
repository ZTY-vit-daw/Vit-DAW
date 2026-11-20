# 裁定：SIM-MUSIC-WIDGET-1(441) 零栈 PASS（真栈手测留窗）

- 日期：2026-11-20 23:45 +0800
- 卡：441 SIM-MUSIC-WIDGET-1（对话流谱面卡）
- 裁定：**零栈三门+渲染面主管复跑全 PASS**；真栈 VitShell 手测留 lead 窗口（不记终 PASS）。
- 代码：d9daabf3 → main 08f24c7b；主管复跑 vitest **821/821**（新增 19）、build exit 0、test:render **11 段全 PASS**（含 PROJECT/ENTRY388/REC-TAPE/WIDGETS/CONVERGED）。

## 实现事实（diff 抽查）

- 谱面渲染复用 374 `scoreworkbench/render.ts renderScoreInto`（beat-grid）+`model.ts validateModel`——容器未整挂（ScoreWorkbenchPanel 强耦合，符合卡面停止边界）。
- UI 侧 MIDI 解析新组件 `project/midiScore.ts`：与 `internal/tutorsim/midi.go` 同语法面（MThd/format 0,1/division/VLQ 界检查全部 fail-closed；单声部守门同 kernel.reload 语义）；fixture 实测 42 音/48 拍/**4/4**/100BPM。
- 交互：材料卡「查看谱面」仅在**已装载且有谱面数据**时出现（fail-honest）；内联展开 MaterialScoreView（VexFlow SVG+meta），默认收起、收起即卸载、渲染异常显形不断流；AudioPlayer 留注释锚点。
- live 链接缝：`tutorScoreLoader`（trackID/clipID→后续真栈卡接 `createAgentScoreBridge.reload()`）——本卡不接线（卡面设计内）。

## 边界与观察

- 真栈手测（VitShell 产品入口）候窗——谱面卡全链（含 live 谱面数据）是后续卡。
- 装置行为：test:render 会覆写 `coord/runs/UI-*` 烟测工件（既有装置行为，未入 commit；§10 登记随装置）。
- 执行侧环境中断一次（junction node_modules→vite @fs 越界白屏，连续 2 次同断点后修 robocopy 真副本）——分类正确、主仓对照排除回归。

## 下一步

- 真栈手测窗（lead）：装载→查看谱面→42 音 4/4 渲染断言；随后 live 谱面数据链接缝候另卡。
