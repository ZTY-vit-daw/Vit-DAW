# Mac 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md)（PC 侧同语义记录见 [PC-RUNTIME-STACK.md](PC-RUNTIME-STACK.md)）。
初始化"未核查"只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。
建立：2026-10-11 / GEN-MAC-BASELINE-1（GEN-CAP-RECON-1 §3.2 必补清单第 3 项：生成线 Mac 真栈运行前建立）。

## 当前占用

- 2026-10-11 22:0x +0800 起 / GEN-API-ADAPTER-1（GLM-5.3 Mac 执行流，ZCode；worktree /Users/timozty/Documents/Vit-DAW-genapi1，分支 port/gen-api-adapter-1）：gen_api_adapter_smoke_mac.sh 端到端冒测（内核复用 GEN-MAC-BASELINE-1 二进制 9a70f064…，agent 含本卡改动新构建；预计短时占用，结束复核端口零残留）
- 2026-10-11 23:43 +0800 起 / GEN-CTX-DISCLOSE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-ctxdisclose1，分支 port/gen-ctx-disclose-1）：`scripts/music_context_disclosure_smoke_mac.sh` 两阶段冒测（改前基线内核 9a70f064 + 候选内核 8e445dbb + agent 两件套，固定端口 7878/5555/5556/5557，run workdir 隔离临时目录，脚本自 teardown）。排队→占用判定：GEN-API-ADAPTER-1 栈 23:15 在跑（pid 38890/38925 亲见），23:20-23:41 三轮复核四端口监听=0、VitApp/vitagent 进程=0，其释放登记未落（疑似网络/写回执中）；脚本两级端口预检保单占互斥。

## 最近一次释放

- 2026-10-11 22:10–22:40 +0800 / VIS-FILM-P0-ROUTE-ENABLE（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-visfilm-p0，分支 port/vis-film-p0）：vitagent（HTTP 7878，无内核/Godot，/health ok）+ webui vite dev（5178）+ 离屏 WKWebView 截图 + 读图探针一轮（读图调用成功，17.3s）。22:40 teardown 复核：7878/5178/5555/5556/5557 监听=0、vitagent/vite/VitApp 进程=0。占用登记 dd9ac1df（登记前核对空闲）；run 工件 `coord/runs/VIS-FILM-P0/run1-20261011-2215/`（实现 commit @ port/vis-film-p0 36d28dca）。
- 2026-10-11 11:45–12:34 +0800 / GEN-MAC-BASELINE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-genbase1，分支 port/gen-mac-baseline-1）：生成链勘测脚本 `scripts/gen_chain_survey_smoke_mac.sh` 四轮（run1/run2/run3=探针迭代，脚本缺陷非栈失败；run4=权威轮 exit 0，30/30 门绿，末轮脚本自 teardown），12:34 复核端口 7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0。占用登记 de3a3b6d（登记前核对空闲）；实现 commit 见 port/gen-mac-baseline-1；run 工件 `coord/runs/GEN-MAC-BASELINE-1/run4-pass-20261011-122948/`。期间另有一次裸内核 ZMQ 直发取证（11:47–11:53，fixture 排障，结束即杀，复核端口=0）。

## 历史占用（归档）

- 2026-10-11 18:15–18:21 +0800 / GEN-MAC-BASELINE-1 验收复跑（Mac 决策侧会话，ZCode）：`gen_chain_survey_smoke_mac.sh` 独立复验一轮 exit 0 全门绿（复用同哈希双二进制，run workdir 独立临时目录，脚本自 teardown）；收尾 lsof 7878/5555/5556/5557 与 pgrep VitApp/vitagent 亲核零残留。
