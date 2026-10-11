# 性能影响结论（probeSpent 账本证据状态 + 明确实验设计）

LOUDNESS-WATCH-DESIGN-1 设计件 5/6（编号让位于拆卡件 05 的可发卡性，本件为卡面验收单独点名的性能结论）。

## 1. 现有账本证据：机制已接线，物理数字未出（如实声明）

- **计量机制就位**（PULL-PROBE-METER-1 已合入）：probe 级工具回执按 `elapsed_ms` 墙钟逐笔入账 `s.ledger.probeSpent`（`agent/internal/agentloop/pull_session.go:459`，语义注释 `:440-446`：index 级零成本不计，render/probe 级逐笔计入）；账本位进 checkpoint（`agentloop/pull_continuation.go:34` ProbeSpent）与预算披露行（`agent/internal/pullharness/budget.go:67-85` `probe_spent=<spent>/<max>`）。
- **分级表现状**：`pull_session.go:486-495` `probeToolTier`——`mix.observe`/`ccb.observation.request` = probe 级（计量），`ref.query`/`project.state` 等 = index 级（零成本），未列名 = unknown（不私定，`pull_session.go:450-451` 注释点名 `clip.warm_waveform_bake` 属两属性工具 unlisted）。
- **物理成本对比轮未跑**：`coord/cards/todo/2026-10-11-G3-REOPEN-COSTCMP-1.md`（池序 73）仍在 todo——push vs pull 双模式 probe 物理计量对比没有 run 工件可引。**因此本设计不给任何"probe 实测耗时 X ms"式数字**，只给机制结论与实验设计（下 §3）。

## 2. 设计侧成本结论（不依赖未出数据即可钉死的部分）

1. **主案（处理后触发）的边际成本下界为零**：T2 搭车路径纯消费已 bake 的 loudness_summary 行（设计① §2.2），比对是纯函数；T1 settle 路径仅在特征陈旧时补 bake，且 bake 本来就是后台 fill 语义（`harness.go:4279` 背景面）——**watch 不挂 ready-gate 同步路径，不增加任何回执等待**（这是设计决定，不是实测结论）。
2. **LLM 调用增量为零**：watch 全链路是机械（钩子→比对→披露），无模型调用；dedupe 键（设计① §2.4）防止同版本重复披露消耗上下文。
3. **周期模式的成本是线性的且默认关**：interval≥60s 校验下限（设计② §2.3）；周期 bake 在 pull 会话内按 probe 分级入账，可被 `MaxProbeCost` 止损线管住（`budget.go:78-80` ProbeExhausted）。
4. **bake 风暴防护**：单次触发 bake 上限 ≤4 clip（设计① §4），大工程 settle 不放大 bake 队列。
5. **计量前提**：watch 关联的 bake 计量要求 `warm_waveform_bake` 进入分级表——依赖 `coord/cards/todo/2026-10-10-PULL-PROBE-TIER-EXT-1.md`（池序 52）先行；未合入前 watch 的 probeSpent 贡献按 unknown 处置（`probeCostKnown=false`，诚实不填 0，`pull_session.go:452-455` 语义）。

## 3. 实验设计（AGENTS §8 全项，供实施卡 B/C 与 G3-REOPEN 基建复用）

### E1：主案搭车成本（挂实施卡 B 真栈烟测顺带产出）

- **运行次数与样本**：同一烟测工程连续 ≥3 次 settle 触发 watch 的 run（确定性钩子，非概率路径，3 样本满足"重复成功比例"最低口径）。
- **测什么**：每次触发新增墙钟 = 分级表就位后账本中 `tool=warm_waveform_bake`（watch 补 bake 部分）逐笔 elapsed_ms 之和 + watch 比对函数自身耗时（Go 侧埋点一次性计时，微秒级预期）；同时记录"复用新鲜行未发 bake"的次数占比。
- **成功条件**：①搭车路径（T2）新增 bake=0（纯复用）；②T1 补 bake 单次触发总墙钟中位数 ≤ 单 clip bake 基线×补 bake 数；③回执/settle 响应延迟增量 = 0（watch 后台化断言：settle 回执时间戳先于 verdict 时间戳）。
- **失败分类**：环境中断（栈起不来/端口占用——按 §9 记录后重跑）；断言失败（回执先于 verdict 不成立=watch 误入同步路径——实现缺陷，返工）；计量缺失（tier 表未扩=依赖未满足，不算失败，改判阻塞）。
- **止损线**：连续 2 次同断点失败即停，按 AGENTS §8 禁止原样重跑，转取证。

### E2：周期模式成本（挂实施卡 C 或独立小轮，周期模式默认关不阻塞交付）

- **样本**：interval=300s × 30min 会话 × 2 run。
- **测什么**：probeSpent 账本增量（tier 表就位后）、bake 次数、`budget_state` 披露行对比（`budget.go:67-85`）。
- **成功条件**：30min/6 次触发仅对陈旧 clip 发 bake（新鲜复用为主）；probeSpent 增量可被 MaxProbeCost 覆盖。
- **失败分类与止损**：同 E1 句式。

### E3：render 级判定成本（内核缺口卡落地后才可设计，本轮仅占位）

- 母线/render 测量路径不存在（设计④缺口 5），任何"render+分析成本"结论在缺口卡落地前**不可测也不可承诺**——列为 KERNEL-LOUDNESS-BS1770-1 及母线测量卡的验收项，不在 watch 卡内跑。

## 4. 结论

- 可立即钉死：主案设计下 watch 对用户可感延迟的增量为零（后台化+纯函数搭车+dedupe+防风暴上限）；LLM 成本为零；周期模式线性成本受账本止损线管束。
- 需实验补证的：bake 物理墙钟数字（E1/E2，基建=PULL-PROBE-TIER-EXT-1 分级扩展 + probeSpent 账本，可与 G3-REOPEN-COSTCMP-1 对比轮共用计量基建但**结论独立成卷**）。
- 本设计不引用任何未发生的实测数字；所有"成本可控"论断均落在设计决定层面（架构选择可审计），实测层诚实挂起待 E1/E2。
