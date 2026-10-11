# LOUDNESS-WATCH-DESIGN-1 设计基线

- 卡：`coord/cards/doing/2026-10-11-LOUDNESS-WATCH-DESIGN-1.md`（响度监听=编译器式响度合规门，设计卡零生产代码）
- 执行流：GLM-5.3-Flash PC（ZCode 会话）/ 领取 origin/main=8e66e674 / 分支 `port/loudness-watch-design-1` / worktree `D:/Vit_DAW_wt_loudwatch1`
- 日期：2026-10-11 晚窗
- 锚点核验：`anchor-verification.txt`——41 项机械核验 **41 PASS / 0 FAIL**（卡面底座锚点 14 项+设计新增锚点 27 项）

## 设计五件索引

| 件 | 文件 | 一句话结论 |
|---|---|---|
| ①触发机制 | [01-trigger-mechanism.md](01-trigger-mechanism.md) | 主案=处理后触发，三钩子全有既有锚点：settle 桥（`free_state_conclusion_bridge.go:65`，主）+动作观察搭车（`harness.go:4279`，零 bake 复用）+项目变更（`free_state_reasoning_loop.go:83`，带诚实边界）；周期可配默认关；实时不可达=生产能力缺失（内核无流式表头），配置可存、激活报 unavailable |
| ②配置面 | [02-config-surface.md](02-config-surface.md) | `loudness_watch_config.v0` 挂 agent 侧 `projectAgentRuntimeState`（RLM-PROFILE-2 已收模式，`server.go:164/198/7245/7256`）；开关默认关（旧工程缺省=零行为变化，§11 全项）；预设=rlm 五内置（R128/AES/Apple/Spotify/GY-T，`profile.go:67-118`）；自定义阈值借 `ValidateDeliveryProfile` 校验；工具面 configure+status 两 spec |
| ③告警面 | [03-alert-dual-consumer.md](03-alert-dual-consumer.md) | agent 消费者四披露位（typed_events/catalog 条目/证据分级映射/M1-4 D1 pack）+用户消费者（chat 事件行+mix_report 三视图段，Godot 后置）；双面同源单存储；unmeasurable 不告警；grade 强制标注防合规级误读 |
| ④测量口径核对 | [04-measurement-audit.md](04-measurement-audit.md) | 就位=近似 integrated（`rmsDb-0.691`，`approximate_rms_lufs_v1`）+sample peak+crest，全 clip 域；缺口五项=BS.1770 门控真口径/ST 窗/M 窗/true peak/母线域测量（RLM 与 manifest 是形备量缺）；内核补齐规格→KERNEL-LOUDNESS-BS1770-1 候选卡 |
| ⑤实施拆卡 | [05-impl-card-specs.md](05-impl-card-specs.md) | 三张达可直接发卡：A 配置面（无依赖立即可发）/B 触发接线（依赖 A+M1-4+TIER-EXT）/C 告警双面（依赖 B）；内核候选卡 1 张登记不发；互斥与串行预检已注 |
| 性能结论 | [06-performance-cost.md](06-performance-cost.md) | 账本机制已接线（`pull_session.go:459` probeSpent 逐笔）但 G3-REOPEN 对比轮未跑=**无物理数字可引（如实）**；设计侧可钉死：主案回执延迟增量=0（后台化+纯函数搭车+dedupe+≤4 clip 上限）、LLM 增量=0；E1/E2 实验设计全项（§8 纪律） |

## 停止条件核对

卡面停止条件="发现 RLM profile/绑定面与卡面锚点重大不符 → 上交"。核对结果：**无重大不符**。

- 卡面 `rlm/profile.go:44-86` 标准预设：实际预设全集为 `:67-118` 五项（卡面点名 R128/AES 在列且阈值一致：R128 -23.5~-22.5/-1 dBTP 实测 `:73`）——DRIFT-1=预设集是卡面超集（多 Apple/Spotify/GY-T 三预设，为后续验收轮新增），方向有利非冲突。
- 卡面 render.profile.bind（chat/server.go+harness.go）：实测 `harness/render_profile.go:25`+`tools/catalog.go:1067`+`chat/server.go:198/7245/7256`，形态与卡面一致。
- 卡面 probe 计量账本：实测 `pull_session.go:49/440-463/486-495`+`pullharness/budget.go:67-85`+`pull_continuation.go:34`，形态与卡面一致。
- 卡面 LUFS 口径面：RLM 窗口字段形备（`:59-61`）、内核近似口径实备（`L3AcousticAnalyzer.cpp:807-810`）、FXM Measurement 字段在（`fxm/types.go:69-71`，生产者未发现已如实登记）——与卡面"测量口径：RLM integrated/short_term_max/momentary_max LUFS；内核 L3AcousticAnalyzer+AudioFeatureTypes 响度面"声明相符（该句描述的是 schema/字段面，生产缺口由本设计④如实展开）。

## 关键设计决定（防实现走样）

1. watch 是 **lint 不是硬门**：任何触发点不阻塞 settle/回执/用户操作；watch 失败只 WARN 不回滚。
2. **不挂 ready-gate 同步路径**（`harness.go:3796` 是阻塞式观察关卡，watch 明确避开）。
3. v1 全部 verdict 强制 `measurement_grade="approximate"`；peak 维度只产"未见 breach"不产 pass（sample peak 看不见 inter-sample 峰）。
4. custom 阈值走 watch 独立 schema，**不改** render.profile.bind 的 builtin-only 契约（`profile.go:232-248`）。
5. 配置面实时枚举值接受存储、激活报 unavailable（尊重意图+不产噪音+如实能力缺口）。
6. 实施卡 B/C 的真栈烟测按 `fs_largeproject_smoke.ps1:126` 门控腿模式扩 `-LoudnessWatch`；回执按 AGENTS §9 全项记录。

## 工件清单

```
coord/runs/LOUDNESS-WATCH-DESIGN-1/
  README.md                    本件
  01-trigger-mechanism.md      设计①触发机制
  02-config-surface.md         设计②配置面
  03-alert-dual-consumer.md    设计③告警双消费者
  04-measurement-audit.md      设计④测量口径核对
  05-impl-card-specs.md        设计⑤实施拆卡（三卡全规格+内核候选）
  06-performance-cost.md       性能结论（证据状态+E1/E2 实验设计）
  anchor-verification.txt      41 项机械锚点核验（41 PASS/0 FAIL）
```
