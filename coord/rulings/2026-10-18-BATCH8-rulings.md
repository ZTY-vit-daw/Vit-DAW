# 2026-10-18 晚窗 批量八裁定（137/139 真栈腿 pass + 141/142 收口 + 140 解阻）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18 午窗确认派发（137/139/140）+ 晚窗真栈窗口执行。
- 基线：origin/main=**7333bb48**（137/139 分支 cherry-pick 后）；本批合入后 main=__MAIN_AFTER__。
- 执行侧：DSH exec-start-draft（task-12）、exec-rendergain（task-13）；协调面由 12:00-18:00 决策流单写。

## 一、FIX-START-DRAFT-SIDE-EFFECT-1（137）——**pass**

- 交付：分支 **port/fix-start-draft-side-effect-1@cc297f64**（生产实现 **b8f0bb9e** + verify cc297f64；已推、worktree 干净；实际基线 origin/main=77a0b02f）。
- 生产 diff：`agent/internal/history/history.go` +220、`working_session.go` +22、`harness/harness.go` +84、`chat/server.go` +28，测试 +873（history 8/harness 5/chat 3 新单测）；配合 118 最小修复保留为第二道防线。
- 实现要点：①读路径零物化（去 `BindProjectIdentity("",uuid)` 副作用；读函数无缓存返回原形状+`available=false/draft_pending=true`；读面不再隐式 ensure root worktree item）；②激活/恢复只读（root 只读扫描→缓存回落→activation pending；成功发布 project path hint）；③写意图显式绑定（save/save_prepare `EnsureProjectPathForWrite`）；④跨工程 uuid 重绑守卫（record+observer WARN，双顺序+16 并发稳定命中）；⑤`CurrentProjectIdentity` 全仓 17 调用方清单+读契约影响面在 FIX.md。
- 验证（决策侧亲核）：**独立全量复跑**（隔离 draft 根）：`go build` exit 0；`go test ./...` **exit 0，101s，95 包 0 FAIL**；真实 draft 根 **10181→10181 零新增**；gofmt 净。
- **真栈腿**：run=**realstack_20261018_1750c**（17:50:30–17:50:47）脚本 exit 0，四断言 **A/B/C/D 全 true**：A 启动/激活新增 draft=**0**（旧行为 1 个 75B 空壳）；D reader 面新增 draft=0/白名单外写入=0（白名单=激活 workspace 请求边界既有持久化 4 项单列）；B `version.project_new` 后新增恰好 **1** 个 draft（旧行为 2：A+B），新 uuid `vitproj_80b78ac7…`；C 新 draft 无旧 uuid 泄漏、种子保留旧 uuid `vitproj_1d7e7367…`；`stale_rebind_warnings=0/activation_pending_lines=0`；LLM=0；teardown 六端口/进程=0（只杀自起 kernel 16944/agent 17764）。
- 边界/偏差采信：首轮未隔离测试写真实 draft 根 18 个测试工件→移入 `%TEMP%\fedraft1\startside_test_quarantine\`（未删，并入 140 处置）；harness.go 注释损坏→checkout 恢复后 edit 重放（最终 diff=b8f0bb9e）；attempt1–3 为断言基线/瞬态竞态修正留档。
- 合入：cherry-pick 入 main（3580474e/581385f8）；卡 137 归 done；**140 解阻**。

## 二、TRACK-LIST-GAP-1（139）——**pass（P2 取证交付）**

- 交付：分支 **port/track-list-gap-1@6bce31fe**（5 commits；零生产代码，仅 `coord/runs/TRACK-LIST-GAP-1/`）；正式 run=**run_20261018_175229**（17:52:20 预检→栈→17:52:44 拆栈只杀自起 pid→17:52:49 六端口/四类进程=0）脚本 exit 0，§8 LLM=0。
- **同帧六面（user-track 口径）**：S0 全 0；**S1×2 外部新建+导入 kernel/hub_cmd/hub_state=6 而 shadow/tool=0（tool≡shadow，tool≠kernel）**；S1_refresh 后全 6（自愈）；S2 外部打开 2 轨 kernel/hub=2 而 shadow/tool=6；S3 agent 路径写全 3。
- 结论：候选①（VSP 通道命中不同内核/编辑实例）**排除**；候选②（新建+导入路径枚举差）**排除为路径特异**（S1×2 与 S2 同签名、S3 反向同步）；**根因坐实**=工具面 `list_tracks` 返回 shadow Summary（每回包含内核 VSP command_ack 却返回 shadow 行；6/6 阶段 tool≡shadow、3 个 stale 阶段 tool≠kernel）；最小复现 **2/2**；自愈链=project.state→VSP state snapshot→shadow.Initialize→tool=6。
- 被测：stock kernel `B6565DCF…`（只读）/ worktree VitAgent `C7A01057…` / vsphub `A1811FA5…`；工件 `coord/runs/TRACK-LIST-GAP-1/run_20261018_175229/`。
- 修复方向（另卡草案）：A 工具面以本命令内核回包为准（shadow 仅 enrichment；陈旧显式 `shadow_stale`+双计数）；B shadow 未知新轨 delta 去抖触发权威快照刷新（保留 orphan 语义）；C 回归用例+消费面锚点（execution_memory.go:229,373-376、message_loop.go:7836）。
- 偏差/纪律：14:58 未登记未 GO 启动事故（自报；run_20261018_145825 仅干跑不作证据；整改 75f3caf2 `-PreflightOnly` 实测只预检+比较口径修正）；kernel-only berth+vsphub；用户 Godot 全程未碰。
- 处置：**新立卡 FIX-TRACKLIST-SHADOW-FRESHNESS-1（池序 143，P1 候选，派发确认=待用户确认）**；cherry-pick 入 main（619f76b9/00c87e8a/a8f53176/84b8ad16/7333bb48）；卡 139 归 done。

## 三、141/142 收口与用户目检

- **142 主整合**已 pass（BATCH7）：前端仓 main=**3ab4dfd**；用户主检出已切 main（tracked 干净），待用户目检。
- **141 MIXER** 修复随 main（3ab4dfd 含 be6da5d）；**待用户真栈复测**（新建轨道→mixer 即时刷新）后 141 卡归 done；本次窗口内未占栈做该复测（栈交用户）。
- **92** GUI 改版已 pass（BATCH6/7 收口）。

## 四、140 放行

- 137 验收通过 → **task-14/FIX-TEST-DRAFT-ISOLATION-1（140，P2）解除 blocked_by**，派 exec-start-draft 串行执行（60 分钟盒；工作树 D:/Vit_DAW_wt_testiso1、分支 port/fix-test-draft-isolation-1 自最新 main；测试隔离不得改产品代码；真实 draft 根前后零新增）。

## 五、下一批候选（待用户排序）

- **143 FIX-TRACKLIST-SHADOW-FRESHNESS-1**（P1 候选，工具面 stale 修复；待用户确认派发；建议修复面=harness/shadow/bridge+消费面用例）。
- **138 42MB 启动恢复**（P1 候选，语义待用户裁定：不恢复/最近工程/显式）。
- **126 KERNEL-L3-TIMEENERGY-1**（P2 已确认待派）、**136 CAP-BOUNDARY-1**（P2，须排池序 54 后串行）。
- **129 宏控制层设计** → **130**（依赖 129 定稿）。
- 其它池卡（VIS-FILM/GENSHELL/GEN-VOCAL/AIGC-DRUM/DEFENSE-SHOW/FE-MOJIBAKE Mac 线等）需逐卡核前提。
