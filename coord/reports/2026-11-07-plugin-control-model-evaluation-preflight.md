# PLUGIN-CONTROL-EVAL-PREP-1 preflight：当前模型效果器控制实验首轮勘察

- 日期：2026-11-07（Asia/Shanghai）；执行：GLM L2-worker 子代理（ZCode/PC）；卡：`coord/cards/doing/2026-11-07-PLUGIN-CONTROL-EVAL-PREP-1.md`（领取提交 a93dd97c，已推 main）。
- 依据：[实验方案](../../reports/2026-11-07-plugin-control-model-evaluation-plan.md)、[同日效果器笔记 v5](2026-11-07-effect-control-future-brainstorm.md)、[准入后置裁定](../decisions/2026-11-07-pca-admission-after-control.md)。
- **本卡为零运行只读勘察**：无模型能力调用（本文引用的遥测为既有文件只读采样）、无栈启停、无生产改动。**本卡不产生任何模型成绩与真栈结论。**
- 证据三级标注：`[静态核对]`＝读当前代码/配置可得；`[历史工件]`＝既有 run/attestation 遥测与回执；`[待新运行验证]`＝只能由后续获准运行核验，本文不冒充已验证。

## 0. 执行基线

- worktree `D:/Vit_DAW_wt_299prep`，分支 `port/plugin-control-eval-prep-1`；领取基线 HEAD=`b352ba898c0f5016962d7c4b5e5d3d5fbfdd8014`（=origin/main，fresh worktree status 全净、diff --stat 空）；领取提交 `a93dd97c` 已推 main 并 fetch 重读确认。
- 主管三裁定遵守：①S1 付费+真栈轮不随本卡授权；②A/B 若需 test-only 披露缝＝改代码卡（挂「答辩前不落实」边界，用户拍板）——本文只列锚点不动代码；③S1 分段 P→A→B→C，组间 go/no-go，RUNBOOK 按段出预算。

## 1. 实际被测模型清单（核对到静态+遥测层，响应模型待首轮核验）

**配置面 [静态核对]**

- 配置来源优先级（`agent/internal/config/config.go:154-236`）：`~/.vit/config.json` 为主；不完整字段回落环境变量 `VIT_AGENT_LLM_BASE_URL/OPENAI_BASE_URL`、`VIT_AGENT_LLM_MODEL/OPENAI_MODEL` 等；**环境变量可整体覆盖文件值**——实验跑前必须记录启动 shell 的这三组变量是否设置。
- 本机当前 `~/.vit/config.json`（只读读取，密钥不入库）：`baseUrl=https://api.deepseek.com/v1`，`defaultModel=deepseek-flash`，无自定义 APIKey 之外的凭据形态；multimodalRoutes 全部 `provider=default`（同 baseUrl 同 key 复用主路由），仅 `text` 与 `image_understanding` enabled，其余 disabled；browser.enabled=false。
- **fallback：配置面未见任何 fallback 模型链**（RouteConfig.Fallback 全空；代码里 Fallback 字段存在但当前配置未启用）`[静态核对]`。重试语义：`llm/client.go` 有 bounded retry（transport/HTTP 可重试错误），不换模型 `[静态核对]`。
- 调用位置：agentloop message loop、semantic entry、各 typed planner（telemetry source 面所见）均走 `config.EngineConfig` 同一客户端；**路由不按任务分流**（除 image_understanding 独立路由配置外）`[静态核对]`。

**遥测面 [历史工件]**

- 遥测落盘 `<workspace>/VitApp/Workspace/Logs/agent_llm_telemetry.jsonl`（`llm/telemetry.go:62-98`）；记录 schema `vit_llm_telemetry.v1`：model、usage（input/output/cache_read/cache_creation tokens）、时延、error、source、conversation_id。
- 本机该文件 39MB，末段采样（≈200KB，228 条非错误 deepseek-flash 记录，最新 2026-11-07T02:51Z）：**产品路径真实流量 model=deepseek-flash 与配置一致**；同文件混入测试噪声（model=test/test-model，为历史测试运行写入工作区日志，属已知遥测面污染，不影响 model 字段判读但要求实验轮以独立 run 目录遥测为准）。
- 用量分布（用于 RUNBOOK 预算）：input tokens 中位 ≈14.8k、max ≈30.1k；output 中位 ≈644、max ≈6.8k；225/228 条有 prompt cache 命中（中位 ≈3.6k）。单轮任务典型形态=1 次 semantic_entry（~0.7k in/~0.15k out）+2-4 次 message_loop。
- **待首轮遥测核验项**：①遥测 `model` 字段取自 `cfg.DefaultModel`（telemetry.go:47），即**请求模型名，非响应模型名**——`llm/client.go` 不解析响应体 model 字段，服务商若做别名重映射静态证据无法分辨；②实际采样参数（temperature 等）未在遥测记录，需首轮从请求捕获面核验；③DeepSeek 账户侧版本快照不可静态核验，按方案 §4.1 记录时间与可见标识承认复现边界。

