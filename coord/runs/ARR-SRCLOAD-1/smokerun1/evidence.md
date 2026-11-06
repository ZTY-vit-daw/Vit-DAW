# ARR-SRCLOAD-1 smokerun1 evidence（段 2 真栈腿，2026-11-07 01:58–02:01）

## 判定：④装载链缺陷（预写分类）——chat 驱动装载被 **PCA 装载门按设计拒绝**，链在治理边界停止；取证上交，单 run 无重跑

- run `20261107_015840` 窗口（01:58:40 启动，py 断言器 01:59:2x 退出）；**arr_srcload_smoke.py exit 4**（arr_srcload_summary.json 权威：exit_class=command_defect, fail_phase=load）；PS1 以 throw 收口（console 转录在卷；wrapper 退出码因执行侧 shell 管道未单独落盘——py exit=4 为权威口径，如实记录）。
- 被测不变量：agent=wt_277impl 现场构建 SHA256 `62F444B7F1F817492DF96FE765CA12FF30CBF1450F116750F6928EFEAC2D28B3`（agent_binary_sha256.txt）；kernel=wt_228 `F9E3DA0A…0D67A` 只读复用；引擎=本机默认（无注入）。

## 链路各环节结果（对照卡面目标 1）

| 环节 | 结果 | 证据 |
|---|---|---|
| 环境预检 plugin_list_available is_instrument>0 | **PASS**：994 插件中 17 件 instrument（Grand Rhapsody Piano/Surge XT/Twin 3/Electric88…） | summary.phases.precheck |
| 编曲话术触发 arrangement 轮 | **PASS**：消息「给这条 MIDI 轨加一个钢琴音源」→ 渲染后 system prompt 含 **Capability pack: arrangement** 全文（Domain/Preconditions/Examples）+ `rack_add_node tool=rack.add_node` 工具行 + `start_render tool=render.start` 工具行（**缺口 #7 修复实栈生效**）——段 1 注册链②四披露面真栈实证 | logs/agent_prompt_render.jsonl（三锚点本 run 提取命中） |
| semantic search type=instrument | **PASS**：模型调用 plugin.semantic_search → 首位=Grand Rhapsody Piano Stereo（is_instrument:true, category Instrument\|Waves, semantic_types[instrument,100]）+ plugin.search 补充 | chat_response.json executed 3 行 |
| rack_add_node（synth Z2） | **FAIL @治理边界**：模型组调 rack_add_node（arg_keys 含 cmd/plugin_identifier/plugin_path/track_id/x,y/**zone_id**——zone 值未落盘不可考）→ **harness pca_processor_load_gate 拒绝**：`missing non-forgeable exact selection authorization`（agent_last.log:62 WARN；executed 行 status=error/postcondition=rack_node_present failed） | chat_response.json 第 3 executed 行 + agent_last.log |
| 模型行为（门后） | 诚实停泊 needs_clarification：点名 Grand Rhapsody Piano + Track 1，请用户在插件浏览器明确选中后确认（未绕门、未幻觉成功） | summary.phases.chat.reply_excerpt |
| 写 clip / 渲染非静音 | **未到达**（装载未成→链停止，驱动器按预写④即停） | — |

## 根因定性（取证结论，非执行侧修复）

- **PCA 装载门行为=按设计**：`enforceAgentProcessorLoadGate`（harness.go:2242-2266）对一切 agent 面装载命令（rack_add_node/plugin.load_to_rack/instantiate_plugin）要求**不可伪造的精确选择授权**（`semanticPluginSelectionAuthorization`，Go 具体值类型 JSON 不可造）。CAP-BOUNDARY-1 冒测曾把同一拒绝钉为**正确行为**。授权的三条既有来源：
  1. **治理推荐流/工作流确认期**：plugin_grabber 装载工作流在 confirm 期盖 `AuthorizeSemanticPluginSelectionLoad`/`AuthorizeProcessorSelectionLoad`（plugin_grabber_workflow.go:1044-1065）——**本 run 消息未路由进该工作流**（语义入口把它送进了 agent-loop 能力轮），故无授权可盖；
  2. UI 插件浏览器用户选中（模型回复正是引导此路）；
  3. full-project-access 轮（`explicitFullProjectAccess` → promoted PCA catalog/certification token 解析，harness.go:2258-2282）。
- **缺口定性（设计级，非段 1 注册缺陷）**：编曲能力轮（本卡新建）**披露面完整**（工具可见+模型正确组调）但**执行面无授权桥**——能力轮内模型自行 search→load 的装载无法过门；授权桥只存在于工作流路径与 full-access 路径。卡面前提「装载走 rack_add_node 工作流」在能力轮路由下不成立。

## 上交主管处置选项（不自行扩域）

- **A. 路由面**：编曲装载话术优先路由 plugin_grabber 装载工作流（语义入口/词表调整；工作流自带确认期授权盖）——改动小，但绕开能力轮直调形态；
- **B. 能力轮授权桥**：agent-loop 内模型 search 命中唯一 instrument 候选→停泊 pending plan→确认期 server 盖 AuthorizeSemanticPluginSelectionLoad（同工作流确认期模式）——结构改动中，治理等价（仍是用户确认授权）；
- **C. 烟测驱动器供选中上下文**（模拟 UI 浏览器选中→requestContext authorization）——最小改（驱动器面），但验证的是「选中后」形态，编曲自由话术→装载的原始缺口仍在；
- **D. full-access 轮跑通出声链**（explicitFullProjectAccess 通路现成）——可最快拿到「装载→clip→渲染非静音」的出声实证（卡面目标 1 的后半），授权面形态另案。

## 计费

- 6 LLM 调用（message_loop 回合+repair 面，telemetry jsonl 在卷；usage 字段读 0=该引擎 telemetry 形态，同 266/276 先例如实记录）——估 **≈¥0.01-0.05**（flash 引擎同规模回合实测带，远低于 ≈¥1-3 预算上界）。

## 占栈协议收口

- 占用登记 `7b6469b0`（01:58 推 main，281 候栈让位注记）；≤60s 复核 01:58:20（fetch 重读 owner 在场+进程/端口全零）；run 后 owner 亲拆 VitAgent 46524/VitApp 33716，**TEARDOWN_ALL_ZERO 亲核 02:01:26**（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+四类进程+smoke python 全零）；释放登记随本笔推 main。
- 工件本目录（RUNBOOK/console/summary/chat_response/fixture project/agent log/telemetry/prompt render/binary sha/gui isolation/evidence）；随 port/arr-srcload-1 入库。
