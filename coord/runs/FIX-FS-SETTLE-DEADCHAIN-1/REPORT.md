# FIX-FS-SETTLE-DEADCHAIN-1 执行报告与上交

- 执行：GLM-5.3 L2 执行会话（PC，ZCode）/ 2026-10-13 深夜–10-14 凌晨
- worktree：D:/Vit_DAW_wt_fsdc1（分支 port/fix-fs-settle-deadchain-1，领取基线 c664ea94=origin/main f937a84d+领取提交）
- 代码交付：B-i 谓词精化（abd65f24）+ C 腿提示硬化（本分支后续提交）；被测二进制 VitAgent A0EA8B1F102CA884（B-i）→ 94CBFA682A7D0236（B-i+C）；内核=主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）
- 真栈：4 run 全 exit 1，**全部死于准入之前，B-i 的 settle seam 从未被真栈触达**；根因=服务端模型档位漂移（见 §三）。卡转 blocked 上交。

## 一、seam 选择：B-i（谓词精化）及其设计依据

**取证**：基线死链轮（TIMEFX run 20261013_232817，resp1 持久化 loop）的**获准入**提案决策只携带 `experiment_materiality={state:none,evaluation:not_ready,attempt:1,evidence_refs:[...]}` 一个 settle 字段——`experiment_target_response=null`、`experiment_round_decision=null`。卡面 B-i 指定的豁免形状与真实死链形状一致。

**选 B-i 弃 B-ii 的依据**：

1. **根因局部性**：死链第一张多米诺是运行时把"字段指针在场"读成结算声明。空态对（state=none+evaluation=not_ready）单独在场不携带任何结算语义——模型在填 schema 占位符。它触发的拒收标记是唯一的吞链源（improvement_proposal_workflow.go:69-99 据此吞掉已准入提案；goalrunner_chat.go:3064 owed 扣留后 armOwedSettlementCheckpointLocked 因在跑切片的 Running 占位返回 nil→lifecycle=none）。
2. **B-i 恢复的是已验证的正常路径**：无拒收标记→提案走普通 needs_experiment 响应路径（park improvement_proposal_confirmation→烟测批准→mix tick→插件装载→typed 执行）。这是每轮通过 D1 使用的链路，无新机制。
3. **B-ii 需要三处新面**：owed 分支的 arm 被在跑切片的 Running 占位挡住（须在 recordGoalResult 内退役在飞记录，触碰"recordGoalResult 先 arm 后释租约"的调度不变量）；D2-only 的 neverActed 引导（free_state_experiment_runtime.go:1003 IsD2MultiRound 门）须扩到 D1；下一片的模型刚幻觉过"已应用"，无确定性引导防其再幻觉。同效果、更大面、更高险。

**实现**（free_state_experiment_runtime.go）：`freeStateVacuousSettleClaimOnUnactedRound(exp, decision)`——仅当 materiality 是唯一 settle 信号、state=none 且 evaluation=not_ready、且当前轮 interventions==0 时，不计入 `settleReportCarried`。

**硬约束核验**：
- 真实 settle 仍需非空 materiality：任何 target_response/round_decision/非 none 状态照旧计为结算企图（有测试钉住）。
- 幻觉 settle 拒收语义保持：空轮上 `EvaluateMateriality` 以 "materiality requires an intervention" 拒收（experiment/runtime.go:733），幻觉结算永不落地；acted 轮完全不受豁免影响（有测试钉住）——EvaluateMateriality 自身不查观察，观察前置门就是 :651 拒收，acted 轮保留字段在场语义因此是证据门的必要条件。
- 只加恢复不加豁免：改动只移除一次误触发拒收，不为任何结算增加接受路径。

**回归测试**（free_state_settle_deadchain_test.go，红绿验证：禁用豁免时反例测试失败于"refusal marker armed"）：
1. `TestVacuousSettleClaimKeepsAdmittedChainAlive`——空态 final=true 反例：无拒收标记、materiality 不落地、提案确认面存活（waiting_confirmation+improvement_proposal_confirmation）。
2. `TestVacuousSettleClaimStillCountsAfterRoundActed`——acted 轮空态仍拒收（证据门不弱化）。
3. `TestRealSettleReportStillRefusedOnUnactedD1Round`——material/target-only/decision-only 三形状照旧拒收。
4. `TestFreeStateVacuousSettleClaimOnUnactedRowMatrix`——谓词形状矩阵。

