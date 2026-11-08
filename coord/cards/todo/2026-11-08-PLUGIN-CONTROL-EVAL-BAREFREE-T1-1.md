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
- （执行侧回填）/（执行侧回填）/（主管回填）
