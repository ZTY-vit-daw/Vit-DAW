# D1-MIXTREAT-KIND-GUARD-1(184) 真栈 chat 腿结果（2026-10-24 晚窗）

- 执行：GLM-5.3 L2 PC 执行会话（ZCode，184 卡会话）；占用登记 360daafd（§2.2，占用前核对进程/端口=0，用户 Godot 编辑器 8736 在场未触碰）。
- 被测：agent=worktree D:/Vit_DAW_wt_d1guard1@port/d1-mixtreat-kind-guard-1 构建（含守卫改动）；kernel=主检出 K3 VitApp.exe SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC 只读亲核；journey berth 隔离（kernel workdir+VIT_PROJECT_XML=run 目录 git 干净副本、VIT_HISTORY_DRAFT_ROOT=run 目录）。
- 入口：`dev_agent_smoke.ps1 -Scenario journey_free_state_nl -JourneyNlPromptBase64 57uZ5Lq65aOw5Yqg54K556m66Ze05oSf`（=「给人声加点空间感」，取证话术逐字复刻，journey_nl_prompt.txt 留档）。
- §8 预算（跑前写明于占用记录）：≤3 轮独立调用；成功=exit 0+用户面英文错误签名零出现+拓扑读回零非法变更（逐轮不变量）；失败分类=环境中断（修复重跑不计轮）/LLM shape miss（不变量仍须绿，形态记录）/确定性断言失败（同签名两次即停）。

## 三轮结果

| 轮 | run 目录 | exit | 形态分类 | needle 扫描 | 拓扑差分 | verdict |
|---|---|---|---|---|---|---|
| 1 | chatnl_20261024_205621 | 0 | no_candidate_found（limit_reached 单轮+续跑收敛） | 0 命中（2 响应+30 事件） | 零漂移 | PASS |
| 2 | chatnl_20261024_210031 | 0 | no_candidate_found（同上） | 0 命中 | 零漂移* | PASS |
| 3 | chatnl_20261024_210506 | 0 | no_candidate_found（J3 分类器面）＋**守卫活触发**（见下） | 0 命中 | 零漂移 | PASS |

\* 轮 2 末态快照为旅程中段（21:01:26）；旅程后窗口探针 GET 全超时（见教训②）。

## Round 3 守卫活触发＝取证形态活复现（本腿核心证据）

agent 日志（chatnl_20261024_210506/agent_last_round3.log，21:06:39）：

```
[mix.treatment.pending] refused D1-S1 admission mismatch conversation=dev_journey_fs_nl_20261024_210519
  goal=goal_2d151e343dce5bc5 action=pan_balance processor=utility target=track:1012 mismatch=action_kind
[continuation.arm] ... status=completed stop=mix_treatment_refused_d1_admission_mismatch lifecycle=none
```

与用户手测现场（forensic_20261024_1950）逐字段对照：

| 维度 | 取证现场（19:50） | Round 3（21:06） |
|---|---|---|
| 话术 | 给人声加点空间感 | 同（base64 复刻） |
| D1-S1 准入 | reverb / reverb_decay_adjust / target vocals 1032 | reverb / reverb_decay_adjust / target 1012（Lead Vocal，运行时状态亲读） |
| 模型提案 | pan_balance（utility）track:1032 | pan_balance（utility）track:1012 |
| 旧行为 | 照存 pending+发可执行卡→approve→英文错误直漏 ChatReply | 拒收：零存储、零发卡、goal 诚实收敛 completed |
| mutation | 0（执行口 fail-closed 兜底） | 0（拒收切片 ledger 边界 executed=0——该轮零工具执行；applied_signal=false；write-lease 文件未创建） |

## 不变量证据面

- **英文签名零出现**：三轮全部用户面（chat 响应 reply×2/轮、hop 响应、交互卡 title/body、事件 title/body×30/轮）needle 扫描零命中（verify_chatnl.py，needles=冻结口错误族 11 条）。旧代码在同形态下会产出 "D1-S1 requires one bounded reverb_decay_adjust" 直漏（取证第 4 轮原文）。
- **拓扑读回零非法变更**：基线=脚本自产 journey_ui_state_after_open.json（2 轨 fixture）；终态=并发只读探针（GET /agent/ui/state，白名单 GET 模式）本轮栈存活窗口内末张快照（r1=0008/r2=0007/r3=0011，pan/volume/gain/fader/mute/solo/plugins/routing 逐字段差分零漂移）；佐证=applied_signal=false×3、mix_tick/applied/intervention 事件 0×3、[vit_checkpoint] zero-change 门记录、r3 拒收切片 executed=0。
- **收敛诚实**：r3 拒收回复=本地化引导文案（「当前实验轮限定为混响类的剂量动作，声像类调整不在本轮剂量范围内。没有修改工程。…」——回复文本由单测 TestPendingMixTreatmentRefusedUnderMismatchedD1S1Admission 钉在响应对象上）；goal 干净收敛、无不可应答 park（lifecycle=none）。

## 教训与边界（记录，不立案）

1. **J3 分类器不识别新 stop reason**：r3 的 mix_treatment_refused_d1_admission_mismatch 发生在后台续跑切片，J3 形态分类器只看 chat POST 响应的 stop_reason（limit_reached），故分类仍报 no_candidate_found——守卫触发以 agent 日志为准（不改脚本，脚本域外）。
2. **探针忙期 GET 超时**：/agent/ui/state 在链忙期（LLM/续跑处理中）5s 超时失败，r1/r2 旅程后窗口未捕获；r3 捕到 21:06:25（拒收前 14s）。后续同型腿建议 60-90s 超时或接受中段快照+executed=0 佐证。
3. **探针跨轮污染假阳性**：r2 探针未停且 40 分钟自杀窗跨入 r3 栈存活期，误捕 r3 berth 默认工程（6 模板轨，含撞号 1032）当作 r2 终态→验证器首跑误报 track_added×4；已加时间界参数修正后三轮全绿。探针应随轮停。
4. **scheduler 终局投递与拆栈竞态**：r3 拒收切片的 scheduler_chain turn.completed 终局事件（承载引导文案）未进 J3 事件捕获窗（捕获 ~21:06:46，拆栈 ~21:06:50）；投递机器（continuation_scheduler）在卡域外，记录不立案。活产品用户面（webui）经同一 chatResponseFromAgentLoopResult 产出——取证现场英文错误正是从该面漏出，本卡在该面换成引导文案。

## 端测边界声明（AGENTS §5）

本腿覆盖：真实三件套中 kernel（K3 真内核）+agent（含改动构建）+真实 LLM 旅程，agent HTTP 面（chat/events/runtime/ui.state）断言。**未覆盖**：webui 渲染面（浏览器级 DOM 断言）与 Godot 前端旅程——本卡改动为 agent 后端守卫与回复文案，不涉渲染层；渲染面边界是否需补腿由主管裁定。
