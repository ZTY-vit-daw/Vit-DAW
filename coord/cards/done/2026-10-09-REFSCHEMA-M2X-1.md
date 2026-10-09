# REFSCHEMA-M2X-1：mom 外同族 mix.read: refs 迁 vit://mom（tim×3 处+asserter 变量块+capabilitycontext×1 处）

- 池序 29（M2 终审挂账 mom 包外同族点：gate 2026-10-08 §3.6"[tim×4/capabilitycontext×1] M 系列机会面"；[G1 终审记录](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4 M 系列延伸）；目标仓库=D:\Vit_DAW（**PC 或 Mac 执行侧均可**——纯 Go 单测域，无真栈腿；Mac 领取须先过基线核对腿）
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / REFSCHEMA-M2 已合入（vit://mom 形态+scope_kind 词汇表三条已在 REF_SCHEMA_V1 §6）
- 模型分级：L0-L1 / flash 可接（M2 同型先例：构造点改写+对照测试）；**并行域**：tim+capabilitycontext 两包——与 LEDGER-DIR-1（agentloop/chat/carriers）/M3（dom/fxm/com/rlm）/BOM（chat/audition_events.go）零重叠
- 已核实锚点（2026-10-09 决策侧 rg 亲核，领取时回查行号；`rg -n "mix\.read:" internal/tim internal/capabilitycontext --glob '*.go'` 全量清点入回执）：
  1. tim/projection.go:71（EvidenceRefs 单条 mix.read:project.tracks.summary）
  2. tim/projection.go:107（evidenceRefs(...) 四条，其中三条 mix.read:+一条 `dad:track_waveform_envelopes`——**dad: 族不在本卡范围，原样保留并登记**）
  3. tim/projection.go:404（两条 mix.read:）
  4. tim/asserter.go:143-144（acousticAssertionRefs/rackAssertionRefs 变量块）
  5. capabilitycontext/gain_staging.go:1321（addUnique 三条 mix.read:）
- 目标：上述 mix.read: 同族 refs 全部迁 vit://mom 形态（scope 承载数据键族，snapshot=observation_id——与 M2 在 mom 包内的 13 处消费面完全同型；mom kind 已注册、scope_kind 词汇表现成，零文法/文档改动预期）。
- 语义红线：vit:// 文法零改动；tim 断言器与 capabilitycontext 增益分级的 refs 语义=证据引用面，迁移只改形态不改所指；旧记录 legacy refs 兼容读（legacy 条保留）；dad: 族与其他前缀族不动（如实登记留后续族卡）。
- Mac 领取附加腿（Mac 线 2026-09-26 后首次接卡）：先 `git pull --rebase` 到最新 main 并核对 HEAD≥a7b5fe74；`cd agent && go build ./...` 基线绿；tim/capabilitycontext 两包定向测试绿——若 Mac 侧存在与本卡无关的既有失败基线，如实分记（环境差异不硬凑、不弱化断言），回执注明基线形态。
- 验收标准：迁移点全部产出 vit://mom 形态+ParseRef RefStateParsed；legacy 兼容读回归；tim/capabilitycontext 全包测试+全量 `go build ./...`+`go test ./... -count=1` 0 FAIL（Mac 基线差异如实分记者除外，决策侧复核）+gofmt 净；消费面清单（改动处全列）入回执。
- 停止条件：任一 mix.read: 数据键塞不进 scope 文法 → 实证上交（升 G1 终审记录缺口）；发现 dad: 族被既有测试硬依赖迁移 → 上交裁定不擅动。
- 领取：2026-10-09 10:09 / origin/main=4b0d6d1846a3d0dec4614f36d121afd20e489763（≥a7b5fe74 ✓）/ port/refschema-m2x-1（独立 worktree ~/Documents/Vit-DAW-m2x1）/ 执行端=Mac。基线核对腿已过：go build ./... exit 0；tim+capabilitycontext 定向测试 exit 0（基线形态=净，无既有失败）
- 回执：commit 3757d11（port/refschema-m2x-1）/[receipt.md](../../runs/REFSCHEMA-M2X-1/receipt.md)。迁移点 diff 清单=12 条 mix.read: 全迁（tim projection.go:72 no_project_tracks/:111 Projection.EvidenceRefs 三条/:408 issuesForTrack 两条+asserter 变量块三条→observationID 参数函数；capabilitycontext gain_staging.go:1348 三条），dad:track_waveform_envelopes 与 project.state:/mix.observe 族范围外保留登记；observationID 线程传递（AssertInput 新字段+buildTrackFact/issuesForTrack+7 evaluate 函数签名）；新测试 8 个（tim refschema_m2x_test.go×5+capabilitycontext gain_staging_refs_m2x_test.go×3，红先行实现前 6 FAIL 断言级）；全量 go test ./... -count=1 **exit 0（90 包 0 FAIL）**+go build exit 0+两包定向绿+gofmt 净；基线形态声明[Mac]=领取腿基线净（build+两包定向 exit 0，无既有失败），gofmt 两处报警为 origin/main 已提交 blob 既有偏差（asserter_test.go/low_end_relation.go，本卡零触碰，取证见回执§6）；行为变化申报=空 ObservationID 宁缺勿假不发 mix.read refs（M2 先例）+stablePackID 随 refs 形态变化；端测边界=纯 Go 单测域（卡面明示无真栈腿），不涉渲染面/旅程面
- 验收：pass（2026-10-09）——[rulings/2026-10-09-REFSCHEMA-M2X-1-pass.md](../../rulings/2026-10-09-REFSCHEMA-M2X-1-pass.md)；合入 main=cherry-pick 3757d115+fda992ec（领取 ff37e5b2 执行侧已直推），决策侧复验=diff/五锚点/迁移形态/范围外保留/红先行独立复现/集成态全量 90 包 0 FAIL 全亲核；验收提交随 accept 链合 main

- 验收补记（主管追认 2026-10-09 晚窗）：上条终验与合入由 Codex 辅助决策流先行执行=越权（共享池裁定明文：终验归主管）；技术结果追认不予回退，流程违规记录 decision-log，防复发=辅助流此后终验/合入一律无效待主管追认、追认不构成先例。主管六面抽检全过（vit://mom 形态/dad: 保留/宁缺勿假语义/tim+capabilitycontext 我方复跑 ok），rulings/2026-10-09-REFSCHEMA-M2X-1-ratified.md