# ARRLOOP-AUTHBRIDGE-1：agent-loop 能力轮装载授权桥（277 治理缝收口）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[277 段 2 取证](../runs/ARR-SRCLOAD-1/smokerun1/evidence.md)+[裁定 D 先行+B 治本]——能力轮披露面完整但无授权桥：不可伪造选择授权只在装载工作流确认期[plugin_grabber_workflow.go:1044-1065]/UI 浏览器选中/full-access promoted 三路径产生；CAP-BOUNDARY-1 钉过同型拦截为正确行为，本卡补的是编曲常规路径的授权产生面而非绕门）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 273；目标仓库=主仓 agent（harness/workflow 授权面）；P1 / 0.5-1 天 / 依赖=277 段 2 工件在库；L1-L2 / PC；验证腿需真栈一次（可复用 arr_srcload_smoke 驱动器）

## 目标

1. **授权桥实现（方案 B）**：agent-loop 能力轮的装载类动作（rack_add_node 等 PCA 门面）在**确认期**产生不可伪造选择授权——与装载工作流确认期同款治理（Authorize* 盖章语义等价），不新增第三套授权机制；用户在环保持（确认卡→用户确认→盖章）；
2. 治理红线：PCA 门本体零改动（enforceAgentProcessorLoadGate 不动——补的是授权产生面）；零裸写；确认链语义与既有装载工作流一致；
3. 配对单测（能力轮装载确认→盖章→过门；未确认→仍拦）+277 段 2 同型 run 真栈验证（编曲话术常规路径走通：search→rAdd 确认→过门→executed）；
4. §8 跑前成文（≈¥1-3 单 run）。

## 约束

文件域=harness 装载确认面+授权盖章接线+测试+（如需）烟测加参；禁动 PCA 门判据本体；禁动判断权归属域（纯执行层授权产生）；261 的 G5 授权面先例（ensure_full_project_access 服务端路径）可参但不混用（那是 authority 切换非选择授权）。

## 停止条件

实现发现能力轮确认期与装载工作流确认期语义冲突（在案裁定）→取证上交转裁定；验证腿断言失败取证不重跑。

## 领取/回执/验收

## 领取

- 2026-11-07 / 段 1 领取（本轮零栈先行：实现+配对单测；真栈验证腿候栈——栈队列第 3 位：281 烟测→277 段 3a→273 验证）
- origin/main hash：`24a8247b3bb632d2dc1c67d8b5cc6928a61a3d37`（领取时 fetch 重读；任务书基准 09f284d8 已被 281 占栈登记笔 [纯 coord/ 变更] 推进，无代码面差异）
- owner：GLM 主会话 L2-worker 子代理（GLM-5.3 / ZCode / PC）
- 分支：`port/arrloop-authbridge-1`（自 origin/main 新建）；worktree：`D:/Vit_DAW_wt_273impl`
- 领取基线：HEAD=`24a8247b3bb632d2dc1c67d8b5cc6928a61a3d37`；worktree `git status --short`=空；`git diff --stat`=空（新 worktree 自 origin/main 检出全净，无领取前已有 diff）
- 前置核对：卡仍在 todo/ ✓ 派发已确认 ✓；doing 四卡（ARR-SRCLOAD-1[impl 已完在 port/arr-srcload-1 分支]、EDU-LEVEL-RUNTIME-B、CLOSURE-ROUTE-ADAPT-1、D1D3-DIGEST-WIRING-1）文件域与本卡不相交 ✓（goalrunner_chat.go 与 277 触发词面同文件不同段，277 段 1 已验收待合入，合入归主管）

## 回执

### 段 1（实现+配对单测，2026-11-07）——零真栈零 LLM；验证腿候栈（栈队列第 3 位：281 烟测→277 段 3a→273 验证）

- **实现 commit**：`499484fc` @ `port/arrloop-authbridge-1`（已推远端）；diff=4 文件 +613/-0：
  - `agent/internal/harness/harness.go`：新盖章函数 `AuthorizeConfirmedAgentLoopLoad(requestContext, trackID, pluginPath, pluginIdentifier) (context, admittedPath, error)`——与既有 Authorize* 家族并排，盖**同一个** `semanticPluginSelectionAuthorization` 具体值（同一 context key，不可由 JSON/模型伪造）；PCA 事实全部取自晋升目录（复用 `promotedProcessorAuthorizationRecords` 双 store 解析；唯一精确匹配+指纹现势；缺身份/无晋升/歧义/过期指纹四形态 fail-closed）；返回已晋升安装路径供派发钉定。**PCA 门本体（enforceAgentProcessorLoadGate）零改动**。
  - `agent/internal/chat/goalrunner_chat.go`：能力轮执行器桥（`pluginGrabberWorkflowExecutor.RunToolCall` base 派发前）：未确认装载→合成确认卡（needs_confirmation，指名精确 track/path/identifier，SEMANTIC-CARD-COPY-1 合同，不派发；确认卡仅当服务端可授权时出现，否则如实报错不虚构）；已确认装载→盖章+路径钉定后走 base executor→装载门照旧核验；显式 full-access→桥旁路（门自身回落解析，分支序与 `semanticTopologyLoadAuthorization` 一致）；free-state 屏障/typed apply 确认/EQ fallback 拦截等既有面全部前置不动。
  - 测试：`agentloop_confirmed_load_authorize_test.go`（harness 4 件）+ `agentloop_load_authbridge_test.go`（chat 5 件），fixture 全隔离（t.TempDir()+t.Setenv 三 store/库路径定向，零源码树/真实工程写入）。
