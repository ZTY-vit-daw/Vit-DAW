# FE-LANE-CATALOG-REPO-WIRE-1：车道参数目录断供——GUI 生产树 ProjectRepository 装配+喂食接线（手测节第三轮缺陷卡）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=用户手测节第三轮：「加载效果器后列表中没有参数；未加载也无默认音量等参数；选择参数列表全空」）
- 派发确认：待用户确认（手测节当场折卡惯例=已确认可开；如需显式确认以用户下一次指令为准）
- 验收负责人：GLM 主管决策流
- 池序 207；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 缺陷卡 / 0.5 天 / 依赖=无（基线=前端 main 3b5df90）
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰手测节（§2.2 增补③）；与 doing 卡文件域不相交。

## 根因（发卡侧亲核，2026-10-28 前端 main 3b5df90）

1. **ProjectRepository 生产零实例化**：全仓 grep 亲证——生产代码（app/ 与 *.tscn 与 project.godot autoload）无任何处实例化 `vit_dock/core/project_repository.gd`（唯一生产引用=vit_dock/core/ui_motion.gd 无关件）；实例化仅存在于 tools/ 探针（preload().new()+add_to_group("project_repository")，如 stack_probe_autolane_layout_grow.gd:211）与 app/rack/tests。
2. **lane 目录 fail-silent**：`track_envelope_lane_2d.gd refresh_plugin_catalog()`（:357-363）`get_first_node_in_group("project_repository")` 返回 null 即 return——目录/轨道组永不填充；`_set_catalog` 内 `get_volume_pan_plugin_item_id_for_track` 解析同样依赖 repo 在场。两消费路全断=选择参数列表全空（轨道组+插件目录都无）。
3. **探针绿证盲区=装配面差集**：205 prodchain/206 layout_grow V3 的「真 repo+真 IPC 生产链」探针均为**探针自建 repo 实例**经生产方法 `apply_kernel_project_state` 喂食——写入路径真实，但生产 GUI 树中该节点不存在，装配面从未被任何断言覆盖。属风险位①第三案（198 装配面零控件/205 注入面零生产链/本卡=repo 数据源节点零装配）；发卡侧认账：205 卡面「无注入生产链」未把「repo 节点在生产树在场」列入断言面。
4. 用户工程形态：6 轨 fixture 工程（<24 走 legacy 直接行路径）+机架挂 Pro-Q 3——机架 UI 有独立数据链故装载正常；内核工程数据完整（default_project.xml 亲核：6 TRACK/15 PLUGIN/1 RACK）。

## 目标

1. **装配**：GUI 生产树实例化 ProjectRepository 并入 group（实例化点勘察定：legacy controller/dock 引导/主场景 _ready，最小侵入优先；勘察结论回执披露）。
2. **喂食**：内核工程状态到达点接 `repo.apply_kernel_project_state(reply)`——候选锚点 `app/legacy/vit_control_v_1.0.gd:394`（`_apply_kernel_project_state_to_legacy_scene(store.project_state)` 入口，虚拟/直接两路径共用）；transport-only 增量不得误触发全量重喂（apply_kernel_transport_only 语义保持）。
3. **后续变化面**：装载/卸载效果器等 rack 拓扑变化后的快照刷新到达链勘察（project_store/hub.project_loaded 既有面）——repo 快照须随工程状态变化更新（否则装后列表仍旧）；缺链则补触发并披露。
4. **fail 不再静默**：repo 缺位/查询异常 push_warning（一次性防刷屏）——装配正确后此路径不应触发，留痕用于诊断。
5. **手测闭环义务**：修复后用户手测重验（选择参数列表=轨道组+已装插件在场）——本卡验收含手测节确认（206 手测节第三轮剩余项随之解锁）。

## 探针义务（风险位①成文后新口径，连续三案 198/205/206 后入律）

- **装配面断言必入**：探针须断言**生产 GUI 装配形态**下 repo group 节点在场+工程状态到达后 `get_rack_nodes_by_track()` 非空（有 rack/plugins 的轨）+`get_volume_pan_plugin_item_id_for_track` 可解析——**禁止以探针自建 repo 替代生产装配验证**（自建实例仅限 repo 单元语义面）。
- 真栈腿禁注入+生产链断言（205 起口径）延续。
- 既有全家回归零回退；lane 目录刷新触发面（开/绑/rack 回执 :231/:276/:713）回归。

## 文件域

`app/legacy/vit_control_v_1.0.gd`（喂食点）+实例化点文件（勘察定，预计 app/legacy 或主场景引导）+`vit_dock/core/project_repository.gd`（仅当需接 hub/触发面）+`app/tracks/track_scene/track_envelope_lane_2d.gd`（仅 fail-loud 面）+探针文件。**喂食点若结构上必须动 project_store/agent bridge 层→上交扩域裁定**。越域即停。

## 停止条件

- 内核状态到达链结构与 :394 锚点假设不符（无单一入口/多入口竞态）→锚点上交；
- hub/快照刷新链已有 repo 驱动设计意图（发现既有方案）→上交锚点不重复造；
- 装配点引入循环依赖或初始化顺序冲突（repo 早于场景 ready 需求）→上交两案。
