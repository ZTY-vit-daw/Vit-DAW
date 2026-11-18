# SHELL-WIN-COMBO-1 attempt 01（2026-11-18 18:44 +0800）：product_failure — 集成缺陷

## 结论
- 409 原生面 PASS：initial_below_reference=true、correction_attempted=false、message_interval activate/setfocus=0/0、734 样本、classifier exit 0。
- 395 呈现面 FAIL：terminal=`unconfirmed`、limited=1、steps=0、seq=4；无假进度、无焦点误报、同窗 sentinel 保留。
- 分类=**集成缺陷（我方 410/411 接缝）**，非 409 回归、非环境。

## 根因（I/O 证据）
- Godot：`[VitShellLaunch] readiness.decided stage=workspace_ready` → `readiness.merged stage=workspace_ready` → `[ShellZ411] launch_result.post_once stage=workspace_ready`（godot-captured.raw.log L106-111）。
- 宿主：`status_opened.json.chain_detail = "scene_accepted: res://vit_dock/scenes/VitDockRoot.tscn"` → 410 的 `launcher_bridge.cpp` 用 `JsonStringValue(result_body,"stage")` 取到的是 payload 里 **先出现的嵌套 observations 样本字段** `"stage":"show_return"`，顶层 `"stage":"workspace_ready"`（位于 observations 之后）被遮蔽。→ SetChainUi stage 错误 → 395 收到 opened 无 stage → terminal unconfirmed。
- 该缺陷在零栈单测中不可见（测试 payload 无嵌套同名 key）。

## 现场与卫生
- stack 清洁：进程 0、Listen/Bound 0、UDP4445 0；shell exit 0、无强清；recents/fixture/两份 authority 六份哈希还原一致；run 工件 5.17 MB 保留。
- 未完成：395 呈现 PASS 与 391 组合结论。停止重跑，转修复卡 415。

## 修复方向
- 宿主解析改为**顶层 key** 语义（深度/字符串状态感知），stage 只认顶层；不可把嵌套同名字段当 stage。
- 保留 409/410 其余契约；新增嵌套遮蔽反例单测。
