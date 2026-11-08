# PLUGIN-CONTROL-EVAL-S1B-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-S1B-1.md（领取 1b8f3cda @ origin/main 8cef79ba+3e6ad7d6；用户裁定 2026-11-08：实验不往后排、立即续跑）
- 冻结计划：runner_frozen/plan.json（快照 blob 259d6b4d，与 307/310 runner_frozen/plan.json 逐字一致；**B_star 段 caps 原样**：18 rounds/216 calls/8.64M in/1.728M out/retrieval_within_round_caps=true；per_round_caps 含 max_retrieval_searches_B_C=6/max_retrieval_fetches_B_C=10）
- 冻结 runner：runner_frozen/ 三件=310 runner_frozen 快照逐字复用（blob：cases=d48ca7cf / driver=57abac09 / plan.json=259d6b4d，与 310 分支同名文件 git hash-object 核对一致）。**B 臂新增行为不进冻结件**：`arm_b_driver.py` 包装器以模块级 hook 包装冻结 driver 的 `count_tool_calls`/`budget_breached`（Python 模块全局调用点解析），唯一新增=每轮检索上限执行（超限停轮记 `model_budget_exhausted:retrieval_searches_over_cap` / `retrieval_fetches_over_cap`）；冻结件字节零改动。
- 缝实现：**main 基线（8cef79ba）= 305 S1-S4 + 308 S5 工具目录裁剪 + S2b 渲染面剥离**（单 env `VIT_PLUGIN_CONTROL_EVAL_SEAM=1` 仅精确 "1" 开缝）**+ 本卡新增 312 B 缝**（卡面授权装置类最小缝扩展，305/308 同模式）：
  - 勘察结论（零栈亲证）：web.search/web.fetch 披露面=产品目录自带，310 A-pure 语料 33 请求含 `web_search tool=web.search risk=`/`web_fetch tool=web.fetch risk=` 目录行——**披露无需缝**；执行面唯一门=harness Invoke 的 `needsConfirmation`（web 对=RiskConfirm+confirm=true）。
  - 312 缝：`agent/internal/harness/harness.go` 新增 `VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL`（仅精确 "1"）→ `pluginControlEvalSeamBRetrievalDirect` 使 web.search/web.fetch 跳过确认门直执；其余 RiskConfirm 工具语义零改动；默认 unset=生产行为逐字节不变；生产启动路径永不设置。
  - 配对单测：`agent/internal/harness/plugin_control_eval_seam_b_retrieval_test.go` 6 项（helper 精确值/非精确值闭/闭合=needs_confirmation 停车对/开=到达 webtools 隐私或 URL 校验错误[零网络出证直执]/非 web RiskConfirm 仍停车）全绿；四包回归 harness+tools+contextruntime+agentloop 全绿（305/308 同款关口）。
- 真栈协议：PC-RUNTIME-STACK 净空亲核→登记推 main→≤60s 复核→启栈→run→TEARDOWN_ALL_ZERO 亲核→释放登记。kernel=wt_228 只读复用（SHA256 F9E3DA0A…，与 300/307/296/310 同一二进制）；**agent=本 worktree（wt_312s1b）现场构建，记 SHA256+tool_count**（含 312 缝，跑前缝单测+四包回归在位复跑）。
- 对照输入：p_baseline/=300 P 轮 capture+summary 快照（blob 6c4db7c4/1a005f14）；a307_reference/=307 A 臂成绩表（blob 85c5d329）+T3_r1 受染负锚；**apure_reference/=310 A-pure 成绩表（arm_apure_summary.json）**——四方对照输入齐备。

## 段构成（单一栈窗口内顺序执行）

