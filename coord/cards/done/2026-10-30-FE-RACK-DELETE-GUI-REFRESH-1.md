# FE-RACK-DELETE-GUI-REFRESH-1：删除回拉第三跳断裂——legacy get_project_state 未发出（UDP 串行队列竞争候选）+已删插件轮询不停

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=212 用户手测重验失败现场取证，[repro_20261030_user_gui/REPORT.md](../../runs/FE-RACK-DELETE-REDRAW-1/repro_20261030_user_gui/REPORT.md)——主管亲读三方日志+前端代码定链）
- 派发确认：已确认（用户 2026-10-30 晚报障「删除后卡片保留在 GUI 画布节点」即诉求；现场取证闭环）
- 验收负责人：GLM 主管决策流
- 池序 221；目标仓库=**前端仓 D:/Godot/project/vit-daw-frontend**；P1 / 0.5-1 天 / 依赖=无（212 收口挂起待本卡）；L1-L2（插桩定案面 L2 判读，修复面 L1 量产可做）

## 断点定案（取证已闭环，执行侧不重考）

删除链：内核 ok+落盘 ✓ → dock 回执回拉 ✓ → VSP snapshot 207 守卫回落 ✓ → store delta 208 守卫回落 ✓ → **legacy get_project_state 未到达内核（两次删除间 ZmqGateway 全量核对零次）** → store error → dock 静默 return → repo/project_loaded/画布全链停摆（Vit_Graph_Rack.gd:7632 注释明示画布无本地回退）。

## root cause 候选（插桩定案，按优先级）

1. **UDP 串行队列竞争（主候选）**：vit_ipc_client.gd:126 `_process_queue` 在 `_active_request` 非空时零调度（严格单在飞）；envelope lane 周期轮询 `env_lane_params_*`（track_envelope_lane_2d.gd，param 族 30s 超时）与回拉 legacy（8s 软超时）同队列——排队未发出即被软超时丢弃。
2. await 轮询挂死（send_command_async_await:375 while pending 轮询，request 字典引用断裂形态——低概率）。
3. `_udp` null 态（udp_not_ready 即时 error——低概率，同 socket delete_plugin 成功）。

## 目标（三面）

1. **插桩定案**：真栈窗口形态复现（拖插件→参数面板/车道激活→删除→观察画布），插桩面=UDP 队列状态（active/depth/软超时丢弃计数）、await 返回值、store legacy 是否入队。推翻主候选则锚点上交两案，不得盲修。
2. **修复**（按定案收敛，三方向权衡执行侧带证据选）：
   - a. 队列面：未发出请求不受软超时丢弃（发出后才起表）或刷新类命令提权/防饿死；
   - b. 回拉面：dock 回拉链失败时重试/兜底（guard error + legacy error 双败时不得静默——至少 request_failed 可观察）；
   - c. 堵塞源：envelope lane 对已删插件停止轮询（track_envelope_lane_2d.gd 消费删除事件）。
3. **回归**：212 双向量删除链（KEY_DELETE+agent）回归零回退；208/207 守卫测试族零回退；envelope lane 轮询既有测试零回退。

## 探针义务

真栈窗口形态（非 headless——竞争窗口依赖 envelope lane 轮询与参数面板在飞，取证已证 headless 探针覆盖不到此面）：复现「拖插件→删除→画布节点消失」用户旅程断言；插桩读回面（队列丢弃计数/await 结果）入 RESULTS。既有 stack_probe_rack_delete_redraw 扩展或同模式新增。

## 文件域

前端仓 `app/kernel/autoloads/vit_ipc_client.gd` + `app/state/stores/project_store.gd` + `vit_dock/scenes/vit_dock_root.gd` + `app/tracks/track_scene/track_envelope_lane_2d.gd` + 测试/探针。修复方向未选前不越域；若定案落在域外（如 rack 面）→锚点上交。

## 停止条件

- 插桩推翻主候选且三个方向均不适用→锚点+证据上交，不盲修；
- 修复需动内核或 agent→上交。

## 领取

- 领取：ZCode GLM-5.3 PC 执行会话（221 卡会话）/ 2026-10-30 晚
- 前端仓基线：HEAD=83456871de9d8faf796fcc2c797b78d0c155d9a1（=origin/main tip）；status 仅 untracked 用户工程目录，零已跟踪改动（diff --stat 空）
- worktree：D:/Godot/project/vit-daw-frontend-rackgui1 @ port/fe-rack-delete-gui-refresh-1 @ 8345687，git 干净
- 插桩（纯观测，VIT_221_IPC_DIAG=1 门控）：vit_ipc_client.gd（queued/sent/reply/soft/hard timeout/await_done/orphan print+get_diag_221_counters 计数器）；project_store.gd（legacy_enter/legacy_return）；vit_dock_root.gd（dock_refresh 链各跳+dock_refresh_STOP 静默点）；track_envelope_lane_2d.gd（descriptor_request）
- 窗口探针：tools/stack_probe_rack_delete_gui_refresh.tscn（212 同族骨架+W 竞争窗口腿+D 插桩读回腿；VIT_PROBE_221_DIAG=1 复现轮）
- 离线自检：probe_vsp_delta_schema_guard 30/30、probe_lane_catalog_fail_loud 18/18、probe_file_reply_volume_face 7/7、probe_221_diag_selfcheck 10/10，全 exit 0

