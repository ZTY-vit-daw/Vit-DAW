# M2-EXP-TIMEDYN-GAP-1：真实工程时域证据缺口——track.time_dynamics 源证据缺失+project.structure partial 拦死语义编排器入口

- 发卡：GLM 主管决策侧 / 2026-10-15 晚窗（依据=M2 体验场次第三轮现场取证，[events/runtime_status round3](../../runs/M2-EXPERIENCE-1/)）
- 派发确认：**已确认（用户 2026-10-15 晚窗「可以开始取证」=取证腿；修复腿待取证后另确认）**；验收负责人：GLM 主管决策流
- 池序 120；目标仓库=D:\Vit_DAW；P1（M2 旅程当前唯一硬阻断）/ 取证先行
- 模型分级：L2 / GLM 执行会话（跨内核/agent 双面）

## 缺陷实锤（2026-10-15 21:00-21:04 现场亲证）

1. 用户真实工程（6 轨，shadow 正常计数）发 M2 主推话术后：忙碌反馈正常（117 修复现场生效）、CCB 观察链真跑（catalog→两次 view 请求完成）、轨迹把假设框到 1017 guitar 轨、round 启动——**最后死在编排器入口审计**：
   `free-state semantic intent cannot enter the progressive-disclosure orchestrator: semantic progressive disclosure observation audit is not successful: partial`，stop_reason=semantic_treatment_capability_boundary，诚实收口（task closed without governed experiment）。
2. 两个 partial 源：**obs #1**（21:01:49，views=[project.structure, mix.multitrack_relationship]）：mix 视图正常交付（MOM 投影在卷），limitations=["project.structure: evidence status is partial"]；**obs #2**（21:03:16，guitar/1017，views=[track.stereo_space, track.time_dynamics]）：stereo_space 正常（balance 5.945dB right_heavy、offline_full_song 捕获在卷），omission_reasons=["**track.time_dynamics: source evidence is missing**"]。
3. 证据生产面锚点：视图键=`{track}.slow.time_energy.summary`+`observation.com_projection`（capabilitycontext/free_state_observation.go:669）；time_energy=l1_static 层由波形包络派生（acousticpackage/status.go:359 timeEnergyFromWaveform）+内核 AudioFeatureType::TimeEnergy（AudioFeatureTypes.cpp:11）在场；**但用户工程的两轮 l3_feature_log 均只含 band/stereo/loudness 三族（6 轨×3=18 条），无 time_energy 条目**——fixture 工程过 D1 门时有瞬态/时域素材（材料资格门 transient_window 指标），真实导入工程没有。

## 修复目标（取证先行）

1. 归因：为什么「新建工程+导入音轨」路径不产 time_energy（与 structure）证据而 fixture/打开既有工程路径有——内核分析器触发条件、agent 按需派生路径、还是 DAD 素材资格链只在 smoke 路径跑；
2. 修复方向二选一（取证后裁定）：①证据生产补齐（内核或 acousticpackage 按需派生）；②编排器审计门对「视图缺失但有诚实 omission 收据」的合格放行口径（不弱化 partial 不升级原则——须区分「证据未生产」与「证据过期/不一致」）。

## 文件域

取证=只读；修复腿视归因定（VitApp 音频特征分析器 或 agent acousticpackage/capabilitycontext/agentloop 审计门），领取前与决策侧核对。

## 验收标准

- 取证件：两路径（新建+导入 vs fixture）证据生产对照+缺口根因；
- 修复后真栈：用户真实工程发同话术 → 轨迹进编排器（不死于 partial 审计）或给出明确可行动的缺证据指引（非裸 boundary），exit 0；
- `go test ./... -count=1` 0 FAIL + gofmt 净（涉内核另加构建 exit 0）。

## 停止条件

归因发现审计门语义与 CCB「partial 原样保留」红线冲突 → 上交决策侧裁定放行口径，不得自行弱化。

## 领取（取证腿）

- 领取时间：2026-10-15 21:28 +0800（晚窗；卡内原误写 22:0x，以本行为准）
- origin/main hash：ec8efed9a9cad6cb42f9d1dbe253bcb746e9169c（≥ef54e06c 要求满足）
- owner：GLM-5.3 L2 / PC 执行会话（ZCode，本会话）
- 分支：port/m2-exp-timedyn-gap-1-recon（worktree D:/Vit_DAW_wt_m2tg1）
- 领取提交：727d1dd8（已推共享 main，远端卡面 owner 核对=本会话）
- 领取时 git status：干净 worktree（新建，基于 origin/main）；主检出另有未跟踪 coord/runs/* 工件与 M VitApp/Workspace/default_project.xml，与本卡无涉

## 回执（取证腿，2026-10-15 21:35 交付）

- 取证件：[coord/runs/M2-EXP-TIMEDYN-GAP-1/FORENSICS.md](../../runs/M2-EXP-TIMEDYN-GAP-1/FORENSICS.md)（四问逐答+行号锚点+A/B 对照+前车之鉴核对+修复方向建议+§8 记账）；轻工件随 port 分支，重件（fixture_iso/agent_drafts）留盘 worktree。
- 真栈 run：m2tg1_20261015_213005，exit 0，栈占用/释放登记在卷（PC-RUNTIME-STACK.md，占用 622e326e→释放见最新条）；LLM 7 次全成功 0 超时；stems 源哈希前后一致。
- 归因一句话：**内核不存在 time_energy 生产者（枚举死路，L3 只产 band/stereo/loudness——用户工程与复刻工程 l3_feature_log 均 18 条同构）；时域段唯一生产者=agent harness 波形 tile collector，且导入路径 6/6 轨级行各带 4 段在场；拦死链路的是 CCB 轨道视图读快照单数目标槽，而导入路径该槽绑定「最后完成填充的轨」（vocals/1032）而非请求目标轨（guitar/1017）→ time_dynamics 视图 omission → receipt executed 降级 partial → 编排器入口门（semantic_progressive_disclosure.go:107-110）拒收。**
- A 轮复现实锤：seq10 观察 omission="track.time_dynamics: source evidence is missing"+audit receipt partial，与用户 obs#2 逐字段同签名；chat 终态 stop=done（模型绕开缺证据轨，未触发入口审计——死点位置随模型分支漂移，根因证据同签名）。B 轮 fixture：time_dynamics=partial-with-evidence（4 段）+receipt=executed→门放行，解释 run12/13 为何过。
- 附带新缺陷（确定性复现×2，建议另立卡）：guitar(1017) 视图键下 stereo 证据实为 vocals(1032) 素材（单数 L3 槽错绑；模型经 source-identity 自行拒用，诚实面正常）。project.structure partial=次级时序依赖源（static.summary←caps.waveform_envelope!=ready，mixboard.go:3168），A 轮未复现该 partial（obs#1 ready）。
- 修复方向建议（FORENSICS.md §4，待决策侧裁定）：①a agent 侧按目标轨从 TrackWaveformEnvelopes 行取段（小改，立刻解阻断）+①b 内核 L3 增 time_energy 生产者（耐久化）为治本组合；②审计门对「not-produced 类 omission+诚实收据」放行须先过红线裁定（不升级 bundle partial，只区分未生产 vs 过期/不一致）。
- 端测边界声明：未启 Godot GUI（agent HTTP 面驱动）；导入命令与 GUI 导入同走内核 ImportService 分析队列，观察链全在 agent 侧——复现边界已在 FORENSICS.md §0 声明。零生产代码改动（main 上只有卡片/资源登记提交）。
- 修复腿待决策侧裁定修复方向后另派（本回执不含修复）。
