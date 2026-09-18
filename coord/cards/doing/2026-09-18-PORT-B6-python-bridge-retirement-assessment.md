# PORT-B6：python bridge 退役评估（决策输入卡，零代码改动）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（层 B 开工前置件——B3 start_page.gd 平台分支的设计依赖本结论）
- 模型分级：L2 / GLM-5.3（职能对照与影响面判断）
- 目标：评估 python bridge 链的 mac 处置：**退役**（Go agent 已全覆盖职能）或 **mac 化保留**。链路事实（PORT_AUDIT §1.3(d) + 决策侧 2026-09-18 复核）：桥脚本 `scripts/bridge_core.py`/`bridge_prod.py`，由 Godot `app/startup/start_page.gd` 经 `python_embed/python.exe` 拉起并注册子进程看护；Go agent 侧对两文件零引用（grep 复核过，报告须独立再验证）。产出评估报告 `docs/PYTHON_BRIDGE_RETIREMENT_ASSESSMENT_2026-09.md`，四节：① bridge 实际职能清单（逐函数/逐端点，附代码锚点）；② Go agent 对照覆盖情况（已覆盖/部分/空白逐项）；③ mac 侧退役影响面（start_page.gd 链路改动点、发布打包剔除 python_embed 的连带、PC 侧是否受影响）；④ 建议（退役/保留）+ 理由与风险
- 文件域：新增 `docs/PYTHON_BRIDGE_RETIREMENT_ASSESSMENT_2026-09.md`（唯一产出文件）；**零代码改动**（只读分析）
- 验收标准：四节齐备且结论有代码锚点支撑；② 节对"Go 零引用"独立验证（grep 命令+输出入报告）；结论可直接支撑 B3 卡设计（退役 → B3 剔除该链；保留 → B3 含 python 运行时 mac 化子项）
- 停止条件：发现 bridge 存在 Go 侧未覆盖且无法判定可否迁移的运行时职能 → 上交证据由决策侧裁定，不自行下"可退役"结论
- 领取：2026-09-18 18:34 CST / origin/main=05ddda4f10bf8dee9a89384411b63236cd1ef50c / 分支 port/b6-python-bridge-assessment
- 回执：
- 验收：
