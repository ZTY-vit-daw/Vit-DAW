# SHELL-WIN-COMBO-1 验收：PASS（391 组合真栈，单条件单次成功路径）

- 卡：`2026-11-18-SHELL-WIN-COMBO-1`（池序 414）；主管=lead（PC 决策流亲跑）。
- 前置：410 producer、411 readiness、412 呈现、413 入口产物、415 stage 解析修复——均已验收合入；前端 main `8c3801f`、主仓协调 `8a4e7691`。
- 运行：`run_20261118_190253`（attempt03，用户 hands-off），`-Condition shell -CardId SHELL-WIN-COMBO-1 -PresentationChecks -TimeoutSec 180 -NaturalConditionWaitSec 60`；单次、零 LLM。

## 结果

| 面 | 结果 |
|---|---|
| 脚本/宿主退出 | script 0 / shell exit 0 |
| 409 原生分类器 | **PASS**（classifier exit 0）：initial_below_reference=true、correction_attempted=false、message_interval activate/setfocus=0/0、sample_count=707 |
| 395 呈现（真实 WebView2 DOM） | **PASS**：terminal=`ready`、completed 卡=1、步骤=3、seq=4、progress=unknown、无假进度文本、shell 条件下 native-focus=shell 且无 focus hint、同窗 sentinel 保留；文案「工作台已打开，工程内容已载入」 |
| 宿主状态 | `chain_state=opened`、`chain_detail="workspace_ready: res://vit_dock/scenes/VitDockRoot.tscn"`；`window_state.foreground=shell`、`daw_hwnd` 非空 |
| 卫生 | shell 正常退出、无强清；进程/端口/UDP 全零；recents/fixture/两仓 Settings/default_project 六份哈希还原一致；产品 LLM=0 |

- 失败轮保留：[attempt01](runs/SHELL-WIN-COMBO-1/ATTEMPT-01.md)（集成缺陷→415）、[attempt02](runs/SHELL-WIN-COMBO-1/ATTEMPT-02.md)（395 呈现 PASS；409 冻结分类器因外部前台干扰 FAIL，未改判）。本次只证明该条件下的**一条成功路径**。

## 覆盖边界

- 单次 shell 自然前台条件；external 自然前台条件由 409 单独验收（本次未重复）。采样为 10ms 轮询与自有 subclass 区间，不覆盖每个渲染帧或绝对零激活；不构成稳定性/成功比例结论。
- 手测入口为官方 Godot 4.6.1 editor/project 直启开发入口；导出模板与 `template_release` DLL 未刷新，不在本次范围。
- 396 GODOT-RETRY-LAUNCHER-1（重试导航微卡，1 行 gd）未并入本次运行；其真栈腿仍未收口，不属 391 判据。
- 395 的 395-B directive 占位文案（launcherData.ts，域外）未合入；不影响本次装载反馈/焦点提示主链。

## 后续

- 391 SHELL-WIN-BEHAVIOR-1 解锁要件（395 兼容 + 组合真栈 + 入口刷新）已满足 → 放行用户手测 v7；手测结论回填后再出 391 ruling。
- 成功工件、原始日志与恢复证明见 `coord/runs/SHELL-WIN-COMBO-1/run_20261118_190253/`；本裁定不替代用户手测。
