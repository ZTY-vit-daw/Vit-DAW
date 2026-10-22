# FE-MIXER-FX-STRIP-AUDIT-1：混音台 FX/接收轨去 send 歧义（Logic 源轨式+Pro Tools 接收轨式）

- 发卡：GLM 主管决策流 / 2026-10-21 深夜（依据=用户 UX 讨论裁定采纳分层方案：混音台源轨=Logic 式 send 推子（既有 SEND 槽位保持）；FX/接收轨=Pro Tools 式纯通道——推条+处理器，不渲染任何读作「发送」的元素）
- 派发确认：已确认（用户裁定，2026-10-21 M2 场次③后讨论）
- 验收负责人：GLM 主管决策流
- 池序 167；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P2 / 0.25 天 / 依赖=无（与 164/166 文件域不交可并行）；模型分级：L1 / flash 可接

## 范围

审计混音台 strip 的 send 元素分布：**仅源轨**渲染 SEND 槽位（Logic 式，量值+目标名）；**FX/接收轨 strip 不出现任何「send」字样/形态控件**——回收入口身份如需表达用「◂ 总线回收」类徽标或干脆不表达（send 指向它的事实已足够，Pro Tools 模型）。发现接收轨现有 send 形态元素即改；源轨 SEND 槽位行为零改变。

## 文件域

`vit_dock/views/mixer_dock_adapter.gd`（strip send 槽位渲染面）邻域；越域即停。

## 验收标准

组件/探针断言：源轨 strip 有 send 槽位、FX 轨 strip 无 send 字样元素（或仅徽标）；161 的 mixer 落库广播链零回退；既有测试零回退；窗口态视觉=用户手测目检项（端测边界声明）。

## 停止条件

FX 轨 strip 的 send 元素承担既有功能面（如断开入口）且移除会丢功能 → 锚点上交设计再裁定。

- 领取：2026-10-22 10:03 / 主仓 origin/main=1ae57f23（前端仓 origin/main=c5cac57，161 已合入基线）/ owner=GLM-5.3 flash（ZCode 执行会话，PC，167 领取流）/ 分支=port/fe-mixer-fx-strip-audit-1（前端仓，基线 c5cac57）/ worktree=D:/Godot/project/vit-daw-frontend-mixerfx / 文件域核对=与 doing（GEN-SUNO-WEB-DESIGN-1）及并行在飞 164（app/rack/Vit_Graph_Rack.gd 邻域）/165（legacy/window_manager 邻域）/166（rack send 渲染邻域，依赖 164 串行未领取）均不交；本卡只触 vit_dock/views/mixer 邻域。/ 领取提交=**179f838b**（已核远端 tip）
- 回执：实现=前端仓 **port/fe-mixer-fx-strip-audit-1@eda2add**（2 文件 +90/-1 全在卡域：`vit_dock/views/mixer/vit_mixer_channel_strip.gd` +38；探针件 `tools/probe_rack_sendroute_macro.gd` +53/-1）。实现面=strip 按 repository send 拓扑 `is_fx_track`（fx_tracks 内核上报面，与 Track_Row FX 徽标/机架同源）判定接收角色：接收轨隐藏 SEND 按钮+面板、仅渲染「◂ 总线回收」徽标（tooltip 无 send 字样）；源轨分支逐行零改变；角色随拓扑双向转换（同挂既有 send_topology_changed，零轮询零新信号）。断言面=sendroute 探针 231→**239 全绿 exit 0**（新增 8：mix167 源轨 SEND 按钮可见+源轨徽标不出现；fx167 接收轨按钮隐藏/面板隐藏/可见面 Button+Label 零 SEND 字样扫描/徽标可见含「总线回收」/角色离开恢复槽位/徽标随之隐藏）；**161 mixer 落库广播链零回退=ext161 13/13 PASS**；既有测试零回退=plugin_search_popup **111/111** + shrink a/b/c PASS。复现命令（前端仓 worktree 根）：`"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_rack_sendroute_macro.gd`（fresh worktree 须先 `--headless --path . --import` 预热导入缓存——首跑曾因冷缓存报 filter_rows/类解析假失败，预热后复现稳定）。停止条件核对=**未触发**：FX 轨 strip 现存 send 形态元素仅两类——「SEND (0)」空槽位按钮（零功能，纯噪声）与该轨作为 send 源的边行（量值+断开）；后者控制在机架 SendOut 卡完整保留（本卡 rack 面零改动，164/166 域），混音台不丢唯一功能面。**端测边界声明**：headless 组件级探针（通道条场景+FakeRepo+真 hub），断言 DOM 形态/可见性/文本与命令构造面；**未覆盖**窗口态渲染视觉（徽标字号/配色/布局观感）与真实 ZMQ/内核链（本卡零 adapter/rack/链路改动，161 链经探针断言零回退）；窗口态视觉=用户手测目检项（卡面既有约定），真栈腿未跑、是否需要归主管裁定。
- 验收：（裁定文件 / 验收 commit）
