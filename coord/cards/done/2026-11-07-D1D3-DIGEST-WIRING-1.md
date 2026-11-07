# D1D3-DIGEST-WIRING-1：d1d3 观察消费面 digest 子图接线修（272 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[272 取证](../../runs/OBSFACE-SELECTIVE-MISS-1/forensics.md)+[272 裁定](../../cards/done/2026-11-07-OBSFACE-SELECTIVE-MISS-1.md)——主缺陷=d1d3AssemblePreMasterReport 从 harness 响应**顶层**读 stereo_status/peak_dbfs/waveform/rankings 而实际嵌套 `out["digest"]`（拍平面 `out["acoustic_digest"]` 键名又不同）→投影结构性恒 missing，M1-4 起先天；单测喂 digest 形状掩盖）
- 派发确认：已确认（主管发卡，自主批次·限额恢复后首批）；验收负责人：GLM 主管决策流
- 池序 274；目标仓库=主仓 agent（d1d3 消费面）；P1 / 0.5 天 / 依赖=272 工件在库；L1-L2 / PC；零栈先行（单测+形状反钉），真栈回归门可搭后续 run

## 目标

1. **接线修**：d1d3AssemblePreMasterReport 改读 `out["digest"]` 子图（拍平面 `out["acoustic_digest"]` 键名差异同步处理）——三面（stereo/peak/rms/crest+waveform+rankings）按 272 对照矩阵逐面修；
2. **单测反钉**：现有单测夹具形状改用真实响应形状（266/265 run 工件中的实盘响应为夹具蓝本——教训=夹具形状反钉真实响应，杜绝顶层/digest 双形状自洽假绿）；
3. **相邻两处顺带勘察**（272 遗留）：①D1 全局 waveform 面 settle 相位 ready→missing 翻转（265 同链未翻转，差异驱动）②265 逐轨 stereo mix.read 7/7 missing vs feature snapshot 19 行 ready（第二处消费错位候选）——勘察结论同卡交付（可修则修=同文件域，跨域则记录）；
4. 回归：D1/D2 门 A 真栈断言可作回归门（搭后续 run）；chat/mixboard 包全绿。

## 约束

文件域=agent/internal/chat/d1d3 消费面+测试；kernel 零改动（铸造层 13/13 ready 亲证）；禁动 harness 响应结构本体（只修读取侧）。

## 停止条件

