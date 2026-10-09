# REFSCHEMA-M2X-1 回执：mom 外同族 mix.read: refs 迁 vit://mom（tim+capabilitycontext）

- 执行端：Mac（worktree `~/Documents/Vit-DAW-m2x1`，分支 `port/refschema-m2x-1`，领取时 origin/main=4b0d6d1846a3d0dec4614f36d121afd20e489763，领取提交 ff37e5b 已直推 main）
- 语义形态：`vit://mom/mix.read:<数据键>/t=all@<observation_id>#-`——scope_kind=mix.read（数据键族头）、scope_value=数据键余部、snapshot=observation_id（M1/M2 身份族语义）、t=all+#- 双段显式。与 M2 mom 包 13 处消费面完全同型。

## 1. 领取时全量清点（rg -n "mix\.read:" internal/tim internal/capabilitycontext --glob '*.go'）

| # | 锚点 | 条目 | 处置 |
|---|---|---|---|
| 1 | tim/projection.go:71 | `mix.read:project.tracks.summary` ×1（no_project_tracks issue） | 迁移 |
| 2 | tim/projection.go:107 | mix.read: ×3（tracks.summary/acoustic.tracks/limitations）+ `dad:track_waveform_envelopes` ×1 | 3 条迁移；**dad: 族范围外，原样保留并在此登记** |
| 3 | tim/projection.go:404 | mix.read: ×2（tracks.summary/acoustic.tracks，issuesForTrack） | 迁移 |
| 4 | tim/asserter.go:143-144 | acousticAssertionRefs ×2 + rackAssertionRefs ×1（变量块） | 迁移（变量→observationID 参数函数） |
| 5 | capabilitycontext/gain_staging.go:1321 | addUnique mix.read: ×3（tracks.summary/risks.headroom/rankings.peak） | 迁移 |

共 12 条 mix.read: 全部迁移。范围外保留：`dad:track_waveform_envelopes`（tim/projection.go:111 现位）、`project.state:plugin_list_hygiene`（tim/asserter.go hygieneAssertionRefs）、`project.state`/`mix.observe[:id]`/`mix.observe`（capabilitycontext gainStagingEvidenceRefs 同函数内）、`rlm:` 系（同 pack 合并面）。仓内其余 `mix.read` 命中均为**工具名**（mix.read 工具），非 ref 字面量，无 ref 前缀匹配型消费方（rg 亲核）。

## 2. 消费面清单（改动处全列）

**agent/internal/tim/projection.go**
- Build 头部 `observationID := strings.TrimSpace(input.ObservationID)` 局部化（原 :98 内联 trim 复用）
- no_project_tracks issue EvidenceRefs → `mixReadRefs(observationID, "project.tracks.summary")`
- Projection.EvidenceRefs → `evidenceRefs(append(mixReadRefs(...三条...), "dad:track_waveform_envelopes")...)`
- `buildTrackFact` / `issuesForTrack` 签名 +`observationID string` 线程传递（单一调用链 Build:48→:391）
- AssertInput 构造处回填 `ObservationID: observationID`
- 文末新增 helper：`momEvidenceRef(scopeKind, scopeValue, observationID)`（走 agentprotocol.FormatRef，错误/缺身份返回 ""）+ `mixReadRefs(observationID, dataKeys...)`；import agentprotocol

**agent/internal/tim/asserter.go**
- AssertInput 新增 `ObservationID string` 字段（含语义注释；包外构造方=零，rg 亲核）
- `acousticAssertionRefs` / `rackAssertionRefs` 包级变量 → `func(observationID string) []string`（hygieneAssertionRefs 不动）
- Evaluate 取 `strings.TrimSpace(input.ObservationID)` 并线程传入 7 个 evaluate 函数（signal_hygiene/level_ceiling/sample_rate/routing/plugin_legality/block_size/plugin_load_state 签名各 +observationID；plugin_hygiene 不涉及）；~35 处 assertionRow/passRow/failRow 调用点改传 `acoustic/rackAssertionRefs(observationID)`；import strings

**agent/internal/capabilitycontext/gain_staging.go**
- 新增包内 helper `mixReadRefs(observationID, dataKeys...)`（同型 agentprotocol.FormatRef 包装；空身份返回空）
- `gainStagingEvidenceRefs` 三条 legacy → `addUnique(refs, mixReadRefs(observationID, ...)...)`（observationID 复用同函数既有读取 :1339）；import agentprotocol

