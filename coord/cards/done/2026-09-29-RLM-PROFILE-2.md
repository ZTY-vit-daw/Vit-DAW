# RLM-PROFILE-2：交付 profile 消费方接线——render 绑定（一 render 一 profile）+RLM 披露面+计量现状实勘

- 优先级 / 预估 / 依赖：P1 / 0.6–0.8 天 / L2-1-RLM-1 已合入（rlm/profile.go 数据层+内置 profile 在案，2026-09-27 pass）；蓝图 D9（AGENTIC_OBSERVATION_ROADMAP §2 D9）；**文件域注意：若需改 capabilitycontext 则与 CCB-PARAM 交叉，须在其合入后领取——本卡目标是零触碰 capabilitycontext（披露走 rlm 投影自带输出面）**
- 模型分级：L1 / Mac 端可接（纯 agent 侧 Go）；flash 亦可
- 已核实事实：DeliveryProfile/RenderProfileBinding schema（rlm.delivery_profile.v0 / rlm.render_profile_binding.v0）+内置 profile（EBU R128/AES TD1004/Apple Music/GY/T 282-2014）在 profile.go；**全仓零消费方**（grep 仅 rlm 包内自引用）；RLM Build/ContextProjectionMap 现行为零改动（上卡验收口径）。
- 待证假设（领取时实锚）：render 面挂点（render.start/交付物命令面的稳定绑定位置）；kernel 流式计量现状（VitApp/Source/Service/AudioFeatureTypes.cpp 含 loudness 面——形态与可消费性待勘）。
- 目标：
  1. **render 绑定接线**：一个 render 挂一个 profile（schema 已在案）——找到 render/交付面的稳定挂点接上；绑定关系持久化随工程（§11 旧态缺省=无绑定，行为不变）。
  2. **RLM 披露面**：RLM 投影输出披露当前绑定 profile 与断言语义（目标带不是单向阈——LoudnessBand 语义已定），走 rlm 包自带投影输出，fail-closed。
  3. **计量现状实勘（入报告不强制接线）**：kernel AudioFeatureTypes loudness 面形态、render 后/播放中监督的可行落点——供后续流式监督卡。
  4. **门**：cd agent && go build ./... && go test ./... -count=1 全量 0 FAIL + vet/gofmt（blob 级）+ rlm/capabilitycontext 契约测试（含反例：未知 profile ID 拒绝/无绑定旧行为不变）。
  5. 回执：接线锚点清单+实勘报告+红绿+泊位声明+HEAD。
