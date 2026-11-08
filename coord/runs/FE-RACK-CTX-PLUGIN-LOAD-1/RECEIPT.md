# FE-RACK-CTX-PLUGIN-LOAD-1 执行回执（工件目录）

- 执行 owner：GLM-5.3-Flash · PC · ZCode flash 执行会话
- 领取：2026-10-10 18:14，主仓基线 3c275461，领取提交 7dff62a0（主仓 main，仅 coord/ 卡片变更）
- 前端仓基线：vit-daw-frontend `ecc722d`（port/vitnote-region-time-1 tip，卡面锚点行号核实基线）
- 实现分支：`port/fe-rack-ctx-load` @ **924318dea1f55f5c9591d573b174a1a2307398e6**（已推前端仓 origin，未触碰任何 main）
- worktree：D:/Godot/project/vit-daw-frontend-fe-rack-ctx（独立 worktree，未踩踏 vitnote 流主工作树）

## 变更面（前端仓，文件域内）

- 新增 `app/rack/rack_plugin_search_popup.gd`（+ `.uid`）：检索式插件列表弹窗组件
- 修改 `app/rack/Vit_Graph_Rack.gd`：+91/-1，8 处局部钩点（preload 常量/两成员变量/_ready 构建一行/右键关闭一行/_rack_finish_right_pan 空分支/空区右键传本地坐标一行/两处左键关闭一行/4.1 节新函数块）
- 新增 `tools/probe_rack_plugin_search_popup.gd`：headless 探针
- 越域核对：未触碰 left_library_dock.gd（数据源只读）、未触碰装载链本体（rack_client/rack_ipc_payload_rules/layout rules 全只读）、未触碰 kernel/agent/主仓代码

## 设计段三件（入回执）

1. **右键热区**：机架面板空白区（空链泳道区+节点间隙，即未命中任何 GraphNode 的区域）右键「按下→原位松开」（位移 < RACK_VIEWPORT_RIGHT_PAN_THRESHOLD=5px）→ 打开检索弹窗；节点右键=既有节点菜单（零变化）；右键拖动=平移视口（零变化）；左键他处/选中/Esc 关闭弹窗；右键重按=关旧开新。
2. **列表形态**：嵌入式 PanelContainer（同 `_context_menu_panel` 模式，z_index 4096，宽 320）= 顶部检索 LineEdit（自动焦点、可清空）+ ItemList 单列行内多字段（行文本「名称 — 厂商 · 类别」；tooltip=identifier+物理路径——id/名/厂商三要素全在位；选中高亮为 ItemList 内建）+ 底部目标轨页脚（「装载到轨：<名>」/「未选定轨道（装载不会执行）」）。过滤=空格分词 AND，字段 name/descriptive_name/manufacturer/category/identifier/format 不区分大小写（与资料库 `_plugin_matches_query` 字段集对齐）；排序=名称首词前缀命中 > 子串命中 > 自然名称序（卡面「前缀+子串过滤」）。键盘 ↑↓ 换选中、回车装载（text_submitted 与 item_activated 双路）、Esc 关闭。空结果=禁用行「无匹配插件」；数据源空=「插件列表加载中…（若长时为空，请确认 agent 在线后重开）」并触发资料库既有公开 `refresh_plugin_library()` 同源刷新（异步，重开弹窗生效）。
3. **装载落点**：视觉位置=右键点击处（弹窗锚点本地坐标 +50,+50 偏移，与节点菜单「镜像克隆」落点偏移先例一致），经 `_drop_data` 既有 zone 推断+网格吸附；链序=既有 `_ipc_rack_add_node`→`add_node_command`（内核追加语义，与拖放同链同序）。选中即装载并关闭弹窗。

## 停止条件裁定（卡面）

**不触发**。既有装载链 `_drop_data(at_position, data)` 契约=位置+载荷字典，无拖放源信息耦合；且已有两处非拖放触发先例：`auto_spawn_node`（资料库双击装载，:1634）与节点菜单「镜像克隆」（:4696 直接调 `_drop_data`）。本卡装载零新写：仅新增触发入口与数据整形（`plugin_drop_payload` 与资料库拖拽载荷同形，已断言）。

