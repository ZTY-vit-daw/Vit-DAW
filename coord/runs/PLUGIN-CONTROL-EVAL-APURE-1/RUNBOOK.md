# PLUGIN-CONTROL-EVAL-APURE-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-APURE-1.md（领取 c151f9d0 @ origin/main 00722e1c）
- 冻结计划：runner_frozen/plan.json（快照 blob 259d6b4d，与 307 runner_frozen/plan.json 逐字一致；A_star 段 caps 原样沿用：18 rounds/216 calls/8.64M in/1.728M out）
- 冻结 runner：runner_frozen/ 三件=307 runner_frozen 快照逐字复用（blob：cases=d48ca7cf / driver=57abac09 / plan.json=259d6b4d，与 D:/Vit_DAW_wt_307s1a 同名文件 git hash-object 双侧核对一致）。**源码零改动**：agent/ 与 VitApp/ 与 scripts/ 只读；A-pure 编排按 AGENTS §5「同模式新增脚本」以工件目录内 `arm_apure_orchestrator.ps1` 承担（复刻 307 ps1 的净空预检/env 冻结/SEAM=1 透传/六口拆除面），驱动器以 `PYTHONPATH=<worktree>/scripts` 解析共享模块。
- 缝实现：**含 308 的现 main**（领取基线 00722e1c；merge 1c77e6f2=305 S1-S4 + 308 S5 工具目录裁剪+S2b 渲染面剥离，单 env `VIT_PLUGIN_CONTROL_EVAL_SEAM=1` 仅精确 "1" 开缝）。缝实现在位亲证：catalog.go pluginControlEvalSeamTrimsCatalogSpec（三出口裁 plugin_grabber 前缀全族）+context.go stripBindingTargetsSemanticOverlay（binding_targets 行剥三语义键）。A-pure 环境 env=1 在 agent 启动前进入进程 env（Set-Item → dev_agent_smoke Start-Process 继承链，与 300/307 同机制）。
- 真栈协议：PC-RUNTIME-STACK 净空亲核→登记推 main→≤60s 复核→启栈→run→TEARDOWN_ALL_ZERO 亲核→释放登记。kernel=wt_228 只读复用（SHA256 f9e3da0a…，与 300/307 P/A 轮同一二进制）；**agent=本 worktree（wt_310apure）现场构建，记 SHA256+tool_count**（含 308 缝，跑前 308 缝单测在位复跑）。
- 对照输入：p_baseline/=300 P 轮 capture+summary 快照（blob 6c4db7c4/1a005f14，与 307 p_baseline 逐字一致）；a307_reference/=307 A 臂成绩表（blob 85c5d329）+**T3_r1 受染负锚**（自 307 run3 logs 提取该 conversation 全 10 行，其中 7 请求行携带 binding_targets 行内 normalized_role/template_role——与 307 回执实录一致，提取脚本输出在案）。

## 段构成（单一栈窗口内顺序执行）