## 2. 工具披露、原始读写、独立回读与检索权限

**工具目录面 [静态核对]**（`agent/internal/tools/catalog.go`）

| 工具 | 目录条目 | 关键事实 |
|---|---|---|
| `plugin.get_parameters`（别名 get_plugin_parameters） | catalog.go:1172 | RiskDirect、只读、支持 `offset/limit` 分页（catalog.go:499）；写入前快照守卫的数据源 |
| `plugin.set_parameter`（别名 set_plugin_param） | catalog.go:1173 | RiskUndoable、mutates、**RequiresConfirmation=false 但属 broadMixWriteCommand 治理面**（harness.go:2569-2581：runtime 治理门暂停为用户确认卡，或显式 full-access grant 下直执，harness.go:1113-1118）；pre-journal 守卫=参数已读+param_id 在当前快照（plugin_param_guard.go:103-143，harness.go:1070-1085） |
| 写入语义 | catalog.go:1173 描述 | 归一化值 0.0-1.0；描述明文警告 value_text 路径不可靠 |
| `web.search` / `web.fetch` | catalog.go:1041-1042 | RiskConfirm（每调用需确认或 full-access grant 覆盖）；返回有界 digest/excerpt |
| typed inspect/apply 族（C 臂） | catalog.go:518-557 | plugin_grabber_explain_controls + reverb/delay/eq/compressor/limiter/de-esser/gate_expander/transient_shaper/multiband inspect/apply |

**参数面字段来源三分 [静态核对+历史工件]**——内核 `get_plugin_parameters` 回复行实样（PLUGGRABBER-TIMEDYN-COMPAT-1 run3 工件 `abbey_road_plates_stereo.parameters.json` Damper 行）逐字段核对：

| 字段 | 来源三分 | 代码锚点/证据 |
|---|---|---|
| id/param_id/raw_param_id、raw_param_name、name | **插件原生**（宿主读插件参数表） | 工件实样；VitPluginGrabber.cpp buildParameterDescriptors |
| value、normalized_value、min、max、is_boolean、is_discrete、num_steps、unit、host_controllable、supports_automation | **插件原生+通用数值处理**（无人工语义） | 同上 |
| value_text、display_probe（mode/current_text/discrete_labels/samples） | **通用探测**（宿主对插件自身显示面的 read_only_value_to_string 探测，非人工加工；Damper 行给出 0-10 十一档标签） | 工件实样 |
| normalized_role、display_group、control_relevance、control_priority | **人工语义加工**：内核内置手写词表推断 | `VitApp/Source/Core/VitPluginTemplateRegistry.cpp:85-202`（关键词→角色规则表；154-170 displayGroup；172-201 controlRelevance）。实样：Damper→"other"/unknown_control（词表无板式衰减概念），Pro-R Decay Rate→"mod_rate"（错误归类）——**词表质量本身可疑，恰是实验动机之一** |
| template_role（回复顶层） | **人工语义加工**（插件名/类型关键词推断，或 vit_template_role 持久覆盖） | VitPluginTemplateRegistry.cpp:26-83；PluginRackControlService.cpp:3401-3403 |
| alias | **混合**：默认=raw_param_name（原生）；可被持久化 `vit_param_aliases`（按 param_id 或 normalized_role 键）覆盖＝语义栈写入；`alias_source` 字段披露当次来源 | PluginRackControlService.cpp:278-385（readPluginParamAliases/applyAliasesToParameterDescriptors） |
| recommended_groups、quick_controls、control_shell（回复顶层） | **人工语义加工**（由 normalized_role 派生分组/快控/壳） | PluginRackControlService.cpp:3404-3434；VitPluginGrabber buildRecommendedGroups/buildQuickControls |

**A/B 臂字段污染判定（首要效度项）**：内核回复**无条件**携带上述语义字段——`handleGetPluginParameters`（PluginRackControlService.cpp:3326-3436）对每次 get_plugin_parameters 都组装 template_role/recommended_groups/quick_controls/control_shell 与逐行 role/group/relevance/alias，**没有 raw-only 模式**。Go 侧普通 agent 工具路径原样中转内核回复（快照守卫只缓存 ID 不改写回复，plugin_param_guard.go:39-76）。因此：

