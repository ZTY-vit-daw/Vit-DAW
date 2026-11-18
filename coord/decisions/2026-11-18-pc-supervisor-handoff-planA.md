# 2026-11-18（晚）PC 主管交接与窗口线收口方案 A

## 用户指令（本会话，2026-11-18 18:1x）

- 指定本 DSH 会话（lead）接任 PC 主管/决策流，负责本轮窗口线收口的拆卡、派发、复核、验收与合入。
- 选择方案 A（完整 391 收口链），接受以 DeepSeek-V4.1-Flash 子代理为执行流、主管预审+细卡+机械移植的方式；今天剩余时间全权交由主管安排。
- 计划按**任务完成度**推进，不以时间为开发节点；用户会在关机前终止并告知，届时执行 gate。
- 其他 agent 不参与开发；仅疑难问题允许向更高模型征询——本会话工具实际可用模型仅 `deepseek-account/deepseek-flash`，无可征询的更高模型通道，按实记录，不假称已征询。

## 承接的既有裁定

- 409 `SHELL-WIN-ZPRODUCT-1` 的真实栈验收结论与 RULING 保持不变，不重开、不改判；前端 `bc8daf6`、主仓工具 `83bd5e77` 为现行产品窗口基线。
- 408 收口、用户 2026-11-17 三项裁定（z 事务路径 / 允许显示后一次纠正 / 原生缝进产品）继续有效。
- 391 父卡维持 blocked，解锁要件=395 兼容/呈现组合真栈 + 实际入口刷新；394/E2 失败候选封存不复活。
- 402（workspace_ready）与 395（呈现）不作为已完成卡处理；其已审 zero-stack 实现由本批 410/411/412 选择性移植/合入，最终以 414 组合真栈验收。

## 本批卡（410–414）

| 卡 | 目标 | 依赖 | 域 |
|---|---|---|---|
| 410 SHELL-WIN-PRODUCER-1 | 394-B/D 的 window_state + launch_progress/replay 移植到 409 基线 | 无 | 前端仓 VitShell |
| 411 SHELL-READY-SEAM-1 | 402 workspace_ready 接入 409 隐藏启动路径（legacy 不变） | 无 | 前端仓 app/vit_dock |
| 412 SHELL-WIN-PRES-MERGE-1 | 395 四提交合入 main，恢复 722 测试+渲染 | 无 | 主仓 webui |
| 413 SHELL-ENTRY-REFRESH-1 | 实际入口产物重建与哈希证据 | 410/411/412 合入 | 两仓主检出 |
| 414 SHELL-WIN-COMBO-1 | 391 组合真栈（409 窗口断言+395 呈现+396 retry 腿） | 410–413 | 真栈单 owner |

## 执行纪律

- 子代理只在独立 worktree/分支工作，不 push main、不启真栈、不碰主检出与其他卡域；主管复核 diff、亲跑验收命令后合入并推送。
- 真栈仅 414 一张卡占用，主管登记/释放并亲核；每 run 新目录、退出码与失败分类原样保留，recents/fixture/权威输入哈希亲证。
- 410/411 只准 cherry-pick/移植 394-B `e9006de`、394-D `7bd812f`、402 `b1f9406` 的对应实现，禁止带入 E/E2 hidden bootstrap、raw WinEvent、OBS 观察器或 409 以外的窗口行为；同确定性断点两次失败即停并上交。
- 本批不改变 409 结论、不触碰 271/397 等其它在飞/阻塞线；真栈与文件域冲突按 PROTOCOL §2.1/§2.2 处理。

## 本会话已做的同步证据

- 主仓：`git pull --ff-only --autostash` 后 HEAD=`738291f6`，本地既有 `CURRENT-STATE.md` 开放录音棚一行与 Settings/default_project/tracktion_engine 改动保留（autostash 已应用，content 双 hunk 在场）。
- 前端仓：`git pull --ff-only` 后 HEAD=`bc8daf6`，tracked clean。
- 真栈：18:0x 亲查 Vit/Godot/agent 进程 0、端口 0；资源记录末条 14:25 已释放。
