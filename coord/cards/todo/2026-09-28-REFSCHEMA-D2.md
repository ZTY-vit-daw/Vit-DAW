# REFSCHEMA-D2：COM paired 消费方升级+D2 三处切换迁移（G1 ruling D 类收官腿）

- 优先级 / 预估 / 依赖：P2 / 0.3 天 / REFSCHEMA-D1 验收（rulings/2026-09-28-REFSCHEMA-D1-pass.md）：D2 三处生成点挂起，消费方硬编码等值断言待升级
- 模型分级：L1 / flash 可接（锚点明确的双端小卡）
- 目标：
  1. **消费方升级（红先行）**：`agent/internal/com/evidence.go:150`（ready 回执校验）与 `paired.go:168`（artifact 校验）的 `dad.compressor_dual_tap:` 前缀等值断言 → 改 `agentprotocol.ParseRef` 后校验（kind==dad.compressor_dual_tap 且 legacy 形态 snapshot==PairID、或 vit:// 形态 scope/snapshot 承载 PairID——两种形态同帧兼容，宽限期语义 §11）。测试：legacy 旧格式与 vit:// 新格式双形态均过、错 kind/错 PairID 仍拒。
  2. **内核三处切换**：`VitProductionCoordinator.cpp:526/:953` + `CompressorDualTapEvidence.cpp:238` 由镜像常量 legacy 前缀切 `makeCompressorDualTapRef`（D1 已落地的 builder 家族；D2 builder 语义已被 VitRefSchemaTests T5 黄金样例锁定：`vit://dad.compressor_dual_tap/track:trk_9/t=0..44100@pair_2f3e#-`）。
  3. **门**：ctest 双配置全绿（`VitApp/cmake-build-*` 多配置生成器，-C Release 与 -C Debug）+ `cd agent && go test ./... -count=1` 全量 0 FAIL。
  4. 回执：消费方双形态测试+三处切换样例对照（旧→新各一行）+双端门+HEAD。
- 文件域：`agent/internal/com`（两文件+测试）、VitApp 三生成点、VitApp/Tests（若需样例更新）；越域即停。
- 约束：宽限期内 legacy 解析路径保留；agentprotocol 零改动（ParseRef 现成）；提交显式列文件+推 port/refschema-d2+coord 直推 main+切回 main。
- 停止条件：ParseRef 对该 kind 的 legacy 形态不满足 snapshot==PairID 校验所需信息 → 实证上交