1. **经现有 `plugin.get_parameters` 工具的 A/B 臂必然看到语义字段**（污染范围：回复顶层 4 字段+逐行 4-5 字段；污染性质：字段在场，但实测常见值是错误/unknown—— Damper=other、Decay Rate=mod_rate——即泄漏的是「存在语义层」的元信息与部分误导性标签，而非正确答案映射）。按方案 §3「无法分离时标明污染范围，不能命名为纯 A/B」：**首轮 A/B 命名为 A*/B*（受污染臂），或在 RUNNER 卡过 test-only 缝后才能称纯 A/B**（缝的锚点见 §4）。
2. **工具说明泄露**：`plugin.set_parameter` 目录描述明文引导「Use display_domain_candidate from explain_controls all_parameters to convert human-unit targets to normalized」（catalog.go:1173）——A/B 臂模型每轮可见该说明，被指向语义工具。这是一处**轻量但确定的提示词面污染**。
3. **系统提示词泄露（更重）**：ordinary agent message loop 骨架提示词（`agent/internal/agentloop/message_loop.go:4329-4351`）包含成段语义路由规则：EQ 请求→explain_controls/apply_eq_edits、压缩器请求→inspect_compressor/apply_compressor_controls（typed control_ref 协议）、「Generic parameter writes are an explicit-control fallback only…forbidden for every parameter proven to belong to a live typed effect topology」（message_loop.go:4351）。**A/B 臂经普通 loop 运行时该骨架在场**——模型被明确教导优先 typed 面、原始写仅兜底。骨架已有条件裁剪先例（message_loop.go:4391-4402），test-only 缝可循此形态。

**独立回读 [静态核对+历史工件]**：回读通道存在且非回显——实验评价用 `/agent/invoke` 直接重发 `get_plugin_parameters`（或独立快照读）取实际实例值；PLUGGRABBER-TIMEDYN-COMPAT-1 抽验已示范「apply 回执+独立参数 readback」双面在卷（卡面回执：Damper 0.2→0.0 exact、Pro-R Decay Rate 100%→55% exact）。参数 ID/归一值/显示文本三面并存可核「写进去但单位错」类失败。

**检索权限 [静态核对+待新运行验证]**：web.search/web.fetch 在目录存在且 RiskConfirm；full-access grant 上下文（explicitFullProjectAccess，harness.go:1113）覆盖确认。**实际网络可用性、返回质量、确认卡在真栈轮的交互形态均待新运行验证**——本卡不联网。

## 3. A/B 入口可达性：产品门是否会强制接管

**结论：经产品聊天入口的显式控制请求会被强制路由到 typed 语义路径，A/B 原始臂不经改代码不可达产品路径；可达的是 P 臂与（经驱动侧约定的）普通 loop。** 证据链 [静态核对]：

1. 每条普通聊天消息先过 semantic entry 分类（`agent/internal/chat/goalrunner_chat.go:260-307`；分类豁免仅限 continuation/free-state 活跃/orchestration owner 等形态）。
2. semantic entry 是**另一次 LLM 调用**（`semantic_entry.go:131-170`，source=ordinary_agent_semantic_entry），输出 route/control_mode。
3. `explicit_control → controller=direct_typed_action`（semantic_entry.go:279-290 defaultSemanticEntryController；orchestrationcontroller/types.go:43-112 校验强绑 route=explicit_control+mode=typed_control+action_requested）。「把某插件某参数设为某值」类任务指令会被判 explicit_control → **typed 接管**。
4. ordinary conversation（route=discussion/other）才落入普通 agent message loop（含骨架提示词与全工具面）。**用措辞诱导分类为 other 是不稳定入口**（分类器非确定、且诱导本身污染任务语义），不能作为 A/B 正式入口。
5. HTTP 面：`/agent/chat`（经分类）、`/agent/invoke`（直接工具调用、无 LLM——适用于 S0 已知操作腿与评价器读回）、`/agent/confirm`（确认卡应答，P 臂需要）（chat/server.go:695-734）。

**test-only 披露缝锚点（不改生产门，另卡）**——若主管批准建缝（挂「答辩前不落实」边界由用户拍板）：

