# FIX-CAPROUTE-RETRIEVAL-1：能力路由轮的工具披露面与知识工具可达性（PENDING 分流①）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=PENDING-DECISIONS 2026-11-08 条目[312 装置发现] + B-R1 终局数据 + 实验线结论档）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流）
- 验收负责人：GLM 主管决策流
- 池序 325；P2 / 0.5 天 / 依赖=零；L1 / PC；零栈勘察+小改

## 背景

312 装置发现：ordinary loop 工具披露按能力路由——命中能力包时披露 42/51 行（不含 web 工具）；零命中时全目录 209 行。B-R1 终局补充：即使全目录可见，模型 0/18 发起 web 检索（主动性结论）；产品中该需求实际由 typed/语义层覆盖。改革方向定向后（模型管语义+机器管编码），知识面主角=语义工具+编码描述。

## 目标

1. 勘察：能力路由轮中 `plugin.semantic_get/search/build_index` 的可达性（在包内还是在全目录）——若语义工具在能力路由轮不可达，这才是当前真缺口（web 面降级为次要）；
2. 形态裁定输入：按改革方向给出披露策略建议（能力包内补语义知识工具行 vs 保持现状+登记）；
3. 若为最小行级改动（catalog/pack 披露行）：实施+单测；若触及披露架构：只出勘察报告候裁定，不动架构。

## 文件域

勘察=只读；若改=catalog/pack 披露面±测试（282 域已合入释放）；零内核零栈（复用 312/317 语料工件）。

## 验收标准

勘察报告（可达性矩阵+建议）+（如实施）单测退出码 0+既有 pack 行为零变化证据。

## 停止条件

披露架构级改动需求 → 上交候裁定，不自行扩域。

## 领取/回执/验收

- 领取：2026-11-09 / origin/main=3c0bdd3b / owner=GLM-5.3 L2 执行子代理（ZCode 会话，PC）/ 分支=port/caproute-retrieval-1 / worktree=D:/Vit_DAW_wt_325impl / 领取基线亲证：HEAD=3c0bdd3b=origin/main，status 全净，diff 空；领取提交=（本提交，hash 见 git log）
- 回执：（2026-11-09，GLM-5.3 L2 执行子代理）
  - **勘察结论=负结论**：`plugin.semantic_get/semantic_search/semantic_build_index` 在能力路由轮**全部可见可达**（披露目录行+Allowed tools 执行面双在场）。317 B-R1 语料逐条核实 63/63 message_loop 请求（含 12/12 包路由任务轮：T1/T4/T5/T6），312 B 臂交叉验证 71/71，无一个 ABSENT。可达通路：plugin 命中轮经 `agentLoopPluginTools` 白名单行（agent/internal/chat/goalrunner_chat.go:5147，扁平收窄集直列）；plugin+mix 双命中轮经 `ModelSummaryForCapabilityPacks` 的 Shared support tools 差集块（agent/internal/tools/capability_pack.go:39-43）。零命中轮回落全目录 209 行（T2/T3）同含。
  - **payload 逐轮证据锚点**：`coord/runs/FIX-CAPROUTE-RETRIEVAL-1/prompt_render_disclosure_rows_317BR1.json`（63 行逐条，含指纹）+`prompt_render_disclosure_rows_312B.json`（71 行）；执行面终极证据=317 capture line18（T1_r1 mix 包轮）assistant 发起 `plugin.semantic_get {"query":"Pro-R"}` → tool_result `status:ok, found:true` 完整 Pro-R 语义条目——写入失败轮的知识寻知链路本身全程可用。
  - **建议=维持现状+登记**：两条既有通路已覆盖全部插件控制场景，无可补的披露行落点；web 对在包路由轮不可见（312 装置发现）按改革定向（web=次要面）维持。机制边界如实登记：纯 mix 观察轮（无任何插件词面命中，如「帮我看整体混音」）Shared support 差集近空、sem 不披露——设计内收窄，317 语料无此类轮，不判缺口；若产品要在观察轮提供语义检索，候主管另裁（行级改动面，goalrunner_chat.go:5175 mix 白名单）。
  - **实施（最小）**：仅 `agent/internal/chat/goalrunner_tool_context_test.go` 追加 `TestAgentLoopCapabilityRoutedTurnsKeepSemanticKnowledgeTools`（317 语料原始文本 T1/T6 锚定三不变量：pack 轮 sem 在 Shared support+Allowed；flat 轮 sem 在主目录+Allowed；两形态 web 对不披露——EDU-SYMBOL-CORE-1 型白名单缺行防回归锚）。产品代码零改动，既有 pack 行为零变化由全包回归背书。
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_325impl/agent）：`go test ./internal/chat -run 'TestAgentLoopCapabilityRoutedTurnsKeepSemanticKnowledgeTools' -count=1` → exit 0；`go test ./internal/chat ./internal/tools -count=1` → exit 0（chat 155.5s+tools 0.56s 全绿）。
  - **commit**：实现+工件=port/caproute-retrieval-1 @ c10da4eb（已推 origin，工件目录 coord/runs/FIX-CAPROUTE-RETRIEVAL-1/{SURVEY.md, prompt_render_disclosure_rows_317BR1.json, prompt_render_disclosure_rows_312B.json}）；领取=fa264c4e（已推 main）。零栈零 LLM 零内核，无 run ID 无计费。
  - **端测边界声明**：零栈勘察卡；无生产代码改动（仅测试文件），AGENTS §5 真栈烟测门槛不适用；行为证据由 312/317 真栈语料 134 请求+机制代码锚点承载。
  - done=自验完成，候主管验收；未合入 main（实现仅在 port 分支）。

### 终验（主管，2026-11-09）

- **pass 负结论整卡合入 a36df77d**——主管抽查终极证据吻合：317 capture line18 逐字核对=`plugin.semantic_get {"query":"Pro-R"}` 在 mix 包路由轮实调成功（与勘察矩阵 T1_r1 行一致）；63/63+71/71 机器核实方法合格；机制锚点（goalrunner_chat.go:5147/capability_pack.go:39-43）与披露走系统提示文本的边界亲证一致。主管亲跑防回归锚测试 exit 0。
- **裁定**：①负结论成立——sem 族三披露形态全可达，无落点可补，维持现状+登记；web 对不可见按改革定向（次要面）维持；②纯 mix 观察轮 sem 不披露=设计内收窄，登记不判缺口（未来要扩=行级小卡另裁）；③原 PENDING「能力路由轮检索不可见」条目据此关闭（真缺口不存在，语义面已覆盖寻知路径）。