- **治理等价性说明（对 evidence 选项 B 草案的一处必要精化，请主管过目）**：草案原文「确认期 server 盖 AuthorizeSemanticPluginSelectionLoad」——该裸章（空 BinaryFingerprint/SubjectKey/ProcessorFamily）**过不了装载门的指纹检查**（harness.go:2298 `auth.BinaryFingerprint == ""` → "binary fingerprint changed before load"；既有 topology 单测注释亦如实承认装载腿死于门下）。故桥盖的是**全字段章**（AuthorizeProcessorSelectionLoad 同值形态，数据自晋升目录服务端解析）——这正是装载工作流确认期 receipt 路径（`pendingPlanExecutionContext`/`semanticTopologyLoadAuthorization` 的 found 分支）所盖的同款章。等价性四点：同一 context key+同一具体值类型；同一门判据零改动（精确匹配+指纹+准入现势全在门内重验）；用户确认触发（非 standing authority）；判断权归属不动（纯执行层授权产生）。
- **测试（全部退出码 0）**：`go build ./...`=0；`go test ./internal/chat ./internal/harness -count=1`=ok（chat 125.8s / harness 18.8s）；新增 9 件全过：
  - harness 4：`TestAuthorizeConfirmedAgentLoopLoadPassesGateForPromotedIdentity`（未盖章→门拦"missing non-forgeable exact selection authorization"+内核零派发 → 盖章→**过门** status=ok+内核恰一次且 path/identifier 精确）/`...PinsAdmittedPathForSeparatorVariants`（分隔符变体→钉已晋升路径→过门）/`...FailsClosed`（缺三元组/无晋升/同路径双族歧义/过期指纹四拒）/`...StampBindsExactTrack`（确认后换轨→门拒"exact identifier, path, or track changed"）。
  - chat 5：`TestAgentLoopLoadBridgeParksUnconfirmedLoadForUserConfirmation`（**未确认→仍拦**：needs_confirmation 停泊+mutation_performed=false+内核零派发+确认卡指名身份）/`TestAgentLoopLoadBridgeStampedConfirmedLoadPassesGate`（**确认→盖章→过门→executed**：内核 rack_add_node 恰一次+已晋升路径+精确 identifier/track）/`...FailsClosedWithoutPromotedAdmission`（无准入：未确认与已确认两方向均拒+零派发）/`...LeavesExplicitFullAccessPathUnchanged`（full-access 面零扰动）/`...LeavesNonManagedShapesToExistingPaths`（身份不全→既有校验如实拒；只读工具不经桥）。
- **端测边界声明（AGENTS §5）**：本段零真栈——授权桥仅单测+编译级验证（fixture 内造真实临时二进制+晋升准入过门，非本机真实插件库存）；编曲话术常规路径真栈验证（search→rAdd 确认卡→用户确认→过门→executed）留验证腿。**验证腿候栈**（栈队列第 3 位）。
- **验证腿前置提醒（主管排栈参考）**：真机上音源（VSTi）若无已晋升 PCA 准入，桥会正确 fail-closed（"no current promoted admission"）——真栈腿能否「过门」取决于本机晋升目录是否含目标 instrument 准入；与 277 段 3a（D 路径 full-access）面临同一前置（authorizeFullProjectAccessLoad 同样要求晋升准入）。建议 3a 结果先出，若 3a 也卡在无晋升准入，需主管裁定是否补 instrument 准入晋升（域外）再排 273 验证腿。
- **停止条件**：未触发（能力轮确认期与装载工作流确认期无语义冲突——桥即工作流确认期语义在执行器面的复刻）。
- **工件**：无 run 工件（零真栈）；实现+测试即本段工件（分支 `port/arrloop-authbridge-1`）。

## 验收

- **段 1 轻验（2026-11-07 早窗，主管）：pass（终验待验证腿）**——盖章形态精化**追认**：裸章死于 harness.go:2298 指纹检查（主管亲读门代码验）；桥=服务端晋升目录解析盖全字段章，与既有 G5 full-access 路径同构（差异仅=用户确认触发 vs standing grant），四点等价性成立——执行侧修正卡面技术细节，记良性互动一笔。9 配对单测主管交叉复跑全绿（未确认停泊零派发/确认盖章过门/FailsClosed 四拒/确认后换轨门拒/full-access 零扰动/非托管形态走既有路径）。验证腿候栈第 3 位，**前置=277 段 3a 结果先出**：若 full-access 轮同样卡在无晋升准入，instrument 准入晋升（域外）须先裁定再排 273 验证腿。
