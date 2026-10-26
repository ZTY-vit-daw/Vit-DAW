# M3-ENVELOPE-CLOSEUP-CURVE-1(195) 真栈腿回执——envelope_curve_20261026_2310

- owner=ZCode GLM-5.3 PC 执行会话（ZCode，195 卡会话）/ 2026-10-26 晚窗
- 被测=前端 worktree D:/Godot/project/vit-daw-frontend-envcurve1 @ port/m3-envelope-closeup-curve-1（曲率编辑交付面：envelope_closeup_canvas 曲率句柄 D3A+曲线采样渲染+c 透传写回+预设四档+吸附精细面；含本 run 期间探针修正提交后的工作树态）
- kernel=**192 自建二进制只读复用** `D:/Vit_DAW_wt_envauto1/VitApp/build/VitApp_artefacts/Release/VitApp.exe` pid 14188（本卡零内核改动亲核：`git log e70bafc7..origin/main -- VitApp/` 为空；sha256 5D31056A 系与 192/194 回执同源）——复用依据=189/194 先例「零内核改动只读复用」
- agent=stack worktree D:/Vit_DAW_wt_envcurve1_stack（origin/main=e610c2b1）构建 VitAgent.exe pid 33984（路径亲核）
- ps1=最小腿 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <192 二进制> -GodotProjectRoot <envcurve1> -RepoRoot <stack worktree>` **exit 0**（ps1_min_leg.log；无 already listening）
- VIT_PROJECT_XML=run 目录 git 干净版隔离副本（947E4D00 基线，源=stack worktree）显式透传

## 探针结果（stack_probe_envelope_curve.tscn，场景模式生产 GUI 传输链 UDP 4445→agent→内核 ZMQ）

- **R1 23:08**：exit 1（24 断言 2 败）——**探针期望缺陷非生产缺陷**：C4 参考坐标把段 1 中点 t 误写 0.75（实为 2.25）；C2 期望值经 GDScript lambda 写外部变量=值捕获无效恒 0。生产面 C1（外部写带 c 读回一致）/C3（预设四档内核读回）首轮即全绿。修正=探针两处（零生产改动）。
- **R2 23:10**：**exit 0，24/24 PASS**
- **R3 23:10**：**exit 0，24/24 PASS**（连续两轮收口）

## 断言面（卡面验收③：写点带 c→内核 get_curve 读回 c 值一致→预设切换→读回）

- setup：clear_project→add_track→plugin_search ValhallaSupermassive（真发现非回退）→rack_add_node 装载外部 VST→描述符预热→目标参数 dry level（0..1）
- C1 开视窗+外部写带非零 c 三点（c=0.6/-0.25）→**canvas display 逐点（t,v,c）=写点+内核 get_curve 对照一致（内核回执携带 c=读回校验面存在实证，停止条件①不触发）**
- C4 句柄位=段中点 t=2.25 处曲线采样值（真描述符值域下几何锚定，与独立探针 H1 同面）
- C2 视窗曲率句柄拖动（生产局部手势机 press/motion/release）→释放恰一笔 set_points region→回显门→**display c=-0.630（拖拽映射值）+内核读回逐点一致**
- C3 预设链四档（apply_curve_preset 生产面，菜单同面）→**fast_in c=-1 / fast_out c=+1 / smooth_s c=+0.5（符号沿用 c>=0 侧）/ linear c=0——每档 display 达标（回显门）+内核读回逐档逐点一致**
- cleanup clear_project ok

## 三纪律（§2.2 增补条款全执行）

- a) 登记 c0be628a 推送后 **23:06:40 即时复核**（距登记 <60s）进程/端口全零（VitApp/VitAgent/VspHub=0；TCP 5555/5556/7878/8787/18787 listen=0；UDP 4444/4445=0）
- b) 无 already listening（kernel 14188=本腿 ps1 自启，ps1 日志 grep=0）
- c) 登记先于启动（23:06 登记→23:06:4x ps1）

## VIT_PROJECT_XML 隔离亲证

- 隔离副本被写：sha 947E4D00→9BB0AD2A（末次写 23:10:23=R3 探针 cleanup 前内核自动保存）=隔离生效
- stack worktree `VitApp/Workspace/default_project.xml` 运行前后同哈希 947E4D00+git 干净=零触碰
- 主检出 default_project.xml=1FEF9CE9（=189 回执基线，本腿零触碰）

## 拆栈

- owner 手拆 VitAgent 33984/VitApp 14188（路径亲核归属本腿）→ 23:10:49 复核：VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0
- 注记：另清理两个本会话探针孤儿 Godot 进程（33164/12836，早前新 worktree 无 .godot 导入缓存时卡死泄漏，命令行亲核=本会话 lane_auto 探针，非他人在飞栈、不占栈端口）→终核 godot=0

## 工件清单

- ps1_min_leg.log（ps1 最小腿 console，exit 0）
- probe_r1.log（探针期望缺陷轮留证）/ probe_r2.log / probe_r3.log（连续两轮 exit 0，SUMMARY_JSON 各含 180 横幅 stack_warning）
- default_project_isolated.xml（隔离副本，运行后被写=生效证据）
