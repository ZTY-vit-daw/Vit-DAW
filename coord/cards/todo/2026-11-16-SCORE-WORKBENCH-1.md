# SCORE-WORKBENCH-1：谱面工作台——节奏/旋律模式编辑+内核真写回读+随谱游标（展演主要新开发）

- 发卡：GLM 主管决策流 / 2026-11-16 凌晨（依据=[369 选型定案](../runs/G0-SCORE-SPIKE-1/REPORT.md)：VexFlow 5+自研 beat-grid 模型；进度驱动无工期）
- 派发确认：已确认（主管发卡，展演关键路径主开发）
- 验收负责人：主管决策流（DG1-A+DG3 关卡）+ 用户手测节点（谱面编辑体验）
- 池序 374；P0；依赖=无（M3 的 2/4 工程语义候 375 拍号卡；M5 真栈腿候栈位）；PC

## 目标（里程碑顺序执行，每 M 独立可验收）

- **M1 模型+推导+渲染**：kernel 形状模型（携带内核 note id 一等公民）+推导规则成文（369 报告 R1-R3 规则+拍内附点归并/弱起延音链/休止填充表=**W18 契约基础**）+VexFlow 5 渲染与命中检测；vitest（jsdom env 成文）全绿。
- **M2 编辑交互**：点击放符/时值切换/附点开关/休止插入/延音建立解除/删除/拖动（16 分网格）全经自研模型推导重绘；**治理粒度=本地编辑缓冲+显式 apply 单次确认**（主管工作假设，RiskConfirm 语义对齐；候用户裁定单确认）。
- **M3 内核往返**：apply_midi_note_patch 真写（time_unit:beats）→get_midi_clip_notes 回读→模型重建回显；保存重开工程回读一致（2/4 工程语义候 375）。
- **M4 随谱游标**：/agent/state playhead_seconds→beat→x（tick→x 游标表 369 已证）+单 tempo fail-honest 门（多段 tempo 降级不静默错位）。
- **M5 端侧验收**：渲染面烟测（浏览器级 DOM 断言）+真栈烟测腿（DG1-A 口径，[STACKGATE §DG1-A](../../docs/STACKGATE_DG1_DG3_V1.md)）exit 0。

## 约束与风险（369 报告 §5 风险表全量继承）

R4 VF5 API 漂移→锁版本+薄适配层；R6 游标轮询粒度受限→必要时评估 events 流不改内核；R7 推导规则集可审查成文；R8 **Verovio 不捆绑本卡**（LGPL+wasm 单独裁定）；零内核/VitApp 改动（出声走 373 模板线）。

## 文件域

`agent/webui/src/scoreworkbench/**`（新目录全套）+`agent/webui/package.json`（+vexflow、+jsdom devDep）+composer/交互卡装配接线点（领取时锚定——与 367 域已合入基线核对）+`scripts/` 烟测扩展（同模式加参数）+coord 工件。禁入：内核/VitApp、gen 域（372）、模板域（373）、chat 内部。

## 验收标准

DG1-A 真栈腿 exit 0+渲染面烟测 PASS+vitest 全绿（基线 584+新增）+M1-M5 逐里程碑工件；用户手测节点=谱面编辑体验（DG3 M 清单）。

## 停止条件

VF5 在真实装配面不可用（npm/许可意外）→取证上交；治理粒度工作假设被用户否决→M2 确认面返工段停候新裁定。

## 领取/回执/验收：待回填
