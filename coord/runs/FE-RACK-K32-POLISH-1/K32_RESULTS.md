# FE-RACK-K32-POLISH-1（K3.2，卡 145）执行回执

- owner：GLM-5.3 L2 PC 执行会话（前端线，ZCode）；领取 2026-10-19 19:31（主仓 main=25518b61，前端基线 1e33284）
- 前端 commit：**d195b3b**（port/fe-sendroute-macro-gui-1 本地，GH001 推送限制照旧——补丁见 `patches/0001-fix-FE-RACK-K32-POLISH-1-card-145.patch`）
- 主仓工件：port/fe-sendroute-macro-stack（stack worktree）

## 根因钉死（② 类型选择不生效）

静态链（OptionButton→`_rack_control_pref`→`vit_sidebar_create_control(kind)`→payload→渲染）逐环核验无罪
（widget factory 分发正确、normalize 正确、payload 构造正确）。**根因=view_state 保存链整体断裂**：

- `viewport_frame.get_view_state()` / adapter `get_view_state()`（输出 `rack_control_pref`）**全仓无调用方**；
- `workspace_layout_manager.capture_all_view_states()` 为空 `pass`；
- dock 侧 context 的 view_state 均为硬编码三键（zoom/scroll_x/scroll_y，vit_dock_root.gd:1494 等）。

⇒ 任何 adapter 重建（frame 内视图换出换回等）后 pref 必回退 `:76` 默认 `"knob"` 且 OptionButton 同步回退显示旋钮
——用户感知「下拉选型后创建的仍是 knob」；画布上都是 knob 卡，「调取总落最新旋钮」为其伴生感知（非独立缺陷，
探针 `vit_sidebar_focus_control_node` 按 node_id 精确匹配，k31 既有断言绿）。

## 五项改动（6 文件，+240/-6）

1. **①位号移除**：`viewport_frame.gd` `set_stage_slot_header` 不再设位号文本（SlotTag 置空+隐藏；槽位序仍存 meta
   供拖放排序）+ `ViewportFrame.tscn` 默认文本清理、节点默认隐藏。视图名+「✕拉下」保留。
2. **②修复**：`viewport_frame.gd` 按 view_type 缓存视图状态（`_load_view_scene` 拆旧前 `get_view_state()` 存
   `_view_state_cache[旧类型]`，同类型重载时灌回 `context.view_state`→新 adapter `setup_view`→`restore_view_state`
   恢复 pref）+ adapter 默认口径统一常量 `DEFAULT_RACK_CONTROL_KIND="knob"`（原 :76 成员初始 knob 与 :135 restore
   默认 slider 两处不一致；:135 为 has() 守卫内死路径默认值，仍统一）。**修复为域内补链**：dock 布局层
   （vit_dock_root/workspace_layout_manager）保存链活化不在本卡文件域。
3. **③「创建并连接」按钮**：`rack_send_target_selector.gd` 命名行加显式按钮（`_confirm_new_fx_button`→
   `_confirm_new_fx()`）；回车（text_submitted）路径保留并存；已有轨行点击即连不变；探针面 `confirm_new_fx_button()`。
4. **④语义澄清**：工具面板加一行说明（「宏=混排宏卡（多旋钮一卡，绑定后再加映射）；控件卡=按上方所选类型建单控件」）
   +双按钮 tooltip。
5. **⑤UX 两项**：创建按钮禁用态 tooltip=「先在机架选中一条轨道」（启用态恢复④语义 tooltip）；
   send 失败可见反馈=rack 新增 `signal send_command_feedback`→adapter 状态行（`_rack_ipc_alert` 补 emit），
   三个静默点全接：IPC 排队失败（既有 alert 补 emit）、connect 回执未确认 plain 边立即回滚+反馈
   （**auto_create `>new:` 与侧链 `send:sc:` 的 pending key 与拓扑 key 不同构，不参与判定防误报**，沿用 TTL 兜底）、
   超时分支清 pending+反馈。

## 验收证据

- **探针**（tools/probe_rack_sendroute_macro.gd）：178→**206 断言全绿 exit 0**（+28 K3.2：源码契约 5 + 帧/adapter
  实例腿 15 + selector 3 + ⑤B 回滚 4 + FakeRepo 扩展）。关键断言：`k32 create control payload kind slider`
  （选 slider→创建→control_add_node payload.kind=="slider"）、`k32 pref survives adapter rebuild`+
  `k32 option UI synced after rebuild`（frame 切走切回重建后 pref 保持+OptionButton 同步）、位号三断言
  （文本空/隐藏/头仍在）、按钮存在+触发+回车并存、⑤B 未确认回滚/auto_create 保留/超时清+反馈。
- **9 套回归零回退**：4 rack 测试（param_surface_panel_factory/project_repository_rack_macros/
  ipc_payload_rules/layout_rules）exit 0 + probe_rack_plugin_search_popup 111/111 + fe_gui_shrink a/b/c PASS。
- **真栈**：run=**k32_sendroute_gui_20261019_194757**，`dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke
  -KernelExe <build-feguik3 722A79D7> -GodotProjectRoot <前端 worktree@d195b3b>` 脚本 **exit 0**，
  SUMMARY_JSON `{"checks_failed":0,"checks_total":25,"status":"pass"}`（六腿：connect auto_create/list 读回/
  state aux 插件对/set_amount 双回读/宏五步含 send dB 联动/disconnect 归零+fx_released）+chat/confirm 腿过。
  被测 kernel=复用 K3 自建 build-feguik3 Release SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC
  （19:46 SHA 复核一致，含 K1）+腿内 go build agent+前端 worktree@d195b3b。占用登记 9b5a4b7a；拆栈=只杀本腿
  pid（VitApp 15828/VitAgent 3892，19:47:47/49 自起已核路径归属）→复核 VitApp/VitAgent/vsphub=0、六端口 tcp+udp=0；
  用户 Godot 编辑器 7712 全程未动。§8：LLM=0（chat 腿走脚本既有白名单话术非验收面）。

## 端测边界声明（AGENTS §5）

本探针+真栈覆盖：adapter/rack/selector/frame 的命令构造面、状态保持面、树内 DOM 形态（headless），及真栈
GUI 同源 payload 经生产传输链（UDP 4445→agent bridge→内核 ZMQ）的六腿读回。**不覆盖**：渲染视觉（位号
实际像素消失/按钮视觉/tooltip 悬停呈现——Godot headless 无渲染）与真实指针交互（点击/悬停/回车焦点流）；
归 **[等待用户] 手测复测四点**（位号消失/类型选择生效（含切走视图再切回后仍生效）/按钮创建 send/宏语义清晰）。
手测入口=用户从 Godot 拉起 DAW 前端（AGENTS §5 手测途径），前端须指向 worktree
D:/Godot/project-vit-daw-frontend-sendmacro@d195b3b。
