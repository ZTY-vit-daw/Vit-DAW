# PC 真实运行栈占用记录

本记录是共享卡池协作的资源登记面，操作协议见 [PROTOCOL.md](../PROTOCOL.md) §2.2。
初始化“未核查”只表示尚未核对当前进程；不声明栈空闲，不接管任何现存进程。

## 当前占用

- 2026-10-14 10:56 +0800 登记 / FIX-FS-SETTLE-DEADCHAIN-1（GLM-5.3 L2 执行会话，PC，ZCode；worktree D:/Vit_DAW_wt_fsdc1r2，分支 port/fix-fs-settle-deadchain-1-rerun2@4737a0c6）：腿 D 复跑——D1 reverb `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb`（单变量=final gate 拒收反馈教学在场）；被测=worktree 构建 agent VitAgent.exe SHA256 AFEA9F30C32A56EF（4737a0c6 含腿 D 教学硬化）+主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）；LLM=deepseek-flash 思考档（~/.vit/config.json 全程未动，登记前探针 HTTP 200/0.68s、echo=deepseek-flash、reasoning_tokens=16=思考档在线）；登记前只读核对四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0（前次占用已于 01:47 释放）。

## 最近一次释放

- 2026-10-14 01:14–01:47 +0800（本机钟）/ FIX-FS-SETTLE-DEADCHAIN-1（GLM-5.3 L2 执行会话，PC，ZCode；worktree D:/Vit_DAW_wt_fsdc1r2，分支 port/fix-fs-settle-deadchain-1-rerun2@8500fd7b）：收口腿复跑两 run（验收修正裁定=正名 deepseek-flash 思考档链路首测）——run5b 20261014_011644 与 run6 20261014_012927 **均 exit 1 "acoustic materiality record is missing"，同签名 2/2 §8 停**。**与 00:10-00:47 四轮的本质差异：本轮两 run 链路均首次真栈走通 B-i 全下游**——reverb 域选择→decay 提案→插件选择/装载→语义动态确认→干预 applied（actual_readback 0.2427/track 1017 guitar、原子控制 1 次、restore_ref 在卷）→post-action 重观测；死点移至执行后 settle 段：模型两次终局尝试（final=true+交人工 A/B 试听，语义正确）experiment_materiality schema 非法（state=""、attempt≤0）被 final gate 正确拒收（acted 轮证据门语义保持，非 B-i 豁免回归）→7 续轮预算耗尽 materiality 记录缺失。run6 debug 流（agent_message_loop_debug_run6.jsonl）两条 free_state_final_gate 拒收原始载荷在卷。LLM 面：run5b 1/10、run6 3/10 次 60s 整超时（思考档时延顶满，transient_llm_error 均自恢复续轮）。另有 run5 20261014_011553=参数形态失败腿（新 worktree 传外部 -KernelExe 未配 -SkipBuild，cmake 步抛错，未启栈无进程影响，工件目录空留盘）。01:47 手动 teardown VitAgent 19764/VitApp 18660/Godot 6876，复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。占用登记 edc3dfe6+时间修正 1a0a5a97；被测=worktree 构建 agent VitAgent.exe SHA256 89D250BB0434CD1F（origin/main 8500fd7b 含 B-i+C 合入，本腿零代码改动）+主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）；工件 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/（console run5/5b/6+run_20261014_011644/012927+debug jsonl），重件留盘可回指 worktree artifacts/free_state_d1_s1/。**收口腿结论上交决策侧：exit 0 未达成；新签名=settle 段 materiality schema 非法（非准入前死链、非 G6 停滞、非 B-i 回归）；卡面升级阶梯（flash 思考档 2/2→v4-pro）由决策侧发。**

## 最近一次释放

