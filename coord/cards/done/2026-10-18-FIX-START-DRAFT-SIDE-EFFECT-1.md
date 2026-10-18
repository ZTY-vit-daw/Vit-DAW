# FIX-START-DRAFT-SIDE-EFFECT-1：启动读路径不得物化 draft + 跨工程 uuid 泄漏守卫（118 归因拆出）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) [ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §2.1/§2.4 + 真栈 D/E 轮 reader 实证）
- 派发确认：**待用户确认**（P1；118 最小修复已合入，本卡为其根因面；建议下一可用窗口派发）
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 137；目标仓库=D:\Vit_DAW；**P1** / 预估 0.5-1 天 / 无依赖
- 模型分级：L2 / GLM 执行会话（agent Go 启动/历史面）

## 根因与证据（118 已取证，勿重复）

1. **启动读路径物化 draft**（已证）：`Server.Start()`→scheduler 首 tick `recoverContinuationWorkspace`（continuation_scheduler.go:778-811）→`CurrentProjectIdentity`（harness.go:518-530）→`BindProjectIdentity("", 内核uuid)`（history.go:131-155）→`defaultDraftProjectPath`（history.go:2260-2288）建 75B `Unsaved.vit` 空壳 draft。读路径不应有落盘副作用。
2. **启动期自主历史 reader**（已证，118 真栈 D/E 轮）：agent activation 后约 **1.5–2s** 出现无外部请求来源的 `harness.invoke tool=version.list / version.worktree_list`，把隐式 draft 的 `.vit_history/vitproj_<旧uuid>/state/worktrees` 物化，使 pristine 前置丢失。因此 118 最小修复仅在「通知早于 reader 落盘」时生效（用户真实时序=是；首装空根仍双 draft）。证据：`coord/runs/FE-START-EMPTY-DRAFT-1/realstack_d_clean_20261018_1421/probe_race/`、`agent_last_snapshot.txt`。
3. **跨工程 uuid 泄漏竞态**（round2 已证，1/5 命中）：`ProjectNew` 后 shadow 刷新完成前并发读路径用旧 uuid 再走 `BindProjectIdentity("", 旧uuid)`→`Open`→`migrateLegacyHistoryWorkspace`（history.go:196-223/2804-2858）在**新 draft 内**物化旧 uuid workspace。

## 目标

- 读路径（identity probe / 自主 reader / shadow 刷新）**零落盘、零 draft 物化**；
- reader 不得破坏 draft pristine、不得跨工程物化历史；
- 泄漏守卫：路径/uuid 绑定校验（旧 uuid 不得在新 draft 内落地）；
- 118 的最小修复（空壳 draft 复用）保留为第二道防线，不回退。

## 文件域

`agent/internal/history/{history.go,working_session.go}`；`agent/internal/harness/harness.go`（CurrentProjectIdentity/reader 调用面）；`agent/internal/chat/server.go`（如恢复路径必要）；同包测试；`coord/runs/FIX-START-DRAFT-SIDE-EFFECT-1/` 工件。越域即停并上报。

## 验收标准

1. 单测：①identity probe/reader 调用后 draft 根**零新增目录/文件**；②pristine draft 在 reader 路径后仍 pristine（或 reader 不再物化）；③并发旧 uuid 绑定不泄漏（复刻 round2 竞态）；④既有 118 两测试与全量测试保持绿。
2. `cd agent && go build ./... && go test ./... -count=1` 0 FAIL + gofmt 净。
3. 真栈（AGENTS §5，PC-RUNTIME-STACK 登记后）：复刻 118 realstack 场景（含 reader 抢跑）断言 `version.project_new` 后 draft 根新增=0、无旧 uuid 泄漏；run ID/工件/被测 agent SHA 入回执。
4. 用户手测 4 条（新建工程 draft 根只新增 1 个 / 打开既有工程不落空壳 / 导入后问轨数 / track.list 轨数一致）由用户排期。

## 停止条件

- 需改内核/前端协议或跨层设计 → 先上报拆卡；
- 不得用弱化 draft 判定/跳过 reader 场景凑通过；
- 不得删除 118 已合入的最小修复。

## 关联

- 发现卡：FE-START-EMPTY-DRAFT-1（118，done；最小修复+适用边界在卡面/REALSTACK_REPORT）
- 相邻：FIX-START-STATE-RECOVERY-1（139 前置语义）、TRACK-LIST-GAP-1（不同根因）

