# FE-RACK-K32-POLISH-1（K3.2）：K3.1 复测反馈四点+UX 短板收口

- 发卡：GLM 主管决策流 / 2026-10-19 晚窗（依据=用户 K3.1 复测反馈：①「位1/位2」标签问询（用户指示去除）②类型选择不生效③新建 send 缺确认按钮④旧宏/控件类型覆盖疑问；+本轮验收已登记两项 UX 短板）
- 派发确认：已确认（用户复测反馈即授权范围；④旧宏策略按主管推荐「彻底切新不迁移」落账——用户未反对推荐项，若用户另行裁定以新裁定为准）
- 验收负责人：GLM 主管决策流
- 池序 145；目标仓库=Godot 前端仓（D:\Godot\project\vit-daw-frontend）；P1 / 0.5 天 / 依赖=K3.1 已内部验收（1e33284）
- 模型分级：L2 / GLM 执行会话（前端线）

## 根因预钉（勿重复取证，②需运行时复现确认）

1. **①位号标签**：`vit_dock/scenes/viewport_frame.gd:52` `_slot_tag.text = "位%d" % (slot_index + 1)`——FE-GUI-B-FOURSLOT 台前槽位头位号（设计期产物）。`ViewportFrame.tscn:30` 默认文本「位1」。
2. **②类型选择不生效（疑点）**：静态链路（OptionButton→`_rack_control_pref`→`vit_sidebar_create_control(kind)`→payload）读码正确；头号嫌疑=**dock 视图重建/槽位重挂载后 `_rack_control_pref` 回退默认**——`graph_rack_dock_adapter.gd:76` 默认 `"knob"` 与 `:135` `restore_view_state` 默认 `"slider"` 两处不一致；次级嫌疑=画布控件渲染 kind 归一。须复现钉死后修。
3. **③选择器提交流**：`rack_send_target_selector.gd:277-289`「＋新建 FX 轨」两步=首击展开命名行→**仅回车（text_submitted）或再击**提交，无显式按钮。
4. **④旧宏不可见=两套数据源自然结果**（非缺陷）：旧宏=repository 宏文档系（旧链路写入）；新调取列表单源读 control_graph。

## 目标

1. **①移除位号**：槽位头默认隐藏「位N」文本（保留视图名+「✕拉下」）；tscn 默认文本清理。若 FE-GUI-B-FOURSLOT 线测试断言位号文本，同步更新（域内）。
2. **②复现+修复**：取证复现（下拉选型 X→创建控件卡→实际 payload kind 与画布渲染 kind）；修复使选择生效并在视图重建/重挂载后保持；统一 `:76`/`:135` 默认值口径。
3. **③确认按钮**：命名行旁加显式「创建并连接」按钮（回车/再击路径保留并存）；已有轨目标行点击即连行为不变。
4. **④语义澄清（轻量）**：「创建宏（混排宏卡）」与「创建控件卡（按所选类型单控件）」在 UI 上加一行说明或 tooltip，使用户明白十词表应用于创建控件卡；十词表全集可用性随②修复自然达成（不新做控件类型）。
5. **⑤UX 短板两项**：工具页「创建宏/创建控件卡」禁用态给原因提示（如「先在机架选中一条轨道」）；send 创建失败（内核拒绝/回滚/超时）给可见反馈（状态栏或轻提示），不静默。

## 决策落账：旧宏策略=彻底切新不迁移

新 UI 单源读 control_graph；repository 宏文档层保留兼容读（旧工程数据不丢、不删）；不做迁移/双源合并；如需找回旧宏，一次性导入工具另立卡再裁定。

## 文件域

前端仓：`vit_dock/views/graph_rack_dock_adapter.gd`、`vit_dock/scenes/viewport_frame.gd`（+ViewportFrame.tscn）、`app/rack/rack_send_target_selector.gd`、`app/rack/Vit_Graph_Rack.gd`（②如涉）、`tools/probe_rack_sendroute_macro.gd`（断言扩展）及相关测试。GH001 推送限制照旧（补丁/清单存主仓 run 目录）。越域即停。

## 验收标准

