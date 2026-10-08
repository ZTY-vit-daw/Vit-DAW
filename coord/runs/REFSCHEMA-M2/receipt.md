# REFSCHEMA-M2 执行回执：mom C 类构造器迁 vit://mom 形态

- 卡：coord/cards/todo/2026-10-08-REFSCHEMA-M2.md（卡面全文权威）
- 规格：coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md §4 M2 行
- 领取：2026-10-08 晚窗 / origin/main `509e37f9` / 分支 `port/refschema-m2`（独立 worktree D:/Vit_DAW_wt_m2，领取时树净）
- 本卡 diff：7 文件（6 改 + 1 新增测试），`+83/-26`（不含新测试文件）；`git diff --stat` 见文末

## 1. 锚点实核（领取时）

- `agent/internal/mom/evidence.go:24/31/38` 三构造器与卡面一致，无漂移。
- `agent/internal/agentprotocol/refschema.go:151` `observation:` legacy 条在位（M1）；**:170 mom kind 已在 projectionKindRegistry**（snapshot 段承载=observation_id 注记在案）——**kind 无需再注册**，卡面第 3 条注册事项因此零改动（仅加 scope_kind 登记注释，见 §5）。

## 2. 迁移形态（新 ref 文法实例）

统一经新增 helper `momEvidenceRef(scopeKind, scopeValue, observationID)`（evidence.go）走 `agentprotocol.FormatRef`，`vit:// 文法零改动`：

| legacy 形态 | vit://mom 形态 |
|---|---|
| `mix.read:track.<id>.<suffix>` | `vit://mom/mix.read:track.<id>.<suffix>/t=all@<observation_id>#-` |
| `acoustic_package_status:<layer>.<feature>` | `vit://mom/acoustic_package_status:<layer>.<feature>/t=all@<observation_id>#-` |
| `observation:<id>` | `vit://mom/observation:<id>/t=all@<id>#-` |

- scope=数据键族（族头进 scope_kind、键余部进 scope_value）——G1 M2 行"scope 承载数据键族"。
- snapshot=observation_id——身份族语义沿 M1 裁定（非内容哈希）；window 恒 `t=all`（legacy 键无采样窗）；hash 恒 `-`（数据面地址非内容寻址，显式未 CAS 化）。
- **文法承载力充足，停止条件未触发**（数据键族完整进 scope；保留字符经 FormatRef 百分号转义，往返闭环有测试）。

## 3. 消费面清单（`grep "mix\.read:\|acoustic_package_status:\|observation:" internal/mom/` 全量清点）

### 3.1 本卡改动处（13 处构造/字面）

| 文件:行（改前） | 内容 |
|---|---|
| evidence.go:24 | trackReadKey 构造体迁移 |
| evidence.go:31 | acousticFeatureRef 构造体迁移（签名加 `input Input` 以取 observation_id） |
| evidence.go:38 | observationRef 构造体迁移 |
| projection.go:80 | `mix.read:project.static.summary` + `mix.read:project.limitations` 两字面迁移 |
| projection.go:114/127/132/136/181/186/190/266 | acousticFeatureRef 8 处调用点适配新签名 |
| projection.go:287 | `mix.read:observation.ab_result.latest` 字面迁移 |
| projection.go:313 | `mix.read:observation.before_after.latest` 字面迁移 |
| masking_relationship.go:87 | `mix.read:project.masking_relationship_inputs` 字面迁移 |
| frequency_relationship.go:28 | `mix.read:project.tracks.summary` + `mix.read:project.frequency_relationship_inputs` 字面迁移 |
| frequency_relationship.go:37/189/238 | frequencyTrackProfile 加 input 参数、observation_id 下传 |
| frequency_relationship.go:603 | `observation:<id>` 回退迁移（经 observationRef） |
| frequency_relationship.go:612-620 | frequencyTrackEvidenceRef 加 observationID 参数，`mix.read:track.<id>.slow.band_energy.summary` 回退迁移 |
| project_relation.go:37/73 | 各 2 条 `mix.read:` 字面迁移 |

