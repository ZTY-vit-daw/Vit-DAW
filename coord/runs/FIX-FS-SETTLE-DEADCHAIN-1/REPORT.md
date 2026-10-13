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
