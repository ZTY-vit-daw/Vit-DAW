# VITNOTE-V2-IMPL-C：机架+资料库供给器——四类面覆盖完成（V2 五卡第四张，P2）

- 池序 30；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-v2-impl-b@3ea8a0a 切出——已验收 HEAD）**；来源=[VITNOTE_V2_INTERACTION_DESIGN.md](../../../docs/VITNOTE_V2_INTERACTION_DESIGN.md) §6.2（机架）/§6.3（资料库）+§6.5 接入指南
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / V2-IMPL-B 已验收（注册处+管线在位）；串行链尾接 IMPL-D
- 模型分级：L1 / GLM 或 flash 可接（契约先例已双份在仓，照葫芦画瓢+两域对象源锚定）
- 已核实事实（设计 §2 F9/F11/F12 + IMPL-B 落地基线，勿重勘）：
  1. IMPL-B 交付先例：`vit_face_supplier_timeline.gd`/`vit_face_supplier_control.gd`（契约四函数+_enter_tree/_exit_tree 对称注册+缺源=null 如实降级）；manager 已 spawn 两供给器——本卡同构再加两枚。
  2. 机架域对象源（F11）：`project_repository.get_rack_nodes(track_id)`（project_repository.gd:767，快照字典数组）；适配器 getter `get_all_project_plugins_for_scene_track`/`get_selected_plugin_*`（graph_rack_dock_adapter.gd:608-642）。
  3. 资料库域对象源（F12）：left_library_dock.gd `get_selected_library_item`（:76）/`get_selected_library_file_path`（:90）/`_available_plugins`（:34）/`_places_roots`（:25）；左抽屉=MediaPoolAdapter（Files/Plugins/Groups 三 tab 的 Tree）。
  4. dock 工作区结构（F9）：无 SubViewport 包裹，GUI 与 `_input` 同在主窗口传播域——供给器矩形即全局矩形。
- 目标：
  1. **机架供给器**（§6.2）：face_id=`rack@<viewport_id>`（每视口一个绑当前选定轨，label=「机架·<轨名>」）；affordance observe（链上节点/参数/连线只读）+propose（装载/移除/写参=RiskConfirm 级，V1 §7 引用）；face_rects=graph_rack_dock_adapter 视图全局矩形；resolve=entries=`get_rack_nodes(选定轨)` 全链清单（node_id/plugin 名/绑定轨/关键参数当前值）。**诚实降级注释**：无逐节点命中矩形，面命中即整链入摘（域引用完整、空间精度粗）——逐节点占比=v2.x。
  2. **资料库供给器**（§6.3）：face_id=`library@left_drawer`（关闭返回 []）；resolve 按当前 tab 报**可见条目**清单（Files=路径/Plugins=`_available_plugins` 名称厂商类别/Groups=编组），上限 50 条+「…及 N 条截断」标记。**诚实降级注释**：Tree 无逐行全局矩形公开 API，以条目级清单代替逐行占比。
  3. manager spawn 扩展+headless 探针扩展：两面新增契约静态断言+resolve 形态断言（伪造/空数据路径）并入既有探针或新增组；`--headless -s` 跑通；**原始输出落 coord/runs/VITNOTE-V2-IMPL-C/ 留档**。
  4. 手测覆盖边界如实申报：真实抽屉开合/机架切换的矩形时效归手测（§10.2 第 3 步跨面完整版挂 V2 手测场）。
