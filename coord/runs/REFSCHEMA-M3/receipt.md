# REFSCHEMA-M3 回执：A 类四包 stableProjectionID 迁 vit:// 带 sha256 段

- 卡：coord/cards/doing/2026-10-09-REFSCHEMA-M3.md（领取行已回填）
- 执行：PC 执行侧会话，独立 worktree D:/Vit_DAW_wt_m3，分支 port/refschema-m3
- 领取时 HEAD：origin/main `935b15a4`（worktree 检出同 commit，树净）
- 领取时 git status --short（主工作树）：`M VitApp/Workspace/Settings/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`M agent/internal/contextruntime/history_refs.go` 及既有 coord/runs 未跟踪工件——本卡未触碰主工作树任何改动

## 1. 锚点回查（领取时实核，零漂移）

`rg -n "stableProjectionID" internal/dom internal/fxm internal/com internal/rlm` 全量清点 22 处（含测试 1 处）：

| 包 | 定义 | 赋值（生产） | 消费（测试） |
|---|---|---|---|
| dom | projection.go:530 | :93、:138 | — |
| fxm | projection.go:207 | :79 | — |
| com | projection.go:467 | projection.go:57；change.go:67、:82；paired.go:119、:155 | change_test.go:193（rehashProjection） |
| rlm | projection.go:835 | :120 | — |

与卡面锚点逐条一致，无行号漂移。

## 2. 实现摘要（五文件改动 + 四新测试文件，diff 243+/8-）

四包 `stableProjectionID` 同型改造：**sha256 种子输入集逐字节不动**（dom/com 剥 ProjectionID/GeneratedAt/LLMContext=内容身份；fxm 剥 ProjectionID/LLMContext 保 GeneratedAt=实例身份；rlm 七字段 \x00 拼接保 GeneratedAt=实例身份），产出的 digest 以 `sha256:` + 前 16 hex（F3 截断）进 vit:// `#hash` 段，经 `agentprotocol.FormatRef` 规范序列化：

| 包 | 形态 | scope 承载 | snapshot 承载 | 身份族（F6/REF_SCHEMA_V1 §7/QUERY_ENGINE §3.3） |
|---|---|---|---|---|
| dom | `vit://dom/<kind>:<id>/t=all@<obs>#sha256:<16hex>` | TargetRef kind/id，缺省 target/unknown（物化层 targetScope 先例同款） | ObservationID，缺省 unknown | 内容身份（种子置空 GeneratedAt） |
| fxm | `vit://fxm/…` 同构 | 同上 | ObservationID，缺省 unknown | 实例身份（GeneratedAt 参与） |
| com | `vit://com/…` 同构 | 同上 | ObservationID，缺省 unknown | 内容身份（种子置空 GeneratedAt） |
| rlm | `vit://rlm/project:current/t=all@<GeneratedAt>#sha256:<16hex>` | project:current（per-project 单例，无工程 ID 字段；"current" 沿物化层 snapshotTokenCurrent 词汇） | GeneratedAt（实例代=生成时刻，保留字符经 canonical 转义），缺省 unknown | 实例身份（GeneratedAt 参与） |

- 注册表核对结论：**四 kind（dom/fxm/com/rlm）无需注册表新增**——legacyPrefixRegistry 已有四条 legacy 条目（refschema.go:131-134，TargetKind=dom/fxm/com/rlm、Slot=hash），`registeredKindSet` 由其 TargetKind 收集，`vit://dom/...` 等新形态 ParseRef 直落 parsed 态（有测试证明）。legacy 条**原样保留**=兼容读（红线达成）。
- **vit:// 文法零改动**：refschema.go 本卡零触碰；agentprotocol 包零改动。
- snapshot 缺省档设计说明：legacy 形态下 ProjectionID 恒有值（哈希总能算出）；vit:// 文法 snapshot 空非法，故 ObservationID/GeneratedAt 缺失时以显式 `unknown` token 兜底（与物化层 targetScope 的 target/unknown 缺省档同风格），不伪造观察身份，ID 全值性（totality）与 legacy 行为一致。identity 仍由 hash 段兜底唯一。

## 3. 消费面清单（改动处全列 + 核对结论）

生产代码仅动四包 stableProjectionID 函数体，六处赋值点经同函数自动跟随；卡外生产代码零改动。消费面核对：

