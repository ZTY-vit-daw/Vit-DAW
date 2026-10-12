# Mac 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md)（PC 侧同语义记录见 [PC-RUNTIME-STACK.md](PC-RUNTIME-STACK.md)）。
初始化"未核查"只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。
建立：2026-10-11 / GEN-MAC-BASELINE-1（GEN-CAP-RECON-1 §3.2 必补清单第 3 项：生成线 Mac 真栈运行前建立）。

## 当前占用

- 状态：空闲

## 最近一次释放

- 2026-10-12 10:21–18:36 +0800 / GEN-CTX-KEYFIELD-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-ctxdisclose1，分支 port/gen-ctx-keyfield-1）：`music_keyfield_roundtrip_smoke_mac.sh` 四轮（run1=候选内核段错误→var 所有权修复；run2=auto-persist 静默跳写定位+两个隔离取证内核；run3=内核修复确认+脚本探针缺陷；run4=权威轮 exit 0 43/43 门，脚本自 teardown；期间另有 3 个手动隔离取证内核实例，结束即杀）。18:36 复核：5555/5556/5557/7878 监听=0、VitApp/vitagent 进程=0。占用登记 a57640f0；实现 commit 13551ab0 @ port/gen-ctx-keyfield-1；run 工件 `coord/runs/GEN-CTX-KEYFIELD-1/run4-pass-20261012-183348/`。

- 2026-10-11 23:42–2026-10-12 09:40 +0800（跨夜会话）/ GEN-CTX-DISCLOSE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-ctxdisclose1，分支 port/gen-ctx-disclose-1）：`scripts/music_context_disclosure_smoke_mac.sh` 四轮（run1=内核设备管理器初始化挂起→环境中断重试；run2=agent 面探针脚本缺陷；run3=门断言脚本缺陷；run4=权威轮 exit 0 24/24 门绿，脚本自 teardown）。09:40 末轮后复核：7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0。占用登记 b48e4549（排队→占用见登记内注记）；实现 commit @ port/gen-ctx-disclose-1 dd08da4a；run 工件 `coord/runs/GEN-CTX-DISCLOSE-1/run1-pass-20261012-094027/`。
- 2026-10-11 22:33–23:25 +0800 / GEN-API-ADAPTER-1（GLM-5.3 Mac 执行流，ZCode；worktree /Users/timozty/Documents/Vit-DAW-genapi1，分支 port/gen-api-adapter-1）：`gen_api_adapter_smoke_mac.sh` 端到端冒测四轮（run1/run2/run3=探针迭代：内核子命令名映射+错误回包 HTTP 400 误判+模拟器证据 atexit+tags 断言与契约不符，均脚本/断言缺陷非栈失败；run4=权威轮 17/17 门 exit 0），内核复用 GEN-MAC-BASELINE-1 二进制 9a70f064（VitApp/Source 与该基线零 diff 已核）+agent 含本卡改动新构建。23:25 末轮后复核：7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0。占用登记 fa25ae47（登记前核对空闲，DISCLOSE-1 按协议排队于后）；run 工件 `coord/runs/GEN-API-ADAPTER-1/run4-pass-20261011-232432/`；实现 commit 1e23fa8f @ port/gen-api-adapter-1。
- 2026-10-11 22:10–22:40 +0800 / VIS-FILM-P0-ROUTE-ENABLE（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-visfilm-p0，分支 port/vis-film-p0）：vitagent（HTTP 7878，无内核/Godot，/health ok）+ webui vite dev（5178）+ 离屏 WKWebView 截图 + 读图探针一轮（读图调用成功，17.3s）。22:40 teardown 复核：7878/5178/5555/5556/5557 监听=0、vitagent/vite/VitApp 进程=0。占用登记 dd9ac1df（登记前核对空闲）；run 工件 `coord/runs/VIS-FILM-P0/run1-20261011-2215/`（实现 commit @ port/vis-film-p0 36d28dca）。
- 2026-10-11 11:45–12:34 +0800 / GEN-MAC-BASELINE-1（GLM-5.3 Mac 执行流，ZCode 会话；worktree /Users/timozty/Documents/Vit-DAW-genbase1，分支 port/gen-mac-baseline-1）：生成链勘测脚本 `scripts/gen_chain_survey_smoke_mac.sh` 四轮（run1/run2/run3=探针迭代，脚本缺陷非栈失败；run4=权威轮 exit 0，30/30 门绿，末轮脚本自 teardown），12:34 复核端口 7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0。占用登记 de3a3b6d（登记前核对空闲）；实现 commit 见 port/gen-mac-baseline-1；run 工件 `coord/runs/GEN-MAC-BASELINE-1/run4-pass-20261011-122948/`。期间另有一次裸内核 ZMQ 直发取证（11:47–11:53，fixture 排障，结束即杀，复核端口=0）。

## 历史占用（归档）

- 2026-10-12 10:18:19–10:18:22 +0800 / GEN-CTX-INJECT-1 真键冒测（Mac 决策侧会话兼执行）：`genctx_truekey_smoke_mac.sh` 单内核直连 ZMQ 一轮 PASS exit 0（占用前 7878/5555/5556/5557 空闲亲核；脚本自 teardown，末轮后无残留）；run 工件 coord/runs/GEN-CTX-INJECT-1/genctx_truekey_mac_20261012-101819-pass/。
- 2026-10-11 18:15–18:21 +0800 / GEN-MAC-BASELINE-1 验收复跑（Mac 决策侧会话，ZCode）：`gen_chain_survey_smoke_mac.sh` 独立复验一轮 exit 0 全门绿（复用同哈希双二进制，run workdir 独立临时目录，脚本自 teardown）；收尾 lsof 7878/5555/5556/5557 与 pgrep VitApp/vitagent 亲核零残留。
