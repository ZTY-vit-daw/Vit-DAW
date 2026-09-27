# FIX-STALE-SAMPLES-1：E8/E16 两处漏标活样本修复（现状锁定测试升级为修复验证）

- 优先级 / 预估 / 依赖：P1 / 0.4 天 / MAT-C 已入现状锁定测试（E8：rlm mergeSourceRows 字段级覆盖无 revision 比较 rlm/projection.go:170-192；E16：B1 pack 同 run 建后不失效 static_mix_gain_staging_context.go:21-23）；设计裁定"修复单开卡"（MATERIALIZATION §4 rlm 段）
- 模型分级：L2 / GLM 亲自或强督导（新鲜度语义修复，行为变化须钉死）
- **执行侧（mac 会话，夜池）**
- 目标：
  1. **E8 修复**：`mergeSourceRows` 合并加 revision 比较（三源 executed 记忆的新鲜度门——旧 revision 不得覆盖新 revision 的字段级合并）；锁定测试的升级断言启用（MAT-C 测试内已注释升级条件：修复后行为红→绿）。
  2. **E16 修复**：static_mix_gain_staging_context 的 pack 行在同 run 重建后失效（同 run 建后不失效=陈旧 pack 误用）；同样用锁定测试升级验证。
  3. **行为边界钉死**：两处修复都是"拒绝陈旧数据覆盖新数据"方向（宁缺勿旧），不做其他行为扩展；旧记录兼容（§11：无 revision 字段的旧记录=维持现状语义还是视为最旧？卡内裁定并写回执）。
  4. **门**：agent 全量 0 FAIL+两锁定测试升级后绿。
  5. 回执：红绿+diff stat+§11 裁定说明+HEAD。
- 约束：文件域 rlm+staticbalance（两文件族）；行为变化范围最小（只加新鲜度门）；提交显式列文件。
- 验收：两测试升级绿+全量 0 FAIL+§11 裁定在案
- 停止条件：revision 语义与消费方冲突（修复导致现役行为回退）→ 冲突实证上交
