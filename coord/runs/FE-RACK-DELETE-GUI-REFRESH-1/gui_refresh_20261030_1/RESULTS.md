# FE-RACK-DELETE-GUI-REFRESH-1（221）真栈腿回执——窗口形态插桩定案+修复+回归

- owner：ZCode GLM-5.3 PC 执行会话（221 卡会话）
- 日期：2026-10-30 19:27 登记（d8d16709）→ 20:0x 拆栈释放
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-rackgui1@port/fe-rack-delete-gui-refresh-1@8345687` + 插桩四文件 + 窗口探针 `stack_probe_rack_delete_gui_refresh`
- 栈：kernel=主检出 `VitApp.exe`（sha256 C1E32DFE… 运行前后零漂移；本卡 C++ 零改动）pid 35116 / agent=stack worktree `D:/Vit_DAW_wt_221stack@e9419676` 自建 pid 24860 / vsphub=主仓 `agent/bin` 只读 pid 38160（启前亲核 8787 空）；VIT_PROJECT_XML=run 目录隔离副本 project_iso.xml（git-clean 基线 blob 287956e7 → 运行后 B0454967/mtime 19:47=隔离生效；主检出 default_project.xml 087B1AA0 mtime 18:11=登记前遗留零触碰）

## 一、插桩定案（DIAG r1-r3，VIT_221_IPC_DIAG=1 + VIT_PROBE_221_DIAG=1）

插桩面：`vit_ipc_client.gd`（queued/sent/reply/soft-hard-timeout/await_done/orphan print + `get_diag_221_counters()` 十计数器）；`project_store.gd`（legacy_enter/legacy_return）；`vit_dock_root.gd`（dock_refresh 链各跳）；`track_envelope_lane_2d.gd`（descriptor_request）。

### 主候选（UDP 串行队列竞争堵塞 legacy）——推翻（代码+数据双证）

1. **代码面**：软/硬超时定时器只在请求**发出时** arm（`_arm_request_timers` 于 `_process_queue` pop_front 后调用），排队中请求无 timer——卡面「排队未发出即被软超时丢弃」的机制在现行代码不成立。
2. **数据面**（r2/r3 窗口轮，含删除窗 30s×0.25s 密集采样 119 点）：soft_timeout=0 / hard_timeout=0 / late_reply_drop=0 / orphan_drop=0 / await_error=0；max_queue_depth=10 / max_queue_wait_ms=281——本机快回形态下单在飞队列零堵塞。legacy get_project_state 在装配期与 B 腿守卫回落的发出均 ok（ipc_6/ipc_7/ipc_8 等，queue_wait 29-91ms）。

### 真 root cause 定案（r3 完整复现用户症状）

**VSP delta-ok 短路吞掉 rack 域删除**：`node_remove`（delete_plugin）的内核变更不在 VSP delta 的 `project.timeline` scope——删除后回拉刷新走 `_refresh_project_state_from_vsp_delta` → **ops 空** → schema 判 ok → **delta-ok 短路返回 store 内存旧态（含已删插件）**→ `_apply_kernel_project_reply_and_emit` 重喂旧态 → repo/画布滞留。

r3 证据链：vst1=1055（真实 Pro-Q 3）→ delete_plugin reply ok（内核+落盘层全对，212 结论维持）→ 删除后 dock_refresh 走 delta-ok（`refresh: vsp delta ok (no legacy hop)`，legacy receipts 全部时刻 < 删除时刻）→ 终态 repo rack_nodes=["1055"] 且 canvas=["1055"]——**画布滞留=用户症状 100% 复现**。

用户取证形态（18:11 会话）= 同症状另一路径：工程带剥列驻留轨行 → 删除后 delta 应用触发 208 守卫（告警亲证）→ legacy 跳 → legacy 未到达内核（ZmqGateway 零记录，取证事实；本腿快回形态无法复现该传输面断链，机制未完全定案）→ dock 静默 return → 同症状。

### 次生问题（r2/r3 双复现）

- **lane catalog 重拉风暴**：delete_plugin 回执 + project_loaded 广播 → 各 lane `refresh_plugin_catalog`/`_refresh_counts_if_expanded` → 同 pid 40ms 内 5 连发 no-wait 描述符请求（队列瞬时 depth 10）。
- **已删插件轮询**：catalog 未刷新前旧 pid 仍被重拉（r2: env_lane_params_1008/1009 旧轨 1007 连发，1009 恒 error；r3: env_lane_params_1055 删除后 error）——取证报告 env_lane_params_1013 形态独立复现。

## 二、修复（三件套，带证据选向）

1. **d（主修）rack 命令族回拉 force_legacy**：`vit_dock_root.gd` `_on_vit_ipc_command_completed_for_dock_repo` 的 rack 结构命令族（instantiate_plugin/rack_add_node/delete_plugin/rack_connect_pins/rack_remove_connection/rack_set_node_clip_scope）回拉 `vit_refresh_project_from_kernel(true)` → `_refresh_project_repository_from_kernel(force_legacy)` 跳过 VSP 两跳直走 store legacy 全量；`project_store.gd` `refresh_project_state(force_legacy)` 跳过 delta 短路。低频命令全量成本可接受（212 时代同族回拉本以 legacy 为主）。**修复后数据：删除时刻+28ms 即 legacy get_project_state ok（ipc_12），画布秒级清除。**
2. **b（保底）回拉终败不静默**：`_refresh_project_repository_from_kernel` 的 reply_error 分支加常态 `push_warning`（原静默 return——用户取证形态 legacy 断链后零告警零信号不可观察）。
3. **c（次生）lane 描述符请求结构面节流**：`track_envelope_lane_2d.gd` `_request_descriptor_for(pid, force)`——非 force 路径加在飞去重（state==pending 跳过）+ 同 pid 500ms 节流；**count 新鲜度面（写回执族 `_COUNT_REFRESH_TRIGGERS` → `_refresh_counts_if_expanded(true)`）强制放行**绕过在飞去重与节流——192 的 ●N 更新语义完整保留（探针 C1/C3 回归修正后全绿）。

未做：方向 a（队列面未发出请求防饿死）——无数据支撑用户形态断链由其造成（快回零堵塞+排队无 timer 事实），避免无证据修复；单在飞+队首 30s TO 的饿死窗口仍在，若用户形态复发可据 [VIT221] 插桩数据再立案。

## 三、验证（退出码全录）

| 面 | 命令/工件 | 结果 |
|---|---|---|
| 窗口正式轮（修复断言，最终形态） | probe_fixed_final_r1/r2.log | **29/29 ×2 连续 exit 0**（P4a/P4b=删除后 repo/canvas 清除 vst1；P4d 子集口径=W 腿 lane 懒建轨级 volume 后 canvas⊆repo 恒真） |
| DIAG 复现轮（基线取证） | probe_diag_r1/r2/r3.log | r1=P1a 探针路径大小写缺陷轮；r2=25 检含签名（已删插件轮询+风暴复现）；r3=29 检含签名（**画布滞留 1055 完整复现**） |
| 212 双向量删除链回归 | regress_212_r1/r2.log | **29/29 ×2 连续 exit 0**（手势+外部向量零回退） |
| 208 守卫族（修复后终态） | probe_vsp_delta_schema_guard | 30/30 exit 0 |
| 207 守卫族 | probe_lane_catalog_fail_loud | 18/18 exit 0 |
| envelope lane 既有族 | probe_envelope_lane_auto/toggle/ui | **25/25 + 27/27 + 36/36 exit 0**（auto 首轮 2 红=节流误伤 C1/C3，force 放行修正后全绿——回归被探针抓住并修正的如实记录） |
| file_reply 体积面 | probe_file_reply_volume_face | 7/7 exit 0 |
| 插桩自检 | probe_221_diag_selfcheck | 10/10 exit 0 |

## 四、纪律与边界

- 三纪律：a) 登记 d8d16709 推送后远端亲读置顶+进程/端口全零复核（<60s 窗口）；b) vsphub 手启 38160 前亲核 8787 空；c) 登记先于启动。
- 探针迭代轮如实留证：r1=P1a 场景路径大小写（探针缺陷）；r2=P4a/P4b FAIL 初判疑团→r3 解开（vst1=1055 真滞留非断言缺陷）；fixed_r1=P1c/P4d 探针口径修正（懒建+子集）——均为探针面，生产面修复一次成型。
- 探针新文件 4 件（stack_probe_rack_delete_gui_refresh.gd/.tscn + probe_221_diag_selfcheck.gd/.tscn）与生产改动 4 文件（160+/18-）均在卡面文件域内，零越域。
- 插桩保留（VIT_221_IPC_DIAG env 门控，默认零输出）——后续用户手测形态若复发，开 env 即得完整时序。
- 前端仓实现**未提交**（卡面指令：不提交不 reset，验收归主管）；主仓协调面（claim e9419676/登记 d8d16709/释放）按协议提交。
- 拆栈：owner 手拆 vsphub 38160/VitAgent 24860/VitApp 35116（三 pid 命令行路径逐一亲核=本腿所属）→ 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。

## 五、上交与残余

1. **用户形态 legacy 传输面断链（未完全定案）**：取证事实=守卫触发后 legacy 未达内核；本腿快回形态无法复现。修复 d+b 已使该路径不再是唯一通道（rack 命令族直走 legacy+失败可见）——若用户形态复发，[VIT221] 插桩（queued/send/reply 全链 print）+b 的 push_warning 会直接暴露断点层。建议用户手测节按 212 收口话术重验（拖插件→参数面板/车道激活→删除→画布消失）。
2. **delta 通道对 rack 域盲**是内核 VSP 面设计事实（node_remove 不产 timeline op）——前端 force_legacy 是适配面；内核侧扩 scope 属域外，交主管裁定是否立项。
3. 单在飞队列饿死窗口（队首 30s TO 请求回复丢失时后续全等 33s/条）——机制在、无触发证据，留档不修。
