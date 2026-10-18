# FE-GUI-C-FLOATING：前端收缩 Phase C（浮卡/真窗口双档）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据同 91）
- 派发确认：已确认（Phase A 裁定随卡）
- 验收负责人：GLM 主管决策流
- 池序 92；目标仓库=前端仓
- **晚窗参考序①（2026-10-15 早窗 /morning 校准：今日主推=M2 体验场次（用户侧），92 早窗不派发；晚窗若派发按池序自取，92 为 PC 可执行首位——基线 1c61482 已就位，转交提示词待出）**
- 优先级 / 预估 / 依赖：P2 / 预估依规格 / **FE-GUI-B 合入后领取**
- 模型分级：L1 / PC 执行流

## 目标（Phase A 回执划入 C 面的断言⑤后两档）

浮卡与真窗口双档重塑（window_manager 现役三活调用面保留为锚，Phase A 已核）；含走带/底栏抽屉终态收口。

## 文件域与红线

同 91（红线 9 文件零触碰+headless probe 端侧门）。

## 验收标准

规格断言⑤终态逐条钉死+全回归+真栈三件套 exit 0+手测清单[等待用户]。

- 领取（2026-10-18 午窗）：2026-10-18 11:52 +0800 / origin/main=b61a773e77abd3331b913e5f29d1649878049cb9 / owner=**DSH PC 执行流 exec-fe-gui-c**（本机 DSH 团队会话；12:00-18:00 决策流按用户 2026-10-18 午窗指令派发）/ 前端仓分支 **port/fe-gui-shrink-c-floating**（自 port/fe-gui-shrink-b-1@1c61482 建；基线与 2026-10-14-BATCH1 裁定「Phase C(92) 解锁，基线=port/fe-gui-shrink-b-1@1c61482」一致）/ 前端仓 worktree **D:/Godot/project/vit-daw-frontend-fe-gui-c** / 主仓探针分支 **port/fe-gui-shrink-c-probe-1**（自 origin/main b61a773e 建）+ worktree **D:/Vit_DAW_wt_feguic_probe**（新增 scripts/run_fe_gui_shrink_phase_c_probe.ps1+工件；验收通过后由决策侧 cherry-pick，同 91 先例）/ 领取提交=（本提交）。领取核对：卡在 todo+派发已确认（Phase A 裁定随卡）；91 已 pass（2026-10-14-BATCH1）且 Phase B 基线 1c61482 在卷；doing 池 LOUDNESS-B 修订腿文件域（主仓 scripts/run_free_state_d1_smoke.ps1+fs_largeproject_smoke.ps1+coord/runs/LOUDNESS-B-TRIGGER-1/）与本卡前端仓域+主仓**新增**探针脚本互斥（不触既有主仓脚本）；真栈腿串行=同一 PC 栈同一时刻仅一 owner，本卡真栈三件套腿排在 LOUDNESS-B 修订腿释放后（§2.2，决策侧确认窗口）。协作模式（DSH 团队）：协调面（卡片/资源记录/验收）由 12-18 决策流单写，实现面由执行流在独立 worktree 写。**真栈窗口：2026-10-18 12:26 由 Lead 登记 PC-RUNTIME-STACK 当前占用（owner=exec-fe-gui-c，计划 12:30–15:30）并发 GO**；执行口径=RepoRoot 用探针 worktree（写隔离）+`-KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe`（C++ 零改动，记 SHA256）+agent 由工作树 Go 源构建（Go 零改动，记 SHA256）+`-GodotProjectRoot` 指本卡前端 worktree；跑完手动拆栈复核四端口/四类进程=0。
- 回执（2026-10-18 午窗，exec-fe-gui-c 执行，Lead 转写）：前端仓 **port/fe-gui-shrink-c-floating@4b8bb9c**（已推，13 files +1728/−9）——浮卡/真窗口双档：抽屉 chip 点击/F2 台后卡「⊞ 浮窗」→ Dock 内浮卡（可拖拽、实时视图实例 `vp_float_*`）；浮卡 ⚠/↗→真 Godot Window 节点（单视图内容重挂、独立窗口策略、不继承旧 detached 脚本）；⇤ 吸回/系统关闭/✕ 关闭语义完整；同视图重复打开=聚焦既有项；window_manager 三活调用面（轨头钢琴卷帘/track·audio detail/master output）零改道保留为锚；底部状态栏（vit_dock 新 widget：台前 N 位·视图名/快捷键提示/agent 外置窗注记，动态刷新）按决策流裁定落地。主仓探针分支 **port/fe-gui-shrink-c-probe-1@1143b790**（已推；新增 scripts/run_fe_gui_shrink_phase_c_probe.ps1 + 工件 coord/runs/FE-GUI-C-FLOATING/）。headless 电池：Phase C **129 钉 PASS**+回归 Phase B 119/Phase A 69/vitnote 六/FE-RACK 76 全 exit 0；红线 git 门 924318d..worktree 9 文件零命中；工件 20261018_120623/。真栈三件套腿：`dev_agent_smoke.ps1 -RepoRoot <探针 worktree> -KernelExe <主检出 staging> -StartKernel -StartUI -MixSmoke -GodotProjectRoot <本卡前端 worktree>` run=**dev_agent_smoke_20261018_123056 SMOKE_EXIT=0**，mix.observe **status=ok obs_20261018T043129_0819166048df**；被测 kernel SHA256 B6565DCF…（C++ 零改动）/agent SHA256 136137E5…（Go 零改动）；LLM 6 次 deepseek-flash 0 error（MixSmoke 既有腿，如实登记）；`-StartUI` 因内核先绑 5555 按既有逻辑跳过（91 同款），补窗口态启动证据（D3D12 真渲染/start_page VSP Hub+Agent tools ready/零 SCRIPT ERROR/退出无残留）；拆栈复核四端口 7878/5555/5556/8787/4445/19876=0+四类进程（含 Godot 变体名）=0。端测边界（AGENTS §5）：headless=装配层端侧门，真实系统窗口外观/位置/焦点/多显示器/指针手感/动效/视觉观感未覆盖→归用户手测；偏差 4 项（走带栏两按钮位未做=91 偏离①沿用/F2「⊞ 浮窗」只挂台后卡/⚠↗ 双入口同语义/底栏按裁定落地）均如实登记。手测清单 9 条 **[等待用户]**（入口=用户从 Godot 拉起 DAW 前端→前端启动页进工程）：①抽屉 chip 点击→浮卡+抽屉收起；②浮卡拖拽跟手；③⚠/↗ 弹独立系统窗口（标题/位置跟随）；④⇤ 吸回/关窗/✕ 语义；⑤F2 台后卡「⊞ 浮窗」→浮卡+面板收起，重复打开=聚焦；⑥多视图多开不遮挡错乱；⑦三活独立窗与 B 后一致；⑧底栏台前 2/3/4 位与空态实时正确+注记可读；⑨走带栏/电平表/AI 助手外置窗与 B 后一致+对话内 mixboard 状态卡正常。
- 验收：**pass（终验·2026-10-18 午窗）**——[rulings/2026-10-18-BATCH1-rulings.md](../../rulings/2026-10-18-BATCH1-rulings.md) §一；决策侧独立复验=远端 hash 亲核（前端 4b8bb9c…/探针 1143b790…）+红线门亲跑（924318d..4b8bb9c 33 文件、9 红线命中 0）+真栈工件亲读（smoke_exit=0/obs=obs_20261018T043129_0819166048df/teardown=0）+栈现场 12:36 亲核四端口/四类进程=0；4 项偏离采信；边界采信（headless 装配层覆盖/窗口态补充证据/cef dll 环境缺失）；主仓探针分支三笔 cherry-pick 入 main（2f871de6/070e8be2/20187f38）；前端仓 main 归一按 2026-10-14-BATCH1 挂账不自动触发；手测清单 9 条交用户排期。
- **手测返工（2026-10-18 午窗，用户实机手测 92 发现；执行侧自验+headless+真栈面未覆盖的呈现/交互面）→ 卡重开 doing**：
  1) **抽屉被台前视窗遮挡/显示不全**：视图抽屉拉下后被台前视图窗格挡住，只有台前无视图时才显示完整 → 层级/绘制顺序缺陷（drawer 当前为 WorkspaceShell 子节点；需核对其相对 DynamicSplitTreeRoot 内 stage frames 的 z/插入顺序/top_level/clip；修复=抽屉面板浮于全部台前窗格之上且完整可见）。
  2) **F2 台前调度列表贴顶/被遮**：视窗按钮弹出的布局列表贴屏幕最上缘、且被走带栏等既有 GUI 挡住 → 定位缺边距 + z 低（VitDockRoot.tscn 中 `TopTransportBar z_index=80`）；修复=顶层 overlay 定位（顶部让出 header+transport 实际高度+间距）并抬高层级（backdrop 层次同步，模态/保存对话框仍在其上）。
  3) **无 UI 动效**：drawer/F2 面板/浮卡/真窗口全部瞬时开关无过渡（代码核对：view_drawer_bar/stage_switch_panel/floating_view_window/floating_view_manager 0 tween）→ 按 V11 规格补过渡：抽屉滑落/收起、面板淡入淡出+位移（backdrop 同步）、浮卡出现/吸回/关闭、台前位推上/拉下过渡；120–220ms、ease、可中断/幂等、headless 兼容。
  - 返工范围=仅前端仓 Phase C 线（vit_dock/+tools/ 探针），红线 9 文件零触碰；不触主仓既有脚本；不 push main。探针补钉：drawer/frame 层级、panel z>80+顶部边距>chrome 高、动效策略常量/存在性；原 129 钉不得回退；回归电池全 exit 0。
  - **验收=返工提交+探针电池 exit 0+用户按 9 条清单复测通过**（入口=用户从 Godot 拉起 C worktree 前端→启动页进工程）；真栈腿按用户手测优先，不另占栈。原终验 pass 仅覆盖 headless+真栈面，本次手测返工为终验补充。
  - 执行流=exec-fe-gui-c（task-8），walk=同分支连续 commit。
