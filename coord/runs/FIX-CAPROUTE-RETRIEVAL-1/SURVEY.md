# FIX-CAPROUTE-RETRIEVAL-1 勘察报告：能力路由轮的语义知识工具可达性

- 卡：coord/cards/doing/2026-11-09-FIX-CAPROUTE-RETRIEVAL-1.md（池序 325）
- 执行：GLM-5.3 L2 执行子代理（ZCode 会话，PC）；worktree=D:/Vit_DAW_wt_325impl；分支=port/caproute-retrieval-1；领取基线=3c0bdd3b（origin/main 全净）；领取提交=fa264c4e（已推 main 亲核）
- 日期：2026-11-09
- §8 口径：零栈零内核零 LLM。纯只读语料勘察（312/317 既有工件）+代码阅读+Go 单测（确定性，无概率性运行面）；运行次数=每测试命令 1 次，成功条件=退出码 0；无失败分类面（确定性单测）。

## 一、结论（一句话）

**负结论：`plugin.semantic_get / semantic_search / semantic_build_index` 在能力路由轮全部可见可达（披露目录行 + Allowed tools 执行面双在场，12/12 包路由任务轮 + 15/15 probe/全目录轮实证）；当前不存在「语义知识面在能力路由轮不可达」的缺口，无需披露面改动。** 与 B/B-R1 的 0/18 web 检索发起互补的事实是：模型在 T1 轮曾用 `plugin.semantic_get` 寻知且执行成功（status=ok, found=true）——知识检索走的是域内语义面，且该面一直可达。

## 二、可达性矩阵（317 B-R1 语料逐条核实，63/63 message_loop 请求）

语料：`coord/runs/PLUGIN-CONTROL-EVAL-BR1-1/arm_br1_20261108_211559/logs/agent_prompt_render.jsonl`（63 条系统提示渲染 = probe T2_r0×9 + 18 任务轮的 54 请求）。逐行机器提取工件：`prompt_render_disclosure_rows_317BR1.json`（本目录，63 行逐条）。

| 披露形态 | 触发条件（路由） | 请求数 | 案例轮 | sem 目录可见位置 | sem 在 Allowed | web 对可见 |
|---|---|---|---|---|---|---|
| 全目录 209 工具 | 能力零命中（词面无能力词且上下文无已知插件名） | 33 | probe T2_r0 + T2/T3 任务轮 | 主目录行（plugin_semantic_get/search/build_index 三行直列） | 是（三件全列） | 是（web_search/web_fetch/browser_search/browser_fetch 四行） |
| 扁平收窄集 41 工具（≈312 口径「42 行」） | plugin 能力命中且无已注册 pack | 15 | T4/T6 任务轮 | 主目录行直列 | 是（三件全列） | 否 |
| 能力包 13+Shared support 37（≈312 口径「51 行」） | plugin+mix 双命中（渲染 mix pack） | 15 | T1/T5 任务轮 | **Shared support tools 差集块**（plugin_semantic_* 三行） | 是（三件全列） | 否 |

- 63/63 请求 sem 三件套全部可见、全部在 Allowed tools 列表内（提取工件字段 `plugin_semantic_visible_in` 无一 ABSENT、`plugin_semantic_missing_from_allowed` 无一非空）。
- 与 317 官方 consolidate 的 visibility_strata 吻合：18 任务轮 = 6 全目录可见（T2/T3）+ 12 包路由（T1/T4/T5/T6），分类 `retrieval_not_disclosed_pack_routed`×12 只针对 **web 对**，不涉及 sem 族。

## 三、312 B 臂交叉验证（71/71 message_loop 请求）

语料：`coord/runs/PLUGIN-CONTROL-EVAL-S1B-1/arm_b_20261108_203434/logs/agent_prompt_render.jsonl`。逐行工件：`prompt_render_disclosure_rows_312B.json`。形态分布：全目录×34 / 扁平收窄×23 / 包+Shared support×14——**三形态 sem 全部可见+Allowed 全列**，与 317 一致。B 臂 probe 载体同为 T2（全目录），结论不受臂差影响。

## 四、执行面证据（不止披露可见，且真实执行成功）

`agent_llm_payload_capture.jsonl`（317，110 行=63 message_loop+28 repair+19 semantic_entry）：

- line18（会话 pce_p_T1_r1_1794143849，T1=mix 包路由轮）msg5 assistant 发起：`plugin.semantic_get {"query":"Pro-R"}`，reason 原文「Need Pro-R's authoritative parameter/control mapping (Mix param id and normalized range) before writing a value.」
- 同请求 msg6 tool_result：`"command_name":"plugin_semantic_get","observed_state":true`，`result_preview` 返回完整 Pro-R 语义条目（id=VST3-Pro-R-4bc24482-78c01fa6 / FabFilter / Fx|Reverb / semantic_types reverb@100 / found=true / status=ok）。

即：在 12/12 包路由轮中最难的写入前寻知场景（T1），语义工具从披露到执行全链路可用。写入失败是治理闸门（结论档 ⓪/⑦/⑧ 已归因），与本卡问题无关。

## 五、形成机制代码锚点（勘察目标 1）

完整链路（四层，全部现行代码）：

