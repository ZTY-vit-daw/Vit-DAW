# FE-LANE-CATALOG-VSPDELTA-GUARD-1（208）真栈腿回执 — vspdelta_20261028_1

- owner：ZCode GLM-5.3 PC 执行会话（208 卡会话）；2026-10-28 晚窗
- run：`coord/runs/FE-LANE-CATALOG-VSPDELTA-GUARD-1/vspdelta_20261028_1`（本目录，随 `port/fe-lane-catalog-vspdelta-guard-1-stack` 入库）
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-vspdelta1@port/fe-lane-catalog-vspdelta-guard-1` @ **0018e50+db1692a**（领取基线 4891bae 亲核=前端 origin/main）；DIAG 复现轮=detached worktree `D:/Godot/project/vit-daw-frontend-vspdelta-diag@4891bae`（基线零修复 diff）+探针拷贝
- kernel：**206 自建二进制只读复用** `D:/Vit_DAW_wt_lgrow1_stack/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（sha256 `c9a258de…` 与 206/207 回执同源；`264704a8..eb6825d4` VitApp/+agent/ 零改动亲核——本卡全部改动在前端仓）pid 19280，本腿 ps1 `-StartKernel` 自启
- agent：stack worktree `D:/Vit_DAW_wt_208stack` 构建（Go 零改动，`go build ./cmd/vitagent` exit 0）pid 33776
- vsphub：**主仓 `D:/Vit_DAW/agent/bin/VspHub.exe` 只读复用**（sha256 `b10cc412…`=207 回执同源；用户手测栈形态含 hub——VSP delta 通道复现保真所需）pid 15636 本腿手启
- VIT_PROJECT_XML：run 目录 git 干净版隔离副本（blob `287956e7` 基线=202/203/205/206/207 首腿同基线，hash-object 亲核）显式透传
- ps1：`-RepoRoot D:/Vit_DAW_wt_208stack -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <206 exe 复用>`（console/vspdelta_stack_leg.ps1，exit 0；**ASCII-only 版**——首轮 UTF-8 无 BOM 中文注释被 PS 5.1 按 GBK 解码吞行尾致参数绑定丢失[StartKernel 未传→只起 agent]，纯 ASCII 重写后正常；上轮遗留 agent pid 19476 路径亲核=本腿产物后终止重跑，188 守卫全程在场）

## 一、勘察结论（卡面三项待勘察，全定案）

1. **首载入口形态**：start_page `_send_open_project_ipc`（open_project IPC 直发）→ `_enter_main_scene` 切 dock 主场景；dock store 实例（`vit_dock_root._ensure_project_store` 自建，与 legacy 主控/start_page 各自独立）首载时 project_state 为空 → `refresh_project_state()` :32 条件不满足 → **直接走 legacy 全量（带轨行 plugins）**。首载窗口形态正常，卡面目标②（首载简装→保证后续全量）的前提不成立——但守卫的 ops 空分支天然兜底：任何简装/剥列驻留态在下一轮 refresh 即被判定回落 legacy 自愈（离线 G6 亲证）。
2. **40 次 get_project_state 调用方归属**：前端发送点唯一（`project_client.gd:58`），调用方=`ProjectStore._refresh_project_state_from_legacy`。GUI 用户链（dock 主场景）上=dock 回落链（VSP 快照守卫拦截后 `store.refresh_project_state()` 落 legacy 分支时才发）；**delta-ok 短路轮次不发**——40 次=首载+delta 通道非 ok 回落（prime 失败/resync/探针断言窗口）轮次累计。
3. **_apply_vsp_track_op 替换语义**：内核 `buildStateDeltaOps`（VspKernelReference.cpp:924-980）轨行 op 三形态——remove（移除）、**add（value=normaliseTrackArray 紧凑轨行，恒无 plugins 键）**、**replace（value=buildTrackCore=紧凑轨行去 clips，无 plugins 键）**。前端 replace/in-place-add 经 `_merge_track_preserving_gui_fields` merge（patch 缺键保留）不剥列；**append 形态（新轨 add、remove+add 同轨、id 不匹配）驻留剥列轨行**——叠加 `_apply_vsp_delta_reply` ops 空也返回 ok 的 delta-ok 短路 → legacy 全量在首载后永久不再到达 → dock 回落链落库剥列/混合态 project_state → repo plugins_by_track 恒空。真栈 DIAG 轮 P4c（新轨 vp 恒空）+P6（驻留跨 ops 空刷新不自愈）逐一坐实。

