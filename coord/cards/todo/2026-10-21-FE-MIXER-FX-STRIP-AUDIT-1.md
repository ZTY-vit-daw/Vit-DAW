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

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 断言面 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
