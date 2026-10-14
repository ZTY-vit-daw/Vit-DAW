# FIX-SETTLE-PARK-REPLAY-RACE-1：停泊人工裁决被旧修订观察回放机器结算压过（109 线第四缺陷修复）

- 发卡：GLM 主管决策侧 / 2026-10-14 深夜（依据=[REPORT.md §九](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/REPORT.md) 根因链+rerun5 上交；109 收口腿 exit 0 前置）
- 派发确认：已确认（主管依已核实取证证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 115；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1（109 硬阻断） / 0.5 天 / 无
- 模型分级：L2 / GLM 执行会话（涉闭包结算语义）

## 缺陷机制（已核实，REPORT §九）

合法 settle 停泊 user_judgment_pending 后 1 秒内：停泊轮闭包回放（chat/audio_closure_controller.go:596-634）把 pre-action 旧修订观察再水合——requires_post_action_observation 已翻 false 故过滤器放行，闭包 Observations 已被 EventProjectRevisionChanged fold（audioclosure/events.go:199-212，按设计）清空故去重失效——旧修订观察（key rev≠闭包 rev，且 SupersededProjectRevisions 为空）撞 driver.go:183 跨修订守卫→settleUnchecked(StopProjectRevisionStale) 机器结算压过停泊→FS-SETTLE-TERMINAL-1 断言（free_state_d1_smoke.py:1106）失败。烟测 2s 轮询必输 1s 竞速=exit 0 结构性不可达，模型档位无关。

## 修复面（三候选，取证后定 seam 并写设计依据）

- (a) 停泊态不回放旧修订观察：round decision=user_judgment_pending / experiment=waiting_for_user 时，回放侧跳过 revision 不匹配的观察。
- (b) fold 登记 superseded：EventProjectRevisionChanged 把被替换 revision 记入 SupersededProjectRevisions（与 governed booking 对齐）——使守卫按既有"superseded=跳过"分支处理而非结算。
- (c) 停泊后闭包回放整段短路。

## 硬约束

**不弱化跨修订守卫语义**（守卫按设计工作：防旧修订证据伪装新鲜；修的是回放侧不该把旧修订观察喂进停泊闭包）；守卫对非停泊路径行为不变；不得让停泊态接受旧修订证据为新鲜（只许"不回放/跳过"，不许"当作有效"）。

## 任务

1. 取证短评（三候选比较，定 seam 写依据）。
2. 修复+单测三形态钉：①停泊态旧修订观察不回放（或等价 seam 效果）；②停泊后模拟回放+2s 窗口不被机器结算（FS-SETTLE-TERMINAL-1 不变量钉）；③非停泊回放路径行为不变（红面：守卫对非停泊旧修订观察仍结算/跳过按既有语义）。
3. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob）。纯单测收口不占真栈（复跑归 109 rerun6）。

## 文件域

agent/internal/chat/audio_closure_controller.go+agent/internal/audioclosure/（events.go/driver.go 按需）+同包测试；不得动 free_state settle/final gate 与 MaterialityEvaluation（109 腿 D/E 域）与 harness 身份面（112 域）。

## 停止条件

三候选均与停泊不变量/回放既有承重语义冲突（修不动）→带分析上交；域外新发现→上交不扩域。

- 领取：2026-10-14 / origin/main=8c286757（卡在远端 todo、派发已确认、无依赖；doing=109 域 free_state_experiment_runtime+goalrunner 不相交、114 前端仓域）/ GLM-5.3 L2 执行会话（PC，ZCode）/ port/fix-settle-park-replay-race-1 / D:/Vit_DAW_wt_fsprr1 / 领取提交=本提交
- 回执：**commit a4bcdf33**（port/fix-settle-park-replay-race-1 已推 origin；chat/audio_closure_controller.go +20/-1 + 新测试 audio_closure_parked_replay_test.go 302 行）。**seam=候选(a) 停泊态回放侧跳过**——取证：候选(b) EventProjectRevisionChanged fold 登记 superseded 会使全部非停泊路径的旧谱系修订观察从 settle 变 skip（revalidation 通道与 governed booking 语义不同：fold 按设计作废旧修订证据，守卫兜底仍需），直接违反"守卫对非停泊路径行为不变"；候选(c) 整段短路连坐 frontier 更新/任务态投影/协议修复记账，且停泊窗口内 settle-family 轮仍需走同一路径；(a) 与 SETTLE-DELIVER-1 症状 B 先例（:582-589 修回放喂入面不动守卫）同构、与 :798 FS-SETTLE-TERMINAL-1 延迟同谓词族（freeStateJudgmentBoundary），唯一同时满足三硬约束。实现：回放循环在 loop 停泊于人工裁决边界时跳过 key.ProjectRevision 非空且≠闭包当前修订的观察（**精确不等、逐字镜像 driver.go:182 守卫触发谓词**，不漏 "REV4"/"rev4" 形态）；置于 dedup/证据顶之前（旧修订观察不得消耗 ceiling）；守卫与 audioclosure 零改动；停泊态只跳过、绝不采纳为有效。**单测三钉红绿已验**（stash 还原基线复跑）：① TestParkedJudgmentReplaySkipsPreActionRevisionObservations（修复前 FAIL 复现 run10 缺陷：停泊闭包被 machine settle；修复后 PASS：旧修订行不入账+轮完成+停泊与试听会话存活）② TestJudgmentParkedClosureSurvivesContinuationReplayWindow（修复前 FAIL；修复后停泊后两次 continuation 回放窗口零 project_revision_stale 结算事件=FS-SETTLE-TERMINAL-1 不变量）③ TestNonParkedReplayKeepsCrossRevisionGuardSettleSemantics（两版 PASS：非停泊同形状回放仍以 project_revision_stale 结算，守卫语义原样）。**自验**：`go test ./... -count=1` exit 0、95 包 0 FAIL；两文件 LF blob gofmt 净。**端测边界声明**：纯单测收口，零真栈运行（本卡域内无真栈要求）；真栈链路验证（合法 settle 停泊→judgment POST 下游）归 109 rerun6 复跑，本修复为其 exit 0 前置。文件域遵守：仅 chat/audio_closure_controller.go+同包测试，未动 settle/final gate、MaterialityEvaluation、harness 身份面、audioclosure。
- 验收：（裁定文件 / 验收 commit）
