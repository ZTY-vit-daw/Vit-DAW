# HYGIENE-FASTPATH-1 回执（执行侧自验）

- 执行 owner：GLM-5.3-flash 执行会话（PC / ZCode flash）
- 领取基线：origin/main 204f0cc8（领取提交 5558a0ce，卡 coord/cards/doing/2026-10-10-HYGIENE-FASTPATH-1.md）
- 实现分支：`port/hygiene-fastpath-1`，实现 commit **ed9f5ade**（已推远端）
- 领取时 worktree（D:/Vit_DAW_wt_hygiene_fp1）status/diff 均为空；本卡新增 diff=4 文件（2 改 2 新增），无领取前遗留改动混入

## 挂账①：三副本机械等价守卫（gain_staging_ref_drift_test.go）

测试文件：`agent/internal/fastpath/gain_staging_ref_drift_test.go`（新增）

- 提取口径：go/parser（ParseComments）按**声明名**定位 FuncDecl，提取 `func` 关键字至收尾大括号（不含 Doc 溯源注释）；不依赖行号。
- 归一口径：仅 IMPL-C 登记的引用改名（`gainStagingRefRegisteredRenames` 四条：messageLoopGainStagingExplicitRequest/StaticBalanceIntentReference/GainStagingStrictReferenceIntent→去前缀导出名 + messageLoopTextHasAny→TextHasAny，整词边界替换）+ 空白归一（逐行 trim、行内空白折叠单空格、丢空行，保留行结构与 token 边界）。**发现未登记的 messageLoop 前缀引用即失败**（强制显式登记，禁止静默放行）。
- 失败消息：打印首个差异行号 + 双方上下文窗口 + 归一后全文（fail-visible）。

### 三组声明提取锚点（测试运行时实测，2026-10-10）

| 副本（internal/fastpath/gain_staging_ref.go） | 原件（internal/agentloop/static_mix_gain_staging_context.go） |
|---|---|
| GainStagingExplicitRequest :10 | messageLoopGainStagingExplicitRequest :1746 |
| StaticBalanceIntentReference :35 | messageLoopStaticBalanceIntentReference :1770 |
| GainStagingStrictReferenceIntent :43 | messageLoopGainStagingStrictReferenceIntent :1777 |

三组锚点与卡面登记一致（原件 1746-1768 / 1770-1775 / 1777-1798）——**当前无漂移，守卫写绿一次通过**（停止条件①未触发）。

### 守卫有效性双向验证

- 正向：三组对照 + 守卫自检全绿。
- 负向（漂移注入探针）：临时从副本删去词条 `"b1_2"` → `TestGainStagingRefCopiesStayVerbatimWithOriginal` 立即 FAIL，差异定位"归一后第 15 行"；还原后复绿。守卫非恒真（另有 `TestGainStagingRefDriftGuardDetectsContentDrift` 比较器自检）。

## 挂账②：newFastPathRouter 轮中 panic → 显式 run 失败

原锚点核对（与卡面一致）：`message_loop.go:430` 构造、:443-444 `slices.Equal` 不等即 panic、`:459`（loop() 内）每轮构造=轮中路径。停止条件②未触发。

### panic→fail 改动 diff 位置

- `agent/internal/agentloop/message_loop.go`
  - `newFastPathRouter`（改后 :433-452）：签名改 `(*fastpath.Router[…], error)`；内联 panic 判定提取为 **`fastPathRegistrationDrift(names []string) error` 判定面**（改后 :459-468，错误消息原文保留"fastpath 注册面漂移…Names()=%v want=%v（L1-5-IMPL-C 完整性契约）"，双侧词条清单完整）。
  - `loop()` 消费点（改后 :477-481）：`fastPaths, err := l.newFastPathRouter()`；漂移分支 = `state.trace` 追加 **Kind=`fastpath_drift`** 事件（消息含注册面漂移关键词+词条清单）→ `r.fail(state, err)` 显式 run 失败。
- `agent/internal/agentloop/pull_session.go` :133-138（**卡面文件域机械扩展，上交裁定**）：`newFastPathRouter` 签名变更的第二个消费点（原 :135 `router: l.newFastPathRouter()`）。适配为零逻辑机械改写——漂移时 `return nil, err`，经 `newPullSession` 既有 error 通路由 `runPull`（pull_entry.go:106-109）`r.fail` 为显式 run 失败。判定依据：不改则编译断裂；改法=卡面目标"漂移=显式 run 失败"在 pull 路径的同一语义延伸，无行为逻辑新增。请验收侧裁定认可。

### 契约零变化声明

`fastpath.DefaultEntryNames`、注册序、`router.go` 注册逻辑、全部 preflight handler 行为零改动；fail-visible 未弱化为日志静默（错误仍显式失败 run + trace 事件）。

### 漂移分支测试（agent/internal/agentloop/fastpath_router_drift_test.go，新增）

- `TestFastPathRegistrationDriftDetectsListDeviations`（3 子例：缺项/乱序/多项）：构造漂移 router 断言判定面返回显式错误（含关键词+双侧清单），recover 探针证明无 panic 逃逸。
- `TestNewFastPathRouterCanonicalRegistrationSucceeds`：正常路径回归锚——无漂移时构造成功且注册面与契约逐项恒等。
- `TestMessageLoopFastPathDriftFailsRunExplicitlyWithoutPanic`：端到端——临时扰动 `fastpath.DefaultEntryNames`（defer 恢复；包内无 t.Parallel，串行安全）使真实构造面漂移，断言 `Start()` 显式失败（Status=Failed、错误含"fastpath 注册面漂移"+"want="、trace 含 `fastpath_drift` 事件、**LLM 零调用**）、不 panic。

## 验收命令与退出码（2026-10-10，worktree D:/Vit_DAW_wt_hygiene_fp1@ed9f5ade）

| 命令（cd agent） | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./internal/fastpath ./internal/agentloop -count=1` | ok（fastpath 0.446s / agentloop 6.424s），exit 0 |
| `go test ./... -count=1`（全量） | exit 0，0 FAIL（后台运行，日志仅留尾部 30 行，exit 0 为准） |
| gofmt（触碰文件 blob 级，CRLF 检出归一后判定） | 5 文件全 CLEAN（工作区 CRLF 为 Windows 检出伪象；新测试文件为 LF） |

触碰文件（4+1）：`internal/fastpath/gain_staging_ref_drift_test.go`（新）、`internal/agentloop/fastpath_router_drift_test.go`（新）、`internal/agentloop/message_loop.go`、`internal/agentloop/pull_session.go`、`internal/fastpath/gain_staging_ref.go` **零改动**（卡内"只加注释如需"未触发）。

## 端测边界声明

**本卡纯单测域，真栈覆盖=无（行为零变化面）。** 依据卡面"不需要真栈（纯单测域）；无资源占用"：无漂移路径行为零变化由既有 agentloop/fastpath 全量测试零改动全绿背书（含 push/pull 双入口回归）；漂移路径为新增分支，仅可经测试内扰动构造，端到端覆盖见 `TestMessageLoopFastPathDriftFailsRunExplicitlyWithoutPanic`。未占用真实运行栈（PC-RUNTIME-STACK 无登记需求）。

## 上交验收侧裁定项

1. `pull_session.go` 文件域机械扩展（见上，锚点齐全）——超出卡面字面文件域（`message_loop.go`+测试），属签名变更的强制编译适配，无行为逻辑新增。
2. trace 事件 Kind 采用卡面两案中的"新增 fastpath_drift"案（另一案=沿用 final_gate）。
