# M2-EXP-TIMEDYN-GAP-1 取证归因件（FORENSICS.md）

- 卡：M2-EXP-TIMEDYN-GAP-1（池序 120，P1）取证腿；执行：GLM-5.3 L2 PC 执行会话（ZCode，本会话）
- worktree：D:/Vit_DAW_wt_m2tg1，分支 port/m2-exp-timedyn-gap-1-recon（基线 origin/main ec8efed9 + 领取 727d1dd8 + 占用登记 622e326e，Go/C++ 零生产改动）
- 真栈 run：m2tg1_20261015_213005（本目录）；被测 agent=worktree 构建 SHA256 1411DD3AAE738FE8（main 内容），内核=主检出 staging VitApp.exe B6565DCF85D1DA86（C++ 面零改动）；模型=deepseek-flash（config 全程未动）
- §8 记账：A 轮 1 次 NL chat（声明 ≤6 调用，实际 7 次：语义入口×1+pullharness×5+message_loop_repair×1，**全部 error='' 0 超时**，超声明 1 次如实登记）；B 轮只读 invoke 零 LLM。止损未触发（1 轮即收敛）。栈拆净（四端口监听=0、进程=0），stems 源素材前后哈希一致（只读纪律守住）。
- 复现边界声明：本 run 未启 Godot GUI，经 agent HTTP 面驱动（invoke+chat）——导入命令与 GUI 导入同走内核 ImportService 分析队列（ImportService.cpp:562-591），观察链全在 agent 侧，GUI 差异仅在导入触发的进程面，不影响本归因结论。

## 0. 结论一览（TL;DR）

**缺陷不是「导入路径不产时域证据」那么简单——时域证据（time_segments）在导入路径上其实采集到了（6/6 轨×4 段，盘面实锤）；真正拦死链路的是「CCB 轨道视图读的是单数目标槽，而导入路径的单数槽绑定到最后一条被填充的轨（vocals/1032），不是请求的目标轨（guitar/1017）」→ 目标轨的 time_energy 视图证据缺失 → omission「source evidence is missing」→ 审计 receipt 从 executed 降级 partial → 语义编排器入口门拒收。** 同一根因还产生了第二个复现缺陷：guitar 视图键下的 stereo 证据实际是 vocals 的素材（track_id=1032）——用户第三轮现场与本次 A 轮都出现，本次 A 轮模型自己通过 source-identity 一致性检查发现并拒用它。

