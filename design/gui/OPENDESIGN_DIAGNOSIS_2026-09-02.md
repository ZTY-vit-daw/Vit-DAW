# OpenDesign 卡在"观察现有 GUI"的诊断记录（2026-09-02）

> 场景：用 Open Design（0.21.1）让 DeepSeek Harness / BYOK API agent 为 Vit-DAW Agent GUI 做设计，
> run 每次都停在 agent 说"我去看一下现有 GUI"之后不再产出。
> 结论：**不是模型不干活，是它想读的 `D:\Vit_DAW\agent\webui`（项目沙箱之外）被权限层自动拒绝**，
> 观察 100% 失败 → run 以 0 artifact 结束（或 empty_output 报错），界面上看起来就是"停住了"。

## 1. 现场证据（本机数据）

OD 数据根目录：
`C:\Users\timoz\AppData\Roaming\Open Design\namespaces\release-stable-win\data`

- run 记录：`runs\<id>\events.jsonl`（22:27–22:35 的 6 次 run，页面目标均为 chat_panel / web-prototype）
- daemon 日志：`logs\daemon\latest.log`

run 47edf8d5（最近一次）：
```json
{"type":"tool_use","name":"bash","input":{"command":"Get-Content ...\"D:\\Vit_DAW\\agent\\webui\\package.json\" ..."}}
{"type":"tool_result","content":"The user rejected permission to use this specific tool call.","isError":true}
{"type":"text_delta","delta":"Now let me inspect the existing agent/webui front-end for stylistic coordination."}
{"event":"end","status":"succeeded","artifactCount":0}
```

run 5e3e42fb（stderr 原始日志）：
```
permission requested: external_directory (D:\Vit_DAW\agent\webui\*); auto-rejecting
tool_use name=glob path=D:\Vit_DAW\agent\webui pattern=**/*
tool_result: The user rejected permission to use this specific tool call.
```

run 95deac1e（byok + glm-5.3-flash）：
```
error: Agent completed without producing any output ... AGENT_EXECUTION_FAILED empty_output
agent_provider_id: byok_opencode, model_id: open-design-byok/glm-5.3-flash
failure_stage: post_tool_resume
```

daemon 日志（认证与各 Provider 体检）：
```
[test:agent] DeepSeek Harness → stream_error: Authentication Fails, Your api key: ****wYfd is invalid
[test:provider] openai open.bigmodel.cn model=glm-4.6 → timeout in 12003ms
[test:provider] openai open.bigmodel.cn model=glm-5.3-flash → 200 in 5121ms
[test:provider] openai api.deepseek.com model=deepseek-v4-flash → 200
```

## 2. 根因

1. **沙箱边界**：OD 项目 = `data\projects\<projectId>\` 内的独立目录。agent 只能自由读写项目内文件；
   任何项目外路径（如 `D:\Vit_DAW\agent\webui\*`）都被判定为 `external_directory`。
   无人交互批准（或策略 auto-reject）→ 工具调用直接失败 →"观察现有 GUI"永远无法完成。
2. **DeepSeek Harness agent 的 API key 无效**：OD 里该连接使用的 key（尾号 `wYfd`）认证失败；
   而 BYOK 直连 `api.deepseek.com` 的 key 是好的（200）。
3. **glm-4.6 偶发超时**（12s abort）→ 不要选 glm-4.6；glm-5.3-flash 正常。

## 3. 修复 / 正确姿势

A. 让 agent 能看到"现有 GUI"的方式（三选一，最稳是 ①）：
   1. 把参考物**放进项目**：截图（直接粘贴进聊天）+ 把 `agent/webui` 关键文件或
      `design/gui/DESIGN_BRIEF.md` 复制进 OD 项目文件区。之后 agent 只读项目内，不再触发拒绝。
   2. 弹出权限请求时点 **Allow**（含"记住"）；若此前选过拒绝，检查权限设置/重置。
   3. prompt 里只给 DESIGN_BRIEF + 截图，明确"不读 D:\Vit_DAW 源码"。

B. 修 DeepSeek Harness 连接：
   - 终端跑 `od agent setup deepseek-harness`（od 不在 PATH 时用安装目录里的 CLI 或先在 OD 设置页看诊断）；
   - Settings → Models：给 DeepSeek Harness agent 填**正确的 DeepSeek API key**（现存的尾号 wYfd 无效）；
   - BYOK 保底路线（当前可用）：`api.deepseek.com` + `deepseek-v4-flash`；或 `open.bigmodel.cn` + `glm-5.3-flash`；别用 glm-4.6。
   - 注意本机 dsh CLI 是 `@deepseek-ai/dsh 0.1.1-rc.2`（候选版），OD 0.21.1 若提示 untested version 属预期，换正式版再看。

## 4. 备选路线（推荐）

本仓库（D:\Vit_DAW）就是 DSH 会话的工作区：DSH 可直接读 `agent/webui/src`、
读 DESIGN_BRIEF、把成品写进 `design/gui/templates`，无沙箱摩擦。见 DESIGN_BRIEF.md §3 交付清单，
可完全绕开 OD 的这一瓶颈（详见对话结论）。
