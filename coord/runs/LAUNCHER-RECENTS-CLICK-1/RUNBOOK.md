# RUNBOOK：383 LAUNCHER-RECENTS-CLICK-1 真栈腿（ui_bridge_smoke.ps1 -RecentsClickLeg）

## 跑前口径（AGENTS §8，冻结于任何真栈运行之前）

- 运行次数：**正式 1 次**（单 run 无重跑）；装置域修复后重跑须分记 attempt。
- 命令：
  `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/ui_bridge_smoke.ps1 -RecentsClickLeg -SkipBuild -AgentBinary <run>/bin/VitAgent.rc383.exe -RepoRoot D:/Vit_DAW_wt_383click -GodotProjectRoot D:/Godot/vit-daw-frontend_wt_383click -KernelExe <现行内核> -RunRoot <run>/`
  （-SkipBuild 前提=被测二进制含本卡改动：agent 现场从 worktree 构建（脚本默认先建再跑，不用 -SkipBuild 的默认路径）；webui=worktree `npm run build` dist）
- 成功条件：**exit 0** 且 report 三腿全过——
  - RC1（点击→打开→对话往返）：smoke.navigate 带 `launcher_smoke_followup=recents_click` → directive.received（磁带卡点击经桥）→ 治理 dialog+switch → open.begin → result=opened + result_ack=opened → scene_added VitDockRoot 同 HWND → /agent/state=烟测桥甲 → 同会话 HTTP 两轮（list 轮 + 桥接播报轮 stop_reason=launcher_bridge_broadcast、context_flow=project、reply 含「已进入《烟测桥甲」、typed_events[0]=volume_switch）；
  - RC2（文件移走）：surface_ready 后 8s 删除烟测桥乙.vit → recents[1] 点击（+25s）→ 治理 switch → result=failed（file_path=乙）+ result_ack=failed + 无 scene_added；
  - RC3（损坏文件）：recents[2]=烟测桥坏.vit（存在但非工程内容）点击 → 治理 switch → result=failed + ack + 无 scene_added。
- **LLM 预算：0 调用**（RC 全腿确定性：launcher list 轮/桥接播报 checkpoint 均先于 LLM；telemetry 落 run 文件断言行数 0；超预算=失败类 budget_exceeded）。
- 失败分类：`env_interrupt`（栈起不来/端口占用/Godot exe 缺失/进程崩——保留原始 console+日志，不算有效轮）/`assertion_failure`（产品断言红）/`apparatus_defect`（装置缺陷，如烟测接缝未触发——修复后该腿重跑 1 次记 attempt，仍败上交）。环境中断只有原始日志在卷才可排除有效轮次。
- 止损线：60 分钟或环境中断两次→释放栈位并按类记录；同断点同配置连败 2 次禁原样重跑（先取证/缩小假设/上交）。
- 已预声明主风险：
  ①RC2 时序窗（列表装载→8s 删除→25s 点击）：若列表装载慢于 8s，agent exists=false 置灰→烟测接缝按 disabled 语义跳过→directive.received 超时=apparatus_defect（如实分记，不改判成功）；
  ②boot scratch 治理判据（L1 先例：boot 态出三键 dialog）——本腿按既有产品语义以 switch 驱动，不判零摩擦成败；
  ③桥接播报轮前置=version.project_opened 会话事实——scene_added 后 settle 5s+绑定轮询双保险；若未触发回落 LLM 计 1 调用且断言红（budget_exceeded 类如实记）。

## 被测物 provenance（AGENTS §9）

- 内核：脚本候选序取 D:\Vit_DAW 主仓构建/部署件（本卡零内核改动；sha256 记入登记行）。
- agent：worktree D:/Vit_DAW_wt_383click 现场构建（含本卡 exists 字段改动），二进制哈希入 report.isolation。
- webui：worktree dist（tsc 0 错+vite build）。
- Godot 前端：隔离 worktree D:/Godot/vit-daw-frontend_wt_383click @ d05043e（=frontend main HEAD；Godot 侧零改动——烟测接缝经既有 VIT_SMOKE_LAUNCHER_FOLLOWUP env 透传）。
- 工件：coord/runs/LAUNCHER-RECENTS-CLICK-1/run_<ts>/（本 worktree/port 分支入库）。

## 隔离与防污染（AGENTS §10）

- 工程 fixture：912.vit 只读源复制 3 份（甲/乙同内容异名=340 先例；坏=非工程文本）。源件哈希前后核对。
- recents（%APPDATA%\Vit-DAW\global_project_config.json）：脚本起栈前快照（before.json 入 run 工件）→ prepend 本腿 fixture → finally 还原；**owner 亲核还原哈希一致**（run 后复核 before/after 哈希）。
- 内核 workspace 隔离（run 目录 kernel_workspace，VIT_PROJECT_XML 指 run 副本）。
- 端口：7878/5555/5556/8787 起栈前净空核对（§2.2 复用禁令：任一监听即停）。

## 步骤

1. 占栈登记推 main 亲核 → ≤60s 复核净空；
2. 跑命令（见上）→ exit 0；
3. teardown 复核（进程/端口全零）→ recents 还原哈希亲证 → 释放行推 main。

## attempt1 记录（2026-11-16 23:13-23:17，run_20261116_2313）

- **产品链全绿**：RC1 磁带卡点击经桥（smoke.navigate 带 recents_click followup 亲证）→ directive.received → 治理 dialog+switch → open.begin → result=opened + result_ack=opened → scene_added VitDockRoot 同 HWND → /agent/state=烟测桥甲——**点击→打开链 attempt1 即证成**。
- **失败=apparatus_defect**：脚本侧「工程内对话往返」轮1 在进入工程后发——launcher gate 关（userEnteredProject 置位，launcher_flow.go:72-80 gate=context.launcher && !userEnteredProject）→「有哪些工程」落 LLM **7 调用（deepseek-flash，telemetry 7 行=预算 0 超支，budget_exceeded 如实记）**→ 断言红 exit 1。RC2/RC3 未跑。
- **修复**（换假设缩小，非原样重跑，§8 允许项）：轮1 移到 Godot boot 前（gate 开=确定性 list 零 LLM，本会话注册 launcher 起源）；轮2 进入工程后发（桥接播报 checkpoint 前置于 LLM——339 leg1 同款已证形态）；内核引导窗口确定性降级（连不上内核）限时重试 3×3s，其余回执即停不盲重试。
- attempt2=RC 全三腿重跑 1 次（新 run 目录不覆盖）；预算仍 0（attempt2 若再触 LLM 即停腿上交）。
