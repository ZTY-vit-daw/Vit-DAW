# SOFTGUIDE-RESCHEDULE-1：146 软引导实验重排腿（L1+L3 修复后按 §E 原协议重跑）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=156 设计稿 §3 第三卡；L3=152 已合入（bbea3445），L1=157 本卡前置）
- 派发确认：已确认（156 验收裁定，随验收 commit）
- 验收负责人：GLM 主管决策流
- 池序 159；目标仓库=D:\Vit_DAW；P2 / 0.25 天 / **依赖=157（FS-NAMED-TARGET-1）合入后跑**（158 可并行不阻塞本卡）
- 模型分级：L1 / flash 可接（驱动脚本机械重跑+记录；判读交主管）

## 范围（规格权威=[146 实验工件 §E 原协议](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md)+[156 设计稿 §3](../../runs/FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)）

146 SOFTGUIDE B2 臂实验重排：L1（frontier 指名轨并入）+L3（回复面分层）合入后按 §E 原协议重跑同款话术矩阵；主判据=**F-gate 0 计数+提案面到达率恢复**（B2 判读预绑定规则已锁硬规则分支，本腿不重判惯例效应，只验证到达率）。

## 文件域

驱动脚本（softguide_exp.py 参数面如需）+工件目录 `coord/runs/SOFTGUIDE-RESCHEDULE-1/`（非生产码）。生产代码零改动。

## 验收标准

重跑工件齐（per-case events+prompt_render 判定面）；F-gate 计数与提案面到达率如实计数（不凑期望）；真栈按 PC-RUNTIME-STACK 登记与在飞卡错峰；§8 预声明（轮次/止损线）在跑前写入工件；端测边界声明。

## 停止条件

重跑仍现 F-gate 签名 → 停止计数，保存原始工件上交（157 修复失效证据，不得原样重跑超两轮）。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（工件路径 / F-gate 计数 / 到达率 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
