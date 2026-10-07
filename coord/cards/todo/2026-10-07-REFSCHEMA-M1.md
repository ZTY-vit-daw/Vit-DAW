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
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 新增用例名 / 全量测试退出码）
- 验收：（裁定文件 / 验收 commit）
