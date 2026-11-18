# SHELL-WIN-COMBO-1 attempt 02（2026-11-18 18:50 +0800）：395 呈现 PASS；409 分类器因外部前台干扰 FAIL

## 结论
- **395 呈现面 PASS**（首次拿到真实 DOM 证据）：terminal=`ready`、completed 卡=1、步骤数=3、seq=4、progress=unknown、无假进度文本、shell 条件下 native-focus=shell 且无 focus hint、同窗 sentinel 保留；UI 文案「工作台已打开，工程内容已载入」。
- **415 修复在真实链生效**：`status_opened.chain_detail = "workspace_ready: res://vit_dock/scenes/VitDockRoot.tscn"`；godot 日志 `readiness.decided/merged stage=workspace_ready` → `launch_result.post_once stage=workspace_ready`。
- **409 冻结分类器 FAIL（product_failure: Godot foreground/focus observed）**：失败来自 open 之后的 10ms polling 样本，不是首显快照本身。
  - native show_return 快照原样干净：below_reference=true、visible=true、correction_attempted=false；message_interval activate=0/setfocus=0。
  - polling 中 tick 199614687 起（interval.end=199612593，**+2094ms**）连续 113 个样本 foreground/focus=11480（Godot），直到观测结束。
  - 同轮更早的 pre-open polling（tick 199608718/199609218）出现**非本栈持有**的前台 pid 22636/16276（均不在 owned 集合，非 Godot），随后才回到 shell。与 attempt01（734 样本全程前台=shell）对比，本轮存在桌面外部前台切换。
  - 因此本轮 409 结论按冻结规则如实保留 FAIL，但归类为**环境/外部前台干扰（疑似用户或其他窗口操作）**，不作为产品首显失败，也不据此改判 409。无法仅凭本 run 证明该次 DAW 前台由用户触发，只记录证据与归属未定。

## 现场与卫生
- shell exit 0、无强清；结束进程/端口/UDP 全零；recents/fixture/两仓 authority 六份哈希还原一致（见 run 目录）。
- run 工件 5.x MB 保留；本轮未取得可记 PASS 的 409 结论。

## 下一轮（唯一一次）前的主管动作
- harness 调整：395 presentation CDP 读取移到 `[Shell409Observer]::Stop()` **之后**，确保 409 冻结观测窗口不被 CDP/node 工作延长（原 attempt02 中 CDP 在观测窗口内，且外部前台干扰窗口恰在 interval.end+2.1s）。
- 条件：仍为 shell 自然前台；**需要用户配合一次 handsoff 窗口**（约 30–40 秒内不动鼠标/键盘、不点击新出现的 DAW 窗口），完成后再交互。
- 若 handsoff 轮仍出现同类外部前台干扰：停止原样重跑，按 §8 转环境取证/与用户核对桌面，而不是继续消耗真栈。
