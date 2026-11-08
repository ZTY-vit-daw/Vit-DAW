# PLUGIN-CONTROL-EVAL-BR1-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-BR1-1.md（领取 2026-11-08 21:02 @ origin/main=3a0a3ab0→领取提交 dd533344；worktree=D:/Vit_DAW_wt_317br1 干净基线；用户裁定：实验线最高优先级连续推进，领取后直接占栈开跑）
- 本卡=312 终验裁定①的兑现：B-R1 显式薄提示变体，直接测「主动性问题解决后知识转化是否发生」；C 段 go/no-go 随本结果定。

## 冻结提示段（卡面原文，逐字）

> 你有 web.search/web.fetch 可查插件手册与参数文档；遇到不确定的参数编码（param_id/量纲/取值范围）先查再写，查过的不重复查

- UTF-8 长度 76 字符；SHA-256=`51c403d8f8901d586ae1180cdc699ff308dbbc5a870e3fe3db7c2cf68c5a202c`
- 同字节三处锚定：Go 缝常量 `pluginControlEvalSeamBR1PromptSegment`（agent/internal/agentloop/message_loop.go）、Go 配对单测独立字面量+哈希（plugin_control_eval_seam_br1_prompt_test.go）、本装置 seam_verify_br1.py / consolidate_br1.py 的 `BR1_PROMPT_SEGMENT`。

## 注入机制勘察结论（零栈亲证，跑前冻结）

1. **wrapper 层零源码注入系统提示=不可行**：/agent/chat 面只收 message+context（冻结 driver 的 chat_context=`{"agent_mode":"chat"}`）；系统提示在 agent 内部组装（`messageLoopSystemPrompt` → `messageLoopOrdinaryFamilySystemSkeleton`，S4 裁剪即在此层），无任何外部通道可从 HTTP 面到达系统提示。
2. **wrapper 层改用户消息（prepend preamble）可行但被否决**：语义入口分类器（ordinary_agent_semantic_entry）的输入含用户文本——312 裁定已证**能力路由决定 web 对可见性**（路由由语义入口+能力匹配决定），改用户文本会引入路由混杂，破坏 B-R1 vs B 的单变量设计；且 ROUND_MESSAGES 是跨臂冻结控制变量（「模型看到的唯一字符串」），不得改写。分类器输入必须与 B 逐字节一致。
3. **实测面证据（312 终局语料 arm_b_20261108_203434）**：71/71 message_loop 请求的 system 面携带普通族骨架（"You are Ask Vit's DAW ReAct runtime…"）；24/24 message_loop_repair 请求零骨架文本；19 semantic_entry 请求为分类器提示。→ 注入靶面=message_loop 系统面唯一且完备。
4. **采选=卡面授权的最小缝（312 检索缝同模式）**：`VIT_PLUGIN_CONTROL_EVAL_SEAM_BR1_PROMPT`（仅精确 "1"）→ 普通族骨架在全部裁剪（S4+verified-entry）之后追加冻结段为最后一块；默认 unset=骨架逐字节不变；生产启动路径永不设置。
5. **配对单测**：`agent/internal/agentloop/plugin_control_eval_seam_br1_prompt_test.go` 6 项（段字节冻结哈希锚/unset 骨架尾锚字节不变/开=段恰一次为末块/非精确值闭/与 S4 主缝组合/verified-entry 态组合）全绿；四包回归 harness+tools+contextruntime+agentloop 全绿（312 同款关口）。

## 冻结件与对照输入

- 冻结三件=310 快照逐字复用（git blob 核对：cases=d48ca7cf / driver=57abac09 / plan.json=259d6b4d，与 312 runner_frozen 一致）；**B_star 段 caps 原样**：18 rounds/216 calls/8.64M in/1.728M out；per_round_caps 含 max_retrieval_searches_B_C=6/max_retrieval_fetches_B_C=10。
- `br1_driver.py`=312 arm_b_driver.py 包装器同构（冻结 driver 模块级 hook：每轮检索上限执行；**提示注入不在 wrapper——在 agent 缝**），冻结件字节零改动。
- 对照输入：p_baseline/（300 P 轮 capture+summary，blob 6c4db7c4/1a005f14）；a307_reference/（307 A 成绩表 85c5d329+T3_r1 受染负锚）；apure_reference/（310 A-pure 成绩表）；**b_reference/arm_b_summary_312.json（312 B 成绩表）——五方对照输入齐备**。
- 装置门：`seam_verify_br1.py`=312 七面复刻+**R1P 第八面**（段在每个 message_loop 请求 system 面在场[持久性]+probe 载体首 message_loop 请求在场[卡面 R1 亲证]+semantic_entry 零段[分类器无混杂]+repair 零段[缝域边界]）；--selftest 17 腿双向检出力（312 的 12 腿+段剥离 fail/段入分类器 fail/持久性断裂 fail/段入 repair fail/带 repair 干净 pass）；312 B 语料交叉验证=七面零失败、仅 R1P 命中（B→B-R1 增量恰为段）。
- 合并：`consolidate_br1.py`=五方对照（B-R1 vs B312 vs A-pure vs A307 vs P）+检索账本（per-round attempts/queries/URLs/结果哈希/catalog_route_face/段触达）+**写入类失败轮逐轮解剖**+三态观测量块（裁定权在主管）。

