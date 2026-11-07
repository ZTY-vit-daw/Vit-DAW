# PLUGIN-CONTROL-EVAL-RUNNER-1：效果器控制评估 runner 建设（S0 验证+P 基线执行）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[299 PREP](../done/2026-11-07-PLUGIN-CONTROL-EVAL-PREP-1.md) next_card_scope+冻结 plan.json（coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json）+[评估方案](../../reports/2026-11-07-plugin-control-model-evaluation-plan.md)）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 300；目标仓库=主仓 scripts+coord 工件；P1 / 0.5-1 天 / 依赖=299 已收口；L2 / PC；**模型矩阵=deepseek-flash 单模型（用户裁定 2026-11-07 晚：deepseek 为测试原型底线，不加对照模型）**
- **授权范围**：本卡含 S0（零 LLM 已知操作验证）+**S1-P（产品基线 6 轮，付费，预算按 plan.json per_round_caps，段级 go/no-go 回报主管）**；A*/B*/C 纯臂**不在本卡**（缝卡另立，候 P 结果）

## 目标

1. **runner 建设**（零栈先行）：按 plan.json next_card_scope——fixture builder（per-case 独立工作副本+哈希）、S0 known-op 验证器（零 LLM，per-case exit code：脚本化正确操作验证 T1-T6 目标可达/枚举单位解释正确/回读恢复有效——正确操作失败=夹具/接口问题不进模型打分）、arm driver（per-round 会话隔离+预算计数器，telemetry 口径）、独立评价器（readback+非目标项 diff+verdict JSON——评价器正确参数/目标状态不进被测模型上下文）、工件新 run_id 目录不覆盖。先例=pgtdc_compat_verify.ps1+b3_pan_layout_agent_smoke.py。
2. **S0 执行**（真栈，候栈排队：296→298→275 后）：六案例全零 LLM 验证 exit 0（或如实 blocked 证据）；§8 跑前成文（确定性零 LLM）。
3. **S1-P 执行**（S0 过后同栈窗口或再登记）：产品基线 6 轮（真实默认模型 deepseek-flash+默认提示词+默认披露），逐轮记录：入口可达/实际披露量/首次提案/执行请求响应/独立回读/非目标变化/终态/调用数+token+费用+时长；失败按 plan.json failure_taxonomy 分类；**单轮形态=任务开始→预算内操作与修正→终态**。
4. 回执：runner diff+S0 六案例结果+S1-P 六轮成绩表（case_id/arm/first_action/final_verdict/failure_tags/readback/mutation_count/tokens/cost/stop_reason）+预算实耗 vs caps；**P 结果落地即回报主管**（A/B/C 缝卡决策输入）。

## 约束

- 文件域=scripts/ 新 py/ps1（+dev_agent_smoke.ps1 加腿如需）+coord/runs/PLUGIN-CONTROL-EVAL-*/ 工件+卡面；**agent/ internals 与 VitApp/ 只读**（缝=EVAL-SEAM-1 另卡）；fixture 走独立临时副本（§10 防污染）；
- 真栈协议（§2.2+增补）全链；与在飞栈腿错峰（登记前 fetch 重读）；
- 评价器期望值不硬编码轨名等易变标识进生产码；密钥不入工件；
- S1-P 付费轮预算超 per_round_caps 即停该轮（stop_rules 按 plan.json）；环境重建上限跑前冻结。

## 停止条件

S0 正确操作腿失败→夹具/接口取证上交（不进模型轮）；S1-P 连续两轮同型环境类失败→止损取证；栈冲突按登记协议等待。

## 领取/回执/验收

- 领取：2026-11-07 18:49 +0800 / origin/main=8568f2e4175b2b293982aae4db33a7d289e86c72 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 300runner）/ 分支=port/plugin-control-eval-runner-1 / worktree=D:/Vit_DAW_wt_300runner / 领取提交=coord/300-claim（hash 见回执节）。领取基线：fresh worktree 自 origin/main，`git status --short` 与 `git diff --stat` 均为空（clean）。冻结计划取自 origin/port/plugin-control-eval-prep-1:coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json（287 行，已通读）。
- （执行侧回填回执）/（主管回填）
