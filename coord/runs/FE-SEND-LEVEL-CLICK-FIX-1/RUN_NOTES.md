# FE-SEND-LEVEL-CLICK-FIX-1（182）run 工件说明

- 领取：主仓 61b1eff1（todo→doing）；前端 worktree `D:/Godot/project/vit-daw-frontend-lvlfix1` @ `port/fe-send-level-click-fix-1`，基=origin/main 9015cfd（178 合入态 fetch 亲核）。
- 占用登记：7c2a5f18（§2.2）；释放见 PC-RUNTIME-STACK.md 本卡条目。

## 腿与工件

| 工件 | 腿 | 结果 |
|---|---|---|
| `lvlclick_gui_*/ps1_console_full.txt` + `sendroute_gui_smoke_console.txt` | 真栈 SendRouteGuiSmoke（dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke；kernel=主检出 K3 722A79D7 只读亲核；被测=前端 worktree；Godot 4.6.1 console headless） | **脚本 exit 0；stack probe 64/64 PASS**（含 L10 真实事件管线八断言：入口/点击送达/弹窗开启/rect 全在屏内/初值 -6.0/grab_focus 成立/上箭头真实点击 +0.5/拓扑读回 -5.5 收敛） |
| `realinput_dedicated_headless.txt` | 独立自驱动腿 `tools/probe_rack_sendlevel_realinput.gd` headless | **PROBE PASS 15/15 exit 0**（真实点击按钮→PanelContainer 叠层弹窗在屏内→LineEdit 焦点→上箭头真实点击 +0.5→payload 三字段 -5.5→关闭→悬空守卫零发包+弹层不重开） |
| `realinput_dedicated_windowed.txt` | 同上**窗口态**（无 --headless，真实窗口） | **PROBE PASS 15/15 exit 0**（验收主面「窗口态真实鼠标注入断言」机器腿） |
| `realinput_dedicated_headless_30sidle.txt` | 同上 + 点击前空等 30s（时间型降级假设排除取证） | PROBE PASS 15/15 exit 0 |
| `k3_main_suite_313.txt` | headless K3 主套件（零回退） | PROBE PASS 313 exit 0（318 基线 - 5 条 lvl178 交互断言移入独立腿，入口结构断言保留；emit 直调全数下线） |
| `zero_regress_*.log`（5 份） | popup 111 / shrink A 87 / B 119 / C 342 / start_feed 244 | 全 0 FAIL、全 exit 0 |
| `stack_probe_direct_64.txt` | 真栈探针直跑（活栈迭代取证腿，内容与烟测腿同构） | 64/64 exit 0 |

## §8 记账

- LLM=0（全部确定性探针与烟测）。
- 真栈腿一次过（首轮 59/60 后定位窗口尺寸断言问题修复，复跑一次全绿；首轮工件 lvlclick_gui_235724 留档含 FAIL 现场）。

## 取证结论（根因与边界声明）

1. **根因裁定**：178 的 PopupPanel 属 Window 族，本工程 `project.godot` `embed_sub_windows=false` 下弹为原生 OS 子窗，窗口态实测其区域鼠标事件穿透回画布、SpinBox 收不到输入（用户两轮手测实证）；同族证据=headless 曾出的「can't grab focus」警告——实际是 SpinBox 自身 focus_mode 默认 NONE，`grab_focus()` 直调必败（182 一并修复：聚焦内部 LineEdit）。修复=回仓内已验证弹层先例（rack_plugin_search_popup/rack_send_target_selector 同款 PanelContainer 叠层：z_index=4096+move_to_front+钳制定位+点外关闭）。
2. **探针断言升级（171 同族纪律）**：全部「点击」断言改 `Viewport.push_input` 真实事件管线（motion→press→release 于目标 rect 中心，经 viewport pick→最深控件→_gui_input）。引擎 4.6 口径亲核：BaseButton 鼠标 press 需先经 MOUSE_ENTER 建立 hovering；`embed_sub_windows=false` 下 hover 记账走原生窗路径、无 WM 鼠标事件即永不派发——探针以 `root.set_embedding_subwindows(true)` 切 embedder 路径 + motion 先行建立 hover；长跑探针进程（K3 主套件/真栈腿）存在进程级 hover 记账通道失活（30s 空等排除时间型；独立干净进程不受影响），helper 在 motion 后 hover 未建立时补发 MOUSE_ENTER 通知重建该 WM 等效前置态——**点击本身全程 push_input 真实管线，无任何 emit 直调**。
3. **端测边界声明**：①窗口态真实鼠标注入=自驱动进程内注入（push_input 于按钮/箭头 rect 中心，完整 pick→gui_input 管线），非 OS 级硬件事件注入；②真栈腿覆盖 agent/内核真实传输（UDP 4445/ZMQ 5555）与 repository 拓扑读回收敛；③最终用户手感（真鼠标点击弹窗内调值）留待用户窗口态手测复验（卡面④）。
