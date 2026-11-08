# FE-GUI-START-FEED-1 R3.1 返工工件（去列表转场末帧闪烁修复）

- 前端仓：`port/fe-start-feed-r3-1@2407c0c`（父=6dfa414 R3 tip；3 文件 +316/−46；已推 origin）
- worktree：`D:/Godot/project/vit-daw-frontend-startfeed-r2`（续用）；frontend main 未触碰
- 领取基线：HEAD=6dfa414、工作树干净（本卡 diff 全量=3 文件）

## 修复内容（app/startup/start_feed_view.gd）

1. **分类记账**：`_transition_tweens` 条目改 `{"tween", "terminal"}`——stagger 瓦片淡入/页头与
   chrome 交叉淡入淡出=terminal（终态即终点）；flight/holder=过程 tween。
2. **收口分流**：`_finish_view_transition(natural)`——自然完成（flight/holder 回调 `bind(true)`）
   只杀非终态、终态 tween 后台放完，并跳过逐瓦片 alpha=1.0 复位与收口选中样式补应用；
   中断路径（resize/重入 set_view_mode/set_projects/开模态/flip）保持全杀+瞬吸附（R3 语义不变）。
3. **选中提前**：`selected_list_index` 同步与 `_update_list_selection_visuals()` 提前到去列表
   转场开始应用（落点瓦片带选中暖色罗列，末帧无样式突现）。
4. **淡出并行（卡面可选润色，已做）**：卡淡出与飞行尾段并行（起于飞行 60%=
   `set_delay(DURATION_SLOW*0.6)`）掩蔽落点交换；收口回调排并行组末尾（0.24s），保证收口的
   卡复位不打断未放完的淡出。
5. **卫生**：`_play_view_transition` 开局与新网格重建（`_rebuild_list_grid`）先清扫记账（防终态
   tween 踩已释放瓦片）；报告新增 `natural` 字段；自然完成不再误标 `snapped=true`（R3 报告语义
   瑕疵，顺带修正——中断钉不受影响）。

## 验证

- 探针 `tools/probe_start_feed_view.gd`：**244 钉 ×3 连跑全 PASS exit 0**（原 228+16；日志
  probe_r31_run{1,2,3}.log）。R3.1 新钉：重入中断（瓦片复位证据/报告 snapped/无存活 tween）、
  自然完成（custom_step 同步步进：idle 到达/not snapped/natural=true）、淡出尾段几何并行
  （fade_start∈[0.08,0.17]+淡出中卡未落位）、选中暖色转场开始即应用（目标瓦片 g=0.97）、
  stagger 存活记账、瓦片无 <0.5→≥0.999 单步跳变、终态稳定。**探针口径说明**：headless 同步
  语境下 tween 完成后 `is_valid()` 翻 false 依赖树自动处理周期，"放完"以终值+稳定性断言；
  `custom_step` 同步步进已 scratch 验证（延迟+时长+回调串行语义成立）。
- 回归电池（r3_1_battery/）：Phase A/B/C、rack 109、vitnote 六、`--import` ×2 全 **exit 0**，
  SCRIPT ERROR=0（12 命令）。
- 离屏取证（r31_visual_evidence/，窗口渲染/VIT_SKIP_DEV_AUTOSTART=1/无栈/用完退出）：
  6 图+shot_console_r31.log。**夹具 12 工程**（3 工程时 stagger 末瓦片 0.24s 恰追平收口、
  长尾不可见，首拍作废已弃）；finalize 前后帧=逐帧缓存+翻转检测动态截取。遥测行：
  `pre_finalize tile_min_alpha=0.000`、`post_finalize tile_min_alpha=0.000`（收口时刻 late 瓦片
  仍在淡、**无同帧拍亮**——R3 旧码此处会瞬置 1.0）、`list_settled tile_min_alpha=1.000`
  （后台放完）。运行 SCRIPT ERROR=0（余 godot_cef dll 环境类 ERROR=R2 已登记边界）。
- 红线门：本卡增量=3 文件（start_feed_view.gd+probe+capture，卡内域）；924318d 起累计 48
  文件中红线 9 文件（telemetry_manager/mix_client/LLM_Chat_Controller/vsp_asset_adapter/
  left_library_dock/右键菜单/拖拽双落点/走带电平链）零命中。

## 边界与待办

- 端测边界声明（AGENTS §5）：headless probe=命令面确定性门；离屏取证=渲染面证据（alpha 遥测
  亲读）；真实转场手感（闪烁是否消除）归**用户复测**。
- [等待用户：R3.1 复测]——worktree `D:/Godot/project/vit-daw-frontend-startfeed-r2`
  （分支 port/fe-start-feed-r3-1@2407c0c）；复测通过前 frontend main 不合并（R3 未合 main，
  主检出无转场，复测须指到本 worktree）。
