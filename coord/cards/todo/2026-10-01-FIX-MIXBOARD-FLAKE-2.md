# FIX-MIXBOARD-FLAKE-2：TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork 二现——负载敏感 flake 修复卡（09-30 条款触发）

- 池序 21（P3 测试卫生；2026-10-01 FS-PARK-TURNFAIL-1 验收期间我方独立复跑二现触发 09-30 VITNOTE-IMPL-4 ruling 首录条款"重复出现即开修复卡"）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无；agent/internal/harness 域（与池内其它卡不同域可并行）
- 模型分级：L1 / GLM 或 flash 可接
- 已核实事实（两次记录在案）：
  1. **首录**（2026-09-30，VITNOTE-IMPL-4 ruling）：harness `TestMixboardSnapshotConcurrentDualWrite`（5s 截止超时）——隔离复跑 3/3 绿+失败路径零能力执行关联+主基线全量绿，归因负载敏感 flake，"首次记录，重复出现即开修复卡"。
  2. **二现**（2026-10-01，FS-PARK-TURNFAIL-1 验收）：`TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork`（internal/harness，5.02s 截止超时）在决策侧全量复跑失败——隔离复跑 3/3 绿+整包复跑绿+当卡 diff 零文件交集+失败类型与首录同（5s 超时）→ 同族 flake（若系改名/拆分，领取时核对测试名演变并回写本卡）。
  3. 形态：全量并发跑（87 包并行）时偶发超截止；单测/单包稳定绿——典型负载敏感时序窗。
- 目标：
  1. 取证：定位超时窗内该测试的实际等待面（channel/mutex 时序化形态 vs 真竞态泄漏）——区分"断言正确但预算紧"与"偶发死锁边界"。
  2. 修复（按定性）：预算窗放宽（如 5s→15s，须论证非常态路径放行）/时序化（受控 channel 编排消除负载敏感窗）/两者组合；**不得弱化断言本体**（NoUnannotatedFork 的双写无未注记分叉语义保持）。
  3. 验证：修复后全量并发跑 ≥3 轮零复现（§8 概率性运行纪律：轮数与成功条件执行前写明）；隔离/单包各 3 轮绿。
- 文件域：agent/internal/harness/（该测试与其被测并发面，实锚后申报；预期 ≤2 文件）。
- 验收标准：全量 0 FAIL×3 轮+断言语义零弱化 diff 审查+定性结论回写本卡。
- 停止条件：取证发现非负载敏感（真竞态缺陷在生产路径）→ 升级 P1 上交另立卡。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 定性 / 三轮全量结果）
- 验收：（裁定文件 / 验收 commit）