相邻勘察发现第三处跨域缺陷→记录上交；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 01:16 UTC / origin/main=09f284d8661b4b6d6561c0269d9d88379415c7bc / owner=GLM 主会话 L2-worker 子代理（ZCode/PC）/ 分支=port/d1d3-digest-wiring-1 / worktree=D:/Vit_DAW_wt_274impl / 领取基线：HEAD=09f284d8（worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；doing 三卡（ARR-SRCLOAD-1=capability_pack+catalog、EDU-LEVEL-RUNTIME-B=edu_jam_chat+server.go 接线、CLOSURE-ROUTE-ADAPT-1=audio_closure_controller）与本卡文件域核对不相交
- 回执（2026-11-07 02:05 UTC，执行侧自验完成待验收）：
  - **diff 清单**（分支 port/d1d3-digest-wiring-1，commit `4c437f68`，基线 09f284d8）：`agent/internal/chat/d1d3_premaster_observation_runtime.go` +51/-19（新增 d1d3DigestView+d1d3AssemblePreMasterReport 三面改读 digest 视图）、`agent/internal/chat/d1d3_premaster_observation_runtime_test.go` +184/-17（夹具反钉真实形状+五组新测试）。合计 2 文件 +235/-36。kernel/harness 零改动，文件域内。
  - **实现要点**：①新增 `d1d3DigestView`——`out["digest"]` 子图为主源，`out["acoustic_digest"]` 拍平面按真实键名归一兜底（`stereo_relation_status`→`stereo_status`；浅拷贝，不污染响应本体，有无污染契约测试）；②`d1d3AssemblePreMasterReport` 三面（stereo/peak/rms/crest+waveform+rankings+missing_metrics+available_detail）全量改读 digest 视图，`observation_id` 顶层优先 digest 兜底；③单测夹具蓝本=265 实盘 `obs_20261106T152825_863f2de0d657`（digest 嵌套形状+实值 -15.873/-30.255/14.383/corr 0.689），另钉 266 settle 形态（waveform status=missing 值全 null+rankings 6 行在场→D1 面 present=ps1:1100 门 A 语义）、顶层旧假形状防回退（必须投影 missing，杜绝双形状自洽假绿）、拍平面兜底、视图无污染五组。
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_274impl @ 4c437f68）：`cd agent && go test ./internal/chat ./internal/mixboard -count=1` → exit 0（chat ok 135.7s / mixboard ok 3.4s）；`go build ./...` → exit 0；`go test ./internal/mom -run 'Test.*(AB|RenderProbe|Projection)' -count=1` → exit 0；开发中 `go test ./internal/chat -run 'TestD1D3' -count=1` → exit 0。
  - **相邻勘察①（D1 全局 waveform settle 相位 ready→missing 翻转，差异驱动已定位到判定函数）**：直接驱动=`normalizeProjectFeatureMaterialFreshness` → `currentProjectFeatureRowOrMissing`（`agent/internal/mixboard/mixboard.go:2534`、:2544-2559）——266v1 翻转时点（obs_171506）WE 槽行已从 `_1031`（track 1027，ready）替换为 `kernel_prepared_waveform_envelope_1055`（track 1007/clip 1055），该行材质身份与工程态失配 → 打 `project_material_identity_mismatch` stub → buildWaveformMetrics 重塑为值全 null+status missing（无 reason 形态与实盘吻合）；同刻逐轨域（obs_171512，selected_track）经 freshWaveformRowOrMissing 的 requestedTarget 分支保留 ready。**265 未翻转对照**：全程 WE=`_1011`（track 1007）材质匹配、settle 相位内 WE 槽未被新行替换（两 run 观察件内嵌 snapshot 亲核）。深层差异（1055 行的产生者/写入路径及材质失配来源，涉 harness 特征物化与 kernel telemetry 链）**跨 mixboard/chat 域，记录上交不扩修**；272「设计内新鲜度失效」假说成立并精化到行级时序，诚实缺失不升级语义维持。
  - **相邻勘察②（265 逐轨 stereo mix.read 7/7 missing vs feature snapshot 19 行 ready，确定性机理已定位）**：`readObservationKey`（`agent/internal/mixboard/catalog.go:469`）`targetID := firstNonEmpty(obs.TargetRef.ID, "target")` 取**观察包自身** TargetRef——d1d3 的 mix.observe 是 full_project scope，实盘 TargetRef=`{id:"current",kind:"project"}`（265 obs_152825 亲核），故 case 仅匹配 `track.current.slow.stereo.summary`；d1d3ReadTrackStereo 请求的真实 track id 键（track.1007…track.1039 等 7 键）**全部走 default → catalog_key_not_found → 7/7 missing**（265 d2_check_chat.json 逐行核对：status=missing 无 error，与该路径产出吻合）。feature snapshot 的 `stereo_relation_summaries` 19 行 ready 在 mix.read 键面**无消费路径**（`.slow.stereo.summary` case 只读单 target current_metrics.stereo_relation，catalog.go:570-571；`project.acoustic.tracks` 行不含 stereo 相关字段）。修复归属=mixboard 键路由（按请求键 track id 解析+snapshot 行集消费）——**域外，不扩修**；chat 侧无替代读取键可用。
  - **端测边界声明（AGENTS §5）**：本卡零真栈——验证边界=Go 单测（chat/mixboard/mom 包）+全量 build，未覆盖真实运行栈上的端侧行为。真实栈回归门已就位待后续 run：266 既有 D1 门 A 观察断言（run_free_state_d1_smoke.ps1:1100-1105，loudness_peak_status=present——修复后由 digest rankings 面驱动满足）+D2/D3 门（stereo_status=present 预期随铸造层 13/13 ready 如实披露）。单 run 语义，无重跑承诺。
- 验收（2026-11-07 早窗，主管）：**pass**——接线修与 272 对照矩阵逐面对齐（digest 视图双源归一+浅拷贝防污染亲读 diff）；单测反钉合格（265 实盘蓝本+266 settle 形态+顶层假形状防回退组=双形状假绿教训入测试面）；主管交叉复跑 chat 全包 128.1s ok+mixboard/mom ok+TestD1D3 专项 ok；相邻两处勘察=取证交付合格（两处均锚点+双证在卷、跨域不扩修）——转卡 292（mix.read 键路由）/293（WE 失配取证）；零真栈边界如实、真栈回归门（266 D1 门 A+D2/D3 门）记 backlog 搭后续 run。实现 4c437f68 cherry-pick 合入 main。详见 [rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)。L1 台账 48h 窗口。
