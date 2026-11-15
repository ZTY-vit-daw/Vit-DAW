# G0-LEGAME-CONTRACT-1：W01 乐局设计源契约冻结草案——vit.legame_design.v0

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=审计 H1+[lesson-designs 课程 YAML](../../lesson-designs/music-creation-unit2/design.v1.yaml)；无工期估算）
- 派发确认：已确认（主管发卡）
- 验收负责人：主管决策流（+用户过目关键裁定项）
- 池序 370；P0；依赖=无；PC

## 目标

1. 以 `coord/lesson-designs/music-creation-unit2/design.v1.yaml`（revision 2，含 2026-11-12 裁定：调号 key_signature/拍号两层/学生作品快照归属）为输入，冻结 `vit.legame_design.v0` schema：最小意图层（字段全可选）+乐局层（objectives/materials/regions/checks/flow 必填段）分层定义（2026-11-12 §15 裁定口径）。
2. 与现有 `vit.level_card.v0`（lesson_card.go，KnownFields 拒未知字段）的迁移/适配契约：编译映射、缺项诊断、卡缓存失效策略（H3）。
3. 待用户裁定项清单（如模糊语义/两可设计点）单列，不擅自定夺。
4. 产出契约文档+schema 草案文件（`docs/` 或 `coord/`）+实施卡草案（解析/校验/装配绑定的开发卡）。

## 非目标

- 不改生产代码；不实现解析器。

## 文件域

只读+文档产出（coord/runs/G0-LEGAME-CONTRACT-1/ + 契约草案落 docs/ 候审）。

## 验收标准

schema 草案完整覆盖设计源 YAML 全字段+迁移契约可执行+裁定项清单；主管过目后转用户裁定关键项。

## 领取/回执/验收：执行完成候验收（done）

### 领取

- 领取时间：2026-11-15 深夜
- 领取时 origin/main hash：`9cc3048ccdda11c3f889476ebfd85eea3cd46c25`（本地 HEAD=origin/main，无被顶）
- 执行 owner：GLM-5.3-Flash（ZCode 免费车道量产执行子代理）/ PC / 本会话（派发提示词单腿任务）
- 分支/工作树：主工作树 `main` @ `D:\Vit_DAW`（派发指定工作目录；纯文档卡，无生产代码文件域）
- 领取前工作树已有改动（权威输入，本卡不触碰）：`M CURRENT-STATE.md`、`M VitApp/Workspace/Settings/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`M coord/decisions/2026-11-11-legame-demo-experience-first.md`、`M tracktion_engine`、若干未跟踪文件
- 领取提交：hash 见收尾回执回填

### 回执（2026-11-15）

- 产物：
  - 契约文档（候审正本）：docs/LEGAME_DESIGN_CONTRACT_V0.md
  - schema 草案：coord/runs/G0-LEGAME-CONTRACT-1/legame-design-v0.schema-draft.yaml
  - 覆盖性核验：coord/runs/G0-LEGAME-CONTRACT-1/design-source-coverage-check.md（附全树路径清单 design-source-paths.txt 589 条）
  - 实施卡草案 ×3：coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md（W01-A 解析校验门 / W01-B 编译器与 manifest / W01-C 装配绑定）
  - 运行记录：coord/runs/G0-LEGAME-CONTRACT-1/PROGRESS.md
- 要点交付：
  1. vit.legame_design.v0 两层 schema（L0 最小意图层全字段可选 / L1 乐局层 objectives/materials/checks/flow+workspace.regions 五硬必填段），完整覆盖设计源 YAML revision 3 全部 17 顶层段 119 条结构字段路径（机械核验 119/119，14 枚举面实测值⊆契约闭集，引用完整性规则实例执行违例 0）；
  2. 与 vit.level_card.v0 迁移/适配契约：单向编译映射表 T-1..T-21（编译产物必须过现行 ValidateLevelCard；禁止从自然语言判据生成判定器——审计 H1 纪律）、缺项诊断三级 E/W/I 带规则号、卡缓存失效四元组键（源 SHA-256/编译器版本/装配输入清单哈希/卡 schema 版本）+manifest 外置——落实审计 H3 level_id 单键与 binding 父目录串工程两项修复口径；
  3. 候用户裁定项 R1-R6 单列（非乐局保留值/判层解读/track_index 指派/多 evidence_checks 拆分/budget-tempo 空值策略/key_signature 词表），未擅自定夺；另两点解释备案供主管否决；
  4. 实施卡草案 3 张对齐审计 W01 五单元，前置条件=裁定项回填后才可发卡领取。
- 边界声明：本卡不改生产代码、不实现解析器（非目标既定）；未跑 go/npm 测试——验收标准为文档契约完备性，以机械覆盖核验（python -I 只读）自证；落 docs/ 的契约属候审文档，CURRENT-STATE.md 索引登记留给决策侧验收时处理（该文件有领取前他人改动，本卡未触碰）。
- 提交：领取 9a2994f3（pull --no-rebase 合并 3997e11b，369 卡并发领取冲突已按双方 doing 态解决）；收尾 50474de2（POOL-INDEX 两度并发冲突按双方状态合并 90e4a8a6/e00cbf41；最终远程 main=e00cbf41 含本卡全部产物）。
- 验收：待主管决策流过目（候验收）；§7 R1-R6 候用户裁定。

### 验收（主管 2026-11-15 深夜）

- 裁定：**技术面 pass**——docs/LEGAME_DESIGN_CONTRACT_V0.md（34 节）：两层 schema（L0 全可选/L1 五必填段）+17 顶层段契约表+T-1..T-21 编译映射+装配绑定契约（UUID binding/clip_alias 作业身份/缓存失效四元组）+设计源 revision 3 覆盖核验 **119/119 结构路径+14 枚举面无溢出**（python -I 只读脚本两轮在卷）；与 §13/§14/§15+H1/H3 逐条对照；零生产改动；并发冲突三度按协议合并零丢失。
- **R1-R6 候用户裁定**（契约 §7）：R1 非乐局保留值/L0 子集、R2 判层机制（schema_version 必填键解读）、R3 track_index 指派、R4 objectives 多 evidence_checks 编译、R5 budget/tempo 空值、R6 key_signature 词表封闭度+music_region 归一——**主管将整理带建议裁定单转用户**。
- **下游 W01-A/B/C 实施卡在 R1-R6 回填前不发**（执行侧已守）。
- CURRENT-STATE 索引登记随 gate（同 371 原因：既有未提交改动不混入）。
