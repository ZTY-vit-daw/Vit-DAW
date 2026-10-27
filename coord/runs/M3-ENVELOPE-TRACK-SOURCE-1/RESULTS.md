# M3-ENVELOPE-TRACK-SOURCE-1：轨级参数包络在场性数据源探测——RESULTS

- 卡：M3-ENVELOPE-TRACK-SOURCE-1（池序 196）/ owner=PC 执行会话（GLM-5.3，ZCode，196 卡会话）
- 日期：2026-10-27（登记 12:28 / 正式轮 12:32–12:3x / 释放 8ae4ea2b）
- 结论一句话：**轨级曲线面 volume/pan 双参数全周期可用**（双轨 add_track×2 下逐参 空披露→set_points 三点→逐点回读一致→跨轨 plugin_id 隔离→clear_region→回空，全部真栈实证）；字面 `master volume`/`master pan` 与 trim/mute/gain 及 Level Meter 插件全参数族不可寻址（逐字留证）；descriptor 面 error 文案与 189 逐字一致。

## 一、Run 元数据（AGENTS §9 工件契约）

- run ID：`coord/runs/M3-ENVELOPE-TRACK-SOURCE-1/tracksrc_20261027_1232`
- 完整命令（正式轮，exit 0）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 `
  -RepoRoot "D:/Vit_DAW_wt_etsrc1" `
  -StartKernel `
  -KernelExe "D:/Vit_DAW_wt_envp1/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe" `
  -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -AutomationCurveSmoke -TrackSourceProbe `
  -RunArtifactsDir "D:/Vit_DAW_wt_etsrc1/coord/runs/M3-ENVELOPE-TRACK-SOURCE-1/tracksrc_20261027_1232"
```

（worktree D:/Vit_DAW_wt_etsrc1 下执行；探测命令体：`python scripts/automation_curve_smoke.py --url tcp://127.0.0.1:5555 --output <run>/track_source_probe_summary.json --probe-tracks 2 --probe-params "volume,pan,master volume,master pan,trim,mute,gain"`）
- 退出码：**0**（ps1 全腿含 190 回归面 13 checks PASS + 196 探测面 PASS anchor 2/2/2）
- HEAD：worktree=port/m3-envelope-track-source-1 @ f0386343（领取提交；scripts/ 两文件为未提交工作改动，diff 归属本卡）；探测期远端 main 前移至 8ae4ea2b（均为他卡/本卡状态提交，无文件域相交）
- 内核：**190 自建二进制只读复用**（D:/Vit_DAW_wt_envp1@72a4f0ea 构建，sha256 `85B4E8DF38C9F174FDEB6CB9C92FA9446C643FA495798199A01D85945D3A60E3`，与 190 run 回执 85b4e8df 系同源）pid 16256 本腿 ps1 自启；agent=etsrc1 worktree 构建 pid 36668。**本卡 VitApp/ 生产源零改动**（worktree `git status -- VitApp/` 空）
- 资源腿：登记 83adfd32（推送后即时复核进程/端口全零 <60s）→ 释放 8ae4ea2b；拆栈复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0
- VIT_PROJECT_XML 隔离：ps1 GUI 腿隔离块自动副本（188 守卫同源）；run 副本 sha `90AFB63C`（内核写入），worktree VitApp/ 运行前后 git 零改动
- 前置轮留证：`tracksrc_20261027_1229`（首跑 bash 反斜杠路径陷阱轮 + 探针 sibling 行传参缺陷轮，console 工件在案；调试后已拆栈重放正式轮）——缺陷在探针自身，内核/命令面无涉

## 二、探测事实（全部真栈逐字留证，工件=track_source_probe_summary.json 28 行 probe_rows）

### 2.1 插件发现面（get_project_state）

add_audio_track×2 → track_id 1007/1010（自动分配）；每轨插件行恒两枚：
`Volume & Pan Plugin`（type=volume，plugin_id 1008/1011）+ `Level Meter`（type=level，plugin_id 1009/1012）。
**多轨寻址=plugin_id 唯一定位**（无 track_id 维；两轨 plugin_id 互异，跨轨隔离检查实证 A 轨三点在写时 B 轨同参恒 point_count=0）。

### 2.2 可寻址参数族（全周期 ok）：volume、pan

| 面 | volume（189 锚点，硬断言） | pan（本卡新实证） |
|---|---|---|
| 空披露 | `{status:ok, time_base:"time", bypass:false, point_count:0, points:[]}` | 同左（形态一致） |
| set_points 三点 | written=3, mode=full, 一次成功（0.8/0.5/0.2） | written=3, mode=full, **一次成功（-0.4/0.0/0.4，值域覆盖，未触发重试）** |
| 逐点回读 | 逐点 (t,v,c) 一致（float32 回显 0.800000011920929≈0.8，容差内） | 逐点一致（-0.400000005960464≈-0.4） |
| 跨轨隔离 | sibling 同参恒空 ✓ | sibling 同参恒空 ✓ |
| clear_region [0,5] | removed_points=3 → point_count=0 → 回读空 | removed_points=3 → point_count=0 → 回读空 |

### 2.3 不可寻址族（fail-closed error，逐字）

`get_curve` 对下列 (插件, param) 全部回：

```json
{"status": "error", "message": "Parameter not found for param_id: <param_id>"}
```

覆盖：volume 插件上 `master volume`/`master pan`/`trim`/`mute`/`gain`（5/7 候选）；Level Meter 插件上全部 7 候选。即轨级可寻址参数族**恰为 volume/pan 两个**（与插件名 "Volume & Pan Plugin" 对齐；resolve 链字面 id 直中，master 别名回退未触发/字面 master 形态反不可寻址——内核 resolvePluginParameterByID 行为如实记录，不评设计）。

