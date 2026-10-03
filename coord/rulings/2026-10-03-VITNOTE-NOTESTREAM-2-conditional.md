# Ruling：VITNOTE-NOTESTREAM-2 — conditional pass（2026-10-03 决策会话）

## 验收

1. **取证采信**（五条带代码锚点回写卡面）：note 问答入主流=工程单图落盘（与 conversation_id 无关）+webui 聚合水合双重同向；误入治理链=beginChatGoal×主控台组包复用（contract_scope 错取选中轨根因）；侧边栏未命名=durableContinuations 投影 title 空。目标 1 完成。
2. **diff 亲核**（Vit_DAW 618d69f9：note_sessions.go 新增 447+server.go 早分叉+/agent/note/archive+runtime/status note_sessions 投影+webui 5 文件；Godot 4391c49：载荷 v3+主控台组包复用链退役+新探针）：早分叉位于 pending 确认/治理链/单图落盘/durable continuation 之前 ✓；**设计裁定三原则全部内化**（不进 D1 治理链=直答 LLM+观察语义+辖区快照注入；命名=裁定 1 模板同源同文；会话键=note_id+run 戳）✓；持久化 fail-open（缺/损/异形→空表+原子写）合 AGENTS §11 ✓。
3. **我方独立复跑**（双验证 worktree 亲跑）：Go——note 9 钉 PASS+chat 全包 123.7s+**全仓 -count=1 EXIT=0**；webui——vitest **425/425**+tsc 0 错；Godot 探针——notestream **21/21**+container 141/141+circle_state 52/52+face_resolve 127/127+panel_summary 16/16+input EXIT=0+--import 零错，与回执逐面吻合。
4. **复跑环境注记**：新鲜 worktree 首跑探针现解析错（`.godot` global class cache 缺失→class_name 解析连锁失败，container EXIT=1/circle 44 项形态）——`--import` 后全部对齐。**复跑纪律点：探针前先 import**（记 gate）。
5. **判据 1-4**：机制级证据三证/探针 D 面/组装源钉/vitest 钉亲读采信（REPORT §三）；真栈级归用户手测（卡面口径）。
6. **端测边界**：全程未起真栈（用户指令）+webui 无浏览器级 DOM 断言（如实申报）——渲染面以 vitest 纯函数钉+即将的手测覆盖，按 §5 由决策侧裁定可交付（有损但已声明+手测在即）。
7. **配套约束**：旧 note 问答路径整体退役非并存开关——**Godot 与 agent 必须同版本配套**（Godot 4391c49 需与 agent 618d69f9 同批上线；合并顺序 Godot 先/同批，验收执行=同批）。

## 裁定

- **conditional pass**：实现+回归+机制证据全过；转正条件=**用户真栈手测复验**：①便签里问「这里是什么内容」→ 得到辖区内容回答（三号场回归判据+零进治理链）；②webui 侧边栏便签流显示默认命名（不显"未命名"）+可改名+点击可回放历史；③主对话流零 note 往返污染；④note 收起重开历史还在。
- Vit_DAW cherry-pick 618d69f9 合 main；Godot `port/vitnote-notestream-2`@4391c49 为手测分支（主树切换待用户关栈）。
- LLM 增强命名维持二期占位；SESSION-SEMANTICS-1（主流自动建+轨迹切换+遗留流）为后续 webui 会话面卡。