四问速答：
1. `{track}.slow.time_energy.summary` 的 time_segments **只有 agent 侧一个生产者**（harness 波形 tile collector）；内核枚举 AudioFeatureType::TimeEnergy 是死枚举，无任何生产路径；视图读的是观察包 MixPackage.current_metrics.time_energy（来自快照**单数 WaveformEnvelope 槽**，不是 6 轨 track_waveform_envelopes 行）。
2. stereo 有值是因为它是**内核 L3 分析器产物**（导入即焙、可按需重焙、按轨持久），视图读的单数槽恰好装着一个 ready 的 stereo 行（ albeit 错轨的）；time_energy 无值是因为它**不是内核产物**，且单数 WE 槽在观察组装时无可用段。
3. project.structure 的 partial 源=内部 `project.static.summary` status=partial（mixboard.go:3168：**caps["waveform_envelope"]!="ready" 即 partial**——用户现场目标 WE 行 stale）；tim_projection 虽也是 partial（bit_depth/channel_count/sample_rate 0/6）但它是 optional-supplemental，不计入视图降级。
4. 审计门锚点=chat/semantic_progressive_disclosure.go:107-110：receipt 状态必须是 ""/ok/ready/success/**executed**；而 free_state_observation.go:642-647 在**任何** omission_reason 存在时把 receipt executed→partial。bundle 级 partial 是放行的（:97 注释明说 DAD source-only 视图设计上就是 partial），**receipt 级 partial 拦死**——门不区分「证据未生产+诚实收据」与「过期/不一致」。

## 1. 四问逐答（含行号锚点，全部在取证基线 ec8efed9 上核）

### Q1：`{track}.slow.time_energy.summary` 的生产责任方、触发条件与消费路径

**内核侧：无生产者（死枚举）。**
- `AudioFeatureType::TimeEnergy` 定义于 VitApp/Source/Service/AudioFeatureTypes.cpp:11（字符串 "time_energy"，产品版本 time_energy.v1，AudioFeatureTypes.cpp:78），但 VitApp/Source 全树除 AudioFeatureTypes 外零引用（grep 亲证）。
- 导入链每个文件只请求两种特征：WaveformEnvelope（ImportImmediate）+ L3AcousticSummary（BackgroundWarm）——VitApp/Source/Service/ImportService.cpp:562-591。
- L3 分析器只发布四族：band_energy_summary / stereo_relation_summary / loudness_summary / segmentation_primitives——L3AcousticAnalyzer.cpp:1105-1118（publishSummaries）。**l3_feature_log 因此只有 band/stereo/loudness 三族**：用户工程 18 条（6 轨×3），本次 A 轮复刻工程同样 18 条（l3_feature_log.jsonl 亲数：6+6+6）。
- 按需路径 `warm_waveform_bake`（ImportService.cpp:1835-1870，feature_type 参数透传）若被请求 time_energy，落入 AudioFeatureService.cpp:196-208 的 fallthrough → publishDeferredStatus：**status="deferred", reason="feature_baker_not_yet_promoted"**（AudioFeatureService.cpp:121-154）。
- 内核波形包络载荷本身不含 time_segments：WaveformEnvelopeBaker.cpp:387-438 只发布 rms/peak/headroom/crest + tile 计数；VitApp/Source 全树 "time_segments" 零命中。

**agent 侧：唯一生产者=harness 波形 tile collector。**
- 内核按 tile 流式发布波形包络（min/max/rms 浮点对），agent 的 waveformFeatureCollector 每 tile 聚合出一段：harness.go:5495-5544（TimeSegments append @ :5535，字段 start/end/rms/rms_dbfs/peak/crest/energy_state）。
- 行物化：harness.go:5563 SnapshotRow 把 time_segments 连同 rms/peak 写进 waveform_envelope 特征行（source="kernel_audio_feature_data_ready"）。
- 声学包状态：acousticpackage/status.go:359（BuildStatus l1_static 层）→ timeEnergyFromWaveform（status.go:871-888）：**行里有 time_segments 才 ready；waveform ready 但无段 → partial "waveform_ready_without_time_segments"**；无波形 → missing "waveform_envelope_required_for_time_energy"。
- 观察触发面：mix.observe 读到 missing/stale 先返回再后台补——harness.go:4531-4545（trigger="mix.observe_read_first_missing_or_stale"），补齐请求只覆盖三类特征 `mixboardObservationFeatureTypes = ["waveform_envelope","spectral_field","l3_acoustic_summary"]`（harness.go:144），对每个可见轨发 warm_waveform_bake（harness.go:4584-4599+4638-4655）；**time_energy 没有独立请求面**——它只能作为 waveform_envelope 烘焙的副产物（tile 流）到达。

**视图消费链（读哪一路）。**
- 视图键定义：capabilitycontext/free_state_observation.go:669——track.time_dynamics keys=[`{track}.slow.time_energy.summary`, `observation.com_projection`]。
- mix.read 解析：mixboard/catalog.go:553-561——rows=**obs.MixPackage.current_metrics["time_energy"]**，status=sourceStatus(obs,"time_energy")（caps，catalog.go:1216-1231）；metrics 行内为空时回退 evidenceTimeEnergyRows（catalog.go:594-603，读观察自带的 FeatureSnapshotEvidence 里**单数** waveform_envelope.time_segments）。
- current_metrics 的填充：mixboard.go:1243（timeSegments=waveformTimeSegments(**featureSnapshot.WaveformEnvelope**)，即单数目标槽）→ buildMixPackage mixboard.go:3219-3225（current_metrics.time_energy=capRows(timeSegments,48)）+ :3209-3212（caps.time_energy = ready iff len(timeSegments)>0）。
- LLM 上下文投影：contextruntime/model_projection.go:1273-1277（time_energy 字段组=status/summary/rows/macro_dynamics/limitations/evidence_refs + com_projection.source_dynamics）。

### Q2：同一工程为什么 stereo_summary 有值而 time_energy 没有

| 环节 | stereo_relation_summary | time_energy |
|---|---|---|
| 生产者 | 内核 L3 分析器（offline_full_song，ImportService.cpp:588 经 L3AcousticSummary 触发） | agent harness collector（harness.go:5535，随 waveform tile 流到达） |
| 持久性 | 按轨落 .vit_derived（l3_feature_log），可按需重焙（warm_waveform_bake feature_type=l3_acoustic_summary → AudioFeatureService.cpp:196-202 路由 L3AcousticAnalyzer） | 不持久于内核；只存在于 agent 侧快照（mixboard_feature_snapshot.json）与观察包 metrics |
| 视图数据源 | metrics["stereo_relation"]=buildStereoRelationSummary(featureSnapshot.**StereoRelationSummary**)（mixboard.go:1263+3223）——单数槽，但导入路径 6 轨 L3 行全部 ready，槽里必然有一个 ready 行 | metrics["time_energy"]=单数 **WaveformEnvelope** 槽的 time_segments（mixboard.go:1243+3221）——槽里是否有段取决于该行是否带段 |
| 单数槽绑定 | `promoteBestL3FeatureRows`（mixboard.go:1637+，target=latestRequestResolvedTarget）——**A 轮实测把 1032(vocals) 的行提升进了 1017(guitar) 的观察**（见 §3 缺陷②） | 同一提升逻辑管 WE 槽（mixboard.go:1570-1576 + 1989-2037 preserveTargetWaveformTimeSegments） |

**A 轮盘面实锤（复刻工程，run 结束时的 mixboard_feature_snapshot.json）**：
- track_waveform_envelopes：6/6 轨，每轨 4 段 time_segments，绑定正确（1017=guitar.wav……1032=vocals.wav）——**导入路径的 agent 侧采集面没有缺口**；
- 单数 waveform_envelope 槽：status=ready 但 **track_id=1032/clip_id=1036（vocals）**——槽绑定的是最后完成填充的轨，不是请求目标；
- stereo_relation_summaries：6 行齐全（1007/1012/1017/1022/1027/1032）。

即：stereo 有值=「内核产物+槽里恰有 ready 行」；time_energy 无值=「非内核产物+单数 WE 槽在观察组装时刻无可用段（或绑定到非目标轨后被目标守卫 stub 掉）」。fixture 路径（B 轮）上同一槽恰好带着 1017 的 4 段（该工程的历史会话状态/上次观察目标=1017），所以视图拿到 4 行 → partial（不是 missing）→ 无 omission → receipt=executed → 门放行。

### Q3：project.structure 的 "evidence status is partial" 源头与具体子项

- limitation 串的生成点：free_state_observation.go:620-622（视图 status ∈ {partial,suspect,approximate} 时追加 `viewID+": evidence status is "+status`）。
- 视图 status 由 assembleFreeStateView（free_state_observation.go:792-841）聚合四键：project.static.summary / project.tracks.summary / observation.tim_projection（**optional-supplemental**，:843-846，其 partial 不计入降级）/ observation.mom_projection（取 project_structure 层 status）。
- 用户现场 obs#1 四键实值（events_round3_2100.json seq10）：tracks.summary=ready、mom project_structure=ready、tim=partial、**static.summary=partial** → 唯一降级键=static.summary。
- static.summary 判定：mixboard.go:3166-3170 buildEnvironmentPackage——`duration<=0 || caps["waveform_envelope"]!="ready"` 即 partial。用户现场 caps.waveform_envelope=**stale**（单数 WE 槽身份不匹配/未刷新）→ partial。
- partial 的具体子项（用户 bundle 的 source_capabilities）：waveform_envelope=stale、ab_result=missing、masking_analysis=missing、realtime_band_energy=missing、realtime_stereo_relation=missing、before_after_delta=pending、lufs_analysis=deferred、reference_match=deferred、waveform_envelope(整体)=stale。其中**结构性常驻项**（phase-5 deferred/依赖播放的 realtime 族）永远在——即 project.structure 视图对任何工程都天然带着 partial 风险，只要目标 WE 槽不 ready。
- tim_projection 自身的 partial 子项=coverage：bit_depth/channel_count/sample_rate 各 missing_count=6（内核工程态不暴露这三项，属已知边界）；risk_summary.overall_risk=none、issue_count=0。
- 本次 A 轮复刻中 obs#1（seq8）static.summary=ready（因为此时单数 WE 槽=1032 的 ready 行）——**证明 structure-partial 是时序/身份依赖的次级 partial，time_dynamics omission 才是确定性主阻断**。

### Q4：审计门「not successful: partial」的判定锚点与放行级别

- 入口包装：chat/semantic_treatment_strategy.go:1322-1324——routeOrdinaryAgentTreatmentStrategy 里 `semanticProgressiveDisclosureAccept(...)` 报错即回 boundary 响应，文案 "free-state semantic intent cannot enter the progressive-disclosure orchestrator: "+err，终态 stop_reason=semantic_treatment_capability_boundary（用户第三轮 21:03:48 实录）。
- 门本体：chat/semantic_progressive_disclosure.go:83-148 semanticProgressiveDisclosureRecordObservation：
  - :91-99 观察包（bundle）级 status：**partial 放行**（注释明确：DAD source-only 视图设计上就是 partial；error/rejected/missing/empty 拒收）；
  - :103-106 必须有 CCB audit_receipt；
  - **:107-110 receipt.status 必须是 ""/ok/ready/success/executed——"partial" 拒收**（本次归因的拦门点）；
  - :111-115 receipt schema v1 + requested_by=model；:117-119 view_set_matches；:122-126 scope/freshness 在场；:131-133 freshness.status=stale 拒收；:138-143 模型视图集=audit 请求集=实际执行集三方一致。
- receipt 被降级的机制：capabilitycontext/free_state_observation.go:642-647——`len(bundle.OmissionReasons)>0` 时把 rejection_reasons 并入 receipt 且 executed→**partial**（A 轮 seq10 复现实录：rejection_reasons=["track.time_dynamics: source evidence is missing"]，receipt.status=partial）。
- 结论：**门实际上要求「零 omission」**。视图 partial（带 limitations）可以过；视图 missing（带 omission）必死。它不区分 omission 的性质——「证据从未生产且收据诚实」与「证据过期/不一致」在同一分支被拦。

## 2. 真栈 A/B 对照（run m2tg1_20261015_213005）

### A 轮：复刻用户路径（project.new → import_folder_as_stems(用户同源 6 stems) → M2 同款话术）

| 检查点 | 结果 | 证据 |
|---|---|---|
| 导入 | ok，shadow engine_track_count=6 | run_report.round_a_track_count=6 |
| 内核 l3_feature_log | band/stereo/loudness 各 6 条=18 条，**无 time_energy**（与用户工程同构） | agent_drafts/.../l3_feature_log.jsonl 亲数 |
| obs#1（project.structure+mix.multitrack_relationship） | bundle=ready、audit=**executed**、零 limitation | a_nl_events.json seq8 |
| obs#2（track.stereo_space+track.time_dynamics @1017 guitar） | stereo_space=ready；**time_dynamics 整视图 omission："track.time_dynamics: source evidence is missing"**；bundle=partial、audit=**partial**、rejection_reasons=[同串] | a_nl_events.json seq10——**与用户 obs#2（21:03:16）逐字段同签名** |
| stereo 证据绑定 | `track.1017.slow.stereo.summary` 事实 track_id=**1032**/clip=1036/源=vocals.wav——**guitar 键下装着 vocals 的证据**（用户第三轮同款，本次确定性复现） | a_nl_events.json seq10 fact 体 |
| 模型行为 | 自行发现「吉他轨的目标级空间读数与素材来源不一致（来源标识指向另一条素材），目标一致性不成立」→ 改测 other(1022)（其 stereo 正确绑定 1022）→ 以建议收口 stop=done/goal=completed（未入编排器，无实验节点） | a_nl_chat_1.json reply + seq11-14 |
| 与用户现场的差异 | 用户第三轮：模型框定假设进 round 后死于编排器入口审计；本次：模型在观察层就绕开了缺失证据轨道，未触发入口审计。**两轮的根因证据（omission+receipt partial）同签名，死点位置取决于模型分支** | events 对照 |

