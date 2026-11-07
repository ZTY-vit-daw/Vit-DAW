# ARR-SRCLOAD-1 (277) 段 3a 取证证据链 — seg3a_20261107_0950

run：2026-11-07 09:53:03–09:55（单 run，无重跑，§8 口径见本目录 RUNBOOK.md，跑前成文）。

## 1. run 身份

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_277impl/scripts/dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_277impl -StartKernel -RestartAgent -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -ArrSrcLoadSmoke -ArrSrcLoadFullAccess -RunArtifactsDir D:\Vit_DAW_wt_277impl\coord\runs\ARR-SRCLOAD-1\seg3a_20261107_0950`
- py exit **4**（fail_phase=clip，exit_class=command_defect）；PS1 在 py 非 0 后 throw（exit 1 预期映射）。
- 被测 agent：wt_277impl 现场构建 `agent/bin/VitAgent.dev-smoke.exe`，构建时间 09:53，
  SHA256 `62F444B7F1F817492DF96FE765CA12FF30CBF1450F116750F6928EFEAC2D28B3`
  （Go 树自段 2 后零改动，与段 2 构建同哈希——如实记录）。
- kernel：wt_228 `VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`
  启前 09:51 亲核一致，只读复用，C++ 面零改动。
- 占用登记 `f325d341`（09:52 推 main，重读确认）；≤60s 复核 09:53:03 全零后启动。

## 2. 链路逐环节（全链原始工件在本目录）

| 环节 | 结果 | 证据 |
|---|---|---|
| 预检 | PASS：994 插件 / 17 件 instrument | summary.phases.precheck |
| fixture | PASS：track 1007，工程存工件目录 | summary.phases.fixture |
| **full-access 晋升** | PASS：baseline=manual_confirmation → POST /agent/authority → full_project_access，persisted=true，GET 复核一致 | summary.phases.authority；agent_last.log:132 `[authority] switch accepted mode=full_project_access workspace_ready=true` |
| chat 单回合 | PASS：stop_reason=done / goal_status=completed / **needs_confirmation=false（无确认卡直执）**，reply=「已成功加载插件「Grand Rhapsody Piano Stereo」。」，log_delta_tools=[plugin.semantic_search, plugin.search, rack.add_node] | summary.phases.chat；chat_response.json |
| **装载（本腿核心）** | **PASS：rack.add_node status=ok，zone_id=Z2，plugin_id=1013，plugin_instance_ready=true，load_state enabled=true processing=true load_error 空** | summary.phases.load；chat_response.json executed[2]；agent_last.log:170 `harness.invoke total_ms=5368 tool=rack.add_node status=ok confirmed=false` |
| 写 clip | PASS：clip 1014，16 音写入+回读一致 | summary.phases.clip |
| 装载可见性断言 | **FAIL（py exit 4 的直接触发点）**：track.plugins 顶层=[New Rack, Volume & Pan Plugin, Level Meter]，不见钢琴名 | summary.fail_reason |
| start_render / 非静音 | **未到达**（断言先行中止） | — |

模型组调原文（journal args_summary）：`rack_add_node` 带
`plugin_identifier=VST3-Grand Rhapsody Piano Stereo-695648d-a6dc4e7d`、
`plugin_path=C:\Program Files\Common Files\VST3\WaveShell1-VST3 17.1_x64.vst3`、
`track_id=1007`、`zone_id=Z2`（identifier/path/track 全精确，与语义库条目一致）。

## 3. exit 4 根因：驱动器断言形状缺陷（域内），非装载失败

- 内核回执（chat_response.json executed[2]）：`rack_item_id=1012`、`plugin_item_id=1013`、
  graph_recent_changes=`node_add node_id=1013 rack_item_id=1012 track_id=1007
  "Added rack node Grand Rhapsody Piano Stereo"`、graph_active_revision=1、
  state_delta.changed_tracks=["1007"]。
- 即：rack_add_node 把音源装为**轨上 rack 容器（'New Rack'）内部的节点**；track.plugins
  顶层列表只列容器与内建插件，这是内核的预期数据形状。
- 段 2 遗留断言（`plugin_name in track.plugins[].name`）按「顶层插件行」书写，段 2 因
  装载门拒绝从未执行到该行，本 run 首次触达即暴露形状错误。
- 装载本身为真：instance_ready=true + processing=true + enabled=true + load_error 空 +
  图 diff 在卷。**物理装载环节实证成立；渲染非静音环节因断言中止未获证。**

## 4. admission-path（授权面如实分账，处置归 273）

- 跑前+跑时双证据：语义库 17 件 instrument；晋升库 v1=116 / v2=89 条全为效果器族；
  **instrument identifier 精确命中 0/17**（summary.phases.authority.pca_promoted_instrument_inventory）。
- 装载门 full-access 回落（authorizeFullProjectAccessLoad）只认晋升 identity → 本 run
  装载放行**只能**经门内 certification-token 回落（authorizeCertificationTokenLoad，
  运维 token `~/.vit/pca_certauth.token` 在场=armed）——门自身的既设路由，非越权绕门；
  agent 日志零行 pca_load_gate 拒绝（未被拦）。
- 行为面：prompt 在场 PCA autonomy 附录（3 轮均含，全文「any other identifier is
  rejected」+效果器 identifier 列表，logs/agent_prompt_render.jsonl）+ arrangement pack
  装载链引导；模型按用户明确意图（钢琴）走 semantic_search type=instrument → 精确
  identifier 组调，未被附录劝退。

## 5. §8 兑现与计费

- 单 run 无重跑：兑现（py exit 4 后未二次启动栈）。
- 失败分类：按 RUNBOOK 跑前预写 exit 4=确定性面缺陷类 → 取证上交不重跑；本文件即取证。
- 计费：5 行 deepseek-flash（默认引擎；semantic_entry 1 + message_loop 3 +
  message_loop_repair 1），input≈46.6k tok（其中 cache-read≈20.1k）、output≈1.2k tok，
  **≪¥0.1**（预算 ≈¥1-3 内；与段 2 同引擎同量级）。telemetry 原始行在本目录 logs/。

## 6. 拆栈（owner 亲拆+亲核）

- PS1 在 py 非 0 throw 后中止，栈残留 VitAgent 45128 + VitApp 19676（09:54:59 亲核发现）；
  owner 09:55:08 亲拆两进程。
- **TEARDOWN_ALL_ZERO 亲核 09:55:23**：六口 TCP（7878/8787/5555/5556/4444/4445）listen=0
  +UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 五类进程全零。

## 7. 工件清单（本目录）

RUNBOOK.md（跑前 §8）/ arr_srcload_summary.json / chat_response.json /
arr_srcload_console.txt / ps1_console.txt / agent_last.log（自 Workspace/Logs 复制）/
logs/agent_llm_telemetry.jsonl / logs/agent_prompt_render.jsonl /
arr_srcload_fixture_project.vit / default_project.xml / gui_project_isolation.json /
.vit_agent/（journal）/ .vit_history/（会话图）——后两者为运行时状态，按仓库 .gitignore（**/.vit_agent/ 等）在盘保留不入库。

## 8. 交主管裁定项（非执行侧自决）

1. **exit 4 定性**：装载环节已实证成立（Z2+instance_ready+processing=true），失败点=段 2
   遗留可见性断言形状错误（驱动器域内）。装载可见性的正确断言形状（rack 节点查询）与
   「断言修形后是否再派一跑拿渲染非静音实证」由主管裁定（§8 单 run 纪律下执行侧不重跑）。
2. **admission-path**：本机 instrument 全未晋升，full-access 装载放行实经 certification-token
   回落。「通路现成」表述按此修正：物理通路=现成（本 run 实证）；授权形态=cert-token
   回落（既设路由但语义为 static_eq 认证通道），治本归 273。
3. 段 3 收口判定：渲染非静音未获证，段 3 未收口；卡留 doing/。
