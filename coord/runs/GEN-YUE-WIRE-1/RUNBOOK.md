# GEN-YUE-WIRE-1 真栈+真实生成烟测腿 RUNBOOK（终冻版，2026-11-16 凌晨）

- 状态：**跑前终冻**。本腿=STACKGATE DG1-C（[docs/STACKGATE_DG1_DG3_V1.md](../../docs/STACKGATE_DG1_DG3_V1.md) §3.3 断言面 C1-C8）；本文件冻结腿定义与 §8 口径，执行时按 PC-RUNTIME-STACK.md 六步。
- 腿形态（§1.1）：kernel+agent berth 形态（`run_yue_wire_smoke.ps1` 编排，无 UI）+ YuE2 引擎独立进程（**由 agent 的引擎托管面按需拉起**，不预启——C7 实证前提；不入三件套栈记录，278 先例）。

## 腿定义与断言映射（DG1-C C1-C8）

装置=`scripts/run_yue_wire_smoke.ps1` + `scripts/yue_wire_smoke.py`（W0-W9）：

| 腿 | 断言面 | 内容 |
|---|---|---|
| W0 | C7 前置 | gen.providers 列 yue-local（含 engine 块在场亲证）；8087 拒连亲证（引擎未跑=bring-up 是真实的） |
| W1 | C1 输入 | kernel ZMQ 真值 fixture：new_project→save_as(run)→add_track→insert_midi_clip→add_midi_notes（16 音符旋律，4/4）→get_midi_clip_notes 回读对账 |
| W2 | C6/C7/C8 | gen.jobs.start 立即返回（RTT<15s）→轮询 status 至 done→引擎 mid-run 被托管拉起（/health 亲证+supervisor 日志文件在场）→WAV RIFF+时长窗（40-150s）+replay JSON+job 档案 JSON |
| W3 | C1/C3 谱面 | replay.abc（引擎实收回显）py 独立解析 Ins 声部→与 W1 音符逐音对账（pitch 精确/时值容差 0.01 拍）；Vocal 声部零发声音符；档案 source_score 原始 MIDI notes=16 全量 |
| W4 | C2/C4 | v2 job（改提示词）→replay1/2.style 各含本版本用户提示词原文且互异（prompt→style 实发映射）；污染 extra_params（abc="POLLUTED-ABC"/style="polluted-style"）未达引擎——学生谱/用户 style 实收（请求面卫生）；v2 谱仍=学生旋律 |
| W5 | C5 | v1/v2 job_id/task_id/WAV 路径互异+均 done+gen.jobs.list 两版本在列（两版本=两次 job） |
| W6 | C6 | v2 运行窗口内 gen.providers 调用 RTT<15s（非阻塞窗口可操作） |
| W8 | C8 | 工件有效性（W2/W5 内联+单独记录） |
| W9 | C3 声学（概率） | v1 WAV 经 SheetSage2 /transcribe 回读→Vocal 声部无发声行（**概率类**，GEN-YUE-DIALECT-1 裁定 B：谱面层确定性成立/声学层不承诺；预写失败类 legW9_vocal_detected） |

## §8 跑前口径（终冻）

