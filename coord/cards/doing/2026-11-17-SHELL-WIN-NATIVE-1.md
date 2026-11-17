# SHELL-WIN-NATIVE-1：391-A 原生窗口焦点与层级

- 池序394；父卡391 SHELL-WIN-BEHAVIOR-1；P1；归属PC。
- 发卡/验收负责人：Codex PC主管 `/root`，2026-11-17。
- 派发确认：已确认，用户本聊天批准今日执行方案并指令开工；依据 `coord/decisions/2026-11-17-autonomous-subagent-execution.md`。
- 目标：VitShell打开Godot DAW时保持对话窗在前、DAW在后（不永久置顶）；仅装载窗口期维持焦点，完成后用户点谁谁在前。提供可经既有status面观察的真实状态以供395消费，禁止虚构焦点/进度。
- 文件域：前端仓 `VitShell/godot_manager.*`、`launcher_bridge.*`、`main.cpp`，最多5实现文件；若须status_server或Godot场景扩域先报告。允许同目录必要单测；不动webui、不动app/shell，后者归396。
- 并行：388主仓UI域不相交。与395的协议接缝先报主管确认；不得擅自启动真栈。
- 验收：原生构建exit0+有意义的窗口状态/生命周期验证；真实三件套按现有vitshell_full_smoke体系扩展，断言焦点与前后层级+装载完成后解除保持，exit0。代码自验不能替代真栈。脚本改动若跨到主仓先请求主管分卡/扩域。无computer-use、键鼠模拟或桌面操作，测试采用应用status+只读窗口查询。
- 停止：前提失效/域外缺口/缺少可靠加载终态→附锚点上报；同确定性断点连败2次停止原样重跑；真实栈未经主管放行不得启动。
- 领取：owner=`/root/window391`（GPT-6.1 Sol/xhigh，PC）；前端基线`d05043e11a79452b9155a89f3405668994eafdcb`，分支`codex/shell-win-native-394`；worktree=`C:/Users/timoz/.codex/worktrees/shell-window-391/vit-daw-frontend`，领取时tracked status为空。主仓领取基线`116f22155b1ba52c5c6e37076799791a3d1ee1a7`。正式开工以本卡领取提交成功同步共享main并核对远端owner为准。
- 回执/验收：待回填。

### 终态边界裁定（2026-11-17）
现launch-result ok来自start_page change_scene_to_packed受理，未证项目仓库加载完成；本卡原生阶段明确命名scene_accepted，以现有终态有限释放临时noactivate，不宣称workspace_ready。允许修复spawn后ResetEvent清掉快速回执的同域竞态。395只呈现实证步骤，不捏造ready；完整workspace_ready若需Godot装配+首次project_repository.apply应独立小卡。main只沿既有set_chain/detail暴露真实状态，不扩status_server生产文件。

### 观测面扩域（394-B，主管2026-11-17）
为保留旧chain_detail纯文本契约，B独立提交授权status_server.h/.cpp新增可选window_state schema vitshell.window_state.v1+main.cpp一处reader接线（3文件）；A原4文件独立可构建。reader在HTTP服务锁外调用，mutex快照元数据并即时只读查询foreground/z/style；不在HTTP线程改窗口。旧detail不改、无窗null/unknown、schema不声称ready。原生焦点临时位在scene_accepted/failed/timeout/exit/shutdown释放；runtime验证候主管放行。

## 零栈回执与主管复核（2026-11-17）

- 执行提交：A=b778f1c5908abca342de9d3e277f1915e24358db；B=e9006de7bdae4803c1a5baf1c288f0c992414bc4，前端分支codex/shell-win-native-394已推。
- 原生build exit0、独立测试目录ctest 2/2 exit0；主管审实际diff（临时noactivate/无topmost/终态释放/旧退出pid校验/HTTP fresh只读观测/旧detail保留），亲复跑build与正确测试目录2/2均exit0。首次误用build根ctest报告No tests were found，已改window-tests目录亲证，不将空运行算验证。
- 技术面待真栈；未合入前端main。单元/构建不证Godot显示时序与真实焦点；402/395集成后再终验。旧wrapper覆盖STARTUPINFO风险待真实运行。

