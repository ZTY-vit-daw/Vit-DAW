# D3-REF-ALIGN-THIN-1 verifyrun2 evidence（重派单门组，2026-11-07 01:30–01:39）

## 判定：①PASS——PS1 exit 0，§8 门槛达成

- run `20261107_013013`（01:30:13 启动，01:37:20 脚本退出），**PS1_EXIT=0**（ps1_exit_attempt1.txt 亲证）；重派授权=卡面 doing/ 权威本主管裁定行（main 30c3cd02）；单 run 第二次也是最后一次授权 run，无重跑。
- 被测不变量亲核：agent=worktree 预构建 SHA256 F4B5A215…7603D（01:29 复核未变，含本卡全部改动）；kernel=wt_228 F9E3DA0A…0D67A 只读复用；引擎=本机默认 deepseek-flash 无注入。

## D3 门组全绿（face P+门 A-D）

- **face P（裸话术 semantic_entry 探针）**：「帮我看看这首歌跟 spotify 的标准差多少」stop_reason=limit_reached（非 unresolved/invalid）——**裸话术过 semantic_entry，不建词表基线（263 教训）真栈成立**（置顶申报②证据兑现；上轮四开关 run 未及执行，本轮补齐）。
- **门 A（check turn）**：stop_reason=d3_ref_align_checked；payload verdict=**aligned**（fixture 依赖如实：19 载体均在带内，非凑数断言）、profile=**builtin:spotify（intent_named）**（fixture 无 render 绑定→意图点名面如设计走通）、measurement_grade=approximate、band≤-14 LUFS 披露、**零 mutation**。
- **门 B**：ref_align_verdicts.json 在盘携带 d3_ref_align trigger 记录（candidates=1）。
- **门 C（动作面诚实）**：aligned→action_face=none_needed，无卡、reason 在场、零 mutation——「无差距不发明动作」分支真栈验证（挂卡分支 gap_found+可定位此前已有 15 单测钉死+D1 settle 同款链真栈在档；本 run fixture 如实产 aligned，不硬凑 gap）。
- **门 D**：mix.report 三视图 D3 节点 not_started→**in_progress**。
- 附带：D1 python runner 主链 PASS（d1_smoke_report.json 在盘）。

## 挂账观察项（主管裁定：记录不追查）——**形态再现**

- D3 check turn 观察（d3_check_chat.json workflow_data.observation）：`loudness_peak_status=missing`、global peak/rms/crest 全 None、stereo_status=missing——**kernel digest D1/D2 全局面缺席形态在单开关 run 再现**（与 verifyrun1 四开关 run 同形；叠加交互归因候选①被本轮收窄排除后仍再现→该形态与开关组合无关，支持「kernel 观察面选择性缺失非确定性」候选②）。
- 同 run 内判据消费面（loudness_summaries 测量行）**健康在场**：19 载体被测量、verdict 如实产出——两数据源分离（digest 披露面 vs 测量行判据面）按设计工作，D3 判据不受 digest 缺席影响。
- 证据件：d3_check_chat.json（observation 节）+ verifyrun1/evidence.md 锚点对照。**仅记录，不定性，不扩查**（POOL-INDEX backlog 由主管处置）。

## 计费（telemetry_run_window.jsonl 原件）

- 19 调用 deepseek-flash（message_loop+repair+semantic_entry），usage in+cache ≈179,840 tok，out 字段读 0（该引擎 telemetry 形态，如实记录）——估 **≈¥0.1-0.3**（预算 ≈¥1-3 内；两腿合计 verifyrun1 ¥0.01-0.05 + verifyrun2 ≈¥0.1-0.3 仍远低于单腿预算上沿）。

## 收口

- owner 亲拆 Godot 23348/VitAgent 40596/VitApp 33056（01:39），**TEARDOWN_ALL_ZERO 亲核 01:39:13**（六口 TCP listen=0+UDP 4444/4445=0+四类进程+python 全零）；释放登记随卡面回执笔推 main。
- 工件本目录 12 件（console/ps1_exit/d3_gates/check_chat/probe_chat/verdict_store/queue_view/d1_smoke_report/runner_stdout/telemetry 快照/binary sha/evidence）。
