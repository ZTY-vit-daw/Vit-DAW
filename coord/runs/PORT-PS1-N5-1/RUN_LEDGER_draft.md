# PORT-PS1-N5-1 RUN_LEDGER 草稿（执行侧工作文件，不提交）

- 测试 HEAD：7b8bb1cc（分支 port/ps1-n5-1；代码树=origin/main 117e9307 的 scripts/agent/VitApp，零改动统计卡）
- 引擎口径：deepseek-v4-flash（~/.vit/config.json 未动，defaultModel 实证）；白名单 ~/.vit/free_state_experiment_plugins.json v5 在位
- 达标线：每件 ≥4/5 绿（exit 0）；§8：各 5 轮固定不加跑，环境类不计有效轮

## ④ run_mix_single_tick_e2e.ps1（observe 回合/vocal 守卫/双跳确认链）

| 轮 | console 日志 | run ID（conv 时间戳） | 退出码 | 终态+族谱 | 环境事件 |
|---|---|---|---|---|---|
| R1 | run1_mix_single_tick_round1.log | observe=20260921_101048 / vocal=20260921_101205 | 1 | 红；observe 绿（提案面 needs_confirmation）→vocal 守卫 settle 后 waiting_clarification 但 reply=「已完成 ccb_observation_request」无"哪条轨"问句 → **F2 中间态标记**（vocal 守卫子回合） | 无（红后 agent/kernel 残留占 7878/5555-5557，已 taskkill 清理，未入断言阶段外行为） |
| R2 | run2_mix_single_tick_round2.log | observe=20260921_101536 / vocal=20260921_101712 | 0 | 绿；**正路径**：observe 提案面 needs_confirmation→vocal 守卫推断分支停车无 tick→hop-1 工具面（stored Track 1010 -0.50dB）→hop-2 d1_post_action_evaluation_required（回读 0→-0.5dB） | 无 |
| R3 | run3_mix_single_tick_round3.log | observe=20260921_101925 / vocal=（日志内） | 0 | 绿；**正路径**：observe 提案面→hop-1 工具面（stored Track 1007 -0.50dB）→hop-2 d1_post_action_evaluation_required（回读 0→-0.5dB） | 无 |
| R4 | run4_mix_single_tick_round4.log | observe=20260921_102317 | 1 | 红；observe 回合 goal=completed/**stop_reason=done**（needs_confirmation=false，turn_kind=settle_slice），reply=完整提案散文「建议候选是把 Track 2 降低 1 dB，等你在确认环节点头后再动…不涉及任何插件」→ **F4 done 不停车**（内容完整终态框架错）；证据=console log+run4_..._seq12_evidence.json | 无 |
| R5 | run5_mix_single_tick_round5.log | observe=20260921_102626 | 0 | 绿；**正路径**：observe 提案面（等待确认 -0.5dB 候选）→hop-1 工具面（stored Track 1007 -0.50dB）→hop-2 d1_post_action_evaluation_required（回读 0→-0.5dB）；vocal 守卫 needs_confirmation 推断分支 | 无 |

**④ 达标判定：3/5 绿（R2/R3/R5）→ 不达标（<4/5）**。红轮族谱：R1=F2（vocal 守卫子回合中间态「已完成 ccb_observation_request」）、R4=F4（observe 回合完整提案散文+stop_reason=done）。两红均模型终态选择类（诚实终止族邻域），无环境类红轮、无同断点两败。

## ⑤ run_vit_product_path_smoke.ps1（vocal focus settle stop_reason 定族）

| 轮 | console 日志 | run ID（工件目录时间戳） | 退出码 | 终态+族谱 | 环境事件 |
|---|---|---|---|---|---|
| R1 | run6_product_path_round1.log | product_path_20260921_103115 | 0 | 绿；**正路径**：vocal focus settle→needs_confirmation 提案面（等待确认…presence 频段提升提案） | 无 |
| R2 | run7_product_path_round2.log | product_path_20260921_103431 | 0 | 绿；**正路径**：vocal focus 提案面（Lead Vocal Track 1 presence 提升+A/B 提案） | 无 |
| R3 | run8_product_path_round3.log | product_path_20260921_103919 | 1 | 红；vocal focus 本身绿（提案面）；红点=clarify ask durable continuation 链 7 轮后 **goal=failed**（agent_log_tail：status=failed stop=failed completed_steps=0；chat_vocal_clarify_ask.json：stop=limit_reached reply=回合失败）→ **F3 无可采纳终态**（SYNC-3 观察过的 goal=failed 形态复发，agent 侧取证对象） | 无 |
| R4 | run9_product_path_round4.log | product_path_20260921_104355 | 1 | 红；未及 vocal focus；红点=Chinese multitrack observe 声学桥断言：band_energy_summary.request_id=mixboard_20260921T024454… vs latest_request=kernel_prepared_spectral_field_1016（kernel/mixboard 双备谱路径瞬态竞态；工件 authoritative 快照终态一致 kernel_prepared_waveform_envelope_1016/ready）→ **F5 复合与环境**（机器复合，非模型终态族；红轮照计通过率，镜像 MAC-4 白名单 F5 轮计入 0/5 口径） | 无 |
| R5 | run10_product_path_round5.log | product_path_20260921_104638 | 0（pass line 锚定：'product-path lifecycle + mix smoke passed'+Summary 段+栈自清；该轮 shell 包装未挂 exit 捕获，日志终局无异常块） | 绿；**正路径**：vocal focus 提案面（track 1007 presence band 提案，RMS/headroom 证据完整） | 无 |

**⑤ 达标判定：3/5 绿（R1/R2/R5）→ 不达标（<4/5）**。**vocal focus 提案面停车率=4/4**（凡跑到该阶段的轮全停车：R1/R2/R3/R5；R4 未及该阶段）——PC 历史口径 7/7 延伸至 **11/11**，放大器假说标的（vocal focus 停车）在 PC 侧保持 100%。红轮均非 vocal focus 停车失败：F3 在 clarify ask 子回合、F5 在声学桥前置阶段。

## 三列对照表骨架（族谱标签 × 三口径）

| 族谱标签 | PC-N5 本卡（flash+白名单在位） | MAC-4 基线（flash+无白名单） | WL-1 复跑（flash+白名单，mac） |
|---|---|---|---|
| **④ 通过率** | **3/5** | 2/5 | （mac 回执后拼入） |
| ④ 正路径（提案面停车→双跳全链） | 3 | 2 | 待拼 |
| ④ F1 能力边界声明 | 0 | 2 | 待拼 |
| ④ F2 中间态标记 | 1 | ?（mac RUN_LEDGER 待拼；通报只点名 F1×2） | 待拼 |
| ④ F3 无可采纳终态 | 0 | ?（同上） | 待拼 |
| ④ F4 done 不停车 | 1 | 0（⑦ 传导 1 例非 ④⑤） | 待拼 |
| ④ F5 复合与环境（单列） | 0 | 0 | 待拼 |
| **⑤ 通过率** | **3/5** | 0/5 | （mac 回执后拼入） |
| ⑤ 正路径（vocal focus 提案面停车） | 3（停车率 4/4，R4 未及） | 0 | 待拼 |
| ⑤ F1 能力边界声明 | 0 | 0 | 待拼 |
| ⑤ F2 中间态标记 | 0 | ≥1（通报「②⑤ 各见」） | 待拼 |
| ⑤ F3 无可采纳终态 | 1（clarify ask 子回合） | 2 | 待拼 |
| ⑤ F4 done 不停车 | 0 | 0 | 待拼 |
| ⑤ F5 复合与环境（单列） | 1（声学桥 request_id 竞态） | 1（白名单缺失→d1_execution_blocked） | 待拼 |

（MAC-4 列来源：coord/reports/2026-09-21-mac-45-situation-for-pc.md §二；④ 的 3 个红轮与 ⑤ 的 5 个红轮中各有 1 轮族谱未在通报点名，标 ? 待 mac 侧 RUN_LEDGER 拼入。）

## §8 账目

- 轮次：④⑤ 各 5 轮固定，无加跑、无轮内作废重跑（零脚本缺陷修复需求；SMOKE_TESTS.md 统计口径两行为卡面预期内文档补充，非缺陷修复）。
- 环境中断：0 轮。轮间 taskkill 清理 ④ 栈残留（红轮 R1/R4 抛错后与绿轮 R2/R3/R5 后各一次）均为轮后处置，不影响各轮退出码判定；⑤ 各轮栈自清。
- LLM key：零入工件（run4 events JSON grep sk-=0；⑤ 工件为脚本生成的 chat/tool JSON，不含 config；console 日志无 key）。引擎=deepseek-v4-flash（~/.vit/config.json 全程未动，defaultModel 实证）。
- 连续环境故障：0 次（未触发 ≥3 暂停线）。
- 测试时 HEAD：领取 7b8bb1cc（=origin/main 117e9307 代码树）；④⑤ 全部 10 轮跑于该代码树（branch port/ps1-n5-1 检出，SMOKE_TESTS.md 行在全部轮次完成后才写）。
