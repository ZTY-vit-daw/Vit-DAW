# FE-SENDROUTE-MACRO-GUI-1（K3）：send 节点+宏控制层前端实现（一次落地）

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=设计件 §9 定稿+用户「一起执行，并不着急」——**Vit_Graph_Rack.gd 同文件一轮改净，不与宏升级拆两轮**）
- 派发确认：**已确认（用户 2026-10-16 同上）**；验收负责人：GLM 主管决策流
- 池序 130；目标仓库=Godot 前端仓（D:\Godot\project\vit-daw-frontend）；P1 / 3-4 天 / **依赖：K1（127）合入 + 宏设计（129）定稿**
- 模型分级：L2 / GLM 执行会话（前端线，PC/Mac 皆可——领取时与 Mac 线核对前端仓文件域）

## 规格（权威=设计件 §9；宏部分=129 设计件定稿后回填）

1. **send 节点渲染**：SendOut（穿通+副本岔出）/SendIn（湿声注入）串联节点——与效果器同构的机架图成员（v4 样图定稿形态：SEND ▸ / ◂ SEND tag、目标名、量、拖排序换位=改 tap/汇入点、右键成对移除、双击跳 FX 轨画布）；**源轨画布无跨轨实体 icon**；
2. **创建流程**：机架右键/菜单「新建发送」→目标选择器（已有轨分组+搜索/每轨侧链口展开/「＋新建 FX 轨」现场建轨命名默认 Reverb Bus 2 类）→选定即连（乐观 pending 边→内核确认回读，rack_pending_edge_rules 回滚模式）；
3. **控制面**：通道条 SEND 槽位（FX 名+量+通/断开关，主控面）；FX 轨画布顶部只读来源列表（点击跳源轨）；
4. **宏控制层**：按 129 定稿实现（控制层节点+binding 虚线+send amount 绑定面+两层线型）；启用 EFFECT_RETURN 枚举+轨列表角色徽标；
5. **命令面**：rack_client 封装 5 命令（track.send.*）+快照面扩展 send 边行（get_rack_edges_for_track 同构缺口 d）。

## 文件域

前端仓 app/rack/{Vit_Graph_Rack.gd 等}、app/shell/side_panel、rack_client.gd、vit_daw_track_kind.gd、rack_ipc_payload_rules.gd；GH001 推送限制照旧（补丁/清单存档主仓 run 目录）。

## 验收标准

- 渲染面烟测（FE 系列脚本模式扩展）：send 节点渲染/选择器流程/SEND 槽位/宏控制层 DOM 级断言；
- 真栈（K1/K2 已合入）：GUI 建发送→track.send.list 读回一致→调量/换位/断开回读；
- **[等待用户] 手测**：Godot 拉起→进工程→机架新建发送→听感干湿（手测入口裁定：从 Godot 拉起，环境变量同 shell 设置）；
- 前端仓测试（tests/test_project_repository_rack_macros.gd 等）零回退。

## 停止条件

- 129 宏设计未定稿不得领取本卡 send+宏合并实现（可申请拆 send 先行的例外，须用户单点确认）；
- 快照面/IPC payload 扩展与内核 schema 偏差 → 停手对齐 K1。

- 领取：（时间 / 前端仓+主仓基线 / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
