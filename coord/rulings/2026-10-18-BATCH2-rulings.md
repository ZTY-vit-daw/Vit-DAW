# 2026-10-18 午窗 批量二裁定（133 缺陷修复终验 / 90 LOUDNESS-B 补证+终验）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18 午窗派发两卡 + 批准「现在派发第三执行流」。
- 基线：origin/main=3d950231（验收前）；本批合入后 main=a1b4238c（133 修复 4 笔 + 90 修订腿 1 笔 cherry-pick 入合入链）。
- 执行侧：DSH 团队执行流 exec-lw-fix（133 修复与 90 补证腿 run）；协调面由 12:00-18:00 决策流单写。

## 一、FIX-LW-CONFIG-PERSIST-1（133）——**终验 pass**

- 交付：分支 **port/fix-lw-config-persist-1@2097653b**（实现 36323529：仅 `agent/internal/chat/server.go` + 新增 `loudness_watch_persist_race_test.go`，+241/−14；证据 30ead248/ba7bb407/2097653b）。
- **Root cause（决策侧采信，证据链亲核）=请求边界 ownership 缺失 + deferred persist 时序**：`handleInvoke` 的 loudness configure 分支先 `writeJSON` 返回、后 `defer syncCurrentProjectWorkspace` 持久化；D1 post-run 上下文 continuation scheduler 每 250ms 的 disk replay（`reloadActiveRuntimeState`→`restoreProjectAgentRuntimeStateLocked` :7539-7554）在「内存已 off、deferred persist 未落地」窗口用旧持久快照（enabled=true）整体恢复内存，随后 persist 把回退值写回磁盘 → configure off 返回 ok+false 后下一 status 即 enabled=true。普通工程上下文 state 小（KB）replay 毫秒级故窗口不出现；D1 state≈20.8MB，实测 configure 14775ms/status 8298ms vs 普通 33-45ms。同族先例=AUTHORITY-LOST-1（authorityModeExplicit）。
- **修复（最小面）**：仿 `authorityModeExplicit` 增加进程内 `loudnessWatchConfigExplicit` latch——显式 configure 后置位；`restoreProjectAgentRuntimeStateLocked` 在 latch 下保留内存显式配置（其余状态照常重放）；同一 workspace identity 切换清 latch（不跨工程泄漏）；不持久化、重启仍以磁盘为准；enabled=true 正常持久化/恢复路径不变。
- **决策侧独立复验（亲做）**：
  1. diff 亲读：+241/−14 仅 2 文件、无越域；latch 语义与 AUTHORITY-LOST-1 同族一致，identity 切换清理到位。
  2. 单测证据链亲核：`pre_fix_failure.txt` 确定性失败签名（`after scheduler replay/deferred sync: enabled=true, want false` 双向）→ `post_fix_pass.txt` 修复后 PASS；`go_build_exit.txt` exit 0；`go_test_all.log` 95 包全 ok、FAIL 行 0。
  3. 真栈工件亲读（run2=20261018_124639）：`loudness_watch_gates.json` 六项 `gate_results` 全 true，含 **config_off_silent=true**（checks 7→8→8 不推进）、runner_exit=0、E1 样本在卷；被测 agent SHA256=**5A25452ADCCE89…**（fix 构建）/kernel SHA256=**B6565DCF…**。
  4. 栈现场亲核（12:54）：四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent=0。
- 边界采信：真栈成功样本 n=1（存在性证据；稳定性由确定性单测+既有基线背书）；run1=20261018_124040 为 D1 腿模型随机分支失败（materiality missing/budget exhausted，非 gate4 签名），执行侧如实取证后重跑、§8 未烧第三轮——采信。
- 卡 133：**done / 验收 pass**。

## 二、LOUDNESS-B-TRIGGER（90）——补证腿 pass + **终验 pass**

- 补证 run=**20261018_124639**（exec-lw-fix 执行，经 Lead 授权；命令/脚本/fixture 与 90 修订腿一致——脚本=90 分支 4f97bc7e 只读复用未改、被测 agent=133 修复分支构建 SHA256 5A25452A…、kernel B6565DCF…）：
  - D1：status=pass + validation=pass（reverb/track 1032、rev 3→4、readback 0.382767379283905、forward_mutation=1、human_audition_ready=true、park fs7 waiting_continue）。
  - **LW 四门全 PASS**：①disclosure ②verdict store schema=`loudness_watch_verdicts.v0`+round-trigger(project_change, records=2) ③dedupe（checks 7→8、records 2=2、keys unique）④**config-off silent（checks_total 8→8 不推进；gate4 由 FAIL 转 PASS）**；A4 零标记 PASS。
  - **E1**=`[settle/project_change] n=1 duration median=18.041ms max=18.041ms bakes_total=0 fresh_rows_total=19 stale_rows_total=0 fresh_reuse_ratio=1.00`（+observation_piggyback n=1 57.164ms）；决策侧亲读 gates/E1 工件。
  - 拆栈：12:52:54 复核六端口（7878/5555/5556/8787/4445/19876）监听=0、四类进程（VitApp/VitAgent/vsphub/Godot 含变体）=0；run1 拆栈 12:46:34 同口径=0。
- **终验结论**：卡 90 三项终验前置全部满足（补证 run ID 复核 / E1 数据亲核 / 裁定 commit）；D1 settle 重锚 + LW 四门 + E1 全链闭合并 **未降口径**；**90 终验 pass**。
- 边界/后续候选（P2，登记不扩本卡）：①dedupe 复检会 relabel trigger provenance（trigger.go:262-274）；②T3 project_change 记录 project_revision 为空；③feature snapshot `loudness_summaries` 行累积重复（19 行=6 clip×3+1）致 measured 重复；④真栈补证为 n=1 成功样本（存在性证据）。
- 卡 90：**done / 验收 pass**。

## 三、合入与栈

- 合入（决策侧 cherry-pick）：133 修复四笔 **8b8bc70e/ed8dce98/3ce0d088/7dd7854e** + 90 修订腿一笔 **2806e547**（脚本+两 run 精简工件）。
- 栈：133 占用 12:36–12:52（run1 12:38–12:46、run2 12:47–12:52）；run2 exit 0；拆栈后六端口/四类进程=0；当前**空闲**。
- 卡面：90/133 均归 done；92 见 [2026-10-18-BATCH1-rulings.md](2026-10-18-BATCH1-rulings.md)。
