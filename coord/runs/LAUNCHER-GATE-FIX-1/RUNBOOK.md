# LAUNCHER-GATE-FIX-1（340）§8 跑前口径（冻结于任何真栈运行之前）

- 卡：coord/cards/doing/2026-11-10-LAUNCHER-GATE-FIX-1.md（339 取证转出的修复卡）
- 基线：origin/main 9e9117ae（领取提交；337/338/339 已合入）；前端 D:/Godot/project/vit-daw-frontend HEAD=ff301b3（335/336 已入本地 main，不动）；内核=部署二进制 D:/Vit_DAW/Export/staging/runtime/VitApp.exe（零内核改动，sha256 b6565dcf85d1da86 与 339 同版）
- 执行：ZCode GLM-5.3 L2 执行子代理（340gate 会话）/ PC / worktree D:/Vit_DAW_wt_340gate / 分支 port/launcher-gate-fix-1
- 被测 agent=worktree 现场构建（门修复：launcher_flow.go 门=UI 声明+user_entered_project 会话位，uuid 判据移除；server.go project_opened 接收点置位；驱动两处断言随门语义同步——assert_gate_open 改记录面/零摩擦前提改弱断言）
- 断言体：scripts/launcher_flow_smoke.py + dev_agent_smoke.ps1 `-LauncherFlow` 块（339 装置 + 340 门语义断言同步）
- 栈协议：coord/resources/PC-RUNTIME-STACK.md 两条登记（占用+释放）

## 本卡复跑范围（339 腿 1/2 复跑；腿 3 降级 339 已 PASS 且零 agent 不受门修复影响，不重跑）

| 腿 | 调用形态 | LLM | agent | UI |
|---|---|---|---|---|
| 1 主链 | `-LauncherFlow -LauncherFlowLeg main -StartKernel -RestartAgent -StartUI -NoChatSmoke -NoStripSilenceSmoke -KernelExe <部署内核> -RunArtifactsDir <340目录>/main_<stamp>` | 有（仅续聊轮，≤10 调用） | 起 | 起 |
| 2 退路 | `-LauncherFlow -LauncherFlowLeg fallback …同上…` | 零（telemetry 断言 0 行） | 起 | 起 |

腿间 owner 亲停上一腿残留（agent/内核/Godot），TEARDOWN 确认后再起下一腿；同腿内单 run 无重跑。

## 成功条件（逐腿，全部满足=exit 0）

- **腿 1 主链**（=339 RUNBOOK 原文，仅门前提更新）：greeting 端点 ok（launcher=true+我在）→ launcher chat「打开小星星乐局」（context.launcher=true）回执 `launcher_directive`（action=open_project、file_path=fixture、requires_governance=true、context_flow=launcher、确认句含《小星星乐局》）——**此即门开的行为证明（340：boot uuid/path 形态只记录不断言）** → UI 同链消费（kernel open_project[encryption=app_bound_aes]+version.project_opened lifecycle notify——**此即 user_entered_project 位置位点**）→ 绑定观察（current_project.path==fixture）→ 同会话次轮回执桥接播报（stop_reason=launcher_bridge_broadcast、context_flow=project、typed_events[0].type=volume_switch、reply 含「已进入《小星星乐局》」）→ 再一轮工程流对话（context_flow=project、reply 非空）→ 真实 UI 侧锚点（Godot 进程存活+内核日志含 UI 经 bridge 拉取的 get_recent_projects+无 launcher 相关 script error）→ telemetry ≤10 行。
- **腿 2 退路**：launcher chat「打开退路」回执 `project_result_cards`≥2（kind=launcher_project、内嵌 launcher_directive）→ 点卡[0] 同链开工程（open+绑定+桥接播报）→ telemetry 恰 0 行（零 LLM 证成）。
- **边界声明（AGENTS §5，照 339）**：GUI 渲染观感/真实点击/浮动窗视觉呈现不在断言面（headless 探针覆盖节点树+数据绑定+状态机；真实 UI 以进程/日志/内核命令锚点证在场）——GUI 最后一米=用户手测（手测 v2 入口=本卡 pass 后放行）。

## 运行次数与 LLM 预算（照 339 冻结口径）

- 每腿 1 次运行，无预演无重跑（装置缺陷/环境中断按下方失败分类处置，修复后该腿重跑 1 次记 attempt）。
- LLM 计费上界：**腿 1 ≤10 调用**（仅「续聊一轮」消耗；launcher 路由/桥接播报/greeting 均确定性零 LLM）。腿 2 = 0。合计上界 10 调用 ×40k in/8k out（≈¥0.05 以内）。
- 实际调用数以 VIT_AGENT_LLM_TELEMETRY_PATH 行数对账（腿 1 ≤10、腿 2 ==0）；超预算即 budget_exceeded。

## 失败分类（exit code 语义，分开记录不合并；照 339）

- `0` PASS（该腿成功条件全满足）。
- `2` env_interrupt：内核起不来/端口被占/构建失败/Godot exe 缺失/进程中途崩——保留原始 console+日志，不算有效轮。
- `3` intent_unresolved / mis_route（LLM 面）：腿 1 续聊轮 context_flow≠project 或空回复；或 launcher 轮意外落入 LLM 回落。
- `4` 确定性面缺陷（取证上交，不重跑）：directive/结果卡/桥接播报/绑定观察/内核 open 任一断言失败；budget_exceeded 含此类。
- 同一确定性断点两连败（同配置）→ 停手上交。

## 止损线

- 任一腿 exit 4 后：不原样重跑该腿；先读工件定位归属（agent 域=337/338/340 行为 / 前端域=335/336 / 内核域 / 脚本缺陷）。**门修复后腿 1 仍败且定位进前端消费域 → 停卡取证上交（卡面停止条件；前端改动需主管另卡）**。脚本自身缺陷→修复后该腿重跑 1 次（记 attempt，仍败即停）。
- LLM 实耗>10 调用（腿 1）→ 停止后续 LLM 轮，已耗轮如实记录。
- 栈异常（端口互染/进程僵死）→ 按环境中断记录，TEARDOWN 净空后从头登记再续。

## 隔离与防污染（§10，照 339 装置）

- 工程 fixture：由 %APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit（311 布位件，只读源）复制到各腿 run 目录 fixtures/（腿 1=小星星乐局.vit+小星星乐局·副本.vit；腿 2=磁带流·退路甲/乙.vit）。
- recent 列表（%APPDATA%\Vit-DAW\global_project_config.json）：每腿起栈前快照+prepend 本腿 fixture 路径；全部腿结束+栈拆净后还原快照（净差零）。
- boot 工程：GUI 腿共享隔离块 VIT_PROJECT_XML=git-clean run-dir 副本（339 同款；本卡不做 Phase B 剥 uuid——门已不读 uuid）。
- agent 草稿根：脚本既有 VIT_HISTORY_DRAFT_ROOT 隔离（%TEMP% per-invocation）。
- 真实用户工程/ sealed fixture 零触碰。

## 工件

- 每腿：`coord/runs/LAUNCHER-GATE-FIX-1/<leg>_<yyyyMMdd_HHmmss>/`（console 全文、driver_summary.json、telemetry 副本、godot log 副本、内核 log 副本、recents 快照前后、回执原文）。
- 收口：`coord/runs/LAUNCHER-GATE-FIX-1/report.json`（两腿对账）+卡面回执。全部随 port/launcher-gate-fix-1 推远端。
