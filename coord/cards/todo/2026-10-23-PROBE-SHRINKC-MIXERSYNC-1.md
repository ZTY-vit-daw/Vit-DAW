# PROBE-SHRINKC-MIXERSYNC-1：shrink_c mixersync 腿预存不稳定修复

- 发卡：GLM 主管决策流 / 2026-10-23 早窗（依据=2026-10-22 gate 风险③；§11 重复出现规则触发）
- 派发确认：已确认（gate 2026-10-22 §3 风险③裁定「应开修复卡」）
- 验收负责人：GLM 主管决策流
- 池序 179；目标仓库=以勘察固定（**主管勘误 2026-10-24**：shrink 探针实位于前端仓 D:\Godot\project\vit-daw-frontend\tools\probe_fe_gui_shrink_phase_c.gd，主仓无此文件；mixersync 断言腿亦可能在主仓 scripts 驱动侧——领取时勘察定位并在领取记录回填文件域）；P3（测试面卫生）/ 0.25 天 / 依赖=无；模型分级：L1 / PC 执行会话（flash 可）
- 与 176/178（前端执行流）文件域不相交可并行，但**真栈腿与在飞卡错峰**（§2.2 登记）。

## 现象

166 卡回执如实上交：shrink_c 场景 mixersync 腿连续三轮各挂 1-2 腿、终轮全过，基线与本支同现=**预存不稳定非本卡引入**。原始失败输出在 166 工件（coord/runs/FE-SEND-UX-UNIFY-1/sendux_gui_20261022_1906/RESULTS.md 邻域）。

## 范围

①读 166 工件失败腿原始输出，失败类型定位（时序竞态/等待窗口不足/环境残留/确定性缺陷）；②隔离复跑取证（≥3 轮）；③按结论修复（等待窗/settle 判据/夹具隔离——修复探针与断言面，不得为凑过改期望值）；④若确属环境类不可修，按 §11 给隔离证据+known flaky 登记（禁止无条件永久豁免）。

## 文件域

以勘察固定（预期 shrink_c 场景探针 mixersync 腿邻域）。越域即停。

## 验收标准

修复后连续 ≥3 轮该腿全绿（或 known flaky 登记含隔离复跑+整包复跑+diff 排除证据）；既有探针零回退；工件入 coord/runs/PROBE-SHRINKC-MIXERSYNC-1/。

## 停止条件

失败根因位于卡片域外（产品 mixersync 逻辑本身）→ 锚点+复现上交另裁。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（失败类型 / commit / 复跑轮次与结果 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
