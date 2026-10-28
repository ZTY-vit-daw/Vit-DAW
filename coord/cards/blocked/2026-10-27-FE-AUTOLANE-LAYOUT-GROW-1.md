# FE-AUTOLANE-LAYOUT-GROW-1：车道布局 b+a——向下弹出行高增长+收起（手测节形态裁定实施）

- 发卡：GLM 主管决策流 / 2026-10-27 晚（依据=用户手测节确认 b+a：车道撑开行高把下方内容推下去+开关收起；overlay 遮挡形态作废）
- 派发确认：已确认（用户 2026-10-27「同意 b+a。可以开卡」）
- 验收负责人：GLM 主管决策流
- 池序 206；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 / 0.5-1 天 / **依赖=205 合入**（同文件域 Track_Row_Controller.gd+track_envelope_lane_2d.gd，串行不并行）
- 模型分级：L2 / PC 执行会话（布局联动首例）
- 并行与资源：真栈腿错峰手测节（§2.2 增补③）。

## 已核事实（发卡侧亲核）

1. **行高机制现成**：Track_Row_Controller 已有动态行高面——`signal request_track_row_height_px(height_px, scene_track_id)`+`custom_minimum_size.y`（DEFAULT 240/clamp [96,560]）——行高动态与按行变更已是现役机制，本卡接入而非新造。
2. 车道容器现挂 `Track_Split/Right_3D_Wrapper` 内底部叠加条=189 C 案 overlay 形态（遮挡波形根因）；本卡改为行内布局流（波形区高度不变，车道区在其下方追加，行高随车道数增长）。

## 目标

1. **b+a 布局**：车道展开=行最小高度增长（经既有 request_track_row_height_px 机制；波形区高度不变）；开关收起/单条关闭=行高回落；overlay 形态废除（车道从波形上方移开，装配树断言=车道区在波形下方非叠加）。
2. **车道区超高处理**：MAX_TRACK_ROW_HEIGHT=560 上限下的多车道超高（勘察定案：车道区内部滚动或压缩，形态从简并回执披露；需要新形态→上交两案）。
3. **行重绑语义**：虚拟池 recycle=回默认收起态（无持久化，缺省即态——202 语义保持）。
4. **命中/手势零回退**：TimelineViewMapping y 面+车道编辑手势（203 面）在新几何下全量回归。
5. **探针**：行高增长/回落断言（展开 N 车道→高度含 N×车道高；全收→回基线）+装配位置断言+既有全家回归。
6. **真栈**：展开→行高增长→车道不遮波形→编辑链回归（建点/拖点/曲率/undo）两轮。

## 文件域

`app/tracks/track_scene/**`（Track_Row_Controller.gd 行高联动+track_envelope_lane_2d.gd 容器布局）。**高度信号消费方若在域外需改→上交扩域裁定不擅自越**。越域即停。

## 停止条件

- 高度信号消费方（行池/列表容器）不支持运行时按行动态变更（结构假设失效）→锚点上交；
- 车道区超高处理需形态裁定（滚动 vs 上限调整）→两案上交；
- 新几何下 TimelineViewMapping/仲裁器命中面冲突超预期→上交。

- 领取：（2026-10-28 10:15 +0800 / origin/main=2d9d801c38a7e8dc9df46d42e737929c60c0da64 / ZCode GLM-5.3 PC 执行会话（ZCode，206 卡会话）/ 分支=port/fe-autolane-layout-grow-1（前端仓 worktree D:/Godot/project/vit-daw-frontend-lgrow1，领取基线 main=9f65dad）+ 协调 worktree D:/Vit_DAW_wt_lgrow1_coord（coord/lgrow1-claim）/ 领取提交=本提交。领取核对：fetch 后卡仍在 todo+派发已确认+依赖 205 已合入（7b95b1b4 accept、前端 9f65dad=origin/main 亲核）；doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）；真栈空闲（2026-10-27 gate 记录；启栈前按 §2.2 另行登记+≤60s 即时复核，并错峰用户手测节））