## 段构成（单一栈窗口内顺序执行；编排=br1_orchestrator.ps1）

1. **活体检索 probe（零 LLM）**：/agent/invoke web.search+web.fetch；门=①两响应均非 needs_confirmation（312 缝亲证）②search ≥1 结果且 fetch HTTP 200。任一不满足=retrieval_unavailable 环境类 exit 2 不进付费轮。
2. **S0 重冻（零 LLM）**：`br1_driver s0`；门=s0_all_verified=true。fixture_failure（307/310 签名#1 在案）/environment_interrupt；同签名两连败止损；环境重建上限 1 次/批（预冻结）。
3. **dryround 三缝开（零 LLM）**：exit 0+遥测零行。
4. **装置验证 probe（付费 1 轮 T2 repeat 0，非 18 任务轮；T2=全目录载体——312 修正裁定沿用）**：门=seam_verify_br1 **八面**全满足才放行任务轮：
   - S2/S2b/S3/S4/S5/CS/S1+typed 门=310 六面原样（A-pure 面零回归）；
   - R1（312 第七面，轮感知化）：web 对目录行在场（probe 全目录载体）+语料门 R1c（full 形会话必带 web 对）；
   - **R1P（317 第八面）**：冻结段在每个 message_loop 请求 system 面在场（含 probe 载体首个）+semantic_entry/repair 零段；
   - 任一不满足=seam_leakage/装置失效 → exit 4 停段取证。
5. **18 任务轮（付费）**：case 序 T1→T6×repeat 1→3（plan ordering 同 300/307/310/312）；每轮 `br1_driver round`；每轮新 conversation_id+新 fixture+预算内操作与修正+终态独立回读+非目标 diff+拆 fixture。检索 attempts/URLs/时间戳/结果哈希/入上下文长度/catalog_route_face/段触达逐轮入账。
6. **语料级 seam 收口（18 轮后）**：seam_verify_br1 对全 capture 复查八面——任何命中=装置失效旗标（轮 verdict 原样、数据有效性交主管裁定）。
7. **合并与五方对照（离线）**：consolidate_br1.py 汇 18 轮成绩表+检索账本+检索行为分析（可见轮/包路由轮分层发起率+五分法分类+**逐写入类失败轮解剖**）+**五方逐案对照 B-R1 vs B(312) vs A-pure(310) vs A(307) vs P**+三态观测量+检索污染观察表。

## §8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 18（6 案例×3 repeat）；probe 1 轮（T2 repeat 0，装置验证载体，付费不计成绩不占 18 名额）；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结——含装置脚本缺陷类，312 先例）。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted:*）：LLM 调用 12/轮；单调用输入 40k/输出 8k；工具调用 30/轮；检索 search ≤6/轮、fetch ≤10/轮（br1_driver hook 执行，含 browser.* 同族计入）；轮墙钟 20 分钟。
- **段级上限**（超限停段，编排轮间检查）：任务轮合计 ≤216 LLM 调用/≤8,640,000 输入/≤1,728,000 输出 tokens（plan segment_caps.B_star 原样）。probe 预算上界=单轮 caps（12 调用；312 实耗 4 调用/59,535 in 参考）。
- **失败分类**：plan failure_taxonomy 全集（含 retrieval_unavailable/retrieval_miss/retrieval_application_error；分析层五分法映射含 312 修正类 retrieval_not_disclosed_pack_routed vs no_query_initiated_despite_disclosure_and_prompt[317 更名，B 期同名类为 no_query_initiated_despite_disclosure]/segment_missing_device_suspect）；`seam_leakage`（八面任何形态+S5 typed 执行=装置失效优先报告立即停段；语料收口时点发现=旗标上交）；环境中断（栈占用/端口/进程/网络类，LLM 60s 超时按 296 晚窗环境模式分类记录单轮失败不断能力结论）；fixture_failure（签名#1 在案）；harness_invalid（编排/脚本自身缺陷）；行为标签分列。**检索污染止损**：任何 URL/查询命中本仓标识（vit_daw/vit-daw/zty-vit-daw/plugin-control-eval）→记入观察表交主管判污染；该轮不删不改判。
- **止损线**：①活体检索 probe 失败=retrieval_unavailable 停段取证（exit 2）；②probe 门失败=装置失效停段取证（exit 4）；③连续两轮同型环境类失败→止损取证上交；④同代码版本同确定性断点两连败→禁原样重跑先取证；⑤缝泄露/typed 执行/越权写入/段入分类器面→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑；模型面亲证=telemetry 行级 model 字段+capture 响应 raw.model。三臂可比性=与 312 B 同模型同判据同装置，唯一差=系统提示段。
- **计费**：用户 DeepSeek 账户；估 ≈¥2-4 上界（312 实耗 118 调用/1.78M in 同量级；段 ~76 字符×3 字节≈每 message_loop 请求 +~100 tokens，可忽略）+检索工具自身成本如实记录（活体 probe 2 次+逐轮 attempts 计数入账；搜索/fetch=网页抓取无 API 计费）；per-round tokens 逐轮入成绩表；cost 字段=missing（价目未核不事后折算）。
- **退出码**：0=18 轮全评估完成且语料 seam 收口+consolidate 全绿（≠任务成功≠产品验收）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用/活体检索 probe 失败；3=段预算/止损停止；4=装置验证失败（seam_leakage/缝未生效）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分；T4 联动对=307 验收裁定沿用（verdict 原样，分析层旗标，判据修正留 C 批次前）。
- **三态预判对号（卡面原文，回执按此对号）**：①检索被发起且写入类转化提升→知识缺口真实且可检索补（薄提示词策略可行）；②检索被发起但写入仍败→知识非唯一缺口，typed 承重结论加固；③仍零发起→主动性问题非提示可解（产品须结构性引导）。consolidate three_state_observables 出观测量，**状态裁定权在主管**。