## 二、实现（前端 0018e50+db1692a，4 文件 +795/-23）

1. `app/state/stores/project_store.gd`：`_apply_vsp_delta_reply` 在 cursor 更新后加 `_project_state_tracks_schema_ok` 守卫——delta 应用后 project_state 轨行缺数组型 plugins 列 → 判非 ok（返回 `vsp_delta_schema_guard_tracks_plugins_missing`）→ `refresh_project_state()` :33-35 `_is_ok` false → 自然回落 legacy 全量覆盖剥列驻留（**refresh 主体零改动**）。判定粒度=**严格版**（非空轨集中任一轨行缺列即触发）——delta 应用后存在 merge 保留行+append 剥列行混合态，207 快照守卫的「任一带列即 ok」在混合态漏判。空轨集豁免（无轨行可剥列；与 207 空轨集保守回落不同——delta 路有上态参照且空工程不应每刷必落 legacy，§七披露）。cursor 在守卫前已前进（不回退，resync 语义零破坏）。fail-loud：一次性 `push_warning`+`get_vsp_delta_schema_guard_count()` 探针面。
2. 探针两件：`tools/stack_probe_lane_catalog_vspdelta_guard.gd/.tscn`（真栈时序面）+`tools/probe_vsp_delta_schema_guard.gd`（-s 离线）。

## 三、探针义务兑现

- **时序面断言必入（本卡新增义务）**：P3 ops 空刷新×3（直调生产 `vit_refresh_project_from_kernel`）+P4 生产 `add_track` 轨行 op 刷新（命令回执触发链）+P6 再触发 ops 空刷新×2 终态收口——断言全部跨过「后续刷新 delta 覆盖/短路」时序点（207 探针盲区=断言在首载窗口内）。
- **DIAG 模式复现用户症状**：`VIT_PROBE_208_DIAG=1`（跑在基线 4891bae detached worktree）——P2b 首载绿（207 快照守卫面）→ **P4c 新轨 vp id 恒空跨 delta 覆盖点**（=用户「首载正常、后续目录空」时序面复现）→ P5e 目录 rack 面到 VST 但轨道组缺省 → P6 剥列驻留跨后续刷新不自愈。
- **§5.1 装配面口径延续**：VitDockRoot.tscn 实件入场零 repo 自建（无 `_REPOSITORY_SCRIPT`、无 repo `.new()`，可 grep 亲证）+P1b/P1c 组节点 dock 子树内亲证+P1d store=dock 子树内实件。
- **禁注入（205 口径）延续**：全程无 `set_plugin_catalog`/`set_track_volume_plugin_id` 调用（可 grep）。
- 离线 `probe_vsp_delta_schema_guard`：**30/30 PASS**（G1 守卫语义+G2 ops 空短路保形+G3/G4 append/remove+add 剥列自愈+G5 replace merge 保留+G6 剥列驻留自愈[卡面目标②兜底]+G7 cursor 不回退+G8 fail-loud 一次性[两 store 实例各自一次性亲证]+G9 源面）。

## 四、真栈结果（§8 声明：LLM=0 全确定性；成功=exit 0；失败分类=assertion_fail/environment_interrupt/script_exception）

| 轮 | 命令形态 | 结果 | 退出码 |
|---|---|---|---|
| DIAG r1（探针 v3） | `VIT_PROBE_208_DIAG=1` 基线 worktree 探针场景 | **23/23 缺陷签名全绿**（P2b 首载绿+P4c 新轨 vp 恒空[P4c/P5e/P6 时序面缺陷签名]——用户症状时序面坐实） | 0 |
| 正式 R1（探针 v3） | 探针场景（修复态） | **25/25 PASS**（P1d store 实件+P1e 守卫在场+P3 ops 空刷新保形+P4c 守卫回落新轨 vp 可解析+P4d 旧轨不伤+P4e 守卫计数≥1+P5a2 repo rack 到达+P5e 目录 VST+轨道组恰 volume/pan+P6 全轨 vp 终态稳定） | 0 |
| 正式 R2（探针 v3） | 同上复跑 | **25/25 PASS** | 0 |