### 3.2 mom 包内保留原样（实证排除，非漏迁）

| 位置 | 字面 | 不迁原因 |
|---|---|---|
| projection.go:84（原:80） | `acoustic_package_status`（裸，无冒号无键） | 裸族指针，无数据键形不成 scope_value——升键归 L1-2 |
| project_relation.go:37/73 | `mix.derive:rank_tracks` | mix.derive: 族，不在本卡三族 |
| projection.go:313 | `mix.derive:before_after` | 同上 |
| frequency_relationship.go:611 | `observation:unbound` | 身份哨兵：无 observation_id 可承载 snapshot，vit 形态伪造不出"无身份"语义；ParseRef 落 legacy 态可解析 |
| frequency_relationship.go:618 | `dad.band_energy_summary:` | D 类他族，自有迁移道 |
| projection.go l2RenderProbeEvidenceRef | `dad.l2_render_probe:` | C4 族，注册表已有条（slot=snapshot），非本卡三族 |

### 3.3 mom 包外同族生产/消费点（不在本卡文件域，全部未动，列出备后续卡）

- 生产：`tim/asserter.go:143-144`、`tim/projection.go:71/107/404`（mix.read: 字面，tim 自挂断言溯源标签，不校验 mom refs——亲核无耦合）；`capabilitycontext/gain_staging.go:1321`（mix.read: 自产自挂，无耦合）。
- 消费：`mixboard/mixboard.go:3723` normalizeObservationRef 与 `chat/plugin_prep_worker.go:1333` 剥离器——输入是模型提供的观察 ID 参数，不经手 mom EvidenceRefs（亲核数据流，零影响）；`queryengine/route.go:148` acoustic_package_status: legacy 路由条——**必须保留**（旧落盘 refs 兼容读）。
- 观察 JSON 中可见的 `tim_projection` 仍携 15 条 legacy mix.read refs（tim 自产），与本卡无关。

## 4. 新测试（internal/mom/evidence_test.go，8 个全 PASS）

1. `TestMomConstructorsEmitParsedMomRefs`——三构造器产出 ParseRef=RefStateParsed，逐段断言（kind=mom/scope 两段/snapshot=observation_id/t=all/#-）。
2. `TestMomConstructorRefsCanonicalRoundTrip`——INV2 规范闭环 parse(format(r))==r。
3. `TestMomConstructorGoldenForms`——三条黄金串锁定线格式。
4. `TestMomConstructorsRequireObservationIdentity`——空 ObservationID 返回 ""（不伪造 snapshot）。
5. `TestLegacyMomRefsRemainParseable`——legacy 兼容回归：`acoustic_package_status:`→legacy 态（slot=scope，TargetKind=acp）；`observation:`→legacy 态（slot=snapshot）；`mix.read:`→opaque 宽容透传（注册表素无此条，改前改后行为一致）。
6. `TestMomRefEscapesReservedCharsInScopeValue`——保留字符转义往返。
7. `TestFrequencyProjectCutRefObservationFallback`——观察回退迁 vit 形态；unbound 哨兵保持 legacy。
8. `TestProjectMixProfileEvidenceRefsParseAsVitMom`——端到端：buildProjectMixProfile 产出全部 vit:// 前缀 refs 解析为 parsed 且 kind=mom。

## 5. agentprotocol/refschema.go 注释登记（零行为，+7 行）

REF_SCHEMA_V1 §6 开放集登记义务：新增三个数据键族 scope_kind `mix.read`/`acoustic_package_status`/`observation` 随注册表注释登记（refschema.go RefSchemePrefix 上方）。**V1 文档 §6 词汇表权威表的同步未动**（docs 不在本卡文件域），留决策侧文档卡。

## 6. 行为变化申报（决策侧注意）

1. **空 ObservationID 投影不再携带 mix.read:/acoustic_package_status: 数据 refs**（legacy 写入面曾发出无身份字面量；vit://mom 的 snapshot 必填、宁缺勿假）。全库测试无断言受影响；运行面真实投影 ObservationID 恒非空（mixboard 落票后才建投影）。
2. frequencyProjectCutRef 的弱授权回退 ref（weakCutRef 路径）从 `observation:<id>` 变为 vit 形态；消费面（frequencycleanup/staticbalance/context_pack）均透传不解析前缀，亲核无影响。

