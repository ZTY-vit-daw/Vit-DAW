# HARNESS-STRUCT-NORMALIZE-1：harness 结构体嵌 map 契约源侧归一化（LEDGER 域外发现）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据=[NIGHT-BATCH rulings §5-1](../../rulings/2026-10-10-NIGHT-BATCH-rulings.md)+LEDGER 回执域外发现）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 53；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无
- 模型分级：L1 / flash 可接

## 缺口（LEDGER-PERSIST 取证实证）

`agent/internal/harness/ccb_observation.go:110-113` 及批量/拒绝分支把 `capabilitycontext.FreeStateObservationBundle` / `RejectedFreeStateObservScoped` **结构体**直接嵌入 map[string]any 工具结果——严格 map 断言消费者读到空。LEDGER 已在 chat 消费侧归一化（freeStateNormalizeMapAny）；agentloop 侧既有 messageLoopMapValue 归一。**源侧不归一=其他潜在严格断言消费者同险敞口**（本缺陷链外未观察到新受害面）。

## 目标

源侧归一化：结构体先 json.Marshal→Unmarshal 成 map[string]any 再嵌入（或等价机械转换），消除类型面歧义；补契约测试：工具结果 map 内任意嵌套值不出现非 JSON 基元/容器形态（结构体零容忍钉住）。

## 文件域

`agent/internal/harness/ccb_observation.go`（+测试）；LEDGER 的消费侧归一化（chat freeStateNormalizeMapAny / agentloop messageLoopMapValue）**保留不动**（纵深防御）。越域即停。

## 验收标准

`go build ./...`+harness/chat/agentloop 三包+全量 `go test ./... -count=1` 0 FAIL+gofmt 净；契约测试绿；回执附改动锚点与响应面 JSON 字节不变对照（json.Marshal 渲染本就正常——工件面应零变化）。

## 停止条件

marshal 往返造成字段丢失/精度变化（如 time.Time 时区）→ 形态上交。

- 领取：2026-10-20 18:52 / origin/main a2a04355（派单时口头基线 03fea178，领取前远端已前进至 a2a04355，本卡按 a2a04355 领取）/ owner=GLM-5.3-Flash（ZCode PC 执行会话）/ 分支=port/harness-struct-normalize-1 / worktree=D:/Vit_DAW_wt_hsnorm1 / 领取提交=本提交（coord/hsnorm1-claim 仅含本卡面变更，fast-forward 推 main）。领取时核对：doing 池=GEN-SUNO-WEB-DESIGN-1（webui 域）/FS-RECEIPT-REVISION-1（chat 域）/SEMANTIC-TIMEFX-BUS-CONVENTION-1（设计卡不改生产代码），与本卡文件域零冲突；目标文件 blob f7ac9b2f 在 03fea178/358b3de7/3c4176b7/a2a04355 四基线一致
- 回执：实现 commit 5af9bd5b（分支 port/harness-struct-normalize-1，已推 origin 待验收；基线 a2a04355）
  - **改动锚点**：`agent/internal/harness/ccb_observation.go`（+51/−16）——新增归一 helper `ccbObservationBundleMap`/`ccbObservationCatalogMap`（marshal→unmarshal 往返）与 `ccbObservationRejected`（拒绝响应构造）；**13 嵌入点**全部改嵌 JSON 形态 map：单路成功（原 110-113 行）/批量成功（原 190-193）/拒绝 9 处（原 51/54/57/66/72/88/93/99/108 行）/目录 2 处（原 16/28 行）。新增测试 `ccb_observation_normalize_test.go`（254 行）。
  - **契约测试名**：`TestCCBObservationBundleMapIsJSONShapedAndLossless`（全字段族往返无损：嵌套 audit_receipt/枚举 omissions/指针区间/计数）/`TestCCBObservationBundleNormalizationKeepsWireValues`（旧形态严格断言读空缺陷钉住+值面 DeepEqual 零变化+字节面 t.Logf 对照）/`TestCCBObservationRequestRejectedResultIsJSONShapedMap`（零值 Harness 走真实拒绝路径，严格断言读到完整 bundle）/`TestCCBObservationCatalogResultIsJSONShapedMap`（目录三态：全量/维度过滤/未知维度拒绝）。四测试全绿。
  - **卡面外同类缺陷发现（同文件域内一并修复，请验收裁认）**：卡面点名的 bundle 之外，`ccbObservationCatalog` 两处同样把 `FreeStateObservationCatalog` 结构体直嵌结果 map，同缺陷类同消费者风险面（agentloop/executionverifiers 皆 map 归一读它），已一并归一（独立 helper，若裁定超范围可单点回退该 2 处）。
  - **停止条件核查**：结构体无 time.Time/大整型/浮点敏感字段，marshal 往返零字段丢失零精度变化（契约测试逐字段断言背书），不触发形态上交。
  - **响应面零变化对照**：①值面零变化——旧（结构体直嵌）vs 新（归一 map）经 json.Marshal 渲染后 Unmarshal 深度相等（契约测试硬断言）；②字节面——差异仅为 bundle 对象内键序（旧=结构体字段序 schema_version 先，新=map 字典序 audit_receipt 先）；对象内嵌套 map 字段（target_ref/freshness/time_window/top_k/audit_receipt 内层）两形态均字典序无差异；③工件面实际零变化——LLM 可见面本就经 messageLoopMapValue/freeStateNormalizeMapAny 归一后渲染，两形态产物一致；journal 持久化为值等价 JSON（键序差异不影响任何消费者，全仓无字节级 diff 消费者）。
  - **验收命令与结果**（worktree D:/Vit_DAW_wt_hsnorm1，HEAD=5af9bd5b）：`go build ./...` OK；`go test ./internal/harness ./internal/chat ./internal/agentloop -count=1` 全 ok（21.3s/123.7s/7.0s）；`go test ./... -count=1` 全量 0 FAIL（复跑两遍确认）；gofmt——内容格式净（`gofmt -l` 报 harness 下 60 文件全系 Windows autocrlf CRLF 检出工件，含大量本卡未触碰文件；剥 CR 后本卡两文件与未触碰文件同样干净，未引入格式漂移）。
  - **消费面核查**：全仓无 `result["bundle"]`/`result["catalog"]` 的结构体类型断言残留——唯一消费者皆经 map 归一（agentloop messageLoopMapValue×2/chat freeStateNormalizeMapAny/executionverifiers verifierMap→protocolvalue.Object），按卡面要求保留不动（纵深防御）。
- 验收：（裁定文件 / 验收 commit）
