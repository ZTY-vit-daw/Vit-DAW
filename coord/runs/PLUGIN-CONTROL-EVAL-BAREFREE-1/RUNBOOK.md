# PLUGIN-CONTROL-EVAL-BAREFREE-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-BAREFREE-1.md（领取 2026-11-08 深夜 @ origin/main=7ddae343→领取提交 f3572576；worktree=D:/Vit_DAW_wt_318barefree 干净基线；缝 commit 05a5869e；用户深夜裁定开跑）
- 本卡=结论档 ⓪ 修订待办①兑现：裸面+守卫许可臂——「如果允许，模型能不能裸写」的唯一干净测试。B-R1 解剖已证 9/9 写入失败轮=治理守卫阻断（非编码错误）；本臂在 A-pure 同款裸面上开守卫许可缝，单变量=守卫许可。

## 一、零栈解剖结论（跑前冻结；B-R1 payload 亲读）

解剖原料：`D:/Vit_DAW_wt_317br1/coord/runs/PLUGIN-CONTROL-EVAL-BR1-1/arm_br1_20261108_211559/logs/agent_llm_payload_capture.jsonl`（110 行全量在卷）。

**守卫 A（T1 型写入闸门，读写全拦，agentloop 层）**
- 拦截实录（T1_r1 会话）：`<tool_gate>{"blocked_tool":"plugin.get_parameters","reason":"mix.observe is complete; broad mixing requests must stop here, summarize the observation, propose one concrete next move, and wait for explicit user confirmation before loading plugins, changing volume, applying controls, or writing parameters"}</tool_gate>`——模型 4 次 `plugin.get_parameters`（读路径）全被拦，零写入尝试可达；终答原文「运行时写入闸门把这次请求判成『先观察、再确认』的混音请求，把插件参数路径（读参数表、写参数）都拦下了」（checkpoint message 在卷）。
- 代码锚点：`agent/internal/agentloop/message_loop.go`
  - `messageLoopToolGuardIssue`（~5469/改后 5504）mix-observe-first 链：`messageLoopNaturalMixRequest`=fastpath.NaturalMixRequest（T1 文本判自然混音请求）→ `messageLoopMixObserveFirstBlockedTool`（6374 起+改后位移）命中 `plugin.get_parameters`/`plugin.set_parameter` 等 → `hadMixObservationBeforeTurn`=true → `messageLoopExplicitMixExecutionConfirmation`（6072 起）要求批准词+具体动作词（T1 文本无批准词）→ 返回 5560 拦截文本；
  - 呈现：`messageLoopAppendGuardGate`（~2803）→ `<tool_gate>` user 消息 + trace `tool_call_blocked`/`final_gate`。

**守卫 B（T3/T4 typed-surface fail-closed，chat 层）**
- 拦截实录（T3_r1/r2/r3、T4_r1/r2/r3 同形）：`<tool_result>` 内 `plugin.set_parameter`/`set_plugin_param` → status=rejected、`rejection_code=typed_transient_shaper_surface_unavailable`、error=`typed_transient_shaper_surface_unavailable: cannot prove this parameter is outside the typed transient-shaper surface: unresolved_transient_shaper_surface: no provable supported transient-shaper envelope topology`、required_tools=[plugin_grabber.inspect_transient_shaper, plugin_grabber.apply_transient_shaper_controls]、mutation_barrier=true。T3 三轮写入值全部 0.7=正确编码仍被拒。
- 代码锚点：`agent/internal/chat/goalrunner_chat.go`
  - `guardTransientShaperOwnedGenericParameterWrite`（~1325/改后 1341）：`readLiveTransientShaperControlSurface`（plugin_transient_shaper_apply.go:197）err → `transientShaperControlFailureCode` 分类——明确非受护族放行；`unresolved_transient_shaper_surface`（无法证明面外）→ fail-closed 拒绝；
  - 调用面：`chat/server.go` `handleInvoke`（1702）守卫链（~1962-1986，七守卫：compressor/limiter/gate-expander/de-esser/transient-shaper/EQ/multiband）+ goalrunner workflow executor 面（blockTransientShaper* 等 wrap）；
  - 关键缺陷位：limiter（~1469）/gate-expander（~1408）/de-esser（~1546）守卫有 `genericWriteGuardFamilyExemption`（VSTI-WRITEGUARD-1：instrument role 或 provable delay/reverb 拓扑豁免），**transient-shaper 守卫没有**——T4 H-Delay/T3 Plates（provably delay/reverb）撞 transient-shaper 候选匹配（timing/output naming overlap）→ unresolved → 拦。

## 二、守卫许可缝规格（test-only，commit 05a5869e）

