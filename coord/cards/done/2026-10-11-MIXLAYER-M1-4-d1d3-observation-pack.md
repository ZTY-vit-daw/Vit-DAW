# MIXLAYER-M1-4：D1-D3 观察能力化（母带前观察只读 pack）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；MIX-CAP-RECON-1 §3 D1-D3 行+M1 表）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 67；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无硬依赖；**与 M1-3 文件域高度重叠——同 owner 串行或先后合入**
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 目标

master_plan §6.1 母带前段 D1（响度峰值）/D2（翻译检查）/D3（参考曲对比）的**观察面**收口为只读能力 pack：

- D1：mix.observe peak/rms/loudness_ranking/headroom + RLM `reference_level_model.projection.v0`（现役）；
- D2：stereo correlation / mono 风险观察（现有面）；
- D3：RLM 投影 user intent+RenderBindings fail-closed 面——**参考曲导入/对比命令缺（audition.* 10 命令在内核 dispatcher 不在 agent catalog）如实登记缺口不动**。

**排序边界**：D/E 母带实现后置维持（2026-10-04 裁定 2）——本卡只做观察能力化，不改实现排序。

## 文件域

chat/ capability seam（与 M1-3 同面：capability_runtime_canary.go+capability_routing.go）+ 新 D 族只读 pack 文件 + 单测。

## 并行与资源

与 M1-3 串行；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 单测：pack 分发+只读性。
- 「D1-D3 观察在自由态循环内可达」冒测断言（同 M1-3 模式）。
- 真栈烟测 exit 0；`go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

mix.observe 命令面/RLM 投影与锚点不符 → 锚点上交。

- 领取：2026-10-11 22:05 +0800 / origin/main=55c997d3 / GLM-5.3 PC 执行流（ZCode 会话，本会话）/ 分支 port/mixlayer-m1-4 / worktree D:/Vit_DAW_wt_mixlayer_m1_4 / 领取提交=643dff54。领取前核对：卡在 todo、派发已确认、M1-3 已合入（6d2295bc 在 main）；doing 六卡（PULL-PROBE-TIER-EXT-1/GEN-API-ADAPTER-1/GEN-CTX-DISCLOSE-1/LOUDNESS-WATCH-DESIGN-1/MIDI-ARGSPEC-ALIGN-1/VIS-FILM-P0-ROUTE-ENABLE）文件域与本卡（chat capability seam+新 D 族 pack+单测+fs_largeproject_smoke.ps1）不相交；M2-TIMEFX-1 未在 doing（同域串行，排本卡合入后）。领取后新增 doing 卡 FE-GUI-SHRINK-IMPL-1（前端仓域，5bde6771）经核对与本卡不相交。
- 回执：实现 commit=7a31c6d5@origin/port/mixlayer-m1-4（15 文件 +1758/-3：新 d1d3_premaster_observation_runtime.go 494 行+单测 337 行+canary 第八分支+owner 四点注册+routing D1/D2/D3 stage case+fs_largeproject_smoke.ps1 -D1D3Observation 腿+run 工件精简件）。验收命令与结果：①`go test ./... -count=1`=92 包 0 FAIL，gofmt 触碰文件净；②单测 11 测全过（注册链五点契约含 D4 fallback 保持+反门+seam 分发 observation_blocked+只读黑名单交叉+白名单外 fail-closed+载体纯函数缺失/在场/超限+RLM 入参接线+needle 契约）；③真栈 run=coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261011_220635，**PASS exit 0**（exit 门=D1D3OBS 五门[schema 行+D1/D2/D3 三面行+D3 缺口登记行+只读声明+A4 标记族零命中]+A1+A4；A2/A3 软记录 classification=capability_blocked 与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线，M1-2 ruling 已接受同语义；栈自 teardown torn_down=true，独立复核端口/进程净）。真栈实录（fixture 工程 61 轨）：D1 digest 全局值 missing 如实+RLM 投影 partial（metric=rms_dbfs 真数据）；D2 逐轨 stereo 读 8 轨/超限 53 轨如实覆盖说明/mono 风险 0；D3 user intent 入投影+绑定 0 条如实+参考曲缺口登记行在。**端测边界声明**：本卡零 webui 渲染面零用户旅程改动（chat 只读观察 Session，无 UI 触碰）——渲染面/旅程门槛不适用；烟测覆盖=agent HTTP 自由态 chat 面可达性+报告确定性渲染，非用户旅程验证。被测=工作树（=7a31c6d5 内容，run 时未提交；head.txt=93e23954 在卷；git_status.txt 因 gitignore 留卷外）。D4/E 实现后置与 D3 命令缺口维持原状未动。占用记录：8e0e52ad 占用/释放已回写（时间误写 22:35 已修正为 22:04:34）。
- 验收：（裁定文件 / 验收 commit）
