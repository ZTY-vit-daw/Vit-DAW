# G3-ATTRIB-2 回执（P1 前缀计量供给 + pull 协议段拆分修复）

- 卡：G3-ATTRIB-2（池序 36；G3-RULING §3 归因卡②，返工③登记修复项）
- 执行：GLM-5.3 执行会话（PC / ZCode），2026-10-09 晚窗；worktree `D:/Vit_DAW_wt_g3a2`，分支 `port/g3-attrib-2`
- 领取基线：origin/main=7eb38fc5（领取提交 28d84fdf，与 ATTRIB-1 领取 d6c4f060 同基不冲突）；领取时工作树净
- 实现提交：`24a6e722`（实现+测试）+ `af12b583`（补提交驱动级测试 protocol_split_test.go——独立复核腿 concern 清偿）
- 真栈占用：无（本卡单测面即可验证，卡面明示不占真栈）；真栈 pull 断裂复验一轮可选归 ATTRIB-1 同栈窗口

## 1. diff 摘要（8+1 文件，540+ 行，全部在声明文件域内）

| 文件 | 变更 |
|---|---|
| `agent/internal/promptruntime/prefix_service.go` | AssemblyReport 加法式字段：`PrefixContentHash`（稳定段实际渲染字节 sha256=最终 system 消息内容）+`PrefixStartsWithPrevious`（P1 starts-with 机械判定，nil=无上一轮不可比）；prefixSnapshot 增 systemBytes；PromptStatsExtras 加法键 prefix_fingerprint/prefix_content_hash/prefix_starts_with_previous（非 nil 才写）；**Assemble/接口签名冻结不动** |
| `agent/internal/promptruntime/prefix_service_test.go` | P1 判据测试×4：正例（无事件轮 starts-with true+哈希恒等）/append-only 尾增长（唯一合法增长，true）/负例（中部扰动→false+PrefixBreaking 归因非空）/遥测键（首轮 starts-with 键缺席） |
| `agent/internal/agentloop/message_loop.go` | 普通族渲染拆为可组合件：`messageLoopOrdinaryFamilyModeRules`（plan/readonly/autonomy 按 state 渲染，逐字提取）+`messageLoopOrdinaryFamilySystemSkeleton`（固定规则帧+输出格式+verified-entry 裁剪）；`messageLoopSystemPrompt` 改组合=历史输出**字节恒等**（复核腿独立 dump 13 状态矩阵 cmp 逐字节一致） |
| `agent/internal/agentloop/pull_session.go` | 宿主供给改双段 `pullProtocolSegments`：中性族直接复用 L1-4 `messageLoopNeutralFamilySystemSkeleton`/`TurnDirectives`；普通族=skeleton+directives（modeRules+目录行+allowed 行，即 §2.4 两个族内漂移源全进动态区）；Snapshot 设双段键、不再设 legacy 单段键 |
| `agent/internal/agentloop/pull_session_test.go` | 宿主级测试：Snapshot 双段供给+legacy 缺席/普通族 parity（skeleton+"\n"+directives==组合渲染）/中性族复用 L1-4/族内连续轮骨架字节恒等（普通族 modeRules+allowed+catalog 逐轮变；中性族 free-state 上下文推进）+族切换骨架变化/拆分前后字节序列对比工件测试 |
| `agent/internal/pullharness/loop.go` | 双段挂载：骨架 SectionStatic stable=true（段 ID `pullharness.protocol` 不变，断裂归因连续）+指令块 SectionRuntime stable=false 进动态区 user 段（`pullharness.protocol_directives`）；legacy 单段键保留兼容挂载（仅骨架键缺席时）；新增 FrameContextProtocolSkeleton/Directives 供给键 |
| `agent/internal/pullharness/protocol_split_test.go` | 驱动级测试×4（经真实 promptruntime.PrefixService 判定面）：双段挂载形态（指令块绝不进 system 消息）/族内连续轮（真实 driver 多轮）骨架层 content_hash 恒等+P1 恒 true+零 ruleset_changed+动态区吸收漂移/族切换恰一条 ruleset_changed:pullharness.protocol+P1 false/驱动级扰动检出 |
| `agent/internal/pullharness/loop_test.go` | fakeSession 增双段供给字段（protocolSkeleton/Directives+skeletonFor/directivesFor 逐轮渲染钩子） |
| `agent/internal/pullharness/session.go` | 仅注释：legacy 键标记被双段键取代 |

