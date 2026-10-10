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

- 领取：（时间 / origin/main hash / owner / 分支 / 领取提交）
- 回执：（commit / 契约测试名 / 响应面零变化对照）
- 验收：（裁定文件 / 验收 commit）
