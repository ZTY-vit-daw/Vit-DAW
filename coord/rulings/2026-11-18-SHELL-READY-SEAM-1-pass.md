# SHELL-READY-SEAM-1 验收：实现面 PASS（零栈），真实装配待组合真栈

- 卡：`2026-11-18-SHELL-READY-SEAM-1`（池序 411）；主管=lead（PC 决策流）。
- 执行：`lead/ready411`（DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-ready-seam-411`，commit `4784f9b5861d6788e6bb83c81a6fa1d93fc05734`（父 bc8daf6）；已合入前端 main `cf33195bb3c3a86d3c77c8338efba014d8097f6b` 并推送。
- 来源：402 `b1f9406` 的 readiness coordinator 与 dock hooks + 409 `bc8daf6` 的 window entry/唯一回执。

## 主管独立复核

| 项 | 结果 |
|---|---|
| 文件域 | `app/shell/vitshell_launch_readiness.gd`(新)、start_page.gd、vitshell_window_entry.gd、vit_dock_root.gd + tools 探针/fixtures + 两个 `.uid` 伴随文件 |
| dock hooks 一致性 | `vit_dock_root.gd` blob `c2f82c34...` 与 402 `b1f9406` 版本逐字节一致 |
| hidden-boot 接线 | start_page 仅在 `VIT_SHELL_REQUIRE_HIDDEN_BOOT=1` 分支创建 readiness（post_receipt=false）并在 arm 前 begin；legacy 409 路径零改动；expect_scene 在 change_scene 前绑定 |
| 唯一回执 | window_entry `_completing/_receipt_posted` 双守卫、单 POST 点；readiness `post_receipt=false` 时抑制自身 POST（探针断言 receipt_attempts=0、suppressed=1） |
| import | 主管亲跑 `--headless --import --check-only` exit 0；执行侧 `--headless --import` exit 0 |
| unit runner | 主管亲跑 `tools/run_vitshell_readiness_unit.ps1` exit 0；semantic 21 checks / seam 28 checks（四情景各恰 1 次 loopback POST，总 4）/ assembly 8/8 |
| 四情景 | ready `ok=true stage=workspace_ready`；快照失败 `ok=false stage=snapshot_failed`；超时 `ok=false stage=workspace_timeout`；legacy `ok=true stage=scene_accepted` |

## 覆盖与边界

- 已证：语义/装配/回执委托与唯一 POST；production scene 的 repository/hub 实件与 `_apply_kernel_project_reply_and_emit` 真 apply 后 ready（assembly 腿，注入空内核回复）。
- 未证（414 负责）：start_page hidden-boot 分支在真实 VitShell 启动下的端到端执行；生产 dock 首快照路径与 readiness hook 的实际命中；真实 HTTP 到 VitShell。若生产快照路径不触发 hook，组合真栈会以 `workspace_timeout`（ok=false）暴露，届时按取证处置，不静默降级。
- `.uid` 伴随文件与 tools 探针属必要交付，主管接受。
- 状态=实现面 PASS，待 414 组合真栈最终裁决。
