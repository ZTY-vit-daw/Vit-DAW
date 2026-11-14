# VitShell 宿主架构设计确认（手测 v5 后产品裁定）

- 日期：2026-11-15 凌晨
- 裁定：用户确认换宿主方案 + 产品定位裁定
- 状态：**设计稿候用户过目，未过目不开工**

## 产品定位（用户原话裁定）

- **VitShell + WebView2（React）= Vit 的主要窗口**——启动即现、常驻，是产品本体
- **工程 GUI（Godot DAW）= 次要窗口**——被激活触发（打开工程）才打开
- 启动态零 Godot 进程/窗口（「删掉启动页」的最终实现路径=Godot 根本不在启动链上）

## 架构

```
现在（废弃）：Godot(先起,藏窗) ─拉起→ agent ─→ 内核；React窗=Godot内挂
目标：
  VitShell.exe（主窗口本体，常驻）
    ├─ WebView2 窗 = React webui（launcher↔project surface 同窗切换）
    ├─ 进程树：拉起 agent（HTTP 7878）→ agent 侧内核链不动
    ├─ 按需拉起：Godot.exe --path <前端工程> --直启工程<X>（次要窗口）
    └─ 退出清理：关主窗=全进程树终止（355 语义搬入宿主）
```

## VitShell 形态

- 小 C++ exe，**复用 extension 的 WebView2 宿主代码**（vit_webview_host 已有：窗创建/消息路由/通知桥/Win32 窗口语义——343/349 成果可搬）
- 职责四件：①主窗（标题 Vit/任务栏/最小化/通知——349 语义）②进程树管理③按需拉 Godot④启动态桥
- WebView2 runtime 依赖：启动检查+错误框（Win10/11 通常自带）

## 启动链时序

```
双击 VitShell → 主窗先出（装载态，不等 agent）→ agent 就绪 → 真数据
→「打开工程」→ 治理确认（React 页内，337 本有设计）→ agent/内核装载
→ VitShell 拉 Godot 直启 DAW（次要窗出现）
→ 工程期：主窗常驻 + DAW 协作；关 DAW=回单主窗形态
→ 关主窗=全树退出
```

## 归属与不变面

| 组件 | 变化 |
|---|---|
| React webui | surface/桥客户端基本不动（消息源从 Godot extension 换宿主） |
| agent / 内核 | **零语义改动**（HTTP/ZMQ/IPC 契约不动；close-project 缺口维持既有降级） |
| Godot 工程 | 新增「直启 DAW」启动参数（复用 open 公共链）；start_page/343/349 窗机保留休眠（legacy/编辑器路径零回归） |
| extension | WebView2 代码抽出共享进 VitShell |

## 开发路径不变声明

- Godot F5/直跑照旧可用（=纯 DAW 形态调试）
- webui 浏览器开发照旧；VitShell 为新增小 C++ 工程

## 拆卡（估 2–3 天量级，候用户过目后发）

1. **SHELL-1** 骨架：VitShell exe（主窗+进程树+装载态+退出清理+真实环境 API 连通——用户观察到的「没连上」在此卡验收面）
2. **SHELL-2** 启动态桥：open/治理（React 页内）/文件对话框 + Godot 直启参数（复用 348 语义）
3. **SHELL-3** 真栈烟测+手测 v6 放行（断言：启动零 Godot 进程、主窗唯一、拉起/退出链）
4. 并行独立卡：launcher 视觉保真返工（对照 v5.1 定稿稿——与宿主无关可同跑）

## 风险（如实）

- WebView2 runtime 缺失环境（罕见，错误框兜底）
- 治理确认走 React 页内（建议）——337 chat 内确认本是在案设计，宿主只兜文件对话框
- Godot 直启需前端小改（boot 参数→dock 场景+装工程）
- 多工程切换仍受内核无 close 命令限制（既有降级，除非另开内核卡）
