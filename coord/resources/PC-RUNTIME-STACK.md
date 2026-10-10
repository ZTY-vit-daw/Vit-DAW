# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 当前占用

- 状态：空闲

## 最近一次释放

- 2026-10-10 18:30–19:15 +0800 / FS-LARGEPROJECT-SMOKE-1（GLM-5.3 PC 执行会话，ZCode D:\Vit_DAW；worktree D:/Vit_DAW_wt_fsLPS1）：四 run（183305/183914=工具性中止，栈由执行侧人工拆除核清；184500=$pid 崩溃后人工拆除核清；185519=末轮脚本自 teardown torn_down=true）；19:15 复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0、Godot vit-daw-frontend=0。占用登记 de1d441b；实现 commit 2291d055@port/fs-largeproject-smoke-1；verdict FAIL（A1/A4 pass，A2/A3 fail）详见卡回执。期间 185519 一轮干净自释放，前三轮人工拆除均在 ABORTED_NOTE/RUN_NOTE 留痕。

## 历史释放（归档）

- 2026-10-09 21:41–21:47 +0800 / G3-ATTRIB-1（GLM-5.3 执行会话，D:\Vit_DAW_wt_g3a1）：harness_ab 四相 run G3-ATTRIB-1/20261009_214138_harnessab SCRIPT-LASTEXITCODE=0；释放证据=脚本 berth teardown（agent pid=23600 / kernel pid=18636 已停）+ 21:52 复核端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0；期间无脚本缺陷重跑、无栈残留。登记提交 82a654ad（被测 HEAD c4c015c5，rebase 后实现提交 hash 以分支为准）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

