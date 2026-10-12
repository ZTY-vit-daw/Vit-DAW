# Ruling：GEN-CTX-KEYFIELD-1 pass（2026-10-12，Mac 决策侧先行裁定）

- **裁定：pass**（调度件 005893b7 授权+PC 复核留档）。合入 main = cherry-pick 13551ab0（实现）；done 移动 bcfa809b 执行侧已直推。
- **决策侧独立复验（亲核，非转述）**：
  - 内核 diff 亲读（+274 行单文件 CommandDispatcher.cpp，lambda 注册守单文件域=append_ghost_track 先例）：keyfield 命名空间（11 scale 词表/音名或 0-127 整数根音解析/读侧 parse_error+raw 调试键**不静默纠错**）；set_project_key/set_project_style_note 全链 fail-closed（clear 互斥/四负例/500 字符不截断）；UAF 安全所有权（juce::var release() 一次性移交，注释锚定 run1 事故）；markAsChanged 四变异点（run2 静默跳写修复，注释锚定 set_tempo 同类潜在洞）；披露挂接替换 DISCLOSE 占位。
  - **双内核独立复跑 PASS exit 0**（20261012-184803 轮，复用 run_meta 同哈希双内核 8e445dbb/b2f53fcd）：§11 三验+四负例 fail-closed+清链幂等+tempo 族回归守门，脚本自 teardown。
  - 迭代史审阅：run1 UAF/run2 静默跳写=两个真内核缺陷自修（修复即锚定注释）；run3 脚本探针缺陷；失败分类如实。
  - agent 面：catalog 注册+单测随实现；集成态全量（含 genctx 共 94 包）exit 0（随本裁定链复跑，见验收提交）。
- **域外引擎级缺陷上交两件采纳并建议 PC 主管立卡**（非本卡引入，forensic 取证在案）：
  1. **set_tempo 开既有工程后静默不落盘**（set 111→ok→重开回 100；同根因=ValueTree 裸写不注册变更→hasChangedSinceSaved 假）——DISCLOSE 往返证明仅覆盖 save_as 形态，tempo 持久化在 open 形态有洞；影响统一化 hard 链可信度，建议 P2 内核修复卡。
  2. **auto-persist 实际落盘把加密 .vit 重写为明文 XML**（VitHeadlessService.cpp:1218 防降级注释自证只护显式保存路径）——系统性暴露，建议 PC 主管定级（涉数据格式与安全面，跨线）。
- **内核基线更新备忘**：Mac 内核新基线=含 keyfield（sha256 `b2f53fcd…`，/tmp/vit-ctxd1-build 增量重建；旧 8e445dbb 备份于 /private/tmp/vit-keyfield1-prep/）。
- 端测边界：内核直连 ZMQ 面+agent catalog 单测；无 webui/旅程改动——与卡面一致。
- 解锁：统一化链闭环（DISCLOSE→KEYFIELD→INJECT 三卡全合入）；CONSUME-1 与 GENSHELL-AGENT-1（依赖 INJECT）进入可领池。
