# FE-INTERACTION-CARD-KIT-1 真栈腿记录（stack_button_20261030_1–4）

- 日期：2026-10-30 晚；执行：ZCode GLM-5.3 PC 执行会话（224 卡会话）
- 登记：ab200578（推 main 后远端亲读置顶+进程/端口全零即时复核；登记行时戳原写 21:3x 系预估偏差，机器钟实为 21:02:09——按 201 先例在释放行修正）；错峰=223 卡 20:41 登记面已注明与 224 错峰，其 21:1x 释放后本腿占用前 fetch 亲读+全零亲核
- 被测：worktree `D:/Vit_DAW_wt_ick1` @ `port/fe-interaction-card-kit-1`（agent=ps1 自建 `agent/bin/VitAgent.exe` 含声明式 card 契约；webui=worktree vite dev（VITE_AGENT_PROXY→7878 真实 agent）；浏览器=Edge channel headless（python-playwright 1.62））
- kernel：主检出 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe` 只读复用，sha256 `C1E32DFE…` 运行前后零漂移（=217–223 腿同源；本卡 C++ 零改动）；vsphub 不需要（agent 直连内核 ZMQ）
- 隔离：VIT_PROJECT_XML=ps1 共享工程隔离段 git 干净 run 副本（`-AutomationCardButtonSmoke` 已入条件列表）；亲证=run 副本 _4 承接内核写入（AE6E706F，mtime 21:08:12=运行窗）；主检出 default_project.xml mtime 19:58:35 **早于本腿登记 21:02**=本腿零触碰（其 M 态=登记前遗留，与 221/223 记录一致）；stack/port worktree 运行后 git 干净（运行时工程文件不入库，218 先例）

## 迭代轮（探针面，如实留证）

- _1（21:0x）：python 断言体 `Dict[str Any]` 类型注解漏逗号语法错——探针缺陷轮，栈立即手拆（kernel 29768+agent 36624 亲核路径=本腿所属）；修复=sed 全量替换
- _2（21:0x）：vite 启动成功但 `Local:` 横幅带 ANSI 色码（`Local:\x1b[36m` 后才是 URL）致 regex 不匹配超时——探针缺陷轮，栈+vite 子进程手拆（node 16516/20944 命令行亲核=本腿 vite）；修复=strip ANSI 后再匹配；**同轮暴露 terminate() 不杀 Windows 进程树（vite node 子进程残留 5178）→拆栈改 taskkill /T /F**
- _3：正式轮 1——全绿 PASS（见下）
- _4：正式轮 2——全绿 PASS，ps1 真实退出码 `TRUE_EXIT=0`（PowerShell `exit $LASTEXITCODE` 捕获）

## 正式轮结果（_3 与 _4 两轮一致，双轮 exit 0）

| 腿 | 断言 | 结果 |
|---|---|---|
| setup | new_project（隔离副本）+单轨+Volume&Pan 插件在场+曲线零基线 | PASS（track 1007/plugin 1008） |
| **B1 浏览器按钮点击（主断言）** | 真实 webui 页发话术「0 到 24 秒，让这条轨的存在感慢慢强起来」→卡以 interaction_card.v1 渲染（六字段类型 shape:select/start·end·region:number×4/curve:range+approve 确认写入/cancel 取消按钮）→字段编辑 start 0.20/end 0.80/curve -0.25→点「确认写入」→「已写入」settle→卡收敛无残留按钮→**内核 get_curve 逐点=(0,0.20,c=-0.25)/(24,0.80)**（编辑值写入生效） | PASS ×2 |
| R2 话术对照轮（并存） | 轨 2 显式 selected_track_id，「45 到 60 秒…」→卡（payload.card 契约在场）→回「确认写入」→治理 settle written=2→内核 (45,0.30)/(60,0.60) 缺省渐入点 | PASS ×2 |
| R3 responder cancel | 第三提案经 /agent/interaction/respond action_id=cancel→goal_status=cancelled→曲线保持 2 点（零写入） | PASS ×2 |

- 工件：`automation_card_button_summary.json`（status=pass）+`automation_card_button_console.txt`+`card_button_shots/b1_card_before_click.png`+`b1_settled.png`（_3/_4 各一套）；运行时工程文件（default_project.xml/*.vit）剔除不入库
- 零 LLM 参与确定性链（词表路由在模型轮前接管）

## 端测覆盖边界声明（§5）

- 浏览器面=真实浏览器+真实 fetch→vite 代理→**真实 agent（含声明式契约构造+治理写入）→真实内核**全链；webui 静态托管为 vite dev（CEF 宿主管线不在本腿——agent 不托管 webui dist，此项属 Godot 前端域）
- 历史水合 settle 态渲染由 vitest 断言覆盖（已确认/已取消+无按钮），真栈腿未重放刷新场景
- CEF 内嵌 webui 的实际装载体（导出 dist）走 release 装配，不在本卡域
