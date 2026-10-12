# GEN-CTX-KEYFIELD-1 run 工件索引

- 卡片：coord/cards/done/2026-10-11-GEN-CTX-KEYFIELD-1.md（回执见卡内）
- 权威通过轮：`run4-pass-20261012-183348/`——`music_keyfield_roundtrip_smoke_mac.sh` exit 0，43/43 门绿。
  基线内核 sha256 `8e445dbb…`（DISCLOSE 时代内核基线），候选内核 sha256 `b2f53fcd…`（本卡增量重建）。
  §11 三验值链（含 set_at 逐字节）：`{"root_name": "A", "root_pitch": 9, "scale": "minor", "set_by": "user", "set_at": "20261012T183300.506+0800"}` 四段相同
  （set 回显 → set 后披露 → auto-persist 重开披露 → save_as 新路径重开披露）。
- 取证目录：`forensic-autopersist-and-var-ownership/`——run1（候选内核段错误，var 所有权双重持有）
  / run2（auto-persist 静默跳写）/ run3（内核修复确认，仅余脚本探针缺陷）的失败明细与三个隔离实验
  （含 set_tempo 同款缺陷回归证据与 .vit 明文降级证据），供决策侧处置域外引擎级缺陷时引用。

## 迭代史（四轮）

| 轮 | 内核 | 结果 | 定性 |
|---|---|---|---|
| run1 | f5c7c453（var 所有权 bug） | 段错误（b07 首次调 set_project_key 即崩） | 功能缺陷一：`juce::var(keyValue.get())` 临时 var 引用计数 0→1→0 把对象删掉，后续 release/UQ 析构全部落在已释放内存。修=release() 一次性移交所有权（仓内 response.release() 先例） |
| run2 | aa8256a0（所有权修复） | R2/R3 丢值（reload 后 null） | 功能缺陷二：裸 `edit->state.setProperty(...,nullptr)` 不注册变更 → `EditFileOperations::save` 的 `hasChangedSinceSaved()` 门为假 → auto-persist 静默跳写但回 true。修=`edit->markAsChanged()`（四变异点） |
| run3 | b2f53fcd（markAsChanged 修复） | 41 门绿，4 门红 | 内核侧修复确认（三验+负例+清链全过）；剩余 4 红门=冒测脚本 kfield 探针缺陷（eval 命名空间无 json → echo 侧恒 None）。修=kfield 注入 json |
| run4 | b2f53fcd | **exit 0，43/43 门绿** | 权威轮 |

run1 期间 agent 全量 go test 与冒测并发抢 CPU，phaseA teardown 拉长至 592s（SIGTERM:143:592）；
run4 同机静置复测 teardown 双阶段 SIGTERM:143:1——证明 run1 的 teardown 拉长属环境噪声（CPU 饱和），
非内核缺陷。环境中断与功能缺陷分开记录（AGENTS §8）。

## 域外引擎级缺陷上交（证据在 forensic 目录，决策侧处置）

1. **set_tempo 在 open 既有工程后同样静默不落盘**（probe_set_tempo_regression_console_transcript.txt：
   set_tempo 111 → ok → 重开 tempo=100）。锚点：CommandDispatcher.cpp `runAutoPersist` →
   VitHeadlessService.cpp `saveProjectToCurrentPathOrDefaultXml`(:688) → `ipcSaveProjectToCurrentPath`(:1355)
   → tracktion `EditFileOperations::save` 的 `hasChangedSinceSaved` 门（tracktion_EditFileOperations.cpp:278）。
   GEN-CTX-DISCLOSE-1 的 tempo 往返证明只走了 save_as 形态（editFileRetriever 未设 → saveAs else 分支
   无条件写），未覆盖 open_project 形态。本卡 markAsChanged 修复只覆盖本卡两命令；set_tempo 修复需另开卡。
2. **auto-persist 任何实际落盘都会把加密 .vit 重写为明文 XML**（vit_file_format_evidence.txt：
   probe.vit/probe3.vit 在 auto-persist 落盘后 file=XML）。Tracktion `EditFileOperations::writeToFile`
   走 `xml->writeTo(file)`；加密只存在于 VitHeadlessService 的 ipcSaveEncryptedProjectToPath（显式
   save_project/save_as 路径）。注释自证（VitHeadlessService.cpp:1218-1220 明言防"直接 Agent 保存
   静默降级为明文"）。此为全 auto-persist 命令面的系统性暴露，非本卡引入。
