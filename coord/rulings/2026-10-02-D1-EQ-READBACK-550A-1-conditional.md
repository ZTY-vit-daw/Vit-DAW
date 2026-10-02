# Ruling：D1-EQ-READBACK-550A-1 — 主面 pass（conditional：exit-0 终验挂 D1-SETTLE-TAIL-MAC-1）（2026-10-02 决策会话）

## 验收核验

1. **取证亲读**（runs/D1-EQ-READBACK-550A-1/forensic1_20261002_111423/）：FORENSIC_CONCLUSION.md 11 档步进表（内核 getValueForState 步位+dB 非线性 -12..+12 全表）、sweep2_results.tsv 17 例吸附实测（离格写→吸附最近档、即时回复与 surface 读回一致）、Q3 连续对照（float32 回显 ~1e-8 << 1e-4 恒过）、五步失败机理还原与 8 轮归因表（FS-SETTLE-TERMINAL-1 §6）逐点吻合——定性**"读回形态不匹配（写参已生效）"**采信；内核零改动结论成立（写参链路无缺陷）。
2. **修复 diff 亲核**（5693f3f）：eqSteppedGrid 双要素门（is_discrete+num_steps≥2+labels 全表可解析，不满足回退连续管线）+非单调拒绝；步位=内核自身 step value（与读回同源比对）；绝对路径最近可达档直写；delta 路径跳 slope 探针（离格探写=网格伪影）+eqSteppedPlan 披露 requested/achieved/deviation_db（audit 不静默）；Apply 不变式——**1e-4 归一化等式原样保留**（staticeq_vsp.go:337/:341/:394 三处 applied_unreconciled 出口未动）+步进物理校验=落计划档位（0.05dB）+写参未生效照旧拦截——**fail-closed 语义不变核实**。文件域=executionports 四文件+两测试+烟测脚本，卡面域内。
3. **我方复跑**：cherry-pick 四连（5693f3f/085f2b3/5605a06/9e7a03a）于 origin/main=6802b11 干净落地零冲突；go build exit 0；**全量 87 包 0 FAIL（exit 0）**；blob gofmt 5/5。
4. **真栈双层证据亲读**（run d1_550a_20261002_182113）：
   - 内核面（kernel_log_evidence_excerpt.txt）：set_plugin_param 写 0.400000005960464（-2dB 档步位）→new/display "-2 dB"、actual_normalized_value 逐位相等——修复前同 delta 动作写 0.4166667（离格）必败，对照形态在取证报告。
   - 回执面（原始 round_1 报告，594KB）：responses[2]/[3] 与 persisted_loop 双处 interventions——technical_application=**applied**（修复前该形态 100% applied_unreconciled）、双通道 requested_normalized==actual_normalized==0.400000005960464、actual_readback_value=-2、revisions 3→4 幂等键在案。
5. 纪律合规：白名单收窄备份/陷阱恢复核实；§9 泊位（与 PLUGINLIST 复跑端口相遇按等待自然释放处置，预检 RED 记环境类不计轮次）；端测边界声明如实。

## 归档补正（决策侧随验收执行）

- 入库 run d1_550a_20261002_182113 的 round_1/round_2 两份报告原为**轮2 报告重复两份**（md5 同 c17265e8，started_at=10:36=轮2 窗口），含 applied 回执证据的真轮1 报告（594KB，error="acoustic materiality record is missing"）未入仓。已用原始工件（~/Documents/vit-d1eq550a-artifacts/d1_550a_20261002_182113/round_1/，保留作回指锚）补正入库 round_1 文件；round_2 文件即原件不动。**教训入档：多轮 run 归档须逐文件核对 md5/时间戳与轮次对应关系。**
- 修复证据不受影响：内核摘录在仓内原本即真；回执面证据经原始工件核验后随本补正入仓。

## exit-0 未达成归因与裁定

- 轮1 error="acoustic materiality record is missing"（settle 尾段调度切片 7 分钟未落 materiality/target_response，round 停 observing/post_action_evaluation，park 已到）；轮2 error="D1 loop projection is missing"（早期变体）。本卡 diff 零触碰 chat/settle 域；PC 同 evaluator 全过、Mac 首跑暴露（mac 时序敏感嫌疑，未证）；取证能见度受 AUTH-RESTORE-LOGSPAM-1 刷屏阻碍（如实申报采信）。执行侧止损分类缺陷（status 恒 fail 误判同点）已自纠，不掩盖轮次事实。
- **裁定：主面 pass（conditional）**——卡面缺陷（550A 读回 100% applied_unreconciled）已修复且有内核+持久化回执双层真栈证据+全量回归；验收项"真栈 550A 锁定场景 exit-0"因域外缺陷未走满，**终验条件=D1-SETTLE-TAIL-MAC-1 落地后 550A 锁定场景（或等价 d1 冒测）复跑 exit-0**。实现随本裁定 cherry-pick 合 main。

## 遗留移交

- **新卡 D1-SETTLE-TAIL-MAC-1（P1）入池 todo**——settle 尾段停滞取证修复；本卡终验挂该卡验收。
- worktree 清理（决策侧本次执行）：/tmp/pluginlist-redeploy-src、/tmp/pluginlist-redeploy-wt（PLUGINLIST ruling 遗留移交项）+ ~/Documents/Vit-DAW-d1eq550a-1（本卡执行 worktree，port 分支已推远端）。
- 原始工件目录 ~/Documents/vit-d1eq550a-artifacts/ 保留（回指锚，非归档）。
