# ADR-AGENT-CLEANUP-0001: Agent 架构减法手术与 B4 控制路径统一

Status: accepted
Date: 2026-07-20
Supersedes: 无（但冻结 SPAL_V0_ADR.md 描述的认证路线的后续投入；SPAL_V0_ADR 保留为历史记录）
Related: PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md（保持不变，继续有效）

> **执行者须知（重要）**：本 ADR 是一次**减法手术**，不是重构。三条铁律：
> 1. **先立后破** —— 任何删除动作，必须在替代路径的测试全部跑绿之后才允许执行。
> 2. **不动骨架** —— v1 Capability Runtime（PlanningSession / CCB / Registry / Proposal-Authorization-Execution-Verification）已成功交付 B2/B3，本次**禁止**对其做任何抽象、合并或重写。发现"顺手重构"的冲动时，停下来，只做本文档明确列出的事。
> 3. **无备份不动刀** —— 仓库当前无 remote、无 upstream。Phase 0A 的离线备份未确认存在并可恢复前，禁止任何删除、revert 或清理动作。
>
> 本文档中所有行号基于 2026-07-20 的工作区状态（分支 codex/auto-mix-single-tick，含未提交改动）。执行时若行号漂移，以函数名/符号名为准。

---

## 1. Context（背景与诊断结论）

2026-07-20 完成了一次全库架构体检（三路并行代码审查：路由门控、插件控制链路、VPS skill 生成管线）。结论：

**诊断 1 —— 插件控制体验差的主因是路由门控层，不是插件工具层。**
用户的自然语言请求要穿过四层堆叠的确定性门控（详见 §4.1），其中 message_loop 的 "observe-first" 工具门在用户请求包含任何声学形容词（低频/浑浊/mud/harsh/reverb…）时屏蔽**全部**插件工具，且显式插件请求的旁路动词表缺失"用/使用/use"，并会被"低频/浑浊"类词汇直接作废。结果就是"用 TDR Nova 切掉浑浊"这类最自然的指令必然被锁进固定观察流程，agent 反复要求观察而不提供插件工具。

**诊断 2 —— plugin_grabber 层单独即可闭合"自然语言 → 正确参数写入 → 验证"的循环。**
该层已具备：参数发现（`get_plugin_parameters` + display_probe，置信度 0.50–0.92）、显示域理解（display_domain 的 min/max/unit/scale + 证据来源）、语义写入（`apply_control` 带安全钳位 + profile 签名过期检查）、写后验证（`new_value_text` 回读）。它缺的只有两样：跨工程持久化学习成果、预映像/回滚纪律 —— 两者分别由本 ADR 的 Phase 4（最小 VPS 文件）和 Phase 3（B2/B3 治理外壳 + SPALVSPPort 复用）补齐。

**诊断 3 —— SPAL/VPS 认证体系 >90% 是仪式，且 staging 产物按设计无法实装。**
为 Pro-Q 3 制作一份 skill 产生 252 MB 工作区、约 30 种 JSON 工件、10 个阶段、5 种信任状态、6 状态凭证生命周期，并要求为该插件新写约 12 个 Go 文件重新编译（vpsforge 26 个子命令中 10 个硬编码给 Pro-Q 3）。运行时真正消费的只有约 120 行 `EQV2Binding` JSON。staging 输出被明文禁止写入正式库，唯一晋升通道是手写的每插件专用晋升程序（`cmd/vpseqv2promote`，TDR Nova 硬编码）。认证体系至今只认证了一个插件（TDR Nova）。

**诊断 4 —— 三条写入路径并存，系统已在绕过自己的认证层。**
raw `set_plugin_param`（约 3 跳）、plugin_grabber（约 8 跳）、SPAL/VPS 凭证路径（约 15 跳、6 道门）并存；显示值↔归一化换算在 C++（`normalisedLogValue`）、Go（`staticEQNormalizedValue`）、探针推断三处重复实现。本周新增的 `learned_grabber_fallback`（agent/internal/chat/equalizer_capability_tool.go:589-606）是系统自己在绕开认证层的实证。