1. **活体检索 probe（零 LLM，编排 phase 6a，付费网络面无 LLM 计费）**：直接 `/agent/invoke` web.search（query="reverb plugin user manual"）+web.fetch（https://www.example.com）；门=①两响应**均不**为 needs_confirmation（=312 缝在运行 agent 上亲证生效；若停车=装置失效 exit 4）②search 返回 ≥1 结果且 fetch HTTP 200（plan §5「目录在场≠运行环境可用；权限与实际结果必须核查」）。任一不满足=**retrieval_unavailable 环境类 exit 2，不进付费任务轮**（B 臂将测的是环境而非假设）。证据=retrieval_live_probe.json。
2. **S0 重冻（零 LLM）**：`arm_b_driver s0`；成功门=s0_all_verified=true。失败分类 fixture_failure（307/310 签名#1：Waves shell 首载 JUCE 超时在案）/environment_interrupt；同签名 fixture_failure 两连败止损；环境重建上限 1 次/批（预冻结，307/310 先例）。
3. **dryround 双缝开（零 LLM）**：`arm_b_driver dryround`；成功=exit 0+遥测零行。
4. **装置验证 probe（付费 1 轮 T1 repeat 0，非 18 任务轮）**：门=`seam_verify_s1b.py`（--selftest 10 腿双向检出力），**七面**全满足才放行任务轮：
   - S2/S2b/S3/S4/S5/CS/S1+typed 门=310 六面原样（五字段零+P 对照检出力/T3_r1 负锚 ≥7 行/生产尾串零/typed 路由首规则零/目录行 `tool=plugin_grabber.` 零+裸读写对在场/content-form control_shell 零/semantic_entry+message_loop 在场+执行面零 typed 调用）=**A-pure 面零回归**（卡面第 3 点）；
   - **R1（B 特有第七面）**：web 对目录行在场（`web_search tool=web.search risk=` 与 `web_fetch tool=web.fetch risk=` ≥1 请求行）——B 臂缺检索披露面=装置失效；
   - R2（informational）：probe 执行面检索调用计数（模型自发，零调用=合法记录不判失败）；
   - 任一不满足=**seam_leakage/装置失效 → exit 4 停段取证**（不进任务轮，付费如实入账）。
5. **18 任务轮（付费）**：case 序 T1→T6×repeat 1→3（plan ordering 同 300/307/310）；每轮 `arm_b_driver round`（冻结 driver round 面+检索上限 hook）；每轮新 conversation_id+新 fixture+预算内操作与修正+终态独立回读+非目标 diff+拆 fixture。检索 attempts/URLs/时间戳/结果哈希/入上下文长度逐轮入账（consolidate 自 capture 提取，plan §5 口径）。
6. **语料级 seam 收口（18 轮后，编排 phase 8b）**：同 seam_verify_s1b 对全 capture 复查六面+R1——任何命中=装置失效旗标（轮 verdict 原样、数据有效性交主管裁定，307/310 同模式先例）。
7. **合并与四方对照（离线）**：consolidate_s1b.py 汇 18 轮成绩表+逐轮 write_path+typed 门化+**检索账本（per-round attempts/queries/URLs/结果哈希/计数 vs 6/10 caps）+检索行为分析**（哪些案例触发检索/查了什么/是否转化为正确 value encoding——plan §5 五分法：未发起/查不到适用/工具网络拒绝/找到未提取/提取对应用错）+**四方逐案对照表 B vs A-pure(310) vs A(307) vs P**（卡面交付核心=「检索能否补上裸面写入的知识缺口」正式答案）+检索污染观察表（URL/查询命中本仓标识→旗标上交主管判污染）。

