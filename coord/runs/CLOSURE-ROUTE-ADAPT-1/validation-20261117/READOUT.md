# 271真实验证回执：budget_exceeded，未通过终验

- 执行owner：Codex `/root/closure271`；分支 `codex/closure-validation-271`；生产修复 `cf22db7f` 已在领取基线中。
- 唯一含LLM场景：`run_20261117_122448`，测试HEAD=`9c43de42`，现场build；完整命令见RUNBOOK第二次启动记录。前置装置轮 `run_20261117_122151` 在启动三件套前因guard格式错误停止，0 LLM，不计有效模型样本。
- **最终结果：outer exit1、Python exit6、budget_exceeded。冻结上限8次调用，实际9次；不放宽、不continue、不改话术、不再含LLM重跑。**

## 分面证据

| 面 | 实际结果 | 原始证据 |
|---|---|---|
| 真三件套 | 内核16856、Godot console12776及GUI9192、agent29260；Godot boot日志在场 | stack_before_chat.json、godot_boot.json、owned_processes.jsonl、logs/godot_closure_271.log |
| 自然路由 | closure.mode=diagnostic；closure_id=audio_closure_52bc586f5c974643 | chat_response.json、closure_evidence.json |
| 实际预算 | task/goal/run/slice四身份匹配；首slice max_turns=8、max_tool_calls=6；turns_used=8、tool_calls_used=2 | runtime_status_after_chat.json、closure_evidence.json |
| 旧断点 | **旧单轮钳制已解除**；此轮耗尽8轮后仍limit_reached/max_turns、waiting_continue，不能说“预算链路完成” | 上述实际slice字段；agent_chat_delta.log |
| 工具进度 | executed仅midi.read_notes和midi.read_clip_data；无midi.analyze_transformations | chat_response.json.executed_kernel_reply；agent_last.log timing |
| golden | 无目标分析result，**无法评估、未通过**；不拿fixture回读或历史direct-invoke替代 | offline_supplement.json（标明离线补证） |
| 账面 | deepseek-flash 9次=入口1+loop8；input153716/output6741/cache-read76800；input/output未超，但calls越8上限 | logs/agent_llm_telemetry.jsonl、summary/phases/run_billing |
| 原始最终状态 | 无no_candidate_found或capability_blocked裁定；非崩溃。失败先归跑前账面止损，不凭8轮停泊认定旧1turn缺陷重现 | chat_response.json、summary、telemetry |

同形冻结message与旧276 rerun1原intent逐字一致，见 `offline_supplement.json`。原prompt-render保留，可审查实际工具披露与每轮上下文；本卡未改生产Go、模型prompt、route或治理门。VSP注册/实时发布因不启hub有连接拒绝WARN；本腿直连ZMQ工具读取已成功，不能将该WARN擅自排除为环境中断或拿它解释目标工具未调。

## 装置缺陷及报告恢复

1. 装置轮122151：guard误要求资源marker前带斜杠，真实格式为“时间 占用 /”。修订 `9c43de42` 改为实际格式、保留raw资源，使用真实f83cca4f首条做PS5.1离线配对检查。无栈/LLM启动；原d1 exit1、recents/fixture一致。
2. 正式轮122448：finally已拆进程并恢复recents，随后PS5.1默认ANSI读取无BOM UTF8 summary导致ConvertFrom-Json失败，原d1未生成。`original_report_absence.json`记录原报告缺失与原console哈希；原console与outer_exit不改。
3. 当前 `d1_smoke_report.json` 为 **OFFLINE_RECONSTRUCTION_NOT_ORIGINAL_RUN_REPORT**：从既存summary、response、runtime、telemetry和owner清理亲证离线重建，保留outer1/py6/budget_exceeded。它不是原run成功报告。
4. 修复只在既有ps1给summary/owned-process/telemetry显式UTF8读取，并捕获summary解码异常避免中断最终报告。用本次真实中文summary注入本地失败适配器，finally字节/absence两案通过（exit0）；只验报告/恢复装置，未再起栈或发chat。

## 被测物与清理

- agent SHA256=`A5151A0631C72A667DDDE0463629D72BBA955ECFF668153C7E33997A397EB1B7`。
- kernel SHA256=`A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E`，主管复制到本worktree独立部署；不含378 D3，边界已预声明，不以此判D3。
- 部署Settings SHA256=`72F0F5112EF926ED77F59F015E6CE492142F8FDF1828E5082E970AA949EDFBB8`，已知M仅运行库，不纳本卡提交、不还原成仓库旧值。
- 前端HEAD=`d05043e11a79452b9155a89f3405668994eafdcb`，boot时已有运行纹理 `M textures/spectrum_data.exr`，保留未重置；未含394/402，非焦点卡证据。
- 源XML前后SHA256=`947E4D00886F146F7DB082215BC8A69E1727A41D5EA339FA064DEE814E0D521C`；fixture生成器前后=`881356FC0DDD99CC85619490E84D2024435C057B301EF1F16D3832B5CD60E6AF`。
- owner于 **12:26:39 +0800** 亲核：全部Vit/Godot进程=0、TCP5555/5556/5557/7878/7879/8787/9333/18787/8087监听=0、UDP4445=0；recents原字节before/after/亲核SHA256=`807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`。见owner_post_teardown.json。主管亦已独立核零及recents一致；资源释放由主管统一登记，本执行流不push main。

日志/摘要/请求响应/telemetry/prompt-render/内核原日志等证据入分支；运行时工程.vit、默认XML复制件、history/lock/cache不纳提交，保留本worktree原run目录。manifest记录全部现场工件hash供核对，不以重建结果覆盖原始缺陷。

`evidence_bytes.zip` 保存所提交run证据的现场原字节，SHA256见 `evidence_bytes.sha256.json`；离线解包校验全部归档条目与manifest相符。Git文本检出可能正规化换行，以该归档核原console等字节hash。全量diff检查在原日志自带的行尾空格处告警，保留原日志不修饰；脚本、RUNBOOK和本报告的定向diff检查exit0。ZIP为本卡证据，明确强制加入被全局zip忽略规则排除的单个文件，不含构建产物/运行工程。

本次只证明271诊断预算豁免在自然生产路由生效；目标分析工具与golden未达，终验仍欠，后续处置交主管裁定。本报告不建议以相同话术原样追跑。
