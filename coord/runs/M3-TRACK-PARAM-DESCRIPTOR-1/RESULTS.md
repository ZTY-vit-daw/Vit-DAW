# M3-TRACK-PARAM-DESCRIPTOR-1（201）真栈腿回执

- 卡：`coord/cards/done/2026-10-27-M3-TRACK-PARAM-DESCRIPTOR-1.md`（池序 201，B 案内核卡）
- owner：ZCode GLM-5.3 PC 执行会话（201 卡会话）；领取=8b72ba73（todo→doing，基线 9671c1b1）
- 实现 commit：`port/m3-track-param-descriptor-1` @ **e9a49fd0**（内核+冒测扩展，3 文件 +359/-2）
- 占用登记：d71315ff（coord/m3-track-param-descriptor-1-stack → main）；登记行时戳写 15:11 系预估偏差，实际机器钟 15:06-15:07（立即复核 15:06:50 全零，ps1 启动 15:07:17），释放行以机器钟为准
- run：`coord/runs/M3-TRACK-PARAM-DESCRIPTOR-1/{trackparam_20261027_r1, trackparam_20261027_r2}`（本目录；工件随 port 分支入库）

## 一、判定面（验收标准逐条）

1. **内核构建 exit 0**：worktree D:/Vit_DAW_wt_tpdc1 自建（cmake -S VitApp -B VitApp/build -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine 只读复用；Release×14 并行），`VitApp.exe`（31.7MB，2026-10-27 15:02）链接成功；仅引擎侧 harfbuzz C4819 代码页警告（既有、与本卡无关）。
2. **真栈冒测两轮全绿（连续）**：
   - R1（trackparam_20261027_r1）：ps1 **exit 0**——`-TrackParamDescriptorSmoke`（201 新腿）**42/42 checks PASS**+`-TrackSourceProbe`（196 面回归）PASS（volume anchor 2/2+full cycle+crosscheck）+`-EnvelopePresenceSmoke`（190 面回归）PASS（13 检全绿，含**外部插件行 diff 级零改动**：唯一披露差异=目标行 envelope_point_count 0→3）。
   - R2 首次尝试：**188 守卫活触发拦停**（R1 内核 pid 29836 残留监听 5555 → kernel start 步显式 throw，拒绝静默复用；console 留证 `trackparam_20261027_r2_attempt1_guard_stop.txt`）。非产品轮（零产品断言执行），分类=执行侧拆栈时序缺口（R1 完成后未先拆栈即起 R2）；守卫行为=188 设计面正确工作。残留归属亲核（路径=D:\Vit_DAW_wt_tpdc1 自建 exe+启动时刻 15:07:27=R1 -StartKernel）后 owner 手拆→进程/端口复核全零。
   - R2（trackparam_20261027_r2，干净重放）：ps1 **exit 0**——三腿同 R1 全绿。
   - 稳定性口径达成：连续两个有效产品轮全绿（R1+R2），非单次 exit 0。
3. **201 新腿覆盖面**（`--probe-descriptor-tracks 2`，断言体=automation_curve_smoke.py probe_track_param_descriptors，42 checks）：
   - 基线：双轨各 volume 插件 `get_plugin_parameters` 回 **ok**（旧 error「Plugin parameters are only available for external plugins」对 type=volume 消失=已裁定契约变更）；**恰 volume/pan 两行**（sorted ids==[pan,volume] 硬断言——master/trim/mute/gain/Level Meter 不虚列，与 196 resolve 可寻址面镜像）；行 schema 全字段在场：id/name/value_text/min/max/display_probe/envelope_point_count；**值域=引擎声明面**（volume 0..1 fader 位、pan -1..1）；value_text 单位披露在场（`+0.00 dB`/`Centre`）；display_probe 采样梯度（volume→-100.00/-21.72/-7.86 dB…；pan→-1.000 Left…Centre…）；envelope_point_count 全零。
   - 生命周期：set_points 三点（track#0 volume+track#1 pan）→ 描述符 count 跟随（两轨各目标行=3）+**跨轨隔离**（track#1 volume 保持 0 / track#0 pan 保持 0）+**同源对照**（描述符 count==automation.get_curve point_count 逐参一致）→ clear_region → 全零回基线。
4. **外部插件行零改动（diff 级）**：190 回归腿在装载 ValhallaSupermassive 的外部插件上跑既有断言面（existing_fields.zero_change.diff_level PASS）——外部行既有字段零漂移，唯一差异=目标行 envelope_point_count（190 已裁定的列）。
5. **消费方全量回归盘点**（§二）；**端测边界声明**（§三）。

## 二、消费方全量回归盘点（目标 3）

逐面盘点+只读回归，**零消费方改动**（agent/ 前端仓零改，卡面文件域守住）：