**历史约束（不可违背）**：项目已经历两次重架构（多代理设计 → v1 能力运行时），两次都失败在同一点 —— 插件参数语义映射（GUI 显示值 ≠ 归一化控制值）。周边架构两次都不是病因。因此本次**只修那个点 + 删多余层**，不做第三次重架构。

---

## 2. Decision（决策）

- **D1** 拆除 message_loop / goalrunner_chat 中针对插件工具的确定性关键词门控，将"观察优先"从**工具级硬拦截**降级为**系统提示词级引导**；把"哪些工具在什么条件下可用"收敛为单一策略模块。
- **D2** plugin_grabber + B2/B3 治理外壳（Proposal → Authorization → Execution → Verification）成为**唯一**的插件参数受治理写入路径。raw `set_plugin_param` 降级为内核内部原语，不再直接暴露给 LLM 规划器。
- **D3** SPAL/VPS 的认证经济（凭证生命周期、徽章分类学、Catalog 派生、conformance 准入、promote 二进制、单体 vps_library_v3.json）**退役删除**；SPAL 中的 Action 编译器、执行端口（预映像/回滚）、`EQV2Binding` 数据结构与数据驱动适配器**保留复用**。
- **D4** VPS 压缩为"**每插件一个 JSON 文件 + 一条 verify 命令 + 实装即拷贝文件**"的最小管线；vpsforge 的 native host（进程外 VST3 worker/witness）与 probe 采集**保留**为 verify 命令的引擎，其余仪式命令删除。
- **D5** 显示值↔归一化换算**只在 C++ 内核侧执行**；Go 侧只发语义/显示域请求、只消费回读结果，apply 路径上 Go 运行时不再生成 normalized 参数值。现状违例：`spal/vps_eq_v2.go:353-358` 经 `staticEQNormalizedValue`（`spal/vps_static_eq.go:177`）在 Go 侧产出 normalized `PhysicalParameter` —— Phase 4 改造该适配器的执行形态。
  **关键限定（防复发旧病）**：C++ 侧的换算权威是"**经 verify 验证过的映射数据**（VPS binding / 学习 profile）+ `valueToString` 往返校验"，**不是**插件自身的 `stringToValue` 单独裁决。插件文本解析不可信（对不可解析文本普遍返回 0.0、显示域与控制域普遍对不上）正是本项目两次卡壳的原始病灶；verified 映射是解药，必须保持第一解析来源。`stringToValue`/`valueToString` 只承担往返验证与兜底角色。枚举 binding 保存"可验证标签 + 经 probe 验证的 normalized 值"二者：标签用于表达与往返校验，normalized 值是 probe 实证的 ground truth，二者由 verify 命令共同盖章，不构成"第四套换算真相"。
- **D6** 观察层双层证据设计（频谱图 PNG + MOM/TOM 数值投影）**保持不变**，作为写入后的验证契约继续使用。

---

## 3. Non-Goals（明确不做的事）

1. 不重构 v1 Capability Runtime、CCB、Capability Registry。CCB 抽象等 B4 落地后有 B2/B3/B4 三个真实实例再提取。
2. 不删除 VSP Hub / `internal/vsphub` / `internal/bridge/vsp_realtime.go`（正交的传输层现代化，不阻塞任何事，不碰）。
3. 不删除任何 docs/ 下的 ADR 与设计文档；退役文档移入 `docs/archive/` 并在文件头加 `Status: superseded by ADR-AGENT-CLEANUP-0001`。
4. 不追求"agent 自动学习生成 skill"的全自动路线；skill 生成允许人工参与，但单插件成本必须降到小时级（见 Phase 4 验收标准）。
5. 不修改观察层 / 投影模型（MOM/TOM/EPM）的采集与格式。
6. 不在本次手术中新增任何多代理/Worker 结构。

---

## 4. 现状清单（执行者的地图）

### 4.1 四层门控（Phase 2 的手术对象）

