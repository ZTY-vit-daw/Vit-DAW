# REFSCHEMA-M1：注册表补全——`observation:` 前缀翻译条目（G1 终审迁移计划首项）

- 池序 11（L1-1 定版迁移余量 M1，G1 终审记录 §4 首项）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P3 / ≤1h / 无（定版已完成，commit 92eae253）
- 模型分级：L1 / **flash 可接**（零行为常量表追加，REFSCHEMA-L0-2 同型先例）
- **并行域：agentprotocol 包独占——与 JOURNEY-1（scripts）/OPT-IMPL-2（chat+webui）/L1-4-IMPL-A（contextruntime/chat）均不同域，可并行**
- 依据：[G1 终审记录 §4 M1](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) + [REF_SCHEMA_V1.md §4](../../../docs/REF_SCHEMA_V1.md) + 盘点 L1-1 §2 C3/B1
- 已核实事实（领取前回查）：
  1. `observation:` 前缀（mom/evidence.go:34-39 `observationRef`，C3 类高频族）**不在** legacyPrefixRegistry——现落三态 opaque（WARN 计数积累）；与已注册的 `audio_observation:`（B2 audioclosure 指纹）是**不同物**：前者是 mixboard 观察票 ID 加头（身份族），后者是内容指纹。
  2. 盘点 §5 痛点②④：`observation:`/`observation_id:`/`.json` 剥头逻辑在 plugin_prep_worker.go:1328-1356 与 mixboard normalizeObservationRef 两处独立存在——注册条目是这两处防御解析最终退役的前置。
  3. 前缀匹配取最长（matchLegacyPrefix）：`observation:` 与 `audio_observation:` 无前缀包含关系（后者不以 `observation:` 开头），零冲突。
- 目标：
  1. `legacyPrefixRegistry` 追加一条：`{LegacyPrefix: "observation:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 C3 mom/evidence.go:34-39"}`——slot=snapshot（obs 票据身份→快照槽，与 G 类同语义；非 hash：obs_ ID 含时间戳+随机数，非内容哈希）；TargetKind 留空（承载待 L1-2 结构化键定，与 audio_observation:/fci_/fcp_ 同款）。
  2. 测试三件：legacy 翻译表新增用例（`observation:obs_20260921T120000_ab12cd34` → legacy 态、slot=snapshot、value=obs_...）；`audio_observation:` 条目不被新条目劫持（两前缀各自命中断言）；opaque 计数回归（`observation:` 头不再计入 opaque 计数、未注册头仍计）。
  3. 注释块更新：注册表注释补 M1 来源行（G1 终审记录 §4）。
- 文件域：`agent/internal/agentprotocol/refschema.go` + `refschema_test.go`（2 文件，越域即停）。
- 约束：零行为追加（不改解析逻辑/转义/文法）；ANCHOR 语义不凭记忆（领取时实锚 mom/evidence.go 现行号）；gofmt 净。
- 验收标准：`cd agent && go build ./...` + `go test ./internal/agentprotocol -count=1` 全绿 + `go test ./... -count=1` 全量 0 FAIL（注册表是共享常量面）+ gofmt blob 净。
- 停止条件：发现既有消费方依赖"`observation:` 落 opaque"的行为（如断言 opaque 计数含 observation 头）→ 锚点+消费方清单上交。
- 领取：2026-10-07 09:50 / origin/main=412820cc（本地 HEAD c01089b4 领取，main 领先 4 commit 未推）/ port/refschema-m1（PC 执行侧；实现于独立 worktree——主树检出并行流 L1-4-IMPL-A 共用，协议 §3 裁定）
- 回执：实现 commit **680dda80**（分支 port/refschema-m1，独立 worktree D:/Vit_DAW_wt_refschemam1，基于领取 commit 7c3a21e4）；新增用例 2 函数：`TestObservationPrefixDoesNotHijackAudioObservation`（双前缀各自命中断言）、`TestObservationHeadNoLongerOpaqueCounted`（头不再计 opaque + observation_id:/合成头仍计），另有三处表用例：三态表 "mom observation ticket head is legacy"、翻译表 observation: 行、注册表契约表第 15 条+计数文案。门实测：`go build ./...` exit 0；`go test ./internal/agentprotocol -count=1` ok；全量 `go test ./... -count=1` **exit 0（88 ok / 0 FAIL）**；blob gofmt 净（worktree CRLF 检出伪影已甄别——git blob LF 全净）。
  - 端测边界声明：纯常量表追加，ParseRef 属未接线运行栈的纯工具层（L0 设计），无渲染面/旅程面涉及；验收按卡面三命令口径。
  - 交付决策侧注意四点：① `docs/REF_SCHEMA_V1.md:55`"legacyPrefixRegistry，14 条"计数已过时（现为 15 条），不在本卡文件域未改，建议随 M2 或勘误卡同步；② 主树检出与并行流 L1-4-IMPL-A 共用（其卡移动 09:50 前后出现在共享索引），本卡实现按协议 §3 切独立 worktree，验收后请决策侧清理；③ 主树 `agent/internal/agentprotocol/materialstore.go` 领取前即 gofmt 不净（既有状态，未动）；④ 未推送——本地 main 领先 origin/main（领取前已有 4 个决策侧 commit + 本卡 2 个卡片 commit），推送时机留决策侧裁定。
- 验收：（裁定文件 / 验收 commit）

- 验收：**pass（2026-10-07 决策侧，rulings/2026-10-07-REFSCHEMA-M1-pass.md）**——diff 逐字核（条目五字段与卡面规格一致）+我方复跑（build/包级/消费面包 com+queryengine+materialize/全量 exit 0）+cherry-pick 680dda80→main 00f96200；worktree 与分支已清；REF_SCHEMA_V1 表一 15 条+G1 终审记录 §3 落账+CURRENT-STATE 计数同步随验收 commit；materialstore.go gofmt 既有不净记 hygiene 债不混卡。