## 回执（2026-10-30 20:0x，执行侧自验完成待主管验收）

- run=coord/runs/FE-RACK-DELETE-GUI-REFRESH-1/gui_refresh_20261030_1（RESULTS.md 含定案证据链+修复锚点+上交三条）
- **定案**：主候选（UDP 串行队列竞争）**代码+数据双推翻**——软/硬超时定时器只在请求发出时 arm（`_arm_request_timers` 于 `_process_queue` 内），排队请求无 timer 不会被软超时丢弃；真栈窗口轮密集采样 119 点 soft/hard_timeout=0、丢弃=0、max_wait 281ms。**真 root cause=VSP delta-ok 短路吞 rack 域删除**：node_remove 不在 project.timeline scope → 删除后回拉 delta ops 空 → delta-ok 短路返回 store 内存旧态（含已删插件）→ repo/画布重喂旧态滞留。r3 完整复现用户症状（Pro-Q 3 vst1=1055，删除 ok 后终态 repo/canvas=["1055"]）。用户取证形态=同症状另一路径（剥列驻留→208 守卫→legacy 未达内核→静默）。次生双复现：lane catalog 风暴（40ms 同 pid 5 连发）+已删插件 error 轮询（env_lane_params_1055）。
- **修复三件套**（前端仓 worktree，未提交按卡面指令）：d=rack 命令族回拉 force_legacy（vit_dock_root.gd 回执处理器传 true+`_refresh_project_repository_from_kernel(force_legacy)`+store `refresh_project_state(force_legacy)`；修复后删除+28ms legacy ok 画布秒清）；b=回拉终败 push_warning（原静默 return）；c=lane `_request_descriptor_for(pid, force)` 结构面在飞+500ms 节流、count 新鲜度 force 放行（192 ●N 语义保留）。
- **验证退出码**：窗口正式轮最终形态 29/29×2 连续 exit 0（probe_fixed_final_r1/r2）；212 双向量删除链回归 29/29×2 exit 0（regress_212_r1/r2）；离线族 208 守卫 30/30+207 18/18+lane 三族 25/27/36+file_reply 7/7+自检 10/10 全绿（lane_auto 首轮 2 红=节流误伤 C1/C3 由既有探针抓住→force 放行修正后全绿，如实留证）。
- **上交三条**：①用户形态 legacy 传输面断链机制未完全定案（快回形态不可复现；d+b 已使其非唯一通道+可见，复发时 [VIT221] 插桩直接暴露断点层）——建议用户手测节重验删除旅程；②delta 通道对 rack 域盲=内核 VSP 面设计事实，扩 scope 属域外交主管裁定；③单在飞饿死窗口机制在无触发证据，留档不修。
- 纪律：claim e9419676/登记 d8d16709/释放 435f2e16 均推共享 main；隔离三面亲证（run 副本承接写入/主检出 087B1AA0 零触碰/kernel C1E32DFE 零漂移）；拆栈复核全零；探针 4 新文件+生产 4 文件（160+/18-）全在卡面文件域。

## 验收（主管裁定 2026-10-30 晚，GLM 主管决策流）

- **整卡 pass（代码面）**。diff 四文件+4 探针逐读：d 面 force_legacy 默认参数零回归面+六命令族精准覆盖+注释机制完整；b 面终败 push_warning 不静默；c 面节流 force 通道保 192 ●N 语义（C1/C3 首轮误伤被既有探针抓住→修正，回归教训如实留证）；ipc_client 纯插桩 env 门控零行为改动。
- **主候选推翻裁定=接受**：timer 仅在发出时 arm（代码亲核 `_arm_request_timers` 于 `_process_queue` 内，排队无 timer）+119 点零丢弃——主管 10-30 取证假说（排队未发出即丢弃）**错误，执行侧修正成立**。真 root cause=delta-ok 短路吞 rack 域删除（r3 复现+机制链完整）：node_remove 不在 delta scope→ops 空→空 ops+schema 合法即返回 ok 旧态→重喂。用户 10-29 形态=同症状另一路径（剥列驻留→guard error→legacy 断链未定案）。
- 验证亲核：窗口正式轮 29/29×2+212 双向量回归 29/29×2+离线族全绿（RESULTS 抽查）。
- **上交三条处置**：①legacy 断链未定案=接受留档（d+b 使其非唯一通道+插桩在场；**裁决点=用户手测重验删除旅程**，复发则 [VIT221] 插桩直接暴露断点层）；②delta rack 域盲=留档不扩 scope（force_legacy 已绕开，内核 VSP 面大改无必要）；③饿死窗口=留档不修（无触发证据）。
- 合入：前端 main=f805bde（worktree 5ac6f00→merge）；212 收口=用户手测重验通过即随本卡闭环。
- 插桩保留裁定：VIT_221_IPC_DIAG 门控面留作复发取证面（env 默认关零开销）。
