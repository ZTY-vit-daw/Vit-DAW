# UI-ENTRY-2 PROGRESS（断点续跑）

- 卡：UI-ENTRY-2（池序 355，P0）启动页门面退役；owner=freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 分支：port/ui-entry-2 双仓；主仓 worktree D:/Vit_DAW_wt_uientry2（基线 origin/main=32307204，status/diff 全净）；前端 worktree D:/Godot/project/vit-daw-frontend_wt_uientry2（基线 main=ae24b52，status/diff 全净）

## 步骤

- [x] S0 勘察：卡面/协议/354 RUN-SUMMARY/agent_chat_window.gd 现状/entrySurfaceBridge.ts 读取；stop condition 预核查完成（见 S0 结论）
- [ ] S1 领取：卡 todo→doing coord-only 推 main
- [ ] S2 M1 关 React 窗=退出整个应用（agent_chat_window.gd `_teardown_after_close` → quit；经 VitIpcClient._exit_tree 连带终止 autostart 子进程）
- [ ] S3 M2 失败链去 Godot 化（health 失败仍导航+错误态留 React 窗+恢复轮；WebView2 不可用=OS.alert+quit；start_page 删编排引用）
- [ ] S4 M4 splash 压制（project.godot boot_splash/show_image=false）
- [ ] S5 M3 工程态往返：**停止条件命中——降级记录**（内核 CommandDispatcher 仅 new/open/save 无 close；前端 project_client 同；dock 无回 launcher 场景路径）→ 不实现，交主管裁定
- [ ] S6 烟测脚本 scripts/ui_entry2_smoke.ps1（四腿：失败注入去 Godot 化/WebView2 错误框退出/autoload 全链+关窗全栈退出断言/legacy 回归）
- [ ] S7 webui 回归（npm install→test→build；零 webui diff 预期）
- [ ] S8 真栈：PC-RUNTIME-STACK 占用登记→推 main→重读→≤60s 复核→跑→finally 拆栈→释放
- [ ] S9 回执：卡面回填→doing→done→coord 推送

## S0 勘察结论（M2/M1 设计锚点）

- 354 现状：agent 不可达=restore_main_window(unavailable)+隐藏 React 窗（Godot 启动页兜底）；关 React 窗=restore_main_window(chat_closed)（回退 Godot 页）。两者均为本卡要退役的回退。
- extension 面（零 cpp 改动即可满足本卡）：`set_parent_window_shown`（SW_HIDE/SW_SHOW 主窗）354 已合入；WM_CLOSE→DestroyWindow→os_event "closed" 已有；dll 已提交入库（extension/bin/），worktree 免重编免 junction。
- 引擎限制（354 probe 铁证）：Godot 层 hide 主窗=降级最小化；真隐藏必走 extension SW_HIDE——沿用。
- load_error 边界（新发现）：extension NavigationCompleted 不查 IsSuccess——连接拒绝类页面失败不发 load_error（只有 Navigate API 失败才发）。故 health 失败后的"重试"以**低速恢复轮**（re-poll→恢复即重导航）实现，不依赖 load_error 链；记为诚实边界。
- autostart 链：VIT_SKIP_DEV_AUTOSTART 未设+editor 模式时 start_page 以 OS.create_process 拉起 kernel/hub/agent 并经 VitIpcClient.register_autostart_child_process 登记；quit 时 VitIpcClient._exit_tree OS.kill 全部登记子进程——关窗=退出编排的 agent 断言面由此成立（生产链=用户从 Godot 拉起全栈）。
- 内核 VIT_PROJECT_XML env 可经 Godot 进程环境继承到 autostart 内核（隔离 workspace）。

## 失败分类（§8 预登记）

- exit 0=PASS；断言失败/环境中断/装置缺陷分开记录；同代码同断点连续 2 败停原样重跑。
- 装置风险预登记：PS1 中文注释须 BOM；EnumWindows delegate 内禁 `$pid`（用 $procId）；OS.alert 原生框为阻塞模态（烟测以 EnumWindows 找 #32770 对话框 + WM_CLOSE 关闭后断言进程退出，不留悬挂模态）。
