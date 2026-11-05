# MATRIX-LEG-PCA-CONTEXT-1：compressor matrix 腿族 PCA 授权面接通（G5）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G5——9/9 门拦=腿过时非门缺陷）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 261；目标=scripts/compressor_compat_matrix_smoke.py（invoke 面）；P2 / 0.5 天 / 无依赖；L1；验证腿需真栈一次（de-esser Matrix 单腿）

## 目标

1. matrix py 腿 `/agent/invoke` 注入 full-project-access context（或 PCA 授权注入面，按 8a415ef7 修复先例的授权语义——不绕门，走门认可的服务端授权路径）；
2. 验证=de-esser Matrix 单腿真栈复跑 9 case 走到断言面（过/如实分列；门拦截=0）；compressor 系列其余三腿修后不强制本卡跑（共享模块同修，下轮体检抽查）。
- §8：零 LLM 预期（原腿设计）；单 run。

## 约束

文件域=该 py 腿+必要时 ps1 入口；禁改 harness 门本体；门语义零削弱。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