- 2026-10-14 00:10–00:47 +0800（本机钟）/ FIX-FS-SETTLE-DEADCHAIN-1（GLM-5.3 L2 执行会话，PC，ZCode；worktree D:/Vit_DAW_wt_fsdc1，分支 port/fix-fs-settle-deadchain-1）：D1 reverb 收口腿四 run（20261014_000822/001259=B-i 构建 A0EA8B1F；20261014_003339/003935=B-i+C 腿构建 94CBFA68）——**四轮均 exit 1 且全部死于准入前模型行为，B-i seam 未被真栈触达**：run1/2=提案携带占位三件套（materiality{none,not_ready}+target_response{absent}+round_decision）且 target 与新鲜观察不匹配→admission invalid+final 吞链；run3/4（C 腿后）=提案已干净无三件套（C 腿真栈生效）但模型不做 G6 要求的轨道级观察，同签名 2/2 烧尽预算终末锁 gate-rejected。根因（环境实锤）：api.deepseek.com /models 现仅 {deepseek-flash, deepseek-v4-pro}，deepseek-chat 请求被服务端别名为 flash（探针回显 model=deepseek-flash）——档位低于基线（231930/232817 两轮 2 次准入的模型）。§8 停止，卡转 blocked 上交（模型档位裁定）。00:47 手动 teardown VitAgent 16036/VitApp 29084/Godot×3（26104/21884/16432），复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。占用登记 0854e899；被测=worktree 构建 agent（两版哈希如上，均含 B-i 修复 abd65f24，后版含 C 腿）+主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）；工件 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/（随 port 分支入库），详见卡面回执。

## 最近一次释放

- 2026-10-13 23:16–23:38 +0800（本机钟）/ TIMEFX-D1-EXIT0-DEEPSEEK-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_d1ds1，分支 port/timefx-d1-exit0-deepseek-1）：deepseek 基线 D1 收口腿三跑——首轮 23:17 参数解析失败（新 worktree 缺 temp/ fixture，未启栈即退出，无进程影响）；run 20261013_231930 与 20261013_232817 两轮**均 exit 1 "D1 must contain exactly one forward mutation"，同断点**（域选择适配生效：4/4 提案 reverb、2 次完整准入 decay_pct=3/4；新阻断=末轮空态 experiment_materiality+final=true 触发 settle 拒收吞链，interventions=0，§8 两轮同断点停止重跑，证据与候选修复菜单见 coord/runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/FORENSICS.md，卡转 blocked 上交）。23:38 手动 teardown VitApp 22628/VitAgent 8948/Godot×4，复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程名核验=0。占用登记 7913e2a2；被测=fix106 构建 VitAgent 0f9de7ed0fed4c3b（复刻至本 worktree agent/bin，哈希亲核）+主检出 VitApp.exe b6565dcf85d1da86（C++ 面零改动）；工件 coord/runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/（随 0814a3ae+9c50bc7f@port/timefx-d1-exit0-deepseek-1 入库），详见卡面回执。
- 2026-10-13 21:45–22:05 +0800（本机钟）/ KERNEL-SETTEMPO-PERSIST-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_settempo，分支 port/kernel-settempo-persist-1）：内核 edit-state 变更注册冒测两 run——baseline_20261013=基线差分取证轮（主检出 VitApp.exe B6565DCF85D1DA86，**exit 1 于 file_written 哈希门=open 形态 set_tempo 后工程文件 sha256 不变=静默跳写实锤**）；fixed_20261013=修复回归轮（worktree 自建 VitApp.exe 21:42 构建含四处 markAsChanged，**PASS exit 0 八检**：set_tempo open 形态 save/reload 往返精确 96+哈希变化/clear_project 往返 clips 归零/undo 往返 96/redo 往返 144）；两轮间 owner 手动拆栈换内核。22:05 末轮后复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0，临时工程文件已清。占用登记 bb5401c2；被测=worktree 构建 VitAgent（Go 面零改动，基线等价）；run 工件 coord/runs/KERNEL-SETTEMPO-PERSIST-1/（随 cd261982@port/kernel-settempo-persist-1 入库），详见卡面回执。

## 历史占用（上一条）

- 2026-10-13 21:45 +0800 登记 / KERNEL-SETTEMPO-PERSIST-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_settempo，分支 port/kernel-settempo-persist-1）：内核 edit-state 变更注册冒测腿（dev_agent_smoke -StartKernel -KernelExe=<被测内核> -SetTempoPersistSmoke -NoChatSmoke -NoStripSilenceSmoke；两轮=基线差分取证轮[主检出 VitApp.exe B6565DCF85D1DA86 无本卡修复]+修复回归轮[worktree 自建 VitApp.exe 含四处 markAsChanged]）；登记前只读核对四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0（前 owner FIX-TIMEFX-AXISFILTER-1 已于 bb3f9c42 释放）。

