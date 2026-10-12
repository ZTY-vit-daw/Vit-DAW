# GENSHELL-PCA-1：PCA 准入面（实施⑤）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡5）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 99；**规格全文=[件五 卡5](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P2 / 0.5-1 天 / GENSHELL-CPP-1（二进制可指纹即领）
- 模型分级：L1 / GLM 或 flash 执行会话

## 要点摘录（以规格为准）

generation_shell processor family 三处词表（processorintent/processorattestation v2/processorregistry，coverage v1=空集）+本地认证路径（FingerprintPath 对 GenShell.vst3）+装载门双向探针（promoted 放行/缺席 pca_load_gate 拒/UI 路径不受影响）。验收=真栈烟测+三处词表一致性单测+全量 0 FAIL。停止条件=CertifyProcessor 硬性要求非空工具对（local_processor_certification.go:234-247）→上交换方案不擅自造 typed apply。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
