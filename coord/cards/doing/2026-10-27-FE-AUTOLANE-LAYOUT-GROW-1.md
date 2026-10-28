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
