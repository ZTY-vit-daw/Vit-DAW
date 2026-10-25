# PROBE-DRYRUN-WARNING-1（池序 180）回执——探针头「干跑=必触栈」警告横幅

- 执行 owner：ZCode GLM-5.3 L1 PC 执行会话（ZCode，180 卡会话）/ 2026-10-25 晚窗
- 前端仓 commit：**f891117**（`port/probe-dryrun-warning-1`，基线 main=5da6360，已推远端）；改动单文件 `tools/stack_probe_sendroute_macro.gd`（+39/-0）
- 验证腿：无栈形态 A/B 对照跑，§2.2 登记 **d50cb06b**——本卡横幅纪律自守：探针任何模式（含本次验证干跑）先登记后跑

## 勘察结论（卡面①）

- 前端仓 stack_probe 场景模式入口**仅一族**：`tools/stack_probe_sendroute_macro.gd`（+`.tscn`）。grep 亲核：它是全仓唯一走 `kernel_command_client`/`VitIpcClient` 生产传输链的探针头，sendroute 族即全部栈探针面。
- 其余 `tools/probe_*.gd`（12 个）为独立 headless 探针（假客户端/纯函数面，边界声明在各自头注释），不走生产传输链、不触栈——无需横幅。**余量登记：无**（不存在第二个栈探针头）。

## 改动（卡面②③）

`tools/stack_probe_sendroute_macro.gd` 三处纯增量（详见 `probedrywarn_nostack_ab/frontend_change.diff`）：

1. `_print_stack_occupation_warning()`（`_ready()` 首行调用）：9 行显眼警告横幅——干跑=必触栈；任何模式（含所谓「干跑」/headless/只读腿）都走生产传输链 VitIpcClient UDP（指令 4445/遥测 4444）→ agent bridge → 内核 ZMQ；执行前必须完成 PC-RUNTIME-STACK §2.2 占用登记并成功推 main；未登记先跑=污染他人在飞栈（134/139/161/166 四起干跑误判同族事故）。
2. 机器可读行 `STACK_TOUCH_WARNING scene=<场景名> time=<ISO 时间戳> transport=VitIpcClient_UDP_4445->agent->kernel registration=PC-RUNTIME-STACK_s2.2 REQUIRED_BEFORE_ANY_RUN_incl_dry_run`——卡面③的时间戳+场景名要求。
3. `SUMMARY_JSON` 内嵌 `stack_warning` 字段（card/rule/scene/time）——工件报告第二源（console 经 ps1 捕获落 run 目录为第一源，双源达成）。

纯 print+字典键增量，零控制流/断言改动；文件头注释补一行横幅说明。

## 验证证据（卡面验收）

A/B 对照：基线=前端仓 detached@5da6360 worktree vs 带横幅 worktree（`port/probe-dryrun-warning-1`），同机同 Godot 4.6.1 console headless，**无栈形态**（不启 kernel/agent/vsphub；UDP 4445 指令无监听目标——IPC Timeout 即生产链触达的证据）：

- **横幅双源在场**（主证据 `probedrywarn_nostack_ab/changed_banner_console_cached.txt`）：启动头部横幅 9 行，含 `STACK_TOUCH_WARNING scene=stack_probe_sendroute_macro.tscn time=2026-10-25T18:48:48 ...`；尾行 `SUMMARY_JSON` 含 `stack_warning` 字段。
- **零回退**：A/B console 唯一差异=横幅 9 行+`stack_warning` 键；`FAIL setup clear_project | IPC Timeout` → SUMMARY fail → 退出码 1/1，与基线逐字节同构（无栈快速失败的正确 fail-closed 行为不变）。GDExtension `godot_cef` 缺库噪音两轮同样在场（预存环境噪音，非本卡）。
- 附首轮无类缓存跑（`*_console.txt`）：fresh-worktree 缺 `.godot` 类缓存致全局类名解析错误（183 回执已记录现象）基线/改动同样在场，A/B 仍只差横幅——两环境一致进一步证明纯增量。`--import` 生成类缓存后复跑即得主证据对。

## 端测边界声明（AGENTS §5）

- 本卡纯探针头 print 增量，零生产码/逻辑改动，卡面明示无真栈腿；本验证为无栈形态 A/B（§2.2 登记 d50cb06b，先登记后跑）。
- **未覆盖**：真栈全绿跑（stack 72/72 与横幅同现）——留待下次 SendRouteGuiSmoke（185 真栈腿或主管验收复跑）自然覆盖；烟测脚本 `dev_agent_smoke.ps1:8344` 仅断言 SUMMARY_JSON 行在场，与横幅行兼容（亲核，本次 SUMMARY_JSON 含 stack_warning 仍在同行的 JSON 串内）。
- 独立探针族（12 个）零触碰、零引用本文件（grep 亲核）；前端仓无其他文件引用 `stack_probe_sendroute_macro`（仅 .tscn 挂载与自身用法注释）。

## §8 声明

LLM=0 全确定性；4 轮探针进程跑（基线/改动 × 无类缓存/有类缓存）；成功条件=横幅双源在场+行为与基线逐字节同构（达成）；失败分类：无 assertion_fail / script_exception（IPC Timeout 为无栈形态预期内环境失败）；止损线未触发。

## 工件清单（probedrywarn_nostack_ab/）

- `changed_banner_console_cached.txt` — 主证据：横幅+STACK_TOUCH_WARNING+SUMMARY_JSON stack_warning
- `baseline_5da6360_console_cached.txt` — 基线对照（类缓存后）
- `changed_banner_console.txt` / `baseline_5da6360_console.txt` — 首轮（无类缓存环境噪音对照）
- `frontend_change.diff` — 前端仓 5da6360..f891117 单文件 diff
