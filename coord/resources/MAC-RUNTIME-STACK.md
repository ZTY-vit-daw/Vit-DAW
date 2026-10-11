# Mac 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md)（PC 侧同语义记录见 [PC-RUNTIME-STACK.md](PC-RUNTIME-STACK.md)）。
初始化"未核查"只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。
建立：2026-10-11 / GEN-MAC-BASELINE-1（GEN-CAP-RECON-1 §3.2 必补清单第 3 项：生成线 Mac 真栈运行前建立）。

## 当前占用

- 状态：空闲

## 最近一次释放

- 2026-10-11 11:45–12:34 +0800 / GEN-MAC-BASELINE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-genbase1，分支 port/gen-mac-baseline-1）：生成链勘测脚本 `scripts/gen_chain_survey_smoke_mac.sh` 四轮（run1/run2/run3=探针迭代，脚本缺陷非栈失败；run4=权威轮 exit 0，30/30 门绿，末轮脚本自 teardown），12:34 复核端口 7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0。占用登记 de3a3b6d（登记前核对空闲）；实现 commit 见 port/gen-mac-baseline-1；run 工件 `coord/runs/GEN-MAC-BASELINE-1/run4-pass-20261011-122948/`。期间另有一次裸内核 ZMQ 直发取证（11:47–11:53，fixture 排障，结束即杀，复核端口=0）。

## 历史占用（归档）

- 2026-10-11 18:15–18:21 +0800 / GEN-MAC-BASELINE-1 验收复跑（Mac 决策侧会话，ZCode）：`gen_chain_survey_smoke_mac.sh` 独立复验一轮 exit 0 全门绿（复用同哈希双二进制，run workdir 独立临时目录，脚本自 teardown）；收尾 lsof 7878/5555/5556/5557 与 pgrep VitApp/vitagent 亲核零残留。