## §8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 18（6 案例×3 repeat）；probe 1 轮（装置验证载体，付费不计成绩不占 18 名额）；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结）。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted:*）：LLM 调用 12/轮；单调用输入 40k/输出 8k；工具调用 30/轮；**检索 search ≤6/轮、fetch ≤10/轮（arm_b_driver hook 执行，含 browser.* 同族计入）**；轮墙钟 20 分钟。
- **段级上限**（超限停段，编排轮间检查）：任务轮合计 ≤216 LLM 调用/≤8,640,000 输入/≤1,728,000 输出 tokens（plan segment_caps.B_star 原样）。probe 预算上界=单轮 caps（12 调用；310 实耗 4 调用/58k in 参考）。
- **失败分类**：plan failure_taxonomy 全集（含 retrieval_unavailable/retrieval_miss/retrieval_application_error；本轮分析层五分法映射：no_query_initiated/no_applicable_material/tool_or_network_refused/found_not_extracted/applied_wrong）；`seam_leakage`（六面任何形态+S5 typed 执行=装置失效优先报告立即停段；语料收口时点发现=旗标上交）；环境中断（栈占用/端口/进程/网络类，LLM 60s 超时按 296 晚窗环境模式分类记录单轮失败不断能力结论）；fixture_failure（签名#1 在案）；harness_invalid（编排/脚本自身缺陷）；行为标签分列。**检索污染止损**：任何 URL/查询命中本仓标识（vit_daw/vit-daw/zty-vit-daw/plugin-control-eval）→记入观察表交主管判污染；该轮不删不改判，数据有效性归主管。
- **止损线**：①活体检索 probe 失败=retrieval_unavailable 停段取证（exit 2）；②probe 门失败=装置失效停段取证（exit 4）；③连续两轮同型环境类失败→止损取证上交；④同代码版本同确定性断点两连败→禁原样重跑先取证；⑤缝泄露/typed 执行/越权写入→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑；模型面亲证=telemetry 行级 model 字段+capture 响应 raw.model。
- **计费**：用户 DeepSeek 账户；估 ≈¥3-6 上界（310 实耗 119 调用/1.68M in 同量级；检索轮输入或高于裸面——上限受 12 LLM 调用/轮封顶）+检索工具自身成本如实记录（活体 probe 2 次+逐轮 attempts 计数入账；搜索/fetch=网页抓取无 API 计费，网络成本不入 LLM 账）；per-round tokens 逐轮入成绩表；cost 字段=missing（价目未核不事后折算）。
- **退出码**：0=18 轮全评估完成且语料 seam 收口+consolidate 全绿（≠任务成功≠产品验收）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用/**活体检索 probe 失败（retrieval_unavailable）**；3=段预算/止损停止；4=装置验证失败（seam_leakage/B 缝未生效）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分；T4 联动对=307 验收裁定沿用（verdict 原样，分析层旗标，判据修正留 C 批次前）。

## 装置已知边界（预声明，结果解读项）

1. **arm 标签残留**：冻结 driver round_record 硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先例）。B 成绩表以 consolidate_s1b.py 重标 "B"；原始记录原样保留（310 A-pure 同裁定）。
2. **browser.* 同族披露**：browser.search/browser.fetch（RiskDirect，无需确认）与 web 对同在产品目录（310 语料亲证 33 请求含 browser.search 目录行；310 A-pure 18 轮零执行）。B 轮模型若自选 browser.* 同计检索族（账本分列）；310 A-pure 零检索=「模型未自发检索」事实在卷，B 的检索增量解读须以此为基线。
3. **确认卡 approve-all 继承**（300 冻结策略）：B 缝使 web 对直执后，确认面理论上只剩其他 RiskConfirm 工具；冻结策略仍 approve-all 在位（不删）。
4. **最小结构残留**（305/308 验收裁定在案）：quick_controls 选择面+recommended_groups 组名+control_shell 键名列表形态=已声明残留放行；内容对象形态=CS 门拦截。
5. **S5 后已知外观**：goalrunner EQ 守卫指引文本仍指向 plugin_grabber.apply_eq_edits（目录已无该工具）=test-only 态装置事实、各臂可比（308 裁定③⑤）。
6. **单轮=单样本**；3 repeat 非稳定性主张（plan §6）。B 结果=单段单 run；P 单 repeat。
7. **B vs A-pure 唯一装置差**=SEAM_B_RETRIEVAL env（web 对直执）；六面零回归门保证其余面等价。**B 数据不得解读为 C 结论**（C=检索+语义适配器，另卡）。

## 工件与可回指性

- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/（本目录）：runner_frozen/（冻结快照+blob）、p_baseline/（P 对照）、a307_reference/（307 A 成绩表+T3_r1 受染负锚）、apure_reference/（310 A-pure 成绩表）、arm_b_driver.py（冻结 driver 包装=检索上限 hook）、seam_verify_s1b.py（七面门+--selftest）、consolidate_s1b.py（四方对照+检索账本+行为分析）、arm_b_orchestrator.ps1（编排）、RUNBOOK.md（本 §8 冻结）、arm_b_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256/tool_count/§8 声明/双 seam env 记录/model env 掩码/teardown 亲核；retrieval_live_probe.json；s0_*/dryround/probe/rounds/ 子目录；logs/ 含 telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-s1b-1 分支入库，不合入 main（归主管）。

