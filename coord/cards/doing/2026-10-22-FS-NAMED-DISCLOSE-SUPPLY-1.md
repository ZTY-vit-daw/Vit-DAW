# FS-NAMED-DISCLOSE-SUPPLY-1：user_named frontier 披露行确定性供应（159 判读缺口②继卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=159 判读：A5/A3 两轮 unt 已解析（1032/1012）但渲染 prompt 全程无 user_named:<id> frontier 披露行，同 fixture 同 unt 的 A1 有——fold/披露面非确定性缺失 → 模型声明轨身份未披露而边界自报/域漂移；156 §① 观察供应面同族）
- 派发确认：已确认（159 RESULTS §2 判读落定）
- 验收负责人：GLM 主管决策流
- 池序 170；目标仓库=D:\Vit_DAW；**P1（比 169 更重：直接卡到达率与意图保真——A5 边界自报+A3 域漂移同源）** / 0.5 天 / 依赖=无（可与 169 并行，文件域不相交）；模型分级：L2 / GLM 执行会话

## 范围

第一步=锚点取证：user_named frontier 披露行的生成与折叠链路（prepare 侧 user_named_target 已解析 → candidate/frontier 投影 → 渲染 rows 的 fold 点）——找出 A1 命中/A5 缺失的分叉条件（候选集非平稳？frontier 竞争丢弃？fold 时机竞态？）；第二步=确定性供应：unt 非空时披露行**必须**在场（同 A1 形态），缺失=缺陷而非可选项；第三步=单测钉死（同 unt 两形态必一致）+真栈 chat 腿复刻 A5 话术对照（159 工件在卷可复用 fixture 配方）。

## 文件域

以勘察结论固定（预期 `agent/internal/chat/free_state_reasoning_loop.go`/`audio_closure_controller.go` frontier 投影面+渲染 fold 邻域）；开工时写入卡面。越域即停。

## 验收标准

单测（unt 非空→披露行必在场+同 unt 幂等）+真栈 chat 腿=A5 话术复刻（unt=1032 已解析**且渲染披露行在场**，模型不再边界自报）+全量 0 FAIL+gofmt 净；端测边界声明。真栈腿登记 PC-RUNTIME-STACK（与 169 错峰）。

## 停止条件

披露行生成点在 semantic_entry 分类器或 CCB bounded disclosure 契约内（注入受红线约束）→ 锚点上交再裁定。

- 领取：2026-10-22 20:09 / origin/main=4e82e41e（领取时远端 tip；本卡发卡面自主管本地 main ee0050de 摘取入共享 main，同 787063ec 先例只同步 coord 面）/ owner=GLM-5.3 L2 PC 执行会话（ZCode）/ 分支=port/fs-named-disclose-supply-1 + 协调分支 coord/fnds170-claim / worktree=D:/Vit_DAW_wt_fnds170（实现期，勘察期零改动暂不建）/ 领取提交=coord/fnds170-claim tip
- 回执：（锚点取证结论 / commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