## 最近一次释放（历史）

- 2026-10-13 21:29–21:44 +0800（本机钟）/ FIX-TIMEFX-AXISFILTER-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_fix106，分支 port/fix-timefx-axisfilter-1）：挂账收口腿复跑两轮——LLM 预探测 21:27 HTTP 200/0.76s 服务恢复后启栈；run 20261013_212900（=reverb 第 12 轮）与 20261013_213535（=第 13 轮）**均 NOT_EXERCISED exit 3、selected_domains 同为 broadband_compression**（引导话术在场"残响拖沓…混响尾巴问题"，模型自主选择持续偏压缩面，提议锚点 "compressor threshold move within +/-10 dB on a dual-channel surface"）；轮内 LLM 全健康（message_loop.llm 1.35–4.4s err=false，非服务故障、非本卡七处修复面——开放运行未进入 reverb 准入，修复面未被触达）；同断点连续两次按 §8 停止原样重跑，delay 加分腿（以 exit 0 为前提）未跑；21:44 手动 teardown VitAgent 11892/VitApp 9112/Godot，复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0（含 godot frontend=0）。占用登记 2a511abc（登记行时间误写 13:31，实际本机钟 21:29 run 启动前——此处修正，同 M2-TIMEFX-2 修正先例；根因=把 UTC 13:27 误作本地钟）；被测=worktree 构建 agent VitAgent.exe SHA256 0f9de7ed0fed4c3b（登记前哈希亲核=挂账轮末版含修复 cf5bf33f）+主检出 VitApp.exe b6565dcf85d1da86（C++ 面零改动）；工件 coord/runs/FIX-TIMEFX-AXISFILTER-1/（console_d1_reverb_12/13.txt+d1_smoke_report_20261013_{212900,213535}.json+console_agent_last_213535.txt+artifacts_index/slim×2，随 main 入库），重件留盘可回指 worktree artifacts/free_state_d1_s1/20261013_{212900,213535}/。**收口腿结论上交决策侧：exit 0 未达成，根因=当前配置模型（deepseek）对该 fixture 的域选择确定性偏 broadband_compression，三笔销账条件未满足。**

## 历史占用（上一条）

- 2026-10-13 13:31 +0800 登记 / FIX-TIMEFX-AXISFILTER-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_fix106，分支 port/fix-timefx-axisfilter-1）：挂账收口腿复跑——D1 reverb `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb`（LLM 预探测 13:27 HTTP 200/0.76s 服务恢复）+可选 delay 族加分腿同占栈；被测=worktree 构建 agent VitAgent.exe SHA256 0f9de7ed0fed4c3b（与挂账轮末版二进制逐位一致=含修复 cf5bf33f，登记前哈希亲核）+主检出 VitApp.exe b6565dcf85d1da86（C++ 面零改动）；登记前只读核对四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。

## 最近一次释放


- 2026-10-13 09:50–18:31 +0800（本机钟）/ FIX-TIMEFX-AXISFILTER-1（GLM-5.3 PC 执行流，ZCode 本会话；worktree D:/Vit_DAW_wt_fix106，分支 port/fix-timefx-axisfilter-1）：D1 reverb 全链冒测 11 run（095626=fixture 缺失环境腿；100927/102300/103532=三处运行时缺口逐层修复轮——post-load surface/动态族 switch/apply 分派；105238=**干预落账轮 parameter_applied=True**+settle 尾被 11:00 LLM 60s 超时吃预算；110320/111010/112913=NOT_EXERCISED×3 上游 LLM 服务不稳（连续 60s timeout 日志）；180315=服务端原文 "Our servers are currently overloaded"；181843=**干预二次复现 applied=True**+post-action CCB 观察被 18:25/18:26 连续 60s 超时阻断）——7 处修复实栈两轮复现有效，exit 0 未达成按 §8 止损收口上交；18:31 手动 teardown VitAgent 24012/VitApp 28860/Godot 25020，复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。占用登记 bb3f9c42；被测=worktree 构建 agent（末版 0f9de7ed0fed4c3b 含全部修复）+主检出 VitApp.exe b6565dcf85d1da86（C++ 面零改动）；工件 coord/runs/FIX-TIMEFX-AXISFILTER-1/（随 port 分支入库），详见卡面回执。

