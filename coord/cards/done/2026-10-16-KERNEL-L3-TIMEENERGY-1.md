# KERNEL-L3-TIMEENERGY-1：内核 L3 分析器 time_energy 生产者补齐（M2 时域证据耐久化，①b）

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=M2-EXP-TIMEDYN-GAP-1(120) 终验裁定：①b 另立 P2 卡）
- 派发确认：**已确认（用户 2026-10-18 早窗 /morning 四线确认）**；验收负责人：GLM 主管决策流
- 池序 126（原 122：先与 Mac FE-CEF-PASTE-SHIM-2 撞号改 125，再与 Mac GENAPI-REALVERIFY-1 撞号改 126，后发者改号 2026-10-16 晚窗）;目标仓库=D:\Vit_DAW；P2（耐久化增强，非阻断——①a 已解 M2 旅程数据门）/ 0.5-1 天
- 模型分级：L2 / GLM 执行会话（内核 C++ 面）

## 背景（M2-EXP-TIMEDYN-GAP-1 取证件+终验在卷）

1. 枚举死路实锤：`AudioFeatureType::TimeEnergy` 在场（VitApp/Source/Service/AudioFeatureTypes.h:11，含名称映射 .cpp:11/.33/.78）但**无生产者**——L3 分析只产 band/stereo/loudness 三族，用户工程与复刻工程 l3_feature_log 均 6 轨×3=18 条同构；
2. 当前时域段唯一生产者=agent harness 波形 tile collector；①a 修复（0686b9b4 已合入）后 CCB 按目标轨读 TrackWaveformEnvelopes 行，time_dynamics 视图 partial-with-evidence、receipt=executed——观察依赖 agent 侧派生而非内核耐久化证据；
3. 对照：fixture 工程过 D1 材料资格门时有瞬态/时域指标（transient_window），说明部分路径存在时域特征，导入路径缺。

## 目标

1. 内核 L3 分析器补 time_energy 生产者：导入/分析队列完成每轨后 l3_feature_log 含 time_energy 条目（与 band/stereo/loudness 同生命周期、同触发条件）；
2. 与 agent 侧波形包络派生（acousticpackage/status.go:359 timeEnergyFromWaveform）口径对齐或显式声明差异（不要求逐字节一致，要求不互相矛盾）；
3. 不动 semantic_progressive_disclosure.go 与 agent 读侧绑定（①a 成果零回退）。

## 文件域

VitApp/Source/Service/ 音频特征分析实现（AudioFeatureTypes.* + L3 分析器）；领取前与决策侧核对具体 seam；agent 侧仅当口径对齐需要时才动 acousticpackage。

## 验收标准

- 新建工程+导入音轨路径：6 轨 l3_feature_log 各含 time_energy 条目（对照取证腿 18 条基线增量在卷）；
- 真栈 run（复用 m2_timedyn_fixverify.ps1 场景或同构）：time_dynamics 视图 partial-with-evidence 且身份=目标轨，exit 0；
- `go test ./... -count=1` 0 FAIL + gofmt 净 + CMake 构建 exit 0；
- ①a 四测零回退（timedyn_gap_target_binding_test.go 全 PASS）。

## 停止条件

- 内核 L3 分析器架构与「按轨补族」假设不符（如分析队列按族分派不可加）→ 上交决策侧，不擅自重构；
- 发现 time_energy 语义与 agent 派生口径矛盾且无法对齐 → 上交裁定口径归属，不硬凑。

- 领取：（2026-10-18 21:02 +0800 / origin/main=445c49ce7312bd8127b016faad07a485038639e1 / GLM-5.3 L2 PC 执行会话（ZCode 本会话，用户任务领取口令指定） / 分支=port/kernel-l3-timeenergy-1 / worktree=D:\Vit_DAW_wt_klte1 / 协调分支=coord/klte1-claim；领取核对=卡在 todo 派发已确认、依赖无、doing 仅 GEN-SUNO-WEB-DESIGN-1（纯 coord/runs 勘察零生产代码零冲突）、真栈无占用）

### 领取前 seam 核对（记卡面报决策侧）

