# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

- 状态：空闲
- 资源：PC 同一套 VitApp 内核、Godot 前端、Go agent
- 机器/工作区：Windows / D:\Vit_DAW_wt_l5d（L1-5-IMPL-D 执行 worktree）；Godot 工程 D:\Godot\project\vit-daw-frontend
- 端口：7878（agent HTTP）、5555/5556（kernel ZMQ）——已释放（2026-10-09 20:10 核对 0 监听）
- 卡 ID：2026-10-09-L1-5-IMPL-D
- owner：GLM-5.3 执行会话（PC / ZCode / L1-5-IMPL-D 实现段）
- 占用时间：2026-10-09 19:47 +0800（登记 ae351b20；路径转义修正 ae351b20 后继）
- 领取提交：d6347019（卡领取）；基线 origin/main=73a57864
- 当前 run ID：L1-5-IMPL-D/20261009_195558（harness_ab 双模，SCRIPT-LASTEXITCODE=0）
- 释放证据：脚本 finally 拆栈（agent pid=2876 / kernel pid=25900 已停）；20:10 复核 VitApp/VitAgent 进程=0、端口 7878/5555/5556 监听=0；工件 coord/runs/L1-5-IMPL-D/20261009_195558/
- 交接/异常：占用前核对空闲（19:47）；期间两次脚本缺陷重跑（Set-Item 参数名/指标 int 强转——见卡回执），无栈残留；19:50 修正登记文件路径转义损坏行
