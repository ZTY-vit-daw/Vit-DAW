# WE-MISMATCH-FORENSICS-1：D1 波形包络特征物化失配取证（274 相邻勘察①转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 回执相邻勘察①](../done/2026-11-07-D1D3-DIGEST-WIRING-1.md)+[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)④——D1 全局 waveform 面 settle 相位 ready→missing 翻转：266v1 settle 相位内 WE 槽行被 `kernel_prepared_waveform_envelope_1055`（track 1007/clip 1055）替换且材质失配 `project_material_identity_mismatch`（`currentProjectFeatureRowOrMissing`，agent/internal/mixboard/mixboard.go:2534/:2544-2559）；265 同链 WE=`_1011` 匹配未翻转。**1055 行产生者/失配来源在 harness 特征物化链，尚未定位**——本卡取证定位，不预设修法）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 293；目标仓库=主仓 agent（harness 特征物化链勘察）；P2 / 0.5 天 / 依赖=274 工件在库；L1-L2 / PC；零栈（265/266 两 run 工件在库对照）

## 目标（取证卡）

1. 定位 `kernel_prepared_waveform_envelope_1055` 行的产生者（harness 特征物化链锚点）与 project material identity 的失配来源（为何 266v1 settle 相位产生 _1055 而 265 同链为 _1011——差异驱动：两 run 工件+console+时间线对照）；
2. 产出最小修建议（若定位到确定性根因）或如实记录「设计内行为」结论（若 1055 替换属 settle 相位的合法重铸——需证据钉死，不得含糊）；
3. 建议面开修卡 or 记录收案，交主管裁定。

## 约束

- 文件域=coord/runs/WE-MISMATCH-FORENSICS-1/ 工件+报告；**源码零改动**（取证卡，发现缺陷只定位不上手修）；
- 265/266 run 工件只读（禁止写回既有 run 目录）。

## 停止条件

证据不足以区分「物化缺陷」vs「设计内重铸」→两案并陈上交（272 waveform 面先例），不得强行择一。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
