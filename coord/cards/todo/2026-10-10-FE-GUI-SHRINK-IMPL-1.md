# FE-GUI-SHRINK-IMPL-1：PC 端四位台前调度布局改造（按 HTML 设计稿实施）

- 发卡：GUI 设计会话（GLM）/ 2026-10-10
- 派发确认：**已确认**（用户 2026-10-10 授权立项：「按照 html 设计稿把现有前端修改好」+设计稿存档为未来依据）
- 验收负责人：GLM 主管决策流
- 目标仓库：**D:\Godot\project\vit-daw-frontend**（仓库外；本卡勘察期只读——**领取前须获写授权**，并在 coord/resources/PC-RUNTIME-STACK.md 登记真栈占用）
- 优先级 / 预估 / 依赖：P1 / Phase A 1-2d，Phase B 2-3d，Phase C 1-2d / 无硬依赖（与 FE-RACK-CTX-PLUGIN-LOAD-1 文件域部分相邻，领取时核对不相交）
- 模型分级：L2（涉及交互引擎重写与机制退役，下游失败面广）

## 设计契约（权威输入，实施以此为准）

- 规格文档：`coord/runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/实施卡规格-第一轮.md`（含布局表/交互清单/退役清单/红线/probe 断言/手测清单/分期）
- 交互稿与终图：`coord/runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/prototypes/`（**V11-PC端.html=本卡蓝本**；V5/V7=移动端设计基线，本卡不实施）
- 该目录已定为**设计基线存档**（README-BASELINE.md），是后续前端迭代与未来移动端设计的依据，实施不得改动其中文件

## 目标

PC 端主工作区由「自由分割树多视口」改为「**四位台前自动铺排**」（1=全屏/2=左右/3=左整+右上下/4=田字；默认=音轨+机架；满员挤出 LRU；Master 置顶第一轨），配套 F2 台前调度面板（左列陈列+前台镜像小卡）、视图抽屉坞（拖拽装载/点击浮窗/平时收起）、浮卡+真窗口双档多开；agent 对话维持外置窗现状。分期交付：**Phase A**=固定双位+退役清单+Master 置顶（删除性工程，最小可交付）→ **Phase B**=四位引擎+LRU+F2 面板+抽屉坞+指针式拖拽 → **Phase C**=浮卡/真窗口双档。

## 文件域（前端仓）

- `vit_dock/scenes/vit_dock_root.gd`、`vit_dock/core/viewport_split_tree_builder.gd`（退役）、`workspace_layout_manager.gd` / `workspace_preset_repository.gd`（降级为数据层，不驱动 UI）
- `vit_dock/scenes/AppHeaderShell.tscn/app_header_shell.gd`（收敛为菜单栏；走带栏 Top_Transport_Bar **仅加按钮位**）
- `vit_dock/scenes/viewport_title_bar.*`（视口标题栏退役）、`views/*_dock_adapter.gd`（换宿主，内容不动）
- **红线零触碰**：`app/kernel/autoloads/telemetry_manager.gd:1963`、`app/kernel/clients/mix_client.gd:39-42/:65/:71`、`app/shell/LLM_Chat_Controller.gd` 数据面、`app/kernel/autoloads/vsp_asset_adapter.gd`、`app/browser/left_library_dock.gd`、机架/轨行右键菜单、插件拖拽双落点、走带/电平数据链

## 验收标准

1. 规格文档中 **headless probe 断言 8 条**全过（默认两位/田字/独占全宽/LRU 挤出/拖拽注入装载/双击上台/Master 首轨/mixboard id 红线回归），按仓库 ps1 冒测体系扩展（复用 dev_agent_smoke.ps1 模式，脚本 exit 0）
2. 真实三件套烟测（内核+Godot 前端+agent）exit 0（AGENTS.md §5 门槛）
3. 手测清单 7 条交付用户（手测入口=用户从 Godot 拉起）
4. 退役清单落地核验：预设 tabs/分割按钮/视口切换菜单/New Window/右缘坞不在 UI 面

## 停止条件

- 发现前端仓结构性异常（如分割树与遥测链隐藏耦合）→ 停止并上交证据
- 红线面任何回归（mixboard id 形态/波形包络物化/聊天观察卡）→ 立即停卡上报
- 连续两次同断言失败→按 §8 补锚点或拆卡，不得原样重跑

## 并行与资源

真栈占用（烟测腿）按 PC-RUNTIME-STACK 登记，与其他在池烟测卡串行；纯代码阶段可并行。

- 领取：（时间 / 前端仓 HEAD / owner / **写授权凭证**）
- 回执：（probe 脚本路径 + run ID + 退出码）
- 验收：（裁定文件）
