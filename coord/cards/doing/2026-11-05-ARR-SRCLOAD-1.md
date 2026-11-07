# ARR-SRCLOAD-1：音源装载最小闭环——编曲语境装载链真栈验证 + 编曲 pack（编曲首张 / 注册链②）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[258 报告](../../reports/2026-11-05-arr-recon-1.md) 卡序草案 + [taxonomy](../../decisions/2026-11-05-capability-domain-taxonomy.md)）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 277；**注册链第 ② 位（前置=276 已验收提交）**；目标=capability_pack 新增 arrangement pack（或扩 midi pack）+ usage 披露补丁；P1 / 1.5-2 天；L1-L2 / PC；真栈烟测腿一次

## 目标（258 报告卡序卡 1 原案）

1. **真栈验证编曲语境装载链**：chat 驱动"加钢琴音源"→ plugin_search / semantic（type=synth）→ `rack_add_node`（synth 默认 Z2）→ 写 MIDI clip（create_midi_clip+add_midi_notes）→ **离线渲染非静音断言**（start_render 后产物非静音）——装载通道出声的第一次实证（A1 修正结论的落地）。
2. **编曲 pack**：装载编排次序（search→rack_add_node Z2→写 clip→试听）+ Z2 语义 + 试听引导 + 与符号分析的衔接占位（详细引导归 283）。
3. **usage 披露补丁**：semantic_search 的 type 值披露（synth/instrument 可见性）。
4. **环境预检**：`plugin_list_available` 断言 `is_instrument` 计数>0；为空按环境前置上交，不视为代码障碍。

## 约束

- 文件域=capability_pack.go（新 pack 段）+ catalog.go usage 披露段 + 烟测脚本扩展 + 测试；**零内核改动**（全部既有命令）；不触 M3 域；`plugin.instantiate` 维持排除（258 验收裁定），装载走 rack_add_node 工作流。

## 验收标准

- 真栈烟测腿 exit 0：装载→写 clip→渲染非静音全链断言过（run ID+工件入回执）；
- pack/usage 改动=引导级，既有九 pack 行为零变化（对照单测）；
- 回执附：装载链各环节返回摘要、渲染产物路径、commit hash。

## 停止条件

- 本机音源库存为空（is_instrument==0）→ 环境前置上交；渲染静音且排查超本卡域→取证上交。

## 领取

- 2026-11-07 / **段 1/N 领取**（装载链勘察[读档零 LLM]+pack 骨架+usage 披露补丁+注册链②；段 1 零真栈零内核——装载链实现+烟测腿留段 2/3，勘察结论先交主管过目）
- origin/main hash：`9e5af73ced275dae3567e77b7a0c522fa02bb273`
- owner：GLM 主会话 L2-worker 子代理（GLM-5.3 / PC）
- worktree：`D:/Vit_DAW_wt_277impl`；分支：`port/arr-srcload-1`（自 origin/main 新建）
- 领取基线：HEAD=`9e5af73ced275dae3567e77b7a0c522fa02bb273`；worktree `git status --short`=空；`git diff --stat`=空（新 worktree 自 origin/main 检出，全净，无领取前已有 diff）
- 前置核对：276（EDU-SYMBOL-CORE-1，注册链①）已终验合入 `be76f431`（review pass `9e5af73c`）✓；doing 两卡（EDU-LEVEL-CONTENT-1 / EXPADMIT-EVIDENCE-RULE-1）文件域与本卡不相交 ✓

## 回执

### 段 1（勘察+pack 骨架+注册链②，2026-11-07）——零真栈零内核，勘察结论待主管过目后再启段 2

- **勘察工件**：`coord/runs/ARR-SRCLOAD-1/recon.md` @ `port/arr-srcload-1` 分支（实现 commit `965046f0` 内）——258 报告内核/agent 锚点全复核有效；**增量发现缺口 #6**（能力触发词表不含音源词→编曲轮装载工具被遮蔽，"给 MIDI 轨加钢琴音源"此前只落 midi 能力，rack_add_node 不可披露）与 **#7**（render.start 不在任何能力白名单→门控轮无法离线渲染）；**未触发停止条件**。
- **择一决策（存档 recon.md §4）**：**新增 arrangement pack**，不扩 midi pack——依据=卡面验收"既有九 pack 行为零变化"硬约束 + 词表/披露分离 + taxonomy 编曲域镜像 + midi pack 刚扩域回归面最小；九 pack 定义零 diff。
- **实现 commit**：`965046f0` @ `port/arr-srcload-1`（已推远端）；diff=6 文件 +295/-1：
  - `agent/internal/tools/capability_pack.go`：新 arrangement pack（装载编排次序 Preconditions：semantic_search type=instrument → rack.add_node Z2 → get_parameters → 写 clip → 试听/渲染；preset 缺口引导；analyze_transformations 衔接占位归 283）
  - `agent/internal/tools/catalog.go`：plugin_semantic_search usage 披露补丁（type 值域 effect|instrument|synth + include_instruments——实现面早已支持，纯披露）
  - `agent/internal/chat/goalrunner_chat.go`：arrangement 触发词表（编曲/配器/音源/合成器/乐器/钢琴/synth/synthesizer/instrument/vsti/sampler/piano）+ `case "arrangement"` + `agentLoopArrangementTools()` 第四披露面白名单（与 pack.Tools 对齐；含 render.start 修缺口 #7）
  - 测试：`catalog_test.go`（pack 骨架钉+usage 披露钉）、`server_test.go`（276 同款三层钉：触发词命中+允许列表含装载链+prompt 披露面工具行）
  - `policy.go` 零改动（无新命令，plugin 族 spec/policy 注册面勘察核实在场——recon.md §2.1）
