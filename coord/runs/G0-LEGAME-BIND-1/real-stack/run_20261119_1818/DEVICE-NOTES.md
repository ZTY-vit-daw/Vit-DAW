# 425 真栈腿装置记录（2026-11-19）

- 卡：G0-LEGAME-BIND-1（池序 425，W01-C）
- 候选：main `33650e22`（425 实现 `bef5e9c1` 已合入）
- berth：`agent/bin/VitAgent.exe` + `VIT_AGENT_SPAWN_RUNTIME=1 VIT_DAW_DEV_ROOT=D:/Vit_DAW`，agent 自拉 headless kernel（`VitApp.exe`）+ VspHub；无 VitShell/GUI
- 二进制：VitAgent `3D9BAC1C147343ECABF0B9F475954A08F95BC54F33721B37628FE4C0548B1CB3`（2026-11-19 18:16）；kernel `A59473614152882B`；hub `B10CC41295066C48`
- 栈位登记：main `9f65db40`（coord/425 berth reserve）

## 轮次

| 轮 | 结果 | 分类 |
|---|---|---|
| attempt1 | 解析失败（Windows PowerShell 5.1 将无 BOM UTF-8 中文脚本读成 ANSI） | 装置/编码 |
| attempt2 | S0–S4 PASS，S5 失败 `edu_jam_no_session` | 装置/测试期望（命令词「乐局状态」按设计跳过 auto-intro；S5 不能用它取证） |
| attempt3 | S0–S7 PASS，`summary.status=passed` | 正式验收轮 |

## attempt2 的契约错配（已修）

1. `AgentBaseUrl` 默认 8787（VspHub）→ 7878（agent HTTP）；S0 增加 `/agent/state` 预检。
2. `/agent/invoke` 请求体必须用 `args`（不是 `arguments`）+ `confirmed=true`；`project.open` 的 kernel 参数是 `file_path`（不是 `path`）。
3. `/agent/state` 的有效工程字段是 `current_project.{path,uuid}`（回退 `project_history`/`shadow`），不是 `active_project_*`。
4. S5 用新会话自然语句（「我回来了」）触发 auto-intro；S6 的「结束乐局」沿用 S5 会话。验收核心仍是 cache_key 不变 + 装载卡 id 一致。
5. 脚本以 UTF-8 BOM 落盘（Windows PowerShell 5.1 中文脚本前置）。

## 权威输入核对

- recents：berth 前快照 `63D0F3F7…DF7B`，teardown 还原后一致（`recents.before.json`）。
- `VitApp/Workspace/Settings/Settings.xml`：前后一致（未改）。
- `VitApp/Workspace/default_project.xml`：**被内核改写**，berth 前 `5BC9C756…5616` → 当前 `E9B668A0…D1E9`。
  - 原因：测试中 `project.new`/`save_as` 时内核更新默认工程模板；berth 只记了哈希、未复制权威文件，无法逐字节还原。
  - 预跑内容：六个 stems 的测试 fixture（路径 `D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\...\spv1_p01\stems\*.wav`），属测试残留，不是用户工程；当前内容=空白 120bpm 默认工程。
  - 处置：如实记录并保留当前文件副本；后续真栈腿必须**在 berth 启动前复制**权威文件并在 teardown 还原（见下）。

## 零 LLM 证据

- `agent_llm_telemetry.jsonl` 未生成（零调用）；全部 chat 走确定性 edu_jam 路由。

## 后续装置修订（本轮未重跑，供下一真栈腿采用）

- berth 包装层在启动前复制 `Settings.xml`/`default_project.xml` 到 run 目录，teardown 还原并哈希亲证；不把 A/B 建工程放在脚本之外。
- 425 脚本增加权威文件快照/还原段；或在 `run_legame_bind_smoke.ps1` 内建 A/B 创建（需新的 `-SetupProjects` 门），避免 setup 阶段污染。

## 工件

- 正式轮：`coord/runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/legame_final/summary.json` + requests/responses/manifest/binding/design 副本
- attempt2：`coord/runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/legame/summary.json`（S5 失败原样保留）
- berth：`coord/runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/berth.json`、`teardown.json`、`agent_last.log`、`agent.stdout.log`/`agent.stderr.log`