## 二、C 腿（可选硬化腿，卡面授权范围；本轮实为必要）

run1/2 取证发现：当夜模型在**首个**提案轮就预填**完整三件套占位**（materiality{none,not_ready}+target_response{absent,…}+round_decision），且 target 与新鲜观察目标不匹配。三件套在场使 `carriesExperimentReport`（free_state_reasoning_loop.go:1047 与 agentloop 输出门同判）跳过 G1-G7 审计 bounce——基线里正是该 bounce（1092 字 gap 消息）教会模型补轨道级观察后重新提案获准入。跳过审计+admission invalid+turn final=另一条准入前死链（域外发现①，见 §四）。

C 腿落地（概率性、不触盲法——仅相位纪律文本，无任何 fixture/轨道/参数真相）：
- `agent/rules/ruleset/resources/sections/neutral_family.output_format.txt` 与 legacy 模板（ccb_model_prompt.go）同步：needs_experiment 示例**删去**三 settle 字段（settle 形状由 post_action 相位的 freeStateSettleReportExample 单独承担）；
- discipline 段新增规则行："A needs_experiment turn carries improvement_proposal only: never fill experiment_materiality, experiment_target_response, or experiment_round_decision on it…"（embed/legacy 双源同步，parity 测试保持绿）；
- 测试：`TestNeutralSkeletonProposalShapeOmitsSettleReportFields` 钉住示例不含三字段+禁令行在场；`TestFreeStatePromptCarriesExperimentEvaluationContract` 改钉契约散文面（原钉的引号形态只在被删示例中存在）。

**真栈生效证据**：run3/4 的模型原始输出（agent_message_loop_debug_tail.jsonl）提案已完全不含三件套。

## 三、exit 0 未达成的根因：服务端模型档位漂移（环境实锤）

- 启栈前探针：请求 model=deepseek-chat（config 未动），**回显 model=deepseek-flash**；
- `GET /models` 现仅返回 `{deepseek-flash, deepseek-v4-pro}`；
- 基线（231930/232817，两轮共 2 次完整准入）由更强档位服务；当夜 flash 档行为退化吻合：run1/2 预填三件套+target 错配+过早 final；run3/4（提案已干净）不做 G6 要求的轨道级观察、以工程级证据反复重提案直到预算烧尽终末锁（"terminal turn produced no admissible final decision"，同签名 2/2，§8 停）。
- 四 run 断点全部在准入之前；B-i 修复面（准入后的 settle 拒收谓词）未被触达——代码正确性由单元回归+红绿验证背书，真栈销账条件（exit 0）在当前服务档位下不可达。

## 四、域外发现上交（不扩域）

1. **carriesExperimentReport 审计跳过**（free_state_reasoning_loop.go:1047 + agentloop/free_state_reasoning.go 输出门同判）：实验尚未准入时，占位三件套使首个提案跳过 G1-G7 审计 bounce，剥夺模型的 gap 反馈回路。C 腿已在提示面缓解（真栈验证生效），但运行时跳过对"仍填三件套的模型"是潜在吞链面——建议新卡评估把跳过条件收紧为"实验已存在（loop.Experiment != nil）"。
2. **模型档位裁定**（决策侧）：deepseek-chat→flash 别名使卡面"LLM=deepseek-chat 勿改 flash"无法按原意满足。菜单：(a) 授权改配置 deepseek-v4-pro 复跑收口腿（恢复基线档位，费用上升）；(b) 等服务端恢复；(c) 接受 NOT_EXERCISED 类收口（B-i/C 代码按单元证据采信）。

## 五、验证清单

- `go test ./... -count=1`：95 包 ok / 0 FAIL（B-i 与 C 腿两阶段各自全量跑过）；
- gofmt：四个改动 Go 文件 LF 归一后 gofmt 干净（工作副本 CRLF 属本机 checkout 形态，blob 为 LF）；
- 真栈 4 run：console+report+agent log 全部在卷（coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/）；占用/释放已按 §2.2 登记（0854e899/c1be027a）；
- LLM 口径行：config defaultModel=deepseek-chat @ api.deepseek.com/v1（未改）；服务端实际服务=deepseek-flash（探针+models 端点实锤，§三）。

