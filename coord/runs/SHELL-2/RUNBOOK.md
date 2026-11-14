# SHELL-2 真栈全旅程烟测 RUNBOOK（跑前冻结，§8 口径）

- 冻结时点：2026-11-15 02:2x +0800（占用登记 a98e0d2a 之后、首跑之前）
- 卡：SHELL-2（启动态桥与工程拉起）；验收=脚本 exit 0

## 运行次数与成功条件

- **计划 1 次主跑**（可选 ≤2 次重跑，仅限环境中断或修复后）；成功=脚本退出码 0 且全部断言腿绿。
- 零 LLM 计费面：旅程为纯链路（按钮→桥→直启），无 chat LLM 调用（打开链不进 LLM）。

## 腿与断言（冻结面）

- **L0 preflight**：Vit 系进程=0；7878/7879/8787 监听=0；占用记录重读=本卡 owner；recents 快照（%APPDATA%/Vit-DAW/global_project_config.json）+ fixture 源哈希留档；fixture 副本落 run 独立目录。
- **L1 build**：VitShell cmake exit 0；agent go build exit 0；webui npm run build exit 0；二进制哈希入 meta。
- **L2 启动**：VitShell 主窗（title=Vit）；phase=ready；surface=launcher；CDP：`[data-launcher-root]` 在场、`[data-launcher-new]`+`[data-launcher-choose-file]` 在场（抽屉按钮腿）；no-reload 哨兵 `window.__vit_no_reload` 置位。
- **L3 首开（零摩擦+直启）**：CDP 真实点击 `[data-launcher-choose-file]` → choose_file 旁路（fixture A）→ /agent/state 空 → 无治理卡（chain_state 不含 waiting_governance）→ Godot 直启（godot_running=true）→ launch-result ok → status chain_state=opened、launch_count=1 → React `vit_launcher_result` opened（CDP bridge-exec 块）→ vit_surface project → CDP：`[data-launcher-root]` 消失（project surface）、no-reload 哨兵完好、page.url 不变。
- **L4 直启质量**：/agent/state current_project.path=fixture A（version.project_opened 到达=340 语义）；8787 监听在卷（hub）；VitApp 进程在卷（kernel，Godot 自启）；Godot 用户日志（%APPDATA%/Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log）含 `[VitShellLaunch] direct.begin`+`launch_result.post`+`direct.enter_ok` 且 scene=res://vit_dock/scenes/VitDockRoot.tscn；**343 休眠断言**：日志无 `mode.*autoload window` 痕迹（AgentChatWindow 休眠，VIT_AGENT_WINDOW_MODE 未设）；无 start_page 渲染=直启分支 structural（visible=false 先于首帧，日志 direct.begin 为分支证据——渲染面视觉断言留手测 v6/SHELL-3，边界入回执）。
- **L5 关 DAW 回主窗**：POST /vitshell/close-daw → Godot 退出（godot_running=false）→ VitShell 发 vit_surface launcher → CDP：`[data-launcher-root]` 回归、no-reload 哨兵完好、page.url 不变（同窗不重载腿）；kernel/hub 随 Godot 退出（VitIpcClient 清理，355 语义）。
- **L6 治理卡切换**：CDP 再点击 `[data-launcher-choose-file]`（fixture B）→ /agent/state 非空（stale A）→ vit_governance_required → CDP：`[data-launcher-governance]` 三键卡在场（保存并切换/直接切换/取消）→ URL `launcher_smoke_governance=switch` 自动经真实桥路径回投 → Godot #2 直启（fixture B）→ launch-result ok → launch_count=2 → /agent/state=fixture B → CDP：治理卡 answered 呈报+project surface。
- **L7 关主窗全退**：POST /close → VitShell 退出 → agent 树+Godot #2 树（含 kernel/hub）全灭（进程=0 亲证）→ 端口 7878/7879/8787 释放 → recents 快照还原（哈希一致或记录差异）→ fixture 副本核对后清理。

## 失败分类与止损

- 环境中断（网络/进程被外部杀/端口被占）→ 记录证据后可重跑（≤2 次），不计功能失败。
- 断言失败 → 停：取证（meta.json+bridge 日志+Godot 日志+status 快照+CDP 截屏文本）入 run 目录，**不原样重跑**；按 AGENTS §8 定位（执行偏差/卡缺陷/假设失效）后处置。
- 直启 boot 冲突（Godot 侧异常退出/launch-result 超时）→ 取证上交（停止条件候选）。
- 两次同断点失败 → 止损，留断点交回。

## 工件

- run 根：`coord/runs/SHELL-2/run_<stamp>/`：meta.json、bridge.log（VitShell [LauncherBridge] 全量）、godot_user_log.txt（拷贝）、status 快照各腿、agent last-log 尾、CDP 断言输出（cdp_asserts.json）、recents 前后哈希、fixture 哈希。
