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

- 领取：（2026-10-28 18:43 +0800 / origin/main=1edcf99fb139b8ed76659f6ddf561f1bdc8593 / ZCode GLM-5.3 PC 执行会话（ZCode，207 卡会话）/ 分支=port/fe-lane-catalog-repo-wire-1（前端仓 worktree D:/Godot/project/vit-daw-frontend-lanecatalog1，领取基线 main=3b5df90 亲核=origin/main）/ 协调 worktree D:/Vit_DAW_wt_207coord（coord/207-claim）/ 领取提交=本提交。领取核对：fetch 后卡仍在 todo（1edcf99f）+派发确认=用户 2026-10-28 晚显式指令领取+依赖=无+doing 仅 GEN-SUNO-WEB-DESIGN-1（Mac 设计卡，域不相交）；真栈记录最新释放=206@2026-10-28 13:2x 亲读，当前无占用登记（启栈前按 §2.2 另行登记+≤60s 即时复核+错峰手测节））

- 回执：（**done 2026-10-28 19:2x +0800，执行侧自验完成，交主管验收**）实现 commit=**前端 cecff4f**（port/fe-lane-catalog-repo-wire-1，5 文件 +629/-1：vit_dock_root.gd 喂食守卫+track_envelope_lane_2d.gd fail-loud/hub 重读+探针两件）；工件分支=**port/fe-lane-catalog-repo-wire-1-stack@d052b187**（run=coord/runs/FE-LANE-CATALOG-REPO-WIRE-1/lanecatalog_20261028_1，RESULTS.md 含七项披露）。**勘察结论（发卡根因修正，停止条件①②证据上交）**：①用户手测 GUI=dock 主场景 VitDockRoot 非legacy——用户手测日志 godot.log 2026-10-28 18:29 亲读（`_ready(vit_dock_root.gd:234)`+`[rack.drop] kernel_track_id=1007 Pro-Q 3`）；三份 coord 在卷先证默认主场景=dock。②「ProjectRepository 生产零实例化」不成立于用户形态——dock root `_ready→ProjectDataManager→ProjectRepository.new()+add_child` 装配链在场（class_name 引用故发卡侧路径 grep 不可见），探针 P1b/c 亲证 repo/hub 组节点=dock 子树内实件；卡面①（装配）结论=已在场零改动。③真实断供双点：VSP 紧凑快照白名单剥轨行 plugins（内核 VspKernelReference normaliseTrackArray，161 卡注释先证）→repo plugins_by_track 恒空→未挂轨 vp id 恒空=目录全空/无默认音量；rack 拓扑命令回执后 repo 已重喂（dock 既有回拉链，本卡零改动）而 lane 目录不跟随（receipt 同步读旧快照+重喂后无重读触发）——即卡面②③的真实形态。④卡面:394 legacy 锚点非用户链路，app/legacy/vit_control_v_1.0.gd **零触碰**；legacy 面（VIT_USE_LEGACY_MAIN=1 显式回退形态）repo 缺位现状维持+fail-loud warning 留痕，是否另行接线交主管裁定。**修复**：vit_dock_root `_vsp_snapshot_tracks_have_plugins` 守卫（缺列回落既有 legacy get_project_state 全量喂食；transport-only 面零改动）+lane 懒挂 hub.project_loaded（展开态目录+●N 重拉=变化面收口）+fail-loud 一次性 push_warning+`get_repo_missing_warn_count()`。**探针（§5.1 装配面口径兑现）**：stack_probe_lane_catalog_repo_wire 零 repo 自建（可 grep）+VitDockRoot.tscn 实件入场+组节点 dock 子树亲证+DIAG/正式双模式；probe_lane_catalog_fail_loud（-s 离线）。**验收面**：真栈 run=lanecatalog_20261028_1——DIAG 复现轮 **23/23 缺陷签名全绿 exit 0**（用户三症状逐一复现）+正式轮 **24/24×2 连续两轮 exit 0**；离线 fail_loud **18/18**（fail-loud 一次性+transport-only 单元语义=卡面增量不误触发面+源标记）；全家离线 24 件零回退（popup 探针首轮缓存态红：基线 worktree 111/111 绿+基线+diff 二分 111/111 绿+清缓存重导后 111/111 绿=代码面无关留证）；栈登记-释放对=00eb75c8 登记/19:18 释放（三纪律+隔离三面亲证见 RESULTS.md §六）。**端测边界声明**：本腿=Godot headless 生产树+agent/内核 IPC 面断言，不含 webui 渲染面与用户旅程面；卡面⑤手测闭环（用户手测重验选择参数列表=轨道组+已装插件在场）待手测节执行。**披露**：vsphub=ps1 最小腿不启 hub，本腿按用户栈形态手启（主仓二进制只读复用 173 同源）——ps1 是否内置 hub 腿交主管参考；喂食守卫触发期每 refresh 双调用（VSP 取后弃用+legacy 全量），内核白名单扩列后自动回单次。

- 验收：（**主管整卡 pass 2026-10-28 晚，GLM 主管决策流；含根因修正裁定**）——①**发卡根因修正接受**：两处前提被运行时证据推翻（「生产零实例化」=发卡侧 grep 域错——限定 app/ 漏 vit_dock/ 目录，教训记档：装配面勘察须全仓路径+运行时日志证据优先；「:394 锚点=用户链路」不成立=用户 GUI=dock 主场景）。执行侧 godot.log 铁证+ProjectDataManager 装配链+内核白名单亲证（主管复核 VspKernelReference.cpp:603-700 零 plugins）全部属实——「证据推翻卡面前提→上交→修正」路径执行正确。②**修复 diff 亲读**：守卫（:1097 回落体亲证=false→既有 legacy IPC refresh_project_state 全量喂食，VSP-disabled 同路径先例=最小面）+lane hub.project_loaded 懒挂重读（展开态目录+●N；收起态开时刷新）+fail-loud 一次性+计数读回面——三面全落卡面目标。③**探针 SUMMARY 三份亲读**：DIAG r1 23 检 0 败（用户三症状逐一复现=红面实证）+FIXED r1/r2 各 24 检 0 败；§5.1 装配面口径兑现亲证（VitDockRoot.tscn 实件+零 repo 自建+组节点 dock 子树）。④栈登记-释放对（19:06→19:18 三 pid 逐一亲核）+隔离三面+vsphub 手启披露核对。⑤popup 缓存态事故定性接受（基线/二分/清缓存三重留证，非代码面）。**两项交裁定处置**：ⓐlegacy 面 repo 接线=**维持现状+fail-loud 留痕，不开卡**（legacy=显式 env 回退形态非用户生产路径，未来切该形态手测再折）；ⓑvsphub ps1 腿缺位+守卫期双调用=**记观察项不开卡**（冒测脚本保真度候选+内核白名单扩列后自动回单次）。合入：前端 main=4891bae（merge）；工件=主仓 merge（lanecatalog_20261028_1 全量）。**手测节第三轮重验解锁**（卡面目标⑤=最后一环：用户手测选择参数列表=轨道组+已装插件在场，随 206 剩余项一并验）。
