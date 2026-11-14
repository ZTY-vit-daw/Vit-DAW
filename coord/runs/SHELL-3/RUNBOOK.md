# SHELL-3 真栈全旅程烟测 RUNBOOK（跑前冻结，AGENTS §8 口径）

- 冻结时点：2026-11-15 04:1x +0800（占用登记 5de1ea2c 之后、两轮调试取证之后、正式跑之前）
- 卡：SHELL-3（全链真栈烟测+手测 v6 放行）；验收=脚本 exit 0
- 装置：`scripts/vitshell_full_smoke.ps1`（主腿+失败注入腿+legacy 腿同脚本）+ `scripts/vitshell_full_smoke_cdp.mjs`（CDP 驱动）；SHELL-2 装置同模式扩展，未另起一套

## 调试轮取证（不计入正式口径）

- run_full_20261115_035009：装置缺陷①——status 轮询 URL 缺 `/status` 后缀（404 全程 unreachable，转写自 SHELL-2 装置时笔误）；同步发现 F1 腿重复 `--agent-exe`（解析取首者胜，坏二进制注入失效）→ 装置修（URL 修正+AgentExePath 覆盖参数）。
- run_full_20261115_035933：装置缺陷②——F2 脚本 TcpListener 释放后 VitShell 健康探针遗留连接使 7878 短暂 linger → R1 agent WSAEADDRINUSE（agent_last.log 锚点）→ 装置修（端口可绑性探针进 preflight/recheck/teardown；F2 注入改 bound-only socket）。
- run_full_20261115_040758：装置缺陷③——**F3 的 WEBVIEW2_BROWSER_EXECUTABLE_FOLDER 注入 env 在 Start-VitShell 返回后未从脚本进程清除**→ R1 继承死目录覆盖→WebView2 runtime probe 失败→系统错误框（进程存活+零 trace+零 status=unreachable 120s）；R1 的 agent_last.log 无 boot 记录（时间窗对齐）为关键锚点 → 装置修（ExtraEnv 用后即清）。
- run_full_20261115_041804：装置缺陷④——**Test-PortBindable 用 TcpListener：ctor 即 bind，Start() 抛错时 Stop() 不释放未启动 listener**（micro_bind.ps1 实证）→ 探针自身泄漏 Bound socket 使后续探针全失败 → 装置修（探针改 raw Socket bind+Close，Close 即时释放实证）。
- run_full_20261115_042646：装置缺陷④修复后验证——F1/F3/R1 全过，F2 可绑性检查仍失败（缺陷④残留效应），触发探针 raw-socket 终版修复。
- diag_manual/repro_f1/repro_port/repro_seq/repro_f2hold/repro_r1loop/micro_bind：诊断工件（VitShell status 契约面实证：bad-agent→phase=error t=2s；重复 flag 首者胜；VIT_WEBVIEW_DEBUG=1 stderr step trace 面；TcpListener Stop pending-connection 延迟释放；额外 env 生命周期）。
- run_full_20261115_040548：调试主腿 1 轮 PASS exit 0（无失败腿场景，含 chat 真帧：agent 确定性无 LLM 路径回包「意图分类服务本次不可用，以上直接来自工程结构事实」=真 agent 帧非客户端兜底）。
- 四例装置缺陷均为执行侧装置设计/笔误，非 SHELL-1/2 产品缺陷；停止条件未触发。

## 运行次数与成功条件

- **主腿计划 3 轮**（`-Rounds 3` 默认值；每轮=完整旅程独立栈：冷启动→…→全退→S10 还原），**成功=3/3 轮 PASS** 且脚本 exit 0；成功比例入回执（本卡核心交付=SHELL-2 单 run 之上的重复成功数据）。
- **失败注入腿 3 条+legacy 腿 1 条**各 1 次（与主腿同脚本）；失败腿判据：pass（预期错误态）或 boundary（注入装置不生效，如实声明），两者都计腿完成；fail 则整体不通过。
- **调试轮**：正式跑前允许装置迭代轮（-Rounds 1），其 run 目录保留取证、不计入稳定性口径；装置迭代失败分类见下。
- 零 LLM 计费面：主腿 VitShell 进程树注入 `VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1`（agent `applyExplicitEnvOverrides` 显式 env 覆盖 `~/.vit/config.json`，真 key 不被动用）。**chat 腿断言口径（冻结）**：真实 user 帧 + 真实 /agent/chat HTTP 往返产生的 assistant 位置帧；LLM 内容生成显式出界——实证回包形态 reply/error 并存（goal_status=failed）→ surface 渲染诚实送达失败帧（「这轮没送达（…）」）。模型内容质量不在 SHELL 链验收范围（归内容侧卡）。

## 腿与断言（冻结面）