- **运行次数**：1 次全链单 run 无重跑；环境类中断（端口占用/内核或 agent 启动失败/引擎托管就绪失败/VRAM 超限）允许 1 次重建重试；同确定性断点连败两次→停、留工件、上交。
- **生成预算**：恰 2 次真实生成（v1/v2 各 1，60s 歌请求）+1 次转写（W9）；引擎冷启动 1 次（agent 托管拉起，就绪预算 300s）。
- **LLM 预算**：**零**——全部腿走 /agent/invoke 确定性工具面，无 chat goal；agent 日志 LLM 相关行采样入工件（stayed-zero 审计）。
- **成功条件**：驱动 py exit 0（全部断言绿）；ps1 exit 0=finished。
- **失败分类**：legW0_registration / legW1_fixture / legW2_start / legW2_status / legW7_engine_bringup / legW3_roundtrip / legW3_vocal_not_rested / legW4_style_not_mapped / legW4_hygiene / legW5_not_two_jobs / legW6_blocking / legW8_artifacts / legW9_transcribe / **legW9_vocal_detected（概率类，预写）** / timeout / env_failure / vram_over_budget（>6451MiB）。
- **止损线**：同形失败两次跨 run→停；VRAM 超 6451MiB=环境中断升级不重跑。
- **退出码**：0=PASS；1=断言失败（face_defect，禁原样重跑）；2=环境中断。
- **写面隔离**：recents 不触碰（berth 形态无 open/save 用户工程——new_project+save_as 到 run 目录；agent 草稿根 VIT_HISTORY_DRAFT_ROOT→run/agent_drafts；genapi config/jobs dir/run 下载全部 run 目录内）；引擎日志 supervisor stdout/stderr→run 目录（engine 部署目录不写）；E:\AI\YuE2 零写入（引擎只读模型）。
- **用户听核**：M1（成曲人耳）待本腿 PASS 后由用户执行登记，非本腿机器断言。

## 跑前自检证据

- 装置 config 干跑：`predryrun_cfg_check.txt`（JSON 逐字段亲证：engine 路径/污染键/超时）。
- py 解析器交叉验证：与 Go 转换器方言样例对账过（tie/和弦/多小节休止/降号拼写）。
- 三门：go build exit 0 / genapi+harness+tools+genctx 定向 test 全绿 / webui vitest 584（主仓同源）。

## 执行记录

- **run_20261116_000009（2026-11-16 00:00–00:01，正式 run，单 run 无重跑）**：driver exit 1（=预写概率类 `legW9_vocal_detected` 首触发）；占用 110c3aa5/释放 fea0829a（PC-RUNTIME-STACK 在卷）。
  - **W0-W8 断言面全绿（DG1-C C1/C2/C4/C5/C6/C7/C8 全部确定性断言 + C3 谱面面）**：C1 转换往返 16/16 音符逐音对账（py 独立解析 replay.abc vs kernel fixture 真值）；C2 v1/v2 replay.style 各含本版用户提示词原文且互异；C3 谱面 Vocal 声部零发声（dispatched 谱亲证）；C4 污染 extra_params（abc="POLLUTED-ABC"/style="polluted-style"）未达引擎、学生谱/用户 style 实收；C5 两版本=两独立 job（job/task/WAV 互异+list 在列）；C6 start 即返+运行窗口 providers RTT=0.05s；C7 引擎由 agent 托管真实拉起（8087 占用前净空亲核零→mid-run up→supervisor 双日志在场）；C8 WAV 60.0s（40-150s 窗内）+replay×2+job 档案 JSON（原始 16 音符全量留档）。
  - **W9 概率类首触发**：转写 Vocal 声部 2 行中末小节 `z6ed` 检出 2 音——GEN-YUE-DIALECT-1 裁定 B 预言的「渲染先验填稀疏伴奏」声学层概率事件（前 3 小节全休止）；非产品缺陷非装置缺陷。处置候主管裁定（沿 278 裁定 B 精神：裁定 B 收口=谱面层确定性+W3 已证+声学层不承诺；或另批统计口径预算）。
  - 预算兑现：生成恰 2 次+转写 1 次+引擎冷启动 1 次；零 LLM（全程确定性 invoke，零 chat goal）；VRAM 真实峰值 4763MiB≪6451（ps1 峰值正则逗号 bug=278 同款 P3，手工复核 15 样本）。
  - 装置弱点 2 项如实：①wire_report.json 的 failure_class 字段未随 fail() 回写落盘（证据在 terminal FAIL 行+report.w9 键，修复候后续装置小卡）；②ps1 经管道运行时 bash 侧退出码被 tail 吞（以 YUE_WIRE_STACK_VERDICT 行为准）。
  - TEARDOWN_ALL_ZERO 亲核 00:1x（六类进程=0+六 TCP=0+UDP 4445=0+WebView2=0）。
  - 工件：本目录全档（wav 由 .gitignore 排除不入仓；replay/job 档案/trail/日志/转写/fixture 工程入库）。