## probe 断言结果（tools/probe_rack_plugin_search_popup.gd，退出码 0=全绿）

- **76 钉全 PASS，连续两次运行 PASS**（probe_run2_*.log 存档；首跑全绿后修 1 处探针自身断言瑕疵——节点菜单未关的既有行为被误假设，产品代码无改动）
- ① 契约静态 15 钉：弹窗 7 方法签名+plugin_selected 信号+机架 6 钩点签名
- ② 纯函数 24 钉：filter_plugins（空查询自然序/前缀层/子串层/分词 AND/大小写/format/厂商/非 dict 剔除/零命中）+ plugin_drop_payload（与资料库拖拽载荷同形十字段+path/name 回退）
- ③ 场景命令面 37 钉：空白右键弹出（条目数/页脚轨名/不开节点菜单）→ 检索过滤（收窄/首行排序/零命中空态行）→ 选中确认=**命令面断言**（ProbeRack 动态子类重载 `_vit_rack_client` 缝记命令字典：cmd=rack_add_node、track_id=t_probe、plugin_identifier/path、auto_connect=true、zone 非空、坐标有限值，弹窗随选关闭）+ Esc 关闭 + 键盘选中钳制 + **回归钉**（节点右键=节点菜单零变化/右键拖动不开任何菜单/_can_drop_data 拖放线零变化 4 例/auto_spawn 双击装载零变化/空清单惰性同源刷新触发一次且在途请求不重触发）
- 回归（同 worktree、含本卡改动的场景全量加载）：`probe_vitnote_face_resolve` PASS、`probe_vitnote_container` PASS（日志存档本目录）

## 环境勘测记录（后续会话可复用）

- 全新 worktree 首次跑探针需先 `Godot_console.exe --headless --path . --import` 建 `.godot` 全局类缓存，否则 `ProjectEventHub` 等 class_name 解析失败
- `-s` 脚本模式不注册 autoload 命名全局：`Vit_Graph_Rack.gd` 引用 `VitDebugFlags` 标识符（7 处）编译失败——探针以 `Engine.register_singleton("VitDebugFlags", fake)` 注入假单例（须在 rack 脚本任何加载之前）；探针机架子类必须运行期动态生成（静态内类继承会让探针自身加载即触发 rack 编译）
- Godot 4.6 `ItemList.ensure_current_is_visible()` 无参；本项目 debug 设置将 Variant 推断警告视为错误

## 渲染面边界声明（卡面验收标准第 4 条）

Godot 前端无 webui 式浏览器断言面，**headless probe 即本卡端侧门**：探针断言到命令构造面（rack_add_node 命令字典），不覆盖真实内核落链、渲染视觉、真实指针/焦点/IME 行为——归用户手测。

## 手测清单（装载为用户可感交互；按用户裁定走 Godot 前端入口，[等待用户排期]）

1. 用户从 Godot 拉起 DAW 前端（D:\Godot\project\vit-daw-frontend），从前端启动页进入目标工程（本卡无需环境变量）。
2. 选中一条轨，右栏机架面板**空白处**（空链区或节点间隙）右键按下→原位松开：应弹出「检索插件」弹窗，底部页脚显示「装载到轨：<轨名>」。
3. 键入检索词（插件名前缀如 `pro`、厂商名如 `fabfilter`、类别名）：列表实时过滤且前缀命中排前；↑↓ 换选中高亮跟随；Esc 关闭；左键点弹窗外关闭。
4. 选中条目按回车（或双击条目）：弹窗关闭，机架对应分区出现新节点并自动连线，行为与左栏拖拽装载一致。
5. 回归三件：左栏插件拖到机架装载行为与改动前一致；节点上右键仍是节点菜单（销毁/克隆/审查）；右键拖动=平移视口。
6. 空态：检索无匹配显示「无匹配插件」；若资料库插件列表尚未拉到（agent 未在线/刚启动），弹窗显示加载中文案，待资料库 Plugins 页有数据后重开弹窗可见全量列表。
