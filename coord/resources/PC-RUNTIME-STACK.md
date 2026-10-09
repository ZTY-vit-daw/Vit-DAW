# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

## 当前占用

- 状态：占用
- 资源：PC 同一套 VitApp 内核、Go agent（harness_ab berth 模式不启 Godot UI——场景约束同 L1-5-IMPL-D 腿4 先例）
- 机器/工作区：Windows / D:\Vit_DAW_wt_g3a1（G3-ATTRIB-1 执行 worktree）
- 端口：7878（agent HTTP）、5555/5556（kernel ZMQ）
- 卡 ID：2026-10-09-G3-ATTRIB-1
- owner：GLM-5.3 执行会话（PC / ZCode / G3-ATTRIB-1）
- 占用时间：2026-10-09 晚 +0800（占用前核对：端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0）
- 领取提交：d6c4f060（卡领取）；实现提交 6560c69c（被测代码 HEAD）；基线 origin/main=7eb38fc5
- 内核二进制：复用 D:\Vit_DAW\Export\staging\runtime\VitApp.exe（sha256 前 16=B6565DCF85D1DA86，mtime 2026-10-06T22:02:38——本卡零内核改动，C++ 面未触碰）
- 预期 run：coord/runs/G3-ATTRIB-1/<timestamp>（harness_ab 四相：base+多话术+确认往返+预算）
- 释放证据：（跑后回填）
- 交接/异常：（跑后回填）