- **L0 preflight**：Vit 系进程=0；7878/7879 监听=0；占用记录重读=本卡 owner；recents 快照+双根 Workspace 运行态快照+fixture 源哈希留档；fixture 副本落 run 独立目录。
- **L1 build**：VitShell cmake exit 0；agent go build exit 0；webui npm run build exit 0；kernel/hub 主检出借用（C++ 零 diff）；二进制哈希入 meta。
- **legacy 腿（声明性）**：godot `--headless --import --check-only --path <worktree>` exit 0；start_page.gd 结构核证（VIT_SHELL_DIRECT env 门+open_project_by_path 公共链引用+launch-result 回执标记在场）→ F5 编辑器路径未被 SHELL 系破坏的声明性证据。
- **失败注入腿**：
  - F1 坏二进制：`--agent-exe` 指向不存在路径 → status phase=error ≤30s；CDP 断言窗内错误页（title=Vit - error、detail 非空、重试/退出按钮）；**VitShell 进程存活**（窗内错误态非崩溃）；**零 Godot**；/close 可收口。
  - F2 端口占位：脚本内 TcpListener 预占 7878 → agent 无法绑定 → `--agent-ready-timeout-ms 15000` 短超时 → phase=error ≤60s；同 F1 断言面；拆栈后 listener 释放。
  - F3 WebView2 禁用：`WEBVIEW2_BROWSER_EXECUTABLE_FOLDER` 指向空目录 → ProbeWebView2Runtime 失败 → 系统错误框路径（进程持 MessageBox、无 status 端点、无 agent、零 Godot）；**注入不生效时记 boundary**（声明留手测，不计产品失败）——装置可行性 M3 实证后如实记录。
- **主腿（每轮）**：
  - **R-L0 冷启动（零 Godot 进程级断言）**：VitShell spawn 后立即、+3s（splash 窗口）、phase=ready 后三次断言 `Godot*=0`；status `godot_running=false`。
  - **R-L1 主窗真数据**：CDP `[data-launcher-root]` `data-launcher-state=ready`+`data-launcher-bridge=available`+无 `data-fixture`；`[data-launcher-greeting]` 非空（live GET /agent/launcher/greeting 真端点数据）；`[data-launcher-thread]` 首条 assistant 帧（greeting 播种）；recents 计数属性留档。
  - **R-L2 首开直启**：CDP 真实点击 `[data-launcher-choose-file]` → choose_file 旁路（fixture A）→ launch_count=1 → `godot_running=true` → project surface：`[data-launcher-root]` detached+`[data-project-root]` `data-project-state=ready`+context name=fixture_a.vit；no-reload 哨兵全程完好；首开治理（governance.none 预期，出现则记录非零摩擦并驱动切换）。
  - **R-L3 工程内对话往返（SHELL-3 新腿）**：CDP 键入 `[data-project-composer] input` → 真实点击发送 → `[data-project-thread]` user 帧出现（author=user）→ assistant 位置帧出现（author=vit，内容按上面冻结口径）→ composer 状态回落非 replying；期间 `Godot*≥1`（DAW 在跑）。
  - **R-L4 关 DAW 回主窗**：POST /vitshell/close-daw → `godot_running=false` → launcher surface 回归（哨兵完好）。
  - **R-L5 治理切换**：二次打开（fixture B）→ 三键卡（保存并切换/直接切换/取消 逐字断言）→ 真实点击直接切换 → launch_count=2 → /agent/state current_project=fixture_b。
  - **R-L6 全退+S10**：POST /close → Vit 系进程=0（含 Godot/kernel/hub）→ 端口释放 → fixture 源哈希一致 → recents+运行态还原（每轮独立还原，轮间无状态泄漏）。

## 失败分类与止损

- 环境中断（进程被外部杀/端口被占/网络抖动）→ 记录证据后可重跑该腿（≤2 次），不计功能失败。
- 装置缺陷（脚本报错、选择器竞速、编码问题——PS5.1 BOM/UTF-8 body 陷阱已入律）→ 修装置后在调试轮重验，不计入正式口径。
- 断言失败（产品/链路面）→ 停：取证（meta*.json+bridge log+CDP 输出+status 快照+进程/端口快照）入 run 目录，**不原样重跑**；属 SHELL-1/2 缺陷→取证上交转返工卡（卡面停止条件），不在本卡修。
- 主腿任一轮断言失败 → 后续轮停止（不续跑凑比例），成功比例如实记录。
- 同一确定性断点连续两次失败 → 止损，留断点交回。

## 工件

- run 根：`coord/runs/SHELL-3/run_full_<stamp>/`（正式）/ `run_dbg_*`（调试，如有）：meta.json（总）、meta_round_N.json（逐轮）、各腿 status 快照+CDP 断言 JSON+bridge log、godot 用户日志拷贝、recents/运行态快照、fixture 哈希对账。
