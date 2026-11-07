# PLUGIN-CONTROL-EVAL-S1A-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-07-PLUGIN-CONTROL-EVAL-S1A-1.md（领取 dbbf6494 @ origin/main d4bd3496）
- 冻结计划：origin/port/plugin-control-eval-prep-1:coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json（快照在本目录 runner_frozen/plan.json，blob 3fdb595a）
- 冻结 runner：origin/port/plugin-control-eval-runner-1@ccf8c78c 的 scripts/plugin_control_eval_{cases,driver}.py 原样快照（本目录 runner_frozen/，blob d48ca7cf 与 aefb5abe 派生；离线 selftest PASS 47 项 exit 0，console=selftest_console.log）。**源码零改动**：agent/ 与 VitApp/ 只读；runner 两件与 300 ps1 均不加改——A 臂编排按 AGENTS §5「按同一模式新增脚本」以工件目录内 `arm_a_orchestrator.ps1` 承担（复刻 300 ps1 的净空预检/env 冻结/六口拆除面），驱动器以 `PYTHONPATH=<worktree>/scripts` 解析共享模块 compressor_compat_matrix_smoke。
- 缝实现：已合入 main 的 1775c10b（305 四缝；验收 pass）。A 臂环境 `VIT_PLUGIN_CONTROL_EVAL_SEAM=1`（仅精确 "1" 开缝）在 agent 启动前进入进程 env（编排脚本 Set-Item → dev_agent_smoke Start-Process 继承链，与 300 遥测 env 同机制）。
- 真栈协议：PC-RUNTIME-STACK 净空亲核→登记推 main→≤60s 复核→启栈→run→TEARDOWN_ALL_ZERO 亲核→释放登记。kernel=wt_228 只读复用（SHA256 f9e3da0a66263f0b6e3fed5cda0fb214c460fce8053234240c470d515ed0d67a，与 300 P 轮同一二进制）；agent=本 worktree 现场构建记 SHA256。

## 段构成（单一栈窗口内顺序执行）

1. **S0 重冻（零 LLM）**：`driver s0 --out-dir <run>/s0_*`；成功门=s0_all_verified=true（六案 known-op verified+T4 允许集冻结+T6 不可满足复核）。失败分类 fixture_failure / environment_interrupt（同 300 §8）；同签名 fixture_failure 两连败止损。S0 为 A 段自证夹具面（300 的 freeze 属 P 轮栈，本段重跑以在本栈本 agent 二进制上冻结）。
2. **dryround 缝开（零 LLM）**：`driver dryround`；成功=管线 verdict pass + exit 0 + 遥测零行核验；兼作 305 卡候队列的「dryround 缝开腿」。
3. **装置验证 probe（付费 1 轮，非 18 任务轮）**：`driver round --case T1 --repeat 0 --rounds-dir <run>/probe`（repeat=0 钉 probe 身份，不入成绩表）。**门（seam_verify.py，全部满足才放行任务轮）**：
   - S2 面：probe 全部请求 payload 中 normalized_role / display_group / control_relevance / template_role / alias 五字段零命中（阳性对照=P 基线 capture 同法 grep 命中 2/2/2/2/…——方法检出力先行证明；alias 以键形态 `"alias"` 检索，裸词命中逐条人工审阅记录）；
   - S3 面：全 payload 无完整生产描述尾串 "display_domain_candidate from explain_controls all_parameters"；若 set_plugin_param 工具披露则描述==裸句 "Set one explicit plugin parameter by normalized value (0.0-1.0)."；
   - S4 面：全 payload 无 typed 路由首规则 "- Before freezing an actionable generic EQ semantic_action"（P 基线 24 命中=对照）；
   - S1 面：probe 的 semantic_entry 在场 + message_loop 普通环调用在场 + agent log delta 无 typed apply 面（plugin_grabber_apply*/apply_{reverb,delay,eq,compressor,limiter,de_esser,gate_expander,transient_shaper,multiband}_* 工具零调用）；
   - 任一不满足=**seam_leakage/装置失效 → exit 4 停段取证**（不进任务轮，付费已耗如实入账）。
4. **18 任务轮（付费）**：case 序 T1→T6 固定，每案 repeat 1→3 顺序（plan ordering：case 外层/repeat 内层，同 300 driver segment 环形态）；每轮 `driver round --case X --repeat N --rounds-dir <run>/rounds --allowance <s0 freeze T4 允许集>`。每轮新 conversation_id+新 fixture（fresh track+plugin 实例+冻结初态核对）+预算内操作与修正（确认卡立即 approve=300 冻结策略继承；澄清回问≤3 次冻结回话）+终态+独立回读+非目标 diff+边界检查+拆 fixture。
5. **合并与对照（离线）**：consolidate_a.py 汇 18 轮成绩表（arm=A 重标口径见下）+预算实耗 vs A 段 caps+逐案 A vs P 对照（P=p_baseline/segment_p_summary.json，blob 1a005f14）。

