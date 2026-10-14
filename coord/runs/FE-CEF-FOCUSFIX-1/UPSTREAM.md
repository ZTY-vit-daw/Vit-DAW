# FE-CEF-FOCUSFIX-1 上游线结论：godot_cef（gdcef）

- 调研：Mac 执行侧（GLM 会话），2026-10-14。全部信息来自本机二进制取证 + GitHub
  公开仓库（dsh0416/godot-cef）issues/releases/PR 页面，检索日 2026-10-14。

## 1. 上游定位（确证）

| 项 | 结论 | 证据 |
|---|---|---|
| 上游仓库 | **dsh0416/godot-cef**（"Godot CEF"，MIT，Copyright 2025-2026 Delton Ding） | bundle id `me.delton.gdcef`（delton=Delton Ding）；Rust+godot-rust（入口 `gdext_rust_init`，crate `gdcef`，`crates/gdcef/src/...`）；GPU 加速 OSR（Metal/Vulkan/D3D12）字符串与仓库描述吻合；类面 CefTexture/CefTexture2D/CefIpcInspector 一致 |
| 我方版本 | **v1.15.4**（2026-09-09 上游发布） | 前端仓 commit 58c6a3e「CEF v1.15.4 universal bin」(2026-09-18)；工作树干净（仅未跟踪运行时目录）=现装即该版 |
| CEF 运行时 | Chromium **152.0.5.0**（branch-heads/7977@{#1831}） | 框架 Info.plist CFBundleShortVersionString + SCMRevision |
| 我方 Godot | 4.6.stable.official.89cea1439 | `Godot --version` |
| IME 机制 | CefTexture 内部动态创建隐藏 LineEdit 作 IME 代理（`on_ime_proxy_text_changed` / `on_ime_proxy_focus_exited` / `_check_ime_focus_after_exit` / `focus_on_editable_field` 为二进制内注册方法）——与 FE-CEF-KEYINPUT-RECON-1 终判的 `@LineEdit@N` 乒乓完全对应 | ClassDB API dump（本机 headless 枚举）+ 上游 IME 文档（godotcef.org/api/ime-support.html："CefTexture 与隐藏 LineEdit 共享单一逻辑浏览器焦点……迟到的 IME 事件无法取回"） |

排除项：Lecrapouille/gdcef（C++，同名不同实现，与我们的 Rust 二进制不符）；raphipod/GDCef（C++）。

## 2. 已知 issue/修复比对（与我们缺陷的对照）

| 上游条目 | 内容 | 与我方缺陷关系 |
|---|---|---|
| **issue #183**「Input box malfunctioning」（2026-03-30 开，2026-09-18 关） | 页面 body/输入框间切换焦点时输入框间歇失焦、延迟、卡顿；Win11/Godot 4.6.1 与 4.5/gdcef v1.13.0；作者归因"input methods related hooks" | **同族**。我方=macOS 变体（R1 窗口级 blur 乒乓+R2 CefTexture↔IME 代理倒焦点） |
| **PR #228**（v1.16.1，2026-09-18 合并，关 #183） | 三同步修复：拖拽所有权；**IME 代理焦点转移会 blur/夺回浏览器**（"unify browser/IME focus"——单一逻辑焦点 owner 取代各浏览器模糊焦点）；CEF 消息泵统一挂 SceneTree 主循环 | **直接命中我方缺陷家族**。且上游在 **macOS ARM64 + Godot 4.6** 上做过 27 断言烟测（双浏览器+原生 LineEdit 独占焦点、重复点击可编辑元素）=我们的复现形态 |
| **PR #236**（v1.16.2，2026-09-26 合并） | IME 代理经 `keep_editing_on_text_submit(true)` 修复"Enter 提交后代理退出编辑态→后续键入全丢" | 同代理家族的另一失灵形态（本卡驱动轮实证的"IME 代理文本重放"缺陷亦属此类代理簿记层，见 VALIDATION.md §3） |
| issue #227（open） | 安全加速 OSR handoff 与 Godot GPU 同步 API 缺失跟踪 | **注意**：#228 的 macOS 验证用的是 software CEF；我方栈是 accelerated OSR（Metal）——升级回归必须覆盖加速路径 |

我方 v1.15.4（09-09）**早于 #228（09-18）与 #236（09-26）**——两修复均未包含。issue 追踪（两页全扫）无其他 open 的 macOS focus/IME 同型。

## 3. 升级路径评估（按卡面：只评估不替换）

**推荐目标：v1.16.2**（最小风险根修候选）
- `addons/godot_cef/godot_cef.gdextension` 与我方逐字节同构（entry_symbol/compatibility_minimum=4.5/库布局），Godot 4.6 可直接承载；
- 含 #228（焦点/IME/消息泵统一）+#236（代理编辑态）+#231（每 Godot 进程独立 CEF profile——**注意会改变 cef-data 目录行为，回归必测**）；
- 资产 `godot_cef-v1.16.2.zip`（1014.4 MB，全平台）；替换=覆盖 `addons/godot_cef/bin/`（建议先归档现 bin 目录）。

次选：v2.1.0（2026-10-12）——compatibility_minimum 提到 4.6（我方满足），但 v2.0.0 起为大版本变更（CEF 运行时版本改由 Cargo.lock 派生、Godot 4.6+ 原生快照 handoff #253、v2.1.0 增浏览器显示事件+隐藏渲染控制）。建议先 v1.16.2 验证焦点面，再评估 2.x。

**回归清单（升级验证面）**：
1. 焦点面（本卡场景）：CEF 容器内 A/B 输入框交替输入+⌘V 粘贴×5 轮（autodrive 驱动场景可复用，`res://vit_dock/diagnostics/cef_focus_autodrive.tscn`）；补偿开关两态各跑一遍（升级后若缺陷消失，OFF 轮应通过）；
2. **accelerated OSR（Metal）渲染**：纹理正常出画面、resize 不闪黑（#227 跟踪面，#228 未在加速路径验证）；
3. IME：中文输入法composition 路径（探针页 compositionstart/end 事件）+ 代理焦点行为；
4. profile/数据目录：#231 per-process profile 后 cef-data 落点与既有会话兼容性；
5. webui 主链：Ask Vit 打开→页面加载→ipc 通道（browserBridge postMessage）→关闭重开；
6. helper 进程守望：v1.16.0 起 watchdog（父进程退出杀 helper）行为；
7. Windows 侧同版本回归（PC 线同型替换时）。

**发布面**：若升级后仍复现我方 macOS 加速栈特有形态→以上取证数据包（FE-CEF-KEYINPUT-RECON-1 ANALYSIS.md+时间线+焦点链+本卡 VALIDATION.md 代理重放证据）起草上游 issue 交决策侧过目。当前判断：#228/#236 已覆盖家族主干，issue 草稿暂不需要。

## 4. 结论

- 根修=升级 gdcef **v1.15.4 → v1.16.2**（drop-in 评估通过，回归清单如上）；本卡宿主补偿为过渡解，升级验收后可退役（开关默认关，退役=删补偿调用或保留作保险）。
- 上游活跃（10-12 仍发版），无弃维风险。
