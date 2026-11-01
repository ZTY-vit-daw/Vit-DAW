# M3-PLUGIN-RESOLVE-CANDIDATES-1：plugin 解析歧义 error 附候选清单（235 卡点拆除）+重验门重跑

- 发卡：GLM 主管决策流 / 2026-11-01（依据=235 验收裁定：载荷亲证教学生效+模型四件全对，卡点钉死=确定性工具解析缺口；主管亲核锚点）
- 派发确认：待确认（235 验收处置连带首排，用户转交时视为确认）
- 验收负责人：GLM 主管决策流
- 池序 236；目标仓库=**主仓 agent**；P1 / 0.25–0.5 天 / 依赖=235 已在 main（merge+25c8126c）✓；L1 / PC 执行会话（锚点已定死；重验判读承 235 测量面）

## 标的（235 验收亲核定案，不重考）

- `agent/internal/harness/harness.go:11749-11752`：`resolvePluginID` track 级解析（无 plugin_id/plugin_name 时）在多可见插件下返回裸 error `multiple visible plugins match; select one plugin or specify plugin_id`——**不列候选**。
- fixture 事实（kernel get_project_state 亲证）：每轨双内建插件（track 1010 = 1011 Volume & Pan Plugin + 1012 Level Meter）→ track 级解析**必然** multiple 歧义。
- 恢复参数面**已在**（:11713 `plugin_id`/`:11724` `plugin_name`，按名 EqualFold 解析分支 :11729-11743），但 error 不告知候选 → 模型无从恢复（235 载荷亲证：模型拒猜→观察兜圈→limit_reached→no_route×3）。任何引擎同卡。

## 目标（三面）

1. **披露面修复**：两处歧义 error 附候选清单——track 级裸歧义（:11752）与按名多命中（:11746）。候选行=可见 pluginRef 的 `ID`+`Name`（+track 归属，track 级已隐含），形态=error 文本内机器可读列表（如 `multiple visible plugins match: [1011 Volume & Pan Plugin, 1012 Level Meter]; specify plugin_id or plugin_name`——勘察定案具体措辞，与现有 error 文风一致）。**红线：解析语义零改动**——仍拒绝猜测、仍 fail 返回 error，只改错误披露；`visiblePluginRefs` 零改动。
2. **单测**：双插件 fixture 歧义 error 含两个候选 id+名；单插件/显式 plugin_id/按名唯一命中三路径字节形态零回退；按名歧义分支同型候选断言。
3. **重验腿（组 1 门重判）**：同码（唯一 diff=本卡目标 1/2）同 fixture 同引擎（deepseek-flash）重跑 AutomationLLMRouteSmoke 组 1，载荷捕获+needle 判读继续开。判据同 235：组 1 no_route 消失 + `tool_chain_ok`（写入形态=automation.set_points 包络：确认卡→确认→写入→get_curve 读回）。预期恢复路径：模型从候选清单选 Volume & Pan（222 指引已教 type=volume 语义）→按名或 id 重试→走完 set_points 链。组 1 门过且预算余量时组 2/3 跑满（非验收门）。

## 预期与诚实边界

本卡=确定性卡点拆除，非模型/指引面动作。若候选清单在场（载荷捕获亲证模型实收含候选）模型仍不恢复→**那时才轮到换引擎实验转主案**（指引+工具面双全下的纯模型行为残差）；预期不承诺通过率。

## 停止条件

