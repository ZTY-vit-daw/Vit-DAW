# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 当前占用

- 状态：占用中
- 卡：FS-LARGEPROJECT-SMOKE-1（池序45，超大真实工程自由态烟测，真栈独占）
- owner：GLM-5.3 PC 执行会话（ZCode，D:\Vit_DAW；worktree D:/Vit_DAW_wt_fsLPS1，分支 port/fs-largeproject-smoke-1）
- 机器/端口：本机 PC / agent 7878 / kernel ZMQ 5555+5556（三件套=VitApp 内核+Godot 前端+Go agent）
- 时间：2026-10-10 晚起（占用前只读核查：端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0、Godot vit-daw-frontend=0）
- 预计形态：61 轨导入+bake 预热（数十分钟级）+pull 模式自由态长程轮，单 run 墙钟可能 ≥2h，≤2 轮；异常未拆净则保留 owner 标"待处置"

## 最近一次释放

- 2026-10-09 21:41–21:47 +0800 / G3-ATTRIB-1（GLM-5.3 执行会话，D:\Vit_DAW_wt_g3a1）：harness_ab 四相 run G3-ATTRIB-1/20261009_214138_harnessab SCRIPT-LASTEXITCODE=0；释放证据=脚本 berth teardown（agent pid=23600 / kernel pid=18636 已停）+ 21:52 复核端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0；期间无脚本缺陷重跑、无栈残留。登记提交 82a654ad（被测 HEAD c4c015c5，rebase 后实现提交 hash 以分支为准）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

