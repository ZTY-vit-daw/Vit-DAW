# Mac 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md)（PC 侧同语义记录见 [PC-RUNTIME-STACK.md](PC-RUNTIME-STACK.md)）。
初始化"未核查"只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。
建立：2026-10-11 / GEN-MAC-BASELINE-1（GEN-CAP-RECON-1 §3.2 必补清单第 3 项：生成线 Mac 真栈运行前建立）。

## 当前占用

- 状态：占用中
- 2026-10-11 11:45 +0800 起 / GEN-MAC-BASELINE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-genbase1，分支 port/gen-mac-baseline-1）：生成链五命令勘测脚本 `scripts/gen_chain_survey_smoke_mac.sh` 真栈烟测（agent 7878 / kernel ZMQ 5555/5556/5557）；占用登记前核对端口监听=0、VitApp/VitAgent 进程=0（PC 栈 MIXLAYER-M1-1 占用为 PC 机器侧，与 Mac 栈互不相干）。

## 最近一次释放

-（尚无——首次建立）

## 历史占用（归档）

-（尚无）