## 六、端测边界声明

改动面=agent/internal/chat（settle 谓词+测试）+ agentloop/ruleset 提示面（C 腿）；未涉 webui/内核/scripts。真栈端测覆盖：域选择→提案→（准入失败边界）全链 4 轮实跑取证；**未覆盖**：准入后路径（intervention 物化/settle/盲法试听）因模型档位墙未触达——该边界即本报告 §三主体。

## 七、腿 D（final gate 拒收反馈教学硬化）+ 复跑 rerun3（2026-10-14 晨，验收裁定二授权）

### 腿 D 代码面（commit 4737a0c6 @ port/fix-fs-settle-deadchain-1-rerun2，已推 origin）

- 拒收消息构造链：`experiment.MaterialityEvaluation.Validate()`（agent/internal/experiment/runtime.go）产生字段错误 → `FreeStateDecision.Validate()` 包 `experiment_materiality:` 前缀（free_state_reasoning.go:157）→ message_loop 输出门包 `invalid free_state decision:`（:946）→ 以 `<final_gate>` user 消息回流模型（message_loop.go:10910/10887）——原消息零形状教学，rerun2 两轮模型续轮瞎猜实证。
- 改动=三字段错误携带被违反字段的合法形状（常量驱动防漂移，零 fixture/轨道/参数真值）：
  - `unsupported materiality state %q (state must be one of: none, subthreshold, material)`；
  - `unsupported materiality evaluation %q (evaluation must be one of: not_ready, insufficient_dose, agent_evaluable, ambiguous)`；
  - `materiality attempt must be a positive integer (attempt >= 1), got %d`。
- 测试：agent/internal/agentloop/free_state_settle_shape_teaching_test.go 三测钉「非法输入→拒收消息含合法形状枚举」（含 run6 实形状复刻 fixture：final=true+settle 三件套+仅 materiality 形状非法）+scope control（attempt 教学不漏 state 枚举）。
- settle 示例核对：freeStateSettleReportExample 的 evaluation 枚举（agent_evaluable|insufficient_dose|ambiguous）为 schema 合法集的**有意子集**——not_ready 须保持 schema 合法（B-i 豁免对 state=none+evaluation=not_ready 依赖它），settle 时刻语义上不该用 not_ready（无证据应走 blocked boundary）；示例与教学无矛盾，不改。
- `go test ./... -count=1` 95 包 0 FAIL；gofmt 净（LF blob 口径，工作副本 CRLF 属本机 autocrlf checkout 形态）。

### 复跑（worktree 同 rerun2，腿 D 提交叠其上；agent 构建 SHA256 AFEA9F30C32A56EF；模型=deepseek-flash 思考档 config 全程未动，启栈前探针 HTTP 200/0.68s、echo=deepseek-flash、reasoning_tokens=16；占用登记 51ed34dc）

三 run 均 exit 1（run7 20261014_105719 / run8 20261014_111306 / run9 20261014_112339），干预 applied 三轮一致（reverb 语义动态执行、actual_readback 0.2427 族、原子控制、restore_ref 在卷），死点全部在执行后 post-action→settle 段，但**三形态互异**：

- **run7**：terminal="acoustic materiality record is missing"。post-action 段 4/14 次 LLM 60s 整超时（transient_llm_error 自恢复）；唯一 settle 尝试 materiality 对象整个缺失 state/attempt 字段（JSON 省略→零值，run6 第一式同源）→ 带教学拒收 1 次（**教学串真栈在场实锤**）→ 模型收到教学后的唯一重试调用又被 60s 超时吃掉 → 预算耗尽 blocked。**教学有效性未被检验**（transient 主导，非同签名复发）。
- **run8**：terminal="D1 requires one post-action CCB observation"（新形态）。LLM 面 0 超时全健康；post-action 重观察请求同一 per-track view 被防重复门连弹 5 次（bounce 消息本身有教学："decide from the recorded evidence or request a different cataloged view or target"），模型不换 view/target 不从已录证据决断 → capability_blocked（"closure made no material progress within its bounded observation window"），cont 4/6。
- **run9**：terminal 与 run8 同签名。1/14 超时；模型完成 post-action 观察请求但**观察包部分返回**（工程绑定缺失，模型自报 limitations 在卷）→ D1 post-action 观察门不满足；随后 4 次 settle 尝试**打地鼠形态**：state=""↔attempt=0 交替（教学在场逐次拒收并指名被违反字段；模型每次只修首个被指字段、重写对象时丢掉其他字段——尝试 2/4 连 evidence_refs 也缺）。预算耗尽 8/7。