零改动核实：agentprotocol/refschema.go 与 mom 包对 origin/main 零 diff（kind=mom 已注册、scope_kind=mix.read 词汇表已在 REF_SCHEMA_V1.md §6:96，M2 落地）——vit:// 文法/注册表/文档零改动，符合卡面预期。

## 3. 新测试（8 个，红先行）

- tim/refschema_m2x_test.go：TestProjectionEvidenceRefsMigratedToVitMom（黄金串+ParseRef parsed+段断言+规范往返+dad: 保留）/ TestProjectionIssuesCarryVitMomRefs（empty_track+no_project_tracks）/ TestAssertionRowsCarryVitMomRefs（断言行 vit://mom+plugin_hygiene legacy 保留钉）/ TestProjectionWithoutObservationIdentityOmitsMixReadRefs（宁缺勿假）/ TestLegacyMixReadRefsRemainParseable（legacy 兼容读：opaque 透传五键）
- capabilitycontext/gain_staging_refs_m2x_test.go：TestGainStagingPackMixReadRefsMigratedToVitMom（黄金串+ParseRef+project.state/mix.observe 保留）/ TestGainStagingMixReadRefsRequireObservationIdentity / TestGainStagingLegacyMixReadRefsRemainParseable

红证据：实现前运行 6 FAIL（tim 4 + capabilitycontext 2，全部断言失败非编译失败）；2 个 legacy 兼容读钉按设计在迁移前后均绿（钉 ParseRef 三态现状不变）。

## 4. 行为变化申报（M2 同型，决策侧已裁定先例）

1. **空 ObservationID 不再发无身份 mix.read refs**（M2「宁缺勿假」同款）：tim 投影/Issue/断言行与 capabilitycontext B1 pack 在 observation_id 缺失时 mix.read 族 refs 缺席（其余族 refs 不受影响）。
2. **capabilitycontext stablePackID 由 EvidenceRefs 派生**（gain_staging.go:172 stablePackID(...pack.EvidenceRefs...)）：同输入迁移前后 PackID 不同（refs 形态变化的必然结果，内容寻址语义本身不变）。
3. refs 字符串结构性变长（每条 `vit://mom/`+`/t=all@<obs>#-` ≈ +51B）：本卡两包内无字节数上限断言；mixboard 预算门在 M2 已扩（70000），本卡不触碰 mixboard 域。

## 5. 验收门（全部命令在最终工作树执行）

| 命令 | 结果 |
|---|---|
| `cd agent && go build ./...` | exit 0 |
| `go test ./internal/tim ./internal/capabilitycontext -count=1` | ok ×2，exit 0 |
| `go test ./... -count=1` | **exit 0，90 包 ok，0 FAIL** |
| `gofmt -l`（本卡 5 个触碰文件） | 空（净） |

## 6. 基线形态声明 [Mac]

- 领取腿：HEAD=4b0d6d1（≥a7b5fe74 ✓）；`go build ./...` exit 0；tim+capabilitycontext 定向测试 exit 0——**基线形态=净，无与本卡无关的既有失败**。
- gofmt 甄别：`gofmt -l` 在两包报 `internal/tim/asserter_test.go` 与 `internal/capabilitycontext/low_end_relation.go` 两处**origin/main 已提交 blob 的对齐格式偏差**（Windows 会话遗留；取证=两文件本卡零触碰、git diff 空、origin/main blob 直接过 gofmt 同样报警）。按"不硬凑不弱化、越卡域不动"处理，如实登记留格式归拢卡，非本卡基线失败。
- 停止条件核查：12 条 mix.read: 数据键全部为纯数据键直落 scope_value（无保留字符，文法转义面兜底未触发）；dad: 族无既有测试硬依赖（tim 测试对 EvidenceRefs 零断言，rg 亲核）——两停止条件均未触发。

## 7. 端测边界声明

本卡为纯 Go 单测域（卡面明示无真栈腿）：vit://mom 形态断言、legacy 兼容读、身份缺失语义均由包内单测覆盖；不涉及 webui 渲染面与用户旅程面。
