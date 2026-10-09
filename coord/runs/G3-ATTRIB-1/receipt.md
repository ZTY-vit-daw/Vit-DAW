# G3-ATTRIB-1 执行回执（归因卡①：行为等价+预算校准+冷启动供给）

- 卡：2026-10-09-G3-ATTRIB-1（doing→done 随本回执）
- 执行：GLM-5.3 执行会话（PC / ZCode），2026-10-09 晚窗
- 分支/worktree：port/g3-attrib-1 @ D:\Vit_DAW_wt_g3a1；领取基线 origin/main=7eb38fc5（领取提交 d6c4f060；并行卡 ATTRIB-2 领取后 rebase，实现提交 rebase 后 hash=c4c015c5）
- 领取时 git status：净（diff=0）；本卡新增 diff 仅四文件域：agent/internal/chat/server.go（+注入面 ~51 行）+ agent/internal/chat/pull_coldstart_supply_test.go（新，5 测试）+ scripts/dev_agent_smoke.ps1（harness_ab 四相扩展 ~456 行）+ coord/runs/G3-ATTRIB-1/（工件）。pull_session.go/loop.go/promptruntime/pull_entry.go 零改动（pull_entry.go 勘察确认无需改——预算与冷启动消费面已读 goal 上下文）。

## 真栈 run

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -Scenario harness_ab -HarnessAbRounds 1 -StartKernel -KernelExe D:/Vit_DAW/Export/staging/runtime/VitApp.exe -RunArtifactsDir coord/runs/G3-ATTRIB-1/20261009_214138_harnessab`
- **SCRIPT-LASTEXITCODE=0**；run ID=G3-ATTRIB-1/20261009_214138_harnessab；head=c4c015c5（rebase 后实现提交）；git_status.txt 在 run 目录
- 栈占用闭环：登记 82a654ad（占用前端口 7878/5555/5556+进程核对=0）→ 脚本 berth teardown（agent 23600/kernel 18636）→ 释放复核 0 监听 0 进程（PC-RUNTIME-STACK.md 已回填）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动，hash/time 已记录）
- §8 预写：场景头注释扩展（轮数/成功条件/失败分类/止损线四相分写）；两止损线均未触发

## 四目标交付

1. **确认往返真栈面 ✓**：`harness_ab_confirm_roundtrip.json`——J3 话术→确认卡（interaction 面/mix_tick 工作流，plan 面兜底同实现）→ approve → goal=completed stop=**mix_tick_applied_reobserved**（确认→执行→应用→重观察一站到达）；事件流 audition×4+mix_tick×2 佐证 A/B 判定站。
2. **多话术采样+等价性结论成文 ✓**：`../equivalence_stratification.json`+`../G3-ATTRIB-1-REPORT.md` §3——**站图等价、分布位移=话术条件化风格差异，非 pull 行为缺陷；停止条件未触发**。关键数据：push f3 出确认卡 / pull f2 直达 done（G3 首轮"push 恒完成/pull 恒确认"为单话术伪象）。
3. **预算校准 ✓**：`harness_ab_budget.json`——max_cycles=1 经 goal 上下文 → 真栈触发 observation_budget_exhausted；分记核对 accounted_separately=true（零 no_candidate 共现）。max_probe_cost 执法仍待计量源（如实披露）。
4. **冷启动供给面 ✓ rendered 实证**：server.go `contextWithPullEngineSnapshot`（四门控：push 零变化/会话首装/absent 显式/调用方优先；注入点=runAgentLoopChat 分支局部副本，直连管线字节不变）；`../coldstart_prefix_delta.json`——pull 首调 prefix 34,316–34,476 vs G3 absent 基线恒 34,027（**+289/+449**）+ agent 日志 6×注入行 = 三事实组字节进稳定前缀；单测 5 门绿。

## 验收门

- go build ./...：0
- go test ./... -count=1：**92 包 0 FAIL**（final：`gotest_full_final.log` EXIT=0；预跑 `gotest_full_prerun.log` 同）
- gofmt：server.go/测试文件 LF 归一化口径净（仓库既有 CRLF 检出状态与主 worktree 同基线，未新增不净文件）
- 真栈：SCRIPT-LASTEXITCODE=0（上）；工件四类齐（confirm/budget/stratification/coldstart delta）

## 异常与登记（主管裁定面）

1. **pull base r1 边界持久化失败（单次，已分记）**：durable_checkpoint_persist_failed——内部链 completed/done（[continuation.arm] 日志）后响应边界降级；同 run 后续 2 个 completed pull goal 同路径成功=瞬态（锁竞争/写入路径候选）。非判定链缺陷、非本卡域；建议另立小卡取证（两模式共用降级路径）。**未触发止损线**（分类 judgment_failed 非 infra_error_*）。
2. VSP 8787 拒连 WARN 全程两模式同载（berth 无该服务），不入失败类。
3. 续跑轮 context merge 的冷启动快照刷新语义（引擎态变→如实断裂）：登记给归因卡②拆分修复一并考虑。
4. 等价性口径限定：站图等价≠质量等价（judgment_ok 两模式均未产出）≠成本等价（llm_calls 17 vs 6 同向 G3 首轮）。

## 覆盖边界（显式声明）

- 真栈面：工程打开→权限→双模 LLM 实验链→（pull）提案确认卡→**用户确认→执行→A/B 重观察→completed**→预算止损站。确认往返经 interaction 面（mix_tick 工作流卡）行使；plan 面（agentloop confirmation）结构性同驱动但本轮未命中（模型挂的是 interaction 卡）。
- 每（族×模式）1 轮（卡面 ≥3 族×2 模式满足）；倾向比例是方向性读数非统计估计。
- 渲染面零改动；chat 直连管线未迁移（OQ-H3 不变）。

## 复核建议

独立复核腿强制（G3 面判定链语义）：建议按 L1-5-IMPL-A-REVIEW-1 范式发复核卡（或本侧 blocked 上交）——复核面=等价性结论读数（equivalence_stratification+coldstart_prefix_delta 两工件对原始遥测复算）+注入面四门控 diff 亲读+预算分记核对。
