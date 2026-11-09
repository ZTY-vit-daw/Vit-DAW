# LAUNCHER-CHAT-FE-1：最简对话窗+启动台挂载+磁带卡（332 拆卡 C1，Godot 域）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=332 设计稿 §7 C1+v2 终选裁定；用户确认开工）
- 派发确认：已确认（用户 2026-11-09「可以按这个方案重做 UI 和交互」）
- 验收负责人：GLM 主管决策流+用户 GUI 终审
- 池序 335；P1 / 2-2.5 天 / 依赖=零（与 337 并行，契约 JSON 先冻结）；L2 / PC（Godot 前端仓）；零栈单测（headless 探针）+真栈腿并入 339

## 目标

1. 新组件 `app/startup/launcher_chat_panel.gd+.tscn`（消息流/输入框/agent 事件卡/流徽章/未就绪禁用态/降级引导卡）——视觉规格权威=`D:/Vit_DAW/agent/webui/design/launcher-route-mondrian-v2.html`（令牌表+磁带卡形态+开场白布局照此实现，Godot 控件化）；
2. start_page 挂载：视觉主位+agent 就绪态接线（就绪前输入禁用+「正在唤醒搭档…」状态行）+磁带流收为背景层/一键展开抽屉（把手放磁带 icon——TapeIconArt 既有类直接复用，规格已在案）；
3. 工程结果卡=磁带卡：复用 tape_atmosphere.gd 纯函数（气氛渐变+带轮量）+TapeIconArt——既有零件零新造；
4. 降级态：agent 探测超时→引导卡形态（「搭档暂时不在。可以直接选工程…」+重试钮）。

## 文件域

前端仓 `app/startup/`（新文件）+`start_page.gd`（挂载+就绪接线 ~30 行插点）；**红线零触碰** LLM_Chat_Controller.gd/LLM_Chat_Window.tscn。

## 验收标准

headless 探针断言（窗节点在场/就绪前禁用后解禁/磁带卡数据绑定）+渲染面边界显式声明（AGENTS §5）+真栈腿并入 339。

## 领取/回执/验收

- **领取**（2026-11-09）：origin/main=987455063a48f54193621f7060b4e1d8927833b8；owner=GLM-5.3 L2 执行子代理（335exec，PC）；实现分支=前端仓 `port/launcher-chat-fe-1` @基线 f805bde0d69365b27a5747292ead36e34d04d000（领取基线亲证：前端仓 main 干净，仅未跟踪运行时目录 `.vit_agent/` 等既有项，与本卡文件域无涉）；worktree=`D:/Godot/project/vit-daw-frontend-lchat1`；领取提交=本提交（仅 coord/ 变更）。doing 三卡（273/275 族 agent 域+WAVESHELL 取证）文件域零冲突已核对。
- 回执（2026-11-09，执行侧自验候主管验收）：
  - **实现 commit**：前端仓 `port/launcher-chat-fe-1` @fc55775（6 文件 +1081/-5：新增 `app/startup/launcher_chat_panel.gd`（683 行）+`.tscn`+`.uid`、`tools/diagnostics/launcher_chat_panel_probe.gd`（38 检）；修改 `app/startup/start_page.gd`（+135 挂载/就绪接线/抽屉把手/`_set_feed_mode` 抽屉化）、`start_page.tscn`（挂 LauncherChatPanel 视觉主位+TapeDrawerBar 红条+FeedLayer 初始收起））。worktree=`D:/Godot/project/vit-daw-frontend-lchat1`。
  - **红线自检（亲证）**：`git diff HEAD -- app/shell/LLM_Chat_Controller.gd app/shell/LLM_Chat_Window.tscn` 输出为空（零 diff）；两文件最后触达提交=历史 397beea，本卡未动。LLM_Chat_Controller.gd 仅只读参考未引用其代码路径。
  - **headless 探针**（`godot --headless --path . --script tools/diagnostics/launcher_chat_panel_probe.gd`，Godot 4.6.1 console exe）：**38/38 检 ×3 轮全过，exit 0**。覆盖：①装配面（生产 start_page.tscn 实例化：panel/流徽章/输入/状态行/抽屉条/把手 TapeIconArt 实例/feed 默认收起/z 序）；②就绪态接线（WAITING 输入禁用+「正在唤醒搭档…」→`_notify_launcher_agent_state(true)` READY 解禁+开场白一次）；③磁带卡（mock 三工程名绑定+TapeIconArt 复用+TapeReel fill=spool_fills_for_name 确定性+带轮量两两不同 0.37/0.28/0.38+几何纯函数 r=19.2）；③b 真实数据源覆盖（真实 entries 替换 mock 并绑名）；④三形态回执渲染（text/launcher_directive 卡含工程名/project_result_cards 2 磁带卡）；⑤用户发送链（UserMessage 入流+send_requested 发出+composer 清空）；⑥降级态（notify(false)→引导卡「搭档暂时不在。」+重试钮+输入禁用+retry_requested 接线）；⑦抽屉开合两态+把手文案翻转；⑧卷分隔条。**回归**：`f5_startup_smoke` exit 0（start_page children=6→VitDockRoot 进入正常）；`clip_fade_gain_gui_probe` exit 0（start_page 入口链无回退）。
  - **契约冻结面（供 337 对齐）**：`render_reply(payload)` 三字段——`{"text"}`=普通对话；`{"launcher_directive":{action,file_path,project_name,reason,requires_governance}}`（§4.1 原样）；`{"project_result_cards":[{path,name,mtime_text}…]}`（结果卡数组，字段名本卡新定）。未知字段 push_warning 留痕（§5.1 fail-silent 面）。
  - **诚实边界/渲染面声明（AGENTS §5）**：①本卡零真栈（真栈腿并入 339）；渲染结果未过浏览器级/像素级断言——headless 探针覆盖节点树+数据绑定+状态机，**视觉呈现（v2 令牌观感/硬投影/布局比例）未经渲染面验证**，待用户 GUI 终审或 339 真栈腿；②消息流对接=337：本卡发送后无自动回执（骨架），开场白=确定性模板（仅真实 recent 数据点名工程名，mock 兜底态中性文案）；右列磁带卡在 recent 数据到达前显示 mock 三工程视觉兜底（展演骨架），真实链到达即覆盖（探针 3b 已断言）；③directive 卡为展示态（执行接线随 337/338），卡面如实标注；④降级重试完整等待腿（8s 探测循环）未在探针内驱动（headless HTTP 拒连会占满 8s），断言到 retry_requested 接线在场+waiting 回转面。
  - **实现说明**：`_set_feed_mode` 的 `_feed.visible` 由硬编码 true 改为随 `_tape_drawer_open`（磁带流默认收起=对话窗主位；把手一键展开全功能经典页——双轨保险落位）；`_apply_recent_list_from_projects` 既有重复双调用 `build_feed_entries` 合并为单次求值（行为等价，产物数组喂双消费方），属本卡文件域内顺手清理。
- 验收：（主管回填）

### 终验（主管，2026-11-09 深夜）

- **pass（零栈段）**——主管亲测：headless 探针复跑 38/38 PASS exit 0（装配/就绪态/磁带卡三工程带轮量两两不同/三形态回执/降级/抽屉）；红线亲证 LLM_Chat_Controller/Window 零 diff；f5/clip_fade 回归采信。前端分支 port/launcher-chat-fe-1 @fc55775 续用于 336（同 worktree 链式）。**渲染观感未验（headless 限制如实）——候 339 真栈腿+用户 GUI 终审**。
