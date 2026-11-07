# D1D3-REGRESS-STACK-1：d1d3 消费面修复真栈回归门（274+292 合入后验证）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)⑤+[292 裁定](../../rulings/2026-11-07-MIXBOARD-TRACKKEY-1-pass.md)⑥——两卡真栈回归门均挂「搭后续 run」；274=266 D1 门 A 断言（loudness_peak_status=present 由 digest rankings 驱动）+D2/D3 门；292=265 D2 门组（预期 6/7 轨可读 corr≈0.689 等+1039 诚实 missing+verdict 转三态可测分支，取值 fixture-dependent 不预断言））
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 296；目标仓库=主仓（跑腿为主，断言预期面按新 ground truth 修正）；P2 / 0.5 天 / 依赖=274（f413fbca）+292（5c8712a6）已合入 main；L1-L2 / PC；真栈占用一次（登记协议照旧）

## 目标

1. 当前 main 上重跑 265 D2 门组+266 D1 门 A/D3 门（复用 run_free_state_d1_smoke.ps1 体系与同款 fixture，只读复用不复制权威 run 工件）；断言预期按两卡回执预告的新 ground truth 书写（D2 门 tracks 面从恒 missing 转 6/7 ready+1039 诚实 missing——**这不是放松门，是把门对准修复后的真实读数**；逐项预期值引用两卡单测锚）。
2. §8 跑前成文（单 run 无重跑；exit 0/2/4 分类；预算如实——确定性腿预期零 LLM）。
3. 结果三态处置：全 PASS=回归门收口记两卡闭环；断言失败=取证分列（修复缺陷回归 vs 断言预期面漂移，不追绿）；环境中断按 §9。
4. 脚本断言修正如需：域=对应冒测脚本断言块（引用锚+新旧预期对照入回执）；**禁止弱化门语义**（present/ready 断言不得降级为存在性检查）。

## 约束

- 文件域=scripts 冒测断言块±工件+卡面；agent 源码零改动（发现产品缺陷只取证转卡）。
- 真栈协议（§2.2+增补）逐条；与在飞栈腿错峰（277 seg3a_r2/281 smokerun3 在队列前——本卡领取后候栈排队，登记前重读）。

## 停止条件

同型断言失败两轮→止损取证；发现第三处消费错位→记录上交。

## 领取/回执/验收

