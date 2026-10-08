# L1-4-IMPL-D 回执（决策侧 L2 亲自，2026-10-08 早窗）

任务卡：`coord/cards/doing/2026-10-08-L1-4-IMPL-D.md`（池序 20；规格=CONTEXT_LAYERING_V1_DESIGN §9 IMPL-D 行+§6.4 收口线+A/B/C 三卡端测边界声明汇总）

## 实现与提交（四腿四 commit，均可独立 revert）

| 腿 | commit | 内容 |
|---|---|---|
| D1 retain 消费接线 | `2bc8843c` | RunTurnBoundaryHook 返回完整 ExitReport（retain 决策 ProjectDir 在位时经 WriteRetains 落盘，失败显式进 ExitViolations 不静默）；ExtraUnits 供给缝；Runner 终态漏斗 retainRunObservationConclusions（观察账本结论行 retain 进工程 L4 账本=§4.2 行 3 跨会话延伸落地；语句级去重防重跑重复入账；暂停面不触发——账本随 input.Context 进 continuation 存续）；遥测加法键 exit_retains_written |
| D2 L1 权威翻转 | `16d3293c` | chat/中性族生产装配从生产常量切 ruleset embed 渲染（含 shared.discipline.evidence_refs 第八段——IMPL-C 预言的"翻转后生产面即含新段"兑现）；legacy 模板降为 fail-open 回落字节不动；parity 测试翻向=canary（断生产面=embed 全量渲染，回退常量即红）+回落模板锁定测试（IMPL-B 迁移零改写承诺持续有效） |
| D3 四层真实装载 | `4cd09238` | chat/中性族 system 从单段切 carriers.Assemble 层序 Section（rules→profile→env→ledger→目录；缺层 absent fail-open；中层全缺席时与 D2 单段形态**字节一致**——renderSections 连接语义保证，既有 parity 测试持续锁定）；AssemblyInput 加法式载体随附字段→报告并入+遥测键 carrier_warnings；DefaultCarrierWorkspaceDir 沿 journal workspace-root 探测约定；L1 corrupt 回落单段（system 消息不缺席） |
| D4 真栈烟测收口 | `b5fd1375` | dev_agent_smoke.ps1 新场景 context_layering（泊位族：-StartKernel 强制+隔离内核工作区+草稿根重定向）；两轮真 LLM 对话只断装配/遥测面（回复文本不断言，§8 纪律） |

## 门与退出码

| 门 | 结果 |
|---|---|
| `cd agent && go build ./...` | **exit 0**（四腿各自过后复跑） |
| `cd agent && go test ./... -count=1` | D1 后全量 **exit 0（90 包 0 FAIL）**；D2/D3 受影响面（chat/agentloop/promptruntime/contextruntime+carriers/ruleset）全绿；**D4 后终局全量复跑 exit 0**（见下方验收行——回执落盘时点以该复跑为准） |
| gofmt | 本卡全部触碰 .go 文件 gofmt -l 零输出（CRLF checkout 伪象按 IMPL-C 先例处理：gofmt -w 归一，blob 内容 LF 不变） |
| 真栈烟测 | **`-Scenario context_layering -StartKernel` exit 0**（run `20261008_111759`，工件 coord/runs/L1-4-IMPL-D/20261008_111759/） |

## 真栈 run 记录（§8 如实分记）

| run | 结果 | 分类 |
|---|---|---|
| 20261008_111242 | 断言失败（泊位身份等待超时） | 脚本缺陷：场景未入泊位族，无内核时 project_uuid 永不稳定——修正=并入 $journeyBerth（要求 -StartKernel） |
| 20261008_111430 | 断言失败（遥测 0 条） | 脚本缺陷：过滤按 source=chat，实际 chat 消息路由进语义入口→message_loop 装配（产品腿全部通过——两轮对话完成） |
| 20261008_111521 | 断言失败（breaks 键"缺失"） | 脚本缺陷：Get-OptionalProperty 对空数组返回 ""——改为原始行 Contains 断言 |
| 20261008_111609 | 断言失败（通配符模式无效） | 脚本缺陷：`[` 是 -like 元字符——改 String.Contains |
| **20261008_111759** | **exit 0 PASS** | 泊位自起自拆；两轮真 LLM 对话；遥测八键在位；prefix_bytes 跨 run 相等（49136=49136）；exit_retains_written=0（chat 面无结论单元，确定性预期） |

