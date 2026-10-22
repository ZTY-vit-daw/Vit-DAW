# FE-SEND-UX-UNIFY-1（166）真栈收口——机架 send 路由合一呈现（默认折叠+配对配色+成对徽标）

- owner=GLM-5.3 L2 PC ZCode 执行会话（166 卡会话）；占用登记 f305bf35（19:1x）→释放复核本条末行
- 被测：前端 worktree D:/Godot/project/project-vit-daw-frontend-sendux1 @ port/fe-send-ux-unify-1 @ **88b1e71**（含本卡改动；基线 bf9c10a=164+167 合并态=卡面指定基线）；agent=stack worktree D:/Vit_DAW_wt_sendux1_stack @ **f9f96ddf**（本地 main=生产代码，Go 零改动；origin/main 尚未含 164 stack 合入故基 local main）；内核=主检出 K3 VitApp.exe SHA256=**722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC**（占用前亲核=164/161 同一二进制只读复用）；不启 UI
- run=**sendux_gui_20261022_1906**（run id 取自首建目录时戳，实际启动 19:2x）：`dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_sendux1_stack -StartKernel -RestartAgent -SendRouteGuiSmoke -KernelExe <主检出 K3> -GodotProjectRoot <前端 worktree> -GodotExe <Godot 4.6.1 console>` **脚本 exit 0，真栈探针 51/51 PASS**（SUMMARY_JSON checks_failed=0）

## 实现面（用户 UX 裁定三点的落地面）

1. **默认折叠**（裁定 1）：load_nodes 配对感知 spawn——相邻 auxsend→auxreturn 对（与 164 `aux_pair_edges_from_plugin_entries` 同一相邻判定=内核 ensureSourceSendPair 形态）默认渲染单一合一 icon（SEND ◈，含目标名+量+展开按钮+链位操作提示）；展开=完整还原 ▸/◂ 对卡（工程视图：端口/参数口/电缆全回 164/K3 渲染面）；折回按钮在对卡 SendOut 上。加载/外部新建恒折叠（展开偏好=会话内 route_key 集，不持久化——view_state 持久化为卡面可选项，本轮不取）。侧链 auxsend 与非相邻形态不折叠（fail-open 保持现状逐卡）。
2. **配对配色**（裁定 2）：`RackSendRouteRules.send_route_color`——路由键（send_edge_key=source→fx；拓扑未到悬空期 sendpair: 代键，拓扑到达 _update_send_route_node_labels 改写+迁移展开偏好）哈希入固定 8 色调色板，跨会话稳定。三面同色消费：合一 icon（tag 字色+徽标 chip）/展开对卡（tag 字色+徽标 chip）/_draw 电缆（`_send_route_cable_color`，含交互 LOD）。
3. **成对徽标**（裁定 3）：徽标=目标轨名（悬空退「悬空」占位），同一 display 派生保证折叠/展开两态同徽标同色。

## 红线核对

- **纯呈现层聚合**：合一节点不带 kernel_plugin_item_id（plugin_id=sendpair: 前缀→边端点解析天然不匹配，零幽灵连接）；对卡事实（pluginList 相邻对）唯一读源不动；`_last_merged_bound_plugins`=已加载行缓存非新数据层。
- **折叠态链位可操作性**：合一无端口+「链位/电缆操作请展开」入口提示；展开即回链位/电缆操作面（164 拖断改道 track.send.disconnect 语义原样）。
- **164 电缆数据链零改**：折叠态派生边 key 仍入 `_confirmed_rack_edge_keys`（探针钉死），防幽灵线门零放松；`aux_pair_edges_from_plugin_entries` 函数零触碰。

## 三面证据

1. **K3 headless 探针 255→289**（tools/probe_rack_sendroute_macro.gd，worktree 88b1e71）：166 纯函数套件 7 断言（调色板尺寸/同键稳定/异键异色/取色在表/空键中性/徽标目标名/悬空占位）+场景 1 折叠默认（合一渲染/对卡隐藏/标题/徽标/路由键/配色/入口提示/**无端口**）→展开还原（对卡/合一移除/路由键色徽标一致/电缆色/tag 色）→折回（合一回来/对卡消失/徽标色保持）→再展开 23 断言+场景 12.5 外部建对默认折叠（合一零操作呈现/对卡隐藏/**折叠态派生边 key 在确认集**）+展开底座断言保持（对卡+电缆+确认门+无重复卡+拖断改道 rack 面零泄漏）+三面同色 3 断言+清空回执合一/电缆/确认集同链移除。既有场景 2-13/K3.1/K3.2/mixer167 断言面零回退（12 非相邻夹具行为不变腿原样过）。
2. **9 套回归**：popup 111/111、shrink a/b/c PASS、start_feed 244/244（终轮）。**shrink_c mixersync 腿不稳定面如实记录**：19:0x 连续三轮（本支×1+基线 bf9c10a×2）各挂 1-2 腿（mixersync add/delete/fader 时序腿），终轮全过——基线与本支同现=预存不稳定非本卡回归（167 mixer 域），已上交决策侧留意。
3. **真栈 51/51**（本 run）：K3 六腿 L1-L6+161 L7 零回退+**166 新腿全过**——L7：外部 connect→**合一 icon 零操作自动出现（默认折叠）+折叠态对卡隐藏+量标签+repository 拓扑落库**；L8：路由键=send:source>fx（真拓扑派生）+徽标=FX 轨名+**配色=固定调色板哈希**+**折叠态派生边 key 在确认集（164 链保持）**+展开还原对卡+auxsend→auxreturn 电缆出现过内核确认门+恰一 send-out+**展开对卡/电缆三面同色同徽标**+外部 disconnect→合一/电缆/确认集同链消失。

## 事故记录（2026-10-22 19:00:25-29，已自报 main 8920e1e4）

166 会话在 159（SOFTGUIDE-RESCHEDULE-1）占用期内未登记触碰其在飞栈（stack_probe 干跑误判同族第 4 例：134/139/161/166）——对 run_20261022_1846 执行了完整探针腿，损害边界待 159 侧判读（本 run 时间窗 19:00:25-29）。该触碰轮 L7/L8 断言虽全过，占用非法不计入验收；本表 run=登记占用后（f305bf35）独立干净轮。

## 端测边界声明

- headless/真栈断言到 DOM/命令面/连接表/确认集/节点 meta 与配色派生值，**不覆盖渲染像素**（合一 icon 视觉形态/电缆颜色呈现/tag 着色实际观感）——归用户手测目检（入口=Godot 前端，既有裁定；建议目检点：加载默认折叠观感/展开折回手势/三面颜色一致性/徽标可读性）。
- 未覆盖面：折叠/展开偏好的 view_state 持久化（卡面可选项未取）；GUI 手势拖缆**新建** send（既有形态=槽位/选择器入口）；FX 轨画布湿链（164 回执提及归 166 域——本卡裁定内容 1-4 未含，如实声明未做，需与否交决策侧）。

## 释放复核

- 19:2x 拆栈（脚本完成态不带 teardown，owner 手拆 VitAgent 30196/VitApp 30164）→复核 VitApp/VitAgent/vsphub/Godot 进程=0、5555/4445/7878/8787 tcp+udp 监听=0。释放登记随本工件提交。
