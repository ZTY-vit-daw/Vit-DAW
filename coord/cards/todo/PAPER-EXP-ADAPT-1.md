# PAPER-EXP-ADAPT-1：实验适配——pv1 七例 .vit 工程构建+适配公开清单+full access 路径确认

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
