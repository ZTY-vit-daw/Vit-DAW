# D1-RUNNER-FORENSICS-1 取证报告（零 LLM 读档，2026-11-05 晚窗）

- 执行：ZCode GLM-5.3 L1 PC 执行会话（256 卡会话）；worktree D:/Vit_DAW_wt_256impl @ port/d1-runner-forensics-1（领取 474e2dde，基线 c3833c67）
- 对照组：133 run2 = FIX-LW-CONFIG-PERSIST-1 run2 `20261018_124639`（大件 D:/Vit_DAW_wt_loudness_watch_b/artifacts/free_state_d1_s1/20261018_124639/，精简件 coord/runs/FIX-LW-CONFIG-PERSIST-1/evidence/）vs armrun2 = AF-D1-LOUDNESS-SETTLE-1 `20261105_215624`（coord/runs/AF-D1-LOUDNESS-SETTLE-1/armrun2/ + 大件 D:/Vit_DAW_wt_254impl/artifacts/free_state_d1_s1/20261105_215624/）
- 变量隔离：**同 fixture（spv1_p01，public_source_project 逐字节同路径）+同话术（reverb）+同引擎（deepseek-flash，armrun2 无注入=本机默认 config 亲读）**；唯一实质变量=代码版本（10-18 vs 11-05，M3 措辞链 243-248 其间合入）。

## 一、假说判定：措辞漂移假说 **证实**（三点证据链闭合）

### ① 载荷 needle 面：M3 措辞句在场差异实证

| 维度 | 133 run2（10-18） | armrun2（11-05） | Δ |
|---|---|---|---|
| system prompt | **14,331 chars**（agent log message_loop.llm 亲证 ×8 轮恒定） | **17,670 chars**（×10 轮恒定） | **+3,339（+23%）** |

增长源逐笔钉死（全部经 `git merge-base --is-ancestor` 亲证在 armrun2 被二进制 F321AF5E 的构建树内，且全部晚于 133 run2）：

1. `4a439588`（227 M3-LLM-GATE-OPEN-1，10-31）：free-state 决策轮 automation 工具面准入——227 卡回执亲证 **system 15,468→17,045B**；
2. `e9a51a06`（235 M3-NEUTRAL-ACTION-PATH-1）：时变行动路径教学段 v1 无条件进 turn_directives；
3. `b6cbc007`（243 M3-WRITE-GATE-TRIGGER-CLARIFY-1，11-04）：v2 触发权句（写入触发权因果修正，ccb_model_prompt.go 三处）；
4. `40cdd189`（246 M3-TERMINAL-ROUTE-WORDING-1，11-04）：v3 终局路由（终局直调句三选/时变形态示例/确认语义桥接/豁免子句，freeStateTimeVaryingActionPathVersion v2→v3）。

D1 场景 system prompt 非 immune：自由态循环共用同一措辞面（227 卡即以 D1 系冒测腿验证部署面）。

### ② 动作序列：mutation 轮次/工具 对照

| | 133 run2 | armrun2 |
|---|---|---|
| 响应数 | 8 | 2 |
| 路径 | plugin_selection_required（轨上 Pro-R）→装载确认→**semantic_dynamic_confirmation_required**→driver 确认→**typed controller 执行**（tx_dyn_9a04690e2f38030b，rev 3→4，readback actual_readback_value=0.3827673793）→post-action 观察（obs_20261018T045006）→audition ready→park fs7 | **improvement_proposal**（总线化混响提案：建混响总线+发送，needs_confirmation）→driver 确认→**semantic_treatment_strategy** 工作流→渐进披露观察审计 partial→**semantic_treatment_capability_boundary** 终态 |
| forward_mutation | **1**（validation 亲证） | **0**（interventions=0×1 round、d1_receipt=null、mutation_performed=False×2、mutation_authority=False 全 bundle） |
| 终态 | pass/park waiting_continue（judgment_park, fs7） | loop.status=**blocked**+latest_decision.status=blocked stop_reason=semantic_treatment_capability_boundary |

关键差异机制：M3 链开放 automation 工具面+时变路径教学+终局直调措辞后，模型行为分支集扩大——「提案→（确认后）语义策略路线→审计边界拒绝→blocked 诚实上收」成为新行为面下的可达分支。

### ③ 轮次时长分布：排干窗口敏感性

