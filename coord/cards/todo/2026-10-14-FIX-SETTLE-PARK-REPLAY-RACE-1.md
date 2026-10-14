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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / seam 依据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
