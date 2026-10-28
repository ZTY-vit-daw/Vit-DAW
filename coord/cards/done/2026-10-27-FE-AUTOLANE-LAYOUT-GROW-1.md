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

## 主管扩域裁定（2026-10-28，GLM 主管决策流；停止条件①处置）

**案一通过**。依据=主线质量优先原则（10-27 GUI 重设计裁定「稳定/高效/质量优先，不追快速落地」）+卡面目标 3 明示虚拟池在卡语义面内——案二两形态均违反卡面语义（压缩波形违反目标 1「波形区高度不变」；虚拟模式禁展开=功能静默不可达），**否决**。

- **文件域追加**（以本裁定为准）：`app/timeline/timeline_track_list_drop_router.gd`，限高度消费路径五面——新按行覆盖信号入口+覆盖字典+`_virtual_record_height` 读覆盖+refresh 签名指纹+覆盖生命周期；其余面（拖拽/池化/滚动/记录管理）不动。
- **通道区分=新独立信号**（裁定依据：同信号加用途参数必改既有 emit 语义面——行侧两处现役 emit 须零破坏）：建议 `request_track_row_height_override_px(height_px, scene_track_id)`；既有 `request_track_row_height_px` 全局缩放语义零改动。
- **覆盖语义**：clamp [96,560]；folder 行（64）/master 行（112）kind 高度保持不读覆盖；记录移除/重应用随记清除（无持久化=202 语义）；recycle 回默认收起态=行重发基线高→覆盖回落（执行侧方案照准）。
- **签名指纹**：覆盖纳入 `_virtual_current_refresh_signature()`（门控 :798-808、提交 :1128——主管亲核行号；上交回执 :889-899/:1040-1048 为近似引用，以本裁定为准）。
- **超高定案（目标 2 收口）**：560 上限下车道区预算 316px（560−240−4），22+N×96≤316→N≤3；**N>3 车道栈内部滚动**（形态从简），无需另开形态裁定。
- **探针义务增补**：①双形态断言=<阈值直接行路径+虚拟池路径（虚拟池形态允许经产品现役 env `VIT_GUI_VIRTUAL_TRACK_LIST_THRESHOLD` 降档构造（:248-255 亲核），属产品机制非注入；断言须同时覆盖默认阈值语义零破坏）；②按行覆盖生效断言（单行展开→仅该行高度变化，他行基线不动）；③其余按卡面目标 5（205 起口径：真栈腿禁注入+生产链断言）。
- **锚点亲核记录**：信号入口忽略 scene_track_id（:1605-1616）/记录高度全局派生（:536-540）/几何重建输出（:542-553）/局部改高复位（:1557-1590）/阈值 24（:24-25）/行侧 emit 两源（Track_Row_Controller.gd :3922/:3925-3930）/签名机制（router :798-808/:1128）——上交证据全部属实，停止条件①触发正当，零代码改动零栈占用合规。
- 卡流转：blocked→todo，owner 不变（同会话续作）；前端 worktree 已清（主管亲核不存在），续作重 build worktree port/fe-autolane-layout-grow-1，领取基线不变（9f65dad）。

- 领取（续作）：（2026-10-28 11:0x +0800 / origin/main=26740b7f（主管扩域裁定提交）/ owner 不变=ZCode GLM-5.3 PC 执行会话（206 卡会话，裁定「owner 不变，同会话续作」）/ 分支=port/fe-autolane-layout-grow-1 重建（worktree D:/Godot/project/vit-daw-frontend-lgrow1，领取基线仍 9f65dad）+ 协调 worktree D:/Vit_DAW_wt_lgrow1_coord（coord/lgrow1-claim）/ 领取提交=本提交。领取核对：卡在 todo（裁定流转 blocked→todo）+扩域裁定已下（26740b7f 亲读全文）+依赖 205 已合入；doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）；真栈腿启栈前按 §2.2 登记+≤60s 复核+错峰手测节）

