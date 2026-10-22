# FE-SEND-CABLE-EXTERNAL-1（164）真栈收口——外部建 aux 边后 GUI 电缆自动出现

- owner=GLM-5.3 L2 PC ZCode 执行会话（164 卡会话）；2026-10-22 10:27 登记（2699d1d2）–10:3x 释放
- 被测：前端 worktree D:/Godot/project/project-vit-daw-frontend-cableext @ port/fe-send-cable-external-1 @ **8fb415c**（含本卡改动；基线 c5cac57=161 已合入面）；agent=stack worktree 构建 SHA256 前 16=**463C63C12FC3A7EE**（=86e854ba 基底代码，Go 零改动）；内核=主检出 K3 build-feguik3 Release VitApp.exe SHA256=**722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC** 只读复用（10:26 哈希亲核一致）；不启 UI
- run=**cableext_gui_20261022_1028**：`dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_cableext_stack -StartKernel -RestartAgent -SendRouteGuiSmoke -KernelExe <主检出 K3> -GodotProjectRoot <前端 worktree> -GodotExe <Godot 4.6.1 console>` **脚本 exit 0，真栈探针 41/41 PASS**（SUMMARY_JSON checks_failed=0）
- §8 记账：LLM=0（chat/confirm 腿走脚本既有白名单话术非验收面，161 同款）；一次过零重跑；零环境中断

## ① 取证锚定（卡面问题①：aux 边在哪个只读回执数据源）

**结论：内核回执族没有任何一份携带显式 auxsend→auxreturn 边；停止条件不触发**（无需内核新命令——边可从两份现成只读回执的内核确认事实派生）：

- 内核侧（VitApp/Source/Service/SendRouteService.cpp `ensureSourceSendPair`）：connect 把 SendOut(auxsend) 与 SendIn(auxreturn) **成对插入源轨 pluginList，SendIn 插在 SendOut 紧后**（pluginIndexOf(sendOut)+1）——相邻对本身即内核确认的连接。
- rack 边回执（CommandDispatcher `createRackState`）只枚举 **rack 实例轨**的 `rack->type->getConnections()`；aux 对住在 track pluginList（非 rack 成员），rack-less 源轨（161 已证的 M2 场次③形态）rack bundle 恒空 → `fetch_edges_for_track` 恒空——这就是「卡有、线无」的机制解释（161 修了卡面数据源，线层无从画出）。
- 可用只读回执：`list_tracks`（每轨 plugins 数组，161 已拉取缓存=卡面数据源，**保序**）+ `track.send.list`（轨级拓扑，标签/槽位用）。派生输入=前者。

## ② 修复面（确认集合成，防幽灵线门零放松）

- `rack_send_route_rules.aux_pair_edges_from_plugin_entries`（纯函数）：相邻 auxsend→auxreturn 派生 audio 边 payload（pin 1/1，key 形 `sid|audio|did|audio` 与 rack 边同构）；**非相邻、FX 轨 receiver→wet 乱序、同 id、非数组输入一律 fail-closed 不产边**。
- `Vit_Graph_Rack._rack_server_edges_for_bound_track`：rack DAG 边（repository）+aux 对派生边，喂**同一** `sync_edges_from_server` 确认集（load_nodes SKIP/REBUILD 双路径）——`_confirmed_rack_edge_keys` 仍只含该函数产物，外部边零特例，`_rack_graph_connection_is_kernel_confirmed_fast` 判定链未动。
- 外部变化链复用 161：project_loaded→track.send.list+list_tracks 双拉取→条目缓存变化→load_nodes 重建→边同步（卡+电缆同帧同链出现；断开=条目清空→重建→边移除）。
- aux 对电缆 GUI 拖断改道既有 send 对移除链（track.send.disconnect，与右键成对移除同命令）；rack.disconnect_pins 零触碰（探针钉死）。

## 三面证据

1. **K3 探针 231→247**（tools/probe_rack_sendroute_macro.gd）：纯函数套件 7 断言（相邻对/形状/非相邻/乱序/尾部悬空/畸形输入/同 id）+场景 12.5 九断言（内核形态回执→卡+电缆同现+过确认门+确认集 key+无双轨重复卡+拖断改道 rack 面零泄漏+清空回执电缆/确认集同链消失）。
2. **9 套回归零回退**：popup 111/111、shrink a/b/c PASS、start_feed 244/244（全绿）。
3. **真栈 41/41**（本 run）：K3 六腿 L1-L6 零回退 + 161 L7 零回退 + **本卡 L8 八断言全过**：foreign request_id connect→SendIn 卡与 SEND ▸ 卡零操作同现→auxsend→auxreturn GraphEdit 连接出现→**过内核确认门（防幽灵线门保持）**→派生边 key 入 `_confirmed_rack_edge_keys`→恰一 send-out 卡（无重复）→外部 disconnect（foreign id）→**卡与电缆同链消失+确认集回零**（2s 快照冷却按生产纪律等待，未绕过）。

## 端测边界声明

- headless 探针断言到 DOM/命令面（连接表+确认集+卡节点），**不覆盖渲染像素**（电缆视觉形状/光晕）——归用户手测（入口=Godot 前端，既有裁定）。
- 本腿不覆盖：GUI 拖缆**新建** send（非断开）路径（现有形态=经 SEND 槽位/目标选择器，右键空白同入口）；FX 轨画布内部（receiver→rack→wet）的湿链连线未派生（receiver/wet 非 rack 成员且乱序，属 166 合一呈现卡域）。
- 已知邻近面：167（FE-MIXER-FX-STRIP-AUDIT-1）在同探针文件 231→239 分支待合入，与本卡 247 增量无函数/块交叠，合并时决策侧注意上下文冲突。

## 工件

- `cableext_gui_20261022_1028/sendroute_gui_smoke_console.txt`（探针 41/41 完整面）+ `cableext_stack_console_tail.txt`（ps1 console 尾段，诚实标注截取）
- 前端实现 commit=8fb415c @ port/fe-send-cable-external-1；本工件目录随 port/fe-send-cable-external-1-stack（86e854ba 基底）入库待决策侧合入
