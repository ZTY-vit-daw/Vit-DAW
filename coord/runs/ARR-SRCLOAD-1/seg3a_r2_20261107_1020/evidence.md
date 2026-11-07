# evidence.md — ARR-SRCLOAD-1 (277) 段 3a 重跑腿 seg3a_r2

run `seg3a_r2_20261107_1020`（写于 2026-11-07 10:30，取证上交主管；本文件跑后取证，判读与 RUNBOOK §8 跑前口径对账）。

## 0. Run 身份

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_277impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_277impl -StartKernel -RestartAgent -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -ArrSrcLoadSmoke -ArrSrcLoadFullAccess -RunArtifactsDir D:\Vit_DAW_wt_277impl\coord\runs\ARR-SRCLOAD-1\seg3a_r2_20261107_1020`
- 退出码：**py exit 4（fail_phase=render）→ PS1 exit 1（预期映射，ps1:9034 throw）**。
- 占栈协议：占用登记 `6f2f1ec5` 10:21 推 main+fetch 重读 owner 确认 →≤60s 复核 10:20:37 ALL_ZERO → 10:20:5x 启栈 → PS1 throw 后 owner 亲拆残留 VitAgent 20700/VitApp 40760 → **TEARDOWN_ALL_ZERO 亲核 10:29:42**（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 全零）；释放登记随本腿推 main。
- 被测基线：worktree `D:/Vit_DAW_wt_277impl` @ `port/arr-srcload-1` HEAD=`014dab79`（断言修 `4f636414` 在 HEAD；本腿源码零改动——git status 仅工件未跟踪）；agent=ps1 现场构建 SHA256 `62F444B7F1F817492DF96FE765CA12FF30CBF1450F116750F6928EFEAC2D28B3`（Go 树自段 2 零改动，与 seg2/seg3a 同哈希如实记）；kernel=wt_228 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A` 10:17 亲核只读复用（mtime 2026-10-30 22:23 未变）。
- 计费（telemetry 亲读）：**4 行 deepseek-flash**——strip-silence chat 子腿 1 行（dev_strip_silence_chat_20261107_102102，706 in/910 out/512 cache；281 先例已申报该默认子腿计费）+ 本场景 3 行（semantic_entry 688/195/512 + message_loop×2）；总量级 ≪¥0.1，预算 ≈¥1-3 内。summary 内 calls=3 为本 conversation 聚合（不含子腿），如实分账。

## 1. 结果判定（对账 RUNBOOK §8 预写）

**exit 4 @render=「装载在场仍渲染静音」分支命中（跑前预写分类④的明列子分支），取证上交不重跑；单 run 已兑现（exit 4 后未二次启动栈）。**

链路逐段（较 seg3a 前进到最后一断言）：

| 环节 | 结果 |
|---|---|
| 预检 | PASS：994 插件 17 件 instrument |
| full-access 晋升 | PASS：baseline=manual_confirmation→POST /agent/authority→full_project_access persisted=True+GET 复核；PCA 晋升库存跑前实测 instrument 0/17（v1 116/v2 89 全效果器） |
| chat 单回合 | PASS：stop_reason=done/goal completed/needs_confirmation=False（免确认卡直执）；模型 3 工具 plugin.semantic_search→plugin.search→rack.add_node；reply「已成功加载插件『Grand Rhapsody Piano Stereo』。」 |
| 装载（断言修形后首触） | **PASS**：rack.add_node status=ok zone=Z2 plugin 1013 rack_item 1012 track 1007；**修形断言全过**——rack_visibility matched_by=id（track.rack.nodes[] 节点 zone Z2/enabled/instance_ready≠false）+容器 rack_item_id 互核+graph diff node_add 1013/1012/1007 三方一致+实例面 load_state enabled=true processing=true load_error 空（`4f636414` 真栈生效，seg3a exit 4 缺陷闭合） |
| clip | PASS：clip 1014 写入+16 音回读一致 |
| start_render/非静音 | **FAIL @render**：job `780f1b51191f4e7eb29bee60dfbe00b5` SUB `render_done`（非 fallback）→wav 8.0s **数字静音**（§2） |

admission-path：与 seg3a 同——instrument 晋升 0/17，放行必经门内 cert-token 回落（agent 日志 pca_load_gate 0 行），授权面归 273 不变。

## 2. 渲染静音物理证据（独立核验）