- 回执：（**blocked 上交 2026-10-28 10:5x +0800**）——**卡面停止条件①触发**：高度信号消费方（虚拟行池）不支持运行时按行动态变更——信号入口为全局统一高度语义，且消费方在卡面文件域外（`app/timeline/`），按卡面「高度信号消费方若在域外需改→上交扩域裁定不擅自越／越域即停」上交。**零代码改动**（worktree 干净、未启真栈、栈占用零登记，无待释放面）。

  ## 证据锚点（领取基线前端 9f65dad 亲读）

  1. **行高信号唯一消费方=虚拟行池** `app/timeline/timeline_track_list_drop_router.gd`（域外；router 基类=VBoxContainer :1）：
     - `_on_virtual_row_height_requested(height_px, _scene_track_id)`（:1605-1616）**忽略第二参 scene_track_id**，直接写全局 `_virtual_row_height`——任一行发信号=**全部行**统一变高（现役语义=虚拟模式行高整体缩放；行侧现有 emit 源=拖拽手柄 :3922 与 `vit_set_track_row_pixel_height` :3925-3930，后者现仅探针调用）；
     - 按行高度在几何层**结构上受支持**（folder 行先例：`_virtual_record_height` :536-540 按 kind 返回 64），**但无按行动态覆盖输入通道**：`_rebuild_virtual_row_geometry`（:542-553）逐记录写的 `_virtual_y/_virtual_height/_virtual_bottom` 均为**输出**（全局高度/kind 派生）；
     - 行自身局部改高会被复位：`_virtual_row_needs_layout`（:1557-1570）检测 min size 偏差→`_layout_virtual_row`（:1571-1590）逐行重设 position/size/custom_minimum_size.y=记录高度——行局部增长在下次 refresh（滚动/重绑/窗变）回滚，车道区随后溢出/回到遮挡；
     - 内容层为普通 Control 绝对定位（`_ensure_virtual_content_layer` :627-646；池实例化统一 min 高 :780），行 min size 不进布局。
  2. **双路径事实**：虚拟化阈值=24 轨（:24-25，`should_virtualize_track_count` :248-255）。**<24 轨=legacy 直接行**（`app/legacy/vit_control_v_1.0.gd:609-680`：行为 router VBoxContainer 直接子节点）→**行自身 min size 天然回流，b+a 域内可实现**；**≥24 轨=虚拟池**→上墙。真栈腿/手测节典型工程（数轨）走非虚拟路径，但卡面目标 3「虚拟池 recycle=回默认收起态」明示虚拟池在本卡语义面内——不能静默降级不披露。
  3. **域内面勘察已完备（裁定后可直接续作）**：车道 overlay C 案现状=`track_envelope_lane_2d.gd:_apply_lane_geometry` :1029-1037（PRESET_BOTTOM_WIDE+z_index=15+offset_top=-total=波形上方叠加条），挂载 `Track_Row_Controller.gd:1125-1137`（Right_3D_Wrapper 子节点）；行场景 `track_row.tscn`=Track_Root VBox(min 240)→Track_Split(HSplit, v-expand)→Header_2D+Right_3D_Wrapper+Resize_Handle_Y(4px)。**域内 b+a 形态可落**：车道容器改挂 Track_Root VBox 内 Track_Split 之后作固定 min 高子节点——Track_Split 为唯一 v-expand 子节点保持波形区高度不变，行 min 高=240+车道区总高（add bar 22+N×96）随车道数增长。
  4. **TimelineViewMapping y 面**：按行实际 global rect 命中（`TimelineViewMapping.gd:250-310`），新几何自动跟随、无映射改动需求；回归面=既有探针族（lane_toggle 40/lane_auto 41/lane_ui 39/autolane 三件/closeup 等）+新探针。
  5. **超高预判（随裁定一并定，非阻塞项）**：行高 560 上限下车道区预算=560−240−4=316→22+N×96≤316→**N≤3**；超 3 条建议车道栈内部滚动（形态从简）。

  ## 上交两案（扩域裁定）

  - **案一（执行侧推荐）扩域最小面**：文件域追加 `app/timeline/timeline_track_list_drop_router.gd`（仅高度消费路径，~30-40 行）：①信号入口增按行分支——scene_track_id 可解析到记录→写按行高度覆盖（通道区分方式待裁：新独立信号 vs 同信号加用途参数，现有拖拽全局缩放语义保留不破坏）；②`_virtual_record_height` 读覆盖（clamp [96,560]，folder 行保持 64）；③refresh signature 纳入覆盖指纹（:889-899/:1040-1048 两处）使增长触发 refresh；④记录移除/重应用时覆盖随记清除（无持久化=202 语义；recycle 回默认收起态→行重发基线高→覆盖回落）。b+a 全路径成立，与卡面目标 1-4 全对齐。
  - **案二域内降级面**：b+a 仅非虚拟路径（<24 轨）成立；≥24 轨两形态择一——车道区在行高内压缩（波形区被压缩=违反目标 1「波形区高度不变」）或虚拟模式禁展开（功能静默不可达）——均有语义边界，需明示接受才可落。
- 验收：（blocked 待主管裁定：案一→扩域后卡回 todo 续作；案二→卡面修订降级边界后回 todo。上交会话=ZCode GLM-5.3 PC 执行会话 2026-10-28）