| 层 | 位置 | 机制 |
|---|---|---|
| L1 工具包选择 | `agent/internal/chat/goalrunner_chat.go` `agentLoopCapabilityNames` (:1357-1521)，`agentLoopToolContext` (:1233-1253) | 纯关键词分类器决定每轮 `AllowedTools`。`plugin` 包 (:1402-1407, :1471-1473) 仅在命中 插件/效果器/均衡/压缩/混响/延迟/plugin/vst/eq/reverb/delay/grabber/param/macro 时加入。static-mix 阶段意图会删除 `track`/`mix` 包 (:1498-1510)。未命中 → 工具调用死于 message_loop.go:544-552（"未知或不允许的工具"）。 |
| L2 能力路由劫持 | `agent/internal/chat/server.go:1723-1733`，`capability_owner.go:17-85`，`inferProjectAwareCapability` (:150-180) | 提到 b2/b3/b4/静态平衡/声像布局/低频关系 → 整轮劫持进 PlanningSession，不进消息循环。message_loop.go:3794 的系统提示词强化此规则。 |
| L3 observe-first 工具门（主犯） | `agent/internal/agentloop/message_loop.go` | `messageLoopToolGuardIssue` (:4546-4594) 拦截每次工具调用；`messageLoopNaturalMixRequest` (:4897-4917) 极宽关键词网判定混音意图；命中后屏蔽表 `messageLoopMixObserveFirstBlockedTool` (:5384-5403) 含**全部**插件工具；放行表 (:5367-5382) 只有观察/读取类。被拦调用不执行，注入 `<tool_gate>` 消息 (:2435-2449) 令 LLM 重答。显式插件旁路 `messageLoopExplicitPluginOrRawRequest` (:5068-5084) 动词表缺"用/使用/use"，且被 `messageLoopLowMudPluginPrepRequest`（mix_treatment_pending.go:315-341，命中 低频/浑浊/mud 即真）在 :4562、:2292、:9181 三处作废。观察完成后同轮插件工具仍被锁 (:4591)，逃逸需下一条用户消息同时含批准词+具体量 (:5086-5101)。终局门 `messageLoopFinalIssue` (:9177-9183) 令回合无法结束。确定性预检 `preflightNaturalMixObservation` (:2135-2180) 在 LLM 发言前强制跑 mix.observe。只读屏障 read_only_observation_guard.go:10-92。已有豁免先例：equalizer 能力工具在 :4556-4558 无条件豁免。 |
| L4 执行器子门（**保留**，这是真正有价值的安全层） | `goalrunner_chat.go` :352-369/:419-422（学习需显式意图）、:335-351/:424-438（EQ 语境拦加载）；`agent/internal/harness/plugin_param_guard.go:124-172`（set 前需参数快照）；`agentloop/plugin_grabber_routing.go`（raw→grabber 语义改写，是矫正不是门） | 保留，微调见 Phase 2。 |

### 4.2 三条写入路径（Phase 3 的收敛对象）

| 路径 | 跳数 | 关键文件 | 处置 |
|---|---|---|---|
| P1 raw `set_plugin_param` | ~3 | tools/catalog.go:789 → harness.go:9440 → PluginRackControlService.cpp:2634-2737 | 从 LLM 工具目录移除，保留为内核原语 |
| P2 plugin_grabber | ~8 | plugin_grabber_routing.go → catalog.go:796 → harness.go:958,1275 → PluginRackControlService.cpp:3568-3612（profile 签名检查 :3609-3612，显示域→归一化 :1096，枚举解析 :1350） | **升级为唯一路径**，包进 B2/B3 外壳 |
| P3 SPAL/VPS 凭证 | ~15，6 道门 | equalizer_capability_tool.go:251 → vps_eq_v2_runtime.go → spal/registry.go:102 → spal/vps_eq_v2.go:98,:135 → executionports/spal_vsp.go:73,:82,:295-311,:161-168,:251,:420 → VspKernelReference.cpp | 认证部分退役；执行端口的预映像/回读/补偿回滚逻辑移植给 P2 |

### 4.3 SPAL/VPS/Forge 拆骨清单

**保留（后续复用的"髓"）：**