## 最近一次释放

- 2026-10-12 22:08–22:26 +0800（本机钟）/ M3-KERNEL-1（GLM-5.3 PC 执行流，L2 内核 C++，ZCode 会话；worktree D:/Vit_DAW-wt-m3k1，分支 port/m3-kernel-1-automation）：automation 三命令真栈冒测两轮——220826 首轮 exit 1=设计发现腿（clear_region 用引擎 removePointsAndCloseGap 时边界值补偿锚点 AutomationCurve.cpp:630-635 使"区域覆盖全曲线"后仍余 2 点，改 removePoints 纯区域删除后修复）；221700 复轮 **PASS exit 0**（13 检全过：get 空+time_base 披露/越界 fail-closed 三反例零写入/写三点/逐点回读一致/区域清除/回读空/VSP CAS 反例错 base_revision 拒绝且曲线不动/无 CAS 基准写/cleanup）；两轮栈均 owner 手动拆除（脚本完成态不自带 teardown），22:26 复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub 进程=0。占用登记 141635df；被测=worktree 自建 VitApp.exe（本卡改内核 C++，22:16 增量重建含 clear_region 修正）+worktree 构建 VitAgent；工件 coord/runs/M3-KERNEL-1/automation_curve_smoke_2026101{2_220826,2_221700}/（随 port 分支入库）。

## 历史占用（上一条）

- 2026-10-12 22:35 +0800 登记 / M3-KERNEL-1（GLM-5.3 PC 执行流，L2 内核 C++，ZCode 会话；worktree D:/Vit_DAW-wt-m3k1，分支 port/m3-kernel-1-automation，协调分支 coord/m3k1-stack）：automation.get_curve/set_points/clear_region 三命令真栈冒测腿（dev_agent_smoke -StartKernel -KernelExe=worktree 构建 VitApp.exe【本卡改内核 C++，必用自建二进制】+ -RestartAgent worktree 构建 + automation_curve_smoke.py 新腿：get 空→越界 fail-closed→写三点→逐点回读→区域清除→回读空→VSP CAS 反例）；登记前只读核对四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub 进程=0（前 owner QUOTA-RECOVERY-LEGS-1 已于 e148249a 释放）。

## 最近一次释放

- 2026-10-12 21:09–22:35 +0800（本机钟）/ QUOTA-RECOVERY-LEGS-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_qrl1，协调分支 coord/qrl1-b，工件分支 port/quota-recovery-legs-1）：三腿五 run+一预检拦阻轮——①D1 reverb run 211011 **exit 1=非额度面功能缺陷上交**（no_axis_covered 门，9ae3aa91 修复首轮真栈验证不充分；额度面 0 错误+新配置 gpt-6.1-sol 接线验证过；栈 D1 后手动拆除 21:3x 核净）②harness_ab run 2122_harnessab **exit 0**（双模式健康链 quota_blocked=0；脚本 berth 自拆）③M1FiveThreeView 两轮 214605/215718 **均 exit 0 PASS 但 settle 面走诚实无记录分支**（正面分支新模型 0/2 未落交决策侧；首轮 213641=编辑器在场被清场预检拦，用户授权关闭后重跑；两轮均脚本自拆）。22:35 复核四端口 7878/5555/5556/8787 监听=0、VitApp/VitAgent 进程=0、Godot=0（编辑器 30420 由用户手动关闭）。占用登记 e1631ad9；被测=worktree 构建 agent（HEAD e1631ad9）+主检出 VitApp.exe B6565DCF85D1DA86（同挂账轮二进制）；工件 coord/runs/QUOTA-RECOVERY-LEGS-1/（精简件随 port/quota-recovery-legs-1），详见卡回执。

## 最近一次释放

