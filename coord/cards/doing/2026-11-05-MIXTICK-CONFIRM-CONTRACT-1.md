# MIXTICK-CONFIRM-CONTRACT-1：mix_tick 确认卡契约行为回归取证（G6）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G6——**主管已裁定 needs_confirmation 仍是现行契约**：M3 治理链设计+249 组 2 同型话术挂卡先例+pending_intact 断言族）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 262；目标=取证为主（chat 措辞面 patch 或行为收窄二选一落点）；P2 / 0.5 天 / 依赖=257 工件在库；L1-L2；验证腿需真栈

## 目标

1. **取证**：mix_single_tick 场景（工件 coord/runs/AF-FAMILY-HEALTH-1/mix_tick_20261105_224459/）复现模型「观察→终局文字建议不挂卡」分支——对照 M3 措辞链前该腿通过形态，钉死漂移点（哪条指引/哪个措辞变化导致提案面不走 improvement_proposal）；
2. **修补裁定输入**：二选一——①措辞指引 patch（提案类话术须挂卡，chat 域小修）②该场景确属「纯建议不需卡」的话术族（则腿断言按裁定更新）——取证后交主管裁定再动手（本卡可先实现①若证据一边倒）；
3. 验证=修补后 mix_single_tick 单 run（§8 预算 ≈¥1-3 单 run）。

## 约束

文件域=chat 措辞面最小修或 scripts 腿断言面二选一（按裁定）；禁动治理链本体；M3 措辞链成果不回退（needle 面不回退）。

## 领取/回执/验收