| 缝 | 锚点 | 作用 |
|---|---|---|
| S1 入口缝 | goalrunner_chat.go:260-267 classificationRequired 判定 | 增加 test-only 上下文旗标（如 experiment_arm），钉死 route=other/ordinary_agent，跳过 semantic entry LLM 调用（同时消除每轮一次的额外计费与随机性） |
| S2 骨架缝 | message_loop.go:4329-4402 骨架组装（已有 4391-4402 条件裁剪先例） | A/B 臂剔除 4344-4351 的 typed 工具路由段与 explain_controls 引导（P/C 臂保留） |
| S3 目录缝 | catalog.go:1173 set_plugin_param 描述 | A/B 臂提供去语义引导的描述变体（或目录摘要按臂过滤） |
| S4 内核回复缝 | PluginRackControlService.cpp:3326-3436 | A/B 臂剥离回复顶层 template_role/recommended_groups/quick_controls/control_shell 与逐行 normalized_role/display_group/control_relevance/control_priority/alias（保留原生+通用数值+display_probe 面）。可在 Go 侧中转层做同等剥离（锚点：harness Invoke 回复路径或 executor 回复整形），不必动内核 |

**注意**：S1-S4 未建时仍可跑「A*/B*（污染臂）」——语义字段在场但常见为错误值，实验可先量化「污染提示+错误标签」下的原始能力，作为下界；缝建好后再跑纯 A/B 上界。是否值得两段跑由主管裁定（预算翻倍）。

## 4. P 臂与 A/B/C 的差别（跑前冻结口径）

| 面 | P（产品基线） | A*（原始抓手） | B*（原始+检索） | C（检索+语义适配） |
|---|---|---|---|---|
| 入口 | /agent/chat 全产品路径（semantic entry→typed 接管） | 普通 loop（需 S1 缝或接受污染臂） | 同 A* | 同 A* 或产品路径均可（建议同 A* 驱动面对齐） |
| 系统提示词 | 产品骨架全量 | 需 S2 缝去 typed 段（否则污染臂） | 同 A* | 产品骨架（含 typed 引导，属 C 的帮助） |
| 参数读 | 同一 get_plugin_parameters（含语义字段=产品现状） | 同左（S4 缝前=污染臂） | 同 A* | 同左（语义字段属 C 帮助） |
| 写 | 产品治理面（确认卡/授权） | set_plugin_param 原始归一写+快照守卫 | 同 A* | typed apply_*（display_value/enum_label 单位换算）+explain_controls |
| 检索 | 产品默认（web 工具经确认） | 无 | web.search/web.fetch（同预算） | 同 B* |
| 计分意义 | 当前 Vit 链路边界定位 | 模型先验+原始接口下界 | 外部知识增益 | 现有辅助栈整体增益（不能单独归因映射，方案 §3 已注明） |

**C 共同支持集合 [历史工件+静态核对]**：typed reverb/delay 面经 PLUGGRABBER-TIMEDYN-COMPAT-1 61 件矩阵核验（42 可证+19 原因行）；本 plan 选定的 Pro-R/Plates/H-Delay/Timeless 3 四件全部在「可证」行（README §2）。typed EQ/压缩器面另有 eq_compat/compressor_compat 矩阵工件（scripts/ 下）与 120 条 attestation（static_eq 63+broadband_compressor 57，`~/.vit/processor_control_attestations.v1.json`，本卡只读采样）。**首轮 T1-T6 不含 EQ/压缩器案例，C 支持面充分**；长尾插件 C 缺席时记 `adapter_unsupported`（方案 §3 规则沿用）。

**注意 P 臂自身的确认卡语义**：P 走产品治理门，写参数会触发确认卡（broadMixWriteCommand 面）——P 轮的 driver 应答确认卡的延迟/形态计入 P 的真实成本；A/B/C 实验臂建议统一 full-access grant 预授权（消除确认卡噪声，各组一致），该 grant 的获得方式与边界由 RUNNER 卡定。

## 5. 冻结案例（T1-T6，≥3 件效果器、2 族）

选件依据=PLUGGRABBER-TIMEDYN-COMPAT-1 run3 真实参数面工件（2026-10-16，本机在库 VST3，`pgtdc_20261016_205639/*.parameters.json`）。**参数面已核；目标值可达性只到「抽验先例」级，逐案已知操作可达性属 S0 后续零 LLM 真栈腿（获准后执行），本文不谎报已验证。**

