# LAUNCHER-SMOKE-1：启动面真栈烟测腿（332 拆卡 C6，收口卡）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流+用户 GUI 终审
- 池序 339；P1 / 1 天 / 依赖=335+336+337+338 全合入；L2 / PC；真栈（栈登记照旧）

## 目标

现有冒测脚本体系加 `-LauncherFlow` 参数（dev_agent_smoke.ps1 块或同模式新脚本段）：

1. **主链**：三件套拉起→启动台白话「打开小星星乐局」→directive→（有在开工程时）治理确认→进 dock→现有浮动窗自动带出+桥接播报首条在场→对话继续一轮（工程流标记）→exit 0；
2. **确定性退路腿**：结果卡点选开工程（零 LLM 依赖）全链 exit 0；
3. **降级腿**：不启 agent→经典磁带流 open 可用+引导卡在卷→exit 0；
4. §8 跑前口径成文（LLM 轮次数/预算/失败分类/止损线——复用 320 装置模式）。

## 文件域

`scripts/`（脚本扩展）+工件目录规范；栈协议两条登记。

## 验收标准

三腿各 exit 0+工件（report.json+日志）回指；渲染面/用户旅程边界显式声明（AGENTS §5）——GUI 最后一米=用户手测（手测 v2.5 入口）。

## 领取/回执/验收

### 领取（执行侧回填 2026-11-09 晚窗）

- 领取时间：2026-11-09（晚窗）
- 领取基线：origin/main `e91e1396ba7f7f09117e96f9a7b935dea68a7fb0`（fetch 重读亲证，卡仍在 todo、无 owner）；worktree HEAD=e91e1396、`git status --short` 空、`git diff --stat` 空（净树建枝）
- owner：ZCode GLM-5.3 L2 执行子代理（339smoke 会话）/ PC
- 分支：`port/launcher-smoke-1`；worktree：`D:/Vit_DAW_wt_339smoke`（独立 worktree，主工作树未触碰）
- 领取提交：本笔 coord/339-claim（仅 coord/ 变更，推 main 后核对远端卡面 owner）
- 前置核对：卡在 todo ✓ 派发已确认 ✓；依赖亲证=agent 仓 origin/main 含 launcher_flow.go/launcher_flow_test.go（337/338 已合入）✓、前端仓 D:/Godot/project/vit-daw-frontend HEAD=ff301b3=本地 main（335/336 启动面在场：launcher_chat_panel/launcher_nav_bridge/launcher_governance_dialog+71 检探针）✓；doing 三卡文件域核对=WAVESHELL-LOADING-FORENSICS-1 判别腿候栈候用户定向（其 scripts 域=既有 -WaveshellStacklegSmoke 参数复用，无在飞 diff；本卡 -LauncherFlow 为新增参数块，不相交）、ARRLOOP-AUTHBRIDGE-1（harness 装载确认域）、CLOSURE-ROUTE-ADAPT-1（chat 路由/closure 预算域）零冲突 ✓；真栈腿将按 §2.2 登记（当前栈记录=空闲 20:09 释放）

### 回执（执行侧回填）