| 消费面 | 盘点结论 | 依赖旧 error 语义？ |
|---|---|---|
| agent 工具面（message_loop plugin.get_parameters 分发+harness publicPluginParametersResult 投影） | 通用透传投影（缺字段→空）；行形状消费=id/param_id/raw_param_id+value（c3 注释点名），全部在镜像行集内；display_probe 有 DisplayProbeSummary 消费→行内已含 | 无 |
| agent 观察/PCA/grabber 工作流（plugin_grabber_*.go、plugin_*_apply/inspect.go、goalrunner capabilityBoundaryLiveParams、c3_space_depth） | 全部定向外部插件 plugin_id（eq/delay/de-esser/limiter/multiband/grabber 装载流）；轨级 volume 插件 id 不进入这些流；外部插件回复路径零改动 | 无 |
| 前端 rack 层（Vit_Graph_Rack 参数口行、rack_ipc_payload_rules、rack_node_display_rules） | rack 目录=rack DAG 成员（外部插件）；轨级 volume 插件非 rack 节点（185 实证轨级 pluginList 只呈现 rack 容器）→面不触达 | 无 |
| 前端车道层（track_envelope_lane_2d.gd 目录扫描） | 目录扫描对象=rack 插件（同上）；若未来纳入轨级插件，ok+rows 为超集（present 代替 error），方向=202 重设计预期 | 无 |
| 既有冒测 196 面（automation_curve_smoke --probe-tracks） | 描述符探针为**条件断言**（仅 reply 为 error 时校验措辞含 external）；回复转 ok 后自然跳过、逐字留证不失败——**真栈回归 R1/R2 实证 PASS** | 无（措辞锚点仅 error 分支） |
| 既有冒测 190 面（envelope_presence_smoke） | 外部插件腿，与轨级路径无交集——真栈回归 R1/R2 实证 PASS | 无 |
| 既有冒测 189 面（probe_envelope_lane_ui.gd mock 探针） | mock 驱动（合成 reply 注入），不触真内核——不受影响 | 无 |
| 189 真栈探针（tools/stack_probe_envelope_lane.gd） | L1 get_plugin_parameters 直调=纯留证（`_record(..., true, ...)` 恒过），无 error 断言 | 无 |
| 192 面（stack_probe_envelope_lane_auto） | 目录/rack 面=外部插件（装载 ValhallaSupermassive）；R1/R2 未跑 Godot 腿（见 §三 边界声明） | 无 |

**结论：未发现依赖旧 error 语义的硬依赖**；「多出的轨级行」影响=纯增量（AGENTS §11 口径：未知消费面忽略新行即可）。

## 三、端测边界声明

- **本腿覆盖**：内核 ZMQ 命令面真栈（新腿 42 检+196 回归+190 回归，连续两轮）；agent 由 ps1 最小腿拉起（构建+重启+健康面）。
- **本腿不覆盖**：webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 两层就位前的既有边界）；Godot 前端腿未跑（202 重设计卡域；本卡「前端仓零改」口径下，189/192 探针面的静态盘点结论=零 error 硬依赖，未做 GUI 级回归）；agent LLM 参与面零轮（无 LLM 依赖，§8 记账：LLM=0）。
- 冒测执行形态：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -TrackParamDescriptorSmoke -TrackSourceProbe -EnvelopePresenceSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <worktree 自建> -RunArtifactsDir <run 目录>`（既有 ps1 体系扩展，未另起机制）。

## 四、三纪律与隔离

- 登记-启动：d71315ff 推送后 15:06:50 即时复核进程/端口全零（距 ps1 启动 15:07:17 <60s 窗口）；登记先于启动。
- already listening：R2 首次尝试守卫活触发（见 §一），无静默复用；R2 干净重放全绿。
- VIT_PROJECT_XML 隔离：ps1 GUI 腿隔离块自动 run 目录 git-clean 副本（marker gui_project_isolation.json：source=git HEAD e9a49fd0）；副本被内核写入（隔离生效）；worktree VitApp/Workspace 运行前后 git 干净；主检出 default_project.xml E7B19859（mtime 14:01:21=本腿前既有值，零触碰）。

## 五、工件清单

- `trackparam_20261027_r1/`：track_param_descriptor_summary.json（42 checks+逐字回执）/track_param_descriptor_console.txt/track_param_descriptor_project.vit/track_source_probe_summary.json+console/（196 回归）/envelope_presence_summary.json+console+project.vit/（190 回归）/default_project.xml+gui_project_isolation.json（隔离面）
- `trackparam_20261027_r2/`：同构全套+`trackparam_20261027_r2_attempt1_guard_stop.txt`（守卫拦停轮 console 留证）
- `RESULTS.md`（本文件）