- 领取：2026-10-18 14:55 +0800 / origin/main=3abe0cfb / owner=DSH PC 执行流 exec-start-draft（模型+机器+会话）/ 分支=port/fix-start-draft-side-effect-1（自 3abe0cfb）/ worktree=D:/Vit_DAW_wt_startside1（新建）/ 领取提交=待建；task-12；用户 2026-10-18 确认派发

- 回执（2026-10-18 晚窗，exec-start-draft 执行，Lead 转写）：分支 **port/fix-start-draft-side-effect-1@cc297f64**（生产实现 b8f0bb9e；verify commit cc297f64；已推，worktree 干净；实际基线 origin/main=77a0b02f——卡面领取行 3abe0cfb 为发卡时点，验收以 77a0b02f 为准）。**实现**（文件域内，前端/内核零改动，118 修复原样保留）：①读路径零物化（CurrentProjectIdentity 去 BindProjectIdentity("",uuid) 副作用；Status/List/WorktreeList path 空时只读进程内缓存、无缓存返回原形状+vailable=false/draft_pending=true；读面不再隐式 ensure root worktree item）；②激活/恢复只读（RecoverableProjectPathForUUIDInRoot 只读扫描→缓存回落→ctivation pending 跳过，不建 draft；激活成功发布 harness project path hint）；③写意图显式绑定（save/save_prepare EnsureProjectPathForWrite）；④泄漏守卫（拒绝 active workspace 改绑旧 uuid，记录 ProjectIdentityRebindRejection 32 条+observer WARN，两种绑定顺序+16 并发测试稳定命中）；⑤CurrentProjectIdentity 调用方全仓 17 处清单+读函数契约影响面逐条列 FIX.md §2。**验证**：go build ./... exit 0；隔离 draft 根 go test ./... -count=1 exit 0（95 包 ok/0 FAIL）；真实 draft 根 10181→10181 零新增；新增单测 16 个（history 8/harness 5/chat 3）覆盖四类+D/E reader 抢跑场景；gofmt 净。**真栈腿**：run=**realstack_20261018_1750c**（17:50:30–17:50:47，Lead GO 窗口内）脚本 exit 0，四断言 **A/B/C/D 全 true**：A 启动/激活阶段新增 draft=**0**（旧行为 1 个 75B 空壳）；D reader 面新增 draft=0/白名单外写入=0（白名单=激活 workspace 请求边界既有持久化 4 项单列）；B ersion.project_new 后新增恰好 **1** 个 draft（旧行为 2：A+B）、新 uuid itproj_80b78ac7…；C 新 draft 无旧内核 uuid 泄漏、种子 draft 保留旧 uuid itproj_1d7e7367…；stale_rebind_warnings=0/activation_pending_lines=0；LLM=0；teardown 17:50:47 六端口=0+本腿进程=0（只杀自起 kernel 16944/agent 17764）；用户 Godot 未触碰。**偏差**：①首轮未隔离测试向真实 draft 根写 18 个测试工件→已移入 %TEMP%\fedraft1\startside_test_quarantine\（未删，并入 140）；②中途 pwsh 文本改写损坏 harness.go 注释→git checkout -- 后仅用 edit 重放，最终 diff 以 b8f0bb9e 为准（已登记）；③attempt1–3 为 D 断言基线/瞬态 lock 竞态修正（非产品失败），stdout 留档 evidence/attempts/。
- 验收：**pass（2026-10-18 晚窗）**——[rulings/2026-10-18-BATCH8-rulings.md](../../rulings/2026-10-18-BATCH8-rulings.md) §一；决策侧亲核=生产 diff 亲读（文件域内/118 修复保留/读契约 compatibility/守卫可观测/写路径显式 ensure）+**独立全量复跑**（隔离 draft 根：go build exit 0、go test ./... exit 0、101s、真实 draft 根 10181→10181）+真栈工件亲读（stack_leg_result.json 四断言 true/failures=[]/reader_new_dirs=[]/reader_forbidden_diffs=[]/new_dirs_after_project_new 恰好 1/新 uuid 与种子旧 uuid 分离/teardown ports_listening=[] stack_processes=[]）+远端 hash 亲核；偏差采信；cherry-pick 入 main（3580474e/581385f8）；卡归 done。**140 解阻**（task-14）随本裁定放行。