## 修正节（2026-11-08 20:4x，付费任务轮启动前冻结——首跑取证后装置修正）

- **首跑取证（arm_b_20261108_202627，保留不删）**：活体检索 probe PASS（web.search ok 3 结果 bing_html/web.fetch HTTP 200/零确认停车=312 缝在运行 agent 亲证生效）+s0/dryround PASS+probe T1_r0 付费 4 调用/59,535 in/2,259 out（记录不评分）+六面全 PASS+S5 裸读写对在场 2/2→**R1 FAIL（0 web 目录行）→exit 4 停段**，编排 finally 亲拆 TEARDOWN_ALL_ZERO。
- **根因（装置脚本缺陷，非 agent 缝缺陷）**：R1 原前提「web 对恒在模型目录」为假——ordinary loop 目录披露是**路由的**（agentLoopToolContext）：能力零命中→全目录（310 形态 209 行含 web 对；310 语料 T2/T3 六会话）；能力命中→过滤包（42/51 行无 web 对；310 语料 T1/T4/T5/T6 十三会话含 probe）。310 语料逐会话核对在案（33 web 行 payload 全部 message_loop/T2/T3 会话）。首跑 probe 载体=T1（包路由）故 R1 必然失败——门正确拦住了错误前提。
- **修正（仅装置脚本，agent 源码与二进制零改动——首跑 agent_sha256=1EBCF09F…与重跑同）**：①R1 轮感知化：probe 载体改 **T2 repeat 0**（全目录载体，R1 可验）；②语料门 R1c：每个 full 形会话必须带 web 对（缺失=装置从全目录剥离了检索面=失效）；包路由会话如实记录（产品有界披露，不判失效）；③selftest 增至 12 腿（full 缺 web→fail/full 带 web→pass 双向检出力）；④consolidate 检索账本加每轮 catalog_route_face（full/pack+web_pair_present），分类加 `retrieval_not_disclosed_pack_routed`（不可见）vs `no_query_initiated_despite_disclosure`（可见而不查）。
- **修正后离线验证**：seam_verify_s1b 对 310 全语料（119 payload/19 会话）=PASS（6 full+13 pack 分类全对/full 缺 web=0）；selftest 12/12；consolidate 310 数据=12 包路由+6 可见未查（与 310 证据一致）。
- **重跑授权**：装置脚本缺陷类=环境重建类，预冻结上限 1 次/批之内（新 run 目录不覆盖）；首跑付费 4 调用如实入账不评分。
- **解读边界（预声明，B 答案的一部分）**：检索处置（treatment）只在全目录轮（T2/T3 型消息）对模型可见——产品有界披露真相，非装置偏差；四方对照中 B 的检索增量解读必须按「可见轮/不可见轮」分层（consolidate 已按轮记录）。

## 诚实边界（预声明）

- 活体检索 probe/s0/dryround=确定性腿（零 LLM）：证明检索可用性/夹具/接口/缝开态管线，不产生模型能力结论。
- 18 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在双缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张）。
- 装置验证门=活体 probe+probe 轮七面+语料级收口三层；probe 干净不免疫后轮泄露（307 教训）——发现时点后于轮次完成时，按先例旗标上交主管裁定数据有效性。
- 检索账本=best-effort 配对（attempt 按消息序、结果按工具序；哈希+长度为证据主体，精确配对不承载结论）。
