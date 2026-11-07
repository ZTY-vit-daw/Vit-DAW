# PLUGIN-CONTROL-EVAL-RUNNER-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md（领取 61c685f2 @ origin/main 8568f2e4）
- 冻结计划：origin/port/plugin-control-eval-prep-1:coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json（287 行）+ RUNBOOK.md + preflight 报告
- runner：scripts/plugin_control_eval_{cases,driver}.py + scripts/plugin_control_eval.ps1（commit 5a6e8a73；selftest 45 项全绿；ps1 parse 零错误）
- 真栈协议：PC-RUNTIME-STACK 登记→≤60s 复核→run→TEARDOWN_ALL_ZERO 亲核释放；候队列 296→298→275 之后（发卡侧注明）

## S0 段（零 LLM 已知操作验证）

- **运行次数：单 run 无重跑**（run_id=启动时间戳，新目录不覆盖）。`plugin_control_eval.ps1 -Mode s0`。
- **LLM 轮次：0**。遥测证据面=per-run `logs/agent_llm_telemetry.jsonl`（env 重定向），S0 结束后核对该文件 0 行（含 strip-silence 类默认子腿——本 wrapper 不启 chat/strip-silence 子腿，预期零行；若出现非零行=环境中断分类，如实记录）。
- **成功条件**：六案例全 `verified`（每案：装载+冻结初态核对→脚本化正确操作→独立回读（重查实例）→非目标行 diff→恢复→恢复回读）+ T4 联动允许集冻结 + T6 不可满足复核 + ps1 exit 0。
- **失败分类**：`fixture_failure`（断言面：目标不可达/枚举单位解释错/回读恢复失效/不可满足前提被推翻→取证上交，不进模型轮）；`environment_interrupt`（栈/传输/端口，原始日志在卷）；不设模型失败类（零 LLM）。
- **止损线**：同签名 fixture_failure 两连败→停上交取证卡，禁止原样重跑。环境中断按 plan stop_rules：原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结）。
- **退出码**：0=全六 verified；1=任一 fixture_failure；2=环境中断；3（本段不用）。

## S0 附带：dryround（零 LLM 管线干跑）

- 单 run；`plugin_control_eval.ps1 -Mode dryround`；成功=scripted known-op 经全管线（fixture→round 簿记→写→独立评价器→工件）verdict=pass + exit 0；LLM=0 同上遥测零行核验。

## S1-P 段（付费产品基线 6 轮）

- **放行条件**：S0 全绿（case_freeze.json s0_all_verified=true；segment-p 驱动器硬门）+ 卡面已授权（本卡含 S1-P）+ 真栈窗口（同一登记窗口内续跑或重新登记）。
- **模型面**：产品默认 `~/.vit/config.json`（deepseek-flash）；启动 shell 三组环境变量记录入 run_report（VIT_AGENT_LLM_MODEL/OPENAI_MODEL 设置时拒跑）；首轮请求捕获（VIT_AGENT_LLM_PAYLOAD_CAPTURE per-run）核验响应 model/采样参数（preflight §1 待验项——首轮后回填观察，静态不冒充）。
- **运行次数：每案例 1 轮×6 案例，T1→T6 固定序，单形态无重跑**（plan run_counts.P；失败轮不重跑，按 failure_taxonomy 记录）。
- **每轮形态**：新 conversation_id+新 fixture（fresh track+plugin 实例+冻结初态核对）→冻结任务消息（中文，评价器期望值不进上下文）→预算内操作与修正（确认卡立即 approve=冻结策略；澄清回问≤3 次冻结回话）→终态→独立回读+非目标 diff+边界检查→拆 fixture。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted）：LLM 调用 12/轮；单调用输入 40k/输出 8k tokens；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（超限停段，剩余轮记 not_run）：72 LLM 调用/2.88M 输入/576k 输出 tokens。
- **失败分类**：plan failure_taxonomy 全集（多标签+最早断点）；行为标签分列（direct_guess/asked_or_searched/self_probed/self_corrected）。
- **止损线**：连续两轮同型环境类失败→止损取证上交（卡面停止条件）；断言/评价失败→取证不重跑（新一轮按预冻计划继续或停段如实报）；fixture 污染/越权写入→立即停批隔离证据（plan stop_rules.pollution_or_leak）。
- **计费**：用户 DeepSeek 账户；per-round telemetry（input/output/cache_read tokens）逐轮入成绩表；cost 字段=missing（价目快照未核，plan budgets.monetary pending），不事后折算。
- **退出码**：0=六轮全评估完成（≠任务成功≠产品验收）；1=driver 层失败；2=环境中断；3=段预算/止损停止（剩余轮如实报 not_run）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分。

## 工件与可回指性

- 每段 run 目录：coord/runs/PLUGIN-CONTROL-EVAL-RUNNER-1/{s0,dryround,segment-p}_<时间戳>/（run_report.json 含 HEAD/分支/内核与 agent SHA256/§8 声明/model env/退出码/teardown 亲核；S0 六案 before/after/restored 快照；P 每轮 readback_before/after+round_record+exchanges/confirmations 全文+agent log delta+telemetry）。
- 遥测/载荷捕获 per-run（env 重定向），agent 日志 copy_* 副本入 run 目录；密钥不入工件（model env 记录键名掩码）。
- 所有工件随 port/plugin-control-eval-runner-1 分支入库；主检出不留未跟踪副本（§6.1）。

## 诚实边界（预声明）

- S0=确定性腿（零 LLM）：证明夹具/接口/目标可达性，不产生任何模型能力结论。
- S1-P=agent HTTP/ZMQ 真栈腿：被测面=产品 chat 链路（语义入口→typed 接管→治理确认→执行→回读），渲染面（webui DOM）不涉及——渲染面与用户旅程门槛（AGENTS §5）不适用于本实验腿，端测边界=S0 确定性腿+P=agent HTTP 真栈腿。
- 单轮成绩=该任务单样本；不据此宣称稳定性（plan §6）。