- 候选披露后出现新确定性断点（如按名重试被拒/参数面不含预期键）→锚点上交，不折第三张小卡连修；
- 组 1 重跑仍三连同型止损且载荷亲证候选在场→**指引+工具面双全证据包上交**（载荷+收敛面+形态分型），转主管换引擎裁定；
- 修复造成既有解析/automation 工具面回归（既有 harness 测试红）→回退两案上交；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/harness/harness.go`（两处 error 措辞+候选组装）+`agent/internal/harness/` 测试文件+`scripts/automation_llm_route_smoke.py`（如判读需微扩：error 恢复路径标记）。越域即停上交。

## 验收标准

- 单测：候选断言三件+零回退断言；`go test ./internal/harness/` 全绿。
- 真栈腿（§5 门槛）：ps1 体系 AutomationLLMRouteSmoke 组 1 exit 0+载荷捕获工件在 run 目录；栈登记/释放全流程。
- 回执分列**链路进度**（候选披露在场）与**最终验收**（组 1 三件：no_route/set_points 形态/候选恢复亲证）；§8 计费披露。

## 领取

- 2026-11-01 下午 +0800 / origin/main=0af2d972afaf666b2a8aa16e5991c70b975dfbd8 / GLM-5.3 L1 PC 执行会话（ZCode sess_402fb247，用户转交口令领取）
- impl worktree=D:/Vit_DAW_wt_236impl（分支 port/m3-plugin-resolve-candidates-1）；协调 worktree=D:/Vit_DAW_wt_236coord（分支 coord/236-claim）
- 领取基线三件（impl worktree）：HEAD=0af2d972afaf666b2a8aa16e5991c70b975dfbd8；status --short=空；diff --stat=空
- 领取前只读核对：卡在 todo 且派发确认（用户转交）；依赖 235 已在 main（0af2d972 即 235 dispatch commit）；doing 唯一卡 GEN-SUNO-WEB-DESIGN-1（Mac 勘察卡，[等待用户]）文件域零冲突；PC 栈记录无"占用中"+四端口 5555/5556/7878/8787 监听=0+VitApp/VitAgent/vsphub/Godot 进程=0
- 栈登记：占用 c32f5e70（2026-11-01 18:31，登记前+启动前双复核+push 后 ≤60s 即时复核进程/端口全零）→ 释放（18:31–18:46，TEARDOWN_ALL_ZERO：VitAgent 26260/VitApp 38452 owner 手动停，TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0）

## 回执

- **实现 commit**：7781678a @ port/m3-plugin-resolve-candidates-1（impl：harness.go 两处歧义 error 附 formatPluginCandidates 候选清单【ID+Name 方括号机器可读】+恢复参数面 plugin_id/plugin_name 披露；红线守住=解析语义零改动两处仍 fail 拒猜+visiblePluginRefs 零改动）；工件 commit 5124acbf 同分支（alignrun1 全量+EVIDENCE.md，clip.wav gitignore 排除同 235 先例）
- **单测**：新增 TestResolvePluginIDAmbiguityErrorsDiscloseCandidates（候选断言三件：track 级双内建 fixture 同 235 卡点形态 `[1011 Volume & Pan Plugin, 1012 Level Meter]`+恢复参数面在场/按名歧义同型候选/两分支仍 fail-closed）+ TestResolvePluginIDUnambiguousPathsUnchanged（零回退三路径：显式 plugin_id 直通 1011/按名 EqualFold 唯一命中 1012/单插件直接解析）；`go test ./internal/harness/ -count=1` 全绿；全量 `go test ./... -count=1` 全绿（exit 0）
- **链路进度**（达成面）：①披露面修复生效且被模型消费——error 全文 `multiple visible plugins match: [1011 Volume & Pan Plugin, 1012 Level Meter]; specify plugin_id or plugin_name` 载荷实收亲证（r2 内 4+ 处）；②候选恢复亲证（组1 三件之一 ✓）——模型从候选清单选 Volume & Pan 调 `{"plugin_id":"1011"}`（payload row32/33/35/36），回传 status=ok，卡面预期恢复路径完整走通；③教学段 needle 延续在场 81/85 行
- **最终验收**（组1 门三件，如实分列）：①no_route 消失 **✗ 未达**——`[no_route,no_route,no_route]` successes=0 三连同型止损（组2/3 skipped）；②set_points 包络形态 **✗ 未达**——三轮 automation.set_points 零调用；③候选恢复亲证 **✓ 达成**。ps1 退出码如实分列：AutomationLLMRouteSmoke 场景腿 `[DONE] COMPLETED exit=0`，**ps1 整体 exit 1**=后续 Chat route smoke 腿 HTTP 超时（dev_agent_smoke.ps1:491，与组1 判读无关腿，环境/时序类非本卡功能面——本卡改动仅 error 文本；§5 字面门槛未满足交主管裁定）
- **停止条件第一条触发→锚点上交（不折第三张小卡连修）**：新确定性断点=恢复调用成功但参数面零披露——ok 回执上下文快照 `"status":"ok","summary":{}`，row37 全 request 核 parameter_id/parameters/value_range/min/max 全 0 计数（仅 param_id×4/gain_db×2 来自 system 规则文本）；模型拿不到 param_id/取值范围→按教学段纪律拒写 set_points（不写未解析参数=拒猜纪律正确）→兜圈 no_route。**锚点=harness.go:12879 publicPluginParametersResult：参数行仅 include_parameters/include_full_parameters/include_parameter_snapshot 为 true 时组装（:12910 门控），教学段未教该键+快照 summary 空回执→参数面整个不在模型可见面**。恢复链（歧义→候选→plugin_id 重试→ok）已通，确定性缺口后移一层：error 披露→结果披露；换引擎对此断点无效（任何引擎拿到 ok+summary{} 都无数据可写，同 235 对裸 error 论证同构）
- **执行侧判断（供裁定，不预设）**：本卡目标 1/2 完全达成（披露面修复+单测全绿+候选恢复亲证）；组1 门未过的责任面已非本卡标的（候选在场且被使用，失败在结果披露面缺口）——处置归主管：参数面披露卡（include_parameters 默认或快照摘要披露）或换引擎实验照跑
- **判读工具说明**：卡面预留的 py 微扩（error 恢复路径标记）未做——payload capture 直读已闭合判读证据链（候选实收+恢复链+断点形态三面），不为微扩而微扩；如主管需要 summary 内自动化标记可开修订卡
- **§8 计费披露**：deepseek-flash 95 计费调用，1,963,632 in（含 cache）/122,019 out（0 env_interrupt 0 canceled）；三轮形态=r1 CCB 兜圈 2exchanges/r2 get_parameters 8 调用（撞歧义→恢复→ok→空回执→兜圈）/r3 CCB 兜圈
- **端测边界声明（§5 渲染面/旅程门槛）**：本卡改动为 agent 内 error 披露文本，不涉 webui 渲染面与 demo 用户旅程；真栈烟测腿覆盖 agent HTTP/ZMQ 接口面（AutomationLLMRouteSmoke）

## 验收

- （主管回填）