1. **能力路由**：`agent/internal/chat/goalrunner_chat.go:4641 agentLoopCapabilityNames(userText, requestContext)`——词面/上下文匹配产出 capabilities（如 ["plugin"]、["plugin","mix"]）。插件命中靠 `toolpolicy.MentionsPlugin`（词面「插件/混响/延迟/EQ…」或上下文已知插件名，plugin_policy.go:109）。T2 零命中的机理：文本「请把 Pro-R 的衰减速率降到当前值的一半」无插件词面，且实验装置 chat_context 无 selected_plugin_name → CollectPluginNames 空 → 不命中 → 全目录。
2. **allowed 集裁剪**：`goalrunner_chat.go:4559 toolNamesForAgentLoopCapabilities`——`agentLoopBaseTools()` + 每能力并行白名单。**`agentLoopPluginTools()`（goalrunner_chat.go:5145-5147）首行显式含 `plugin.semantic_search/get/build_index`**——这是 sem 进入 plugin 命中轮 allowed 集的锚点（EDU-SYMBOL-CORE-1 型第四注册面；缺行即静默遮蔽，本卡已核实不缺）。web 对不在任何能力白名单 → 包路由轮剔除（312 装置发现的机理，与 sem 命运不同就在这两行的差）。
3. **披露文本组装**：`agent/internal/tools/capability_pack.go:23 ModelSummaryForCapabilityPacks`：
   - 命中 capability 在 `defaultCapabilityPacks()`（track/midi/media/mix/static_mix_*/project_*）→ 渲染包描述+包 Tools∩allowed（85 行）+ **Shared support tools 差集块（39-43 行：allowed − 包Tools − daw.invoke）**——T1/T5 的 sem 三件套因 plugin+mix 双命中（allowed 含 sem）而落入此块披露；
   - 命中 capability 无 pack 定义（plugin 即此）→ `CapabilityPacks()` 空 → 回落 `ModelSummaryForTools(toolNames)` 扁平收窄集（T4/T6 形态）；
   - 零命中 → `ModelCatalogSummary()` 全目录 209 行（T2/T3 形态）。
4. **拼入系统提示**：`agent/internal/agentloop/message_loop.go:4267 messageLoopSystemPrompt`（4280-4289：CatalogSummary+AllowedTools 拼入），工具披露走系统提示文本行而非 OpenAI tools 数组（capture 110 行 request 均无 tools 字段——披露面=系统提示，亲证）。

## 六、机制边界（如实登记，非缺口裁定）

- **纯 mix 观察轮不含 sem**：用户文本只命中 mix（如「帮我看整体混音」，无任何插件词面/已知插件名）时，allowed=base+`agentLoopMixTools()`（goalrunner_chat.go:5175，无 sem），mix pack Tools 覆盖后 Shared support 差集近空 → sem 不披露。这是有界披露的设计内收窄（观察轮没有插件语义检索的词面需求；对话一旦提及插件名，下一轮路由即补上 sem）。317 语料中无此类轮（T1/T5 均双命中），本报告不将其定为缺口；若产品要在纯观察轮也提供语义检索，属披露策略扩面，候主管另行裁定（涉及 mix 白名单加行或 pack Tools 扩表，仍是行级改动面）。
- **web 对在包路由轮不可见**（312 装置发现，PENDING 挂账对象）：按 2026-11-09 改革定向（模型管语义+机器管编码；web 为次要面），维持现状；本卡不动。

## 七、建议（勘察目标 2）

1. **维持现状+登记**：sem 三件套经 `agentLoopPluginTools` 白名单行（plugin 命中轮）与 Shared support 差集块（plugin+pack 双命中轮）两条既有通路覆盖全部插件控制场景，312/317 双语料 134/134 请求实证无缺口。**不建议为 sem 补任何披露行**（无落点：两条通路均已含）。
2. **防回归锚（本卡唯一改动）**：新增单测 `TestAgentLoopCapabilityRoutedTurnsKeepSemanticKnowledgeTools`（agent/internal/chat/goalrunner_tool_context_test.go），用 317 语料原始文本（T1/T6）锚定三不变量：plugin+mix pack 轮 sem 在 Shared support 块+Allowed；plugin flat 轮 sem 在主目录+Allowed；两种路由形态 web 对保持不披露。EDU-SYMBOL-CORE-1 先例（白名单缺行→工具从每个收窄轮静默消失）证明此处是脆弱点，负结论需要测试钉住。
3. 纯 mix 观察轮 sem 不可见（§六）登记为披露策略备选议题，不扩域。

## 八、改动与验证

- diff：仅 `agent/internal/chat/goalrunner_tool_context_test.go` 追加一个测试函数（产品代码零改动，披露面零变化）。
- 测试命令与退出码（worktree D:/Vit_DAW_wt_325impl/agent 下）：
  - `go test ./internal/chat -run 'TestAgentLoopCapabilityRoutedTurnsKeepSemanticKnowledgeTools' -count=1` → exit 0（ok）
  - `go test ./internal/chat ./internal/tools -count=1` → 退出码见卡面回执（既有回归）
- 端测边界声明：本卡为零栈勘察+单测锚定，无 agent 侧生产改动（测试文件不进产品二进制），AGENTS §5 真栈烟测门槛不适用；可达性行为证据以 312/317 真栈语料（134 请求逐条）+机制代码锚点承载。

## 九、工件清单（本目录，随 port/caproute-retrieval-1 分支入库）

- `SURVEY.md`——本报告
- `prompt_render_disclosure_rows_317BR1.json`——317 语料 63 请求逐条披露形态提取（含指纹前 16 位）
- `prompt_render_disclosure_rows_312B.json`——312 语料 71 请求同规格提取
