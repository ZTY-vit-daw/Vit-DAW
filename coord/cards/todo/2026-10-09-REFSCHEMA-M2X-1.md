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
- 领取：（时间 / origin/main hash / 分支名 / 执行端 PC|Mac）
- 回执：（commit hash / 迁移点 diff 清单 / 新测试名 / 全量退出码 / 基线形态声明[Mac]）
- 验收：（裁定文件 / 验收 commit）
