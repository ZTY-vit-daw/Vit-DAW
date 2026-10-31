# M3-OBS-EVIDENCE-FIXTURE-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-31-M3-OBS-EVIDENCE-FIXTURE-1.md（池序 230）
- 执行：ZCode GLM-5.3 PC 执行会话（230 卡会话）；worktree D:/Vit_DAW-wt-m3obs @ port/m3-obs-evidence-fixture-1
- 领取基线：origin/main=6bd5b804（229 合入后主干），领取提交 1b79bc29；实现 commit 见卡面回执
- 栈登记：6fda77e5（18:2x 置顶，fetch 重读+进程/端口全零亲核+登记先于启动）；释放见文末拆栈记录
- 命令（判定逻辑原口径不动——三组×5 轮、组级 ≥4/5、同型三连止损承 220；改的只有 setup 工程形态（选项 A）+判定分类段新增 capability_blocked 型（选项 C））：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW-wt-m3obs/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir D:/Vit_DAW-wt-m3obs/coord/runs/M3-OBS-EVIDENCE-FIXTURE-1/obsfixture_20261031_1
```

- run：obsfixture_20261031_1；**ps1 exit 0 COMPLETED**；kernel=228 worktree VitApp.exe sha256 f9e3da0a 运行前后零漂移（只读复用，本卡 C++ 零改动，228/229 同形）。
- 前置失败腿（不计入验收轮）：leg1 setup 解析缺陷（/agent/invoke 回复 `result.bundle` 两层结构，脚本按顶层 bundle 取）→ 域内修正（result unwrap，b4/CCB smoke 同款解析）→ 拆栈全零后重跑。修正前观察请求本身已成功（audit_receipt.actual_executed_view_ids=["track.basic_energy"] 在 leg1 控制台在卷）。

## 一、链路进度（§7 分列一）

### 本卡两面实现兑现（scripts 域）

- **选项 A（fixture 补确定性 clip）兑现亲证**：setup stage 2 三环证据链全过——
  1. clip 在场：两轨各 1 clip（`clip_counts={"1007":1,"1010":1}`，get_project_state 亲证；wav=脚本生成 48kHz/16bit/stereo/6.0s，48k 帧确定性双音+升包络，run 目录自足，sha 级确定——离线验证两字节相同）；
  2. acoustic package missing→**partial**：229 全 missing → 本轮 l3_deep **band_energy_summary=ready、loudness_summary=ready、stereo_relation_summary=ready**（摘录 rounds/setup_acoustic_package_status.json）；
  3. 轨道级观察返回事实：`ccb.observation_request(track.basic_energy, target=1010)` **1 次尝试**即返回 bundle.views（非 omissions）——观察面从结构性不可用转为可用。
- **选项 C（capability_blocked 判定归类）兑现亲证**：g1_r2 首次正确归类 `capability_blocked`（结构信号 `stop_reason=capability_blocked` 直击；229 同款申报被误记 no_route）。三重信号面：stop_reason 结构信号+申报词表+异步续跑回执（runtime state free_state_reasoning_loops）。既有四型+boundary 语义零回退（离线词表验证：boundary 插件文本仍归 boundary_ok、plain advice 仍落 no_route、229 三轮原文重放判定全部按预期分型）。
- choreo-pending 腿：**pending_intact**（组 1 round 5 嵌入，确认链不受新 fixture 干扰，218 LEG A 形态保持）。

### 验收门（229 原口径）——未达

- **组 1 no_route 消失：未达**（verdicts=[no_route, capability_blocked, no_route, no_route, no_route]，no_route 三连止损 r3/r4/r5；组 2/3 按 220 止损设计 skipped 未消耗）。
- **set_points 确认链端到端首绿：未达**（模型未到执行相）。
- ps1 exit 0=验证卡完成语义（执行到止损=完成），非验收门通过。

## 二、新根因锚点（观察管线 identity 绑定缺陷，越域上交）

阻塞点从 229 的「fixture 裸轨无源证据」修复后**再下移一层**：观察事实的 bake 完成了，但被 identity 绑定面判 stale——

1. **同一栈一次运行内四个 project identity**（evidence/identity_break_and_stale_l1.json 在卷）：
   - 内核 edit state（add_audio_clip 时导入 bake 携带）：`vitproj_03a02521…`
   - agent 观察链解析（最终 package upsert）：`vitproj_ce9e36c7…`
   - save_as_project 落盘工程文件：`vitproj_8093b39d…`
   - agent 启动时打开的隔离 default_project.xml：`vitproj_43f8fefc…`
2. **l1_static bake 事实在场但被判 stale（reason=project_mismatch）**：`peak_rms_summary.ref` 携带完整事实（rms_dbfs=-11.889、peak_dbfs=-4.188、crest_db=7.701、time_segments=2）——数据在盘上，因导入 bake identity（03a0）≠ 观察链 identity（ce9e）被 MergeStatus/identity mismatch 判 stale。
3. **观察面后果**：`track.1010.fast.levels` 读出全 null（rms/peak/crest/headroom/status=missing）——电平观察事实不可用。229 r3「电平观察的 levels 读取失败」在补 clip 后**依然成立**；229 r1「工程标识不一致」申报的实体即此。
4. **模型行为正确**（三轮一致）：每轮 catalog→observation_request→plugin.get_parameters 完整观察链在场；r2 诚实申报「目标证据/目标一致性 两项准入检查未过」→ capability_blocked（终态架构期望行为：不瞎写不绕道）；r1/r3/r4/r5 预算尽转异步续跑（排队文本→no_route）。
5. **修复面归属**：agent 侧工程 identity 解析/观察归一层（identity 多源不一致：内核 edit state、canonical 注册、落盘工程）——**agent 生产面，越出本卡 scripts 文件域**，按卡面停止条件 2 上交转卡。FS 准入门语义零触碰（B 否裁定维持：模型停边=证据缺失下的诚实行为）。

## 三、§8 计费与失败分类

- LLM 真实计费：**56 调用 deepseek-flash，798,418 input / 87,721 output tokens**（telemetry 逐调用在 logs/agent_llm_telemetry.jsonl；本轮遥测 usage 字段有值——229 时为空）。
- 失败分类：no_route×4（异步续跑排队形态）+capability_blocked×1（正确归类）；零 env_interrupt/confirm_broken/write_fail/lexicon_hit/wrong_tool。
- 运行次数：2 腿（leg1 setup 解析缺陷域内修正后重跑，§8 不属同版本同断点原样重跑——缺陷锚点明确且修正域内）；leg2 一次到位，止损后按设计停止，无迭代轮。

## 四、工件清单

- automation_llm_route_summary.json（含 setup 三环亲证+组终局）；automation_llm_route_console.txt
- rounds/：g1_r1..r5.json（逐轮 exchanges+verdicts+kernel 证据）、choreo_pending_g1.json（pending_intact）、setup_acoustic_package_status.json（acoustic 摘录：229 missing vs 本轮 partial/l3 ready/l1 stale 对比在卷）
- evidence/：identity_break_and_stale_l1.json（identity 断裂图谱+l1 stale 细节+levels null 观察+r2 申报原文+计费）
- logs/：agent_llm_telemetry.jsonl（56 调用）、agent_prompt_render.jsonl
- automation_llm_route_clip.wav（确定性 fixture 本体，1152044B）
- 派生态剔除按 218/222/229 先例（.vit_agent/.vit_history/.vit_derived/default_project.xml/工程文件在摘录后剔除）

## 五、拆栈

- owner 手拆 VitAgent 36660（pid 路径亲核=wt-m3obs/agent/bin 自建）/VitApp 21244（wt_228 内核）→ 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/5178 listen=0、UDP 4444/4445=0。kernel f9e3da0a 运行前后零漂移。
