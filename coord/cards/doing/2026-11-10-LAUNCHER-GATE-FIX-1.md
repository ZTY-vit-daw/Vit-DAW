# LAUNCHER-GATE-FIX-1：launcher 门判据修复（uuid→会话语义）+339 腿 1/2 复跑（340）

- 发卡：GLM 主管决策流 / 2026-11-10 凌晨（依据=[339 取证](../runs/LAUNCHER-SMOKE-1/report.json)：337 `launcherFlowApplicable` 的 `CurrentProjectIdentity.uuid==""` 判据在真栈结构性不可达——Phase A default_project.xml 自带 uuid/Phase B VspKernelReference.cpp:721 无条件占位 `project_current`→shadow.go:196 恒非空；336 UI 用 path 语义/337 门用 uuid 语义不同构；337 单测以 fake kernel 绕过该面）
- 派发确认：已确认（主管发卡，P1 关键路径——手测 v2 的最后一块）
- 验收负责人：GLM 主管决策流
- 池序 340；P1 / 0.5-1 天 / 依赖=339 装置已合入（RUNBOOK/复跑命令现成）；L2 / PC；零栈修+真栈复跑

## 修复设计（主管裁定，方向已定细节卡内定）

**门语义改为「UI 声明 + 会话事实」双门，废弃 uuid 空判**：
1. **主门=UI 声明**：`context.launcher==true`（设计稿 §3.1 原义：launcher 提示不是权限声明，仅路由分支——UI 在启动台态自报，可信面=非权限边界）；
2. **副门=会话事实**：agent 进程内 `user_entered_project` 位——boot 起 false，收到 `version.project_opened` 播报置 true（进程内不重置）；launcher 分支要求 `launcher==true && !user_entered_project`；
3. uuid/path 判据**移除**（真栈不可达证据在卷）；336 UI 的 path 自查（治理确认面）维持不动——两面语义差异在卡面注明（UI 管"要不要确认"，BE 管"走不走 launcher 路由"）；
4. placeholder `project_current` 识别不做（VSP 面域外，会话位已覆盖语义）。

## 目标

1. `launcher_flow.go` 门修复+`user_entered_project` 位接线（server.go 既有 project_opened 接收点 ~10 行）；
2. 单测：placeholder/default uuid 不再阻塞门（用 339 取证的真栈形态做 fixture：uuid=占位/带默认工程 uuid 两型）；project_opened 后位置 true 且门关；launcher 位不影响 authority 断言回归；
3. **339 腿 1/2 复跑**（装置/RUNBOOK 现成）：主链（白话开工程→directive→治理确认→进 dock→桥接播报→续聊 context_flow=project）exit 0 + 确定性退路腿 exit 0——§8 口径照 339 冻结（腿 1 ≤10 LLM 调用）；栈协议两条登记；
4. 338 的桥接播报/续聊段随腿 1 一并真栈证成（门修后自然达）。

## 文件域

agent/internal/chat/launcher_flow.go+server.go（project_opened 接收点）+测试；工件 coord/runs/LAUNCHER-GATE-FIX-1/（复跑 run 记录）；前端零改动。

## 验收标准

单测全绿+chat 全包不回退+**腿 1/腿 2 exit 0**（复跑工件在卷）——手测 v2 就绪的最后判据。

## 停止条件

门修复后腿 1 仍败且定位进前端消费域 → 取证上交（前端改动需主管授权另卡）。

## 领取/回执/验收

- 领取：2026-11-10 00:03 +0800 / origin/main 7744c5b1 / owner ZCode GLM-5.3 L2 执行子代理（340gate 会话）/ PC / 分支 port/launcher-gate-fix-1 / worktree D:/Vit_DAW_wt_340gate / 领取提交=coord/340-claim（hash 推送后回填）
- 回执：（执行侧回填）
- 验收：（主管回填）
