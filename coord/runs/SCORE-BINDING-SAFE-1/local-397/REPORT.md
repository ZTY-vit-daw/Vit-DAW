# 397 SCORE-BINDING-SAFE-1 执行回执（零栈自验）

- Owner：`/root/score392_recon`，PC；branch=`codex/score-binding-397`。
- Worktree：`C:/Users/timoz/.codex/worktrees/score-binding-397/Vit_DAW`。
- 领取基线：`bb10c48b17d78608fec0d456ca3feb4fbfaf24a7`；初始 `git status --short` / `git diff --stat` 均空。领取已由主管同步共享 main；本执行不改卡态、不推 main、不自裁终验。
- 状态：生产实现完成，局部/全量 WebUI 与零栈渲染自验通过；**等待主管复核与真实栈验收，未达到最终交付门槛**。

## 实现

生产文件恰好5个：`scoreworkbench/ScoreTrackWorkbench.tsx`、`binding.ts`、`ScoreWorkbenchPanel.tsx`、`scoreworkbench.css`、`workbench/Workbench.tsx`（均 `agent/webui/src/` 下）。

- 左侧轨道列表显式选择；头部常驻显示轨道和片段身份。只接受 `clip_type=midi`；单MIDI片段选轨即装载，多片段显式选段；空轨、音频轨、缺失类型各有提示。
- 编排容器每次完成读取后再安排下一次轮询，刷新当前工程路径/轨道与片段存在性；不消费 GUI 选轨来改变编辑目标（398边界）。
- 缓冲绑定成功装载的 project/track/clip key。dirty 切换弹出“继续编辑 / 放弃修改并切换”；无自动 apply。装载/写入中禁切换；读取失败、项目变化、删除/改类型、接口离线锁定写入，保留脏缓冲供查看。恢复连接时不覆盖脏缓冲。
- load/apply generation 失效的回包不更新模型、snapshot或已保存态；重复确认通过同步ref锁阻止。apply 使用既有明确clip+track桥，写前/回读后复核当前目标，仍由原桥做单次confirmed replace_region与音符比对。
- 保留本地编辑模式及记谱核心；未改 kernel/model/render/derive/edit、App、launcher、project、backend、Godot或全局CSS。
- 经主管明确授权，`scripts/score_workbench_smoke.mjs` 仅迁移原两处选段前置：从状态清单找到轨道 → Playwright点击生产轨道按钮 → 原片段选择。J1/J2/J3原断言保留；本执行未运行真栈脚本。

## 命令与结果

所有 npm/node命令在本worktree `agent/webui/` 执行，另标仓库根目录者除外。

| 命令 | exit | 证据/说明 |
|---|---:|---|
| `npm ci` | 0 | 本worktree独立node_modules；未共享可写依赖目录 |
| 初次 `npm run build` | 1 | 迁移import时漏保留尾部re-export依赖 `createNoKernelBridge`；TS2304，已恢复import后修正。未归类为环境或flaky |
| `npm run test -- src/scoreworkbench/ScoreWorkbenchPanel.test.tsx` | 0 | 原17项通过；桥测试补显式target。apply错误测试先成功load，再测试拒绝，避免继续允许load失败后写入 |
| `npm run test -- src/scoreworkbench/ScoreTrackWorkbench.test.tsx` 首轮 | 0 | `binding-vitest.log`，9项 |
| `npm run build` 修正后 | 0 | `build.log` |
| `node src/scoreworkbench/render-smoke.mjs local-397/render-1` | 0 | `render-1.log`及其报告/截图，11断言 |
| `npm run test` | 0 | `full-vitest.log.gz`，59文件/715项。压缩保留原始字节，SHA256/原字节数见`full-vitest-log-integrity.json`。既有VexFlow/jsdom canvas测量警告在原始输出中，未掩盖 |
| `npm run test -- src/scoreworkbench/ScoreTrackWorkbench.test.tsx` 最终 | 0 | `binding-vitest-final.log`，11项。全量之后补两项：刷新不跟随GUI、pending重复确认/拒绝保缓冲；未将全量计数虚报为717 |
| `npm run build` 最终 | 0 | `build-final.log`；既有大chunk提示保留 |
| `node src/scoreworkbench/render-smoke.mjs local-397/render-final` | 0 | `render-final.log`、`render-final/render-smoke-report.json`，11断言、0 pageerror |
| `node --check scripts/score_workbench_smoke.mjs`（根目录） | 0 | 两处真栈前置迁移语法通过 |
| `node --check agent/webui/src/scoreworkbench/render-smoke.mjs`（根目录） | 0 | 本卡零栈render脚本语法通过 |
| `git diff --check`（根目录） | 0 | 无空白错误 |

新增反例覆盖：dirty A→B取消/放弃后B读取失败；apply pending禁切/禁编辑并切工程后忽略旧回包；load pending跨工程（复用相同轨/clip ID）；clip删除/改音频与恢复后保脏；写前新鲜状态拒绝跨工程；状态接口离线；多MIDI/音频/空轨/未知类型；明确目标写入成功回读；不跟随GUI；重复确认只写一次/拒绝保缓冲。

## 渲染与覆盖边界

渲染扩展沿用现有 `createServer(Vite) + chromium(headless) + report/screens` 模式，真实 `index.html -> main/App -> Workbench -> ScoreTrackWorkbench -> ScoreWorkbenchPanel` 装配，不是探针自行创建Panel替代生产挂载。状态与工具响应由浏览器HTTP路由fixture提供，测试实际API调用缝。覆盖无隐式目标、空轨、多片段显式选择/音频排除、身份条、dirty取消、B读取失败禁写、明确target+confirmed apply；1440与420宽截图人工检查。截图在 `render-final/screens/`。

**零栈边界**：本执行未启动VitApp、Go agent、Godot、VitShell；未占用PC真栈；没有真实内核写入/持久化或GUI点击披露链证据；J1-J3只保留断言，尚未复跑。截图中的“已保存”仅代表fixture返回经既有桥比对一致，不作为真实保存证据。

**接口限制**：当前 `/agent/ui/state.project` 只有project_path，没有project_uuid或epoch；使用披露路径构成身份键（反斜杠归一化），不以ui_context旧选轨缓存冒充身份。前端写前/写后检查不能提供内核级project CAS，不能取消已经发出的内核请求；若要求消除验证到执行之间并发切工程的TOCTOU窗口，应由主管另开后端协议卡。本卡证明目标绑定、写前fail-closed和延迟回包隔离，未声称增加该协议保证。

## 待主管安排的真栈验收

1. 按资源协议独占真实栈，使用现有ps1体系和隔离工程副本；前后核对fixture哈希，遵守recents快照/还原纪律。
2. 在生产Workbench选轨→编辑→显式apply→HTTP/ZMQ核对目标音符回读、非目标未改；覆盖读取失败禁写和多片段选段。
3. 运行迁移后的 `scripts/score_workbench_smoke.mjs` 既有腿，J1/J2/J3与原A面断言均需exit0。此处只迁控制前置，不新增替代判据。
4. 保存完整命令/exit/runID/HEAD/dirty状态/d1_smoke_report与agent原始日志；用户手测仍从Godot启动链进入，不能拿本零栈结果替代。

分支与worktree保留供主管复核；本回执不表示决策验收或已合入。