1. **触发链零改动**：`ImportService.cpp` `requestImportAcousticPackages`（:548-592）每轨导入时已发一次 `L3AcousticSummary` 请求 → `publishSummaries` 按请求扇出三族 publish。time_energy 挂同一扇出（`requested==TimeEnergy || requested==L3AcousticSummary`）即天然满足「与三族同生命周期、同触发」，ImportService 不动。
2. **内核 seam**（卡面文件域内）：`L3AcousticAnalyzer.cpp` 补 `publishTimeEnergy`（L3 FFT 帧网格=4096 样本非重叠窗的时域 RMS/峰值包络，数据源=`FrameObservation.rmsDbfs/peakDbfs` 已在卷）+ `publishSummaries` 派发；`AudioFeatureService.cpp` 路由表加 `TimeEnergy`（当前该类型落 `publishDeferredStatus`「feature_baker_not_yet_promoted」即枚举死路）。
3. **agent 侧落卷门**（目标①的必需通路，非 acousticpackage——登记请决策侧知悉）：`harness.go:4560 isL3AcousticSummaryFeature`（事件入口门）与 `projectworkspace/l3_feature_log.go:366 isL3FeatureType`（落卷白名单）各需收 `time_energy`，否则内核行在持久化前被丢弃。两处均为白名单加一行，`acousticpackage` 零改动；`writeMixboardReadyL3SummarySnapshot` 对 time_energy 为无害空操作（落卷先于快照）。
4. **口径声明（显式差异、不矛盾）**：agent `l1_static.time_energy` = 波形包络 `time_segments` 派生（细网格，status.go:359）；内核 L3 time_energy = 同一音频在 L3 帧网格（~93ms@44.1k）的时域 RMS/峰值实测，payload 显式 `method/window_ms/hop_ms/segment_publish_limit` 可比性键（循 segmentation primitives 先例）。同族测量、不同网格、各自声明参数——不逐字节一致也不互相矛盾。读侧绑定（`semantic_progressive_disclosure.go` / TrackWaveformEnvelopes）零触碰。

- 回执：（**执行侧自验完成 2026-10-18 21:30 +0800**；实现 commit=484a11cc@port/kernel-l3-timeenergy-1（5 文件 +461/-3：L3AcousticAnalyzer.cpp publishTimeEnergy+派发/AudioFeatureService.cpp 路由/harness.go+projectworkspace l3_feature_log.go 白名单/新增 scripts/kernel_l3_timeenergy_verify.ps1）；工件 commit=bb6d59fc 同分支（run klte1_20261018_211544 全卷）；真栈 run=**klte1_20261018_211544 exit 0 verdict=timeenergy_verify_pass（§8 零 LLM）**——验收①：l3_feature_log 恰 24 行=6 轨×4 族（18 条基线零回退+6 条 time_energy 增量在卷；样例行 status=ready/schema=dad_l3_time_energy.v1/method=time_domain_frame_rms_envelope_v1/window_ms=92.88/segments=216 无截断/rms 全扫描分布+integrated 锚）；验收②：ccb_observation_request 确定性腿 time_dynamics=partial-with-evidence+身份=目标轨(1017)+audit receipt=executed+time_energy rows=4 披露=①a 绑定零回退；验收③：CMake 构建 exit 0（VS 18 2026，产物 build-klte1/VitApp_artefacts/Release/VitApp.exe）+go test ./... -count=1 exit 0 零 FAIL+gofmt 净（LF 归一判定，仓检出 CRLF 系统性）；验收④：timedyn_gap_target_binding_test.go 四测全 PASS。栈占用登记 bffd0224、拆栈后独立复核四端口/四类进程=0。**端测边界声明**：真栈腿零 LLM 确定性（invoke 腿），未跑 NL 话术腿——①a 修复腿的 LLM 面已由 m2tg2 run 覆盖且本卡零触碰该面；渲染面/用户旅程不涉（纯内核+落卷面）。seam 落实情况=领取节 4 点全按记录执行，ImportService/semantic_progressive_disclosure.go/acousticpackage 零改动）
- 验收：（裁定文件 / 验收 commit）
- 验收（2026-10-18 深夜 GLM 主管决策流）：**pass**——seam 四点按领取记录全落（ImportService 触发链零改动/publishTimeEnergy 挂 L3AcousticSummary 同扇出/路由表收 TimeEnergy 解枚举死路/agent 落卷双门白名单各一行，acousticpackage 零改动）；口径声明显式差异合规（同族测量不同网格+可比性键，不矛盾）。决策侧独立复验：worktree klte1 `go test ./... -count=1` **0 FAIL**；工件亲核=run_report verdict=timeenergy_verify_pass+l3_feature_log **24 行=6 轨×4 族**（三族各 6 零回退+time_energy 6 增量）；栈占用/释放登记在卷（bffd0224/21:29 复核净）；CMake exit 0 与被测 kernel SHA 采信（执行侧卷证）。impl 1f7a1c37+工件 faddf995 已 cherry-pick 入 main=317d5eab。卡终态=done。
