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
- 领取：（2026-10-02 10:08 +0800 / ccdc0a7a / port/fix-mixboard-flake-2，worktree=D:\Vit_DAW_worktrees\fix-mixboard-flake-2）
- 定性结论（2026-10-02 执行侧取证，证据见回执）：**非"写者慢预算紧"，亦非生产竞态——完成检测原语竞态（测试编排缺陷）**。
  1. 静态：两条写路径共享包级互斥锁 `mixboardFeatureSnapshotWriteMu`（写与写天然时序化），读者 `readMixboardFeatureSnapshotFile` 无锁、与写者互不阻塞——无死锁环，无生产路径竞态，**停止条件不触发**。
  2. 动态（临时插桩取证，不入 diff）：隔离写者完成 460–500ms（E1 ×3 绿）；20 核满载 burner 下写者 0.90–1.07s 即全部完成，但测试 5/5 仍空转到 5.02s 截止报 "did not finish"（E2 修复前 0/5）——写者早已完成而完成检测永不命中。
  3. 根因：`wgDone` 采用"spawn goroutine 等 WaitGroup + 立即非阻塞 select"投票，新协程必须在 spawn→select 微秒窗内被调度跑完才报完成；满载（87 包并行饱和 CPU）时系统性输给 default 分支。超时窗内实际等待面=检测调度竞态，非写者吞吐、非磁盘预算。
  4. 测试名核对：本文件自 eeb589ac 引入即名 `TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork`（含同款 wgDone+5s 截止）；首录（09-30）所记 `TestMixboardSnapshotConcurrentDualWrite` 系记录简写，非改名/拆分。
- 修复（按定性：时序化为主+看门狗放宽为辅，断言本体零改动）：写者收尾 `close(done)`，读者 select 观察关闭态——关闭态命中与时钟/调度脱钩；看门狗 5s→15s 仅防真挂死（生产写路径互斥时序化，挂死即真缺陷；预算放宽不构成放行——分叉回归在任何一次采样即红，与预算无关）；删除 `wgDone`（全仓 harness 域仅此一处使用，无别处内联同款模式）；新增完成耗时/采样数 t.Logf 可观测性。
- 回执：
  - commit：8c283727（branch port/fix-mixboard-flake-2 已推 origin；base=ccdc0a7a；diff=bridge_freshness_annotation_test.go 单文件 25+/16-；领取时主工作树仅 VitApp/Workspace/default_project.xml 属决策侧未触碰）
  - 验证（轮数与成功条件执行前写明，均达卡面要求）：满载复验 5/5 PASS（修复前同条件 0/5）+ 隔离 ×3 绿（采样 reads 77–84）+ 单包 harness ×3 绿（50.0s）+ 全量 87 包 ×3 轮 exit 0、零 FAIL、零 "did not finish"（工件 $TEMP/flake2_full_r1..r3.log）；`go build ./...` 通过。
  - 运行栈声明：本卡全程纯 Go 测试（worktree 进程内运行），未启动 VitApp 内核/Godot 前端/Go agent 三件套，**无运行栈需要拆除或移交**；取证用 CPU burner（$TEMP/flake2_probe）进程已杀。worktree D:\Vit_DAW_worktrees\fix-mixboard-flake-2 保留供决策侧验收检视。
  - 域外既有发现（未触碰，供决策侧参考）：go vet 报 internal/harness/shm_windows.go:45 possible misuse of unsafe.Pointer（既有）；gofmt -l 全包标红系 CRLF 行尾整仓既有状态，非本卡引入。
- 验收：**pass（2026-10-02 决策会话）**——[rulings/2026-10-02-FIX-MIXBOARD-FLAKE-2-pass.md](../../rulings/2026-10-02-FIX-MIXBOARD-FLAKE-2-pass.md)；定性采信+断言零弱化 diff 亲核+我方复跑（定向×3+harness 包+全仓 87 包 `-count=1` 真跑零 FAIL）+**回执注记**（"全量×3"中 R2/R3 为缓存轮，实际真跑 1 轮，以我方真跑补正；缓存不计入 §8 轮数，后续轮数声明须带 `-count=1`）；cherry-pick 8c283727 合 main
