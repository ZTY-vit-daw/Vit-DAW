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

- 领取：2026-10-25 晚窗 / origin/main=aa06fca7 / owner=GLM-5.3 L1 PC 执行会话（ZCode，SMOKE-STACK-GUARD-1 卡会话）/ 分支 port/smoke-stack-guard-1 / worktree D:\Vit_DAW_wt_guard1 / 领取提交见回执（本行随卡片 mv 提交）。领取时基线 `git rev-parse HEAD`=aa06fca7，主工作树 `git status --short` 仅本卡域外工件/运行时状态（VitApp/Workspace 运行时、coord/runs 工件、agent/webui/design 未跟踪设计稿），`git diff --stat` 领取前 scripts/ 零改动。领取前真栈核查：5555/5556/4445/7878/8787/18787 tcp listen=0 udp=0，VitApp/VitAgent/VspHub/Godot 进程=0。
- 回执：done 待验收（2026-10-25 21:2x）。实现 commit=**1d145979**（分支 port/smoke-stack-guard-1 已推 origin；worktree D:\Vit_DAW_wt_guard1）。两守卫证据（run=coord/runs/SMOKE-STACK-GUARD-1/guard1_20261025_2025/，RESULTS.md 全文）：①假监听 7878/5555 → agent/kernel 步守卫显式 throw exit≠0+指引文案在场（T1/T2b），真栈占用同形中止（T4，185 事故形态复刻拦截），`-AllowForeignStack` 复用路径走通 exit=0+双 FOREIGN warn（T5）；②GUI 腿隔离默认生效（T7b exitcode.txt=0）：VIT_PROJECT_XML 自动指 run 目录 git 干净副本、marker 时间线自洽（副本 21:14:08 被内核写=187 二轮同款证据形态）、主检出 default_project.xml 两轮 sha256 逐字节一致零触碰、探针 16/16。回归模式清单：T5 复用态默认腿族 exit=0+T7b 干净端口默认腿族 13 ok 行+exitcode=0（journey 零改动=diff 亲读，未重跑）。端测边界：本卡无 webui/Godot 渲染面改动（E2E/JOURNEY 层级不适用）；T3 退出码未捕获（driver 挂起+console 清理失误，T7b 补证）；t7 一轮 ps1 console 损失已披露并以 T7b 重跑补齐。真栈占用 73be5071（20:29 登记/20:46:39 即时复核）→已拆栈释放（复核六端口/UDP/四类进程=0）。
- 验收：**整卡通过**（GLM 主管决策流 2026-10-25 晚窗）。①diff 亲读：ps1 +136 行纯增量（守卫①外来 pid 中止+自启 pid 放行+-AllowForeignStack 显式放行带 owner 核对警告，指引文案引用 187×185 事故与 §2.2 增补；守卫②GUI 腿隔离默认+marker 落盘，显式传入优先，journey 逻辑不动）。②工件亲核：T1/T2b 假监听中止 console 在卷+T4 真栈中止+T5 放行 exit 0+T7b 隔离腿 16/16 exit 0+主检出 default_project.xml 两轮 sha256 指纹一致（零触碰亲证）。③两披露采信：driver 挂起（run 内工件非生产码，修复版随工件入库备查）+t3/t7 console 清理失误损失（T7b 同栈同腿族补齐）；journey 未重跑=diff 零触碰+理由成立。§2.2 增补条款自即日起有脚本级执行面。合入：主仓本 merge。