- 领取：2026-11-06T18:54+08:00 / origin/main=bc33eb2e / owner=GLM-5.3（PC，ZCode 执行会话 262impl） / 分支=port/mixtick-confirm-contract-1（worktree=D:\Vit_DAW_wt_262impl，实现基线=2a801e81） / 领取提交=本提交（仅含本卡状态，fast-forward → main）
- 回执：**取证腿完成（零 LLM 零栈）——卡前提被证据推翻，修补方向交主管裁定，实现/验证腿 HELD**。工件=port/mixtick-confirm-contract-1（commit ab798299）`coord/runs/MIXTICK-CONFIRM-CONTRACT-1/forensics.md`（结论+代码锚点）+`excerpts.md`（29 条原始输出/指纹面/T32 唯一 diff 摘录）。要点：①「模型不挂卡」不成立——29 条 loop debug 亲证模型始终输出结构化 needs_observation（带 tool_calls），从未给终局文字；切片后 T32 正确产 needs_experiment+improvement_proposal，响应面 improvement_proposal_confirmation_required+确认卡落库（timing :321），契约在模型侧与响应面均成立；②真实失败链三层：L1 模型 27 次重请求已返回 view set（要「相对电平」）被去重门弹回（M3+23% 同族背景，但 M3 增量逐条核对无一合法化该重复）→L2 弹回反馈未进入重建请求（T6-T31 指纹 fd94d36a×26 恒定，模型看不到拒绝史）→180s 排干切片→L3 settle 只读 goal.status（runner res=completed/done 而确认面实在 continuations[].pending_interaction，未读）→got 'done'；③决定性证据：T32 与 T6 的 directives 唯一 diff=`ccb_model_prompt.go:292` 闭合句「The evidence you hold is sufficient for a bounded proposal…」633 字符，注入即提案；④裁定选项：**A**（chat 措辞面）:292 句在「台账同 view set 已 usable」条件（去重门同判据）下提前进 turn_directives / **B**（scripts 面）settle 在 goal=completed 但 pending_interaction 有 answerable park 时走既有重建 walk 恢复确认面；①原措辞（提案话术须挂卡）不对症、②（断言收窄）证据反对；域外发现两项（L2 反馈丢失=message_loop.go:10947 append 被重建丢弃；L3 goal 级翻转点未精确定位——245 回执已记录同族 goal/durable 分歧缺口）建议另开卡。验证腿 HELD 双因：裁定未下+真栈被 264（B4-ABLATION-1，19:22 登记）占用中。
- 验收（2026-11-06 晚窗，主管·阶段性）：**取证腿 pass+裁定=A（chat 措辞面）**——①「模型不挂卡」前提推翻接受（29 条亲证+T32 决定性证据，模型侧契约成立）；②真实失败链 L1（去重门 27 次弹回）+L2（弹回反馈不进重建请求）=**生产行为面真缺陷**（模型被困 180s 靠排干救出），L3=仪器缺陷（settle 读错面）；③裁定理由：T32 证明注入即提案=链路健康仅 timing 晚——**A 治本**（292 闭合句在去重门同判据条件下提前进 turn_directives），A 生效后 observe turn 响应即 improvement_proposal_confirmation_required，L3 形态大概率不再触发；**B（settle 重建 walk）暂不动**，留分叉（验证腿若仍见 L3 形态再补小修）；原选项①②按取证证据弃用。域外两项处置：L2 反馈丢失（message_loop.go:10947 append 被重建丢弃）=真学习面缺陷**开卡 267**；L3 goal 翻转点挂 245 族既有缺口不新开。实现腿（A patch）+验证腿（单 run ≈¥1-3）按用户边界待 265 检查后随自主批次派发（栈现为 265 占用）。
- 回执·实现腿（2026-11-06 22:00+08:00，262impl 会话；receipt 见 port 分支 2bbe640f）：**裁定 A patch 完成**——impl commit 72718892：:292 闭合句族提取为 messageLoopFreeStateClosureGuidance（逐字节恒定，测试锚定）+新增 messageLoopFreeStateDuplicateViewSetClosureDirective（去重门同判据 messageLoopFreeStateAlreadyObservedIssue 条件下提前进 turn_directives）；不新增话术族、needle 面只增、message_loop.go 与门体零改动；新增 4 组配对测试；go build exit 0+agentloop/chat 全包绿；origin/main@46497e3b 已合入（6dbb28a4）。
- 回执·验证腿（2026-11-07 00:00+08:00，262impl 验证会话）：**单 run 20261106_234509 PS1 exit 1——判定=③断言失败/新形态取证上交（非①PASS 非②弹回族），单 run 无重跑兑现**。被测=wt_262impl @ 0b686c80（origin/main e2651b74 合入无冲突+build/agentloop/chat 绿）；agent=现场构建 SHA256 B280318D…；kernel=wt_228 F9E3DA0A 只读复用；引擎=deepseek-flash 默认无注入；占栈协议全履约（净空 23:44:13→登记 0d93336a→≤60s 复核 23:44:57→单 run→TEARDOWN_ALL_ZERO 亲核 23:54:55→释放登记 6ff6c781）。**要点**：①模型 6 轮观察内一击产结构化 needs_experiment+有效 improvement_proposal.v1（proposal_valid=true，target=track:1010，track_gain_adjust -0.5dB，pending_candidates 在库）——**257 弹回族零复发零切片零排干**（59.9s 完成观察段）；②A patch 触发条件（重复 view set 请求）本 run **从未发生**——prompt render 6 轮亲证 A 指令与 :292 闭合句族均未注入，模型完全自主到达提案（LLM 随机分支换了探索路径：structure→两轨 energy→目标轨 time_dynamics→提案）；③真实阻断点=**D1-S1 实验准入门拒绝且终局化（262 文件域外）**：提案 evidence_refs 引用同目标更早观察 obs_154552(basic_energy)+seed，门绑定 loop.LatestObservation=obs_154607(time_dynamics) 要求必含其 ID（free_state_experiment_runtime.go:176-179）→free_state_reasoning_loop.go:1684-1693 改写 blocked/free_state_experiment_admission_invalid→响应面终局 done→turn_close_guard.go:56 孤儿收口，确认卡未建（与 257 L3 形态也不同族——本 run 响应面本身即 done，非 settle 读错面）；④计费 7 调用 in 101,603+cache 22,784/out 8,503 tok ≈¥0.1-0.2（预算 ≈¥1-3 远低上界）；⑤修补方向三选项（准入拒绝改回喂/挂「方案乙 park」确认面｜门校验放宽为任一目标级新鲜观察｜措辞面教引用最新观察=域内但新话术族）交主管裁定——见 verifyrun1/evidence.md §三。工件=port/mixtick-confirm-contract-1（2acea3a8）coord/runs/MIXTICK-CONFIRM-CONTRACT-1/verifyrun1/（RUNBOOK §8 跑前成文 a05cea2b+console+telemetry+agent_last+runtime state+evidence 六件）。
- （最终验收：实现+验证腿完成后回填——验证腿已交取证，待主管对新形态处置裁定后闭合）
