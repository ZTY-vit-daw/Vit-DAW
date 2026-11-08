# PLUGIN-CONTROL-EVAL-BAREFREE-1：裸面+守卫许可臂（真裸写能力测量——「如果允许，模型能不能」的唯一干净测试）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[结论档 ⓪ 修订](../../rulings/2026-11-08-plugin-control-eval-line-conclusion.md)——9/9 写入失败轮 payload 解剖=治理守卫阻断非编码错误；**用户裁定 2026-11-08 深夜：需要一轮新实验，确保模型真的能够裸写参数**）
- 派发确认：已确认（用户裁定+主管发卡）；验收负责人：GLM 主管决策流
- 池序 318；目标仓库=主仓 agent（守卫许可缝）+scripts/工件；P1 / 0.5-1 天 / 依赖=317 工件在库（缝已合入 main）；L2 / PC；零栈先行+真栈

## 目标

1. **零栈解剖先行**：T1 写入闸门机理定位（B-R1 payload `agent_llm_payload_capture.jsonl` T1 三轮 tool_result 错误原文+代码锚点——「先观察、再确认」判定面在读/写路径的拦截点）+T3/T4 typed-surface 守卫定位（`typed_transient_shaper_surface_unavailable` 错误源）；
2. **守卫许可缝（test-only）**：新 env 键（如 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE`，312 检索缝同模式——**默认 unset 生产字节不变**）：缝开时 ①T1 型写入闸门对插件参数读写路径放行 ②typed-surface 守卫对裸写放行（受护族允许落地）；配对单测（缝开=受护参数裸写落地/缝关=现状拦截两向钉）；
3. **BAREFREE 18 轮**：A-pure 同款裸面（S1-S5+S2b）+守卫许可缝开+**无检索无提示**（单变量：唯一差异=守卫许可）；cases/判据/caps/T4 标注与 317 逐字同；
4. **装置门**：既有面全复刻+**守卫许可亲证**（probe：受护参数裸写真实落地+回读变化）；缝关态回归亲证（unset=拦截恢复）；
5. **判定核心（结论档待办①兑现）**：T1/T3/T4 在守卫许可下的 pass 率=**真裸写能力**；写入落地后 readback 校验编码正确性（value_encoding 终于可测）；consolidate **新增 guard_blocked_write 分类**（tool_result 守卫错误形态识别——终态标签≠机理的教训落地，本臂若再现守卫拦截不得误标 value_encoding_error）；
6. **回报**：barefree vs A-pure（同面守卫开关对照）为核心比较+并入全阶梯对照；「模型裸写受护族能力」正式答案。

## 约束

- 文件域=守卫所在包（chat/agentloop/policy/tools 按解剖定位）±测试+工件+编排复刻；生产默认不变红线；不触 L1 三件（314/315）域（edu_jam/server 布位段）；
- 真栈协议全链；栈排队（L1 真栈腿可能候栈——登记前重读，先到先得不抢）。

## 停止条件

解剖发现守卫与语义入口/routing 深度耦合无法缝隔离→取证上交转装置设计裁定；缝泄露任何形态停段；同型环境失败两轮止损。

## 预算

≈¥2-4 上界；§8 跑前成文。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
