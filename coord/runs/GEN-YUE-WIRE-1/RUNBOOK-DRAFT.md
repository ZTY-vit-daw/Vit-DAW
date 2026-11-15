# GEN-YUE-WIRE-1 真栈+真实生成烟测腿 RUNBOOK（草案——候栈第 2 位，跑前须按 371 DG 口径终冻）

- 状态：**草案，未执行**。本腿候栈位第 2 位（第 1 位=367 烟测腿）；栈释放后按 coord/resources/PC-RUNTIME-STACK.md §2.2 全协议（净空亲核→登记推 main→≤60s 复核→启栈→TEARDOWN_ALL_ZERO 亲核释放）排队执行。
- 前置：G0-STACKGATE-1（371）DG1-DG3 口径冻结后，本腿验收断言按其终冻口径对齐；371 未合入前本腿不启动。

## 被测面

- worktree `D:/Vit_DAW_wt_yue1` @ `port/gen-yue-wire-1`（六项实现+单测/集成测）。
- agent=worktree 现场构建 `agent/bin/VitAgent.exe`；内核=只读复用主仓预构建 VitApp.exe（零内核改动，hash 记录）；前端按 371 口径（若 DG 定义要求渲染面，按其定义起 Godot/VitShell 链）。
- YuE2 引擎=E:\AI\YuE2 部署，按 278 先例自管启停（不入 PC-RUNTIME-STACK 三件套记录）；本卡新增 engine 托管面负责按需启动/就绪/停止（provider engine 块配置）。

## 腿定义（草案，跑前按 371 终冻）

1. **leg0 注册披露**：gen.providers 列 yue-local（含 engine 块在场）。
2. **leg1 确定性转换链（零 LLM）**：真内核读学生谱（get_midi_clip_notes + music_context）→ ConvertMidiToYueABC → 引擎实收 ABC 与往返断言一致（解析回读音高/拍位）。
3. **leg2 后台 job 全链（真实生成）**：gen.jobs.start（prompt 改写+instrumental+clip_id+track_id）→ 状态面 preparing→running→done → 音频+replay 工件 → 版本档案 JSON（原 MIDI notes/源谱/dispatched/replay 路径）→ ingest 落轨回执。第二版本=第二个 job（改提示词→style 实发差异断言）。
4. **leg3 引擎托管腿**：引擎未跑→gen.jobs.start 触发按需启动→就绪→生成→gen 引擎自停（只停自己启动的进程）亲证；外部引擎在场→复用不杀亲证。
5. **leg4 请求面卫生腿**：provider extra_params 带 abc/style/lyrics 覆盖键 → 引擎实收=学生谱/学生 style（卫生生效）+limitations 披露在场。

## §8 跑前口径（草案；跑前若 371 口径冲突，以 371 为准并在此记录差异）

- 运行次数：每腿单 run 无重跑；环境类中断允许一次重建重登；同确定性断点连败两次→停止取证上交。
- 成功条件：各腿断言全过 exit 0；leg2 两版本工件可播（人耳听核由用户节点另行登记，非本腿机器断言）。
- 失败分类：`engine_startup_failure`（托管启动失败——对照卡面停止条件）/ `assertion_failure`（功能断言）/ `env_interrupt`（栈/端口/GPU 环境）/ `harness_invalid`（装置脚本缺陷）分记；VRAM 采样峰值对照 6.3GB 线（278 先例），超限=环境中断。
- 止损线：leg2 真实生成预算 2 次（每版本 1 次）+leg1 引擎渲染 1 次=3 次生成；LLM 零计费面（本腿全部确定性命令驱动，不进 chat；如 371 口径要求 chat 驱动，另冻 chat 预算≤10 调用 deepseek-flash ≈¥0.05）。
- 写面：S10 recents 快照还原纪律（AGENTS §10）；run 独立 fixtures 副本；工件=coord/runs/GEN-YUE-WIRE-1/<run stamp>/，随 port/gen-yue-wire-1 或 stack 分支入库。

## 已完成段（零栈，2026-11-15/16 深夜）

单测+定向集成测（详见卡面回执）：genapi 包全绿（含 yue_abc 往返 5 例+yue_prepare 4 例+engine 4 例）、harness 包全绿（含 gen_jobs 生命周期 3 例）、tools/genctx 全绿、go build ./... exit 0、webui vitest 584/584（主仓同源 src，本卡零 webui 改动）。
