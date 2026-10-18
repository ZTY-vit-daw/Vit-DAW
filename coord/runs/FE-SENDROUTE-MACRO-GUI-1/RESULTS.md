# FE-SENDROUTE-MACRO-GUI-1（K3/130）真栈烟测结果

- 执行：GLM-5.3 L2 PC ZCode 执行会话（K3 本卡会话）；占用登记 cd23b1c0（21:5x 预检六端口/四类进程=0 后登记，GO=用户任务领取口令+卡面验收）；释放登记见 PC-RUNTIME-STACK 本次释放条目。
- 基线：主仓 origin/main=cd23b1c0（含 K1 f4926543+K2 d4ea37ae）；前端仓 port/fe-sendroute-macro-gui-1@8cc6787（worktree D:/Godot/project-vit-daw-frontend-sendmacro）。
- 被测 kernel=本腿自建 `D:/Vit_DAW_wt_fesendmacro_stack/VitApp/build-feguik3/VitApp_artefacts/Release/VitApp.exe`（CMake VS 18 2026 Release，2026-10-18 22:23 构建，**SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC**；tracktion_engine 子模块本地引用克隆 b439749=origin/main 记录 SHA）+ 腿内 go build 自建 agent（main 基线）。stock staging B6565DCF 不含 K1，不可用作被测（K2 run1 同证）。
- §8 记账：LLM=0 次调用（chat/strip-silence 腿关，全确定性命令面）。

## Run 1：`-StartKernel -RestartAgent -SendRouteGuiSmoke`（run=sendroute_gui_20261018_2250）

- **脚本 exit 0（"dev agent smoke completed"）**；真栈探针 **25/25 PASS**（console 工件 `sendroute_gui_smoke_console.txt`，SUMMARY_JSON status=pass）。
- 断言面（GUI 同源命令面经生产传输链 VitIpcClient UDP 4445→agent bridge→内核 ZMQ；payload 全部由前端 rack_ipc_payload_rules 构造）：
  - L1 connect auto_create（选择器「＋新建 FX 轨」同构 payload）：created_fx_track+fx_name 前缀+缺省量 -6.0；
  - L2 track.send.list 读回一致（source/fx/amount）；
  - L3 get_project_state 源轨 plugins 含 type=auxsend+auxreturn（SendOut/SendIn 渲染数据源）；
  - L4 set_amount（SEND 槽位同构 payload）-12.5 回执+list 双回读；
  - L5 宏三步：control_add_macro（macropanel 快照）→control_add_binding（target_param_id="send level"+SendOut 实例——领取决策点①硬接线口的真栈验证）→control_set_macro_values（applied_bindings=1+control_graph 快照 current_values.macro_1=0.8）→**send amount dB 联动**（实测 +1.5371=fader 换算 20·ln(0.8)+6，tracktion_AudioUtilities volScaleFactor=20，公式亲核）；
  - L6 disconnect：fx_released=true+list sends/dangling 归零。
- 迭代记录（§8 如实）：首两轮失败均为探针自身缺陷非产品断言——①`KernelCommandClient.send` 双参误用单参（SCRIPT ERROR 即修）；②dB 联动期望式假设 gainToDecibels 错误（实测 1.537 反推公式亲核后修）；③上一腿 auto_create 残留同名轨（K1 run2 同课）→FX 名带时间戳+按本腿 node_id 计数。修后手跑 25/25 → 正式腿一次过。另 ps1 腿首版 PS5.1 原生 stderr 重定向把 GDExtension 缺库噪音外抛且工件空→改 cmd /c 原生重定向。
- 中间调试轮（sendroute_gui_20261018_2230 目录，console 空=首版腿重定向缺陷工件）：手跑探针对活栈三轮，已计入上述迭代记录。

## Run 2：`-StartUI -MixSmoke`（run=window_boot_20261018_2258）

- **脚本 exit 0**（mix.observe status=ok obs_20261018T142915_c3e11297c20f + dev agent smoke completed）。
- 窗口态启动证据（92 卡同款——内核先绑 5555 时 -StartUI 按既有逻辑跳过自启，手动拉起补证）：Godot 窗口 pid 16468（D:/Godot/Godot_v4.6.1-stable_win64.exe --path 本卡前端 worktree），启动日志 `godot_windowed_boot.log`：**D3D12 12_0 Forward+ NVIDIA RTX 4070 真实渲染、零 SCRIPT ERROR**（7 行日志：引擎头+设备行+telemetry 4444 bind 警告=全既有启动形态同款），正常关闭退出无残留。
- 拆栈：agent（Stop-Process）+kernel（Stop-Process）+窗口 Godot（Stop-Process）→ 复核六端口（7878/5555/5556/8787/4445/19876）tcp+udp 监听=0、VitApp/VitAgent/vsphub=0；Godot 变体 2=138 已上报的 20:08 孤儿（pid 9036/21188）全程未动。

## 边界声明（AGENTS §5/§7 口径）

- 真栈腿覆盖=GUI 命令构造面×生产传输链×内核真实落链回读（send 五命令+control 三命令+拓扑快照+dB 联动）；**不覆盖**：窗口内真实指针交互（选择器点击/绑定拖拽/旋钮转动）与听感——归 DOM 探针（headless 场景树 151 断言，前端仓）与[等待用户] 手测（卡面）。
- 「拖排序换位」无内核命令面（move_plugin 已在 DAG rack 模式废弃，PluginRackControlService 亲核拒绝路径）——send 节点位置语义=连线 DAG（与效果器同构），真栈腿以 connect/disconnect/set_amount 回读覆盖，索引换位如实登记为无命令面。
- 前端仓测试面（另一腿）：`tools/probe_rack_sendroute_macro.gd` 三段式 151/151 exit 0 + 既有 8 套零回退（4 rack 测试+装载弹窗探针 111/111+fe_gui_shrink a/b/c）。
