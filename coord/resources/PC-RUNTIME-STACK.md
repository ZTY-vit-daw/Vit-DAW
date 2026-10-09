# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 当前占用

- 状态：空闲
- 最近占用：2026-10-09 21:41–21:47 +0800 / G3-ATTRIB-1（GLM-5.3 执行会话，D:\Vit_DAW_wt_g3a1）：harness_ab 四相 run G3-ATTRIB-1/20261009_214138_harnessab SCRIPT-LASTEXITCODE=0；释放证据=脚本 berth teardown（agent pid=23600 / kernel pid=18636 已停）+ 21:52 复核端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0；期间无脚本缺陷重跑、无栈残留。登记提交 82a654ad（被测 HEAD c4c015c5，rebase 后实现提交 hash 以分支为准）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