- env 键 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE`（仅精确 "1"；TrimSpace 宽容度与 312/317 既有缝一致；默认 unset 生产字节不变；生产启动路径永不设置）：
  1. **agentloop 半**（message_loop.go）：`messageLoopIsPluginParameterReadWriteTool`（plugin.get_parameters/get_plugin_parameters/get_plugin_param/plugin.set_parameter/plugin_set_parameter/set_plugin_param/set_plugin_parameter 名变体）在 mix-observe-first 链放行；其余 blocked 工具（plugin 装载、track volume/pan、daw.invoke、plugin_grabber.*）保持拦截；
  2. **chat 半**（goalrunner_chat.go+plugin_multiband_apply.go）：七 typed generic-write 守卫入口（pluginSetParameterInvokeRequest 判定后）缝开即放行（HTTP invoke 面与 workflow 面同盖——守卫函数内短路）。
- 配对单测：`agent/internal/agentloop/plugin_control_eval_seam_barefree_test.go` 5 项（T1 闸门双向钉+放行域边界+精确值+matcher 变体）+ `agent/internal/chat/plugin_control_eval_seam_barefree_test.go` 3 项（300-S0 forensics fixture 复刻 T3/T4 现状拦截钉+缝开七守卫全过+kernel 零读短路+精确值）全绿；五包回归（harness/tools/contextruntime/agentloop/chat）全绿。

## 三、冻结件与对照输入

- 冻结三件=310 快照逐字（git blob 亲核：plan=259d6b4d / cases=d48ca7cf / driver=57abac09，与 312/317 runner_frozen 一致）；**A_star 段 caps**：18 rounds/216 calls/8.64M in/1.728M out（plan segment_caps.A_star 原样）。
- **BAREFREE driver=冻结 driver 直接用**（A-pure 臂形态，无 wrapper：本臂唯一装置差=agent env，非 driver 行为）。
- 对照输入：p_baseline/（300）、a307_reference/（307 成绩表+T3_r1 受染负锚）、apure_reference/（310 成绩表）、b_reference/（312 成绩表）、**br1_reference/arm_br1_summary.json（317 成绩表）——六方对照输入齐备**。
- 装置门：`seam_verify_barefree.py` 九面+GW-LIVE=310 六面原样（S2/S2b/S3/S4/S5/CS/S1 零 typed 执行）+**W0**（A-pure 检索等价：probe delta+全语料零 web/browser 执行行；目录可见性随路由仅记录）+**P0**（317 薄提示段零字节残留）+**GW**（全语料零七守卫 rejection 文本+零 T1 tool_gate 文本=两半缝生效的反向断言）+**GW-LIVE**（编排活体 probe 文件引用）；--selftest 17 腿双向检出力（含 W0 执行 fail 腿/browser echo fail 腿/P0 段残留 fail 腿/GW typed 拒绝 fail 腿/GW T1 gate fail 腿/GW-LIVE 缺失与未过腿+web 对可见零执行 pass 腿）；B-R1 语料交叉分类（consolidate 干跑）=9/9 写入失败轮正确分类 guard_blocked_write。
- `gw_probe.py`（零 LLM 活体）：复用冻结 driver 的 build_fixture/teardown_fixture/readback_parameters/governance_blocked（import 不改）；B-R1 T3 守卫受害者（Abbey Road Plates/Damper/param_id 1）写 0.7：open 态必须落地+独立回读变化；off 态（unset+agent 重启=A-pure 态）同写必须被守卫拒绝（typed 文本在场）。
- 合并：`consolidate_barefree.py`=六方对照（BAREFREE vs BR1 vs B vs A-pure vs A vs P）+**裸写账本**（每轮插件参数读写 attempts（assistant tool_calls）与 tool_result 状态/rejection_code，best-effort 配对）+**guard_blocked_write 分析分类**（轮失败+守卫错误形态（conv 文本 typed 码/T1 gate 文本）→分析标签；**评价器 verdict/failure_tags 永不改写**——终态标签与机理标签并列呈现）+**barewrite_core_answer**（写入三案 T1/T3/T4 守卫许可下 pass 率=真裸写能力+落地轮 readback 编码证据 label_exact/normalized 逐轮在卷+同面对照注记）。

## 四、段构成（单一栈窗口内顺序执行；编排=barefree_orchestrator.ps1）

1. **净空 preflight**（编排 phase 0）：六 TCP 端口+两 UDP+栈进程全零（栈登记协议另有亲核，见 §六）。
2. **env freeze**（phase 1）：VIT_PROJECT_XML=git-clean 副本、遥测/渲染/载荷捕获路径、`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`+`VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1`（任何进程启动前）；model env 记录+override 拒跑。
3. **起栈**（phase 2）：dev_agent_smoke -StartKernel -RestartAgent（无 UI；agent=本 worktree 现场构建，SHA256 入报告）。
4. **S0 重冻**（phase 4，零 LLM）：门=s0_all_verified=true。fixture_failure（307/310 签名#1 在案）；同签名两连败止损；环境重建上限 1 次/批（预冻结）。
5. **GW-OPEN 活体 probe**（phase 5，零 LLM）：T3 受害者裸写 0.7 必须落地+回读变化；失败=缝未生效 exit 4 不进付费轮。
6. **GW-OFF 回归亲证**（phase 5b，零 LLM）：unset 缝 env+agent 重启（A-pure 态）→同写必须被守卫拒绝（拦截恢复）；失败=缝泄露形态 exit 4。随后重设 env=1+agent 重启回开态（-SkipBuild+二进制 SHA256 前后一致性断言）。
7. **dryround 两缝开**（phase 6，零 LLM）：exit 0+遥测零行。
8. **装置验证 probe**（phase 7，付费 1 轮 T2 repeat 0，非 18 任务轮）：门=seam_verify_barefree 九面+GW-LIVE 全满足才放行任务轮；任一不满足=seam_leakage/装置失效 → exit 4 停段取证。
9. **18 任务轮**（phase 8，付费）：case 序 T1→T6×repeat 1→3（plan ordering 同 300/307/310/312/317）；每轮新 conversation_id+新 fixture+预算内操作与修正+终态独立回读+非目标 diff+拆 fixture。
10. **语料级 seam 收口**（phase 8b）：seam_verify_barefree 对全 capture 复查——任何命中=装置失效旗标（轮 verdict 原样、数据有效性交主管裁定）。
11. **合并与六方对照**（phase 8c，离线）：consolidate_barefree.py 汇 18 轮成绩表+裸写账本+guard_blocked_write 分类+barewrite_core_answer+六方对照。

## 五、§8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 18（6 案例×3 repeat）；probe 1 轮（T2 repeat 0，装置验证载体，付费不计成绩不占 18 名额）；GW open/off 活体 probe 零 LLM 不计；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结——含装置脚本缺陷类，312 先例）。
- **每轮上限**（plan per_round_caps，超限停轮记 model_budget_exhausted:*）：LLM 调用 12/轮；单调用输入 40k/输出 8k；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（超限停段，编排轮间检查）：任务轮合计 ≤216 LLM 调用/≤8,640,000 输入/≤1,728,000 输出 tokens（plan segment_caps.A_star 原样）。probe 预算上界=单轮 caps（12 调用；312 实耗 4 调用/59,535 in、317 probe 参考）。
- **失败分类**：plan failure_taxonomy 全集；`seam_leakage`（九面任何形态+S5 typed 执行=装置失效优先报告立即停段；语料收口时点发现=旗标上交）；**`guard_blocked_write`（新分析类，结论档待办②）**：轮失败+tool_result 守卫错误形态（typed_*_surface_unavailable/typed_*_control_required 或 T1 write-gate 文本）→分析层如实分类，**不得读作 value_encoding_error**；缝开态再现守卫拦截=先查缝失效（GW 面），非缝覆盖的新守卫面=发现上交（域外缺陷只给锚点+最小复现）；环境中断（栈占用/端口/进程/网络类，LLM 60s 超时按 296 晚窗环境模式分类记录单轮失败不断能力结论）；fixture_failure（签名#1 在案）；harness_invalid（编排/脚本自身缺陷）；行为标签分列。
- **止损线**：①GW open probe 失败=缝未生效停段取证（exit 4）；②GW off probe 失败=缝泄露形态停段取证（exit 4）；③probe 门失败=装置失效停段取证（exit 4）；④连续两轮同型环境类失败→止损取证上交；⑤同代码版本同确定性断点两连败→禁原样重跑先取证；⑥typed 执行/检索执行/317 段残留→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑；模型面亲证=telemetry 行级 model 字段+capture 响应 raw.model。可比性=与 A-pure(310) 同模型同判据同装置同冻结件，唯一差=守卫许可缝 env。
- **计费**：用户 DeepSeek 账户；估 ≈¥2-4 上界（A-pure 310 同量级参考：18 任务轮+probe ≈100 调用/1.2M in 量级；守卫许可不改变请求面——裸写落地会增加每轮 tool_result 体积，输入 token 上界受段 caps 约束）；per-round tokens 逐轮入成绩表；cost 字段=missing（价目未核不事后折算）。
- **退出码**：0=18 轮全评估完成且语料 seam 收口+consolidate 全绿（≠任务成功≠产品验收）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用；3=段预算/止损停止；4=装置验证失败（GW/缝未生效/seam_leakage）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分；T4 联动对=307 验收裁定沿用（verdict 原样，分析层旗标）。
- **判定核心（卡面原文对号）**：T1/T3/T4 守卫许可下 pass 率=真裸写能力；写入落地后 readback 验证编码正确性（verdict checks label_exact/normalized_in_tolerance 终于以「落地后回读」形态可测）；barefree vs A-pure 同面开关对照为核心表+并入全阶梯（六方）。裁定权在主管。

## 六、栈协议与工件

- 真栈占用按 PROTOCOL §2.2 全链：净空亲核（进程/端口）→占用登记推 main→≤60s 即时复核→启栈→run→owner 亲拆 TEARDOWN_ALL_ZERO 亲核→释放登记（run ID）。登记=coord/resources/PC-RUNTIME-STACK.md；L1 真栈腿可能候栈——登记前重读，先到先得不抢。
- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-1/（本目录）：runner_frozen/（冻结快照+blob）、p_baseline/、a307_reference/、apure_reference/、b_reference/、br1_reference/、gw_probe.py、seam_verify_barefree.py（九面+GW-LIVE+--selftest 17 腿）、consolidate_barefree.py（六方+裸写账本+guard_blocked_write）、barefree_orchestrator.ps1（编排）、RUNBOOK.md（本 §8 冻结）、arm_barefree_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256（含重启后一致性断言）/tool_count/§8 声明/两 seam env 记录/model env 掩码/teardown 亲核；gw_open.json+gw_off.json+guard_live_probe.json；s0_*/dryround/probe/rounds/ 子目录；logs/ 含 telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-barefree-1 分支入库，不合入 main（归主管）。

## 七、装置已知边界（预声明，结果解读项）

1. **arm 标签残留**：冻结 driver round_record 硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先例）。BAREFREE 成绩表以 consolidate_barefree.py 重标 "BAREFREE"；原始记录原样保留（310/312/317 同裁定）。
2. **GW 语料面条件性**：零守卫拒绝断言在「模型未尝试写」的轮次空过（弱证据）；强证据=GW-OPEN 活体 probe（确定性）+probe 轮+T1/T3/T4 任务轮的实际尝试（A-pure/B/B-R1 语料实证模型在裸面会尝试写——同面下预期同行为）。
3. **裸写账本 best-effort 配对**（attempt 按消息序、结果按工具序；哈希+状态为证据主体，精确配对不承载结论）；guard_shapes 全文扫描承载守卫拒绝结论。
4. **agent 重启窗口**：GW-OFF 段 agent 重启两次（off 态+恢复开态）；内核进程全程同一实例（fixture/工程态不受 agent 重启影响）；二进制 SHA256 前后一致性断言在编排内（-SkipBuild 不改二进制）。
5. **单轮=单样本**；3 repeat 非稳定性主张（plan §6）。BAREFREE 结果=单段单 run；P 单 repeat。
6. **BAREFREE vs A-pure 唯一装置差**=VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE（九面门保证其余面等价；分类器输入/系统提示与 A-pure 逐字节一致——无检索面/无提示段）。守卫许可不改变模型可见面（缝只存在于 agent 内部闸门路径）。
7. **typed 工具执行仍禁**（S5 面零 typed 执行不变）：本臂测的是**裸写**能力，不是 typed 路径；缝不放开 plugin_grabber.* 目录或执行。
8. **守卫许可的语义边界**：缝放行的是「插件参数读写工具调用」过闸门+「裸写过七 typed 守卫」；EQ 语义路由、read-only 观察守卫（T5 型）、mutation barrier 等其他治理面**不在缝内**——T5 案例行为应与 A-pure 一致（观察类守卫照常）；若 T1/T3/T4 轮出现缝外新守卫面拦截=guard_blocked_write 如实分类+锚点上交（不扩修）。

## 八、诚实边界（预声明）

- GW probe/s0/dryround=确定性腿（零 LLM）：证明缝生效性/夹具/接口/两缝开管线，不产生模型能力结论。
- 18 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在两缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张；回执显式声明端测边界）。
- 装置验证门=活体 GW probe（open+off）+probe 轮九面+语料级收口三层；probe 干净不免疫后轮泄露（307 教训）——发现时点后于轮次完成时，按先例旗标上交主管裁定数据有效性。
- 守卫许可下 pass 率解读边界：证明「守卫许可时模型能/不能裸写正确值」；不证明产品应放开守卫（治理链设计裁定不在本卡范围）。