1. **S0 重冻（零 LLM）**：`driver s0 --out-dir <run>/s0_*`；成功门=s0_all_verified=true（六案 verified+T4 允许集冻结）。失败分类 fixture_failure / environment_interrupt；同签名 fixture_failure 两连败止损（300 §8 继承；307 run1 的 T3 冷载时序签名在案为先例——Waves shell 首载 JUCE 超时属 fixture_failure 签名#1，重跑过一次的环境重建上限 1 次/批预冻结）。
2. **dryround 缝开（零 LLM）**：`driver dryround`；成功=verdict pass+exit 0+遥测零行。
3. **装置验证 probe（付费 1 轮 T1 repeat 0，非 18 任务轮）**：门=`seam_verify_apure.py`（自测 8/8 exit 0 双向检出力在卷），全部满足才放行任务轮：
   - **S2**：五字段（normalized_role/display_group/control_relevance/template_role/alias 键形态）A-pure 请求 payload 全零 vs P 对照四字段命中（方法检出力先行）；
   - **S2b**：渲染面负锚检出力——307 T3_r1 受染 capture（a307_reference/）同法检测必须命中 ≥7 请求行（该形态 7/7/7 实录）；A-pure 侧三语义键零命中（S2 覆盖）；binding_targets 键=原生保留（308 设计：只剥行内三键），计数 informational 不判泄露；
   - **S3**：无生产描述尾串；
   - **S4**：无 typed 路由首规则（P 对照 24 命中）；
   - **S5**：目录行形态 `tool=plugin_grabber.` 零命中（307 同法 5666 命中=蓝本对照）+**裸读写对在场**（目录行 `tool=plugin.get_parameters` 与 `tool=plugin.set_parameter` ≥1——裸面缺失=A-pure 臂无裸写面，同为装置失效）；plugin_grabber 裸串命中 informational（已知许可形态：repair 回显幻觉调用/goalrunner EQ 守卫文本，308 验收裁定③⑤）；
   - **CS**：content-form control_shell 兜底（308 验收裁定①）——`"control_shell"\s*:\s*\{` 内容对象形态零命中；键名列表形态（result_keys）=已声明残留放行计数；
   - **S1+typed 门**：probe 会话 semantic_entry+message_loop 在场+agent log delta 执行面（message_loop.tool/harness.invoke）**零 plugin_grabber/typed 适配器调用**（S5 后目录无该族；任何执行=裁剪失效）；
   - 任一不满足=**seam_leakage/装置失效 → exit 4 停段取证**（不进任务轮，付费如实入账）。
4. **18 任务轮（付费）**：case 序 T1→T6×repeat 1→3（plan ordering 同 300/307）；每轮 `driver round --case X --repeat N --rounds-dir <run>/rounds --allowance <s0 freeze T4 允许集>`；每轮新 conversation_id+新 fixture+预算内操作与修正（确认卡立即 approve=冻结策略继承；澄清回问≤3）+终态独立回读+非目标 diff+拆 fixture。
5. **语料级 seam 收口（18 轮后，编排 phase 8b）**：同 seam_verify_apure 对全 capture（18 轮+probe 全 payload 行）复查——S2 三语义键+content-form control_shell+S5 目录行+typed 路由四面全量；**任何命中=装置失效旗标**（卡面停止条件触发时点=发现时，轮 verdict 原样、数据有效性交主管裁定——307 同模式先例）。
6. **合并与对照（离线）**：consolidate_apure.py 汇 18 轮成绩表+逐轮 write_path+typed 门化（任何轮 typed_adapter 执行=exit 2 device_invalid 旗标）+**三方逐案对照表 A-pure vs A(307) vs P**（交付核心）+T4 联动对分析旗标（307 验收裁定沿用：verdict 原样不改判据，规则修正留 B/C 批次前）。

## §8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 18（6 案例×3 repeat）；probe 1 轮（装置验证载体，付费不计成绩不占 18 名额）；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结，307 先例：PS 编排脚本自身缺陷类中断重跑全段新目录）。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted）：LLM 调用 12/轮；单调用输入 40k/输出 8k tokens；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（超限停段，剩余轮记 not_run_segment_cap，编排轮间检查）：任务轮合计 ≤216 LLM 调用/≤8,640,000 输入/≤1,728,000 输出 tokens（plan segment_caps.A_star 原样）。probe 预算上界=单轮 caps（12 调用，实耗如实入账不占段 caps——307 同款预冻结；307 实耗 6 调用/142k in）。
- **失败分类**：plan failure_taxonomy 全集（多标签+最早断点）；`seam_leakage`（S2/S2b/CS/S5 任何形态=装置失效优先报告立即停段；语料收口时点发现=旗标上交）；环境中断（栈占用/端口/进程/网络类）；fixture_failure（rack_add_node JUCE 超时签名#1 在案）；harness_invalid（编排/脚本自身缺陷，307 先例 PS GBK 读 UTF-8）；行为标签分列（direct_guess/asked_or_searched/self_probed/self_corrected）。
- **止损线**：①probe 门失败=装置失效停段取证；②连续两轮同型环境类失败→止损取证上交；③同代码版本同确定性断点两连败→禁原样重跑先取证；④缝泄露/typed 执行/越权写入→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑；模型面亲证=telemetry 行级 model 字段+capture 响应 raw.model。
- **计费**：用户 DeepSeek 账户；估 ≈¥3-6 上界（307 全程实耗 109 调用/1.87M in/50k out 参考；A-pure 轮数与 caps 同款，裸面操作数不高于 307）；per-round tokens 逐轮入成绩表；cost 字段=missing（价目未核不事后折算）。
- **退出码**：0=18 轮全评估完成且语料 seam 收口+consolidate 全绿（≠任务成功≠产品验收）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用；3=段预算/止损停止；4=装置验证失败（seam_leakage，含语料收口时点发现）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分。

