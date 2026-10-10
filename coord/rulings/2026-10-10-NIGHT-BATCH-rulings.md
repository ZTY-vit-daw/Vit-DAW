# Ruling：2026-10-10 夜间批验收——LEDGER-PERSIST/M5/SCRIPT-HYGIENE 三卡 pass（含 exit-0 张力裁定与 blocker 解除条件裁定）

- 裁定：三卡全 **pass**。cherry-pick `db6f2b11→fc5ff01d`、`9d80a573→2f2d0531`、`a487251e→5679d3c9`；合并态 `go build` exit 0+**全量 92 包 0 FAIL** 我方复跑。
- 四层亲核：diff 亲读（LEDGER 核心 hunk：freeStateNormalizeMapAny JSON 往返+边界观测日志；M5 五词条；SCRIPT-HYGIENE 五项）/工件核对（LEDGER 取证双 run+回归 run 203348+REVIEW 六节；M5 消费链清单+R5 复跑；SCRIPT 干跑探针）/锚点核对（ccb_observation.go:110-113 结构体嵌入实证、:814 死通道、G6/G8 门位置）。

## 1. FS-LEDGER-PERSIST-1：pass（blocker 主缺陷修复验证成立）

- **双层根因采信**（真栈插桩实证链完整）：①chat `freeStateCCBObservations` 严格 map 断言丢弃 harness 结构体 bundle（仅 RecentObservation 兜底存活末笔→早轮 mix scan 收据到不了台账→G3 误杀）；②Result 快照不承载 loop（contextruntime.Build 摘要投影→:814 导入是结构性死通道）；③cycle=0 归因修正为非缺陷（只计已执行动作，语义一致）——复核腿同判。
- **修复验证三面**：A2 站图 fail→**PASS** 翻转（缺陷轮 rounds=0/observations=0 → 修复轮 1/1/1）；终态对照 receipt_count 1→2、G3 fail→pass、loop capability_blocked→awaiting_experiment 存活、closure fs2→fs4、跨轮 stored_receipts=2；最小反例四测+取证双 run 工件。
- **exit-0 张力裁定（上交项①）**：卡面"exit 0"与"A3 视模型行为如实分记"的内部冲突系发卡侧措辞缺陷（本 ruling 认账）。裁定：本卡验收目的=**缺陷修复验证**，其判据=A2 翻转+终态对照+跨轮存活实证（已满足），exit-0 不作为本卡验收条件；A3 FAIL=模型证据完备性行为（G6/G8 拒提案，G3 已过），**非台账缺陷残留**——复核判读采信，分记诚实。**SMOKE 全绿（含 A3）维持为汇合点收口条件**（见 §4）。
- 上交项②（栈登记时间笔误）：已随批订正，非违规。

## 2. REFSCHEMA-M5：pass

五词条注册（22→27）+兄弟族 plugin_load_batch{,.reconcile} opaque 钉住余量；消费链=纯响应面零解析消费→注册归一（两形态分记正确）；R5 登记不动作复跑实证；G1 终审 M5 行批注+R5 注记回写核实。**G1 迁移账 M5 行完结。**

## 3. SMOKE-SCRIPT-HYGIENE-1：pass

五项全落地+回归零弱化逐块复查；**栈所有权守卫**（pre-net throw 早于清场时无条件拆栈可误杀他人栈——实现中新发现隐患的修复）为超卡面正值项，采信；语法 0 错+干跑探针实证；agent_binary/source 扫描的运行时行为静态审查边界如实声明。

## 4. 汇合点 blocker 解除条件裁定

- **缺陷面解除**：EVENING-BATCH §5 的 blocker（台账持久化缺陷）**修复验证成立，解除**。
- **三线启动**：维持"SMOKE 回归通过前不启动"原裁定字面已不适用（A3 为概率面非缺陷面）。主管建议=**放行三线启动**（A1/A2/A4 实证链路健康，A3 概率面与三线并行追：多 run 可期/证据引导调优另议）；**最终放行决定权留用户**（AB 试听往返是你定义的烟测验收口径组成部分，全绿 run 作为汇合点收口条件保留）。
- A3 挂账登记：61 轨工程上模型提案被 G6/G8 证据完备性门拒绝（G3 已过、loop 存活、诚实 settle 7 轮/6 nudge 未挂卡）——概率面，调优方向=证据引导，待用户裁。

## 5. 后续卡两张（随本 ruling 立卡）

1. **HARNESS-STRUCT-NORMALIZE-1**（P3）：harness 结构体嵌 map 契约源侧归一化（LEDGER 域外发现）。
2. **FS-BLOCKED-SURFACE-CLOSURE-1**（P3）：capabilityBlockedBoundaryResponse 闭包分支扩面（机制定位已随 LEDGER 回执入档）。