### B 轮：fixture 打开路径（spv1_p01 隔离副本 project.open → ccb_observation_request 只读单腿，零 LLM）

| 检查点 | 结果 | 证据 |
|---|---|---|
| obs#1（project 两视图 @1017） | bundle=**ready**、audit=**executed**、零 limitation；static.summary=ready | b_ccb_obs_project.json |
| obs#2（track.stereo_space+track.time_dynamics @1017） | stereo=ready；time_dynamics=**partial**（非 missing！）；bundle=partial、audit=**executed**（无 omission，只有 limitation "track.time_dynamics: evidence status is partial"） | b_ccb_obs_track.json |
| time_energy 事实体 | `{rows:4, status:"missing"}`——metrics 里 4 段在场（caps 口径仍 missing），evidence/视图=partial | b_ccb_obs_track.json fact 亲读 |

**B 轮与 run12/13 的关系**：run12/13（FIX-FS-SETTLE-DEADCHAIN-1 D1 reverb，fixture 打开路径）确实过了编排器入口并完成干预/结算——本 B 轮解释了为什么：fixture 路径上 time_dynamics 是 partial-with-evidence（4 段在场），不产生 omission，receipt 保持 executed，门放行。轻工件无 receipt 明细、重工件 worktree 已清，故按卡面许可用本只读单腿补验 view 状态。

