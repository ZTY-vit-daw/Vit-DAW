# M3-C4-AUTO-LLM-ROUTE-VERIFY-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-29-M3-C4-AUTO-LLM-ROUTE-VERIFY-1.md（池序 220）
- 执行：ZCode GLM-5.3 PC 执行会话（220 卡会话）；worktree D:/Vit_DAW_wt_m3c4lr1 @ port/m3-c4-auto-llm-route-verify-1
- 领取基线：origin/main=61a25d33（领取提交 dc64a4ab）；占用登记 4dd7b6b1（2026-10-30 19:0x）
- 一句话结论：**停止条件 2 命中——217 的 automation 路由指引在变体话术的 LLM 装配分支（message_loop_neutral_family_selection）上未插入，模型从未收到指引；组 1 三连 no_route 止损成立，组 2/3 剩余轮次未消耗；属部署缺陷锚点上交，非模型路由能力结论。**

## 一、执行形态与轮次判定

- 命令（两 run 同形，worktree 根执行）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_m3c4lr1/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir <run 目录>
```

- **run 1（llmroute_20261030_1，19:05）**：组 1 三轮判 confirm_broken 触发止损——**此判定后被推翻**：分类器 blob 启发式把含 `needs_confirmation` 字段名的回执误判为确认卡（脚本面缺陷，迭代轮留证不计产品结论）。脚本修复提交见 port 分支第二笔。
- **run 2（llmroute_20261030_2，19:11）**：组 1 三轮稳定 **no_route** 止损（v2 分类器：工具调用证据=全程仅 `ccb.observation_request`/`ccb.observation_catalog`，零次 automation.* 调用）；随后脚本 guidance 检查判 `GUIDANCE_MISSING` exit 2——经 render 逐行核对为**真警报**（见 §二）。
- 被测栈：kernel=主检出 VitApp.exe（sha256 c1e32dfe5338a9e1… 217/218 同源只读复用，运行前后零漂移亲核）；agent=worktree 自建（main 61a25d33 同源，本卡 agent 面零改动）；无 UI；vsphub 不需要。
- setup：隔离工程（VIT_PROJECT_XML=run 目录 git 干净副本）+ 双裸音轨（无外部插件——卡面组 2 前提）+轨级 Volume&Pan（plugin 1011）解析+基线曲线空断言，两 run 各自成立。
- **LLM 轮次真实计费**（deepseek-flash=用户 ~/.vit/config.json 真实引擎配置）：run 1=15 调用（156,415 in / 10,415 out）；run 2=10 调用（105,356 in / 7,627 out）；**合计 25 调用 / 261,771 input / 18,042 output tokens**（含 run 1 迭代轮）。逐调用记录在 `logs/agent_llm_telemetry.jsonl`（两 run 各自）。

## 二、核心发现：装配分支间歇缺陷（停止条件 2，锚点上交）

prompt render 逐行证据（`logs/agent_prompt_render.jsonl`，两 run 同一二进制同话术形态）：

| 轮次消息 | 装配 tag | system 长度 | 含 automation 指引 |
|---|---|---|---|
| 两 run 全部变体话术首 turn（10/10） | `message_loop_neutral_family_selection` | 14,772 B | **否** |
| run 1 r1 的「确认写入」turn（唯一例外） | `message_loop_system` | 52,335 B | **是** |

- **机制**：变体话术（词表未命中）进 message loop 后走 neutral family selection 装配路径，该路径的 system prompt **不含 217 的 automation 路由指引行**（embed 双插入只覆盖完整 system 装配）；「确认写入」类确认话术才触发 message_loop_system 完整装配。**间歇性=消息类型路由，非随机**。
- **归因链**：模型在全部 10 个变体话术 turn 上从未收到指引 → 工具面只出现 ccb.observation_request/catalog（观察探索）→ 三连 no_route。**指引在场的唯一 turn**（run 1 r1 被动「确认写入」语境）模型行为=如实答「当前没有待确认的写入操作」+观察探索，未虚写未绕主路——1 个弱样本，不足以评估指引效果。
- 卡面停止条件 2 原文命中：「chat 面根本未装配 system prompt embed（217 指引未生效形态）→锚点上交，属部署缺陷非模型行为」。精确形态=**embed 未插入 neutral_family_selection 分支**。
- 附带行为面留档（不折结论）：run 1 r2 跟进轮模型有 `track.add_audio`+`track.delete` 探索性副作用（免确认工具族直执，加轨后即删）；run 2 r2/r3 的「继续」跟进被 continuation 队列吞掉（「任务已在自动续跑队列中；这条消息不会创建新任务或重复执行」——零 LLM 调用的确定性回执）。

## 三、验证矩阵执行状态（卡 §8 口径如实记录）

| 组 | 话术 | LLM 样本 | 判定 | 组级 |
|---|---|---|---|---|
| 1 | 这段让它越来越突出一点 | 3（run2 正式）+3（run1 迭代留证） | no_route ×3（同型三连，止损） | **fail（止损）** |
| 2 | 帮我把空间铺开 | 0 | 未执行（组 1 止损+停止条件 2，剩余轮次不消耗） | not run |
| 3 | 30 秒之后动起来 | 0 | 未执行（同上） | not run |

- 卡面标的 1（指引→工具选择）：**无法在指引缺席的装配路径上评估**——部署缺陷前置。
- 卡面标的 2（set_points 确认链）：未到达（模型从未调用 set_points，确认链无从触发）。
- 卡面标的 3（choreo pending 互不干扰）：未到达（pending 腿嵌于每组末轮窗口，组 1 在第 3 轮止损）。
- 失败分类逐轮记录：run 2 三轮均 no_route（detail 与完整回执原文在 `rounds/g1_r*.json`）；无 wrong_tool/confirm_broken/write_fail 实证（run 1 的 confirm_broken 三连为分类器误判已推翻，留证不计数）。

## 四、锚点与折卡建议（上交决策侧）

1. **主锚点（部署缺陷）**：`message_loop_neutral_family_selection` 装配分支的 system prompt 不含 automation 路由指引——render 证据 `llmroute_20261030_{1,2}/logs/agent_prompt_render.jsonl` 逐行可回指；代码面入口=`agent/internal/agentloop/` message loop 的 family selection 装配路径与 217 embed 的插入点（server.go:6309 指引行+embed 双插入只覆盖完整装配）。修复属生产代码，本卡零生产改动纪律不修，**建议折修复卡**。
2. 次生观察面缺陷（同锚点族，207「VSP 紧凑快照剥轨行 plugins」同族）：模型经观察面看到的工程=「Track 1、Track 2 无插件」，与内核真值（轨级 Volume&Pan plugin 1011 在场）不符——即便指引插入 neutral 分支，观察面不披露轨级插件仍会阻碍模型解析 plugin_id。建议修复卡一并评估。
3. turn 预算面：首 turn 中性阶段+观察探索即耗尽 turn 预算（`limit=max_turns`，MaxTurns 默认 8/slice）截停为 waiting_continue——模型到达 automation 工具链的路径长度超出预算。修复卡评估时一并考虑。
4. 修复后的重验卡建议直接复用本卡断言体（`scripts/automation_llm_route_smoke.py`，v2 分类器+工具对齐+边界真值对照已就位）。

## 五、迭代留痕与事故披露

1. run 1 三轮 confirm_broken=分类器误判（blob `"needs_confirmation"` 子串过宽）——脚本修复（v2：显式确认面精确匹配+limit_reached「继续」跟进+agent log 工具调用增量对齐+边界真值标注）后 run 2 定案。迭代轮完整留证不删。
2. 等待用户手测栈：18:10:43-19:03:27 用户手测栈在场（start_page 途径四进程），本腿轮询等待其自行释放后登记（§2.2 错峰兑现，非事故）。
3. 环境注记（非本腿所为，如实留档）：主检出 `VitApp/Workspace/default_project.xml` mtime=18:11:12（用户手测期内核写入）；`VitApp/Workspace/Settings.xml` 在本腿收尾核对时已不存在（领取基线时为 M 态）——两者均为用户手测期遗留，本腿全程 VIT_PROJECT_XML 隔离副本（19:05/19:11 mtime 亲证承接写入），主检出工程文件本腿零触碰。
4. run 2 ps1 exit≠0=脚本按停止条件 2 设计的 throw（GUIDANCE_MISSING 上交形态），非腿崩溃；summary/rounds/logs 工件完整。

## 六、成本与端测边界声明

- **覆盖**=agent /agent/chat HTTP 面真实 LLM 轮次（用户引擎 deepseek-flash 真实计费 25 调用）×prompt render 逐 turn 装配取证×agent log 工具调用流×内核 ZMQ setup/曲线面基线；两 run 独立栈。
- **不覆盖**=指引在场且装配正确前提下的模型路由行为（部署缺陷前置未达）；set_points 确认链（未触发）；choreo pending 互不干扰（未达）；组 2/3 话术（轮次未消耗）；webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）。
- **本卡零生产改动**（默认形态）：全部改动=scripts/ 两文件（断言体新增+ps1 四处扩展+迭代修复），生产代码零触碰；暴露缺陷按 §四折卡。
