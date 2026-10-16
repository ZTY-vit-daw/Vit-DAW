# LOUDNESS-B-TRIGGER：响度监听触发接线（编译器式三钩子）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=设计件五 卡B）
- 派发确认：已确认（主管依已验收设计发卡）
- 验收负责人：GLM 主管决策流
- 池序 90；**规格全文=[设计件五 卡B](../../runs/LOUDNESS-WATCH-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：PC 混音线；P1 / 预估见规格 / **LOUDNESS-A 合入后领取**（M1-4 与 TIER-EXT 前置已满足）
- 模型分级：L1 / GLM 执行会话（PC）

## 要点摘录

触发主案=处理后三钩子（settle 桥 chat/free_state_conclusion_bridge.go:65 主+动作观察搭车 harness.go:4279 零 bake+LatestProjectChange 诚实边界）；周期可配默认关；实时=能力缺失配置激活报 unavailable；dedupe+≤4 clip 上限；回执延迟增量=0 与 LLM 增量=0 的设计钉死须在实现保持。

- 领取：2026-10-16 18:26 +0800 / origin/main=0cc87c43b969b747d8a29ddc54b7f21fffb15b9a / owner=GLM-5.3 L2 PC 执行会话（ZCode，本会话） / 分支 port/loudness-watch-trigger-1 / worktree D:/Vit_DAW_wt_loudness_watch_b / 领取提交=coord/loudnessb-claim（本提交）
- 领取时核对：卡在 todo、派发已确认；三前置满足（LOUDNESS-A-CONFIG pass 合入+agent-loop 可达边界转本卡、PULL-PROBE-TIER-EXT-1 pass=e6cacb88、MIXLAYER-M1-4 pass=e0e2f4ac）；唯一在飞 doing 卡 FE-CEF-V1162-GAPS-1（Mac 前端仓+coord/runs 工件，文件域零交集）；GEN-API-ADAPTER-1 已 done+pass（发卡侧注意的 scripts 域重叠核对消解）；0cc87c43 基线上锚点亲核=T1 桥 recordFreeStateSettlement/RecordFreeStateConclusion=:75/WARN 语义在、T2 requestMixObservationFeaturesBackground=:4531（设计行号 :4279 因后续合入漂移，形态一致）、T3 LatestProjectChange 字段=:83/overlay 复制=:363、M1-4 只读面在（d1d3 装配文件域外，披露走 harness mix.observe 载体面+边界申报）
- 回执（阶段一·实现完成，真栈腿待环境）：**impl commit=64b3ccfd**（port/loudness-watch-trigger-1，已推 origin）——8 文件 +2268/-1：`loudnesswatch/trigger.go`（触发层 Runtime：配置门/新鲜分区/纯函数 Compare 复用/陈旧补 bake ≤4 超限 scope_capped/verdict 落盘 dedupe 键=project_uuid+project_revision+verdict_kind（同键 recheck 计数不重复记录）/周期 ticker 默认关（interval 信任 ValidateConfig 的 ≥60s 钳制）/E1 计量 check_duration_ms+bakes_issued+fresh/stale 行数；store 外来 schema fail-closed、历史上限 50）+`trigger_test.go` 13 测；chat 桥 T1（`recordFreeStateSettlement` 成功持久化后后台 goroutine 触发，spawn 点 panic 守卫，watch 失败只 WARN 不回滚 settle）+`free_state_reasoning_loop.go` T3（`prepareFreeStateReasoningContext` 装配后 LatestProjectChange 非空触发，不碰 overlay 语义，即时性诚实边界=循环节奏约束）+`loudness_watch_trigger_test.go` 8 测（含回执先于 verdict 阻塞证法+panic 不回滚 settle）；harness T2（`prepareMixObservationAcousticPackage` 观察流完成点搭车零新增 bake，覆盖域=本次观察 covered targets=resolve/visible 同背景 fill 逻辑）+mix.observe 响应增益 `loudness_watch` 披露段（⑥=M1-4 D1 只读载体面；nil hook=零新增键零行为）+invokeLocal `loudness_watch_configure/status` 两 case（卡A agent-loop 可达前置义务兑现；nil hook=诚实 unavailable 错误）+`RequestLoudnessWatchBake` 陈旧补 bake 面（feature_type=loudness_summary，逐笔失败只 WARN）+`loudness_watch_hook_test.go` 6 测；`fs_largeproject_smoke.ps1 -LoudnessWatch` 腿（4.6 预置：disabled 基线 status 断言+R128 configure 断言；8.8 四门：披露载体在场/verdict 文件落盘+round 触发记录（settle/project_change，kind 不硬断言=封存纪律）/dedupe（observe×2 checks_total 增而记录数稳+键唯一）/配置关零行为（disable 后 observe checks_total 不动）+A4 标记扫描+verdicts.json 工件复制入 run 目录）。
- 验收命令（阶段一）：`go build ./...` 过；`go test ./... -count=1`=**95 包 0 FAIL**；触碰文件 gofmt 净（`gofmt -l` 命中全部为 autocrlf CRLF 检出伪影，去 CR 后逐文件净——TIER-EXT 先例同款）；smoke 脚本 PS Parser 零错误。
- 真栈烟测：**[等待用户]** 环境受阻——run1=fs_largeproject_smoke_20261016_185317 exit 2（环境中断：用户 Godot 编辑器 pid 16472 自 18:16 在场，command line 含 vit-daw-frontend 挡 Phase 0 clear-field；占用纪律不擅杀用户进程；等待至 20:31 未退出，监视器按期收口未启 run；栈全程未启动，20:31 复核四端口四进程零）。占用登记 2338dba0→受阻修订 361b2f5f（含登记核对缺陷更正：进程名 'Godot' 未覆盖变体名）。**重跑条件=用户关闭 Godot 编辑器后按协议重新登记占用再启 -LoudnessWatch 腿**；E1 数据提取脚本已备（coord/runs/LOUDNESS-B-TRIGGER-1/extract_e1.py，读 run 目录 verdicts.json 出 settle 样本成本+复用占比）。
- 边界申报：①M1-4 D1 pack 报告/回执文本呈现面（d1d3_premaster_observation_runtime.go）在卡 B 文件域外——本卡只落 harness mix.observe 载体级披露（observe 响应 loudness_watch 段，d1d3InvokeReadOnly 消费面在腿内断言在场）；pack 装配文本收口建议归卡 C（其文件域含 chat 披露层）或主管另裁。②周期模式 ticker 由三钩子惰性启动（首 T1/T3/注册点 EnsurePeriodic）：纯 idle 启动（无任何 agent 活动）不 tick，与 T3 即时性诚实边界同族，如需启动即 tick 须 server.go 装配点（域外）。③T2 锚从设计件 requestMixObservationFeaturesBackground 收敛到其唯一调用者 prepareMixObservationAcousticPackage（观察流完成点=每次 mix.observe 必经，腿可确定性触发；设计锚函数含于其中；ready-gate 路径共用此函数但触发为后台 goroutine 不阻塞——设计① §2.2 '不挂同步阻塞面'纪律保持）。④probeSpent 账本：本卡 bake 走 harness 内核面非 pull 会话工具面，不计入 probeSpent（设计① §3 '运行在 pull 会话内时'前提不成立），如实申报。
- 回执（待补·真栈腿）：run ID / exit 0 / E1 三样本+复用占比 / 拆栈释放记录
- 验收：（裁定文件 / 验收 commit）
