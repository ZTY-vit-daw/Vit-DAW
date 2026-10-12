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

- 领取：2026-10-12 21:07 +0800 / origin/main=173b417a / owner=GLM-5.3 PC 执行会话（ZCode，L1 真栈腿） / 协调分支=coord/qrl1，工件分支=port/quota-recovery-legs-1 / worktree=D:/Vit_DAW_wt_qrl1 / 领取提交=0280e14c
- 回执：（工件全部落 coord/runs/QUOTA-RECOVERY-LEGS-1/，精简件随 port/quota-recovery-legs-1 入库，重件留盘可回指）
  - **腿① D1 reverb：exit 1 = 非额度面功能缺陷，按停止条件上交**。run=artifacts/free_state_d1_s1/20261012_211011（console=leg1_d1_reverb_console_212026.log；前两跳失败为调用形态：worktree 缺 temp fixture→显式 -PublicManifest 指主检出；缺 agent 预构建→go build 预构建）。链路读数：G1-G7 准入全绿+域选对（selected_domains=[reverb]）+自由态触发（loop 持久化至 fs6_target_confirmed）+冻结轴=单主轴 decay_time（正是 9ae3aa91 修复的目标形态），但轮内治理过滤输出空集→stop=no_axis_covered_candidates→0 干预→"exactly one forward mutation" 门 fail。关键定性：旧模型 110817 已命中同门（修复后无再跑机会），本轮=该修复首轮真栈验证=**不充分**；且独立面全绿——pcactl query-v2（Pro-R/reverb/decay_time）eligible=promoted+现场指纹匹配（leg1_d1_211011/pcactl_query_v2_proR_decay_time.json），~/.vit 白名单 reverb 节+v2 认证 promoted 覆盖 decay_time+semantics 条目全在（machine_state_snapshot.json）→缺陷在运行时过滤路径非机器数据。**额度面干净：0 quota 错误，新配置首轮第一跳成功=接线验证通过**。→ M2-TIMEFX-1 run 111841 / M2-TIMEFX-2 run 195448 两笔挂账**不可收**，需决策侧开修复卡。
  - **腿② harness_ab：exit 0 收口**（run=2122_harnessab）。push 1 基线+3 族轮全健康分类；pull 基线轮 judgment_failed（stop=durable_checkpoint_persist_failed，1 条如实分记非额度非模型面）+3 族轮健康（done/done/capability_blocked）；confirm 往返 exercised=1hop→judgment_terminal；budget 触发+accounted_separately=true；**双模式 quota_blocked=0**（G3-REOPEN 差异性截断腿恢复）。轮级调用数三口径（analysis.json）：push 全调用 10（基线 3+族 3/3/1，probe 839ms）vs pull 6（各轮 1，probe 0ms）。证据交主管作 G3 重开终判输入，**未切换 harness 缺省**。
  - **腿③ M1FiveThreeView：两轮均 exit 0 PASS（M1Q 四门+M15 八门+A1+A4），但 settle 面均走诚实无记录分支**（timeline=0=queue sources=0，latest 空；run=fs_largeproject_smoke_20261012_214605/215718，run_report 已拷入 leg3_m15/）→正面分支（latest_conclusion_id 非空+与盘上一致）新模型 0/2 未落；旧模型对照：M1-2 194717 曾落 loop_closure_settle(blocked) 1 次、M1-5 两轮亦未落——轮形=模型随机分支，按模型行为差异分记不算断言失败；§8 止损（两轮同形态不再加跑）。**M1-5 正面真栈证据仍欠**，交决策侧；确定性生产者已勘察（settle_deliver -M1Survival 确定性落 loop-closure 结论，接 mix_report 需补存盘/开工程管线，草案存储重启约束见其脚本头注）。首轮 213641=Godot 编辑器在场被清场预检拦（BOUNDARY-PERSIST-1），用户裁定关闭编辑器后重跑（本会话 AskUserQuestion 记录）。
  - 补充：腿①栈活时 /agent/invoke mix_report=诚实空分支且盘面↔报告一致（leg1_post_round_mix_report.json）。
  - **模型 id：gpt-6.1-sol @ https://www.hejuapi.com/v1**（~/.vit/config.json 机器全局，2026-10-12 20:51 起）；所有 run 工件已记（run_model_metadata.json×3+machine_state_snapshot）。
  - **跨模型不可比声明**：G3-ATTRIB-1 的 17vs6、G3-REOPEN 的 886/950ms、M2-TIMEFX-1 105611 等历史读数均旧模型所产，与本轮读数不可直接比；本轮=gpt-6.1-sol 新基线。
  - 被测二进制：agent=worktree 构建（HEAD e1631ad9=领取基线+协调提交）；内核=主检出 VitApp.exe（sha256 前缀 B6565DCF85D1DA86，2026-10-06 22:02 构建，与 195448/105611/192737/193106 挂账轮同一二进制；-SkipBuild 按 §9 记录在案）。
  - 端测边界声明：三腿均真实三件套栈端侧烟测（内核+agent+Godot，退出码直取）；未覆盖面=webui 渲染面与用户旅程（本卡零渲染面改动）。无额度中断轮；栈三起三拆（D1 手动拆除核净 21:3x；harness/M15 脚本自拆+复核端口 0/进程 0）。
- 验收：（裁定文件）