- **测试（全部退出码 0）**：`go build ./...`=0；`go test ./internal/tools/ ./internal/policy/ -count=1`=ok；`go test ./internal/chat/ -count=1`=ok（152.0s）；新增 3 测试全过（TestArrangementCapabilityPackSkeleton / TestPluginSemanticSearchUsageDisclosesInstrumentType / TestAgentLoopArrangementCapabilityDisclosesLoadChain）。
- **端测边界声明（AGENTS §5）**：本段零真栈——pack/披露改动仅单测+编译级验证；装载链真栈验证（chat 驱动装载→写 clip→渲染非静音）留段 2/3 烟测腿；既有九 pack 行为零变化由对照单测钉（九 pack 定义零 diff）+全包回归绿背书。
- **段 2 就绪**：勘察结论交主管过目（recon.md §6 留题：VSTi 索引预检/装载双通道路由分流/plugingrabber 词表缺钢琴单独词[域外微补丁建议随段 2 处置]/渲染静音断言脚本面）；过目通过后段 2 开装载链烟测腿（真栈按 PC-RUNTIME-STACK.md 登记协议，当前栈被 269 verifyrun1 占用中）。

### 段 2（装载链烟测腿，2026-11-07 01:58–02:01）——py exit 4=预写④装载链缺陷，取证上交待主管裁定

- **单 run `20261107_015840`**：占栈协议全链（登记 `7b6469b0` 01:58 推 main+281 候栈让位注记→≤60s 复核 01:58:20→启栈→run→**TEARDOWN_ALL_ZERO 亲核 02:01:26**→释放登记随本笔）；kernel=wt_228 `F9E3DA0A…0D67A` 只读复用；agent=wt_277impl 现场构建 SHA256 `62F444B7…D28B3`；工件 `coord/runs/ARR-SRCLOAD-1/smokerun1/`（11 件）@ `port/arr-srcload-1` `88f41837`；计费 6 LLM 调用 ≈¥0.01-0.05（预算 ≈¥1-3 内）。
- **§8 口径**：RUNBOOK 跑前成文（单 run 无重跑/预算/失败分类四列）；**分类④命中即停已兑现**——不重跑、零 Go 生产码改动。
- **链路结果**：预检 **PASS**（994 插件 17 件 instrument：Grand Rhapsody Piano/Surge XT 等）；**段 1 注册链②真栈全实证**（arrangement pack 全文进 system prompt + rack_add_node/render.start 工具行在场=缺口 #6/#7 修复生效；模型正确走 search→semantic type=instrument 命中钢琴首件→组调 rack.add_node 含 zone_id）；**装载 FAIL @治理边界**：`pca_load_gate: missing non-forgeable exact selection authorization`（harness.go:2242 enforceAgentProcessorLoadGate；CAP-BOUNDARY-1 曾钉同一拒绝为正确行为）→ 模型诚实 needs_clarification 停泊；写 clip/渲染非静音未到达。
- **根因（evidence.md）**：设计级缺口——agent-loop 能力轮披露面完整但**无授权桥**；不可伪造选择授权（semanticPluginSelectionAuthorization）只在工作流确认期（plugin_grabber_workflow.go:1044-1065 盖 Authorize*）/UI 浏览器选中/full-access 轮三条既有路径产生，本 run 消息路由进能力轮故无授权可盖。**非段 1 注册缺陷、非内核缺陷**。
- **四处置选项交主管**（evidence.md）：A 编曲装载话术优先路由装载工作流；B 能力轮补确认期授权桥（search 唯一命中→pending plan→confirm 期盖授权，治理等价）；C 烟测驱动器供 UI 选中上下文（最小改但只验「选中后」形态）；D full-access 轮先拿装载→clip→渲染非静音出声实证（通路现成，授权面形态另案）。
- **段 2 顺带交付**：已授权域外微补丁落地（plugingrabber 词表补钢琴/piano，`commands_test.go` 正负例钉）；烟测扩展 `scripts/arr_srcload_smoke.py`+`dev_agent_smoke.ps1 -ArrSrcLoadSmoke`（透传缝四处预声明）；prep commit `37598d75`。
- **段 3 就绪度**：待主管对④裁定（A/B/C/D 择一或组合）后定——出声实证腿（装载→clip→渲染非静音）在 D 路径下可最快复用本驱动器（一次 full-access chat 改参即可）；B 路径则需新实现卡。

### 段 3a（D 路径 full-access 出声实证腿，2026-11-07 09:53–09:55）——py exit 4 @clip（装载环节实证成立；渲染非静音未到达；断言形状缺陷取证上交）

