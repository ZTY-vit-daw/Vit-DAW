# YUE-VOCAL-FORENSICS-1：YuE2 纯器乐断言矛盾取证（C3 人声 vs 实听歌词）+v2 完整性

- 发卡：主管 2026-11-16 晚（承接 372 用户听核 M1 回执两项 OPEN；非返工——372 本体 pass 维持）
- 池序 381；P1；依赖=372 run 工件在主仓✓（双版 WAV 已备份至 `coord/runs/GEN-YUE-WIRE-1/run_20261116_000009/downloads/`）；零栈优先（离线取证），复现腿按需占栈
- 上游事实：用户实听 v1/v2 ——「确定是合理的音乐」（可听性 PASS）但**有歌词/人声**（非纯器乐）且 **v2 只有一句**（内容密度存疑）。C3 断言（instrumental 真实下发+Vocal 声部零发声）与 C8 断言（WAV 60.0s）均绿——断言与产出矛盾。

## 目标（取证三问，先取证后定修）

1. **C3 断言语义复核**：C3 测的是 replay.abc 请求侧还是引擎实收/生效侧？instrumental 参数在 YuE2 本地引擎的真实生效链（引擎是否支持纯器乐、参数名/方言映射是否正确、是否存在「下发但引擎忽略」面）。
2. **实听对照取证**：离线分析双版 WAV（频谱/人声段定位——可用现有分析工具或最小脚本），定位歌词段的时间区间，与 replay/style/abc 记录交叉（style 里是否有歌词来源；v2 的 prompt 与 v1 差异点）。
3. **v2 完整性**：v2 WAV 60s 内实际内容分布（哪段有音乐哪段空/循环）——「只有一句」是引擎输出短内容+填充，还是 render/pipeline 截断。
4. 产出定性结论：引擎能力边界 vs 请求链 bug vs 断言设计缺陷，各自锚点；修复/规避建议（instrumental 正确下发方式或产品面声明纯器乐不可达）。

## 边界

- 取证不修引擎（引擎为外部 YuE2 本地引擎，若属引擎能力边界→登记+产品面裁定，不硬改）；agent 侧若发现请求链 bug→最小修复+回归+真栈腿。
- 不得复用 372 W9「稀疏配器残留」口径消化成段歌词（量级不同，卡面已明示）。

## 文件域

`agent/internal/genapi*`/YuE 方言与 provider 链（领取时锚定）+分析脚本+`coord/runs/YUE-VOCAL-FORENSICS-1/` 工件。

## 验收

三问各有证据锚点（日志/频谱工件/代码行）；结论定性清晰（能力边界/请求链 bug/断言缺陷三分）；若有修复，修复-回归-真栈链完整；主管亲核工件。

## 领取/回执/验收

- **领取**：量产执行子代理（GLM-5.3-Flash）2026-11-16；worktree `D:\Vit_DAW_wt_yueforensic`，分支 `port/yue-vocal-forensics-1`；领取基线 HEAD `d5c20765`；零栈执行（全程未启动内核/agent/引擎进程，未占 PC-RUNTIME-STACK）。
- **执行完成（待决策验收）**：三问取证完成，报告 `coord/runs/YUE-VOCAL-FORENSICS-1/REPORT.md`（commit `631aab2c`，已推远端）。
  - **问①**：C3 断言=请求侧谱面（`yue_wire_smoke.py` W3 检查 replay.abc 的 Vocal 行），非引擎生效侧；instrumental 生效链=style 英文词+谱面休止（`yue_prepare.go`），引擎 API 无 instrumental 参数（stderr job 回显仅六字段）、双声部 ABC 被解析为 1 轨——**请求链无 bug（not_forwarded 如实声明，无静默面）；人声=引擎无歌词自由生成，属引擎能力边界（主因）**。
  - **问②**：SheetSage2 转写回读（372 run w9 工件）显示 v1 verse 段 Vocal 声部承载成段主旋律（Ins 反而全休止），粗锚约 27-55s——量级远超 W9 offender `"C"z6ed|`（间隙段 2 音），372「稀疏配器残留」口径不能消化（卡面红线执行确认）；numpy 粗判工件 `wav_forensics.json`；v1/v2 请求侧谱面相同，人声非请求注入，两版人声差异与 lm_seed/style 采样差异一致。
  - **问③**：v2=**引擎短输出**（semantic end token at step 1274 → 51.0s），WAV 字节数 9,784,108=44+2,446,016×4 逐样本精确对账，**无填充、无管线截断**；「C8 双版本 WAV 60.0s」口径不成立——装置对 v1 仅 40-150s 宽窗、**对 v2 无时长断言**，50.96s 短输出在 372 run 中从未被检出。
