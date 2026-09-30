# Ruling：FIX-CONFIRM-CARD-1 — pass（2026-09-30 决策会话）

- 实现：8c8cc79d@port/fix-confirm-card-1 → 合并 main 59a9e8a0（cherry-pick）
- 亲核项：
  1. diff 对应：8 文件 +1098/−24，全部 webui 域 + E2E 烟测脚本，零 Go 改动（与回执一致）。
  2. E2E 工件亲读：artifacts/e2e_webui1/20260930_100412，webui_rendered_dom_report.json verdict=**pass**；K1/K2 DOM 工件在位（dom-confirm-k1-{before,after,settled}、dom-confirm-k2-{direct,console,storage}）。
  3. **我复跑**：npm run test 于合并态 main——**354/354 全绿**（含新增 lifecycle 终结/替代、直执目标/剥卡、merge 不复活按钮三描述块）。
  4. 语义抽查：fullAccessDirectExecutionTargets 谓词=三型确认卡（confirmation/mix_tick_confirmation/mix_treatment_confirmation）+approval.requested、二元 approve/cancel 且无表单才命中，proposal_approval/选卡/表单排除——与裁定方向精确一致；manual_confirmation 路径不变。
- 裁定方向落实：full access 下 RiskConfirm 不出前置卡、直执+事后回执入流+台账 ✓（K2 断言）；缺陷②三联根因（回合终结收卡/同族替代收卡/已结算不复活按钮）与 K1 断言（终结后零可点击残留）对应 ✓。渲染前拦截+复用既有 interaction/confirm 通路、authority API 只消费——接口冻结遵守，停止条件未触发定性正确。
- 边界采信：①proposal_approval 维持 GUI-T4 呈现兜底未纳入真直执（capability session 停 waiting 的申报属实，真直执需 agent 侧配合——留待 vit note 线 §7.2 升级提案机制统一处置，不另开孤立卡）；②improvement_proposal_confirmation 排除（方案类产品面）；③直执通告文案避 "confirm" 启发式坑已在常量处注释钉住（K2 二轮实证）。
- 遗留：**[等待用户手测]** pv1_p01 副本 full access 路径 C 复验——与 M1 手测复验项（audition 轨迹消失确认）同场顺带，不阻塞本裁定。
