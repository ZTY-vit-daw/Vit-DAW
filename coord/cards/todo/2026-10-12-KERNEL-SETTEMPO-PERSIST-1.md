# KERNEL-SETTEMPO-PERSIST-1：set_tempo open 形态静默跳写修复（变更注册补齐）

- 发卡：GLM 主管决策侧 / 2026-10-12 晚窗（依据=GEN-CTX-KEYFIELD-1 域外上交证据+Mac 裁定 3bcaa39a 采纳「P2 修复卡建议」；[PC 复核留档](../../rulings/2026-10-12-PC-COUNTERSIGN-MAC2.md)）
- 派发确认：已确认（主管依已验收卡的上交换证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 95；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（内核卫生卡）
- 模型分级：L1 / GLM 执行会话（PC，内核 C++）

## 背景（上交证据摘要）

KEYFIELD 迭代史 run2 定位：**裸 `setProperty` 写 edit state 不注册变更→`EditFileOperations` 静默跳写**（不落盘）；同款缺陷存在于 set_tempo 的 open 形态（有值写入路径已注册，open/清除形态未注册——领取时以实际代码为准）。

## 目标

1. **同模式命令全清单化**：grep CommandDispatcher 全部写 edit state 的命令，列出「裸 setProperty 未注册变更」的命令/形态清单（不止 set_tempo）。
2. **修复**：清单内全部补 `markAsChanged`/等价变更注册（KEYFIELD 5ea95f75 同款修法）。
3. **回归门**：真栈冒测——set_tempo open 形态写后 save/reload 往返一致（§11 三验形态：set_at 逐字节比对），受影响命令逐条正例。

## 文件域

VitApp/Source/Service/CommandDispatcher.cpp（+冒测脚本扩展 SMOKE_TESTS 登记）；领取时与在飞卡核对（M2-TIMEFX-2 探针注册同文件不同区段，PROTOCOL §2.1）。

## 验收标准

内核构建 exit 0+真栈冒测 exit 0（往返一致正例+清单在卷）；域外发现新缺陷→上交不扩域。

## 停止条件

markAsChanged 语义与 5ea95f75 修法不符（内核已重构）→ 锚点上交。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
