# ARR-SRCLOAD-1 (277) 段 3a 重跑腿 seg3a_r2 RUNBOOK — 渲染非静音实证

写于跑前（2026-11-07 10:19，§8 跑前口径成文；跑后不回改本节）。

## 目的（主管段 3a 验收裁定）

seg3a（09:50）已实证装载链四环节（full-access 晋升→chat 免确认卡直执→装载门放行
Z2/instance_ready/processing=true→clip 16 音回读一致），exit 4 定性=驱动器可见性断言
形状缺陷（域内）。断言已修形（`4f636414`：check_rack_visibility 按 track.rack.nodes[]
查询+装载腿 graph diff/实例面增密，只增不减；离线 21/21 绿）。本腿=同场景单 run 重跑，
拿 **start_render→渲染产物非静音（peak_abs>500 且 rms_abs>20）**，段 3 收口。

## 场景（段 3a 逐字复用，零新增改参）

- 预检：`plugin_list_available` is_instrument>0（seg3a 实测 17 件；==0=环境前置 exit 2）。
- fixture：new_project → save_as_project（工件目录）→ add_track（唯一轨）。
- full-access 改参（段 3a 同款）：shadow 绑定等待（223 先例 wait_shadow_tracks(1)）→
  `POST /agent/authority {"authority_mode":"full_project_access"}` → GET 复核；
  chat 请求体零 authority 断言。
- chat 单回合（默认引擎计费）：话术逐字同段 2/3a「给这条 MIDI 轨加一个钢琴音源」，
  context={agent_mode:chat}；确认卡兜底（/agent/confirm 一次）保留。
- 断言链（修形后）：executed rack_add_node status=ok + zone_id=Z2 + **rack 容器内部节点
  可见性**（track.rack.nodes[] 按 plugin_id/rack_item_id 匹配，节点 zone Z2/enabled/
  instance_ready≠false，容器 rack_item_id 与 executed 回执互核；顶层插件行降为证据记录）
  + graph diff 三方一致（node_add node_id/rack_item_id/track_id）+ 实例面
  （load_state enabled/processing/load_error 空）→ insert_midi_clip + add_midi_notes
  （16 拍 C 大调琶音 16 音）回读 → start_render → render_done/文件面 →
  **渲染非静音物理计量：peak_abs>500 且 rms_abs>20**（int16 标度，持续能量线）。

## §8 口径（跑前定死，沿用段 3a）

- **单 run 无重跑**：任何退出码（含 0）后不再第二次启动栈；PASS 也只有存在性证明。
- **预算**：≈¥1-3 上界（一个 chat 回合+轮内少量往返，默认引擎 deepseek-flash 如实
  分账；telemetry 聚合进 summary 作计费证据）。
- **失败分类（py exit code，跑前成文）**：
  - 0=PASS：装载（修形断言）→clip→渲染非静音全链断言过——段 3 收口；
  - 2=环境：栈/传输/fixture/预检缺件（含 is_instrument==0 前置上交）；
  - 3=llm_behavior_miss：模型未发起装载（诚实停泊/无关动作）——如实记录交主管；
  - 4=确定性面缺陷：装载执行但链路违背设计（zone≠Z2/确认后失败/**装载在场仍渲染
    静音**/修形后可见性断言仍失败=断言修形前提被推翻），或 full-access 轮装载仍被
    PCA 门拦（前提推翻）——取证上交不重跑。
- **止损线**：单 run。非 0 即停：2 类记环境证据待重派；3/4 类取证上交主管裁定。
- 计费分账：本腿 LLM 调用全部记入回执（telemetry 行数+token 数）。

## 栈协议（§2.2+2026-10-25 增补）

1. fetch 重读 PC-RUNTIME-STACK.md（origin/main，e28bf8b6=281 10:07 释放在卷）+ 亲核
   进程端口（10:18 六口 TCP listen=0+UDP 4444/4445=0+五类进程=0 全零）；
2. 占用登记（277 seg3a_r2 / owner / 时间）推 main + 重读确认 owner；
3. 登记推送后 ≤60s 内即时复核再启动；
4. agent=wt_277impl 现场构建（-RestartAgent，Go 树自段 2 零改动[断言修 4f636414 仅
   py]，SHA256 记工件）；kernel=wt_228 VitApp.exe SHA256
   F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A
   10:17 亲核只读复用（225-281 同款，C++ 面零改动）；工件目录=本目录（新 run 不覆盖）；
5. finally 拆栈亲核：六口 TCP listen=0 + UDP 4444/4445=0 + 五类进程
   （VitApp/VitAgent/vsphub/Godot/python 驱动）全零；
6. 释放登记+run ID 推 main。

命令（登记完成后执行）：

```
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_277impl/scripts/dev_agent_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_277impl -StartKernel -RestartAgent `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe `
  -ArrSrcLoadSmoke -ArrSrcLoadFullAccess `
  -RunArtifactsDir D:\Vit_DAW_wt_277impl\coord\runs\ARR-SRCLOAD-1\seg3a_r2_20261107_1020
```