- 2026-10-12 19:38–19:57 +0800（本机钟）/ M2-TIMEFX-2（GLM-5.3 PC 执行流，ZCode PC 会话；worktree D:/Vit_DAW_wt_m2_timefx2，分支 port/m2-timefx-2，实现 25a2b91a+9cd14877）：①timefx 探针冒测两轮（reverb 195100 / delay 195128 均 PASS exit 0 28/28 门；迭代修复三轮为脚本面缺陷——rack x/y、域感知写参、非 JSON 事件跳过）②D1 reverb 复跑挂账腿 run 20261012_195448（exit 3=API 预扣费额度失败余 0.000190，额度仍未恢复环境中断如实记录，链路本身全通）；timefx 冒测栈由 ps1 finally 自拆，D1 栈残留 VitAgent 23576/VitApp 21324/Godot 30332 由 owner 手动拆除，19:57 复核 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。占用登记 0fc9e930（登记时间在占用行误写 21:05，实际提交约 19:38 于首轮 run 194136 前——此处修正，同 M1-3 修正先例）；工件 coord/runs/M2-TIMEFX-2/（随 port/m2-timefx-2 入库），详见卡回执。**遗留：M2-TIMEFX-1 NL 全链挂账腿继续挂 [等待用户]（API 额度恢复后复跑需重新登记占用）。**

## 最近一次释放

- 2026-10-12 约 18:58–19:37 +0800（本机钟）/ G3-REOPEN-COSTCMP-1（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_g3rc1，分支 port/g3-reopen-costcmp-1）：①三轮启栈失败腿（18:58/19:17/19:22，取证后钉死=dev_agent_smoke 相对 -RunArtifactsDir 使 journey berth 的 VIT_PROJECT_XML 相对透传→内核按自身 CWD 拼双段路径→service startup failed exit 0 @~1.6s；包装器捕获退出码+部署目录内核日志留证；脚本已硬化修复）②对比轮两 run=**PASS exit 0**（20261012_192737_run1 / 20261012_193106_run2，双模式各 1 base+3 族轮+pull confirm/budget 腿；LLM 面被 API 额度耗尽差异性截断=环境中断如实记录，probe 物理计量完整）；两 run 均脚本自 teardown（agent/kernel stopped），19:37 后复核四端口 7878/5555/5556/8787 监听=0、VitApp/VitAgent 进程=0。占用登记 fcef17a1；被测=worktree 构建（agent @ port/g3-reopen-costcmp-1 含脚本扩展；内核复用主检出 VitApp.exe B6565DCF85D1DA86 同版二进制，C++ 面零改动）；工件 coord/runs/G3-REOPEN-COSTCMP-1/（随 port 分支入库精简件），详见卡回执。

## 历史释放（上一条）

- 2026-10-12 约 19:40–23:30 +0800（本机钟）/ M2-TIMEFX-1（GLM-5.3 执行流，ZCode PC 会话，M2 时域线；worktree D:/Vit_DAW_wt_m2_timefx1，分支 port/m2-timefx-1）：①参数面探测两轮（内核 ZMQ 直连 rack_add_node 装载 Pro-R/Timeless 3→get_plugin_parameters，探测轨即建即删）②in-agent 认证 job 四轮全 completed（两族各两轮，apply/readback/restore/快照核对/临时轨清理自含）③D1 冒测四轮（105611/110302/110817=功能性门暴露三轮各自修复；111841=API 额度耗尽环境中断）——各轮 dev_agent_smoke 启栈、人工/脚本拆栈交替；23:30 手动 teardown：VitApp/VitAgent/Godot 全停，复核端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent 进程=0。占用登记 a7fccf41；被测=worktree 构建（agent 含全部修复；内核复用主检出 VitApp.exe 同版二进制，C++ 面零改动）；工件 coord/runs/M2-TIMEFX-1/（随 port/m2-timefx-1 入库），详见卡回执。**遗留：API 额度耗尽阻塞 NL 全链烟测末轮（卡面 [等待用户]）——额度恢复后复跑一轮需重新登记占用。**

## 最近一次释放

- 2026-10-11 22:55–23:10 +0800 / FE-GUI-SHRINK-IMPL-1（GLM-5.3-Flash PC 执行流，ZCode flash 会话；前端仓主检出 D:/Godot/project/vit-daw-frontend，分支 port/fe-gui-shrink-1）：①首轮 dev_agent_smoke -StartKernel -StartUI -MixSmoke 于 Mix observe 腿中断（脚本既有 StrictMode 缺陷，非功能失败；VitApp/VitAgent 残留已由本 owner 手动清理核净）②脚本修复后复跑=**PASS exit 0**（全腿绿含 mix.observe obs_20261011T142837_56c255d1b6ff）③补充 godot 窗口启动检查（D3D12 真实渲染，start_page 就绪零 SCRIPT ERROR）④23:10 手动 teardown：Godot×2/VspHub/VitAgent(19076)/VitApp(2548) 全停，复核端口 7878/5555/5556 监听=0（仅 TIME_WAIT 残迹）、VitApp/VitAgent/Godot/VspHub 进程=0。占用登记 65cf890c；工件 coord/runs/FE-GUI-SHRINK-IMPL-1/（随 e70e817d@主仓 port/fe-gui-shrink-1 入库），详见卡回执。注：dev_agent_smoke 完成态不自带 teardown（设计如此），owner 需自行拆栈——本轮按此执行。

