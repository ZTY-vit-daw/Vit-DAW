# Ruling: REFSCHEMA-D1 — pass（四处族迁移+D2 正当挂起）（2026-09-28 决策侧）

## 裁定

**pass**。内核 D 类生成点 ref 迁移 L0 文法验收通过：D1/D3/D4/D5 四处族全迁（D5 实勘扩至 7 处含报告后新增 segmentation×3），**D2 三处按卡面停止条件正当挂起**（消费方硬编码旧格式）。实现合入 main（cherry-pick 2fe21671 → 1524cfb5）。

## 决策侧复跑与亲核

- **ctest 双配置决策侧独立复跑**：执行侧 worktree `ctest -C Release` 与 `-C Debug` 均 **10/10 全绿**（多配置生成器，含 VitRefSchemaTests/VitL3Evidence/VitSegmentationPrimitives/VitRenderWatchdog 复编译目标）。
- **D2 挂起证据亲核**：`com/evidence.go:150` 与 `com/paired.go:168` 均为 `dad.compressor_dual_tap:` 前缀**等值断言**（require EvidenceRef == prefix+PairID）——改内核即断 COM paired 消费，停止条件"消费方硬编码旧格式→挂起+清单上交"严格成立。三处生成点改引镜像常量+挂起注释（迁移后每处一行可解）。
- **实勘增益采信**：行号漂移逐个实取（D3 :465→:548 等）；发现 L1-1 报告**漏列的第三处 D2 生成点**（CompressorDualTapEvidence.cpp:238）；D5 由报告 4 处实勘至 7 处。
- **agent 零文件改动亲核**：实现提交 8 文件全部在 VitApp/（含 CMakeLists 与测试）——消费面零触碰与回执一致。
- 迁移样例对照（旧→新各一行，测试实测输出）在回执核验：`dad.l2_render_probe:render-1` → `vit://dad.l2_render_probe/track:1007/t=all@render-1#-` 等四族，格式与 agentprotocol L0 文法一致（kind/scope/window/snapshot/hash 段、`#-` 未 CAS 化 ruling 口径）。
- 红先行：stash 迁移→旧生成点构建→VitRefSchemaTests 9 FAIL（T8a-f/T9a-c）→恢复后全绿——红形态真实。
- 边界声明采信：主 VitApp 目标全链接构建未跑（改动文件已在 ctest 目标全量编译链接；主 CMake 仅文件列表追加）——**记为后续卡/真栈腿的验证项**，不阻塞本卡（卡面门=ctest 双配置+go 全量）。

## 遗留与后续

- **REFSCHEMA-D2 消费方升级卡已入池**（P2）：com 两处等值断言改 ParseRef 后校验（兼容 legacy 前缀与 vit:// 双形态）→ 内核三处一行切换 → ctest+go 双端门。D2 builder 语义已被 T5 黄金样例锁定，落地路径明确。
- 主 VitApp 全链接构建验证归入下次内核腿或真栈烟测轮。
- 执行侧 worktree 由决策侧清理（ctest 复跑完成后）。
