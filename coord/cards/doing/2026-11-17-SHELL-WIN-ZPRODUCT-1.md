# SHELL-WIN-ZPRODUCT-1：DAW 打开 z 锚定产品修订

- 2026-11-18 11:30 +08:00：用户指定今日由 Codex `/root`（PC）接任窗口线主管；验收与后续放栈由本主管承担。
- 领取：owner Codex `/root/01a1ecb4-7a8f-7a50-9de5-6341ddde8b20`（GPT-6.1 Sol/high，PC）；共享 main 基线 `d4653d2f`；前端 `main@2d92d55`（detached HEAD），独立 worktree `C:/Users/timoz/.codex/worktrees/shell-win-zproduct-409/vit-daw-frontend`；领取时 `git status --short` 与 `git diff --stat` 均为空。
- 当前仅放行前端生产装配和无窗构建/测试。408 探针只读，禁止带入 394/402 未验收实现；保留 396 的 `app/shell/agent_chat_window.gd` 和 406 的取证脚本/探针文件域。主仓验收腿待首段复核后另行锚定，不授权占栈或启窗。
- 2026-11-18 12:15 +08:00 阶段二准备（同一执行 owner）：启动时序与失败回收窄修已无窗复核。追加现有产品守卫的只读取证和 `scripts/vitshell_open_chain_smoke.ps1` 的 opt-in 409 验收腿；主仓实现 worktree `C:/Users/timoz/.codex/worktrees/shell-zproduct-smoke-409/Vit_DAW`，detached `90b42269`。只允许构建、纯测试和离线验证，**未授予真栈和开窗权限**。标准与工件见 [STAGE2-GATES.md](../../runs/SHELL-WIN-ZPRODUCT-1/STAGE2-GATES.md)。

- 池序409；P1；PC；发卡/验收=GLM 主管决策流（ZCode 会话）；派发确认=已确认（用户 2026-11-17 三项裁定〔路径/显示后纠正可接受/原生缝可进产品〕+当晚裁定 A，见 [decisions/2026-11-17-window-z-path-and-seam-ruling.md](../../decisions/2026-11-17-window-z-path-and-seam-ruling.md) 与 408 卡收口节）。
- 前置：408 取证 PASS（同线程 CHANGING z 事务机制已证，工件主仓 9873eaf1/前端 e3e6374）；停机期焦点违约已裁定产品窗外 known behavior。
- 单目标=产品路径上 DAW 首次 ordinary 显示时：z 锚到对话窗之下、零激活、既有 geometry 装载逻辑不回退。实现=已批准原生缝（产品 Godot 工程显式加载的测试级→产品级 extension）装配 guard；VitShell→loader 显式传递 reference（HWND+nonce+PID，校验失败 fail-open 现状行为并留日志）；配「显示后立即一次 z 纠正」兜底（用户裁定②）。408 未跑的 external 自然前台腿并入本卡真栈验证。
- 文件域（领取时锚定，越域停报）：前端仓 VitShell 生产宿主窗口管理+产品工程 extension 显式装配；主仓既有 ps1 烟测体系加产品断言腿。不动内核/agent/395 呈现面；禁止远程注入（只随工程显式加载）。
- 验收：①真栈腿（VitShell 产品入口拉完整栈）：打开工程→对话窗前台保持断言+DAW 在对话窗之下的 z 断言+external 前台条件腿；②渲染面/旅程覆盖边界按仓库 §5 显式声明；③用户手测节点（391 原案：打开工程对话窗不退后；装载动效属 395 域不在本卡）。
- 停止条件：产品装配下机制失效同断点两次→停转取证；越域/需改内核→停上报。
- 领取/回执/验收：待回填。