| 消费点 | 类型 | 结论 |
|---|---|---|
| com/change.go Before/AfterProjectionID 记录 | 溯源引用 | 同轮构建同形态，无断裂；其值进 change 投影种子属既有语义 |
| chat/semantic_compressor_execution.go:76-83 | 同源一致性校验 | 双方同取 comContext，形态同步，通过 |
| capabilitycontext/gain_staging.go:640/:1156 | rlm_projection_id 透传 | 只断言非空（gain_staging_test.go:557），通过 |
| materialize/adapters.go:422/:442 | payload 透传 | 空值容忍式，通过 |
| mixboard/catalog.go:163/:174 | 目录透传 | 形态无关，通过 |
| queryengine/route.go:56/:105/:115/:134 | legacy 翻译路由 | 兼容读面，本卡零触碰 |
| harness/materialize_onpath_test.go:174 | ProjectionID 相等断言（同源） | 通过（"stableProjectionID 不剥离 observation_id" 注释语义不变——vit:// 形态下 observation_id 同时显式进 snapshot 段） |
| materialize/recompute_test.go:873/:882 | fixture 手写伪 ID 直填 | 透传面，通过 |
| **mixboard/persistence_v2_test.go:127** | **写盘断言（卡外唯一改动）** | 原 `strings.Contains(ToLower(text), ProjectionID 原文)` 依赖 legacy 全小写 hex 巧合；vit:// snapshot 段嵌时间戳 obsID（含大写 T）后两侧大小写失配。修为两侧同口径小写比对（断言强度不变：ID 完整在盘）。非功能缺陷，持久化内容本就正确。 |

预算门核对（M2 教训预警项）：mixboard ObservationMaxBytes=2MB、mixboard_test.go:596 预算 70000——本卡 ref 变长未触任何预算门（全量 0 FAIL 实证），**无需扩域授权**。

## 4. 新测试（6 个，全绿）

- dom：`TestProjectionIDVitRefForm`（形态/ParseRef parsed/段核对/hash=种子前 16/内容身份回归/legacy 读）、`TestProjectionIDScopeEscaping`（保留字符转义往返）、`TestLegacyProjectionRecordRoundTrip`（旧记录 JSON 往返+legacy 解析）
- fxm：`TestProjectionIDVitRefForm`（同构+实例身份回归：GeneratedAt 变→ID 变）
- com：`TestProjectionIDVitRefForm`（同构+内容身份回归）、`TestChangeDeltaProjectionIDCarriesVitChildIDs`（change 投影子 ID 同形态）
- rlm：`TestProjectionIDVitRefForm`（project:current/snapshot=GeneratedAt 转义还原/实例身份回归/legacy 读）
- 兼容读回归补充说明：四包 types.go 零改动=旧记录反序列化兼容的结构性保证；rlm 真实旧工程 fixture（testdata/real_old_c1_project_observe.json.gz，sha256 锁定）既有加载测试保持通过（输入侧兼容）。

## 5. 验收门（全过）

| 门 | 结果 |
|---|---|
| `go build ./...` | 退出码 0 |
| `go test ./... -count=1` 全量 | **退出码 0，90 包 ok，0 FAIL** |
| gofmt（本卡改动 9 文件，LF 归一后逐文件核） | 全净（工作树 CRLF 为检出形态，blob 级净，与 M2 同口径） |
| 四包构造器产出 vit:// + ParseRef 全部 RefStateParsed | 有测试证明 |
| legacy 兼容读回归（旧形态/旧记录往返） | 有测试证明 |
| 消费面清单入回执 | 本文件 §3 |

## 6. 停止条件核查（未触发）

- **内容哈希核查**：dom/com/fxm/rlm 四族种子均为纯确定性输入（json.Marshal 或七字段拼接）。fxm/rlm 的 GeneratedAt 参与是 **REF_SCHEMA_V1 §7 明文裁定**（"fxm/rlm 时间戳参与=实例身份"，盘点 I1 落点）与 F6 注记、QUERY_ENGINE §3.3 一致——不构成"实为非内容哈希"的上交情形；迁移按实例身份语义原样保留哈希输入集。
- scope/snapshot 承载：按物化层 targetScope/snapshot=observation_id 既有先例+QUERY_ENGINE §3.3 口径落位（rlm 沿实例身份族语义取 GeneratedAt），无两形态争议。

## 7. 提交

- port/refschema-m3 分支 commit：见卡面回执行回填
- 全量测试原始输出：/tmp/m3_fulltest.log（会话内临时工件，90 ok 0 FAIL；关键结论以本回执与卡面为准）
