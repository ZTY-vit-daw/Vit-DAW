# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

- 状态：占用中
- 资源：PC 同一套 VitApp 内核、Godot 前端、Go agent
- 机器/工作区：Windows / D:\Vit_DAW_wt_l5d（L1-5-IMPL-D 执行 worktree）；Godot 工程 D:\Godot\project\vit-daw-frontend
- 端口：7878（agent HTTP）、5555/5556（kernel ZMQ）——harness_ab berth 场景占用
- 卡 ID：2026-10-09-L1-5-IMPL-D
- owner：GLM-5.3 执行会话（PC / ZCode / L1-5-IMPL-D 实现段）
- 占用时间：2026-10-09 19:47 +0800
- 领取提交：d6347019（卡领取）；基线 origin/main=73a57864
- 当前 run ID：L1-5-IMPL-D/harness_ab_&lt;timestamp&gt;（运行后回填工件路径）
- 释放证据：（拆栈结果 / 端口与进程核对 / 工件路径）
- 交接/异常：占用前核对端口 7878/5555/5556 与 VitAgent/VitApp 进程均空闲（2026-10-09 19:47）；19:50 修正路径转义损坏行