- 文件域：新建 `app/tracks/vitnote/vit_face_supplier_rack.gd`+`vit_face_supplier_library.gd`（+.uid）+`vit_note_manager.gd`（spawn 两行）+tools/ 探针——预计 ≤5 文件；**graph_rack_dock_adapter/left_library_dock/project_repository 只读调用零修改**（越界即停）。
- 约束：新分支 `port/vitnote-v2-impl-c` 自 port/vitnote-v2-impl-b@3ea8a0a 切出；不动 main；运行时目录勿动；无自动化测试基建如实申报。
- 验收标准：headless 探针全绿 EXIT=0（含两面新增组）+`--import` 零 SCRIPT ERROR+探针输出落 runs/+两供给器 resolve 形态与设计 §6.2/§6.3 逐字段对上（含截断标记/空 tab/关闭抽屉空矩形路径）+三只读文件零改动核实+用户手测 §10.2 第 3 步（跨面完整版）复验（挂 V2 手测场）。
- 停止条件：域对象源实锚发现 F11/F12 形态与设计不符（如 getter 缺失/形态变更）→ 带锚点上交，不擅自改三只读文件。
- 领取：2026-10-02 / Godot 仓基线 port/vitnote-v2-impl-b@3ea8a0a442660b5f5c022139af570bf3a151b783（领取时 HEAD=3ea8a0a，工作树仅未跟踪运行时目录、无未提交代码改动）/ 分支 port/vitnote-v2-impl-c
- 回执（done，2026-10-02）：
  - **commit**：Godot 仓 port/vitnote-v2-impl-c @ 4e0b862（自 impl-b@3ea8a0a 切出，已推 origin）；diff=6 路径 685+/9-：新建 vit_face_supplier_rack.gd(192 行)+vit_face_supplier_library.gd(208 行)+2 生成 .uid+vit_note_manager.gd(+14：spawn 扩至四供给器)+tools/probe_vitnote_face_resolve.gd(+278：IMPL-C 新增组+③段四面断言+三伪造类）。
  - **探针输出留档**：coord/runs/VITNOTE-V2-IMPL-C/（README+run1 红+run2/run3 全绿+import 零错共 5 件）。run1 103 项 6 红=探针搭建缺陷（FakeRackAdapter 忘设节点名，非实现缺陷）；run2/run3 两轮 127/127 PASS EXIT=0；--import EXIT=0 SCRIPT/PARSE ERROR=0。
  - **两面 resolve 形态与 §6.2/§6.3 对上**（探针逐键断言）：rack=track_id/track_name/entries[node_id,plugin_name,track_id,type,params]+无 time_window+面命中即整链入摘实证；library=tab/entries/total_items+截断标记（60→50+truncated_items=10）+空 tab/关闭抽屉空矩形/Tree 缺失回退根清单/未知 tab 规整路径。真路径证据：dock 场景全视口解析出 4 面（含真 rack@vp_rack_main——真 _viewport_id）；library 真 browser_tree 枚举 6 条；legacy 无机架视图 rack 正确缺席。
  - **三只读文件零修改+lane 域零改动**：git diff 3ea8a0a 核实（graph_rack_dock_adapter/left_library_dock/project_repository 空 diff；track_scene/timeline/rack 空改动）。域对象源实锚 F11/F12 全部在位（停止条件未触发）。
  - **运行栈声明**：本卡验收面=headless 探针+--import（沿 IMPL-B 先例），真实运行栈本机**未起**（无占用、无需拆解移交）；渲染面/用户旅程门槛的端测覆盖边界=未覆盖，归手测 §10.2 第 3 步[跨面完整版]挂 V2 手测场（真实抽屉开合/机架切换时效+真实内核下 rack 链内容）。
  - 自验清单：静态契约 18 项/沙盒 89 项/两面烟测各 10 项全绿；诚实降级注释双供给器在位；50 条截断=MAX_ENTRIES 常量+truncated_items 键仅截断时出现。
- 验收：**pass（2026-10-02 决策会话；跨面目检挂 V2 手测场）**——[rulings/2026-10-02-VITNOTE-V2-IMPL-C-pass.md](../../rulings/2026-10-02-VITNOTE-V2-IMPL-C-pass.md)；两供给器 diff 亲核（聚合形态偏离申报采信+双诚实降级在位）+三只读文件零改动核实+我方复跑（headless 探针 127/127 EXIT=0 两轮+import 零错误自跑）+红绿链工件可回指；V2-IMPL-D 依赖解锁
