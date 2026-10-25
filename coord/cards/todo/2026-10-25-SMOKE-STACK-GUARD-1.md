# SMOKE-STACK-GUARD-1：冒测脚本栈守卫+GUI 场景腿工程隔离默认

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=187×185 同刻双真栈互染事故裁定——ps1 "Start or reuse" 静默复用为碰撞放大器+SendRouteGuiSmoke 工程污染两伤（183 首轮/185 首轮）根因 [runs/M3-ENVELOPE-DATACHAN-1/RESULTS.md §五](../../runs/M3-ENVELOPE-DATACHAN-1/RESULTS.md)+[FE-FX-WETCHAIN-BADGE-1/ANALYSIS-stopcondition.md §四](../runs/FE-FX-WETCHAIN-BADGE-1/ANALYSIS-stopcondition.md)）
- 派发确认：已确认（主管事故裁定链，2026-10-25）
- 验收负责人：GLM 主管决策流
- 池序 188；目标仓库=主仓 D:\Vit_DAW（scripts/）；P3（流程卫生）/ 0.25 天 / 无依赖（领取时核对在飞卡文件域——187 已合入，scripts 无在飞占用）
- 模型分级：L1 / PC 执行会话

## 目标（两守卫）

1. **already-listening 不静默复用**：`dev_agent_smoke.ps1` 的 kernel/agent 启动步骤遇端口已被占用且占用进程**非本 run 自启**时，改为显式中止（throw，退出非 0，错误信息指引 PC-RUNTIME-STACK §2.2 占用面核对与属主确认）；新增 `-AllowForeignStack` 显式放行参数（取证/接管场景用，默认关）。
2. **GUI 场景腿工程隔离默认**：SendRouteGuiSmoke / AutomationDatachanGuiSmoke（及后续同族 GUI 场景腿）模式下，`VIT_PROJECT_XML` 未显式传入时默认指向**git 干净版 default_project.xml 的 run 目录隔离副本**（复刻 183/185 手工修正先例为脚本默认；显式传入优先）。journey berth 既有隔离逻辑不动。

## 文件域

`scripts/dev_agent_smoke.ps1`（仅此一件；README/注释随件）。越域即停。

## 验收标准

- 构造已占用端口场景（可用假监听进程）→ 脚本显式中止退出非 0+指引文案在场；带 `-AllowForeignStack` 复跑走通既有复用路径；
- GUI 场景腿隔离默认生效证据：跑一轮 SendRouteGuiSmoke（或 AutomationDatachanGuiSmoke），run 工件含隔离副本路径与被写时间戳（187 二轮同款证据形态），主检出 Workspace 零触碰，exit 0；
- 既有其余模式（journey/chat/strip 等）零行为改变（diff 亲读+至少一回归模式跑通）；工件入 coord/runs/SMOKE-STACK-GUARD-1/。

## 停止条件

ps1 启动步骤结构大改（守卫插入点失锚）→ 锚点上交。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 两守卫证据 / 回归模式清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
