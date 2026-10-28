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

## 领取

- 领取时间：2026-10-28 19:55 +0800
- origin/main hash：f28682cd0c6549401a16da7811d944d251f61a6e
- 执行 owner：GLM-5.3 / PC / ZCode 执行会话（本会话）
- 派发确认：用户 2026-10-28 19:55 前后在本会话直接下达领取指令（手测节当场折卡，用户触发即确认）
- 执行分支：port/fe-lane-catalog-vspdelta-guard-1
- worktree：D:/Godot/project/vit-daw-frontend-vspdelta1
- 前端领取基线：main 4891bae（已核 origin/main = 4891baef4267f6eb0bc11ef9a28b324a030ee410）
- 领取提交：8ef8cc6f（已推 main 并核对远端卡面 owner，正式领取）

## 回执（2026-10-28 20:4x，执行侧自验完成，交主管验收）

- 实现 commit（前端 port/fe-lane-catalog-vspdelta-guard-1，已推）：**0018e50**（修复+探针两件）+**db1692a**（真栈探针三轮迭代定稿）；4 文件 +795/-23。
- 修复落点：`project_store.gd _apply_vsp_delta_reply` cursor 更新后加 `_project_state_tracks_schema_ok` 守卫——delta 应用后轨行缺数组型 plugins 列→判非 ok→refresh 主体零改动自然回落 legacy 全量自愈；严格版判定（非空轨集任一缺列即触发，覆盖混合态）；空轨集豁免；cursor 前进不回退；一次性 fail-loud+`get_vsp_delta_schema_guard_count()` 探针面。**首载入口零改动**（勘察否定简装前提+守卫 ops 空分支自愈兜底[离线 G6 亲证]）。
- 勘察三项定案：①首载=start_page open_project IPC→dock 主场景→store 首载 legacy 全量**带轨行 plugins**（绿窗正常）；②40 次 get_project_state=ProjectStore legacy 分支累计（发送点唯一 project_client.gd:58；delta-ok 短路轮次不发）；③内核 delta 轨行 op=remove/add（value=紧凑轨行恒无 plugins）/replace（value=buildTrackCore 无 plugins）——前端 replace/in-place-add merge 保留不剥列，**append 形态（新轨 add/remove+add 同轨/id 不匹配）驻留剥列**+ops 空也 ok 的短路叠加=断供根因，真栈 DIAG P4c/P6 坐实。
- 真栈：run=coord/runs/FE-LANE-CATALOG-VSPDELTA-GUARD-1/vspdelta_20261028_1（工件随 **port/fe-lane-catalog-vspdelta-guard-1-stack@e18916e8** 入库，RESULTS.md 七项披露）。**DIAG 复现轮 23/23 缺陷签名全绿 exit 0**（基线 4891bae detached worktree；P2b 首载绿→P4c 新轨 vp 恒空跨 delta 覆盖点=用户症状时序面复现→P6 驻留不自愈）；**正式轮 25/25×2 连续两轮 exit 0**（时序面+装配面+修复面全绿）。栈登记 46dcae9c/释放全记录见 PC-RUNTIME-STACK（三纪律亲证；首跑 PS5.1 GBK 吞行尾致参数丢失事故留痕，188 守卫拦截+亲核处置，无外会话污染）。
- 离线：probe_vsp_delta_schema_guard **30/30 PASS**（-s 模式）；全家回归 **24/24 首轮全绿 exit 0**（worktree --import 后，无缓存态事故）。
- 端测边界声明：本腿=agent HTTP/ZMQ 面+Godot headless 生产树断言，**不含 webui 渲染面与用户旅程面**；卡面目标③手测闭环（用户重验双向场景）待用户手测节执行——真栈 P4/P5 已覆盖双向自动等价（先展开再装载/先装载再展开）。
- 停止条件三项均未触发（首载链与假设相符；守卫与 resync/cursor 语义无冲突[离线 G7 亲证 cursor 不回退]；40 次溯源未发现依赖剥列形态的消费方）。

- 验收：（**主管整卡 pass 2026-10-28 晚，GLM 主管决策流**）——①守卫 diff 亲读：落点=_apply_vsp_delta_reply cursor 更新后（cursor 前进不回退亲证=判非 ok 时 revision 已消费，避免旧 base 重放剥列 ops；legacy 回落后 _prime_vsp_state_cursor 幂等对齐）；严格版判定（非空轨集任一缺列）覆盖混合态正确（与 207 快照守卫宽松版差异理由成立——快照全或无/delta 混合态）；ops 空也过守卫=历史驻留自愈兜底（离线 G6 亲证）。②SUMMARY 三份亲读：DIAG 23/23（P4c 新轨 vp 恒空跨 delta 覆盖点=用户症状时序面复现坐实——本卡新增时序面断言义务兑现）+FIXED 25/25×2；离线 30/30（G7 cursor/G8 两实例一次性 fail-loud 在卷）；全家 24/24。③勘察三项定案接受（发卡侧叙事两处细化：40 次=legacy 累计而非另有轮询；驻留=append 形态+短路叠加而非恒短路——执行侧 DIAG P4c/P6 实证）。④栈纪律核对：登记 20:25:06→20:25:24 即时复核+双轮预检；首跑 PS 5.1 GBK 吞行尾事故=如实留痕+188 守卫拦截+pid 亲核本腿产物+无外会话污染（处置正确）；隔离三面+拆栈 20:39:25 全零。⑤六披露全准（判定粒度差异/cursor 时机/vsphub 补位保真[无 hub 则 delta prime 失败恒走 legacy=207 探针未复现 delta 断供的机制解释]/首载零改动依据[条件性目标前提被否定+G6 兜底]）。合入：前端 main=36cda0e（merge）；工件=主仓 merge（vspdelta_20261028_1 全量）。**手测节第三轮重验再解锁**（第三次：修复后用户重拉 Godot 验证参数列表+206 剩余项；守卫触发期双调用过渡成本随内核白名单扩列观察项延续）。