### §8 停止与上交

run8+run9 同代码版本（AFEA9F30）同 terminal 断点连续 2/2 → 停止线触发；三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）**未销**（exit 0 未达成）。上交决策侧的裁定菜单：

1. **升级梯子下一步=deepseek-v4-pro 复跑**（卡面既定梯：flash 思考档 2/2→升 v4-pro；本腿 run8/9 构成同签名 2/2，触发条件成立但由决策侧发）。
2. **打地鼠新证据**（腿 D 的直接产出）：单字段首错教学不足以让 flash 思考档一次拼齐 materiality 完整对象。可选硬化菜单：拒收消息附带完整 materiality 对象形状（四字段全列 JSON 骨架，仍零真值）——即把 freeStateSettleReportExample 的 experiment_materiality 子对象并入拒收反馈；或 schema 错误一次性列出全部被违反字段（Validate 由短路改全收集）。
3. **post-action 观察门两子形态**（run8/run9）：防重复门 vs 模型不换 view；部分返回观察包（工程绑定缺失）不计入 post-action 门——是否属产品缺口（fresh post-action 证据获取被结构性阻断）建议决策侧评估是否立取证卡。

### 验证与工件

- 真栈 3 run：console（run7/8/9）+ d1_smoke_report.json×3 + debug jsonl 切片（run7×1/run8×5/run9×4 条 final_gate 记录含原始 raw）+ telemetry 切片（14+13+14 次调用）+ run9 agent_last_log 全部在卷 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/；重件留盘可回指 worktree D:/Vit_DAW_wt_fsdc1r2/artifacts/free_state_d1_s1/。
- 拆栈：12:0x 手动 teardown VitAgent 22320/VitApp 30532/Godot 10836，复核四端口 5555/5556/7878/8787 监听=0、进程名核验=0。
- 端测边界：真栈覆盖=域选择→提案→准入→装载→执行 applied→post-action 重观测边界→settle 拒收（含教学在场）边界×6；未覆盖=合法 settle 落地与人工 A/B 试听下游（三 run 均未产生合法 materiality 记录）。

## 八、腿 E（取证先行+打地鼠硬化，rerun4，验收裁定三授权；2026-10-14 下午）

详细取证卷=[FORENSICS-LEG-E.md](FORENSICS-LEG-E.md)（全部锚点+工件回指，只读取证零栈启动）。

### ① 取证结论：实锤产品缺口 → 按裁定三停止上交，③ 复跑腿不执行

- **run9 子形态（硬实锤）**：post-action 重观测请求本身合法且执行成功（模型带 priority_reason=contradiction+unresolved_questions 通过防重复门，journal rec 26 原文在卷），但返回的观察包 project_package 身份三元组全空（revision=''/epoch=''/hash=''，uuid 在；pre-action 同路径产物 revision='9' 正常）→ binding unbound（capabilitycontext/free_state_observation.go:522-531 canonicalBound 分支）→ chat/free_state_reasoning_loop.go:1275 `freeStatePostActionObservationEligible` 要求 ProjectRevision 非空 → 观察永不记 post_action=true → D1 断言（free_state_d1_smoke.py:1693）结构性不可满足。上游=harness.go:1844 `vspLegacyStateWithBinding` 在 VSP 快照 reply 缺 revision/epoch/hash（run9 session latest_change 记录身份字段 before=11→缺失、graph_revision after=1）时用空身份**整体覆盖** shadow；**对照组 run6 同路径 post-action 观察 revision='7' 绑定正常**（run6 因此死在 materiality 而非此门）→ 时序相关间歇缺陷，任何模型档位触发即死同门（v4-pro 空烧判断成立）。修复面=harness 身份保持（agent）或 kernel/VSP 快照恒带身份（C++），**均域外，上交不扩域**。
- **run8 子形态（缺陷候选+模型行为混合）**：5 连弹按门自身 per-view 可用性语义正当（被弹视图确有 usable 条目，录自 revision 6 改动前；partial 算 usable）；但①弹回教学"decide from the recorded evidence"在 post_action 段被 settle(:1085)/blocked(:1214)/needs_action(:1026)/satisfied(:1188) 四门结构性封死（requires_post_action_observation=true 全部要求本周期新观察）=**教的替代路径不可达**；②"换 view/target"出口可达而模型 5 次未走（第 5 次口头称换实际仍请求同一 view，debug rec 4 原文）；③revision 豁免 fingerprint 整集锚定（free_state_gate.go:1061-1076）对模型合法收窄的请求结构性够不着；④提示规则 ccb_model_prompt.go:670 明令的"experiment's view set on the applied target 重取"恰是被弹形状。修复面=agentloop 提示/门/豁免，域外上交。

