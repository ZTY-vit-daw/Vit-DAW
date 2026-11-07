# ARR-SRCLOAD-1 (277) 段 3a RUNBOOK — D 路径 full-access 出声实证腿

写于跑前（2026-11-07 09:50，§8 跑前口径成文；跑后不回改本节）。

## 目的（主管段 2 裁定①）

复用 arr_srcload_smoke 驱动器，chat 话术走 **full-access promoted 轮**（261 G5
ensure_full_project_access 先例=服务端 `/agent/authority` 控制面切换，非请求体断言），
拿「装载过门→写 clip→渲染输出非静音」整链物理实证。授权面形态另案 273（B 治本在飞）。

## 场景（段 2 骨架不变 + 一处改参）

- 预检：`plugin_list_available` is_instrument>0（段 2 实测 994 插件 17 件 instrument；
  ==0 按卡面停止条件=环境前置上交 exit 2）。
- fixture：new_project → save_as_project（工件目录）→ add_track（唯一轨）。
- **改参（本段唯一新增）**：shadow 绑定等待（AUTHORITY-LOST-1：identity 重绑竞态，
  223 先例同款 wait_shadow_tracks(1)）→ `POST /agent/authority
  {"authority_mode":"full_project_access"}` → GET 复核。chat 请求体零 authority 断言
  （bindChatAuthorityMode 自动盖服务端状态对，伪造面不存在）。
- chat 单回合（默认引擎计费）：话术与段 2 逐字相同「给这条 MIDI 轨加一个钢琴音源」，
  context={agent_mode:chat}；确认卡兜底（段 2 同款 /agent/confirm 一次）保留。
- 断言链：executed rack_add_node status=ok + zone_id=Z2 + 装载插件在轨可见 →
  insert_midi_clip + add_midi_notes（16 拍 C 大调琶音 16 音）回读 → start_render →
  render_done/文件面 → **渲染非静音物理计量：peak_abs>500 且 rms_abs>20**
  （int16 标度；单发杂散 click rms≈6 过不了 RMS 线——「出声」必须是有持续能量的音频）。

## 跑前静态链核对（写入跑前，防事后倒推）

- authority 链全静态核实：`/agent/authority` POST（唯一授权面）→ server.authorityMode →
  handleChat `bindChatAuthorityMode`（server.go:2577，空断言=盖服务端状态+explicit 对）→
  agentloop `executeTool` Context=cloneMap(input.Context)（runner.go:541，Source="agentloop"
  非空→装载门生效）→ harness `enforceAgentProcessorLoadGate` full-access 回落
  `authorizeFullProjectAccessLoad`（harness.go:2242-2258/2417）。
- **PCA 晋升库存跑前实测（py 同款只读扫描，2026-11-07 09:47）**：语义库 17 件
  instrument；晋升库 v1=116 条 / v2=89 条全部为效果器族（static_eq/broadband_compressor/
  limiter/reverb/…），**instrument identifier 精确命中=0/17**。装载门 full-access 回落
  只认晋升 identity → 本机 instrument 全部未晋升。
- 推论（预写两种可判形态，跑后按实归类，不倒推）：
  - 若模型组调 rack.add_node 且装载成功：装载门对本机 instrument 的放行**只能**经门内
    certification-token 回落（authorizeCertificationTokenLoad；运维 token
    `~/.vit/pca_certauth.token` 在场=armed）。这是装载门自身的既设路由，非越权绕门；
    py summary 已固定记录该 admission-path 推断+晋升库存快照，回执如实分账。
  - 若装载门仍拒绝（pca_load_gate 原文出现在 log/reply）：=推翻「通路现成」前提 →
    exit 4 取证上交（py 已定向捕获 pca_load_gate 拒绝原文行）。
  - 模型行为面（PCA autonomy catalog 附录称"any other identifier is rejected"与
    arrangement pack 装载链引导之间的取舍）是本 run 真正观察对象之一：若模型因此
    诚实停泊不发起装载 → exit 3 如实记录。

## §8 口径（跑前定死）

- **单 run 无重跑**：任何退出码（含 0）后不再第二次启动栈；PASS 也只有存在性证明。
- **预算**：≈¥1-3 上界（一个 chat 回合+轮内少量往返，默认引擎如实分账；telemetry
  按 conversation 聚合进 summary 作计费证据）。
- **失败分类（py exit code，跑前成文）**：
  - 0=PASS：装载→clip→渲染非静音全链断言过；
  - 2=环境：栈/传输/fixture/预检缺件（含 is_instrument==0 前置上交）；
  - 3=llm_behavior_miss：模型未发起装载（诚实停泊/无关动作）——模型行为如实记录；
  - 4=确定性面缺陷：装载执行但链路违背设计（zone≠Z2/确认后失败/装载在场仍静音渲染），
    **或 full-access 轮装载仍被 PCA 门拦（前提推翻）**——取证上交不重跑。
- **止损线**：单 run。非 0 即停：2 类记环境证据待重派；3/4 类取证上交主管裁定
  （3 类同 face 是否触发前提推翻由主管判，卡面不预写结论）。
- 计费分账：本腿 LLM 调用全部记入回执（telemetry 行数+token 数）。

## 栈协议（§2.2+2026-10-25 增补）

1. fetch 重读 PC-RUNTIME-STACK.md（origin/main）+ 亲核进程端口；
2. 占用登记推 main（卡 277 段 3a / owner / 时间）+ 重读确认；
3. 登记推送后 ≤60s 内即时复核再启动；
4. agent=wt_277impl 现场构建（-RestartAgent，SHA256 记工件）；kernel=wt_228
   VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A
   只读复用（225-281 同款，C++ 面零改动，启前亲核哈希）；工件目录=本目录（新 run 不覆盖）；
5. finally 拆栈亲核：六口 TCP listen=0 + UDP 4444/4445=0 + 五类进程
   （VitApp/VitAgent/vsphub/Godot/python 驱动）全零；
6. 释放登记+run ID 推 main。

命令（登记完成后执行）：

```
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_277impl/scripts/dev_agent_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_277impl -StartKernel -RestartAgent `
  -KernelExe <wt_228 VitApp.exe 绝对路径> `
  -ArrSrcLoadSmoke -ArrSrcLoadFullAccess `
  -RunArtifactsDir D:\Vit_DAW_wt_277impl\coord\runs\ARR-SRCLOAD-1\seg3a_20261107_0950
```