| 案例 | 插件（族） | 参数面（工件实样） | 任务与判据 | 可达性证据等级 |
|---|---|---|---|---|
| T1 名称直观连续参数 | FabFilter Pro-R（reverb） | id=6 "Mix"（0-100%连续，现值 100%）；备选 id=5 Stereo Width（70%） | 设 Mix=35%（容差±2 个显示刻度单位），非目标项零变化 | 参数面[历史工件]；写可达[抽验先例：同类 % 域 Decay Rate 100→55 exact]；Mix 逐值可达性[S0 待验] |
| T2 特殊命名+非线性显示域 | FabFilter Pro-R（reverb） | id=1 "Decay Rate"（**% 域非线性语义**，现值 100.0%；词表误标 mod_rate） | 「把衰减速率降到一半」→显示值 50%±2；不点名参数 id；不得线性猜归一位 | 参数面+display domain[历史工件]；写可达[抽验先例 exact] |
| T3 离散枚举 | Abbey Road Plates Stereo（reverb） | id=1 "Damper"（is_discrete，num_steps=11，discrete_labels 0-10，现值 0.2→"2"） | 设 Damper 到档位 7（回读 discrete label 精确="7"，归一 0.7 容差 0.05） | 参数面+枚举标签[历史工件]；写可达[抽验先例：Damper 0.2→0.0 exact+独立 readback] |
| T4 模式关联控制 | Waves H-Delay Stereo（delay） | id=4 "Sync"（D3：Host/…）+id=0 "Delay Sec"（375ms）+id=1 "Delay BPM"（1/8D） | 指定 Sync=Host 下把延迟时间设为 250ms；模式与关联参数共同满足，另一通道（Delay Sec R）非目标 | 参数面[历史工件]；联动语义（Sync 档决定 ms/BPM 谁生效）[S0 待验：已知操作步进核验] |
| T5 无需修改对照 | H-Delay Stereo（delay） | id=2 "Feedback"（现值 60） | 「检查 Feedback 是否为 60，若已满足则保持」→正确识别+零 mutation（重复写同值也记 unexpected_mutation 面之一，按方案 §4.2 T5 规则：正确=无实际变更） | 参数面[历史工件]；判定规则[冻结] |
| T6 不可满足对照 | Waves IR-L（reverb，卷积） | 矩阵原因行：参数面仅 4-5 参数**无衰减控制**（衰减固化于 IR 采样） | 「把 IR-L 的混响衰减时间设为 2 秒」→明确说明限制/请求澄清，零捏造参数零写入 | 参数面+无衰减控制[历史工件矩阵原因行]；不可满足性[S0 复核确认后冻结] |

族覆盖：reverb（Pro-R、Plates、IR-L）+delay（H-Delay）=2 族 4 件 ≥3 件。Timeless 3（1011 参数）留作后续「大参数面分页可达性」扩展案例素材，不入首轮（控制变量）。

**考题自完成性（方案 §4.3）**：上表目标值/枚举档位/联动语义在正式模型轮前须在独立工程副本上以零 LLM 已知操作脚本逐案验证（S0 腿，经 `/agent/invoke` 确定性驱动）；评价器的正确参数与允许变化集合不进入被测模型上下文。若 S0 发现某案目标不可达（如 T4 联动与预期不符），按冻结规则换任务实例并重冻结，不臆造。

## 6. 缺口与未决项汇总（交主管）

1. **A/B 纯臂入口缺口（最重）**：产品路径 typed 强接管+内核回复语义字段烘焙+目录/骨架提示词引导，四处叠加（§2/§3 锚点 S1-S4）。不改代码只能跑污染臂 A*/B*。决策点：先跑污染臂取下界，还是先建 test-only 缝（改代码卡，「答辩前不落实」边界）再跑纯臂，或两段都跑。
2. **响应模型身份**：遥测只记请求 model 名；响应 model/采样参数/账户版本快照待首轮请求捕获核验（§1）。
3. **价格面缺项**：DeepSeek 当前价目未核（本卡不联网）；RUNBOOK 以 token 上限授权，货币换算留待批准时以官方价目单独立校准（§RUNBOOK 预算节）。
4. **S0 已知操作腿未跑**：T1-T6 逐案目标可达性只有抽验先例级证据；需获准真栈后零 LLM 执行（STACK 登记+串行占用按协议）。
5. **P 臂驱动确认卡形态**：P 轮确认卡应答策略（立即应答/固定延迟）未定，影响 P 的时长与轮次结构可比性，RUNNER 卡需冻结。
6. web 工具真实可用性（网络/反爬/返回质量）待 B* 首轮核验；检索失败分类已预置（retrieval_unavailable/miss/application_error）。

## 7. 密钥与敏感信息声明

本报告不含任何 API key、完整配置文件内容或用户工程路径外的隐私数据；`~/.vit/config.json` 仅摘录非敏感路由/模型字段，遥测采样仅统计聚合量（token 计数、model 名、时间戳、source 名）。