| 资产 | 位置 | 复用目的 |
|---|---|---|
| 进程外 VST3 宿主（worker + witness exe） | `VPSForge/native-host/` | Phase 4 `verify` 命令的执行引擎；全库最难重写的 C++ 资产 |
| 参数表面采集 | vpsforge `probe` / `ingest-surface` 子命令及其后端 | Phase 4 自动生成 bindings 草稿 |
| Action 编译器 | `agent/internal/spal/manifest.go`（`ExecutionManifest.ToActionSet`，`CompileManifest` :32，`FreezeSPALProposal` :204） | Phase 3 B4 外壳 |
| 执行端口（预映像/回滚纪律） | `agent/internal/executionports/spal_vsp.go`（Preflight :82、Apply :295-311、回读校验 :161-168、补偿回滚 :251/:420） | Phase 3 B4 外壳 |
| `EQV2Binding` / `ParameterBinding` / `EnumParameterBinding` 数据结构 | `agent/internal/spal/vps_eq_v2.go` | Phase 4 最小 VPS 文件的核心 schema |
| 数据驱动运行时适配器 | `spal.NewVPSEQV2Adapter`（消费端 `chat/vps_eq_v2_runtime.go`） | Phase 4 运行时，已数据驱动，无需改造 |
| L2 信号方向验证 | `executionports/spal_l2_signal.go` | Phase 3 验证契约可选项 |

**删除（"仪式"，Phase 5 执行，且仅在 Phase 2–4 验收通过后）：**

| 目标 | 位置 | 说明 |
|---|---|---|
| 凭证生命周期与状态机 | `agent/internal/vps/types.go`（`CredentialStatus.CanTransitionTo` :88-105、三重指纹、5 值信任分类）、`library.go` Catalog 派生 | 替换为 Phase 4 的单文件 `verified` 戳 + 单指纹 |
| 徽章分类学 | `agent/internal/vps/badge_contract.go`、`VPSForge/docs/SPAL_EFFECT_TASK_BADGE_TAXONOMY_V1.md` | 无运行时消费者（除认证机本身） |
| 晋升二进制 | `agent/cmd/vpseqv2promote/`（含指纹移植 main.go:45,169-188 与算术参数 ID main.go:190-209 两个隐患） | 整目录删除 |
| vpsforge 仪式命令 | main.go 中 10 个 `pro-q-3-*` / `equalizer-v2-*` 子命令及 `internal/vpsforge/` 对应的 ~12 个插件专用 Go 文件；preflight 的 11 工件生成 (preflight.go:223-301) 中与 verify 无关者；witness 仪式流程 | 保留 init/status/validate/probe/ingest-surface/host/serve/worker 相关 |
| FXM 准入门槛 | `measure` 命令在关键路径上的强制性 | 命令本身可保留为研究工具，从 skill 生产关键路径移除 |
| 单体库 | `%APPDATA%\Vit\Agent\vps_library_v3.json` 读写逻辑（`chat/vps_eq_v2_runtime.go` 的 resolveVPSEQV2Provider 装载处） | 替换为扫描 VPS 目录 |
| capability.equalizer.plan 的凭证解析路径 | `chat/equalizer_capability_tool.go` 中 Catalog/Credential 解析与 `plugin_learning_fallback: "forbidden"` 语义 | 该工具改为直接走 Phase 3 的统一路径，或整体退役由 B4 action 取代（执行者按 Phase 3 落地形态二选一，倾向后者） |
| staging 隔离运行时 | `pro-q-3-static-eq-stage-vps` 生成的 `staging_vps_runtime/` + PowerShell 启动器机制 | "staging 不可实装"的设计随单体库一起消亡 |
| 磁盘垃圾 | `VPSForge/staging/preflight/` 下约 19 个工作区共约 1.8 GB（含 Pro-Q 3 的 252 MB 主工作区与 9 个 FabFilter 插件的废弃工作区） | 确认 bindings 已迁移到新格式后删除；删除前将 `pro_q_3_static_eq_staging_vps.json` 的 `spal_eq_v2_staging_binding` 块（约 120 行，真正的知识资产）转换为新格式文件 |

### 4.4 已知隐患 bug（Phase 1 修复）

