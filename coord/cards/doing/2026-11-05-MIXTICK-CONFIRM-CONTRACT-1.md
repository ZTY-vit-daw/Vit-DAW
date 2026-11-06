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
- （最终验收：实现+验证腿完成后回填）