## 7. 门禁结果（诚实记录：1 项未过，根因在卡外）

- `go build ./...`：**exit 0**。
- `go test ./internal/mom/ ./internal/agentprotocol/ -count=1`：**全绿**。
- 全量 `go test ./... -count=1`：**88 包 ok，exit 1**，2 个 FAIL：
  1. **`mixboard/TestFrequencyStereoProjectionOmitsRawWaveformTimeSegments`——本卡唯一真实未过项，根因=卡外预算门饱和，非实现缺陷**：
     - 该测试断言 MOM 观察 JSON ≤50000 字节（mixboard_test.go:596-598，防原始波形泄漏的体积护栏；泄漏本身的 forbidden-string 断言全部通过，仅字节上限爆）。
     - 亲测 HEAD 基线（scratch worktree@509e37f9 探针）：**49984 字节，预算余量仅 16 字节**（另一次运行 49969，抖动 ±15）。
     - 本卡分支：**58399 字节**（+8415）。载荷内 mom ref 实例 165 条（digest 容器 69 + mom_projection 容器 96），全为 vit 形态、零重复零遗漏；÷165≈51 字节/条=纯 ref 变长（snapshot 携带完整 observation_id ~33 字符 + `vit://mom/`+`/t=all@`+`#-` ~20 字符）。
     - **裁定含义**：G1 M2 行强制的 snapshot=observation_id + ruling #2/#3 双段显式，使任何忠实的 vit://mom 迁移都必然 +~8.4KB；mom/agentprotocol 文件域内不存在能同时满足"迁移"与"50000 预算"的实现。该预算门属 mixboard 测试文件（卡外），按 §11 不擅改，上交裁定。可选路径：(a) 决策侧扩域调预算（泄漏断言独立于字节数存在，护栏目的不损）；(b) 另开卡做载荷瘦身（如 digest 容器 refs 去重）；(c) 其他裁定。
  2. `chat/TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection`——**环境中断型 flaky，与本卡无关**：失败类型=`TempDir RemoveAll cleanup: unlinkat ...state: The directory is not empty`（Windows 临时目录清理竞态，非断言失败）；HEAD 复跑 1×过、本分支隔离复跑 3×全过；全量跑时 chat 包耗时 92s（高负载）。按 §11 记录失败类型，不判 known flaky（首次观察）。
- gofmt：**blob 级净**。说明：本机 `core.autocrlf=true`（工作树 CRLF），`gofmt -l` 对工作树全量文件报警属行尾噪声；HEAD blob 在 go1.26.2 gofmt 下零 diff（版本兼容已亲核），本卡 7 文件经 gofmt 规范化后 blob 净。执行早期 `gofmt -w` 曾波及两个包共 19 个未触碰文件（纯行尾），已即时 `git checkout --` 恢复至 HEAD，未混入本卡 diff。

## 8. diff --stat（本卡）

```
agent/internal/agentprotocol/refschema.go    |  7 ++++++
agent/internal/mom/evidence.go               | 36 ++++++++++++++++++++++++----
agent/internal/mom/frequency_relationship.go | 20 ++++++++++------
agent/internal/mom/masking_relationship.go   |  2 +-
agent/internal/mom/project_relation.go       | 15 ++++++++++--
agent/internal/mom/projection.go             | 29 +++++++++++++---------
agent/internal/mom/evidence_test.go          | (新增 213 行)
```

## 9. 验收建议

实现面、legacy 兼容面、测试面均按卡面完成且绿；"全量 0 FAIL"一项因卡外预算门饱和不可在本卡文件域内达成（§7.1 证据）。请决策侧就 mixboard 预算门裁定扩域或另开卡后再收 M2；若裁定调预算，本分支可当场补一个 mixboard_test 一行改动（预算上调或改为泄漏断言主导）+ 复跑全量后交付。
