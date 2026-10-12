# FE-GUI-RACK-LIST-1：机架右键装载列表改造（品牌/类型分组→检索阅览）

- 发卡：GLM 主管决策侧 / 2026-10-12 晚窗（依据=用户手测 A 第 4 条反馈原话："右键打开的这个列表太差了，完全摊开罗列的，一定要恢复到像左侧资料库那样的带有品牌和类型标签的状态，其实最好是一种检索阅览列表的状态，但是你可以先做，我们再看"）
- 派发确认：已确认（用户明确"先做"授权 v1）
- 验收负责人：GLM 主管决策流
- 池序 103；目标仓库=**前端仓（Godot DAW 前端工程）**
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无
- 模型分级：L1 / GLM 前端执行会话（PC）
- 前置先例：FE-RACK-CTX-PLUGIN-LOAD-1（右键装载通道本体，done）——本卡只改列表呈现，不动装载链路

## 目标

1. **v1（必做）**：机架右键插件列表从完全摊平罗列改为**分组呈现**——与左侧资料库同款形态：按厂商（品牌）与类型（category）两级标签分组；组头可折叠展开；列表项保持现有装载动作与确认流不变。
2. **v2（随 v1 附带，做不动可单独回执申报）**：列表头加**检索框**（按插件名/厂商/类型过滤，空输入=全组）。
3. 数据源对齐：分组字段用插件清单既有元数据（与左侧资料库同源，不新造分类体系）；无厂商/类型元数据的条目归「其他」组不丢弃。

## 文件域

前端仓右键菜单/列表组件+其测试；不碰装载链路（右键→选择→装载→确认）与左侧资料库本体（只对齐呈现形态）。

## 验收标准

- 前端仓构建/既有测试全绿；headless 探针断言（FE 前端先例口径）：分组节点存在（厂商×类型两级）、组内条目可点、装载动作回归。
- 用户目检验收（手测入口=Godot 拉起）：分组形态与左侧资料库观感一致、折叠/展开（v2：检索过滤）正常、装载链路回归。

## 停止条件

插件清单元数据缺厂商/类型字段致分组无米下锅 → 上交字段缺口（不私造分类）。

- 领取：2026-10-12 20:52 / origin/main=41717534633fa4679b2939241c030a8dfc825f23（本地一致）/ owner=GLM-5.3 · PC · ZCode 前端执行会话（用户 2026-10-12 晚窗口令指派领取本卡）/ 前端仓分支=port/fe-rack-list-1，基线=vit-daw-frontend c56e38f586f10b8c154d538f7485c644b1d731aa（活线 port/fe-gui-shrink-1 tip，含先例 FE-RACK-CTX-PLUGIN-LOAD-1 924318d；卡面锚点 rack_plugin_search_popup.gd/left_library_dock._rebuild_plugins_tree:618 在该基线核实）/ 前端 worktree=D:/Godot/project/vit-daw-frontend-fe-rack-list（独立 worktree）/ 领取提交=ebb914c2f7267fdb26d208c9dfc04d9ad71cf065
- 回执：实现 commit=vit-daw-frontend **7c3a7f208c72477f89b7fea145be53d24506c3ee**（port/fe-rack-list-1 已推前端仓 origin，main 未触碰；worktree 保留供复验）/ diff 域=卡内 2 文件（rack_plugin_search_popup.gd 摊平 ItemList→厂商/类型两级 Tree+探针升级；Vit_Graph_Rack.gd 与 left_library_dock.gd 零 diff，装载链与资料库本体未动）/ 探针=109 钉×2 连跑 exit 0（分组形态+组头折叠往返+键盘跨折叠组导航+其他组 Unknown/Other+空态+装载命令面；先例装载/回归钉全保留）；回归 vitnote face_resolve+container+shrink phase A 三面 PASS；--import exit 0 零 SCRIPT ERROR / 端测边界声明=Godot 前端无浏览器断言面，headless probe 即端侧门，真实渲染视觉/指针双击节奏/焦点流转/本机真实清单元数据归手测 / 工件=D:/Vit_DAW/coord/runs/FE-GUI-RACK-LIST-1/（RECEIPT.md+探针/回归/构建日志，按 2026-09-29 政策留盘不入库）/ 停止条件未触发（分组字段=资料库同源 manufacturer/category 在场）/ [等待用户:目检排期——手测清单 6 条入 RECEIPT.md]
- 验收：（裁定文件 / 用户目检）
