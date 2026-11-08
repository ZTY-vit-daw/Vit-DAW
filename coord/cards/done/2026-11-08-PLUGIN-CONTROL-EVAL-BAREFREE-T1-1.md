# PLUGIN-CONTROL-EVAL-BAREFREE-T1-1：T1 补完腿（第三层 harness 守卫 seam+T1×3 定向复跑）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[318 终验裁定①](../done/2026-11-08-PLUGIN-CONTROL-EVAL-BAREFREE-1.md)+用户裁定「确保模型真的能够裸写参数控制」——T1 是最后一个未测及案例）
- 派发确认：已确认（主管发卡·定向补完）；验收负责人：GLM 主管决策流
- 池序 319；目标仓库=主仓 agent（harness 守卫缝）+工件；P2 / 0.25 天 / 依赖=318 已合入；L2 / PC；真栈一次

## 目标

1. **第三层守卫解剖+seam**：`broadMixObserveFirstWriteGuard`（`agent/internal/harness/harness.go:2590`+Invoke ~1040，「不认 mix.observe 完成」的写入门）——同 318 模式入 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE` 缝（默认 unset 字节不变+配对单测：缝开=T1 型写入放行/缝关=现状拦截）；**harness.go 域预声明**（与 273 未合入分支不同函数区，合入冲突归主管）；
2. **T1×3 定向复跑**（非全 18）：A-pure 面+全守卫许可（318 缝+本腿缝）——T1 案例三轮同判据；装置门=守卫许可亲证扩展（T1 型写入 probe 落地）；
3. 判定：T1 pass 率=最后一个案例的真裸写答案；guard_blocked_write 分类维持；
4. 预算 ≈¥1（3 轮×~13 调用级）；§8 跑前成文；栈排队（L1 真栈腿协调）。

## 停止条件
守卫与 mix.observe 语义深度耦合无法缝隔离（需要合法完成态证明）→取证上交转裁定（此时 T1 答案=「守卫要求完成态是设计语义」）。

## 领取/回执/验收
- 领取：2026-11-08 深夜 +0800 / origin/main=51a419094a5f7458d64879e481db0c8de4d85646（fetch 亲证，≥发卡要求 51a41909，含 318 缝与全套工件）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 319 t1 补完；派发已确认）/ 分支=port/plugin-control-eval-barefree-t1-1 / worktree=D:/Vit_DAW_wt_319t1 / 领取提交=coord/319-claim→main。领取基线：fresh worktree 自 origin/main 51a41909，`git status --short`=空、`git diff --stat`=空（全净）。在飞卡域核对：doing 八卡（EDU-ASK-ANCHOR-1=282impl 已完未合入、ARRLOOP-AUTHBRIDGE-1、CLOSURE-ROUTE-ADAPT-1、WAVESHELL-OFFRENDER-FIX-1=C++ 域、LEGAME 三卡、PLUGIN-CONTROL-EVAL-S1B-1=工件目录+编排+tools 缝域）与本卡（agent/internal/harness/harness.go 守卫函数区±配对单测 + coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-T1-1/ 工件目录 + 本卡面）不相交；**harness.go 域预声明**（卡面原文）：本腿守卫缝与 273 未合入分支（Authorize* 家族，harness.go:2223-2248 一带）同文件不同函数区——本腿只动 broadMixObserveFirstWriteGuard 守卫函数区，diff 自检不触 Authorize*，合入冲突归主管。318 缝在位亲证：agentloop `pluginControlEvalSeamBareWriteOpen`（message_loop.go:4360 一带）+chat 同名助手（goalrunner_chat.go:153 一带）均在 HEAD。
- 回执：（执行侧回填）
- 验收：（主管回填）