### ② 打地鼠硬化（commit 1923b22a @ port/fix-fs-settle-deadchain-1-rerun4，已推 origin；基线 origin/main 4476b9ed 含腿 D 3ec1cf96）

- `MaterialityEvaluation.Validate`（experiment/runtime.go）短路改**全字段收集**：state/evaluation/attempt/evidence_refs 逐项+crossover 规则一次全列；拒收消息附 `materialityShapeExample` 完整骨架（四字段全列零真值；evaluation 用 schema 全集四枚举——not_ready 须保持 schema 合法供 B-i 豁免对，与 agentloop settle 示例的有意子集并存，口径注释在代码内）。
- 单测（agentloop/free_state_settle_shape_teaching_test.go）：两新钉=组合违反（run9 实形状 state=""+attempt=0+evidence_refs 缺）一次列出三字段错误+骨架四字段在场；腿 D 三钉原样保持（含 scope control：attempt 单独违反时消息不出现 "state must be one of:" 字面句——骨架只含枚举联合不含该句，测试通过验证兼容）。
- 自验：`go test ./... -count=1` **95 包 0 FAIL**；gofmt 净（LF blob 口径；工作副本 CRLF 属本机 autocrlf checkout 形态）。
- 模型口径行：**本腿零真栈运行（裁定三"实锤即停栈"触发），config=~/.vit/config.json 全程未动**（deepseek-flash @ api.deepseek.com/v1 维持裁定三状态）；无新 60s 超时样本。

### 端测边界声明（腿 E）

真栈覆盖=零（本腿按裁定不启栈，取证全部只读自 rerun3 工件）；代码面覆盖=experiment Validate 全字段收集+骨架（单元级，95 包 0 FAIL）。未覆盖=真栈 settle 段（腿 D 后无新 run）、合法 settle 落地与人工 A/B 下游。**三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）未销**（exit 0 未达成且按裁定三停止烧栈）。

### 上交决策侧

1. run9 绑定缺口修复面二选一（或并行）：agent 侧 harness 保留最后已知身份 vs kernel/VSP 侧快照恒带身份（C++ 面）——建议先 agent 侧硬化（可单测钉）再真栈复跑取证间歇性。
2. run8 教学不可达分支+豁免粒度错位+提示矛盾三锚点是否立卡（agentloop 面）。
3. 腿 E②代码是否采信合入（1923b22a，95 包 0 FAIL+腿 D 钉保持）；若采信，下一真栈腿=修复卡落地后复跑（本腿未烧栈）。

## 九、rerun5 收口腿（验收裁定五授权；2026-10-14 晚，run10/run11）

**模型口径行——基线=deepseek-flash 思考档**：~/.vit/config.json defaultModel=deepseek-flash @ api.deepseek.com/v1 全程未动；启栈前 LLM 预探测 HTTP 200/0.84s、echo=deepseek-flash、reasoning_tokens=28=思考档在线（探针存卷 llm_probe_rerun5.json）。worktree D:/Vit_DAW_wt_fsdc1r2 @ **port/fix-fs-settle-deadchain-1-rerun5=origin/main 51752087**（含 112=26757b34+113=1f4e2919+腿E=08c64ffb；本腿零代码改动，分支即基线标记）；agent 构建 VitAgent.exe SHA256 **AEAA48053634968B**；内核=主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）；占用登记 46f4ce19。另记环境事实（与 rerun3 等价非新差异）：VSP Hub 8787 两 run 全程未起（run9 对照同为注册 pending 零成功），realtime publish 降级非致命，vsp_state_snapshot 走内核 ZMQ 正常。

