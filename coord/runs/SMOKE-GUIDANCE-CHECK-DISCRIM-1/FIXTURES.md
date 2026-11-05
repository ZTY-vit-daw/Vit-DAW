# SMOKE-GUIDANCE-CHECK-DISCRIM-1 验证工件说明

卡 252（scripts/automation_llm_route_smoke.py guidance 判别面三态化）的离线验证工件。
纯脚本面：无栈、无 LLM、无网络。真栈全仲裁场景复验按卡面约定挂下次全仲裁 run 搭车。

## fixtures/

| 文件 | 构造 | 用途 |
|---|---|---|
| `telemetry_251_g3run1_all_arbitration.jsonl` | **251 归档原样复制**（`coord/runs/M3-GROUP3-DISCRIMINATE-1/g3run1/agent_llm_telemetry.jsonl`，13 行，13/13 `source=ordinary_agent_semantic_entry`，零 message-loop 行） | 态③判据输入：全仲裁形态的真数据 |
| `telemetry_mixed_with_message_loop.jsonl` | 251 归档前 12 行 + 1 行构造 `source=message_loop` 行（同 schema，message_loop.go:555 面的行形态） | fail-closed 对照：文件缺失+非全仲裁必须维持 fail |
| `render_needle_present.txt` | 含双 needle（`automation.set_points` + `automation.get_curve`）的 render 文本 | 态①对照组 |
| `render_needle_absent.txt` | 无 needle 的 render 文本 | 态②真缺陷形态 |

「文件缺失」态由驱动用不存在的路径表达（`agent_prompt_render_never_created.jsonl`，不落盘）。

## 验证工件

| 文件 | 内容 | 退出码 |
|---|---|---|
| `verify_console.txt` | `python -m py_compile`（两文件，OK）+ `python -m unittest test_automation_llm_route_smoke -v`（11/11 OK） | 0 |
| `control_diff_driver.py` | 对照驱动：从领取基线 `76305d87` 提取的**真实改前模块** vs 候选模块，4×4 fixture 矩阵逐组合比对 summary 片段与 stop-condition-2 决策 | 0 |
| `control_diff_report.json` | 16 组合结果：15 same + 1 intended_change（=文件缺失+251 全仲裁归档→`n_a_all_arbitration`）+ 0 regression | — |
| `control_diff_console.txt` | 驱动 console（PASS） | 0 |

## 等价性口径（对照组 diff=0）

- 「与改前 diff=0」的比对对象是**基线提交里的真实模块**（非测试内复刻）：`git show 76305d87:scripts/automation_llm_route_smoke.py`，由 `control_diff_driver.py` 以 importlib 加载。
- 比对面 = 调用点会写入 summary 的片段（键集+键序+值）与 stop-condition-2 触发决策（exit 2 与否）。全 16 组合里与改前**唯一**分歧组合 = 「render 文件应在场而缺席 + telemetry 全 semantic_entry 族」→ 改为 guidance n/a 且不触发 exit 2（即本卡目标修复，report 中 `verdict=intended_change` 恰 1 条）。
- `not_configured`（运行未传 `--prompt-render-path`）组合按调用点门控（`if prompt_render_path is not None`）两侧同样跳过检查，不计 stop。
- `python -m unittest` 内另有 hermetic 版等价断言（测试文件内嵌改前 3 行逻辑，跨 24 组合枚举），双保险。

## 态③落点链（同输入在完整脚本下的行为推演）

251 g3run1 同形态输入（render 文件缺失+13/13 semantic_entry）：
改前 `check_prompt_guidance→False→exit 2（GUIDANCE_MISSING）→ ps1 :8796 throw exit 1`；
改后 `check→n_a_all_arbitration→apply 返回 False→exit_code 保持 0→status=COMPLETED→ps1 走 exit 0 OK 分支`（ps1 无需改动、逐字节未动）。
此推演的函数级依据=单测+对照矩阵断言；端到端确认挂下次真栈全仲裁 run（卡面既定边界）。
