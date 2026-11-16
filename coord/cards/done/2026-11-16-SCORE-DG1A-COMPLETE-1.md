# SCORE-DG1A-COMPLETE-1：补完 DG1-A 的 A7/A8 出声+差分断言（微卡）

- 发卡：主管 2026-11-16 晨（承接 374 验收遗留——A7/A8 当时 skipped-pending-373，现 373/375 已合入解锁）
- 池序 377；P1；依赖=373/375 已合入✓；真栈候栈位
- 目标：score_workbench_smoke.mjs 补 A7（谱面编辑→内核出声探针非静音，复用 373 DG1-B 判据）+A8（编辑前后 WAV 差分）断言；真栈轮 exit 0；三门不回退。
- 文件域：scripts/score_workbench_smoke.mjs+dev_agent_smoke 透传缝只读+coord 工件。
- 验收：DG1-A 全断言含 A7/A8 真栈 PASS 亲证。

## 领取

- 时间：2026-11-16 19:10（PC）
- origin/main hash：d5c20765（gate 2026-11-16 晨，与本地一致）
- owner：GLM-5.3 L2 执行子代理 / PC / ZCode 377 领取会话
- 分支：port/score-dg1a-complete-1；worktree：D:/Vit_DAW_wt_377dg1a
- 领取基线：HEAD=d5c20765；`git status --short` 空（worktree 全净，无领取前 diff）
- 依赖核对：373 SRC-TEMPLATE-1 终验 pass（a3f78fc4）✓；375 KERNEL-TIMESIG-1 终验 pass（65e10a79）✓；doing 四卡（ARRLOOP-AUTHBRIDGE-1/CLOSURE-ROUTE-ADAPT-1/WAVESHELL-LOADING-FORENSICS-1/LAUNCHER-ZEROFRICTION-1）与本卡文件域无交集 ✓

## 回执

- **实现 commit**：88d26a94（feat：score_workbench_smoke.mjs P5 相位+内嵌 pyzmq 内核 ZMQ helper+WAV 计量差分）@ port/score-dg1a-complete-1（已推）；工件 commit：88b4e2a3（六 run family+RUNBOOK §8/§9）同分支。
- **真栈终判**：正式轮 run_dg1a_20261116_1952 **PASS exit 0**（ps1 exit 0+腿 exit_code 0，19 断言全绿，27.5s）——A1-A6 十五项（面板手势链，120bpm fixture）+A4-template-2-4-disclosed（music_context 80bpm/2/4）+A7-edit-readback（占位 60→79 落核：edited 在场+占位消失+数恒 8）+**A7-probe-non-silent**（373 B2 逐字：l2_render_probe ready/nonzero/max_abs≥0.01/nan_inf=0）+**A8-edit-differential**（373 B4 逐字：改前 peak 14622/rms 2247.43+改后 peak 13094/rms 2207.16 双侧非静音≫阈〔500/20〕，max_abs_diff=**18334**≫500，render_done 双完成）。
- **占用/释放**：占用 9dd2192b（净空亲核 19:26 全零）→释放 f3a1a2a1（TEARDOWN_ALL_ZERO 亲证 19:53:59：TCP 六口=0+UDP 4445=0+Vit/Godot 进程=0+recents 807226F4 终核一致；装置 ps1 块还原哈希亲证在 console）。
- **attempt 台账（§8，全在 RUNBOOK §8/§9）**：attempt1=driver_defect（mjs 导入 createHash 误自 node:fs→node:crypto）；attempt2=env#1（组装 kernel_runtime 缺插件数据库→Surge 不装载「no_renderable_audio_content」→补部署 Settings 修复）；attempt3=**D4 域外产品缺陷发现**；attempt4/5=driver_defect（wavMeter fmt 偏移/width 单位，修复后离线真实 WAV 全链验证）；attempt6=正式轮 PASS。零产品断言失败、零原样重跑（每轮代码/装置有变更）、环境类 1 次未触止损线。
- **D4 域外缺陷（锚点上交决策侧，未扩修）**：非 120bpm 工程谱面面板装载崩溃——内核 get_midi_clip_data 对 80bpm 工程 8 拍 clip 披露 length_seconds=8.0（拍数冒充秒，真值 6.0s；`VitApp/Source/Service/MidiService.cpp:1758-1768`）→webui 桥 clipLengthBeats=8×80/60=10.6667（`agent/webui/src/scoreworkbench/kernel.ts:131-140`）→面板 applyPatch useMemo 渲染期 `beatsRoundedToTicks` 抛 ModelError(grid_misaligned) 无错误边界→整面板卸载（`ScoreWorkbenchPanel.tsx:401`+`model.ts:96-102`）。120bpm 下 8×120/60=16 恰整数网格不触发（374 A1-A6 未暴露原因）。最小复现：open 373 模板→/app/ 谱面页→选旋律 clip→pageerror+面板消失（run_dg1a_20261116_1936/screens/fatal.png+活栈诊断在 RUNBOOK）。
- **A7/A8 编辑缝边界声明（端测覆盖边界）**：编辑经 agent/kernel tool face `mutate_midi_notes`（373 DG1-B B4 逐字同款=本卡指定复用的判据原编辑形态；STACKGATE §3.0 装置锚认可缝「agent HTTP/tool face→内核 ZMQ」）——面板手势缝被 D4 阻断（如实记录不降级不硬凑）；A1-A6 仍为面板真实指针手势（不受影响）。渲染面=浏览器级 DOM 断言（P1-P4 截图在卷）；A7/A8 出声面=内核离线探针+整曲 WAV 对（探针 JSON/WAV 计量在 run 工件）。
- **三门**：vitest 668/668（exit 0）/npm run build（exit 0）/go build ./...（exit 0）——worktree 内跑前亲证。
- **LLM 计费上界=0 兑现**（-NoChatSmoke+VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1 死端点+全确定性 invoke/UI/ZMQ 面，零 chat 调用）。
- 工件：coord/runs/SCORE-DG1A-COMPLETE-1/（@ port/score-dg1a-complete-1；RUNBOOK §1-§9+六 run 全保真；WAV 对留 worktree 本地核验〔仓库先例不入库〕，计量/差分在 score_workbench_summary.json）。

## 验收：待主管