- 文件域：agent/internal/rlm（主）+render/交付面挂点（领取时实锚）；capabilitycontext 零触碰（若避不开→停止上交，与 CCB-PARAM 排序）。
- 验收标准：绑定+披露接线绿+全量 0 FAIL+旧行为零变化（无绑定工件往返不变）
- 停止条件：render 面无稳定挂点（命令语义缺口）→ 实证清单上交，不擅扩内核命令面
- 领取：2026-09-29 Mac 执行侧（GLM-5.3）。领取时 origin/main=400e375；分支 port/rlm-profile-2。`git status --short`：` M VitApp/Workspace/Settings/Settings.xml`、` M VitApp/Workspace/default_project.xml`（运行时工程状态，不混入）+ untracked `.zcodeignore`、`VitApp/Workspace/Artifacts/`、`coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/`（领取前已有，未触碰）。领取时先跑基线门。
- 回执（2026-09-29 Mac 执行侧，实现 commit 19a2093 @ port/rlm-profile-2）：
  - **接线锚点清单**：①render 面稳定挂点实证=render.start 内核 job_id 生命周期（catalog.go:911 start_render → harness.go renderResults 终态缓存[render_done/render_failed 遥测落]，VitProductionCoordinator.cpp:724 内核侧 render_done 源头）——绑定键=job_id，仅 status=ready（交付物已产出）可绑，未知/failed render fail-closed 拒；②新本地工具 `render_profile_bind`（render.profile.bind）/`render_profile_list`（render.profile.list）——tools/catalog.go 注册+harness invokeLocal 分发（harness.go，模式同 save_as_folder 本地工具先例），零内核命令面扩展；③一 render 一 profile 由 rlm.RenderBindingIndex.Bind 强制（换绑拒、同绑幂等）；④披露走 rlm 包自带投影输出面：Input.RenderBindings（可选，缺席时 Build 输出与旧逐字节一致——测试锚定无 render_profile_bindings 键）→ Projection.RenderBindings 披露槽（binding+参数向量+断言语义"target band not a one-sided threshold"），unresolvable 绑定（如持久化记录指向已退役内置）保留披露行+错误文本+投影级 limitation `rlm_render_profile_binding_unresolvable`，不静默；⑤持久化随工程=chat projectAgentRuntimeState 增可选 `render_profile_bindings`（server.go，schema 既有 vit_project_agent_runtime.v2 兼容：旧态无字段=nil=零绑定行为不变，§11）；restore 逐条过 NewRenderProfileBinding fail-closed 校验，非法条目丢弃计数（RestoreRenderProfileBindings 返回 dropped）。
  - **capabilitycontext 零触碰核实**：git diff 全程 capabilitycontext+queryengine 0 行（CCB-PARAM/L1-3-IMPL-B 避让区）；rlm.Build 唯一调用方 gain_staging.go 不动，Input 增字段对现有调用点零影响。
  - **红绿**：红=修前 vet exit 1（disclosure_test/render_profile_test/render_profile_binding_state_test 新 API 全未定义，coord/runs/RLM-PROFILE-2/red_test_before_impl.txt+red_test_exit.txt）；绿=四目标包全 PASS（rlm/harness/chat/tools，green_test_after_impl.txt GREEN_EXIT=0）。
  - **门**：`cd agent && go build ./...` 通过；`go test ./... -count=1` 全量 **87 包 ok / 0 FAIL**（与领取基线 87 包一致）；vet 本卡文件干净（既有警告 2 条在 semantic_treatment_strategy.go，领取前已有非本卡文件）；gofmt 本卡文件干净（其余 ungofmt 文件为领取前既有，未越域整理）。
  - **diff（vs 领取时，本卡新增）**：11 文件 +631/−7——改 6（server.go +23/harness.go +15/profile.go +16/projection.go +9/types.go +13/catalog.go +2，均 agent/internal/{chat,harness,rlm,tools}），新 5（rlm/disclosure.go 101、harness/render_profile.go 96、测试 3 个 363 行）。VitApp/Workspace/{Settings.xml,default_project.xml} 为领取前已有运行时状态，未触碰未混入。
  - **计量实勘（目标 3，报告 coord/runs/RLM-PROFILE-2/METERING_RECON.md）**：内核 loudness=L3AcousticAnalyzer.cpp:790-812 publishLoudnessSummary——integrated_lufs=rmsDb−0.691 **近似**（诚实自标 approximate_rms_lufs_v1），无 BS.1770 K-weighting/gating/short-term/momentary 窗，**无 true peak 计量器**（全内核 grep 仅插件参数词表命中）；触发=导入期（ImportService.cpp:588→AudioFeatureService.cpp:199）；**render_done 路径零计量挂接**（VitProductionCoordinator.cpp:724 只报 job 状态+file_path）；监督卡落点评估：A=agent 侧读 RenderResult.FilePath 做真 BS.1770（零内核改动，需 Go 侧 DSP 基建）/B=内核 L3 扩真 BS.1770+挂 render_done（须决策侧另立卡）；近似算法不可作交付级断言依据（profile 容差 ±0.5–2 LU vs 近似偏差数 LU）。
  - **端测边界声明（AGENTS §5）**：纯 agent 侧接线（工具目录+本地分发+投影可选槽+sidecar 持久化），零内核/渲染链/现行为变更（Build 无绑定输出逐字节一致测试锚定；capabilitycontext 消费面零改动）；未跑真实栈烟测——工具面与持久化属 agent 内部面，是否需补真栈烟测由决策侧裁定。
  - **泊位声明**：本卡全程未启动任何进程（无真栈/无烟测脚本/无浏览器/无 watcher），无泊位需拆除。
  - **停止条件未触发**：render 面稳定挂点实证存在（job_id 生命周期+终态缓存），无需上交内核命令语义缺口。
  - **未提交**：按 §12，实现 commit 已在 port/rlm-profile-2（19a2093），待决策侧验收；工作树仅剩领取前运行时状态。
- 验收：
- 验收：**pass（2026-09-29 决策会话）**——决策侧复跑 87 包 0 FAIL+blob gofmt 11/11+避让区零触碰亲核+契约测试名实核读（逐字节不变/fail-closed×3/一render一profile）；免真栈裁定（纯 agent 侧+默认路径逐字节不变，新工具烟测归 IMPL-C 端测收口）；计量监督落点裁定（A/B）列 L2 线待裁；rulings/2026-09-29-RLM-PROFILE-2-pass.md
