# Ruling: FIX-TIMEFX-AXISFILTER-1 —— pass（修复面终裁；收口腿挂账 [等待 LLM 服务恢复]）

- 裁定人：GLM 主管决策流（ZCode 会话）/ 2026-10-13 晚
- 依据：用户"可以进行验收吗"授权；执行侧回执 5cab42d7（实现 cf5bf33f + 工件 37641a2a @ port/fix-timefx-axisfilter-1）
- 合入：merge 2c241ca3（impl+工件双入库；两份干预落账 FULL report 各约 11.8MB 为该轮主证据，重件留盘可回指 worktree artifacts/free_state_d1_s1/）

## 一、独立复核记录（决策侧亲核）

1. **七处修复 diff 亲读**：①axisCoverageWhitelistIdentifiers 加 reverb/delay case（whitelist.Reverb/.Delay identifiers）=原 no_axis_covered_candidates 空集根因，与上交证据"独立面三绿、断点在运行时过滤"完全吻合；②semanticTreatmentBuildSurfaces 接 BuildReverb/DelaySummaryWithBoundary；③readSemanticDynamicSurface 双 switch（builder+attachControlRefs）；④semanticDynamicBindings 两族 sections；⑤semanticDynamicRoleAliases 两族轴词；⑥applySemanticDynamicController 两族分派；⑦reverbUnitCompatible % 分支补 decay/size（decay_pct 量纲同步漏项）。七处与回执逐条一致，注释密度与周边相称。
2. **独立亲跑**：go build ./... EXIT=0；go test ./... -count=1 EXIT=0、0 FAIL（worktree=port 分支检出 99d5978a，测试前后工作树零改动）；gofmt blob 级核对=净（工作树命中 6 文件系 CRLF 检出伪影，与 M2-TIMEFX-1 验收同口径）。
3. **真栈证据亲核**：11 run 工件在卷（console×11+agent 末轮日志+slim 索引×10+两轮 FULL report）；105238 与 181843 两轮干预落账（G1-G8 全绿→治理过滤→Pro-R 装载→ticket→typed 执行→settlement receipt，parameter_applied=True+readback_verified）——**本卡缺陷的实质（干预链不通）已被两轮实证修复**。
4. **环境阻断定性亲核**：180315 轮 summary 原文 "Our servers are currently overloaded. Please try again later."（上游服务端自报）；agent 末轮日志 60s 超时（err=true）与 42s 慢成功（err=false）交替=间歇过载非宕机、非配置/密钥/额度问题（同配置同日撑起 G1-G8+两轮干预复现）。§8 止损适用，执行侧停止重跑判断正确。
5. **栈纪律核对**：占用登记 bb3f9c42（09:50）→释放 4459ccfe（18:31，四端口四进程核净）合规。

## 二、裁定

**pass**（修复面终裁）。M2-TIMEFX-1 先例口径：环境阻断的收口腿挂账，不阻塞修复面合入与解锁。

**挂账（销账条件）**：LLM 服务（gpt-6.1-sol @ hejuapi 中转）稳定后，复跑 `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 一轮 **exit 0**（需重新登记栈占用；-SkipBuild 前核对 worktree 二进制含修复——merge 后亦可在主检出重建跑）。届时三账同收销账：M2-TIMEFX-1（run 111841 挂账）+M2-TIMEFX-2（run 195448 挂账）+本卡。delay 族同话术一轮=可选加分，建议与收口腿同一次占栈跑掉。销账由决策侧复核 run 工件后在本文件补记。

**解锁**：用户 M2 体验场次（10-04 约定）——链路已两轮实证，体验本身走 NL 链同样需 LLM 服务恢复，与收口腿同条件。

## 三、域外上交采纳

执行侧上交 `agent/internal/chat/c2_dynamic_control_runtime.go c2VerificationViews`（:903）缺 reverb/delay case（default 返回 nil；不在 D1 冒测链，故本次两轮复现未踩到）——**采纳立卡 2026-10-13-FIX-C2-TIMEFX-VIEWS-1（池序 107）**，含验证视图选型取证前置。执行侧不扩域上交的处理符合 AGENTS §11。

## 四、工作树与遗留

- worktree D:/Vit_DAW_wt_fix106 **保留至挂账销账**（收口腿复跑可用 -RepoRoot worktree -SkipBuild）；销账后由决策侧拆除（长路径 rmdir 兜底先例适用）。
- 基线既有非本卡引入两处（semantic_treatment_strategy.go unreachable code，go vet）如实登记，不在本卡处置。
- 工件体量注记：两份 FULL report 约 23.7MB 入 main 属一次性证据成本（分支已推远端，对象已存在）；后续同类 FULL report 建议执行侧在提交前给出体量行，>5MB 时先出 slim+抽样方案再入库。

- 验收 commit：（本裁定随卡面归档提交，见 card 面回填）

## 补记（2026-10-13 晚）：DeepSeek 旁路收口尝试——挂账维持原条件

用户切换 LLM 配置至 DeepSeek 官流（api.deepseek.com/v1，deepseek-chat；PC 决策侧探测实证 chat=非推理模式/flash=推理模式后定型）。收口腿按预案执行（预探测 200/0.76s 通过；worktree agent 0f9de7ed 哈希亲核含修复）：

- run 20261013_212900 / 213535 两轮均 **NOT_EXERCISED exit 3**：selected_domains 同为 broadband_compression——reverb 引导话术在场（"残响拖沓…混响尾巴"），模型自主域选择确定性偏压缩面；轮内 LLM 全健康（1.35-4.4s err=false）。§8 同断点连续两次停止原样重跑，delay 加分腿（以 exit 0 为前提）未跑。工件 f9cfc0c1 入 main。
- 定性：**模型分支问题，非服务故障、非修复面**（开放运行未进入 reverb 准入，七处修复未被触达）。执行侧处置正确。
- **挂账维持原条件**：三笔销账（M2-TIMEFX-1 111841 / M2-TIMEFX-2 195448 / 本卡）仍以 gpt-6.1-sol（hejuapi 中转）服务恢复后 D1 reverb 一轮 exit 0 为准；DeepSeek 旁路证据留卷。
- **M2 体验场次不建议在 deepseek-chat 下排**：同分支问题会复现（reverb 话术被选到压缩面）。
- 基线取舍（hejuapi 恢复切回 vs 留 DeepSeek）→ 2026-10-14/15 中期盘点裁定。worktree D:/Vit_DAW_wt_fix106 继续保留至销账。

- 验收 commit（补记）：随本批裁定提交（见 card 面回填；首轮归档=146b5cec）。

## 补记二（2026-10-13 深夜）：用户裁定——基线切换 DeepSeek，挂账条件改写

- 用户裁定原文要旨：「我们已经更换了 API 配置，你也测试通了，肯定不能继续等 GPT 了，我们改用 deepseek 了」——**gpt-6.1-sol 等待条件作废，deepseek-chat（api.deepseek.com/v1）为新基线**。
- 三笔挂账（M2-TIMEFX-1 111841 / M2-TIMEFX-2 195448 / 本卡）销账条件由「gpt-6.1-sol 恢复后 exit 0」改写为：**deepseek 基线下 D1 reverb 一轮 exit 0**（经意图路由适配卡 TIMEFX-D1-EXIT0-DEEPSEEK-1，池序 108）。
- 补记一"M2 体验场次不建议 deepseek 下排"同口径改写为：体验场次待 108 卡落地后排（同一 blocker：域选择偏压缩面）。
- 基线正式登记与跨模型证据口径 → 2026-10-14/15 中期盘点。
