# PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-BAREFREE-T1-1.md（领取 2026-11-08 深夜 @ origin/main=51a41909→领取提交 8b43e461；worktree=D:/Vit_DAW_wt_319t1 干净基线；缝 commit 见回执；栈登记链接见回执）
- 本卡=318 终验裁定①兑现：T1 补完腿——第三层 harness 守卫缝 + T1×3 定向复跑。318 已证 T3 3/3（真裸写能力）+T4 0/3（真实编码边界），T1 0/3 是**第三层守卫拦截**（`broadMixObserveFirstWriteGuard`，harness.go:2590+Invoke~1040，`pre_journal_validation_failed stage=broad_mix_observe_first_write_guard` 三轮在卷）——T1 真裸写能力**未测及**。本腿在 318 同款 env 上把同键缝补到第三层，单变量=第三层守卫许可，测最后一个案例。

## 一、缝规格（test-only，默认 unset 字节不变）

- env 键 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE`（与 318 agentloop/chat 两半同键同语义：仅精确 "1"，TrimSpace 宽容；生产启动路径永不设置）。
- **harness 第三层半**（本卡新增，agent/internal/harness/harness.go）：`broadMixObserveFirstWriteGuard` 在 `broadMixWriteCommand` 判定后、qualified 豁免前——缝开+`broadMixPluginParameterWriteCommand`（镜像 broadMixWriteCommand 的插件参数写条目：set_plugin_param/plugin.set_parameter/plugin_set_parameter）→ 放行；其余 broad-mix 写（装载/音量/声像/发送/宏）保持拦截；retired 工具检查在缝之前（retired 面永不豁免）。
- 配对单测：`agent/internal/harness/plugin_control_eval_seam_barefree_t1_test.go` 7 项（守卫级缝关拦/缝开过/缝外 broad-mix 写仍拦×4/精确值+TrimSpace/retired 不豁免/Invoke 级双向 pre-journal 亲证）全绿；harness 全包+五包回归（harness/tools/contextruntime/agentloop/chat）全 exit 0。
- **harness.go 域预声明兑现**：diff 只增守卫函数区 37 行，不触 Authorize* 家族（2223-2248 一带，273 未合入分支域）。

## 二、装置（318 全套直接复用 + 三件新）

- 冻结三件=318 runner_frozen 逐字复用（blob 259d6b4d/d48ca7cf/57abac09，自 318 工件目录 import 不复制不修改）；driver round 子命令原生支持 `--case T1 --repeat N`（T1×3 定向无需改 driver）。
- **gw_probe_t1.py**（新，零 LLM）：T1 型第三层 probe——Pro-R param 6（Mix）写 0.35，**invoke context 携带 T1 任务原文**「请把 Pro-R 混响的 Mix 设为 35%。」（engages broad-mix guard：含 "mix" 自然请求词+无 ExplicitPluginRequest 动词）；open=落地+独立回读变化；off=守卫拒绝+值不变。基线=冻结初态 100%/1.0（build_fixture 针钉）。
- **seam_verify_t1.py**（318 版复制扩展）：310 六面+W0+P0+GW 原样；**GWH 新建议面**（harness broad-mix 错误文本计数——缝开态非参数 broad-mix 写仍会被拦、文本可合法出现，故计数上报不门禁；机理归层归 consolidate 逐轮）；**GW-LIVE-T1 新门面**（t1 open/off 双 ok）；selftest 21 腿双向检出力（含 T1 面缺失/未过腿+GWH 计数腿）。
- **consolidate_t1.py**（318 版复制变体）：CASES=[T1]、caps=3 轮/36 调用/1.44M in/288k out（plan A_star 按 3/18 缩放）；guard_blocked_write 分类维持（缝开态 harness write-guard 命中参数写=缝失效证据，非模型失败）；七方对照（BAREFREE-T1 vs BAREFREE-318 vs BR1 vs B312 vs A-pure vs A307 vs P，T1 列）；318 语料干跑验证=3 轮全分类 guard_blocked_write:harness_write_guard、A307 3/3、P pass 在卷。
- **barefree_t1_orchestrator.ps1**（318 编排同模式）：净空 preflight→env freeze（两 seam env 任何进程启动前）→起栈（本 worktree 现场构建 agent，SHA256 入报告）→S0 重冻→GW open×2（T3+T1）→GW off×2（unset+agent 重启=A-pure 态）→恢复开态（二进制 SHA256 前后一致断言）→dryround→probe 轮（T1 r0，付费不计成绩）→seam_verify_t1 门→T1×3→语料收口→consolidate→owner 亲拆。

## 三、段构成（单一栈窗口内顺序执行）

同 318 §四（phase 0-8c），差异：phase 5=双味 open probe；phase 5c=双味 off probe；phase 7 probe 载体=T1 r0；phase 8=T1×3。

## 四、§8 声明（运行次数/预算/失败分类/止损）

- **运行次数**：单 run 无重跑（run_id=arm_barefree_t1_<启动时间戳>，新目录不覆盖）。任务轮恰好 3（T1×repeat 1..3，plan ordering 内 T1 段）；probe 1 轮（T1 repeat 0，装置验证载体，付费不计成绩不占 3 名额）；GW open/off 活体 probe 零 LLM 不计；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才排除，补跑新 run_id，环境重建上限 1 次/批（预冻结，含装置脚本缺陷类）。
- **每轮上限**（plan per_round_caps 原样，超限停轮记 model_budget_exhausted:*）：LLM 调用 12/轮；单调用输入 40k/输出 8k；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（plan A_star 按 3/18 缩放，编排轮间检查）：任务轮合计 ≤36 LLM 调用/≤1,440,000 输入/≤288,000 输出 tokens；probe 预算上界=单轮 caps（12 调用）。
- **失败分类**：plan failure_taxonomy 全集；seam_leakage（十二面任何门禁形态=装置失效优先报告立即停段；语料收口时点发现=旗标上交）；**guard_blocked_write（318 分类维持）**：轮失败+tool_result 守卫错误形态（typed_* 码/T1 gate 文本/harness write-guard stage）→分析层如实分类；**本腿注记：缝开态 harness write-guard 命中插件参数写=第三层缝失效证据（装置类），非模型 value_encoding_error**；环境中断（栈占用/端口/进程/网络类）；fixture_failure（307/310 签名#1 在案）；harness_invalid（编排/脚本自身缺陷）。
- **止损线**：①GW open（任一味）失败=缝未生效停段取证（exit 4）；②GW off（任一味）失败=缝泄露形态停段取证（exit 4）；③probe 门失败=装置失效停段取证（exit 4）；④连续两轮同型环境类失败→止损取证上交；⑤同代码版本同确定性断点两连败→禁原样重跑先取证；⑥typed 执行/检索执行/317 段残留→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **模型面**：产品默认 ~/.vit/config.json（defaultModel=deepseek-flash）；VIT_AGENT_LLM_MODEL/OPENAI_MODEL env 设置时拒跑；模型面亲证=telemetry 行级 model 字段+capture 响应 raw.model。可比性=与 BAREFREE-318/A-pure 同模型同判据同装置同冻结件，唯一差=第三层缝（同键）。
- **计费**：用户 DeepSeek 账户；卡面预算 ≈¥1 上界（3 轮×~13 调用级+probe；318 参考 18 轮+probe=108 调用/1.58M in ≈¥2-3 → 本腿 1/6 量级）；per-round tokens 逐轮入成绩表；cost=missing（价目未核不事后折算）。
- **退出码**：0=3 轮全评估完成且语料 seam 收口+consolidate 全绿（≠任务成功≠产品验收）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用；3=段预算/止损停止；4=装置验证失败（GW/缝未生效/seam_leakage）。
- **评价独立面**：verdict 只由 readback+行 diff 产生（评价器期望值来自冻结 cases 模块，从未进入被测模型上下文）；模型自述不计分；T1 判据=label/归一化/非目标零扰动（冻结 cases T1 条目：display 35±2.0）与 318/全阶梯逐字同。
- **判定核心（卡面对号）**：T1 三轮守卫全许可下 pass 率=最后一个案例的真裸写答案；guard_blocked_write 分类维持；BAREFREE-T1 vs BAREFREE-318=第三层同面开关对照。

## 五、栈协议与工件

- 真栈占用按 PROTOCOL §2.2 全链：净空亲核（进程/端口）→占用登记推 main→≤60s 即时复核→启栈→run→owner 亲拆 TEARDOWN_ALL_ZERO 亲核→释放登记（run ID）。登记=coord/resources/PC-RUNTIME-STACK.md；L1 真栈腿可能候栈——登记前重读，先到先得不抢。
- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-T1-1/（本目录）：gw_probe_t1.py、seam_verify_t1.py（十二面+--selftest 21 腿）、consolidate_t1.py（七方+裸写账本+guard_blocked_write）、barefree_t1_orchestrator.ps1、RUNBOOK.md（本 §8 冻结）、arm_barefree_t1_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256（含重启后一致性断言）/§8 声明/两 seam env 记录/model env 掩码/teardown 亲核；gw_open.json+gw_off.json+guard_live_probe.json+gw_t1_open.json+gw_t1_off.json+guard_live_probe_t1.json；s0_*/dryround/probe/rounds/ 子目录；logs/ 含 telemetry+prompt render+payload capture+agent log 副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。
- 全部工件随 port/plugin-control-eval-barefree-t1-1 分支入库，不合入 main（归主管）。

## 六、装置已知边界（预声明）

1. **arm 标签残留**：冻结 driver round_record 硬编码 "arm":"P"、conversation 前缀 pce_p_（300 先例）。成绩表以 consolidate_t1.py 重标 "BAREFREE-T1"；原始记录原样保留。
2. **probe 载体差异**：318 probe=T2 r0（全目录载体裁定），本腿 probe=T1 r0（本腿目标案例即载体；S1 面只要求 probe 轮有 semantic_entry+message_loop 在卷，T1 轮满足；目录面差异由 W0/检索等价面与 318 逐字节同 env 保证）。
3. **GWH 建议面边界**：harness broad-mix 错误文本在缝开态可合法出现（非参数写仍拦），故语料面计数不门禁；第三层缝失效的确定性证明=GW-LIVE-T1 活体 probe+配对单测；逐轮机理归层=consolidate（ledger 记录被拦工具名）。
4. **T1 off probe 归层证据**：off 态同写可能被 chat typed 守卫（Pro-R 若撞 transient-shaper 候选匹配）或 harness broad-mix 守卫先拦——probe 记录 broad_mix_text_present/typed_text_present 哪个在场；「拦截恢复」的硬判据=任一治理拒绝+值不变；harness 层恢复的确定性证明=配对单测（Invoke 级双向）。
5. **单轮=单样本**；3 repeat 非稳定性主张（plan §6）。BAREFREE-T1 结果=单段单 run。
6. **BAREFREE-T1 vs BAREFREE-318 唯一装置差**=第三层缝（同 env 键）；分类器输入/系统提示与 318 逐字节一致。守卫许可不改变模型可见面。
7. **typed 工具执行仍禁**（S5 面零 typed 执行不变）。
8. **守卫许可的语义边界**：缝放行的是「插件参数写」过 harness 第三层（+318 两半）；EQ 语义路由、read-only 观察守卫（T5 型）、mutation barrier 等其他治理面不在缝内；若 T1 轮出现缝外新守卫面拦截=guard_blocked_write 如实分类+锚点上交（不扩修）。

## 七、诚实边界（预声明）

- GW probe/s0/dryround=确定性腿（零 LLM）：证明缝生效性/夹具/接口/三缝开管线，不产生模型能力结论。
- 3 任务轮=agent HTTP/ZMQ 真栈腿（产品 chat 链路在三缝开 env 下的普通工具循环）；渲染面（webui DOM）与用户旅程不涉及（实验腿不适用 AGENTS §5 渲染面门槛，无 E2E-WEBUI/JOURNEY 覆盖主张；回执显式声明端测边界）。
- 装置验证门=活体 GW probe（open+off×两味）+probe 轮十二面+语料级收口三层；probe 干净不免疫后轮泄露（307 教训）——发现时点后于轮次完成时，按先例旗标上交主管裁定数据有效性。
- 守卫许可下 pass 率解读边界：证明「守卫全许可时模型能/不能裸写正确值」；不证明产品应放开守卫（治理链设计裁定不在本卡范围）。
