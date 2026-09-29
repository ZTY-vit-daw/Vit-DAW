# PAPER-EXP-ADAPT-1：实验适配——pv1 七例 .vit 工程构建+适配公开清单+full access 路径确认

- 池序 2（P0 真栈重卡，重锚 tag experiment-baseline-2026-09-29=4b695354 已切定解锁，见尾部解锁注记；与在飞的 L1-3-GUARD-1 不同域可并行）
- 优先级 / 预估 / 依赖：P0 / 0.5–1 天 / 实验基线重锚 tag 由决策侧在 PILOT 前切定（decisions/2026-09-29-thesis-positioning-decision.md §一.1）；规格出处=coord/runs/PAPER-EXP-RECON-1/EXPERIMENT_CALIBRATION.md §三 U1 表+§四建议卡
- 模型分级：L1 / GLM 首选（真实栈操作），flash 可接材料仓侧部分
- 目标：
  1. **pv1 七例 → .vit 工程**：真实栈上经内核导入（project.import_audio_files，VspKernelReference.cpp:83-106）为 suite_v1_lm 七例各建 .vit 工程（材料仓 experiments/out/suite_v1_lm/，只读复制到隔离工作区，§10 防污染）。
  2. **适配版公开清单**：补 project_path 字段+stem_files 对象形态（{track,file} 数组），schema_version 对齐 runner 门（semantic_processor_agent_project_smoke_public_manifest.v1）——材料侧出适配清单优先，不动 runner。
  3. **full access 脚本化路径确认**：确认/打通 runner 以 full access 形态跑正式轮（参考 journey 冒测栈的权限切换驱动）；必要时（可选）sealed schema 适配。
  4. **验收=PILOT 前置就绪**：`scripts/run_free_state_d1_smoke.ps1 -PublicManifest <适配清单> -PublicCaseId pv1_p01 -PromptFlavor neutral`（及 p07 控制例）exit 0——即 PILOT 试跑 2 例通过（试跑身份不进论文，数据口径见校准报告 §四 EXP-VIT-PILOT）。
- 文件域：材料仓 experiments/（清单与工件）+ 真栈运行按 AGENTS §9 工件契约落 coord/runs/paper-exp/；**不改 runner 断言**（改=停止上交）。
- 约束：三模型凭证（GLM/GPT/Claude 经中转）由用户提供，列缺口不阻塞本卡；温度口径 0.2 硬编码在案（client.go:392）；真栈泊位声明必附。
- 验收标准：PILOT 2 例（p01+p07）exit 0 + 适配清单入库 + 工件可回指
- 停止条件：内核导入面无法为 stems 建工程（命令语义缺口）→ 实证清单上交，不擅改内核
- 领取：2026-09-29 19:30 / origin/main 92d53b07 / port/paper-exp-adapt-1（worktree D:/Vit_DAW_worktrees/paper-exp-adapt-1，基于 tag experiment-baseline-2026-09-29=4b695354 開工；主树 92d53b07 与 tag 仅差 coord 两提交，代码等价）
- 回执（2026-09-29 19:55，**blocked 上交**）：
  - **目标 1 ✅** 七例 .vit 工程真栈建成（`project.new→clear→save_as→import_folder_as_stems→DAD→save`，spv1 同款链；卡面锚点 import_audio_files 为同族文件清单形态，folder 形态语义等价已声明）；各 6 轨、DAD ready 6/6、fine evidence passed。
  - **目标 2 ✅** 适配清单入库：材料仓 `experiments/out/suite_v1_lm_runner/fixture_manifest.json`（材料仓 commit `258b1ba`，含 42 stems sha256 副本+七工程+构建报告+README）；runner `load_public_case` 七例全过。
  - **目标 3 ✅** full access 脚本化路径确认：真栈六步实证（切换/回显/同身份存活/跨身份重置 AUTHORITY-LOST-1/开工程后切换持久/恢复默认）；正式轮序列=物化副本→project.open→同身份切 full→`--reuse-existing-project` 跑 py（跳过重开保权限），现行代码零开发可行。
  - **目标 4 ❌ 阻断（证据推翻卡面前提）**：runner 材料合格门（`qualify_material`，全 flavor 无条件）按 spv1 合成材料校准，拒真实音乐 stems：**七例拒六（仅 p05 过），PILOT 两例 p01/p07 均被拒**（guitar RMS −52.2/−65.1 dBFS ≤ −45 地板；p06 另栽瞬态门、p03 栽 bass crest）。真栈两轮 exit 1 实证（responses=[]，LLM 前确定性拒绝，非概率性失败）：`artifacts/free_state_d1_s1/{20260929_194221,20260929_194244}`。无 skip 旗标、无清单侧合法绕行（裁轨破坏盲法对等）、改素材违 56b 定稿纪律——穷尽核对后上交。
  - 工件：port/paper-exp-adapt-1 `7a55a97b`+`14aaa984`（coord/runs/paper-exp/：主报告 PAPER-EXP-ADAPT-1_REPORT.md+四组工件）；真栈泊位声明在主报告 §0（本会话独占栈 19:31–19:45，GUARD-1 全程零占栈，authority 已复原 manual_confirmation）。
  - 上交点：决策侧三选项（主报告 §4.4）——①开 runner 门实验适配卡（重校准地板/fixture-set 作用域化，属断言变更须立卡）；②PILOT 例替换 p05（无控制对照，仅记录）；③改素材（违定稿纪律，仅记录）。
- 解锁注记（2026-09-29 晚，决策侧）：重锚 tag 已切定 **experiment-baseline-2026-09-29**=4b695354（含 IMPL-A/B/C 全链+今日四卡验收态；queryengine 属新模块非被测对象，符合定位决策 §一.1"新模块可在树中"）——**依赖满足，随时可领**。材料仓实锚=`C:\Users\timoz\Documents\毕业设计`（experiments/out/suite_v1_lm/ 七例已核在）。若 PILOT 开跑前混音诊断调控链再有实现改动，由决策侧裁定是否前移 tag；PILOT 数据产生后 tag 冻结不再动。
