# D1-SETTLE-TAIL-MAC-1 冒测终验记录

## 通过 run（本目录 d1_550a_20261003_180509/）

- 脚本：`scripts/run_d1_eq_readback_550a_smoke_mac.sh`（修复后 agent，工作树 port/d1-settle-tail-mac-1@1662943；kernel 复用 `--kernel-bin` e8c736d4c9cc4d2d…——由本 worktree 源码（origin/main 8c3ff03 VitApp/，含 df5ca28）cmake Debug 构建；tracktion=/Users/timozty/Documents/Vit-DAW/tracktion_engine @b439749；agent 现建 ae4d9f4d→run5 实际二进制 sha 见 agent_bin.sha256）
- **final_exit=0**：round 1 exit=1（模型方差断点 "D1 must contain exactly one forward mutation"，与 settle 无关）；round 2 **exit=0 PASS**——readback verified on the locked 550A scenario。
- round_2 关键断言（d1_smoke_report.json）：
  - validation.status=pass；static_eq @ track 1017，before_rev=6→after_rev=7，forward_mutation_count=1，readback_key=actual_readback_value，readback_value=-2，readback_verified=True，parameter_applied=True；
  - settle 记录落地（materiality/target_response 非 null，responses[3]/[4] 的 d1_receipt.layers.target_response）；
  - fs_settle_terminal_park={goal_status: waiting_continue, **judgment_park: true**, closure_phase: fs7_improvement_proposal}——判定泊位形态正确（FS-SETTLE-TERMINAL-1 负向不变量由 evaluator 同轮校验通过）；
  - durable 面两条 settle 检查点记录（d1_settle_checkpoint=true）均 completed——尾段真实跑完，一条被取代记录 cancelled。
- 同时满足 D1-EQ-READBACK-550A-1 终验条件（rulings/2026-10-02-D1-EQ-READBACK-550A-1-conditional.md：550A 锁定场景复跑 exit-0）。
- 诚实边界声明：本 run 证明修复后 agent 在锁定场景端到端 exit-0；apply-over-answerable-park 这一特定交错本身由单测钉住（TestAppliedBoundaryArmsSettleCheckpointOverAnswerablePark 等 3 钉），不声明本轮真实走了该交错（轮内 goal 状态序列无法从报告完全重建该点）。

## 失败尝试留痕（环境类，工件在 ~/Documents/vit-d1eq550a-artifacts/ 只读保留）

| run | 结局 | 定性 |
|---|---|---|
| d1_550a_20261002_220614 | 手动终止 | cmake FetchContent 下载 libzmq 无代理挂起（执行侧为下载注入代理的前置状态） |
| d1_550a_20261002_231042 | exit 2 | 代理下 configure/build 成功（e8c736d4 kernel 即此产物）；probe kernel CoreAudio 初始化>90s 被脚本上限杀——启动竞态 |
| d1_550a_20261003_000534 | 手动终止 | 代理 env 污染整个栈：kernel 进程外扫描 WaveShell1 600s watchdog 超时只出 1 插件→semantic index 被重建为 1 条→round1 干预 plugin_identifier not found（断点 "D1 receipt requires distinct before/after revisions"） |
| d1_550a_20261003_024011 | exit 2 | 无代理重跑仍现同一 600s 扫描超时（warm-up curl 窗口耗尽） |
| A/B 取证（/tmp/d1settle_ab，2026-10-03 晨） | 旧 kernel fc77dd75 270s→719；新 kernel e8c736d4 260s→719 | 二进制无罪；run3/4 超时=深夜机器状态瞬态（负载/温控）。机器恢复后 run5 即过 |

- 附带影响：机器本地索引 `~/.vit/plugin_semantics.json` 曾被 run3 重建为 1 条废索引；run5 的 warm-up 扫描已将其重建回 719（scan_reply.json plugin_count=719），机器态复原。
- 教训（给后续卡的移交注记）：给冒测脚本注入代理 env 会污染整栈（kernel 子进程继承）；需要代理只为 cmake 下载服务时，应只对 cmake 命令局部注入，或预取 _deps；WaveShell1 全量探测 ~4.5 分钟，与 600s watchdog 的余量在机器受压时会翻车——深夜长跑前先做一次 A/B 速测。
