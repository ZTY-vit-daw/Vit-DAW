# FE-ENVELOPE-LANE-TOGGLE-1（池序 198）真栈腿回执

- owner：ZCode GLM-5.3 PC 执行会话（ZCode，198 卡会话）
- run：`coord/runs/FE-ENVELOPE-LANE-TOGGLE-1/envelope_lane_toggle_20261027`
- 日期：2026-10-27 12:55 登记–12:57 释放（登记提交 899f5de6）
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-lanetoggle1` @ `port/fe-envelope-lane-toggle-1@09503b2`（轨道头三态开关 MenuButton「自动/开/关」+默认档回归 auto 交付面）
- kernel：stack worktree `D:/Vit_DAW_wt_lanetoggle1_stack` 自建 `VitApp/build/VitApp_artefacts/Release/VitApp.exe`（sha256 前缀 **246054A1**；源=origin/main=46400263 领取基线，`-DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine` 主检出只读；本卡零内核改动——CMake configure+build exit 0）pid 25780 本腿 ps1 自启
- agent：stack worktree 构建 `agent/bin/VitAgent.exe`（Go 零改动，46400263 源）pid 36184
- VIT_PROJECT_XML：run 目录 git 干净版隔离副本 `project_iso.xml`（基线 **947E4D00**，源=stack worktree `VitApp/Workspace/default_project.xml` 同哈希亲核）显式透传

## 一、判定面（全绿收口）

1. **ps1 最小腿 exit 0**：`dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_lanetoggle1_stack -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <自建 kernel>`（console=`ps1_console.log`；无 already listening——kernel/agent 均本腿 ps1 自启）。
2. **stack_probe_envelope_lane_toggle 连续两轮全绿**：R1 exit 0 **25/25**（`probe_r1_console.log`）+ R2 exit 0 **25/25**（`probe_r2_console.log`），零断言失败零修正轮（无 194/195 式 R1 探针缺陷轮）。180 横幅双源在场（console 横幅行+SUMMARY_JSON `stack_warning`）。

腿明细（R1/R2 同形全过）：

- setup：clear_project→add_track→plugin_search **真发现**（ValhallaSupermassive，非回退）→rack_add_node 装载外部 VST→描述符预热+目标参数（dry level 锚点）；
- **S1 默认档（198 核心面）：真行装配后 `get_envelope_lane_mode()=="auto"` 零显式 set 调用**（2025-10-25 用户裁定）；轨道头三态控件在真实装配树在场（MenuButton，文本「自动」+radio 勾选 auto 项）；
- S2 默认 auto 扫描：目录注入（192 同款探针侧注入面）→`field_seen=true`（190 字段到达前端消费链）→全零=车道保持收起+内核对照一致；
- L2 set_points 三平坦点→bus 全量回执→**车道自动展开**（visible+hit 词表+override=0 纯自动）+内核对照目标行 `envelope_point_count==3`+**控件选中态仍=自动（可见性≠档位）**；
- L3 **手动切「关」（菜单 handler 面→行 API→车道）**：mode=off+车道收起+控件「关」；**粘性实证：内核三点仍在+车道保持收起**；
- L4 clear_region 全域→内核全零+车道保持收起（off 档零自动行为）；
- cleanup clear_project。

## 二、三纪律（§2.2 增补①②③）

- a) 登记 899f5de6 推送 12:55:42 →**12:55:52 即时复核**（10s，<60s）：进程/端口全零后启动；
- b) 无 already listening（kernel 25780/agent 36184 均本腿 ps1 自启，命令行路径亲核）；
- c) 登记先于启动；与 200 错峰互查兑现（其 12:50 释放记录在案+登记前 fetch 重读+端口/进程全零亲核；另 4 个非栈 Godot 探针进程留档未动）。

## 三、隔离与零污染亲证

- 隔离副本 `project_iso.xml`：947E4D00（基线）→**CFBFEECF**（运行后）=内核只写隔离副本；
- stack worktree 运行前后 git 干净；
- 主检出 `D:/Vit_DAW/VitApp/Workspace/default_project.xml` 运行前后 **13A155AA** 不变（该值=会话开局既有 M，200 回执同注——非本腿触碰）；
- 拆栈：owner 手拆自录 pid 25780/36184 → 12:57:01 复核 VitApp/VitAgent/vsphub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。

## 四、-s 探针面（stack 腿外，headless 无栈）

- 新探针 `tools/probe_envelope_lane_toggle.gd`：**40/40 PASS exit 0**（M 装配面 6/D 默认档 4/T 三选联动 10/S 双向同步 8/R 回退 3+段守卫 5+setup 4——M6 槽位断言含 shrink 不挤占面）；
- 既有探针零回退（六件全绿，计数与卡面一致）：lane_auto **41/41**、lane_ui **39/39**、closeup **79/79**、curve **55/55**、badge **33/33**、datachan **50/50**。

## 五、执行侧披露（交主管裁定）

1. **lane_auto 探针 A1/A2 期望值更新（越出卡面「探针新文件」字面域）**：卡目标 2（默认档=auto）与 192 探针 A1「default mode off」硬断言**结构性互斥**（同一行装配面读 `get_envelope_lane_mode()`），不可能两全且 A1/A2 不在本卡文件域。按卡面引用的 2025-10-25 用户裁定（默认档回归 auto=189/192 两段偏离收回）将 A1/A2 期望由 off 改 auto，断言面不变（仍 41 检、默认态+关闭面+拒收面三断言原样，探针头注释同步注记）。**请主管裁定：认可此期望值更新 / 改由 192 域修正卡处理。**
2. **越域最小实现披露**：`bind_virtual_track` 补包络车道 kernel id 跟随（约 10 行，主 rebind 相位+folder 分支）——勘察发现虚拟化行池重绑只重绑波形 lane，包络车道不重绑=默认 auto 下换轨后在场性扫描**对准旧轨**（生产 GUI 缺陷面，非补则默认 auto 在真实 GUI 扫错轨）。同文件域内（Track_Row_Controller.gd），最小面修复。
3. **进程误杀事故（流程违规自报）**：12:35:30 本会话拆「自以为全是本会话泄漏」的 3 个 Godot 探针进程（6536/16084=本会话 lane_toggle 两轮✓自有），其中 **12836（12:31:51 起）经命令行核对非本会话进程**（本会话该时段无 lane_auto/lane_ui 调用；形态=并行执行流 -s 回归批，200 会话 12:39 登记注记其在飞）。-s 探针只读无栈端口零持久损害，对方流程 12:35:52 起自行重跑恢复。教训：**拆进程必须逐 pid 命令行亲核后再动手，不得按名字/时间窗批量拆**。
4. **默认 auto 性能注记（卡面停止条件②未触发，留观察面）**：auto 档下 bus 全量回执触发在场性重扫=192 既有设计；默认 auto 使其常态化（每条 auto 轨×目录内插件数一条 get_plugin_parameters 请求，异步 IPC 不阻塞帧）。本腿单轨+单插件形态无卡顿/超帧；多轨多插件工程的真实扫描压力未在本卡验收面内，留手测节观察（停止条件②如触发另开卡取证）。

## 六、端测边界声明

- 本腿覆盖：装配面（真实行装配树控件在场+选中态）、档位语义（默认/三选/双向同步/off 回退）、生产传输链（UDP 4445→agent→内核 ZMQ 全链：装载→写点→内核读回对照）；
- **未覆盖（留手测面）**：渲染像素/字体观感、下拉弹出的真实鼠标点击旅程（headless 经菜单 handler 面驱动）、多轨 GUI 布局挤占的真机观感、控件手感。

## 七、§8 记账

LLM=0（全确定性：ps1+探针）；运行轮次=ps1×1+探针×2（零重跑零环境中断零原样重试）；失败分类=无。
