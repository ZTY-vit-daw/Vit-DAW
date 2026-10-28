# FE-LANE-CATALOG-VSPDELTA-GUARD-1：参数目录断供第三层——store VSP delta 短路守卫（手测节第三轮缺陷卡 II）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=用户 207 合入后重测仍空；发卡侧取证在卷）
- 派发确认：待用户确认（手测节当场折卡惯例）
- 验收负责人：GLM 主管决策流
- 池序 208；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 缺陷卡 / 0.5 天 / 依赖=207 已合入（基线=前端 main 4891bae）
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰手测节（§2.2 增补③）

## 根因（发卡侧亲核，2026-10-28 前端 main 4891bae；用户 19:40 会话取证）

1. **207 修复在工作**：用户 19:40 会话跑新代码（日志亲证）+无 fail-loud 警告（repo 在场）+内核日志 get_project_state 被调 40 次（dock 守卫回落链在发）。
2. **第三层断点=store 层 VSP delta 短路**：`app/state/stores/project_store.gd refresh_project_state()`（:31-39）——`project_state` 非空且 `_vsp_enabled()`（默认真，仅 env 关）时先走 `_refresh_project_state_from_vsp_delta()`；**delta 返回 ok 即直接 return（`_apply_vsp_delta_reply` 对 ops 空也返回 status=ok）→ `_refresh_project_state_from_legacy()`（全量，带轨行 plugins）在首载后被永久短路**。剥列/简装轨行驻留 project_state → dock `_apply_kernel_project_reply_and_emit(store.project_state)` 落库 → repo plugins_by_track 恒空 → 目录全空。
3. **207 探针绿证盲区=时序面**：探针断言时机在首载窗口内（legacy 全量在场），未跨「后续刷新 delta 覆盖/短路」时序点；用户真实使用跨过。装配面口径（§5.1）不覆盖时序差集。
4. 已排除（发卡侧亲证）：工程数据完好（每轨 TRACK 直接子级 PLUGIN type=volume/level 在场）；agent 二进制 10-24 构建后 Go 零代码改动（git log 亲核）；repo 装配在场；用户 GUI=dock 主场景。

## 待勘察（执行侧，勘察结论回执披露）

- 首载入口形态：启动页进工程 → project_state 首次填充走哪条链、轨行带不带 plugins；
- 40 次 get_project_state 调用方归属（store legacy vs 其他面板轮询）；
- `_apply_vsp_track_op` 替换语义下 delta op 是否以剥列轨行整体替换。

## 目标

1. **delta 通道守卫**：delta 应用后 `project_state` 轨行缺 `plugins` 列（数组型）→ 判非 ok → 回落 legacy 全量（与 207 守卫同语义：缺列回落而非静默采用）；或 refresh 分支加同款 schema 判定——形态勘察定案，最小面优先。
2. 首载入口若为简装填充（无轨行 plugins）→ 同样保证后续 refresh 走全量（首载后第一次 refresh 即修复态）。
3. 手测闭环：修复后用户重验（选择参数列表=轨道组+已装插件在场，含「先展开再装效果器」与「先装再展开」双向）。

## 探针义务（§5.1+时序面增补）

- **时序面断言必入**：断言须跨过 delta 覆盖点——首载→触发 N 次后续刷新（含 ops 空/ops 含轨行 op 两形态）→ 断言 repo 轨行 plugins 仍在+目录非空；DIAG 模式复现用户症状（首载后 delta 短路→目录空）。
- §5.1 装配面口径延续（零 repo 自建+实件入场）；205 起禁注入口径延续；既有全家回归零回退。

## 文件域

`app/state/stores/project_store.gd`（delta 守卫）+首载入口文件（勘察定）+探针文件。**修复若须动 project_client/vsphub/内核面→上交扩域裁定**。越域即停。

## 停止条件

- 首载链结构与假设不符（project_state 首载即剥列但另有正规修复设计）→上交锚点；
- delta 守卫与既有 resync 语义冲突（cursor 机制被破坏）→上交两案；
- 40 次调用溯源发现其他消费方依赖 delta 剥列形态→上交。
