# PROBE-DRYRUN-WARNING-1：探针头加「干跑=必触栈」警告横幅

- 发卡：GLM 主管决策流 / 2026-10-23 早窗（依据=2026-10-22 gate 风险⑧：stack_probe 干跑误判同族事故第 4 例——134/139/161/166，最近一例 166 会话 19:00:25-29 误判 headless 不触栈实际执行完整探针腿污染 159 在飞栈）
- 派发确认：已确认（gate 2026-10-22 §3 风险⑧建议开流程改进小卡）
- 验收负责人：GLM 主管决策流
- 池序 180；目标仓库=前端仓 D:\Godot\project\vit-daw-frontend（**主管勘误 2026-10-24**：stack_probe 实位于前端仓 tools/stack_probe_sendroute_macro.gd 等，主仓无此文件族；原文"目标仓库=D:\Vit_DAW"系发卡误写）；P3（流程卫生）/ 0.25 天 / **依赖=183 合入**（stack_probe_sendroute_macro.gd 与 183 拖动断言同文件，并行必冲突，发卡侧把关）；模型分级：L0-L1 / PC 执行会话
- 文件域与 176/178 不相交可并行；纯探针头改动无真栈腿，如有验证腿按 §2.2 登记。

## 范围

①勘察 stack_probe 场景模式入口（stack_probe_sendroute_macro 等探针头）；②启动头部加显眼警告横幅：本探针任何模式（含所谓「干跑」）都走生产传输链 VitIpcClient→4445→agent→内核，**执行前必须完成 PC-RUNTIME-STACK §2.2 占用登记**；③警告行含时间戳+场景名，console 与工件报告双源。

## 验收标准

探针启动输出含警告横幅行（console+报告双源断言或截图）；既有探针零回退；工件入 coord/runs/PROBE-DRYRUN-WARNING-1/。

## 停止条件

无（极小卡；若勘察发现探针入口分散多处，取最近四事故涉及的 sendroute 族入口为主，其余登记余量）。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 警告行证据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