1. **枚举静默归零**：`VitApp/Source/Service/PluginRackControlService.cpp` 未提交的 `resolveEnumApplyValue` 第一分支接受 `param.stringToValue(requestedText)` 的结果，只要有限且在值域内。JUCE/Tracktion 对不可解析文本普遍返回 0.0，而 0.0 在值域内 → 未匹配的枚举标签**静默应用状态 0**。修复：仅当对该结果做 `valueToString` 往返、且与请求文本大小写/空白不敏感匹配时才信任；否则落入离散标签匹配分支；无标签匹配则显式报错。
2. **vpseqv2promote 指纹移植 + 算术参数 ID**：随 Phase 5 整体删除该二进制而消亡，Phase 1 无需单独修，但**在删除前禁止再次运行该程序签发凭证**。
3. **grabber fallback 缺 track 校验**：`equalizer_capability_tool.go:686-691` 的 fallback 只回传 `plugin_id`；后续 `apply_control` 若目标 track 与 inspect 时不一致，唯一防线是 C++ 侧签名门 (:3612)。Phase 3 的 B4 外壳预映像天然覆盖此洞；若 Phase 3 前该 fallback 仍在使用，在 fallback payload 中补 `track_id` 并在 apply 侧校验。

---

## 5. 执行计划（Phase 0–5，严格按序）

### Phase 0 —— 安全基线与工作区落盘（前置，1 天内）

2026-07-20 执行环境核对发现基线远比最初评估更脏：约 1,489 个未跟踪文件（总状态项 1,503），其中包括本 ADR 要复用/删除的**核心实现本身**（`equalizer_capability_tool.go`、`vps_eq_v2_runtime.go`、`spal/manifest.go`、`spal/vps_eq_v2.go`、`executionports/spal_vsp.go`、`vps/types.go`、`vps/badge_contract.go`、`cmd/vpseqv2promote`、`cmd/vpsforge`、`VPSForge/native-host` 源码、本 ADR —— 全部未入 Git）；仓库无 remote/upstream；根仓库缺 `.gitmodules` 但保留 3 个 gitlink（`git submodule status` 直接失败）；`tracktion_engine` 内部有 6 文件约 276 行修改且 `modules/juce` dirty。因此 Phase 0 扩展为三步：

**Phase 0A —— 不可丢失备份（先于一切，对应铁律 3）：**
1. 对根仓库与 `tracktion_engine` 分别制作离线备份：最低要求 `git bundle` + 工作区完整副本到仓库外介质；
2. 备份用户级 `%APPDATA%\Vit\Agent\vps_library_v3.json`（约 4.1 MB）；外部备份 `VitApp/Workspace/Settings/Settings.xml` 与 `VitApp/Workspace/default_project.xml`（运行态数据：插件扫描结果、实例 ID、默认工程内容）；
3. 对全部未跟踪文件三分类：**source**（入 Git）/ **knowledge asset**（binding 等知识资产，转换后入 Git）/ **generated artifact**（不入 Git，本阶段不删除）；
4. 本阶段不删除任何 staging、artifacts、VPS 库或工程文件。

**Phase 0B —— 架构基线提交：**
把已交付但未跟踪的现状代码入库：B2/B3 能力运行时、SPAL/VPS/Forge 现状代码、VSP Hub、对应测试/脚本/设计文档（含本 ADR）。exe、`artifacts/`、`.vit_history/`、staging 生成物、运行态 XML 不入库（同步补 `.gitignore`）。此提交是后续一切 diff 的参照系。

**Phase 0C —— 拆分现有已跟踪修改：**
1. 提交 A（资产，**在 0B 基线之后提交**，保证可独立 cherry-pick）：2026-07-20 实装测试修复的 6 个真 bug —— 波段 token 碰撞 `hasBoundedBandToken`（auto_learn.go:322-337）与扫描上限 8→32（:18）、数字段序 `eqBandIndex`（:205-211）、开关误绑 `booleanToggleBonus`（:303-308）、枚举域降级 `looksLikeEnumDisplayDomainText`（display_domain.go:123-129）+ 测试（plugin_skill_test.go:277）、C++ 枚举写入路径（PluginRackControlService.cpp 约 216 行 diff）、forbidden 措辞死路缓解（equalizer_capability_tool.go:601-602 + server.go 提示词）；
2. 提交 B：message_loop.go / harness.go 中的混音观察与 waveform-bake 守卫调整、smoke 脚本等；
3. `tracktion_engine` 修改单独封存（本地分支或 patch 文件），gitlink/`.gitmodules` 状态单独修复或明确封存备案，**不得混入删除手术**；
4. `Settings.xml` / `default_project.xml` 外部备份确认后 revert，不提交运行态数据；
5. 全部手术在新分支进行，每个 Phase 结束打 tag。

