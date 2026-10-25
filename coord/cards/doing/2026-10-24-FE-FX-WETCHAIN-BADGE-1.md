# FE-FX-WETCHAIN-BADGE-1：FX 轨卡内部湿链徽标化呈现（B 方案）

- 发卡：GLM 主管决策流 / 2026-10-24 晚窗（依据=[decisions/2026-10-24-fx-badge-and-vittape-rename.md](../../decisions/2026-10-24-fx-badge-and-vittape-rename.md) §一——用户 B 方案裁定，关闭 176 勘察上交挂账）
- 派发确认：已确认（用户现场裁定：传统 DAW 无 A 形态/画布连线臃肿/FX 效果器不该显示在发送画布）
- 验收负责人：GLM 主管决策流
- 池序 185；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P3（体验补全，非差口）/ 0.25-0.5 天 / 依赖=176 已合入（787150f）；模型分级：L1 / PC 前端执行会话
- 并行：**与 180 串行（主管补注 2026-10-24 gate：范围⑤真栈腿断言须改 stack_probe_sendroute_macro.gd L 腿=与 180 警告横幅同文件，不可并行）——顺序=180 先（极小卡）→185 后**；真栈腿按 §2.2 错峰。
- **主管补注 2026-10-25：180 已验收合入（前端 main=ee03262）——本卡解锁可领，领取基线=ee03262**。顺带件：180 横幅第 7 行单字笔误「他会在飞栈」→「他人在飞栈」（stack_probe_sendroute_macro.gd 同文件顺带修，零额外文件域，180 验收留档）。

## 目标

FX 轨卡（SendIn/总线对卡的 FX 侧）上以**徽标/摘要**呈现内部湿处理链：效果器名（或类型 icon+数量），随 rack 变更经既有拓扑回执同步面刷新。**红线：零画布连线、零锚点语义改动、零 `can_connect_rack_topology` 门触碰**（B 方案的定义性边界）。

## 范围

①数据派生：徽标内容从同源 pluginList+rack 成员序派生（176 勘察锚点：内核 `normaliseFxChain` 反环门 2 规整形态=receiver(AuxReturn)@头+rack 成员+wetSend(AuxSend)@rack 后，SendRouteService.cpp:365-408——前端只消费既有 repository 拓扑回执，不加新请求面）；②呈现：FX 轨卡头/尾部徽标行（仓内既有徽标先例形态勘察择型——如 166 的路由徽标/调色板邻域），空 rack=空徽标或「（空）」，侧链/悬空形态 fail-closed；③刷新：随 `_apply_send_route_node_title` 既有回执同步面刷新（183 同位），拖动/弹窗操作不互抢；④探针：独立探针腿断言（徽标在场且内容与拓扑一致/rack 装载与卸载后刷新/普通轨无徽标/空 FX 轨形态）+K3 等既有探针零回退；⑤真栈腿（SendRouteGuiSmoke 模式，§2.2 登记）徽标断言入栈探针 L 腿（装载→徽标出现→卸载→徽标消失）。

## 文件域

以勘察固定（预期 `app/rack/Vit_Graph_Rack.gd` 对卡呈现邻域+`rack_send_route_rules.gd` 派生纯函数+探针件+stack_probe L 腿）。越域即停。

## 验收标准

独立探针全绿+既有探针零回退（K3/popup/realinput/shrink abc/start_feed/fader 35）+真栈腿 exit 0（徽标装载/卸载断言）；工件入主仓 coord/runs/FE-FX-WETCHAIN-BADGE-1/。

## 停止条件

pluginList 回执面不含 FX 轨 rack 成员（数据源与勘察假设不符）→ 锚点上交再裁。

