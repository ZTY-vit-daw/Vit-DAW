# VIS-FILM-P0-ROUTE-ENABLE run1（2026-10-11 22:15–22:35 +0800）

- 卡片：coord/cards/doing/2026-10-11-VIS-FILM-P0-ROUTE-ENABLE.md（领取 ce44e5a2）
- worktree：/Users/timozty/Documents/Vit-DAW-visfilm-p0 @ port/vis-film-p0（自 origin/main 563080dd 新建，领取时零源码 diff）
- 栈登记：dd9ac1df（占用）/ MAC-RUNTIME-STACK.md
- 被测二进制：vitagent sha256=ec361b204554bacea77936f02716cc71dbbd930d8bea4bfb4d4fc40d84505c08（本 worktree `go build ./...` 全绿后构建）

## 执行链（按序）

1. **config 启用（Mac 侧①复刻）**：主管 38532a7a 只落 PC 本机 `~/.vit/config.json`；Mac 本机原值 `image_understanding.enabled:false`，本腿同款一键改 true（差异校验=仅此一处，备份 `config.json.bak-before-enable`）。其余 10 路由 untouched。
2. **真实 agent 进程在位**：`bin/vitagent` 以 `VIT_AGENT_HTTP_ADDR=127.0.0.1:7878 VIT_AGENT_REQ_TIMEOUT_MS=3000 VIT_AGENT_LAST_LOG_PATH=<run>/agent_log/agent.log` 起进程（PID 34014），`GET /health` → `{"service":"VitAgent","status":"ok"}`。无内核/Godot（规格允许；日志中 5555/5556/8787 connection-refused WARN=预期降级，非失败）。
3. **真实截图**：webui `npm ci` + `npm run dev`（:5178，vite v6.4.3，`GET /app/` 200）→ `wksnap.swift`（离屏 WKWebView，窗口置于显示区外 (-10000,-10000)、activationPolicy=.prohibited、无键鼠无桌面截屏——AGENTS §5 合规）加载 `http://127.0.0.1:5178/app/`，settle 9s 后 takeSnapshot → `screenshot_webui_1280x720.png`（逻辑 1280×720 / 物理 2560×1440 Retina 2x / 212,741 bytes）。内容目检=真实渲染的 Ask Vit 三栏 UI（非空白页）。
4. **读图回路**：一次性探针二进制（`agent/cmd/visfilmprobe/main.go`，**跑完即删**，设计卡探针同款先例）以生产接线执行 `config.Load()`（真实 ~/.vit/config.json）→ `&llm.Client{}`（与 chat/server.go:484 同构造）→ `CompleteImageUnderstanding`，prompt 见 `probe_prompt.txt`，`VIT_AGENT_LLM_TELEMETRY_PATH` 指 run 目录。
   - 结果：**成功**。stdout `[probe] OK wall=17343ms rawKind=responses usage{in:1149 out:2720}`；`probe_result.json` `err:""`；telemetry 落 1 条无错记录（`llm_telemetry_probe.jsonl`）。
   - 退出码底账：shell 为 zsh，`${PIPESTATUS[0]}` 拼写未捕获到码（记录为空）；成功以代码路径推定=末行 `[probe] result written` 仅在 err==nil 分支后打印（该分支即 return 0 路径），并以 err:""+telemetry 无错+OK 行三重带内证据佐证。诚实声明：rc 未直接亲采。
5. **质量评注**：`quality_annotation.md`——ground truth（执行侧多模态亲读同图+webui 已知字符串交叉核对）对拍模型输出：布局✅/文字誊写逐条吻合✅/计数量级正确（±2-3）✅/不确定性诚实自报✅。**结论=质量足够，无需升级路由模型**（决策项不触发）。

## 验收项对照（卡面）

| 验收项 | 证据 |
|---|---|
| run 目录含 agent 原始日志 | `agent_log/agent.log` + `agent_stdout.log` + `vitagent.pid` |
| 读图请求/响应工件 | 请求=prompt+图（sha256/尺寸/bytes 在 `probe_result.json`）；响应=`probe_result.json` text/usage/timings + `probe_stdout.txt` + `llm_telemetry_probe.jsonl` |
| 描述文本质量评注 | `quality_annotation.md` |
| agent 进程读图调用零 panic | `agent_log/agent.log` grep -ci panic = 0；`probe_stdout.txt` = 0 |
| `image_understanding_route_unavailable` 不再出现 | agent 日志 grep -c = 0；探针 route 解析 enabled=true 直接放行 |
| ≥1280×720 真实截图 | 物理 2560×1440（Retina 2x），真实 webui 渲染 |
| 停止条件（端点拒收大图 4xx/5xx） | 未触发（HTTP 200 正常返回） |

## 端测覆盖边界声明（AGENTS §5）

- `CompleteImageUnderstanding` 的调用执行体=本 worktree 构建的一次性探针二进制（真实 OS 进程、生产代码路径 config.Load→&llm.Client{}→CompleteImageUnderstanding，与未来 B4 进程内消费方逐字同 wiring）；**不在 vitagent PID 内**——因生产 agent 进程内当前零调用方（消费方=B4 卡，池序在后）。vitagent 真实进程在位腿（/health+webui 后端）已覆盖"agent 进程在位+enabled 路由不破坏启动"。
- 截图为离屏 WKWebView 渲染快照（无头浏览器同类），非用户桌面截屏。

## 工件清单

```
run1-20261011-2215/
  README.md                     ← 本文件
  config.json.bak-before-enable ← Mac config 启用前备份
  wksnap.swift                  ← 离屏截图器源（工件即源）
  screenshot_webui_1280x720.png ← 被读的真实截图
  probe_prompt.txt              ← 读图 prompt
  probe_stdout.txt              ← 探针 stdout（含模型响应全文）
  probe_result.json             ← 结构化结果（usage/timings/sha256/err）
  llm_telemetry_probe.jsonl     ← 生产 telemetry 面 1 条记录
  agent_log/agent.log           ← vitagent 运行日志（含降级 WARN 底账）
  agent_stdout.log, vitagent.pid, vite_dev.log
  bin/                          ← 构建产物（不入 git：vitagent/wksnap/visfilmprobe 二进制）
```