### 结果：同 terminal 签名 2/2，§8 停止上交

run10（20261014_195628）与 run11（20261014_201520）均 exit 1，terminal 同签名：

```
FS-SETTLE-TERMINAL-1: a machine boundary settled the closure out from under
the parked human judgment: fs9_terminal settlement={"reason": "project_revision_stale",
"summary": "observation belongs to a different project revision", ...}
```

（断言源=free_state_d1_smoke.py:1106，FS-SETTLE-TERMINAL-1 停泊不变量：轮停泊 user_judgment_pending 时闭包不得带机器结算。）

**链路历史最深（两轮一致）**：域选择 reverb → decay 提案 →（run10 首案 G6/G8 弹回后二次提案准入，属正常门工作）→ 插件选择 waiting_clarification（烟测确认）→ 干预 applied（run10：track 1017 guitar、rev3→4、actual_readback 0.6315、原子控制 1 次、restore_ref 在卷；run11：rev6→7、readback 0.2925、原子 true）→ **post-action CCB 观察请求直接放行入账**（run10 obs_120033@rev4 / run11 obs_121949@rev7，D1 post-action 门两轮全过=run9 签名死透）→ settle 首试 schema 非法被拒（教学在场）→ **重试一次即产出完整合法 materiality 并正确停泊 user_judgment_pending（experiment=waiting_for_user）** → 停泊后 **1 秒内**被 audioclosure 跨修订守卫以 project_revision_stale 机器结算压过 → 烟测 FAIL。

### 卡面三锚点逐项记录（per run）

1. **112（VSP 身份保持）**：两 run agent 日志 `identity_preserved_on_incomplete_snapshot` 均 0 次、refresh status 无 identity_preserved 键——**run9 形态（观察包工程绑定缺失）未复发**：两轮全部观察回执绑定三元组完整（run10 rev2/2/4、run11 rev5/5/7，epoch/hash 全在），112 修复路径未被触发（无缺口快照出现），标记不出现属预期而非失覆盖；若后续复跑中 run9 形态复发且标记仍缺，才构成"修复未覆盖该路径"。
2. **113（post-action 观察门反馈面）**：run10 无弹回（模型整集重取同一组目标级视图，经精确集通道直接放行，obs ready 入账）；run11 有一条弹回实录（20:17:59 debug 流）："closure candidate frontier requires only target-level track.\* observations; project.\* and mix.\* cannot be retried now"——**阶段感知新文案在场，不含 "decide from the recorded evidence"**（锚点二通过）。模型随后换 target 级视图成功（弹回教学可达性修复真栈生效）。收窄子集豁免通道（freeStateNarrowedReObservationAdmitted）两轮均未被模型选用（整集重取已够），该分支未 exercised。
3. **腿E（打地鼠硬化）**：两 run settle 拒收消息均含**全部违反字段一次列全+完整 JSON 骨架**（run10：state+attempt+evidence_refs 三违反+骨架四字段；run11：state+attempt 两违反+骨架）。**教学有效性首次真栈验证通过**：两轮模型在收到一次教学后的下一个 settle 尝试即拼出完整合法对象（run10 round-1 materiality={subthreshold, insufficient_dose, summary, evidence_refs=[obs_120033], attempt=1}；run11 同形）——rerun3 run9 的四连打地鼠形态死透。

### 新缺陷根因链（确定性产品缺陷，域外=audioclosure/chat 闭包回放面，不扩域）

触发序列（run10 亲核，run11 同构复验）：