- 领取：（2026-10-25 晚窗 / 主仓 origin/main=c6cef2da / 前端仓 main=ee03262（180 合入亲核） / owner=ZCode GLM-5.3 L1 PC 前端执行会话（FE-FX-WETCHAIN-BADGE-1 专属） / 分支 port/fe-fx-wetchain-badge-1 / worktree D:/Godot/project/vit-daw-frontend-wetchain1 / 领取提交=本提交）
- 回执：（2026-10-25 晚窗——**停止条件触发，上交裁定**；headless 全绿+实现就位，真栈首轮 exit 1 两族失败，族 2=数据源与勘察假设不符）
  - **上交件**：[runs/FE-FX-WETCHAIN-BADGE-1/ANALYSIS-stopcondition.md](../../runs/FE-FX-WETCHAIN-BADGE-1/ANALYSIS-stopcondition.md)（触发证据/两侧源码根因/替代数据面亲核/待裁三点）。
  - **触发**：真栈 L12 实证 list_tracks 轨级 pluginList 只呈现 rack 容器实例（名 "New Rack"，tracktion 新建 rack 默认名），rack 成员（TDR Nova）住 rack DAG 内——卡面①「从同源 pluginList+rack 成员序派生」字面路径不成立。**替代面已亲核**：get_project_state 回执 `tracks[].rack.nodes`（内核 CommandDispatcher.cpp:1797 createRackState）携带成员全字段（name/type/item id 序），前端 repo 已缓存已消费（project_repository rack_nodes 快照=167 侧链口同一面）——**零新请求面约束可保持**，待裁定换源继续（预计 ≤0.25 天，呈现层/探针结构可复用）。
  - **headless 断言面**：独立腿 probe_fx_wet_chain_badge **35/35 PASS**（纯函数 13：成员序派生/aux 对+系统隐藏剔除/畸形跳过/文本截断「（空）」/单/双/≥3 前二+计数；装配 22：合一 icon 徽标 fail-closed 隐藏→快照显化/红线零新增画布连线/普通插件卡与 SendOut 卡无徽标/展开 SendIn 卡同文本/轻刷新不重建（实例同一）/拖动门不互抢/空 rack「（空）」/悬空+缓存 miss 隐藏/折叠重建保持/构建面守卫）。**零回退**：K3 313 / popup 111 / realinput 15 / shrink a·b·c / start_feed 244 / fader 35 全绿（popup 首败=worktree 缺 .godot/imported 产物——基线 ee03262 对照全绿+import 重建后全绿，环境因素非 diff；183 回执 worktree 缓存坑的第二形态，后续新 worktree 应复制 .godot 后先跑 --import）。
  - **真栈首轮**：sendroute_gui_20261025_193814 exit 1（81 断言 8 败）——族 1 五败=主检出 build 目录预存边 1032→1037@+6.0 污染（183 首轮同款；根因=dev_agent_smoke VIT_PROJECT_XML 仅 journey berth 设置，复跑修正=调用方 shell 显式透传，已录上交件 §四）；族 2 三败=停止条件触发面（上交）。§2.2 登记 e5facf80→释放复核进程/端口=0。
  - **commit**：前端仓 `port/fe-fx-wetchain-badge-1`（rules 两纯函数+Vit_Graph_Rack 徽标行/缓存/刷新面+独立探针+stack L12+180 横幅笔误顺带修）；主仓 `port/fe-fx-wetchain-badge-1-stack`（run 工件+headless 工件+上交件）。
  - **端测边界声明**：headless 探针=注入回执面（fake transport）；真栈腿仅首轮（含污染与停止条件触发），**徽标装载/卸载端到端绿尚未达成**——待裁定后换源复跑（VIT_PROJECT_XML 隔离透传）补齐。
- 验收：（进行中——主管停止条件裁定 2026-10-25 晚窗：**换源批准，本卡继续**）
  - **裁定①（换源）**：数据源从 list_tracks 轨级 plugins 换 **get_project_state 的 tracks[].rack.nodes**（前端消费 repo rack_nodes 快照面 get_rack_nodes_by_track，project_repository.gd:106-114/:287——167 目标选择器同源面）。「零新请求面、只消费既有回执」实质约束保持；**不开内核改面**（list_tracks 补成员方案否决——越本卡文件域且替代面已足）。
  - **裁定②（成员序）**：rack.nodes 序列序=卡面①「rack 成员序」的等价物，认可；若复跑发现回执序不稳定（去重/排序异常）再上交。
  - **裁定③（复跑）**：族 1 预存边污染按 183 先例 VIT_PROJECT_XML 隔离副本复跑。**复跑三纪律（188 落地前）**：a) 启动前 fetch PC-RUNTIME-STACK 并即时复核进程/端口（距启动 ≤60s，不得沿用早前检查）；b) ps1 报 already listening→停下人工核归属，禁止静默复用；c) §2.2 登记推送后再启动。
  - **事故注记（主管更正）**：本卡 19:38:07 §2.2 登记成立（先前「未登记」判断系分支未及时可见，更正销账）；首轮与 C1(187) 同刻双真栈腿互染（C1 静默复用本卡自启栈 11900/17316，双方探针在同一内核交错 clear/add——首轮族 1 归因维持预存边，但读回混乱可能含串扰成分）；复跑隔离后以干净结果为准。**换源影响=派生输入+刷新挂点（ANALYSIS §三候选 a：project_loaded 外部变化面追加 repo rack_nodes 重读+轻量标签刷新=与 161 同构最小改）**，呈现层/探针结构/L12 骨架复用，预计 ≤0.25 天。