### 前车之鉴核对（FS-LARGEPROJECT-SMOKE-1）

- 该卡的 import_folder_as_stems 路径 **D1D3 观察腿 PASS 是另一个观察面**（d1d3_premaster_observation：loudness/RLM/stereo 摘要，读 mix.read 的 stereo/loudness 键——d1d3_premaster_observation_runtime.go:23/238），**不覆盖 CCB time_dynamics 视图**——不能用它证明「导入路径时域证据在场」。
- 同卡 NL 流（fs_largeproject_smoke_20261011_210405）的 CCB 观察**同样有 omission**："track.timbre_frequency: source evidence is missing"（nl_events.json 亲读），终态 capability_blocked/free_state_admission_gate_failed——**导入路径的 CCB 轨道视图证据缺口在 2026-10-11 就已存在**（当时缺的是目标轨 band_energy 槽，本次用户路径缺的是 time_energy 槽，同一机制族：单数目标槽在观察组装时刻无该目标可用行）。
- 缺口**不在「导入」触发面**（GUI 与 agent 命令导入同走内核 ImportService 分析队列），而在「观察组装时单数槽的目标绑定」。

## 3. 附带发现（新缺陷登记，建议决策侧triage）

1. **L3 目标行错绑（确定性复现×2）**：guitar(1017) 视图键下的 stereo 证据实为 vocals(1032) 素材（clip_revision 内嵌 track=1032 源=vocals.wav）。锚点族：mixboard.go:1637 promoteBestL3FeatureRows + latestRequestResolvedTarget（快照单数槽被「最后到达行」占据）+ CCB 视图键按请求目标命名但不校验行内 track_id。用户现场（21:03:16）与 A 轮（seq10）同签名；B 轮 fixture 与 A 轮 seq12（track 1022）绑定正确——错绑方向与「最后完成填充的轨」一致。模型侧 source-identity 一致性披露让它自行拒用了错证据（诚实面工作正常），但证据面本身在说谎。
2. 单数 WE 槽与 TrackWaveformEnvelopes 的目标一致性守卫（preserveTargetWaveformTimeSegments，mixboard.go:1989-2037）在 CCB 观察路径上未能把 1017 的带段行扶正进目标槽（A 轮 time_energy 仍 missing）——时序（填充异步未完成即组装）或路径（CCB 观察不经过该守卫）二选一，本轮未完全钉死；修复腿可加打点确认。