- 2026-10-11 22:04–22:11 +0800（本机钟）/ MIXLAYER-M1-4（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_4，分支 port/mixlayer-m1-4）：fs_largeproject_smoke.ps1 -D1D3Observation 一 run=**PASS exit 0**（D1D3OBS 五门全过：观察报告 schema 行 d1d3.premaster.observation.v0+D1/D2/D3 三面行+D3 参考曲命令缺口登记行+只读声明+A4 固定编排标记族零命中；A1/A4 pass；A2/A3 软记录，classification=capability_blocked 与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线——M1-2 ruling 已接受同语义；栈自 teardown torn_down=true，agent pid 5956 / kernel pid 17452 已停）；22:12 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 8e0e52ad（登记时间误写 22:35，实际提交 22:04:34、run 启动 22:06:35——此处修正，同 M1-3 修正先例）；被测=工作树（=实现 commit 7a31c6d5 内容，run 时实现未提交，head.txt=93e23954 在卷）；run 工件精简件随 7a31c6d5@port/mixlayer-m1-4 入库（coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261011_220635），详见卡回执。

- 2026-10-11 21:04–21:15 +0800（本机钟）/ MIXLAYER-M1-5（GLM-5.3 PC 执行流，ZCode 会话 sess_155c5ae2；worktree D:/Vit_DAW_wt_mixlayer_m1_5，分支 port/mixlayer-m1-5）：fs_largeproject_smoke.ps1 -M1QueueRoundtrip -M1FiveThreeView 两 run——210405=**首轮 PASS exit 0**（M15 八门全过：mix_report invoke ok+schema+最近 settle 结论行+queue 节点状态行 23 节点+E1 延期可见+双折叠一致 0=0+settle 行真实性+A4 零标记；M1Q 四门+A1/A4 pass；round 形态 goal_terminal 无 settle→settle 行走无记录诚实分支）；210903=第二轮同形态 PASS exit 0（§8 止损：不再加跑，有结论分支归单测覆盖）；两轮均脚本自 teardown（ports free, processes gone），末轮后实测端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 435ebd4b；被测 HEAD=8e9ce5f1（实现 commit，两轮 head.txt 在卷）；run 工件精简件随 6cc0ec06@port/mixlayer-m1-5 入库（coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_2026101{1_210405,1_210903}），详见卡回执。

- 2026-10-11 20:49–21:16 +0800 / MIXLAYER-M1-3（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_3，分支 port/mixlayer-m1-3）：fs_largeproject_smoke.ps1 -C3Observation 一 run=**PASS exit 0**（C3OBS 三门全过：观察报告 schema 行+只读声明+A4 标记族零命中；A1/A4 pass；A2/A3 软记录，classification=capability_blocked 与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线——M1-2 ruling 已接受同语义；栈自 teardown torn_down=true）；21:16 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 e7645a65（登记时间误写 21:05，实际占用自 20:49 run 启动——此处修正）；run 工件 coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261011_204949（随实现 commit 入库），详见卡回执。

- 2026-10-11 20:05–20:16 +0800 / MIXLAYER-M1-2（GLM-5.3 PC 执行流，ZCode 会话；worktree D:/Vit_DAW_wt_mixlayer_m1_2，分支 port/mixlayer-m1-2）：fs_largeproject_smoke.ps1 -M1QueueRoundtrip 两 run——194717=首轮（M1Q 读面过/注释命令被语义入口吞→chat 拦截修复+门槛语义补丁，round 形态 improvement_proposal 与 FS-LARGEPROJECT-SMOKE-1 基线同族）；200448=**末轮 PASS exit 0**（M1Q 读→标注 E1 延期→重读往返四门全过+A1/A4 pass+A2/A3 软记录，栈自 teardown）；20:16 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 cf46f950；run 工件 coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_2026101{1_194717,1_200448}（已随实现 commit 6511d700 入库精简件），详见卡回执。