**未触碰**：pull_entry.go / chat 目录 / server.go / scripts（ATTRIB-1 禁域，复核腿 diff 核对为空）；HARNESS §11 冻结面（Session 六方法签名、PrefixService 接口、ExitWiring）零改动。

## 2. 两目标达成证据

**目标1（P1 计量供给）**：`prefix_bytes 跨 turn starts-with 恒等` 现可机械判定——判据字段 `PrefixStartsWithPrevious`（服务内对同 SessionKey 上一轮最终 system 消息字节做 strings.HasPrefix，字节级不推断语义）+`PrefixContentHash`（前缀段独立哈希）；遥测加法键随既有 PromptStats 面进 LLM telemetry。负例已锚：人为中部扰动稳定段→判定 false 且 ruleset_changed 归因非空（变异法验证有牙：植入恒真判定被负例抓住）。

**目标2（协议段拆分修复）**：宿主双段供给+驱动双段挂载落地；G3-RULING §2.4 登记的会话内断裂两源（族切换主源/族内逐轮可变块次源）中，**族内漂移全部落动态区**（普通族 modeRules+allowed/目录行；中性族逐轮指令），**族切换保留为唯一合法 ruleset_changed 断裂**（段 ID 连续，per-block 归因可判：骨架变化=层断裂，指令变化=动态区）。

## 3. 拆分前后稳定段字节序列对比（工件：`protocol_split_comparison.json` 同目录）

| 族 | 拆分前稳定段（整协议段 stable=true） | 拆分后骨架（stable=true） | 拆分后指令块（动态区） |
|---|---|---|---|
| 普通（两轮演化：plan 翻转+allowed/目录重算） | 17535→18086B **漂移**（=断裂源） | 17453→17453B **恒等** | 81→632B 吸收漂移 |
| 中性（两轮演化：free-state 预算/饱和推进） | 18629→19181B **漂移** | 14207→14207B **恒等** | 4420→4972B 吸收漂移 |

中性骨架 14207B 与 G3 裁定报告 §2.2 push prefix_bytes≈14207 同源互证（L1-4 骨架）；普通族骨架是组合串的头前缀（skeleton_is_head_of_old_combined=true），中性族组合串=指令块在前。

## 4. 验收门执行记录

- `cd agent && go build ./...` → exit 0
- `go test ./... -count=1` → **92 包全 ok，0 FAIL**（12 行为 no test files）
- `gofmt -l`（改动文件）→ 空（净）；注：全仓 gofmt -l 因 autocrlf CRLF 检出大量行尾噪声（基线即如此），入库内容 LF 恒定
- P1 判据测试绿（含负例）；族内恒等断言绿（host 级+驱动级）
- 独立复核腿（强制）：`independent_review.md` 同目录——总评 pass-with-concerns，前置 concern（protocol_split_test.go 漏提交）已由 af12b583 清偿；复核含独立 13 状态矩阵字节对照（与基线逐字节一致）与变异法测试有效性验证

## 5. 覆盖边界与诚实声明

- **单测面验证**：本卡全部验证为 Go 单测（含经真实 PrefixService 的驱动级判定面）；未跑真栈（卡面允许，真栈复验可选归 ATTRIB-1 同栈窗口）——"pull 会话内 ruleset_changed 降至族切换类"的真栈复验待该窗口。
- 遗留健壮性备注（复核腿登记，非缺陷）：`pullProtocolSegments` 普通族分支对 nil state 不设防（生产不可达：newPullSession 拒 nil）；两处构造性锁测试（parity/neutral-reuse）不计为行为证据。
- metrics 语义变化登记：拆分后 PrefixBytes=骨架+冷启动底座（指令块计入 DynamicBytes）——与 G3 报告 §2.2 口径（协议段整段计 prefix）相比，prefix/dynamic 分界移动是本修复的预期结果，跨版本对比时须知。
- 停止条件未触发：拆分与 Router 预算契约/ExitWiring 语义无冲突；L1-4 拆分面接口相容（中性族两函数直接复用）。
