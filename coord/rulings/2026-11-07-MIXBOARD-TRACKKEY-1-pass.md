# Ruling 2026-11-07 MIXBOARD-TRACKKEY-1：pass

- 验收：GLM 主管决策流 / 2026-11-07 早窗
- 实现 commit：b7a9965e @ port/mixboard-trackkey-1（cherry-pick 合入 main，hash 见回执）

## 验收结论

1. **键路由修 pass**：default 分支新增 track-keyed stereo 双面（slow+realtime）TargetRef 语义路由——读取键解析到观察包已携带的逐轨行集（三级行集解析：内联 snapshot→evidenceSnapshot 参数→FeatureSnapshotEvidence 回溯）；**铸造面零改动红线合规**（只修读取侧）；严格同轨身份绑定（track_id EqualFold 相等才披露，位移/无绑定行不披露）；无行轨诚实逐轨 missing（`stereo_relation_row_missing_for_track`）。
2. **单测反钉 pass**：265 实盘形状八组——正锚 6 轨真实 id 键 ready（1007 corr 0.689/balance -1.475 实盘 round3 值钉死）+负锚 1039 无行轨（reason≠catalog_key_not_found 反钉修复前恒 miss 形状）+未知键六形不误吞+自身目标键（track.current.*）回归锚语义不变。
3. **交叉复跑 pass**：主管亲跑 mixboard（ok 3.8s）+mom（ok 0.6s）；chat 全包执行侧绿在卷（148.3s，域不相交）。
4. **实盘形状亲核修正两点**（执行侧，采信）：19 行 ready 实为 6 轨（1039 无行）；digest.stereo_status=ready——reverify1「全局缺席」系 272 顶层读取 bug 所致，两案已分清。
5. **勘察四处同型候选=记录交付合格**：①`tools/capability_pack.go:475` 提示词宣传 `track.{id}.slow.time_energy.summary` 为稳定键但 full_project 恒 miss（**域外 tools/——挂 PD-5**：提示词键宣传与键面真实性对账，另发小卡）；②`catalog.go:1182-1185` relationshipEvidenceRefs 发射 `track.<id>.fast.levels` ref 同型 miss（同包不同消费面，记录）；③band_energy 面/④static.identity 面同型休眠（未发现 chat 消费方）；另记 chat 侧 `d1d3_premaster_observation_runtime.go:261-266` 对新诚实 missing 的 read/missing 分类细微差（265 D2 门后续 run 解读口径）。
6. **端测边界（AGENTS §5）如实**：零真栈=单测+build 级；真栈回归门=265 D2 门组断言（预期 6/7 轨可读+1039 诚实 missing+verdict 转入三态可测分支，取值 fixture-dependent 不预断言）搭后续 run。

## 账务

- 零 LLM 零真栈，无计费。本裁定入 L1 台账，48h 翻案窗口。