**验收**：离线备份存在且做过一次恢复演练；`git status` 干净；`agent/` 下 `go test ./... -count=1` 与 0 号基线一致（78 包全绿）。

### Phase 1 —— 隐患修复（1 天内）

1. 修复 §4.4-1 枚举静默归零（含往返校验），在 Go 侧 plugin_skill_test.go 或 C++ 侧补测试：不可解析标签必须报错而非应用状态 0。
2. 若 grabber fallback 在 Phase 3 前仍是活路径，补 §4.4-3 的 track_id 校验。

**验收**：新增测试跑绿；现有 plugingrabber 测试全绿。

### Phase 2 —— 路由止血 + 门控策略模块化（1–2 天）

**2a 止血（当天可验证）：**
1. `messageLoopExplicitPluginOrRawRequest`（message_loop.go:5068-5084）动词表补 `用 / 使用 / use / apply / 通过`；新增"点名插件"检测：对象表已有 tdr/nova/zl 等，扩展为可识别已加载/已学习插件名的检测器。
2. 收窄 `messageLoopLowMudPluginPrepRequest` 的作废逻辑：**仅当未点名任何插件时**才允许 低频/浑浊/mud 词汇取消显式插件旁路（三处调用点同步：message_loop.go:4562、:2292、:9181）。
3. 仿照 equalizer 豁免先例（:4556-4558），将携带完整 `track_id`+`plugin_id`+已学习控制的 `plugin_grabber.apply_control` 加入豁免；或等价地从 `messageLoopMixObserveFirstBlockedTool`（:5390-5398）移除 grabber 工具。安全兜底由 L4 承担（学习需显式意图、参数快照前置、profile 签名、确认流程 —— 全部保留）。
4. `agentLoopCapabilityNames`（goalrunner_chat.go:1402-1407）：点名第三方插件（即使无"插件"字样）也必须选入 `plugin` 包，否则 L3 修完仍死在 L1。
5. 观察后同轮锁（:4591）与严苛确认句式（:5086-5101）：放宽为"观察结果可用 + 用户本轮明确指向插件动作"即可放行；"必须先观察"的原则移入系统提示词（:3782/:3789 区域改写为引导性而非禁止性措辞），由 LLM 自行权衡。
6. 更新测试：`plugin_grabber_routing_test.go`、`read_only_observation_guard_test.go`、`message_loop_test.go` 广义混音门用例、仿 `equalizer_capability_guard_test.go` 新增"点名插件 apply 不被拦截"用例。**新增回归用例（必须）**："用 TDR Nova 切掉 200Hz 附近的浑浊" 必须在单轮内到达 `plugin_grabber.apply_control` 调用（可被 L4 确认流程暂停，但不得被 observe-first 门拦截）。

**2b 策略模块化（防复发，本 Phase 内完成）：**
门控裁决散落在两个巨石文件（message_loop.go 9000+ 行、goalrunner_chat.go ~2000 行）的几十个函数里，是"修一门长一门"的病灶。新建独立包（建议 `agent/internal/toolpolicy/`）：
- 单一入口 `Decide(turnContext, toolCall) -> Allow | Deny(reason)`，内部为**一张可读的规则表**；
- L1 的包选择与 L3 的门控裁决迁入该包；message_loop / goalrunner_chat 只调用裁决，不再各自持有关键词表；
- 每条规则必须有对应表驱动测试；
- 迁移采用"新包裁决 + 旧逻辑断言一致"的影子模式跑一轮测试后切换，避免行为漂移。

**验收**：上述回归用例绿；既有全量 agent 测试绿；用真实会话验证"点名插件 + 声学形容词"不再触发观察死锁。**此 Phase 完成即可实测验证"grabber 直连可自由控制插件"的核心假设，结果反馈决定 Phase 3 细节。**

