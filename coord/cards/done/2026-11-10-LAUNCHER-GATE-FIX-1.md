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

- 领取：2026-11-10 00:03 +0800 / origin/main 7744c5b1 / owner ZCode GLM-5.3 L2 执行子代理（340gate 会话）/ PC / 分支 port/launcher-gate-fix-1 / worktree D:/Vit_DAW_wt_340gate / 领取提交 9e9117ae（已推 main 亲证）
- 回执（2026-11-10 00:20 +0800，执行侧自验完成候主管验收）：
  - **实现 commit af55101b + 工件 commit 013eddb4**（均 @ port/launcher-gate-fix-1，已推远端）
  - diff 文件清单（5 文件 +203/-39）：agent/internal/chat/launcher_flow.go（门=UI 声明+user_entered_project 双门，uuid 判据废弃+位方法）、agent/internal/chat/server.go（Server 字段+handleInvoke project_opened 到达即置位）、agent/internal/chat/launcher_flow_test.go（门测试重写为 4 子用例：339 PhaseA/PhaseB 真栈形态 fixture 不阻塞门+project_opened 到达门关+authority 不变）、scripts/launcher_flow_smoke.py（两处装置断言随门语义同步：assert_gate_open 改记录面[门开行为证明=directive 回执]+零摩擦前提改弱断言[真栈 boot path=草稿回退恒非空，339 证据；两面语义差异已在驱动注释注明：UI 管"要不要确认"=真栈恒走确认卡，BE 管"走不走 launcher 路由"]）、coord/runs/LAUNCHER-GATE-FIX-1/RUNBOOK.md（§8 跑前冻结）。驱动断言同步=门修复链必要组成（旧断言是 337 门镜像，不改则腿 1 必 exit 4、验收标准自相矛盾）；336 前端零改动（卡面维持不动达成）
  - 单测：TestLauncherFlowGateUIFlagAndSessionFact 4/4 PASS；chat 全包 `go test ./internal/chat/ -count=1` → **ok 158.741s 零回退**；`go build ./...` exit 0
  - **腿 1 主链 exit 0**（run=coord/runs/LAUNCHER-GATE-FIX-1/main_20261110_001205）：boot 形态与 339 取证同款（uuid=default 自带+path=草稿回退）→ 门开行为证明=directive 回执全字段 → project_opened 位置位点 → 绑定观察 → 桥接播报（launcher_bridge_broadcast/volume_switch/已进入《小星星乐局》）→ 工程流 context_flow=project → UI 锚点三件；**338 桥接/续聊段随本腿真栈证成**
  - **腿 2 退路 exit 0**（run=coord/runs/LAUNCHER-GATE-FIX-1/fallback_20261110_001320）：2 张 launcher_project 结果卡+点卡[0] 同链开工程+桥接+telemetry 恰 0 行
  - §8 对账：腿 1 LLM 实耗 **1 调用**（deepseek-flash 684 in/165 out，≈¥0.001）≪ 上界 10；腿 2 = 0；单 run 无重跑（两腿各一次过，无 attempt）
  - 栈登记两条：占用 baa8c4e2（净空亲核 00:10:48+≤60s 复核）/释放见本批提交（TEARDOWN_ALL_ZERO 亲核 00:15:20+recents 哈希一致还原）
  - 渲染边界（AGENTS §5）：断言面=agent HTTP 回执+内核 ZMQ+真实 UI 进程/日志/锚点；GUI 渲染观感/浮动窗视觉呈现=手测 v2（本卡 pass=手测 v2 启动面就绪）
  - 工件：coord/runs/LAUNCHER-GATE-FIX-1/（RUNBOOK+两 run 目录全保真+report.json 对账）
- 验收：（主管回填）

### 终验（主管，2026-11-10 晨）

- **pass 整卡合入 main**——主管亲测门测试 4/4+build exit 0；真栈两腿双 exit 0 采信（boot 形态与 339 取证同款=修复针对性强；§8 对账 1 调用 ≈¥0.001；栈两登记+recents 哈希还原亲证）。装置断言两处随门语义同步**追认**（门修复链必要组成，卡面理由成立）。338 桥接/续聊段随腿 1 真栈证成——启动面 C 链（C1/C2/C4/C5/C6+修复）全链收口。
- **里程碑：手测 v2（AI-first 启动面）就绪**——步骤清单另发（coord/transfer/2026-11-10-launcher-handtest-steps.md）。
