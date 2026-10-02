# VITNOTE-V2-IMPL-C 验收工件留档（结论可回指原始工件）

- 日期：2026-10-02；工作仓：D:\Godot\project\vit-daw-frontend，分支 port/vitnote-v2-impl-c（自 port/vitnote-v2-impl-b@3ea8a0a442660b5f5c022139af570bf3a151b783 切出，实现 commit=4e0b862，已推 origin）
- Godot：D:\Godot\Godot_v4.6.1-stable_win64_console.exe（v4.6.1.stable.official.14d19694e，与 IMPL-A/IMPL-B 同二进制）

## 命令与退出码（Godot 仓根目录执行）

| 工件 | 命令 | 退出码 |
|---|---|---|
| probe_face_resolve_run1.txt | `"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_vitnote_face_resolve.gd` | 1（103 项 6 红——探针搭建缺陷：FakeRackAdapter 忘设节点名 GraphRackDockAdapter，供给器按场景根名枚举故测不到；非实现缺陷，首红留档） |
| probe_face_resolve_run2.txt | 同上（探针修正后） | 0，127/127 PASS |
| probe_face_resolve_run3_rerun.txt | 同上（确定性复跑） | 0，127/127 PASS |
| import_zero_script_error.txt | `... --headless --path . --import` | 0，SCRIPT/PARSE ERROR 计数=0（同时生成两新供给器 .uid） |

（原始输出为控制台重定向文本，沿 IMPL-B 先例以 .txt 落档，内容字节不变。）

## 探针结果（run2/run3 两轮一致，checks=127 failed=0）

- 静态契约 18 项：**四**供给器脚本四函数（identity/affordance/rects/resolve）齐备+参数个数；manager resolve_circle/_circle_hint_text 在位。
- IMPL-B 既有套件原样保留全绿（管线算术/噪声门/合并决胜/失败语义/生命周期/端到端接线）。
- IMPL-C 新增组（rack/library）：
  - 空数据降级形态：无 adapter/无 dock/无 repo（headless 沙盒）→ rack domain={track_id:"",track_name:"",entries:[]}；library domain={tab:"places",entries:[],total_items:0}——空清单=合法辖区（§6.0）。
  - 伪造数据路径：FakeRackAdapter（名=GraphRackDockAdapter+_viewport_id=vp_probe+选定轨 t_probe）+FakeRepo（get_rack_nodes 2 条规整形/get_track 鼓轨/get_track_groups 2 编组）→ identity face_id=="rack@vp_probe"、label=="机架·鼓轨"、rects==枚举矩形、resolve 全链 2 条（node_id/plugin_name/track_id/type/params 逐键断言+params 值透传）、resolve_circle 圈住即入面且 domain.entries=2（**面命中即整链入摘**——§6.2 诚实降级实证）。
  - FakeLibraryDock（组 vit_media_library+三 tab 变量+无 browser_tree 属性）→ plugins tab 3 条（name/manufacturer/category/identifier 逐键）、**60 条→50+truncated_items=10+total_items=60**（§6.3「…及 N 条截断」标记）、places tab Tree 缺失回退 _places_roots 根清单 2 条（kind=dir+path）、groups tab 2 编组（group_id/name/member_count）、未知 tab 规整回 places、dock 隐藏→rects []（关闭抽屉路径）、resolve_circle 圈住即入面。
  - 伪造节点套件尾显式 remove_child 离场（组注册对称）——③段真实场景组查询零污染（fake adapter detached 断言）。
- 两面场景烟测：legacy+dock 的 VitNoteLayer 均持**四**供给器子节点+组注册；resolve_circle 全视口形态级断言+真实几何路径跑通：
  - legacy faces=["control@main", "library@left_drawer", "timeline@Main_Control"]（无机架视图，rack 正确缺席=降级）；
  - dock faces=["control@main", "library@left_drawer", **"rack@vp_rack_main"**, "timeline@VitDockRoot"]——**真机架视图被真实枚举**（真 _viewport_id 进 face_id）；rack domain entries=0 track_id=（headless 无内核无选定轨=合法降级）；library domain tab=places entries=6（**真 browser_tree 可见行枚举**走通，非伪造路径）。
- 退出时 ObjectDB leaks 警告为探针环境产物（headless 场景 teardown，IMPL-A/IMPL-B 同象），退出码 0 为门槛判据。

## 与卡片的对位

- §6.2 机架供给器：face_id=rack@<viewport_id>（每视口一个绑当前选定轨——单静态供给器下以聚合形态落地：枚举全部可见机架视图矩形+resolve 锚定首个可见 adapter 的选定轨；adapter 无组注册且三只读文件零修改约束下无法加注册钩，注释如实声明）、label=「机架·<轨名>」；affordance observe+propose（装载/移除/写参=RiskConfirm 级 V1 §7 引用）；face_rects=graph_rack_dock_adapter 视图全局矩形；resolve=get_rack_nodes(选定轨) 全链清单（node_id/plugin 名/绑定轨/关键参数当前值）+time_window 无。**双诚实降级注释**（无逐节点矩形=面命中即整链入摘；逐节点占比=v2.x）。
- §6.3 资料库供给器：face_id=library@left_drawer；关闭返回 []；resolve 按当前 tab 报可见条目（Files=路径/Plugins=名称厂商类别/Groups=编组）；**上限 50 条+「…及 N 条截断」标记**（truncated_items 键，仅截断时出现）。诚实降级注释（Tree 无逐行全局矩形公开 API——条目级清单代逐行占比）。
- manager spawn 扩展两行（§6.0 注册处同构——四类面覆盖完成）；探针扩展并入既有探针（卡片目标 3「并入既有探针或新增组」的前一分支）。
- 域对象源实锚核对（停止条件未触发）：F11 `project_repository.get_rack_nodes`（:767 快照字典数组，规整后 id/item_id/name/type/params 保证在位）+adapter getter（get_selected_kernel_track_id :608 区）形态与设计一致；F12 left_library_dock 锚点（get_selected_library_item :76/get_selected_library_file_path :90/_available_plugins :34/_places_roots :25/_active_library_tab :33）全在位。
- 文件域核实：`git diff 3ea8a0a` 仅 6 路径=2 新建供给器 .gd+2 生成 .uid+manager+探针；**graph_rack_dock_adapter/left_library_dock/project_repository 零修改**；lane 域（app/tracks/track_scene/、app/timeline/、app/rack/）零改动。

## 未覆盖边界（如实申报，归用户手测 §10.2 第 3 步[跨面完整版]——挂 V2 手测场）

- 真实抽屉开合/切 tab/机架切换选定轨的矩形与清单**时效**（headless 无法驱动真交互；供给器禁缓存、逐次实时查询，但真实动态行为未目检）；
- 渲染视觉/IME/输入链（IMPL-A/B 边界不变）；
- 真实内核在场时 rack resolve 的真实插件链内容（headless 无内核，entries=0 降级路径已断言；真数据路径归手测）；
- 面板头部多面摘要呈现归 IMPL-D（vit_note_panel.gd 不在本卡文件域，零改动）。