## 历史释放（M1-2 期间覆盖的上一条）

- 2026-10-11 11:05–12:18 +0800 / MIXLAYER-M1-1（GLM-5.3 PC 执行流，ZCode 会话 sess_30083b31；worktree D:/Vit_DAW_wt_mixlayer_m1_1，分支 port/mixlayer-m1-1）：settle_deliver_smoke.ps1 -M1Survival 共 9 run（105423/110451=judgment_not_armed 模型随机分支×2 触发止损→脚本场景修复；111730/112607=判定命中但行面失败→取证定位原卡 $NeedleSettle/$Round2Text B64 非法 UTF-8 遗留 bug 并修复；113450=M1 门通过但重启探针暴露 draft 隔离→门改跨会话读回；114450/120538=无 settle 形态；115519=M1 门完整通过（当轮 exit 1 系行面坏 needle）；121359=**末轮全链 exit 0**：判定 POST→M1 记录落盘+新会话黑板读回(improved)→行面 1 行→round2 无 stale→重启探针留痕 empty_draft_isolation），各 run 均脚本自 teardown；12:18 后复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 32fcab06；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）；run 工件在本机 coord/runs/SETTLE-DELIVER-SMOKE-20261011_*（worktree 内），详见卡回执。

## 历史释放（归档）

- 2026-10-10 20:16–20:50 +0800 / FS-LEDGER-PERSIST-1（GLM-5.3 PC 执行会话，ZCode D:\Vit_DAW；worktree D:/Vit_DAW_wt_fslp1）：三 run（201713/202326=取证插桩复现轮，203348=修复后回归轮，均脚本自 teardown：末轮 console "stack released: ports free, processes gone"），20:50 复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0。占用登记 0d3d90cc；实现 commit db6f2b11@port/fs-ledger-persist-1；回归 verdict FAIL(A3 模型行为) 但 A2 站图 PASS=缺陷修复面，详见卡回执。E:\ 三轮 mid-check 干净。

## 历史释放（归档）

- 2026-10-10 18:30–19:15 +0800 / FS-LARGEPROJECT-SMOKE-1（GLM-5.3 PC 执行会话，ZCode D:\Vit_DAW；worktree D:/Vit_DAW_wt_fsLPS1）：四 run（183305/183914=工具性中止，栈由执行侧人工拆除核清；184500=$pid 崩溃后人工拆除核清；185519=末轮脚本自 teardown torn_down=true）；19:15 复核端口 7878/5555/5556 监听=0、VitAgent/VitApp 进程=0、Godot vit-daw-frontend=0。占用登记 de1d441b；实现 commit 2291d055@port/fs-largeproject-smoke-1；verdict FAIL（A1/A4 pass，A2/A3 fail）详见卡回执。期间 185519 一轮干净自释放，前三轮人工拆除均在 ABORTED_NOTE/RUN_NOTE 留痕。

- 2026-10-09 21:41–21:47 +0800 / G3-ATTRIB-1（GLM-5.3 执行会话，D:\Vit_DAW_wt_g3a1）：harness_ab 四相 run G3-ATTRIB-1/20261009_214138_harnessab SCRIPT-LASTEXITCODE=0；释放证据=脚本 berth teardown（agent pid=23600 / kernel pid=18636 已停）+ 21:52 复核端口 7878/5555/5556 监听=0、VitApp/VitAgent 进程=0；期间无脚本缺陷重跑、无栈残留。登记提交 82a654ad（被测 HEAD c4c015c5，rebase 后实现提交 hash 以分支为准）；内核复用主检出 B6565DCF85D1DA86（C++ 面零改动）。

## 历史占用（归档）

- 2026-10-09 19:47–20:10 / L1-5-IMPL-D（GLM-5.3 执行会话，D:\Vit_DAW_wt_l5d）：harness_ab 双模 run L1-5-IMPL-D/20261009_195558 exit 0；已释放（20:10 核对进程=0、端口 7878/5555/5556 监听=0）；详见卡回执。