| | 133 run2 | armrun1 | armrun2 |
|---|---|---|---|
| LLM 调用数 | 8 | 41 | 10 |
| 单调用时长 | **1.9-19.5s**（12:48:36-12:51:11） | 9-20s | 2.1-19.3s（13:58:09-14:00:02） |
| 链总时长 | ≈2.6 分钟 | 600s 未排干 | ≈2 分钟收敛 |

**结论**：单调用延迟分布两时代同族（133 也有 19.5s 峰——「9-20s 峰」非 11-05 新引入回归）；armrun1 超时机制=**轮数（41）×晚高峰延迟**乘积效应超 600s 窗口，而 41 轮形态是模型随机分支（armrun2 同引擎同 fixture 10 轮收敛亲证非代码面必然）。600s 窗口对正常链（8-10 轮）有 3-4 倍余量。

## 二、零 mutation 分支合法性判定：**合法模型分支，非 agent 面回归**

1. **准入正常**：boundary=admitted（两响应 free_state_reasoning_loop/admission_receipt 亲证）；admission 治理面完整（typed_action={reverb, reverb_decay_adjust, decay_pct:-5}、experiment_budget=1、rollback_plan/verification_plan/dose_bounds 全在场）；
2. **诚实零写**：零 interventions、d1_receipt=null、journal 零行、全响应 mutation_performed=False——与 2026-10-13 hallucinated-settle 死链（FIX-FS-SETTLE-DEADCHAIN-1：模型幻觉宣称完成+零插件，bug 已修）**本质不同**：无任何完成宣称、无 fabricated receipt、提案确认面完整在场（response 0 needs_confirmation=true）；
3. **治理边界拒绝合法**：stop_reason=semantic_treatment_capability_boundary+reply 明示「progressive disclosure observation audit is not successful: partial」——渐进披露观察审计 partial 的 fail-closed 拒绝是 CCB 披露层设计行为（缺失/partial/stale 原样保留不升级，AGENTS §5 投影纪律），非缺陷；
4. **链路健康**：settle 链 2 分钟完整收敛（goal completed/stop=done，executed=5/ccb_observations=4）——254 终验裁定亲证。

## 三、修复（按取证+90 卡先例分支容忍）

- `scripts/free_state_d1_smoke.py`：
  - `validate_d1` 主断言链分支化——`interventions` 为空时进 `validate_zero_mutation_boundary`（新），非空时 **1-mutation 主链全部既有断言原样保留**（逐字未动）；
  - **0 次分支紧断言集**（每条源自取证钉死的合法形态）：①无卡无写——`d1_receipt` 必须为空（拦幻觉 receipt=死链契约）②journal `free_state_d1_s1` 行数==0（写达为零服务端亲证，非仅模型自报）③边界申报在场——latest_decision.status ∈ {blocked, capability_blocked} 且 stop_reason 非空（拦静默停泊）④全响应 `mutation_performed` 无 True（拦卡外写）；治理预算纪律（budget==1/max_action_attempts==1/typed domain·kind·dose bounds）在 validate_d1 公共前缀已断言、两分支共享不放宽；
  - 1 次[确认链写达读回]分支=既有主链（receipt 前后 revision/transaction/idempotency/readback、d1_receipt 全 flag、audition 双候选、journal==1、select 不变性）零改动。
- **排干窗口：不改动**——取证结论=非延迟敏感（单调用分布两时代同族；41 轮形态为随机分支，armrun2 亲证非必然；600s 对正常链余量 3-4 倍）。不参数化不放大（禁无证据放宽同理禁无证据改动）。
- agent 生产代码零改动（文件域仅 scripts runner 面；py 文件为 ps1 包装的 python runner 本体，属卡面「scripts/run_free_state_d1_smoke.ps1 断言/窗口段」的 runner 面）。

## 四、验证（离线腿）

- `python -m py_compile` PASS；
- 文档式测试全 PASS（新增 `test_validate_zero_mutation_boundary_forms`：armrun2 形态正例+五反例——幻觉 receipt/waiting_continue 停泊/无边界申报静默停/写宣称/journal 行；既有 honest_refusal/find_d1_loop/continuation_interaction/processor_selection/sealed_target×3 全绿）；
- **真实数据双向回放**：armrun2 真实 loop projection 过新分支→`{"status":"pass","branch":"zero_mutation_boundary",...,"boundary_stop_reason":"semantic_treatment_capability_boundary"}`；133 run2 真实 loop（tx_dyn_9a04690e receipt）→interventions=1 走主链，legacy 断言全保留。
