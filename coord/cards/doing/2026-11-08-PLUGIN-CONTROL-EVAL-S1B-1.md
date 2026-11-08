# PLUGIN-CONTROL-EVAL-S1B-1：S1-B 臂执行（裸面+联网检索 18 轮——「检索能否补上知识缺口」）

- 发卡：GLM 主管决策流 / 2026-11-08 晚窗（依据=[310 A-pure 终验](../done/2026-11-08-PLUGIN-CONTROL-EVAL-APURE-1.md)——写入类全败于 value_encoding_error=知识/编码型缺口，正是 B 段检索假设靶面+plan.json B_star 段冻结+**用户裁定 2026-11-08：实验不往后排，现在就继续测**）
- 派发确认：已确认（主管发卡·分段 go 第 3 段，307/310 同模式）；验收负责人：GLM 主管决策流
- 池序 312；目标仓库=主仓 scripts（复用冻结 runner）+工件；P1 / 0.5 天 / 依赖=310 工件在库（branch port/plugin-control-eval-apure-1）；L2 / PC；真栈占用（**立即占用——当前空闲**）

## 目标

1. **B 臂 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`（S1-S5+S2b 全生效=A-pure 同款裸面）+ **web.search / web.fetch 启用**（检索预算=plan per_round_caps：≤6 search/≤10 fetch 每轮；retrieval attempts/URLs/hashes 逐轮记录入工件——plan §5 口径）；
2. **检索启用机制勘察**（task 前零栈）：310 runner_frozen driver+plan.json 对 B_star 段的检索支持面——若已有开关/driver 参数直接用；**若需最小缝扩展**（如检索工具面披露开关），授权装置类 seam 扩展（305/308 同模式：默认 unset 字节不变+配对单测），文件域=tools 检索披露面±测试；
3. **装置门七面**（task 轮前 probe+语料级收口）：310 六面复刻（S5 目录无 typed/S2b 无语义键+T3_r1 负锚/content-form control_shell/typed_adapter write_path=0/模型面双证）+ **B 特有第七面：检索面亲证**（模型工具目录含 web.search/fetch+首轮若触发检索则 attempts/URLs 日志在卷；A-pure 面零回归=typed 仍零+语义键仍零）；
4. **判定口径**：plan.json B_star caps 原样（18 rounds/≤216 calls/per_round_caps 含检索上限）；T4 联动对沿用 307/310 标注法不改判据；
5. **回报**：结果落地即回报主管——**B vs A-pure vs A vs P 四方对照表**=「检索能否补上裸面写入的知识缺口」正式答案（用户裁定：实验线=编曲/生成线路的前置结论，优先级最高）。

## 约束

- 文件域=工件目录+卡面+编排脚本（arm_b 版复刻 310 arm_apure 模式）±（如需）检索披露缝+测试；源码零改动红线（除授权缝扩展）；agent 源码只读；
- 真栈协议全链；工件新目录；评价器期望不进模型上下文；检索若被模型用于绕开装置（如搜到本仓库文档）→记录 URL 清单交主管判污染。

## 停止条件

缝泄露任何形态→装置失效停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 预算

≈¥3-6 上界（310 实耗同量级：119 调用/1.68M in）+检索工具自身成本如实记录；§8 跑前成文。

## 领取/回执/验收

- 领取：2026-11-08 晚窗 +0800 / origin/main=8cef79ba367cd1c100d714bf7e1e1cc83dadbcd4（fetch 亲证，≥发卡基线）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 312 s1b runner；发卡人已置卡并派发确认） / 分支=port/plugin-control-eval-s1b-1 / worktree=D:/Vit_DAW_wt_312s1b / 领取提交=coord/312-claim→main（本提交——发卡侧卡面此前未入库，随领取以 doing/ 状态首次入库）。领取基线：fresh worktree 自 origin/main 8cef79ba，`git status --short`=空、`git diff --stat`=空（全净）。在飞卡域核对：doing 四卡（EDU-ASK-ANCHOR-1=282impl 已完在 port/edu-ask-anchor-1 未合入 main、ARRLOOP-AUTHBRIDGE-1、CLOSURE-ROUTE-ADAPT-1、WAVESHELL-OFFRENDER-FIX-1=C++ 域）与本卡（coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/ 工件目录+编排脚本+本卡面±已授权检索披露缝+测试）不相交——282impl 的 catalog.go 改动只在其未合入分支，main 基线干净可用。308 缝在位亲证：`agent/internal/tools/catalog.go` pluginControlEvalSeamTrimsCatalogSpec + `agent/internal/contextruntime/context.go` stripBindingTargetsSemanticOverlay + seam2 测试文件均在 HEAD。agent 二进制=本 worktree 现场构建（SHA256+tool_count 随 run_report）；runner 三件=310 runner_frozen 快照逐字复用（blob 核对随 RUNBOOK）。
- 回执（执行侧 2026-11-08 21:0x +0800 收口，候主管终验）：
  - **diff 文件清单**（全在 `port/plugin-control-eval-s1b-1`；agent 源码改动**仅授权缝一处**：`agent/internal/harness/harness.go` 检索直执缝 + `plugin_control_eval_seam_b_retrieval_test.go` 配对单测 6/6 + 四包回归 harness/tools/contextruntime/agentloop 全绿；runner 三件=310 runner_frozen 逐字复用，入库 blob 核对 plan=259d6b4d/cases=d48ca7cf/driver=57abac09；P 对照 6c4db7c4/1a005f14+307 成绩表 85c5d329+310 A-pure 成绩表均同 blob 复用）：`coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/`＝RUNBOOK.md（§8 冻结+**修正节**）+runner_frozen/ 三件+p_baseline/+a307_reference/+apure_reference/+arm_b_driver.py（冻结 driver 包装=每轮检索 6/10 上限 hook，冻结件字节零改动）+seam_verify_s1b.py（七面门，--selftest **12/12**）+consolidate_s1b.py（四方对照+检索账本+路由面）+arm_b_orchestrator.ps1（PARSE_OK）+两个 run 目录（20261108_202627 首试装置取证/**20261108_203434 终局 exit 0**，每 run 新目录）。commits：预冻结 83da28c0→装置修正 3d715fdf→收口 fd6ffe86。
  - **勘察结论（task 前零栈）**：web.search/web.fetch 披露面=产品目录自带（310 语料亲证 33 请求含目录行）——无需披露缝；执行面唯一门=harness Invoke needsConfirmation（web 对=RiskConfirm）→做卡面授权最小缝 `VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL=1`（默认 unset 字节不变+仅精确"1"+仅 web 对直执+其余 RiskConfirm 语义零改动）。**装置发现（首跑取证+310 语料逐会话复核）**：ordinary loop 工具披露是**按能力路由**的——零命中→全目录 209 行（含 web 对；T2/T3 型）vs 命中→能力包 42/51 行（无 web 对；T1/T4/T5/T6 型）；B 的检索处置只在全目录轮对模型可见=产品有界披露真相（RUNBOOK 修正节+解读边界在卷）。
  - **离线自测**：driver selftest（经包装器）=exit 0（47 项）；seam_verify_s1b --selftest=**12/12 exit 0**（310 八腿+web 对剥离→fail/full 目录缺 web→fail/full 带 web→pass 双向检出力）；consolidate 以 310 数据离线验证（成绩表重演 9/18+四方对照成型+路由分类 12 包+6 可见与 310 证据一致）。
  - **栈窗**：净空亲核 20:24:06 全零→占用登记 15cc7fec→≤60s 复核 20:24:44 owner 在卷+全零→**首跑 20261108_202627**：活体检索 probe PASS（web.search ok 3 结果 bing_html/web.fetch HTTP 200/**零确认停车=312 缝在运行 agent 亲证生效**）+s0/dryround PASS+六面全 PASS→**R1 拦停 exit 4**（R1 前提错误=装置脚本缺陷，非缝缺陷；付费 4 调用/59,535 in 如实入账不评分；编排 finally 亲拆 TEARDOWN_ALL_ZERO）→装置修正（R1 轮感知化+probe 载体改 T2_r0+R1c 语料门+selftest 12/12+310 语料离线 PASS[6 full+13 pack 分类全对]；agent 源码与二进制零改动，首跑/终局 agent SHA256 同）→**终局 20261108_203434 exit 0**→执行侧复核 TEARDOWN_ALL_ZERO **20:52:07** 亲核→释放登记 ff38ed54。
  - **装置验证门（task 轮前，七面全 PASS）**：活体检索腿（缝亲证+网络亲证）；S2 五字段全零（P 对照四字段命中）；S2b 负锚检出力 7/7/7；S3 生产尾串 0；S4 typed 路由首规则 0（P=24）；S5 目录行 plugin_grabber 0+裸读写对在场（probe 2/2 行，语料级 38 行）；CS content-form 0；S1+typed 零执行（语料级 consolidate device_gate 空单）；**R1 web 对目录行在场（probe 载体 T2_r0=全目录形态，8/8 行）+R1c 语料级 every full 形会话带 web 对（0 缺失）**。语料级收口=verdict pass、零泄露会话。
  - **B 18 轮成绩表**（run=arm_b_20261108_203434；arm=B 重标口径：冻结 driver 原始记录 arm="P" 原样在卷；wp=write_path；路由=目录披露形态[full=检索可见/pack=不可见]）：
    | case rep | verdict | failure_tags | mut | first_action | wp | 路由 | calls | in_tok | out_tok | cache_r | stop |
    |---|---|---|---|---|---|---|---|---|---|---|---|
    | T1 r1/r2/r3 | **fail×3** | value_encoding_error×3 | 0 | mix.observe/None/None | none_obs×3 | pack×3 | 7/4/4 | 120,225/59,702/60,640 | 3,471/1,656/1,927 | 64,256/25,472/25,600 | goal_completed×3 |
    | T2 r1/r2/r3 | **pass×3** | r2=model_budget_exhausted | 1 | plugin.set_parameter×3 | **bare×3** | **full×3（检索可见）** | 11/7/6 | 230,410/149,812/93,311 | 5,074/5,459/2,082 | 115,840/67,328/47,488 | r1=runtime_goal_completed/r2=输入超限停(写入已落 pass)/r3=goal_completed |
    | T3 r1/r2/r3 | **fail×3** | model_task_failure×3 | 0 | None×3 | none_obs×3 | **full×3（检索可见）** | 6/5/5 | 65,705/68,153/67,525 | 2,181/3,526/1,781 | 34,304/33,024/34,304 | goal_completed×2/goal_failed |
    | T4 r1/r2/r3 | **fail×3** | value_encoding_error×3 | 0 | None/plugin.get_parameters/None | none/bare/none | pack×3 | 8/8/6 | 107,957/155,829/73,440 | 5,289/8,852/3,740 | 35,328/50,560/23,680 | goal_completed×3（T4 联动对旗标沿用） |
    | T5 r1/r2/r3 | **pass×3**（零 mutation） | — | 0 | None/plugin.get_parameters×2 | none_obs×3 | pack×3 | 4/4/4 | 35,883/35,760/35,071 | 1,646/1,139/1,285 | 12,160/12,288/12,160 | goal_completed×3 |
    | T6 r1/r2/r3 | **pass×3**（限制声明+零写） | — | 0 | plugin.open/None/plugin.semantic_search | none_obs×3 | pack×3 | 6/3/5 | 52,089/32,922/50,893 | 1,934/1,044/4,277 | 16,000/9,728/15,872 | goal_completed×3 |
    - 轮级 **9/18 pass**；案例级 **3/6**（T2/T5/T6）——与 A-pure(310) 完全同形。
  - **四方逐案对照（交付核心）**：
    | case | B(312) | A-pure(310) | A(307) | P | 差异解读 |
    |---|---|---|---|---|---|
    | T1 reverb 写 | **0/3**（pack 路由检索不可见，value_encoding_error 零写入） | 0/3 | 3/3（typed×3） | pass | 检索不可见+即使可见也未必用——B 未变 |
    | T2 reverb 写 | **3/3**（检索可见但**零检索发起**，bare×3 裸写成功） | 3/3（bare×3） | 3/3 | pass | 裸写唯一成功形态复现；检索在场未改变行为 |
    | T3 reverb 写 | **0/3**（检索可见但零发起，model_task_failure 零写入） | 0/3 | 3/3（typed×3） | pass | **B 假设靶面案例（知识缺口+检索可见）上模型一次都没查** |
    | T4 delay 写 | **0/3**（r2 有 bare 写尝试未落值） | 0/3 | 1/3 | fail | 三臂最弱案持续；B 未变 |
    | T5 零写识别 | 3/3 | 3/3 | 3/3 | pass | 四方等价 |
    | T6 零写声明 | 3/3 | 3/3 | 3/3 | pass | 四方等价 |
    - 案例级：**B 3/6 = A-pure 3/6 < A(307) 5/6 ≈ P 5/6**。
  - **检索行为分析（正式答案）**：**18 轮零检索执行、零检索尝试**（attempts/URLs 全空账本在卷；活体 probe 已证检索工具真实可用+缝直通——排除"工具坏了"解释）。分类计数=`retrieval_not_disclosed_pack_routed`×12（T1/T4/T5/T6——产品有界披露下 web 对不在目录）+`no_query_initiated_despite_disclosure`×6（**T2/T3 全目录轮 web 对在场模型仍未发起任何查询**）。检索污染观察表=0 命中；caps 零超限。**结论：「检索能否补上裸面写入的知识缺口」——在产品默认披露与无提示条件下，模型不走检索路径：缺口不被弥补的首要原因是主动性问题（可见也不发起），其次是路由可见性（12/18 轮包路由不可见）；知识到达后的转化问题本轮未获测量机会（零检索样本）**。C 段与 plan §9"薄检索提示词"（R1/R2 时机提示）决策输入：单纯开放检索≠检索会被使用。
  - **预算实耗 vs caps**：任务轮 **103/216 调用、1,495,327/8,640,000 in、56,363/1,728,000 out（全在 caps 内**；cache_read 635,392 逐轮在卷）；终局 probe T2_r0=11 调用/229,095 in/6,912 out（≤12 上界内）；首跑 probe 4 调用/59,535 in（装置取证如实入账）；**本卡全程计费合计=118 调用/1,783,957 in/65,534 out/cache 776,896，估 ≈¥2-4（卡面 ¥3-6 上界内），用户 DeepSeek 账户**；检索工具自身成本=活体 probe 2 次+零任务轮调用（网页抓取无 LLM 计费）；cost=missing（价目未核不折算）。模型面双证：telemetry **114/114 deepseek-flash 零错误**+capture raw.model 114/114（296 晚窗超时模式未命中）。
  - **真栈登记**：占用 15cc7fec（20:24，净空亲核 20:24:06→复核 20:24:44）；释放 ff38ed54（TEARDOWN_ALL_ZERO 20:52:07 亲核）；kernel=wt_228 F9E3DA0A 只读复用；**agent=wt_312s1b 现场构建 1EBCF09F1723DE9F93BD0EBB55B59AF34D9549ACC9F1B36E5048EBFFCD72B948（首跑/终局同一二进制；tool_count=256）**。
  - **端测边界声明（AGENTS §5）**：活体 probe/s0/dryround=确定性腿（零 LLM）；18 任务轮=agent HTTP/ZMQ 真栈腿（双缝开 env 产品 chat 普通工具循环）；装置门+语料收口=payload/log 取证面。渲染面（webui DOM）与用户旅程不涉及（实验腿，无 E2E-WEBUI/JOURNEY 覆盖主张）。单轮=单样本，3 repeat 非稳定性主张（plan §6）。B=「A-pure 面+web 对直执」（检索可见性受产品路由决定，逐轮在卷）；B 数据不解读为 C 结论。
  - 工件分支=`port/plugin-control-eval-s1b-1`（收口 commit fd6ffe86 已推）；工件根=`coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/`。**B 结果落地回报主管**：C 段决策输入（plan B_star→C go/no-go）+「检索补缺口」正式答案=零发起事实+路由可见性分层（上方四方对照与检索行为分析）。
- 验收：（主管回填）