- wav `arr_srcload_render.wav`：2ch/44100 Hz/16-bit/**352800 frames=8.0 s**（=16 拍@120BPM=edit 长度——缺省 range 覆盖全 clip）。
- python wave 模块逐字节核验：数据 1,411,200 字节**全零（nonzero_bytes=0）**，data sha256 `7a9c38df5318a026…`；驱动器计量 peak_abs=0/rms_abs=0.0（-120 dBFS）。wav 因 .gitignore:48 `*.wav` 留盘本目录不入库（前例「大件留盘」口径）。
- **8.0 s 长度本身证明活 edit 在场内容**：endSec=edit->getLength()（TransportAudioService.cpp:1198-1221 缺省 range）；且 Leg B 门 `editHasRenderableAudioContent`（VitProductionCoordinator.cpp:62-89）放行要求「MIDI clip+轨上 enabled takes-MIDI 插件」同时在场——空 edit 应拒绝 `no_renderable_audio_content` 或长度 0。
- 采样率注记：wav 44100（params.sampleRateForAudio 取设备现行速率，TransportAudioService.cpp:868-872 附近逻辑）；工程 audio_settings render_default_sample_rate_hz=48000——速率差不构成静音成因（速率错配致变速而非全零），仅记录。

## 3. 活栈只读探针（拆栈前 10:24–10:26，零 LLM，非重跑）

get_project_state（ZMQ 直调，同驱动器 fixture 通道）track 1007：

- 轨标量面全干净：**mute=false/solo=false/fader_db=0.0/gain 0dB/pan 0**、output_device_id=out_b7a4b4c1、output_routing_name=(default audio output)、**output_to_device_resolved=true**、routing_bus_enabled=false。
- rack（rack_item_id=1012）图模型自证全链在场：
  - node 1013 Grand Rhapsody Piano Stereo：enabled=true/**plugin_instance_ready=true**/plugin_load_state=ready/zone_id=Z2/clip_scope=track/template_role=instrument/**audio_reachable_from_rack_input=true**/**vit_effective_in_output_path=true**/shared_by_clip_ids=[1014]（VST3 路径 WaveShell1-VST3 17.1）。
  - edges 4 条：`RACK_INPUT|0→1013|0`（**MIDI 针脚，pin0**）、`1013|1→RACK_OUTPUT|1`+`1013|2→RACK_OUTPUT|2`（音频 L/R）、`1013|0→RACK_OUTPUT|0`（MIDI out）——**MIDI 入→乐器→音频出接线在内核图模型中完整**。
  - clip 1014：midi，0–8.0 s。
- 结论：**「图模型接线完整可达」与「离线渲染输出全零」并存**——轨静音/mute/路由标志与 rack 接线均已排除，缺陷面收窄到**离线渲染执行路径本身**（内核 C++ 域）。

## 4. 内核源锚点（只读勘察，零改动）

- 渲染链：`TransportAudioService.cpp:1185-1223` handleStartRender（file_path+bit_depth→jmax(16,·)；缺省 range=[0, edit 长度]；useMasterPlugins 默认 true）→ `VitProductionCoordinator.cpp:811-` startOfflineRender → `te::EditRenderer::render(params)`（:875）。
- Leg B 门：`VitProductionCoordinator.cpp:62-89` editHasRenderableAudioContent——只核插件能力位（takesMidiInput/producesAudioWhenNoAudioInput）×clip 类型在场，**不核离线图实际合成**（本 run 证其充分性缺口：门放行≠出声）。
- 装载接线：`PluginRackControlService.cpp:3530-3655` rack_add_node（RackType::addPlugin autoConnect=true+vitTryChainNewRackPluginSerial）——活栈 edges（§3）证明 MIDI/音频针脚均已接，装载面无罪。
- 渲染静音归因候选（**均内核域，本卡零内核改动红线内不修**，交决策侧定扩域/开卡）：
  1. `te::EditRenderer` 离线图构建未把轨上 MIDI clip 事件馈入 rack 容器 MIDI 输入（离线 vs 实时播放图的 MIDI 路由差异——tracktion 引擎源非本仓 vendored，深挖需内核取证腿）；
  2. 离线渲染路径中外链 VSTi 的处理实例/处理使能（processing=true 语义是否覆盖离线图）；
  3. rack 输出→轨输出→master 的离线合路面（useMasterPlugins=true 链）。
- **次级观察（持久化面，单独记录不扩修）**：主 .vit 文件 mtime=10:21:11.245（fixture 步 save_as/add_track 后），此后 rack_add_node（10:21:34）与 clip/notes（10:21:34–35）**未回写主 .vit**——journal checkpoint `c_20261107T022134_780dd7cd revision=4`（agent 日志 10:21:34）落 .vit_history 而主文件停留 fixture 态（无 rack 无 clip；`<RACKS/>` 空+track 1007 仅 volume/level）。渲染输入=活 edit 不受影响（§2 长度证明），但**重开该 .vit 将丢 rack+clip**——持久化时序/回写策略交决策侧知悉（.vit_agent/.vit_history 运行时目录按 gitignore 留盘本目录）。

## 5. 交主管裁定项

1. **exit 4 定性与处置**：卡面停止条件「渲染静音且排查超本卡域→取证上交」已触发——装载/接线/轨标志三层排除后缺陷面收窄至内核离线渲染路径（§3/§4）；选项：内核取证/修复卡（新卡）vs 277 以「装载链四环节+渲染链到 render_done」收口交主管裁量。
2. **断言修 4f636414 采认**：修形断言（rack.nodes[] 可见性+graph diff+实例面）真栈首次触达即 PASS（§1 装载行）。
3. **次级观察 .vit 主文件不回写**（§4 末）：是否开持久化面卡。

## 6. 工件清单（本目录，随 port/arr-srcload-1 入库；wav/.vit_agent/.vit_history 留盘不入库）

RUNBOOK.md（跑前 §8）/evidence.md（本件）/ps1_console.txt（PS1 全程）/arr_srcload_console.txt（py 控制台）/arr_srcload_summary.json（全链判定+计量）/chat_response.json（chat 回合原始）/arr_srcload_fixture_project.vit（fixture 态主文件，§4 次级观察证据）/default_project.xml+gui_project_isolation.json（隔离面）/logs/（telemetry 4 行+prompt_render）/arr_srcload_render.wav（留盘）/.vit_agent+.vit_history（运行时，留盘）/agent 日志=worktree `VitApp/Workspace/Logs/agent_last.log`（未拷贝，259 行在盘可回指）。