## §8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 18（6 案例×3 repeat）；probe 1 轮（装置验证载体，不计成绩不占 18 名额）；失败轮不重跑，按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结）。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted）：LLM 调用 12/轮；单调用输入 40k/输出 8k tokens；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（超限停段，剩余轮记 not_run_segment_cap，编排脚本轮间检查）：任务轮合计 ≤216 LLM 调用 / ≤8,640,000 输入 / ≤1,728,000 输出 tokens（plan segment_caps.A_star）。probe 与 s0/dryround 额度：probe ≤12 调用计入计费报告但不占任务轮段 caps（plan 无 probe 条目——本 RUNBOOK 显式预冻结：probe 预算上界=单轮 caps，实耗如实入账）。
- **失败分类**：plan failure_taxonomy 全集（多标签+最早断点）；A 臂特有 `seam_leakage`（五字段泄露/typed 接管恢复/骨架残留任一=装置失效优先报告，立即停段）；行为标签分列（direct_guess/asked_or_searched/self_probed/self_corrected）。
- **止损线**：①probe 门失败=装置失效停段取证（卡面停止条件）；②连续两轮同型环境类失败→止损取证上交；③同代码版本同确定性断点两连败→禁原样重跑，先取证；④fixture 污染/越权写入/arm 信息泄露→立即停批隔离证据（plan stop_rules.pollution_or_leak）。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash, baseUrl=api.deepseek.com/v1，apiKey 在 config 不在 env）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑（同 300）；模型面亲证=per-run telemetry 行级 model 字段+payload capture 响应 raw.model。
- **计费**：用户 DeepSeek 账户；per-round input/output/cache_read tokens 逐轮入成绩表；cost 字段=missing（plan budgets.monetary pending，价目快照未核不事后折算）。
- **退出码**：0=18 轮全评估完成（≠任务成功≠产品验收）；1=driver/编排层失败；2=环境中断/栈占用；3=段预算/止损停止（剩余轮如实报 not_run）；4=装置验证失败（seam_leakage）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分。

## 装置已知边界（预声明，结果解读项）

1. **arm 标签残留**：冻结 driver 的 round_record/内部字段硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先于臂概念交付，本卡源码零改动不加改）。A 臂成绩表以 consolidate_a.py 按 run 目录（本 run）+ repeat 重标为 A；原始 round_record 原样保留不改写。
2. **确认卡 approve-all 继承**：300 冻结策略对一切确认卡立即 approve（含 web 检索确认面）。A=不联网指本段不授予检索预算（B 臂才开检索）；若 A 轮模型自发走产品确认门检索且被冻结策略放行，如实记录 web.* 调用并标 `retrieval_breach_device_behavior` 上交主管裁量——装置不可在不触冻结脚本的前提下改此策略（卡面：源码零改动）。
3. **S2 最小结构残留**（305 验收裁定+A 卡面口径声明项）：quick_controls 选择面+recommended_groups 组名仍是内核语义结构；A=「裸参数+最小结构残留」臂。
4. **B/C 检索臂不在本段**；A 数据不得解读为 B/C 结论。

## 工件与可回指性

- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/（本目录）：runner_frozen/（冻结快照+blob 哈希）、p_baseline/（P 对照输入，blob 与 300 分支逐字一致）、arm_a_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256/§8 声明/seam env 记录/model env 掩码/teardown 亲核；s0_*/dryround/probe/rounds/ 各子目录；logs/ 含 per-run telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-s1a-1 分支入库；主检出不留未跟踪副本（§6.1）。

## 诚实边界（预声明）

- s0/dryround=确定性腿（零 LLM）：证明夹具/接口/目标可达性与缝开态管线可用，不产生模型能力结论。
- 18 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张）。
- 单轮=单样本；3 repeat 是重复样本非稳定性证明（plan §6 口径：比例主张需另行规定样本数）。

## 附录 A：跑中追加（2026-11-07 22:35 +0800，run2 取证后；上文冻结文本未动）

- **run1（arm_a_20261107_222543）**：s0 T3（Abbey Road Plates，案例序首个 Waves shell 插件）rack_add_node「Timed out waiting for JUCE command handling」fixture_failure（T1/T2/T4/T5/T6 verified；零 LLM）。签名#1；判冷载时序类（T4/T5 同 shell 族后到即 verified）。按 §8 允许的第二次尝试重跑。
- **run2（arm_a_20261107_222801）**：s0 六案全 verified（T3 过——冷载假设成立）；dryround 缝开 pass 零 LLM；**装置验证门 pass**（S2 五字段 A=0 vs P 对照 70/30/30/36；S4 骨架规则 A=0 vs P=24；S1 semantic_entry+message_loop 在场）；probe（T1_r0）与 T1_r1 两轮 verdict=pass 后，**编排脚本自身缺陷中断**：PS5.1 Get-Content 默认 GBK 读 UTF-8 round_record.json 致 ConvertFrom-Json 失败（我的工件缺陷，非冻结 runner/非源码；修复=-Encoding UTF8）。已耗 LLM=probe 5 轮调用+T1_r1 5 轮调用（计入实耗报告，不占 18 名额；两轮工件原样在卷）。
- **装置观察（重大口径项，上交主管）**：probe 与 T1_r1 的模型写入均走 **typed 适配器** `plugin_grabber.inspect_reverb → plugin_grabber.apply_reverb_controls`（message_loop.tool 面；prompt render 含 plugin_grabber 工具材料 60 处）。四缝（S1-S4）不移除普通工具目录中的 typed plugin_grabber 工具——305 缝范围与 305 验收裁定未把工具目录列入缝面；本卡冻结验证门（五字段+无 typed 接管路由）按卡面口径**通过**。A 数据解读=「无骨架引导下模型自选工具面（含 typed 适配器可用）」，非「仅裸参数工具」；逐轮 write_path（typed_adapter/bare/none）由 consolidate_a.py 提取入成绩表，主管裁量 A 数据定性与是否另立 S5 缝卡。
- 追加工件改动（非门语义变更）：seam_verify.py 增 informational typed_adapter 计数（不参与门判定）；consolidate_a.py 增 tool_faces 提取；arm_a_orchestrator.ps1 修 -Encoding UTF8。run3 重跑全段（s0+dryround+probe+18 轮，新 run 目录）。
