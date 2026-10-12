# LOUDNESS-A-CONFIG：响度监听配置面（工程级开关+预设+用户阈值）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=LOUDNESS-WATCH-DESIGN-1 验收 pass 的实施拆卡规格件五 卡A）
- 派发确认：已确认（主管依已验收设计发卡）
- 验收负责人：GLM 主管决策流
- 池序 89；**规格全文=[设计件五 卡A](../../runs/LOUDNESS-WATCH-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：PC 混音线（D1 响度域·持续证据面）；P1 / 预估见规格 / 无依赖（可立即领）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 要点摘录

loudness_watch_config.v0 挂 projectAgentRuntimeState（RLM-PROFILE-2 先例锚点在规格内）；默认关；rlm 五内置预设+自定阈值（ValidateDeliveryProfile 借用）；持久化与旧工程缺省语义（AGENTS §11）。

- 领取：2026-10-12T09:59+0800 / origin/main 809cab5d5c44b810e3bd521fc08f5a8966327044 / owner=GLM-5.3-Flash PC 执行会话（Windows PC，ZCode flash）/ 分支 port/loudness-watch-config-1 / worktree D:/Vit_DAW_wt_loudness_watch_1 / 领取提交=（本提交）
- 领取时基线：HEAD=809cab5d 工作树已有改动=VitApp/Workspace/Settings/Settings.xml + VitApp/Workspace/default_project.xml（运行时状态，非本卡域）+ coord/runs 未跟踪工件；本卡 diff 从零起算
- 锚点亲核：server.go:164/198/7245/7256 全命中（当前 809cab5d 基线复核，非仅引设计件 8e66e674 核验记录）；rlm/profile.go 形态与规格一致
- 回执：**impl commit=91ce26fd（port/loudness-watch-config-1，已推 origin）**——7 文件 1363+/4-：新包 `agent/internal/loudnesswatch/`（config.go/compare.go+两测试文件）、`chat/server.go`（运行态字段+快照/恢复+两工具拦截器分发）、`chat/loudness_watch_state_test.go`（新）、`tools/catalog.go`（两 spec 行）。
- 单测清单（卡面九项逐一对应）：①旧记录无字段=缺省关零行为→`TestLoudnessWatchAbsentFieldRestoresDisabled`（含持久化形态不新增键断言）②写入→快照→恢复→读回全等含 custom 指针阈值→`TestLoudnessWatchConfigRoundTripThroughRestore`+包级 `TestApplyUpdateMergeAndPointerRoundTrip`（nil 指针保持 nil/全等 DeepEqual）③坏 schema/坏枚举=关+dropped 计数→`TestLoudnessWatchInvalidRecordsDropAndCount`（bad_schema/bad_trigger/unknown_builtin 三子例，status 披露 restore_dropped）+包级 `TestBadSchemaVersionAndUnknownEnumsFailClosed`④builtin 未知 id fail-closed→`TestUnknownBuiltinIDFailsClosed`（+五内置全集 `TestValidateAcceptsAllFiveBuiltinProfiles`）⑤custom 带序/跨窗序/TP>0/非有限逐项拒绝→`TestCustomThresholdViolationsRejected` 四子例（错误消息含字段与值）⑥realtime 存储往返+unavailable 原因→`TestRealtimeStoredNotActivated`+`TestLoudnessWatchRealtimeStatusDisclosure`（unavailable/no_realtime_producer）+periodic 缺省 300/<60 拒绝→`TestPeriodicIntervalDefaultAndFloor`⑦Compare 超上限/低于有下限/nil-min 不查下限/行缺失/ST-M 恒 unmeasurable→`TestCompareBandSemantics`+`TestCompareUnmeasurableCases`（breach 支配+维度披露保持）⑧peak 必要条件（>上限⇒breach；≤⇒no_breach_seen 非 pass+inter-sample 免责文案）→`TestComparePeakNecessaryCondition`⑨verdict 文案带近似口径限定→`TestVerdictTextCarriesApproximateQualifier`（三态）+行键形态兼容 `TestCompareConsumesExistingRowKeyShapes`；chat invoke 面→`TestLoudnessWatchConfigureFace`+`TestLoudnessWatchInvokeRecognition`（三种形态识别）。
- 验收命令：`cd agent && go build ./...` 过；`go test ./... -count=1` **94 包 exit 0 / 0 FAIL**（loudnesswatch+chat+tools 聚焦三包 ok）；触碰七文件 `gofmt -l` 净。
- 锚点偏差记档（两处，均非阻塞未触停止条件）：①catalog spec 实际 `catalog.go:1069`（设计件记 :1067，形态逐参一致）②`render_profile_bind` 既有分发在 harness `invokeLocal`（harness.go:2463），本卡两工具按卡面文件域落 `chat/server.go handleInvoke` HTTP 拦截器面（:1481 fall-through 前）——**边界申报**：agent-loop 执行器路径（executor→harness.Invoke）不含两 case，LLM 经该路径调用会落 kernel 失败；HTTP invoke 面（webui/直连 /agent/invoke）可达。如需 agent-loop 可达，归卡 B（harness.go 在其文件域）扩 case，交主管裁量。
- verdict schema 名按卡 A 权威用 `loudness_watch.v0`（设计① §2.4 写 `loudness_watch.verdict.v0`，以卡为准）。
- 端测边界声明（AGENTS §5）：卡面定为纯单测卡（"不需要真栈"）；本卡无渲染面/webui 改动，未跑真栈烟测；触发面真栈验证归卡 B `-LoudnessWatch` 腿。
- 领取基线精确值修正：领取提交 813cbf58 的父=005893b7（主管 dispatch 提交，仅 coord/reports 单文件，agent/ 代码面与记录的 809cab5d 等同）；实现自 005893b7 起，main 在飞期间前移（Mac 线 a57640f0）与本卡文件域零交集。
- 验收：**pass** / 2026-10-12 晚窗（[rulings/2026-10-12-EVENING-BATCH1-rulings.md](../../rulings/2026-10-12-EVENING-BATCH1-rulings.md)；九项单测亲核+决策侧合并态 95 包 0 FAIL；agent-loop 可达边界转 LOUDNESS-B 前置义务；impl merge 合入 main）
