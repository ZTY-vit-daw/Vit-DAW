# Ruling：PORT-PC-ADOPT-1 pass（2026-09-18）

- 卡：`coord/cards/done/2026-09-18-PORT-PC-ADOPT-1-adopt-and-pc-fingerprints.md`
- 实现：`port/pc-adopt-1` 双 commit——`b502e362`（收编 main.go）+ `85e69695`（driver Git Bash 适配 141+/35-）→ 均 cherry-pick 入 main
- 裁定：**pass；C3 验收② 的 pending_pc_reference 就此闭合**

## 三段核验

1. **A（收编）**：`b502e362` diff 逐行核对=有界重试语义（`required && attempt >= requiredRegistrationAttempts` 判定、30×2s 常量、Error/Warn 增强、冷启动注释），message 含归属溯源（sess_655bbfcf v0.93 打包会话）——与 C2-PCR 考古记录一致零夹带。**决策侧独立复验**：将该 main.go 置于本仓执行 `GOOS=windows GOARCH=amd64 go build ./cmd/vitagent` 与 darwin/arm64 双双 exit 0。主工作树孤儿 diff 全程未动、经 autostash 往返复核不变——收编纪律满分。v0.93 打包会话的孤儿欠账正式闭合。
2. **B（PC 指纹生产）**：MSVC worker+witness 双目标 exit 0 零 error——**C3 的 win32 面构造级等价论证就此从"论证"变"实证"**；driver 适配（141+/35-）为域内 uname 守卫（工具面/路径形态/netstat/产物名/R4 win 臂环境事实记录/darwin 臂 token 保持），对齐 C2-PCR 模式；权威 run `20260918-213539` 24/24 PASS、installation 一致 22/22、语义索引只读保证。
3. **C（双端对照）**：**决策侧独立复核**——transfer 到达件 `pc_fingerprints.json` 与 mac 本地 C3 权威 run 指纹直接比对：**主体级 24/24 逐字节一致，族级全 MATCH**（六语义族/十二 Waves 族）。终稿 `cross_platform_parameter_metadata_hash_stable=true` 采信。**"参数元数据哈希跨平台稳定"预期（PORT_AUDIT §1.4）正式成立**——mac 与 Windows 对同版本 Waves 主体暴露相同参数面，校准面跨端可移植的最后一块证据落位。

## 纪律记录

PROTOCOL §3 推送纪律增补后首个 PC 收口：rebase 后 coord-only diff 核对再推 main——新纪律生效首例，执行正确。

## transfer 归档

`inbox-mac/2026-09-18-mac_fingerprints.json`（PC 已消费）与 `inbox-pc/` 两份终稿（本侧已消费复核）随本 ruling 移入 `received/`。
