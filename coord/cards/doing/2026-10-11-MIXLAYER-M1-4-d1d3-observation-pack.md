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

- 领取：2026-10-11 22:05 +0800 / origin/main=55c997d3 / GLM-5.3 PC 执行流（ZCode 会话，本会话）/ 分支 port/mixlayer-m1-4 / worktree D:/Vit_DAW_wt_mixlayer_m1_4 / 领取提交=见卡 git log（本提交）。领取前核对：卡在 todo、派发已确认、M1-3 已合入（6d2295bc 在 main）；doing 六卡（PULL-PROBE-TIER-EXT-1/GEN-API-ADAPTER-1/GEN-CTX-DISCLOSE-1/LOUDNESS-WATCH-DESIGN-1/MIDI-ARGSPEC-ALIGN-1/VIS-FILM-P0-ROUTE-ENABLE）文件域与本卡（chat capability seam+新 D 族 pack+单测+fs_largeproject_smoke.ps1）不相交；M2-TIMEFX-1 未在 doing（同域串行，排本卡合入后）。
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