四次失败全为**脚本断言面迭代**（非产品失败——run 111430 起产品腿每轮全通过）；无同形重复失败。

## 真栈证据（run 20261008_111759）

- message_loop 装配记录 section_stats 22 键：prefix_bytes=49136 / dynamic_bytes=11691 / breaks=[] / exit_violations=0 / exit_retains_written=0 / carrier_warnings=0 / history_refs_total=0 / history_refs_parsed=0——**L1-4 全链遥测面在真栈在位**（工件 agent_llm_telemetry.jsonl + context_layering_telemetry_snapshot.jsonl）。
- 两个独立 goal run 的 prefix_bytes 逐字节相等——L1 ruleset embed 渲染跨 run 字节稳定（P1 的跨 run 面；run 内逐轮 P1 由 T-A1 单测锁）。

## 端测覆盖边界声明（设计 §6.4 渲染面纪律）

1. **装配顺序变更影响面=LLM 请求侧 system 消息组成**（D2 新增纪律段/D3 层序 Section），非 webui/Godot 渲染面——本卡零 webui/UI 文件改动，E2E-WEBUI-1 覆盖面不涉装配内容；用户可见回复面不变。
2. **retain 落盘腿的真栈面**：本场景 chat 驱动的 goal run 无自由态观察账本（确定性 0 retain），retain 写入由单测覆盖（TestRunTurnBoundaryHookConsumesRetains/TestRetainRunObservationConclusions*）；真栈 goal 面由 J1/J2/J3 旅程泊位过同一装配路径覆盖（其断言面不含账本文件——如实声明：**自由态结论 retain 的真栈落盘样本待下一旅程轮采集**）。
3. **agentloop 通用路径（非中性族）的 L1 未入 ruleset manifest**（IMPL-B 迁移面只含 chat+中性族两族）——四层装载仅覆盖两族；通用路径保持既有 system（无回归）。
4. **L4 genesis 头部段未接线**：TOM/RLM 摘要行采集属工程打开事件面（装配面不持有投影采集点），归后续卡；账本当前仅承载 retain 条目。
5. **OQ-3（CAS 白名单数据）**：真栈 chat 面单元分布=history_message 全量、0 个 tool_result/observation_bundle 供给（语义入口 goal run 的工具结果未喂给——喂给面按"装配面可得"边界未扩）；v1 白名单维持仅 tool_result，**真栈样本不足以复核裁定，OQ-3 维持开放**。
6. **OQ-5（多 system 消息兼容）**：未触发——装配保持单 system 消息形态（层序 Section 合并渲染），无降级路径需求。

## 红线对账

1. 五处既有退场机制零改动（D1 只增消费面与终态延伸挂点；chat 截尾/快照折叠/观察账本窗口/冷引用/ExpiresAfterContextChange 锚点零 diff）。
2. L1-3 接口冻结零触碰（queryengine/agentprotocol 零 diff）。
3. 账本只追加：WriteRetains 全经 carriers.AppendLedgerEntry；T-B5 append-only 语义延续（去重在供给面先过滤，写入器不管）。
4. G1-G8 门零改动（本卡不涉自由态 admission）。
5. 22 键 allow-list 未扩（账本独立写入器路径）。
6. ruleset 既有七段字节零改动（D2 只切换消费侧；embed 资源与 IMPL-C 提交态零 diff）。

## 文件数申报

生产+测试+脚本 16 文件（新增 7：exit_retain.go/exit_retain_test.go/exit_hook_consumption_test.go/carrier_dirs.go/carrier_report_test.go/carrier_assembly_test.go/carrier_neutral_test.go；修改 9：chat server+parity、agentloop message_loop/ccb_model_prompt+parity、contextruntime history_refs、promptruntime prompt+prefix_service、scripts/dev_agent_smoke.ps1）——卡面预计 ≤12，超数因四腿各自配套测试分文件；全部在申报域内（越域=0）。
