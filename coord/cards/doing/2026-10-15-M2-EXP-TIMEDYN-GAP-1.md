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

- 领取时间：2026-10-15 22:0x +0800（晚窗）
- origin/main hash：ec8efed9a9cad6cb42f9d1dbe253bcb746e9169c（≥ef54e06c 要求满足）
- owner：GLM-5.3 L2 / PC 执行会话（ZCode，本会话）
- 分支：port/m2-exp-timedyn-gap-1-recon（worktree D:/Vit_DAW_wt_m2tg1）
- 领取提交：（领取 push 后回填 hash）
- 领取时 git status：干净 worktree（新建，基于 origin/main）；主检出另有未跟踪 coord/runs/* 工件与 M VitApp/Workspace/default_project.xml，与本卡无涉