探针迭代过程（三轮）：v1 两断言时机缺陷（P1d 懒建 store 时机过早+P6 DIAG 应取缺陷签名）→v2 P5e descriptor 链等待不足→v2.5 FORENSIC_C 取证（repo rack_nodes 竞态定位）→v3 定稿（+P5a2 到达等待）。迭代中间轮 log：probe_forensic_tmp.log 在卷（取证轮，exit 1=P5e 竞态断言，非被测面缺陷）。

## 五、离线全家回归（24 件+新增 1 件）

- 24 件全家（207 同清单，worktree 全量 `--import` 后首轮）：**24/24 全绿 exit 0**（offline_regression/ 目录 24 份 log；`probe_vitnote_input`=取证跑形态无断言面，exit 0=基线等价）。
- 新增 `probe_vsp_delta_schema_guard` 30/30 绿。

## 六、三纪律与隔离亲证

- 登记 `46dcae9c` 推送 20:25:06 后 **20:25:24 即时复核**（远端亲读置顶+进程/端口全零，<60s 窗口）+driver `PREFLIGHT_CLEAN` 二次预检（20:26:07/20:30:10 两轮）；无 already listening（kernel/agent=ps1 自启；vsphub=手启 15636 前亲核 8787 空）；登记先于启动。
- **首跑参数丢失事故（如实记录）**：v1 腿脚本 UTF-8 无 BOM 中文注释被 PS 5.1 按 GBK 解码吞行尾→`StartKernel` 未绑定→只起 agent（pid 19476）未起 kernel（ps1 其余腿过而整体 exit 0）；188 守卫拦住二轮重跑→pid 19476 路径亲核=本腿产物→终止重跑。纯 ASCII 重写后正常。**无外会话栈污染**（遗留进程为本腿自启产物，处置留痕）。
- VIT_PROJECT_XML 隔离三面：run 副本 `287956e7→3e69f8cd`（内核写入=隔离生效）；stack worktree VitApp/ 运行后 git 干净；主检出 default_project.xml `427f2ff7` mtime 19:40:27=用户 19:40 会话遗留值**零触碰**。
- 拆栈=owner 手拆 vsphub 15636/agent 33776/kernel 19280（三 pid 路径逐一亲核=本腿所属）→20:39:25 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/UDP 4444/4445 全零。

## 七、披露

1. **判定粒度与 207 差异**：本卡守卫=严格版（任一轨行缺列即回落），207 快照守卫=宽松版（任一带列即 ok）。理由：快照整体来自内核紧凑面（全或无），delta 应用后存在混合态——宽松版会漏判剥列新轨。空轨集判定也不同（207 回落/本卡豁免）——delta 路有上态参照，且空工程不应每刷必落 legacy。
2. **cursor 前进时机**：守卫判非 ok 时 cursor 已消费 delta revision（`_update_vsp_cursor_from_reply` 在守卫前）——避免下次 delta 以旧 base 重放同一批剥列 ops（每轮多一轮 delta+legacy）；legacy 回落后 `call_deferred("_prime_vsp_state_cursor")` 再对齐（幂等）。
3. **vsphub 补位**（207 披露延续）：ps1 最小腿不启 hub，本腿按用户栈形态手启——VSP delta 通道复现保真必需（无 hub 时 store delta 通道 prime 失败恒走 legacy，缺陷不可复现）。
4. **端测边界**：本腿=agent HTTP/ZMQ 面+Godot headless 生产树断言，**不含 webui 渲染面与用户旅程面**；手测闭环义务（卡面目标③：用户重验选择参数列表=轨道组+已装插件在场，含双向场景）待用户手测节执行——真栈探针 P4/P5 已覆盖双向自动等价（先展开再装载=P4/P5 链、先装载再展开=P5 主体）。
5. **探针迭代轮**：v1/v2/forensic 中间轮的 FAIL 均为探针断言时机缺陷（懒建 store 时机/descriptor 等待/repo 竞态），非被测面差异；定稿 v3 DIAG/正式三轮全绿。v1 DIAG 轮（20/22）中 18 项有效断言与 v3 一致无矛盾。
6. **首载入口零改动裁定依据**：卡面目标②为条件性目标（「若为简装填充」），勘察否定其前提（首载=legacy 全量带 plugins）；守卫 ops 空分支的自愈兜底（G6）覆盖一切简装驻留路径。首载入口文件未被触碰。
7. **腿脚本编码纪律**：PS 5.1 下 BOM-less UTF-8 中文注释可吞行尾破坏参数绑定（本次实证）——后续腿脚本建议纯 ASCII 或带 BOM。