- 领取：2026-11-07 18:33 +0800 / origin/main=ae537693（领取时最新，a93dd97c 后又前进 297/295/278 三笔零栈协调笔）/ owner=GLM 主会话 L2-worker 子代理（ZCode/PC，会话 296regress）/ 分支=port/d1d3-regress-stack-1（worktree=D:\Vit_DAW_wt_296regress）+coord/296-claim（worktree=D:\Vit_DAW_wt_296coord）/ 领取基线：HEAD=ae537693（两 worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；在飞卡文件域核对：299=plugin-control 预研（S1 真栈轮不随卡授权）、297=VitApp C++ 内核（零栈先行，复现腿候栈）、295=tools/capability_pack、278=外置引擎烟测自管启停、273=harness（停候栈）、275/271=agent——与本卡文件域（scripts 冒测断言块±工件+卡面）不相交
- 回执（执行侧 2026-11-07 19:0x +0800，run1 后）：
  - **结果三态=断言失败取证上交（§8 预写①兑现，非全 PASS）**：run `20261107_184225` **PS1 exit 1**——失败点=**runner 主链 D1 验证尾**（free_state_d1_smoke.py:1770 `materiality is not None`→ps1:664 throw）；**D1-settle/D2/D3 三门组脚本序在 runner 后未及执行=274/292 回归门未验证**（是未到达，非门断言失败）。单 run 授权用尽，未重跑、未在活栈手动补门腿（防追绿）。
  - **runner 失败机理（取证分列，详见 [run1/evidence.md](../../runs/D1D3-REGRESS-STACK-1/run1/evidence.md)）**：响应链 transient_llm_error（首轮 LLM 超时，telemetry 1 error 行）→提案→plugin_selection（Pro-R）→装载确认（Reverb Bus+1032 send）→semantic_dynamic 确认→**capability_blocked**（「时域观察返回缺源数据、投影不可用…重复同一观察集不会带来新证据」）；round settle 尾键全缺（materiality/target_response/decision/audition 均 null）vs 265 reverify1 健康形态完整在场；**1770 之前断言全过**（readback_verified/恰一 post-action 观察/fresh+revision-bound=应用链健康，1-mutation）；capability_boundary 载荷 code=proposal_missing、blocked_by G7_fresh_revision_bound_refs/G8_target_consistency、track 1032。归因分列：①LLM 概率腿 settle 边界停（首要候选；FX 总线新轨无素材+首轮瞬态扰动）②295 修前提示词面（3ff051b0 不在基线 merge-base 亲证，其修 f6d1982e 已在现 main——重派自然消解该变量）③274/292 回归=无证据指向（失败面与两修消费面零交集）④294 卫生=无直接交叠（动作面 1032+新轨，非 1007）。
  - **断言修正交付（本卡 ps1 改动，port/d1d3-regress-stack-1 272f27e9）**：`scripts/run_free_state_d1_smoke.ps1` D2 门组 +119 行——**旧预期→新预期对照**：①observation.stereo_status 记录不判（265 时代如实 missing）→**断言 present**（274 digest 视图修后真值）；②tracks 面无断言（265 时代 7/7 missing 恒 catalog_key_not_found）→**断言 7/7/6/1 形状**（7 轨考虑/7 行/6 行带 correlation/1 行无指标）+**1039 行 track_id=1039 且 status=read**（292 后 mix.read reason=stereo_relation_row_missing_for_track≠catalog_key_not_found→chat 读取面记 read-无指标；status=missing=修复前形态必败）+**1007 锚值 corr=0.689±0.0005/balance=-1.475±0.0005**（catalog_trackkey_test.go 实盘 round3 钉死；其余轨值 fixture-dependent 只记录）+**payload tracks_measured=6**（read-无指标行不计测量）；③gate_results 新增 global_stereo/tracks_face 两键。**门语义未放松**：present/ready 全部带值/带形状断言，PARSE_OK+离线双形状干跑（修复后形状全过/修复前形状必败）亲证。D1 门 A（present，:1100-1105）与 D3 门组断言零改动（本就适配三态真值）。
  - **占栈协议全程**：fetch 重读+净空亲核 18:39 全零→占用登记 eed96ab3 推 main→18:40:25 ≤60s 即时复核全零→18:42 起栈（kernel pid=7528=wt_228 F9E3DA0A/agent pid=16216=worktree 构建 B49B816B370EAC743DE9DC9D0F229720281BFA9818E73A4159D82D25AB63D650，tool_count=296，shadow tracks=7 与 292 镜像一致）→run→owner 亲拆 VitAgent 16216/VitApp 7528/Godot_v4.6.1-stable_win64 10096→**TEARDOWN_ALL_ZERO 亲核 19:00:42**（六口 TCP+UDP 4444/4445+五类进程全零）→释放登记 a084d047 已推。attempt0=启动腿 bash 反斜杠路径事故（PS1 127，零时长，栈未启动亲证——run 未消耗，266 栈前自纠先例）。
  - **计费**：30 调用 deepseek-flash（默认引擎无注入），input 419,531/output 61,162/cache_read 212,604 tok，1 超时行（=首轮 transient_llm_error），估 ≈¥0.3-0.6（§8 预算 ≈¥0.1-1.5 内）；确定性门腿零 LLM 预期维持（门未及执行，无门腿 LLM 行）。
  - **工件**：coord/runs/D1D3-REGRESS-STACK-1/run1/（RUNBOOK §8 跑前成文 272f27e9 于 run 前+evidence.md 取证+console×2+ps1_exit×2+d1_smoke_report.json 全量+agent_last_run1.log+telemetry 30 行+二进制 SHA256），port/d1d3-regress-stack-1 2731b2b2 已推 origin；大件同批入库（13.8MB 报告，D3 verifyrun2 先例）。
  - **交主管裁定项**：①回归门未收口——重派口径建议（现 main 已含 295 f6d1982e，键宣传面修后重跑可同时消解归因②；可参 266 先例单门组降参收窄变量）②runner capability_blocked 形态是否开取证卡（若重派再现即两现开卡）③D2 门组断言修正可先行验收（离线双形状干跑在卷，随重派 run 活栈兑现）。
  - **端测边界声明（AGENTS §5）**：本卡改动=冒测脚本断言块（未触 agent 源码/webui/demo 旅程）；真栈验收=run_free_state_d1_smoke.ps1 门组——**run1 exit 1 未达 §5 门槛**，D1A/D2/D3 门组零活栈覆盖如实申报；两渲染面门槛（E2E-WEBUI-1/JOURNEY-1）不适用。
- 验收（2026-11-07 晚窗，主管）：**rework（返工重跑一次）+D2 断言修正预收**——run1 取证合格（失败机理分列规范，归因①LLM settle 边界停为首要候选、归因②基线未含 295 修亲证成立）；D2 门组断言修正（present 断言+1039 反钉+1007 锚值±0.0005+离线双形状干跑）**预收 pass**，活栈兑现随重跑。**返工意见**：①基线=现 main（含 295 修 f6d1982e，消解归因②）重跑一次，非原样重跑（版本已变）②capability_blocked 形态若重跑再现（两现）→停止，开取证卡③重跑排栈队列 300 S0/P 之后④预算沿用 ≈¥0.6/run。卡移回 todo 候重跑腿。L1 台账 48h 窗口。