## 装置已知边界（预声明，结果解读项）

1. **arm 标签残留**：冻结 driver round_record 硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先于臂概念交付，本卡源码零改动）。A-pure 成绩表以 consolidate_apure.py 按 run 目录重标 "A-pure"；原始记录原样保留。
2. **确认卡 approve-all 继承**（300 冻结策略，含 web 检索确认面）：A-pure=不联网段不授予检索预算；模型自发检索被冻结策略放行时如实记录 web.* 调用并标 `retrieval_breach_device_behavior` 上交。
3. **最小结构残留**（305/308 验收裁定在案）：quick_controls 选择面+recommended_groups 组名+control_shell **键名列表**形态（result_keys）=已声明残留放行；**内容对象形态**=CS 门拦截（308 裁定①）。
4. **S5 后已知外观**：goalrunner EQ 守卫指引文本仍指向 plugin_grabber.apply_eq_edits（目录已无该工具）=test-only 态装置事实、两臂可比（308 裁定③⑤记录接受）；守卫文本若入 payload，plugin_grabber 裸串 informational 计数不判失效。
5. **T4 联动对**：307 验收裁定=疑似冻结规则缺陷（人类等价双通道写入），verdict 原样，分析层旗标；判据修正留 B/C 批次前，本卡不改。
6. **B/C 检索臂不在本段**；A-pure 数据不得解读为 B/C 结论。A-pure=「真纯裸参数面」（无 typed 工具+无渲染语义回灌）臂——与 A(307)=「自选工具面」、P=「生产全语义」三方对照后才是「越过控制脚本」问题的正式答案。
7. **单轮=单样本**；3 repeat 非稳定性主张（plan §6）。

## 工件与可回指性

- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-APURE-1/（本目录）：runner_frozen/（冻结快照+blob）、p_baseline/（P 对照，blob 与 300/307 逐字一致）、a307_reference/（307 A 成绩表+T3_r1 受染负锚）、seam_verify_apure.py（六面门+--selftest 双向检出力自测）、consolidate_apure.py（三方对照+typed 门化）、arm_apure_orchestrator.ps1（编排）、RUNBOOK.md（本 §8 冻结）、arm_apure_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256/tool_count/§8 声明/seam env 记录/model env 掩码/teardown 亲核；s0_*/dryround/probe/rounds/ 子目录；logs/ 含 telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-apure-1 分支入库；主检出不留未跟踪副本（§6.1）。

## 诚实边界（预声明）

- s0/dryround=确定性腿（零 LLM）：证明夹具/接口/目标可达性与缝开态管线可用，不产生模型能力结论。
- 18 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张）。
- 装置验证门=probe 轮亲证+语料级收口双层；probe 干净不免疫后轮泄露（307 教训：门设计按枚举面，语料复查为收口证据面）——发现时点后于轮次完成时，按 307 先例旗标上交主管裁定数据有效性。