- 探针扩展全绿：②选择 slider→创建→`control_add_node` payload.kind=="slider"+重建/重挂载后 pref 保持；③命名行「创建并连接」按钮存在与触发断言；①位号文本不渲染断言。
- 前端仓既有 9 套回归零回退。
- 真栈：`dev_agent_smoke -SendRouteGuiSmoke` 复跑 exit 0（用栈前登记 `coord/resources/PC-RUNTIME-STACK.md`，被测 kernel 须含 K1=722A79D7；用户手测优先，遇占用让行）。
- **[等待用户] 手测复测**：四点逐项确认（位号消失/类型选择生效/按钮创建 send/宏语义清晰）。

## 停止条件

- ②复现失败（选择确实生效）→ 如实回报现象差异，不硬修；
- ③涉及选择器架构重构（超出加按钮粒度）→ 上交方案再定；
- FE-GUI-B-FOURSLOT 槽位头改动影响其测试契约且无法域内解决 → 上交。

- 领取：2026-10-19 19:31 / 主仓 origin/main=25518b61（前端基线 1e33284，两 worktree 开工前核对干净）/ owner=GLM-5.3 L2 PC 执行会话（前端线，ZCode）/ 分支 port/fe-sendroute-macro-gui-1（前端仓续用）/ worktree D:\Godot\project-vit-daw-frontend-sendmacro + D:\Vit_DAW_wt_fesendmacro_stack（构建/工件）/ 领取提交=coord/fek32-claim→main（hash 见 git log）
- 回执（2026-10-19 20:0x，执行侧自验完成待主管验收）：
  - **commit**：前端仓 **d195b3b**（port/fe-sendroute-macro-gui-1 本地，GH001 照旧；补丁存 `coord/runs/FE-RACK-K32-POLISH-1/k32_sendroute_gui_20261019_194757/patches/0001-fix-FE-RACK-K32-POLISH-1-card-145.patch`，随主仓 port/fe-sendroute-macro-stack@f39f595e 入库）。完整回执=coord/runs/FE-RACK-K32-POLISH-1/K32_RESULTS.md。
  - **②根因钉死（非预钉的 adapter 重建 pref 回退本身，更深一层）**：view_state 保存链整体断裂——`get_view_state()` 全仓无调用方+`capture_all_view_states()` 空 pass+dock context view_state 硬编码三键 ⇒ 任何 adapter 重建后 pref 必回退 knob 且 OptionButton 同步回退（用户感知「创建的仍是 knob」；「调取总落最新旋钮」=画布全是 knob 卡的伴生感知，focus 按 node_id 精确匹配无独立缺陷）。静态 payload/normalize/widget factory 链逐环核验无罪。**修复=域内补链**：viewport_frame 按 view_type 缓存视图状态（拆旧前存/同类型重载灌回）+默认口径统一常量 DEFAULT_RACK_CONTROL_KIND；dock 布局层保存链活化不在卡域（如需彻底持久化到布局另立卡）。
  - **探针断言**：178→**206 全绿 exit 0**（+28：payload.kind=slider/重建后 pref 保持+UI 同步/位号三断言/③按钮存在+触发+回车并存/⑤B 未确认回滚+auto_create 保留防误报+超时清+反馈/源码契约 5）。
  - **9 套回归零回退**：4 rack 测试 exit 0+plugin_search_popup 111/111+shrink a/b/c PASS。
  - **真栈 run**：**k32_sendroute_gui_20261019_194757** `dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke -KernelExe <build-feguik3 722A79D7 SHA 复核> -GodotProjectRoot <前端 worktree@d195b3b>` **exit 0**，探针 25/25 PASS+chat/confirm 腿过；占用登记 9b5a4b7a/释放复核净（六端口 tcp+udp=0；用户 Godot 7712 未动）。§8 LLM=0，一次过零重跑。
  - **端测边界声明**：不覆盖渲染视觉（位号像素消失/按钮/tooltip 悬停呈现）与真实指针交互——归 **[等待用户] 手测复测四点**（位号消失/类型选择生效含切走视图再切回/按钮创建 send/宏语义清晰）；手测入口=用户从 Godot 拉起前端（worktree D:/Godot/project-vit-daw-frontend-sendmacro@d195b3b），AGENTS §5 途径。
  - **git status**：前端 worktree 领取基线 1e33284 干净，本卡 d195b3b 后干净；stack worktree 领取时已有 default_project.xml 运行态改动+build-feguik3/（K3 遗留未动），本卡新增仅 coord/runs/FE-RACK-K32-POLISH-1/（已提交 f39f595e）+运行态改动。
- 验收：（裁定文件 / 验收 commit）
