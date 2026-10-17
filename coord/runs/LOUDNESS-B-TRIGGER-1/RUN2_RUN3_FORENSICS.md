# LOUDNESS-B-TRIGGER 真栈补证腿 run2/run3 取证与上交（2026-10-17 晚窗）

- 会话：GLM-5.3 L2 PC 执行会话（ZCode，本晚补证腿）；worktree D:/Vit_DAW_wt_loudness_watch_b @ 8ada1337（=impl 64b3ccfd + E1 提取脚本），零生产代码改动。
- 占用登记 31bf798d（19:26，登记前亲核四端口/四类进程=0）。
- 结论先行：**exit 0 未达成，E1 无法提取；根因=烟测腿设计前提（NL 轮 settle 触发 T1）与本 fixture 模型分支现实不符，非实现缺陷**。按 §8/AGENTS §11 上交决策侧，不烧剩余轮次（依据见 §4）。

## 1. run2 = fs_largeproject_smoke_20261017_192734，exit 2（环境中断，不占止损）

- Phase 0 清场过、agent 构建过、kernel 起 pid 18108；死点=dev_agent_smoke UI 门「UI did not expose kernel command port within 20s」（`UiPortWaitSeconds` 默认 20s，fs_largeproject_smoke 调用面未透传）。
- 根因实证：内核日志 `Workspace/Logs/VitHeadlessServer2026-10-17_19-27-56.log` 显示 Waves VST3 插件表全量扫描（class_selection/component_create 数百条）——冷启动绑定 5555 晚于 20s 门（TIM-SMOKE 教训同族面：脚本注释自认 cold kernel start 可超 20s，但该参数只调了 `WaitSeconds=90`=agent HTTP 面，UI 门不可达）。当天 19:27 为本日首次拉起 staging 内核（上一份内核日志=10-16 20:56）→ 冷盘。
- 残留处置：脚本清理只停了两个 godot、kernel pid 18108 残留占 5555/5556——本 owner 手动 Stop-Process 拆净后复核四端口/四类进程=0。
- 精简工件：`run2_20261017_192734_slim/`（run_report/dev_agent_smoke_error/pull_env/head/git_status）。

## 2. run3 = fs_largeproject_smoke_20261017_193105，exit 1（classification=script_exception）

栈与前置全过：bringup 13s（缓存热）、61 轨导入 1s、bake 61/61（30s）、authority、**4.6 两门过**（pre_status_disabled=true 零成本基线披露；R128 configure ok digest=0079aa999954c1ee…）。

失败链（每环有工件锚点）：

1. NL 轮（真实 LLM，deepseek-flash config 未动）1 turn 收口 stop=done goal=completed——模型以**纯文本混音分析作答**（识别「sattelites moog one 2」中频堆积、提议 -2dB@1kHz Q≈1 与 A/B 验证），未驱动自由态站点机制：`nl_events_census.json` 零 trajectory.* 事件、无 settle 结论、无动作 applied（confirm 轮 exercised=False not_driven_no_card）。
2. → T1（需 `RecordFreeStateConclusion` 成功持久化）未触发；T3（需 `loop.LatestProjectChange` 非空——该字段只在自由态实验动作 applied 后的 post-action 刷新填充，`free_state_reasoning_loop.go:2447`）未触发。
3. → harness `loudnessWatchHook` 保持 nil（设计如此：注册点=三钩子自身，`free_state_conclusion_bridge.go:222` EnsureLoudnessWatchHook 惰性；grep 证无启动期注册点；chat HTTP configure 拦截器只写 `s.loudnessWatchConfig` 不注册钩子——故 4.6 configure 能 ok 而钩子仍 nil）。
4. → Phase 8.8 三次 mix_request_observation 响应 result 无 `loudness_watch` 键（`observe1_result_key_census.json`：loudness_watch_in_result=false）——**nil hook=零新增键零行为，实现面严格符合设计**。
5. → `$lwPost.verdicts_path` 恒空 → 证据复制步 `Test-Path -LiteralPath ""` 抛未捕获异常 → 「uncaught script exception」exit 1（脚本健壮性缺陷：本应判为 LW 门 FAIL 而非 script_exception；另 Read-LwVerdictStore 在空路径上空转烧满三个 30s deadline）。

LLM 记账：7 次调用全成功 0 超时（ordinary_agent_semantic_entry 1 + pullharness 5 + message_loop_repair 1；声明预算=NL 轮 1 chat+≤6 nudge+≤4 hop 内）。拆栈：脚本 finally 自拆（agent 22552/kernel 23628/godot 21572+9104 已停），19:4x 复核四端口（7878/5555/5556/8787）监听=0、四类进程（VitApp/VitAgent/vsphub/Godot 含变体）=0。

## 3. run3 在真栈上**已证**与**未触达**的面

已证（正向证据）：
- impl 二进制真栈拉起（285 工具）+61 轨全链（导入/bake/authority）；
- LW configure/status HTTP 披露面（4.6 两门，R128 preset + config_digest）；
- nil-hook 诚实退化（observe 响应零新增键）。

未触达（结构性）：T2 搭车披露/verdict 落盘/dedupe/配置关静默四门与 E1 计量——全部以钩子注册为前提，而注册只经 T1/T3。**单测面已覆盖这些语义**（trigger 13 测+chat 8 测+harness 6 测，95 包 0 FAIL）；真栈缺的是「T1/T3 实际发生过」这个前提本身。

## 4. 为什么不烧剩余轮次（§8 依据）

同 fixture（sattelites 61 轨）同话术同模型的历史样本——FS-LARGEPROJECT-SMOKE-1 基线 4 run（A2/A3 fail）、M1-2×2/M1-3/M1-4/M1-5×2（capability_blocked 同族/「round 形态 goal_terminal 无 settle」）——**10+ 样本零 settle**；run3 再证。§8 允许剩余轮次的条件是「明确属于环境中断或前段模型随机分支」，本断点的模型分支在历史样本中稳定复现，不构成可期望改变底牌的随机性；原样重跑=对已知确定性结局烧预算。按 AGENTS §11「证据推翻任务卡前提→停止并上交」收口。

## 5. 上交决策侧的选项（文件域/验收口径裁定）

1. **腿重锚**：把 LW 真栈证明挪到能确定性驱动 settle 的腿（D1 reverb 族 run_free_state_d1_smoke 先例——但该族 settle 本身也历经多轮修复才稳定，成本须知）；或
2. **口径放宽**：T1/T3 语义已有单测覆盖，真栈腿接受「configure/status 面+nil-hook 诚实退化」为可达验收面，四门与 E1 改由单测/后置腿背书；或
3. **确定性注入**：设计一个 smoke 专用 settle 注入面（新代码，需新卡）。
- 附带：脚本健壮性修复（空 verdicts_path 守卫，本卡文件域内）建议随上述裁定以修订卡处理。
- 本腿工件：run3 全量重件留盘 worktree `coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261017_{192734,193105}/`；精简件+本件随 port/loudness-watch-trigger-1 入库。
