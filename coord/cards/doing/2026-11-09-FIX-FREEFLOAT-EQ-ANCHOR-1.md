# FIX-FREEFLOAT-EQ-ANCHOR-1：free_floating EQ 派生带锚类缺口收口（PENDING 分流④，TBTECH withheld 落盘）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 终验裁定①：derive 修复 8c0ef051 使规则统一生效，TBTECH Kirchhoff-EQ 派生行 withheld 不落盘候主管点头；bx_tonebox=类缺口在案）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流；**TBTECH 行落盘=主管预授权**：验证充分即落，无需再候）
- 验收负责人：GLM 主管决策流
- 池序 328；P2 / 0.5 天 / 依赖=322 derive 修复已在位（8c0ef051 已合入）；L1 / PC；**零栈优先**（derive 离线跑）

## 目标

1. **TBTECH 行落盘**：重跑 derive（离线）→ 核对 Kirchhoff-EQ 派生行（收据自证带锚，322 工件在卷）→ 白名单 EQ 段外科 apply（同款纪律：diff 仅增行、双备份、Go 加载门亲证）；
2. **类缺口登记**：bx_tonebox（无收据自证→仍诚实排除）与「默认态全 Unused 的 EQ 无收据外带锚来源」类缺口写成登记段（进白名单 provenance 或 coord 登记，不硬造锚）；
3. 披露面亲证：白名单加载门 PASS + static_eq 段计数更新在卷。

## 文件域

`scripts/pca_autosweep.py` derive 产物 + `~/.vit/free_state_experiment_plugins.json`（先备份）+ 工件 `coord/runs/FIX-FREEFLOAT-EQ-ANCHOR-1/`；零 agent 源码。

## 验收标准

derive exit 0 + 外科 apply diff 仅增 TBTECH 行 + 加载门 PASS + 双备份在案 + 类缺口登记文本在卷。

## 停止条件

重跑 derive 后 TBTECH 行不再派生（规则漂移）→ 停手取证上交，不手改派生输出。

## 领取/回执/验收

- 领取：2026-11-09（晚窗）/ origin/main=31d0b9d1d336d3d2e459c5c018b77c7794a4d8fe（fetch 亲证）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 328 freefloat-eq-anchor，派发已确认+TBTECH 落盘预授权在卡面）/ 执行分支=port/freefloat-eq-anchor-1 @ worktree D:/Vit_DAW_wt_328impl（领取基线 HEAD=31d0b9d1、status 0 行、diff 空，亲证）/ 协调分支=coord/328-claim @ worktree D:/Vit_DAW_wt_328coord / 领取提交=本卡 todo→doing 推 main。依赖亲证：8c0ef051 已在 origin/main（merge-base 祖先亲证）。doing 四卡文件域（EDU-ASK-ANCHOR/ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-OFFRENDER-FIX）与本卡（scripts derive 产物+~/.vit 白名单+工件目录）不相交。主检出他人未提交改动（新 todo 卡×4+POOL-INDEX.md/CURRENT-STATE.md 等）为卡域外权威输入，不碰。
