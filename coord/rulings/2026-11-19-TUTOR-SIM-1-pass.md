# 裁定：TUTOR-SIM-1 三步导师循环 PASS

- 日期：2026-11-19 22:31 +0800
- 卡：435 TUTOR-SIM-1
- 裁定：**服务端导师循环 + 真实 flash 三步 smoke PASS**；UI 自动开场留给 TUTOR-SIM-2。
- 代码：97708aee → main 16d3b47f；agent 二进制 1739E82C…；脚本 scripts/run_tutor_sim_smoke.ps1；实验 YAML coord/experiments/tutor-sim-1/tutor_sim.yaml。
- 真栈 smoke：coord/runs/TUTOR-SIM-1/smoke_20261119_2235（status=passed，exit 0）。

## 验收事实

- start 无命令词：POST /agent/tutor/start 返回第 1/3 步，LLM 开场“欢迎来到实验，现在是第 1/3 步。请计算 3 + 4，并只输入数字…”。
- 错输入 0：passed=false，step_index=1，stop=tutor_sim_retry，不推进。
- 7 → 第 2 步；42 → 第 3 步；123 → completed；LLM 回复分别确认通过并给下一步，未泄露答案。
- 完成后追加输入：不推进、不再调 LLM、stop=tutor_sim_already_completed；未落普通 agent loop（无 goal_id）。
- 所有可见 reply 无「开始乐局/写好了/乐局状态/乐局事件」命令词；answer_leak_warnings 为空。
- 判定是服务端 trim+exact；expect 不进 LLM prompt；LLM 只负责开场/引导/反馈，不能自宣通过。
- 真实模型：flash（config.Load，默认设置）；单次正式轮；无重跑。
- teardown：Vit 系进程/TCP 六口/UDP 4445 全零；recents/Settings/default_project 还原一致。

## 覆盖边界

- 会话仅内存；完成后不持久化。
- UI 自动开场/工程打开触发未做（TUTOR-SIM-2）；本轮用 headless HTTP 验证。
- 实验 YAML 是独立 vit.tutor_sim.v0，不是生产 design.v1 契约。
- LLM 措辞概率性；smoke 硬断言状态机与零命令词，回复仅作人设/引导抽查。
- 生产乐局的 YAML 节点教学/材料/控件/检查尚未接入；下一步 TUTOR-SIM-2 先做 UI 自动开场，再接状态/问题控件与生产 YAML 映射。

## 下一步

- TUTOR-SIM-2：工程打开自动开场 + UI 展示（不按钮、不命令词）；随后状态/问题控件、生产 YAML 节点 brief、材料/音频。
- 433 A2a work 反馈作为导师工具候选；434 W02-A1-HANDTEST-V0 产品 v0 未过，superseded。
