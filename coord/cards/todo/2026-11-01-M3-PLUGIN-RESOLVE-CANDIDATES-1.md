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

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
