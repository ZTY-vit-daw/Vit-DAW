# MIXBOARD-TRACKKEY-1：mix.read 逐轨观察键路由修（274 相邻勘察②转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 回执相邻勘察②](../done/2026-11-07-D1D3-DIGEST-WIRING-1.md)+[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)④——逐轨 stereo mix.read 7/7 missing vs feature snapshot 19 行 ready：`readObservationKey` 取观察包自身 TargetRef.ID（full_project 观察=`"current"`，agent/internal/mixboard/catalog.go:469/:570），d1d3 请求携带的真实 track id 键恒走 default→`catalog_key_not_found`；feature snapshot 的 `stereo_relation_summaries` 在 mix.read 键面无消费路径=第二处消费错位）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 292；目标仓库=主仓 agent（mixboard 键路由）；P1 / 0.5 天 / 依赖=274 已验收合入（digest 视图在 main，逐轨面回归基准就位）；L2 / PC；零栈先行（真栈回归门搭后续 run）

## 目标

1. **键路由修**：track-keyed 观察读取对 full_project 观察包的正确解析（TargetRef 语义路由——track id 请求应命中 full_project 观察内的对应 track 面，而非按观察包自身 ref 键恒 miss）；`stereo_relation_summaries` 的 mix.read 消费路径打通或如实记录不可消费边界；
2. 单测反钉：265 实盘形状夹具（真实 track id 键→ready 读数 7/7；修复前恒 missing 的负锚）；回归 `go test ./internal/mixboard ./internal/mom ./internal/chat -count=1` 全绿+build 0；
3. 勘察顺带：catalog 键面是否存在同型第三处消费错位（只勘察，发现即记录上交不扩修）。

## 约束

- 文件域=agent/internal/mixboard（catalog.go 键路由±同包消费面）+测试；不触 chat/d1d3 消费面（274 已收口）、不触在飞卡域（273/275/277/281）；
- 观察包铸造面（TargetRef 语义本体）零改动——若修复必须改铸造面→跨域上交转裁定；
- 零真栈：真栈回归门=265 D2 门断言搭后续 run。

## 停止条件

