# run3 取证（R3 收尾腿：runner 主链 py 断言失败——post-action CCB 观察缺席；第三型确定性断点，与 run1/run2 均不同型）

- run：`20261108_184359`（18:43:59 起栈 18:44:03 agent 起活→19:00:37 runner 收尾→ps1:664 throw；单 run 授权用尽，未重跑、未改断言、未手动补腿）。
- 被测基线：port/d1d3-regress-stack-r2 **cb029d66**（=f281d887+本腿 D2 tracks 面断言分支感知修正，仅 ps1 断言块 +76/-25）。274 f413fbca / 292 5c8712a6 / 295 f6d1982e 三修祖先亲证在基线。agent 二进制 SHA256 82A70AA8F9147CBF9F49190BD15BF50F98FE31DC5CD7BBD33FDEE4339DAEA3A7（**与 run2 逐字节同哈希**——零 agent 源码改动下的可复现构建）；kernel F9E3DA0A…（run1/run2 同款只读复用，18:41 亲核）。

## 结果总览（PS1 exit 1）

| 段 | 结果 |
|---|---|
| runner 主链 D1-S1 | **FAIL**（py:1767 `D1 requires one post-action CCB observation`→ps1:664 throw；**apply 本身成功**：mutation=1、readback_verified=true、revision 4→7、track_count=7） |
| D1-settle / D2 / D3 门组 | **均未到达**（脚本序在 runner 后，:664 先 throw）——D2 分支感知断言活栈未兑现、D3 首次活栈执行未达 |
| LLM | 41 调用 deepseek-flash（agent 日志 message_loop.llm 面亲数），**err=2 且均 ms=60000**（60s 整超时，turn33/34 连续两轮）；总 chars 2,549,099 |

## 失败点与机理（证据链）

1. **失败断言**：`free_state_d1_smoke.py:1767 require(len(post_observations)==1)`——experiment.rounds[0].observations 仅 1 行且 `post_action=False`（pre-action 种子观察 obs_…104623_d7b023080665）；1764 `readback_verified=true` 已过（比 run1 的 1770 materiality 断言**早一位、且 apply 已完成**）。
2. **响应链**（d1_smoke_report.json responses×6）：resp0 improvement_proposal_confirmation_required→resp1 plugin_selection_required→resp2 装载（pca_admission_receipt，18:48:11 revision=4 checkpoint，18:50:19 FX 轨创建 shadow tracks 增）→resp3 semantic_dynamic_confirmation_required（"尚未写入任何参数"）→**resp4 limit_reached（limit_type=timeout）** checkpoint+continuation cont_9a07ec…（18:53:25，revision=7=apply 后，d1_receipt parameter_applied=true）→resp5 续跑状态。
3. **尾段双 60s LLM 超时**（agent_last.log 亲数）：18:57:35 turn33 `message_loop.llm ms=60000 err=true`→stop=transient_llm_error（executed=6）；18:59:35 turn34 `ms=60001 err=true`→stop=transient_llm_error（**executed=0**）。这两轮正是欠 post-action 观察的续跑轮。
4. **诚实治理面在场**：19:00:37 `[free-state-experiment] settle report refused until the fresh post-action observation is recorded on the round`（settle 被拒=治理正确）+`[continuation.owed] completed turn withheld`（欠账回合扣发、goal 还原 waiting_continue）+终态 `[chain.terminal-summary] decision=needs_experiment`、goal 挂 `experiment_budget_awaits_settlement`；persisted_loop `requires_post_action_observation=true`、`settle_refused_round_id=round-1-a696…` 在档。
5. **decision 内容**（latest_decision.summary）：「处理后的混响总线返回的是空轨身份、电平投影缺失的工程数据，无法据此测出缩短衰减带来的可闻变化量」——post-apply 观察数据面对新建 FX 总线轨的品质边界（与 265/296 卡族已知的 FX 新轨无源数据面同族），但**致死点是 LLM 超时轮堵死观察落账**，非观察数据本身拒绝。
6. **旁证记录（非致死）**：尾段两次 `track.add_audio status=error`（18:57:41/18:59:46，kernel 拒绝加轨）随 LLM 错误轮出现，记录在卷不定性。
7. **环境一致性**：VSP 8787 pending 告警 run2 同款在场（56 行 vs 50 行，vsphub 两轮均未启）——排除环境差异变量。

## 与 run1/run2 对照（三型分列）

| | run1 | run2 | run3（本 run） |
|---|---|---|---|
| 失败点 | py:1770 materiality（settle 尾全缺） | D2 门组 tracks 面两断言（ps1:1695 throw） | **py:1767 post-action 观察缺席（ps1:664 throw）** |
| 链形态 | apply 后 capability_blocked（G7/G8，post-load） | 0-mutation 合法边界（orchestrator 入口 audit partial） | **apply 成功+续跑轮双 60s LLM 超时→观察欠账→settle 诚实拒绝** |
| runner exit | 1 | 0 | 1 |
| LLM 面 | 30 调用 1 超时行 | 9 调用 0 error | **41 调用 2 超时（连续）** |
| 274/292 消费面 | 未到达 | 真栈全过 | 未到达 |

- run3 断点（1767）为**首次出现**，非 run1（1770）/run2（D2 门）同型再现——「同型确定性断点再败」止损条款按字面未触发；但**单 run 授权已用尽**（裁定②：最后一轮），无论何型均不再重跑。
- 与 run1 归因①（LLM 概率腿 settle 边界停）**同族不同位**：run1=首轮瞬态+G7/G8 blocked 于 apply 后验证；run3=apply 完成后观察轮连续两轮 60s 硬超时。LLM 服务质量（60s 整超时连续两现）为本 run 首要候选归因；agent 治理面（settle 拒绝/欠账扣发）行为诚实无缺陷证据。
- capability_blocked **未再现**（裁定④两现条件仍不满足）。

## 交主管裁定项

1. **回归门收口口径**：274/292 消费面真栈实证由 run2 承担（主管已实质 pass）；run3 未新增门组覆盖。D2 分支感知断言修正（裁定①）离线双形状干跑 8 形状全符合在卷（run3/dryrun_console.txt），**活栈兑现仍空**——三轮 runner 均未能把链送到 D2 门组位（三型三因：G7/G8 blocked / 0-mutation 边界 / LLM 双超时）。
2. **D3 门组活栈覆盖仍空**（三轮均未到达）：是否拆最小前置腿（如单独起栈跑 D2+D3 门组、跳过 runner 主链——脚本是否有此形态需查证）或另开收口卡，由主管定。
3. **LLM 60s 连续双超时**（turn33/34，ms=60000/60001 整）是否升级为环境/服务面问题记录（本机 deepseek-flash 晚窗服务质量；run1 首轮超时+run3 双超时=三现超时行跨轮分布），由主管定。
4. 尾段 track.add_audio 双 error 与「空轨身份」decision 内容是否开取证卡（FX 新轨观察数据面品质，265/296 族已知边界），由主管定。

## 工件清单（run3/）

RUNBOOK.md（§8 跑前成文）·dryrun_console.txt+dryrun_d2_branch_assert.ps1（裁定①离线双形状干跑）·console_run3.txt·ps1_exit_run3.txt·run3_meta.txt·d1_smoke_report.json（全量 14.2MB）·agent_last.log·agent_binary_sha256.txt
