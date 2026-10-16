# FE-CEF-PASTE-SHIM-2：⌘V 垫片捕获点升级（gui_input→Window 级，真实焦点路径收不到事件）

- 发卡：Mac 决策侧 / 2026-10-16（依据=用户实机复现：产品设置页真实 ⌘V 仍不进；GAPS-1 垫片验证面的自动化/真实路径分叉被用户轮暴露）
- 派发确认：已确认（用户 2026-10-16「我进去手测发现还是无法复制」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 122
- 归属：Mac 线（Godot 前端宿主层）；P1 / 0.5 天 / 依赖=FE-CEF-V1162-GAPS-1（垫片在 13d670e）
- 模型分级：L2 / GLM 执行会话

## 缺陷底账（决策侧 2026-10-16 定位）

1. GAPS-1 垫片挂点=CefTexture `gui_input` 信号（vit_cef_web_view_host.gd:200-205/_on_browser_gui_input:498）。
2. **真实用户路径**：真实点击聚焦页面输入框后，v1.16.2 焦点重构的常态是 **IME 代理 LineEdit（@LineEdit@N）持有 Godot 焦点**——键盘事件按焦点链路由给该子控件，**CefTexture 的 gui_input 收不到 ⌘V**（GAPS 硬门轮日志原句佐证："⌘V 由 IME 代理 LineEdit 持焦点，gui_input 不达，shim 零触发"，当时被归因环境阻塞，实为真实路径常态）。
3. **自动化路径为何过**：paste_diag T3/T4 用 eval focus（JS 聚焦页面元素）前置——不触发 Godot 侧代理焦点迁移，CefTexture gui_input 反而能收到合成 ⌘V。此分叉即 GAPS-1 验证盲区（VALIDATION 边界里"真实点击聚焦→⌘V 完整路径归用户终验"一句已预警，用户轮暴露）。
4. 用户 2026-10-16 实机：产品设置页（真实点击聚焦）⌘V 不进。键入/焦点腿维持已修状态不受影响。

## 修复目标

1. **捕获点升级**：OSR 宿主 Window 级 `_unhandled_key_input`（或 `_input`+窗口内守卫）接收 ⌘V——无论焦点在 CefTexture 还是其子代理 LineEdit，未被焦点控件消费的键盘事件可达 Window 级；守卫沿用现有（cef_paste_shim_enabled 开关/组合态 __vitCefComposing/activeElement 可编辑+非 readOnly/disabled/剪贴板非空），并加"目标浏览器可见"守卫（多窗口/隐藏态不出手）。旧 gui_input 挂点移除或保留为冗余（防双触发：两处都触发时第二个被组合态/重复注入守卫拦——领取时定，回执记录）。
2. **引擎边界路由验证（可无人值守）**：input_hook_test 场景扩展——真实焦点路由形态（把 Godot 焦点给一个 LineEdit 子控件）下注入 ⌘V，断言 Window 级捕获收到且垫片 fire（CEF 输入门不管 Godot 内部路由，此验证不受 OS key 限制）。
3. **[等待用户] 终验**：用户轮产品设置页真实点击聚焦+真实 ⌘V 粘贴入框。

## 文件域

- 前端仓续 port/fe-cef-v1162-gaps-1 基础（13d670e 之上）：vit_cef_web_view_host.gd+vit_dock/diagnostics/cef_input_hook_test 场景；
- 主仓：coord/runs/FE-CEF-PASTE-SHIM-2/ 工件；agent/webui 零改动；前端 GH001 推送限制照旧。

## 验收标准

- 引擎边界路由验证 exit 0（新捕获点在代理 LineEdit 持焦点形态下收到 ⌘V 且 fire）；
- [等待用户] 终验真实 ⌘V 入框过后本卡 pass；
- 回执注明双挂点策略与守卫行为。

## 停止条件

- Window 级也收不到（焦点路由被 gdcef 内部完全吞掉）→ 上交换捕获策略评估（含代理 LineEdit 事件桥），不硬试。

- 领取：2026-10-16 18:48 CST / origin/main 6ad07ad8 / GLM-5.3 L2 Mac 执行侧 ZCode 会话（本卡会话）/ 实现=前端仓 port/fe-cef-v1162-gaps-1（基线 13d670e，前端仓主工作树，本线单流）/ 主仓协调 worktree Vit-DAW-pasteshim2-coord 分支 coord/pasteshim2-claim / 领取提交 54283662
- 回执：前端仓 commit **255fa69**（port/fe-cef-v1162-gaps-1；patch 存档 coord/runs/FE-CEF-PASTE-SHIM-2/frontend_255fa69.patch，GH001 限制照旧）；run ID=FE-CEF-PASTE-SHIM-2（r1/r2，工件 coord/runs/FE-CEF-PASTE-SHIM-2/：VALIDATION.md+两轮 log+patch+stat）。**路由验证证据**：LEG-R 三断言两轮全过——ProxySimLineEdit 持焦点形态（focus_owner=/root/VitCefWebViewWindow/CefTexture/ProxySimLineEdit）注入 ⌘V：capture_key_delta=2（Window 级 _input 收到 press+release）/ fire_delta=1（防重复断言）/ 页面标记 358/351ms 到达；r2 硬退出码 **exit 0**；gui_input_delta=0 同场复证旧挂点盲区（卡面缺陷底账 2 直接实证）。**双挂点策略（领取时裁定）**：主=宿主 Window 子树捕获节点 `_input`（GUI 焦点链之前分发——IME 代理 LineEdit 持焦点且消费键盘事件均送达，选 _input 而非 _unhandled_key_input 因后者依赖焦点 owner 不消费、行为不保证；事件按 viewport 分发天然限定本窗口=窗口内守卫）+ 浏览器可见守卫（_visible+window.visible，隐藏态不出手）；副=旧 gui_input 挂点保留（eval 聚焦/CefTexture 持焦点形态覆盖）；防双触发=同一 InputEvent 实例跨阶段（_input→gui_input）共享实例 id 记账去重，同实例只出手一次，fire_delta==1 断言在案；守卫链沿用（开关/组合态/activeElement 可编辑/剪贴板非空）+诊断计数 paste_shim_fire_count；回滚 v1.15.4 须 VIT_CEF_PASTE_SHIM=0 纪律不变。**端测边界声明**：本卡改动域=Godot 前端宿主层，agent/webui 零改动（不触发 agent 侧烟测门槛，前端渲染面无布局/交互变更）；LEG-R 页面聚焦前置=eval focus（合成点击被 v1.16.2 CEF 输入门在无 OS key 会话丢弃，GAPS-1 已取证）+ ProxySimLineEdit 为 IME 代理的结构等价模拟（browser 子控件 LineEdit）非 gdcef 代理本体 + 真实 IME composition 守卫行为——三项均归 [等待用户] 终验（卡面目标 3，步骤已写入 run VALIDATION.md：产品设置页真实点击聚焦+真实 ⌘V）。停止条件未触发（Window 级捕获两轮均收到）。
- 验收：2026-10-16 Mac 决策侧——交付采信（Window _input 主捕获设计=焦点链前分发+viewport 天然限窗；实例 id 去重防双触发；LEG-R 两轮 exit 0 且 gui_input_delta=0 实验坐实旧盲区）；瑕疵记录=run 工件未入库（场景在前端仓可复跑，裁定以回执断言+代码亲读为据）；**[等待用户] 终验=产品设置页真实点击聚焦+真实 ⌘V 入框，用户明确反馈后方可终局 pass**（上轮教训：不再凭不完整日志落 pass）
