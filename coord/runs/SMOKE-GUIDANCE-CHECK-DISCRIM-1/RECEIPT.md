# RECEIPT — SMOKE-GUIDANCE-CHECK-DISCRIM-1（卡 252）

- 执行：GLM-5.3 PC L1 执行会话（ZCode）/ worktree `D:/Vit_DAW_wt_252impl` / 分支 `port/smoke-guidance-check-discrim-1`
- 领取提交：`8816da59`（基线 `76305d87`；领取窗口遇 250 卡先推，按 PROTOCOL §2.1.3 fetch 重读远端——本卡仍无 owner、文件域无冲突——重放后推送成功）
- 纯脚本面：无栈、无 LLM、无网络；不占 PC-RUNTIME-STACK。

## 一、命令与退出码

| 命令 | 退出码 |
|---|---|
| `python -m py_compile scripts/automation_llm_route_smoke.py scripts/test_automation_llm_route_smoke.py` | 0 |
| `cd scripts && python -m unittest test_automation_llm_route_smoke -v` | 0（11/11 OK） |
| `python control_diff_driver.py --old-module <git show 76305d87 提取> --new-module <候选> --output control_diff_report.json` | 0（16 组合：15 same / 1 intended_change / 0 regression） |

console 原文：`verify_console.txt`、`control_diff_console.txt`；fixture 构造说明见 `FIXTURES.md`。

## 二、改动面（scripts/automation_llm_route_smoke.py，+112/−10；新增 scripts/test_automation_llm_route_smoke.py）

1. `classify_telemetry_sources`（:1309）——态③判据：telemetry 行按 `source` 归族（`message_loop*`=message-loop 面、含 `semantic_entry`=仲裁面、其余含缺 source=other）；`all_arbitration = rows>0 且全部为 semantic_entry 族`。unknown/无 source 行一律计入 other（不构成全仲裁证据，fail-closed）。
2. `check_prompt_guidance`（:1357）——三态：①文件在场+双 needle→`ok/rendered=True`；②文件在场缺 needle→`needle_missing/rendered=False`（维持真缺陷 fail）；③文件缺失+telemetry 全仲裁→`n_a_all_arbitration`（附判据证据）；文件缺失但**无**全仲裁正证据（含 telemetry 缺失/空/mixed/unknown）→`missing/rendered=False`（与改前逐字节同判）；`--prompt-render-path` 未传（None）→不进判别（调用点本就整段跳过，函数层与改前同判）。OSError 读失败→`unreadable/False` 同改前。
3. `apply_prompt_guidance_summary`（:1392）——非③态 summary 片段与 stop-condition-2 决策与改前完全一致（布尔字段、键集不变）；③态写 `prompt_guidance_rendered="n/a (all-arbitration)"`+`prompt_guidance_n_a`（reason+telemetry 计数证据），**不触发 exit 2**。
4. 调用点（:1669-1670）——两行接线，`exit_code=2` 仅在 apply 返回 True（即改前的失败族）时置位。
5. **dev_agent_smoke.ps1 零改动**：态③下 py 退出码 2→0，ps1 既有 exit-0 分支（"executed to completion"）即正确出口；exit 2/3/else 分支字节不动，无需同步。

## 三、等价性证据（红线：非全仲裁路径行为字节不动）

- 对照驱动加载**真实改前模块**（`git show 76305d87:scripts/automation_llm_route_smoke.py`，非复刻），4 render 态 × 4 telemetry 态 = 16 组合，比对「调用点将写入的 summary 片段（键集+值）」与「stop-condition-2 决策」：15 组合 `same`，唯一分歧=目标修复组合（render 缺失+251 归档 13/13 全仲裁→n/a），0 `REGRESSION`。
- unittest 另含 hermetic 双保险：内嵌改前 3 行逻辑，跨 24 种函数级组合枚举断言等价，且钉死「n/a 仅能由 唯一输入组合 达成」。
- 态③端到端推演（同 251 g3run1 输入）：改前 False→exit 2（GUIDANCE_MISSING）→ps1 throw exit 1；改后 n/a→exit 0→status=COMPLETED→ps1 OK 分支。函数级已断言；**端到端真栈确认挂下次全仲裁场景 run 搭车（卡面既定验证边界，本卡不重跑真栈）**。

## 四、诚实边界

- 本卡验证=脚本面单测+fixture 矩阵；未跑真栈（卡面明示不重跑，AGENTS §5 真栈门槛由搭车 run 承接）。
- telemetry 判据锚定 `source` 字段字面值（`message_loop*` 前缀/含 `semantic_entry`）；未来新增 source 族会落入 other→fail-closed（宁可误报缺陷不放过装配回归），如需扩族属后续卡。
- 251 归档 telemetry 原样复制为 fixture（13 行全 semantic_entry）；mixed fixture 中 message_loop 行为同 schema 构造行（251 归档零 message-loop 行，无从复制）。
- 执行偏差一处：control_diff_console 首跑因 shell cd 后重定向相对路径漂移落 worktree 根，未提交前即移入 run 目录（git status 核对无残留）。
- done=执行侧自验完成，最终验收归 GLM 主管决策流。

## 五、工件坐标

- 本目录（`coord/runs/SMOKE-GUIDANCE-CHECK-DISCRIM-1/`，`port/smoke-guidance-check-discrim-1` 分支）：`FIXTURES.md`、`verify_console.txt`、`control_diff_driver.py`、`control_diff_report.json`、`control_diff_console.txt`、`fixtures/`（251 归档 telemetry 原样件+派生件+render 对照件）。
- 改前模块提取件在 `$TEMP/vit252_ctrl/`（可再生：`git show 76305d87:scripts/automation_llm_route_smoke.py`），不入库。
