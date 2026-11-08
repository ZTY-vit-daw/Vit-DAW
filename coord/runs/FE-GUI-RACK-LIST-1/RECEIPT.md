# FE-GUI-RACK-LIST-1 回执：机架右键装载列表分组改造（厂商/类型两级分组 + 检索框）

- 卡：`coord/cards/done/2026-10-12-FE-GUI-RACK-LIST-1.md`（池序 103）
- 执行：GLM-5.3 前端执行会话（PC，ZCode，用户 2026-10-12 晚窗口令指派）
- 目标仓：`D:\Godot\project\vit-daw-frontend`（Godot DAW 前端）
- 领取基线：前端仓活线 `port/fe-gui-shrink-1` tip = `c56e38f586f10b8c154d538f7485c644b1d731aa`（含先例 FE-RACK-CTX-PLUGIN-LOAD-1 `924318d`）；主仓 origin/main=`41717534`，领取提交=`ebb914c2`（doing 移动）+`16a07816`（hash 回填）
- 实现 commit：**`7c3a7f208c72477f89b7fea145be53d24506c3ee`**（分支 `port/fe-rack-list-1`，已推前端仓 origin；前端仓 main 未触碰）
- worktree：`D:/Godot/project/vit-daw-frontend-fe-rack-list`（独立 worktree，保留供决策侧复验）

## Diff 域（卡内 2 文件，越域=零）

| 文件 | 变更 |
|---|---|
| `app/rack/rack_plugin_search_popup.gd` | 摊平 ItemList → 厂商/类型两级分组 Tree（+241/-105 内含） |
| `tools/probe_rack_plugin_search_popup.gd` | 探针升级：分组钉 + 先例钉全保留（109 钉） |

**装载链与左侧资料库本体零改动**：`Vit_Graph_Rack.gd` 无 diff（钩点契约 open_at/close_popup/set_footer_text/plugin_selected 签名全保留）；`left_library_dock.gd` 无 diff（分组口径逐行对齐 `_rebuild_plugins_tree:618`，只读复刻不引用其内部）。

## 设计三件（对齐口径）

1. **v1 分组形态**：Tree 三级=厂商（manufacturer）→类型（category）→插件条目；与左侧资料库同款——两级键 naturalnocase 自然排序、**厂商头常展开、类型头无检索词时折叠**；组头折叠入口=Tree 内建折叠箭头（引擎原生，同资料库）+双击组头切换（`toggle_group`，双击叶条目仍=装载，经 `_leaf_items` 成员判定分流）。叶条目=插件名+tooltip（identifier+路径）+图标（UIAssets plugin chip 16px，资料库同款同源资产）。
2. **v2 检索框**：列表头既有检索框原样保留（`filter_plugins` 纯函数零改动：空格分词 AND + 前缀命中优先 + 六字段）；过滤后**仅保留命中组**（空组整组消失），命中组类型头自动展开直读命中项；↑↓ 键盘导航按叶序扁平序移动、跨折叠组时自动展开祖先组落地可见，越界钳制。
3. **兜底分组（卡面「其他」组）**：缺厂商/缺类型（含空串/空白）归资料库同款兜底标签 **Unknown / Other**（`left_library_dock._rebuild_plugins_tree` 同名同义，不私造分类体系），条目不丢弃；`group_plugins` 纯函数直断面。

## 验收证据

- **探针**：`tools/probe_rack_plugin_search_popup.gd` **109 钉 ×2 连跑 exit 0**（probe_run1/2 日志）。新增面：`group_plugins` 纯函数 8 钉（两级自然序/兜底 Unknown-Other/空串兜底/非 dict 剔除/组内自然序/原始 dict 引用/空输入）+ 场景分组形态钉（厂商头×类型头两级结构/厂商头展开/类型头空检索折叠/叶=插件名+原始 dict metadata+图标）+ 组头双击折叠往返 + 键盘跨折叠组导航（选中叶+祖先展开+钳制）+ 其他组场景（缺元数据条目归 Unknown/Other）+ 空态提示行 ×2 + 叶条目激活=第二装载命令。**先例钉全保留**：装载命令面 8 钉（rack_add_node/track_id/plugin_identifier/plugin_path/auto_connect/zone/有限坐标）+ 回归钉（节点右键=节点菜单不开弹窗/右键拖动=平移/`_can_drop_data`×4/auto_spawn 双击装载/空清单惰性同源刷新×3/Esc 关闭）。
- **回归**：vitnote `face_resolve` PASS / vitnote `container` PASS / `fe_gui_shrink_phase_a` PASS，各 exit 0（日志在本目录）。
- **构建门**：`--headless --import` exit 0，0 SCRIPT ERROR（import_final.log）。

## 端测边界声明

Godot 前端无浏览器级断言面（同先例口径）：headless probe 即端侧门，断言到命令构造面（假 rack client 记录 `rack_add_node` 命令字典）。**不覆盖**：真实渲染视觉（树缩进/图标观感/折叠箭头命中区/分组与左侧资料库的观感一致性）、真实指针双击节奏、焦点流转、真实插件清单元数据（本机实际清单里厂商/类型字段的真实取值面）。归用户目检。

## 手测清单（用户目检；入口=用户从 Godot 拉起 DAW 前端→进入目标工程→机架面板）

1. 机架空白区右键 → 弹窗为厂商/类型两级分组树，观感对齐左侧资料库（厂商头展开、类型头折叠）。
2. 类型头折叠箭头点击 / 双击组头 → 折叠↔展开往返正常。
3. 展开某类型组 → 点选插件条目 → 回车或双击 → 装载链走通（链上节点出现、确认流与拖放装载一致）。
4. 检索框输入厂商名/类型词/插件名 → 仅剩命中组且自动展开；清空 → 全组恢复。
5. 输入无命中词 → 「无匹配插件」行；Esc 关闭弹窗。
6. （若本机清单含无厂商/类型元数据的插件）确认其归入 Unknown/Other（「其他」）组不丢失。

## 环境注记

- 新 worktree 首跑需 `--import` 建缓存（先例勘测结论，本卡复现一次）。
- 本机 Godot 4.6.1 的 `TreeItem` 无 `ensure_visible`；选中叶滚动改用 `Tree.scroll_to_item(item)`（API 勘测件已离场）。
- 卡面停止条件未触发：插件清单元数据含 manufacturer/category 字段（资料库分组同源字段在场，弹窗侧 `FakeDock` 夹具与真件字段同形）。

## 工件清单（本目录，按 2026-09-29 政策留盘不入库）

`probe_run1_20261012.log` / `probe_run2_20261012.log`（109 钉×2）/ `regression_probe_vitnote_face_resolve.log` / `regression_probe_vitnote_container.log` / `regression_probe_fe_gui_shrink_phase_a.log` / `import_final.log` / `RECEIPT.md`