### 394-C 回执关联修订（2026-11-17）

402证据指出root进程内幂等不能阻止旧Godot迟到HTTP回执终结新launch。394同卡C段（不另领重叠域卡）：LauncherBridge在spawn前设置VIT_SHELL_LAUNCH_ID为本次request_id nonce；OnLaunchResult锁内核对chain_in_flight+echo launch_id，缺/错/上轮id不唤醒并记录；必要GodotLaunch字段最多bridge.cpp+manager.h/cpp三文件。新native与402成对部署，旧无echo只能显式legacy模式接受，不能默认兼容绕验证。先402形成独立gd commit，再394-C独立native commit；均待组合真栈，不在此记录最终PASS。

### 394-D 真实阶段发布与只读重放（2026-11-17）

- 395只读取证确认WebUI无vit_window_state消费者，现saving/closing_daw/spawning/waiting_launch_result只存status，terminal stage也未传UI；父391需真实步骤反馈，批准同卡D段（0c712d94干净组合基线）仅bridge.h/cpp两个生产文件，独立commit。
- SetChainUi追加可选stage；发布schema=vitshell.launch_progress.v1的vit_launch_progress（request_id、单调seq、phase/detail、真实可选stage），仅既有实际阶段。opened stage来自nonce已校验result_body；未知stage不升ready。保持旧五type与chain_detail语义。
- ask_vit_shell/window_presentation_ready非空subscription_id握手：消费者先挂listener，native只读fresh WindowStateJson+最后progress replay，同seq不自增、不启动/切工程/操作窗口，无初态则unknown。
- 锁内只快照，dispatch/user hook锁外；无栈配对测试验证seq/转义/stage/晚订阅/错channel/缺ID/重放不变，真栈仍候388释放。

## 394-E 首次激活真实反例与修订准备（2026-11-17）

- 组合normal首腿内外exit1，依赖矩阵停止。真实native事件与139样本中5个加载违规：guardactive且同DAW HWND初现时foreground=daw、shell_above=false、noactivate=false，约63ms后style/z修正但仍DAW前台，约391ms才回shell。后续nonce/workspace_ready和guard释放不冲销该反例。
- 根因方向=OUTOFCONTEXT事件纠正晚、Godot wrapper/engine可覆盖STARTUPINFO；仍须方案与实测证据，不把猜想写已修根因。主管交原Sol/xhigh执行owner只读定位，并按用户“最疑难仅参谋”授权Astra只读征询（不实现、不终验）。
- 新独立生产修订worktree=C:/Users/timoz/.codex/worktrees/shell-first-focus-394/vit-daw-frontend，branch=codex/shell-first-focus-394，基线1301345、status/diff初空；owner=/root/window391。本段只领取诊断/方案，生产修改待主管方案裁定，保持≤5原生生产文件、不触395UI或402B已验装置域。
- 禁止全局输入/CBT DLL植入、嵌入合窗、永久topmost、周期强抢焦点；用户切外部应用须尊重，终态/失败/超时/退出清理。不能降低原noactivate断言。真实栈当前entry388已拆净候释放，不授本owner启栈。

### E阶段review必要接缝（2026-11-17）

主管review指出GD fire-and-forget bootstrap与terminal两个HTTP可乱序，fixed202不证明UI prepared；批准E同一安全提交6生产文件（原5+start_page.gd仅await begin）。begin等bootstrap POST完成后只读status核同directPID/hidden_boot_state prepared/guardactive，失败停止后续create/open；native早ready拒绝仍保留，不能把提交顺序当barrier。此为明确必要单行跨函数接缝例外，避免不安全中间提交。

显示顺序先在hidden时styleNOACTIVATE+z behind+workarea，再SetWindowPos SHOWWINDOW|NOACTIVATE同次指定z显现，禁止先show后退。源码/Win32文档仍不替真栈，隐藏后currentmode不要求1（WM_WINDOWPOSCHANGED会更新），验证feature初始effective mode1避免误拒。E-OBS另紧邻≤3原生文件保原WinEvent目标/时间/guard区间，先记事实再策略动作，不将dedup快照冒称原始事件。