### Phase 3 —— B4 治理外壳：grabber 包进 B2/B3 机制（2–4 天）

按既定 B4 决策（memory: vit-b4-effect-control-path-decision）执行：

1. 新建轻量 B4 action 包裹 `plugin_grabber.apply_control`，复用 B2/B3 的 Proposal 冻结 → Authorization → Execution → Verification 外壳。CCB 按既定决策手写第三个采集函数（类比 `acquireCapabilityCanaryB2Context`，见 capability_runtime_canary.go），**不抽象**。
2. Execution 端口移植 `SPALVSPPort` 的纪律到 grabber 写入：写前预映像捕获（参照 spal_vsp.go:73）、写后回读比对（参照 :161-168，容差 1e-4 或按参数类型定）、失配时向预映像补偿回滚、补偿失败即 fail-closed（参照 :251/:420）。
3. Verification 契约接入双层证据：频谱图 PNG 前后对比（方向）+ MOM/TOM 数值投影（定量），两者一致才 pass（memory: vit-spectrogram-two-tier-observation）。
4. raw `set_plugin_param` 从 LLM 工具目录（tools/catalog.go:789 的注册处）移除；`plugin_grabber_routing.go` 的语义改写随之简化或删除。
5. `capability.equalizer.plan` 的处置：Phase 3 改为**薄兼容层** —— 保留工具名，内部委派给 B4 action；其 Catalog/Credential 解析与 `plugin_learning_fallback: "forbidden"` 语义就地废弃。Phase 5 再整体删除该工具。（先立后破，风险低于立即退役。）

**验收**：B4 action 在真实工程上完成一次"提案→授权→写入→回读→双层证据验证"全流程；人为注入一次参数身份错误（改错 parameter_id）验证回滚触发；B2/B3 既有测试不受影响。

### Phase 4 —— 最小 VPS 管线（2–3 天）

1. **格式**：定义 `<plugin>.vps.json`（约 30–150 行/插件）：
   - `plugin`: name、format、version、**单一** installation hash（三重指纹简化为一）；
   - `capability`: 如 `equalizer.v2`；
   - `bindings`: 语义槽位 → `{parameter_id, unit, min, max, scale}` 或 `{parameter_id, values: {label: normalized}}`（即现有 `EQV2Binding` 形状；结构校验复用 vps/types.go:595-601 处 `CredentialConformance.validates` 已调用的校验逻辑，剥离凭证包装。**注意**：受 D5 约束，`NewVPSEQV2Adapter` 不能按现状直接复用 —— 现状它在 Go 侧产出 normalized 值（见 D5 违例说明），需按第 4 点改造执行形态）;
   - `verified`: bool + 指纹 + 时间戳（由 verify 命令写入，人工不得手填）；
   - 可选 `bounds` 安全钳位覆盖。
