# PROGRESS — A-F 族 ablation 预勘察（B2/B3/C1 入口要素预扫）

任务：读档零 LLM 勘察，产出预勘察报告 report.md（264 方法学应用）。
边界：纯读档（grep/读文件/git log）；零代码改动零栈零 LLM；预判不冒充运行结论。

## 步骤日志

- [x] S1 读背景：B4-ABLATION-1-pass.md §三 + af-family-health.md §三/§五
- [x] S2 定位+勘察完毕（锚点全到手，基线 HEAD=0401a6d8）：
  - 词表 capability_owner.go:225-283（B2 :227-235 12+词 / B3 :236-240 5 词 / B4 :241-247 / C1 :248-252；257 报告行号 212/221/231 系 15ef1050 基线，偏移=B4 钩插入 98fc4ca4）
  - 守卫：:53-58（B4/C1 反门）+ :107-115（explicitB4/C1InvocationMarker）
  - canary dispatch：capability_runtime_canary.go:33 handleCapabilityRuntimeCanary（B3 :109 / B4 :116 / C1 :119 / B2 :136+）；调用点 server.go:2747（先于仲裁器）
  - 仲裁器：semantic_entry.go:46 阈值 0.50 + :72-103 判据 prompt + :267 conf<0.50 强制 unresolved + :389 确定性模板；调用点 goalrunner_chat.go:261
  - 263 组 3 证据：c4-ablation-1-ablation-verdict.md §二（conf 0.22/0.22/0.32 + rejection 逐字=缺动作/目标/预期结果）
  - 264 对照：b4-ablation-1-ablation-verdict.md §二（「分析一下当前工程的低频关系」三要素明确→放行→mix.observe→done）
  - 263 裁定：M3-GROUP3-DISCRIMINATE-1-pass.md（13 行全 ordinary_agent_semantic_entry 零 message_loop）
- [x] S3 每族裸话术预判完成：B2/B3/C1 各 5 条（逐词核对不含词表词），预判理由按 264 三要素框架（动作/目标/域+预期结果）给出，全部标注「预判，待 ablation run 验证」
- [x] S4 report.md 落盘（D:/Vit_DAW/coord/runs/AF-CLAN-PRERECON/report.md）：入口链结构（锚点级）+conf 判据要素（263/264 对照）+三族预判表+四维汇总+下一拆候选建议（B3→B2→C1，交主管裁定）+诚实边界+锚点索引
- [x] S5 终态 DONE

## 终态：DONE（2026-11-07）

- 基线：HEAD 0401a6d8；本卡仅新增本目录两文件（PROGRESS.md/report.md），零代码/脚本/栈改动，零 LLM 调用。
- 产物：report.md（§一入口链 / §二判据要素 / §三-五三族预判表 / §六汇总 / §七 run 设计注意 / §八诚实边界 / 附锚点索引）。
- 遇到的问题：无阻塞。备注两项：①257 报告词表行号（:212/:221/:231）与当前 HEAD 行号有偏移，原因=B4-ABLATION-1 钩插入（98fc4ca4），报告中已双注；②工作树领取时已有他卡改动（VitApp Workspace 运行态+UX-AB 工件），与本卡文件域不相交，未触碰。
- 断点续跑：本卡一次会话内完成，未感知重派。
