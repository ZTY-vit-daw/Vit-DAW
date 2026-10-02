# Ruling：FIX-MIXBOARD-FLAKE-2 — pass（2026-10-02 决策会话；附回执注记）

## 验收四层

1. **diff 直读**（8c283727，bridge_freshness_annotation_test.go 单文件 25+/16-）：
   - 完成检测改事件驱动：写者收尾 `close(done)`，读者 select 观察关闭态（channel 置 nil+pending 计数，两写者各一）；**断言本体零改动**（轮询采样与未注记分叉检查循环原样，reads==0 守卫原样）；看门狗 5s→15s 只防真挂死（生产写路径包级互斥时序化，挂死即真缺陷——预算放宽不构成放行：分叉回归任何一次采样即红，与预算无关，论证成立）；`wgDone` 删除（全仓 harness 域唯一使用点，diff 内可见）；t.Logf 完成耗时/采样数可观测性增益。
2. **定性采信**：spawn-then-select 投票原语要求新协程在微秒窗内被调度跑完，满载（87 包饱和 CPU）系统性输给 default 分支——满载取证对照强（修复前 0/5、写者 0.9-1.07s 完成而读者空转到 5.02s 截止；修复后同条件 5/5）；静态分析（包级互斥时序化写写、读者无锁不阻塞——无死锁环）排除生产竞态，**停止条件正确未触发**。测试名 lineage 澄清（首录 `TestMixboardSnapshotConcurrentDualWrite` 系记录简写、非改名/拆分）已回写卡面。
3. **我方独立复跑**（worktree 内，全部真跑）：定向 `-count=3` 3/3 PASS（475-491ms，reads=80，可观测性生效）；harness 包 `-count=1` PASS 16.4s；**全仓 87 包 `-count=1` 真跑零 FAIL、零 "did not finish"**。
4. **回执注记（如实修正，不构成返工）**：回执"全量 87 包 ×3 轮"中 **R2/R3 为缓存轮**（日志 87/87 行 `(cached)`，未加 `-count=1` 致 go test 缓存复用）——缓存轮不重执行测试、不能证明零复现，实际真跑 1 轮（R1）；连同我方真跑共 2 轮全绿。§8 轮数纪律：**缓存不计入轮数**，后续回执轮数声明须带 `-count=1`。修复本身经我方独立真跑验证充分。

## 裁定

- **pass**；cherry-pick 8c283727 合 main（验收 commit 见卡内）；执行侧 worktree（D:\Vit_DAW_worktrees\fix-mixboard-flake-2）验收后清理（§3 协议）。
- 域外既有发现（go vet shm_windows.go:45 unsafe.Pointer 提示、gofmt CRLF 整仓状态）不属本卡，登观察不入池（既有状态非本卡引入）。
