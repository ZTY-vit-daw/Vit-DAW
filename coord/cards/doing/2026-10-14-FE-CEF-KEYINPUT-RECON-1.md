# FE-CEF-KEYINPUT-RECON-1：CEF OSR 容器输入会话死亡取证（键入+粘贴双失效）

- 发卡：Mac 决策侧 / 2026-10-14（依据=用户 2026-10-13 两轮实机取证+决策侧代码/上游调研，底账见下）
- 派发确认：已确认（用户 2026-10-14「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 111（109/110 已占用：FIX-FS-SETTLE-DEADCHAIN-1 / FIX-CARRIES-REPORT-TIGHTEN-1）
- 归属：Mac 线（Godot 前端宿主层×webui 诊断面）；P1 / 0.5-1 天 / 无依赖
- 模型分级：L2 / GLM 执行会话

## 取证底账（决策侧 2026-10-13/14）

用户容器=Godot 前端经 godot_cef `CefTexture`（OSR 纹理浏览器，`vit_dock/scenes/vit_cef_web_view_host.gd`，B7B）内嵌 webui。用户实机两轮取证，缺陷画像：

1. 进入设置页后，**第一次点击的输入框可输入，且只有第一次输入生效**；
2. 之后**键入与粘贴（⌘V）全部失效**（其他框与本框均不可输入）；
3. 新旧 webui（9-19 旧 dist 与 10-13 修复版 dist）同样失灵——webui 层修复不改变现象。

webui 层排除证据：FE-CONFIG-EDIT-1 修复版源码（IAB 真浏览器真键盘）16/16 全过 + 10-13 复验 16/16 + dist bundle 内容核对无误；React 受控输入确定性代码不产生间歇性失灵；粘贴失效排除"键盘事件转发单链"假设（paste 走浏览器进程编辑命令，仅要求焦点状态正确）。

当前主假设：**CEF 焦点会话死亡**——首次点击后某时刻 CEF 浏览器认为自身无焦点（Godot 焦点链↔gdcef↔CEF 浏览器进程的状态断点），从此丢弃一切输入。上游同类：cefpython #284（Linux/Mac 多焦点问题）、CEF 论坛 OSR 键盘转发/焦点处理（macOS 需额外处理）。备选假设（探针排除）：React 事件委托失效（webui 层翻案可能）。

## 目标

1. **诊断探针**：webui 加独立诊断入口（**不动设置页产品代码、不动已部署 dist**——独立路径/端口服务，如 `__input_probe` 静态页或 dev 栈），注入三层事件时间线记录：
   - 焦点层：`document.activeElement` 变化、window focus/blur；
   - 页面事件层：keydown/keyup/input/paste 是否到达页面（含时间戳与 target）；
   - React 层：onChange 是否执行、受控值是否被重渲染抹回（如用受控 input+值快照）。
   时间线就地可视化（页面浮层或 POST 落盘），无需 devtools。
2. **用户单轮复现采集**：[等待用户] 探针就位后按底账画像复现一次（第一次点击→第一次输入→切换/后续输入→粘贴），全时间线留存。
3. **浏览器直开对照**：Safari/Chrome 直开同地址设置页（探针页或产品页），同一操作序列——正常/异常如实记录。
4. **前端侧焦点链日志**（前端仓独立 git，分支纪律同主仓）：vit_dock 宿主层 focus/grab_focus/焦点变化处加可开关日志（默认关，仅本卡取证用），采集复现窗口内 Godot 侧焦点事件序列。
5. **结论**：断点层判定（CEF/Godot 焦点层 vs React 层）+ 修复面定界（gdcef 使用层可修 / 插件升级 / 上游 issue / 换宿主方案）+ 最小复现条件。

## 文件域

- 主仓 agent/webui：新增独立诊断入口（不改设置页/产品组件逻辑）；
- 前端仓（/Users/timozty/Documents/vit-daw-frontend，独立 git，当前分支 port/leg5-telemetry-probe 有他人改动——领取时记录其状态，不整理不动）：vit_dock 宿主层可开关日志；
- 工件：coord/runs/FE-CEF-KEYINPUT-RECON-1/。
- 与在飞卡核对文件域互斥（尤其 webui 域）。

## 验收标准

- 探针构建+服务方式记录（端口/路径/启动命令），产品 dist 未被覆盖（部署目录 mtime 前后核对）；
- 用户复现时间线 + 浏览器直开对照 + 前端焦点日志三件工件入 run 目录；
- 断点层结论有数据支撑（时间线明确指向哪层断开），修复面定界明确。

## 停止条件

- 探针显示三层全部正常但用户仍复现失灵 → 原始数据上交，不猜；
- 前端仓改动风险超界（他人 WIP 冲突）→ 前端侧日志降级为只读分析（代码审读+现有信号），不强行改。

- 领取：2026-10-14 18:24 +0800 / origin/main=4476b9ed / GLM-5.3 L2 执行会话（Mac，ZCode）/ 分支 port/fe-cef-keyinput-recon-1 / 实现 worktree /Users/timozty/Documents/Vit-DAW-fecef1（coord 操作走 /Users/timozty/Documents/Vit-DAW-fecef1-coord）/ 领取提交 045ae77f（hash 回填=045ae77f 后续提交）
- 回执：（commit / run ID / 三件工件路径 / 断点层结论）
- 验收：（裁定文件 / 验收 commit）