- **agent 侧修复（文件域内，措辞级）**：`yue_prepare.go`/`yue_abc.go` 移除「validated instrumental path」超卖措辞，disclosure 改为「request-side conditioning only；engine-side muting not guaranteed」；`yue_prepare_test.go` 加防回退断言。回归：`go build ./...` 0、`go test ./internal/genapi/ -count=1` 0。行为零变更（请求 body 字段未动），零栈裁定未触发复现腿。
- **登记待决策侧**：①装置 `yue_wire_smoke.py` C8 对 v2 无时长断言（建议加请求时长比例窗），脚本在 372 文件域、本卡未动；②372 回执「C3 instrumental 真实下发亲证」「C8 双版本 WAV 60.0s」两处口径需勘误注记；③产品面建议声明「yue_local 纯器乐不可保证」。
- **四件套**：命令/退出码全录 REPORT.md §执行命令（worktree add 0、python -I 分析 0（首跑暴露阈值 bug 已修后复跑 0，非重跑凑过——修复为峰值相对阈值设计错误）、sha256sum 0、go build 0、go test 0（修复前后各一次均绿）、commit/push 0）；产物路径=worktree `coord/runs/YUE-VOCAL-FORENSICS-1/`（REPORT.md、analyze_wav.py、wav_forensics.json、wav_forensics_summary.txt）+ 分支 `port/yue-vocal-forensics-1` commit `631aab2c`；问题=①分析脚本首跑阈值设计缺陷（median×3 超 peak，gate 全零）——属自检发现自修，证据留存于 REPORT.md 方法边界节；②文件域外登记项见上，未越域修补。
- **验收**：待主管亲核工件（REPORT+json 工件+diff）后裁定。

### 验收（主管 2026-11-16 晚）

- 裁定：**pass**——三问证据链亲核（C3=yue_wire_smoke.py W3 请求侧锚点+引擎回显六字段日志；人声=SheetSage2 回读成段主旋律 27-55s，W9 口径不可消化红线确认；v2=semantic end token+字节级对账无截断）；三分定性成立（**主因=引擎能力边界**，请求链无 bug，断言缺陷=C8 对 v2 无时长断言）；diff 7 文件零越域（措辞级修复+防回退断言，行为零变更）；四件套齐全；首跑阈值 bug 自修诚实留证。
- 合入：cherry-pick 631aab2c→main（1001a216）；372 卡面勘误注记已补（两处口径作废面在卷）。
- 登记项处置：①C8 装置补腿（yue_wire_smoke.py 加 v2 时长比例窗断言）=**主管授权同执行流扩域补腿**（同一分支续 commit）；②372 勘误=已完成；③产品面声明「yue_local 纯器乐不可保证」候用户裁定（随 YuE 展演口径）。
- 372 M1 两项 OPEN 由此卡收口：人声=引擎能力边界、v2 短输出=引擎语义终止；M1 终态=可听性 PASS+能力边界披露（纯器乐不可达），展演口径候用户。

### 补腿回执（381-C8，2026-11-16 晚，授权扩域执行完成）

- **授权依据**：主管验收裁定 pass 后授权原登记项①扩域补腿（同执行流、同分支续 commit），worktree `D:\Vit_DAW_wt_yueforensic` 分支 `port/yue-vocal-forensics-1`。
- **补腿 commit**：`731a7d9f`（381-C8 补腿: assert both renders against request-duration ratio window），已推远端（`631aab2c..731a7d9f`）。改动 3 文件 +128/-1：
  - `scripts/yue_wire_smoke.py`：新增模块级 `wav_duration_in_request_window()`（比例窗 **[0.6, 1.5]** × 请求时长 60s）+ W5 腿对 **v1/v2 双版**记录 `duration_v1/v2` 入 wire_report 并断言，违窗报新失败类 `legW8_short_output`。窗阈值论证：两实测样本 v1=59.999s/60s（ratio 0.99998）、v2=50.959s/60s（ratio 0.849）均在窗内——本次 v2 行为保持「记录性观察」不误杀，窗用于检出**严重**短输出（如 0.5x）与超长（1.667x）；上限 1.5 依据引擎 frame budget clamp 行为留测量裕量。v1 原 40-150s 宽窗保留（372 已验收语义不动，纯增量、零弱化）。
  - `scripts/run_yue_wire_smoke.ps1`：fail-class 事先声明清单同步补 `legW8_short_output`（纯注释，AGENTS §8 概率性运行协议声明面）。
  - `coord/runs/YUE-VOCAL-FORENSICS-1/dryrun_c8_window.py`：零栈干跑验证工件（commit 在卷，可复跑）。
- **验证方式（零栈，未启引擎/agent/内核）**：
  - 干跑 `python -I dryrun_c8_window.py <v1.wav> <v2.wav>` 退出码 **0**，6/6 行全 OK：双真件（v1 True、v2 True——现有工件不误杀）+ 合成反例 30s（0.5x，检出）、100s（1.667x，检出）+ 非法输入 fail-closed（dur=-1、requested=0 均 False）；真件只读自主仓 run 工件，反例 WAV 写系统临时目录不落源码树。
  - `python -I -m py_compile scripts/yue_wire_smoke.py` 0；PowerShell PSParser 语法检查 `run_yue_wire_smoke.ps1` 通过（装置仅注释/断言改动，go 面未触、无 go 回归需求）。
- **边界声明**：断言逻辑提取为模块级函数（干跑可复用），W5 插入点在身份检查之后；`__main__` 保护在卷，装置行为面仅新增时长断言与 duration 记录。真栈复验（下次 372 同款 run 自动携带新断言）留待下次占栈 run 自然覆盖，本腿零栈裁定。
- worktree 按指令保留（主管核后统一清理）。
