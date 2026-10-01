# FS-ADOPT-CLOSURE-1 真栈烟测记录

- 脚本：`scripts/run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_worktrees\fs-adopt-closure-1 -PublicManifest <主仓 fixture> -ContinuationAdoptionProbe -SkipBuild -KernelExe <主仓 Release 内核>`（扩展现有 D1 烟测体系；探针=python `run_continuation_adoption_probe` 本卡扩展段 (e)/(f)）
- 栈：VitApp 内核（主仓 Release 构建，本卡 VitApp/ 零改动，复用先例）+ Godot 前端（D:\Godot\project\vit-daw-frontend）+ worktree 构建 agent（`-RestartAgent` 安装，`-SkipBuild` 模式，被测二进制 sha256 前 16 位=`bd3522044f6d3eb5`，worktree HEAD=23dfc239 含本卡修复，§9 记录）
- 场景：观察问答→自由态实验→应用→settle 报告→judgment park（fs7/waiting_continue）→**park 存活下新用户消息（采纳触发）→该消息自开的新 goal 必须真实观察执行→再追加第二个新 goal 会话不楔死**
- §8 声明（执行前）：最多 2 轮有效；成功=exit 0；失败分类 no_candidate_found/capability_blocked/断言失败/崩溃/环境中断分开记录；同断点连续 2 次非环境失败止损上交。
- 等栈约束（卡片）：用户活栈 20:32 起占 7878/5555——代码+单测先行，21:21:04 端口空闲（用户关栈）后才发起烟测。

## Run 1（红→归因，20261001_212148，D1-S1 FAIL：post-action CCB observation 缺失）

失败在 D1 前段（未到 park/探针）：模型提案件落在 native-tool confirmation 边界（stop_reason=improvement_proposal_native_tool_confirmation_required，waiting_confirmation），后续轮 limit_reached 零执行，实验未 apply。归因（**模型随机分支，非代码断点、非环境中断**）：

- 本卡三个代码标记（`[judgment-park-continuation]`、`[audio-closure] finished-task guard`、`adopted_by_continuation`）在该轮 agent 日志中**零出现**——本卡修改路径未参与该轮任何分支；
- VSP 8787 connect-refused WARN 为背景噪音（绿轮先例 FS-PARK 20261001_113015 同栈同噪音）。
- 分类=断言失败（D1 前段提案件走确认通道），消耗第 1/2 轮有效轮次；非同断点确定性失败，按 §8 允许重跑剩余轮次。

## Run 2（绿，20261001_212716，**exit 0 PASS**）

- `D1-S1 CONTINUATION_ADOPTION PASS` + `D1-S1 PASS`（validate_d1 常规断言全过）
- FS-PARK 原有断言面全绿：采纳触发消息无 error/非 failed、全新 goal（parked goal_2d549c4c9e461460 → continuation goal_a0adfb41b107a52b，run 同步全新）、回复落盘、parked goal=completed+task settled/terminal+task_settled 带采纳 summary、事件流 round.decision+settled=adopted_by_continuation、无人耳判断事件、revision 7 不变
- 本卡新增断言面（探针 (e)/(f)）全绿：
  - (e) 采纳触发消息自开的新 goal **真实观察执行**（执行证据断言过）+旧 closure 以诚实停因结算：`adopted_closure_reason=adopted_by_continuation`（绝不 satisfied/守卫兜底映射）；agent 日志锚点 `[judgment-park-continuation] settled adopted park closure=audio_closure_4e6fdb309571fab0 reason=adopted_by_continuation`
  - (f) 第二个新 goal（goal_504b9afa12810717）全新身份、无 error、非 failed、回复落盘、无零执行罐头句
  - 分层验证：绿轮 `finished-task guard` 零触发——源头层先行收口（守卫为纵深防御未上场，符合设计）
- 工件：`artifacts-red-20261001_212148/`、`artifacts-green-20261001_212716/`（各含 d1_smoke_report.json+digest；绿轮含 agent_last.log）；全量在 worktree `artifacts/free_state_d1_s1/`
- 泊位声明：烟测完成即拆除运行栈——**7878/5555/5556 无监听，Godot 前端已停，栈已拆除，未移交**

## 红绿对照

- 红（修复前）：M1 第四轮活栈取证（coord/runs/M1-RETEST-20261001/FORENSIC-NOTE.md 追记段+evidence/events-webui_mupimj6f.json）——goal_65e6beee 零执行吃 StopTaskSettled 罐头句。
- 单测红：fs_adopt_closure_test.go 四钉，源文件 stash 后三红一绿（修复前红面）。
- 绿（修复后）：本轮 Run 2（上）。

## 端测覆盖边界声明（AGENTS §5 渲染面条款）

本烟测覆盖 agent HTTP 面（/agent/chat、/agent/events、runtime 状态、project.state）与内核栈；**未覆盖** webui 渲染面。顺带记录（域外观察，未修）：诊断契约本轮完成时 audioClosureSettleFromResult 的 ContractID 分支把 StateSettled 映射 StopTaskSettled，措辞复用「任务已经满足…」罐头句——与零执行楔死无关（有真实执行+诚实结算），属既有措辞面，是否调整归决策侧。
