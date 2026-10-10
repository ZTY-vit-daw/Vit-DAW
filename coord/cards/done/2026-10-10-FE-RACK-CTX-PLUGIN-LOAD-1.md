# FE-RACK-CTX-PLUGIN-LOAD-1：机架面板右键装载插件——检索式插件列表弹窗（Cubase/Pro Tools 习惯，MAX/MSP 组件式）

- 发卡：GLM 主管决策侧 / 2026-10-10（用户交互提案：初始界面三栏中插件拖拽需横跨左右两栏，右键装载补齐机架本地入口）
- 派发确认：已确认（用户 2026-10-10 提案并要求考虑；发卡含设计先行段）
- 验收负责人：GLM 主管决策流
- 池序 46；目标仓库=**D:\Godot\project\vit-daw-frontend**（仓库外 Godot 前端）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无
- 模型分级：L1 / flash 可接（纯 GDScript 前端交互；B 系列前端卡先例）

## 现状与动机（用户 2026-10-10）

三栏布局（左=资料库/中=音轨/右=机架）中音频素材拖拽路径短，但**插件装载必须从左栏一路拖到右栏**。传统 DAW（Cubase/Pro Tools）的机架/插槽右键装载 + MAX/MSP 式组件检索列表是成熟交互习惯。

## 已核实事实（2026-10-10 实读）

1. 插件列表数据源已存在：`app/browser/left_library_dock.gd:51` `_available_plugins`（经 plugin list 请求面拉取，:227/:262/:625）——弹窗复用同源，不新做扫描。
2. 机架面板：`app/rack/Vit_Graph_Rack.gd`（右键上下文入口在此挂）；adapter 选定轨方法 `get_selected_kernel_track_id()`（vit_control_v_1.0.gd:1051）。
3. 既有装载链：拖放装载线（vit_face_supplier_rack.gd:52 "拖入装载走既有拖放线"）——右键装载**必须走同一条装载命令链**，只是新增触发入口。

## 目标（设计先行段 → 实现段）

1. **设计段**（简短，交互形态三件入回执）：右键热区定义（空链区域/节点间隙/任意区域，倾向空链+间隙）、列表形态（插件 id/名/厂商列+**检索框**（前缀+子串过滤）+键盘导航+回车装载）、装载落点（右键处的链尾/插入位）。
2. **实现段**：`Vit_Graph_Rack.gd` 右键上下文菜单 + 新弹窗组件（检索过滤/高亮当前选中/空结果态）+ 选中后调用既有装载链；资料库数据源只读复用；不碰 kernel/agent/装载链本体。

## 文件域

`app/rack/Vit_Graph_Rack.gd` + 新增弹窗组件（`app/rack/` 下）+ 资料库数据源**只读**引用；越域即停。

## 验收标准

- Godot headless probe 断言（tools/probe_*.gd 先例）：右键弹出/检索过滤命中/选中发起装载命令（命令面断言，非目检）。
- 单视口回归：拖放装载线行为零变化。
- 手测清单条目：装载为用户可感交互，手测按用户裁定走 Godot 前端入口（拉起→右键→检索→装载→目检链上节点出现），清单入回执交用户排期。
- 渲染面边界声明：Godot 前端无 webui 式浏览器断言面，headless probe 即端侧门。

## 停止条件

装载命令链对非拖放触发形态有隐藏耦合（如装载必须携带拖放源信息）→ 锚点+形态上交，不私改装载链。

## 并行与资源

不占真栈（headless probe + 用户手测自定窗口）；与 vitnote 卡/后端卡文件域不相交。

- 领取：2026-10-10 18:14 / 主仓 origin/main=3c275461e49cbfd39303225269cfdc818c5438e1（本地一致）/ owner=GLM-5.3-Flash · PC · ZCode flash 执行会话（用户 2026-10-10 口令指派领取本卡）/ 前端仓分支=port/fe-rack-ctx-load，基线=vit-daw-frontend ecc722d12e8d7818fbe8bd77019f2b15e37ba1d7（port/vitnote-region-time-1 tip，已推远端、tracked 干净；卡面锚点行号在该基线核实）/ 前端 worktree=D:/Godot/project/vit-daw-frontend-fe-rack-ctx（独立 worktree，不踩踏 vitnote 流主工作树）/ 领取提交=7dff62a0fcf9e94a6cb60e6c8ad85605297a9999（仅 coord/ 卡片变更）
- 回执：实现 commit=vit-daw-frontend **924318dea1f55f5c9591d573b174a1a2307398e6**（port/fe-rack-ctx-load，已推前端仓 origin；任何 main 未触碰）/ 设计三件=①右键热区=机架空白区（空链泳道+节点间隙）右键原位松开（<5px 阈值），节点右键菜单与右键拖平移零变化 ②列表=嵌入式面板：检索框（空格分词 AND，name/厂商/类别/ID/format 字段，前缀命中>子串>自然序）+ItemList（名—厂商·类别，tooltip=identifier+路径）+目标轨页脚+空态禁用行+加载态 ③落点=右键点击处+50,50（镜像克隆先例偏移）经 `_drop_data` 既有 zone 推断/吸附/IPC 链 / 停止条件=不触发（`_drop_data` 契约无拖放源耦合，已有双击装载与镜像克隆两处非拖放先例）/ **probe=76 钉全 PASS×2 连跑（退出码 0）**，含命令面断言（ProbeRack 动态子类重载 `_vit_rack_client` 缝记 rack_add_node 命令字典：track_id/identifier/path/auto_connect/zone/有限坐标）+回归钉（节点菜单零变化/右键拖动不开/`_can_drop_data` 零变化/auto_spawn 零变化/空清单惰性同源刷新）+vitnote 两面场景探针回归 PASS / 渲染面边界声明=Godot 前端无浏览器断言面，headless probe 即端侧门，真实指针/焦点/视觉归手测 / 手测清单=6 条入工件 RECEIPT.md，[等待用户:手测排期] / 工件=D:/Vit_DAW/coord/runs/FE-RACK-CTX-PLUGIN-LOAD-1/（RECEIPT.md+probe_run2 log+vitnote 回归 log×2）/ 环境注记=新 worktree 需先 `--import` 建缓存；`-s` 模式 autoload 命名全局缺失，探针注入假 VitDebugFlags 单例+运行期动态子类（详见 RECEIPT.md 环境勘测记录）
- 验收：**pass（2026-10-10 晚窗主管）**——[rulings/2026-10-10-EVENING-BATCH-rulings.md](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) §4；前端仓 924318d 合入活线 port/vitnote-region-time-1 并推 origin。探针**独立复跑 PASS 76/76 exit 0**；装载链零新写亲验（_drop_data 契约+两处非拖放先例）。手测清单 6 条[等待用户排期]；前端 main 落后活线 13+ 笔登记为卫生项待裁。
