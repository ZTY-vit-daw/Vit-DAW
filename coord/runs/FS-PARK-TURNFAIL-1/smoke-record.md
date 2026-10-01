# FS-PARK-TURNFAIL-1 真栈烟测记录

- 脚本：`scripts/run_free_state_d1_smoke.ps1 -ContinuationAdoptionProbe`（本卡新增参数，扩展既有 D1 烟测体系，python 侧 `--continuation-adoption-probe` 探针 `run_continuation_adoption_probe`）
- 栈：VitApp 内核（主仓 Release 构建，本卡 VitApp/ 零改动）+ Godot 前端（D:\Godot\project\vit-daw-frontend）+ worktree 构建 agent（sha256 前 16 位：run1=d5cd18dcbbdbd738 / run2=cc593b1f514f649f，`-RestartAgent` 安装）
- 场景：观察问答→自由态实验（static_eq）→应用→settle 报告→judgment park（fs7/waiting_continue，与 2026-09-30 野外失败同形态，`fs_settle_terminal_park` 校验过）→**park 存活下新用户消息**（"换一个话题：帮我看看当前工程的整体状况，先观察再简要回答"）
- §8 声明（执行前）：最多 2 轮有效；成功=exit 0；失败分类 no_candidate_found/capability_blocked/断言失败/崩溃/环境中断分开记录；同断点连续 2 次非环境失败止损上交。

## Run 1（红→归因，20261001_112221，exit 1）

断言失败："the parked loop did not settle on continuation: blocked"。归因（非环境、非原样重跑）：

1. **修复主体已生效**：续入消息未 turn.failed、新 goal/run（goal_30b9d816≠parked goal_3f9be307）、回复落盘、agent 日志 `[judgment-park-continuation] parked round adopted by continuation`——采纳结算链完整走完。
2. **缺陷①（探针断言面）**：会话 loop 槽为单槽设计，续入消息的新 loop 替换了已结算 loop；采纳结算的持久真值在 parked goal 的任务语义（settled/terminal/history 末条 task_settled 带采纳 summary）与事件流——探针改按持久面断言。
3. **缺陷②（trajectory 校验门）**：`EvaluationState.Valid()` 未含 adopted_by_continuation → `trajectory.settled` 事件被拒（`unsupported trajectory outcome`）——补枚举（types.go:304）。

修复两处后重建 agent 重跑（补锚点重跑，非原样）。

## Run 2（绿，20261001_113015，**exit 0 PASS**）

- `D1-S1 CONTINUATION_ADOPTION PASS` + `D1-S1 PASS`（validate_d1 常规断言也全过：static_eq 域 exercised）
- park 校验（探针前置）：`{"goal_status":"waiting_continue","judgment_park":true,"closure_phase":"fs7_improvement_proposal"}` —— 与野外失败同形态的 park 下进入探针
- 采纳探针断言全绿：
  - (a) 无 turn.failed / 无 error（修复前红面：2026-09-30 野外 seq47 turn.failed + task 全线 failed）
  - (b) 新 turn id：parked goal_8f6855a03b55134c/run_dee81121866572da → continuation goal_55b99e117bc73580/run_91081cc58b9a52c7（生命周期翻转根除）
  - (c) 回答落盘（reply 非空）
  - (d) 采纳结算：parked goal=completed；task semantic=settled/terminal/pending_interaction 清除；history 末条 task_settled summary 含 adopted_by_continuation；事件流 trajectory.round.decision(next_decision=adopted_by_continuation)+trajectory.settled(outcome=adopted_by_continuation) 落地；**无 trajectory.user_judgment.recorded（无人耳判断被伪造）**
  - (e) 已应用状态保留：project revision 7 前后不变
- 工件：`artifacts-green-20261001_113015/`（d1_smoke_report.json + agent_last.log + console.log）、`artifacts-red-20261001_112221/`（run1 报告）；完整 run 工件另存 worktree `artifacts/free_state_d1_s1/20261001_113015/`
- 泊位声明：烟测完成即拆除运行栈（7878/5555 无监听，Godot 前端已停）——**栈已拆除，未移交**

## 红绿对照

- 红（修复前）：野外取证 coord/runs/M8-FORENSIC-20260930/evidence/（同场景 turn.failed 事件流 seq47 + message-loop 三拒链）——M8 事故即红轮文档。
- 绿（修复后）：本轮 run 2（上）。

## 端测覆盖边界声明（AGENTS §5 渲染面条款）

本烟测覆盖 agent HTTP 面（/agent/chat、/agent/events、runtime 状态、project.state）与内核栈；**未覆盖** webui 渲染面（A/B 卡终态样式渲染、消息序动效）——A/B 卡联动归 AB-JUDGMENT-CARD-1，消息序归 WEBUI-MSG-ORDER-1（会话②并行卡）。