1. 干预落地使工程 revision 前进（run10 2→4、run11 5→7），闭包经 `EventProjectRevisionChanged` 重绑（事件 seq24/seq29）——该 fold（audioclosure/events.go:199-212）**按设计清空 Observations map**（旧修订证据作废）。
2. 模型合法 settle 停泊 → `requires_post_action_observation` 翻 **false**（被合法 settle 满足；持久化 loop 实录）。
3. 停泊轮的闭包回放（chat/audio_closure_controller.go:596-634）：ledger 再水合把全部观察（含 pre-action rev2/rev5 两枚）追加回放；post-action 过滤器（:599-606）因标志已 false **不再拦旧修订观察**；去重检查（audioClosureHasObservationID）因闭包 Observations 已被清空**不认识** pre-action 两枚 → 放行。
4. `driver.RecordObservation`（audioclosure/driver.go:183）跨修订守卫：key rev（run10 "2"/run11 "5"）≠ 闭包 state rev（"4"/"7"），且 `SupersededProjectRevisions` 为空（revision 前进经 project_revision_changed 而非 governed booking 登记，旧 revision 未入 superseded 集）→ `settleUnchecked(StopProjectRevisionStale, "observation belongs to a different project revision")` → 机器结算在停泊 1 秒内落账（run10 12:01:48 试听启用→12:01:49.135 结算；run11 20:22:06→20:22:07.167）。
5. 烟测 2s 轮询必输 1s 结算竞速 → **exit 0 在此形态下结构性不可达**（与裁定三 v4-pro 空烧判断同构：任何模型档位产出合法 settle 即死同门）。

修复面候选（决策侧裁定，本卡不扩域）：回放侧在停泊态（round decision=user_judgment_pending / experiment waiting_for_user）不回放旧修订观察；或 project_revision_changed fold 将被替换 revision 登记 superseded（与 governed booking 对齐）；或停泊后闭包回放整段短路。

### 60s 超时与 LLM 面（逐 run）

- run10：11 次调用全 deepseek-flash，**0 错误 0 超时**（max http_ms 31702）——历轮最健康。
- run11：13 次调用，**1 次 60s 整超时**（transient 自恢复续轮，非失败主因；max 60000）。

### 验证与工件

- 真栈 2 run：console（run10/11）+ d1_smoke_report.json×2 + agent_last_log×2 + debug 切片（run10×1/run11×2 条含弹回与拒收原始载荷）+ telemetry 切片（11+13）+ LLM 探针在卷 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/；重件留盘 worktree artifacts/free_state_d1_s1/20261014_195628、20261014_201520（含 project/ 运行态与 agent_runtime_state.json 闭包事件流）。
- 另记调用形态失败腿一条（同 rerun2 run5 先例按环境中断类记录）：run10 首次启动命令经 bash 双引号传 `.\scripts\...` 反斜杠被吞→PowerShell -File 找不到脚本即退（未启栈零进程影响），改正斜杠后同文件覆盖重跑。
- 拆栈：20:25 手动 teardown VitAgent 18772/VitApp 23040/Godot_v4.6.1 16708，复核四端口 5555/5556/7878/8787 监听=0、进程名核验=0；占用登记 46f4ce19/释放随本回执提交。

### 端测边界声明（rerun5）

真栈覆盖=域选择→提案→准入→装载→干预 applied→post-action 观察入账→settle 拒收（教学在场）→**合法 settle 停泊人工裁决边界**→机器结算压过停泊（×2）；**未覆盖**=guarded audition judgment POST 及其下游（停泊在人工裁决 POST 前被机器结算终结，合法 settle 落账与人工 A/B 试听下游仍未走通）。**三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）未销**（exit 0 未达成）。

### 上交决策侧

1. **新 terminal 签名 2/2 成立**（FS-SETTLE-TERMINAL-1/project_revision_stale）：卡面梯子"flash 思考档 2/2→升 v4-pro"触发条件形式上满足，但本签名为**确定性产品缺陷**（§九根因链：停泊后旧修订观察回放撞跨修订守卫，与模型档位无关，任何档位产出合法 settle 即死同门）——v4-pro 复跑预判空烧，是否仍按梯子升档或先修缺陷由决策侧裁定。
2. 三修（112+113+腿E）真栈效果全部正向：run9 绑定缺失未复发、run8 弹回教学不可达已修（新文案+模型成功换视图）、打地鼠死透（两轮一次教学后即合法 settle）——链路被推至历史最深（人工裁决停泊）。建议就 §九根因链立修复卡（audioclosure 回放面，本卡域外）。
3. delay 加分腿未跑（exit 0 前提未达）。