## 4. 修复方向建议（供决策侧裁定；组合建议=①+②）

- **①证据生产/绑定补齐（治本，推荐主腿）**：
  - ①a（agent 侧，小）：CCB `track.<id>.slow.time_energy.summary` 的解析（mixboard/catalog.go:553-561）在 metrics 为空时**按请求目标轨从 TrackWaveformEnvelopes 行取段**（该数据 A 轮实测 6/6 在场）——即把 evidenceTimeEnergyRows 的回退源从「单数快照槽」扩到「按 track_id 匹配的轨级行」。这不违反 peer 投影红线（仍是 DAD 之上同一快照的读端目标绑定修正）。
  - ①b（内核侧，中）：给 AudioFeatureType::TimeEnergy 落一个真生产者（L3AcousticAnalyzer 已有逐帧 rms/peak 网格，publishSummaries 加一族 time_energy summary 即可随导入/重焙持久化）——彻底消除「段只在 agent 内存」的时序脆弱性。两步可分卡：①a 立刻解用户阻断，①b 做耐久化。
- **②审计门放行口径（治标，须谨慎裁剪）**：chat/semantic_progressive_disclosure.go:107-110 对 receipt.status="partial" 增加「omission 全部为 not-produced 类（'source evidence is missing'）且非 stale/不一致类」的合格放行，同时把放行事实写进 receipt（保持诚实披露）。**红线校验**：这不是把 partial 升级成 ready——bundle 级 partial 原样保留、omission 收据原样保留，只是编排器入口不再把「从未生产」与「过期/不一致」混为同一拒收分支。若决策侧认为该口径削弱 CCB「partial 原样保留」红线的执行面，则只走①。
- 若同时采纳 ①a+②，用户路径当场通（证据在场→视图 partial-with-evidence→receipt 本就不降级）；①b 落地后 fixture 与导入路径行为对齐。
- 另建议把 §3-1 错绑缺陷单独立卡（文件域=mixboard 提升逻辑/视图键校验），不并入本卡修复腿。

## 5. 工件索引

- 本目录 m2tg1_20261015_213005/：run_report.json、a_nl_chat_1.json（终态+reply）、a_nl_events.json（15 事件含三个观察 bundle 全文）、a_import_*.json、a_stems_manifest_{before,after}.json、b_ccb_obs_{project,track}.json、b_ccb_summary.json、b_project_{open,state}.json、agent_llm_telemetry.jsonl（7 条）、dev_agent_smoke_console.log、agent_drafts/（Unsaved.vit×2、.vit_derived l3_feature_log.jsonl、.vit_agent mixboard_feature_snapshot.json——重件留盘 worktree）、fixture_iso/（隔离副本，重件）。
- 控制台全文：/tmp/m2tg1_run1_console.log（同步 tee）。
- 用户现场对照件：coord/runs/M2-EXPERIENCE-1/events_round3_2100.json + README.md 第三轮节。
- 驱动脚本：scripts/m2_timedyn_forensics.ps1（worktree，随 port 分支）。
