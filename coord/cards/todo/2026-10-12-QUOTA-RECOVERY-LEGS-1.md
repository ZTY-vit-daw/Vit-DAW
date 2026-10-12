# QUOTA-RECOVERY-LEGS-1：API 额度恢复后三挂账腿聚合复跑（一窗收口）

- 发卡：GLM 主管决策侧 / 2026-10-12 晚窗（依据=rulings/2026-10-12-EVENING-BATCH1/BATCH2 两批量验收的额度阻塞挂账聚合）
- 派发确认：已确认（主管依已验收卡挂账条款发卡）
- 验收负责人：GLM 主管决策流
- 池序 102；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.25-0.5 天 / **触发条件=API 额度恢复**（恢复前不领取不占栈）
- **补注（主管 2026-10-12 深夜）：触发条件已满足**——用户更换新 API 配置（测试模型=GPT-6.1 sol），原预扣费墙解除；①首轮第一跳成功即同时验证新配置接线；②**工件必须记录模型 id**（runs 元数据+回执），并声明模型切换语境：历史对照证据（G3-ATTRIB-1 的 17 vs 6、M2-TIMEFX-1 run 105611 准入）为旧模型所产，跨模型数值不可直接比，本轮读数为新模型基线；③若新模型下话术行为异常（域选错/不触发自由态），如实记录按模型行为差异分记，不算断言失败。
- 模型分级：L1 / GLM 执行会话（PC，真栈腿）

## 三腿（一次栈占用顺序收口）

1. **D1 reverb 全链单 run exit 0**：`scripts/run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb`——补 M2-TIMEFX-1 run 111841 与 M2-TIMEFX-2 run 195448 两次额度中断腿；exit 0 即双卡挂账全收。
2. **G3 LLM 面健康链对比**：harness_ab 双模式同话术族健康完成各≥1 轮（轮级调用数口径）——补 G3-REOPEN 差异性截断腿；结果交主管作 G3 重开终判输入。
3. **mix_report 有结论正面分支**：任一真栈轮收尾后 `/agent/invoke` mix_report 断言 latest_conclusion_id 非空+与盘上 free_state_conclusions 一致——补 M1-5 正面真栈证据。

## 文件域

零生产代码（若腿 2 需脚本参数微调，限 harness_ab 既有体系）；工件落 coord/runs/QUOTA-RECOVERY-LEGS-1/。

## 验收标准

三腿各自 exit code+关键断言工件在卷；额度再中断→如实记录环境中断并保留本卡（不记失败）；腿 2 结论只交证据不自行切换 harness 缺省。

## 停止条件

额度恢复后腿 1 仍失败（非额度面）→ 按功能缺陷上交换卡。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（run ID / 三腿结果 / 端测边界声明）
- 验收：（裁定文件）
