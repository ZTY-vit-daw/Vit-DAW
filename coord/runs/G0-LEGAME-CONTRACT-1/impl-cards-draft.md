# 实施卡草案 ×3 — vit.legame_design.v0 落地（W01 实施拆分，候主管确认发卡）

- 来源：G0-LEGAME-CONTRACT-1（池序 370）产物；契约权威=`docs/LEGAME_DESIGN_CONTRACT_V0.md`（候审）
- 对齐：审计 WORK-PACKAGES W01 五单元（源 schema 校验 / 材料引用相对路径 / 新旧映射 / 修订绑定缓存失效 / 导出移动重开兼容）
- 前置：**§7 六项候用户裁定未回填前，三卡不得领取开工**（契约 §9；草案默认值未获授权）
- 依赖链：W01-A → W01-B → W01-C（串行；A/B 纯 Go 单测可并行勘察，C 涉真实栈验收串行占栈）

---

## W01-A 设计源解析与语义校验门

- 目标：新包实现 `vit.legame_design.v0` 的 Parse+Validate，输出 E/W/I 聚合诊断；不改任何现有包行为。
- 文件域：`agent/internal/legamedesign/`（新建；解析/结构/校验/诊断），fixture 测试目录。**禁改** `agent/internal/edu/lesson_card.go`。
- 要点：
  1. yaml `KnownFields(true)` fail-closed（未知字段拒绝，与 lesson_card 同口径）；schema_version 判层（未知值拒绝，保留值 vit.project_intent.v0 仅登记不实现解析路径）。
  2. 按 schema 草案逐键落结构体；L1 五必填段检查；§5.2 规则号逐条实现（E-SCHEMA/E-REQ/E-REF/E-MAT(path 部分)/E-CAP(表接口 stub)/W-CHK/W-MAT/W-BUDGET/I-NOTE）。
  3. 诊断聚合输出（规则号前缀可机读+人读路径）；禁止静默纠正。
- 验收（全部退出码 0）：
  - `go test ./internal/legamedesign/ -count=1`：正例（设计源 music-creation-unit2 fixture 副本，0 E）+每条 E/W 规则至少 1 负例+未知字段/未知枚举/引用悬空负例。
  - `go build ./...`。
  - fixture 用设计源副本（`t.TempDir()` 落盘），不改 `coord/lesson-designs/` 原件（§10 纪律）。
- 停止条件：校验门规则全部有测试对账；诊断格式定稿；不做编译器（归 B）。
- 非目标：不碰 projectpackage/chat；不做非乐局注入。

## W01-B 编译器 design→level_card + 缺项诊断 + manifest

- 依赖：W01-A 合入；§7 R3/R4/R5/R6 裁定回填。
- 目标：实现 §6.2 映射表 T-1..T-21（按裁定修订后的版本），输出 LevelCard+compile manifest；终检复用现行 `ValidateLevelCard`。
- 文件域：`agent/internal/legamedesign/compile*`（或主管指定 `internal/educompile` 新包）；**单向依赖** compile→edu；禁改 edu/projectpackage。
- 要点：
  1. T-14 判定面三分支白名单（deterministic→clip_exists/track_non_empty/tempo_set 结构叶；rubric→verbal_rubric；user_judgment→event 映射登记表）；白名单外一律降 advisory+W-CHK，**禁止从 criterion 文本生成判定**。
  2. T-13 材料准入（file+catalog 核验状态→assets+sha256）；T-11 clip_origins/bar_span 推导；T-17 write_boundary 仅由 CAP-BRIDGE 闭包表产生（表=登记接口，本卡给出表结构与 first-course 登记条目，接线归 C）。
  3. manifest 四元组键（源 SHA-256/编译器版本/装配输入清单哈希/卡 schema 版本）+W 诊断清单；失效策略按契约 §6.5（含 fail-closed：重编译失败保留旧缓存但阻断使用）。
- 验收：
  - `go test ./internal/legamedesign/... -count=1`：music-creation-unit2 fixture 编译产物通过 `ValidateLevelCard`（0 拒绝项）；W 清单与设计源 pending_questions/未备料清单对账；缓存键变化→重编译判定单测；手改 compiled 卡→manifest 校验拒绝单测。
  - `go build ./...`。
- 停止条件：映射表全行有测试；manifest 格式定稿；不做 chat/装配接线（归 C）。
- 非目标：不改 level_card 校验门；不承诺自然语言判据自动可判定。

## W01-C 工程包装配绑定（H3 收口腿）

- 依赖：W01-B 合入；真实栈排位（按 §5/§9 占栈纪律，烟测腿串行）。
- 目标：把编译产物接进工程包链路——M1 导入入口（§15 方案 A）触发解析→编译→装配；binding UUID 化；缓存四元组生效；材料相对路径随包迁移。
- 文件域：`agent/internal/projectpackage/`（发现/装载/编译触发点）、`agent/internal/chat/edu_jam_chat.go`（binding 读取点——`legameBindingPath` 父目录静态推导改 UUID 目录隔离）、`scripts/`（新增装配烟测段，扩展既有 ps1 体系）。**此卡文件域与在飞卡重叠风险高，领取前须主管核对文件域不相交。**
- 要点：
  1. 绑定键=工程 UUID（修复 A.vit/B.vit 同父目录串 binding，H3 审计口径）；缓存禁 level_id 单键。
  2. 装配输入清单（材料文件 sha256+catalog 哈希）进 manifest 键；导出/移动/重开后键重算→命中或重编译（W01 单元 5 兼容性）。
  3. CAP-BRIDGE 闭包表→write_boundary 实际命令面接线。
- 验收（真实栈烟测，退出码 0 为交付门槛，§5）：
  - 扩展现有冒测脚本：导入设计源 YAML→编译装配→agent HTTP 接口验证乐局装载、binding 指向正确工程、region/alias 寻址非轨首推导；**A/B 两个工程不串 binding，重开不切到别版**（审计 W01 acceptance 原文）；revise 源文档→缓存失效→新卡生效。
  - recents 快照还原纪律（§10 装置腿条款）按烟测脚本既有约定执行。
- 停止条件：三断言（不串 binding/重开不切版/修订失效）真实栈 PASS；越界缺陷（如内核侧问题）停手上交，不扩域修补。
- 非目标：每轮注入语境（W02）；快照语义（W02）；YuE2 链路（372）。

---

## 发卡前检查单（主管用）

1. §7 R1–R6 裁定结论回填契约 §10 修订记录；
2. 解释备案①（workspace.regions 不提升）与 T-12 kind 映射提案值无否决；
3. W01-C 与在飞卡文件域核对（367 SHELL-6、369 worktree 均不涉 projectpackage/edu_jam_chat，当前无冲突，领取时复核最新池）；
4. 卡 ID 建议：W01-A=`G0-LEGAME-PARSE-1`、W01-B=`G0-LEGAME-COMPILE-1`、W01-C=`G0-LEGAME-BIND-1`（池序续排）。