### 2.4 descriptor 面（B 案前提复核）

`get_plugin_parameters`（track_id+轨级 volume 插件）双轨均回，**与 189 L1 逐字一致**（190 内核上文案未变）：

```json
{"status": "error", "message": "Plugin parameters are only available for external plugins"}
```

→ envelope_point_count 在轨级仍不可达（描述符面拒绝在先），192 自动档轨级缺口在最新内核面依旧成立。

### 2.5 停止条件核查

- 寻址锚点与 189/M3-RECON §2.3 相符（{plugin_id, param_id} 无 track_id、volume 插件发现面同款、descriptor 文案逐字一致）→ 未触发
- set_points 对轨级参数零 error（volume/pan 全 ok）→ 未触发；无需转 B 案证据卡

## 三、数据源两案对比（只裁证据不实施；供主管择一后另开消费实施卡）

**A 案=前端侧轨级 get_curve 探测接入 192 扫描腿（零内核改动）**
- 成本：192 扫描腿对轨级参数直接以 `{plugin_id, param_id∈{volume,pan}}` 调 automation.get_curve（本卡实证全周期可用）；需改 192 消费端轨道枚举→插件发现（get_project_state plugins[] 过滤 type=volume）→逐参 get_curve 的探测编排，纯 agent/前端侧改动。
- 成本（第二行）：探测候选族可钉死 volume/pan 两参（本卡证其余族恒 error），无需泛化探测逻辑；不可寻址回执文案稳定（"Parameter not found"）可作 fail-closed 分类依据。
- 兼容：零内核改动=旧工程/旧内核无版本耦合；get_curve 本身在 legacyReadOnlyCommands 白名单（M3-RECON §3.3 已裁），刷新链内只读调用无 CAS/写风险。
- 兼容（第二行）：192 现有外部插件行路径零触碰（轨级与 rack 插件两数据源并存，描述符面 error 照旧如实披露）。
- 发现时机（刷新链 vs 写回执重扫）：**走刷新链**——get_curve 是只读白名单命令，随 192 每轮刷新即可发现轨级包络在场性（点数>0），无需写动作触发。
- 发现时机（第二行）：代价=每轨两参一次 ZMQ 往返（每轮刷新 N轨×2 次）；无变化检测捷径（靠轮询对比，与 192 既有扫描节奏同构）。

**B 案=内核轨参数面小扩（envelope_point_count 覆盖轨级）**
- 成本：需内核改动——get_plugin_parameters 对轨级内置插件从"拒绝"改为"回参数行"（或新增轨级描述符面）+envelope_point_count 列挂接（190 的 appendEnvelopePointCountsToParameterDescriptors 已有单点先例，但前提是描述符面先放行轨级）；动 kernel + 可能连带 VSP 白名单/签名面（parameterSurfaceSignature 零污染约束 190 已处理，新增面要重新核）。
- 成本（第二行）：需 C++ 构建+真栈回归轮（内核卡成本基线，参照 190 单卡节奏）；"仅外部插件"边界是既有契约文案，改动=契约变更须主管明裁。
- 兼容：改内核行为面=所有消费 get_plugin_parameters 的既有链路（agent 观察/PCA/grabber）回执形态变化风险；旧内核二进制与新 agent 混跑时轨级行缺失（fail-closed 但需口径）。
- 兼容（第二行）：对 192 而言数据源统一在描述符面（单一查询点），但这是用内核契约变更换消费端简化。
- 发现时机（刷新链 vs 写回执重扫）：**天然走刷新链**——描述符列随既有 get_plugin_parameters 刷新即见 point_count，无需独立探测编排。
- 发现时机（第二行）：一次实现全量生效（不依赖每轮逐参 get_curve 轮询）；但生效前提=新内核二进制部署，旧栈上不生效。

**执行侧意见（供参考，不代替裁定）**：A 案零内核改动+本卡已实证全部寻址细节，成本低、可立即开消费卡；B 案统一数据源但动内核契约面。倾向 A。

## 四、端测边界声明（AGENTS §5/§7）

- 本腿=**真实栈内核 ZMQ 命令面直连**（内核+agent 真进程，190 验收链内核二进制；legacy cmd 形态+VSP 封套面由同跑的 190 回归腿覆盖）；未覆盖：Godot GUI 渲染面/用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面依旧）、webui 呈现、轨级包络的 GUI 车道呈现细节（189 已证的 C1 面不在本卡范围）。
- 单 run exit 0=存在一条成功路径（AGENTS §8 口径）；探测断言面锚 volume（189 已证）+pan/隔离为单轮实证，未做重复成功率采样（如需稳定性样本另裁）。
- 候选参数族为在案清单（volume/pan/master volume/master pan/trim/mute/gain）；未列参数 id（若存在）未探测，属如实在案清单边界，非穷举声明。

## 五、工件清单

- `tracksrc_20261027_1232/`（正式轮）：track_source_probe_summary.json（28 行逐字回执）/ track_source_probe_console.txt / automation_curve_summary.json+console（190 回归）/ default_project.xml（隔离副本 90AFB63C）/ gui_project_isolation.json / `../tracksrc_20261027_1232_ps1_console.log`（ps1 全程）
- `tracksrc_20261027_1229/`（前置缺陷轮留证：bash 路径陷阱+探针 sibling 传参缺陷，console 在案）
- scripts/ 改动：automation_curve_smoke.py（探测模式增量，默认路径零改动——同栈 190 腿 13 checks 全绿即回归证据）+ dev_agent_smoke.ps1（-TrackSourceProbe 腿+GUI 隔离条件一行）
