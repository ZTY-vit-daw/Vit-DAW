# SHELL-WIN-STAGE-PARSE-FIX-1 验收：PASS（零栈 + 真实链生效）

- 卡：`2026-11-18-SHELL-WIN-STAGE-PARSE-FIX-1`（池序 415）；主管=lead。
- 执行：`lead/producer410`（DeepSeek-V4.1-Flash，续用 410 代理）；分支 `codex/shell-stage-fix-415`，commit `8c2c2b4413d74168f68d59e5a8d8938c6f4ba8db`（父 cf33195）；已合入前端 main `8c3801f41924cb9d0ffccc8a0872372401e3319c` 并推送。
- 修复：新增 `JsonTopLevelStringValue`（深度/字符串/转义感知，仅 depth=1 成员），launch-result 的 `stage` 改用它读取；scene/detail/ok 与 409/410/411 其他契约不变。

## 复核

| 项 | 结果 |
|---|---|
| 文件域 | 仅 launcher_bridge.cpp/.h + launcher_presentation_test.cpp（3 文件，+226/−2） |
| 主管复跑 | ctest 3/3 exit 0；直跑 launcher_presentation_test PASS（含嵌套遮蔽→workspace_ready、仅嵌套→空、转义/截断反例共 16 断言） |
| 真实链证据（414 attempt02） | `status_opened.chain_detail="workspace_ready: ..."`；395 presentation PASS（terminal=ready/completed/steps=3）——确认 410 顶层解析缺陷已消除 |
| 其余 | 未改 gd/extension/webui/主仓；零真栈（415 自身） |

## 边界

本卡只修解析缺陷；414 组合卡的成功仍受 409 冻结分类器约束（attempt02 因外部前台干扰 FAIL，环境归属未定）。415 判 PASS 依据=缺陷修复 + 零栈测试 + 真实链字段生效；不等于 414/391 已完成。