2. **生成**：`vpsforge draft <plugin>` —— 从现有 probe/ingest-surface 的 `surface_snapshot.json` 自动生成 bindings 草稿（标签、显示文本、单位已在采集数据中），人工只做核对与语义命名。
3. **验证**：`vpsforge verify <file>` —— 用现有 native worker 加载插件，对每条 binding 执行：写入→**新鲜**回读→显示文本比对（min/mid/max 三点；枚举全档扫描）→状态还原；全通过则盖 `verified` 戳+指纹。
4. **实装与执行形态（落实 D5）**：agent 启动时扫描用户级 VPS 目录（如 `%APPDATA%\Vit\Agent\vps\`），替代 vps_library_v3.json 单体库；VPS 文件的 bindings 下发给内核、并入该插件的运行时 profile，成为 C++ 侧换算的数据源。Go 适配器改造执行形态：编译产物为"语义/显示域指令 + 枚举标签 + binding 引用"，**不再生成 normalized 值**；最终 normalized 解析在 C++ 完成 —— 数值参数复用 grabber 现有 display_domain→normalized 路径（PluginRackControlService.cpp:1096），枚举参数走"verified 标签表优先 + `valueToString` 往返校验"路径（即 Phase 1 修复后的 `resolveEnumApplyValue`）；VPS 文件中的 min/max/scale 在 Go 侧仅用于范围预校验、提案展示与 verify 证据，不作运行时换算权威。运行时保留唯一廉价安全检查：现场指纹/参数表面摘要与文件不符 → 拒绝派发。**这同时解决 grabber profile 只有工程作用域的问题**：学习/验证产物落为用户级文件，工程只引用。
5. **迁移**：把 TDR Nova 现有凭证的 binding、以及 Pro-Q 3 staging 文件 `VPSForge/staging/preflight/fabfilter-pro-q-3-20260717-r3/pro_q_3_static_eq_staging_vps.json` 中的 `spal_eq_v2_staging_binding` 块，转换为新格式并跑 verify。

**验收**：TDR Nova 与 Pro-Q 3 两个文件 verify 通过并被 B4 action 实际消费完成一次参数写入；**从"新插件"到"agent 可用"全程 ≤ 2 小时且零 Go 代码改动**（这是对 1.5 天/插件病灶的直接验收）。

### Phase 5 —— 删除手术（1–2 天，仅在 Phase 2–4 全部验收后）

按 §4.3 删除清单执行，顺序：
1. 删 `cmd/vpseqv2promote/`；
2. 删 vpsforge 的 10 个插件专用子命令与 `internal/vpsforge/` 对应源文件、witness 仪式、preflight 冗余工件生成；
3. 删 `internal/vps/` 的凭证状态机、徽章、Catalog（保留被 Phase 4 复用的结构校验函数，随迁至新包）；
4. 删 vps_library_v3.json 读写路径与 staging 隔离运行时机制；
5. 处置 `capability.equalizer.plan`（按 Phase 3 决定）；
6. 磁盘清理：确认 bindings 迁移完成、且 Phase 0A 离线备份仍然有效后，删 `VPSForge/staging/preflight/` 下约 1.8 GB（约 19 个工作区）；同时备份后移除 `vps_library_v3.json` 及其历史备份的运行时读取路径；
7. 文档归档：SPAL_V0_ADR.md、SPAL_VPS_V3_FOUNDATION.md、SPAL_LAB_V0_MANUAL.md、SPAL_REFERENCE_EQ_PRODUCT_PATH_V0.md、FXM_AND_VPS_AUTHORING_V1.md、README_FOR_VPS_AUTHORING_AGENTS.md、VPSForge/docs/ 徽章分类学 → 移入 `docs/archive/`，文件头标注 superseded。
8. 每删一层跑一次全量测试（Go：`agent/` 全部；C++ 侧现有 smoke：scripts/run_vit_product_path_smoke.ps1 与 scripts/SMOKE_TESTS.md 所列）。

**验收**：全量测试绿；`grep -r`（不区分大小写）确认无残留对已删符号的引用；B2/B3/B4 三个能力在真实工程各跑通一次。

---

## 6. 风险与回滚

| 风险 | 缓解 |
|---|---|
| Phase 2 拆门后 LLM 出现不受控写入 | L4 全保留（快照前置、显式学习意图、确认流程）+ Phase 3 预映像/回滚是最终防线；Phase 2 与 Phase 3 之间不做公开演示 |
| 删除时误伤复用件 | §4.3 保留清单为白名单；删除 PR 中任何触及白名单文件的改动需单独说明 |
| 行号漂移导致改错位置 | 一律以符号名定位；执行前 grep 校验 |
| 影子模式发现新旧裁决不一致 | 不一致处逐条列出人工裁定，禁止静默取旧或取新 |
| 每 Phase 独立 tag | 任一 Phase 失败可回退到上一 tag，已验收 Phase 不受影响 |

## 7. 成功标准（整体）

1. "用 <插件名> 处理 <声学问题>" 单轮到达受治理的 apply（可暂停于用户确认，不得死于观察门）。
2. 插件参数写入路径全库唯一，预映像/回读/回滚纪律生效，参数身份错误可被检出并回滚。
3. 新插件接入 ≤ 2 小时、零编译。
4. agent/ 下与插件控制相关的 LOC 净减 ≥ 20k（SPAL/VPS/Forge 路径现约 30–35k，保留件约 5–8k）。
5. B2/B3 无回归。