- **单 run `seg3a_20261107_0950`**：占栈协议全链（登记 `f325d341` 09:52 推 main+重读确认→≤60s 复核 09:53:03→启栈→run→PS1 throw 后 owner 亲拆残留 VitAgent 45128/VitApp 19676→**TEARDOWN_ALL_ZERO 亲核 09:55:23**→释放登记随本笔）；kernel=wt_228 `F9E3DA0A…0D67A` 09:51 亲核只读复用；agent=wt_277impl 现场构建 SHA256 `62F444B7…D28B3`（Go 树自段 2 零改动同哈希如实记）；prep `0fd5e239`（--full-access 改参+RUNBOOK）；工件 `coord/runs/ARR-SRCLOAD-1/seg3a_20261107_0950/`（随 port/arr-srcload-1 `79413c4e`+`ea2c0963`）；计费 5 行 deepseek-flash ≪¥0.1（预算 ≈¥1-3 内）。
- **§8 口径**：RUNBOOK 跑前成文（单 run 无重跑/预算/失败分类 0-2-3-4 含「full-access 轮仍被门拦=前提推翻→4」分支）；**单 run 已兑现**——exit 4 后未二次启动栈。
- **链路结果（较段 2 前进到渲染前一步）**：预检 PASS（17 instrument）→ full-access 晋升 PASS（baseline=manual→POST /agent/authority→full_project_access persisted+GET 复核；261 G5 先例路径）→ **chat 单回合装载直执 PASS**：stop_reason=done / **needs_confirmation=false**（免确认卡直执）/ 模型走 plugin.semantic_search→plugin.search→rack.add_node（组调 identifier+path+track+zone 全精确，未被 PCA autonomy 附录「any other identifier is rejected」劝退）→ **装载门放行且内核执行**：rack.add_node status=ok、zone_id=Z2、plugin_id=1013、**plugin_instance_ready=true、load_state enabled=true processing=true load_error 空**、graph diff `node_add rack_item_id=1012 track_id=1007`（Grand Rhapsody Piano 实例在跑）→ clip 1014 写入 16 音回读一致 → **FAIL @装载可见性断言**：track.plugins 顶层=[New Rack, Volume & Pan Plugin, Level Meter] 不含钢琴名 → start_render/非静音未到达。
- **exit 4 根因（evidence.md §3）**：**驱动器断言形状缺陷（域内）非装载失败**——rack_add_node 把音源装为轨上 rack 容器内部节点（内核预期形状），段 2 遗留断言按「顶层插件行」书写且从未被触达（段 2 死于更早的门拒），本 run 首次触达即暴露。装载环节本身实证成立。
- **admission-path 如实分账（evidence.md §4）**：跑前+跑时双证 instrument 晋升 0/17（v1 116/v2 89 全效果器）→ 本 run 装载放行**必经**门内 certification-token 回落（`~/.vit/pca_certauth.token` armed，门自身既设路由非绕门；agent 日志零行 pca_load_gate）。「通路现成」按此修正：**物理通路=现成（本 run 实证）；授权形态=cert-token 回落**，治本归 273。
- **交主管**：①exit 4 定性与断言修形（rack 节点查询形状）后是否再派一跑拿渲染非静音实证（§8 纪律下执行侧不重跑）；②admission-path 表述修正采认；③段 3 未收口（渲染非静音未获证），**卡留 doing/**。

## 验收

- （主管回填）
- **段 1 验收（2026-11-07 凌晨，主管）：pass+段 2 留题四项过目全部通过**——勘察锚点全有效（停止条件未触发）+两条新缺口（#6 音源词遮蔽/#7 render.start 白名单）发现即修；择一决策（新增 arrangement pack 不扩 midi）理由充分追认；主管交叉复跑三层测试钉全绿。留题处置：①VSTi 索引预检=段 2 前置合理 ②双通道分流验证=段 2 烟测覆盖面 ③plugingrabber 词表缺钢琴词=**域外微补丁授权**（276 第四披露面同族一行级，段 2 顺带）④渲染静音断言=段 2 内设计。段 2（装载链烟测腿）排栈队列。
- **段 2 验收+四选项裁定（2026-11-07 凌晨，主管）：pass（取证+实证）+D 先行+B 开卡 273**——段 1 注册链②真栈全实证（arrangement 轮+pack 进 prompt+缺口 #6/#7 修复实栈生效+模型正确 search→rAdd）；FAIL 环节=PCA 装载门治理边界正确拦截（能力轮披露面完整但无授权桥=设计级缝，与 261 G5 同族——模型诚实停泊未绕门）；**裁定**：①D 先行=复用驱动器 full-access 改参单 run 拿「装载→clip→渲染非静音」出声实证（物理链先证通）=段 3a ②B 开卡 273（能力轮确认期授权桥——装载工作流同款治理等价）=治本收口 ③A（话术优先路由装载工作流）触判断权域挂 PD 备选 ④C 弃（只验选中后形态）。域外微补丁（钢琴词）已按授权落地追认。
