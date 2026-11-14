# REC-ISOLATION-CONFIGDIR-1：装置内核用户配置目录隔离（VIT_DAW_USER_CONFIG_DIR）

- 发卡：GLM 主管决策流 / 2026-11-14 晚窗（依据=[RECENTS-HYGIENE-1 终验裁定](../../rulings/2026-11-14-RECENTS-HYGIENE-1-pass.md) a 案采纳）
- 派发确认：已确认（主管发卡；**P3——栈空闲窗口执行，不占展演前主线**）
- 验收负责人：GLM 主管决策流
- 池序 351；P3 / 1 天 / 依赖=零（真栈腿候 PC 栈空闲，与 343 错峰）；L1-L2 / PC

## 背景

342 取证钉死：内核 open/save/save_as 成功后无条件 `prependRecentProject`（`VitApp/Source/Service/VitHeadlessService.cpp:1207/1353/1475`）写用户级 `%APPDATA%\Vit-DAW\global_project_config.json`——装置与用户旅程同权，纯纪律兜底（AGENTS §10 新条款）有崩溃窗口与竞态盲区。结构性根除=装置走独立用户配置目录。

## 目标

1. 内核加 env 覆盖入口：`VIT_DAW_USER_CONFIG_DIR`，`VitGlobalProjectConfig::getConfigFile()` 优先读该 env（含 projects 根的派生路径如受影响一并处理）；**fail-open**——未设时行为与现完全一致（老二进制语义不变）。
2. 装置 launcher（`scripts/dev_agent_smoke.ps1` 及同族装置脚本）统一注入 per-run 临时目录（与 VIT309_*/VIT301_GUARD 同款机制）；teardown 连目录清理。
3. 隔离面烟测断言：装置 run 后**用户真实 `global_project_config.json` 哈希零漂移**（不止净差零——是根本不被打开写入）。
4. 339 的 recents 种子机制改为种隔离文件（探针语义不变，自种自验更干净）；未发现其他断言依赖用户真实 recents（342 取证在案，若实现中发现反例→停止上报）。

## 文件域

`VitApp/Source/Core/VitGlobalProjectConfig.cpp/.h`（env 读取）+ `VitApp/Source/Service/VitHeadlessService.cpp` 仅在路径派生需跟随时的最小改动 + `scripts/` 装置 launcher 注入 + 测试/烟测断言 + `coord/runs/REC-ISOLATION-CONFIGDIR-1/`。禁动：prependRecentProject 写入逻辑本体（不改变用户旅程行为）、agent/WebUI。

## 验收标准

- C++ 编译 + 既有测试不回退；新增单测覆盖 env 设/未设两态。
- 真栈烟测（装置 run）：exit 0 + 用户真实 global_project_config.json 哈希前后零漂移亲证 + 隔离目录内 recents 正常工作（种子→读出链路活）。
- AGENTS §10 b 条款保留声明（兜底不撤销）。

## 停止条件

- env 覆盖导致内核启动/加载用户工程回归 → 停，取证上报。
- 发现烟测断言依赖用户真实 recents 内容且无法改种隔离文件 → 停，上交清单候裁定。

## 领取

- 时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交：待回填

## 回执

- commit hash / run ID / 哈希对账 / 端测覆盖边界声明：待回填

## 验收

- 裁定文件 / 验收 commit：待回填