- 回执：（**执行侧自验完成 2026-10-28 13:4x +0800**）实现=**port/fe-autolane-layout-grow-1@cf25ccc+a8a22a0**（已推远端；基线 9f65dad）；真栈腿=**coord/runs/FE-AUTOLANE-LAYOUT-GROW-1/layoutgrow_20261028_1**（工件随 **port/fe-autolane-layout-grow-1-stack@a2547571** 入库；RESULTS.md）。交付面（裁定案一全落）：①lane 容器行内布局流（Track_Split 后固定 min 高子节点；overlay z15 叠加形态废除；header 宽 spacer 动态跟随 Track_Split 分割位=波形 x 轴对齐；可见时间范围显式绑 wrapper；hit/focus 改 strip 实际 rect）；②行高联动 U+lane_area（clamp 560；展开=emit request_track_row_height_override_px、收起/重绑=0 清除；拖拽/程序化设高基面=U 派生；bind_virtual_track switched=车道回默认收起态=202 语义+重绑自愈幂等重发）；③router 高度消费五面（新独立信号+覆盖字典 clamp[96,560] folder64/master112 不读+签名两处指纹+生命周期=差分随记清除/clear 全清；**既有 request_track_row_height_px 全局缩放语义与行侧两 emit 源零改动**）；④超高=N≤3 全展开（预算=560−U−4），N>3 车道栈 ScrollContainer 内部滚动（裁定定案）。验收面：离线新探针 probe_autolane_layout_grow **11/11**（D 段直接行形态：增长/回落/装配位置/spacer 对齐/超高滚动/U 基面/重绑收起）+**既有全家 23 件零回退全绿 exit 0**（rebuild 75 含 S 段形态更新+4 新断言/edit 63/prodchain 65/toggle 27/auto 25/ui 36/teardown 50/datachan 50/shrink×3/rack×4/start_feed/vitnote×6）+自愈补丁后核心六件复跑全绿；**真栈连续两轮 exit 0**：stack_probe_autolane_layout_grow **16/16×2**（V0 env 降档机制=产品现役 env+V1 虚拟池激活+V2 行装配+V3 按行覆盖生产链默认车道落定仅行 A 增长 358/他行 240+L1 真栈行内布局波形下方非叠加+V4 独立覆盖+V5 全局缩放零破坏共存+V6 覆盖清除+V7 记录移除/重放一致性+V8 默认阈值零破坏）+stack_probe_autolane_edit 编辑链新几何回归 **30/30×2**（建点/拖点/曲率/undo 零改动复跑=命中/手势零回退实证）。**§2.2 三纪律**：登记 1f66f40a 推送 12:55:50→12:56:03 即时复核（13s<60s+远端亲读置顶）+PREFLIGHT_CLEAN 二次预检 12:56:11+无 already listening+登记先于启动；释放 d2d8b056（拆栈复核进程/端口全零）；VIT_PROJECT_XML 隔离亲证（287956e7→c0b6072f 内核写入生效/worktree 本体 287956e7 零污染/主检出 78b828ee 零触碰）；错峰手测节（疑似用户手测 Godot 已自行退出后启栈，非本腿处置）。**披露五项（RESULTS.md §五）**：ⓐ仲裁器 envelope 面几何边界（车道区出 wrapper rect——mapping 域外不在扩域五面未改；编辑手势主路径 strip _gui_input 先收零影响；差异面=车道头部间隙点击分派语义）；ⓑN>3 滚动条占宽 ~12px（≤3 条零影响）；ⓒV7 语义细化（同 id 重放=lane 态保留+bind 自愈一致；真删轨=recycle 收起归 D10——生产 add_track 新 id 无同 id 复活路径）；ⓓ既有探针期望更新四处（198 先例：rebuild S 段/lane_ui A2+A10/A11/edit 独立形态显式宽/prodchain 释放路径×3）；ⓔ**端测边界声明**：覆盖=装配/行高联动/虚拟池按行覆盖/生产链目录（真 repo+真 IPC）/编辑链全旅程；未覆盖=渲染像素观感、真实鼠标点击旅程、多轨大工程真机滚动观感——留手测面（206 合入后手测节即本卡 UI 面）。迭代轮 r1-r3 红轮留证（探针侧修正：records 过滤/store 环境补全/行装配等待/V7 语义）+生产补丁一轮（bind 自愈，域内）。**异常如实记录**：无环境中断；探针 exit 0 时的 freed-object teardown 噪声（退出期释放顺序，断言面不受影响，r4/r5 SUMMARY_JSON 均零失败）。
- 验收：（done 待主管裁定——建议核对：实现 diff（cf25ccc+a8a22a0）/工件分支 port/fe-autolane-layout-grow-1-stack@origin/RESULTS.md 五项披露/栈记录登记-释放对 1f66f40a↔d2d8b056）

- 验收：（**主管整卡 pass 2026-10-28**，GLM 主管决策流）——①**diff 亲读三面全落裁定案一**：router 五面（新独立信号+幽灵覆盖 no-op 守卫+clamp [96,560]+is_equal_approx 防抖+签名两处同步指纹+差分随记清除——**既有 `_on_virtual_row_height_requested` 全局缩放入口与行侧两 emit 源原样未动亲证**）；行侧 U 基面派生+recycle 收起+bind 自愈幂等重发+超高预算=560−U−4（裁定公式）；车道侧 overlay z15/负 offset 庚除+行内 min 高+ScrollContainer 内部滚动+hit/focus 实际 rect+spacer x 对齐+wrapper 显式绑定（探针独立挂载保留回退面）。②**真栈终轮 SUMMARY 亲读**：layout_grow R5 16 检 0 败（V3 按行覆盖生产链仅行 A 358/他行 240+L1 波形下方非叠加+V5 全局缩放共存+V8 默认阈值零破坏=裁定探针义务三项全达）+edit R2 30 检 0 败（undo/redo 内核读回一致）。③栈登记-释放对（1f66f40a↔d2d8b056，13s 即时复核）+VIT_PROJECT_XML 隔离（287956e7→c0b6072f）核对；④红轮 r1-r3 留证+生产补丁域内披露合规（198 先例四处探针期望更新亲核=diff stat）；⑤五披露准——ⓐ仲裁器 wrapper rect 边界/ⓑ滚动条占宽 12px/ⓒV7 语义细化三项接受（mapping 域外未改正确，差异面留手测观察）；ⓔ**端测边界声明接受**（渲染像素观感/真实鼠标旅程/多轨滚动观感=手测节面）。合入：前端 main=3b5df90（merge）；工件=主仓 merge（layoutgrow_20261028_1 全量）；**手测节第三轮解锁**（gate 10-27 §5 首排②）。
