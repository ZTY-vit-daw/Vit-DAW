# FIXTURE-WE-HYGIENE-1：smoke fixture 波形包络遗留卫生（293 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[293 取证报告](../../reports/2026-11-07-we-mismatch-forensics.md)+[293 裁定](../done/2026-11-07-WE-MISMATCH-FORENSICS-1.md)——`semantic_processor_project_smoke_v1_80085263a651cf20` fixture 携带 2026-09-05 strip_silence smoke 遗留：`.vit_agent/vitproj_b3f28381…/mixboard/mixboard_feature_snapshot.json` WE 槽 `_1055` 行+twe 末行 (1007,1055)+`.vit_derived/.../acoustic_package_status.json` (1007,1055) 声学包，与 9/6 工程态（数字 id 全集 1001-1036 无 1055）**内在不一致**；读侧桥脱落时裸槽浮出→D1 waveform 面 settle 相位诚实翻转（设计内披露，物化链无罪））
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 294；目标仓库=主仓 fixture（smoke 工件域）；P2 / 0.5 天 / 依赖=293 工件在库；L1-L2 / PC；零栈

## 目标

1. **卫生处置**：剔除/重置 fixture 的 9/5 遗留面（WE 槽 `_1055` 行、twe (1007,1055) 行、(1007,1055) 声学包条目），使 fixture 与其工程态自洽；**先复制到独立工作区操作+验证，再按权威 fixture 更新流程换入**（§10 防污染纪律：不直接改权威 fixture，前后核对哈希）；
2. 验证：卫生后的 fixture 在 293 同型读路径下 WE 视图稳定 ready（无裸槽浮出路径）——离线验证即可（293 工件轨迹复放/读侧函数离线跑）；如需一次真栈回归（266 D1 门组 settle 相位波形稳定）记候栈；
3. 顺带盘点：同型 fixture 是否还有其他日期遗留面（只记录）。

## 约束

- 文件域=fixture 副本工作区+报告；**源码零改动**；265/266/293 run 工件只读；
- sealed fixture 纪律：期望值/轨名不硬编码进代码或日志。

## 停止条件

剔除操作破坏 fixture 其他面板一致性（如 twe 规范行/声学包其他条目引用断裂）→停下取证上交，转重建流程评估。

## 领取/回执/验收

- 领取：2026-11-07 / origin/main=`66313d86` / owner=GLM-5.3（ZCode 执行子代理，PC） / 分支 `port/fixture-we-hygiene-1`（报告）+ `coord/294-claim`（卡状态） / worktree `D:/Vit_DAW_wt_294hyg`（实现）+ `D:/Vit_DAW_wt_294coord`（协调）/ 领取基线：HEAD=`66313d86`，status/diff --stat 均空（新 worktree 自 origin/main 检出）
- （回执执行侧回填）/（主管回填）
