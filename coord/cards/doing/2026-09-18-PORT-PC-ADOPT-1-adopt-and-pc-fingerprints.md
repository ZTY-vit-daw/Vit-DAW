# PORT-PC-ADOPT-1：孤儿修复收编 commit + PC 参数面指纹生产（闭 C3 双端对照）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 依赖 C3 已验收（双平台代码在 main，PluginProbe win32 面原样保留）
- 模型分级：L2 / GLM-5.3（指纹语义对照 + 收编纪律）
- 目标：三段——
  - **A（收编 commit）**：将主工作树孤儿 diff `agent/cmd/vitagent/main.go`（required 注册有界重试，10+/3-，归属 2026-09-17 晚 `sess_655bbfcf` v0.93 打班会话的"安装包启动卡死"修复——C2-PCR ruling 裁定收编）正式提交：分支 `port/pc-adopt-1` 单 commit，message 注明归属溯源与功能语义；**提交前逐行核对 diff 与 C2-PCR 回执所记一致**（requiredRegistrationAttempts=30×2s 有界重试），零夹带
  - **B（PC 指纹生产）**：用 C3 双平台代码在 PC 产参数面指纹：PluginProbe native-host MSVC 构建（win32 面应原样编译——C3 构造级等价论证的实证点）+ reprobe driver 对 12 族（Mono/Stereo 共享同面，24 体任选代表或全跑）探针，产出 `pc_fingerprints.json`；driver（`PluginProbe/reprobe/reprobe_parameter_surface.sh`）如需 Git Bash 适配 → 域内最小 uname 守卫（对齐 C2-PCR 脚本模式），列明
  - **C（双端对照收口）**：与 transfer 到达件 `inbox-mac/2026-09-18-mac_fingerprints.json`（mac 权威 run 20260918-202010 的 24 主体指纹）逐族对照，产出对照终稿（12 族逐族 verdict=identical/different，different 附参数面 diff 摘要），**闭 C3 验收②的 pending_pc_reference**；终稿判定"参数元数据哈希跨平台稳定"预期成立与否
- 文件域：A=`agent/cmd/vitagent/main.go`（仅收编既有 diff）；B=`PluginProbe/`（driver 平台适配如需）；C=工件落 PC 本地目录，对照终稿 JSON 投 transfer 仓 `inbox-pc/`（带日期）回传 mac
- 验收标准：A：分支单 commit、diff 逐行=孤儿 diff、message 含溯源；B：MSVC 构建 exit 0 + 12 族（或 24 体）指纹 JSON + run ID/各段日志；C：12 族对照表全有 verdict + 终稿已投 transfer
- 停止条件：MSVC 编译 win32 面失败（C3 等价论证失效）→ 附原始错误转 blocked；不一致族 >1/3 → 停止上交对照证据由决策侧裁定口径；孤儿 diff 在开工前已被改动/消失 → 停止上报
- 领取：2026-09-18 21:20 CST / origin/main dddf6455（领取时 HEAD=dddf6455，工作树仅 M agent/cmd/vitagent/main.go 即 A 段收编对象，diff 已逐行核对与 C2-PCR 回执一致 10+/3-）/ 分支 port/pc-adopt-1（独立 worktree D:/Vit_DAW_worktrees/pc-adopt-1）
- 回执：
- 验收：
