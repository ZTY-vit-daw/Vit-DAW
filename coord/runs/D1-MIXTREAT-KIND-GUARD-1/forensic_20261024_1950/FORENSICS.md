# D1-MIXTREAT-KIND-GUARD-1 取证（2026-10-24 19:49–19:50，用户手测现场）

- 取证人：GLM 主管决策流（ZCode 验收会话）；现场=用户 19:48 从 Godot 前端拉起的三件套（VitApp 17716+VspHub 6636+VitAgent 34128，agent=主检出 bin，含全部已合入 Go 改动；今日两卡前端域、Go 零改动——本缺陷与 183/179 无关）。
- 证据固化：agent_last_rolling_snapshot.log（滚动 500 行，固化时未冲掉）+ agent_runtime_state_snapshot.json（会话 webui_mvmc6dkv 完整状态）。只读取证，未触碰现场进程。

## 会话原文（runtime_state 快照）

| 轮 | 角色 | 内容 |
|---|---|---|
| 1 | user | 给人声加点空间感（场次③标准话术，8 字） |
| 2 | assistant | 我还在继续处理这个任务…按你的指名，目标轨=vocals/1032。 |
| 3 | user | 可以执行（approve，interaction_74ea6d9b1b0637ab） |
| 4 | assistant | **D1-S1 requires one bounded reverb_decay_adjust**（原始英文内部错误直接作为回复） |

## 时间线（agent_last.log 19:49:50–19:50:32）

1. 19:49:53 free-state.seed booked（observation booked，scan_satisfied=true）。
2. 19:50:14 第 3 轮 CCB 观察 obs_20261024T115014_46f54bbe5cc1 → 由此武装 D1-S1 准入：**reverb 域，action_kind=reverb_decay_adjust，decay_pct=5，target=track:1032**（假设文本含「左右相关性偏高、声像偏窄且尾音偏短」——pan 观察被织入 reverb 假设）。
3. 19:50:31 第 4 轮模型回复提出改善治疗=**pan_balance**（processor=utility，target=track:1032）→ `[mix.treatment.pending] stored`（goalrunner_chat.go:3179，**无 action_kind vs 活跃准入一致性校验**）→ stop=semantic_treatment_capability_boundary → f6=waiting_interaction（交互卡向用户征询执行确认）。
4. 19:50:31 resolver：status=**ready_pan_tick**（pan 候选就绪）。
5. 19:50:32 用户 approve → executeResolvedMixTreatmentMixTick（mix_treatment_confirmation.go:370）构造 candidate.Operation=**track_pan_adjust** → executePendingMixTickCandidate（mix_tick_confirmation.go:228）→ 活跃准入 PluginBound（reverb）→ **executeD1StaticEQ 冻结口** → d1PluginParamPlanWithBinding（free_state_d1_plan_table.go:739-741）`candidate.Operation != spec.ActionKind` → error → d1BlockedResponse（free_state_d1_runtime.go:790，**Reply=原始 reason**）→ 用户看到英文报错。
6. 执行面零 mutation（executed=0、gate skipped zero-change turn）——fail-closed 有效，现场工程未被改动。

## 缺陷分类（两层）

1. **产品缺口（主）**：语义治疗 pending 存储/交互卡发布前，未与活跃 D1-S1 准入做 **action_kind 一致性**校验；不匹配的治疗照常走「就绪→征询→批准」全流程，到执行口才 fail-closed——用户被引导批准了一个永不可能执行的动作。一致性校验仅存在于冻结深处（同函数还有 target 一致性门「D1-S1 candidate target does not match admitted observation target」，同为执行口迟检——157 卡回执已点名 109/151 族邻面，本例是同族的动作种类维度）。
2. **回复面缺陷（次）**：d1BlockedResponse 把内部英文 reason 直接作为 ChatResponse.Reply（free_state_d1_runtime.go:790 `Error: reason, Reply: reason`），用户旅程面漏内部错误文本，无本地化无下一步引导（对照 mix_treatment_confirmation.go 各 status 已有中文事件文案）。
3. 上游模型行为（记录不立案）：准入=reverb_decay_adjust 而模型提案=pan_balance——146/159 族「提案与准入一致性」上游面的新实例；修复不得依赖模型自觉，须结构性守卫。

## 复现要件

- 话术「给人声加点空间感」→ 任意让模型以 pan/gain 类 utility 治疗收束的随机分支即可复现（模型行为分支）；结构性复现=在活跃 PluginBound D1-S1 准入下 approve 一个异种类 mix tick 候选（单测可确定性构造）。

## 现场处置

- 现场由用户保留中；取证完成后可关（证据已固化本目录）。会话 goal 链 parked（continuation.park non_terminal=1）不影响新会话。
