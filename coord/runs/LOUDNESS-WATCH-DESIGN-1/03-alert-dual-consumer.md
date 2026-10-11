# 设计③：告警面（agent + 用户双消费者）

LOUDNESS-WATCH-DESIGN-1 设计件 3/6。本件回答"verdict 发到哪、谁看见、能干什么"。
双消费者共用同一 verdict 记录（设计① §2.4 schema），不双写两套真相。

## 1. 消费者 A：agent 自由态循环（证据面）

模型必须能在自由态循环内"看见"watch 结论，才能按用户功能定义据此提降噪/限幅提案。四个披露位，全部复用既有模式：

### 1.1 typed event（工具回执内联）

- 模式锚点：`agent/internal/harness/harness.go:3537-3538` 与 `:3838-3839` `acousticPackageStatusTypedEvent` 进 `typed_events` 的既有形态。
- 接线：T1/T2 触发的 watch 检查完成后，verdict 以 `typed_events` 附加到当次工具回执/settle 回执载荷（kind=`loudness_watch_verdict`）。webui 侧 typed_events 已有类型面（`agent/webui/src/types.ts` 含 typed_events）。

### 1.2 CCB catalog 条目（可请求披露）

- 模式锚点：`agent/internal/mixboard/catalog.go:265-266` `catalogEntry("project.risks.headroom", "derived", …, "cheap", …)`。
- 接线：新增 `project.risks.loudness_watch` 条目（derived/cheap），rows = 最近 verdict 的 breach 摘要行；`mix_read key=project.risks.loudness_watch` 可请求。观察摘录位同 `catalog.go:204-217`（loudness_ranking/headroom 摘录同款 capRows 形态）。
- 摘录与最新结论行遵循 M1-5 双折叠一致性教训（4af4b6d3）：catalog 摘要与 status 工具读**同一份** verdict 存储，不各写各的。

### 1.3 证据分级映射（决定模型该多信）

- 模式锚点：`agent/internal/capabilitycontext/context_manifest.go:144-151` 证据清单已分级——`integrated_lufs` 标 `Primary`、`approximate_lufs` 标 `Approximate`（`:145`，Usage "approximate B1.2 loudness fallback"）、`peak_dbfs` 标 headroom。
- 接线：verdict 的 `measurement_grade` 直接由测量行来源决定——当前内核只产 `approximate_rms_lufs_v1`（设计④ §2），故 **v1 全部 verdict 强制 `measurement_grade="approximate"`**，breach 文案显式带"近似口径，非合规级"限定；BS.1770 真口径生产者落地（设计④ §4 卡）后才允许 grade 升级。grade 与 manifest 的 Approximate 标记同语义，模型侧消费词一致（`gain_staging.go:359` 已消费 approximate_lufs 同名键）。

### 1.4 M1-4 观察包消费（D1 面）

- 卡面钉死的消费面：`coord/cards/todo/2026-10-11-MIXLAYER-M1-4-d1d3-observation-pack.md` D1 行 = mix.observe peak/rms/loudness_ranking/headroom + RLM `reference_level_model.projection.v0`（`agent/internal/rlm/types.go:4`）。
- 接线：watch 状态与最近 verdict 通过 M1-4 的 D1 只读 pack 披露（实现卡 B 排在 M1-4 合入后的原因）；pack 只读纪律不变（M1-3 c3ReadOnlyTools 同款白名单语义，M1-4 卡面验收标准引用）。

### 1.5 提案联动（模型消费后的动作）

- verdict 载荷的 `breach_details`（metric/measured/threshold/delta_db 逐条）足够模型定位"超多少、超哪条"；模型提修复走**既有 mix 控制面与 PCA 准入**，watch 不开新写通道、不自动执行。降噪/限幅类提案的候选生成是模型侧行为，不在 watch 机械职责内。

## 2. 消费者 B：用户呈现面

### 2.1 chat 事件消息（webui 呈现主路径）

- 模式锚点：`agent/internal/chat/audition_events.go`（audition 会话事件组装/修订/回读先例，`:604-609` 项目身份取用）。
- 接线：T1 settle 触发的 verdict 同时以会话事件落 chat 流（"响度检查：-19.8 LUFS，超出 EBU R128 上限 +2.7 LU（近似口径）"形态的一行结论 + 可展开的 breach 明细），webui 按既有消息渲染路径呈现。

### 2.2 mix_report 三视图行（权威汇总位）

- 模式锚点：`agent/internal/chat/mix_report_three_view.go`（M1-5 已收：最近 settle 结论行 + queue 节点行 + 数据来源行的渲染形态；`mixboard/decision_ledger.go:168-170` LatestSettleConclusion 折叠先例）。
- 接线：三视图增"响度 watch"段——最近 verdict 一行（verdict/grade/触发来源/时间）+ breach 计数；无 verdict 如实显"未运行"（诚实缺省，与 M1-5 无结论分支同纪律）。双消费者数据同源：三视图行与 catalog 条目读同一 verdict 存储。

### 2.3 Godot 前端呈现

- **不进 v1 实施卡**。理由：渲染面烟测体系（E2E-WEBUI-1/旅程烟测）未就位时，涉渲染面改动须按 AGENTS §5 显式声明覆盖边界；chat/webui 面先落，Godot 侧告警面板（如母线表头+告警徽标）列为后续候选，由决策侧按渲染面烟测层级进度另卡。

### 2.4 端测边界声明（§5 门槛预登记）

- 涉 webui 渲染的改动（2.1/2.2 的呈现部分）在实施卡 C 验收时**必须**：①vitest 断言渲染行存在与字段完整；②真栈冒测断言 verdict 行可见；③渲染面端侧烟测层级未就位的现状在回执显式声明，交决策侧裁定是否足以交付（AGENTS §5 渲染面门槛原文要求）。

## 3. 告警去噪与红线

- **dedupe**：设计① §2.4 键（project_uuid+project_revision+verdict_kind）；同版本重复检查不重复发事件，但 status 工具可查全历史。
- **unmeasurable 不告警**：只有 pass/breach 进 chat 事件与三视图行；unmeasurable 只落 status 工具与 typed_events 供模型诊断（避免"测不了"刷屏）。
- **红线**：watch 不自动改任何混音参数、不自动重渲染、不阻塞任何流程（设计① §4）；告警文案必须带口径 grade 限定词，禁止把 approximate 级结论写成"符合/不符合 EBU R128"的合规判定语气（AGENTS §7 证据等级）。
