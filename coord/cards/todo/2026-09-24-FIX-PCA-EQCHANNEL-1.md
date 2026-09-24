# FIX-PCA-EQCHANNEL-1：EQ 族认证通道接入 autosweep（68 分类命中 → 认证 → 白名单多候选）

- 优先级 / 预估 / 依赖：P1 / 0.3-0.5 天 / FIX-PCA-AUTOSWEEP-1 验收遗留（例外队列 68 条 static_eq 命中无 runner）；用户 2026-09-23"各族多少能用"问题在 EQ 族的收尾
- 模型分级：L1 / GLM-5.3 flash 可接（复用 autosweep 骨架+EQ-1 确定性认证通道）
- **执行侧（PC 侧卡；mac 侧 EQ 已三候选无需跟随，但通道为共享脚本两端可用）**
- **背景**：autosweep 认证扫的 runner 面覆盖 v2 store 五族（de_esser/transient/limiter/gate/multiband）+broadband（v1），**static_eq 无通道**——68 个分类命中全部滞留例外队列；EQ-1 时期 Q10 走的是"零 LLM phase3_live_smoke 收据+pcactl import"专用通道（未并入 sweep）。
- 目标：
  1. `scripts/pca_autosweep.py` certify 相增 static_eq 通道：EQ-1 确定性路径批量化（identifier 实例化→增益带覆盖写 upsert/modify/disable/undo→回读→快照→phase3 式收据→import 晋升；零 LLM/零手编红线同卡面）
  2. 重跑 certify+derive（幂等 state.json 续跑）：EQ 候选从 1（bx_hybrid legacy S0）扩至命中集；**bx_hybrid/Vertigo legacy 补认证**（PC-FULL 裁定①的执行落点——sweep 扫全库含 legacy 条目）
  3. EQ 多候选自选验证腿（五环口径，同 BROADBAND/PLUGIN-SELECT 先例）
  4. sweep_report 例外队列中 static_eq 桶清零或残余记因
- 文件域：`scripts/pca_autosweep.py`（+测试/自检输出）；零 agent 代码改动预期
- 验收：①EQ 通道收据抽核 ≥3（含 ≥1 Waves 壳 EQ + bx_hybrid legacy 补认证）零 LLM；②白名单 EQ 族多候选+溯源；③overlay 回归；④EQ 自选腿五环 exit 0；⑤例外队列 static_eq 处置记录
- 停止条件：EQ 带结构参数锚派生大面积失败（>半数）→ 记录上交（可能需带面探测扩展）
- 领取：
- 回执：
- 验收：