修复发现 full_project/track 观察的键语义存在在案裁定约束→锚点取证上交；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 早窗 / origin/main=acc17453f45c9ba1da17eb1043d2b3d307017823 / owner=GLM 主会话 L2-worker 子代理（ZCode/PC）/ 分支=port/mixboard-trackkey-1 / worktree=D:/Vit_DAW_wt_292impl / 领取基线：HEAD=acc17453（worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；在飞卡文件域核对：293=coord/runs 取证零源码、273=harness/chat（已停候栈）、275/281=py+edu+server、277=capability_pack+catalog 消费面（与 mixboard 键路由不相交，卡面红线条目在案）
- 回执（2026-11-07 早窗，执行侧自验完成待验收）：
  - **diff 清单**（分支 port/mixboard-trackkey-1，commit `b7a9965e`，基线 acc17453）：`agent/internal/mixboard/catalog.go` +106/-1（default 分支 track-keyed stereo 语义路由 + parseTrackKeyedStereoKey/readTrackKeyedStereoFace/observationFeatureRows 三助手 + Store.Read 证据快照触发扩展）、`agent/internal/mixboard/catalog_trackkey_test.go` +338 新增（265 实盘形状夹具八组测试）。合计 2 文件 +444/-1。观察包铸造面（TargetRef 语义本体）零改动；chat/d1d3 消费面零改动。
  - **实盘形状亲核补充**（265 工作区 `D:/Vit_DAW_wt_265rv/artifacts/free_state_d1_s1/20261106_232316/project/.vit_agent/vitproj_b3f…/observations/obs_20261106T152825_863f2de0d657.json`）：①stereo_relation_summaries 19 行 ready 实为 **6 轨**（1007×4、1012/1017/1022/1027/1032 各 3）——**track 1039 无任何行**（天然负锚，夹具已按此钉）；②该包 digest.stereo_status=ready（singular 槽已被提升为 1007 行）——reverify1 d2 报告的「全局 stereo_relation 缺席」是 272 顶层读取 bug（274 已修）所致，与本包不符，两案已分清；③realtime_stereo_relation_summaries=0 行；④包无 evidence_refs（内联行集即工作形态），track_waveform_envelopes/band_energy_summaries 同为 6 轨。
  - **实现要点**：①`track.<id>.slow.stereo.summary`/`track.<id>.realtime.stereo.summary` 两面在 default 分支路由：按请求键轨道 id 从 feature snapshot 行集取**严格同轨**（track_id EqualFold 相等，位移/无 track_id 行一律不披露）行，经 `buildStereoRelationSummary` 披露——与自身目标键（`track.current.*`→current_metrics.stereo_relation）完全同形；②行选择复用 `bestRealtimeFeatureRow`（状态位→目标分→完备度，与 singular 槽提升 promoteBestL3FeatureRows 同一判定链），265 实盘 1007 同材质 4 行取最优确定；③无行轨道返回诚实逐轨 missing（`stereo_relation_row_missing_for_track`/`realtime_stereo_…`），键已路由不再是 catalog_key_not_found；④行集解析三级：内联 GlobalSummary.feature_snapshot（v2 持久化保留 summary 行；265 实盘形态）→evidenceSnapshot 参数（Store.Read 对 track-keyed stereo 键与 `.raw.` 同款触发 lazyFeatureSnapshot）→FeatureSnapshotEvidence 回溯（hard-projected 包 {status:projected,evidence_ref} 形态兜底）；⑤自身目标键精确 case 臂语义原样保留（defensive 守卫：路由不接管 id==TargetRef.ID 的键）。
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_292impl @ b7a9965e）：`cd agent && go build ./...` → exit 0；`go test ./internal/mixboard -count=1` → exit 0（ok 4.656s）；`go test ./internal/mom -count=1` → exit 0（ok 0.551s）；`go test ./internal/chat -count=1` → exit 0（ok 148.318s）；开发中 `go test ./internal/mixboard -run 'TestMixReadTrackKeyed|TestReadTrackKeyed' -count=1` → exit 0（ok 1.170s，首轮通过零重跑）。
  - **正负锚断言摘录**（catalog_trackkey_test.go）：正锚=`TestMixReadTrackKeyedStereoServesFullProjectSnapshotRows`（6 轨键→status=ready+correlation_estimate 逐轨值 1007=0.689/1012=0.31/1017=0.95/1022=0.12/1027=0.55/1032=0.77，1007 另钉 balance_db=-1.475+correlation_state=coherent=实盘 round3 值；披露行 track_id 必须等于请求轨=身份绑定）；负锚①=`TestMixReadTrackKeyedStereoMissingTrackIsHonestPerTrackMissing`（1039 无行→status=missing+reason=stereo_relation_row_missing_for_track，**显式断言 reason≠catalog_key_not_found**=修复前恒 miss 形状反钉）；负锚②=`TestMixReadTrackKeyedStereoDoesNotSwallowUnknownKeys`（stereo.detail/fast.levels/band_energy/空 id/前缀错/胡键 六形仍 catalog_key_not_found=路由不误吞）；回归锚=`TestMixReadTrackKeyedStereoOwnTargetKeyUnchanged`（track.current.slow.stereo.summary 仍 ready clip_id=1011=singular 槽原语义）；realtime 面/证据参数兜底/位移行不披露（含无 track_id 行）三组补钉。
  - **第三处勘察结论（只记录不扩修）**：同型 default-miss 在 catalog 键面另有四处候选——①`tools/capability_pack.go:475` 提示词把 `track.{id}.slow.time_energy.summary` 宣传为稳定 catalog 键，但 full_project 观察上真实 track id 恒 miss 且逐轨 time_energy 材质边界受限（current_metrics 仅单目标；track WE 行 time_segments 被内联投影剥离，仅证据 blob 有）——**域外（tools/），上交裁定**；②`catalog.go` relationshipEvidenceRefs（:1182-1185）为 focus/peer 轨发射 `track.<id>.fast.levels` 证据 ref，full_project 观察上该键真实 id 恒 miss（材质在场：track_waveform_envelopes 6 轨）——同包不同消费面，**记录不扩修**；③`track.<X>.slow.band_energy.summary`（band_energy_summaries 6 轨在场）与 ④`track.<X>.static.identity`（targetTrackIdentity 只读自身 ref；ProjectPackage.tracks 有身份行）同型休眠面，未发现 chat 侧消费方。另记两条非键面注记：BuildCatalog 仍只广告自身目标 track 键（逐轨键可读未广告，d1d3 直读键不受影响，良性）；**chat 侧分类细微差（274 已收口域，供 265 D2 门后续 run 参考）**——`chat/d1d3_premaster_observation_runtime.go:261-266` 对 reason≠catalog_key_not_found 的 item 一律 Status="read"，本修后 1039 类无行轨将计为 read（无 correlation）而非 missing，D2 门 tracks_measured/skipped 语义届时需按此解读。
  - **端测边界声明（AGENTS §5）**：本卡零真栈——验证边界=Go 单测（mixboard 新增八组+全包、mom、chat 三包）+全量 build，未覆盖真实运行栈端侧行为。真栈回归门就位待后续 run：265 D2 门断言（run_free_state_d1_smoke.ps1 D2 门组四门）——修复后逐轨面预期 6/7 轨可读（corr≈0.689 等）+1039 逐轨诚实 missing+全局 digest stereo ready（274 修后读法），D2 verdict 预期从 unmeasurable 转入三态可测分支（具体取值 fixture-dependent 不预断言）。单 run 语义，无重跑承诺。
- 验收：（主管回填）