## 装置已知边界（预声明，结果解读项）

1. **arm 标签残留**：冻结 driver round_record 硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先例）。B-R1 成绩表以 consolidate_br1.py 重标 "B-R1"；原始记录原样保留（310/312 同裁定）。
2. **browser.* 同族披露**：browser.search/browser.fetch（RiskDirect）与 web 对同在全目录（310 语料亲证；310/312 均零执行）。B-R1 轮模型若自选 browser.* 同计检索族（账本分列）。
3. **确认卡 approve-all 继承**（300 冻结策略，不删）。
4. **最小结构残留**（305/308 验收裁定在案）：quick_controls 选择面+recommended_groups 组名+control_shell 键名列表形态=已声明残留放行；内容对象形态=CS 门拦截。
5. **S5 后已知外观**：goalrunner EQ 守卫指引文本仍指向 plugin_grabber.apply_eq_edits=装置事实、各臂可比（308 裁定③⑤）。
6. **单轮=单样本**；3 repeat 非稳定性主张（plan §6）。B-R1 结果=单段单 run；P 单 repeat。
7. **B-R1 vs B 唯一装置差**=系统提示冻结段（VIT_PLUGIN_CONTROL_EVAL_SEAM_BR1_PROMPT）；八面门保证其余面等价；分类器输入逐字节同 B（R1P 零段断言）。**B-R1 数据不得解读为 C 结论**（C=检索+语义适配器，另卡）。
8. **段×有界披露交互（317 特有）**：薄提示段在**每个** message_loop 轮在场（含包路由轮），但 web 对目录行只在全目录轮在场——包路由轮模型被告知「你有 web.search」而目录/Allowed 不列。这是产品有界披露的真实交互（非装置偏差；312 修正+PENDING-DECISIONS 在案）；发起率解读按可见轮/包路由轮分层（consolidate visibility_strata）。包路由轮内模型对不可见工具的调用尝试按账本如实记录（尝试 vs 执行分列）。

## 工件与可回指性

- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-BR1-1/（本目录）：runner_frozen/（冻结快照+blob）、p_baseline/、a307_reference/、apure_reference/、**b_reference/**（312 B 成绩表）、br1_driver.py、seam_verify_br1.py（八面门+--selftest 17 腿）、consolidate_br1.py（五方对照+检索账本+解剖）、br1_orchestrator.ps1（编排）、RUNBOOK.md（本 §8 冻结）、arm_br1_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256/tool_count/§8 声明/三 seam env 记录/model env 掩码/teardown 亲核；retrieval_live_probe.json；s0_*/dryround/probe/rounds/ 子目录；logs/ 含 telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-br1-1 分支入库，不合入 main（归主管）。

## 诚实边界（预声明）

- 活体检索 probe/s0/dryround=确定性腿（零 LLM）：证明检索可用性/夹具/接口/三缝开管线，不产生模型能力结论。
- 18 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在三缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张；回执显式声明端测边界）。
- 装置验证门=活体 probe+probe 轮八面+语料级收口三层；probe 干净不免疫后轮泄露（307 教训）——发现时点后于轮次完成时，按先例旗标上交主管裁定数据有效性。
- 检索账本=best-effort 配对（attempt 按消息序、结果按工具序；哈希+长度为证据主体，精确配对不承载结论）。
